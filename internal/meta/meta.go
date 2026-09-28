// Package meta ergänzt Titel um Beschreibung, Poster und Genres.
// Quellen in dieser Reihenfolge: manuelle Korrektur > NFO + lokale Bilder > TMDB > Dateiname.
// Bilder werden einmal lokal gespeichert, damit Fernseher nie ins Internet müssen.
package meta

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Meta struct {
	Title         string   `json:"title,omitempty"`
	OriginalTitle string   `json:"originalTitle,omitempty"`
	Series        string   `json:"series,omitempty"` // nur Episoden
	Year          int      `json:"year,omitempty"`
	Season        int      `json:"season,omitempty"`
	Episode       int      `json:"episode,omitempty"`
	Overview      string   `json:"overview,omitempty"`
	PosterPath    string   `json:"poster,omitempty"`   // Dateiname unter ImgDir(), leer = kein Bild
	BackdropPath  string   `json:"backdrop,omitempty"` // dito; bei Episoden das Standbild
	TMDBID        int      `json:"tmdbId,omitempty"`   // bei Episoden die Serie
	IMDBID        string   `json:"imdbId,omitempty"`
	Rating        float64  `json:"rating,omitempty"`
	Genres        []string `json:"genres,omitempty"`
	Uncertain     bool     `json:"uncertain,omitempty"` // Titel/Jahr passten nicht eindeutig → „Bitte prüfen“
	Source        string   `json:"source"`              // manual, nfo, tmdb, filename

	posterURL, backdropURL string // Quelle vor dem Herunterladen: URL oder lokaler Pfad
}

// Query sind die Felder aus scan.Item, die für die Suche gebraucht werden (kein Import, kein Zyklus).
type Query struct {
	ID, Path, Title, Series string
	Year, Season, Episode   int
}

// DefaultTMDBKey wird beim Release-Build eingesetzt: -ldflags "-X github.com/flimmer-media/flimmer/internal/meta.DefaultTMDBKey=…"
var DefaultTMDBKey string

type Resolver struct {
	CacheDir string // Metadaten unter meta/, Bilder unter img/
	TMDB     *TMDB
}

// New nimmt den Schlüssel aus den Einstellungen; leer → FLIMMER_TMDB_KEY → DefaultTMDBKey.
// Ohne jeden Schlüssel gibt es nur NFO und Dateinamen.
func New(cacheDir, key string) *Resolver {
	key = cmp(key, cmp(os.Getenv("FLIMMER_TMDB_KEY"), DefaultTMDBKey))
	return &Resolver{CacheDir: cacheDir, TMDB: NewTMDB(key)}
}

// ImgDir enthält die Originalbilder (Poster w500, Hintergrund w1280, lokale Kopien).
func (r *Resolver) ImgDir() string { return filepath.Join(r.CacheDir, "img") }

func (r *Resolver) cacheFile(id string) string { return filepath.Join(r.CacheDir, "meta", id+".json") }

// Resolve liefert immer Metadaten, notfalls aus dem Dateinamen. Fehler nur bei Netzproblemen mit TMDB;
// dann taugt das Ergebnis trotzdem (Source "filename") und der nächste Scan versucht es erneut.
func (r *Resolver) Resolve(ctx context.Context, q Query) (*Meta, error) {
	cached := r.readCache(q.ID)
	if cached != nil && cached.Source == "manual" {
		return cached, nil
	}
	m, tmdbID := fromNFO(q)
	if m != nil {
		r.images(ctx, q.ID, m)
		return m, nil
	}
	if cached != nil {
		if cached.Source == "filename" {
			return fromFilename(q), nil
		}
		return cached, nil
	}
	if !r.TMDB.Enabled() {
		return fromFilename(q), nil // nicht cachen: mit Schlüssel später erneut versuchen
	}
	m, err := r.lookup(ctx, q, tmdbID)
	if err != nil {
		return fromFilename(q), err // Netzfehler nicht cachen
	}
	if m == nil {
		// ponytail: „nicht gefunden“ bleibt gecacht, bis die Datei umbenannt oder per Identify korrigiert wird.
		return fromFilename(q), writeJSON(r.cacheFile(q.ID), Meta{Source: "filename"})
	}
	r.images(ctx, q.ID, m)
	return m, writeJSON(r.cacheFile(q.ID), m)
}

