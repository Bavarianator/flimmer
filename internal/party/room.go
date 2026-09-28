// Package party: Gemeinsam schauen. Der Server ist die Autorität über den Raumzustand; Clients gleichen ihre
// Uhr per GET /api/time ab und rechnen die Soll-Position selbst aus: pos + (jetzt − serverTs) · rate.
// Jeder Teilnehmer holt seinen Wiedergabe-Plan passend zum eigenen Gerät über die normale play-Route.
package party

import (
	"slices"
	"time"
)

// BufferTimeout: So lange wartet der Raum auf einen puffernden Teilnehmer, dann geht es ohne ihn weiter.
const BufferTimeout = 10 * time.Second

// State ist das, was jeder Client bekommt.
type State struct {
	MediaID  string   `json:"mediaId"`
	Pos      float64  `json:"pos"`      // Sekunden zum Zeitpunkt ServerTs
	ServerTs int64    `json:"serverTs"` // Unix-ms
	Rate     float64  `json:"rate"`
	Paused   bool     `json:"paused"`  // effektiv: vom Nutzer pausiert ODER jemand puffert
	Waiting  []string `json:"waiting"` // Namen der Puffernden
	Host     string   `json:"host"`    // Benutzer-ID des Erstellers
}

// Room ist die reine Zustandsmaschine – keine Goroutinen, keine Uhr; now kommt von außen (testbar).
type Room struct {
	mediaID    string
	pos        float64
	anchor     time.Time // Zeitpunkt, zu dem pos galt
	rate       float64
	userPaused bool
	waiting    map[string]time.Time // Mitglied → Frist
	names      map[string]string    // Mitglied → Anzeigename
	host       string
}

func NewRoom(mediaID, host string, now time.Time) *Room {
	return &Room{mediaID: mediaID, anchor: now, rate: 1, userPaused: true, host: host,
		waiting: map[string]time.Time{}, names: map[string]string{}}
}

func (r *Room) paused() bool { return r.userPaused || len(r.waiting) > 0 }

// Position zum Zeitpunkt now.
func (r *Room) Position(now time.Time) float64 {
	if r.paused() {
		return r.pos
	}
	return r.pos + now.Sub(r.anchor).Seconds()*r.rate
}

// freeze legt die aktuelle Position fest – vor jeder Änderung von paused/rate nötig.
func (r *Room) freeze(now time.Time) {
	r.pos, r.anchor = r.Position(now), now
}

func (r *Room) State(now time.Time) State {
	st := State{MediaID: r.mediaID, Pos: r.Position(now), ServerTs: now.UnixMilli(), Rate: r.rate,
		Paused: r.paused(), Waiting: []string{}, Host: r.host}
	for m := range r.waiting {
		st.Waiting = append(st.Waiting, r.names[m])
	}
	slices.Sort(st.Waiting)
	return st
}

// Action ist eine Aktion eines Teilnehmers.
type Action struct {
	Type      string  `json:"type"` // play, pause, seek, rate, media, buffering, chat, reaction
	Pos       float64 `json:"pos"`
	Rate      float64 `json:"rate"`
	MediaID   string  `json:"mediaId"`
	Buffering bool    `json:"buffering"`
	Text      string  `json:"text"`
}

// Apply wendet a an und meldet, ob sich der Zustand geändert hat (dann wird er an alle verteilt).
// Chat und Reaktionen ändern den Zustand nicht, sie werden vom Hub nur weitergereicht.
func (r *Room) Apply(member string, a Action, now time.Time) (changed bool, err error) {
	r.Expire(now)
	switch a.Type {
	case "play":
		r.freeze(now)
		r.userPaused = false
	case "pause":
		r.freeze(now)
		r.userPaused = true
	case "seek":
		if a.Pos < 0 {
			return false, errBad("Position muss ≥ 0 sein")
		}
		r.pos, r.anchor = a.Pos, now
		// Nach dem Spulen puffern alle kurz; wer es meldet, landet in waiting.
	case "rate":
		if a.Rate < 0.25 || a.Rate > 4 {
			return false, errBad("Geschwindigkeit 0,25–4")
		}
		r.freeze(now)
		r.rate = a.Rate
	case "media":
		if a.MediaID == "" {
			return false, errBad("mediaId fehlt")
		}
		r.mediaID, r.pos, r.anchor, r.userPaused = a.MediaID, 0, now, true
		clear(r.waiting)
	case "buffering":
		_, was := r.waiting[member]
		if a.Buffering == was {
			return false, nil
		}
		r.freeze(now)
		if a.Buffering {
			r.waiting[member] = now.Add(BufferTimeout)
		} else {
			delete(r.waiting, member)
		}
	case "chat", "reaction":
		return false, nil
	default:
		return false, errBad("unbekannte Aktion " + a.Type)
	}
	return true, nil
}

// Expire nimmt Teilnehmer aus der Warteliste, die länger als BufferTimeout puffern. true = Zustand geändert.
func (r *Room) Expire(now time.Time) bool {
	changed := false
	for m, deadline := range r.waiting {
		if !now.Before(deadline) {
			r.freeze(now)
			delete(r.waiting, m)
			changed = true
		}
	}
	return changed
}

// NextDeadline: wann Expire das nächste Mal etwas zu tun hat (Null-Zeit = nie).
func (r *Room) NextDeadline() time.Time {
	var next time.Time
	for _, d := range r.waiting {
		if next.IsZero() || d.Before(next) {
			next = d
		}
	}
	return next
}

func (r *Room) Join(member, name string) { r.names[member] = name }

// Members liefert die Anzeigenamen aller Teilnehmer (sortiert, doppelte Geräte eines Benutzers einmal).
func (r *Room) Members() []string {
	out := []string{}
	for _, n := range r.names {
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out
}

// Leave: Wer geht, blockiert niemanden mehr.
func (r *Room) Leave(member string, now time.Time) bool {
	delete(r.names, member)
	if _, ok := r.waiting[member]; ok {
		r.freeze(now)
		delete(r.waiting, member)
		return true
	}
	return false
}

type errBad string

func (e errBad) Error() string { return string(e) }
