package db

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// Backup → frische Installation → Restore ergibt identische Daten.
func TestBackupRestore(t *testing.T) {
	src, dir := open(t)
	src.Setup(ctx, User{ID: "a", Name: "Anna", Admin: true, PassHash: "h"}, func(s *Settings) { s.ServerName, s.Dirs = "Wohnzimmer", []string{"/filme"} })
	src.CreateUser(ctx, User{ID: "b", Name: "Ben"})
	src.SetProgress(ctx, "b", "film", 3000, 6000)
	src.SetPref(ctx, "b", "Dark", TrackPref{Audio: "ger"})
	backup := filepath.Join(dir, "backup.db")
	if err := src.BackupTo(ctx, backup); err != nil {
		t.Fatal(err)
	}

	dst, _ := open(t) // frische Installation mit eigenem Admin und anderem Schlüssel
	dst.Setup(ctx, User{ID: "x", Name: "Neu", Admin: true, PassHash: "h"}, func(*Settings) {})
	if err := dst.Restore(ctx, backup); err != nil {
		t.Fatal(err)
	}
	for name, f := range map[string]func(*DB) any{
		"users":    func(d *DB) any { u, _ := d.Users(ctx); return u },
		"settings": func(d *DB) any { s, _ := d.Settings(ctx); return s },
		"progress": func(d *DB) any { p, _ := d.Progress(ctx, "b"); return p["film"].Pos },
		"pref":     func(d *DB) any { return d.Pref(ctx, "b", "Dark") },
	} {
		if a, b := f(src), f(dst); !reflect.DeepEqual(a, b) {
			t.Errorf("%s nach Restore verschieden:\n %v\n %v", name, a, b)
		}
	}
	if u, _ := dst.User(ctx, "x"); u != nil {
		t.Error("alter Admin der frischen Installation ist noch da")
	}
}

func TestRestoreRejects(t *testing.T) {
	d, dir := open(t)
	junk := filepath.Join(dir, "junk.db")
	os.WriteFile(junk, []byte("kein sqlite"), 0o600)
	if err := d.Restore(ctx, junk); err == nil {
		t.Error("Datenmüll akzeptiert")
	}
	noAdmin := filepath.Join(dir, "leer.db")
	d.BackupTo(ctx, noAdmin) // frische DB ohne Benutzer
	if _, ok := d.Restore(ctx, noAdmin).(ErrBadBackup); !ok {
		t.Error("Backup ohne Admin akzeptiert")
	}
}

func TestNightlyKeepsSeven(t *testing.T) {
	d, dir := open(t)
	bdir := filepath.Join(dir, "backups")
	for i := range 9 {
		if err := d.nightlyBackup(ctx, bdir, time.Date(2026, 9, 1+i, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), 7); err != nil {
			t.Fatal(err)
		}
	}
	files, _ := filepath.Glob(filepath.Join(bdir, "*.db"))
	if len(files) != 7 || filepath.Base(files[0]) != "flimmer-2026-09-03.db" {
		t.Fatalf("%v", files)
	}
	if m := d.Maintenance(ctx); m.BackupFile != files[6] || d.Check(ctx) != "ok" {
		t.Fatalf("Wartung: %+v", m)
	}
}
