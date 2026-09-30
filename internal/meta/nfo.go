package meta

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Kodi-NFO: <movie>, <tvshow> oder <episodedetails>. Felder, die in einem Typ fehlen, bleiben leer.
type nfo struct {
	Title         string `xml:"title"`
	OriginalTitle string `xml:"originaltitle"`
	ShowTitle     string `xml:"showtitle"`
	Year          string `xml:"year"`
	Premiered     string `xml:"premiered"`
	Aired         string `xml:"aired"`
	Plot          string `xml:"plot"`
	Outline       string `xml:"outline"`
	Rating        string `xml:"rating"`
	Ratings       []struct {
		Default bool    `xml:"default,attr"`
		Value   float64 `xml:"value"`
	} `xml:"ratings>rating"`
	UniqueIDs []struct {
		Type  string `xml:"type,attr"`
		Value string `xml:",chardata"`
	} `xml:"uniqueid"`
	TMDBID  string   `xml:"tmdbid"`
	IMDBID  string   `xml:"imdbid"`
	ID      string   `xml:"id"`
	Genres  []string `xml:"genre"`
	MPAA    string   `xml:"mpaa"`          // Kodi: „FSK 12“, „de:12“ …
	Cert    string   `xml:"certification"` // „DE:12 / US:PG-13“
	Season  string   `xml:"season"`
	Episode string   `xml:"episode"`
	Thumbs  []struct {
		Aspect string `xml:"aspect,attr"`
		URL    string `xml:",chardata"`
	} `xml:"thumb"`
	Fanart    []string `xml:"fanart>thumb"`
	SortTitle string   `xml:"sorttitle"`
	Tagline   string   `xml:"tagline"`
	Studios   []string `xml:"studio"`
	Countries []string `xml:"country"`
	Tags      []string `xml:"tag"`
	Status    string   `xml:"status"`
	Directors []string `xml:"director"`
	Writers   []string `xml:"credits"`
	Actors    []struct {
		Name  string `xml:"name"`
		Role  string `xml:"role"`
		Thumb string `xml:"thumb"`
	} `xml:"actor"`
	Set struct { // <set>Name</set> oder <set><name>Name</name></set>
		Name string `xml:"name"`
		Text string `xml:",chardata"`
	} `xml:"set"`
}

var (
	reTMDBURL = regexp.MustCompile(`themoviedb\.org/(?:movie|tv)/(\d+)`)
	reIMDB    = regexp.MustCompile(`\btt\d{7,}\b`)
)

// ParseNFO liest eine Kodi-NFO. Auch reine URL-NFOs („https://www.themoviedb.org/movie/335984“) liefern
// wenigstens die IDs; dann ist Title leer und Resolve holt den Rest von TMDB.
func ParseNFO(path string) (*Meta, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m := &Meta{Source: "nfo"}
	var n nfo
	// Nur das erste Element zählt, angehängte URLs (Hybrid-NFO) werden ignoriert.
	if err := xml.Unmarshal(b, &n); err == nil {
		m.Title, m.OriginalTitle, m.Series = strings.TrimSpace(n.Title), strings.TrimSpace(n.OriginalTitle), strings.TrimSpace(n.ShowTitle)
		m.Overview = strings.TrimSpace(cmp(n.Plot, n.Outline))
		m.Year = year0(cmp(n.Year, cmp(n.Premiered, n.Aired)))
		m.Season, _ = strconv.Atoi(strings.TrimSpace(n.Season))
		m.Episode, _ = strconv.Atoi(strings.TrimSpace(n.Episode))
		m.Genres = n.Genres
		if m.Age = fsk(n.MPAA); m.Age == nil {
			m.Age = fsk(n.Cert)
		}
		m.Rating, _ = strconv.ParseFloat(strings.TrimSpace(n.Rating), 64)
		for _, r := range n.Ratings {
			if r.Default || m.Rating == 0 {
				m.Rating = r.Value
			}
		}
		for _, id := range n.UniqueIDs {
			switch strings.ToLower(id.Type) {
			case "tmdb":
				m.TMDBID, _ = strconv.Atoi(strings.TrimSpace(id.Value))
			case "imdb":
				m.IMDBID = strings.TrimSpace(id.Value)
			}
		}
		if m.TMDBID == 0 {
			m.TMDBID, _ = strconv.Atoi(strings.TrimSpace(n.TMDBID))
		}
		m.IMDBID = cmp(m.IMDBID, cmp(strings.TrimSpace(n.IMDBID), reIMDB.FindString(n.ID)))
		for _, t := range n.Thumbs {
			if u := strings.TrimSpace(t.URL); u != "" && (t.Aspect == "poster" || t.Aspect == "" && m.posterURL == "") {
				m.posterURL = u
			}
		}
		if len(n.Fanart) > 0 {
			m.backdropURL = strings.TrimSpace(n.Fanart[0])
		}
		m.SortTitle, m.Tagline, m.Status = strings.TrimSpace(n.SortTitle), strings.TrimSpace(n.Tagline), strings.TrimSpace(n.Status)
		m.Studios, m.Countries, m.Tags = n.Studios, n.Countries, n.Tags
		for _, d := range n.Directors {
			m.People = append(m.People, Person{Name: strings.TrimSpace(d), Kind: "director"})
		}
		for _, w := range n.Writers {
			m.People = append(m.People, Person{Name: strings.TrimSpace(w), Kind: "writer"})
		}
		for _, a := range n.Actors {
			m.People = append(m.People, Person{Name: strings.TrimSpace(a.Name), Role: strings.TrimSpace(a.Role), Kind: "actor", Image: strings.TrimSpace(a.Thumb)})
		}
		if set := strings.TrimSpace(cmp(n.Set.Name, n.Set.Text)); set != "" {
			m.Collection = &Collection{Name: set}
		}
	}
	if m.TMDBID == 0 {
		if s := reTMDBURL.FindSubmatch(b); s != nil {
			m.TMDBID, _ = strconv.Atoi(string(s[1]))
		}
	}
	m.IMDBID = cmp(m.IMDBID, reIMDB.FindString(string(b)))
	return m, nil
}

