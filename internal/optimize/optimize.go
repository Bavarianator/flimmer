// Package optimize erzeugt nachts für Titel, die auf einem bekannten Gerät Rot sind (Echtzeit-Transcoding
// zu langsam), eine MP4-Version (H.264, Ton AAC/AC3/EAC3, faststart) unter <data>/optimized/<id>.mp4.
// Der Medienordner wird nie beschrieben. Es läuft höchstens ein Job, mit nice/ionice. Abgebrochen wird bei
// aktiver Wiedergabe, bei knappem Speicher und wenn der Job nicht mehr ins Zeitfenster passt.
package optimize

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/flimmer-media/flimmer/internal/playback"
	"github.com/flimmer-media/flimmer/internal/probe"
	"github.com/flimmer-media/flimmer/internal/transcode"
)

// Item ist ein Titel aus dem Katalog.
type Item struct {
	ID    string
	Title string
	Path  string
	Media *probe.Media
}

type Options struct {
	Dir      string                                       // <data>/optimized
	FFmpeg   string                                       // Pfad zu ffmpeg, leer = "ffmpeg"
	Items    func(ctx context.Context) ([]Item, error)    // Katalog; Reihenfolge = Priorität (z. B. neueste zuerst)
	Profiles func(ctx context.Context) []playback.Profile // bekannte Geräte
	Speed    func() float64                               // hwaccel.Accel.Speed, 0 = unbekannt
	Busy     func() bool                                  // läuft gerade eine Wiedergabe?
	Window   func() (from, to int, on bool)               // volle Stunden, Standard 2–6 Uhr; on=false schaltet ab
	MinFree  uint64                                       // so viel muss frei bleiben, Standard 20 GB
}

// Status für die Einstellungsseite.
type Status struct {
	On        bool   `json:"on"`
	Window    string `json:"window"` // z. B. "2–6 Uhr"
	Current   *Job   `json:"current,omitempty"`
	Done      int    `json:"done"`    // fertige Versionen
	Pending   int    `json:"pending"` // Titel, die noch drankommen
	LastError string `json:"lastError,omitempty"`
}

type Job struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Percent float64 `json:"percent"`
}

type Optimizer struct {
	opts Options
	now  func() time.Time

	mu     sync.Mutex
	st     Status
	failed map[string]string // ID → Grund; ponytail: nur bis zum Neustart gemerkt, in die DB wenn das nervt
	hdr    map[string]bool   // ID → HDR-Quelle (Cache für hdrOf)
}

func New(opts Options) *Optimizer {
	if opts.FFmpeg == "" {
		opts.FFmpeg = "ffmpeg"
	}
	if opts.MinFree == 0 {
		opts.MinFree = 20 << 30
	}
	if opts.Window == nil {
		opts.Window = func() (int, int, bool) { return 2, 6, true }
	}
	if opts.Speed == nil {
		opts.Speed = func() float64 { return 0 }
	}
	if opts.Busy == nil {
		opts.Busy = func() bool { return false }
	}
	return &Optimizer{opts: opts, now: time.Now, failed: map[string]string{}, hdr: map[string]bool{}}
}

// Lookup liefert die optimierte Version eines Titels oder "", wenn es keine (aktuelle) gibt.
// Ist das Original neuer als die Version, gilt sie als veraltet.
func (o *Optimizer) Lookup(id, srcPath string) string {
	p := filepath.Join(o.opts.Dir, id+".mp4")
	opt, err := os.Stat(p)
	if err != nil {
		return ""
	}
	src, err := os.Stat(srcPath)
	if err != nil || src.ModTime().After(opt.ModTime()) {
		return ""
	}
	return p
}

func (o *Optimizer) Status() Status {
	o.mu.Lock()
	defer o.mu.Unlock()
	st := o.st
	from, to, on := o.opts.Window()
	st.On, st.Window = on, fmt.Sprintf("%d–%d Uhr", from, to)
	if st.Current != nil {
		c := *st.Current
		st.Current = &c
	}
	return st
}

// StatusHandler: GET, nur Admin.
func (o *Optimizer) StatusHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(o.Status())
}

