package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/flimmer-media/flimmer/internal/auth"
	"github.com/flimmer-media/flimmer/internal/state"
)

const (
	cookieName = "flimmer"
	mediaTTL   = 6 * time.Hour
	sessionTTL = 180 * 24 * time.Hour // ungenutzte Sessions verfallen
)

type ctxKey struct{}

// userFrom liefert den angemeldeten Benutzer (Kopie) oder nil.
func userFrom(r *http.Request) *state.User {
	u, _ := r.Context().Value(ctxKey{}).(*state.User)
	return u
}

// Ohne Anmeldung erreichbar. Alles andere unter /api/ braucht Session, Bearer-Token oder Medien-Token.
var public = map[string]bool{
	"GET /api/status":        true,
	"GET /api/users":         true,
	"POST /api/login":        true,
	"POST /api/logout":       true,
	"POST /api/pair":         true,
	"GET /api/setup":         true, // Setup-Routen prüfen selbst: offen nur, solange kein Admin existiert
	"GET /api/setup/dirs":    true,
	"GET /api/setup/count":   true,
	"POST /api/setup":        true,
	"POST /api/setup/ffmpeg": true,
}

// authenticate hängt den Benutzer an den Request. Medien-URLs tragen das Token im Pfad
// (/api/m/<token>/items/…), weil <video src>, hls.js-Segmente und TVs weder Header noch Cookies schicken;
// relative Segment-URLs aus der Playlist behalten den Pfad-Präfix automatisch.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rest, ok := strings.CutPrefix(r.URL.Path, "/api/m/"); ok {
			tok, path, _ := strings.Cut(rest, "/")
			uid, ok := auth.CheckMediaToken(s.secret(), tok)
			u := s.user(uid)
			if !ok || u == nil || r.Method != http.MethodGet || !strings.HasPrefix(path, "items/") {
				http.Error(w, "Link abgelaufen – bitte neu starten", http.StatusUnauthorized)
				return
			}
			r2 := r.WithContext(context.WithValue(r.Context(), ctxKey{}, u))
			r2.URL.Path = "/api/" + path
			r2.URL.RawPath = ""
			next.ServeHTTP(w, r2)
			return
		}
		if u := s.sessionUser(r); u != nil {
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, u))
		}
		// GET /api/pair/{code} pollt der TV vor der Anmeldung.
		isPoll := r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/pair/") && strings.Count(r.URL.Path, "/") == 3
		// ponytail: Bilder öffentlich, weil <img> auf TVs kein Bearer schickt; enthalten nur Poster/Standbilder.
		isImage := r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/images/")
		if !strings.HasPrefix(r.URL.Path, "/api/") || userFrom(r) != nil || public[r.Method+" "+r.URL.Path] || isPoll || isImage {
			next.ServeHTTP(w, r)
			return
		}
		var setup bool
		s.State.View(func(d *state.Data) { setup = !d.SetupDone() })
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": "login", "setup": setup})
	})
}

func (s *Server) secret() (b []byte) {
	s.State.View(func(d *state.Data) { b = d.Settings.Secret })
	return b
}

func (s *Server) user(id string) *state.User {
	var out *state.User
	s.State.View(func(d *state.Data) {
		if u := d.User(id); u != nil {
			c := *u
			out = &c
		}
	})
	return out
}

func (s *Server) sessionUser(r *http.Request) *state.User {
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if tok == "" {
		if c, err := r.Cookie(cookieName); err == nil {
			tok = c.Value
		}
	}
	if tok == "" {
		return nil
	}
	h := auth.HashToken(tok)
	var out *state.User
	touch := false
	s.State.View(func(d *state.Data) {
		sess, ok := d.Sessions[h]
		if !ok || time.Since(sess.LastSeen) > sessionTTL {
			return
		}
		if u := d.User(sess.UserID); u != nil {
			c := *u
			out = &c
			touch = time.Since(sess.LastSeen) > time.Hour // nicht bei jedem Segment schreiben
		}
	})
	if touch {
		s.State.Update(func(d *state.Data) error {
			if sess, ok := d.Sessions[h]; ok {
				sess.LastSeen = time.Now()
				d.Sessions[h] = sess
			}
			return nil
		})
	}
	return out
}

