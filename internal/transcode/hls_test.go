package transcode

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
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

// Regression: nach einem Sprung sind segment_times relativ zum Startsegment,
// sonst entstehen falsche Grenzen (Riesensegmente bzw. Segmente ohne Ton).
func TestArgsAfterSeek(t *testing.T) {
	job := Job{Input: "x.mkv", Segments: Segments(nil, 60), AudioIndex: 1, AudioCodec: "copy", VideoCodec: "h264"}
	a := strings.Join(args(job, 5, "/tmp/x"), " ")
	for _, want := range []string{"-ss 30.200000 -noaccurate_seek", "-copyts -i x.mkv", "-segment_start_number 5",
		"-segment_times 6.000000,12.000000,18.000000,24.000000", "-force_key_frames 36.000000,42.000000"} {
		if !strings.Contains(a, want) {
			t.Fatalf("fehlt %q in %s", want, a)
		}
	}
}

func TestWatchDowngradesSlowTranscode(t *testing.T) {
	watchWindow = 50 * time.Millisecond
	defer func() { watchWindow = 6 * time.Second }()
	for _, tc := range []struct {
		name   string
		perTic float64 // Sekunden Video pro 20 ms Wanduhr
		paused bool
		slow   bool
	}{
		{"schnell", 0.1, false, false}, // 5× Echtzeit
		{"langsam", 0.01, false, true}, // 0,5× Echtzeit
		{"langsam aber gedrosselt", 0.01, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &Manager{lowRes: map[string]bool{}}
			s := &session{job: Job{Input: "film.mkv"}, paused: tc.paused}
			r, w := io.Pipe()
			go func() {
				out := 0.0
				for i := 0; i < 15; i++ {
					fmt.Fprintf(w, "out_time_us=%d\nprogress=continue\n", int(out*1e6))
					out += tc.perTic
					time.Sleep(20 * time.Millisecond)
				}
				w.Close()
			}()
			m.watch(s, r)
			if s.slow != tc.slow || m.lowRes["film.mkv"] != tc.slow {
				t.Fatalf("slow=%v lowRes=%v, want %v", s.slow, m.lowRes["film.mkv"], tc.slow)
			}
		})
	}
}

func TestArgsDownscaleOnlyInSoftware(t *testing.T) {
	job := Job{Input: "x.mkv", Segments: Segments(nil, 30), AudioIndex: -1, AudioCodec: "aac", VideoCodec: "h264", Height: 720}
	if a := strings.Join(args(job, 0, "/tmp/x"), " "); !strings.Contains(a, "-vf scale=-2:720") || !strings.Contains(a, "-progress pipe:1") {
		t.Fatalf("Software: %s", a)
	}
	job.InputArgs = []string{"-hwaccel", "vaapi"}
	if a := strings.Join(args(job, 0, "/tmp/x"), " "); strings.Contains(a, "scale=") {
		t.Fatalf("HW-Pfad darf keinen Software-Scaler bekommen: %s", a)
	}
	if v, h, ok := ParseVideo("h264-720"); !ok || v != "h264" || h != 720 {
		t.Fatal("ParseVideo")
	}
}
