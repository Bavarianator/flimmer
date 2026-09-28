// Package livetv bringt Live-TV und Programm (EPG) in Flimmer: Kanäle aus einer M3U-Liste oder der lineup.json
// eines HDHomeRun, Programm aus XMLTV. Gespielt wird per ffmpeg-Remux nach HLS (stream.go); ein Kanal teilt sich
// unter allen Zuschauern einen ffmpeg, der nach 30 s ohne Abruf endet. Kein DVR, kein Timeshift.
package livetv

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TV struct {
	DB  *sql.DB
	Dir string // Arbeitsordner der Live-Streams, z. B. <cache>/livetv
	// Allow entscheidet, ob der Benutzer Live-TV sehen darf (Gäste nicht). nil = niemand: lieber zu streng als offen.
	Allow func(r *http.Request) bool
	// URL baut die abspielbare Adresse eines Kanals samt Medien-Token (der Player schickt keine Header).
	URL func(r *http.Request, channelID string) string

	kick chan struct{}

	mu      sync.RWMutex
	chs     []Channel
	byID    map[string]Channel
	guide   *guide
	updated time.Time
	lastErr string

	smu     sync.Mutex
	streams map[string]*stream
}

func New(db *sql.DB, dir string) *TV {
	return &TV{DB: db, Dir: dir, kick: make(chan struct{}, 1), streams: map[string]*stream{}, byID: map[string]Channel{}}
}

// Config liegt als Schlüssel livetv_* in der Tabelle settings.
type Config struct {
	Source string // M3U-URL, lineup.json eines HDHomeRun oder Dateipfad
	EPG    string // XMLTV-URL oder Dateipfad (auch .gz), leer = kein Programm
	Video  string // "copy" (Standard) oder "h264"
}

var keys = map[string]func(*Config) *string{
	"livetv_source": func(c *Config) *string { return &c.Source },
	"livetv_epg":    func(c *Config) *string { return &c.EPG },
	"livetv_video":  func(c *Config) *string { return &c.Video },
}

func (tv *TV) config(ctx context.Context) (c Config) {
	rows, err := tv.DB.QueryContext(ctx, "SELECT key, value FROM settings WHERE key LIKE 'livetv\\_%' ESCAPE '\\'")
	if err != nil {
		return c
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) == nil && keys[k] != nil {
			*keys[k](&c) = v
		}
	}
	return c
}

func (tv *TV) save(ctx context.Context, c Config) error {
	for k, f := range keys {
		v := *f(&c)
		q, args := "DELETE FROM settings WHERE key = ?", []any{k}
		if v != "" {
			q, args = "INSERT INTO settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", []any{k, v}
		}
		if _, err := tv.DB.ExecContext(ctx, q, args...); err != nil {
			return err
		}
	}
	return nil
}

