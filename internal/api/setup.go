package api

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/flimmer-media/flimmer/internal/ffmpeg"
	"github.com/flimmer-media/flimmer/internal/scan"
	"github.com/flimmer-media/flimmer/internal/state"
	"github.com/flimmer-media/flimmer/internal/update"
)

func (s *Server) setupDone() (done bool) {
	s.State.View(func(d *state.Data) { done = d.SetupDone() })
	return done
}

// setupAllowed: Ordner durchsuchen und ffmpeg laden darf jeder, solange es keinen Admin gibt – danach nur Admins.
func (s *Server) setupAllowed(w http.ResponseWriter, r *http.Request) bool {
	if !s.setupDone() {
		return true
	}
	return requireAdmin(w, r)
}

type ffmpegInfo struct {
	OK          bool          `json:"ok"`
	CanDownload bool          `json:"canDownload"`
	Hint        string        `json:"hint"`
	Download    ffmpeg.Status `json:"download"`
}

func (s *Server) ffmpegInfo() ffmpegInfo {
	i := ffmpegInfo{OK: s.FFmpeg.Load(), CanDownload: ffmpeg.CanDownload() && s.FF != nil, Hint: ffmpeg.Hint()}
	if s.FF != nil {
		i.Download = s.FF.Status()
	}
	return i
}

func (s *Server) setupInfo(w http.ResponseWriter, r *http.Request) {
	if s.setupDone() {
		writeJSON(w, map[string]bool{"done": true})
		return
	}
	var set state.Settings
	s.State.View(func(d *state.Data) { set = d.Settings })
	name := set.ServerName
	if name == "" {
		name, _ = os.Hostname()
	}
	writeJSON(w, map[string]any{"done": false, "serverName": name, "language": cmp(set.Language, "de"),
		"dirs": nonNil(set.Dirs), "suggestions": suggestions(), "ffmpeg": s.ffmpegInfo()})
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerName string   `json:"serverName"`
		Language   string   `json:"language"`
		Name       string   `json:"name"`
		Password   string   `json:"password"`
		Dirs       []string `json:"dirs"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	dirs, err := cleanDirs(req.Dirs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	admin := state.User{ID: state.NewID(), Color: 220}
	if err := applyUser(&admin, userReq{Name: &req.Name, Password: &req.Password, Admin: ptr(true)}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = s.State.Update(func(d *state.Data) error {
		if d.SetupDone() { // zweiter Tab oder zweites Gerät war schneller
			return errMsg("Flimmer ist bereits eingerichtet")
		}
		d.Users = append(d.Users, admin)
		d.Settings.ServerName = strings.TrimSpace(req.ServerName)
		d.Settings.Language = cmp(req.Language, "de")
		d.Settings.Dirs = dirs
		return nil
	})
	if writeErr(w, err) {
		return
	}
	if err := s.State.Flush(); err != nil {
		http.Error(w, "Einstellungen konnten nicht gespeichert werden: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.Lib.SetDirs(dirs)
	writeJSON(w, withToken{toPublic(admin), s.newSession(w, r, admin.ID, "Einrichtung")})
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	var set state.Settings
	s.State.View(func(d *state.Data) { set = d.Settings })
	writeJSON(w, map[string]any{"serverName": set.ServerName, "language": cmp(set.Language, "de"), "dirs": nonNil(set.Dirs),
		"tmdbKey": set.TMDBKey != "", "suggestions": suggestions(), "ffmpeg": s.ffmpegInfo(), "lanUrl": s.LANURL,
		"updateCheck": !set.NoUpdates, "update": s.availableUpdate(), "version": update.Version})
}

func (s *Server) availableUpdate() *update.Release {
	if s.Updates == nil {
		return nil
	}
	return s.Updates.Available()
}

func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerName *string  `json:"serverName"`
		Language   *string  `json:"language"`
		Dirs       []string `json:"dirs"`
		TMDBKey    *string  `json:"tmdbKey"` // "" entfernt den eigenen Key
		Update     *bool    `json:"updateCheck"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	var dirs []string
	if req.Dirs != nil {
		var err error
		if dirs, err = cleanDirs(req.Dirs); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	s.State.Update(func(d *state.Data) error {
		if req.ServerName != nil {
			d.Settings.ServerName = strings.TrimSpace(*req.ServerName)
		}
		if req.Language != nil {
			d.Settings.Language = *req.Language
		}
		if req.TMDBKey != nil {
			d.Settings.TMDBKey = strings.TrimSpace(*req.TMDBKey)
		}
		if req.Dirs != nil {
			d.Settings.Dirs = dirs
		}
		if req.Update != nil {
			d.Settings.NoUpdates = !*req.Update
		}
		return nil
	})
	s.State.Flush()
	if req.Dirs != nil {
		s.Lib.SetDirs(dirs)
	}
	s.settings(w, r)
}

func cleanDirs(in []string) ([]string, error) {
	var out []string
	for _, d := range in {
		d = filepath.Clean(strings.TrimSpace(d))
		if d == "." || slices.Contains(out, d) {
			continue
		}
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() || !filepath.IsAbs(d) {
			return nil, errMsg("Ordner nicht gefunden: " + d)
		}
		out = append(out, d)
	}
	return nonNil(out), nil
}

type dirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// dirs: Ordner-Browser für die Einrichtung. Ohne path gibt es Vorschläge und Laufwerke.
func (s *Server) dirs(w http.ResponseWriter, r *http.Request) {
	if !s.setupAllowed(w, r) {
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeJSON(w, map[string]any{"path": "", "parent": "", "dirs": suggestions()})
		return
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		http.Error(w, "Pfad muss absolut sein", http.StatusBadRequest)
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		http.Error(w, "Ordner kann nicht gelesen werden", http.StatusNotFound)
		return
	}
	out := []dirEntry{}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") || strings.HasPrefix(e.Name(), "$") {
			continue
		}
		full := filepath.Join(path, e.Name())
		if fi, err := os.Stat(full); err == nil && fi.IsDir() { // Stat folgt Symlinks (z. B. NAS-Mounts)
			out = append(out, dirEntry{e.Name(), full})
		}
		if len(out) == 1000 {
			break
		}
	}
	slices.SortFunc(out, func(a, b dirEntry) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	parent := filepath.Dir(path)
	if parent == path {
		parent = "" // Wurzel → zurück zu den Vorschlägen
	}
	writeJSON(w, map[string]any{"path": path, "parent": parent, "dirs": out})
}

