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
	"strings"
	"sync/atomic"
	"time"

	"github.com/Bavarianator/flimmer/internal/auth"
	"github.com/Bavarianator/flimmer/internal/db"
	"github.com/Bavarianator/flimmer/internal/ffmpeg"
	"github.com/Bavarianator/flimmer/internal/hwaccel"
	"github.com/Bavarianator/flimmer/internal/images"
	"github.com/Bavarianator/flimmer/internal/livetv"
	"github.com/Bavarianator/flimmer/internal/meta"
	"github.com/Bavarianator/flimmer/internal/optimize"
	"github.com/Bavarianator/flimmer/internal/party"
	"github.com/Bavarianator/flimmer/internal/probe"
	"github.com/Bavarianator/flimmer/internal/remote"
	"github.com/Bavarianator/flimmer/internal/scan"
	"github.com/Bavarianator/flimmer/internal/share"
	"github.com/Bavarianator/flimmer/internal/transcode"
	"github.com/Bavarianator/flimmer/internal/update"
	"github.com/Bavarianator/flimmer/internal/vpn"
)

type Server struct {
	Lib       *scan.Library
	HLS       *transcode.Manager
	DB        *db.DB
	Meta      *meta.Resolver    // nil = keine Metadaten
	Images    *images.Store     // nil = keine Bilder
	FF        *ffmpeg.Installer // nil = kein Download möglich
	CacheDir  string
	UploadDir string // Ziel von POST /api/upload (unter den Daten, nie in den Medienordnern)
	Web       fs.FS
	Pages     fs.FS           // setup.html, settings.html
	LANURL    string          // z. B. http://192.168.1.20:8096, für QR-Code und Anzeige
	QR        http.Handler    // PNG mit LANURL
	Log       *LogRing        // letzte Log-Zeilen für die Diagnose, nil = keine
	Updates   *update.Checker // nil = keine Update-Prüfung
	Remote    *remote.Remote  // nil = kein Fernzugriff möglich
	// RemoteToggle startet bzw. stoppt die Portfreigabe, wenn der Admin den Fernzugriff umschaltet.
	RemoteToggle func(on bool)
	FFmpeg       atomic.Bool                   // ffmpeg/ffprobe gefunden
	HW           atomic.Pointer[hwaccel.Accel] // gesetzt, sobald hwaccel.Detect fertig ist

	Optimizer atomic.Pointer[optimize.Optimizer] // gesetzt, sobald die Hardware gemessen ist
	Share     *share.Share                       // Einladungen; nil = aus
	BackupDir string                             // Ziel der Sicherungen (wie db.Nightly); leer = keine Aufgabe „Sicherung“
	LiveTV    *livetv.TV                         // nil = kein Live-TV
	Port      int                                // HTTP-Port, für die VPN-Adressen unter /api/vpn
	party     *party.Hub

	alt      altCache
	pairing  auth.Pairing
	limiter  auth.Limiter
	streams  streams
	extras   extraCache
	activity activityLog
	running  running
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
	mux.HandleFunc("POST /api/users", adminOnly(s.logged("user", "Benutzer angelegt", s.createUser)))
	mux.HandleFunc("PUT /api/users/{id}", adminOnly(s.logged("user", "Benutzer geändert", s.updateUser)))
	mux.HandleFunc("DELETE /api/users/{id}", adminOnly(s.logged("user", "Benutzer gelöscht", s.deleteUser)))
	mux.HandleFunc("POST /api/pair", s.pairStart)
	mux.HandleFunc("GET /api/pair/{code}", s.pairPoll)
	mux.HandleFunc("POST /api/pair/{code}/confirm", s.pairConfirm)
	mux.HandleFunc("GET /api/pair/{code}/qr", s.pairQR)

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

	// Gemeinsam schauen: Uhrenabgleich, Räume, SSE; jeder holt seinen Plan über die normale play-Route.
	s.party = &party.Hub{
		User: func(r *http.Request) (string, string, bool) {
			u := userFrom(r)
			if u == nil {
				return "", "", false
			}
			return u.ID, u.Name, true
		},
		Allowed: func(r *http.Request, mediaID string) bool {
			it := s.Lib.Get(mediaID)
			return it != nil && s.allowed(r, it)
		},
		EventsURL: func(r *http.Request, id string) string {
			return "/api/m/" + auth.MediaToken(s.secret(), userFrom(r).ID, mediaTTL) + "/party/" + id + "/events"
		},
	}
	mux.HandleFunc("GET /api/time", party.Time)
	mux.HandleFunc("POST /api/party", s.party.Create)
	mux.HandleFunc("GET /api/party", func(w http.ResponseWriter, r *http.Request) {
		if u := userFrom(r); u == nil || s.isGuest(r.Context(), u.ID) { // Gäste kommen nur per Einladungslink in Gruppen
			writeJSON(w, []any{})
			return
		}
		s.party.List(w, r)
	})
	mux.HandleFunc("POST /api/upload", s.upload)
	mux.HandleFunc("POST /api/upload/link", s.linkStart)
	mux.HandleFunc("GET /api/upload/link", s.linkListe)
	mux.HandleFunc("GET /api/party/{id}", s.party.Get)
	mux.HandleFunc("GET /api/party/{id}/events", s.party.Events)
	mux.HandleFunc("POST /api/party/{id}/actions", s.party.Action)
	if s.Share != nil {
		mux.HandleFunc("POST /api/invites", adminOnly(s.logged("invite", "Einladung erstellt", s.Share.CreateHandler)))
		mux.HandleFunc("GET /api/invites", adminOnly(s.Share.ListHandler))
		mux.HandleFunc("DELETE /api/invites/{id}", adminOnly(s.logged("invite", "Einladung zurückgenommen", s.Share.RevokeHandler)))
		mux.HandleFunc("POST /api/invites/redeem", s.logged("invite", "Einladung eingelöst", s.Share.RedeemHandler)) // ohne Anmeldung, eigenes Rate-Limit
	}
	mux.HandleFunc("GET /api/settings/optimize", adminOnly(s.optimizeStatus))
	mux.HandleFunc("GET /api/settings/backup", adminOnly(s.logged("backup", "Sicherung heruntergeladen", s.backupDownload)))
	mux.HandleFunc("POST /api/settings/restore", adminOnly(s.logged("backup", "Sicherung eingespielt", s.restore)))

	// Dashboard
	mux.HandleFunc("GET /api/admin/overview", adminOnly(s.overview))
	mux.HandleFunc("GET /api/sessions", adminOnly(s.sessions))
	mux.HandleFunc("GET /api/activity", adminOnly(s.activities))
	mux.HandleFunc("GET /api/devices", adminOnly(s.devices))
	mux.HandleFunc("DELETE /api/devices/{id}", adminOnly(s.deleteDevice))
	mux.HandleFunc("GET /api/tasks", adminOnly(s.taskList))
	mux.HandleFunc("POST /api/tasks/{id}/run", adminOnly(s.runTask))
	mux.HandleFunc("GET /api/logs", adminOnly(s.logs))
	// Alle festen Nutzer (keine Gäste): die Android-App merkt sich die Adresse für unterwegs.
	vpnH := vpn.Handler(s.Port)
	mux.HandleFunc("GET /api/vpn", func(w http.ResponseWriter, r *http.Request) {
		if u := userFrom(r); u == nil || s.isGuest(r.Context(), u.ID) {
			http.Error(w, "nicht erlaubt", http.StatusForbidden)
			return
		}
		vpnH.ServeHTTP(w, r)
	})

	// Live-TV (docs/livetv-vpn.md): Gäste nie; der Player holt HLS mit Medien-Token im Pfad.
	if tv := s.LiveTV; tv != nil {
		tv.Allow = func(r *http.Request) bool { u := userFrom(r); return u != nil && !s.isGuest(r.Context(), u.ID) }
		tv.URL = func(r *http.Request, id string) string {
			return "/api/m/" + auth.MediaToken(s.secret(), userFrom(r).ID, mediaTTL) + "/livetv/channels/" + id + "/index.m3u8"
		}
		mux.HandleFunc("GET /api/livetv", adminOnly(tv.StatusHandler))
		mux.HandleFunc("PUT /api/livetv", adminOnly(tv.ConfigHandler))
		mux.HandleFunc("POST /api/livetv/refresh", adminOnly(tv.RefreshHandler))
		mux.HandleFunc("GET /api/livetv/vorlagen", adminOnly(tv.PresetsHandler))
		mux.HandleFunc("GET /api/livetv/channels", tv.ChannelsHandler)
		mux.HandleFunc("GET /api/livetv/guide", tv.GuideHandler)
		mux.HandleFunc("POST /api/livetv/channels/{id}/play", tv.PlayHandler)
		mux.HandleFunc("GET /api/livetv/channels/{id}/{file}", tv.FileHandler)
	}
	if s.Remote != nil {
		mux.HandleFunc("GET /api/remote", adminOnly(s.Remote.StatusHandler))
		mux.HandleFunc("POST /api/remote/check", adminOnly(s.Remote.CheckHandler))
		mux.HandleFunc("POST /api/remote/pair", adminOnly(s.Remote.PairHandler))
		mux.HandleFunc("GET /api/remote/ping", s.Remote.PingHandler) // ruft das Relay ohne Anmeldung zurück
	}

	// Bibliothek und Wiedergabe
	mux.HandleFunc("POST /api/library", s.library)
	mux.HandleFunc("POST /api/home", s.home)
	mux.HandleFunc("GET /api/search", s.search)
	mux.HandleFunc("GET /api/favorites", s.favorites)
	mux.HandleFunc("PUT /api/favorites/{key}", s.setFavorite)
	mux.HandleFunc("DELETE /api/favorites/{key}", s.setFavorite)
	mux.HandleFunc("GET /api/collections", s.collections)
	mux.HandleFunc("POST /api/collections", adminOnly(s.createList(db.Collection)))
	mux.HandleFunc("PUT /api/collections/{id}", adminOnly(s.updateList(db.Collection)))
	mux.HandleFunc("DELETE /api/collections/{id}", adminOnly(s.deleteList(db.Collection)))
	mux.HandleFunc("GET /api/playlists", s.playlists)
	mux.HandleFunc("POST /api/playlists", s.createList(db.Playlist))
	mux.HandleFunc("PUT /api/playlists/{id}", s.updateList(db.Playlist))
	mux.HandleFunc("DELETE /api/playlists/{id}", s.deleteList(db.Playlist))
	mux.HandleFunc("GET /api/items/{id}", s.itemDetails)
	mux.HandleFunc("GET /api/series/{name}", s.series)
	mux.HandleFunc("GET /api/people/{name}", s.person)
	mux.HandleFunc("POST /api/items/{id}/play", s.play)
	mux.HandleFunc("POST /api/items/{id}/progress", s.progress)
	mux.HandleFunc("POST /api/items/{id}/watched", s.watched)
	for _, v := range []string{"", "/o"} { // /o = optimierte Version
		mux.HandleFunc("GET /api/items/{id}"+v+"/file", s.file)
		mux.HandleFunc("GET /api/items/{id}"+v+"/hls/{audio}/{acodec}/{vcodec}/index.m3u8", s.playlist)
		mux.HandleFunc("GET /api/items/{id}"+v+"/hls/{audio}/{acodec}/{vcodec}/{seg}", s.segment)
	}
	mux.HandleFunc("GET /api/items/{id}/subs/{index}", s.subtitle)
	mux.HandleFunc("GET /api/devices/{device}/profile", s.deviceProfile)
	mux.HandleFunc("PUT /api/devices/{device}/profile", s.deviceProfile)
	if s.Meta != nil {
		mux.Handle("GET /api/items/{id}/search", adminOnly(s.Meta.SearchHandler(s.metaLookup)))
		mux.Handle("POST /api/items/{id}/identify", adminOnly(s.Meta.IdentifyHandler(s.metaLookup, s.identified)))
		mux.HandleFunc("PUT /api/items/{id}/meta", adminOnly(s.editMeta))
	}
	if s.Images != nil {
		mux.Handle("GET /api/images/{id}/{kind}", s.Images.Handler(s.imageLookup))
		mux.HandleFunc("GET /api/images/{id}/frame", s.frameImage)
		if s.Meta != nil {
			mux.HandleFunc("GET /api/images/person/{name}/profile", s.personImage)
		}
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

// BaseURL ist die Adresse für Links und QR-Codes: die öffentliche, wenn der Fernzugriff von außen bestätigt ist
// (public=true), sonst die LAN-Adresse.
func (s *Server) BaseURL() (string, bool) {
	if s.Remote != nil {
		if st := s.Remote.Status(); st.Reachable && st.PublicURL != "" {
			return st.PublicURL, true
		}
	}
	return s.LANURL, false
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

// spa liefert statische Dateien und fällt für Client-Routen auf index.html zurück. Unbekannte API-Pfade sind 404.
func spa(web fs.FS) http.Handler {
	files := http.FileServerFS(web)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
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
		if path, _, _, ok := s.source(w, r, it, false); ok {
			http.ServeFile(w, r, path) // Range-Requests, If-Modified-Since usw. aus der Stdlib
		}
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
	ac, night, aok := transcode.ParseAudio(acodec) // z. B. "aac-night" im Nachtmodus
	valid := err == nil && aok && vok
	if !valid {
		http.Error(w, "ungültige Parameter", http.StatusBadRequest)
		return transcode.Job{}, false
	}
	path, media, kf, ok := s.source(w, r, it, true)
	if !ok {
		return transcode.Job{}, false
	}
	if audio >= 0 && !hasStream(media, audio, "audio") {
		http.Error(w, "Tonspur existiert nicht", http.StatusBadRequest)
		return transcode.Job{}, false
	}
	job := transcode.Job{
		Input:         path,
		Segments:      transcode.Segments(kf, media.Duration),
		AudioIndex:    audio,
		AudioCodec:    ac,
		Night:         night,
		AudioChannels: channels(media, audio),
		VideoCodec:    vc,
		Height:        height,
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
	if err != nil || (ext != ".vtt" && ext != ".sup") || !hasStream(it.Media, idx, "subtitle") {
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

// item liefert den Titel der Anfrage – für Gäste nur, wenn ihre Einladung ihn erlaubt (sonst 404).
// Alle Titel-Routen (play, file, hls, subs, progress, watched) gehen hier durch.
func (s *Server) item(w http.ResponseWriter, r *http.Request) *scan.Item {
	it := s.Lib.Get(r.PathValue("id"))
	if it == nil || !s.allowed(r, it) {
		http.NotFound(w, r)
		return nil
	}
	return it
}

// channels liefert die Kanalzahl der Tonspur idx (0, wenn unbekannt) – für den Stereo-Downmix im Nachtmodus.
func channels(m *probe.Media, idx int) int {
	for _, st := range m.Streams {
		if st.Index == idx {
			return st.Channels
		}
	}
	return 0
}

func hasStream(m *probe.Media, idx int, typ string) bool {
	for _, st := range m.Streams {
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
