// Package hwaccel findet den schnellsten funktionierenden H.264-Encoder per echtem Testencode.
// Encoder-Listen lügen (Distributionen bauen NVENC/QSV ohne passende Hardware ein), deshalb zählt nur,
// ob ffmpeg einen Clip tatsächlich dekodieren und kodieren kann.
package hwaccel

import (
	"bufio"
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Accel struct {
	Name   string   // vaapi, qsv, nvenc, videotoolbox, v4l2m2m, software
	Input  []string // Args vor -i
	Encode []string // Args nach -i: Encoder, Profil, Qualität, Pixelformat; keine GOP-/Keyframe-Optionen
	Speed  float64  // Echtzeit-Faktor beim Testencode (1080p), 0 = unbekannt
}

// RenderDevice ist die DRM-Render-Node für VAAPI/QSV.
// ponytail: nur die erste Node; bei mehreren GPUs per Flag wählbar machen.
var RenderDevice = "/dev/dri/renderD128"

// run ist in Tests austauschbar.
var run = func(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "ffmpeg", args...).CombinedOutput()
}

type candidate struct {
	Accel
	encoder string // muss in `ffmpeg -encoders` auftauchen
	device  bool   // braucht RenderDevice
}

// Reihenfolge = Vorrang. V4L2 (Raspberry Pi) nur als Notlösung vor Software.
// ponytail: HW-Decode mit hw-Frames scheitert, wenn die GPU den Quell-Codec nicht dekodiert (z. B. HEVC 10 bit
// auf alter Hardware); dann pro Datei auf software ausweichen, sobald transcode Fehler zurückmeldet.
func candidates() []candidate {
	return []candidate{
		{Accel: Accel{Name: "nvenc",
			Input:  []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"},
			Encode: []string{"-vf", "scale_cuda=format=yuv420p", "-c:v", "h264_nvenc", "-preset", "p4", "-profile:v", "high", "-rc", "vbr", "-cq", "23"},
		}, encoder: "h264_nvenc"},
		{Accel: Accel{Name: "qsv",
			Input:  []string{"-hwaccel", "qsv", "-hwaccel_output_format", "qsv", "-qsv_device", RenderDevice},
			Encode: []string{"-vf", "scale_qsv=format=nv12", "-c:v", "h264_qsv", "-profile:v", "high", "-global_quality", "23"},
		}, encoder: "h264_qsv", device: true},
		{Accel: Accel{Name: "vaapi",
			Input:  []string{"-hwaccel", "vaapi", "-hwaccel_output_format", "vaapi", "-vaapi_device", RenderDevice},
			Encode: []string{"-vf", "scale_vaapi=format=nv12", "-c:v", "h264_vaapi", "-profile:v", "high", "-qp", "23"},
		}, encoder: "h264_vaapi", device: true},
		{Accel: Accel{Name: "videotoolbox",
			Input:  []string{"-hwaccel", "videotoolbox"},
			Encode: []string{"-c:v", "h264_videotoolbox", "-profile:v", "high", "-pix_fmt", "yuv420p", "-b:v", "8M"},
		}, encoder: "h264_videotoolbox"},
		{Accel: Accel{Name: "v4l2m2m",
			Encode: []string{"-c:v", "h264_v4l2m2m", "-pix_fmt", "yuv420p", "-b:v", "8M"},
		}, encoder: "h264_v4l2m2m"},
	}
}

const clipSeconds = 3

var software = Accel{Name: "software",
	Encode: []string{"-c:v", "libx264", "-preset", "veryfast", "-profile:v", "high", "-pix_fmt", "yuv420p", "-crf", "23"}}

// EncoderNamen: erlaubte Werte der Dashboard-Einstellung „Encoder erzwingen“ (leer = auto).
var EncoderNamen = []string{"auto", "software", "nvenc", "qsv", "vaapi", "videotoolbox", "v4l2m2m"}

// ValidEncoder meldet, ob das Dashboard diesen Encoder erzwingen darf.
func ValidEncoder(name string) bool {
	return slices.Contains(EncoderNamen, name)
}

// Detect testet die Kandidaten der Reihe nach und liefert den ersten, der funktioniert; sonst libx264.
// Dauert je nach Hardware einige Sekunden, also einmal beim Start aufrufen.
func Detect(ctx context.Context) Accel {
	encoders, clip, cleanup := prepare(ctx)
	if encoders == nil {
		return software
	}
	defer cleanup()
	if a, ok := detect(ctx, encoders, clip, func(candidate) bool { return true }); ok {
		return a
	}
	sw := software
	sw.Speed, _ = measure(ctx, sw, clip)
	return sw
}

