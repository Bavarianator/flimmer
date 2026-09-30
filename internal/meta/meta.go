// Package meta ergänzt Titel um Beschreibung, Poster und Genres.
// Quellen in dieser Reihenfolge: manuelle Korrektur > NFO + lokale Bilder > TMDB > Dateiname.
// Ohne TMDB-Schlüssel springen TVmaze (Serien) und Wikidata/Wikipedia (Filme) ein, siehe keyless.go.
// Bilder werden einmal lokal gespeichert, damit Fernseher nie ins Internet müssen.
package meta

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
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
	Age           *int     `json:"age,omitempty"`       // FSK 0/6/12/16/18, nil = unbekannt; Episoden erben die Serie
	Uncertain     bool     `json:"uncertain,omitempty"` // Titel/Jahr passten nicht eindeutig → „Bitte prüfen“
	Source        string   `json:"source"`              // manual, nfo, tmdb, tvmaze, wikidata, filename
	// Keyless: aus einer schlüsselfreien Quelle (auch „nicht gefunden“). Wird ein TMDB-Schlüssel eingetragen,
	// werden diese Titel neu aufgelöst.
	Keyless bool `json:"keyless,omitempty"`

	SortTitle string   `json:"sortTitle,omitempty"`
	Tagline   string   `json:"tagline,omitempty"`
	Studios   []string `json:"studios,omitempty"` // bei Serien die Sender
	Countries []string `json:"countries,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	// People: bei Filmen am Titel, bei Serien nur am Serien-Eintrag (Show), nicht an jeder Folge.
	People     []Person    `json:"people,omitempty"`
	Collection *Collection `json:"collection,omitempty"` // Filmreihe (TMDB belongs_to_collection, NFO <set>)
	EndYear    int         `json:"endYear,omitempty"`    // nur Serien
	Status     string      `json:"status,omitempty"`     // nur Serien, TMDB: „Returning Series“, „Ended“ …
	// Locked nennt Felder (JSON-Namen), die Aktualisieren nicht überschreibt; gesetzt über PUT /api/items/{id}/meta.
	Locked []string `json:"locked,omitempty"`

	posterURL, backdropURL string // Quelle vor dem Herunterladen: URL oder lokaler Pfad
}

// Person ist ein Mitwirkender. Image ist die Quelle: TMDB-Pfad („/abc.jpg“), URL oder lokale Datei aus der NFO.
type Person struct {
	Name   string `json:"name"`
	Role   string `json:"role,omitempty"` // Rolle bzw. Aufgabe
	Kind   string `json:"kind"`           // actor, director, writer, composer, producer
	Image  string `json:"image,omitempty"`
	TMDBID int    `json:"tmdbId,omitempty"`

	wdID string // Wikidata-Objekt, nur beim Auflösen
}

type Collection struct {
	TMDBID int    `json:"tmdbId,omitempty"`
	Name   string `json:"name"`
}

// Query sind die Felder aus scan.Item, die für die Suche gebraucht werden (kein Import, kein Zyklus).
type Query struct {
	ID, Path, Title, Series string
	Year, Season, Episode   int
}

// DefaultTMDBKey wird beim Release-Build eingesetzt: -ldflags "-X github.com/Bavarianator/flimmer/internal/meta.DefaultTMDBKey=…"
var DefaultTMDBKey string

type Resolver struct {
	DB       *sql.DB // Tabelle meta (Migration in internal/db)
	CacheDir string  // Bilder unter img/
	TMDB     *TMDB
	Keyless  *Keyless // nil = ohne TMDB-Schlüssel nur NFO und Dateiname

	bios sync.Map // TMDB-Personen-ID → Lebenslauf
}

// New nimmt den Schlüssel aus den Einstellungen; leer → FLIMMER_TMDB_KEY → DefaultTMDBKey.
// Ohne jeden Schlüssel kommen Poster und Texte aus den schlüsselfreien Quellen.
// Ein alter JSON-Cache (<cacheDir>/meta/*.json) wird einmalig in die DB übernommen und danach umbenannt.
func New(db *sql.DB, cacheDir, key string) (*Resolver, error) {
	key = cmp(key, cmp(os.Getenv("FLIMMER_TMDB_KEY"), DefaultTMDBKey))
	r := &Resolver{DB: db, CacheDir: cacheDir, TMDB: NewTMDB(key), Keyless: NewKeyless()}
	return r, r.importJSON(context.Background())
}

// importJSON übernimmt den Datei-Cache aus der Zeit vor SQLite. Vorhandene DB-Zeilen gewinnen (INSERT OR IGNORE).
func (r *Resolver) importJSON(ctx context.Context) error {
	dir := filepath.Join(r.CacheDir, "meta")
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(files) == 0 {
		return nil
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, f := range files {
		b, err := os.ReadFile(f)
		var m Meta
		if err != nil || json.Unmarshal(b, &m) != nil {
			continue // kaputte Datei: wird beim nächsten Scan neu geholt
		}
		id := strings.TrimSuffix(filepath.Base(f), ".json")
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO meta(item_id, source, json, uncertain, updated_at) VALUES(?,?,?,?,?)`,
			id, m.Source, string(b), m.Uncertain, time.Now().UnixMilli()); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("meta: %d Einträge aus dem alten JSON-Cache übernommen", len(files))
	return os.Rename(dir, dir+".importiert")
}

