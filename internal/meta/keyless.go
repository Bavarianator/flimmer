package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// userAgent: Wikimedia lehnt Anfragen ohne sprechenden User-Agent ab, TVmaze bittet darum.
const userAgent = "Flimmer (https://github.com/Bavarianator/flimmer)"

// Keyless holt Metadaten ohne API-Schlüssel, solange kein TMDB-Schlüssel eingetragen ist:
// Serien von TVmaze (Poster, Standbilder, Texte meist englisch), Filme von Wikidata (Titel, Jahr, FSK,
// TMDB-/IMDb-ID) mit Beschreibung aus der deutschen und Plakat aus der englischen Wikipedia.
// ponytail: Wikipedia-Plakate sind klein (~260 px, Fair Use) – für Karten reicht das, ein TMDB-Schlüssel
// ersetzt sie später automatisch.
type Keyless struct {
	HTTP      *http.Client
	TVmaze    string // https://api.tvmaze.com
	Wikidata  string // https://www.wikidata.org/w/api.php
	Wikipedia string // https://%s.wikipedia.org/w/api.php, %s = Sprache

	tv, wiki pace

	// Letzte Serie: Die Bibliothek löst Folgen einer Serie nacheinander auf, eine Anfrage pro Serie genügt.
	mu       sync.Mutex
	lastName string
	lastShow *tvmazeShow // nil = nicht gefunden
}

func NewKeyless() *Keyless {
	return &Keyless{HTTP: &http.Client{Timeout: 15 * time.Second}, TVmaze: "https://api.tvmaze.com",
		Wikidata: "https://www.wikidata.org/w/api.php", Wikipedia: "https://%s.wikipedia.org/w/api.php",
		tv: pace{gap: 500 * time.Millisecond}, wiki: pace{gap: 200 * time.Millisecond}} // TVmaze: 20 Anfragen/10 s
}

// pace hält Abstand zwischen Anfragen an einen Dienst.
type pace struct {
	gap  time.Duration
	once sync.Once
	tick <-chan time.Time
}

func (p *pace) wait(ctx context.Context) error {
	p.once.Do(func() { p.tick = time.NewTicker(max(p.gap, time.Millisecond)).C })
	select {
	case <-p.tick:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// get holt JSON; 404 ergibt found=false ohne Fehler.
func (k *Keyless) get(ctx context.Context, p *pace, u string, out any) (found bool, err error) {
	if err := p.wait(ctx); err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := k.HTTP.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out)
	case http.StatusNotFound:
		return false, nil
	}
	return false, fmt.Errorf("%s: %s", req.URL.Host, resp.Status)
}

// ---------- Serien: TVmaze ----------

type tvmazeImage struct {
	Original string `json:"original"`
}

type tvmazeRating struct {
	Average float64 `json:"average"`
}

type tvmazeShow struct {
	Name       string       `json:"name"`
	Genres     []string     `json:"genres"`
	Premiered  string       `json:"premiered"`
	Ended      string       `json:"ended"`
	Status     string       `json:"status"` // Running, Ended …
	Summary    string       `json:"summary"`
	Image      *tvmazeImage `json:"image"`
	Rating     tvmazeRating `json:"rating"`
	Network    *named       `json:"network"`    // Fernsehsender
	WebChannel *named       `json:"webChannel"` // Streamingdienst, z. B. Netflix
	Externals  struct {
		IMDB string `json:"imdb"`
	} `json:"externals"`
	Embedded struct {
		Cast []struct {
			Person struct {
				Name  string       `json:"name"`
				Image *tvmazeImage `json:"image"`
			} `json:"person"`
			Character struct {
				Name string `json:"name"`
			} `json:"character"`
		} `json:"cast"`
		Episodes []struct {
			Name    string       `json:"name"`
			Season  int          `json:"season"`
			Number  int          `json:"number"`
			Airdate string       `json:"airdate"`
			Summary string       `json:"summary"`
			Image   *tvmazeImage `json:"image"`
			Rating  tvmazeRating `json:"rating"`
		} `json:"episodes"`
	} `json:"_embedded"`
}

