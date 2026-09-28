// Package transcode erzeugt HLS aus beliebigen Dateien. Segmentgrenzen liegen auf echten Keyframes,
// damit auch bei kopiertem Video (Remux) sofort eine vollständige Playlist existiert und Spulen sauber klappt.
package transcode

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Bavarianator/flimmer/internal/audio"
	"github.com/Bavarianator/flimmer/internal/probe"
)

type Segment struct{ Start, End float64 }

const (
	target      = 6.0 // Sekunden pro Segment
	startTarget = 2.0 // die ersten Segmente kürzer → schneller Start
	startCount  = 3
)

// Segments gruppiert Keyframes zu Segmenten von ca. target Sekunden.
// Ohne Keyframes (z. B. reines Audio) wird in festen Abständen geschnitten.
func Segments(keyframes []float64, duration float64) []Segment {
	if len(keyframes) == 0 || keyframes[0] > 0.5 {
		keyframes = append([]float64{0}, keyframes...)
	}
	if len(keyframes) == 1 {
		for t := target; t < duration; t += target {
			keyframes = append(keyframes, t)
		}
	}
	var segs []Segment
	start := keyframes[0]
	for _, kf := range keyframes[1:] {
		want := target
		if len(segs) < startCount {
			want = startTarget
		}
		if kf-start >= want {
			segs = append(segs, Segment{start, kf})
			start = kf
		}
	}
	if duration > start {
		segs = append(segs, Segment{start, duration})
	} else if len(segs) > 0 {
		segs[len(segs)-1].End = duration
	}
	return segs
}

