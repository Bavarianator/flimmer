package transcode

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Bavarianator/flimmer/internal/probe"
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
	if a := strings.Join(args(job, 0, "/tmp/x"), " "); !strings.Contains(a, `-vf scale=-2:min(720\,ih)`) || !strings.Contains(a, "-preset ultrafast") || !strings.Contains(a, "-progress pipe:1") {
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

func TestParseVideo(t *testing.T) {
	for _, tt := range []struct {
		in    string
		codec string
		h     int
		ok    bool
	}{
		{"copy", "copy", 0, true},
		{"h264", "h264", 0, true},
		{"h264-720", "h264", 720, true},
		{"h264-1080", "h264", 1080, true},
		{"h264-sdr", "h264-sdr", 0, true},
		{"h264-1080-sdr", "h264-sdr", 1080, true},
		{"h264-480", "", 0, false},
		{"hevc", "", 0, false},
		{"h264-sdr-720", "", 0, false},
	} {
		c, h, ok := ParseVideo(tt.in)
		if c != tt.codec || h != tt.h || ok != tt.ok {
			t.Errorf("%s: %s %d %v", tt.in, c, h, ok)
		}
	}
}

func TestVideoArgs(t *testing.T) {
	vaapi := []string{"-vf", "scale_vaapi=format=nv12", "-c:v", "h264_vaapi", "-qp", "23"}
	for _, tt := range []struct {
		name string
		got  []string
		want string
	}{
		{"Software ohne Änderung", videoArgs(false, 0, false, nil), "-c:v libx264"},
		{"VAAPI skaliert auf der GPU", videoArgs(false, 720, true, vaapi), `-vf scale_vaapi=w=-2:h=min(720\,ih):format=nv12 -c:v h264_vaapi`},
		{"VAAPI ohne Höhe unverändert", videoArgs(false, 0, true, vaapi), "-vf scale_vaapi=format=nv12 -c:v h264_vaapi"},
		{"HDR → SDR in Software, auch wenn VAAPI angeboten", VideoArgs(&probe.Stream{HDR: "hdr10"}, 1080, vaapi), `-vf scale=-2:min(1080\,ih),zscale=t=linear`},
		{"SDR-Quelle: kein Tone-Mapping", VideoArgs(&probe.Stream{}, 1080, nil), `-vf scale=-2:min(1080\,ih) -c:v libx264`},
	} {
		if g := strings.Join(tt.got, " "); !strings.HasPrefix(g, tt.want) {
			t.Errorf("%s:\n got %s\nwant %s…", tt.name, g, tt.want)
		}
	}
	if g := strings.Join(VideoArgs(&probe.Stream{HDR: "hlg"}, 0, nil), " "); !strings.Contains(g, "tonemap=hable") || strings.Contains(g, "scale=-2") {
		t.Errorf("HLG ohne Höhe: %s", g)
	}
}

func TestParseAudio(t *testing.T) {
	for in, want := range map[string]string{"copy": "copy/false", "aac": "aac/false", "eac3-night": "eac3/true", "aac-night": "aac/true", "copy-night": "", "dts": ""} {
		c, n, ok := ParseAudio(in)
		if got := fmt.Sprintf("%s/%v", c, n); ok != (want != "") || ok && got != want {
			t.Errorf("%s: %s %v", in, got, ok)
		}
	}
}

func TestAudioFilter(t *testing.T) {
	job := Job{Input: "x.mkv", Segments: Segments(nil, 30), AudioIndex: 1, AudioCodec: "aac", VideoCodec: "copy", AudioChannels: 6}
	a := strings.Join(args(job, 0, "/tmp/x"), " ")
	if !strings.Contains(a, "-af aformat=channel_layouts=5.1,pan=stereo") || !strings.Contains(a, "-ac 2") {
		t.Errorf("5.1 → Stereo ohne Center-Downmix: %s", a)
	}
	job.AudioCodec, job.Night = "eac3", true
	a = strings.Join(args(job, 0, "/tmp/x"), " ")
	if !strings.Contains(a, "-af acompressor") || strings.Contains(a, "pan=stereo") {
		t.Errorf("EAC3 5.1 Nachtmodus: %s", a)
	}
	if k1, k2 := job.Key(), (Job{Input: job.Input, AudioIndex: 1, AudioCodec: "eac3", VideoCodec: "copy"}).Key(); k1 == k2 {
		t.Error("Nachtmodus braucht einen eigenen Cache-Key")
	}
}
