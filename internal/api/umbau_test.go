package api

import (
	"log/slog"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/Bavarianator/flimmer/internal/auth"
	"github.com/Bavarianator/flimmer/internal/db"
	"github.com/Bavarianator/flimmer/internal/livetv"
	"github.com/Bavarianator/flimmer/internal/meta"
)

// Tests der Endpunkte aus docs/umbau-jellyfin.md („Neue Server-Endpunkte“).

func TestFavorites(t *testing.T) {
	f := newFixture(t,
		fItem{"hp", "/m/Harry Potter (2001).mkv", ""},
		fItem{"heat", "/m/Heat (1995).mkv", ""},
		fItem{"bb1", "/m/Breaking Bad/Season 01/Breaking Bad S01E01.mkv", ""},
	)
	tok := f.user(db.User{Name: "anna", PassHash: "x"})
	serie := "/api/favorites/" + url.PathEscape("serie:Breaking Bad")
	for _, p := range []string{"/api/favorites/" + f.ids["hp"], serie} {
		if c := f.call(tok, "PUT", p, nil, nil); c != 204 {
			t.Fatalf("PUT %s: %d", p, c)
		}
	}
	if c := f.call(tok, "PUT", "/api/favorites/gibtsnicht", nil, nil); c != 404 {
		t.Errorf("unbekannter Titel: %d", c)
	}
	var got []string
	if f.call(tok, "GET", "/api/favorites", nil, &got); !slices.Contains(got, f.ids["hp"]) || !slices.Contains(got, "serie:Breaking Bad") || len(got) != 2 {
		t.Fatalf("GET: %v", got)
	}
	f.call(tok, "DELETE", serie, nil, nil)
	// Andere Profile haben eigene Favoriten, Gäste sehen nur erlaubte Titel.
	other := f.user(db.User{Name: "ben", PassHash: "x"})
	if f.call(other, "GET", "/api/favorites", nil, &got); len(got) != 0 {
		t.Errorf("fremde Favoriten: %v", got)
	}
	g := f.guest(f.ids["heat"])
	if c := f.call(g, "PUT", "/api/favorites/"+f.ids["hp"], nil, nil); c != 404 {
		t.Errorf("Gast merkt gesperrten Titel: %d", c)
	}
	if f.call(tok, "GET", "/api/favorites", nil, &got); len(got) != 1 || got[0] != f.ids["hp"] {
		t.Errorf("nach DELETE: %v", got)
	}
}

func TestHomeRecentRows(t *testing.T) {
	f := newFixture(t,
		fItem{"heat", "/m/Heat (1995).mkv", ""},
		fItem{"bb1", "/m/Breaking Bad/Season 01/Breaking Bad S01E01.mkv", ""},
		fItem{"bb2", "/m/Breaking Bad/Season 01/Breaking Bad S01E02.mkv", ""}, // später hinzugefügt
	)
	tok := f.user(db.User{Name: "anna", PassHash: "x"})
	var rows []homeRow
	f.call(tok, "POST", "/api/home", map[string]any{}, &rows)
	got := map[string][]string{}
	for _, r := range rows {
		got[r.ID] = titles(r.Items)
	}
	if len(rows) != 2 || !slices.Equal(got["recent-movies"], []string{f.ids["heat"]}) || !slices.Equal(got["recent-series"], []string{f.ids["bb2"]}) {
		t.Fatalf("Reihen: %v", got)
	}
	if c := f.call(tok, "GET", "/api/gibtsnicht", nil, nil); c != 404 {
		t.Errorf("unbekannte API-Route: %d", c)
	}
}