// Run prüft jede Minute, ob ein Job laufen darf, bis ctx endet.
func (o *Optimizer) Run(ctx context.Context) {
	if err := os.MkdirAll(o.opts.Dir, 0o755); err != nil {
		o.setErr(err)
		return
	}
	for {
		o.tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
		}
	}
}

// tick startet höchstens einen Job und kehrt zurück, wenn er fertig oder abgebrochen ist.
func (o *Optimizer) tick(ctx context.Context) {
	from, to, on := o.opts.Window()
	now := o.now()
	if !on || !inWindow(now.Hour(), from, to) || o.opts.Busy() {
		return
	}
	todo, err := o.pending(ctx)
	if err != nil {
		o.setErr(err)
		return
	}
	if len(todo) == 0 {
		return
	}
	it := todo[0]
	err = o.encode(ctx, it, windowEnd(now, to))
	var slow errTooSlow
	switch {
	case err == nil:
		o.setErr(nil)
	case errors.As(err, &slow), errors.Is(err, errFailed):
		o.mu.Lock()
		o.failed[it.ID] = err.Error()
		o.mu.Unlock()
		o.setErr(fmt.Errorf("%s: %w", it.Title, err))
	default: // Wiedergabe, Speicher, Fenster zu Ende: später neu versuchen
		o.setErr(fmt.Errorf("%s: %w", it.Title, err))
	}
}

// pending sucht Titel, die auf mindestens einem Gerät Rot sind und noch keine aktuelle Version haben,
// und räumt Versionen weg, deren Titel es nicht mehr gibt oder die veraltet sind.
func (o *Optimizer) pending(ctx context.Context) ([]Item, error) {
	items, err := o.opts.Items(ctx)
	if err != nil {
		return nil, err
	}
	profiles := o.opts.Profiles(ctx)
	speed := o.opts.Speed()
	known := map[string]bool{}
	var todo []Item
	for _, it := range items {
		known[it.ID] = true
		if it.Media == nil || o.Lookup(it.ID, it.Path) != "" || o.failedReason(it.ID) != "" {
			continue
		}
		if !slices.ContainsFunc(profiles, func(p playback.Profile) bool { return playback.Decide(it.Media, p, speed).Light == playback.Red }) {
			continue
		}
		todo = append(todo, it)
	}
	entries, _ := os.ReadDir(o.opts.Dir)
	done := 0
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".mp4")
		if !ok {
			if strings.HasSuffix(e.Name(), ".mp4.part") { // Rest eines abgebrochenen Laufs (z. B. Absturz)
				os.Remove(filepath.Join(o.opts.Dir, e.Name()))
			}
			continue
		}
		if !known[id] {
			os.Remove(filepath.Join(o.opts.Dir, e.Name()))
			continue
		}
		done++
	}
	o.mu.Lock()
	o.st.Done, o.st.Pending = done, len(todo)
	o.mu.Unlock()
	return todo, nil
}

// hdrOf sagt, ob die Quelle HDR ist (dann Tone-Mapping nach SDR). Erst das Feld von probe (fängt auch
// Dolby Vision Profil 5 ohne Transfer-Tag), sonst – für Probe-Caches von vor dem Feld – color_transfer per ffprobe.
func (o *Optimizer) hdrOf(ctx context.Context, it Item) (bool, error) {
	if v := it.Media.First("video"); v != nil && v.HDR != "" {
		return true, nil
	}
	o.mu.Lock()
	v, ok := o.hdr[it.ID]
	o.mu.Unlock()
	if ok {
		return v, nil
	}
	trc, err := colorTransfer(ctx, it.Path)
	if err != nil { // nicht cachen: Laufwerk kann kurz weg sein
		return false, err
	}
	v = trc == "smpte2084" || trc == "arib-std-b67"
	o.mu.Lock()
	o.hdr[it.ID] = v
	o.mu.Unlock()
	return v, nil
}

// colorTransfer liest color_transfer des ersten Videostreams (in Tests austauschbar).
var colorTransfer = func(ctx context.Context, path string) (string, error) {
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=color_transfer", "-of", "csv=p=0", path).Output()
	return strings.TrimSpace(string(out)), err
}

