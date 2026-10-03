package api

import (
	"context"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os/exec"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Bavarianator/flimmer/internal/playback"
	"github.com/Bavarianator/flimmer/internal/update"
)

// LogRing behält die letzten Log-Einträge für Diagnose und Dashboard und schreibt sie zusätzlich nach Out.
// main hängt ihn per slog.SetDefault(slog.New(ring.Handler())) ein; damit landet auch log.Printf hier (als Info).
type LogRing struct {
	Out io.Writer // z. B. os.Stderr; nil = nur Ringpuffer

	mu      sync.Mutex
	entries []LogEntry
}

type LogEntry struct {
	Time  time.Time         `json:"time"`
	Level string            `json:"level"` // debug, info, warn, error
	Msg   string            `json:"msg"`
	Attrs map[string]string `json:"attrs,omitempty"`
}

const ringMax = 1000

func (l *LogRing) add(e LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.Out != nil { // Format wie das log-Paket, damit die Konsole aussieht wie bisher
		var b strings.Builder
		b.WriteString(e.Time.Format("2006/01/02 15:04:05 "))
		if e.Level != "info" {
			b.WriteString(strings.ToUpper(e.Level) + " ")
		}
		b.WriteString(e.Msg)
		for _, k := range slices.Sorted(maps.Keys(e.Attrs)) {
			b.WriteString(" " + k + "=" + strconv.Quote(e.Attrs[k]))
		}
		b.WriteByte('\n')
		io.WriteString(l.Out, b.String())
	}
	l.entries = append(l.entries, e)
	if len(l.entries) > ringMax+ringMax/4 { // selten umkopieren
		l.entries = append([]LogEntry(nil), l.entries[len(l.entries)-ringMax:]...)
	}
}

// Entries liefert höchstens limit Einträge ab Stufe min, neueste zuerst.
func (l *LogRing) Entries(min slog.Level, limit int) []LogEntry {
	out := []LogEntry{}
	if l == nil {
		return out
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := len(l.entries) - 1; i >= 0 && len(out) < limit; i-- {
		var lv slog.Level
		if lv.UnmarshalText([]byte(l.entries[i].Level)) == nil && lv >= min {
			out = append(out, l.entries[i])
		}
	}
	return out
}

// Lines liefert die letzten 200 Einträge als Text, älteste zuerst (Diagnose).
func (l *LogRing) Lines() []string {
	list := l.Entries(slog.LevelDebug, 200)
	out := make([]string, 0, len(list))
	for i := len(list) - 1; i >= 0; i-- {
		e := list[i]
		out = append(out, e.Time.Format("2006/01/02 15:04:05 ")+e.Msg)
	}
	return out
}

func (l *LogRing) Handler() slog.Handler { return &ringHandler{ring: l} }

type ringHandler struct {
	ring   *LogRing
	attrs  []slog.Attr
	prefix string // aus WithGroup
}

func (h *ringHandler) Enabled(_ context.Context, lv slog.Level) bool { return lv >= slog.LevelInfo }

func (h *ringHandler) Handle(_ context.Context, r slog.Record) error {
	e := LogEntry{Time: r.Time, Level: strings.ToLower(r.Level.String()), Msg: r.Message}
	add := func(a slog.Attr) bool {
		if e.Attrs == nil {
			e.Attrs = map[string]string{}
		}
		e.Attrs[h.prefix+a.Key] = a.Value.Resolve().String()
		return true
	}
	for _, a := range h.attrs {
		add(a)
	}
	r.Attrs(add)
	h.ring.add(e)
	return nil
}

func (h *ringHandler) WithAttrs(as []slog.Attr) slog.Handler {
	c := *h
	for _, a := range as {
		a.Key = h.prefix + a.Key
		c.attrs = append(slices.Clip(c.attrs), a)
	}
	return &c
}

func (h *ringHandler) WithGroup(name string) slog.Handler {
	c := *h
	c.prefix += name + "."
	return &c
}

// stream merkt sich eine Wiedergabe; aktiv ist sie, solange der Player Daten holt.
// Die Felder ohne JSON-Namen füllen play und die progress-Herzschläge für die Sitzungen im Dashboard.
type stream struct {
	User     string          `json:"user"`
	Title    string          `json:"title"`
	Device   string          `json:"device"`
	Method   playback.Method `json:"method"`
	Light    playback.Light  `json:"light"`
	Reasons  []string        `json:"reasons"`
	Started  time.Time       `json:"started"`
	LastSeen time.Time       `json:"lastSeen"`

	color          int
	itemID, client string
	pos, dur       float64
	paused         bool
	beat           time.Time // letzter progress-Herzschlag
}

type streams struct {
	mu sync.Mutex
	m  map[string]*stream // Benutzer-ID|Titel-ID
}

func (s *streams) start(key string, st stream) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]*stream{}
	}
	st.Started, st.LastSeen = time.Now(), time.Now()
	s.m[key] = &st
	for k, v := range s.m { // Verlauf begrenzen
		if time.Since(v.LastSeen) > 24*time.Hour {
			delete(s.m, k)
		}
	}
}

