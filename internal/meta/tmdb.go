package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// TMDB ist ein minimaler Client für api.themoviedb.org/3. Ohne Schlüssel liefern alle Methoden nil, nil.
type TMDB struct {
	Key      string // v3-API-Key oder v4-Lesetoken („eyJ…“)
	BaseURL  string
	ImageURL string // Präfix für Bildgrößen, z. B. https://image.tmdb.org/t/p
	HTTP     *http.Client

	once sync.Once
	tick <-chan time.Time
}

// Abstand zwischen API-Anfragen; TMDB erlaubt ~50/s, wir bleiben bei 20/s.
const minGap = 50 * time.Millisecond

func NewTMDB(key string) *TMDB {
	return &TMDB{Key: key, BaseURL: "https://api.themoviedb.org/3", ImageURL: "https://image.tmdb.org/t/p",
		HTTP: &http.Client{Timeout: 15 * time.Second}}
}

func (t *TMDB) Enabled() bool { return t != nil && t.Key != "" }

const lang, fallbackLang = "de-DE", "en-US"

type details struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	Name          string  `json:"name"`
	OriginalTitle string  `json:"original_title"`
	OriginalName  string  `json:"original_name"`
	Overview      string  `json:"overview"`
	ReleaseDate   string  `json:"release_date"`
	FirstAirDate  string  `json:"first_air_date"`
	AirDate       string  `json:"air_date"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	StillPath     string  `json:"still_path"`
	VoteAverage   float64 `json:"vote_average"`
	IMDBID        string  `json:"imdb_id"`
	SeasonNumber  int     `json:"season_number"`
	EpisodeNumber int     `json:"episode_number"`
	Genres        []struct {
		Name string `json:"name"`
	} `json:"genres"`
}

// get ruft path auf; 404 ergibt found=false ohne Fehler.
func (t *TMDB) get(ctx context.Context, path string, q url.Values, language string, out any) (found bool, err error) {
	if q == nil {
		q = url.Values{}
	}
	q.Set("language", language)
	req, err := http.NewRequestWithContext(ctx, "GET", "", nil)
	if err != nil {
		return false, err
	}
	if strings.HasPrefix(t.Key, "eyJ") {
		req.Header.Set("Authorization", "Bearer "+t.Key)
	} else {
		q.Set("api_key", t.Key)
	}
	req.URL, err = url.Parse(t.BaseURL + path + "?" + q.Encode())
	if err != nil {
		return false, err
	}
	t.once.Do(func() { t.tick = time.NewTicker(minGap).C })
	select {
	case <-t.tick:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	resp, err := t.HTTP.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, json.NewDecoder(resp.Body).Decode(out)
	case http.StatusNotFound:
		return false, nil
	}
	// ponytail: kein Retry bei 429; der Scan läuft sequenziell und bleibt weit unter dem Limit.
	return false, fmt.Errorf("tmdb %s: %s", path, resp.Status)
}

// fetch holt Details auf Deutsch und füllt fehlende Texte aus der englischen Fassung.
func (t *TMDB) fetch(ctx context.Context, path string) (*Meta, error) {
	var d details
	if ok, err := t.get(ctx, path, nil, lang, &d); !ok || err != nil {
		return nil, err
	}
	if d.Overview == "" || cmp(d.Title, d.Name) == "" {
		var en details
		if _, err := t.get(ctx, path, nil, fallbackLang, &en); err != nil {
			return nil, err
		}
		d.Overview = cmp(d.Overview, en.Overview)
		d.Title, d.Name = cmp(d.Title, en.Title), cmp(d.Name, en.Name)
	}
	m := &Meta{
		Title:         cmp(d.Title, d.Name),
		OriginalTitle: cmp(d.OriginalTitle, d.OriginalName),
		Year:          year0(cmp(d.ReleaseDate, cmp(d.FirstAirDate, d.AirDate))),
		Season:        d.SeasonNumber,
		Episode:       d.EpisodeNumber,
		Overview:      d.Overview,
		TMDBID:        d.ID,
		IMDBID:        d.IMDBID,
		Rating:        d.VoteAverage,
		Source:        "tmdb",
	}
	if m.OriginalTitle == m.Title {
		m.OriginalTitle = ""
	}
	for _, g := range d.Genres {
		m.Genres = append(m.Genres, g.Name)
	}
	if d.PosterPath != "" {
		m.posterURL = t.ImageURL + "/w500" + d.PosterPath
	}
	if bd := cmp(d.BackdropPath, d.StillPath); bd != "" {
		m.backdropURL = t.ImageURL + "/w1280" + bd
	}
	return m, nil
}

// Candidate ist ein Suchtreffer für „Falsch erkannt?“.
type Candidate struct {
	TMDBID        int    `json:"tmdbId"`
	Title         string `json:"title"`
	OriginalTitle string `json:"originalTitle,omitempty"`
	Year          int    `json:"year,omitempty"`
	Overview      string `json:"overview,omitempty"`
	Poster        string `json:"poster,omitempty"` // TMDB-URL (w185), nur für die Auswahl im Browser
}

// Search liefert bis zu 10 Treffer; tv=true sucht Serien.
func (t *TMDB) Search(ctx context.Context, query string, year int, tv bool) ([]Candidate, error) {
	if !t.Enabled() {
		return nil, nil
	}
	kind, yearParam := "movie", "year"
	if tv {
		kind, yearParam = "tv", "first_air_date_year"
	}
	q := url.Values{"query": {query}}
	if year > 0 {
		q.Set(yearParam, strconv.Itoa(year))
	}
	var res struct {
		Results []details `json:"results"`
	}
	if _, err := t.get(ctx, "/search/"+kind, q, lang, &res); err != nil {
		return nil, err
	}
	var out []Candidate
	for _, d := range res.Results[:min(10, len(res.Results))] {
		c := Candidate{TMDBID: d.ID, Title: cmp(d.Title, d.Name), OriginalTitle: cmp(d.OriginalTitle, d.OriginalName),
			Year: year0(cmp(d.ReleaseDate, d.FirstAirDate)), Overview: d.Overview}
		if d.PosterPath != "" {
			c.Poster = t.ImageURL + "/w185" + d.PosterPath
		}
		out = append(out, c)
	}
	return out, nil
}

// best nimmt den ersten Treffer. Sicher ist er nur, wenn Titel (deutsch oder original) und Jahr passen;
// ohne Treffer mit Jahr wird ohne Jahr gesucht, weil Jahre in Dateinamen oft um eins daneben liegen.
func (t *TMDB) best(ctx context.Context, title string, year int, tv bool) (id int, uncertain bool, err error) {
	cs, err := t.Search(ctx, title, year, tv)
	if err != nil {
		return 0, false, err
	}
	retried := false
	if len(cs) == 0 && year > 0 {
		retried = true
		if cs, err = t.Search(ctx, title, 0, tv); err != nil {
			return 0, false, err
		}
	}
	if len(cs) == 0 {
		return 0, false, nil
	}
	c := cs[0]
	titleOK := norm(c.Title) == norm(title) || norm(c.OriginalTitle) == norm(title)
	yearOK := year == 0 || c.Year == year
	return c.TMDBID, retried || !titleOK || !yearOK, nil
}

// norm vergleicht Titel ohne Groß-/Kleinschreibung, Satzzeichen und Leerzeichen.
func norm(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}

func (t *TMDB) SearchMovie(ctx context.Context, title string, year int) (*Meta, error) {
	if !t.Enabled() {
		return nil, nil
	}
	id, uncertain, err := t.best(ctx, title, year, false)
	if id == 0 || err != nil {
		return nil, err
	}
	m, err := t.Movie(ctx, id)
	if m != nil {
		m.Uncertain = uncertain
	}
	return m, err
}

func (t *TMDB) SearchTV(ctx context.Context, title string) (*Meta, error) {
	if !t.Enabled() {
		return nil, nil
	}
	id, uncertain, err := t.best(ctx, title, 0, true)
	if id == 0 || err != nil {
		return nil, err
	}
	m, err := t.TV(ctx, id)
	if m != nil {
		m.Uncertain = uncertain
	}
	return m, err
}

func (t *TMDB) Movie(ctx context.Context, id int) (*Meta, error) {
	if !t.Enabled() {
		return nil, nil
	}
	return t.fetch(ctx, "/movie/"+strconv.Itoa(id))
}

func (t *TMDB) TV(ctx context.Context, id int) (*Meta, error) {
	if !t.Enabled() {
		return nil, nil
	}
	return t.fetch(ctx, "/tv/"+strconv.Itoa(id))
}

// Episode liefert Titel, Beschreibung und Standbild einer Episode (ohne Serien-Poster/Genres).
func (t *TMDB) Episode(ctx context.Context, tvID, season, episode int) (*Meta, error) {
	if !t.Enabled() {
		return nil, nil
	}
	return t.fetch(ctx, fmt.Sprintf("/tv/%d/season/%d/episode/%d", tvID, season, episode))
}
