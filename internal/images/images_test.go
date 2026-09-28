package images

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// poster schreibt ein 600×900-PNG in einer Farbe.
func poster(t *testing.T, c color.RGBA) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 600, 900))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
	}
	p := filepath.Join(t.TempDir(), "poster.png")
	f, _ := os.Create(p)
	png.Encode(f, img)
	f.Close()
	return p
}

func size(t *testing.T, path string) image.Point {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, err := jpeg.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	return image.Pt(cfg.Width, cfg.Height)
}

func TestScaled(t *testing.T) {
	s := New(t.TempDir())
	src := Source{Path: poster(t, color.RGBA{200, 40, 40, 255})}
	for _, tt := range []struct{ w, want int }{{300, 300}, {1920, 600}} { // nie hochskalieren
		p, err := s.Scaled(context.Background(), src, tt.w)
		if err != nil {
			t.Fatal(err)
		}
		if got := size(t, p); got.X != tt.want || got.Y != tt.want*3/2 {
			t.Errorf("w=%d: %v, will %d breit im Seitenverhältnis 2:3", tt.w, got, tt.want)
		}
	}
	// Neues Poster (andere Änderungszeit/Größe) → neue Cache-Datei statt altem Bild
	p1, _ := s.Scaled(context.Background(), src, 300)
	os.WriteFile(src.Path, mustRead(t, poster(t, color.RGBA{0, 0, 255, 255})), 0o644)
	os.Chtimes(src.Path, fixed, fixed)
	p2, _ := s.Scaled(context.Background(), src, 300)
	if p1 == p2 {
		t.Error("geändertes Poster liefert alten Cache")
	}
}

func TestColor(t *testing.T) {
	s := New(t.TempDir())
	c, err := s.Color(context.Background(), Source{Path: poster(t, color.RGBA{200, 40, 40, 255})})
	if err != nil {
		t.Fatal(err)
	}
	// JPEG verschiebt Farben leicht
	var r, g, b int
	if _, err := fmtSscan(c, &r, &g, &b); err != nil || abs(r-200) > 6 || abs(g-40) > 6 || abs(b-40) > 6 {
		t.Errorf("Farbe %s, erwartet ≈ #c82828", c)
	}
}

func TestHandler(t *testing.T) {
	s := New(t.TempDir())
	src := Source{Path: poster(t, color.RGBA{10, 20, 30, 255})}
	mux := http.NewServeMux()
	mux.Handle("GET /api/images/{id}/{kind}", s.Handler(func(id, kind string) (Source, bool) {
		return src, id == "x" && kind == "poster"
	}))
	for _, tt := range []struct {
		url   string
		code  int
		width int
		cache string
	}{
		{"/api/images/x/poster?w=250&v=abc", 200, 300, "public, max-age=31536000, immutable"},
		{"/api/images/x/poster", 200, 300, "public, max-age=86400"},
		{"/api/images/x/backdrop", 404, 0, ""},
		{"/api/images/x/quatsch", 404, 0, ""},
		{"/api/images/y/poster", 404, 0, ""},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", tt.url, nil))
		if rec.Code != tt.code {
			t.Errorf("%s: %d, erwartet %d", tt.url, rec.Code, tt.code)
			continue
		}
		if tt.code != 200 {
			continue
		}
		if got := rec.Header().Get("Cache-Control"); got != tt.cache {
			t.Errorf("%s: Cache-Control %q", tt.url, got)
		}
		cfg, err := jpeg.DecodeConfig(rec.Body)
		if err != nil || cfg.Width != tt.width {
			t.Errorf("%s: Breite %d (%v), erwartet %d", tt.url, cfg.Width, err, tt.width)
		}
	}
}

func TestStandbild(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg fehlt")
	}
	video := filepath.Join(t.TempDir(), "ep.mkv")
	if b, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=640x360:rate=25:duration=5",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", video).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	s := New(t.TempDir())
	p, err := s.Scaled(context.Background(), Source{Path: video, Video: true, Duration: 5}, 300)
	if err != nil {
		t.Fatal(err)
	}
	if got := size(t, p); got != image.Pt(300, 168) {
		t.Errorf("Standbild %v, erwartet 300×168", got)
	}
}

var fixed = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func mustRead(t *testing.T, p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fmtSscan(c string, r, g, b *int) (int, error) { return fmt.Sscanf(c, "#%02x%02x%02x", r, g, b) }

func abs(x int) int { return max(x, -x) }
