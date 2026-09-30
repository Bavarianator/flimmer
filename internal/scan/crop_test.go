package scan

import (
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Bavarianator/flimmer/internal/db"
)

// Erster Abruf ohne Ergebnis, Messung im Hintergrund, danach aus der Datenbank; ein Titel ohne Balken speichert w = 0.
func TestCropStored(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg fehlt")
	}
	dir := t.TempDir()
	for name, vf := range map[string]string{"Balken (2020).mkv": ",pad=320:240:0:52:black", "Voll (2021).mkv": ""} {
		size := "320x136"
		if vf == "" {
			size = "320x240"
		}
		if b, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size="+size+":rate=25:duration=6"+vf,
			"-c:v", "libx264", filepath.Join(dir, name)).CombinedOutput(); err != nil {
			t.Fatalf("Clip: %v %s", err, b)
		}
	}
	store, err := db.Open(filepath.Join(t.TempDir(), "flimmer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	l := NewLibrary(store.DB, []string{dir})
	if err := l.Scan(t.Context()); err != nil {
		t.Fatal(err)
	}
	bars, full := l.Get(ID(filepath.Join(dir, "Balken (2020).mkv"))), l.Get(ID(filepath.Join(dir, "Voll (2021).mkv")))
	for _, it := range []*Item{bars, full} {
		if c := l.Crop(it); c != nil {
			t.Fatalf("erster Abruf schon mit Ergebnis: %+v", c)
		}
		deadline := time.Now().Add(time.Minute)
		for n := 0; n == 0 && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			store.QueryRow("SELECT count(*) FROM crops WHERE item_id = ?", it.ID).Scan(&n)
			if n == 0 {
				l.Crop(it) // lief gerade eine andere Messung, stößt der nächste Abruf neu an
			}
		}
	}
	if c := l.Crop(bars); c == nil || c.Y != 0.2167 || c.H != 0.5667 {
		t.Errorf("Balken: %+v", c)
	}
	if c := l.Crop(full); c != nil {
		t.Errorf("ohne Balken: %+v", c)
	}
}
