package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/Bavarianator/flimmer/internal/playback"
	"github.com/Bavarianator/flimmer/internal/scan"
)

// search: GET /api/search?q=&limit=&device= – fehlertolerant, beste Treffer zuerst, je Serie ein Treffer.
// Gäste und Kinderprofile sehen nur, was sie auch sonst sehen dürfen. device (optional) wählt das gespeicherte
// Geräteprofil für die Ampel; ohne gilt ein leeres Profil.
func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 200 {
		http.Error(w, "Suchbegriff zu lang", http.StatusBadRequest)
		return
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		limit = 20
	}
	limit = min(limit, 100)
	hits, err := s.DB.Search(r.Context(), q)
	if writeErr(w, err) {
		return
	}
	var list []*scan.Item
	var score []float64
	bySeries := map[string]int{} // Serie → Index in list
	for _, h := range hits {
		it := s.Lib.Get(h.ID)
		if it == nil || !s.allowed(r, it) {
			continue
		}
		if it.Series != "" {
			if i, ok := bySeries[it.Series]; ok {
				// gleich gut: die früheste Folge vertritt die Serie
				if o := list[i]; h.Score == score[i] && (it.Season < o.Season || it.Season == o.Season && it.Episode < o.Episode) {
					list[i] = it
				}
				continue
			}
			if len(list) == limit {
				continue
			}
			bySeries[it.Series] = len(list)
		} else if len(list) == limit {
			continue
		}
		list, score = append(list, it), append(score, h.Score)
	}
	var p playback.Profile
	if dev := r.URL.Query().Get("device"); reDevice.MatchString(dev) {
		if b := s.DB.Device(r.Context(), dev); b != nil {
			json.Unmarshal(b, &p) // unlesbar: leeres Profil
		}
	}
	writeJSON(w, s.items(r, p, list))
}
