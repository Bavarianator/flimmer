package livetv

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ponytail: 3 gleichzeitige Kanäle, weil ein 2-Kern-NAS mehr nicht schafft; ein Kanal wird von allen Zuschauern geteilt.
// Konfigurierbar machen, sobald jemand stärkere Hardware hat.
const (
	maxStreams = 3
	idleAfter  = 30 * time.Second
	startWait  = 20 * time.Second
)

var errBusy = errors.New("alle Live-TV-Plätze sind belegt")

// stream ist ein laufender ffmpeg, der eine Quelle nach HLS remuxt. Er endet nach idleAfter ohne Abruf.
type stream struct {
	ch     Channel
	dir    string
	cancel context.CancelFunc
	done   chan struct{} // geschlossen, wenn ffmpeg beendet ist
	last   atomic.Int64  // UnixNano des letzten Abrufs
	err    string        // nach done gültig: letzte ffmpeg-Meldung ohne Adresse
}

func (s *stream) touch() { s.last.Store(time.Now().UnixNano()) }

// ready wartet, bis die Playlist das erste Segment nennt.
func (s *stream) ready(ctx context.Context) error {
	t := time.NewTicker(50 * time.Millisecond)
	defer t.Stop()
	timeout := time.After(startWait)
	for {
		if b, err := os.ReadFile(filepath.Join(s.dir, "index.m3u8")); err == nil && bytes.Contains(b, []byte(".ts")) {
			return nil
		}
		select {
		case <-s.done:
			return errors.New("ffmpeg beendet: " + s.err)
		case <-timeout:
			return errors.New("kein Bild nach " + startWait.String())
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

type tail struct {
	mu sync.Mutex
	b  []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.b = append(t.b, p...)
	if len(t.b) > 1000 {
		t.b = t.b[len(t.b)-1000:]
	}
	return len(p), nil
}

// open liefert den laufenden Stream des Kanals oder startet ihn.
func (tv *TV) open(ch Channel) (*stream, error) {
	if s := tv.stream(ch.ID); s != nil {
		s.touch()
		return s, nil
	}
	video := tv.config(context.Background()).Video
	src, audio := bestVariant(ch.url, video) // fragt das Netz, deshalb vor der Sperre
	tv.smu.Lock()
	defer tv.smu.Unlock()
	if s := tv.streams[ch.ID]; s != nil {
		s.touch()
		return s, nil
	}
	if len(tv.streams) >= maxStreams {
		return nil, errBusy
	}
	dir := filepath.Join(tv.Dir, ch.ID)
	os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "ffmpeg", ffmpegArgs(src, audio, dir, video)...)
	msg := &tail{}
	cmd.Stderr = msg
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	s := &stream{ch: ch, dir: dir, cancel: cancel, done: make(chan struct{})}
	s.touch()
	tv.streams[ch.ID] = s
	go func() {
		err := cmd.Wait()
		msg.mu.Lock()
		s.err = strings.TrimSpace(strings.ReplaceAll(string(msg.b), ch.url, redact(ch.url)))
		msg.mu.Unlock()
		if err != nil && ctx.Err() == nil {
			log.Printf("Live-TV %s: ffmpeg endete: %v %s", ch.Name, err, s.err)
		}
		close(s.done)
		tv.smu.Lock()
		if tv.streams[ch.ID] == s {
			delete(tv.streams, ch.ID)
		}
		tv.smu.Unlock()
		os.RemoveAll(dir)
	}()
	return s, nil
}

func (tv *TV) stream(id string) *stream {
	tv.smu.Lock()
	defer tv.smu.Unlock()
	return tv.streams[id]
}

// reap beendet Streams, die niemand mehr abruft; beim Ende von ctx alle.
func (tv *TV) reap(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			tv.reapIdle(0)
			return
		case <-t.C:
			tv.reapIdle(idleAfter)
		}
	}
}

func (tv *TV) reapIdle(after time.Duration) {
	tv.smu.Lock()
	defer tv.smu.Unlock()
	for _, s := range tv.streams {
		if time.Since(time.Unix(0, s.last.Load())) >= after {
			s.cancel()
		}
	}
}

var hlsAttr = regexp.MustCompile(`([A-Z0-9-]+)=("[^"]*"|[^,]*)`)

// attrs zerlegt die Attributliste einer HLS-Zeile (#EXT-X-STREAM-INF:…, #EXT-X-MEDIA:…).
func attrs(line string) map[string]string {
	m := map[string]string{}
	for _, a := range hlsAttr.FindAllStringSubmatch(line[strings.IndexByte(line, ':')+1:], -1) {
		m[a[1]] = strings.Trim(a[2], `"`)
	}
	return m
}

