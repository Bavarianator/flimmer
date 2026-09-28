// Package api stellt Bibliothek, Wiedergabe-Pläne und Streams per HTTP bereit.
package api

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/flimmer-media/flimmer/internal/playback"
	"github.com/flimmer-media/flimmer/internal/scan"
	"github.com/flimmer-media/flimmer/internal/transcode"
)

type Server struct {
	Lib      *scan.Library
	HLS      *transcode.Manager
	CacheDir string
	Web      fs.FS
	FFmpeg   bool // ffmpeg/ffprobe gefunden
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("POST /api/library", s.library)
	mux.HandleFunc("POST /api/items/{id}/play", s.play)
	mux.HandleFunc("GET /api/items/{id}/file", s.file)
	mux.HandleFunc("GET /api/items/{id}/hls/{audio}/{acodec}/{vcodec}/index.m3u8", s.playlist)
	mux.HandleFunc("GET /api/items/{id}/hls/{audio}/{acodec}/{vcodec}/{seg}", s.segment)
	mux.HandleFunc("GET /api/items/{id}/subs/{index}", s.subtitle)
	mux.HandleFunc("POST /api/rescan", s.rescan)
	mux.Handle("GET /", spa(s.Web))
	return cors(mux)
}

// ponytail: CORS offen, solange es keine Anmeldung gibt; mit Einladungs-Tokens auf bekannte Origins einschränken.
func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
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
	}{s.Lib.Status(), s.FFmpeg})
}

type libraryItem struct {
	*scan.Item
	Duration float64         `json:"duration"`
	Light    playback.Light  `json:"light"`
	Method   playback.Method `json:"method"`
}

// library gibt alle Titel mit Ampel für genau das anfragende Gerät zurück.
func (s *Server) library(w http.ResponseWriter, r *http.Request) {
	var p playback.Profile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Geräteprofil fehlt", http.StatusBadRequest)
		return
	}
	var out []libraryItem
	for _, it := range s.Lib.All() {
		plan := playback.Decide(it.Media, p)
		out = append(out, libraryItem{Item: it, Duration: it.Media.Duration, Light: plan.Light, Method: plan.Method})
	}
	writeJSON(w, out)
}

type playResponse struct {
	playback.Plan
	URL      string  `json:"url"`
	Duration float64 `json:"duration"`
	Title    string  `json:"title"`
}

func (s *Server) play(w http.ResponseWriter, r *http.Request) {
	it := s.item(w, r)
	if it == nil {
		return
	}
	var p playback.Profile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Geräteprofil fehlt", http.StatusBadRequest)
		return
	}
	plan := playback.Decide(it.Media, p)
	resp := playResponse{Plan: plan, Duration: it.Media.Duration, Title: it.Title}
	base := "/api/items/" + it.ID
	if plan.Method == playback.DirectPlay {
		resp.URL = base + "/file"
	} else {
		resp.URL = base + "/hls/" + strconv.Itoa(plan.AudioIndex) + "/" + plan.AudioCodec + "/" + plan.VideoCodec + "/index.m3u8"
	}
	log.Printf("play %q: %s (%v)", it.Title, plan.Method, plan.Reasons)
	writeJSON(w, resp)
}

func (s *Server) file(w http.ResponseWriter, r *http.Request) {
	if it := s.item(w, r); it != nil {
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
	valid := err == nil && (acodec == "copy" || acodec == "aac" || acodec == "eac3") && (vcodec == "copy" || vcodec == "h264")
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
	return transcode.Job{
		Input:      it.Path,
		Segments:   transcode.Segments(kf, it.Media.Duration),
		AudioIndex: audio,
		AudioCodec: acodec,
		VideoCodec: vcodec,
	}, true
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
