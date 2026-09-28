package share

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Bavarianator/flimmer/internal/db"
)

func setup(t *testing.T) (*Share, *db.DB) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "flimmer.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	if err := d.UpdateSettings(ctx, func(s *db.Settings) { s.Dirs = []string{"/media/filme", "/media/serien"} }); err != nil {
		t.Fatal(err)
	}
	_, err = d.ExecContext(ctx, `INSERT INTO items(id, path, size, mtime, added_at, kind, title, container, duration, bitrate)
VALUES('it1', '/media/privat/urlaub.mkv', 1, 1, 1, 'movie', 'Urlaub', 'mkv', 60, 1)`)
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{DB: d, Port: 8097, Login: func(http.ResponseWriter, *http.Request, db.User) error { return nil }})
	return s, d
}

func TestInviteLifecycle(t *testing.T) {
	s, d := setup(t)
	ctx := context.Background()
	scope := Scope{Libraries: []string{"/media/filme"}, Items: []string{"it1"}}

	for _, c := range []struct {
		scope Scope
		ttl   time.Duration
		max   int
	}{
		{Scope{}, 24 * time.Hour, 0},
		{Scope{Libraries: []string{"/etc"}}, 24 * time.Hour, 0},
		{Scope{Items: []string{"gibtsnicht"}}, 24 * time.Hour, 0},
		{scope, time.Minute, 0},
		{scope, 24 * time.Hour, 1000},
	} {
		if _, _, err := s.Create(ctx, "admin", "", c.scope, c.ttl, c.max); err == nil {
			t.Errorf("Create(%+v, %v, %d) hätte scheitern müssen", c.scope, c.ttl, c.max)
		}
	}

	token, inv, err := s.Create(ctx, "admin", "Für Oma", scope, 48*time.Hour, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Redeem(ctx, token[:len(token)-2]+"AA", "x"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("gefälschte Signatur: %v", err)
	}
	u, err := s.Redeem(ctx, token, "  Oma\x07 ")
	if err != nil {
		t.Fatal(err)
	}
	if u.Name != "Oma" || u.Admin || u.PassHash != "" || !s.IsGuest(ctx, u.ID) {
		t.Fatalf("Gast: %+v", u)
	}
	sc, err := s.Scope(ctx, u.ID)
	if err != nil || sc == nil {
		t.Fatalf("Scope: %v %v", sc, err)
	}
	for path, want := range map[string]bool{
		"/media/filme/a.mkv": true, "/media/filme/x/b.mkv": true, "/media/filme-privat/c.mkv": false,
		"/media/serien/d.mkv": false, "/media/filme/../privat/e.mkv": false,
	} {
		if sc.Allows("?", path) != want {
			t.Errorf("Allows(%s) != %v", path, want)
		}
	}
	if !sc.Allows("it1", "/media/privat/urlaub.mkv") {
		t.Error("freigegebener Einzeltitel fehlt")
	}
	if sc, err := s.Scope(ctx, "admin"); sc != nil || err != nil {
		t.Error("normaler Benutzer darf keinen Scope haben")
	}

	if _, err := s.Redeem(ctx, token, "Opa"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Redeem(ctx, token, "Dritter"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("max. 2 Nutzungen: %v", err)
	}
	list, _ := s.List(ctx)
	if len(list) != 1 || list[0].Uses != 2 || list[0].Guests != 2 || list[0].Note != "Für Oma" {
		t.Fatalf("List: %+v", list)
	}

	// Widerruf löscht Gäste samt Sessions.
	d.CreateSession(ctx, "h1", u.ID, "", time.Hour)
	if err := s.Revoke(ctx, inv.ID); err != nil {
		t.Fatal(err)
	}
	if su, _ := d.SessionUser(ctx, "h1", time.Hour); su != nil || s.IsGuest(ctx, u.ID) {
		t.Fatal("Gast lebt nach Widerruf weiter")
	}
	if err := s.Revoke(ctx, inv.ID); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("doppelter Widerruf: %v", err)
	}
}

func TestExpiry(t *testing.T) {
	s, d := setup(t)
	ctx := context.Background()
	token, _, err := s.Create(ctx, "admin", "", Scope{Items: []string{"it1"}}, 2*time.Hour, 0)
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.Redeem(ctx, token, "Gast")
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Now().Add(3 * time.Hour) }
	if _, err := s.Scope(ctx, u.ID); !errors.Is(err, ErrExpired) {
		t.Fatalf("abgelaufen: %v", err)
	}
	if _, err := s.Redeem(ctx, token, "Spät"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("abgelaufen einlösen: %v", err)
	}
	if err := s.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	if gone, err := d.User(ctx, u.ID); err != nil || gone != nil {
		t.Fatal("abgelaufener Gast nicht aufgeräumt")
	}
}