// count zählt Videos unter path, höchstens 1,5 s lang – für „123 Videos gefunden“ im Assistenten.
func (s *Server) count(w http.ResponseWriter, r *http.Request) {
	if !s.setupAllowed(w, r) {
		return
	}
	root := filepath.Clean(r.URL.Query().Get("path"))
	if !filepath.IsAbs(root) {
		http.Error(w, "Pfad muss absolut sein", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
	defer cancel()
	n := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && scan.IsVideo(p) {
			n++
		}
		return ctx.Err()
	})
	writeJSON(w, map[string]any{"videos": n, "complete": err == nil})
}

func (s *Server) ffmpegDownload(w http.ResponseWriter, r *http.Request) {
	if !s.setupAllowed(w, r) {
		return
	}
	if s.FF == nil || !ffmpeg.CanDownload() {
		http.Error(w, ffmpeg.Hint(), http.StatusNotImplemented)
		return
	}
	s.FF.Start(context.WithoutCancel(r.Context()))
	writeJSON(w, s.ffmpegInfo())
}

// suggestions: typische Orte für Filme – nur, was es auf diesem Rechner gibt.
func suggestions() []dirEntry {
	var cands []string
	if runtime.GOOS == "windows" {
		for c := 'C'; c <= 'Z'; c++ {
			cands = append(cands, string(c)+`:\`)
		}
	} else {
		cands = append(cands, "/media", "/mnt", "/srv", "/data", "/volume1", "/Volumes")
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, sub := range []string{"Videos", "Movies", "Filme", "Serien", "Downloads", ""} {
			cands = append(cands, filepath.Join(home, sub))
		}
	}
	out := []dirEntry{}
	for _, c := range cands {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			out = append(out, dirEntry{c, c})
		}
	}
	return out
}

func cmp(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func ptr[T any](v T) *T { return &v }