func TestItemDetails(t *testing.T) {
	f := newFixture(t,
		fItem{"heat", "/m/Heat (1995).mkv", `{"title":"Heat","tagline":"A Los Angeles Crime Saga","studios":["Warner"],
			"people":[{"name":"Al Pacino","role":"Vincent Hanna","kind":"actor","image":"/pacino.jpg"}]}`},
		fItem{"bb1", "/m/Breaking Bad/Season 01/Breaking Bad S01E01.mkv", ""},
	)
	f.s.DB.Exec(`INSERT INTO streams(item_id, idx, type, codec, language, forced) VALUES(?, 2, 'subtitle', 'subrip', 'ger', 1)`, f.ids["heat"])
	m := f.s.Lib.MetaFor(f.ids["heat"])
	f.s.Lib.Load(t.Context()) // liest die Spuren neu, vergisst aber die Metadaten
	f.s.Lib.SetMeta(f.ids["heat"], m)
	admin := f.user(db.User{Name: "admin", Admin: true, PassHash: "x"})
	anna := f.user(db.User{Name: "anna", PassHash: "x"})
	var d struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Tagline string `json:"tagline"`
		Path    string `json:"path"`
		Video   struct {
			Codec string `json:"codec"`
		} `json:"video"`
		Audio  []streamOut `json:"audio"`
		Subs   []streamOut `json:"subs"`
		People []personOut `json:"people"`
	}
	if c := f.call(admin, "GET", "/api/items/"+f.ids["heat"], nil, &d); c != 200 || d.Title != "Heat" || d.Tagline == "" || d.Path == "" ||
		d.Video.Codec != "h264" || len(d.Audio) != 1 || len(d.Subs) != 1 || !d.Subs[0].Forced || len(d.People) != 1 || d.People[0].Image == "" {
		t.Fatalf("Details: %d %+v", c, d)
	}
	var plain map[string]any
	if f.call(anna, "GET", "/api/items/"+f.ids["heat"], nil, &plain); plain["path"] != nil {
		t.Errorf("Pfad für Nicht-Admin: %v", plain["path"])
	}
	// Listen tragen keine Personen.
	var list []libraryItem
	if f.call(anna, "POST", "/api/library", map[string]any{}, &list); len(list) != 2 || list[0].Meta != nil && len(list[0].Meta.People) > 0 {
		t.Errorf("Personen in der Liste: %+v", list[0].Meta)
	}
	var se seriesOut
	if c := f.call(anna, "GET", "/api/series/"+url.PathEscape("Breaking Bad"), nil, &se); c != 200 || se.Name != "Breaking Bad" {
		t.Errorf("Serie: %d %+v", c, se)
	}
	g := f.guest(f.ids["bb1"])
	if c := f.call(g, "GET", "/api/items/"+f.ids["heat"], nil, nil); c != 404 {
		t.Errorf("Gast sieht gesperrten Titel: %d", c)
	}
	if c := f.call(g, "GET", "/api/series/Gibtsnicht", nil, nil); c != 404 {
		t.Errorf("unbekannte Serie: %d", c)
	}
}

func TestPeopleAndEditMeta(t *testing.T) {
	f := newFixture(t,
		fItem{"heat", "/m/Heat (1995).mkv", `{"title":"Heat","people":[{"name":"Al Pacino","role":"Vincent Hanna","kind":"actor","image":"/nirgends/pacino.jpg"}]}`},
		fItem{"hp", "/m/Harry Potter (2001).mkv", ""},
		fItem{"bb1", "/m/Breaking Bad/Season 01/Breaking Bad S01E01.mkv", ""},
		fItem{"bb2", "/m/Breaking Bad/Season 01/Breaking Bad S01E02.mkv", ""},
	)
	ctx := t.Context()
	f.s.Meta.Save(ctx, "serie:Breaking Bad", &meta.Meta{Title: "Breaking Bad", Overview: "Chemie", Studios: []string{"AMC"},
		People: []meta.Person{{Name: "Al Pacino", Role: "Gast", Kind: "actor"}}})
	admin := f.user(db.User{Name: "admin", Admin: true, PassHash: "x"})
	anna := f.user(db.User{Name: "anna", PassHash: "x"})

	var p personPage
	if c := f.call(anna, "GET", "/api/people/"+url.PathEscape("Al Pacino"), nil, &p); c != 200 || p.Image == "" ||
		!slices.Equal(titles(p.Items), []string{f.ids["bb1"], f.ids["heat"]}) || len(p.Roles) != 2 {
		t.Fatalf("Person: %d %+v", c, p)
	}
	if c := f.call(anna, "GET", "/api/people/Niemand", nil, nil); c != 404 {
		t.Errorf("unbekannte Person: %d", c)
	}
	var se seriesOut
	if f.call(anna, "GET", "/api/series/"+url.PathEscape("Breaking Bad"), nil, &se); se.Overview != "Chemie" || len(se.People) != 1 {
		t.Errorf("Serie mit Show-Daten: %+v", se)
	}

	// Bearbeiten: nur Admins, geänderte Felder werden gesperrt, Personenbilder kommen nicht vom Client.
	edit := map[string]any{"title": "Heat (1995)", "age": 16, "people": []map[string]string{{"name": "Al Pacino", "kind": "actor", "image": "http://evil/x.jpg"}}}
	if c := f.call(anna, "PUT", "/api/items/"+f.ids["heat"]+"/meta", edit, nil); c != 403 {
		t.Errorf("Nicht-Admin bearbeitet: %d", c)
	}
	var d struct {
		Title  string      `json:"title"`
		Meta   meta.Meta   `json:"meta"`
		Locked []string    `json:"locked"`
		People []personOut `json:"people"`
	}
	if c := f.call(admin, "PUT", "/api/items/"+f.ids["heat"]+"/meta", edit, &d); c != 200 || d.Meta.Title != "Heat (1995)" ||
		*d.Meta.Age != 16 || !slices.Equal(d.Locked, []string{"age", "people", "title"}) || f.s.Lib.MetaFor(f.ids["heat"]).People[0].Image != "/nirgends/pacino.jpg" {
		t.Fatalf("PUT meta: %d %+v", c, d)
	}
	if c := f.call(admin, "PUT", "/api/items/"+f.ids["heat"]+"/meta", map[string]any{"age": 7}, nil); c != 400 {
		t.Errorf("FSK 7: %d", c)
	}
	var d2 struct {
		Meta   meta.Meta `json:"meta"`
		Locked []string  `json:"locked"`
	}
	if c := f.call(admin, "PUT", "/api/items/"+f.ids["hp"]+"/meta", map[string]any{"overview": "Zauberei", "locked": []string{}}, &d2); c != 200 ||
		d2.Meta.Overview != "Zauberei" || d2.Meta.Source != "manual" || len(d2.Locked) != 0 {
		t.Errorf("Titel ohne Metadaten: %d %+v", c, d2)
	}
}

