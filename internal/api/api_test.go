package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/flimmer-media/flimmer/internal/scan"
	"github.com/flimmer-media/flimmer/internal/transcode"
)

// Scan → Status → Playlist (Keyframes erst auf Abruf) → Range-Request auf die Originaldatei.
func TestEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg fehlt")
	}
	media, data := t.TempDir(), t.TempDir()
	clip := filepath.Join(media, "Testfilm (2024).mkv")
	if b, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=8",
		"-g", "25", "-c:v", "libx264", clip).CombinedOutput(); err != nil {
		t.Fatalf("testclip: %v %s", err, b)
	}
	lib := scan.NewLibrary([]string{media}, data)
	if err := lib.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer((&Server{Lib: lib, HLS: transcode.NewManager(t.TempDir()), CacheDir: data,
		Web: fstest.MapFS{"index.html": {}}, FFmpeg: true}).Handler())
	defer srv.Close()

	var st struct {
		Scanning bool  `json:"scanning"`
		Found    int64 `json:"found"`
	}
	res, err := http.Get(srv.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	json.NewDecoder(res.Body).Decode(&st)
	if st.Found != 1 || st.Scanning {
		t.Fatalf("status %+v", st)
	}

	id := lib.All()[0].ID
	res, err = http.Get(srv.URL + "/api/items/" + id + "/hls/-1/copy/copy/index.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	if n := strings.Count(string(b), ".ts"); res.StatusCode != 200 || n < 2 {
		t.Fatalf("playlist %d mit %d Segmenten:\n%s", res.StatusCode, n, b)
	}

	req, _ := http.NewRequest("GET", srv.URL+"/api/items/"+id+"/file", nil)
	req.Header.Set("Range", "bytes=0-99")
	res, err = http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != http.StatusPartialContent {
		t.Fatalf("range: %v %v", err, res.Status)
	}
}
