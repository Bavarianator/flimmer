package party

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Hub verwaltet die Räume und verteilt Ereignisse per Server-Sent Events (Standardbibliothek, läuft auch auf webOS 4).
type Hub struct {
	// User liefert den angemeldeten Benutzer (ID, Anzeigename).
	User func(r *http.Request) (id, name string, ok bool)
	// Allowed prüft, ob der Benutzer den Titel sehen darf (Gäste mit Einladung). nil = alles erlaubt.
	Allowed func(r *http.Request, mediaID string) bool
	// EventsURL liefert die SSE-Adresse für diesen Benutzer, z. B. mit Medien-Token im Pfad für TVs,
	// deren EventSource keine Header senden kann. nil = /api/party/{id}/events.
	EventsURL func(r *http.Request, roomID string) string

	mu    sync.Mutex
	rooms map[string]*hubRoom
	now   func() time.Time
}

type hubRoom struct {
	room    *Room
	subs    map[string]chan event // Mitglied → Ereignisse
	expire  *time.Timer
	cleanup *time.Timer
}

type event struct {
	name string
	data []byte
}

const (
	idleRoom  = 30 * time.Minute // leere Räume verschwinden danach
	heartbeat = 15 * time.Second
	subBuffer = 64
)

func (h *Hub) clock() time.Time {
	if h.now != nil {
		return h.now()
	}
	return time.Now()
}

// Time: GET /api/time – für den NTP-artigen Uhrenabgleich der Clients (Median aus mehreren Messungen).
func Time(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, `{"now":%d}`, time.Now().UnixMilli())
}

