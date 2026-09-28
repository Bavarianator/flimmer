// Package subs findet externe Untertitel neben dem Video, wählt Untertitel samt Forced-Automatik und
// liefert Textuntertitel als WebVTT (ASS wird dabei auf Text vereinfacht; Stile rendert der Client per libass).
package subs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Bavarianator/flimmer/internal/lang"
)

// External ist eine Untertiteldatei neben dem Video, z. B. „Film.de.forced.srt“.
type External struct {
	Path     string `json:"-"`
	Format   string `json:"format"`             // srt, ass, vtt, sup
	Language string `json:"language,omitempty"` // 639-2/B, leer = unbekannt
	Forced   bool   `json:"forced,omitempty"`
	SDH      bool   `json:"sdh,omitempty"` // für Hörgeschädigte
	Default  bool   `json:"default,omitempty"`
	Title    string `json:"title"` // Anzeige, z. B. „Deutsch (erzwungen)“ – leer, wenn nichts erkannt
}

var formats = map[string]string{".srt": "srt", ".ass": "ass", ".ssa": "ass", ".vtt": "vtt", ".sup": "sup"}

// Find listet externe Untertitel zu videoPath, sortiert nach Dateiname (stabil → Index taugt als ID).
func Find(videoPath string) []External {
	dir, file := filepath.Split(videoPath)
	base := strings.TrimSuffix(file, filepath.Ext(file))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var videos []string // andere Videos im Ordner: „Film 2.srt“ gehört zu „Film 2.mkv“, nicht zu „Film.mkv“
	for _, e := range entries {
		if videoExt[strings.ToLower(filepath.Ext(e.Name()))] {
			videos = append(videos, strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
		}
	}
	var out []External
	for _, e := range entries {
		name := e.Name()
		f, ok := formats[strings.ToLower(filepath.Ext(name))]
		if !ok || e.IsDir() {
			continue
		}
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		rest, ok := strings.CutPrefix(stem, base)
		if !ok || !belongs(stem, base) || slices.ContainsFunc(videos, func(v string) bool { return len(v) > len(base) && belongs(stem, v) }) {
			continue
		}
		x := External{Path: filepath.Join(dir, name), Format: f}
		for _, tok := range strings.FieldsFunc(strings.ToLower(rest), func(r rune) bool { return strings.ContainsRune(".-_ []()", r) }) {
			switch tok {
			case "forced", "foreign":
				x.Forced = true
			case "sdh", "cc", "hi":
				x.SDH = true
			case "default":
				x.Default = true
			default:
				if l := lang.Normalize(tok); l != "" && x.Language == "" {
					x.Language = l
				}
			}
		}
		x.Title = title(x)
		out = append(out, x)
	}
	return out
}

var videoExt = map[string]bool{".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true, ".ts": true,
	".m2ts": true, ".webm": true, ".wmv": true, ".mpg": true, ".mpeg": true}

// belongs: stem ist base oder base plus Trenner und Zusatz („Film.de.forced“).
func belongs(stem, base string) bool {
	rest, ok := strings.CutPrefix(stem, base)
	return ok && (rest == "" || strings.ContainsRune(".-_ ", rune(rest[0])))
}

var names = map[string]string{"ger": "Deutsch", "eng": "Englisch", "fre": "Französisch", "spa": "Spanisch", "ita": "Italienisch",
	"dut": "Niederländisch", "por": "Portugiesisch", "rus": "Russisch", "pol": "Polnisch", "tur": "Türkisch", "jpn": "Japanisch"}

func title(x External) string {
	t := names[x.Language]
	if t == "" {
		t = x.Language
	}
	var extra []string
	if x.Forced {
		extra = append(extra, "erzwungen")
	}
	if x.SDH {
		extra = append(extra, "für Hörgeschädigte")
	}
	if len(extra) > 0 {
		t = strings.TrimSpace(t + " (" + strings.Join(extra, ", ") + ")")
	}
	return t
}

// Track ist eine wählbare Untertitelspur (eingebettet oder extern) für Choose.
type Track struct {
	Language string
	Forced   bool
	SDH      bool
}

// Mode ist die Untertitel-Einstellung des Benutzers.
type Mode string

const (
	Auto   Mode = ""       // Standard: volle Untertitel nur, wenn der Benutzer die Tonsprache nicht versteht; sonst nur Forced
	Always Mode = "always" // immer volle Untertitel in der bevorzugten Sprache
	Off    Mode = "off"    // nur Forced (fremdsprachige Stellen), nie volle Untertitel
)

// Choose wählt eine Untertitelspur oder -1. audioLang ist die Sprache der laufenden Tonspur,
// prefs die Sprachkette des Benutzers (z. B. ["ger", "eng"]).
func Choose(tracks []Track, audioLang string, prefs []string, mode Mode) int {
	audio := lang.Normalize(audioLang)
	understood := audio != "" && slices.ContainsFunc(prefs, func(p string) bool { return lang.Normalize(p) == audio })
	if mode == Always || (mode == Auto && !understood && audio != "") {
		for _, sdhOK := range []bool{false, true} { // SDH nur, wenn es nichts anderes gibt
			var langs []string
			var idx []int
			for i, t := range tracks {
				if !t.Forced && (sdhOK || !t.SDH) {
					langs, idx = append(langs, t.Language), append(idx, i)
				}
			}
			if i := lang.Pick(langs, prefs); i >= 0 {
				return idx[i]
			}
		}
	}
	// Forced in der Tonsprache: übersetzt z. B. die klingonischen Stellen eines deutschen Films.
	for i, t := range tracks {
		if t.Forced && audio != "" && lang.Normalize(t.Language) == audio {
			return i
		}
	}
	return -1
}

// ErrBinary: Bild-Untertitel (PGS/.sup) werden roh ausgeliefert und vom Client gezeichnet.
var ErrBinary = errors.New("Bild-Untertitel lassen sich nicht in WebVTT umwandeln")

// maxSize begrenzt, was eingelesen wird; echte Untertitel sind selten über 1 MB.
const maxSize = 16 << 20

// WriteVTT schreibt einen externen Textuntertitel als WebVTT nach w. Nicht-UTF-8-Dateien (in Deutschland
// oft Windows-1252) werden umkodiert, sonst stünden „Ã¤“ statt „ä“ im Bild.
func WriteVTT(ctx context.Context, ffmpeg string, x External, w io.Writer) error {
	if x.Format == "sup" {
		return ErrBinary
	}
	f, err := os.Open(x.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxSize+1))
	if err != nil {
		return err
	}
	if len(data) > maxSize {
		return fmt.Errorf("%s: Untertitel zu groß", filepath.Base(x.Path))
	}
	enc := charset(data)
	if x.Format == "vtt" && enc == "" {
		_, err := w.Write(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")))
		return err
	}
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	if enc != "" {
		args = append(args, "-sub_charenc", enc)
	}
	demux := map[string]string{"srt": "srt", "ass": "ass", "vtt": "webvtt"}[x.Format]
	args = append(args, "-f", demux, "-i", "pipe:0", "-map", "0:s:0", "-f", "webvtt", "pipe:1")
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	cmd.Stdin = bytes.NewReader(data)
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = w, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Untertitel umwandeln: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// WriteASS schreibt eine externe .ass/.ssa-Datei roh (mit Stilen) nach w, für Clients mit eigenem SSA-Renderer
// (Media3, libass). Ist sie nicht UTF-8, rekodiert ffmpeg sie als ASS nach UTF-8 – Stile und Positionen bleiben.
func WriteASS(ctx context.Context, ffmpeg string, x External, w io.Writer) error {
	if x.Format != "ass" {
		return errors.New("keine ASS-Datei")
	}
	data, err := os.ReadFile(x.Path)
	if err != nil {
		return err
	}
	enc := charset(data)
	if enc == "" {
		_, err := w.Write(data)
		return err
	}
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-nostdin", "-sub_charenc", enc,
		"-f", "ass", "-i", "pipe:0", "-map", "0:s:0", "-c:s", "ass", "-f", "ass", "pipe:1")
	cmd.Stdin = bytes.NewReader(data)
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = w, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ASS umkodieren: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// charset liefert "" für UTF-8 (auch mit BOM) und UTF-16 mit BOM (erkennt ffmpeg selbst), sonst CP1252.
// ponytail: alles, was kein UTF-8 ist, gilt als Westeuropäisch; osteuropäische/kyrillische Altdateien bräuchten Erkennung
func charset(b []byte) string {
	if bytes.HasPrefix(b, []byte{0xff, 0xfe}) || bytes.HasPrefix(b, []byte{0xfe, 0xff}) || utf8.Valid(b) {
		return ""
	}
	return "CP1252"
}
