// Package share verwaltet Einladungen: Link und QR mit HMAC-signiertem Token, befristet, mit Scope
// (Bibliotheken bzw. einzelne Titel), maximaler Nutzungszahl und Widerruf. Beim Einlösen entsteht ein Gast
// (kein Admin, sieht nur den Scope). Mit Passwort kann sich der Gast später per Name und Passwort auf anderen
// Geräten anmelden, auch von außerhalb. Durchgesetzt wird der Scope in internal/api über Scope/Allows.
package share

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/Bavarianator/flimmer/internal/db"
	"github.com/Bavarianator/flimmer/internal/discovery"
	"github.com/Bavarianator/flimmer/internal/ratelimit"
)

// Scope sagt, was ein Gast sehen darf.
type Scope struct {
	Libraries []string `json:"libraries,omitempty"` // Bibliothekspfade (aus den Einstellungen)
	Items     []string `json:"items,omitempty"`     // einzelne Titel
}

// Allows: Darf der Gast diesen Titel sehen? nil-Scope = normaler Benutzer, alles erlaubt.
func (s *Scope) Allows(itemID, path string) bool {
	if s == nil {
		return true
	}
	if slices.Contains(s.Items, itemID) {
		return true
	}
	for _, l := range s.Libraries {
		if within(path, l) {
			return true
		}
	}
	return false
}

// within prüft auf Pfad-Ebene: /media/filme erlaubt nicht /media/filme-privat.
func within(path, dir string) bool {
	path, dir = filepath.Clean(path), filepath.Clean(dir)
	return path == dir || strings.HasPrefix(path, dir+string(filepath.Separator))
}

type Invite struct {
	ID        string    `json:"id"`
	Note      string    `json:"note"`
	Scope     Scope     `json:"scope"`
	Expires   time.Time `json:"expires"`
	MaxUses   int       `json:"maxUses"` // 0 = unbegrenzt
	Uses      int       `json:"uses"`
	Guests    int       `json:"guests"`
	CreatedBy string    `json:"createdBy"`
	Created   time.Time `json:"created"`
}

type Options struct {
	DB *db.DB
	// BaseURL liefert die Adresse für Links: remote.Status().PublicURL, wenn Fernzugriff läuft (public=true),
	// sonst die LAN-Adresse. Ohne Angabe: discovery.LANURL(Port).
	BaseURL func() (url string, public bool)
	Port    int
	// Login meldet den neuen Gast an (Session-Cookie von internal/api) und liefert das Token für Apps.
	Login func(w http.ResponseWriter, r *http.Request, u db.User) (string, error)
	// UserID liefert den angemeldeten Admin einer Anfrage (für created_by).
	UserID func(r *http.Request) string
	// ClientIP für das Rate-Limit beim Einlösen; Standard RemoteAddr.
	ClientIP func(r *http.Request) net.IP
}

type Share struct {
	opts    Options
	limiter ratelimit.Limiter
	now     func() time.Time
}

var (
	ErrInvalid = errors.New("Einladung ungültig, abgelaufen oder aufgebraucht")
	ErrExpired = errors.New("Gastzugang abgelaufen")
	ErrName    = errors.New("Diesen Namen gibt es schon – bitte einen anderen wählen")
)

const (
	maxTTL     = 90 * 24 * time.Hour
	maxUsesCap = 100
)

func New(opts Options) *Share {
	if opts.BaseURL == nil {
		opts.BaseURL = func() (string, bool) { return discovery.LANURL(opts.Port), false }
	}
	if opts.ClientIP == nil {
		opts.ClientIP = func(r *http.Request) net.IP {
			host, _, _ := net.SplitHostPort(r.RemoteAddr)
			return net.ParseIP(host)
		}
	}
	// Einlösen: 5 Versuche am Stück, dann einer pro Minute – Raten eines 128-Bit-Tokens ist ohnehin aussichtslos,
	// das Limit bremst vor allem das massenhafte Anlegen von Gästen mit einem geleakten Link.
	return &Share{opts: opts, limiter: ratelimit.Limiter{Burst: 5, Rate: 1.0 / 60}, now: time.Now}
}

// --- Token: base64url(Kern 16 B ‖ HMAC-SHA256(k, Kern)[:16]) mit k = HMAC(secret, "invite") ---
// Der eigene Schlüssel k trennt Einladungen von Medien-Tokens: keine Signatur taugt für beides.

