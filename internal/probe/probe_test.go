package probe

import (
	"os"
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
