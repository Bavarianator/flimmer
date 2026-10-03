package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Bavarianator/flimmer/internal/auth"
	"github.com/Bavarianator/flimmer/internal/db"
	"github.com/Bavarianator/flimmer/internal/playback"
	"github.com/Bavarianator/flimmer/internal/scan"
	"github.com/Bavarianator/flimmer/internal/update"
)

// Dashboard (nur Admin): Übersicht, Sitzungen, Aktivitäten, Geräte, Aufgaben, Protokoll.

var started = time.Now()

// --- Aktivitäten ---

type Activity struct {
	Time time.Time `json:"time"`
	Kind string    `json:"kind"` // login, play, scan, error, invite, backup, task, user
	User string    `json:"user,omitempty"`
	Text string    `json:"text"`
}

// activityLog ist ein Ringpuffer im Speicher; nach einem Neustart ist er leer.
type activityLog struct {
	mu   sync.Mutex
	list []Activity
}

const activityMax = 500

func (a *activityLog) add(kind, user, text string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.list = append(a.list, Activity{Time: time.Now(), Kind: kind, User: user, Text: text})
	if len(a.list) > activityMax+activityMax/4 {
		a.list = append([]Activity(nil), a.list[len(a.list)-activityMax:]...)
	}
}

// last liefert die letzten n, neueste zuerst.
func (a *activityLog) last(n int) []Activity {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := []Activity{}
	for i := len(a.list) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, a.list[i])
	}
	return out
}

// Activity trägt ein Ereignis ein (auch für main, z. B. Scans aus scan.Library.OnScan).
func (s *Server) Activity(kind, user, text string) { s.activity.add(kind, user, text) }

// ScanDone ist der Callback für scan.Library.OnScan.
func (s *Server) ScanDone(found int, err error) {
	if err != nil {
		s.activity.add("error", "", "Scan fehlgeschlagen: "+err.Error())
		return
	}
	s.activity.add("scan", "", "Bibliothek gescannt: "+strconv.Itoa(found)+" Titel")
}

func (s *Server) activities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.activity.last(queryLimit(r, 50, activityMax)))
}

func queryLimit(r *http.Request, def, max int) int {
	n, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || n <= 0 {
		return def
	}
	return min(n, max)
}

// logged trägt nach erfolgreicher Antwort (2xx) eine Aktivität ein, z. B. für Handler aus anderen Paketen.
func (s *Server) logged(kind, text string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		h(sw, r)
		if sw.code < 300 {
			user := ""
			if u := userFrom(r); u != nil {
				user = u.Name
			}
			s.activity.add(kind, user, text)
		}
	}
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// --- Sitzungen (aus play und den progress-Herzschlägen, siehe streams in diag.go) ---

type sessionOut struct {
	ID        string          `json:"id"`
	User      string          `json:"user"`
	UserColor int             `json:"userColor"`
	Device    string          `json:"device"`
	Client    string          `json:"client"`
	ItemID    string          `json:"itemId"`
	Title     string          `json:"title"`
	Position  float64         `json:"position"`
	Duration  float64         `json:"duration"`
	Paused    bool            `json:"paused"`
	Method    playback.Method `json:"method"`
	Light     playback.Light  `json:"light"`
	Reason    string          `json:"reason,omitempty"`
	Since     time.Time       `json:"since"`
}

// playbackMethod übersetzt die Wiedergabeart in die drei Stufen des Dashboards.
func playbackMethod(m playback.Method) playback.Method {
	switch m {
	case playback.DirectPlay:
		return "direct"
	case playback.Transcode:
		return "transcode"
	}
	return "remux"
}

func (s *Server) sessions(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.streams.sessions()) }

// clientName macht aus dem User-Agent eine kurze Bezeichnung für Sitzungen und Geräte.
func clientName(ua string) string {
	l := strings.ToLower(ua)
	for _, c := range []struct{ key, name string }{
		{"okhttp", "Android-App"}, {"flimmer-android", "Android-App"}, {"dalvik", "Android-App"}, {"web0s", "LG webOS"}, {"webos", "LG webOS"},
		{"tizen", "Samsung Tizen"}, {"vidaa", "Hisense VIDAA"}, {"android", "Android-Browser"}, {"iphone", "iPhone"}, {"ipad", "iPad"},
		{"edg/", "Edge"}, {"firefox", "Firefox"}, {"chrome", "Chrome"}, {"safari", "Safari"}, {"curl", "curl"},
	} {
		if strings.Contains(l, c.key) {
			return c.name
		}
	}
	if ua == "" {
		return ""
	}
	return "Sonstiges"
}

