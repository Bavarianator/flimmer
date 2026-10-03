package db

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

var ctx = context.Background()

func open(t *testing.T) (*DB, string) {
	t.Helper()
	dir := t.TempDir()
	d, err := Open(filepath.Join(dir, "flimmer.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d, dir
}

func TestMigrateAndSettings(t *testing.T) {
	d, dir := open(t)
	var v int
	d.QueryRow("PRAGMA user_version").Scan(&v)
	if v < 1 {
		t.Fatalf("user_version %d", v)
	}
	s, err := d.Settings(ctx)
	if err != nil || len(s.Secret) != 32 {
		t.Fatalf("Secret fehlt: %v %v", s, err)
	}
	if err := d.UpdateSettings(ctx, func(s *Settings) { s.ServerName, s.Dirs = "Wohnzimmer", []string{"/a", "/b"} }); err != nil {
		t.Fatal(err)
	}
	d.Close()

	// Wieder öffnen: Einstellungen und Schlüssel bleiben, keine erneute Migration.
	d2, err := Open(filepath.Join(dir, "flimmer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()
	s2, _ := d2.Settings(ctx)
	if s2.ServerName != "Wohnzimmer" || len(s2.Dirs) != 2 || string(s2.Secret) != string(s.Secret) {
		t.Fatalf("nach Neustart: %+v", s2)
	}
}

func TestUsersSessionsCascade(t *testing.T) {
	d, _ := open(t)
	admin := User{ID: "a", Name: "Anna", Admin: true, PassHash: "x"}
	if err := d.Setup(ctx, admin, func(s *Settings) { s.Dirs = []string{"/m"} }); err != nil {
		t.Fatal(err)
	}
	if err := d.Setup(ctx, User{ID: "m", Name: "Mallory", Admin: true}, func(*Settings) {}); err != ErrSetupDone {
		t.Fatalf("zweites Setup: %v", err)
	}
	d.CreateUser(ctx, User{ID: "b", Name: "Ben"})
	d.CreateSession(ctx, "h1", "b", "TV", time.Hour)
	d.SetProgress(ctx, "b", "film", 500, 1000)
	if u, _ := d.SessionUser(ctx, "h1", time.Hour); u == nil || u.Name != "Ben" {
		t.Fatalf("Session: %+v", u)
	}
	if err := d.DeleteUser(ctx, "b", func(User, int) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if u, _ := d.SessionUser(ctx, "h1", time.Hour); u != nil {
		t.Fatal("Session überlebt gelöschten Benutzer")
	}
	if p, _ := d.Progress(ctx, "b"); len(p) != 0 {
		t.Fatal("Fortschritt überlebt gelöschten Benutzer")
	}
}

func TestProgressRules(t *testing.T) {
	d, _ := open(t)
	d.CreateUser(ctx, User{ID: "u", Name: "U"})
	if p, _ := d.SetProgress(ctx, "u", "x", 60, 6000); p.Pos != 0 || p.Watched {
		t.Fatalf("unter 3 %%: %+v", p)
	}
	if all, _ := d.Progress(ctx, "u"); len(all) != 0 {
		t.Fatal("unter 3 % gespeichert")
	}
	if p, _ := d.SetProgress(ctx, "u", "x", 5500, 6000); !p.Watched {
		t.Fatalf("ab 90 %%: %+v", p)
	}
	if p, _ := d.SetProgress(ctx, "u", "x", 10, 6000); !p.Watched {
		t.Fatalf("Reinschauen löscht gesehen: %+v", p)
	}
	d.SetProgress(ctx, "u", "y", 3000, 6000)
	if p := d.ProgressOf(ctx, "u", "y"); p.Pos != 3000 {
		t.Fatalf("ProgressOf: %+v", p)
	}
	d.SetPref(ctx, "u", "Dark", TrackPref{Audio: "ger"})
	d.SetPref(ctx, "u", "Dark", TrackPref{Subtitle: "off"})
	if p := d.Pref(ctx, "u", "Dark"); p.Audio != "ger" || p.Subtitle != "off" {
		t.Fatalf("Pref: %+v", p)
	}
}

// Viele gleichzeitige Schreiber (Fortschritt von mehreren Geräten) dürfen nicht an „database is locked“ scheitern.
func TestConcurrentWrites(t *testing.T) {
	d, _ := open(t)
	d.CreateUser(ctx, User{ID: "u", Name: "U"})
	var wg sync.WaitGroup
	errs := make(chan error, 200)
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 10 {
				if _, err := d.SetProgress(ctx, "u", string(rune('a'+i)), float64(100+j), 1000); err != nil {
					errs <- err
				}
				d.Progress(ctx, "u")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestImportState(t *testing.T) {
	d, dir := open(t)
	path := filepath.Join(dir, "state.json")
	os.WriteFile(path, []byte(`{"version":1,
		"settings":{"serverName":"Alt","dirs":["/filme"],"secret":"MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="},
		"users":[{"id":"a","name":"Anna","admin":true,"passHash":"h"}],
		"sessions":{"abc":{"userId":"a","lastSeen":"2026-09-28T10:00:00Z"}},
		"progress":{"a":{"film":{"pos":600,"dur":6000,"updated":"2026-09-28T10:00:00Z"}}},
		"prefs":{"a":{"Dark":{"audio":"ger"}}},
		"devices":{"tv1":{"x":1}}}`), 0o600)
	ok, err := d.ImportState(ctx, path)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	s, _ := d.Settings(ctx)
	if s.ServerName != "Alt" || s.Dirs[0] != "/filme" || string(s.Secret) != "0123456789abcdef0123456789abcdef" || !d.SetupDone(ctx) {
		t.Fatalf("Einstellungen: %+v", s)
	}
	if p := d.ProgressOf(ctx, "a", "film"); p.Pos != 600 || d.Pref(ctx, "a", "Dark").Audio != "ger" || string(d.Device(ctx, "tv1")) != `{"x":1}` {
		t.Fatal("Fortschritt/Pref/Gerät nicht übernommen")
	}
	if _, err := os.Stat(path + ".migrated"); err != nil {
		t.Fatal("state.json nicht umbenannt")
	}
}

// Eine bestehende Datenbank auf älterem Stand wird hochmigriert – vorher entsteht flimmer.db.bak.
func TestMigrateExisting(t *testing.T) {
	d, dir := open(t)
	d.CreateUser(ctx, User{ID: "u", Name: "U"})
	for _, q := range []string{"ALTER TABLE streams DROP COLUMN forced", "ALTER TABLE streams DROP COLUMN hearing_impaired",
		"ALTER TABLE items DROP COLUMN probe_version", "ALTER TABLE streams DROP COLUMN hdr",
		"ALTER TABLE streams DROP COLUMN dv_profile", "ALTER TABLE streams DROP COLUMN dv_compat",
		"ALTER TABLE sessions DROP COLUMN client", "ALTER TABLE sessions DROP COLUMN ip", "ALTER TABLE users DROP COLUMN upload",
		"PRAGMA user_version = 1"} {
		if _, err := d.Exec(q); err != nil {
			t.Fatal(q, err)
		}
	}
	d.Close()
	d2, err := Open(filepath.Join(dir, "flimmer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()
	var v int
	d2.QueryRow("PRAGMA user_version").Scan(&v)
	if v != latestVersion() {
		t.Fatalf("user_version %d", v)
	}
	if _, err := d2.Exec("SELECT probe_version FROM items; SELECT hdr FROM streams"); err != nil {
		t.Fatal(err)
	}
	if u, _ := d2.User(ctx, "u"); u == nil {
		t.Fatal("Daten bei Migration verloren")
	}
	if _, err := os.Stat(filepath.Join(dir, "flimmer.db.bak")); err != nil {
		t.Fatal("kein Backup vor der Migration")
	}
}

func TestListsAndSessions(t *testing.T) {
	d, _ := open(t)
	d.CreateUser(ctx, User{ID: "u", Name: "U"})
	if err := d.CreateList(ctx, Collection, "", List{ID: "c", Name: "Alle", Items: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	d.CreateList(ctx, Playlist, "u", List{ID: "p", Name: "Meine", Items: []string{}})
	if l, _ := d.Lists(ctx, Collection, ""); len(l) != 1 || l[0].Items[0] != "a" {
		t.Errorf("Sammlungen: %+v", l)
	}
	if _, err := d.UpdateList(ctx, Playlist, "fremd", "p", func(*List) error { return nil }); err != ErrNotFound {
		t.Errorf("fremde Liste: %v", err)
	}
	d.CreateSession(ctx, "0123456789abcdef-rest", "u", "TV", time.Hour)
	d.SetSessionClient(ctx, "0123456789abcdef-rest", "LG webOS", "192.168.1.5")
	if s, _ := d.Sessions(ctx, time.Hour); len(s) != 1 || s[0].ID != "0123456789abcdef" || s[0].Client != "LG webOS" || s[0].User != "U" {
		t.Errorf("Sessions: %+v", s)
	}
	if err := d.DeleteSessionID(ctx, "0123456789abcdef"); err != nil || d.DeleteSessionID(ctx, "0123456789abcdef") != ErrNotFound {
		t.Errorf("abmelden: %v", err)
	}
}

func TestWatchStats(t *testing.T) {
	d, _ := open(t)
	if err := d.CreateUser(ctx, User{ID: "u1", Name: "Anna", Color: 2}); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"direct", "transcode"} {
		if err := d.AddWatch(ctx, "u1", "i1", 10, m, "Chrome"); err != nil {
			t.Fatal(err)
		}
	}
	w, err := d.WatchStats(ctx, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Nutzer) != 1 || w.Nutzer[0].Sekunden != 20 || w.Nutzer[0].Name != "Anna" || len(w.Titel) != 1 || len(w.Stunden) != 1 {
		t.Fatalf("%+v", w)
	}
	if len(w.Methoden) != 1 || w.Methoden[0].Name != "transcode" {
		t.Fatalf("Methode: %+v", w.Methoden)
	}
}
