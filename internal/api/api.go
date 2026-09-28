// Package api stellt Bibliothek, Wiedergabe-Pläne und Streams per HTTP bereit.
package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/flimmer-media/flimmer/internal/auth"
	"github.com/flimmer-media/flimmer/internal/db"
	"github.com/flimmer-media/flimmer/internal/ffmpeg"
	"github.com/flimmer-media/flimmer/internal/hwaccel"
	"github.com/flimmer-media/flimmer/internal/images"
	"github.com/flimmer-media/flimmer/internal/meta"
	"github.com/flimmer-media/flimmer/internal/remote"
	"github.com/flimmer-media/flimmer/internal/scan"
	"github.com/flimmer-media/flimmer/internal/transcode"
	"github.com/flimmer-media/flimmer/internal/update"
)

type Server struct {
	Lib      *scan.Library
	HLS      *transcode.Manager
	DB       *db.DB
	Meta     *meta.Resolver    // nil = keine Metadaten
	Images   *images.Store     // nil = keine Bilder
	FF       *ffmpeg.Installer // nil = kein Download möglich
	CacheDir string
	Web      fs.FS
	Pages    fs.FS           // setup.html, settings.html
	LANURL   string          // z. B. http://192.168.1.20:8096, für QR-Code und Anzeige
	QR       http.Handler    // PNG mit LANURL
	Log      *LogRing        // letzte Log-Zeilen für die Diagnose, nil = keine
	Updates  *update.Checker // nil = keine Update-Prüfung
	Remote   *remote.Remote  // nil = kein Fernzugriff möglich
	// RemoteToggle startet bzw. stoppt die Portfreigabe, wenn der Admin den Fernzugriff umschaltet.
	RemoteToggle func(on bool)
	FFmpeg       atomic.Bool                   // ffmpeg/ffprobe gefunden
	HW           atomic.Pointer[hwaccel.Accel] // gesetzt, sobald hwaccel.Detect fertig ist

	pairing auth.Pairing
	limiter auth.Limiter
	streams streams
}

func (s *Server) hw() hwaccel.Accel {
	if a := s.HW.Load(); a != nil {
		return *a
	}
	return hwaccel.Accel{}
}

func (s *Server) Handler() http.Handler {
	s.limiter = auth.Limiter{Max: 5, Window: time.Minute}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.status)

	// Anmeldung und Benutzer
	mux.HandleFunc("GET /api/users", s.users)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.logout)
	mux.HandleFunc("GET /api/me", s.me)
	mux.HandleFunc("POST /api/users", adminOnly(s.createUser))
	mux.HandleFunc("PUT /api/users/{id}", adminOnly(s.updateUser))
	mux.HandleFunc("DELETE /api/users/{id}", adminOnly(s.deleteUser))
	mux.HandleFunc("POST /api/pair", s.pairStart)
	mux.HandleFunc("GET /api/pair/{code}", s.pairPoll)
	mux.HandleFunc("POST /api/pair/{code}/confirm", s.pairConfirm)

	// Einrichtung und Einstellungen
	mux.HandleFunc("GET /api/setup", s.setupInfo)
	mux.HandleFunc("POST /api/setup", s.setup)
	mux.HandleFunc("GET /api/setup/dirs", s.dirs)
	mux.HandleFunc("GET /api/setup/count", s.count)
	mux.HandleFunc("POST /api/setup/ffmpeg", s.ffmpegDownload)
	mux.HandleFunc("GET /api/settings", adminOnly(s.settings))
	mux.HandleFunc("PUT /api/settings", adminOnly(s.saveSettings))
	mux.HandleFunc("GET /api/settings/review", adminOnly(s.review))
	mux.HandleFunc("POST /api/rescan", adminOnly(s.rescan))
	mux.HandleFunc("GET /api/diagnostics", adminOnly(s.diagnostics))
	if s.Remote != nil {
		mux.HandleFunc("GET /api/remote", adminOnly(s.Remote.StatusHandler))
		mux.HandleFunc("POST /api/remote/check", adminOnly(s.Remote.CheckHandler))
		mux.HandleFunc("POST /api/remote/pair", adminOnly(s.Remote.PairHandler))
		mux.HandleFunc("GET /api/remote/ping", s.Remote.PingHandler) // ruft das Relay ohne Anmeldung zurück
	}

	// Bibliothek und Wiedergabe
	mux.HandleFunc("POST /api/library", s.library)
	mux.HandleFunc("POST /api/home", s.home)
	mux.HandleFunc("POST /api/items/{id}/play", s.play)
	mux.HandleFunc("POST /api/items/{id}/progress", s.progress)
	mux.HandleFunc("POST /api/items/{id}/watched", s.watched)
	mux.HandleFunc("GET /api/items/{id}/file", s.file)
	mux.HandleFunc("GET /api/items/{id}/hls/{audio}/{acodec}/{vcodec}/index.m3u8", s.playlist)
	mux.HandleFunc("GET /api/items/{id}/hls/{audio}/{acodec}/{vcodec}/{seg}", s.segment)
	mux.HandleFunc("GET /api/items/{id}/subs/{index}", s.subtitle)
	mux.HandleFunc("GET /api/devices/{device}/profile", s.deviceProfile)
	mux.HandleFunc("PUT /api/devices/{device}/profile", s.deviceProfile)
	if s.Meta != nil {
		mux.Handle("GET /api/items/{id}/search", adminOnly(s.Meta.SearchHandler(s.metaLookup)))
		mux.Handle("POST /api/items/{id}/identify", adminOnly(s.Meta.IdentifyHandler(s.metaLookup, s.identified)))
	}
	if s.Images != nil {
		mux.Handle("GET /api/images/{id}/{kind}", s.Images.Handler(s.imageLookup))
	}

	// Seiten
	mux.Handle("GET /setup", s.page("setup.html"))
	mux.Handle("GET /settings", s.page("settings.html"))
	mux.Handle("GET /pages.css", s.page("pages.css"))
	if s.QR != nil {
		mux.Handle("GET /api/qr", s.QR)
	}
	mux.Handle("GET /", s.root(spa(s.Web)))
	return cors(s.authenticate(mux))
}

