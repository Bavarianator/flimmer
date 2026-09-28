package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ncruces/go-sqlite3/driver"
)

// Close schreibt das WAL zurück in die Hauptdatei, damit flimmer.db allein vollständig ist (Backup per Kopie).
func (d *DB) Close() error {
	d.ExecContext(context.Background(), "PRAGMA wal_checkpoint(TRUNCATE)")
	return d.DB.Close()
}

// Maintenance ist das Ergebnis der letzten Wartung und des letzten Nacht-Backups (für die Diagnose).
type Maintenance struct {
	CheckedAt  time.Time `json:"checkedAt"`
	Integrity  string    `json:"integrity"` // "ok" oder die erste Fehlermeldung von SQLite
	BackupAt   time.Time `json:"backupAt"`
	BackupFile string    `json:"backupFile"`
	SizeBytes  int64     `json:"sizeBytes"`
}

func (d *DB) Maintenance(ctx context.Context) Maintenance {
	var m Maintenance
	get := func(k string) string {
		var v string
		d.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", k).Scan(&v)
		return v
	}
	if ms, err := strconv.ParseInt(get("maint_at"), 10, 64); err == nil {
		m.CheckedAt = toTime(ms)
	}
	if ms, err := strconv.ParseInt(get("backup_at"), 10, 64); err == nil {
		m.BackupAt = toTime(ms)
	}
	m.Integrity, m.BackupFile = get("maint_result"), get("backup_file")
	if fi, err := os.Stat(d.path); err == nil {
		m.SizeBytes = fi.Size()
	}
	return m
}

func (d *DB) setKV(ctx context.Context, kv map[string]string) {
	for k, v := range kv {
		d.ExecContext(ctx, "INSERT INTO settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", k, v)
	}
}

// Check führt PRAGMA optimize und integrity_check aus und merkt sich das Ergebnis.
func (d *DB) Check(ctx context.Context) string {
	d.ExecContext(ctx, "PRAGMA optimize")
	result := integrity(ctx, d.DB)
	d.setKV(ctx, map[string]string{"maint_at": strconv.FormatInt(now(), 10), "maint_result": result})
	if result != "ok" {
		log.Printf("Datenbank: Integritätsprüfung meldet: %s", result)
	}
	return result
}

func integrity(ctx context.Context, q *sql.DB) string {
	var r string
	if err := q.QueryRowContext(ctx, "PRAGMA integrity_check(1)").Scan(&r); err != nil {
		return err.Error()
	}
	return r
}

// BackupTo schreibt eine konsistente Kopie nach path (läuft auch während Zugriffen).
func (d *DB) BackupTo(ctx context.Context, path string) error {
	os.Remove(path)
	_, err := d.ExecContext(ctx, "VACUUM INTO ?", path)
	return err
}

// Restore spielt eine Backup-Datei im laufenden Betrieb ein. Vorher wird sie geprüft (lesbar, intakt,
// Flimmer-Datenbank, nicht neuer als diese Version) und die aktuelle Datenbank nach flimmer.db.bak gesichert.
func (d *DB) Restore(ctx context.Context, path string) error {
	if err := validate(ctx, path); err != nil {
		return err
	}
	if err := d.backup(ctx); err != nil {
		return fmt.Errorf("Sicherung vor dem Einspielen: %w", err)
	}
	conn, err := d.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	err = conn.Raw(func(dc any) error {
		return dc.(driver.Conn).Raw().Restore("main", path)
	})
	if err != nil {
		return err
	}
	if err := d.migrate(ctx); err != nil { // ältere Backups auf den aktuellen Stand bringen
		return err
	}
	return d.ensureSecret(ctx)
}

// ErrBadBackup: Die Datei ist kein brauchbares Flimmer-Backup.
type ErrBadBackup string

func (e ErrBadBackup) Error() string { return string(e) }

func validate(ctx context.Context, path string) error {
	q, err := sql.Open("sqlite3", "file:"+uriPath(path)+"?mode=ro")
	if err != nil {
		return err
	}
	defer q.Close()
	var version, admins int
	if err := q.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return ErrBadBackup("Die Datei ist keine Flimmer-Datenbank.")
	}
	if r := integrity(ctx, q); r != "ok" {
		return ErrBadBackup("Die Datei ist beschädigt: " + r)
	}
	if version < 1 {
		return ErrBadBackup("Die Datei ist keine Flimmer-Datenbank.")
	}
	if version > latestVersion() {
		return ErrBadBackup("Das Backup stammt aus einer neueren Flimmer-Version – bitte erst Flimmer aktualisieren.")
	}
	if err := q.QueryRowContext(ctx, "SELECT count(*) FROM users WHERE admin = 1").Scan(&admins); err != nil || admins == 0 {
		return ErrBadBackup("Im Backup gibt es keinen Admin – so könnte sich danach niemand mehr anmelden.")
	}
	return nil
}

func latestVersion() int {
	n := 0
	for _, f := range migrationFiles() {
		n = max(n, f.version)
	}
	return n
}

// Nightly sichert einmal pro Nacht (ab 3 Uhr) nach dir/flimmer-JJJJ-MM-TT.db, behält die letzten keep
// und prüft wöchentlich die Integrität. Blockiert bis ctx endet.
func (d *DB) Nightly(ctx context.Context, dir string, keep int) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		now := time.Now()
		m := d.Maintenance(ctx)
		if now.Sub(m.CheckedAt) > 7*24*time.Hour {
			d.Check(ctx)
		}
		today := now.Format("2006-01-02")
		if now.Hour() >= 3 && !strings.Contains(m.BackupFile, today) {
			if err := d.nightlyBackup(ctx, dir, today, keep); err != nil && ctx.Err() == nil {
				log.Printf("Nacht-Backup: %v", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (d *DB) nightlyBackup(ctx context.Context, dir, day string, keep int) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, "flimmer-"+day+".db")
	if err := d.BackupTo(ctx, path); err != nil {
		return err
	}
	d.setKV(ctx, map[string]string{"backup_at": strconv.FormatInt(now(), 10), "backup_file": path})
	old, _ := filepath.Glob(filepath.Join(dir, "flimmer-????-??-??.db"))
	slices.Sort(old) // Datum im Namen → alphabetisch = chronologisch
	for len(old) > keep {
		os.Remove(old[0])
		old = old[1:]
	}
	return nil
}