// TVmaze-Genres, die TMDB auf Deutsch anders nennt; der Rest bleibt, wie er ist.
var genresDE = map[string]string{"Adventure": "Abenteuer", "Comedy": "Komödie", "Crime": "Krimi", "Family": "Familie",
	"Children": "Kinder", "History": "Historie", "Romance": "Liebesfilm", "Science-Fiction": "Science Fiction",
	"War": "Kriegsfilm", "Music": "Musik", "Documentary": "Dokumentarfilm", "Nature": "Natur", "Sports": "Sport"}

func (k *Keyless) show(ctx context.Context, name string) (*tvmazeShow, error) {
	k.mu.Lock()
	if k.lastName == name {
		defer k.mu.Unlock()
		return k.lastShow, nil
	}
	k.mu.Unlock()
	var s tvmazeShow
	found, err := k.get(ctx, &k.tv, k.TVmaze+"/singlesearch/shows?"+url.Values{"q": {name}, "embed[]": {"episodes", "cast"}}.Encode(), &s)
	if err != nil {
		return nil, err
	}
	var show *tvmazeShow
	if found {
		show = &s
	}
	k.mu.Lock()
	k.lastName, k.lastShow = name, show
	k.mu.Unlock()
	return show, nil
}

// Episode sucht die Serie bei TVmaze; fehlt die Folge dort, gibt es wenigstens Serien-Poster und -Text.
func (k *Keyless) Episode(ctx context.Context, q Query) (*Meta, error) {
	show, err := k.show(ctx, q.Series)
	if show == nil || err != nil {
		return nil, err
	}
	m := &Meta{Title: q.Title, Series: show.Name, Year: year0(show.Premiered), Season: q.Season, Episode: q.Episode,
		Overview: plain(show.Summary), Rating: show.Rating.Average, IMDBID: show.Externals.IMDB,
		Uncertain: norm(show.Name) != norm(q.Series), Source: "tvmaze", Keyless: true}
	for _, g := range show.Genres {
		m.Genres = append(m.Genres, cmp(genresDE[g], g))
	}
	if show.Image != nil {
		m.posterURL = show.Image.Original
	}
	for _, e := range show.Embedded.Episodes {
		if e.Season != q.Season || e.Number != q.Episode {
			continue
		}
		// Der Titel aus dem Dateinamen ist meist deutsch, TVmaze fast immer englisch: nur ersetzen, wenn er fehlt.
		if q.Title == "" || q.Title == "Episode "+strconv.Itoa(q.Episode) {
			m.Title = cmp(e.Name, m.Title)
		}
		m.Year = cmp0(year0(e.Airdate), m.Year)
		m.Overview = cmp(plain(e.Summary), m.Overview)
		if e.Rating.Average > 0 {
			m.Rating = e.Rating.Average
		}
		if e.Image != nil {
			m.backdropURL = e.Image.Original
		}
	}
	return m, nil
}

// Show liefert die Serie selbst (Beschreibung, Sender, Besetzung mit Fotos) für "serie:<Name>"; nil = unbekannt.
// Kostet keine eigene Anfrage, wenn eben eine Folge derselben Serie aufgelöst wurde.
func (k *Keyless) Show(ctx context.Context, name string) (*Meta, error) {
	show, err := k.show(ctx, name)
	if show == nil || err != nil {
		return nil, err
	}
	m := &Meta{Title: show.Name, Year: year0(show.Premiered), Overview: plain(show.Summary), Rating: show.Rating.Average,
		IMDBID: show.Externals.IMDB, Status: show.Status, Source: "tvmaze", Keyless: true}
	if show.Status == "Ended" {
		m.EndYear = year0(show.Ended)
	}
	for _, g := range show.Genres {
		m.Genres = append(m.Genres, cmp(genresDE[g], g))
	}
	for _, n := range []*named{show.Network, show.WebChannel} {
		if n != nil && n.Name != "" {
			m.Studios = append(m.Studios, n.Name)
		}
	}
	for _, c := range show.Embedded.Cast[:min(len(show.Embedded.Cast), castMax)] {
		p := Person{Name: c.Person.Name, Role: c.Character.Name, Kind: "actor"}
		if c.Person.Image != nil {
			p.Image = c.Person.Image.Original
		}
		m.People = append(m.People, p)
	}
	if show.Image != nil {
		m.posterURL = show.Image.Original
	}
	return m, nil
}