// ImgDir enthält die Originalbilder (Poster w500, Hintergrund w1280, lokale Kopien).
func (r *Resolver) ImgDir() string { return filepath.Join(r.CacheDir, "img") }

// Resolve liefert immer Metadaten, notfalls aus dem Dateinamen. Fehler nur bei Netzproblemen mit TMDB;
// dann taugt das Ergebnis trotzdem (Source "filename") und der nächste Scan versucht es erneut.
func (r *Resolver) Resolve(ctx context.Context, q Query) (*Meta, error) {
	return r.resolve(ctx, q, false)
}

// Refresh holt die Metadaten neu, auch wenn sie schon im Cache liegen (Aufgabe „Metadaten aktualisieren“).
// Manuelle Zuordnungen behalten ihre TMDB-ID, gesperrte Felder bleiben wie sie sind.
func (r *Resolver) Refresh(ctx context.Context, q Query) (*Meta, error) {
	return r.resolve(ctx, q, true)
}

func (r *Resolver) resolve(ctx context.Context, q Query, fresh bool) (*Meta, error) {
	old := r.cached(ctx, q.ID)
	m, save, err := r.find(ctx, q, old, fresh)
	if m != old {
		keepLocked(m, old)
	}
	if save != nil {
		keepLocked(save, old)
		err = errors.Join(err, r.store(ctx, q.ID, save))
	}
	return m, err
}

// find löst einen Titel auf; save ist, was in den Cache soll (nil = nichts).
func (r *Resolver) find(ctx context.Context, q Query, old *Meta, fresh bool) (m, save *Meta, err error) {
	cached := old
	if fresh {
		cached = nil
	}
	if old != nil && old.Source == "manual" {
		if !fresh || old.TMDBID == 0 || !r.TMDB.Enabled() {
			return old, nil, nil
		}
		if m, err = r.lookup(ctx, q, old.TMDBID); m == nil || err != nil {
			return old, nil, err
		}
		m.Source = "manual"
		r.images(ctx, q.ID, m)
		return m, m, nil
	}
	m, tmdbID, show := fromNFO(q)
	if show != nil && show.Title != "" {
		r.storeShow(ctx, q.Series, show)
	}
	if m != nil {
		r.images(ctx, q.ID, m)
		return m, nil, nil
	}
	if cached != nil && !(cached.Keyless && r.TMDB.Enabled()) {
		if cached.Source == "filename" {
			return fromFilename(q), nil, nil
		}
		return cached, nil, nil
	}
	if !r.TMDB.Enabled() {
		return r.keyless(ctx, q)
	}
	if old != nil && tmdbID == 0 && q.Series == "" {
		tmdbID = old.TMDBID // Wikidata kennt oft die TMDB-ID: dann trifft der neue Schlüssel exakt
	}
	if m, err = r.lookup(ctx, q, tmdbID); err != nil {
		return fromFilename(q), nil, err // Netzfehler nicht cachen
	}
	if m == nil {
		// ponytail: „nicht gefunden“ bleibt gecacht, bis die Datei umbenannt oder per Identify korrigiert wird.
		return fromFilename(q), &Meta{Source: "filename"}, nil
	}
	r.images(ctx, q.ID, m)
	return m, m, nil
}

