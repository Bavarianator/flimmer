package api

import (
	"encoding/json"
	"log"
	"net/http"
	"slices"

	"github.com/Bavarianator/flimmer/internal/images"
	"github.com/Bavarianator/flimmer/internal/meta"
	"github.com/Bavarianator/flimmer/internal/scan"
)

type personRole struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

type personPage struct {
	Name  string        `json:"name"`
	Image string        `json:"image,omitempty"`
	Bio   string        `json:"bio,omitempty"`
	Items []libraryItem `json:"items"` // Filme; je Serie die erste Folge
	Roles []personRole  `json:"roles"`
}

// forPeople ruft f für jeden sichtbaren Film und jede sichtbare Serie (vertreten durch die erste Folge) mit ihren
// Personen auf. Personen einer Serie stehen am Serien-Eintrag, aus Folgen-NFOs auch an einzelnen Folgen.
// ponytail: läuft über den ganzen Katalog (O(Titel × Personen)), reicht für Heimbibliotheken; sonst ein Index.
func (s *Server) forPeople(r *http.Request, f func(it *scan.Item, people []meta.Person)) {
	var shows map[string]*meta.Meta
	if s.Meta != nil {
		var err error
		if shows, err = s.Meta.Shows(r.Context()); err != nil {
			log.Printf("Serien lesen: %v", err)
		}
	}
	done := map[string]bool{}
	for _, it := range s.visible(r, s.Lib.All()) {
		m := s.Lib.MetaFor(it.ID)
		if it.Series == "" {
			if m != nil {
				f(it, m.People)
			}
			continue
		}
		if done[it.Series] {
			continue
		}
		done[it.Series] = true
		var people []meta.Person
		if show := shows[it.Series]; show != nil {
			people = show.People
		}
		for _, ep := range s.visible(r, s.Lib.Episodes(it.Series)) { // Gäste sehen evtl. nicht jede Folge
			if em := s.Lib.MetaFor(ep.ID); em != nil {
				people = append(slices.Clip(people), em.People...)
			}
		}
		f(it, people) // All() sortiert nach Staffel/Folge: it ist die erste sichtbare Folge
	}
}

// person: GET /api/people/{name}?device=
func (s *Server) person(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	out := personPage{Name: name, Roles: []personRole{}}
	var list []*scan.Item
	var found *meta.Person
	s.forPeople(r, func(it *scan.Item, people []meta.Person) {
		for _, p := range people {
			if p.Name != name {
				continue
			}
			if found == nil || found.Image == "" && p.Image != "" || found.TMDBID == 0 && p.TMDBID != 0 {
				found = &p
			}
			if len(list) == 0 || list[len(list)-1] != it {
				list = append(list, it)
			}
			out.Roles = append(out.Roles, personRole{ID: it.ID, Role: cmp(p.Role, p.Kind)})
		}
	})
	if found == nil {
		http.NotFound(w, r)
		return
	}
	if found.Image != "" {
		out.Image = personImageURL(*found)
	}
	if found.TMDBID != 0 && s.Meta != nil {
		out.Bio = s.Meta.Biography(r.Context(), found.TMDBID)
	}
	out.Items = s.items(r, s.deviceProfileOf(r), list)
	writeJSON(w, out)
}

// personImage: GET /api/images/person/{name}/profile?w= – wie Poster über den Bild-Cache; ohne Anmeldung (TVs).
func (s *Server) personImage(w http.ResponseWriter, r *http.Request) {
	name, src := r.PathValue("name"), ""
	s.forPeople(r, func(_ *scan.Item, people []meta.Person) {
		for _, p := range people {
			if src == "" && p.Name == name && p.Image != "" {
				src = p.Image
			}
		}
	})
	if src == "" {
		http.NotFound(w, r)
		return
	}
	local, err := s.Meta.PersonImage(r.Context(), src)
	if err != nil {
		log.Printf("Personenbild %s: %v", name, err)
		http.NotFound(w, r)
		return
	}
	s.Images.Serve(w, r, images.Source{Path: local})
}

// editMeta: PUT /api/items/{id}/meta (Admin). Geänderte Felder werden gesperrt, außer locked gibt die Liste vor.
func (s *Server) editMeta(w http.ResponseWriter, r *http.Request) {
	it := s.item(w, r)
	if it == nil {
		return
	}
	var body map[string]json.RawMessage
	if !readJSON(w, r, &body) {
		return
	}
	var locked []string
	if v, ok := body["locked"]; ok {
		if json.Unmarshal(v, &locked) != nil {
			http.Error(w, "locked muss eine Liste sein", http.StatusBadRequest)
			return
		}
		locked = append([]string{}, locked...) // [] entsperrt alles
		delete(body, "locked")
	}
	old := s.Lib.MetaFor(it.ID)
	if old == nil {
		old = &meta.Meta{Title: it.Title, Year: it.Year, Series: it.Series, Season: it.Season, Episode: it.Episode, Source: "filename"}
	}
	m, err := meta.Edit(old, body, locked)
	if err == nil {
		err = checkMeta(m, old)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if m.Source == "filename" {
		m.Source = "manual" // von Hand gepflegt: kein späterer Abgleich überschreibt es
	}
	m.Uncertain = false
	if writeErr(w, s.Meta.Save(r.Context(), it.ID, m)) {
		return
	}
	s.Lib.SetMeta(it.ID, m)
	writeJSON(w, s.detail(r, it))
}

// checkMeta prüft von Hand eingegebene Werte. Bilder von Personen kommen nie vom Client, sondern bleiben die bekannten.
func checkMeta(m, old *meta.Meta) error {
	if m.Title == "" {
		return errMsg("Der Titel darf nicht leer sein")
	}
	if m.Age != nil && !slices.Contains([]int{0, 6, 12, 16, 18}, *m.Age) {
		return errMsg("FSK muss 0, 6, 12, 16 oder 18 sein")
	}
	if m.Rating < 0 || m.Rating > 10 || m.Year < 0 || m.Year > 3000 {
		return errMsg("Bewertung (0–10) oder Jahr ungültig")
	}
	known := map[string]meta.Person{}
	for _, p := range old.People {
		known[p.Name] = p
	}
	people := m.People[:0]
	for _, p := range m.People {
		if p.Name == "" {
			continue
		}
		if !slices.Contains([]string{"actor", "director", "writer", "composer", "producer"}, p.Kind) {
			p.Kind = "actor"
		}
		p.Image, p.TMDBID = known[p.Name].Image, known[p.Name].TMDBID
		people = append(people, p)
	}
	m.People = people
	return nil
}
