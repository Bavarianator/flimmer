// Package playback entscheidet pro Datei und Gerät, wie abgespielt wird – ohne Seiteneffekte, voll testbar.
package playback

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/flimmer-media/flimmer/internal/lang"
	"github.com/flimmer-media/flimmer/internal/probe"
	"github.com/flimmer-media/flimmer/internal/subs"
)

// Profile beschreibt, was ein Gerät nachweislich abspielen kann.
type Profile struct {
	Name       string   `json:"name"`
	Containers []string `json:"containers"` // mp4, mkv, ts, webm
	Video      []string `json:"video"`      // h264, hevc, hevc10, av1, vp9
	Audio      []string `json:"audio"`      // aac, ac3, eac3, mp3, opus, flac, dts, truehd
	NativeHLS  bool     `json:"nativeHls"`
	MaxBitrate int64    `json:"maxBitrate"` // 0 = unbegrenzt
	// HDR-Formate des Bildschirms: hdr10, hlg, hdr10+, dv. nil = unbekannt (ältere Clients: kein Tone-Mapping),
	// leer = SDR (HDR-Quellen werden umgerechnet statt blass gezeigt).
	HDR []string `json:"hdr"`

	// Wunsch für diese Wiedergabe (kein Geräte-Merkmal, reist aber im selben Body mit):
	AudioTrack int    `json:"audioTrack,omitempty"` // Stream-Index der gewählten Tonspur, 0 = automatisch
	AudioLang  string `json:"audioLang,omitempty"`  // bevorzugte Sprache (z. B. pro Serie gemerkt)
	// Sprachkette des Benutzers („de“, „ger“, „deu“ – egal, wird normalisiert), z. B. ["de", "en"].
	AudioLangs   []string `json:"audioLangs,omitempty"`
	SubtitleMode string   `json:"subtitleMode,omitempty"` // "" automatisch, "always", "off" (siehe subs.Mode)
	Night        bool     `json:"night,omitempty"`        // Nachtmodus: Dynamik komprimieren, erzwingt Ton-Transcode
}

// AudioTrack beschreibt eine wählbare Tonspur.
type AudioTrack struct {
	Index    int    `json:"index"`
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
	Codec    string `json:"codec"`
	Channels int    `json:"channels,omitempty"`
	Default  bool   `json:"default,omitempty"`
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
	Forced   bool   `json:"forced,omitempty"`
	SDH      bool   `json:"sdh,omitempty"`
}

type Plan struct {
	Method     Method       `json:"method"`
	Light      Light        `json:"light"`
	Reasons    []string     `json:"reasons,omitempty"`
	AudioIndex int          `json:"audioIndex"`
	AudioCodec string       `json:"audioCodec"` // Ziel-Codec, "copy" wenn unverändert
	VideoCodec string       `json:"videoCodec"` // "copy" oder "h264[-720|-1080][-sdr]" (siehe transcode.ParseVideo)
	Subtitles  []Subtitle   `json:"subtitles"`
	Audio      []AudioTrack `json:"audio"`
	// SubtitleIndex ist die vorgeschlagene Untertitelspur (Stream-Index) oder -1: volle Untertitel nur,
	// wenn der Benutzer die Tonsprache nicht versteht, sonst höchstens Forced (siehe subs.Choose).
	SubtitleIndex int      `json:"subtitleIndex"`
	Notes         []string `json:"notes,omitempty"` // Hinweise ohne Einfluss auf die Methode (z. B. WLAN bei 4K-Remux)
}

// Better meldet, ob a für den Zuschauer besser ist als b: erst die Ampel, dann weniger Umwandlung.
func Better(a, b Plan) bool {
	light := map[Light]int{Green: 2, Yellow: 1, Red: 0}
	method := map[Method]int{DirectPlay: 3, DirectStream: 2, TranscodeAudio: 1, Transcode: 0}
	if light[a.Light] != light[b.Light] {
		return light[a.Light] > light[b.Light]
	}
	return method[a.Method] > method[b.Method]
}