func Playlist(segs []Segment) string {
	var b strings.Builder
	maxDur := 0.0
	for _, s := range segs {
		maxDur = max(maxDur, s.End-s.Start)
	}
	fmt.Fprintf(&b, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n", int(maxDur+0.999))
	for i, s := range segs {
		fmt.Fprintf(&b, "#EXTINF:%.6f,\n%d.ts\n", s.End-s.Start, i)
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}

// Job beschreibt, was ffmpeg tun soll.
type Job struct {
	Input         string
	Segments      []Segment
	AudioIndex    int      // -1 = kein Ton
	AudioCodec    string   // copy, aac, eac3
	AudioChannels int      // Kanäle der Quellspur (für den Stereo-Downmix mit lauter Sprache), 0 = unbekannt
	Night         bool     // Nachtmodus: Dynamik komprimieren (nur beim Neukodieren)
	VideoCodec    string   // copy, h264, h264-sdr (mit Tone-Mapping)
	Height        int      // >0: beim Transcoding auf diese Höhe verkleinern
	InputArgs     []string // vor -i, z. B. Hardware-Decoding (hwaccel.Accel.Input)
	Encoder       []string // Video-Encoder (hwaccel.Accel.Encode); leer = libx264
}

func (j Job) Key() string {
	return fmt.Sprintf("%s|%d|%s|%s|%d|%v", j.Input, j.AudioIndex, j.AudioCodec, j.VideoCodec, j.Height, j.Night)
}

// ParseVideo zerlegt den Video-Teil der HLS-URL: "copy" oder "h264[-720|-1080][-sdr]".
// Die Höhe verkleinert (nie vergrößert), "-sdr" rechnet HDR per Tone-Mapping nach SDR um;
// dann ist codec "h264-sdr" (Software-Pfad, siehe VideoArgs).
func ParseVideo(v string) (codec string, height int, ok bool) {
	if v == "copy" {
		return v, 0, true
	}
	rest, ok := strings.CutPrefix(v, "h264")
	if !ok {
		return "", 0, false
	}
	codec = "h264"
	if r, sdr := strings.CutSuffix(rest, "-sdr"); sdr {
		codec, rest = "h264-sdr", r
	}
	switch rest {
	case "":
	case "-720":
		height = 720
	case "-1080":
		height = 1080
	default:
		return "", 0, false
	}
	return codec, height, true
}

// ParseAudio zerlegt den Ton-Teil der HLS-URL: "copy", "aac" oder "eac3", die beiden letzten optional mit
// "-night" (Nachtmodus). Kopierter Ton kann keinen Nachtmodus haben.
func ParseAudio(a string) (codec string, night bool, ok bool) {
	codec, night = strings.CutSuffix(a, "-night")
	switch {
	case codec == "aac", codec == "eac3":
		return codec, night, true
	case codec == "copy" && !night:
		return codec, false, true
	}
	return "", false, false
}

// Standard-Encoder ohne Hardware.
var software = []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p", "-profile:v", "high"}

// toneMap rechnet HDR10/HLG (PQ/HLG, BT.2020) nach SDR BT.709 um: linearisieren, Farbraum wechseln,
// Spitzlichter mit hable weich abrollen, zurück nach BT.709 8 bit. desat=0 behält die Farbsättigung.
// ponytail: nur Software (zscale); tonemap_vaapi/opencl wären auf neuer Hardware schneller, i965 kann es nicht.
const toneMap = "zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0," +
	"zscale=t=bt709:m=bt709:r=tv,format=yuv420p"

// VideoArgs liefert Filter und Encoder für eine H.264-Umwandlung: höchstens height Pixel hoch (0 = Originalhöhe,
// nie hochskaliert), bei HDR-Quelle (v.HDR != "") mit Tone-Mapping nach SDR. encode sind Encoder-Args
// (hwaccel.Accel.Encode oder eigene); nil = libx264. Tone-Mapping läuft in Software – Encoder, die Hardware-Frames
// erwarten (mit eigenem -vf wie scale_vaapi), werden dann durch libx264 ersetzt.
// Keine GOP-/Keyframe-Optionen: die setzt der Aufrufer (-force_key_frames bzw. Standard).
func VideoArgs(v *probe.Stream, height int, encode []string) []string {
	return videoArgs(v != nil && v.HDR != "", height, false, encode)
}

// hwInput: Hardware-Decoding ist aktiv (Frames evtl. im GPU-Speicher) – dann nie ein Software-Filter.
func videoArgs(sdr bool, height int, hwInput bool, encode []string) []string {
	enc := append([]string(nil), encode...)
	hwFilter := slices.Index(enc, "-vf")
	if len(enc) == 0 || sdr && hwFilter >= 0 {
		enc, hwFilter = append([]string(nil), software...), -1
	}
	var chain []string
	if hwFilter >= 0 { // Hardware-Frames: skalieren mit dem Filter des Backends (scale_vaapi=…, scale_cuda=…)
		f := enc[hwFilter+1]
		if name, opts, ok := strings.Cut(f, "="); ok && height > 0 && strings.HasPrefix(name, "scale_") {
			f = name + "=w=-2:h=min(" + strconv.Itoa(height) + "\\,ih):" + opts
		}
		chain = append(chain, f)
		enc = slices.Delete(enc, hwFilter, hwFilter+2)
	} else if height > 0 && !hwInput {
		chain = append(chain, "scale=-2:min("+strconv.Itoa(height)+"\\,ih)")
	}
	if sdr {
		chain = append(chain, toneMap)
	}
	if len(chain) == 0 {
		return enc
	}
	return append([]string{"-vf", strings.Join(chain, ",")}, enc...)
}

// Ein Session-Prozess erzeugt fortlaufend Segmente ab einem Startsegment – kontinuierlich,
// damit der Ton an Segmentgrenzen keine Lücken/Knackser bekommt.
type session struct {
	job      Job
	dir      string
	first    int
	head     int // höchstes geschriebenes Segment
	cleaned  int // Segmente darunter sind schon gelöscht
	cmd      *exec.Cmd
	done     chan struct{}
	lastUsed time.Time
	paused   bool
	slow     bool // Speed-Wächter: ffmpeg schafft keine Echtzeit
}

type Manager struct {
	TempDir string

	mu       sync.Mutex
	sessions map[string]*session
	swOnly   map[string]bool // Dateien, bei denen der HW-Pfad scheiterte (z. B. Codec nicht HW-dekodierbar)
	lowRes   map[string]bool // Dateien, die der Speed-Wächter auf 720p heruntergestuft hat
}

const (
	ahead    = 8                // so viele Segmente darf ffmpeg dem Player vorauseilen, dann wird pausiert
	restart  = 3                // liegt das gewünschte Segment weiter als das vorne, neu starten (Spulen)
	idleKill = 60 * time.Second // ungenutzte Sessions beenden
)

func NewManager(tempDir string) *Manager {
	m := &Manager{TempDir: tempDir, sessions: map[string]*session{}, swOnly: map[string]bool{}, lowRes: map[string]bool{}}
	go m.reaper()
	return m
}

// Segment liefert den Pfad einer fertigen Segmentdatei (wartet, bis ffmpeg sie geschrieben hat).
func (m *Manager) Segment(ctx context.Context, job Job, n int) (string, error) {
	if n < 0 || n >= len(job.Segments) {
		return "", fmt.Errorf("segment %d existiert nicht", n)
	}
	key := job.Key()
	for {
		m.mu.Lock()
		s := m.sessions[key]
		if s != nil {
			s.lastUsed = time.Now()
			produced := s.produced()
			if n < s.cleaned || n > produced+restart || s.slow {
				s.stop()
				s = nil
			} else {
				s.throttle(n, produced)
			}
		}
		if s == nil {
			var err error
			s, err = m.start(job, n)
			if err != nil {
				m.mu.Unlock()
				return "", err
			}
			m.sessions[key] = s
		}
		path := s.segPath(n)
		ready := s.ready(n)
		exited := s.exited()
		m.mu.Unlock()

		if ready {
			return path, nil
		}
		if exited {
			m.mu.Lock()
			retry := len(s.job.InputArgs)+len(s.job.Encoder) > 0 && !m.swOnly[job.Input]
			if retry {
				// ponytail: fällt pro Datei dauerhaft (bis Neustart) auf Software zurück.
				log.Printf("transcode: Hardware-Pfad für %s gescheitert, nutze Software", filepath.Base(job.Input))
				m.swOnly[job.Input] = true
				s.stop()
				delete(m.sessions, key)
			}
			m.mu.Unlock()
			if retry {
				continue
			}
			return "", fmt.Errorf("ffmpeg beendet, Segment %d fehlt", n)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-s.done:
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (m *Manager) start(job Job, first int) (*session, error) {
	dir, err := os.MkdirTemp(m.TempDir, "hls-")
	if err != nil {
		return nil, err
	}
	if m.swOnly[job.Input] {
		job.InputArgs, job.Encoder = nil, nil
	}
	if m.lowRes[job.Input] && job.VideoCodec != "copy" && job.Height == 0 {
		job.Height = 720
	}
	s := &session{job: job, dir: dir, first: first, head: first - 1, cleaned: first, done: make(chan struct{}), lastUsed: time.Now()}
	s.cmd = exec.Command("ffmpeg", args(job, first, dir)...)
	s.cmd.Stderr = &tailWriter{}
	var progress io.ReadCloser
	if job.VideoCodec != "copy" && job.Height == 0 {
		progress, _ = s.cmd.StdoutPipe()
	}
	if err := s.cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	if progress != nil {
		go m.watch(s, progress)
	}
	go func() {
		err := s.cmd.Wait()
		if err != nil && !s.killed() {
			fmt.Fprintf(os.Stderr, "ffmpeg: %v\n%s\n", err, s.cmd.Stderr.(*tailWriter).String())
		}
		close(s.done)
	}()
	return s, nil
}

// seekPad: ffmpeg zieht bei Formaten mit B-Frames ~0,13 s vom Sprungziel ab (siehe ffmpeg_demux.c)
// und landet sonst auf dem VORHERIGEN Keyframe. Etwas hinter den Keyframe zu springen trifft ihn genau.
const seekPad = 0.2

func args(job Job, first int, dir string) []string {
	segs := job.Segments[first:]
	a := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	if first > 0 {
		a = append(a, "-ss", ftoa(segs[0].Start+seekPad), "-noaccurate_seek")
	}
	a = append(a, job.InputArgs...)
	// -copyts: Zeitstempel bleiben die der Quelle – egal, ab welchem Segment ffmpeg gestartet wurde,
	// passen die Segmente nahtlos aneinander.
	a = append(a, "-copyts", "-i", job.Input, "-map", "0:v:0?")
	if job.AudioIndex >= 0 {
		a = append(a, "-map", "0:"+strconv.Itoa(job.AudioIndex))
	}
	a = append(a, "-sn", "-dn", "-map_chapters", "-1", "-map_metadata", "-1")

	// Der Segment-Muxer misst segment_times relativ zum ersten Paket und zählt ab 0 (unabhängig von
	// segment_start_number), -force_key_frames dagegen arbeitet auf den (absoluten) Ausgabe-Zeitstempeln.
	var rel, abs []string
	for _, s := range segs[1:] {
		rel = append(rel, ftoa(s.Start-segs[0].Start))
		abs = append(abs, ftoa(s.Start))
	}
	if job.VideoCodec == "copy" {
		a = append(a, "-c:v", "copy")
	} else {
		a = append(a, "-progress", "pipe:1", "-stats_period", "1")
		enc := job.Encoder
		if job.Height > 0 { // Herunterstufen soll auch bei SD-Quellen spürbar Rechenzeit sparen
			enc = append([]string(nil), enc...)
			if len(enc) == 0 {
				enc = append(enc, software...)
			}
			if i := slices.Index(enc, "-preset"); i >= 0 && enc[i+1] == "veryfast" {
				enc[i+1] = "ultrafast"
			}
		}
		a = append(a, videoArgs(job.VideoCodec == "h264-sdr", job.Height, len(job.InputArgs) > 0, enc)...)
		if len(abs) > 0 {
			a = append(a, "-force_key_frames", strings.Join(abs, ","))
		}
	}
	switch job.AudioCodec {
	case "copy":
		a = append(a, "-c:a", "copy")
	case "eac3":
		a = append(a, audioArgs(job, 6, "-c:a", "eac3", "-b:a", "640k")...)
	default:
		a = append(a, audioArgs(job, 2, "-c:a", "aac", "-b:a", "192k")...)
	}
	a = append(a, "-muxdelay", "0", "-muxpreload", "0",
		"-f", "segment", "-segment_format", "mpegts", "-segment_start_number", strconv.Itoa(first))
	if len(rel) > 0 {
		a = append(a, "-segment_times", strings.Join(rel, ","))
	} else {
		a = append(a, "-segment_time", "100000")
	}
	return append(a, "-avoid_negative_ts", "disabled", filepath.Join(dir, "%d.ts"))
}

// audioArgs: Stereo-Downmix mit angehobenem Center (Sprache verständlich) und Nachtmodus per audio.Filter;
// -ac bleibt als Absicherung, falls die Kanalzahl der Quelle unbekannt ist.
func audioArgs(job Job, out int, enc ...string) []string {
	if f := audio.Filter(job.AudioChannels, out, job.Night); f != "" {
		enc = append(enc, "-af", f)
	}
	return append(enc, "-ac", strconv.Itoa(out))
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', 6, 64) }

func (s *session) segPath(n int) string { return filepath.Join(s.dir, strconv.Itoa(n)+".ts") }

func (s *session) exited() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// Ein Segment ist fertig, wenn das nächste existiert oder ffmpeg sauber durch ist.
func (s *session) ready(n int) bool {
	if _, err := os.Stat(s.segPath(n)); err != nil {
		return false
	}
	if _, err := os.Stat(s.segPath(n + 1)); err == nil {
		return true
	}
	return s.exited() && s.cmd.ProcessState.Success()
}

// produced liefert das höchste bereits geschriebene Segment.
func (s *session) produced() int {
	for {
		if _, err := os.Stat(s.segPath(s.head + 1)); err != nil {
			return s.head
		}
		s.head++
	}
}

// throttle hält ffmpeg an, wenn es zu weit vorauseilt (spart CPU und Plattenplatz),
// und räumt bereits abgespielte Segmente weg.
func (s *session) throttle(requested, produced int) {
	for ; s.cleaned < requested-2; s.cleaned++ {
		os.Remove(s.segPath(s.cleaned))
	}
	if s.exited() {
		return
	}
	if produced-requested > ahead && !s.paused {
		s.paused = pause(s.cmd.Process)
	} else if produced-requested <= ahead/2 && s.paused {
		resume(s.cmd.Process)
		s.paused = false
	}
}

func (s *session) killed() bool { return s.dir == "" }

func (s *session) stop() {
	if s.paused {
		resume(s.cmd.Process)
	}
	dir := s.dir
	s.dir = ""
	s.cmd.Process.Kill()
	<-s.done
	os.RemoveAll(dir)
}

// Close beendet alle ffmpeg-Prozesse und räumt ihre Segmente weg (beim Herunterfahren).
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, s := range m.sessions {
		s.stop()
		delete(m.sessions, k)
	}
}

// watch ist der Speed-Wächter: Kommt ffmpeg nicht mit ≥1,05× Echtzeit hinterher, obwohl der Player
// wartet (nicht gedrosselt), wird die Datei für diese Laufzeit auf 720p heruntergestuft – lieber
// etwas unschärfer als Stocken.
var watchWindow = 6 * time.Second

func (m *Manager) watch(s *session, progress io.Reader) {
	var winStart time.Time
	var winOut, out float64
	sc := bufio.NewScanner(progress)
	for sc.Scan() {
		k, v, _ := strings.Cut(sc.Text(), "=")
		switch k {
		case "out_time_us":
			if us, err := strconv.ParseFloat(v, 64); err == nil {
				out = us / 1e6
			}
		case "progress":
			m.mu.Lock()
			paused, slow := s.paused, s.slow
			m.mu.Unlock()
			now := time.Now()
			if paused || winStart.IsZero() {
				winStart, winOut = now, out // gedrosselt zählt nicht
				continue
			}
			if slow || now.Sub(winStart) < watchWindow {
				continue
			}
			if speed := (out - winOut) / now.Sub(winStart).Seconds(); speed < 1.05 {
				log.Printf("transcode: %s nur %.2f× Echtzeit – wechsle auf 720p", filepath.Base(s.job.Input), speed)
				m.mu.Lock()
				s.slow = true
				m.lowRes[s.job.Input] = true
				m.mu.Unlock()
			}
			winStart, winOut = now, out
		}
	}
}

func (m *Manager) reaper() {
	for range time.Tick(10 * time.Second) {
		m.mu.Lock()
		for k, s := range m.sessions {
			if time.Since(s.lastUsed) > idleKill {
				s.stop()
				delete(m.sessions, k)
			}
		}
		m.mu.Unlock()
	}
}

// tailWriter behält nur das Ende der ffmpeg-Ausgabe für Fehlermeldungen.
type tailWriter struct {
	mu  sync.Mutex
	buf []byte
}

func (t *tailWriter) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > 4096 {
		t.buf = t.buf[len(t.buf)-4096:]
	}
	return len(p), nil
}

func (t *tailWriter) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}
