package optimize

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flimmer-media/flimmer/internal/playback"
	"github.com/flimmer-media/flimmer/internal/probe"
)

func TestWindow(t *testing.T) {
	for _, c := range []struct {
		h, from, to int
		in          bool
	}{{2, 2, 6, true}, {5, 2, 6, true}, {6, 2, 6, false}, {1, 2, 6, false}, {23, 22, 6, true}, {3, 22, 6, true}, {12, 22, 6, false}} {
		if inWindow(c.h, c.from, c.to) != c.in {
			t.Errorf("%d in %d–%d", c.h, c.from, c.to)
		}
	}
	now := time.Date(2026, 9, 28, 23, 30, 0, 0, time.Local)
	if e := windowEnd(now, 6); !e.Equal(time.Date(2026, 9, 29, 6, 0, 0, 0, time.Local)) {
		t.Errorf("Ende %v", e)
	}
	if e := windowEnd(now.Add(-20*time.Hour), 6); e.Day() != 28 {
		t.Errorf("Ende am selben Tag erwartet: %v", e)
	}
}

func TestArgs(t *testing.T) {
	it := Item{Path: "/m/film.mkv", Media: &probe.Media{Streams: []probe.Stream{
		{Index: 0, Type: "video", Codec: "hevc"},
		{Index: 1, Type: "audio", Codec: "eac3", Channels: 6},
		{Index: 2, Type: "audio", Codec: "dts", Channels: 6},
		{Index: 3, Type: "audio", Codec: "opus", Channels: 2},
	}}}
	a := strings.Join(Args(it, false, "/d/x.mp4.part"), " ")
	if !strings.Contains(a, "-vf scale=") || strings.Contains(a, "tonemap") {
		t.Errorf("SDR: %s", a)
	}
	if h := strings.Join(Args(it, true, "/d/x.mp4.part"), " "); !strings.Contains(h, "tonemap") {
		t.Errorf("HDR ohne Tone-Mapping: %s", h)
	}
	for _, want := range []string{"-c:a:0 copy", "-c:a:1 eac3", "-c:a:2 aac", "-movflags +faststart", "-f mp4 /d/x.mp4.part", "-sn"} {
		if !strings.Contains(a, want) {
			t.Errorf("fehlt %q in %s", want, a)
		}
	}
	if strings.Contains(a, "/m/film.mkv.") || !strings.Contains(a, "-i /m/film.mkv") {
		t.Error("Eingabe falsch")
	}
}

var tv = playback.Profile{Name: "TV", Containers: []string{"mp4", "mkv"}, Video: []string{"h264"}, Audio: []string{"aac"}}

func media(codec string) *probe.Media {
	return &probe.Media{Container: "matroska,webm", Duration: 60, Streams: []probe.Stream{
		{Index: 0, Type: "video", Codec: codec, Width: 1920, Height: 1080, PixFmt: "yuv420p"},
		{Index: 1, Type: "audio", Codec: "aac", Channels: 2}}}
}

func fakeTransfer(t *testing.T, trc map[string]string) {
	old := colorTransfer
	colorTransfer = func(_ context.Context, path string) (string, error) {
		if v, ok := trc[filepath.Base(path)]; ok {
			return v, nil
		}
		return "", errors.New("ffprobe kaputt")
	}
	t.Cleanup(func() { colorTransfer = old })
}

func TestPendingAndLookup(t *testing.T) {
	fakeTransfer(t, map[string]string{"a.mkv": "bt709"})
	dir, lib := t.TempDir(), t.TempDir()
	src := filepath.Join(lib, "a.mkv")
	os.WriteFile(src, nil, 0o644)
	items := []Item{
		{ID: "red", Path: src, Media: media("hevc")},
		{ID: "green", Path: src, Media: media("h264")},
		{ID: "stale", Path: src, Media: media("hevc")},
	}
	o := New(Options{Dir: dir, MinFree: 1, Items: func(context.Context) ([]Item, error) { return items, nil },
		Profiles: func(context.Context) []playback.Profile { return []playback.Profile{tv} }})

	old := time.Now().Add(-time.Hour)
	os.WriteFile(filepath.Join(dir, "stale.mp4"), nil, 0o644)
	os.Chtimes(filepath.Join(dir, "stale.mp4"), old, old) // älter als das Original
	os.WriteFile(filepath.Join(dir, "gone.mp4"), nil, 0o644)
	os.WriteFile(filepath.Join(dir, "red.mp4.part"), nil, 0o644)

	todo, err := o.pending(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, it := range todo {
		ids = append(ids, it.ID)
	}
	if !slices.Equal(ids, []string{"red", "stale"}) {
		t.Fatalf("pending = %v", ids)
	}
	for _, f := range []string{"gone.mp4", "red.mp4.part"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			t.Errorf("%s nicht aufgeräumt", f)
		}
	}
	if o.Lookup("stale", src) != "" {
		t.Error("veraltete Version darf nicht gelten")
	}
	os.WriteFile(filepath.Join(dir, "red.mp4"), nil, 0o644)
	if o.Lookup("red", src) == "" {
		t.Error("frische Version nicht gefunden")
	}
	o.failed["stale"] = "x"
	if todo, _ := o.pending(context.Background()); len(todo) != 0 {
		t.Errorf("gescheiterte/fertige Titel erneut: %v", todo)
	}
}

