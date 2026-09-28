// Package db öffnet die SQLite-Datenbank flimmer.db (CGO-frei) und bringt das Schema per eingebetteter Migrationen
// auf Stand. Vor jeder Migration einer bestehenden Datenbank entsteht flimmer.db.bak.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
)

//go:embed migrations/*.sql
var migrations embed.FS

type DB struct {
	*sql.DB
	path string
}

// Open öffnet (oder erzeugt) die Datenbank und migriert sie.
func Open(path string) (*DB, error) {
	// Im URI nur die Sonderzeichen maskieren; Schrägstriche bleiben (auch C:/… unter Windows).
	uri := strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23").Replace(filepath.ToSlash(path))
	dsn := "file:" + uri + "?_txlock=immediate" +
		"&_pragma=busy_timeout(10000)&_pragma=foreign_keys(1)&_pragma=journal_mode(wal)&_pragma=synchronous(normal)"
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	d := &DB{DB: sqlDB, path: path}
	ctx := context.Background()
	if err := d.migrate(ctx); err != nil {
		sqlDB.Close()
		return nil, err
	}
	if err := d.ensureSecret(ctx); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) migrate(ctx context.Context) error {
	var version int
	if err := d.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("Datenbank %s nicht lesbar: %w", d.path, err)
	}
	files, _ := fs.Glob(migrations, "migrations/*.sql")
	slices.Sort(files)
	for _, f := range files {
		n, err := strconv.Atoi(strings.SplitN(strings.TrimPrefix(f, "migrations/"), "_", 2)[0])
		if err != nil || n <= version {
			continue
		}
		if version > 0 {
			if err := d.backup(ctx); err != nil {
				return fmt.Errorf("Backup vor Migration %d: %w", n, err)
			}
		}
		script, _ := migrations.ReadFile(f)
		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(script)); err != nil {
			tx.Rollback()
			return fmt.Errorf("Migration %s: %w", f, err)
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(n)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("Migration %s: %w", f, err)
		}
		version = n
	}
	return nil
}

// backup schreibt eine konsistente Kopie nach flimmer.db.bak (ersetzt die vorige).
func (d *DB) backup(ctx context.Context) error {
	tmp := d.path + ".bak.tmp"
	os.Remove(tmp)
	if _, err := d.ExecContext(ctx, "VACUUM INTO ?", tmp); err != nil {
		return err
	}
	return os.Rename(tmp, d.path+".bak")
}

func now() int64 { return time.Now().UnixMilli() }

func toTime(ms int64) time.Time { return time.UnixMilli(ms) }

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