func (o *Optimizer) failedReason(id string) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.failed[id]
}

func (o *Optimizer) setErr(err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.st.LastError = ""
	if err != nil {
		o.st.LastError = err.Error()
	}
}

var (
	errFailed = errors.New("ffmpeg ist gescheitert")
	errBusy   = errors.New("abgebrochen, weil gerade jemand schaut")
	errSpace  = errors.New("abgebrochen, weil der Speicherplatz knapp wird")
	errWindow = errors.New("abgebrochen, weil das Zeitfenster vorbei ist")
)

// checkEvery: so oft prüft ein laufender Job, ob er abbrechen muss (Tests verkürzen das).
var checkEvery = 5 * time.Second

type errTooSlow struct{ need time.Duration }

func (e errTooSlow) Error() string {
	return "passt nicht ins Zeitfenster (bräuchte noch ca. " + e.need.Round(time.Minute).String() + "); Zeitfenster vergrößern"
}

// encode erzeugt die Version für it; deadline ist das Ende des Zeitfensters.
func (o *Optimizer) encode(ctx context.Context, it Item, deadline time.Time) error {
	if free := diskFree(o.opts.Dir); free != 0 && free < o.opts.MinFree {
		return errSpace
	}
	hdr, err := o.hdrOf(ctx, it)
	if err != nil {
		return fmt.Errorf("HDR-Prüfung: %w", err) // nie ohne Tone-Mapping raten: blasse Version gälte als grün
	}
	part := filepath.Join(o.opts.Dir, it.ID+".mp4.part")
	defer os.Remove(part)

	jobCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	name, args := niceCmd(o.opts.FFmpeg, Args(it, hdr, part))
	cmd := exec.CommandContext(jobCtx, name, args...)
	var stderr tail
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	o.mu.Lock()
	o.st.Current = &Job{ID: it.ID, Title: it.Title}
	o.mu.Unlock()
	defer func() {
		o.mu.Lock()
		o.st.Current = nil
		o.mu.Unlock()
	}()
	if err := cmd.Start(); err != nil {
		return err
	}

	start := o.now()
	var mu sync.Mutex
	var pos float64 // Sekunden
	read := make(chan struct{})
	go func() {
		defer close(read)
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			if v, ok := strings.CutPrefix(sc.Text(), "out_time_us="); ok {
				us, _ := strconv.ParseFloat(v, 64)
				mu.Lock()
				pos = us / 1e6
				mu.Unlock()
				if it.Media.Duration > 0 {
					o.mu.Lock()
					if o.st.Current != nil {
						o.st.Current.Percent = min(100, 100*us/1e6/it.Media.Duration)
					}
					o.mu.Unlock()
				}
			}
		}
	}()
	go func() {
		t := time.NewTicker(checkEvery)
		defer t.Stop()
		for {
			select {
			case <-jobCtx.Done():
				return
			case <-t.C:
			}
			mu.Lock()
			p := pos
			mu.Unlock()
			if err := o.shouldStop(start, p, it.Media.Duration, deadline); err != nil {
				cancel(err)
				return
			}
		}
	}()
	<-read // erst lesen, dann Wait: Wait schließt die Pipe
	err = cmd.Wait()
	if cause := context.Cause(jobCtx); cause != nil && !errors.Is(cause, context.Canceled) {
		return cause
	}
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: %s", errFailed, strings.TrimSpace(stderr.String()))
	}
	return os.Rename(part, filepath.Join(o.opts.Dir, it.ID+".mp4"))
}

// shouldStop entscheidet während des Jobs über einen Abbruch (nil = weiter).
func (o *Optimizer) shouldStop(start time.Time, pos, dur float64, deadline time.Time) error {
	now := o.now()
	switch {
	case o.opts.Busy():
		return errBusy
	case diskFree(o.opts.Dir) != 0 && diskFree(o.opts.Dir) < o.opts.MinFree:
		return errSpace
	case !now.Before(deadline):
		return errWindow
	}
	// Nach 2 Minuten ist die Geschwindigkeit verlässlich genug für eine Hochrechnung.
	if el := now.Sub(start); el >= 2*time.Minute && pos > 0 && dur > pos {
		need := time.Duration((dur - pos) / pos * float64(el))
		if now.Add(need).After(deadline) {
			return errTooSlow{need}
		}
	}
	return nil
}