// bestVariant wählt aus einer HLS-Hauptliste die Variante mit der höchsten Bitrate, bei video == "h264" höchstens
// 720p (mehr schafft x264 auf einem kleinen NAS nicht). ffmpeg nähme mit -map 0:v:0 die erste, bei ARD/ZDF 360p.
// audio ist die Tonspur, wenn die Hauptliste den Ton getrennt führt (#EXT-X-MEDIA TYPE=AUDIO mit URI, so bei
// ZDF/ARD): die Standardspur der Gruppe, sonst Deutsch, sonst die erste. Ohne sie liefe die Variante stumm.
// Keine Hauptliste oder ein Fehler: src bleibt, audio leer.
func bestVariant(src, video string) (best, audio string) {
	best = src
	if u, err := url.Parse(src); err != nil || !strings.HasPrefix(u.Scheme, "http") || !strings.HasSuffix(u.Path, ".m3u8") {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	abs := func(ref string) string {
		if v, err := resp.Request.URL.Parse(ref); err == nil {
			return v.String()
		}
		return ""
	}
	type ton struct{ uri, lang string }
	gruppen := map[string][]ton{} // Standardspur steht vorn
	standard := map[string]bool{}
	bestBW, bestGroup := -1, ""
	var inf map[string]string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "#EXT-X-MEDIA:"):
			if a := attrs(line); a["TYPE"] == "AUDIO" && a["URI"] != "" {
				t := ton{abs(a["URI"]), strings.ToLower(a["LANGUAGE"])}
				if a["DEFAULT"] == "YES" {
					standard[a["GROUP-ID"]] = true
					gruppen[a["GROUP-ID"]] = append([]ton{t}, gruppen[a["GROUP-ID"]]...)
				} else {
					gruppen[a["GROUP-ID"]] = append(gruppen[a["GROUP-ID"]], t)
				}
			}
		case strings.HasPrefix(line, "#EXT-X-STREAM-INF:"):
			inf = attrs(line)
		case inf != nil && line != "" && line[0] != '#':
			bw, _ := strconv.Atoi(inf["BANDWIDTH"])
			h := 0
			if r := inf["RESOLUTION"]; strings.Contains(r, "x") {
				h, _ = strconv.Atoi(r[strings.IndexByte(r, 'x')+1:])
			}
			if v := abs(line); v != "" && (video != "h264" || h <= 720) && bw > bestBW {
				best, bestBW, bestGroup = v, bw, inf["AUDIO"]
			}
			inf = nil
		}
	}
	if t := gruppen[bestGroup]; len(t) > 0 {
		audio = t[0].uri
		if !standard[bestGroup] { // keine Standardspur: lieber Deutsch als Audiodeskription o. Ä.
			for _, x := range t {
				if x.lang == "deu" || x.lang == "ger" || x.lang == "de" {
					audio = x.uri
					break
				}
			}
		}
	}
	return
}

// ffmpegArgs remuxt nach HLS mit kurzen Segmenten. Video wird kopiert (schneller Start, kaum Last) oder mit
// video == "h264" entschachtelt und neu kodiert (für MPEG-2 und interlacte Sender im Browser).
// ponytail: x264 in Software und nur die erste Tonspur; Hardware-Encoder aus internal/hwaccel und Tonwahl später.
func ffmpegArgs(src, audio, dir, video string) []string {
	a := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-fflags", "+genpts+discardcorrupt",
		"-analyzeduration", "1000000", "-probesize", "1000000"}
	if u, err := url.Parse(src); err == nil && strings.HasPrefix(u.Scheme, "http") {
		a = append(a, "-rw_timeout", "10000000") // hängende Quelle nach 10 s abbrechen
	}
	a = append(a, "-protocol_whitelist", "http,https,tcp,tls,udp,rtp,rtsp,rtmp,crypto", "-i", src)
	if audio != "" { // Ton kommt aus eigener Playlist (HLS mit getrennten Tonspuren)
		a = append(a, "-rw_timeout", "10000000", "-protocol_whitelist", "http,https,tcp,tls,crypto", "-i", audio,
			"-map", "0:v:0?", "-map", "1:a:0",
			"-copyts", "-start_at_zero") // Zeitstempel der Quellen behalten: Bild und Ton bleiben synchron
	} else {
		a = append(a, "-map", "0:v:0?", "-map", "0:a:0?")
	}
	a = append(a, "-sn", "-dn")
	if video == "h264" {
		a = append(a, "-vf", "yadif=deint=interlaced", "-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
			"-pix_fmt", "yuv420p", "-force_key_frames", "expr:gte(t,n_forced*2)")
	} else {
		a = append(a, "-c:v", "copy")
	}
	return append(a, "-c:a", "aac", "-b:a", "160k", "-ac", "2",
		"-f", "hls", "-hls_time", "2", "-hls_init_time", "1", "-hls_list_size", "6",
		"-hls_flags", "delete_segments+omit_endlist", "-hls_segment_filename", filepath.Join(dir, "s%d.ts"),
		filepath.Join(dir, "index.m3u8"))
}
