package meta

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Bavarianator/flimmer/internal/db"
)

func TestParseNFO(t *testing.T) {
	m, err := ParseNFO("testdata/movie.nfo")
	if err != nil {
		t.Fatal(err)
	}
	want := Meta{Title: "Blade Runner 2049", OriginalTitle: "Blade Runner 2049", Year: 2017,
		Overview: "Dreißig Jahre nach den Ereignissen des ersten Films …", TMDBID: 335984, IMDBID: "tt1856101",
		Rating: 7.5, Genres: []string{"Science Fiction", "Drama"}, Source: "nfo",
		posterURL: "https://example.org/poster.jpg", backdropURL: "https://example.org/fanart.jpg"}
	if !reflect.DeepEqual(*m, want) {
		t.Errorf("\n got %+v\nwant %+v", *m, want)
	}

	ep, _ := ParseNFO("testdata/episode.nfo")
	if ep.Title != "Geheimnisse" || ep.Series != "Dark" || ep.Season != 1 || ep.Episode != 1 || ep.Year != 2017 {
		t.Errorf("Episode: %+v", ep)
	}

	u, _ := ParseNFO("testdata/url.nfo")
	if u.Title != "" || u.TMDBID != 603 {
		t.Errorf("URL-NFO: %+v", u)
	}
}

// fakeTMDB antwortet wie api.themoviedb.org; deutsche Episodenbeschreibung fehlt absichtlich (→ en-US).
func fakeTMDB(t *testing.T) (*TMDB, *atomic.Int32) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if strings.HasPrefix(r.URL.Path, "/img/") {
			w.Write([]byte("JPEG"))
			return
		}
		if r.URL.Query().Get("api_key") != "k" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		de := r.URL.Query().Get("language") == "de-DE"
		q := r.URL.Query().Get("query")
		var body string
		switch r.URL.Path {
		case "/search/movie":
			body = `{"results":[]}`
			switch {
			case q == "Matrix" && r.URL.Query().Get("year") == "": // Jahr aus dem Dateinamen falsch → zweiter Versuch ohne
				body = `{"results":[{"id":603,"title":"Matrix","original_title":"The Matrix","release_date":"1999-03-30"}]}`
			case q == "The Matrix" && r.URL.Query().Get("year") == "1999":
				body = `{"results":[{"id":603,"title":"Matrix","original_title":"The Matrix","release_date":"1999-03-30","poster_path":"/p.jpg"},{"id":604,"title":"Matrix Reloaded","release_date":"2003-05-15"}]}`
			}
		case "/movie/603":
			body = `{"id":603,"title":"Matrix","original_title":"The Matrix","overview":"Neo …","release_date":"1999-03-30",
				"poster_path":"/p.jpg","backdrop_path":"/b.jpg","vote_average":8.2,"imdb_id":"tt0133093","genres":[{"name":"Action"}]}`
		case "/search/tv":
			body = `{"results":[{"id":70523,"name":"Dark","first_air_date":"2017-12-01"}]}`
		case "/tv/70523":
			body = `{"id":70523,"name":"Dark","overview":"Serie","first_air_date":"2017-12-01","poster_path":"/dark.jpg","backdrop_path":"/darkb.jpg","genres":[{"name":"Drama"}]}`
		case "/tv/70523/season/1/episode/2":
			if de {
				body = `{"name":"Lügen","overview":"","air_date":"2017-12-01","still_path":"/s.jpg","season_number":1,"episode_number":2,"vote_average":7}`
			} else {
				body = `{"name":"Lies","overview":"Englischer Text","air_date":"2017-12-01","still_path":"/s.jpg","season_number":1,"episode_number":2}`
			}
		default:
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &TMDB{Key: "k", BaseURL: srv.URL, ImageURL: srv.URL + "/img", HTTP: srv.Client()}, &calls
}

func TestResolveMovieTMDB(t *testing.T) {
	tm, calls := fakeTMDB(t)
	r := resolver(t, tm)
	q := Query{ID: "abc", Path: filepath.Join(t.TempDir(), "Matrix (2001).mkv"), Title: "Matrix", Year: 2001}

	m, err := r.Resolve(context.Background(), q)
	if err != nil || m == nil {
		t.Fatalf("Resolve: %v, %v", m, err)
	}
	if m.Title != "Matrix" || m.OriginalTitle != "The Matrix" || m.Year != 1999 || m.TMDBID != 603 || m.Genres[0] != "Action" {
		t.Errorf("Meta: %+v", m)
	}
	if !m.Uncertain {
		t.Error("Jahr 2001 passt nicht zu 1999: Treffer muss unsicher sein")
	}
	if m.PosterPath != "w500-p.jpg" || m.BackdropPath != "w1280-b.jpg" {
		t.Errorf("Bilder: %q %q", m.PosterPath, m.BackdropPath)
	}
	if b, _ := os.ReadFile(filepath.Join(r.ImgDir(), m.PosterPath)); string(b) != "JPEG" {
		t.Errorf("Poster nicht gespeichert: %q", b)
	}

	before := calls.Load()
	m2, err := r.Resolve(context.Background(), q)
	if err != nil || calls.Load() != before {
		t.Errorf("zweiter Aufruf muss aus dem Cache kommen (%d → %d Anfragen, %v)", before, calls.Load(), err)
	}
	if m2.Title != m.Title || m2.PosterPath != m.PosterPath {
		t.Errorf("Cache: %+v", m2)
	}
}