func mac(secret, core []byte) []byte {
	k := hmac.New(sha256.New, secret)
	k.Write([]byte("invite"))
	h := hmac.New(sha256.New, k.Sum(nil))
	h.Write(core)
	return h.Sum(nil)[:16]
}

func (s *Share) secret(ctx context.Context) ([]byte, error) {
	st, err := s.opts.DB.Settings(ctx)
	if err != nil {
		return nil, err
	}
	if len(st.Secret) < 16 {
		return nil, errors.New("Server-Geheimnis fehlt")
	}
	return st.Secret, nil
}

func key(core []byte) string {
	sum := sha256.Sum256(core)
	return hex.EncodeToString(sum[:])
}

// parse prüft die Signatur (ohne DB-Zugriff) und liefert den DB-Schlüssel.
func (s *Share) parse(ctx context.Context, token string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil || len(raw) != 32 {
		return "", ErrInvalid
	}
	secret, err := s.secret(ctx)
	if err != nil {
		return "", err
	}
	if !hmac.Equal(raw[16:], mac(secret, raw[:16])) {
		return "", ErrInvalid
	}
	return key(raw[:16]), nil
}

// --- Verwaltung ---

// Create legt eine Einladung an und liefert das Token – es wird nur jetzt einmal gezeigt (in der DB steht der Hash).
func (s *Share) Create(ctx context.Context, by, note string, scope Scope, ttl time.Duration, maxUses int) (string, Invite, error) {
	if ttl < time.Hour || ttl > maxTTL {
		return "", Invite{}, errors.New("Gültigkeit muss zwischen 1 Stunde und 90 Tagen liegen")
	}
	if maxUses < 0 || maxUses > maxUsesCap {
		return "", Invite{}, fmt.Errorf("höchstens %d Nutzungen", maxUsesCap)
	}
	if len(scope.Libraries)+len(scope.Items) == 0 {
		return "", Invite{}, errors.New("Bitte mindestens eine Bibliothek oder einen Titel freigeben")
	}
	st, err := s.opts.DB.Settings(ctx)
	if err != nil {
		return "", Invite{}, err
	}
	for _, l := range scope.Libraries {
		if !slices.Contains(st.Dirs, l) {
			return "", Invite{}, fmt.Errorf("unbekannte Bibliothek %q", l)
		}
	}
	for _, id := range scope.Items {
		var n int
		if s.opts.DB.QueryRowContext(ctx, "SELECT count(*) FROM items WHERE id = ?", id).Scan(&n); n == 0 {
			return "", Invite{}, fmt.Errorf("unbekannter Titel %q", id)
		}
	}
	secret, err := s.secret(ctx)
	if err != nil {
		return "", Invite{}, err
	}
	core := make([]byte, 16)
	rand.Read(core)
	token := base64.RawURLEncoding.EncodeToString(append(core, mac(secret, core)...))
	now := s.now()
	inv := Invite{ID: key(core), Note: trimName(note, 200), Scope: scope, Expires: now.Add(ttl), MaxUses: maxUses,
		CreatedBy: by, Created: now}
	js, _ := json.Marshal(scope)
	_, err = s.opts.DB.ExecContext(ctx, `INSERT INTO invites(id, created_by, note, scope, expires, max_uses, created_at)
VALUES(?, ?, ?, ?, ?, ?, ?)`, inv.ID, by, inv.Note, string(js), inv.Expires.UnixMilli(), maxUses, now.UnixMilli())
	return token, inv, err
}