var reTag = regexp.MustCompile(`<[^>]*>`)

// plain macht aus TVmaze-HTML („<p><b>Dark</b> …</p>“) Text.
func plain(s string) string {
	return strings.TrimSpace(html.UnescapeString(reTag.ReplaceAllString(s, "")))
}

// ---------- Filme: Wikidata + Wikipedia ----------

// FSK-Freigaben als Wikidata-Objekte (Eigenschaft P1981).
var fskItems = map[string]int{"Q20644794": 0, "Q20644795": 6, "Q20644796": 12, "Q20644797": 16, "Q20644798": 18}

var reFilm = regexp.MustCompile(`(?i)film|movie`)

type wdEntity struct {
	Labels map[string]struct {
		Value string `json:"value"`
	} `json:"labels"`
	Sitelinks map[string]struct {
		Title string `json:"title"`
	} `json:"sitelinks"`
	Claims map[string][]struct {
		Mainsnak   wdSnak              `json:"mainsnak"`
		Qualifiers map[string][]wdSnak `json:"qualifiers"`
	} `json:"claims"`
}

type wdSnak struct {
	Datavalue struct {
		Value json.RawMessage `json:"value"`
	} `json:"datavalue"`
}

type wdRef struct {
	ID string `json:"id"`
}

// Stab aus Wikidata: Eigenschaft → Art und Aufgabe der Person.
var wdCrew = []struct{ prop, kind, role string }{
	{"P57", "director", "Regie"}, {"P58", "writer", "Drehbuch"}, {"P86", "composer", "Musik"}, {"P162", "producer", "Produktion"}}

// wdMax: Wikidata nimmt höchstens 50 IDs bzw. Titel je Anfrage.
const wdMax = 50