// Identify legt einen Titel manuell auf eine TMDB-ID fest („Falsch erkannt?“). Das schlägt jede andere Quelle.
// Bei Episoden ist tmdbID die Serie; für eine ganze Serie einmal pro Episode aufrufen.
func (r *Resolver) Identify(ctx context.Context, q Query, tmdbID int) (*Meta, error) {
	if !r.TMDB.Enabled() {
		return nil, errors.New("kein TMDB-Schlüssel")
	}
	m, err := r.lookup(ctx, q, tmdbID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("TMDB-ID %d nicht gefunden", tmdbID)
	}
	m.Source, m.Uncertain = "manual", false
	r.images(ctx, q.ID, m)
	return m, writeJSON(r.cacheFile(q.ID), m)
}

func (r *Resolver) readCache(id string) *Meta {
	b, err := os.ReadFile(r.cacheFile(id))
	if err != nil {
		return nil
	}
	var m Meta
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	return &m
}

func fromFilename(q Query) *Meta {
	return &Meta{Title: q.Title, Series: q.Series, Year: q.Year, Season: q.Season, Episode: q.Episode, Source: "filename"}
}

func (r *Resolver) lookup(ctx context.Context, q Query, tmdbID int) (*Meta, error) {
	if q.Series == "" {
		if tmdbID > 0 {
			return r.TMDB.Movie(ctx, tmdbID)
		}
		return r.TMDB.SearchMovie(ctx, q.Title, q.Year)
	}
	var show *Meta
	var err error
	if tmdbID > 0 {
		show, err = r.TMDB.TV(ctx, tmdbID)
	} else {
		show, err = r.TMDB.SearchTV(ctx, q.Series)
	}
	if err != nil || show == nil {
		return nil, err
	}
	ep, err := r.TMDB.Episode(ctx, show.TMDBID, q.Season, q.Episode)
	if err != nil {
		return nil, err
	}
	if ep == nil { // Episode fehlt bei TMDB: wenigstens die Serie zeigen
		ep = &Meta{Title: q.Title, Season: q.Season, Episode: q.Episode, Source: "tmdb"}
	}
	ep.Series, ep.TMDBID, ep.Genres, ep.posterURL, ep.Uncertain = show.Title, show.TMDBID, show.Genres, show.posterURL, show.Uncertain
	ep.Overview = cmp(ep.Overview, show.Overview)
	ep.backdropURL = cmp(ep.backdropURL, show.backdropURL)
	return ep, nil
}

// images lädt Poster und Hintergrund nach ImgDir() und ersetzt die Quellen durch lokale Dateinamen.
// Ein fehlendes Bild ist kein Fehler: Die UI zeigt dann die dominante Farbe bzw. ein Standbild.
func (r *Resolver) images(ctx context.Context, id string, m *Meta) {
	for _, img := range []struct {
		src  string
		dst  *string
		kind string
	}{{m.posterURL, &m.PosterPath, "poster"}, {m.backdropURL, &m.BackdropPath, "backdrop"}} {
		if img.src == "" {
			continue
		}
		name := imageName(id, img.kind, img.src, r.TMDB.ImageURL)
		if err := r.save(ctx, img.src, filepath.Join(r.ImgDir(), name)); err != nil {
			log.Printf("meta: %v", err)
			continue
		}
		*img.dst = name
	}
}

// TMDB-Bilder haben eindeutige Namen; fremde URLs und lokale Dateien bekommen einen Hash bzw. die Titel-ID.
func imageName(id, kind, src, tmdbImages string) string {
	ext := strings.ToLower(filepath.Ext(src))
	if ext == "" || len(ext) > 5 {
		ext = ".jpg"
	}
	switch {
	case tmdbImages != "" && strings.HasPrefix(src, tmdbImages+"/"): // …/t/p/w500/abc.jpg → w500-abc.jpg
		return filepath.Base(filepath.Dir(src)) + "-" + filepath.Base(src)
	case strings.HasPrefix(src, "http"):
		h := sha1.Sum([]byte(src))
		return "nfo-" + hex.EncodeToString(h[:6]) + ext
	}
	return id + "-" + kind + ext
}

func (r *Resolver) save(ctx context.Context, src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	var body io.ReadCloser
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		req, err := http.NewRequestWithContext(ctx, "GET", src, nil)
		if err != nil {
			return err
		}
		resp, err := r.TMDB.HTTP.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("bild %s: %s", src, resp.Status)
		}
		body = resp.Body
	} else {
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		body = f
	}
	defer body.Close()
	b, err := io.ReadAll(io.LimitReader(body, 20<<20))
	if err != nil {
		return err
	}
	return writeAtomic(dst, b)
}

func writeJSON(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeAtomic(path, b)
}

func writeAtomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if err = errors.Join(err, f.Close()); err != nil {
		os.Remove(f.Name())
		return err
	}
	return os.Rename(f.Name(), path)
}

// cmp liefert den ersten nicht leeren String.
func cmp(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