func (s *Share) List(ctx context.Context) ([]Invite, error) {
	rows, err := s.opts.DB.QueryContext(ctx, `SELECT i.id, i.note, i.scope, i.expires, i.max_uses, i.uses, i.created_by, i.created_at,
(SELECT count(*) FROM guests g WHERE g.invite_id = i.id) FROM invites i ORDER BY i.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Invite{}
	for rows.Next() {
		var inv Invite
		var scope string
		var exp, created int64
		if err := rows.Scan(&inv.ID, &inv.Note, &scope, &exp, &inv.MaxUses, &inv.Uses, &inv.CreatedBy, &created, &inv.Guests); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(scope), &inv.Scope)
		inv.Expires, inv.Created = time.UnixMilli(exp), time.UnixMilli(created)
		out = append(out, inv)
	}
	return out, rows.Err()
}

// Revoke widerruft eine Einladung: ihre Gäste werden gelöscht (samt Sessions und Fortschritt), dann sie selbst.
func (s *Share) Revoke(ctx context.Context, id string) error {
	tx, err := s.opts.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM users WHERE id IN (SELECT user_id FROM guests WHERE invite_id = ?)", id); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, "DELETE FROM invites WHERE id = ?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return db.ErrNotFound
	}
	return tx.Commit()
}

// Redeem löst eine Einladung ein und legt einen Gast an. Die Nutzung wird atomar gezählt, sodass max_uses
// auch bei gleichzeitigen Anfragen hält. passHash leer = Gast ohne Passwort (nur diese Session).
func (s *Share) Redeem(ctx context.Context, token, name, passHash string) (db.User, error) {
	k, err := s.parse(ctx, token)
	if err != nil {
		return db.User{}, err
	}
	name = trimName(name, 40)
	if name == "" {
		name = "Gast"
	}
	if passHash != "" { // Anmeldung per Name: der Name muss eindeutig sein
		all, err := s.opts.DB.Users(ctx)
		if err != nil {
			return db.User{}, err
		}
		for _, u := range all {
			if strings.EqualFold(u.Name, name) {
				return db.User{}, ErrName
			}
		}
	}
	res, err := s.opts.DB.ExecContext(ctx, `UPDATE invites SET uses = uses + 1
WHERE id = ? AND expires > ? AND (max_uses = 0 OR uses < max_uses)`, k, s.now().UnixMilli())
	if err != nil {
		return db.User{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return db.User{}, ErrInvalid
	}
	id := make([]byte, 8)
	rand.Read(id)
	u := db.User{ID: hex.EncodeToString(id), Name: name, PassHash: passHash}
	err = s.opts.DB.CreateUser(ctx, u)
	if err == nil {
		_, err = s.opts.DB.ExecContext(ctx, "INSERT INTO guests(user_id, invite_id) VALUES(?, ?)", u.ID, k)
		if err != nil {
			s.opts.DB.ExecContext(ctx, "DELETE FROM users WHERE id = ?", u.ID)
		}
	}
	if err != nil {
		s.opts.DB.ExecContext(ctx, "UPDATE invites SET uses = uses - 1 WHERE id = ?", k)
		return db.User{}, err
	}
	return u, nil
}

// Scope liefert die Einschränkung eines Benutzers: nil für normale Benutzer, ErrExpired für abgelaufene Gäste.
func (s *Share) Scope(ctx context.Context, userID string) (*Scope, error) {
	var scope string
	var exp int64
	err := s.opts.DB.QueryRowContext(ctx, `SELECT i.scope, i.expires FROM guests g JOIN invites i ON i.id = g.invite_id
WHERE g.user_id = ?`, userID).Scan(&scope, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if s.now().UnixMilli() >= exp {
		return nil, ErrExpired
	}
	var sc Scope
	if err := json.Unmarshal([]byte(scope), &sc); err != nil {
		return nil, err
	}
	return &sc, nil
}

// IsGuest: Gäste gehören nicht in die Profilauswahl und dürfen sich nie ohne Session anmelden.
func (s *Share) IsGuest(ctx context.Context, userID string) bool {
	var n int
	s.opts.DB.QueryRowContext(ctx, "SELECT count(*) FROM guests WHERE user_id = ?", userID).Scan(&n)
	return n > 0
}

// Cleanup löscht Gäste abgelaufener Einladungen und Einladungen, die seit 30 Tagen abgelaufen sind.
func (s *Share) Cleanup(ctx context.Context) error {
	now := s.now()
	_, err := s.opts.DB.ExecContext(ctx, `DELETE FROM users WHERE id IN
(SELECT g.user_id FROM guests g JOIN invites i ON i.id = g.invite_id WHERE i.expires <= ?)`, now.UnixMilli())
	if err != nil {
		return err
	}
	_, err = s.opts.DB.ExecContext(ctx, "DELETE FROM invites WHERE expires <= ?", now.Add(-30*24*time.Hour).UnixMilli())
	return err
}

// Run räumt stündlich auf, bis ctx endet.
func (s *Share) Run(ctx context.Context) {
	for {
		s.Cleanup(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Hour):
		}
	}
}

// trimName entfernt Steuerzeichen und kürzt auf n Zeichen.
func trimName(s string, n int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	if r := []rune(s); len(r) > n {
		s = string(r[:n])
	}
	return s
}