// Args baut den ffmpeg-Aufruf: Video H.264 (max. 1080p), Ton AAC/AC3/EAC3 bleibt, alles andere wird
// EAC3 (Surround) bzw. AAC (Stereo). Untertitel bleiben im Original, das Server-seitig ausgeliefert wird.
// Skalieren und HDR→SDR-Tone-Mapping kommen aus transcode.VideoArgs (dasselbe Rezept wie beim Streaming).
// ponytail: immer libx264; hwaccel-Encoder (VAAPI ~7× schneller auf dem Celeron) mit vault-0e einhängen, wenn Nächte zu kurz sind
func Args(it Item, hdr bool, out string) []string {
	a := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y", "-i", it.Path,
		"-map", "0:v:0", "-map", "0:a?", "-sn", "-dn", "-map_chapters", "-1"}
	var v probe.Stream
	if f := it.Media.First("video"); f != nil {
		v = *f
	}
	if hdr && v.HDR == "" {
		v.HDR = "hdr10" // alter Probe-Cache: ffprobe sagt PQ/HLG, das Feld fehlt noch
	}
	a = append(a, transcode.VideoArgs(&v, 1080, []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "21",
		"-pix_fmt", "yuv420p", "-profile:v", "high", "-level:v", "4.1"})...)
	for i, s := range it.Media.All("audio") {
		n := strconv.Itoa(i)
		switch {
		case s.Codec == "aac" || s.Codec == "ac3" || s.Codec == "eac3":
			a = append(a, "-c:a:"+n, "copy")
		case s.Channels > 2:
			a = append(a, "-c:a:"+n, "eac3", "-b:a:"+n, "640k", "-ac:a:"+n, "6")
		default:
			a = append(a, "-c:a:"+n, "aac", "-b:a:"+n, "192k", "-ac:a:"+n, "2")
		}
	}
	return append(a, "-movflags", "+faststart", "-progress", "pipe:1", "-f", "mp4", out)
}

// niceCmd senkt die Priorität: nice 19 und, unter Linux, ionice Idle-Klasse.
// ponytail: unter Windows läuft ffmpeg mit normaler Priorität; BELOW_NORMAL per CreationFlags, wenn es stört
func niceCmd(ffmpeg string, args []string) (string, []string) {
	if runtime.GOOS == "windows" {
		return ffmpeg, args
	}
	cmd := append([]string{ffmpeg}, args...)
	if p, err := exec.LookPath("ionice"); err == nil {
		cmd = append([]string{p, "-c", "3"}, cmd...)
	}
	if p, err := exec.LookPath("nice"); err == nil {
		cmd = append([]string{p, "-n", "19"}, cmd...)
	}
	return cmd[0], cmd[1:]
}

func inWindow(h, from, to int) bool {
	if from <= to {
		return h >= from && h < to
	}
	return h >= from || h < to // über Mitternacht, z. B. 22–6
}

// windowEnd ist der nächste Zeitpunkt nach now mit Stunde to:00.
func windowEnd(now time.Time, to int) time.Time {
	end := time.Date(now.Year(), now.Month(), now.Day(), to, 0, 0, 0, now.Location())
	if !end.After(now) {
		end = end.AddDate(0, 0, 1)
	}
	return end
}

// tail behält die letzten 2 KB von ffmpegs Fehlerausgabe.
type tail struct{ b bytes.Buffer }

func (t *tail) Write(p []byte) (int, error) {
	t.b.Write(p)
	if t.b.Len() > 2048 {
		rest := t.b.Bytes()[t.b.Len()-2048:]
		t.b = *bytes.NewBuffer(append([]byte(nil), rest...))
	}
	return len(p), nil
}

func (t *tail) String() string { return t.b.String() }
