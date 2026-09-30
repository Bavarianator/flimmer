// Package probe liest Container-, Codec- und Keyframe-Informationen per ffprobe.
package probe

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/Bavarianator/flimmer/internal/ffmpeg"
)

type Stream struct {
	Index           int    `json:"index"`
	Type            string `json:"type"` // video, audio, subtitle
	Codec           string `json:"codec"`
	Profile         string `json:"profile,omitempty"`
	PixFmt          string `json:"pixFmt,omitempty"`
	Width           int    `json:"width,omitempty"`
	Height          int    `json:"height,omitempty"`
	Channels        int    `json:"channels,omitempty"`
	Language        string `json:"language,omitempty"`
	Title           string `json:"title,omitempty"`
	Default         bool   `json:"default,omitempty"`
	Forced          bool   `json:"forced,omitempty"`          // Untertitel nur für fremdsprachige Stellen
	HearingImpaired bool   `json:"hearingImpaired,omitempty"` // SDH: mit Geräuschbeschreibungen
	// HDR: "", "hdr10", "hdr10+", "hlg" oder "dv" (Dolby Vision; die Basisschicht steht in DVCompat).
	HDR       string `json:"hdr,omitempty"`
	DVProfile int    `json:"dvProfile,omitempty"` // 5, 7, 8 …
	DVCompat  int    `json:"dvCompat,omitempty"`  // Basisschicht laut bl_signal_compatibility_id: 1 HDR10, 2 SDR, 4 HLG, 0 keine (Profil 5)
}

type Media struct {
	Container string   `json:"container"` // ffprobe format_name, z. B. "matroska,webm" oder "mov,mp4,m4a,3gp,3g2,mj2"
	Duration  float64  `json:"duration"`
	Bitrate   int64    `json:"bitrate"`
	Streams   []Stream `json:"streams"`
}

func (m *Media) First(typ string) *Stream {
	for i := range m.Streams {
		if m.Streams[i].Type == typ {
			return &m.Streams[i]
		}
	}
	return nil
}

func (m *Media) All(typ string) []Stream {
	var out []Stream
	for _, s := range m.Streams {
		if s.Type == typ {
			out = append(out, s)
		}
	}
	return out
}

type ffprobeOut struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
	Streams []struct {
		Index         int    `json:"index"`
		CodecType     string `json:"codec_type"`
		CodecName     string `json:"codec_name"`
		Profile       string `json:"profile"`
		PixFmt        string `json:"pix_fmt"`
		Width         int    `json:"width"`
		Height        int    `json:"height"`
		Channels      int    `json:"channels"`
		ColorTransfer string `json:"color_transfer"`
		SideData      []struct {
			Type      string `json:"side_data_type"`
			DVProfile int    `json:"dv_profile"`
			DVCompat  int    `json:"dv_bl_signal_compatibility_id"`
		} `json:"side_data_list"`
		Disposition struct {
			Default         int `json:"default"`
			Forced          int `json:"forced"`
			HearingImpaired int `json:"hearing_impaired"`
		} `json:"disposition"`
		Tags map[string]string `json:"tags"`
	} `json:"streams"`
}

// File probt Container und Streams. Schnell, weil nur der Header gelesen wird; den Keyframe-Index liefert Keyframes.
func File(ctx context.Context, path string) (*Media, error) {
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", path).Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe %s: %w", path, err)
	}
	m, err := parse(out)
	if err != nil {
		return nil, err
	}
	if v := m.First("video"); v != nil && v.HDR == "hdr10" && hdr10Plus(ctx, path, v.Index) {
		v.HDR = "hdr10+"
	}
	return m, nil
}

