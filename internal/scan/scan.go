// Package scan durchsucht Medienordner, erkennt Filme/Episoden am Dateinamen und probt jede Datei (mit Cache).
package scan

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Bavarianator/flimmer/internal/meta"
	"github.com/Bavarianator/flimmer/internal/probe"
)

type Item struct {
	ID      string       `json:"id"`
	Path    string       `json:"-"`
	Title   string       `json:"title"`
	Year    int          `json:"year,omitempty"`
	Series  string       `json:"series,omitempty"`
	Season  int          `json:"season,omitempty"`
	Episode int          `json:"episode,omitempty"`
	Size    int64        `json:"size"`
	Added   time.Time    `json:"added"`
	Media   *probe.Media `json:"-"`
}

var videoExt = []string{".mkv", ".mp4", ".m4v", ".mov", ".avi", ".ts", ".m2ts", ".webm", ".wmv", ".mpg"}

func IsVideo(path string) bool { return slices.Contains(videoExt, strings.ToLower(filepath.Ext(path))) }

var (
	reEpisode = regexp.MustCompile(`(?i)[. _-]*s(\d{1,2})[. _-]*e(\d{1,3})`)
	reYear    = regexp.MustCompile(`[(\[. _-]((?:19|20)\d{2})(?:[)\]. _-]|$)`)
	reNoise   = regexp.MustCompile(`(?i)[. _-](2160p|1080p|720p|480p|4k|uhd|bluray|blu-ray|web-?dl|webrip|hdtv|x264|x265|h\.?264|h\.?265|hevc|remux|hdr|dts|aac|ac3|german|dl)\b.*$`)
)

// Parse leitet Titel, Jahr und Staffel/Episode aus dem Dateinamen ab.
// Episoden ohne Serienname im Dateinamen erben ihn vom Ordner (Serie/Staffel 1/S01E01.mkv).
func Parse(path string) Item {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	it := Item{}
	if m := reEpisode.FindStringSubmatchIndex(name); m != nil {
		it.Season, _ = strconv.Atoi(name[m[2]:m[3]])
		it.Episode, _ = strconv.Atoi(name[m[4]:m[5]])
		it.Series = clean(name[:m[0]])
		if it.Series == "" {
			dir := filepath.Base(filepath.Dir(path))
			if strings.HasPrefix(strings.ToLower(dir), "staffel") || strings.HasPrefix(strings.ToLower(dir), "season") {
				dir = filepath.Base(filepath.Dir(filepath.Dir(path)))
			}
			it.Series = clean(dir)
		}
		it.Title = clean(reNoise.ReplaceAllString(name[m[1]:], ""))
		if it.Title == "" {
			it.Title = "Episode " + strconv.Itoa(it.Episode)
		}
		return it
	}
	// Letzte Jahreszahl gewinnt: „Blade Runner 2049 (2017)“.
	if all := reYear.FindAllStringSubmatchIndex(name, -1); all != nil && all[len(all)-1][0] > 0 {
		m := all[len(all)-1]
		it.Year, _ = strconv.Atoi(name[m[2]:m[3]])
		name = name[:m[0]]
	}
	it.Title = clean(reNoise.ReplaceAllString(name, ""))
	return it
}

func clean(s string) string {
	s = strings.NewReplacer(".", " ", "_", " ").Replace(s)
	return strings.Trim(strings.Join(strings.Fields(s), " "), " -")
}

func ID(path string) string {
	h := sha1.Sum([]byte(path))
	return hex.EncodeToString(h[:6])
}

// Library hält alle Einträge im Speicher (schnelle Listen) und den Katalog in SQLite:
// Probe-Ergebnisse (items/streams) und Keyframe-Indizes überleben so einen Neustart.
type Library struct {
	Dirs  []string
	Extra []string // weitere Ordner neben Dirs (Uploads); SetDirs lässt sie stehen
	DB    *sql.DB
	Meta  *meta.Resolver // nil = keine Metadaten
	// Colorize ermittelt die Platzhalterfarbe ("#rrggbb") direkt nach den Metadaten, nicht erst beim Abruf.
	Colorize func(ctx context.Context, it *Item, m *meta.Meta) string
	// OnScan meldet das Ende jedes Scans aus Run (für die Aktivitäten im Dashboard); nil = niemand.
	OnScan func(found int, err error)

	mu       sync.RWMutex
	items    map[string]*Item
	sorted   []*Item            // Cache für All(), nil = neu sortieren
	byAdded  []*Item            // Cache für Newest()
	series   map[string][]*Item // Cache für Episodes()
	metas    map[string]*meta.Meta
	metaErr  map[string]bool // Netzfehler bei TMDB → nächster Lauf versucht es erneut
	colors   map[string]string
	kf       map[string][]float64 // nur Titel, die gerade abgespielt werden
	inflight map[string]chan struct{}

	scanMu   sync.Mutex
	cropMu   sync.Mutex // eine Balken-Erkennung zur Zeit
	scanning atomic.Bool
	lastScan atomic.Int64 // Unix-ms des letzten vollständigen Scans
	found    atomic.Int64
	wake     chan struct{}
	metaWake chan struct{} // neue Titel: Metadaten sofort holen, nicht erst nach dem ganzen Scan
	scanned  chan struct{} // geschlossen nach dem ersten vollständigen Scan
	once     sync.Once
}