// keyless löst ohne TMDB-Schlüssel auf. Auch „nicht gefunden“ wird gecacht (als Keyless), damit nicht jeder
// Scan erneut fragt; ein später eingetragener Schlüssel versucht es trotzdem noch einmal.
func (r *Resolver) keyless(ctx context.Context, q Query) (m, save *Meta, err error) {
	if r.Keyless == nil {
		return fromFilename(q), nil, nil // nicht cachen: mit Schlüssel später erneut versuchen
	}
	if q.Series == "" {
		m, err = r.Keyless.Movie(ctx, q.Title, q.Year)
	} else if m, err = r.Keyless.Episode(ctx, q); m != nil {
		if show, _ := r.Keyless.Show(ctx, q.Series); show != nil { // gleiche Anfrage wie die Folge, aus dem Speicher
			r.storeShow(ctx, q.Series, show)
		}
	}
	if err != nil {
		return fromFilename(q), nil, err // Netzfehler nicht cachen
	}
	if m == nil {
		return fromFilename(q), &Meta{Source: "filename", Keyless: true}, nil
	}
	r.images(ctx, q.ID, m)
	return m, m, nil
}

// Editable sind die Felder (JSON-Namen), die PUT /api/items/{id}/meta ändern und sperren darf.
var Editable = []string{"title", "originalTitle", "sortTitle", "year", "overview", "tagline", "genres", "tags",
	"studios", "age", "rating", "people"}

// Edit übernimmt Änderungen (JSON-Namen aus Editable) in eine Kopie von m und sperrt sie. locked (nicht nil) ersetzt
// die Sperrliste. Unbekannte Felder sind ein Fehler.
func Edit(m *Meta, fields map[string]json.RawMessage, locked []string) (*Meta, error) {
	for f := range fields {
		if !slices.Contains(Editable, f) {
			return nil, fmt.Errorf("Feld %q lässt sich nicht bearbeiten", f)
		}
	}
	for _, f := range locked {
		if !slices.Contains(Editable, f) {
			return nil, fmt.Errorf("Feld %q lässt sich nicht sperren", f)
		}
	}
	out, err := overlay(m, fields)
	if err != nil {
		return nil, err
	}
	if locked == nil {
		locked = slices.Clone(m.Locked)
		for f := range fields {
			if !slices.Contains(locked, f) {
				locked = append(locked, f)
			}
		}
		slices.Sort(locked)
	}
	out.Locked = locked
	if len(out.Locked) == 0 {
		out.Locked = nil
	}
	return out, nil
}

// overlay legt Felder (JSON-Namen) über eine Kopie von m; die Bildquellen vor dem Download bleiben erhalten.
func overlay(m *Meta, fields map[string]json.RawMessage) (*Meta, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	all := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &all); err != nil {
		return nil, err
	}
	for k, v := range fields {
		if string(v) == "null" {
			delete(all, k)
		} else {
			all[k] = v
		}
	}
	b, _ = json.Marshal(all)
	out := &Meta{}
	if err := json.Unmarshal(b, out); err != nil {
		return nil, err
	}
	out.posterURL, out.backdropURL = m.posterURL, m.backdropURL
	return out, nil
}

// keepLocked übernimmt die in old gesperrten Felder nach m.
func keepLocked(m, old *Meta) {
	if m == nil || old == nil || len(old.Locked) == 0 {
		return
	}
	b, _ := json.Marshal(old)
	var all map[string]json.RawMessage
	json.Unmarshal(b, &all)
	fields := map[string]json.RawMessage{}
	for _, f := range old.Locked {
		fields[f] = cmpRaw(all[f])
	}
	if out, err := overlay(m, fields); err == nil {
		out.Locked = old.Locked
		*m = *out
	}
}

// cmpRaw: fehlendes Feld (omitempty) heißt leer.
func cmpRaw(v json.RawMessage) json.RawMessage {
	if v == nil {
		return json.RawMessage("null")
	}
	return v
}

// Save speichert von Hand bearbeitete Metadaten (PUT /api/items/{id}/meta).
func (r *Resolver) Save(ctx context.Context, id string, m *Meta) error { return r.store(ctx, id, m) }