func TestResolveEpisodeFallbackEnglisch(t *testing.T) {
	tm, _ := fakeTMDB(t)
	r := resolver(t, tm)
	m, err := r.Resolve(context.Background(), Query{ID: "ep", Path: "/nirgends/Dark/Staffel 1/S01E02.mkv", Series: "Dark", Season: 1, Episode: 2})
	if err != nil || m == nil {
		t.Fatalf("Resolve: %v, %v", m, err)
	}
	want := Meta{Title: "Lügen", Series: "Dark", Year: 2017, Season: 1, Episode: 2, Overview: "Englischer Text",
		PosterPath: "w500-dark.jpg", BackdropPath: "w1280-s.jpg", TMDBID: 70523, Rating: 7, Genres: []string{"Drama"}, Source: "tmdb"}
	m.posterURL, m.backdropURL = "", ""
	if !reflect.DeepEqual(*m, want) {
		t.Errorf("\n got %+v\nwant %+v", *m, want)
	}
}

func TestResolveNFOGewinnt(t *testing.T) {
	tm, calls := fakeTMDB(t)
	dir := t.TempDir()
	video := filepath.Join(dir, "Blade Runner 2049 (2017).mkv")
	nfo, _ := os.ReadFile("testdata/movie.nfo")
	os.WriteFile(strings.TrimSuffix(video, ".mkv")+".nfo", []byte(strings.NewReplacer("https://example.org/poster.jpg", "", "https://example.org/fanart.jpg", "").Replace(string(nfo))), 0o644)
	os.WriteFile(filepath.Join(dir, "poster.jpg"), []byte("LOKAL"), 0o644)

	r := resolver(t, tm)
	m, err := r.Resolve(context.Background(), Query{ID: "br", Path: video, Title: "Blade Runner 2049", Year: 2017})
	if err != nil {
		t.Fatal(err)
	}
	if m.Source != "nfo" || m.Title != "Blade Runner 2049" || calls.Load() != 0 {
		t.Errorf("NFO muss ohne TMDB gewinnen: %+v, %d Anfragen", m, calls.Load())
	}
	if b, _ := os.ReadFile(filepath.Join(r.ImgDir(), m.PosterPath)); string(b) != "LOKAL" {
		t.Errorf("lokales poster.jpg nicht übernommen: %q (%s)", b, m.PosterPath)
	}
}

func TestResolveURLNFONutztTMDBID(t *testing.T) {
	tm, _ := fakeTMDB(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "movie.nfo"), []byte("https://www.themoviedb.org/movie/603\n"), 0o644)
	r := resolver(t, tm)
	// Titel ist absichtlich falsch: gesucht wird nicht, die ID aus der NFO zählt.
	m, err := r.Resolve(context.Background(), Query{ID: "x", Path: filepath.Join(dir, "film.mkv"), Title: "Unsinn"})
	if err != nil || m == nil || m.TMDBID != 603 {
		t.Fatalf("Resolve: %+v, %v", m, err)
	}
}

func TestOhneSchluessel(t *testing.T) {
	r := resolver(t, NewTMDB(""))
	m, err := r.Resolve(context.Background(), Query{ID: "x", Path: "/nirgends/film.mkv", Title: "Matrix"})
	if err != nil || m.Source != "filename" || m.Title != "Matrix" {
		t.Errorf("ohne Schlüssel: %+v, %v", m, err)
	}
	var n int
	r.DB.QueryRow(`SELECT count(*) FROM meta`).Scan(&n)
	if n != 0 {
		t.Error("ohne Schlüssel darf nichts gecacht werden")
	}
}

