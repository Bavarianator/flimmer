// Package scan durchsucht Medienordner, erkennt Filme/Episoden am Dateinamen und probt jede Datei (mit Cache).
package scan

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
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

	"github.com/flimmer-media/flimmer/internal/probe"
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
	Media   *probe.Media `json:"-"`
}

var videoExt = []string{".mkv", ".mp4", ".m4v", ".mov", ".avi", ".ts", ".m2ts", ".webm", ".wmv", ".mpg"}

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

// Library hält alle Einträge im Speicher; Probe-Ergebnisse landen in einer JSON-Cache-Datei,
// Keyframe-Indizes (groß, nur für HLS nötig) als eigene Datei pro Titel.
// ponytail: JSON-Cache statt DB; SQLite erst, wenn Messungen es verlangen.
type Library struct {
	Dirs     []string
	CacheDir string

	mu       sync.RWMutex
	items    map[string]*Item
	kf       map[string][]float64 // nur Titel, die gerade abgespielt werden
	inflight map[string]chan struct{}

	scanMu   sync.Mutex
	scanning atomic.Bool
	found    atomic.Int64
	wake     chan struct{}
}

func NewLibrary(dirs []string, cacheDir string) *Library {
	return &Library{Dirs: dirs, CacheDir: cacheDir, wake: make(chan struct{}, 1)}
}

// Run scannt sofort, dann alle every und auf Rescan hin; dazwischen misst Prefetch die Keyframes.
func (l *Library) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if err := l.Scan(ctx); err != nil && ctx.Err() == nil {
			log.Printf("scan: %v", err)
		} else {
			log.Printf("scan: %d Titel", len(l.All()))
		}
		l.Prefetch(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-l.wake:
		}
	}
}

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

type cacheEntry struct {
	Size  int64        `json:"size"`
	MTime time.Time    `json:"mtime"`
	Media *probe.Media `json:"media"`
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

func (l *Library) All() []*Item {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]*Item, 0, len(l.items))
	for _, it := range l.items {
		out = append(out, it)
	}
	slices.SortFunc(out, func(a, b *Item) int {
		if c := strings.Compare(a.Series+a.Title, b.Series+b.Title); a.Series != b.Series || a.Series == "" {
			return c
		}
		if a.Season != b.Season {
			return a.Season - b.Season
		}
		return a.Episode - b.Episode
	})
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

	cacheFile := filepath.Join(l.CacheDir, "probe-cache.json")
	cache := map[string]cacheEntry{}
	if b, err := os.ReadFile(cacheFile); err == nil {
		_ = json.Unmarshal(b, &cache)
	}
	l.mu.Lock()
	if l.items == nil {
		l.items = map[string]*Item{}
	}
	dirs := slices.Clone(l.Dirs)
	l.mu.Unlock()

	items := map[string]*Item{}
	newCache := map[string]cacheEntry{}
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("scan: %v", err)
				return nil
			}
			if d.IsDir() || !slices.Contains(videoExt, strings.ToLower(filepath.Ext(path))) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			c, ok := cache[path]
			if !ok || c.Size != info.Size() || !c.MTime.Equal(info.ModTime()) {
				m, err := probe.File(ctx, path)
				if err != nil {
					log.Printf("scan: %v", err)
					return ctx.Err()
				}
				log.Printf("scan: %s neu", filepath.Base(path))
				c = cacheEntry{Size: info.Size(), MTime: info.ModTime(), Media: m}
			}
			newCache[path] = c
			it := Parse(path)
			it.ID, it.Path, it.Size, it.Media = ID(path), path, info.Size(), c.Media
			items[it.ID] = &it
			l.mu.Lock()
			l.items[it.ID] = &it
			l.mu.Unlock()
			l.found.Add(1)
			return ctx.Err()
		})
		if err != nil {
			return err
		}
	}
	l.mu.Lock()
	l.items = items
	l.kf = nil // Dateien können sich geändert haben
	l.mu.Unlock()
	b, err := json.Marshal(newCache)
	if err != nil {
		return err
	}
	return writeAtomic(cacheFile, b)
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

type kfEntry struct {
	Size      int64     `json:"size"`
	MTime     time.Time `json:"mtime"`
	Keyframes []float64 `json:"keyframes"`
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
	path := filepath.Join(l.CacheDir, "keyframes", it.ID+".json")
	var e kfEntry
	if b, err := os.ReadFile(path); err == nil && json.Unmarshal(b, &e) == nil &&
		e.Size == info.Size() && e.MTime.Equal(info.ModTime()) {
		return e.Keyframes, nil
	}
	start := time.Now()
	kf, err := probe.Keyframes(ctx, it.Path, v.Index)
	if err != nil {
		return nil, err
	}
	log.Printf("scan: Keyframes %s (%s)", filepath.Base(it.Path), time.Since(start).Round(time.Millisecond))
	b, _ := json.Marshal(kfEntry{Size: info.Size(), MTime: info.ModTime(), Keyframes: kf})
	os.MkdirAll(filepath.Dir(path), 0o755)
	return kf, writeAtomic(path, b)
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

// writeAtomic schreibt erst in eine Temp-Datei und benennt dann um – nie eine halbe Datei nach Absturz.
func writeAtomic(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return err
	}
	return os.Rename(f.Name(), path)
}