// storeShow merkt sich die Serie unter "serie:<Name>" samt Bildern. Eine NFO schlägt TMDB; unverändert wird nicht geschrieben.
func (r *Resolver) storeShow(ctx context.Context, series string, show *Meta) {
	if series == "" {
		return
	}
	id := "serie:" + series
	var b string
	r.DB.QueryRowContext(ctx, `SELECT json FROM meta WHERE item_id = ?`, id).Scan(&b)
	if show.Source != "nfo" && strings.Contains(b, `"source":"nfo"`) {
		return
	}
	r.images(ctx, id, show)
	if nb, err := json.Marshal(show); err == nil && string(nb) == b {
		return
	}
	if err := r.store(ctx, id, show); err != nil {
		log.Printf("meta: Serie %s: %v", series, err)
	}
}

// Shows liefert alle gespeicherten Serien nach Name.
func (r *Resolver) Shows(ctx context.Context) (map[string]*Meta, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT item_id, json FROM meta WHERE item_id LIKE 'serie:%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]*Meta{}
	for rows.Next() {
		var id, b string
		var m Meta
		if rows.Scan(&id, &b) == nil && json.Unmarshal([]byte(b), &m) == nil {
			out[strings.TrimPrefix(id, "serie:")] = &m
		}
	}
	return out, rows.Err()
}

// PersonImage lädt das Bild eines Mitwirkenden einmal nach ImgDir() und liefert den lokalen Pfad.
func (r *Resolver) PersonImage(ctx context.Context, src string) (string, error) {
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		return src, nil // lokale Datei aus der NFO
	}
	dst := filepath.Join(r.ImgDir(), imageName("", "", src, r.TMDB.ImageURL))
	return dst, r.save(ctx, src, dst)
}

// Biography liefert den Lebenslauf zu einer TMDB-Person, gemerkt für die Laufzeit des Servers.
func (r *Resolver) Biography(ctx context.Context, tmdbID int) string {
	if v, ok := r.bios.Load(tmdbID); ok {
		return v.(string)
	}
	bio, err := r.TMDB.Biography(ctx, tmdbID)
	if err != nil {
		log.Printf("meta: Person %d: %v", tmdbID, err)
		return ""
	}
	r.bios.Store(tmdbID, bio)
	return bio
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
	keepLocked(m, r.cached(ctx, q.ID))
	r.images(ctx, q.ID, m)
	return m, r.store(ctx, q.ID, m)
}

// Show liefert die Metadaten einer Serie (Beschreibung, Sender, Personen), nil wenn unbekannt.
// Sie liegen in der Tabelle meta unter "serie:<Name>", Name wie im Dateinamen (scan.Item.Series).
func (r *Resolver) Show(ctx context.Context, series string) *Meta {
	return r.cached(ctx, "serie:"+series)
}

func (r *Resolver) cached(ctx context.Context, id string) *Meta {
	var b string
	if r.DB.QueryRowContext(ctx, `SELECT json FROM meta WHERE item_id = ?`, id).Scan(&b) != nil {
		return nil // auch sql.ErrNoRows
	}
	var m Meta
	if json.Unmarshal([]byte(b), &m) != nil {
		return nil
	}
	return &m
}

func (r *Resolver) store(ctx context.Context, id string, m *Meta) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO meta(item_id, source, json, uncertain, updated_at) VALUES(?,?,?,?,?)
		ON CONFLICT(item_id) DO UPDATE SET source=excluded.source, json=excluded.json, uncertain=excluded.uncertain, updated_at=excluded.updated_at`,
		id, m.Source, string(b), m.Uncertain, time.Now().UnixMilli())
	return err
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
	r.storeShow(ctx, q.Series, show)
	ep, err := r.TMDB.Episode(ctx, show.TMDBID, q.Season, q.Episode)
	if err != nil {
		return nil, err
	}
	if ep == nil { // Episode fehlt bei TMDB: wenigstens die Serie zeigen
		ep = &Meta{Title: q.Title, Season: q.Season, Episode: q.Episode, Source: "tmdb"}
	}
	ep.Series, ep.TMDBID, ep.Genres, ep.posterURL, ep.Uncertain, ep.Age = show.Title, show.TMDBID, show.Genres, show.posterURL, show.Uncertain, show.Age
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
		req.Header.Set("User-Agent", userAgent) // Wikimedia lehnt Anfragen ohne sprechenden User-Agent ab
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
