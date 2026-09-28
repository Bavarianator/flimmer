package playback

import (
	"testing"

	"github.com/flimmer-media/flimmer/internal/probe"
)

var lgTV = Profile{
	Name:       "LG webOS",
	Containers: []string{"mp4", "mkv", "ts"},
	Video:      []string{"h264", "hevc", "hevc10"},
	Audio:      []string{"aac", "ac3", "eac3", "mp3"},
	NativeHLS:  true,
}

var oldBrowser = Profile{
	Name:       "Browser",
	Containers: []string{"mp4"},
	Video:      []string{"h264"},
	Audio:      []string{"aac", "mp3", "opus", "flac"},
}

func media(container, video, pix, audio string, ch int, subs ...string) *probe.Media {
	m := &probe.Media{Container: container, Bitrate: 8_000_000}
	idx := 0
	if video != "" {
		m.Streams = append(m.Streams, probe.Stream{Index: idx, Type: "video", Codec: video, PixFmt: pix})
		idx++
	}
	if audio != "" {
		m.Streams = append(m.Streams, probe.Stream{Index: idx, Type: "audio", Codec: audio, Channels: ch})
		idx++
	}
	for _, s := range subs {
		m.Streams = append(m.Streams, probe.Stream{Index: idx, Type: "subtitle", Codec: s})
		idx++
	}
	return m
}

func TestDecide(t *testing.T) {
	const mp4, mkv = "mov,mp4,m4a,3gp,3g2,mj2", "matroska,webm"
	tests := []struct {
		name   string
		m      *probe.Media
		p      Profile
		method Method
		light  Light
		audio  string
		video  string
	}{
		{"mp4 h264 aac auf TV", media(mp4, "h264", "yuv420p", "aac", 2), lgTV, DirectPlay, Green, "copy", "copy"},
		{"mkv hevc10 eac3 auf TV", media(mkv, "hevc", "yuv420p10le", "eac3", 6), lgTV, DirectPlay, Green, "copy", "copy"},
		{"mkv hevc dts 5.1 auf TV -> nur Ton, EAC3", media(mkv, "hevc", "yuv420p", "dts", 6), lgTV, TranscodeAudio, Yellow, "eac3", "copy"},
		{"mkv h264 truehd stereo-fähiger Browser", media(mkv, "h264", "yuv420p", "truehd", 8), oldBrowser, TranscodeAudio, Yellow, "aac", "copy"},
		{"mkv h264 aac im Browser -> Remux", media(mkv, "h264", "yuv420p", "aac", 2), oldBrowser, DirectStream, Green, "copy", "copy"},
		{"mkv h264 flac im Browser -> Ton neu (flac nicht in TS)", media(mkv, "h264", "yuv420p", "flac", 2), oldBrowser, TranscodeAudio, Yellow, "aac", "copy"},
		{"hevc im Browser -> voll", media(mkv, "hevc", "yuv420p", "aac", 2), oldBrowser, Transcode, Red, "copy", "h264"},
		{"hevc + dts im Browser -> voll inkl. Ton", media(mkv, "hevc", "yuv420p", "dts", 6), oldBrowser, Transcode, Red, "aac", "h264"},
		{"av1 10bit bleibt av1", media(mkv, "av1", "yuv420p10le", "opus", 2), Profile{Containers: []string{"mkv"}, Video: []string{"av1"}, Audio: []string{"opus"}}, DirectPlay, Green, "copy", "copy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(tt.m, tt.p)
			if got.Method != tt.method || got.Light != tt.light || got.AudioCodec != tt.audio || got.VideoCodec != tt.video {
				t.Fatalf("got %s/%s audio=%s video=%s (%v), want %s/%s audio=%s video=%s",
					got.Method, got.Light, got.AudioCodec, got.VideoCodec, got.Reasons, tt.method, tt.light, tt.audio, tt.video)
			}
		})
	}
}

func TestBitrateLimitForcesTranscode(t *testing.T) {
	p := lgTV
	p.MaxBitrate = 4_000_000
	got := Decide(media("mov,mp4", "h264", "yuv420p", "aac", 2), p)
	if got.Method != Transcode {
		t.Fatalf("got %s", got.Method)
	}
}

func TestSubtitlesNeverBurnedIn(t *testing.T) {
	got := Decide(media("matroska", "h264", "yuv420p", "aac", 2, "subrip", "ass", "hdmv_pgs_subtitle", "dvd_subtitle"), lgTV)
	if got.Method != DirectPlay {
		t.Fatalf("Untertitel dürfen die Methode nicht verschlechtern, got %s", got.Method)
	}
	want := []string{"vtt", "vtt", "pgs"}
	if len(got.Subtitles) != len(want) {
		t.Fatalf("got %+v", got.Subtitles)
	}
	for i, f := range want {
		if got.Subtitles[i].Format != f {
			t.Fatalf("sub %d: got %s want %s", i, got.Subtitles[i].Format, f)
		}
	}
}
