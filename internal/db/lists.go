package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// --- Favoriten ---

// Favorites liefert die Favoriten-Schlüssel eines Profils, neueste zuerst.
func (d *DB) Favorites(ctx context.Context, user string) ([]string, error) {
	return d.strings(ctx, "SELECT key FROM favorites WHERE user_id = ? ORDER BY created_at DESC, key", user)
}

// SetFavorite merkt (on) oder entfernt einen Favoriten; beides ist idempotent.
func (d *DB) SetFavorite(ctx context.Context, user, key string, on bool) error {
	q, args := "DELETE FROM favorites WHERE user_id = ? AND key = ?", []any{user, key}
	if on {
		q, args = "INSERT OR IGNORE INTO favorites(user_id, key, created_at) VALUES(?, ?, ?)", []any{user, key, now()}
	}
	_, err := d.ExecContext(ctx, q, args...)
	return err
}

// --- Sammlungen und Wiedergabelisten ---

// List ist eine Sammlung oder Wiedergabeliste; Items sind Titel-IDs oder "serie:<Name>".
type List struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Overview string   `json:"overview,omitempty"`
	Items    []string `json:"items"`
}

// Kind einer Liste; Sammlungen gehören niemandem (user ""), Wiedergabelisten einem Profil.
const (
	Collection = "collection"
	Playlist   = "playlist"
)

// owner: NULL für Sammlungen, sonst die Benutzer-ID (Vergleich per IS, damit NULL passt).
func owner(user string) any {
	if user == "" {
		return nil
	}
	return user
}

func (d *DB) Lists(ctx context.Context, kind, user string) ([]List, error) {
	rows, err := d.QueryContext(ctx, "SELECT id, name, overview, items FROM lists WHERE kind = ? AND user_id IS ? ORDER BY name COLLATE NOCASE, id", kind, owner(user))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []List{}
	for rows.Next() {
		l, err := scanList(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func scanList(sc interface{ Scan(...any) error }) (List, error) {
	var l List
	var items string
	if err := sc.Scan(&l.ID, &l.Name, &l.Overview, &items); err != nil {
		return l, err
	}
	if json.Unmarshal([]byte(items), &l.Items) != nil || l.Items == nil {
		l.Items = []string{}
	}
	return l, nil
}

func (d *DB) CreateList(ctx context.Context, kind, user string, l List) error {
	items, _ := json.Marshal(l.Items)
	_, err := d.ExecContext(ctx, "INSERT INTO lists(id, kind, user_id, name, overview, items, created_at) VALUES(?, ?, ?, ?, ?, ?, ?)",
		l.ID, kind, owner(user), l.Name, l.Overview, string(items), now())
	return err
}

// UpdateList ändert eine Liste in einer Transaktion; ErrNotFound, wenn sie nicht existiert oder jemand anderem gehört.
func (d *DB) UpdateList(ctx context.Context, kind, user, id string, f func(l *List) error) (List, error) {
	var l List
	err := d.tx(ctx, func(tx *sql.Tx) error {
		var err error
		l, err = scanList(tx.QueryRowContext(ctx, "SELECT id, name, overview, items FROM lists WHERE id = ? AND kind = ? AND user_id IS ?", id, kind, owner(user)))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := f(&l); err != nil {
			return err
		}
		items, _ := json.Marshal(l.Items)
		_, err = tx.ExecContext(ctx, "UPDATE lists SET name = ?, overview = ?, items = ? WHERE id = ?", l.Name, l.Overview, string(items), id)
		return err
	})
	return l, err
}

func (d *DB) DeleteList(ctx context.Context, kind, user, id string) error {
	res, err := d.ExecContext(ctx, "DELETE FROM lists WHERE id = ? AND kind = ? AND user_id IS ?", id, kind, owner(user))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// strings liest eine Spalte Text; nie nil, damit JSON [] statt null liefert.
func (d *DB) strings(ctx context.Context, q string, args ...any) ([]string, error) {
	rows, err := d.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// --- Geräte (Sessions) für das Dashboard ---

// Session ist ein angemeldetes Gerät. ID sind die ersten 16 Zeichen des Token-Hashes, nie das Token.
type Session struct {
	ID       string    `json:"id"`
	UserID   string    `json:"userId"`
	User     string    `json:"user"`
	Device   string    `json:"name"`
	Client   string    `json:"client"`
	IP       string    `json:"ip,omitempty"`
	LastSeen time.Time `json:"lastSeen"`
}

// SessionID ist die kurze ID zu einem Token-Hash (wie in Sessions).
func SessionID(hash string) string { return hash[:min(16, len(hash))] }

// Sessions liefert die gültigen Sessions, zuletzt benutzte zuerst.
func (d *DB) Sessions(ctx context.Context, ttl time.Duration) ([]Session, error) {
	rows, err := d.QueryContext(ctx, `SELECT substr(s.hash, 1, 16), s.user_id, u.name, s.device, s.client, s.ip, s.last_seen
		FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.last_seen >= ? ORDER BY s.last_seen DESC`, now()-ttl.Milliseconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var s Session
		var ms int64
		if err := rows.Scan(&s.ID, &s.UserID, &s.User, &s.Device, &s.Client, &s.IP, &ms); err != nil {
			return nil, err
		}
		s.LastSeen = toTime(ms)
		out = append(out, s)
	}
	return out, rows.Err()
}

// SetSessionClient merkt Client (z. B. „Android“) und IP einer frisch angelegten Session.
func (d *DB) SetSessionClient(ctx context.Context, hash, client, ip string) error {
	_, err := d.ExecContext(ctx, "UPDATE sessions SET client = ?, ip = ? WHERE hash = ?", client, ip, hash)
	return err
}

// DeleteSessionID meldet ein Gerät ab (ID wie in Sessions).
func (d *DB) DeleteSessionID(ctx context.Context, id string) error {
	if len(id) != 16 {
		return ErrNotFound
	}
	res, err := d.ExecContext(ctx, "DELETE FROM sessions WHERE substr(hash, 1, 16) = ?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Schlüssel/Wert (z. B. letzte Läufe der Aufgaben) ---

// KV liest einen Wert aus settings ("" wenn nicht gesetzt).
func (d *DB) KV(ctx context.Context, key string) string {
	var v string
	d.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&v)
	return v
}

func (d *DB) SetKV(ctx context.Context, key, value string) error {
	_, err := d.ExecContext(ctx, "INSERT INTO settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, value)
	return err
}

// BackupNow sichert sofort wie das Nacht-Backup (Aufgabe „Sicherung erstellen“).
func (d *DB) BackupNow(ctx context.Context, dir string, keep int) error {
	return d.nightlyBackup(ctx, dir, time.Now().Format("2006-01-02"), keep)
}