// newSession legt eine Session an; im Browser zusätzlich als Cookie.
func (s *Server) newSession(w http.ResponseWriter, r *http.Request, userID, device string) string {
	tok, h := auth.NewToken()
	s.State.Update(func(d *state.Data) error {
		for k, sess := range d.Sessions { // aufräumen, sonst wächst state.json mit jedem Login
			if time.Since(sess.LastSeen) > sessionTTL {
				delete(d.Sessions, k)
			}
		}
		d.Sessions[h] = state.Session{UserID: userID, Device: device, LastSeen: time.Now()}
		return nil
	})
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: tok, Path: "/", MaxAge: int(sessionTTL.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil})
	return tok
}

type withToken struct {
	publicUser
	Token string `json:"token"`
}

type publicUser struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       int    `json:"color"`
	Admin       bool   `json:"admin"`
	HasPassword bool   `json:"hasPassword"`
}

func toPublic(u state.User) publicUser {
	return publicUser{ID: u.ID, Name: u.Name, Color: u.Color, Admin: u.Admin, HasPassword: u.PassHash != ""}
}

// users: Profilauswahl vor dem Login (Netflix-Stil).
func (s *Server) users(w http.ResponseWriter, r *http.Request) {
	out := []publicUser{}
	s.State.View(func(d *state.Data) {
		for _, u := range d.Users {
			out = append(out, toPublic(u))
		}
	})
	writeJSON(w, out)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, toPublic(*userFrom(r)))
}

// clientIP ist bewusst nur RemoteAddr: X-Forwarded-For ist fälschbar.
func clientIP(r *http.Request) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

// inLAN: Profile ohne Passwort gibt es nur im Heimnetz und nie hinter einem Reverse-Proxy
// (dort käme sonst jede Anfrage aus dem Internet scheinbar von 127.0.0.1).
func inLAN(r *http.Request) bool {
	ip := net.ParseIP(clientIP(r))
	proxied := r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("Forwarded") != "" || r.Header.Get("X-Real-IP") != ""
	return ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) && !proxied
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Device   string `json:"device"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	ip := clientIP(r)
	if !s.limiter.Allow(ip) {
		http.Error(w, "Zu viele Versuche – bitte eine Minute warten", http.StatusTooManyRequests)
		return
	}
	u := s.user(req.User)
	ok := u != nil && ((u.PassHash == "" && !u.Admin && inLAN(r)) || (u.PassHash != "" && auth.CheckPassword(u.PassHash, req.Password)))
	if !ok {
		s.limiter.Fail(ip)
		http.Error(w, "Name oder Passwort falsch", http.StatusUnauthorized)
		return
	}
	writeJSON(w, withToken{toPublic(*u), s.newSession(w, r, u.ID, req.Device)})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		h := auth.HashToken(c.Value)
		s.State.Update(func(d *state.Data) error { delete(d.Sessions, h); return nil })
	}
	if tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		h := auth.HashToken(tok)
		s.State.Update(func(d *state.Data) error { delete(d.Sessions, h); return nil })
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Path: "/", MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

// requireAdmin schreibt 403 und liefert false, wenn der Benutzer kein Admin ist.
func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if u := userFrom(r); u == nil || !u.Admin {
		http.Error(w, "Nur für Admins", http.StatusForbidden)
		return false
	}
	return true
}

func adminOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) {
			h(w, r)
		}
	}
}

type userReq struct {
	Name     *string `json:"name"`
	Password *string `json:"password"` // "" entfernt das Passwort (nicht für Admins)
	Admin    *bool   `json:"admin"`
	Color    *int    `json:"color"`
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req userReq
	if !readJSON(w, r, &req) {
		return
	}
	u := state.User{ID: state.NewID()}
	if err := applyUser(&u, req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.State.Update(func(d *state.Data) error { d.Users = append(d.Users, u); return nil })
	writeJSON(w, toPublic(u))
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	var req userReq
	if !readJSON(w, r, &req) {
		return
	}
	var out state.User
	err := s.State.Update(func(d *state.Data) error {
		u := d.User(r.PathValue("id"))
		if u == nil {
			return errNotFound
		}
		c := *u
		if err := applyUser(&c, req); err != nil {
			return err
		}
		if u.Admin && !c.Admin && admins(d) == 1 {
			return errMsg("Der letzte Admin kann nicht herabgestuft werden")
		}
		*u, out = c, c
		return nil
	})
	if !writeErr(w, err) {
		writeJSON(w, toPublic(out))
	}
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.State.Update(func(d *state.Data) error {
		u := d.User(id)
		if u == nil {
			return errNotFound
		}
		if u.Admin && admins(d) == 1 {
			return errMsg("Der letzte Admin kann nicht gelöscht werden")
		}
		d.Users = slicesDelete(d.Users, id)
		delete(d.Progress, id)
		delete(d.Prefs, id)
		for k, sess := range d.Sessions {
			if sess.UserID == id {
				delete(d.Sessions, k)
			}
		}
		return nil
	})
	if !writeErr(w, err) {
		w.WriteHeader(http.StatusNoContent)
	}
}

func applyUser(u *state.User, req userReq) error {
	if req.Name != nil {
		u.Name = strings.TrimSpace(*req.Name)
	}
	if u.Name == "" || len(u.Name) > 40 {
		return errMsg("Bitte einen Namen angeben (höchstens 40 Zeichen)")
	}
	if req.Admin != nil {
		u.Admin = *req.Admin
	}
	if req.Color != nil {
		u.Color = ((*req.Color % 360) + 360) % 360
	}
	if req.Password != nil {
		switch {
		case *req.Password == "":
			u.PassHash = ""
		case len(*req.Password) < 4:
			return errMsg("Das Passwort braucht mindestens 4 Zeichen")
		default:
			u.PassHash = auth.HashPassword(*req.Password)
		}
	}
	if u.Admin && u.PassHash == "" {
		return errMsg("Admins brauchen ein Passwort")
	}
	return nil
}

func admins(d *state.Data) int {
	n := 0
	for _, u := range d.Users {
		if u.Admin {
			n++
		}
	}
	return n
}

func slicesDelete(users []state.User, id string) []state.User {
	out := users[:0]
	for _, u := range users {
		if u.ID != id {
			out = append(out, u)
		}
	}
	return out
}

// --- TV-Kopplung ---

func (s *Server) pairStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Device string `json:"device"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	code, secret := s.pairing.Start(req.Device)
	writeJSON(w, map[string]any{"code": code, "secret": secret, "expiresIn": 600})
}

