// Package scan durchsucht Medienordner, erkennt Filme/Episoden am Dateinamen und probt jede Datei (mit Cache).
package scan

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
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

// Library hält alle Einträge im Speicher; Probe-Ergebnisse landen in einer JSON-Cache-Datei.
// ponytail: JSON-Cache statt DB; SQLite kommt mit Benutzern/Fortschritt.
type Library struct {
	Dirs      []string
	CacheFile string

	mu    sync.RWMutex
	items map[string]*Item
}

type cacheEntry struct {
	Size  int64        `json:"size"`
	MTime time.Time    `json:"mtime"`
	Media *probe.Media `json:"media"`
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

func (l *Library) Scan(ctx context.Context) error {
	cache := map[string]cacheEntry{}
	if b, err := os.ReadFile(l.CacheFile); err == nil {
		_ = json.Unmarshal(b, &cache)
	}
	items := map[string]*Item{}
	newCache := map[string]cacheEntry{}
	for _, dir := range l.Dirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("scan: %v", err)
				return nil
			}
			if d.IsDir() || !slices.Contains(videoExt, strings.ToLower(filepath.Ext(path))) {
				return nil
			}
			info, err := os.Stat(path) // folgt Symlinks, anders als d.Info()
			if err != nil {
				return nil
			}
			c, ok := cache[path]
			if !ok || c.Size != info.Size() || !c.MTime.Equal(info.ModTime()) {
				start := time.Now()
				m, err := probe.File(ctx, path)
				if err != nil {
					log.Printf("scan: %v", err)
					return nil
				}
				log.Printf("scan: %s geprüft (%s)", filepath.Base(path), time.Since(start).Round(time.Millisecond))
				c = cacheEntry{Size: info.Size(), MTime: info.ModTime(), Media: m}
			}
			newCache[path] = c
			it := Parse(path)
			it.ID, it.Path, it.Size, it.Media = ID(path), path, info.Size(), c.Media
			items[it.ID] = &it
			return ctx.Err()
		})
		if err != nil {
			return err
		}
	}
	l.mu.Lock()
	l.items = items
	l.mu.Unlock()
	b, err := json.Marshal(newCache)
	if err != nil {
		return err
	}
	return os.WriteFile(l.CacheFile, b, 0o644)
}