func TestRedeemConcurrent(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	token, _, err := s.Create(ctx, "admin", "", Scope{Items: []string{"it1"}}, 24*time.Hour, 3)
	if err != nil {
		t.Fatal(err)
	}
	var ok atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			if _, err := s.Redeem(ctx, token, "x"); err == nil {
				ok.Add(1)
			}
		})
	}
	wg.Wait()
	if ok.Load() != 3 {
		t.Fatalf("%d Einlösungen, erwartet 3", ok.Load())
	}
}

func TestHandlers(t *testing.T) {
	s, _ := setup(t)
	var loggedIn string
	s.opts.Login = func(_ http.ResponseWriter, _ *http.Request, u db.User) error { loggedIn = u.ID; return nil }
	s.opts.UserID = func(*http.Request) string { return "admin1" }

	body, _ := json.Marshal(map[string]any{"note": "Kino", "items": []string{"it1"}, "hours": 24, "maxUses": 0})
	rec := httptest.NewRecorder()
	s.CreateHandler(rec, httptest.NewRequest("POST", "/api/invites", bytes.NewReader(body)))
	var out struct {
		URL, QR, Hint string
		Invite        Invite
	}
	json.NewDecoder(rec.Body).Decode(&out)
	if rec.Code != 200 || !strings.Contains(out.URL, "/einladung#") || !strings.HasPrefix(out.QR, "data:image/png;base64,") ||
		!strings.Contains(out.Hint, "Heimnetz") || out.Invite.CreatedBy != "admin1" {
		t.Fatalf("Create: %d %+v", rec.Code, out)
	}
	token := out.URL[strings.Index(out.URL, "#")+1:]

	redeem := func(tok string) int {
		b, _ := json.Marshal(map[string]string{"token": tok, "name": "Gast"})
		req := httptest.NewRequest("POST", "/api/invites/redeem", bytes.NewReader(b))
		req.RemoteAddr = "203.0.113.9:1234"
		rec := httptest.NewRecorder()
		s.RedeemHandler(rec, req)
		return rec.Code
	}
	if c := redeem(token); c != 200 || loggedIn == "" {
		t.Fatalf("Einlösen: %d", c)
	}
	if c := redeem("kaputt"); c != http.StatusGone {
		t.Fatalf("falsches Token: %d", c)
	}
	codes := []int{}
	for range 5 {
		codes = append(codes, redeem("kaputt"))
	}
	if codes[len(codes)-1] != http.StatusTooManyRequests {
		t.Fatalf("Rate-Limit greift nicht: %v", codes)
	}

	rec = httptest.NewRecorder()
	s.ListHandler(rec, httptest.NewRequest("GET", "/api/invites", nil))
	if rec.Code != 200 || strings.Contains(rec.Body.String(), token) {
		t.Fatalf("List darf das Token nicht verraten: %s", rec.Body)
	}
	rec = httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/invites/x", nil)
	req.SetPathValue("id", out.Invite.ID)
	s.RevokeHandler(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("Widerruf: %d", rec.Code)
	}
}