// --- Übersicht ---

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	set, err := s.DB.Settings(r.Context())
	if writeErr(w, err) {
		return
	}
	type release struct {
		Version string `json:"version"`
		URL     string `json:"url"`
	}
	server := struct {
		Name    string    `json:"name"`
		Version string    `json:"version"`
		OS      string    `json:"os"`
		Arch    string    `json:"arch"`
		Uptime  int64     `json:"uptime"`
		Started time.Time `json:"started"`
		Update  *release  `json:"update,omitempty"`
	}{Name: set.ServerName, Version: update.Version, OS: runtime.GOOS, Arch: runtime.GOARCH,
		Uptime: int64(time.Since(started).Seconds()), Started: started}
	if server.Name == "" {
		server.Name, _ = os.Hostname()
	}
	if s.Updates != nil {
		if u := s.Updates.Available(); u != nil {
			server.Update = &release{u.Version, u.URL}
		}
	}
	lib := struct {
		Movies    int        `json:"movies"`
		Series    int        `json:"series"`
		Episodes  int        `json:"episodes"`
		SizeBytes int64      `json:"sizeBytes"`
		LastScan  *time.Time `json:"lastScan,omitempty"`
	}{}
	seen := map[string]bool{}
	for _, it := range s.Lib.All() {
		lib.SizeBytes += it.Size
		switch {
		case it.Series == "":
			lib.Movies++
		case !seen[it.Series]:
			seen[it.Series] = true
			lib.Series++
			fallthrough
		default:
			lib.Episodes++
		}
	}
	if t := s.Lib.LastScan(); !t.IsZero() {
		lib.LastScan = &t
	}
	disks := laufwerke(append(slices.Clone(set.Dirs), s.CacheDir))
	writeJSON(w, map[string]any{"server": server, "library": lib, "disks": disks,
		"sessions": s.streams.sessions(), "activity": s.activity.last(20)})
}

// --- Geräte: angemeldete Sessions ---

type deviceOut struct {
	db.Session
	Current bool `json:"current,omitempty"` // das Gerät dieser Anfrage
}

func (s *Server) devices(w http.ResponseWriter, r *http.Request) {
	list, err := s.DB.Sessions(r.Context(), sessionTTL)
	if writeErr(w, err) {
		return
	}
	cur := ""
	if tok := sessionToken(r); tok != "" {
		cur = db.SessionID(auth.HashToken(tok))
	}
	out := make([]deviceOut, 0, len(list))
	for _, d := range list {
		out = append(out, deviceOut{d, d.ID == cur})
	}
	writeJSON(w, out)
}

func (s *Server) deleteDevice(w http.ResponseWriter, r *http.Request) {
	if !writeErr(w, s.DB.DeleteSessionID(r.Context(), r.PathValue("id"))) {
		s.activity.add("user", userFrom(r).Name, "Gerät abgemeldet")
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- Protokoll ---

func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
	min := slog.LevelInfo
	if lv := r.URL.Query().Get("level"); lv != "" && min.UnmarshalText([]byte(lv)) != nil {
		http.Error(w, "level: debug, info, warn oder error", http.StatusBadRequest)
		return
	}
	writeJSON(w, s.Log.Entries(min, queryLimit(r, 200, ringMax)))
}

// --- Geplante Aufgaben ---

type task struct {
	id, name, group, desc string
	run                   func(ctx context.Context, progress func(float64)) error
	// auto meldet Läufe, die ohne diese Aufgabe stattfanden (Scan-Schleife, Nacht-Backup …); nil = keine.
	auto func() (at time.Time, errText string)
	next func() time.Time // nil = nur von Hand
}

type taskRun struct {
	LastRun    time.Time `json:"lastRun"`
	LastResult string    `json:"lastResult"` // ok, error
	LastError  string    `json:"lastError,omitempty"`
	Duration   float64   `json:"duration"` // Sekunden
}

type taskOut struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Group       string     `json:"group"`
	Description string     `json:"description"`
	LastRun     *time.Time `json:"lastRun,omitempty"`
	LastResult  string     `json:"lastResult,omitempty"`
	LastError   string     `json:"lastError,omitempty"`
	Duration    float64    `json:"duration,omitempty"`
	Next        *time.Time `json:"next,omitempty"`
	Running     bool       `json:"running"`
	Progress    *float64   `json:"progress,omitempty"`
}

