package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// --- Downloads und Mediathek-Abos ---

// Download ist ein Eintrag aus Mediathek, Abo, Link oder Upload. Mediathek-Einträge mit Status „wartet“ bilden die
// Warteschlange von internal/mediathek.
type Download struct {
	ID       int64      `json:"id"`
	Quelle   string     `json:"quelle"` // mediathek | abo | link | upload
	Titel    string     `json:"titel"`
	Sender   string     `json:"sender,omitempty"`
	Datei    string     `json:"datei,omitempty"`
	Bytes    int64      `json:"bytes"`
	Status   string     `json:"status"` // wartet | laeuft | fertig | fehler | abgebrochen
	Fehler   string     `json:"fehler,omitempty"`
	Erstellt time.Time  `json:"erstellt"`
	Ende     *time.Time `json:"ende,omitempty"`
	Anteil   float64    `json:"anteil,omitempty"` // nur laufende Mediathek-Downloads, setzt internal/mediathek

	ExtID string `json:"-"` // MediathekViewWeb-ID
	User  string `json:"-"`
	URL   string `json:"-"`
}

// ErrSchonDa: Diese Sendung (ext_id) steht schon in den Downloads.
var ErrSchonDa = errors.New("Diese Sendung wurde schon geladen oder steht in der Warteschlange")

const downloadCols = "id, source, title, channel, file, bytes, status, error, created_at, done_at, COALESCE(ext_id, ''), user_id, url"

func scanDownload(sc interface{ Scan(...any) error }) (Download, error) {
	var d Download
	var erstellt int64
	var ende sql.NullInt64
	err := sc.Scan(&d.ID, &d.Quelle, &d.Titel, &d.Sender, &d.Datei, &d.Bytes, &d.Status, &d.Fehler, &erstellt, &ende, &d.ExtID, &d.User, &d.URL)
	d.Erstellt = toTime(erstellt)
	if ende.Valid {
		t := toTime(ende.Int64)
		d.Ende = &t
	}
	return d, err
}

// AddDownload trägt einen Download ein und liefert seine ID; mit ExtID nur einmal (sonst ErrSchonDa).
func (d *DB) AddDownload(ctx context.Context, dl Download) (int64, error) {
	var ext, ende any
	if dl.ExtID != "" {
		ext = dl.ExtID
	}
	if dl.Status == "fertig" || dl.Status == "fehler" {
		ende = now()
	}
	res, err := d.ExecContext(ctx, `INSERT INTO downloads(source, ext_id, user_id, title, channel, url, file, bytes, status, error, created_at, done_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(ext_id) DO NOTHING`,
		dl.Quelle, ext, dl.User, dl.Titel, dl.Sender, dl.URL, dl.Datei, dl.Bytes, dl.Status, dl.Fehler, now(), ende)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, ErrSchonDa
	}
	return res.LastInsertId()
}