// beat nimmt einen progress-Herzschlag auf; ohne play vorher (z. B. nach einem Neustart) reicht st als Grundlage.
// sek ist die gesehene Zeit seit dem vorigen Herzschlag (Statistik): 0 beim ersten, nach einer Pause und nach Lücken
// über 30 s. method ist leer, wenn play fehlte.
func (s *streams) beat(key string, st stream, pos, dur float64, paused bool) (sek float64, method playback.Method) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.m[key]
	if cur == nil {
		if s.m == nil {
			s.m = map[string]*stream{}
		}
		st.Started = time.Now()
		cur = &st
		s.m[key] = cur
	}
	now := time.Now()
	if d := now.Sub(cur.beat); !cur.beat.IsZero() && d < 30*time.Second && !paused && !cur.paused {
		sek = d.Seconds()
	}
	cur.pos, cur.dur, cur.paused = pos, dur, paused
	cur.beat, cur.LastSeen = now, now
	return sek, cur.Method
}

// sessions: Wiedergaben mit Herzschlag in den letzten 60 s, neueste zuerst.
func (s *streams) sessions() []sessionOut {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []sessionOut{}
	for key, st := range s.m {
		if time.Since(st.beat) > time.Minute {
			continue
		}
		out = append(out, sessionOut{ID: key, User: st.User, UserColor: st.color, Device: st.Device, Client: st.client,
			ItemID: st.itemID, Title: st.Title, Position: st.pos, Duration: st.dur, Paused: st.paused,
			Method: playbackMethod(st.Method), Light: st.Light, Reason: strings.Join(st.Reasons, ", "), Since: st.Started})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Since.After(out[j].Since) })
	return out
}

func (s *streams) touch(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st := s.m[key]; st != nil {
		st.LastSeen = time.Now()
	}
}

// list: aktive (letzte Minute) und kürzliche Wiedergaben, neueste zuerst.
func (s *streams) list() (active, recent []stream) {
	s.mu.Lock()
	defer s.mu.Unlock()
	active, recent = []stream{}, []stream{}
	for _, st := range s.m {
		if time.Since(st.LastSeen) < time.Minute {
			active = append(active, *st)
		} else {
			recent = append(recent, *st)
		}
	}
	byStart := func(l []stream) { sort.Slice(l, func(i, j int) bool { return l[i].Started.After(l[j].Started) }) }
	byStart(active)
	byStart(recent)
	return active, recent[:min(len(recent), 10)]
}

var ffmpegVersion = sync.OnceValue(func() string {
	out, err := exec.Command("ffmpeg", "-hide_banner", "-version").Output()
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimPrefix(line, "ffmpeg version ")
})

func (s *Server) diagnostics(w http.ResponseWriter, r *http.Request) {
	hw := s.hw()
	active, recent := s.streams.list()
	free, total := diskSpace(s.CacheDir)
	version := ""
	if s.FFmpeg.Load() {
		version = ffmpegVersion()
	}
	writeJSON(w, map[string]any{
		"version": update.Version, "os": runtime.GOOS + "/" + runtime.GOARCH, "cpus": runtime.NumCPU(),
		"ffmpeg": version, "hw": hw.Name, "hwSpeed": hw.Speed,
		"diskFree": free, "diskTotal": total, "cacheDir": s.CacheDir,
		"scan": s.Lib.Status(), "active": active, "recent": recent, "log": s.Log.Lines(),
		"db": s.DB.Maintenance(r.Context()),
	})
}