func NewLibrary(db *sql.DB, dirs []string) *Library {
	return &Library{Dirs: dirs, DB: db, wake: make(chan struct{}, 1), metaWake: make(chan struct{}, 1), scanned: make(chan struct{})}
}

// Load füllt die Bibliothek aus der Datenbank – sie ist damit sofort nach dem Start da, noch vor dem ersten Scan.
func (l *Library) Load(ctx context.Context) error {
	cat, err := loadCatalog(ctx, l.DB)
	if err != nil {
		return err
	}
	items := map[string]*Item{}
	for _, c := range cat {
		it := c.item()
		items[it.ID] = it
	}
	l.mu.Lock()
	l.items, l.sorted, l.byAdded, l.series = items, nil, nil, nil
	l.metas, l.colors, l.metaErr, l.kf = nil, nil, nil, nil // nach einem Restore neu auflösen
	l.mu.Unlock()
	l.metaSoon()
	return nil
}

func (c *cached) item() *Item {
	it := Parse(c.path)
	it.ID, it.Path, it.Size, it.Added, it.Media = c.id, c.path, c.size, time.UnixMilli(c.added), c.media
	return &it
}

// Run scannt sofort, dann alle every und auf Rescan hin; dazwischen misst Prefetch die Keyframes.
// Metadaten laufen daneben: erst für die bekannten Titel (nach einem Neustart kommen sie aus dem Cache),
// dann für jeden neu gefundenen – Poster erscheinen so während des ersten Scans, nicht erst danach.
func (l *Library) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	go func() {
		for {
			l.resolveMeta(ctx)
			select {
			case <-ctx.Done():
				return
			case <-l.metaWake:
			}
		}
	}()
	for {
		err := l.Scan(ctx)
		if err != nil && ctx.Err() == nil {
			log.Printf("scan: %v", err)
		} else {
			log.Printf("scan: %d Titel", len(l.All()))
		}
		if l.OnScan != nil && ctx.Err() == nil {
			l.OnScan(len(l.All()), err)
		}
		l.metaSoon() // auch Netzfehler vom letzten Durchlauf erneut versuchen
		l.once.Do(func() { close(l.scanned) })
		l.Prefetch(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-l.wake:
		}
	}
}

// Scanned ist geschlossen, sobald der erste Scan durch ist – danach ist die Maschine ruhiger
// (z. B. für die Hardware-Messung, die sonst mit dem Scan um die CPU konkurriert).
func (l *Library) Scanned() <-chan struct{} { return l.scanned }

// SetDirs ersetzt die Medienordner (Setup, Einstellungen) und scannt neu.
func (l *Library) SetDirs(dirs []string) {
	l.mu.Lock()
	l.Dirs = slices.Clone(dirs)
	l.mu.Unlock()
	l.Rescan()
}

// Rescan stößt einen Scan an, ohne zu warten; mehrfaches Drücken ergibt einen Scan.
func (l *Library) Rescan() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

type Status struct {
	Scanning bool  `json:"scanning"`
	Found    int64 `json:"found"`
}

// ErrBusy meldet, dass bereits ein Scan läuft.
var ErrBusy = errors.New("scan läuft bereits")

func (l *Library) Status() Status {
	return Status{Scanning: l.scanning.Load(), Found: l.found.Load()}
}

func (l *Library) Get(id string) *Item {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.items[id]
}

// Episodes liefert die Folgen einer Serie nach Staffel/Episode (zwischengespeichert; nicht verändern).
func (l *Library) Episodes(series string) []*Item {
	all := l.All()
	l.mu.RLock()
	m := l.series
	l.mu.RUnlock()
	if m == nil {
		m = map[string][]*Item{}
		for _, it := range all {
			if it.Series != "" {
				m[it.Series] = append(m[it.Series], it)
			}
		}
		l.mu.Lock()
		if l.series == nil {
			l.series = m
		}
		l.mu.Unlock()
	}
	return m[series]
}

