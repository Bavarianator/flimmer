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
}

type Media struct {
	Container string    `json:"container"` // ffprobe format_name, z. B. "matroska,webm" oder "mov,mp4,m4a,3gp,3g2,mj2"
	Duration  float64   `json:"duration"`
	Bitrate   int64     `json:"bitrate"`
	Streams   []Stream  `json:"streams"`
	Keyframes []float64 `json:"keyframes,omitempty"` // Zeitstempel in Sekunden, Grundlage für HLS-Segmentgrenzen
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
		Index       int    `json:"index"`
		CodecType   string `json:"codec_type"`
		CodecName   string `json:"codec_name"`
		Profile     string `json:"profile"`
		PixFmt      string `json:"pix_fmt"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		Channels    int    `json:"channels"`
		Disposition struct {
			Default int `json:"default"`
		} `json:"disposition"`
		Tags map[string]string `json:"tags"`
	} `json:"streams"`
}

// File probt eine Datei inkl. Keyframe-Index.
func File(ctx context.Context, path string) (*Media, error) {
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", path).Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe %s: %w", path, err)
	}
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
		m.Streams = append(m.Streams, Stream{
			Index: s.Index, Type: s.CodecType, Codec: s.CodecName, Profile: s.Profile, PixFmt: s.PixFmt,
			Width: s.Width, Height: s.Height, Channels: s.Channels,
			Language: s.Tags["language"], Title: s.Tags["title"], Default: s.Disposition.Default == 1,
		})
	}
	if v := m.First("video"); v != nil {
		m.Keyframes, err = keyframes(ctx, path, v.Index)
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}

// keyframes liest nur die Pakete (kein Decoding) und sammelt die Zeitstempel der Keyframes.
// ponytail: liest die ganze Datei einmal beim Scan; bei riesigen Bibliotheken später parallel/im Hintergrund.
func keyframes(ctx context.Context, path string, stream int) ([]float64, error) {
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
