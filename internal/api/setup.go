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

	"github.com/Bavarianator/flimmer/internal/db"
	"github.com/Bavarianator/flimmer/internal/ffmpeg"
	"github.com/Bavarianator/flimmer/internal/scan"
	"github.com/Bavarianator/flimmer/internal/update"
)

func (s *Server) setupDone() bool { return s.DB.SetupDone(context.Background()) }

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
	set, err := s.DB.Settings(r.Context())
	if writeErr(w, err) {
		return
	}
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
	admin := db.User{ID: newID(), Color: 220}
	if err := applyUser(&admin, userReq{Name: &req.Name, Password: &req.Password, Admin: ptr(true)}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Atomar: Ein zweiter Tab oder ein zweites Gerät kann nicht auch Admin werden.
	err = s.DB.Setup(r.Context(), admin, func(set *db.Settings) {
		set.ServerName = strings.TrimSpace(req.ServerName)
		set.Language = cmp(req.Language, "de")
		set.Dirs = dirs
	})
	if writeErr(w, err) {
		return
	}
	s.Lib.SetDirs(dirs)
	tok, err := s.newSession(w, r, admin.ID, "Einrichtung")
	if !writeErr(w, err) {
		writeJSON(w, withToken{toPublic(admin), tok})
	}
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	set, err := s.DB.Settings(r.Context())
	if writeErr(w, err) {
		return
	}
	writeJSON(w, map[string]any{"serverName": set.ServerName, "language": cmp(set.Language, "de"), "dirs": nonNil(set.Dirs),
		"tmdbKey": set.TMDBKey != "", "suggestions": suggestions(), "ffmpeg": s.ffmpegInfo(), "lanUrl": s.LANURL,
		"updateCheck": !set.NoUpdates, "remote": set.Remote, "optimize": set.Optimize, "remoteAvailable": s.Remote != nil, "update": s.availableUpdate(), "version": update.Version})
}

func (s *Server) availableUpdate() *update.Release {
	if s.Updates == nil {
		return nil
	}
	return s.Updates.Available()
}

func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerName *string      `json:"serverName"`
		Language   *string      `json:"language"`
		Dirs       []string     `json:"dirs"`
		TMDBKey    *string      `json:"tmdbKey"` // "" entfernt den eigenen Key
		Update     *bool        `json:"updateCheck"`
		Remote     *bool        `json:"remote"`
		Optimize   *db.Optimize `json:"optimize"`
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
	err := s.DB.UpdateSettings(r.Context(), func(set *db.Settings) {
		if req.ServerName != nil {
			set.ServerName = strings.TrimSpace(*req.ServerName)
		}
		if req.Language != nil {
			set.Language = *req.Language
		}
		if req.TMDBKey != nil {
			set.TMDBKey = strings.TrimSpace(*req.TMDBKey)
		}
		if req.Dirs != nil {
			set.Dirs = dirs
		}
		if req.Update != nil {
			set.NoUpdates = !*req.Update
		}
		if req.Remote != nil {
			set.Remote = *req.Remote
		}
		if o := req.Optimize; o != nil {
			o.From, o.To, o.MinFreeGB = min(max(o.From, 0), 23), min(max(o.To, 0), 23), max(o.MinFreeGB, 0)
			set.Optimize = *o
		}
	})
	if writeErr(w, err) {
		return
	}
	if req.Dirs != nil {
		s.Lib.SetDirs(dirs)
	}
	if req.Remote != nil && s.RemoteToggle != nil {
		s.RemoteToggle(*req.Remote)
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
