// Package update fragt einmal täglich GitHub, ob es eine neuere Version gibt. Nur ein Hinweis mit Link –
// kein Selbst-Update (Sicherheitsrisiko, und in Docker ohnehin falsch).
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Version wird beim Release-Build gesetzt: -ldflags "-X github.com/flimmer-media/flimmer/internal/update.Version=v0.1.0"
var Version = "dev"

// API ist in Tests austauschbar.
var API = "https://api.github.com/repos/flimmer-media/flimmer/releases/latest"

type Release struct {
	Version   string    `json:"version"`
	URL       string    `json:"url"`
	Published time.Time `json:"published"`
}

// Latest holt das neueste veröffentlichte Release (Entwürfe und Vorabversionen zählen nicht, das filtert GitHub).
func Latest(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", API, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "flimmer/"+Version)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // noch kein Release
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: %s", resp.Status)
	}
	var r struct {
		TagName     string    `json:"tag_name"`
		HTMLURL     string    `json:"html_url"`
		PublishedAt time.Time `json:"published_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &Release{Version: r.TagName, URL: r.HTMLURL, Published: r.PublishedAt}, nil
}

// Newer meldet, ob latest eine höhere Version als current ist ("v1.2.3", "1.2", Suffixe wie "-rc1" werden ignoriert).
func Newer(latest, current string) bool {
	l, c := parse(latest), parse(current)
	if l == nil || c == nil {
		return false
	}
	for i := range 3 {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

func parse(v string) []int {
	v = strings.TrimPrefix(v, "v")
	v, _, _ = strings.Cut(v, "-")
	parts := strings.Split(v, ".")
	if len(parts) > 3 {
		return nil
	}
	out := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out[i] = n
	}
	return out
}

// Checker hält das Ergebnis der letzten Prüfung für die Einstellungsseite.
type Checker struct {
	mu        sync.Mutex
	available *Release
}

// Run prüft kurz nach dem Start und dann alle 24 h, solange enabled() true liefert. Entwicklerbuilds ("dev") prüfen nie.
func (c *Checker) Run(ctx context.Context, enabled func() bool) {
	if Version == "dev" {
		return
	}
	t := time.NewTimer(time.Minute) // nicht beim Start mit Scan und Probe konkurrieren
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if enabled() {
			c.check(ctx)
		}
		t.Reset(24 * time.Hour)
	}
}

func (c *Checker) check(ctx context.Context) {
	r, err := Latest(ctx)
	if err != nil {
		log.Printf("update: %v", err) // offline ist normal, nur loggen
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.available = nil
	if r != nil && Newer(r.Version, Version) {
		c.available = r
		log.Printf("update: Flimmer %s ist verfügbar: %s", r.Version, r.URL)
	}
}

// Available liefert das neuere Release oder nil.
func (c *Checker) Available() *Release {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.available
}