// Run lädt Kanäle und Programm sofort, dann alle 6 Stunden oder auf Zuruf (Einstellung geändert), bis ctx endet.
func (tv *TV) Run(ctx context.Context) {
	go tv.reap(ctx)
	for {
		if err := tv.Refresh(ctx); err != nil {
			log.Printf("Live-TV: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-tv.kick:
		case <-time.After(6 * time.Hour):
		}
	}
}

// Refresh lädt Kanalliste und Programm neu. Bei einem Fehler bleiben die alten Daten stehen.
func (tv *TV) Refresh(ctx context.Context) error {
	c := tv.config(ctx)
	if c.Source == "" {
		tv.set(nil, nil, "")
		return nil
	}
	chs, err := fetch(ctx, c.Source, time.Minute, parseChannels)
	if err != nil {
		err = errors.New("Kanalliste: " + err.Error())
		tv.fail(err)
		return err
	}
	var g *guide
	var msg string
	if c.EPG != "" {
		if g, err = fetch(ctx, c.EPG, 5*time.Minute, func(r io.Reader) (*guide, error) { return parseXMLTV(r, time.Now()) }); err != nil {
			msg = "Programm: " + err.Error() // Kanäle laufen trotzdem
			g = nil
		}
	}
	tv.set(chs, g, msg)
	if msg != "" {
		return errors.New(msg)
	}
	return nil
}

func (tv *TV) fail(err error) {
	tv.mu.Lock()
	tv.lastErr = err.Error()
	tv.mu.Unlock()
}

func (tv *TV) set(chs []Channel, g *guide, msg string) {
	if g != nil { // Kanäle ohne tvg-id über den Anzeigenamen dem XMLTV-Kanal zuordnen
		for i, c := range chs {
			if _, ok := g.progs[c.epg]; !ok {
				chs[i].epg = g.names[strings.ToLower(c.Name)]
			}
		}
	}
	m := make(map[string]Channel, len(chs))
	for _, c := range chs {
		m[c.ID] = c
	}
	tv.mu.Lock()
	tv.chs, tv.byID, tv.guide, tv.updated, tv.lastErr = chs, m, g, time.Now(), msg
	tv.mu.Unlock()
}

func (tv *TV) channel(id string) (Channel, bool) {
	tv.mu.RLock()
	defer tv.mu.RUnlock()
	c, ok := tv.byID[id]
	return c, ok
}

func (tv *TV) programs(c Channel) []Program {
	if tv.guide == nil {
		return nil
	}
	return tv.guide.progs[c.epg]
}

// --- Handler (der api-Besitzer hängt sie ein; Status, PUT und refresh nur für Admins) ---

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// StatusHandler: GET, nur Admin. Zeigt nur Schema und Rechner der Adressen, nie Zugangsdaten.
func (tv *TV) StatusHandler(w http.ResponseWriter, r *http.Request) {
	c := tv.config(r.Context())
	tv.mu.RLock()
	defer tv.mu.RUnlock()
	writeJSON(w, map[string]any{"source": redact(c.Source), "epg": redact(c.EPG), "video": orCopy(c.Video), "channels": len(tv.chs), "programs": tv.guide.count(), "updated": tv.updated, "error": tv.lastErr})
}

func orCopy(v string) string {
	if v == "" {
		return "copy"
	}
	return v
}

// ConfigHandler: PUT, nur Admin. {source, epg, video}; fehlende Felder bleiben, "" löscht. Lädt danach neu.
func (tv *TV) ConfigHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Source, EPG, Video *string }
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "ungültige Anfrage", http.StatusBadRequest)
		return
	}
	c := tv.config(r.Context())
	for _, f := range []struct {
		in  *string
		out *string
	}{{req.Source, &c.Source}, {req.EPG, &c.EPG}} {
		if f.in == nil {
			continue
		}
		v := strings.TrimSpace(*f.in)
		if u, err := url.Parse(v); v != "" && (err != nil || (u.Scheme != "http" && u.Scheme != "https" && !strings.HasPrefix(v, "/"))) {
			http.Error(w, "Adresse muss mit http://, https:// oder einem absoluten Dateipfad beginnen", http.StatusBadRequest)
			return
		}
		*f.out = v
	}
	if req.Video != nil {
		if *req.Video != "copy" && *req.Video != "h264" {
			http.Error(w, `video: "copy" oder "h264"`, http.StatusBadRequest)
			return
		}
		c.Video = *req.Video
	}
	if err := tv.save(r.Context(), c); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tv.RefreshHandler(w, r)
}

// RefreshHandler: POST, nur Admin. Lädt im Hintergrund neu.
func (tv *TV) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	select {
	case tv.kick <- struct{}{}:
	default:
	}
	w.WriteHeader(http.StatusAccepted)
}

func (tv *TV) allowed(w http.ResponseWriter, r *http.Request) bool {
	if tv.Allow == nil || !tv.Allow(r) {
		http.Error(w, "Live-TV ist für dieses Konto nicht freigegeben", http.StatusForbidden)
		return false
	}
	return true
}

type nowNext struct {
	Channel
	Now  *Program `json:"now"`
	Next *Program `json:"next"`
}

