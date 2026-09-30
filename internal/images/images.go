// Package images liefert Poster, Hintergründe und Standbilder in der Größe, die das Gerät braucht.
// Skaliert wird einmal und als JPEG gecacht; schwache TVs laden so 300 px statt 2000 px.
package images

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"time"

	"github.com/Bavarianator/flimmer/internal/ffmpeg"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Source beschreibt, woraus ein Bild entsteht: eine Bilddatei oder – für Standbilder – ein Video.
type Source struct {
	Path     string
	Video    bool      // Standbild aus dem Video ziehen
	Duration float64   // Sekunden, nur bei Video; ohne At/Pick kommt das Bild von 20 %
	At       float64   // Sekunden: genau dieses Bild (Kapitel, /frame)
	Pick     []float64 // Sekunden: Kandidaten, das hellste gewinnt (Hintergrund-Ersatz, meidet dunkle Szenen)
}

// Lookup liefert die Quelle für Titel id und Art kind (poster, backdrop, still).
type Lookup func(id, kind string) (Source, bool)

type Store struct {
	Dir    string // Cache für skalierte Bilder, Standbilder und Farben
	sem    chan struct{}
	frames chan struct{} // höchstens zwei ffmpeg-Läufe für Standbilder gleichzeitig (NAS mit 2 Kernen)
}

func New(dir string) *Store {
	return &Store{Dir: dir, sem: make(chan struct{}, runtime.NumCPU()), frames: make(chan struct{}, 2)}
}

// Feste Breiten, damit der Cache nicht für jede krumme Zahl eine Datei anlegt.
var widths = []int{160, 300, 500, 780, 1280, 1920}

func bucket(w int) int {
	for _, b := range widths {
		if w <= b {
			return b
		}
	}
	return widths[len(widths)-1]
}

// Handler: GET /api/images/{id}/{kind}?w=300[&v=…]
// Mit v (z. B. der Poster-Dateiname aus meta) ändert sich die URL, wenn sich das Bild ändert → immutable.
func (s *Store) Handler(lookup Lookup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := r.PathValue("kind")
		if !slices.Contains([]string{"poster", "backdrop", "still"}, kind) {
			http.NotFound(w, r)
			return
		}
		src, ok := lookup(r.PathValue("id"), kind)
		if !ok {
			http.NotFound(w, r)
			return
		}
		s.Serve(w, r, src)
	}
}

// Serve liefert src skaliert auf ?w= (Standard 300) als JPEG; mit ?v= als unveränderlich gecacht.
func (s *Store) Serve(w http.ResponseWriter, r *http.Request, src Source) {
	width, _ := strconv.Atoi(r.URL.Query().Get("w"))
	if width <= 0 {
		width = 300
	}
	path, err := s.Scaled(r.Context(), src, bucket(width))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.URL.Query().Get("v") != "" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, path)
}

// key hängt an Pfad, Größe und Änderungszeit: Ein neues Poster ergibt automatisch neue Cache-Dateien.
func key(src Source) (string, error) {
	fi, err := os.Stat(src.Path)
	if err != nil {
		return "", err
	}
	b := fmt.Appendf(nil, "%s|%d|%d", src.Path, fi.Size(), fi.ModTime().UnixNano())
	if src.At > 0 || len(src.Pick) > 0 { // ohne bleibt der Schlüssel wie vorher (bestehender Cache gilt weiter)
		b = fmt.Appendf(b, "|%g|%v", src.At, src.Pick)
	}
	if src.Video { // Standbilder seit der Entzerrung anamorpher Videos (SAR) neu erzeugen
		b = append(b, "|sar"...)
	}
	h := sha1.Sum(b)
	return hex.EncodeToString(h[:8]), nil
}

// Scaled liefert den Pfad eines JPEGs mit höchstens width Pixeln Breite (nie hochskaliert).
func (s *Store) Scaled(ctx context.Context, src Source, width int) (string, error) {
	k, err := key(src)
	if err != nil {
		return "", err
	}
	out := filepath.Join(s.Dir, k+"-"+strconv.Itoa(width)+".jpg")
	if _, err := os.Stat(out); err == nil {
		return out, nil
	}
	s.sem <- struct{}{} // höchstens NumCPU gleichzeitig, schont den Pi beim ersten Laden einer Übersicht
	defer func() { <-s.sem }()
	if _, err := os.Stat(out); err == nil {
		return out, nil
	}
	img, err := s.decode(ctx, src, k)
	if err != nil {
		return "", err
	}
	b := img.Bounds()
	if b.Dx() > width {
		dst := image.NewRGBA(image.Rect(0, 0, width, b.Dy()*width/b.Dx()))
		draw.BiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
		img = dst
	}
	return out, writeJPEG(out, img)
}

