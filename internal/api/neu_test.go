package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Bavarianator/flimmer/internal/auth"
	"github.com/Bavarianator/flimmer/internal/db"
	"github.com/Bavarianator/flimmer/internal/images"
	"github.com/Bavarianator/flimmer/internal/meta"
	"github.com/Bavarianator/flimmer/internal/scan"
	"github.com/Bavarianator/flimmer/internal/share"
	"github.com/Bavarianator/flimmer/internal/transcode"
)

// fixture ist ein Server mit kleiner Bibliothek ohne ffmpeg: Titel stehen direkt in der DB.
type fixture struct {
	t   *testing.T
	s   *Server
	srv *httptest.Server
	ids map[string]string // Kurzname → Titel-ID
}

type fItem struct {
	key, path, metaJSON string
}

func newFixture(t *testing.T, items ...fItem) *fixture {
	t.Helper()
	ctx := context.Background()
	store, err := db.Open(filepath.Join(t.TempDir(), "flimmer.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.UpdateSettings(ctx, func(s *db.Settings) { s.Dirs = []string{"/m"} }); err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, ids: map[string]string{}}
	metas := map[string]*meta.Meta{}
	for i, it := range items {
		p := scan.Parse(it.path)
		id := scan.ID(it.path)
		kind := "movie"
		if p.Series != "" {
			kind = "episode"
		}
		if _, err := store.Exec(`INSERT INTO items(id, path, size, mtime, added_at, kind, title, year, series, season, episode, container, duration, bitrate)
			VALUES(?, ?, 1, 1, ?, ?, ?, ?, ?, ?, ?, 'mov,mp4', 600, 1000000)`, id, it.path, i, kind, p.Title, p.Year, p.Series, p.Season, p.Episode); err != nil {
			t.Fatal(err)
		}
		store.Exec(`INSERT INTO streams(item_id, idx, type, codec, pix_fmt) VALUES(?, 0, 'video', 'h264', 'yuv420p'), (?, 1, 'audio', 'aac', '')`, id, id)
		if it.metaJSON != "" {
			var m meta.Meta
			if err := json.Unmarshal([]byte(it.metaJSON), &m); err != nil {
				t.Fatal(err)
			}
			metas[id] = &m
			if _, err := store.Exec(`INSERT INTO meta(item_id, source, json, updated_at) VALUES(?, 'tmdb', ?, 1)`, id, it.metaJSON); err != nil {
				t.Fatal(err)
			}
		}
		f.ids[it.key] = id
	}
	lib := scan.NewLibrary(store.DB, nil)
	if err := lib.Load(ctx); err != nil {
		t.Fatal(err)
	}
	for id, m := range metas {
		lib.SetMeta(id, m)
	}
	f.s = &Server{Lib: lib, HLS: transcode.NewManager(t.TempDir()), DB: store, CacheDir: t.TempDir(), Web: fstest.MapFS{},
		Meta: &meta.Resolver{DB: store.DB, CacheDir: t.TempDir(), TMDB: meta.NewTMDB("")}, Images: images.New(t.TempDir())}
	f.s.Share = share.New(share.Options{DB: store, BaseURL: func() (string, bool) { return "http://flimmer", false },
		Login: f.s.GuestLogin, UserID: func(r *http.Request) string { return UserFrom(r).ID }})
	f.srv = httptest.NewServer(f.s.Handler())
	t.Cleanup(f.srv.Close)
	return f
}

// user legt einen Benutzer an und liefert ein Session-Token.
func (f *fixture) user(u db.User) string {
	f.t.Helper()
	ctx := context.Background()
	if u.ID == "" {
		u.ID = u.Name
	}
	if err := f.s.DB.CreateUser(ctx, u); err != nil {
		f.t.Fatal(err)
	}
	return f.session(u.ID)
}

func (f *fixture) session(uid string) string {
	tok, h := auth.NewToken()
	if err := f.s.DB.CreateSession(context.Background(), h, uid, "test", time.Hour); err != nil {
		f.t.Fatal(err)
	}
	return tok
}

// guest lädt einen Gast mit Zugriff auf genau diese Titel ein und liefert sein Token.
func (f *fixture) guest(items ...string) string {
	f.t.Helper()
	ctx := context.Background()
	tok, _, err := f.s.Share.Create(ctx, "admin", "", share.Scope{Items: items}, 24*time.Hour, 0)
	if err != nil {
		f.t.Fatal(err)
	}
	u, err := f.s.Share.Redeem(ctx, tok, "Gast", "")
	if err != nil {
		f.t.Fatal(err)
	}
	return f.session(u.ID)
}

// call schickt eine Anfrage und dekodiert JSON nach out (falls nicht nil). body string = roh.
func (f *fixture) call(tok, method, path string, body, out any) int {
	f.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = bytes.NewReader([]byte(b))
	default:
		j, _ := json.Marshal(b)
		rd = bytes.NewReader(j)
	}
	req, _ := http.NewRequest(method, f.srv.URL+path, rd)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if out != nil && res.StatusCode < 300 {
		if err := json.Unmarshal(b, out); err != nil {
			f.t.Fatalf("%s %s: %v: %s", method, path, err, b)
		}
	}
	return res.StatusCode
}

func titles(list []libraryItem) []string {
	out := []string{}
	for _, it := range list {
		out = append(out, it.ID)
	}
	return out
}

func TestSearchRoute(t *testing.T) {
	f := newFixture(t,
		fItem{"lotr", "/m/Der Herr der Ringe (2001).mkv", `{"title":"Der Herr der Ringe","originalTitle":"The Lord of the Rings"}`},
		fItem{"hp", "/m/Harry Potter (2001).mkv", ""},
		fItem{"bb1", "/m/Breaking Bad/Season 01/Breaking Bad S01E01.mkv", ""},
		fItem{"bb2", "/m/Breaking Bad/Season 01/Breaking Bad S01E02.mkv", ""},
	)
	tok := f.user(db.User{Name: "anna", PassHash: "x"})
	var got []libraryItem
	if c := f.call(tok, "GET", "/api/search?q=her+der+ringe", nil, &got); c != 200 || len(got) == 0 || got[0].ID != f.ids["lotr"] {
		t.Fatalf("her der ringe: %d %v", c, titles(got))
	}
	if f.call(tok, "GET", "/api/search?q=lord+of+the+rings", nil, &got); len(got) == 0 || got[0].ID != f.ids["lotr"] {
		t.Errorf("Originaltitel: %v", titles(got))
	}
	// Je Serie ein Treffer, die früheste Folge.
	if f.call(tok, "GET", "/api/search?q=breaking", nil, &got); len(got) != 1 || got[0].ID != f.ids["bb1"] {
		t.Errorf("Serie: %v", titles(got))
	}
	if f.call(tok, "GET", "/api/search?q=", nil, &got); len(got) != 0 {
		t.Errorf("leere Suche: %v", titles(got))
	}
	// Gäste finden nur, was ihre Einladung erlaubt.
	g := f.guest(f.ids["hp"])
	if f.call(g, "GET", "/api/search?q=her+der+ringe", nil, &got); len(got) != 0 {
		t.Errorf("Gast findet gesperrten Titel: %v", titles(got))
	}
	if f.call(g, "GET", "/api/search?q=harry", nil, &got); len(got) != 1 {
		t.Errorf("Gast findet erlaubten Titel nicht: %v", titles(got))
	}
	if c := f.call("", "GET", "/api/search?q=harry", nil, nil); c != 401 {
		t.Errorf("ohne Anmeldung: %d", c)
	}
}