// Downloads liefert die neuesten Einträge; quellen leer = alle.
func (d *DB) Downloads(ctx context.Context, limit int, quellen ...string) ([]Download, error) {
	q, args := "SELECT "+downloadCols+" FROM downloads", []any{}
	if len(quellen) > 0 {
		q += " WHERE source IN (?" + strings.Repeat(", ?", len(quellen)-1) + ")"
		for _, s := range quellen {
			args = append(args, s)
		}
	}
	rows, err := d.QueryContext(ctx, q+" ORDER BY id DESC LIMIT ?", append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Download{}
	for rows.Next() {
		dl, err := scanDownload(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, dl)
	}
	return out, rows.Err()
}

// Download liefert einen Eintrag oder ErrNotFound.
func (d *DB) Download(ctx context.Context, id int64) (Download, error) {
	dl, err := scanDownload(d.QueryRowContext(ctx, "SELECT "+downloadCols+" FROM downloads WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return dl, err
}

// NextDownload holt den ältesten wartenden Eintrag und setzt ihn auf „laeuft“; ok=false, wenn keiner wartet.
func (d *DB) NextDownload(ctx context.Context) (dl Download, ok bool, err error) {
	err = d.tx(ctx, func(tx *sql.Tx) error {
		dl, err = scanDownload(tx.QueryRowContext(ctx, "SELECT "+downloadCols+" FROM downloads WHERE status = 'wartet' ORDER BY id LIMIT 1"))
		if err != nil {
			return err
		}
		dl.Status = "laeuft"
		_, err = tx.ExecContext(ctx, "UPDATE downloads SET status = 'laeuft' WHERE id = ?", dl.ID)
		return err
	})
	if errors.Is(err, sql.ErrNoRows) {
		return dl, false, nil
	}
	return dl, err == nil, err
}

// FinishDownload setzt das Ergebnis: fertig (mit Datei und Größe), fehler oder abgebrochen.
func (d *DB) FinishDownload(ctx context.Context, id int64, status, file string, bytes int64, fehler string) error {
	_, err := d.ExecContext(ctx, "UPDATE downloads SET status = ?, file = ?, bytes = ?, error = ?, done_at = ? WHERE id = ?",
		status, file, bytes, fehler, now(), id)
	return err
}

// ResetDownloads: Beim Start laufen keine Downloads mehr; die abgebrochenen warten wieder.
func (d *DB) ResetDownloads(ctx context.Context) error {
	_, err := d.ExecContext(ctx, "UPDATE downloads SET status = 'wartet' WHERE status = 'laeuft'")
	return err
}

func (d *DB) DeleteDownload(ctx context.Context, id int64) error {
	_, err := d.ExecContext(ctx, "DELETE FROM downloads WHERE id = ?", id)
	return err
}

// HasDownloadTitle: Gibt es diesen Titel schon (gleicher Film auf zwei Sendern)? Abgebrochene und Fehler zählen nicht.
func (d *DB) HasDownloadTitle(ctx context.Context, title string) bool {
	var n int
	d.QueryRowContext(ctx, "SELECT COUNT(*) FROM downloads WHERE title = ? AND status IN ('wartet', 'laeuft', 'fertig')", title).Scan(&n)
	return n > 0
}

// DownloadQuelle fasst die Downloads einer Quelle zusammen (Statistik).
type DownloadQuelle struct {
	Quelle string `json:"quelle"`
	Anzahl int    `json:"anzahl"`
	Bytes  int64  `json:"bytes"`
	Fehler int    `json:"fehler"`
}

// DownloadStats zählt die Downloads seit since pro Quelle.
func (d *DB) DownloadStats(ctx context.Context, since time.Time) ([]DownloadQuelle, error) {
	rows, err := d.QueryContext(ctx, `SELECT source, SUM(status = 'fertig'), SUM(CASE WHEN status = 'fertig' THEN bytes ELSE 0 END),
		SUM(status = 'fehler') FROM downloads WHERE created_at >= ? GROUP BY source ORDER BY 2 DESC`, since.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DownloadQuelle{}
	for rows.Next() {
		var q DownloadQuelle
		if err := rows.Scan(&q.Quelle, &q.Anzahl, &q.Bytes, &q.Fehler); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// Abo lädt neue Sendungen einer Mediathek-Suche automatisch.
type Abo struct {
	ID         int64     `json:"id"`
	Text       string    `json:"text"`
	Sender     string    `json:"sender,omitempty"`
	MinMinuten int       `json:"minMinuten,omitempty"`
	Erstellt   time.Time `json:"erstellt"`
}

func (d *DB) Abos(ctx context.Context) ([]Abo, error) {
	rows, err := d.QueryContext(ctx, "SELECT id, query, channel, min_minutes, created_at FROM mediathek_abos ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Abo{}
	for rows.Next() {
		var a Abo
		var erstellt int64
		if err := rows.Scan(&a.ID, &a.Text, &a.Sender, &a.MinMinuten, &erstellt); err != nil {
			return nil, err
		}
		a.Erstellt = toTime(erstellt)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (d *DB) AddAbo(ctx context.Context, a Abo) (Abo, error) {
	a.Erstellt = time.Now()
	res, err := d.ExecContext(ctx, "INSERT INTO mediathek_abos(query, channel, min_minutes, created_at) VALUES(?, ?, ?, ?)",
		a.Text, a.Sender, a.MinMinuten, a.Erstellt.UnixMilli())
	if err == nil {
		a.ID, err = res.LastInsertId()
	}
	return a, err
}

func (d *DB) DeleteAbo(ctx context.Context, id int64) error {
	_, err := d.ExecContext(ctx, "DELETE FROM mediathek_abos WHERE id = ?", id)
	return err
}