// Newest liefert alle Titel, neueste zuerst (zwischengespeichert wie All; nicht verändern).
func (l *Library) Newest() []*Item {
	all := l.All()
	l.mu.RLock()
	out := l.byAdded
	l.mu.RUnlock()
	if out != nil {
		return out
	}
	out = slices.Clone(all)
	slices.SortStableFunc(out, func(a, b *Item) int { return b.Added.Compare(a.Added) })
	l.mu.Lock()
	if l.byAdded == nil {
		l.byAdded = out
	}
	l.mu.Unlock()
	return out
}

// All liefert alle Titel sortiert (Serie, Staffel, Episode bzw. Titel). Die Liste wird zwischengespeichert,
// bis sich der Katalog ändert – bei 50 000 Titeln kostet das Sortieren sonst jede Anfrage ~1 s.
// Aufrufer dürfen die Liste nicht verändern.
func (l *Library) All() []*Item {
	l.mu.RLock()
	out := l.sorted
	l.mu.RUnlock()
	if out != nil {
		return out
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.sorted != nil {
		return l.sorted
	}
	type keyed struct {
		key string
		it  *Item
	}
	ks := make([]keyed, 0, len(l.items))
	for _, it := range l.items {
		ks = append(ks, keyed{it.Series + it.Title, it})
	}
	slices.SortFunc(ks, func(a, b keyed) int {
		if c := strings.Compare(a.key, b.key); a.it.Series != b.it.Series || a.it.Series == "" {
			return c
		}
		if a.it.Season != b.it.Season {
			return a.it.Season - b.it.Season
		}
		return a.it.Episode - b.it.Episode
	})
	out = make([]*Item, len(ks))
	for i, k := range ks {
		out[i] = k.it
	}
	l.sorted = out
	return out
}

// Scan durchsucht alle Ordner. Neue Titel erscheinen sofort, entfernte verschwinden am Ende.
func (l *Library) Scan(ctx context.Context) error {
	if !l.scanMu.TryLock() {
		return ErrBusy
	}
	defer l.scanMu.Unlock()
	l.scanning.Store(true)
	defer l.scanning.Store(false)
	l.found.Store(0)

	cat, err := loadCatalog(ctx, l.DB)
	if err != nil {
		return err
	}
	l.mu.Lock()
	if l.items == nil {
		l.items = map[string]*Item{}
	}
	dirs := append(slices.Clone(l.Dirs), l.Extra...)
	l.mu.Unlock()

	items := map[string]*Item{}
	var offline []string
	for _, dir := range dirs {
		// Ein nicht erreichbarer Ordner (NAS aus, USB-Platte ab) darf keine Titel löschen.
		if entries, err := os.ReadDir(dir); err != nil || len(entries) == 0 {
			log.Printf("scan: %s ist nicht erreichbar oder leer – vorhandene Titel bleiben erhalten", dir)
			offline = append(offline, dir)
			continue
		}
		// WalkDir betritt keine Symlink-Wurzel (NAS: /srv/media → /mnt/disk1) – dann gäbe es 0 Funde und alle
		// Titel des Ordners würden gelöscht. Deshalb das Ziel durchlaufen, Pfade aber unter dem Ordnernamen führen,
		// den der Admin gewählt hat (stabile IDs).
		root, err := filepath.EvalSymlinks(dir)
		if err != nil {
			root = dir
		}
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if root != dir {
				path = filepath.Join(dir, strings.TrimPrefix(path, root))
			}
			if err != nil {
				log.Printf("scan: %v", err)
				return nil
			}
			if d.IsDir() || !IsVideo(path) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			var it *Item
			if c := cat[path]; c != nil && c.size == info.Size() && c.mtime == info.ModTime().UnixNano() && c.probeVersion == ProbeVersion {
				it = c.item()
			} else {
				m, err := probe.File(ctx, path)
				if err != nil {
					log.Printf("scan: %v", err)
					return ctx.Err()
				}
				log.Printf("scan: %s neu", filepath.Base(path))
				p := Parse(path)
				p.ID, p.Path, p.Size, p.Added, p.Media = ID(path), path, info.Size(), info.ModTime(), m
				if c := cat[path]; c != nil {
					p.Added = time.UnixMilli(c.added) // „Neu hinzugefügt“ bleibt beim ersten Auftauchen
				}
				if err := saveItem(ctx, l.DB, &p, info.ModTime().UnixNano()); err != nil {
					return err
				}
				it = &p
				defer l.metaSoon() // erst nach dem Eintragen unten
			}
			items[it.ID] = it
			l.mu.Lock()
			l.items[it.ID], l.sorted, l.byAdded, l.series = it, nil, nil, nil
			l.mu.Unlock()
			l.found.Add(1)
			return ctx.Err()
		})
		if err != nil {
			return err
		}
	}

	var gone []string
	for path, c := range cat {
		if items[c.id] != nil {
			continue
		}
		if slices.ContainsFunc(offline, func(dir string) bool { return strings.HasPrefix(path, filepath.Clean(dir)+string(filepath.Separator)) }) {
			items[c.id] = c.item()
			continue
		}
		gone = append(gone, c.id)
	}
	if err := deleteItems(ctx, l.DB, gone); err != nil {
		return err
	}
	l.mu.Lock()
	l.items, l.sorted, l.byAdded, l.series = items, nil, nil, nil
	l.kf = nil // Dateien können sich geändert haben
	l.mu.Unlock()
	l.lastScan.Store(time.Now().UnixMilli())
	return nil
}