func (s *Store) decode(ctx context.Context, src Source, k string) (image.Image, error) {
	path := src.Path
	if src.Video {
		path = filepath.Join(s.Dir, k+"-frame.jpg")
		if _, err := os.Stat(path); err != nil {
			if err := s.bestFrame(ctx, src, path); err != nil {
				return nil, err
			}
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("bild %s: %w", filepath.Base(path), err)
	}
	return img, nil
}

// bestFrame zieht das Standbild nach out: bei At, sonst das hellste der Kandidaten in Pick, sonst bei 20 % der
// Laufzeit (vor dem Intro-Ende, selten ein Schwarzbild). Höchstens zwei laufen gleichzeitig, mit niedriger Priorität.
func (s *Store) bestFrame(ctx context.Context, src Source, out string) error {
	times := src.Pick
	switch {
	case src.At > 0:
		times = []float64{src.At}
	case len(times) == 0:
		times = []float64{src.Duration * 0.2}
	}
	select {
	case s.frames <- struct{}{}:
		defer func() { <-s.frames }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if _, err := os.Stat(out); err == nil { // entstand, während wir gewartet haben
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	best, bestLuma := "", -1.0
	for i, t := range times {
		tmp := out + ".tmp" + strconv.Itoa(i) + ".jpg"
		defer os.Remove(tmp)
		if err := frame(ctx, src.Path, t, tmp); err != nil {
			if len(times) == 1 {
				return err
			}
			continue // ein Kandidat hinter dem Ende o. ä.: die anderen reichen
		}
		if l, err := luma(tmp); err == nil && l > bestLuma {
			best, bestLuma = tmp, l
		}
	}
	if best == "" {
		return fmt.Errorf("standbild %s: kein Bild", filepath.Base(src.Path))
	}
	return os.Rename(best, out)
}

func frame(ctx context.Context, path string, at float64, out string) error {
	// Erst anamorphe Videos (DVD, SAR ≠ 1) entzerren, dann auf höchstens 1280 px verkleinern.
	name, args := ffmpeg.Nice("ffmpeg", []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-ss", strconv.FormatFloat(at, 'f', 2, 64), "-i", path,
		"-frames:v", "1", "-vf", "scale='trunc(iw*sar/2)*2':ih,setsar=1,scale='min(1280,iw)':-2", "-q:v", "3", out})
	if b, err := exec.CommandContext(ctx, name, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("standbild %s: %v: %s", filepath.Base(path), err, b)
	}
	if fi, err := os.Stat(out); err != nil || fi.Size() == 0 { // hinter dem Ende schreibt ffmpeg nichts und meldet keinen Fehler
		return fmt.Errorf("standbild %s bei %.0f s: kein Bild", filepath.Base(path), at)
	}
	return nil
}

// luma ist die mittlere Helligkeit (0–255) eines JPEGs, gemessen an einer 16×16-Verkleinerung.
func luma(path string) (float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		return 0, err
	}
	small := image.NewRGBA(image.Rect(0, 0, 16, 16))
	draw.ApproxBiLinear.Scale(small, small.Bounds(), img, img.Bounds(), draw.Src, nil)
	var sum float64
	for i := 0; i < len(small.Pix); i += 4 {
		sum += 0.299*float64(small.Pix[i]) + 0.587*float64(small.Pix[i+1]) + 0.114*float64(small.Pix[i+2])
	}
	return sum / float64(len(small.Pix)/4), nil
}

// Color liefert eine Platzhalterfarbe (#rrggbb), die die UI zeigt, bis das Bild geladen ist.
// Durchschnitt eines 16×16-Vorschaubilds: billig und ruhig genug als Fläche. Das Ergebnis wird gecacht.
func (s *Store) Color(ctx context.Context, src Source) (string, error) {
	k, err := key(src)
	if err != nil {
		return "", err
	}
	cache := filepath.Join(s.Dir, k+".color")
	if b, err := os.ReadFile(cache); err == nil && len(b) == 7 {
		return string(b), nil
	}
	// Die Kartengröße gleich mit erzeugen (läuft im Scan): Die erste Übersicht muss dann nichts mehr skalieren.
	if _, err := s.Scaled(ctx, src, 300); err != nil {
		return "", err
	}
	path, err := s.Scaled(ctx, src, widths[0])
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		return "", err
	}
	small := image.NewRGBA(image.Rect(0, 0, 16, 16))
	draw.ApproxBiLinear.Scale(small, small.Bounds(), img, img.Bounds(), draw.Src, nil)
	var r, g, b int
	for i := 0; i < len(small.Pix); i += 4 {
		r, g, b = r+int(small.Pix[i]), g+int(small.Pix[i+1]), b+int(small.Pix[i+2])
	}
	n := len(small.Pix) / 4
	c := hexColor(color.RGBA{uint8(r / n), uint8(g / n), uint8(b / n), 255})
	return c, os.WriteFile(cache, []byte(c), 0o644)
}

func hexColor(c color.RGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

func writeJPEG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	err = jpeg.Encode(f, img, &jpeg.Options{Quality: 85})
	if err = errors.Join(err, f.Close()); err != nil {
		os.Remove(f.Name())
		return err
	}
	return os.Rename(f.Name(), path)
}

// Prune löscht skalierte Bilder und Standbilder, die älter als maxAge sind; sie entstehen beim nächsten Abruf neu.
// Farben (*.color) bleiben, die braucht der Scan. Liefert die Zahl gelöschter Dateien.
func (s *Store) Prune(maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || e.IsDir() || filepath.Ext(e.Name()) == ".color" || time.Since(info.ModTime()) < maxAge {
			continue
		}
		if os.Remove(filepath.Join(s.Dir, e.Name())) == nil {
			n++
		}
	}
	return n, nil
}