// hdrCheck prüft HDR/Dolby Vision gegen den Bildschirm. toneMap: Das Bild muss nach SDR umgerechnet werden,
// sonst erscheint es blass (HDR10/HLG auf SDR) oder grün-lila (DV Profil 5 ohne DV).
func hdrCheck(v *probe.Stream, p Profile) (toneMap bool, reason string) {
	if v == nil || v.HDR == "" || p.HDR == nil {
		return false, ""
	}
	has := func(f string) bool { return slices.Contains(p.HDR, f) }
	format := v.HDR
	if format == "dv" {
		switch {
		case has("dv") && v.DVProfile != 7:
			return false, ""
		case v.DVCompat == 2: // SDR-Basisschicht
			return false, ""
		case v.DVCompat == 1 || v.DVCompat == 6 || v.DVProfile == 7: // 8.1, 7: HDR10-Basis
			format, reason = "hdr10", "Dolby Vision Profil "+itoa(v.DVProfile)+" läuft als HDR10-Basisschicht"
		case v.DVCompat == 4: // 8.4
			format, reason = "hlg", "Dolby Vision läuft als HLG-Basisschicht"
		default: // Profil 5: keine kompatible Basisschicht
			return true, "Dolby Vision Profil 5 braucht ein Dolby-Vision-Gerät – wird nach SDR umgerechnet, Farben können abweichen"
		}
	}
	if has(format) || format == "hdr10+" && has("hdr10") {
		return false, reason
	}
	return true, "Gerät zeigt kein " + strings.ToUpper(format) + " – wird nach SDR umgerechnet (Tone-Mapping)"
}

func itoa(n int) string { return strconv.Itoa(n) }

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
	plan := Plan{AudioIndex: -1, AudioCodec: "copy", VideoCodec: "copy", SubtitleIndex: -1}

	v := m.First("video")
	codecOK := v != nil && slices.Contains(p.Video, VideoKey(v))
	if v != nil && !codecOK {
		plan.Reasons = append(plan.Reasons, "Video-Codec "+VideoKey(v)+" wird vom Gerät nicht unterstützt")
	}
	toneMap, hdrReason := hdrCheck(v, p)
	if hdrReason != "" {
		plan.Reasons = append(plan.Reasons, hdrReason)
	}
	videoOK := codecOK && !toneMap

	a := chooseAudio(m, p)
	night := p.Night && a != nil
	// Eine andere als die Standard-Tonspur kann der native Player nicht zuverlässig wählen → Remux mit genau dieser Spur.
	otherTrack := a != nil && a != defaultAudio(m)
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
	case videoOK && audioOK && containerOK && bitrateOK && !otherTrack && !night:
		plan.Method, plan.Light = DirectPlay, Green
	case videoOK && bitrateOK && audioOK && !night && (a == nil || slices.Contains(tsAudio, a.Codec)):
		plan.Method, plan.Light = DirectStream, Green
	case videoOK && bitrateOK:
		plan.Method, plan.Light = TranscodeAudio, Yellow
		plan.AudioCodec = targetAudio(a, p)
	default:
		plan.Method, plan.Light = Transcode, Red
		// Jede umgewandelte HDR-Quelle wird tone-gemappt: H.264 8 bit trägt kein HDR.
		sdr := v != nil && v.HDR != ""
		need := requiredSpeed(v)
		if sdr {
			need *= toneMapCost
		}
		height := 0
		if v != nil && v.Height > 1080 {
			height = 1080 // 4K wird nie in 4K neu kodiert
		}
		switch {
		case speed >= need:
			plan.Light = Yellow
		case v != nil && v.Height > 720 && speed >= need*reduce720:
			plan.Light, height = Yellow, 720
			plan.Reasons = append(plan.Reasons, "wird in 720p umgewandelt, damit nichts ruckelt")
		default:
			if v != nil && v.Height > 720 {
				height = 720 // bestmöglicher Versuch
			}
			msg := "Server ist für Echtzeit-Transcoding zu langsam"
			if sdr || v != nil && v.Height > 1080 {
				msg += " – für 4K/HDR die Hintergrund-Optimierung nutzen"
			}
			plan.Reasons = append(plan.Reasons, msg)
		}
		plan.VideoCodec = "h264"
		if height > 0 {
			plan.VideoCodec += "-" + itoa(height)
		}
		if sdr {
			plan.VideoCodec += "-sdr"
		}
		if a != nil && (night || !(audioOK && slices.Contains(tsAudio, a.Codec))) {
			plan.AudioCodec = targetAudio(a, p)
		}
	}
	if night && plan.AudioCodec != "copy" {
		plan.AudioCodec += "-night" // transcode.ParseAudio; eigener Cache-Key, eigene Segmente
		plan.Reasons = append(plan.Reasons, "Nachtmodus: Ton wird leiser/lauter ausgeglichen")
	}

	if (plan.Method == DirectPlay || plan.Method == DirectStream) && m.Bitrate > wifiLimit {
		plan.Notes = append(plan.Notes, fmt.Sprintf("Sehr hohe Bitrate (%d Mbit/s): über WLAN kann es stocken – besser per Kabel oder mit Bitrate-Grenze", m.Bitrate/1_000_000))
	}

	for _, s := range m.All("subtitle") {
		sub := Subtitle{Index: s.Index, Language: s.Language, Title: s.Title, Forced: s.Forced, SDH: s.HearingImpaired}
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
	var tracks []subs.Track
	for _, s := range plan.Subtitles {
		tracks = append(tracks, subs.Track{Language: s.Language, Forced: s.Forced, SDH: s.SDH})
	}
	if a != nil {
		if i := subs.Choose(tracks, a.Language, prefs(p), subs.Mode(p.SubtitleMode)); i >= 0 {
			plan.SubtitleIndex = plan.Subtitles[i].Index
		}
	}
	for _, st := range m.All("audio") {
		plan.Audio = append(plan.Audio, AudioTrack{Index: st.Index, Language: st.Language, Title: st.Title,
			Codec: st.Codec, Channels: st.Channels, Default: st.Default})
	}
	return plan
}

