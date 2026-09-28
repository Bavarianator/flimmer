package subs

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFind(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"Film.mkv", "Film 2.mkv", "Film 2.de.srt", "Film 2.srt", "Film.de.srt", "Film.German.forced.srt",
		"Film.en.SDH.ass", "Film.sup", "Filmabend.srt", "Film.txt", "Film.eng.default.vtt"} {
		os.WriteFile(filepath.Join(dir, n), nil, 0o644)
	}
	list := func(video string) ([]External, string) {
		got := Find(filepath.Join(dir, video))
		var lines []string
		for _, x := range got {
			lines = append(lines, filepath.Base(x.Path)+"|"+x.Format+"|"+x.Language+"|"+x.Title)
		}
		return got, strings.Join(lines, "\n")
	}
	got, s := list("Film.mkv")
	want := strings.Join([]string{
		"Film.German.forced.srt|srt|ger|Deutsch (erzwungen)",
		"Film.de.srt|srt|ger|Deutsch",
		"Film.en.SDH.ass|ass|eng|Englisch (für Hörgeschädigte)",
		"Film.eng.default.vtt|vtt|eng|Englisch",
		"Film.sup|sup||",
	}, "\n")
	if s != want {
		t.Fatalf("Find:\n%s\nerwartet:\n%s", s, want)
	}
	if !got[0].Forced || !got[2].SDH || !got[3].Default {
		t.Fatalf("Flags: %+v", got)
	}
	if _, s := list("Film 2.mkv"); s != "Film 2.de.srt|srt|ger|Deutsch\nFilm 2.srt|srt||" {
		t.Fatalf("Film 2: %s", s)
	}
}

func TestChoose(t *testing.T) {
	tracks := []Track{
		{Language: "ger", Forced: true}, // 0
		{Language: "eng"},               // 1
		{Language: "ger", SDH: true},    // 2
		{Language: "ger"},               // 3
		{Language: "eng", Forced: true}, // 4
	}
	de := []string{"ger", "eng"}
	for _, c := range []struct {
		name  string
		audio string
		prefs []string
		mode  Mode
		want  int
	}{
		{"deutscher Ton, versteht Deutsch → nur Forced", "deu", de, Auto, 0},
		{"japanischer Ton → volle deutsche, nicht SDH", "jpn", de, Auto, 3},
		{"englischer Ton, versteht Englisch → englische Forced", "eng", de, Auto, 4},
		{"immer → volle deutsche", "ger", de, Always, 3},
		{"aus, japanischer Ton → nichts", "jpn", de, Off, -1},
		{"aus, deutscher Ton → Forced bleibt", "ger", de, Off, 0},
		{"unbekannte Tonsprache → nichts", "", de, Auto, -1},
		{"nur Französisch bevorzugt, nichts da", "jpn", []string{"fre"}, Auto, -1},
	} {
		if got := Choose(tracks, c.audio, c.prefs, c.mode); got != c.want {
			t.Errorf("%s: %d, erwartet %d", c.name, got, c.want)
		}
	}
	if got := Choose([]Track{{Language: "ger", SDH: true}}, "jpn", de, Auto); got != 0 {
		t.Errorf("SDH als letzter Ausweg: %d", got)
	}
}

func TestWriteVTT(t *testing.T) {
	dir := t.TempDir()
	vtt := filepath.Join(dir, "a.vtt")
	os.WriteFile(vtt, []byte("\xef\xbb\xbfWEBVTT\n\n00:00.000 --> 00:01.000\nHallo\n"), 0o644)
	var b bytes.Buffer
	if err := WriteVTT(context.Background(), "", External{Path: vtt, Format: "vtt"}, &b); err != nil || !strings.HasPrefix(b.String(), "WEBVTT") {
		t.Fatalf("vtt: %v %q", err, b.String())
	}
	if err := WriteVTT(context.Background(), "", External{Format: "sup"}, &b); err != ErrBinary {
		t.Fatalf("sup: %v", err)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg fehlt")
	}
	srt := filepath.Join(dir, "a.srt")
	os.WriteFile(srt, []byte("1\r\n00:00:01,000 --> 00:00:02,500\r\nGr\xfc\xdfe aus M\xfcnchen\r\n"), 0o644) // Windows-1252
	b.Reset()
	if err := WriteVTT(context.Background(), "", External{Path: srt, Format: "srt"}, &b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "Grüße aus München") || !strings.Contains(b.String(), "00:01.000 --> 00:02.500") {
		t.Fatalf("srt: %q", b.String())
	}
	ass := filepath.Join(dir, "a.ass")
	os.WriteFile(ass, []byte(`[Script Info]
ScriptType: v4.00+

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,0,2,10,10,10,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,{\b1}Fett{\b0} und normal
`), 0o644)
	b.Reset()
	if err := WriteVTT(context.Background(), "", External{Path: ass, Format: "ass"}, &b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "Fett") || strings.Contains(b.String(), `{\b1}`) {
		t.Fatalf("ass: %q", b.String())
	}
}

func TestWriteASS(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg fehlt")
	}
	ass := filepath.Join(t.TempDir(), "a.ass")
	os.WriteFile(ass, []byte("[Script Info]\nScriptType: v4.00+\n\n[V4+ Styles]\n"+
		"Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n"+
		"Style: Schild,Arial,20,&H0000FFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,0,8,10,10,10,1\n\n[Events]\n"+
		"Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"+
		"Dialogue: 0,0:00:01.00,0:00:02.00,Schild,,0,0,0,,{\\pos(100,50)}Stra\xdfe\n"), 0o644) // Windows-1252
	var b bytes.Buffer
	if err := WriteASS(context.Background(), "", External{Path: ass, Format: "ass"}, &b); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Straße", "Style: Schild", `{\pos(100,50)}`} {
		if !strings.Contains(b.String(), want) {
			t.Fatalf("fehlt %q in %q", want, b.String())
		}
	}
}