// running hält laufende Aufgaben und ihren Fortschritt (-1 = unbekannt).
type running struct {
	mu sync.Mutex
	m  map[string]float64
}

// tasks listet die vorhandenen Jobs; was der Server nicht hat (z. B. kein Live-TV), fehlt.
func (s *Server) tasks() []task {
	daily := func(hour int) func() time.Time {
		return func() time.Time {
			now := time.Now()
			t := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
			if !t.After(now) {
				t = t.AddDate(0, 0, 1)
			}
			return t
		}
	}
	list := []task{
		{id: "scan", name: "Bibliothek scannen", group: "Bibliothek", desc: "Sucht neue, geänderte und entfernte Dateien in allen Medienordnern.",
			run: func(ctx context.Context, _ func(float64)) error {
				if err := s.Lib.Scan(ctx); errors.Is(err, scan.ErrBusy) {
					return errMsg("Es läuft bereits ein Scan")
				} else if err != nil {
					return err
				}
				s.ScanDone(len(s.Lib.All()), nil)
				return nil
			},
			auto: func() (time.Time, string) { return s.Lib.LastScan(), "" }},
		{id: "meta", name: "Metadaten aktualisieren", group: "Bibliothek",
			desc: "Holt Beschreibungen, Personen und Bilder für alle Titel neu. Gesperrte Felder bleiben erhalten.", run: s.Lib.RefreshMeta},
		{id: "optimize", name: "Nachts vorbereiten", group: "Wiedergabe",
			desc: "Legt für Titel, die sonst umgewandelt werden müssten, eine abspielbare Version an.",
			run: func(ctx context.Context, progress func(float64)) error {
				o := s.Optimizer.Load()
				if o == nil {
					return errMsg("Noch nicht bereit: Die Hardware wird gerade gemessen")
				}
				return o.RunNow(ctx, progress)
			},
			next: func() time.Time {
				set, _ := s.DB.Settings(context.Background())
				from, _, on := set.Optimize.Window()
				if !on {
					return time.Time{}
				}
				return daily(from)()
			}},
		{id: "db", name: "Datenbank pflegen", group: "Wartung", desc: "Prüft die Datenbank auf Fehler und optimiert sie.",
			run: func(ctx context.Context, _ func(float64)) error {
				if res := s.DB.Check(ctx); res != "ok" {
					return errors.New(res)
				}
				return nil
			},
			auto: func() (time.Time, string) {
				m := s.DB.Maintenance(context.Background())
				if m.Integrity == "ok" {
					return m.CheckedAt, ""
				}
				return m.CheckedAt, m.Integrity
			},
			next: func() time.Time {
				if at := s.DB.Maintenance(context.Background()).CheckedAt; !at.IsZero() {
					return at.Add(7 * 24 * time.Hour)
				}
				return time.Time{}
			}},
	}
	if s.BackupDir != "" {
		list = append(list, task{id: "backup", name: "Sicherung erstellen", group: "Wartung",
			desc: "Sichert die Datenbank (Einstellungen, Benutzer, Fortschritt); die letzten 7 bleiben.",
			run: func(ctx context.Context, _ func(float64)) error {
				if err := s.DB.BackupNow(ctx, s.BackupDir, 7); err != nil {
					return err
				}
				s.activity.add("backup", "", "Sicherung erstellt")
				return nil
			},
			auto: func() (time.Time, string) { return s.DB.Maintenance(context.Background()).BackupAt, "" },
			next: daily(3)})
	}
	if tv := s.LiveTV; tv != nil {
		list = append(list, task{id: "livetv", name: "Live-TV-Programm laden", group: "Live-TV",
			desc: "Lädt Kanalliste und Programm (XMLTV) neu; läuft sonst alle 6 Stunden.",
			run:  func(ctx context.Context, _ func(float64)) error { return tv.Refresh(ctx) },
			auto: tv.Updated,
			next: func() time.Time {
				if at, _ := tv.Updated(); !at.IsZero() {
					return at.Add(6 * time.Hour)
				}
				return time.Time{}
			}})
	}
	if m := s.Mediathek; m != nil {
		list = append(list, task{id: "mediathek", name: "Mediathek-Abos prüfen", group: "Bibliothek",
			desc: "Sucht neue Sendungen für die Mediathek-Abos und reiht sie zum Download ein; läuft sonst alle 6 Stunden.",
			run:  func(ctx context.Context, _ func(float64)) error { return m.PruefeAbos(ctx) },
			auto: m.Geprueft,
			next: func() time.Time {
				if at, _ := m.Geprueft(); !at.IsZero() {
					return at.Add(6 * time.Hour)
				}
				return time.Time{}
			}})
	}
	if s.Images != nil {
		list = append(list, task{id: "images", name: "Bild-Cache aufräumen", group: "Wartung",
			desc: "Löscht verkleinerte Bilder und Standbilder, die älter als 30 Tage sind. Sie entstehen bei Bedarf neu.",
			run: func(context.Context, func(float64)) error {
				n, err := s.Images.Prune(30 * 24 * time.Hour)
				slog.Info("Bild-Cache aufgeräumt", "dateien", n)
				return err
			}})
	}
	return list
}

