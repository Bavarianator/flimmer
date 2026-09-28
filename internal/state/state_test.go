package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadAndBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Update(func(d *Data) error {
		d.Users = append(d.Users, User{ID: "a", Name: "Anna", Admin: true})
		d.SetProgress("a", "film", 600, 6000)
		return nil
	})
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".bak", mustRead(t, path), 0o600); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, []byte("{kaputt"), 0o600) // Absturz o. Ä.

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.View(func(d *Data) {
		if !d.SetupDone() || d.Progress["a"]["film"].Pos != 600 || len(d.Settings.Secret) != 32 {
			t.Fatalf("aus Backup falsch geladen: %+v", d)
		}
	})
}

func TestProgressRules(t *testing.T) {
	d := &Data{Progress: map[string]map[string]Progress{}}
	if p := d.SetProgress("u", "x", 60, 6000); p.Pos != 0 || p.Watched || len(d.Progress["u"]) != 0 {
		t.Fatalf("unter 3 %% gespeichert: %+v", p)
	}
	if p := d.SetProgress("u", "x", 5500, 6000); !p.Watched {
		t.Fatalf("ab 90 %% nicht gesehen: %+v", p)
	}
	if p := d.SetProgress("u", "x", 10, 6000); !p.Watched {
		t.Fatalf("Reinschauen hat „gesehen“ gelöscht: %+v", p)
	}
}

func mustRead(t *testing.T, p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