// Create: POST /api/party {mediaId} → {id, state}
func (h *Hub) Create(w http.ResponseWriter, r *http.Request) {
	uid, _, ok := h.User(r)
	if !ok {
		http.Error(w, "Bitte anmelden", http.StatusUnauthorized)
		return
	}
	var req struct {
		MediaID string `json:"mediaId"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil || req.MediaID == "" {
		http.Error(w, "mediaId fehlt", http.StatusBadRequest)
		return
	}
	if h.Allowed != nil && !h.Allowed(r, req.MediaID) {
		http.NotFound(w, r)
		return
	}
	b := make([]byte, 6)
	rand.Read(b)
	id := hex.EncodeToString(b)
	now := h.clock()
	h.mu.Lock()
	if h.rooms == nil {
		h.rooms = map[string]*hubRoom{}
	}
	hr := &hubRoom{room: NewRoom(req.MediaID, uid, now), subs: map[string]chan event{}}
	h.rooms[id] = hr
	h.scheduleCleanup(id, hr)
	st := hr.room.State(now)
	h.mu.Unlock()
	writeJSON(w, map[string]any{"id": id, "state": st, "eventsUrl": h.eventsURL(r, id)})
}

// Get: GET /api/party/{id} → {state, members}
func (h *Hub) Get(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	hr := h.rooms[r.PathValue("id")]
	var st State
	var members []string
	if hr != nil {
		st, members = hr.room.State(h.clock()), hr.room.Members()
	}
	h.mu.Unlock()
	if hr == nil {
		http.Error(w, "Raum gibt es nicht (mehr)", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"state": st, "members": members, "eventsUrl": h.eventsURL(r, r.PathValue("id"))})
}

// Events: GET /api/party/{id}/events – SSE. Erstes Ereignis „hello“ mit der eigenen Mitglieds-ID,
// danach „state“, „members“, „chat“, „reaction“. Mit der Verbindung endet die Mitgliedschaft.
func (h *Hub) Events(w http.ResponseWriter, r *http.Request) {
	uid, name, ok := h.User(r)
	if !ok {
		http.Error(w, "Bitte anmelden", http.StatusUnauthorized)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming nicht möglich", http.StatusInternalServerError)
		return
	}
	id := r.PathValue("id")
	b := make([]byte, 4)
	rand.Read(b)
	member := uid + "." + hex.EncodeToString(b) // ein Benutzer kann mit mehreren Geräten dabei sein
	ch := make(chan event, subBuffer)

	h.mu.Lock()
	hr := h.rooms[id]
	if hr == nil {
		h.mu.Unlock()
		http.Error(w, "Raum gibt es nicht (mehr)", http.StatusNotFound)
		return
	}
	if h.Allowed != nil && !h.Allowed(r, hr.room.mediaID) {
		h.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	hr.room.Join(member, name)
	hr.subs[member] = ch
	if hr.cleanup != nil {
		hr.cleanup.Stop()
		hr.cleanup = nil
	}
	now := h.clock()
	hello, _ := json.Marshal(map[string]any{"member": member, "state": hr.room.State(now)})
	h.broadcastLocked(hr, "members", map[string]any{"members": hr.room.Members()})
	h.mu.Unlock()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no") // Reverse-Proxys nicht puffern lassen
	fmt.Fprintf(w, "retry: 2000\nevent: hello\ndata: %s\n\n", hello)
	flusher.Flush()

	defer func() {
		h.mu.Lock()
		delete(hr.subs, member)
		if hr.room.Leave(member, h.clock()) {
			h.broadcastLocked(hr, "state", map[string]any{"state": hr.room.State(h.clock()), "action": "leave"})
			h.scheduleExpire(hr)
		}
		h.broadcastLocked(hr, "members", map[string]any{"members": hr.room.Members()})
		if len(hr.subs) == 0 {
			h.scheduleCleanup(id, hr)
		}
		h.mu.Unlock()
	}()

	tick := time.NewTicker(heartbeat)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, open := <-ch:
			if !open {
				return // zu langsam gelesen – der Client verbindet sich neu (retry)
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.name, ev.data)
			flusher.Flush()
		case <-tick.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// Action: POST /api/party/{id}/actions {member, type, …}
func (h *Hub) Action(w http.ResponseWriter, r *http.Request) {
	uid, name, ok := h.User(r)
	if !ok {
		http.Error(w, "Bitte anmelden", http.StatusUnauthorized)
		return
	}
	var req struct {
		Member string `json:"member"`
		Action
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, "ungültige Anfrage", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(req.Member, uid+".") {
		http.Error(w, "fremdes Mitglied", http.StatusForbidden)
		return
	}
	if req.Type == "media" && h.Allowed != nil && !h.Allowed(r, req.MediaID) {
		http.NotFound(w, r)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	hr := h.rooms[r.PathValue("id")]
	if hr == nil || hr.subs[req.Member] == nil {
		http.Error(w, "nicht im Raum", http.StatusNotFound)
		return
	}
	now := h.clock()
	switch req.Type {
	case "chat", "reaction":
		text := strings.TrimSpace(req.Text)
		if text == "" || len(text) > map[string]int{"chat": 500, "reaction": 16}[req.Type] {
			http.Error(w, "Text leer oder zu lang", http.StatusBadRequest)
			return
		}
		h.broadcastLocked(hr, req.Type, map[string]any{"from": name, "member": req.Member, "text": text, "ts": now.UnixMilli()})
	default:
		changed, err := hr.room.Apply(req.Member, req.Action, now)
		var bad errBad
		if errors.As(err, &bad) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if changed {
			st := hr.room.State(now)
			h.broadcastLocked(hr, "state", map[string]any{"state": st, "by": name, "action": req.Type})
			h.scheduleExpire(hr)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// broadcastLocked verteilt an alle; wer nicht mitkommt, wird getrennt und verbindet sich neu (h.mu gehalten).
func (h *Hub) broadcastLocked(hr *hubRoom, name string, v any) {
	data, _ := json.Marshal(v)
	for m, ch := range hr.subs {
		select {
		case ch <- event{name, data}:
		default:
			close(ch)
			delete(hr.subs, m)
		}
	}
}

// scheduleExpire weckt den Raum, wenn der nächste Puffer-Timeout fällig ist (h.mu gehalten).
func (h *Hub) scheduleExpire(hr *hubRoom) {
	if hr.expire != nil {
		hr.expire.Stop()
		hr.expire = nil
	}
	next := hr.room.NextDeadline()
	if next.IsZero() {
		return
	}
	hr.expire = time.AfterFunc(next.Sub(h.clock()), func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		now := h.clock()
		if hr.room.Expire(now) {
			h.broadcastLocked(hr, "state", map[string]any{"state": hr.room.State(now), "action": "timeout"})
		}
		h.scheduleExpire(hr)
	})
}

// scheduleCleanup entfernt einen leeren Raum nach idleRoom (h.mu gehalten).
func (h *Hub) scheduleCleanup(id string, hr *hubRoom) {
	if hr.cleanup != nil {
		hr.cleanup.Stop()
	}
	hr.cleanup = time.AfterFunc(idleRoom, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if len(hr.subs) == 0 && h.rooms[id] == hr {
			if hr.expire != nil {
				hr.expire.Stop()
			}
			delete(h.rooms, id)
		}
	})
}

func (h *Hub) eventsURL(r *http.Request, id string) string {
	if h.EventsURL != nil {
		return h.EventsURL(r, id)
	}
	return "/api/party/" + id + "/events"
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