func TestShouldStop(t *testing.T) {
	now := time.Date(2026, 9, 29, 3, 0, 0, 0, time.Local)
	var busy atomic.Bool
	o := New(Options{Dir: t.TempDir(), MinFree: 1, Busy: busy.Load})
	o.now = func() time.Time { return now }
	deadline := now.Add(time.Hour)
	if err := o.shouldStop(now.Add(-time.Minute), 10, 100, deadline); err != nil {
		t.Fatalf("zu früh für Hochrechnung: %v", err)
	}
	// 10 min für 600 s von 7200 s → noch 110 min, Fenster hat nur 60.
	var slow errTooSlow
	if err := o.shouldStop(now.Add(-10*time.Minute), 600, 7200, deadline); !errors.As(err, &slow) {
		t.Fatalf("zu langsam erwartet: %v", err)
	}
	if err := o.shouldStop(now.Add(-10*time.Minute), 3600, 7200, deadline); err != nil {
		t.Fatalf("passt: %v", err)
	}
	busy.Store(true)
	if err := o.shouldStop(now, 0, 100, deadline); !errors.Is(err, errBusy) {
		t.Fatalf("Wiedergabe: %v", err)
	}
	busy.Store(false)
	if err := o.shouldStop(now, 0, 100, now); !errors.Is(err, errWindow) {
		t.Fatalf("Fenster: %v", err)
	}
}

func TestHDROf(t *testing.T) {
	fakeTransfer(t, map[string]string{"pq.mkv": "smpte2084", "hlg.mkv": "arib-std-b67", "sdr.mkv": "bt709", "leer.mkv": "", "dv5.mkv": ""})
	o := New(Options{Dir: t.TempDir()})
	dv := media("hevc")
	dv.Streams[0].HDR = "dv" // Profil 5: kein Transfer-Tag, aber probe erkennt es
	for name, want := range map[string]bool{"pq.mkv": true, "hlg.mkv": true, "sdr.mkv": false, "leer.mkv": false, "dv5.mkv": true} {
		m := media("hevc")
		if name == "dv5.mkv" {
			m = dv
		}
		if got, err := o.hdrOf(context.Background(), Item{ID: name, Path: "/x/" + name, Media: m}); err != nil || got != want {
			t.Errorf("%s: %v %v", name, got, err)
		}
	}
	if _, err := o.hdrOf(context.Background(), Item{ID: "k", Path: "/x/kaputt.mkv", Media: media("hevc")}); err == nil {
		t.Error("ffprobe-Fehler muss durchkommen")
	}
}

// TestHDRProbe prüft die echte ffprobe-Abfrage an einem als PQ markierten Clip.
func TestHDRProbe(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg fehlt")
	}
	src := filepath.Join(t.TempDir(), "pq.mkv")
	out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=320x180:duration=1",
		"-vf", "setparams=color_trc=smpte2084:color_primaries=bt2020:colorspace=bt2020nc", "-c:v", "libx264", src).CombinedOutput()
	if err != nil {
		t.Skipf("Testclip: %v %s", err, out)
	}
	o := New(Options{Dir: t.TempDir(), MinFree: 1})
	ctx := context.Background()
	m, err := probe.File(ctx, src)
	if err != nil {
		t.Skip(err)
	}
	m.Streams[0].HDR = "" // wie ein alter Probe-Cache ohne das Feld
	it := Item{ID: "pq", Path: src, Media: m}
	if hdr, err := o.hdrOf(ctx, it); err != nil || !hdr {
		t.Fatalf("PQ-Clip nicht als HDR erkannt: %v", err)
	}
	// Die Version ist SDR (BT.709), nicht blass-PQ.
	if err := o.encode(ctx, it, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	trc, err := colorTransfer(ctx, filepath.Join(o.opts.Dir, "pq.mp4"))
	if err != nil || trc != "bt709" {
		t.Fatalf("Ergebnis color_transfer = %q (%v)", trc, err)
	}
}

// TestEncode läuft nur mit ffmpeg: echte MP4-Version aus einem MKV mit HEVC-fremdem Codec (mpeg4) und MP3.
func TestEncode(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg fehlt")
	}
	lib, dir := t.TempDir(), t.TempDir()
	src := filepath.Join(lib, "clip.mkv")
	out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=640x360:rate=25:duration=3",
		"-f", "lavfi", "-i", "sine=duration=3", "-c:v", "mpeg4", "-c:a", "libmp3lame", src).CombinedOutput()
	if err != nil {
		t.Skipf("Testclip: %v %s", err, out)
	}
	ctx := context.Background()
	m, err := probe.File(ctx, src)
	if err != nil {
		t.Skip(err)
	}
	it := Item{ID: "clip", Title: "Clip", Path: src, Media: m}
	o := New(Options{Dir: dir, MinFree: 1})
	if err := o.encode(ctx, it, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	p := o.Lookup("clip", src)
	if p == "" {
		t.Fatal("keine Version")
	}
	v, err := probe.File(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(v.Container, "mp4") || v.First("video").Codec != "h264" || v.First("audio").Codec != "aac" {
		t.Fatalf("Ergebnis: %+v", v)
	}
	if playback.Decide(v, tv, 0).Light != playback.Green {
		t.Fatal("Version sollte auf dem TV grün sein")
	}

	// Abbruch bei Wiedergabe: keine Datei, kein .part.
	old := checkEvery
	checkEvery = 20 * time.Millisecond
	defer func() { checkEvery = old }()
	o2 := New(Options{Dir: t.TempDir(), MinFree: 1, Busy: func() bool { return true }})
	if err := o2.encode(ctx, it, time.Now().Add(time.Hour)); err != nil && !errors.Is(err, errBusy) {
		t.Fatalf("Abbruch: %v", err)
	}
	if entries, _ := os.ReadDir(o2.opts.Dir); len(entries) > 1 { // höchstens eine fertige Version, falls ffmpeg schneller war
		t.Fatalf("Reste: %v", entries)
	}
}
