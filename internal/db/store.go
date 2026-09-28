package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// --- Einstellungen ---

// Optimize steuert die Hintergrund-Optimierung (MP4-Version für Titel, die sonst transkodiert würden).
type Optimize struct {
	Off       bool `json:"off"`  // Standard: an
	From      int  `json:"from"` // volle Stunden; beide 0 = Standard 2–6 Uhr
	To        int  `json:"to"`
	MinFreeGB int  `json:"minFreeGB"` // 0 = Standard 20 GB
}

// Window liefert das Zeitfenster mit Standardwerten.
func (o Optimize) Window() (from, to int, on bool) {
	if o.From == 0 && o.To == 0 {
		return 2, 6, !o.Off
	}
	return o.From, o.To, !o.Off
}

type Settings struct {
	ServerName string
	Language   string
	Dirs       []string
	TMDBKey    string // leer = eingebauter Projekt-Key
	NoUpdates  bool   // Update-Hinweis abgeschaltet
	Remote     bool   // Fernzugriff eingeschaltet (Portfreigabe im Router)
	Optimize   Optimize
	Secret     []byte // HMAC-Schlüssel für Medien-Tokens
}

func (d *DB) Settings(ctx context.Context) (Settings, error) {
	return settings(ctx, d.DB)
}

type querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
}

func settings(ctx context.Context, q querier) (Settings, error) {
	var s Settings
	rows, err := q.QueryContext(ctx, "SELECT key, value FROM settings")
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return s, err
		}
		switch k {
		case "server_name":
			s.ServerName = v
		case "language":
			s.Language = v
		case "tmdb_key":
			s.TMDBKey = v
		case "no_updates":
			s.NoUpdates = v == "1"
		case "remote":
			s.Remote = v == "1"
		case "opt_off":
			s.Optimize.Off = v == "1"
		case "opt_from":
			s.Optimize.From, _ = strconv.Atoi(v)
		case "opt_to":
			s.Optimize.To, _ = strconv.Atoi(v)
		case "opt_min_free_gb":
			s.Optimize.MinFreeGB, _ = strconv.Atoi(v)
		case "secret":
			s.Secret, _ = hex.DecodeString(v)
		}
	}
	if err := rows.Err(); err != nil {
		return s, err
	}
	s.Dirs = []string{}
	libs, err := q.QueryContext(ctx, "SELECT path FROM libraries ORDER BY id")
	if err != nil {
		return s, err
	}
	defer libs.Close()
	for libs.Next() {
		var p string
		if err := libs.Scan(&p); err != nil {
			return s, err
		}
		s.Dirs = append(s.Dirs, p)
	}
	return s, libs.Err()
}

// UpdateSettings liest, ändert und schreibt in einer Transaktion.
func (d *DB) UpdateSettings(ctx context.Context, f func(s *Settings)) error {
	return d.tx(ctx, func(tx *sql.Tx) error {
		s, err := settings(ctx, tx)
		if err != nil {
			return err
		}
		f(&s)
		return writeSettings(ctx, tx, s)
	})
}

func writeSettings(ctx context.Context, tx *sql.Tx, s Settings) error {
	kv := map[string]string{"server_name": s.ServerName, "language": s.Language, "tmdb_key": s.TMDBKey,
		"no_updates": fmt.Sprint(b2i(s.NoUpdates)), "remote": fmt.Sprint(b2i(s.Remote)), "secret": hex.EncodeToString(s.Secret),
		"opt_off": fmt.Sprint(b2i(s.Optimize.Off)), "opt_from": fmt.Sprint(s.Optimize.From), "opt_to": fmt.Sprint(s.Optimize.To),
		"opt_min_free_gb": fmt.Sprint(s.Optimize.MinFreeGB)}
	for k, v := range kv {
		if _, err := tx.ExecContext(ctx, "INSERT INTO settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", k, v); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM libraries"); err != nil {
		return err
	}
	for _, p := range s.Dirs {
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO libraries(path) VALUES(?)", p); err != nil {
			return err
		}
	}
	return nil
}

// ensureSecret legt den HMAC-Schlüssel beim ersten Start an.
func (d *DB) ensureSecret(ctx context.Context) error {
	b := make([]byte, 32)
	rand.Read(b)
	_, err := d.ExecContext(ctx, "INSERT OR IGNORE INTO settings(key, value) VALUES('secret', ?)", hex.EncodeToString(b))
	return err
}

// --- Benutzer ---

type User struct {
	ID       string
	Name     string
	Color    int
	Admin    bool
	PassHash string // leer = Profil ohne Passwort (nur im Heimnetz)
}

const userCols = "id, name, color, admin, pass_hash"

func scanUser(sc interface{ Scan(...any) error }) (User, error) {
	var u User
	err := sc.Scan(&u.ID, &u.Name, &u.Color, &u.Admin, &u.PassHash)
	return u, err
}

