package meta

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Lookup findet die Such-Felder eines Titels anhand der Item-ID aus der URL.
type Lookup func(id string) (Query, bool)

// SearchHandler: GET /api/items/{id}/search?q=…&year=… → []Candidate.
// Ohne q wird mit dem erkannten Titel (bzw. Seriennamen) gesucht.
func (r *Resolver) SearchHandler(lookup Lookup) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		q, ok := lookup(req.PathValue("id"))
		if !ok {
			http.NotFound(w, req)
			return
		}
		text := req.URL.Query().Get("q")
		if text == "" {
			text = cmp(q.Series, q.Title)
		}
		year, _ := strconv.Atoi(req.URL.Query().Get("year"))
		cs, err := r.TMDB.Search(req.Context(), text, year, q.Series != "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSONResponse(w, cs)
	}
}

// IdentifyHandler: POST /api/items/{id}/identify {"tmdbId":123} → Meta.
// changed wird mit dem neuen Ergebnis aufgerufen, damit die Bibliothek es übernimmt.
func (r *Resolver) IdentifyHandler(lookup Lookup, changed func(id string, m *Meta)) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		q, ok := lookup(req.PathValue("id"))
		if !ok {
			http.NotFound(w, req)
			return
		}
		var body struct {
			TMDBID int `json:"tmdbId"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.TMDBID <= 0 {
			http.Error(w, "tmdbId fehlt", http.StatusBadRequest)
			return
		}
		m, err := r.Identify(req.Context(), q, body.TMDBID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if changed != nil {
			changed(q.ID, m)
		}
		writeJSONResponse(w, m)
	}
}

func writeJSONResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