// DetectForced testet nur den Encoder prefer – die Dashboard-Einstellung „Encoder erzwingen“.
// „auto“/„software“ zählen mit; passt ein erzwungener Encoder nicht (Gerät fehlt, Test bricht ab),
// greift die automatische Wahl, damit der Server weiter funktioniert.
func DetectForced(ctx context.Context, prefer string) Accel {
	switch {
	case prefer == "" || prefer == "auto":
		return Detect(ctx)
	case prefer == "software":
		a := software
		_, clip, cleanup := prepare(ctx)
		if clip != "" {
			defer cleanup()
			a.Speed, _ = measure(ctx, a, clip)
		}
		return a
	case !ValidEncoder(prefer):
		log.Printf("Encoder %q ist unbekannt – automatische Wahl", prefer)
		return Detect(ctx)
	}
	encoders, clip, cleanup := prepare(ctx)
	if encoders == nil {
		return software
	}
	defer cleanup()
	if a, ok := detect(ctx, encoders, clip, func(c candidate) bool { return c.Name == prefer }); ok {
		return a
	}
	log.Printf("Encoder %s erzwungen, funktioniert aber nicht – automatische Wahl", prefer)
	return Detect(ctx)
}

// prepare erzeugt den Testclip (echter H.264-Clip statt lavfi-Quelle, damit auch der HW-Decoder
// mitgetestet wird – wie beim echten Transcoding) und listet ffmpegs Encoder auf.
// encoders == nil bedeutet: ffmpeg läuft nicht, keinerlei Beschleunigung möglich.
func prepare(ctx context.Context) (encoders []byte, clip string, cleanup func()) {
	encoders, _ = run(ctx, "-hide_banner", "-encoders")
	dir, err := os.MkdirTemp("", "flimmer-hwaccel")
	if err != nil {
		return nil, "", func() {}
	}
	cleanup = func() { os.RemoveAll(dir) }
	clip = filepath.Join(dir, "clip.mp4")
	if _, err := run(ctx, "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi",
		"-i", "testsrc2=size=1920x1080:rate=25:duration="+strconv.Itoa(clipSeconds),
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", clip); err != nil {
		return nil, "", cleanup
	}
	return encoders, clip, cleanup
}

// detect testet die Kandidaten, auf die want zutrifft, und liefert den ersten funktionierenden.
func detect(ctx context.Context, encoders []byte, clip string, want func(candidate) bool) (Accel, bool) {
	for _, c := range candidates() {
		if !want(c) {
			continue
		}
		if !bytes.Contains(encoders, []byte(" "+c.encoder+" ")) {
			continue
		}
		if c.device {
			if _, err := os.Stat(RenderDevice); err != nil {
				continue
			}
		}
		if speed, ok := measure(ctx, c.Accel, clip); ok {
			c.Speed = speed
			return c.Accel, true
		}
	}
	return Accel{}, false
}

// measure kodiert den Clip mit a und liefert den Echtzeit-Faktor.
func measure(ctx context.Context, a Accel, clip string) (float64, bool) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	args := append([]string{"-hide_banner", "-loglevel", "error", "-nostats", "-progress", "pipe:1", "-stats_period", "0.25"}, a.Input...)
	args = append(args, "-i", clip)
	args = append(args, a.Encode...)
	args = append(args, "-an", "-f", "null", "-")
	start := time.Now()
	out, err := run(ctx, args...)
	if err != nil {
		return 0, false
	}
	if s := steadySpeed(out); s > 0 {
		return s, true
	}
	return clipSeconds / time.Since(start).Seconds(), true
}

// steadySpeed liefert den Echtzeit-Faktor ohne Anlaufkosten (GPU-Initialisierung, Prozessstart), die ffmpegs
// eigenes speed= (Medienzeit / Wanduhr seit Start) bei kurzen Clips stark drücken: Aus jeder Progress-Meldung
// folgt die Wanduhr als out_time/speed; die Steigung zwischen erster und letzter Meldung ist der Dauerbetrieb.
// Zu wenige Meldungen → letztes speed=.
func steadySpeed(out []byte) float64 {
	type point struct{ media, wall float64 }
	var pts []point
	var media float64
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		k, v, _ := strings.Cut(sc.Text(), "=")
		switch k {
		case "out_time_us":
			media, _ = strconv.ParseFloat(v, 64)
			media /= 1e6
		case "speed":
			if sp, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(v, "x")), 64); err == nil && sp > 0 && media > 0 {
				pts = append(pts, point{media, media / sp})
			}
		}
	}
	if len(pts) >= 2 {
		first, last := pts[0], pts[len(pts)-1]
		if dw := last.wall - first.wall; dw > 0.2 {
			return (last.media - first.media) / dw
		}
	}
	return lastSpeed(out)
}

// lastSpeed liest den letzten „speed=1.23x“-Wert aus ffmpegs -progress-Ausgabe.
func lastSpeed(out []byte) float64 {
	var speed float64
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "speed="); ok {
			if f, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(v, "x")), 64); err == nil {
				speed = f
			}
		}
	}
	return speed
}
