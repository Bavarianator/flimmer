package api

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Bavarianator/flimmer/internal/db"
	"github.com/Bavarianator/flimmer/internal/mediathek"
)

// Mediathek (Dashboard › Mediathek, nur Admins): Sendungen von ARD, ZDF, arte … suchen, laden und abonnieren.

// GET /api/mediathek/suche?q=&sender=&min=&offset=
func (s *Server) mediathekSuche(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	min, _ := strconv.Atoi(q.Get("min"))
	off, _ := strconv.Atoi(q.Get("offset"))
	treffer, gesamt, err := mediathek.Suche(r.Context(), mediathek.Anfrage{Text: q.Get("q"), Sender: q.Get("sender"), MinMinuten: min, Offset: off})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"treffer": treffer, "gesamt": gesamt})
}

// POST /api/mediathek/laden {"treffer": Treffer}
func (s *Server) mediathekLaden(w http.ResponseWriter, r *http.Request) {
	var req struct{ Treffer mediathek.Treffer }
	if !readJSON(w, r, &req) {
		return
	}
	if p, err := url.Parse(req.Treffer.Video); err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" {
		http.Error(w, "Treffer ohne Video-Link", http.StatusBadRequest)
		return
	}
	dl, err := s.Mediathek.Laden(r.Context(), req.Treffer, "mediathek", userFrom(r).ID)
	if errors.Is(err, db.ErrSchonDa) {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if writeErr(w, err) {
		return
	}
	writeJSON(w, dl)
}

// GET /api/mediathek/downloads
func (s *Server) mediathekDownloads(w http.ResponseWriter, r *http.Request) {
	l, err := s.Mediathek.Downloads(r.Context())
	if !writeErr(w, err) {
		writeJSON(w, l)
	}
}

// DELETE /api/mediathek/downloads/{id}: wartend/laufend → abgebrochen, sonst aus der Liste (die Datei bleibt).
func (s *Server) mediathekAbbrechen(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if !writeErr(w, s.Mediathek.Abbrechen(r.Context(), id)) {
		w.WriteHeader(http.StatusNoContent)
	}
}

// GET /api/mediathek/abos
func (s *Server) mediathekAbos(w http.ResponseWriter, r *http.Request) {
	l, err := s.DB.Abos(r.Context())
	if !writeErr(w, err) {
		writeJSON(w, l)
	}
}

// POST /api/mediathek/abos {"text", "sender", "minMinuten"}: prüft gleich danach alle Abos.
func (s *Server) mediathekAboNeu(w http.ResponseWriter, r *http.Request) {
	var a db.Abo
	if !readJSON(w, r, &a) {
		return
	}
	if a.Text = strings.TrimSpace(a.Text); a.Text == "" || len(a.Text) > 200 {
		http.Error(w, "Bitte einen Suchbegriff für das Abo", http.StatusBadRequest)
		return
	}
	a, err := s.DB.AddAbo(r.Context(), a)
	if writeErr(w, err) {
		return
	}
	s.Mediathek.Kick()
	writeJSON(w, a)
}

// DELETE /api/mediathek/abos/{id}: Bereits geladene Sendungen bleiben.
func (s *Server) mediathekAboWeg(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if !writeErr(w, s.DB.DeleteAbo(r.Context(), id)) {
		w.WriteHeader(http.StatusNoContent)
	}
}
