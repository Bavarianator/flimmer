package ffmpeg

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

var sums = []byte(`7be4e989f0b038d3cc777d97469e2c1b0db07b6eacb64b04d6db7509c1fc3fe7  ffmpeg-master-latest-win64-gpl.zip
36189b91862a6bf75c57223887c36c73654e3ada01d8bea64d5e1ba667ee7b7b  ffmpeg-n8.1-latest-win64-gpl-8.1.zip
e6db684f1527f4c2280b017c7af19ebd359424eee8b35974bc35b4d7ee110989  ffmpeg-n9.0-latest-win64-gpl-9.0.zip
14a3dc0a93aac34e531b69f337d6f0dcb525491918c0b19b2bce936dcbc9822b  ffmpeg-n9.0-latest-win64-gpl-shared-9.0.zip
3b4797796a062ff3d46ccaa45e9d29acd5ecc9e4eae0cb5102bf58e0591dd3f6  ffmpeg-n9.0-latest-linux64-gpl-9.0.tar.xz
6deacdeb98afe9a3c5857f864c6d16f0787d7e1b1cf7ebe69710857ce6ee1428  ffmpeg-n9.0-latest-linuxarm64-gpl-9.0.tar.xz
`)

func TestPickAsset(t *testing.T) {
	for plat, want := range map[string]string{
		"win64":      "ffmpeg-n9.0-latest-win64-gpl-9.0.zip",
		"linux64":    "ffmpeg-n9.0-latest-linux64-gpl-9.0.tar.xz",
		"linuxarm64": "ffmpeg-n9.0-latest-linuxarm64-gpl-9.0.tar.xz",
	} {
		if name, _, err := pickAsset(sums, plat); err != nil || name != want {
			t.Errorf("%s: %s %v", plat, name, err)
		}
	}
}

// Eine Binary mit falschem Hash darf nie im Zielordner landen.
func TestExtractZipChecksum(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "a.zip")
	f, _ := os.Create(zipPath)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("__MACOSX/._ffprobe")
	w.Write([]byte("müll"))
	w, _ = zw.Create("ffprobe")
	w.Write([]byte("echt"))
	zw.Close()
	f.Close()

	sum := sha256.Sum256([]byte("echt"))
	dst := filepath.Join(dir, "ffprobe")
	if err := extractZip(zipPath, "ffprobe", dst, "00"+hex.EncodeToString(sum[1:])); err == nil {
		t.Fatal("falscher Hash akzeptiert")
	}
	if _, err := os.Stat(dst); err == nil {
		t.Fatal("Datei trotz falschem Hash geschrieben")
	}
	if err := extractZip(zipPath, "ffprobe", dst, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "echt" {
		t.Fatalf("Inhalt %q", b)
	}
}

func TestProgress(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(make([]byte, 1000)) }))
	defer srv.Close()
	i := &Installer{}
	f, err := i.download(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(f)
	if st := i.Status(); st.Percent != 100 {
		t.Fatalf("Fortschritt %d %%", st.Percent)
	}
}
