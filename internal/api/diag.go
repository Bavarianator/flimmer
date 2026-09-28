package api

import (
	"bytes"
	"net/http"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/flimmer-media/flimmer/internal/playback"
	"github.com/flimmer-media/flimmer/internal/update"
)

// LogRing behält die letzten Log-Zeilen für die Diagnose (log.SetOutput(io.MultiWriter(os.Stderr, ring))).
type LogRing struct {
	mu    sync.Mutex
	lines []string
	part  []byte
}

const ringMax = 200

func (l *LogRing) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.part = append(l.part, p...)
	for {
		i := bytes.IndexByte(l.part, '\n')
		if i < 0 {
			break
		}
		l.lines = append(l.lines, string(l.part[:i]))
		l.part = l.part[i+1:]
	}
	if len(l.lines) > ringMax {
		l.lines = append([]string(nil), l.lines[len(l.lines)-ringMax:]...)
	}
	return len(p), nil
}

func (l *LogRing) Lines() []string {
	if l == nil {
		return []string{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.lines...)
}

// stream merkt sich eine Wiedergabe; aktiv ist sie, solange der Player Daten holt.
type stream struct {
	User     string          `json:"user"`
	Title    string          `json:"title"`
	Device   string          `json:"device"`
	Method   playback.Method `json:"method"`
	Light    playback.Light  `json:"light"`
	Reasons  []string        `json:"reasons"`
	Started  time.Time       `json:"started"`
	LastSeen time.Time       `json:"lastSeen"`
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
	})
}
