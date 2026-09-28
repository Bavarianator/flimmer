package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/flimmer-media/flimmer/internal/auth"
	"github.com/flimmer-media/flimmer/internal/db"
)

const (
	cookieName = "flimmer"
	mediaTTL   = 6 * time.Hour
	sessionTTL = 180 * 24 * time.Hour // ungenutzte Sessions verfallen
)

type ctxKey struct{}

// userFrom liefert den angemeldeten Benutzer (Kopie) oder nil.
func userFrom(r *http.Request) *db.User {
	u, _ := r.Context().Value(ctxKey{}).(*db.User)
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
			u := s.user(r.Context(), uid)
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
		setup := !s.setupDone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": "login", "setup": setup})
	})
}

func (s *Server) secret() []byte {
	set, err := s.DB.Settings(context.Background())
	if err != nil {
		log.Printf("Einstellungen lesen: %v", err)
	}
	return set.Secret
}

func (s *Server) user(ctx context.Context, id string) *db.User {
	u, err := s.DB.User(ctx, id)
	if err != nil {
		log.Printf("Benutzer lesen: %v", err)
	}
	return u
}

func (s *Server) sessionUser(r *http.Request) *db.User {
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if tok == "" {
		if c, err := r.Cookie(cookieName); err == nil {
			tok = c.Value
		}
	}
	if tok == "" {
		return nil
	}
	u, err := s.DB.SessionUser(r.Context(), auth.HashToken(tok), sessionTTL)
	if err != nil {
		log.Printf("Session prüfen: %v", err)
	}
	return u
}

// newSession legt eine Session an; im Browser zusätzlich als Cookie.
func (s *Server) newSession(w http.ResponseWriter, r *http.Request, userID, device string) (string, error) {
	tok, h := auth.NewToken()
	if err := s.DB.CreateSession(r.Context(), h, userID, device, sessionTTL); err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: tok, Path: "/", MaxAge: int(sessionTTL.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil})
	return tok, nil
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

func toPublic(u db.User) publicUser {
	return publicUser{ID: u.ID, Name: u.Name, Color: u.Color, Admin: u.Admin, HasPassword: u.PassHash != ""}
}

// users: Profilauswahl vor dem Login (Netflix-Stil).
func (s *Server) users(w http.ResponseWriter, r *http.Request) {
	all, err := s.DB.Users(r.Context())
	if writeErr(w, err) {
		return
	}
	out := []publicUser{}
	for _, u := range all {
		out = append(out, toPublic(u))
	}
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
	u := s.user(r.Context(), req.User)
	ok := u != nil && ((u.PassHash == "" && !u.Admin && inLAN(r)) || (u.PassHash != "" && auth.CheckPassword(u.PassHash, req.Password)))
	if !ok {
		s.limiter.Fail(ip)
		http.Error(w, "Name oder Passwort falsch", http.StatusUnauthorized)
		return
	}
	tok, err := s.newSession(w, r, u.ID, req.Device)
	if !writeErr(w, err) {
		writeJSON(w, withToken{toPublic(*u), tok})
	}
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		s.DB.DeleteSession(r.Context(), auth.HashToken(c.Value))
	}
	if tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		s.DB.DeleteSession(r.Context(), auth.HashToken(tok))
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
	u := db.User{ID: newID()}
	if err := applyUser(&u, req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !writeErr(w, s.DB.CreateUser(r.Context(), u)) {
		writeJSON(w, toPublic(u))
	}
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	var req userReq
	if !readJSON(w, r, &req) {
		return
	}
	out, err := s.DB.UpdateUser(r.Context(), r.PathValue("id"), func(u *db.User, admins int) error {
		wasAdmin := u.Admin
		if err := applyUser(u, req); err != nil {
			return err
		}
		if wasAdmin && !u.Admin && admins == 1 {
			return errMsg("Der letzte Admin kann nicht herabgestuft werden")
		}
		return nil
	})
	if !writeErr(w, err) {
		writeJSON(w, toPublic(out))
	}
}

// deleteUser löscht samt Sessions, Fortschritt und Sprachwahl (Fremdschlüssel mit CASCADE).
func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	err := s.DB.DeleteUser(r.Context(), r.PathValue("id"), func(u db.User, admins int) error {
		if u.Admin && admins == 1 {
			return errMsg("Der letzte Admin kann nicht gelöscht werden")
		}
		return nil
	})
	if !writeErr(w, err) {
		w.WriteHeader(http.StatusNoContent)
	}
}

func applyUser(u *db.User, req userReq) error {
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

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
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
		u := s.user(r.Context(), uid)
		if u == nil {
			http.Error(w, "Benutzer existiert nicht mehr", http.StatusNotFound)
			return
		}
		tok, err := s.newSession(w, r, uid, device)
		if !writeErr(w, err) {
			writeJSON(w, map[string]any{"token": tok, "user": toPublic(*u)})
		}
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
	if errors.Is(err, db.ErrNotFound) {
		err = errNotFound
	}
	if errors.Is(err, db.ErrSetupDone) {
		err = errMsg(err.Error())
	}
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