func TestNichtGefundenWirdGecacht(t *testing.T) {
	tm, calls := fakeTMDB(t)
	r := resolver(t, tm)
	q := Query{ID: "nf", Path: "/nirgends/x.mkv", Title: "Gibt es nicht"}
	for range 2 {
		if m, err := r.Resolve(context.Background(), q); err != nil || m.Source != "filename" || m.Title != "Gibt es nicht" {
			t.Fatalf("%+v, %v", m, err)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("%d Anfragen, erwartet 1 (zweites Mal aus dem Cache)", calls.Load())
	}
}

func TestSicherBeiPassendemTitelUndJahr(t *testing.T) {
	tm, _ := fakeTMDB(t)
	m, err := tm.SearchMovie(context.Background(), "The Matrix", 1999)
	if err != nil || m == nil || m.Uncertain || m.TMDBID != 603 {
		t.Fatalf("%+v, %v", m, err)
	}
}

// Manuelle Korrektur schlägt sogar eine NFO.
func TestIdentifySchlaegtNFO(t *testing.T) {
	tm, _ := fakeTMDB(t)
	dir := t.TempDir()
	video := filepath.Join(dir, "film.mkv")
	os.WriteFile(filepath.Join(dir, "film.nfo"), []byte("<movie><title>Falscher Film</title></movie>"), 0o644)
	r := resolver(t, tm)
	q := Query{ID: "f", Path: video, Title: "film"}

	if m, _ := r.Resolve(context.Background(), q); m.Title != "Falscher Film" {
		t.Fatalf("vorher: %+v", m)
	}
	m, err := r.Identify(context.Background(), q, 603)
	if err != nil || m.Source != "manual" || m.Title != "Matrix" {
		t.Fatalf("Identify: %+v, %v", m, err)
	}
	if m, _ := r.Resolve(context.Background(), q); m.Source != "manual" || m.Title != "Matrix" {
		t.Errorf("nachher: %+v", m)
	}
}

func TestHandler(t *testing.T) {
	tm, _ := fakeTMDB(t)
	r := resolver(t, tm)
	lookup := func(id string) (Query, bool) {
		return Query{ID: id, Path: "/nirgends/x.mkv", Title: "The Matrix"}, id == "x"
	}
	var got *Meta
	mux := http.NewServeMux()
	mux.Handle("GET /api/items/{id}/search", r.SearchHandler(lookup))
	mux.Handle("POST /api/items/{id}/identify", r.IdentifyHandler(lookup, func(id string, m *Meta) { got = m }))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/items/x/search?year=1999", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"tmdbId":604`) || !strings.Contains(rec.Body.String(), "/w185/p.jpg") {
		t.Errorf("search: %d %s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/items/x/identify", strings.NewReader(`{"tmdbId":603}`)))
	if rec.Code != 200 || got == nil || got.Source != "manual" {
		t.Errorf("identify: %d %s", rec.Code, rec.Body)
	}

	for _, req := range []*http.Request{
		httptest.NewRequest("POST", "/api/items/x/identify", strings.NewReader(`{}`)),
		httptest.NewRequest("GET", "/api/items/unbekannt/search", nil),
	} {
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code < 400 {
			t.Errorf("%s %s: %d, erwartet Fehler", req.Method, req.URL, rec.Code)
		}
	}
}

func resolver(t *testing.T, tm *TMDB) *Resolver {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "flimmer.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return &Resolver{DB: d.DB, CacheDir: t.TempDir(), TMDB: tm}
}

// Alter Datei-Cache wird einmal übernommen, danach umbenannt; manuelle Korrekturen bleiben manuell.
func TestImportJSON(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "flimmer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	cache := t.TempDir()
	os.MkdirAll(filepath.Join(cache, "meta"), 0o755)
	for id, m := range map[string]Meta{
		"a": {Title: "Matrix", TMDBID: 603, Source: "manual"},
		"b": {Title: "Unsicher", Source: "tmdb", Uncertain: true},
	} {
		b, _ := json.Marshal(m)
		os.WriteFile(filepath.Join(cache, "meta", id+".json"), b, 0o644)
	}
	os.WriteFile(filepath.Join(cache, "meta", "kaputt.json"), []byte("{"), 0o644)

	r, err := New(d.DB, cache, "")
	if err != nil {
		t.Fatal(err)
	}
	m, _ := r.Resolve(context.Background(), Query{ID: "a", Path: "/nirgends/x.mkv", Title: "x"})
	if m.Source != "manual" || m.Title != "Matrix" {
		t.Errorf("Import: %+v", m)
	}
	var uncertain int
	d.QueryRow(`SELECT uncertain FROM meta WHERE item_id = 'b'`).Scan(&uncertain)
	if uncertain != 1 {
		t.Error("uncertain nicht übernommen")
	}
	if _, err := os.Stat(filepath.Join(cache, "meta")); err == nil {
		t.Error("alter Cache-Ordner nicht umbenannt → Import liefe bei jedem Start")
	}
	if _, err := New(d.DB, cache, ""); err != nil { // zweiter Start: nichts mehr zu tun
		t.Fatal(err)
	}
}
