package meta

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeKeyless antwortet wie TVmaze, Wikidata und Wikipedia (de/en).
func fakeKeyless(t *testing.T) (*Keyless, *atomic.Int32) {
	var calls atomic.Int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("User-Agent") != userAgent {
			http.Error(w, "User-Agent fehlt", http.StatusForbidden)
			return
		}
		q := r.URL.Query()
		var body string
		switch {
		case strings.HasPrefix(r.URL.Path, "/img/"):
			body = "JPEG"
		case r.URL.Path == "/singlesearch/shows" && q.Get("q") == "Dark" && slices.Contains(q["embed[]"], "episodes"):
			body = `{"name":"Dark","genres":["Drama","Crime"],"premiered":"2017-12-01","summary":"<p><b>Dark</b> &amp; düster</p>",
				"image":{"original":"` + srv.URL + `/img/dark.jpg"},"rating":{"average":null},"externals":{"imdb":"tt5753856"},
				"_embedded":{"episodes":[{"name":"Secrets","season":1,"number":1,"airdate":"2017-12-01","summary":"<p>Ein Kind.</p>",
				"image":{"original":"` + srv.URL + `/img/s1e1.jpg"},"rating":{"average":7.9}},
				{"name":"Lies","season":1,"number":2,"airdate":"2017-12-01","summary":null,"image":null}]}}`
		case r.URL.Path == "/w/api.php" && q.Get("action") == "wbsearchentities":
			body = `{"search":[]}`
			if q.Get("search") == "Nosferatu" {
				body = `{"search":[{"id":"Q1","label":"Nosferatu","description":"Roman (1922)"},
					{"id":"Q151895","label":"Nosferatu – Eine Symphonie des Grauens","description":"Film von F. W. Murnau (1922)","match":{"text":"Nosferatu"}},
					{"id":"Q2","label":"Nosferatu","description":"Film von Werner Herzog (1979)"}]}`
			}
		case r.URL.Path == "/w/api.php" && q.Get("action") == "wbgetentities" && q.Get("ids") == "Q151895":
			body = `{"entities":{"Q151895":{"labels":{"de":{"value":"Nosferatu – Eine Symphonie des Grauens"}},
				"sitelinks":{"dewiki":{"title":"Nosferatu – Eine Symphonie des Grauens"},"enwiki":{"title":"Nosferatu"}},
				"claims":{"P577":[{"mainsnak":{"datavalue":{"value":{"time":"+1922-03-15T00:00:00Z"}}}},{"mainsnak":{"datavalue":{"value":{"time":"+1921-03-04T00:00:00Z"}}}}],
				"P1981":[{"mainsnak":{"datavalue":{"value":{"entity-type":"item","id":"Q20644796"}}}}],
				"P4947":[{"mainsnak":{"datavalue":{"value":"603"}}}],"P345":[{"mainsnak":{"datavalue":{"value":"tt0013442"}}}]}}}}`
		case r.URL.Path == "/de/w/api.php" && q.Get("prop") == "extracts":
			body = `{"query":{"pages":[{"title":"x","extract":"Nosferatu ist ein Stummfilm.\n"}]}}`
		case r.URL.Path == "/en/w/api.php" && q.Get("pilicense") == "any":
			body = `{"query":{"pages":[{"title":"Nosferatu","original":{"source":"` + srv.URL + `/img/nosferatu.jpg?utm_source=en.wikipedia.org"}}]}}`
		default:
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &Keyless{HTTP: srv.Client(), TVmaze: srv.URL, Wikidata: srv.URL + "/w/api.php", Wikipedia: srv.URL + "/%s/w/api.php"}, &calls
}

func TestKeylessFilm(t *testing.T) {
	k, _ := fakeKeyless(t)
	r := resolver(t, NewTMDB(""))
	r.Keyless = k
	m, err := r.Resolve(context.Background(), Query{ID: "n", Path: "/nirgends/Nosferatu (1922).mp4", Title: "Nosferatu", Year: 1922})
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "Nosferatu – Eine Symphonie des Grauens" || m.Year != 1921 || m.Source != "wikidata" || !m.Keyless ||
		m.TMDBID != 603 || m.IMDBID != "tt0013442" || m.Age == nil || *m.Age != 12 || m.Overview != "Nosferatu ist ein Stummfilm." || m.Uncertain {
		t.Errorf("Film: %+v (Age %v)", m, m.Age)
	}
	if b, _ := os.ReadFile(filepath.Join(r.ImgDir(), m.PosterPath)); string(b) != "JPEG" {
		t.Errorf("Plakat nicht gespeichert: %q (%s)", b, m.PosterPath)
	}
}

func TestKeylessSerieEineAnfrageProSerie(t *testing.T) {
	k, calls := fakeKeyless(t)
	r := resolver(t, NewTMDB(""))
	r.Keyless = k
	// Deutscher Titel aus dem Dateinamen bleibt, „Episode 2“ (kein Titel im Namen) wird ersetzt.
	e1, err := r.Resolve(context.Background(), Query{ID: "e1", Path: "/x/Dark S01E01 Geheimnisse.mkv", Series: "Dark", Title: "Geheimnisse", Season: 1, Episode: 1})
	if err != nil {
		t.Fatal(err)
	}
	e2, _ := r.Resolve(context.Background(), Query{ID: "e2", Path: "/x/Dark S01E02.mkv", Series: "Dark", Title: "Episode 2", Season: 1, Episode: 2})
	if e1.Title != "Geheimnisse" || e1.Overview != "Ein Kind." || e1.Rating != 7.9 || e1.Genres[1] != "Krimi" || e1.BackdropPath == "" || e1.PosterPath == "" {
		t.Errorf("Folge 1: %+v", e1)
	}
	if e2.Title != "Lies" || e2.Overview != "Dark & düster" || e2.BackdropPath != "" || e2.PosterPath != e1.PosterPath {
		t.Errorf("Folge 2: %+v", e2)
	}
	if n := calls.Load(); n != 3 { // Serie + Poster + Standbild
		t.Errorf("%d Anfragen, erwartet 3", n)
	}
}

// Nicht gefunden wird gecacht; ein später eingetragener TMDB-Schlüssel löst trotzdem neu auf,
// und zwar exakt über die TMDB-ID, die Wikidata geliefert hat.
func TestKeylessWeichtSchluessel(t *testing.T) {
	k, calls := fakeKeyless(t)
	r := resolver(t, NewTMDB(""))
	r.Keyless = k
	miss := Query{ID: "m", Path: "/x/Gibt es nicht.mkv", Title: "Gibt es nicht"}
	for range 2 {
		if m, _ := r.Resolve(context.Background(), miss); m.Source != "filename" {
			t.Fatalf("Miss: %+v", m)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("Miss nicht gecacht: %d Anfragen", calls.Load())
	}
	q := Query{ID: "n", Path: "/x/Nosferatu (1922).mp4", Title: "Nosferatu", Year: 1922}
	r.Resolve(context.Background(), q)

	r.TMDB, _ = fakeTMDB(t)
	m, err := r.Resolve(context.Background(), q)
	if err != nil || m.Source != "tmdb" || m.Title != "Matrix" { // fakeTMDB kennt unter 603 „Matrix“
		t.Errorf("mit Schlüssel: %+v, %v", m, err)
	}
	if m, _ := r.Resolve(context.Background(), miss); m.Keyless {
		t.Errorf("Miss mit Schlüssel nicht neu versucht: %+v", m)
	}
}

func TestFSK(t *testing.T) {
	for in, want := range map[string]int{"12": 12, "0": 0, "FSK 16": 16, "de:12 / us:PG-13": 12, "US:R / DE:18": 18, "Germany:FSK 6": 6, "FSK12": 12} {
		if a := fsk(in); a == nil || *a != want {
			t.Errorf("fsk(%q) = %v, erwartet %d", in, a, want)
		}
	}
	for _, in := range []string{"", "PG-13", "Rated R", "13", "made:12"} {
		if a := fsk(in); a != nil {
			t.Errorf("fsk(%q) = %d, erwartet unbekannt", in, *a)
		}
	}
}

// fakeRecorded spielt aufgezeichnete Antworten von Wikidata, Wikipedia und TVmaze ab (testdata/keyless, gekürzt).
// Bild-URLs zeigen auf den Testserver, damit kein Test ins Netz geht.
func fakeRecorded(t *testing.T) (*Keyless, *atomic.Int32) {
	var calls atomic.Int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		q := r.URL.Query()
		file := ""
		switch {
		case strings.HasPrefix(r.URL.Path, "/img/"):
			w.Write([]byte("JPEG"))
			return
		case r.URL.Path == "/singlesearch/shows" && slices.Equal(q["embed[]"], []string{"episodes", "cast"}):
			file = "tvmaze_dark.json"
		case r.URL.Path == "/w/api.php" && q.Get("action") == "wbsearchentities":
			file = "wd_search.json"
		case r.URL.Path == "/w/api.php" && q.Get("action") == "wbgetentities" && strings.Contains(q.Get("props"), "claims"):
			file = "wd_entity.json"
		case r.URL.Path == "/w/api.php" && q.Get("action") == "wbgetentities" && q.Get("props") == "labels" && q.Get("languagefallback") == "1" &&
			len(strings.Split(q.Get("ids"), "|")) <= 50:
			file = "wd_labels.json"
		case r.URL.Path == "/w/api.php" && q.Get("prop") == "pageimages" && len(strings.Split(q.Get("titles"), "|")) <= 50:
			file = "wd_images.json"
		case r.URL.Path == "/de/w/api.php" && q.Get("prop") == "extracts":
			file = "wp_de.json"
		case r.URL.Path == "/en/w/api.php" && q.Get("prop") == "pageimages":
			file = "wp_en.json"
		default:
			http.NotFound(w, r)
			return
		}
		b, err := os.ReadFile(filepath.Join("testdata", "keyless", file))
		if err != nil {
			t.Error(err)
		}
		for _, host := range []string{"https://upload.wikimedia.org", "https://thumb.wikimedia.org", "https://static.tvmaze.com"} {
			b = []byte(strings.ReplaceAll(string(b), host, srv.URL+"/img"))
		}
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return &Keyless{HTTP: srv.Client(), TVmaze: srv.URL, Wikidata: srv.URL + "/w/api.php", Wikipedia: srv.URL + "/%s/w/api.php"}, &calls
}

func TestKeylessFilmVoll(t *testing.T) {
	k, calls := fakeRecorded(t)
	r := resolver(t, NewTMDB(""))
	r.Keyless = k
	m, err := r.Resolve(context.Background(), Query{ID: "n", Path: "/nirgends/Nosferatu (1922).mkv", Title: "Nosferatu", Year: 1922})
	if err != nil {
		t.Fatal(err)
	}
	if n := calls.Load(); n != 7 { // Suche, Film, Namen, Personenbilder, de-Text, en-Plakat, Plakat laden
		t.Errorf("%d Anfragen, erwartet 7", n)
	}
	if !slices.Contains(m.Genres, "Horrorfilm") || !slices.Equal(m.Studios, []string{"Prana Film"}) || !slices.Equal(m.Countries, []string{"Deutschland"}) ||
		m.TMDBID != 653 || !strings.HasPrefix(m.Overview, "Nosferatu – Eine Symphonie des Grauens ist ein deutscher Spielfilm") {
		t.Errorf("Film: %+v", m)
	}
	kinds := map[string]string{}
	var schreck Person
	for _, p := range m.People {
		kinds[p.Name] = p.Kind
		if p.Name == "Max Schreck" {
			schreck = p
		}
	}
	if kinds["Friedrich Wilhelm Murnau"] != "director" || kinds["Henrik Galeen"] != "writer" || kinds["Hans Erdmann"] != "composer" ||
		kinds["Albin Grau"] != "producer" || len(m.People) > 4+castMax {
		t.Errorf("Stab: %v", kinds)
	}
	if schreck.Role != "Graf Orlok" || !strings.Contains(schreck.Image, "/img/wikipedia/commons/") || strings.Contains(schreck.Image, "utm_") {
		t.Errorf("Besetzung: %+v", schreck)
	}
}

func TestKeylessSerieVoll(t *testing.T) {
	k, calls := fakeRecorded(t)
	r := resolver(t, NewTMDB(""))
	r.Keyless = k
	if _, err := r.Resolve(context.Background(), Query{ID: "e1", Path: "/x/Dark S01E01.mkv", Series: "Dark", Title: "Episode 1", Season: 1, Episode: 1}); err != nil {
		t.Fatal(err)
	}
	show := r.Show(context.Background(), "Dark")
	if show == nil || !slices.Equal(show.Studios, []string{"Netflix"}) || show.EndYear != 2020 || show.Status != "Ended" ||
		len(show.People) != 3 || show.People[0].Role != "Jonas Kahnwald" || !strings.Contains(show.People[0].Image, "/img/") {
		t.Fatalf("Serie: %+v", show)
	}
	if n := calls.Load(); n > 3 { // eine TVmaze-Anfrage, dazu Poster und Standbild
		t.Errorf("%d Anfragen", n)
	}
}

func TestParagraphs(t *testing.T) {
	if got := paragraphs("Eins.\n\nZwei.\nDrei.\n\n\nVier.", 3); got != "Eins.\n\nZwei.\n\nDrei." {
		t.Errorf("%q", got)
	}
}