func TestCollectionsAndPlaylists(t *testing.T) {
	f := newFixture(t,
		fItem{"m1", "/m/Matrix (1999).mkv", `{"title":"Matrix","year":1999,"collection":{"tmdbId":2344,"name":"Matrix Filmreihe"}}`},
		fItem{"m2", "/m/Matrix Reloaded (2003).mkv", `{"title":"Matrix Reloaded","year":2003,"collection":{"tmdbId":2344,"name":"Matrix Filmreihe"}}`},
		fItem{"hp", "/m/Harry Potter (2001).mkv", `{"title":"Harry Potter","collection":{"tmdbId":1241,"name":"Harry Potter Filmreihe"}}`},
		fItem{"bb1", "/m/Breaking Bad/Season 01/Breaking Bad S01E01.mkv", ""},
	)
	admin := f.user(db.User{Name: "admin", Admin: true, PassHash: "x"})
	anna := f.user(db.User{Name: "anna", PassHash: "x"})

	var cs []collectionOut
	f.call(anna, "GET", "/api/collections", nil, &cs)
	if len(cs) != 1 || !cs[0].Auto || cs[0].ID != "auto-2344" || !slices.Equal(cs[0].Items, []string{f.ids["m1"], f.ids["m2"]}) {
		t.Fatalf("Filmreihen: %+v", cs)
	}
	if c := f.call(anna, "POST", "/api/collections", map[string]any{"name": "Lieblinge"}, nil); c != 403 {
		t.Errorf("Nicht-Admin legt Sammlung an: %d", c)
	}
	var c1 collectionOut
	if c := f.call(admin, "POST", "/api/collections", map[string]any{"name": "Lieblinge", "items": []string{f.ids["hp"], "serie:Breaking Bad"}}, &c1); c != 200 || len(c1.Items) != 2 {
		t.Fatalf("POST: %d %+v", c, c1)
	}
	if c := f.call(admin, "PUT", "/api/collections/"+c1.ID, map[string]any{"remove": []string{f.ids["hp"]}, "add": []string{"gibtsnicht"}}, nil); c != 400 {
		t.Errorf("unbekannter Titel: %d", c)
	}
	f.call(admin, "PUT", "/api/collections/"+c1.ID, map[string]any{"remove": []string{f.ids["hp"]}, "overview": "Nur Serien"}, &c1)
	if c1.Overview != "Nur Serien" || !slices.Equal(c1.Items, []string{"serie:Breaking Bad"}) {
		t.Errorf("PUT: %+v", c1)
	}
	// Gäste sehen nur Sammlungen mit erlaubten Titeln.
	if f.call(f.guest(f.ids["hp"]), "GET", "/api/collections", nil, &cs); len(cs) != 0 {
		t.Errorf("Gast: %+v", cs)
	}
	if c := f.call(admin, "DELETE", "/api/collections/"+c1.ID, nil, nil); c != 204 {
		t.Errorf("DELETE: %d", c)
	}

	// Wiedergabelisten gehören dem Profil; items ersetzt die Reihenfolge.
	var p db.List
	f.call(anna, "POST", "/api/playlists", map[string]any{"name": "Abend", "items": []string{f.ids["m1"], f.ids["m2"]}}, &p)
	f.call(anna, "PUT", "/api/playlists/"+p.ID, map[string]any{"items": []string{f.ids["m2"], f.ids["m1"]}, "add": []string{f.ids["hp"]}}, &p)
	if !slices.Equal(p.Items, []string{f.ids["m2"], f.ids["m1"], f.ids["hp"]}) {
		t.Errorf("Reihenfolge: %v", p.Items)
	}
	var ps []db.List
	if f.call(admin, "GET", "/api/playlists", nil, &ps); len(ps) != 0 {
		t.Errorf("fremde Listen: %+v", ps)
	}
	if c := f.call(admin, "DELETE", "/api/playlists/"+p.ID, nil, nil); c != 404 {
		t.Errorf("fremde Liste gelöscht: %d", c)
	}
	if f.call(anna, "GET", "/api/playlists", nil, &ps); len(ps) != 1 || ps[0].Name != "Abend" {
		t.Errorf("GET: %+v", ps)
	}
}