func (d *DB) Users(ctx context.Context) ([]User, error) {
	rows, err := d.QueryContext(ctx, "SELECT "+userCols+" FROM users ORDER BY created_at, name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// User liefert nil ohne Fehler, wenn es den Benutzer nicht gibt.
func (d *DB) User(ctx context.Context, id string) (*User, error) {
	u, err := scanUser(d.QueryRowContext(ctx, "SELECT "+userCols+" FROM users WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

// SetupDone: Es gibt einen Admin, die Einrichtung ist abgeschlossen.
func (d *DB) SetupDone(ctx context.Context) bool {
	var n int
	d.QueryRowContext(ctx, "SELECT count(*) FROM users WHERE admin = 1").Scan(&n)
	return n > 0
}

var ErrSetupDone = errors.New("Flimmer ist bereits eingerichtet")

// Setup legt den ersten Admin samt Einstellungen an – atomar, damit ein zweiter Tab nicht auch Admin wird.
func (d *DB) Setup(ctx context.Context, admin User, f func(s *Settings)) error {
	return d.tx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM users WHERE admin = 1").Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrSetupDone
		}
		if err := insertUser(ctx, tx, admin); err != nil {
			return err
		}
		s, err := settings(ctx, tx)
		if err != nil {
			return err
		}
		f(&s)
		return writeSettings(ctx, tx, s)
	})
}

func (d *DB) CreateUser(ctx context.Context, u User) error { return insertUser(ctx, d.DB, u) }

func insertUser(ctx context.Context, q querier, u User) error {
	_, err := q.ExecContext(ctx, "INSERT INTO users("+userCols+", created_at) VALUES(?, ?, ?, ?, ?, ?)",
		u.ID, u.Name, u.Color, b2i(u.Admin), u.PassHash, now())
	return err
}

var ErrNotFound = errors.New("nicht gefunden")

// UpdateUser ändert einen Benutzer in einer Transaktion; f sieht die Zahl der Admins (Schutz des letzten Admins).
func (d *DB) UpdateUser(ctx context.Context, id string, f func(u *User, admins int) error) (User, error) {
	var out User
	err := d.tx(ctx, func(tx *sql.Tx) error {
		u, admins, err := userForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := f(&u, admins); err != nil {
			return err
		}
		out = u
		_, err = tx.ExecContext(ctx, "UPDATE users SET name = ?, color = ?, admin = ?, pass_hash = ? WHERE id = ?",
			u.Name, u.Color, b2i(u.Admin), u.PassHash, id)
		return err
	})
	return out, err
}

// DeleteUser löscht samt Sessions, Fortschritt und Sprachwahl (ON DELETE CASCADE).
func (d *DB) DeleteUser(ctx context.Context, id string, check func(u User, admins int) error) error {
	return d.tx(ctx, func(tx *sql.Tx) error {
		u, admins, err := userForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := check(u, admins); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
		return err
	})
}

func userForUpdate(ctx context.Context, tx *sql.Tx, id string) (User, int, error) {
	u, err := scanUser(tx.QueryRowContext(ctx, "SELECT "+userCols+" FROM users WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return u, 0, ErrNotFound
	}
	if err != nil {
		return u, 0, err
	}
	var admins int
	err = tx.QueryRowContext(ctx, "SELECT count(*) FROM users WHERE admin = 1").Scan(&admins)
	return u, admins, err
}

// --- Sessions ---

// CreateSession speichert den Token-Hash und räumt abgelaufene Sessions weg.
func (d *DB) CreateSession(ctx context.Context, hash, userID, device string, ttl time.Duration) error {
	return d.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE last_seen < ?", now()-ttl.Milliseconds()); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO sessions(hash, user_id, device, last_seen) VALUES(?, ?, ?, ?)", hash, userID, device, now())
		return err
	})
}

