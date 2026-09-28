package transcode

import (
	"strings"
	"testing"
)

func TestSegmentsFollowKeyframes(t *testing.T) {
	var kf []float64
	for t := 0.0; t < 60; t += 2.5 { // GOP 2,5 s
		kf = append(kf, t)
	}
	segs := Segments(kf, 61)
	if segs[0].Start != 0 || segs[len(segs)-1].End != 61 {
		t.Fatalf("Anfang/Ende falsch: %+v", segs)
	}
	for i, s := range segs {
		if i > 0 && s.Start != segs[i-1].End {
			t.Fatalf("Lücke bei %d: %+v", i, segs)
		}
		onKF := false
		for _, k := range kf {
			onKF = onKF || k == s.Start
		}
		if !onKF {
			t.Fatalf("Segment %d beginnt nicht auf Keyframe: %v", i, s.Start)
		}
		if i < startCount && s.End-s.Start > 3 {
			t.Fatalf("Startsegment %d zu lang: %v", i, s.End-s.Start)
		}
	}
}

func TestSegmentsWithoutKeyframes(t *testing.T) {
	segs := Segments(nil, 20)
	if len(segs) != 4 || segs[3].End != 20 {
		t.Fatalf("%+v", segs)
	}
}

func TestPlaylist(t *testing.T) {
	p := Playlist([]Segment{{0, 2}, {2, 8.5}})
	for _, want := range []string{"#EXT-X-TARGETDURATION:7", "#EXTINF:6.500000,\n1.ts", "#EXT-X-ENDLIST"} {
		if !strings.Contains(p, want) {
			t.Fatalf("fehlt %q in\n%s", want, p)
		}
	}
}

// Regression: nach einem Sprung muss ffmpeg trotzdem alle Grenzen kennen,
// sonst wird der Rest der Datei ein einziges Riesensegment.
func TestArgsAfterSeekKeepAllBoundaries(t *testing.T) {
	job := Job{Input: "x.mkv", Segments: Segments(nil, 60), AudioIndex: 1, AudioCodec: "copy", VideoCodec: "copy"}
	a := strings.Join(args(job, 5, "/tmp/x"), " ")
	for _, want := range []string{"-ss 30.000000", "-segment_start_number 5", "-segment_times 6.000000,12.000000,"} {
		if !strings.Contains(a, want) {
			t.Fatalf("fehlt %q in %s", want, a)
		}
	}
}
