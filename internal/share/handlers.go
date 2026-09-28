package share

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Bavarianator/flimmer/internal/db"
	"github.com/Bavarianator/flimmer/internal/discovery"
)

// CreateHandler: POST /api/invites, nur Admin.
// Body {note, libraries, items, hours, maxUses} → {invite, url, qr (PNG als data-URL), hint}.
func (s *Share) CreateHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Note      string   `json:"note"`
		Libraries []string `json:"libraries"`
		Items     []string `json:"items"`
		Hours     int      `json:"hours"`
		MaxUses   int      `json:"maxUses"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "ungültige Anfrage")
		return
	}
	by := ""
	if s.opts.UserID != nil {
		by = s.opts.UserID(r)
	}
	token, inv, err := s.Create(r.Context(), by, req.Note, Scope{Libraries: req.Libraries, Items: req.Items},
		time.Duration(req.Hours)*time.Hour, req.MaxUses)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	base, public := s.opts.BaseURL()
	// Token im Fragment: landet nicht in Server-Logs, Proxys oder im Referer.
	link := strings.TrimSuffix(base, "/") + "/einladung#" + token
	var hint string
	switch {
	case !public:
		hint = "Dieser Link funktioniert nur im Heimnetz. Schalte in den Einstellungen den Fernzugriff ein, damit Gäste von unterwegs schauen können."
	case strings.HasPrefix(link, "http://"):
		hint = "Die Verbindung ist noch unverschlüsselt. Sobald das HTTPS-Zertifikat da ist, wird der Link sicher."
	}
	out := map[string]any{"invite": inv, "url": link, "hint": hint}
	if png, err := discovery.QR(link); err == nil {
		out["qr"] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	}
	writeJSON(w, out)
}

// ListHandler: GET /api/invites, nur Admin. Tokens sind nicht mehr abrufbar, nur ihre Verwaltungsdaten.
func (s *Share) ListHandler(w http.ResponseWriter, r *http.Request) {
	list, err := s.List(r.Context())
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, list)
}

// RevokeHandler: DELETE /api/invites/{id}, nur Admin.
func (s *Share) RevokeHandler(w http.ResponseWriter, r *http.Request) {
	switch err := s.Revoke(r.Context(), r.PathValue("id")); {
	case errors.Is(err, db.ErrNotFound):
		fail(w, http.StatusNotFound, "Einladung nicht gefunden")
	case err != nil:
		fail(w, http.StatusInternalServerError, err.Error())
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// RedeemHandler: POST /api/invites/redeem, OHNE Anmeldung. Body {token, name} → legt den Gast an und meldet ihn an.
func (s *Share) RedeemHandler(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.Allow(s.opts.ClientIP(r), s.now()) {
		w.Header().Set("Retry-After", "60")
		fail(w, http.StatusTooManyRequests, "Zu viele Versuche, bitte in einer Minute noch einmal.")
		return
	}
	var req struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "ungültige Anfrage")
		return
	}
	u, err := s.Redeem(r.Context(), req.Token, req.Name)
	if errors.Is(err, ErrInvalid) {
		fail(w, http.StatusGone, err.Error())
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.opts.Login(w, r, u); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"id": u.ID, "name": u.Name})
}

func fail(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