func (s *Server) pairPoll(w http.ResponseWriter, r *http.Request) {
	uid, device, done, ok := s.pairing.Poll(r.PathValue("code"), r.URL.Query().Get("secret"))
	switch {
	case !ok:
		http.Error(w, "Code abgelaufen", http.StatusNotFound)
	case !done:
		w.WriteHeader(http.StatusAccepted)
	default:
		u := s.user(uid)
		if u == nil {
			http.Error(w, "Benutzer existiert nicht mehr", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"token": s.newSession(w, r, uid, device), "user": toPublic(*u)})
	}
}

var reCode = regexp.MustCompile(`^\d{6}$`)

func (s *Server) pairConfirm(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	key := "pair:" + userFrom(r).ID
	if !s.limiter.Allow(key) {
		http.Error(w, "Zu viele Versuche – bitte eine Minute warten", http.StatusTooManyRequests)
		return
	}
	device, ok := s.pairing.Confirm(code, userFrom(r).ID)
	if !reCode.MatchString(code) || !ok {
		s.limiter.Fail(key)
		http.Error(w, "Code unbekannt oder abgelaufen", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"device": device})
}

// --- Hilfen ---

type errMsg string

func (e errMsg) Error() string { return string(e) }

var errNotFound = errMsg("nicht gefunden")

// writeErr schreibt err passend (404/400/500) und meldet, ob es einen Fehler gab.
func writeErr(w http.ResponseWriter, err error) bool {
	switch e := err.(type) {
	case nil:
		return false
	case errMsg:
		code := http.StatusBadRequest
		if e == errNotFound {
			code = http.StatusNotFound
		}
		http.Error(w, e.Error(), code)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return true
}

// readJSON liest höchstens 64 KB; sendBeacon schickt text/plain, deshalb kein Content-Type-Zwang.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(v); err != nil {
		http.Error(w, "ungültige Anfrage", http.StatusBadRequest)
		return false
	}
	return true
}