func TestDashboard(t *testing.T) {
	f := newFixture(t,
		fItem{"heat", "/m/Heat (1995).mkv", ""},
		fItem{"bb1", "/m/Breaking Bad/Season 01/Breaking Bad S01E01.mkv", ""},
		fItem{"bb2", "/m/Breaking Bad/Season 01/Breaking Bad S01E02.mkv", ""},
	)
	admin := f.user(db.User{Name: "admin", Admin: true, PassHash: "x"})
	anna := f.user(db.User{Name: "anna", PassHash: "x", Color: 120})
	if c := f.call(anna, "GET", "/api/admin/overview", nil, nil); c != 403 {
		t.Fatalf("Übersicht für Nicht-Admin: %d", c)
	}

	// Sitzung aus dem progress-Herzschlag.
	f.call(anna, "POST", "/api/items/"+f.ids["heat"]+"/progress", map[string]any{"pos": 120, "paused": true}, nil)
	var ov struct {
		Library struct {
			Movies, Series, Episodes int
		} `json:"library"`
		Sessions []sessionOut `json:"sessions"`
	}
	if f.call(admin, "GET", "/api/admin/overview", nil, &ov); ov.Library.Movies != 1 || ov.Library.Series != 1 || ov.Library.Episodes != 2 ||
		len(ov.Sessions) != 1 || ov.Sessions[0].User != "anna" || !ov.Sessions[0].Paused || ov.Sessions[0].Position != 120 || ov.Sessions[0].UserColor != 120 {
		t.Fatalf("Übersicht: %+v", ov)
	}

	// Aktivitäten: Benutzer anlegen landet im Ringpuffer.
	f.call(admin, "POST", "/api/users", map[string]any{"name": "ben", "password": "geheim"}, nil)
	var acts []Activity
	if f.call(admin, "GET", "/api/activity?limit=5", nil, &acts); len(acts) == 0 || acts[0].Kind != "user" || acts[0].User != "admin" {
		t.Errorf("Aktivitäten: %+v", acts)
	}

	// Geräte: eigenes ist markiert, abmelden sperrt das Token.
	var devs []deviceOut
	f.call(admin, "GET", "/api/devices", nil, &devs)
	var annaDev string
	current := 0
	for _, d := range devs {
		if d.Current {
			current++
		}
		if d.User == "anna" {
			annaDev = d.ID
		}
	}
	if len(devs) != 2 || current != 1 || annaDev == "" {
		t.Fatalf("Geräte: %+v", devs)
	}
	if c := f.call(admin, "DELETE", "/api/devices/"+annaDev, nil, nil); c != 204 {
		t.Errorf("abmelden: %d", c)
	}
	if c := f.call(anna, "GET", "/api/favorites", nil, nil); c != 401 {
		t.Errorf("abgemeldetes Gerät: %d", c)
	}

	// Aufgaben: Liste, Lauf merken, unbekannte ID.
	if c := f.call(admin, "POST", "/api/tasks/db/run", nil, nil); c != 202 {
		t.Fatalf("Aufgabe starten: %d", c)
	}
	var tasks []taskOut
	for range 100 {
		f.call(admin, "GET", "/api/tasks", nil, &tasks)
		if i := slices.IndexFunc(tasks, func(t taskOut) bool { return t.ID == "db" }); i >= 0 && !tasks[i].Running && tasks[i].LastResult != "" {
			if tasks[i].LastResult != "ok" || tasks[i].LastRun == nil {
				t.Errorf("Datenbank pflegen: %+v", tasks[i])
			}
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(tasks) < 4 {
		t.Errorf("Aufgaben: %+v", tasks)
	}
	if c := f.call(admin, "POST", "/api/tasks/gibtsnicht/run", nil, nil); c != 404 {
		t.Errorf("unbekannte Aufgabe: %d", c)
	}

	// Protokoll: slog-Einträge mit Stufe und Attributen.
	f.s.Log = &LogRing{}
	lg := slog.New(f.s.Log.Handler())
	lg.Info("alles gut")
	lg.With("titel", "Heat").Error("kaputt", "code", 7)
	var logs []LogEntry
	if f.call(admin, "GET", "/api/logs?level=error", nil, &logs); len(logs) != 1 || logs[0].Msg != "kaputt" || logs[0].Attrs["titel"] != "Heat" || logs[0].Attrs["code"] != "7" {
		t.Errorf("Protokoll: %+v", logs)
	}
	if f.call(admin, "GET", "/api/logs", nil, &logs); len(logs) != 2 || logs[0].Msg != "kaputt" {
		t.Errorf("Protokoll ab info: %+v", logs)
	}
}

func TestLiveTVAndVPNRoutes(t *testing.T) {
	f := newFixture(t, fItem{"heat", "/m/Heat (1995).mkv", ""})
	f.s.LiveTV = livetv.New(f.s.DB.DB, t.TempDir())
	f.srv.Close()
	f.srv = httptest.NewServer(f.s.Handler())
	t.Cleanup(f.srv.Close)
	admin := f.user(db.User{Name: "admin", Admin: true, PassHash: "x"})
	anna := f.user(db.User{Name: "anna", PassHash: "x"})
	g := f.guest(f.ids["heat"])
	var chs []any
	if c := f.call(anna, "GET", "/api/livetv/channels", nil, &chs); c != 200 || len(chs) != 0 {
		t.Errorf("Kanäle: %d %v", c, chs)
	}
	if c := f.call(g, "GET", "/api/livetv/channels", nil, nil); c != 403 {
		t.Errorf("Gast sieht Live-TV: %d", c)
	}
	if c := f.call(anna, "GET", "/api/livetv", nil, nil); c != 403 {
		t.Errorf("Status für Nicht-Admin: %d", c)
	}
	if c := f.call(admin, "GET", "/api/livetv", nil, nil); c != 200 {
		t.Errorf("Status: %d", c)
	}
	// Medien-Token im Pfad reicht für die HLS-Dateien (unbekannter Kanal → 404, nicht 401).
	tok := auth.MediaToken(f.s.secret(), "anna", time.Hour)
	if c := f.call("", "GET", "/api/m/"+tok+"/livetv/channels/x/index.m3u8", nil, nil); c != 404 {
		t.Errorf("Medien-Token für Live-TV: %d", c)
	}
	var st struct {
		Addrs []any  `json:"addrs"`
		Hint  string `json:"hint"`
	}
	if c := f.call(admin, "GET", "/api/vpn", nil, &st); c != 200 || st.Addrs == nil || st.Hint == "" {
		t.Errorf("VPN: %d %+v", c, st)
	}
	if c := f.call(anna, "GET", "/api/vpn", nil, nil); c != 200 {
		t.Errorf("VPN für Nicht-Admin: %d", c)
	}
	if c := f.call(g, "GET", "/api/vpn", nil, nil); c != 403 {
		t.Errorf("VPN für Gast: %d", c)
	}
	if c := f.call("", "GET", "/api/vpn", nil, nil); c != 401 {
		t.Errorf("VPN ohne Anmeldung: %d", c)
	}
}

func TestFramesAndBackdropFallback(t *testing.T) {
	f := newFixture(t, fItem{"heat", "/m/Heat (1995).mkv", `{"title":"Heat"}`})
	anna := f.user(db.User{Name: "anna", PassHash: "x"})
	var list []libraryItem
	if f.call(anna, "POST", "/api/library", map[string]any{}, &list); len(list) != 1 || list[0].Backdrop != "/api/images/"+f.ids["heat"]+"/backdrop?w=1280" {
		t.Fatalf("Hintergrund-Ersatz: %+v", list)
	}
	it := f.s.Lib.Get(f.ids["heat"]) // 600 s lang
	for in, want := range map[float64]int{12.4: 10, 0: 5, 700: 595, 597.5: 595} {
		if got := frameAt(it, in); got != want {
			t.Errorf("frameAt(%v) = %d, erwartet %d", in, got, want)
		}
	}
	for path, want := range map[string]int{"/api/images/" + f.ids["heat"] + "/frame?t=x": 400, "/api/images/" + f.ids["heat"] + "/frame?t=9999": 400,
		"/api/images/gibtsnicht/frame?t=10": 404} {
		if c := f.call("", "GET", path, nil, nil); c != want {
			t.Errorf("%s: %d, erwartet %d", path, c, want)
		}
	}
}