// ChannelsHandler: GET, jeder Zugelassene. Kanäle mit „läuft jetzt“ und „danach“.
func (tv *TV) ChannelsHandler(w http.ResponseWriter, r *http.Request) {
	if !tv.allowed(w, r) {
		return
	}
	tv.mu.RLock()
	defer tv.mu.RUnlock()
	now := time.Now()
	out := make([]nowNext, 0, len(tv.chs))
	for _, c := range tv.chs {
		e := nowNext{Channel: c}
		p := tv.programs(c)
		for i := range p {
			if p[i].Stop.After(now) {
				if !p[i].Start.After(now) {
					e.Now = &p[i]
					if i+1 < len(p) {
						e.Next = &p[i+1]
					}
				} else {
					e.Next = &p[i]
				}
				break
			}
		}
		out = append(out, e)
	}
	writeJSON(w, out)
}

// GuideHandler: GET ?hours=6 (max. 48) &channel=<id>. Sendungen der nächsten Stunden je Kanal.
func (tv *TV) GuideHandler(w http.ResponseWriter, r *http.Request) {
	if !tv.allowed(w, r) {
		return
	}
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
	if hours < 1 || hours > 48 {
		hours = 6
	}
	only := r.URL.Query().Get("channel")
	from := time.Now()
	to := from.Add(time.Duration(hours) * time.Hour)
	type row struct {
		ID       string    `json:"id"`
		Programs []Program `json:"programs"`
	}
	rows := []row{}
	tv.mu.RLock()
	for _, c := range tv.chs {
		if only != "" && c.ID != only {
			continue
		}
		list := []Program{}
		for _, p := range tv.programs(c) {
			if p.Stop.After(from) && p.Start.Before(to) {
				list = append(list, p)
			}
		}
		if len(list) > 0 {
			rows = append(rows, row{c.ID, list})
		}
	}
	tv.mu.RUnlock()
	writeJSON(w, map[string]any{"from": from, "to": to, "channels": rows})
}

// PlayHandler: POST. Startet den Kanal schon jetzt (kurze Umschaltzeit) und nennt die HLS-Adresse.
func (tv *TV) PlayHandler(w http.ResponseWriter, r *http.Request) {
	if !tv.allowed(w, r) {
		return
	}
	ch, ok := tv.channel(r.PathValue("id"))
	if !ok || tv.URL == nil {
		http.NotFound(w, r)
		return
	}
	if _, err := tv.open(ch); err != nil {
		tv.openFailed(w, ch, err)
		return
	}
	writeJSON(w, map[string]any{"url": tv.URL(r, ch.ID), "title": ch.Name, "live": true})
}

var fileRE = regexp.MustCompile(`^(index\.m3u8|s\d+\.ts)$`)

// FileHandler: GET …/channels/{id}/{file}: die Playlist (startet den Stream bei Bedarf und wartet aufs erste Segment) und Segmente.
func (tv *TV) FileHandler(w http.ResponseWriter, r *http.Request) {
	if !tv.allowed(w, r) {
		return
	}
	ch, ok := tv.channel(r.PathValue("id"))
	name := r.PathValue("file")
	if !ok || !fileRE.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	if name != "index.m3u8" {
		s := tv.stream(ch.ID)
		if s == nil {
			http.NotFound(w, r)
			return
		}
		s.touch()
		w.Header().Set("Content-Type", "video/mp2t")
		http.ServeFile(w, r, filepath.Join(s.dir, name))
		return
	}
	s, err := tv.open(ch)
	if err == nil {
		err = s.ready(r.Context())
	}
	if err != nil {
		tv.openFailed(w, ch, err)
		return
	}
	b, err := readPlaylist(s)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(b)
}

func (tv *TV) openFailed(w http.ResponseWriter, ch Channel, err error) {
	if errors.Is(err, errBusy) {
		http.Error(w, "Alle Live-TV-Plätze sind belegt. Bitte später erneut versuchen.", http.StatusServiceUnavailable)
		return
	}
	log.Printf("Live-TV %s: %v", ch.Name, strings.ReplaceAll(err.Error(), ch.url, redact(ch.url)))
	http.Error(w, "Der Sender lässt sich gerade nicht öffnen.", http.StatusBadGateway)
}

func readPlaylist(s *stream) ([]byte, error) { return os.ReadFile(filepath.Join(s.dir, "index.m3u8")) }