// Crop liefert den Bildausschnitt ohne eingebrannte Balken; nil = keine Balken oder noch nicht gemessen.
// Ist er unbekannt, misst ein Hintergrundlauf (höchstens einer gleichzeitig, das NAS hat 2 Kerne); der nächste
// Abruf hat das Ergebnis. Ist gerade eine andere Messung dran, versucht es der nächste Abruf erneut.
func (l *Library) Crop(it *Item) *probe.Rect {
	var r probe.Rect
	err := l.DB.QueryRow("SELECT x, y, w, h FROM crops WHERE item_id = ?", it.ID).Scan(&r.X, &r.Y, &r.W, &r.H)
	switch {
	case err == nil && r.W == 0:
		return nil
	case err == nil:
		return &r
	case !errors.Is(err, sql.ErrNoRows) || !l.cropMu.TryLock():
		return nil
	}
	go func() {
		defer l.cropMu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		c, err := probe.Crop(ctx, it.Path, it.Media)
		if err != nil {
			log.Printf("scan: Balken: %v", err) // nicht speichern: Laufwerk kann gleich wieder da sein
			return
		}
		if c == nil {
			c = &probe.Rect{}
		}
		if _, err := l.DB.Exec("INSERT OR REPLACE INTO crops(item_id, x, y, w, h) VALUES(?, ?, ?, ?, ?)", it.ID, c.X, c.Y, c.W, c.H); err != nil {
			log.Printf("scan: Balken speichern: %v", err)
		}
	}()
	return nil
}

// LastScan ist das Ende des letzten vollständigen Scans (Null, solange keiner lief).
func (l *Library) LastScan() time.Time {
	if ms := l.lastScan.Load(); ms > 0 {
		return time.UnixMilli(ms)
	}
	return time.Time{}
}

// RefreshMeta holt die Metadaten aller Titel neu (Aufgabe „Metadaten aktualisieren“); gesperrte Felder bleiben.
// progress bekommt den erledigten Anteil (0..1). Netzfehler brechen nicht ab, der erste wird zurückgegeben.
func (l *Library) RefreshMeta(ctx context.Context, progress func(float64)) error {
	if l.Meta == nil {
		return errors.New("Metadaten sind ausgeschaltet")
	}
	all := l.All()
	var first error
	for i, it := range all {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		m, err := l.Meta.Refresh(ctx, it.Query())
		if err != nil && first == nil {
			first = fmt.Errorf("%s: %w", filepath.Base(it.Path), err)
		}
		l.SetMeta(it.ID, m)
		if l.Colorize != nil {
			l.SetColor(it.ID, l.Colorize(ctx, it, m))
		}
		progress(float64(i+1) / float64(len(all)))
	}
	return first
}