// root schickt auf die Einrichtung, solange es keinen Admin gibt.
func (s *Server) root(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && !s.setupDone() {
			http.Redirect(w, r, "/setup", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) page(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if name == "setup.html" && s.setupDone() {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, s.Pages, name)
	})
}

// identified übernimmt eine Korrektur aus „Falsch erkannt?“ samt neuer Platzhalterfarbe.
func (s *Server) identified(id string, m *meta.Meta) {
	s.Lib.SetMeta(id, m)
	it := s.Lib.Get(id)
	if it == nil || s.Images == nil || s.Meta == nil {
		return
	}
	if src, ok := ImageSource(it, m, s.Meta.ImgDir(), "poster"); ok {
		if c, err := s.Images.Color(context.Background(), src); err == nil {
			s.Lib.SetColor(id, c)
		}
	}
}

// CORS offen ist hier unbedenklich: Cookies sind SameSite=Lax und werden nie mit
// Access-Control-Allow-Credentials freigegeben; TV-Apps (file://-Origin) schicken ihr Bearer-Token selbst.
func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range, Authorization")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
			return
		}
		h.ServeHTTP(w, r)
	})
}

// spa liefert statische Dateien und fällt für Client-Routen auf index.html zurück.
func spa(web fs.FS) http.Handler {
	files := http.FileServerFS(web)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(web, r.URL.Path[1:]); err != nil || r.URL.Path == "/" {
			http.ServeFileFS(w, r, web, "index.html")
			return
		}
		files.ServeHTTP(w, r)
	})
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, struct {
		scan.Status
		FFmpeg bool `json:"ffmpeg"`
	}{s.Lib.Status(), s.FFmpeg.Load()})
}

func (s *Server) file(w http.ResponseWriter, r *http.Request) {
	if it := s.item(w, r); it != nil {
		s.touch(r, it)
		http.ServeFile(w, r, it.Path) // Range-Requests, If-Modified-Since usw. aus der Stdlib
	}
}