// year0 liest das Jahr aus „2017“ oder „2017-10-04“.
func year0(date string) int {
	date = strings.TrimSpace(date)
	y, _ := strconv.Atoi(date[:min(4, len(date))])
	return y
}

// fromNFO sucht NFO und lokale Bilder nach Kodi-Konvention.
// Liefert die fertigen Metadaten, wenn die NFO einen Titel hat; sonst höchstens eine TMDB-ID als Suchhilfe.
// show ist die tvshow.nfo einer Serie samt lokaler Bilder (nil ohne).
func fromNFO(q Query) (m *Meta, tmdbID int, show *Meta) {
	dir := filepath.Dir(q.Path)
	base := strings.TrimSuffix(q.Path, filepath.Ext(q.Path))
	names := []string{base + ".nfo"}
	if q.Series == "" {
		names = append(names, filepath.Join(dir, "movie.nfo"))
	}
	for _, p := range names {
		if nm, err := ParseNFO(p); err == nil {
			m = nm
			break
		}
	}
	showDir := dir
	if l := strings.ToLower(filepath.Base(dir)); strings.HasPrefix(l, "staffel") || strings.HasPrefix(l, "season") {
		showDir = filepath.Dir(dir)
	}
	if q.Series != "" {
		if show, _ = ParseNFO(filepath.Join(showDir, "tvshow.nfo")); show != nil {
			show.posterURL = cmp(findArt(filepath.Join(showDir, "tvshow"), showDir, "poster", "folder"), show.posterURL)
			show.backdropURL = cmp(findArt(filepath.Join(showDir, "tvshow"), showDir, "fanart", "backdrop"), show.backdropURL)
		}
	}
	if m == nil || m.Title == "" {
		switch {
		case m != nil && q.Series == "" && m.TMDBID > 0:
			return nil, m.TMDBID, show
		case show != nil && show.TMDBID > 0:
			return nil, show.TMDBID, show
		}
		return nil, 0, show
	}
	// Lokale Bilder schlagen URLs aus der NFO: <name>-poster.jpg, poster.jpg, folder.jpg, <name>-fanart.jpg, fanart.jpg.
	artDir := dir
	if q.Series != "" {
		artDir = showDir
		if show != nil {
			m.Series = cmp(m.Series, show.Title)
			m.TMDBID = cmp0(show.TMDBID, m.TMDBID)
			m.Genres = append(m.Genres, show.Genres...)
			if m.Age == nil {
				m.Age = show.Age
			}
			m.posterURL = cmp(show.posterURL, m.posterURL)
			m.backdropURL = cmp(m.backdropURL, show.backdropURL)
		}
	}
	m.posterURL = cmp(findArt(base, artDir, "poster", "folder"), m.posterURL)
	m.backdropURL = cmp(findArt(base, artDir, "fanart", "backdrop"), m.backdropURL)
	if q.Series != "" {
		m.Season, m.Episode = cmp0(m.Season, q.Season), cmp0(m.Episode, q.Episode)
	}
	return m, m.TMDBID, show
}

func findArt(base, dir string, kinds ...string) string {
	for _, k := range kinds {
		for _, ext := range []string{".jpg", ".png", ".webp"} {
			for _, p := range []string{base + "-" + k + ext, filepath.Join(dir, k+ext)} {
				if _, err := os.Stat(p); err == nil {
					return p
				}
			}
		}
	}
	return ""
}

// reFSK: reine Zahl (TMDB-DE) oder eine deutsche Angabe in Texten wie „FSK 12“, „de:12 / us:PG-13“, „Germany:FSK 16“.
var reFSK = regexp.MustCompile(`(?i)^\s*(\d{1,2})\s*$|(?:fsk|\bde:|germany:)\s*(?:fsk\s*)?(\d{1,2})\b`)

// fsk liest eine FSK-Freigabe; alles andere (z. B. „PG-13“) ist unbekannt (nil).
func fsk(s string) *int {
	m := reFSK.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	n, _ := strconv.Atoi(m[1] + m[2])
	if n != 0 && n != 6 && n != 12 && n != 16 && n != 18 {
		return nil
	}
	return &n
}

func cmp0(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}