func taskKey(id string) string { return "task_" + id }

func (s *Server) taskList(w http.ResponseWriter, r *http.Request) {
	out := []taskOut{}
	for _, t := range s.tasks() {
		var last taskRun
		json.Unmarshal([]byte(s.DB.KV(r.Context(), taskKey(t.id))), &last)
		if t.auto != nil {
			if at, e := t.auto(); at.After(last.LastRun) {
				last = taskRun{LastRun: at, LastResult: "ok"}
				if e != "" {
					last.LastResult, last.LastError = "error", e
				}
			}
		}
		o := taskOut{ID: t.id, Name: t.name, Group: t.group, Description: t.desc,
			LastResult: last.LastResult, LastError: last.LastError, Duration: last.Duration}
		if !last.LastRun.IsZero() {
			o.LastRun = &last.LastRun
		}
		if t.next != nil {
			if n := t.next(); !n.IsZero() {
				o.Next = &n
			}
		}
		s.running.mu.Lock()
		if p, ok := s.running.m[t.id]; ok {
			o.Running = true
			if p >= 0 {
				o.Progress = &p
			}
		}
		s.running.mu.Unlock()
		out = append(out, o)
	}
	writeJSON(w, out)
}

// runTask: POST /api/tasks/{id}/run → 202; läuft sie schon, 409.
func (s *Server) runTask(w http.ResponseWriter, r *http.Request) {
	i := slices.IndexFunc(s.tasks(), func(t task) bool { return t.id == r.PathValue("id") })
	if i < 0 {
		http.NotFound(w, r)
		return
	}
	t := s.tasks()[i]
	s.running.mu.Lock()
	if _, busy := s.running.m[t.id]; busy {
		s.running.mu.Unlock()
		http.Error(w, "Die Aufgabe läuft bereits", http.StatusConflict)
		return
	}
	if s.running.m == nil {
		s.running.m = map[string]float64{}
	}
	s.running.m[t.id] = -1
	s.running.mu.Unlock()
	user := userFrom(r).Name
	go s.execTask(t, user)
	w.WriteHeader(http.StatusAccepted)
}

// execTask führt eine Aufgabe aus und merkt sich den Lauf in der Datenbank. Sie läuft über das Ende der Anfrage hinaus.
func (s *Server) execTask(t task, user string) {
	start := time.Now()
	progress := func(p float64) {
		s.running.mu.Lock()
		s.running.m[t.id] = p
		s.running.mu.Unlock()
	}
	err := t.run(context.Background(), progress)
	run := taskRun{LastRun: start, LastResult: "ok", Duration: time.Since(start).Seconds()}
	if err != nil {
		run.LastResult, run.LastError = "error", err.Error()
		slog.Error("Aufgabe fehlgeschlagen", "aufgabe", t.name, "fehler", err)
		s.activity.add("error", user, "Aufgabe „"+t.name+"“ fehlgeschlagen: "+err.Error())
	} else {
		s.activity.add("task", user, "Aufgabe „"+t.name+"“ erledigt")
	}
	b, _ := json.Marshal(run)
	if err := s.DB.SetKV(context.Background(), taskKey(t.id), string(b)); err != nil {
		slog.Error("Aufgabenlauf speichern", "fehler", err)
	}
	s.running.mu.Lock()
	delete(s.running.m, t.id)
	s.running.mu.Unlock()
}