// SessionUser liefert den Benutzer einer gültigen Session (nil, wenn keine) und hält sie am Leben.
func (d *DB) SessionUser(ctx context.Context, hash string, ttl time.Duration) (*User, error) {
	var lastSeen int64
	row := d.QueryRowContext(ctx, "SELECT u.id, u.name, u.color, u.admin, u.pass_hash, s.last_seen FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.hash = ?", hash)
	var u User
	err := row.Scan(&u.ID, &u.Name, &u.Color, &u.Admin, &u.PassHash, &lastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	age := time.Since(toTime(lastSeen))
	if age > ttl {
		return nil, nil
	}
	if age > time.Hour { // nicht bei jedem Segment schreiben
		d.ExecContext(ctx, "UPDATE sessions SET last_seen = ? WHERE hash = ?", now(), hash)
	}
	return &u, nil
}

func (d *DB) DeleteSession(ctx context.Context, hash string) error {
	_, err := d.ExecContext(ctx, "DELETE FROM sessions WHERE hash = ?", hash)
	return err
}

// --- Fortschritt ---

type Progress struct {
	Pos     float64   `json:"pos"`
	Dur     float64   `json:"dur"`
	Watched bool      `json:"watched"`
	Updated time.Time `json:"updated"`
}

// SetProgress wendet die Regeln an: unter 3 % gilt als nicht angefangen, ab 90 % als gesehen;
// kurzes Reinschauen macht „gesehen“ nicht rückgängig.
func (d *DB) SetProgress(ctx context.Context, user, item string, pos, dur float64) (Progress, error) {
	p := Progress{Pos: pos, Dur: dur, Updated: time.Now()}
	switch {
	case dur > 0 && pos/dur >= 0.9:
		p.Pos, p.Watched = 0, true
	case dur > 0 && pos/dur < 0.03:
		p.Pos = 0
	}
	err := d.tx(ctx, func(tx *sql.Tx) error {
		if p.Pos == 0 && !p.Watched {
			var was bool
			err := tx.QueryRowContext(ctx, "SELECT watched FROM progress WHERE user_id = ? AND item_id = ?", user, item).Scan(&was)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if !was {
				_, err := tx.ExecContext(ctx, "DELETE FROM progress WHERE user_id = ? AND item_id = ?", user, item)
				return err
			}
			p.Watched = true
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO progress(user_id, item_id, pos, duration, watched, updated_at) VALUES(?, ?, ?, ?, ?, ?)
			ON CONFLICT(user_id, item_id) DO UPDATE SET pos = excluded.pos, duration = excluded.duration, watched = excluded.watched, updated_at = excluded.updated_at`,
			user, item, p.Pos, p.Dur, b2i(p.Watched), p.Updated.UnixMilli())
		return err
	})
	return p, err
}

func (d *DB) Progress(ctx context.Context, user string) (map[string]Progress, error) {
	rows, err := d.QueryContext(ctx, "SELECT item_id, pos, duration, watched, updated_at FROM progress WHERE user_id = ?", user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Progress{}
	for rows.Next() {
		var id string
		var p Progress
		var ms int64
		if err := rows.Scan(&id, &p.Pos, &p.Dur, &p.Watched, &ms); err != nil {
			return nil, err
		}
		p.Updated = toTime(ms)
		out[id] = p
	}
	return out, rows.Err()
}

func (d *DB) ProgressOf(ctx context.Context, user, item string) Progress {
	var p Progress
	var ms int64
	if d.QueryRowContext(ctx, "SELECT pos, duration, watched, updated_at FROM progress WHERE user_id = ? AND item_id = ?", user, item).
		Scan(&p.Pos, &p.Dur, &p.Watched, &ms) == nil {
		p.Updated = toTime(ms)
	}
	return p
}

func (d *DB) DeleteProgress(ctx context.Context, user, item string) error {
	_, err := d.ExecContext(ctx, "DELETE FROM progress WHERE user_id = ? AND item_id = ?", user, item)
	return err
}

// --- Sprachwahl pro Serie ---

type TrackPref struct {
	Audio    string `json:"audio,omitempty"`
	Subtitle string `json:"subtitle,omitempty"` // "off" = bewusst aus
}

func (d *DB) Pref(ctx context.Context, user, series string) TrackPref {
	var p TrackPref
	d.QueryRowContext(ctx, "SELECT audio, subtitle FROM series_prefs WHERE user_id = ? AND series = ?", user, series).Scan(&p.Audio, &p.Subtitle)
	return p
}

// SetPref übernimmt nur nicht-leere Felder.
func (d *DB) SetPref(ctx context.Context, user, series string, p TrackPref) error {
	_, err := d.ExecContext(ctx, `INSERT INTO series_prefs(user_id, series, audio, subtitle) VALUES(?, ?, ?, ?)
		ON CONFLICT(user_id, series) DO UPDATE SET
			audio = CASE WHEN excluded.audio = '' THEN audio ELSE excluded.audio END,
			subtitle = CASE WHEN excluded.subtitle = '' THEN subtitle ELSE excluded.subtitle END`,
		user, series, p.Audio, p.Subtitle)
	return err
}

// --- Geräteprofile ---

// Device liefert das gespeicherte Profil-JSON oder nil.
func (d *DB) Device(ctx context.Context, id string) []byte {
	var b []byte
	d.QueryRowContext(ctx, "SELECT profile FROM devices WHERE id = ?", id).Scan(&b)
	return b
}

// Devices liefert alle gespeicherten Geräteprofile (JSON).
func (d *DB) Devices(ctx context.Context) ([][]byte, error) {
	rows, err := d.QueryContext(ctx, "SELECT profile FROM devices ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]byte
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (d *DB) SetDevice(ctx context.Context, id string, profile []byte) error {
	_, err := d.ExecContext(ctx, `INSERT INTO devices(id, profile, updated_at) VALUES(?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET profile = excluded.profile, updated_at = excluded.updated_at`, id, string(profile), now())
	return err
}

// tx führt f in einer Transaktion aus (BEGIN IMMEDIATE, siehe _txlock in Open).
func (d *DB) tx(ctx context.Context, f func(tx *sql.Tx) error) error {
	d.writeMu.Lock()
	defer d.writeMu.Unlock()
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := f(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}