// enrich ergänzt Genres (P136), Stab, Besetzung (P161, Rolle aus P453/P4633), Studio (P272) und Land (P495).
// Zwei kleine Anfragen: deutsche Namen aller Begriffe und Personen, dann die Personenbilder (P18) als Vorschau.
func (k *Keyless) enrich(ctx context.Context, e *wdEntity, m *Meta) error {
	type ref struct{ id, kind, role, roleID string }
	var people []ref
	for _, c := range wdCrew {
		for _, r := range claims[wdRef](e, c.prop) {
			people = append(people, ref{id: r.ID, kind: c.kind, role: c.role})
		}
	}
	for _, c := range e.Claims["P161"][:min(len(e.Claims["P161"]), castMax)] {
		var r wdRef
		if json.Unmarshal(c.Mainsnak.Datavalue.Value, &r) != nil || r.ID == "" {
			continue
		}
		p := ref{id: r.ID, kind: "actor"}
		for _, q := range c.Qualifiers["P453"] { // Rolle als eigenes Objekt
			var role wdRef
			if json.Unmarshal(q.Datavalue.Value, &role) == nil {
				p.roleID = role.ID
			}
		}
		for _, q := range c.Qualifiers["P4633"] { // Rolle als Text
			json.Unmarshal(q.Datavalue.Value, &p.role)
		}
		people = append(people, p)
	}
	var ids []string
	add := func(id string) {
		if id != "" && !slices.Contains(ids, id) && len(ids) < wdMax {
			ids = append(ids, id)
		}
	}
	for _, p := range people {
		add(p.id)
	}
	for _, prop := range []string{"P136", "P272", "P495"} {
		for _, r := range claims[wdRef](e, prop) {
			add(r.ID)
		}
	}
	for _, p := range people {
		add(p.roleID)
	}
	if len(ids) == 0 {
		return nil
	}
	var ents struct {
		Entities map[string]wdEntity `json:"entities"`
	}
	q := url.Values{"action": {"wbgetentities"}, "ids": {strings.Join(ids, "|")}, "props": {"labels"}, "languages": {"de"},
		"languagefallback": {"1"}, "format": {"json"}}
	if _, err := k.get(ctx, &k.wiki, k.Wikidata+"?"+q.Encode(), &ents); err != nil {
		return err
	}
	label := func(id string) string {
		e := ents.Entities[id]
		return e.Labels["de"].Value
	}
	labels := func(prop string) (out []string) {
		for _, r := range claims[wdRef](e, prop) {
			if l := label(r.ID); l != "" {
				out = append(out, l)
			}
		}
		return out
	}
	m.Genres, m.Studios, m.Countries = labels("P136"), labels("P272"), labels("P495")
	var who []string
	for _, p := range people {
		if name := label(p.id); name != "" {
			m.People = append(m.People, Person{Name: name, Role: cmp(label(p.roleID), p.role), Kind: p.kind, wdID: p.id})
			if !slices.Contains(who, p.id) {
				who = append(who, p.id)
			}
		}
	}
	// Personenbilder: PageImages auf Wikidata liefert das Bild aus P18 als fertige Commons-Vorschau.
	var res struct {
		Query struct {
			Pages []struct {
				Title     string `json:"title"`
				Thumbnail struct {
					Source string `json:"source"`
				} `json:"thumbnail"`
			} `json:"pages"`
		} `json:"query"`
	}
	q = url.Values{"action": {"query"}, "prop": {"pageimages"}, "piprop": {"thumbnail"}, "pithumbsize": {"400"},
		"titles": {strings.Join(who[:min(len(who), wdMax)], "|")}, "format": {"json"}, "formatversion": {"2"}}
	if _, err := k.get(ctx, &k.wiki, k.Wikidata+"?"+q.Encode(), &res); err != nil {
		return err
	}
	img := map[string]string{}
	for _, p := range res.Query.Pages {
		img[p.Title], _, _ = strings.Cut(p.Thumbnail.Source, "?")
	}
	for i := range m.People {
		m.People[i].Image = img[m.People[i].wdID]
	}
	return nil
}

// claims liefert die Werte einer Wikidata-Eigenschaft, dekodiert als T.
func claims[T any](e *wdEntity, prop string) []T {
	var out []T
	for _, c := range e.Claims[prop] {
		var v T
		if json.Unmarshal(c.Mainsnak.Datavalue.Value, &v) == nil {
			out = append(out, v)
		}
	}
	return out
}

