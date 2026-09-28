// Package probe liest Container-, Codec- und Keyframe-Informationen per ffprobe.
package probe

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

type Stream struct {
	Index    int    `json:"index"`
	Type     string `json:"type"` // video, audio, subtitle
	Codec    string `json:"codec"`
	Profile  string `json:"profile,omitempty"`
	PixFmt   string `json:"pixFmt,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Channels int    `json:"channels,omitempty"`
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
	Default  bool   `json:"default,omitempty"`
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
			Default int `json:"default"`
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