func parse(out []byte) (*Media, error) {
	var p ffprobeOut
	if err := json.Unmarshal(out, &p); err != nil {
		return nil, err
	}
	m := &Media{Container: p.Format.FormatName}
	m.Duration, _ = strconv.ParseFloat(p.Format.Duration, 64)
	m.Bitrate, _ = strconv.ParseInt(p.Format.BitRate, 10, 64)
	for _, s := range p.Streams {
		if s.CodecType != "video" && s.CodecType != "audio" && s.CodecType != "subtitle" {
			continue
		}
		if s.CodecType == "video" && s.CodecName == "mjpeg" { // eingebettete Cover
			continue
		}
		st := Stream{
			Index: s.Index, Type: s.CodecType, Codec: s.CodecName, Profile: s.Profile, PixFmt: s.PixFmt,
			Width: s.Width, Height: s.Height, Channels: s.Channels,
			Language: s.Tags["language"], Title: s.Tags["title"], Default: s.Disposition.Default == 1,
			Forced: s.Disposition.Forced == 1, HearingImpaired: s.Disposition.HearingImpaired == 1,
		}
		switch s.ColorTransfer {
		case "smpte2084":
			st.HDR = "hdr10"
		case "arib-std-b67":
			st.HDR = "hlg"
		}
		for _, sd := range s.SideData {
			if sd.Type == "DOVI configuration record" {
				st.HDR, st.DVProfile, st.DVCompat = "dv", sd.DVProfile, sd.DVCompat
			}
		}
		m.Streams = append(m.Streams, st)
	}
	return m, nil
}

// hdr10Plus: HDR10+ steht nur als dynamische Metadaten in den Frames; der erste Frame reicht als Nachweis.
func hdr10Plus(ctx context.Context, path string, stream int) bool {
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", strconv.Itoa(stream),
		"-read_intervals", "%+#1", "-show_entries", "frame=side_data_list", "-of", "json", path).Output()
	return err == nil && bytes.Contains(out, []byte("SMPTE2094-40"))
}

// Keyframes liest nur die Pakete (kein Decoding) und sammelt die Zeitstempel der Keyframes (Sekunden),
// die Grundlage für HLS-Segmentgrenzen. Liest die ganze Datei, deshalb getrennt vom schnellen File.
func Keyframes(ctx context.Context, path string, stream int) ([]float64, error) {
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", strconv.Itoa(stream),
		"-show_entries", "packet=pts_time,flags", "-of", "csv=p=0", path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("keyframes %s: %w", path, err)
	}
	return parseKeyframes(out), nil
}

func parseKeyframes(out []byte) []float64 {
	var kf []float64
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		ts, flags, ok := strings.Cut(sc.Text(), ",")
		if !ok || !strings.HasPrefix(flags, "K") {
			continue
		}
		if t, err := strconv.ParseFloat(ts, 64); err == nil {
			kf = append(kf, t)
		}
	}
	sort.Float64s(kf) // Pakete kommen in Decode-Reihenfolge
	return kf
}

// Extra sind Angaben, die nur die Detailseite braucht; deshalb nicht im Scan, sondern auf Abruf.
type Extra struct {
	FPS      float64   `json:"fps,omitempty"`
	Chapters []Chapter `json:"chapters,omitempty"`
}

type Chapter struct {
	Start float64 `json:"start"` // Sekunden
	Name  string  `json:"name"`
}

// Extras liest Bildrate und Kapitel (nur der Kopf der Datei).
func Extras(ctx context.Context, path string) (Extra, error) {
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-print_format", "json", "-show_chapters",
		"-select_streams", "v:0", "-show_entries", "stream=avg_frame_rate", path).Output()
	if err != nil {
		return Extra{}, fmt.Errorf("ffprobe %s: %w", path, err)
	}
	return parseExtras(out)
}

