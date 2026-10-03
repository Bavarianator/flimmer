package db

import (
	"context"
	"time"
)

// --- Statistik: Sehzeit ---

// AddWatch zählt Sehzeit auf die laufende Stunde; die letzte Methode und der letzte Client der Stunde gewinnen.
func (d *DB) AddWatch(ctx context.Context, user, item string, sek float64, method, client string) error {
	_, err := d.ExecContext(ctx, `INSERT INTO watch(user_id, item_id, hour, seconds, method, client) VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, item_id, hour) DO UPDATE SET seconds = seconds + excluded.seconds, method = excluded.method, client = excluded.client`,
		user, item, time.Now().Truncate(time.Hour).UnixMilli(), sek, method, client)
	return err
}

type Anteil struct {
	Name     string  `json:"name"`
	Sekunden float64 `json:"sekunden"`
}

type NutzerZeit struct {
	Name     string  `json:"name"`
	Farbe    int     `json:"farbe"`
	Sekunden float64 `json:"sekunden"`
	Titel    int     `json:"titel"` // verschiedene Titel
}

type TitelZeit struct {
	ID       string  `json:"id"`
	Titel    string  `json:"titel"`           // füllen Aufrufer aus der Bibliothek
	Serie    string  `json:"serie,omitempty"` // bei Folgen
	Sekunden float64 `json:"sekunden"`
	Nutzer   int     `json:"nutzer"` // verschiedene Profile
}

type WatchStats struct {
	Stunden  map[int64]float64 // Unix-ms der Stunde → Sekunden
	Nutzer   []NutzerZeit
	Titel    []TitelZeit // die 10 meistgesehenen
	Methoden []Anteil
	Clients  []Anteil
}

// WatchStats fasst die Sehzeit seit since zusammen.
func (d *DB) WatchStats(ctx context.Context, since time.Time) (WatchStats, error) {
	ms := since.UnixMilli()
	w := WatchStats{Stunden: map[int64]float64{}}
	rows, err := d.QueryContext(ctx, "SELECT hour, SUM(seconds) FROM watch WHERE hour >= ? GROUP BY hour", ms)
	if err != nil {
		return w, err
	}
	for rows.Next() {
		var h int64
		var s float64
		if err := rows.Scan(&h, &s); err != nil {
			rows.Close()
			return w, err
		}
		w.Stunden[h] = s
	}
	rows.Close()

	rows, err = d.QueryContext(ctx, `SELECT u.name, u.color, SUM(w.seconds), COUNT(DISTINCT w.item_id) FROM watch w
		JOIN users u ON u.id = w.user_id WHERE w.hour >= ? GROUP BY w.user_id ORDER BY 3 DESC`, ms)
	if err != nil {
		return w, err
	}
	w.Nutzer = []NutzerZeit{}
	for rows.Next() {
		var n NutzerZeit
		if err := rows.Scan(&n.Name, &n.Farbe, &n.Sekunden, &n.Titel); err != nil {
			rows.Close()
			return w, err
		}
		w.Nutzer = append(w.Nutzer, n)
	}
	rows.Close()

	rows, err = d.QueryContext(ctx, `SELECT item_id, SUM(seconds), COUNT(DISTINCT user_id) FROM watch WHERE hour >= ?
		GROUP BY item_id ORDER BY 2 DESC LIMIT 10`, ms)
	if err != nil {
		return w, err
	}
	w.Titel = []TitelZeit{}
	for rows.Next() {
		var t TitelZeit
		if err := rows.Scan(&t.ID, &t.Sekunden, &t.Nutzer); err != nil {
			rows.Close()
			return w, err
		}
		w.Titel = append(w.Titel, t)
	}
	rows.Close()

	if w.Methoden, err = d.anteile(ctx, "method", ms); err != nil {
		return w, err
	}
	w.Clients, err = d.anteile(ctx, "client", ms)
	return w, err
}

// anteile summiert die Sehzeit nach einer Spalte (method, client); spalte stammt nie vom Nutzer.
func (d *DB) anteile(ctx context.Context, spalte string, ms int64) ([]Anteil, error) {
	rows, err := d.QueryContext(ctx, "SELECT "+spalte+", SUM(seconds) FROM watch WHERE hour >= ? GROUP BY 1 ORDER BY 2 DESC LIMIT 10", ms)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Anteil{}
	for rows.Next() {
		var a Anteil
		if err := rows.Scan(&a.Name, &a.Sekunden); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// WatchedItems zählt die Titel, die mindestens ein Profil zu Ende gesehen hat.
func (d *DB) WatchedItems(ctx context.Context) (n int) {
	d.QueryRowContext(ctx, "SELECT COUNT(DISTINCT item_id) FROM progress WHERE watched = 1").Scan(&n)
	return n
}