// Keyframes liefert den Keyframe-Index eines Titels. Er wird beim ersten Bedarf per ffprobe gelesen
// und als Datei gecacht; gleichzeitige Anfragen für denselben Titel warten auf eine gemeinsame Messung.
func (l *Library) Keyframes(ctx context.Context, it *Item) ([]float64, error) {
	for {
		l.mu.Lock()
		if kf, ok := l.kf[it.ID]; ok {
			l.mu.Unlock()
			return kf, nil
		}
		ch, busy := l.inflight[it.ID]
		if !busy {
			ch = make(chan struct{})
			if l.inflight == nil {
				l.inflight = map[string]chan struct{}{}
			}
			l.inflight[it.ID] = ch
		}
		l.mu.Unlock()
		if busy {
			select {
			case <-ch:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		kf, err := l.keyframeFile(ctx, it)
		l.mu.Lock()
		delete(l.inflight, it.ID)
		if err == nil {
			if l.kf == nil {
				l.kf = map[string][]float64{}
			}
			l.kf[it.ID] = kf
		}
		close(ch)
		l.mu.Unlock()
		return kf, err
	}
}

// keyframeFile liest den Index aus dem Cache oder misst ihn neu, wenn die Datei sich geändert hat.
func (l *Library) keyframeFile(ctx context.Context, it *Item) ([]float64, error) {
	v := it.Media.First("video")
	if v == nil {
		return nil, nil // reines Audio: Segments schneidet in festen Abständen
	}
	info, err := os.Stat(it.Path)
	if err != nil {
		return nil, err
	}
	if kf, ok := loadKeyframes(ctx, l.DB, it.ID, info.Size(), info.ModTime().UnixNano()); ok {
		return kf, nil
	}
	start := time.Now()
	kf, err := probe.Keyframes(ctx, it.Path, v.Index)
	if err != nil {
		return nil, err
	}
	log.Printf("scan: Keyframes %s (%s)", filepath.Base(it.Path), time.Since(start).Round(time.Millisecond))
	return kf, saveKeyframes(ctx, l.DB, it.ID, info.Size(), info.ModTime().UnixNano(), kf)
}

// MetaFor liefert Metadaten eines Titels, nil solange noch nicht aufgelöst.
func (l *Library) MetaFor(id string) *meta.Meta {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.metas[id]
}

func (l *Library) ColorFor(id string) string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.colors[id]
}

func (l *Library) SetColor(id, c string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.colors == nil {
		l.colors = map[string]string{}
	}
	l.colors[id] = c
}

// SetMeta ersetzt die Metadaten (nach „Falsch erkannt?“).
func (l *Library) SetMeta(id string, m *meta.Meta) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.metas == nil {
		l.metas = map[string]*meta.Meta{}
	}
	l.metas[id] = m
}

// Query beschreibt einen Titel für internal/meta.
func (it *Item) Query() meta.Query {
	return meta.Query{ID: it.ID, Path: it.Path, Title: it.Title, Series: it.Series, Year: it.Year, Season: it.Season, Episode: it.Episode}
}

// metaSoon weckt die Metadaten-Auflösung, ohne zu warten; mehrere Weckrufe ergeben einen Durchlauf.
func (l *Library) metaSoon() {
	select {
	case l.metaWake <- struct{}{}:
	default:
	}
}

// resolveMeta holt nacheinander Metadaten für alle Titel ohne; Resolve cacht selbst, TMDB wird je Titel nur einmal gefragt.
// Läuft nur in der Metadaten-Goroutine von Run; Titel, die währenddessen dazukommen, nimmt der nächste Durchlauf.
func (l *Library) resolveMeta(ctx context.Context) {
	if l.Meta == nil {
		return
	}
	for _, it := range l.All() {
		if ctx.Err() != nil {
			return
		}
		l.mu.RLock()
		done := l.metas[it.ID] != nil && !l.metaErr[it.ID]
		l.mu.RUnlock()
		if done {
			continue
		}
		m, err := l.Meta.Resolve(ctx, it.Query())
		if err != nil && ctx.Err() == nil {
			log.Printf("meta %s: %v", filepath.Base(it.Path), err)
		}
		l.SetMeta(it.ID, m)
		l.mu.Lock()
		if l.metaErr == nil {
			l.metaErr = map[string]bool{}
		}
		l.metaErr[it.ID] = err != nil
		l.mu.Unlock()
		if l.Colorize != nil {
			l.SetColor(it.ID, l.Colorize(ctx, it, m))
		}
	}
}

// Prefetch misst im Hintergrund die Keyframes aller Titel, damit der erste Start per HLS nicht warten muss.
// Er läuft nacheinander, damit Platte und CPU frei für laufende Wiedergaben bleiben.
func (l *Library) Prefetch(ctx context.Context) {
	for _, it := range l.All() {
		if ctx.Err() != nil || len(l.wake) > 0 { // angeforderter Rescan hat Vorrang
			return
		}
		if _, err := l.keyframeFile(ctx, it); err != nil && ctx.Err() == nil {
			log.Printf("scan: %v", err)
		}
	}
}
