package scan

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Bavarianator/flimmer/internal/db"
)

// Katalog überlebt einen Neustart, Keyframes kommen aus der DB, ein fehlender Ordner löscht nichts.
func TestCatalog(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg fehlt")
	}
	ctx := context.Background()
	media, data := t.TempDir(), t.TempDir()
	clip := filepath.Join(media, "Heat (1995).mkv")
	if b, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=160x120:rate=25:duration=4",
		"-g", "25", "-c:v", "libx264", clip).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, b)
	}
	store, err := db.Open(filepath.Join(data, "flimmer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	lib := NewLibrary(store.DB, []string{media})
	if err := lib.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	it := lib.All()[0]
	kf, err := lib.Keyframes(ctx, it)
	if err != nil || len(kf) < 3 {
		t.Fatalf("keyframes %v %v", kf, err)
	}

	// „Neustart“: neue Library lädt aus der DB, ohne zu scannen.
	lib2 := NewLibrary(store.DB, []string{media})
	if err := lib2.Load(ctx); err != nil {
		t.Fatal(err)
	}
	it2 := lib2.Get(it.ID)
	if it2 == nil || it2.Title != "Heat" || it2.Media.First("video") == nil || it2.Media.Duration < 3 {
		t.Fatalf("nach Load: %+v", it2)
	}
	if kf2, ok := loadKeyframes(ctx, store.DB, it.ID, it.Size, mtime(t, clip)); !ok || len(kf2) != len(kf) {
		t.Fatalf("Keyframes nicht aus der DB: %v", kf2)
	}

	// Bibliotheksordner als Symlink (NAS: /srv/media → /mnt/disk1): Titel werden gefunden, Pfad bleibt unter dem Link.
	link := filepath.Join(t.TempDir(), "medien")
	if err := os.Symlink(media, link); err != nil {
		t.Fatal(err)
	}
	libLink := NewLibrary(store.DB, []string{link})
	if err := libLink.Scan(ctx); err != nil || len(libLink.All()) != 1 || !strings.HasPrefix(libLink.All()[0].Path, link) {
		t.Fatalf("Symlink-Ordner: %v %d", err, len(libLink.All()))
	}
	if err := lib2.Scan(ctx); err != nil { // wieder unter dem Originalpfad
		t.Fatal(err)
	}

	// Ordner weg (NAS aus): Titel bleiben.
	gone := media + "-weg"
	os.Rename(media, gone)
	if err := lib2.Scan(ctx); err != nil || lib2.Get(it.ID) == nil {
		t.Fatalf("Titel bei fehlendem Ordner gelöscht: %v", err)
	}
	// Ordner wieder da, Datei aber gelöscht: jetzt verschwindet der Titel.
	os.Rename(gone, media)
	os.WriteFile(filepath.Join(media, "notiz.txt"), []byte("x"), 0o644)
	os.Remove(clip)
	if err := lib2.Scan(ctx); err != nil || lib2.Get(it.ID) != nil {
		t.Fatalf("gelöschte Datei bleibt: %v", err)
	}
}

func mtime(t *testing.T, p string) int64 {
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return fi.ModTime().UnixNano()
}