func (s *Server) job(w http.ResponseWriter, r *http.Request) (transcode.Job, bool) {
	it := s.item(w, r)
	if it == nil {
		return transcode.Job{}, false
	}
	audio, err := strconv.Atoi(r.PathValue("audio"))
	acodec, vcodec := r.PathValue("acodec"), r.PathValue("vcodec")
	vc, height, vok := transcode.ParseVideo(vcodec)
	valid := err == nil && (acodec == "copy" || acodec == "aac" || acodec == "eac3") && vok
	if !valid {
		http.Error(w, "ungültige Parameter", http.StatusBadRequest)
		return transcode.Job{}, false
	}
	if audio >= 0 && !hasStream(it, audio, "audio") {
		http.Error(w, "Tonspur existiert nicht", http.StatusBadRequest)
		return transcode.Job{}, false
	}
	kf, err := s.Lib.Keyframes(r.Context(), it)
	if err != nil {
		log.Printf("keyframes %q: %v", it.Title, err)
		http.Error(w, "Datei konnte nicht gelesen werden", http.StatusInternalServerError)
		return transcode.Job{}, false
	}
	job := transcode.Job{
		Input:      it.Path,
		Segments:   transcode.Segments(kf, it.Media.Duration),
		AudioIndex: audio,
		AudioCodec: acodec,
		VideoCodec: vc,
		Height:     height,
	}
	if vc == "h264" {
		hw := s.hw()
		job.InputArgs, job.Encoder = hw.Input, hw.Encode
	}
	return job, true
}

func (s *Server) playlist(w http.ResponseWriter, r *http.Request) {
	job, ok := s.job(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Write([]byte(transcode.Playlist(job.Segments)))
}

func (s *Server) segment(w http.ResponseWriter, r *http.Request) {
	job, ok := s.job(w, r)
	if !ok {
		return
	}
	if it := s.Lib.Get(r.PathValue("id")); it != nil {
		s.touch(r, it)
	}
	seg := r.PathValue("seg")
	n, err := strconv.Atoi(seg[:len(seg)-len(filepath.Ext(seg))])
	if err != nil || filepath.Ext(seg) != ".ts" {
		http.NotFound(w, r)
		return
	}
	path, err := s.HLS.Segment(r.Context(), job, n)
	if err != nil {
		log.Printf("segment %d: %v", n, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "video/mp2t")
	http.ServeFile(w, r, path)
}

// subtitle liefert Text-Untertitel als WebVTT und PGS roh (.sup) für das Client-Overlay – nie eingebrannt.
// Ergebnis wird gecacht, weil ffmpeg dafür die ganze Datei lesen muss.
func (s *Server) subtitle(w http.ResponseWriter, r *http.Request) {
	it := s.item(w, r)
	if it == nil {
		return
	}
	name := r.PathValue("index")
	ext := filepath.Ext(name)
	idx, err := strconv.Atoi(name[:len(name)-len(ext)])
	if err != nil || (ext != ".vtt" && ext != ".sup") || !hasStream(it, idx, "subtitle") {
		http.NotFound(w, r)
		return
	}
	out := filepath.Join(s.CacheDir, "subs", it.ID+"-"+strconv.Itoa(idx)+ext)
	if _, err := os.Stat(out); err != nil {
		os.MkdirAll(filepath.Dir(out), 0o755)
		args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-i", it.Path, "-map", "0:" + strconv.Itoa(idx)}
		if ext == ".vtt" {
			args = append(args, "-f", "webvtt")
		} else {
			args = append(args, "-c", "copy", "-f", "sup")
		}
		f, err := os.CreateTemp(filepath.Dir(out), "*"+ext) // eigene Temp-Datei je Anfrage, kein Race bei Doppelklick
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		f.Close()
		tmp := f.Name()
		if b, err := exec.CommandContext(r.Context(), "ffmpeg", append(args, "-y", tmp)...).CombinedOutput(); err != nil {
			os.Remove(tmp)
			log.Printf("untertitel %s: %v %s", name, err, b)
			http.Error(w, "Untertitel konnten nicht gelesen werden", http.StatusInternalServerError)
			return
		}
		os.Rename(tmp, out)
	}
	if ext == ".vtt" {
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	}
	http.ServeFile(w, r, out)
}

func (s *Server) rescan(w http.ResponseWriter, r *http.Request) {
	s.Lib.Rescan()
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) item(w http.ResponseWriter, r *http.Request) *scan.Item {
	it := s.Lib.Get(r.PathValue("id"))
	if it == nil {
		http.NotFound(w, r)
	}
	return it
}

func hasStream(it *scan.Item, idx int, typ string) bool {
	for _, st := range it.Media.Streams {
		if st.Index == idx && st.Type == typ {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
