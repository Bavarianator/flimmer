package scan

import (
	"context"
	"database/sql"
	"encoding/binary"
	"math"

	"github.com/flimmer-media/flimmer/internal/probe"
)

// cached ist, was die Datenbank über eine Datei weiß; Size/MTime entscheiden, ob neu geprobt wird.
type cached struct {
	id, path string
	size     int64
	mtime    int64 // Nanosekunden
	added    int64
	media    *probe.Media
}

// loadCatalog liest alle Titel samt Streams mit zwei Abfragen.
func loadCatalog(ctx context.Context, db *sql.DB) (map[string]*cached, error) {
	out := map[string]*cached{}
	byID := map[string]*cached{}
	rows, err := db.QueryContext(ctx, "SELECT id, path, size, mtime, added_at, container, duration, bitrate FROM items")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		c := &cached{media: &probe.Media{}}
		if err := rows.Scan(&c.id, &c.path, &c.size, &c.mtime, &c.added, &c.media.Container, &c.media.Duration, &c.media.Bitrate); err != nil {
			return nil, err
		}
		out[c.path], byID[c.id] = c, c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	srows, err := db.QueryContext(ctx, "SELECT item_id, idx, type, codec, profile, pix_fmt, width, height, channels, language, title, is_default FROM streams ORDER BY item_id, idx")
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	for srows.Next() {
		var id string
		var s probe.Stream
		if err := srows.Scan(&id, &s.Index, &s.Type, &s.Codec, &s.Profile, &s.PixFmt, &s.Width, &s.Height, &s.Channels, &s.Language, &s.Title, &s.Default); err != nil {
			return nil, err
		}
		if c := byID[id]; c != nil {
			c.media.Streams = append(c.media.Streams, s)
		}
	}
	return out, srows.Err()
}

// saveItem schreibt einen neu geprobten Titel; ein alter Keyframe-Index gilt dann nicht mehr.
func saveItem(ctx context.Context, db *sql.DB, it *Item, mtime int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	kind := "movie"
	if it.Series != "" {
		kind = "episode"
	}
	m := it.Media
	_, err = tx.ExecContext(ctx, `INSERT INTO items(id, path, size, mtime, added_at, kind, title, year, series, season, episode, container, duration, bitrate)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET size = excluded.size, mtime = excluded.mtime, kind = excluded.kind, title = excluded.title,
			year = excluded.year, series = excluded.series, season = excluded.season, episode = excluded.episode,
			container = excluded.container, duration = excluded.duration, bitrate = excluded.bitrate`,
		it.ID, it.Path, it.Size, mtime, it.Added.UnixMilli(), kind, it.Title, it.Year, it.Series, it.Season, it.Episode,
		m.Container, m.Duration, m.Bitrate)
	if err != nil {
		return err
	}
	for _, q := range []string{"DELETE FROM streams WHERE item_id = ?", "DELETE FROM keyframes WHERE item_id = ?"} {
		if _, err := tx.ExecContext(ctx, q, it.ID); err != nil {
			return err
		}
	}
	for _, s := range m.Streams {
		if _, err := tx.ExecContext(ctx, `INSERT INTO streams(item_id, idx, type, codec, profile, pix_fmt, width, height, channels, language, title, is_default)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			it.ID, s.Index, s.Type, s.Codec, s.Profile, s.PixFmt, s.Width, s.Height, s.Channels, s.Language, s.Title, s.Default); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func deleteItems(ctx context.Context, db *sql.DB, ids []string) error {
	for _, id := range ids {
		if _, err := db.ExecContext(ctx, "DELETE FROM items WHERE id = ?", id); err != nil { // Streams/Keyframes per CASCADE
			return err
		}
	}
	return nil
}

func loadKeyframes(ctx context.Context, db *sql.DB, id string, size, mtime int64) ([]float64, bool) {
	var data []byte
	err := db.QueryRowContext(ctx, "SELECT data FROM keyframes WHERE item_id = ? AND size = ? AND mtime = ?", id, size, mtime).Scan(&data)
	if err != nil {
		return nil, false
	}
	kf := make([]float64, len(data)/8)
	for i := range kf {
		kf[i] = math.Float64frombits(binary.LittleEndian.Uint64(data[i*8:]))
	}
	return kf, true
}

func saveKeyframes(ctx context.Context, db *sql.DB, id string, size, mtime int64, kf []float64) error {
	data := make([]byte, 8*len(kf))
	for i, f := range kf {
		binary.LittleEndian.PutUint64(data[i*8:], math.Float64bits(f))
	}
	_, err := db.ExecContext(ctx, `INSERT INTO keyframes(item_id, size, mtime, data) VALUES(?, ?, ?, ?)
		ON CONFLICT(item_id) DO UPDATE SET size = excluded.size, mtime = excluded.mtime, data = excluded.data`, id, size, mtime, data)
	return err
}
