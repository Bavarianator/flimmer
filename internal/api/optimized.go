package api

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/Bavarianator/flimmer/internal/optimize"
	"github.com/Bavarianator/flimmer/internal/playback"
	"github.com/Bavarianator/flimmer/internal/probe"
	"github.com/Bavarianator/flimmer/internal/scan"
)

// Optimierte Versionen (internal/optimize) liegen neben dem Cache, nie im Medienordner. Die Wiedergabe nimmt sie,
// wenn playback.Better sie für das Gerät besser findet; ihre URLs tragen /o/ (…/items/{id}/o/file bzw. …/o/hls/…),
// relative HLS-Segmente erben das.

// altCache merkt sich Probe-Ergebnis und Keyframes optimierter Dateien (Schlüssel: Pfad + mtime).
type altCache struct {
	mu sync.Mutex
	m  map[string]altInfo
}

type altInfo struct {
	media *probe.Media
	kf    []float64
}

func (c *altCache) get(ctx context.Context, path string, keyframes bool) (altInfo, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return altInfo{}, err
	}
	key := path + "|" + fi.ModTime().String()
	c.mu.Lock()
	info, ok := c.m[key]
	c.mu.Unlock()
	if !ok {
		if info.media, err = probe.File(ctx, path); err != nil {
			return altInfo{}, err
		}
	}
	if keyframes && info.kf == nil {
		if v := info.media.First("video"); v != nil {
			if info.kf, err = probe.Keyframes(ctx, path, v.Index); err != nil {
				return altInfo{}, err
			}
		}
	}
	c.mu.Lock()
	if c.m == nil {
		c.m = map[string]altInfo{}
	}
	c.m[key] = info
	c.mu.Unlock()
	return info, nil
}

// optimizedPlan: Gibt es eine optimierte Version, die für dieses Gerät besser ist, liefert es deren Plan.
func (s *Server) optimizedPlan(ctx context.Context, it *scan.Item, p playback.Profile, plan playback.Plan) (playback.Plan, bool) {
	o := s.Optimizer.Load()
	if o == nil {
		return plan, false
	}
	path := o.Lookup(it.ID, it.Path)
	if path == "" {
		return plan, false
	}
	info, err := s.alt.get(ctx, path, false)
	if err != nil {
		return plan, false
	}
	alt := playback.Decide(info.media, p, s.hw().Speed)
	if !playback.Better(alt, plan) {
		return plan, false
	}
	alt.Subtitles = plan.Subtitles // Untertitel kommen weiter aus dem Original
	return alt, true
}

// source liefert Pfad und Medieninfo für file/hls: das Original oder – bei …/o/… – die optimierte Version.
func (s *Server) source(w http.ResponseWriter, r *http.Request, it *scan.Item, keyframes bool) (string, *probe.Media, []float64, bool) {
	if !strings.Contains(r.URL.Path, "/"+it.ID+"/o/") {
		if !keyframes {
			return it.Path, it.Media, nil, true
		}
		kf, err := s.Lib.Keyframes(r.Context(), it)
		if err != nil {
			http.Error(w, "Datei konnte nicht gelesen werden", http.StatusInternalServerError)
			return "", nil, nil, false
		}
		return it.Path, it.Media, kf, true
	}
	var path string
	if o := s.Optimizer.Load(); o != nil {
		path = o.Lookup(it.ID, it.Path)
	}
	if path == "" {
		http.Error(w, "optimierte Version gibt es nicht mehr – bitte neu starten", http.StatusNotFound)
		return "", nil, nil, false
	}
	info, err := s.alt.get(r.Context(), path, keyframes)
	if err != nil {
		http.Error(w, "Datei konnte nicht gelesen werden", http.StatusInternalServerError)
		return "", nil, nil, false
	}
	return path, info.media, info.kf, true
}

func (s *Server) optimizeStatus(w http.ResponseWriter, r *http.Request) {
	o := s.Optimizer.Load()
	if o == nil {
		writeJSON(w, map[string]any{"waiting": true})
		return
	}
	o.StatusHandler(w, r)
}

// optimizeItems: Katalog für die Hintergrund-Optimierung, neueste zuerst.
func (s *Server) OptimizeItems(ctx context.Context) ([]optimize.Item, error) {
	var out []optimize.Item
	for _, it := range s.Lib.Newest() {
		out = append(out, optimize.Item{ID: it.ID, Title: it.Title, Path: it.Path, Media: it.Media})
	}
	return out, nil
}

// Busy: Läuft gerade eine Wiedergabe? Dann wartet die Optimierung.
func (s *Server) Busy() bool {
	active, _ := s.streams.list()
	return len(active) > 0
}
