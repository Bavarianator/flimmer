package probe

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseKeyframes(t *testing.T) {
	out := []byte("0.000000,K__\n0.041000,___\n4.004000,K_\n2.002000,K__\nN/A,K__\n")
	got := parseKeyframes(out)
	want := []float64{0, 2.002, 4.004}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// Fixtures: hdr10/hlg sind echte ffprobe-Ausgaben von x265-Clips; DV-Dateien lassen sich mit ffmpeg nicht
// erzeugen (kein Konfigurationsrecord beim Muxen), daher dieselbe Ausgabe mit dem dokumentierten DOVI-Record.
func TestHDR(t *testing.T) {
	for _, tt := range []struct {
		file            string
		hdr             string
		profile, compat int
	}{
		{"hdr10", "hdr10", 0, 0},
		{"hlg", "hlg", 0, 0},
		{"dv81", "dv", 8, 1},
		{"dv5", "dv", 5, 0},
		{"dv7", "dv", 7, 6},
	} {
		b, err := os.ReadFile("testdata/" + tt.file + ".json")
		if err != nil {
			t.Fatal(err)
		}
		m, err := parse(b)
		if err != nil {
			t.Fatal(err)
		}
		v := m.First("video")
		if v.HDR != tt.hdr || v.DVProfile != tt.profile || v.DVCompat != tt.compat || v.PixFmt != "yuv420p10le" {
			t.Errorf("%s: HDR=%q DV=%d/%d PixFmt=%s", tt.file, v.HDR, v.DVProfile, v.DVCompat, v.PixFmt)
		}
	}
}

func TestForcedUndSDH(t *testing.T) {
	b, err := os.ReadFile("testdata/subs.json") // echte ffprobe-Ausgabe: Spur 1 forced (ger), Spur 2 SDH (eng)
	if err != nil {
		t.Fatal(err)
	}
	m, err := parse(b)
	if err != nil {
		t.Fatal(err)
	}
	s := m.All("subtitle")
	if len(s) != 2 || !s[0].Forced || s[0].HearingImpaired || s[0].Language != "ger" || s[1].Forced || !s[1].HearingImpaired {
		t.Errorf("%+v", s)
	}
}

func TestParseExtras(t *testing.T) {
	e, err := parseExtras([]byte(`{"streams":[{"avg_frame_rate":"24000/1001"}],
		"chapters":[{"start_time":"0.000000","tags":{"title":"Anfang"}},{"start_time":"312.5","tags":{}}]}`))
	if err != nil || e.FPS != 23.976 || len(e.Chapters) != 2 || e.Chapters[0].Name != "Anfang" || e.Chapters[1].Start != 312.5 {
		t.Fatalf("%+v %v", e, err)
	}
}

func TestParseCrop(t *testing.T) {
	out := []byte("[Parsed_cropdetect_0 @ 0x1] x1:0 x2:1919 y1:142 y2:937 w:1920 h:796 x:0 y:142 pts:1 t:0.04 limit:0.094000 crop=1920:796:0:142\n" +
		"[Parsed_cropdetect_0 @ 0x1] x1:0 x2:1919 y1:140 y2:939 w:1920 h:800 x:0 y:140 pts:2 t:0.08 limit:0.094000 crop=1920:800:0:140\n" +
		"[out#0/null @ 0x2] video:1kB audio:0kB")
	r, ok := parseCrop(out)
	if !ok || r != (pixRect{0, 140, 1920, 800}) {
		t.Fatalf("parseCrop: %+v %v", r, ok)
	}
	if _, ok := parseCrop([]byte("keine Zeile")); ok {
		t.Error("ohne crop= darf nichts herauskommen")
	}
	// Vereinigung: 2,39:1 an zwei Stellen, an einer etwas höher → das größere Rechteck.
	c := cropOf([]pixRect{{0, 140, 1920, 800}, {0, 132, 1920, 816}, {0, 140, 1920, 800}}, 1920, 1080)
	if c == nil || c.X != 0 || c.Y != 0.1222 || c.W != 1 || c.H != 0.7556 {
		t.Errorf("Vereinigung: %+v", c)
	}
	if c := cropOf([]pixRect{{4, 2, 1912, 1076}}, 1920, 1080); c != nil {
		t.Errorf("≥ 98 %% in beiden Richtungen ist kein Balken: %+v", c)
	}
	if c := cropOf([]pixRect{{600, 300, 700, 400}}, 1920, 1080); c != nil {
		t.Errorf("dunkle Szene (< 50 %% Fläche): %+v", c)
	}
}

// Ein 320×136-Bild mit schwarzem Rand oben und unten auf 320×240 (Letterbox wie bei 2,35:1 in 4:3).
func TestCropClip(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg fehlt")
	}
	clip := filepath.Join(t.TempDir(), "balken.mkv")
	if b, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x136:rate=25:duration=10,pad=320:240:0:52:black",
		"-c:v", "libx264", clip).CombinedOutput(); err != nil {
		t.Fatalf("Clip: %v %s", err, b)
	}
	m, err := File(t.Context(), clip)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Crop(t.Context(), clip, m)
	if err != nil || c == nil || c.X != 0 || c.W != 1 || c.Y != 0.2167 || c.H != 0.5667 {
		t.Fatalf("Crop: %+v %v", c, err)
	}
}
