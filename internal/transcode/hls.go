// Package transcode erzeugt HLS aus beliebigen Dateien. Segmentgrenzen liegen auf echten Keyframes,
// damit auch bei kopiertem Video (Remux) sofort eine vollständige Playlist existiert und Spulen sauber klappt.
package transcode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
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
	Input      string
	Segments   []Segment
	AudioIndex int    // -1 = kein Ton
	AudioCodec string // copy, aac, eac3
	VideoCodec string // copy, h264
	Encoder    []string
}

func (j Job) Key() string {
	return fmt.Sprintf("%s|%d|%s|%s", j.Input, j.AudioIndex, j.AudioCodec, j.VideoCodec)
}

// Ein Session-Prozess erzeugt fortlaufend Segmente ab einem Startsegment – kontinuierlich,
// damit der Ton an Segmentgrenzen keine Lücken/Knackser bekommt.
type session struct {
	job      Job
	dir      string
	first    int
	cmd      *exec.Cmd
	done     chan struct{}
	lastUsed time.Time
	paused   bool
}

type Manager struct {
	TempDir string

	mu       sync.Mutex
	sessions map[string]*session
}

const (
	ahead    = 8                // so viele Segmente darf ffmpeg dem Player vorauseilen, dann wird pausiert
	restart  = 3                // liegt das gewünschte Segment weiter als das vorne, neu starten (Spulen)
	idleKill = 60 * time.Second // ungenutzte Sessions beenden
)

func NewManager(tempDir string) *Manager {
	m := &Manager{TempDir: tempDir, sessions: map[string]*session{}}
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
			if n < s.first || n > produced+restart {
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
	s := &session{job: job, dir: dir, first: first, done: make(chan struct{}), lastUsed: time.Now()}
	s.cmd = exec.Command("ffmpeg", args(job, first, dir)...)
	s.cmd.Stderr = &tailWriter{}
	if err := s.cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, err
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

func args(job Job, first int, dir string) []string {
	segs := job.Segments[first:]
	a := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	if first > 0 {
		a = append(a, "-ss", ftoa(segs[0].Start))
	}
	a = append(a, "-copyts", "-i", job.Input, "-map", "0:v:0?")
	if job.AudioIndex >= 0 {
		a = append(a, "-map", "0:"+strconv.Itoa(job.AudioIndex))
	}
	a = append(a, "-sn", "-dn", "-map_chapters", "-1", "-map_metadata", "-1")

	var bounds []string
	for _, s := range segs[1:] {
		bounds = append(bounds, ftoa(s.Start))
	}
	if job.VideoCodec == "copy" {
		a = append(a, "-c:v", "copy")
	} else {
		enc := job.Encoder
		if len(enc) == 0 {
			// ponytail: nur Software-x264; HW-Encoder (VAAPI/QSV/NVENC/V4L2) kommen mit internal/hwaccel.
			enc = []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p", "-profile:v", "high"}
		}
		a = append(a, enc...)
		if len(bounds) > 0 {
			a = append(a, "-force_key_frames", strings.Join(bounds, ","))
		}
	}
	switch job.AudioCodec {
	case "copy":
		a = append(a, "-c:a", "copy")
	case "eac3":
		a = append(a, "-c:a", "eac3", "-b:a", "640k", "-ac", "6")
	default:
		a = append(a, "-c:a", "aac", "-b:a", "192k", "-ac", "2")
	}
	a = append(a, "-f", "segment", "-segment_format", "mpegts", "-segment_start_number", strconv.Itoa(first))
	if len(bounds) > 0 {
		a = append(a, "-segment_times", strings.Join(bounds, ","))
	} else {
		a = append(a, "-segment_time", "100000")
	}
	return append(a, "-avoid_negative_ts", "disabled", filepath.Join(dir, "%d.ts"))
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

func (s *session) produced() int {
	n := s.first - 1
	for {
		if _, err := os.Stat(s.segPath(n + 1)); err != nil {
			return n
		}
		n++
	}
}

// throttle hält ffmpeg an, wenn es zu weit vorauseilt (spart CPU und Plattenplatz),
// und räumt bereits abgespielte Segmente weg.
func (s *session) throttle(requested, produced int) {
	for i := s.first; i < requested-2; i++ {
		os.Remove(s.segPath(i))
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