// requiredSpeed: 1,5× Reserve bei 1080p, größere Quellen kosten beim Dekodieren proportional mehr.
// reduce720: 720p-Encoding kostet grob die Hälfte von 1080p (Dekodieren der Quelle bleibt gleich teuer).
const reduce720 = 0.5

// toneMapCost: zscale+tonemap in Software kostet grob so viel wie das Encoding selbst.
// ponytail: Schätzwert; mit tonemap_vaapi/opencl deutlich weniger – messen, wenn HW-Tone-Mapping kommt.
const toneMapCost = 2.0

// wifiLimit: darüber reicht WLAN oft nicht mehr zuverlässig (4K-Remux mit 80–120 Mbit/s).
const wifiLimit = 80_000_000

func requiredSpeed(v *probe.Stream) float64 {
	f := 1.5
	if v != nil && v.Width*v.Height > 1920*1080 {
		f *= float64(v.Width*v.Height) / (1920 * 1080)
	}
	return f
}

// prefs ist die Sprachkette: die pro Serie gemerkte Sprache vor der allgemeinen des Benutzers.
func prefs(p Profile) []string {
	if p.AudioLang == "" {
		return p.AudioLangs
	}
	return append([]string{p.AudioLang}, p.AudioLangs...)
}

// chooseAudio: ausdrücklich gewählte Spur, sonst die erste Sprache der Kette, die es gibt (Sprachkennungen
// normalisiert: „ger“ = „deu“ = „de“), bei gleicher Sprache bevorzugt die Standardspur; sonst die Standardspur.
func chooseAudio(m *probe.Media, p Profile) *probe.Stream {
	var all []*probe.Stream
	var langs []string
	for i := range m.Streams {
		s := &m.Streams[i]
		if s.Type != "audio" {
			continue
		}
		if p.AudioTrack > 0 && s.Index == p.AudioTrack {
			return s
		}
		all, langs = append(all, s), append(langs, s.Language)
	}
	d := defaultAudio(m)
	i := lang.Pick(langs, prefs(p))
	if i < 0 || d != nil && lang.Normalize(d.Language) == lang.Normalize(all[i].Language) {
		return d
	}
	return all[i]
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

// targetAudio wählt, wohin umgewandelt wird: Mehrkanal bleibt Mehrkanal (EAC3), sonst AAC –
// jeweils nur, wenn das Gerät es kann. Kann es keins von beiden, bleibt AAC als letzter Versuch.
func targetAudio(a *probe.Stream, p Profile) string {
	eac3, aac := slices.Contains(p.Audio, "eac3"), slices.Contains(p.Audio, "aac")
	if eac3 && (a != nil && a.Channels > 2 || !aac) {
		return "eac3"
	}
	return "aac"
}
