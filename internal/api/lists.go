package api

import (
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/Bavarianator/flimmer/internal/db"
)

// Listen-Schlüssel sind Titel-IDs oder "serie:<Name>" (Favoriten, Sammlungen, Wiedergabelisten).

// seen prüft einen Listen-Schlüssel gegen die Sichtbarkeit des Benutzers: Den Titel bzw. mindestens eine Folge
// der Serie muss es geben, und der Benutzer muss sie sehen dürfen.
func (s *Server) seen(r *http.Request, key string) bool {
	if name, ok := strings.CutPrefix(key, "serie:"); ok {
		return len(s.visible(r, s.Lib.Episodes(name))) > 0
	}
	it := s.Lib.Get(key)
	return it != nil && s.allowed(r, it)
}

// seenOnly filtert Schlüssel auf das, was der Benutzer gerade sieht (fehlende Dateien bleiben gespeichert).
func (s *Server) seenOnly(r *http.Request, keys []string) []string {
	out := []string{}
	for _, k := range keys {
		if s.seen(r, k) {
			out = append(out, k)
		}
	}
	return out
}

// --- Favoriten ---

func (s *Server) favorites(w http.ResponseWriter, r *http.Request) {
	keys, err := s.DB.Favorites(r.Context(), userFrom(r).ID)
	if !writeErr(w, err) {
		writeJSON(w, s.seenOnly(r, keys))
	}
}

// setFavorite: PUT merkt, DELETE entfernt. Merken geht nur für sichtbare Titel.
func (s *Server) setFavorite(w http.ResponseWriter, r *http.Request) {
	key, on := r.PathValue("key"), r.Method == http.MethodPut
	if on && !s.seen(r, key) {
		http.NotFound(w, r)
		return
	}
	if !writeErr(w, s.DB.SetFavorite(r.Context(), userFrom(r).ID, key, on)) {
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- Sammlungen und Wiedergabelisten ---

type collectionOut struct {
	db.List
	Auto bool `json:"auto,omitempty"` // Filmreihe aus den Metadaten, nicht bearbeitbar
}

// collections: GET /api/collections – eigene Sammlungen und Filmreihen, Einträge nur, soweit sichtbar.
func (s *Server) collections(w http.ResponseWriter, r *http.Request) {
	lists, err := s.DB.Lists(r.Context(), db.Collection, "")
	if writeErr(w, err) {
		return
	}
	out := []collectionOut{}
	for _, l := range lists {
		l.Items = s.seenOnly(r, l.Items)
		if len(l.Items) > 0 || !s.restricted(r) { // leere Sammlungen nur für Benutzer, die alles sehen
			out = append(out, collectionOut{List: l})
		}
	}
	writeJSON(w, append(out, s.autoCollections(r)...))
}

// autoCollections: Filmreihen aus TMDB (belongs_to_collection) bzw. NFO (<set>), ab zwei sichtbaren Filmen, nach Jahr.
func (s *Server) autoCollections(r *http.Request) []collectionOut {
	type film struct {
		id   string
		year int
	}
	films, names := map[string][]film{}, map[string]string{}
	for _, it := range s.visible(r, s.Lib.All()) {
		m := s.Lib.MetaFor(it.ID)
		if it.Series != "" || m == nil || m.Collection == nil {
			continue
		}
		key := "auto-" + strconv.Itoa(m.Collection.TMDBID)
		if m.Collection.TMDBID == 0 {
			h := sha1.Sum([]byte(m.Collection.Name))
			key = "auto-set-" + hex.EncodeToString(h[:6])
		}
		films[key] = append(films[key], film{it.ID, cmp0(m.Year, it.Year)})
		names[key] = m.Collection.Name
	}
	out := []collectionOut{}
	for key, fs := range films {
		if len(fs) < 2 {
			continue
		}
		slices.SortStableFunc(fs, func(a, b film) int { return a.year - b.year })
		c := collectionOut{List: db.List{ID: key, Name: names[key], Items: []string{}}, Auto: true}
		for _, f := range fs {
			c.Items = append(c.Items, f.id)
		}
		out = append(out, c)
	}
	slices.SortFunc(out, func(a, b collectionOut) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out
}

func cmp0(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

// playlists: GET /api/playlists – nur die eigenen.
func (s *Server) playlists(w http.ResponseWriter, r *http.Request) {
	lists, err := s.DB.Lists(r.Context(), db.Playlist, userFrom(r).ID)
	if writeErr(w, err) {
		return
	}
	for i := range lists {
		lists[i].Items = s.seenOnly(r, lists[i].Items)
	}
	writeJSON(w, lists)
}

type listReq struct {
	Name     *string  `json:"name"`
	Overview *string  `json:"overview"`
	Items    []string `json:"items"` // POST: Inhalt; PUT: ersetzt die Liste (neue Reihenfolge)
	Add      []string `json:"add"`
	Remove   []string `json:"remove"`
}

// listOwner: Sammlungen gehören allen, Wiedergabelisten dem Profil.
func listOwner(r *http.Request, kind string) string {
	if kind == db.Collection {
		return ""
	}
	return userFrom(r).ID
}

func (s *Server) createList(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req listReq
		if !readJSON(w, r, &req) {
			return
		}
		l := db.List{ID: newID(), Items: []string{}}
		if err := s.applyList(r, &l, req); writeErr(w, err) {
			return
		}
		if !writeErr(w, s.DB.CreateList(r.Context(), kind, listOwner(r, kind), l)) {
			writeJSON(w, l)
		}
	}
}

func (s *Server) updateList(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req listReq
		if !readJSON(w, r, &req) {
			return
		}
		l, err := s.DB.UpdateList(r.Context(), kind, listOwner(r, kind), r.PathValue("id"), func(l *db.List) error { return s.applyList(r, l, req) })
		if !writeErr(w, err) {
			l.Items = s.seenOnly(r, l.Items)
			writeJSON(w, l)
		}
	}
}

func (s *Server) deleteList(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !writeErr(w, s.DB.DeleteList(r.Context(), kind, listOwner(r, kind), r.PathValue("id"))) {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

const listMax = 5000

// applyList übernimmt eine Änderung. Neue Einträge muss der Benutzer sehen dürfen; doppelte zählen einmal.
func (s *Server) applyList(r *http.Request, l *db.List, req listReq) error {
	if req.Name != nil {
		l.Name = strings.TrimSpace(*req.Name)
	}
	if l.Name == "" || len(l.Name) > 100 {
		return errMsg("Bitte einen Namen angeben (höchstens 100 Zeichen)")
	}
	if req.Overview != nil {
		l.Overview = strings.TrimSpace(*req.Overview)
	}
	if req.Items != nil {
		l.Items = []string{}
	}
	for _, k := range append(req.Items, req.Add...) {
		if !s.seen(r, k) {
			return errMsg("Unbekannter Titel: " + k)
		}
		if !slices.Contains(l.Items, k) {
			l.Items = append(l.Items, k)
		}
	}
	l.Items = slices.DeleteFunc(l.Items, func(k string) bool { return slices.Contains(req.Remove, k) })
	if len(l.Items) > listMax {
		return errMsg("Zu viele Einträge")
	}
	return nil
}