func parseExtras(out []byte) (Extra, error) {
	var p struct {
		Streams []struct {
			Rate string `json:"avg_frame_rate"`
		} `json:"streams"`
		Chapters []struct {
			Start string            `json:"start_time"`
			Tags  map[string]string `json:"tags"`
		} `json:"chapters"`
	}
	var e Extra
	if err := json.Unmarshal(out, &p); err != nil {
		return e, err
	}
	if len(p.Streams) > 0 {
		n, d, _ := strings.Cut(p.Streams[0].Rate, "/")
		num, _ := strconv.ParseFloat(n, 64)
		den, _ := strconv.ParseFloat(d, 64)
		if den > 0 {
			e.FPS = float64(int(num/den*1000+0.5)) / 1000 // 23.976
		}
	}
	for _, c := range p.Chapters {
		start, _ := strconv.ParseFloat(c.Start, 64)
		e.Chapters = append(e.Chapters, Chapter{Start: start, Name: c.Tags["title"]})
	}
	return e, nil
}

// Rect ist ein Bildausschnitt in Anteilen des ganzen Bildes (0..1), damit er auch für anamorphe Videos und
// umgewandelte Streams mit anderer Auflösung stimmt.
type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Crop sucht eingebrannte schwarze Balken per cropdetect an drei Stellen (10, 50 und 80 % der Laufzeit, je 2 s)
// und nimmt die Vereinigung der Rechtecke. nil heißt: keine Balken (≥ 98 % in beiden Richtungen), eine dunkle
// Szene (< 50 % Fläche) oder kein Video. Läuft mit niedriger Priorität.
func Crop(ctx context.Context, path string, m *Media) (*Rect, error) {
	v := m.First("video")
	if v == nil || v.Width == 0 || v.Height == 0 || m.Duration <= 0 {
		return nil, nil
	}
	var all []pixRect
	for _, at := range []float64{0.1, 0.5, 0.8} {
		name, args := ffmpeg.Nice("ffmpeg", []string{"-hide_banner", "-nostdin", "-ss", strconv.FormatFloat(m.Duration*at, 'f', 2, 64),
			"-i", path, "-t", "2", "-map", "0:v:0", "-vf", "cropdetect=limit=0.094:round=2:reset=0", "-an", "-sn", "-f", "null", "-"})
		out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("cropdetect %s: %w", path, err)
		}
		if r, ok := parseCrop(out); ok {
			all = append(all, r)
		}
	}
	return cropOf(all, v.Width, v.Height), nil
}

type pixRect struct{ x, y, w, h int }

// parseCrop liest das letzte „crop=w:h:x:y“; mit reset=0 ist das schon die Vereinigung über alle Bilder der Stelle.
func parseCrop(out []byte) (pixRect, bool) {
	i := bytes.LastIndex(out, []byte("crop="))
	if i < 0 {
		return pixRect{}, false
	}
	line, _, _ := bytes.Cut(out[i+5:], []byte("\n"))
	var r pixRect
	if n, _ := fmt.Sscanf(strings.TrimSpace(string(line)), "%d:%d:%d:%d", &r.w, &r.h, &r.x, &r.y); n != 4 || r.w <= 0 || r.h <= 0 {
		return pixRect{}, false
	}
	return r, true
}

// cropOf vereinigt die Rechtecke und wendet die Schwellen an (siehe Crop).
func cropOf(all []pixRect, width, height int) *Rect {
	if len(all) == 0 {
		return nil
	}
	x1, y1, x2, y2 := width, height, 0, 0
	for _, r := range all {
		x1, y1 = min(x1, r.x), min(y1, r.y)
		x2, y2 = max(x2, r.x+r.w), max(y2, r.y+r.h)
	}
	x1, y1, x2, y2 = max(x1, 0), max(y1, 0), min(x2, width), min(y2, height)
	fw, fh := float64(x2-x1)/float64(width), float64(y2-y1)/float64(height)
	if fw >= 0.98 && fh >= 0.98 || fw*fh < 0.5 {
		return nil
	}
	round := func(f float64) float64 { return math.Round(f*10000) / 10000 }
	return &Rect{X: round(float64(x1) / float64(width)), Y: round(float64(y1) / float64(height)), W: round(fw), H: round(fh)}
}