// Movie sucht den Film bei Wikidata. Hinweise: „Film“ in der Beschreibung (2), das Jahr ±1 darin (2),
// gleicher Titel (1). Ein Kandidat braucht mindestens 3 Punkte; sicher ist er nur mit allen.
func (k *Keyless) Movie(ctx context.Context, title string, year int) (*Meta, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute) // alle Anfragen zu einem Film zusammen
	defer cancel()
	var res struct {
		Search []struct {
			ID          string `json:"id"`
			Label       string `json:"label"`
			Description string `json:"description"`
			Match       struct {
				Text string `json:"text"`
			} `json:"match"`
		} `json:"search"`
	}
	q := url.Values{"action": {"wbsearchentities"}, "search": {title}, "language": {"de"}, "uselang": {"de"},
		"type": {"item"}, "limit": {"10"}, "format": {"json"}}
	if _, err := k.get(ctx, &k.wiki, k.Wikidata+"?"+q.Encode(), &res); err != nil {
		return nil, err
	}
	id, best := "", 0
	for _, c := range res.Search {
		score := 0
		if reFilm.MatchString(c.Description) {
			score += 2
		}
		if year > 0 && (strings.Contains(c.Description, strconv.Itoa(year)) ||
			strings.Contains(c.Description, strconv.Itoa(year-1)) || strings.Contains(c.Description, strconv.Itoa(year+1))) {
			score += 2
		}
		if norm(c.Label) == norm(title) || norm(c.Match.Text) == norm(title) {
			score++
		}
		if score > best {
			id, best = c.ID, score
		}
	}
	if best < 3 {
		return nil, nil
	}
	full := 3
	if year > 0 {
		full = 5
	}

	var ents struct {
		Entities map[string]wdEntity `json:"entities"`
	}
	q = url.Values{"action": {"wbgetentities"}, "ids": {id}, "props": {"labels|sitelinks|claims"}, "languages": {"de"},
		"sitefilter": {"dewiki|enwiki"}, "format": {"json"}}
	if _, err := k.get(ctx, &k.wiki, k.Wikidata+"?"+q.Encode(), &ents); err != nil {
		return nil, err
	}
	e, ok := ents.Entities[id]
	if !ok {
		return nil, nil
	}
	m := &Meta{Title: cmp(e.Labels["de"].Value, title), Uncertain: best < full, Source: "wikidata", Keyless: true}
	first := 0 // Erstaufführung: das früheste Veröffentlichungsdatum
	for _, t := range claims[struct {
		Time string `json:"time"`
	}](&e, "P577") {
		if y := year0(strings.TrimPrefix(t.Time, "+")); y > 0 && (first == 0 || y < first) {
			first = y
		}
	}
	m.Year = cmp0(first, year)
	for _, a := range claims[struct {
		ID string `json:"id"`
	}](&e, "P1981") {
		if n, ok := fskItems[a.ID]; ok {
			m.Age = &n
			break
		}
	}
	if ids := claims[string](&e, "P4947"); len(ids) > 0 {
		m.TMDBID, _ = strconv.Atoi(ids[0])
	}
	if ids := claims[string](&e, "P345"); len(ids) > 0 {
		m.IMDBID = ids[0]
	}
	if err := k.enrich(ctx, &e, m); err != nil {
		return nil, err
	}

	// Deutsche Wikipedia: Einleitung als Beschreibung. Englische: Plakat aus der Infobox (die deutsche hat
	// keine Plakate, ihr freies Bild ist oft ein Darstellerfoto).
	if de := e.Sitelinks["dewiki"].Title; de != "" {
		var err error
		m.Overview, _, err = k.page(ctx, "de", de, url.Values{"prop": {"extracts"}, "exintro": {"1"}, "explaintext": {"1"}})
		if err != nil {
			return nil, err
		}
		m.Overview = paragraphs(m.Overview, 3)
	}
	if en := e.Sitelinks["enwiki"].Title; en != "" {
		var err error
		_, m.posterURL, err = k.page(ctx, "en", en, url.Values{"prop": {"pageimages"}, "piprop": {"original"}, "pilicense": {"any"}})
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}

// paragraphs kürzt einen Text auf die ersten n Absätze (Leerzeilen zählen nicht).
func paragraphs(s string, n int) string {
	var out []string
	for _, p := range strings.Split(s, "\n") {
		if p = strings.TrimSpace(p); p != "" && len(out) < n {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n\n")
}

// page liefert Einleitungstext und Bild-URL (ohne Tracking-Parameter) eines Wikipedia-Artikels.
func (k *Keyless) page(ctx context.Context, lang, title string, q url.Values) (extract, image string, err error) {
	q.Set("action", "query")
	q.Set("titles", title)
	q.Set("format", "json")
	q.Set("formatversion", "2")
	var res struct {
		Query struct {
			Pages []struct {
				Extract  string `json:"extract"`
				Original struct {
					Source string `json:"source"`
				} `json:"original"`
			} `json:"pages"`
		} `json:"query"`
	}
	if _, err := k.get(ctx, &k.wiki, fmt.Sprintf(k.Wikipedia, lang)+"?"+q.Encode(), &res); err != nil {
		return "", "", err
	}
	for _, p := range res.Query.Pages {
		src, _, _ := strings.Cut(p.Original.Source, "?")
		return strings.TrimSpace(p.Extract), src, nil
	}
	return "", "", nil
}
