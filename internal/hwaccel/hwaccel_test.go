package hwaccel

import (
	"context"
	"fmt"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fake simuliert ffmpeg: listet encoders und lässt nur Testencodes mit einem der working-Encoder gelingen.
func fake(t *testing.T, encoders string, working ...string) {
	t.Helper()
	orig := run
	t.Cleanup(func() { run = orig })
	run = func(ctx context.Context, args ...string) ([]byte, error) {
		if slices.Contains(args, "-encoders") {
			return []byte(encoders), nil
		}
		i := slices.Index(args, "-c:v")
		if i < 0 {
			return nil, errors.New("kein Encoder")
		}
		enc := args[i+1]
		if enc == "libx264" && slices.Contains(args, "lavfi") { // Testclip erzeugen
			return nil, nil
		}
		if !slices.Contains(working, enc) {
			return []byte("Device creation failed"), errors.New("exit status 1")
		}
		return []byte("frame=75\nspeed=2.5x\nprogress=continue\nframe=75\nspeed=4.25x\nprogress=end\n"), nil
	}
}

func device(t *testing.T, exists bool) {
	orig := RenderDevice
	t.Cleanup(func() { RenderDevice = orig })
	RenderDevice = filepath.Join(t.TempDir(), "renderD128")
	if exists {
		os.WriteFile(RenderDevice, nil, 0o644)
	}
}

const allEncoders = " V....D h264_nvenc  x\n V..... h264_qsv  x\n V....D h264_vaapi  x\n V..... h264_v4l2m2m  x\n V....D libx264  x\n"

func TestDetect(t *testing.T) {
	tests := []struct {
		name     string
		encoders string
		working  []string
		device   bool
		want     string
	}{
		{"NVENC gewinnt", allEncoders, []string{"h264_nvenc", "h264_vaapi", "libx264"}, true, "nvenc"},
		{"NVENC eingebaut, aber keine Karte", allEncoders, []string{"h264_vaapi", "libx264"}, true, "vaapi"},
		{"VAAPI ohne Render-Node übersprungen", allEncoders, []string{"h264_vaapi", "h264_v4l2m2m", "libx264"}, false, "v4l2m2m"},
		{"Encoder fehlt in der Liste", " V....D libx264  x\n", []string{"h264_vaapi", "libx264"}, true, "software"},
		{"nichts funktioniert", allEncoders, []string{"libx264"}, true, "software"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake(t, tt.encoders, tt.working...)
			device(t, tt.device)
			a := Detect(context.Background())
			if a.Name != tt.want {
				t.Fatalf("Name = %s, will %s", a.Name, tt.want)
			}
			if a.Speed != 4.25 {
				t.Errorf("Speed = %v, will letzten Wert 4.25", a.Speed)
			}
			if a.Name == "vaapi" && !slices.Contains(a.Input, RenderDevice) {
				t.Errorf("Input ohne Render-Node: %v", a.Input)
			}
		})
	}
}

// transcode setzt Keyframes selbst (-force_key_frames); eigene GOP-Optionen würden das aushebeln.
func TestKeineGOPOptionen(t *testing.T) {
	for _, c := range append(candidates(), candidate{Accel: software}) {
		for _, arg := range append(c.Input, c.Encode...) {
			if slices.Contains([]string{"-g", "-keyint_min", "-force_key_frames", "-sc_threshold", "-forced-idr"}, arg) || strings.Contains(arg, "keyint") {
				t.Errorf("%s setzt %s", c.Name, arg)
			}
		}
	}
}

// Echte Hardware prüfen: FLIMMER_HW=1 go test -run TestEcht -v ./internal/hwaccel
func TestEcht(t *testing.T) {
	if os.Getenv("FLIMMER_HW") == "" {
		t.Skip("nur mit FLIMMER_HW=1")
	}
	a := Detect(context.Background())
	t.Logf("%s, %.2fx Echtzeit (1080p)\nInput:  %v\nEncode: %v", a.Name, a.Speed, a.Input, a.Encode)
}

// 3-s-Clip, 1 s GPU-Initialisierung, danach 2× Echtzeit: ffmpegs speed= fällt auf 1,2×, gemeint sind 2×.
func TestSteadySpeed(t *testing.T) {
	var b strings.Builder
	for _, wall := range []float64{1.25, 1.5, 2.0, 2.5} {
		media := (wall - 1) * 2
		fmt.Fprintf(&b, "frame=1\nout_time_us=%d\nspeed=%.3fx\nprogress=continue\n", int(media*1e6), media/wall)
	}
	if got := steadySpeed([]byte(b.String())); got < 1.95 || got > 2.05 {
		t.Errorf("steadySpeed = %.2f, will 2", got)
	}
	if got := steadySpeed([]byte("speed=0.8x\n")); got != 0.8 {
		t.Errorf("Rückfall auf speed= : %.2f", got)
	}
}
