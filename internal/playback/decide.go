// Package playback entscheidet pro Datei und Gerät, wie abgespielt wird – ohne Seiteneffekte, voll testbar.
package playback

import (
	"slices"
	"strings"

	"github.com/flimmer-media/flimmer/internal/probe"
)

// Profile beschreibt, was ein Gerät nachweislich abspielen kann.
type Profile struct {
	Name       string   `json:"name"`
	Containers []string `json:"containers"` // mp4, mkv, ts, webm
	Video      []string `json:"video"`      // h264, hevc, hevc10, av1, vp9
	Audio      []string `json:"audio"`      // aac, ac3, eac3, mp3, opus, flac, dts, truehd
	NativeHLS  bool     `json:"nativeHls"`
	MaxBitrate int64    `json:"maxBitrate"` // 0 = unbegrenzt
}

type Method string

const (
	DirectPlay     Method = "direct-play"     // Originaldatei per HTTP-Range
	DirectStream   Method = "direct-stream"   // HLS, Video und Audio kopiert
	TranscodeAudio Method = "transcode-audio" // HLS, Video kopiert, nur Ton neu
	Transcode      Method = "transcode"       // HLS, alles neu
)

type Light string

const (
	Green  Light = "green"
	Yellow Light = "yellow"
	Red    Light = "red"
)

type Subtitle struct {
	Index    int    `json:"index"`
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
	Format   string `json:"format"` // vtt (Server konvertiert) oder pgs (Client-Overlay)
}

type Plan struct {
	Method     Method     `json:"method"`
	Light      Light      `json:"light"`
	Reasons    []string   `json:"reasons,omitempty"`
	AudioIndex int        `json:"audioIndex"`
	AudioCodec string     `json:"audioCodec"` // Ziel-Codec, "copy" wenn unverändert
	VideoCodec string     `json:"videoCodec"` // "copy" oder "h264"
	Subtitles  []Subtitle `json:"subtitles"`
}

// In MPEG-TS-Segmenten sauber transportierbare Audio-Codecs.
var tsAudio = []string{"aac", "ac3", "eac3", "mp3"}

var textSubs = []string{"subrip", "ass", "ssa", "mov_text", "webvtt", "text"}

func ContainerKey(formatName string) string {
	switch {
	case strings.Contains(formatName, "mp4"):
		return "mp4"
	case strings.Contains(formatName, "matroska"), strings.Contains(formatName, "webm"):
		return "mkv"
	case strings.Contains(formatName, "mpegts"):
		return "ts"
	}
	return formatName
}

// VideoKey unterscheidet 10-bit-Varianten, weil viele TVs HEVC nur in 8 bit dekodieren.
func VideoKey(s *probe.Stream) string {
	if strings.Contains(s.PixFmt, "10") && s.Codec != "av1" && s.Codec != "vp9" {
		return s.Codec + "10"
	}
	return s.Codec
}

// Decide wählt die Wiedergabeart. speed ist der gemessene Echtzeit-Faktor des Servers beim
// 1080p-H.264-Encode (hwaccel.Accel.Speed, 0 = unbekannt) – daraus folgt Gelb oder Rot beim Transcoding.
func Decide(m *probe.Media, p Profile, speed float64) Plan {
	plan := Plan{AudioIndex: -1, AudioCodec: "copy", VideoCodec: "copy"}

	v := m.First("video")
	videoOK := v != nil && slices.Contains(p.Video, VideoKey(v))
	if v != nil && !videoOK {
		plan.Reasons = append(plan.Reasons, "Video-Codec "+VideoKey(v)+" wird vom Gerät nicht unterstützt")
	}

	a := defaultAudio(m)
	audioOK := true
	if a != nil {
		plan.AudioIndex = a.Index
		audioOK = slices.Contains(p.Audio, a.Codec)
		if !audioOK {
			plan.Reasons = append(plan.Reasons, "Ton "+a.Codec+" wird vom Gerät nicht unterstützt")
		}
	}

	containerOK := slices.Contains(p.Containers, ContainerKey(m.Container))
	if !containerOK {
		plan.Reasons = append(plan.Reasons, "Container "+ContainerKey(m.Container)+" nicht direkt abspielbar")
	}

	bitrateOK := p.MaxBitrate == 0 || m.Bitrate <= p.MaxBitrate
	if !bitrateOK {
		plan.Reasons = append(plan.Reasons, "Bitrate zu hoch für die Verbindung")
	}

	switch {
	case videoOK && audioOK && containerOK && bitrateOK:
		plan.Method, plan.Light = DirectPlay, Green
	case videoOK && bitrateOK && audioOK && (a == nil || slices.Contains(tsAudio, a.Codec)):
		plan.Method, plan.Light = DirectStream, Green
	case videoOK && bitrateOK:
		plan.Method, plan.Light = TranscodeAudio, Yellow
		plan.AudioCodec = targetAudio(a, p)
	default:
		plan.Method, plan.Light = Transcode, Red
		if speed >= requiredSpeed(v) {
			plan.Light = Yellow
		} else {
			plan.Reasons = append(plan.Reasons, "Server ist für Echtzeit-Transcoding zu langsam")
		}
		plan.VideoCodec = "h264"
		if a != nil && !(audioOK && slices.Contains(tsAudio, a.Codec)) {
			plan.AudioCodec = targetAudio(a, p)
		}
	}

	for _, s := range m.All("subtitle") {
		sub := Subtitle{Index: s.Index, Language: s.Language, Title: s.Title}
		switch {
		case slices.Contains(textSubs, s.Codec):
			sub.Format = "vtt"
		case s.Codec == "hdmv_pgs_subtitle":
			sub.Format = "pgs"
		default:
			continue // z. B. dvd_subtitle – wird nie eingebrannt, lieber weglassen als ruckeln
		}
		plan.Subtitles = append(plan.Subtitles, sub)
	}
	return plan
}

// requiredSpeed: 1,5× Reserve bei 1080p, größere Quellen kosten beim Dekodieren proportional mehr.
func requiredSpeed(v *probe.Stream) float64 {
	f := 1.5
	if v != nil && v.Width*v.Height > 1920*1080 {
		f *= float64(v.Width*v.Height) / (1920 * 1080)
	}
	return f
}

func defaultAudio(m *probe.Media) *probe.Stream {
	var first *probe.Stream
	for i := range m.Streams {
		s := &m.Streams[i]
		if s.Type != "audio" {
			continue
		}
		if s.Default {
			return s
		}
		if first == nil {
			first = s
		}
	}
	return first
}

// Mehrkanal bleibt Mehrkanal, wenn das Gerät EAC3 kann; sonst Stereo-AAC (kann jedes Gerät).
func targetAudio(a *probe.Stream, p Profile) string {
	if a != nil && a.Channels > 2 && slices.Contains(p.Audio, "eac3") {
		return "eac3"
	}
	return "aac"
}
