package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/flimmer-media/flimmer/internal/auth"
	"github.com/flimmer-media/flimmer/internal/db"
	"github.com/flimmer-media/flimmer/internal/scan"
	"github.com/flimmer-media/flimmer/internal/transcode"
)

// bigLibrary legt n Titel (Hälfte Episoden) und users Benutzer mit je 300 Fortschritts-Einträgen an.
func bigLibrary(t testing.TB, n, users int) (*Server, string) {
	ctx := context.Background()
	store, err := db.Open(filepath.Join(t.TempDir(), "flimmer.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	tx, _ := store.Begin()
	for i := range n {
		id := fmt.Sprintf("%012x", i)
		series, season, episode, kind := "", 0, 0, "movie"
		if i%2 == 1 {
			series, season, episode, kind = fmt.Sprintf("Serie %d", i/200), 1+(i/20)%10, 1+i%20, "episode"
		}
		path := fmt.Sprintf("/m/%s/Titel %d (%d).mkv", kind, i, 1950+i%70)
		if series != "" {
			path = fmt.Sprintf("/m/%s/S%02dE%02d.mkv", series, season, episode)
		}
		_, err := tx.Exec(`INSERT INTO items(id, path, size, mtime, added_at, kind, title, year, series, season, episode, container, duration, bitrate)
			VALUES(?, ?, 1, 1, ?, ?, ?, 2000, ?, ?, ?, 'matroska,webm', 5400, 8000000)`, id, path, int64(i), kind, fmt.Sprint("Titel ", i), series, season, episode)
		if err == nil {
			_, err = tx.Exec(`INSERT INTO streams(item_id, idx, type, codec, pix_fmt) VALUES(?, 0, 'video', 'hevc', 'yuv420p10le'), (?, 1, 'audio', 'eac3', ''), (?, 2, 'subtitle', 'subrip', '')`, id, id, id)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for u := range users {
		uid := fmt.Sprint("u", u)
		store.CreateUser(ctx, db.User{ID: uid, Name: uid, Admin: u == 0, PassHash: "x"})
		for j := range 300 {
			store.SetProgress(ctx, uid, fmt.Sprintf("%012x", (u*997+j*131)%n), float64(100+j), 5400)
		}
	}
	lib := scan.NewLibrary(store.DB, nil)
	if err := lib.Load(ctx); err != nil {
		t.Fatal(err)
	}
	tok, h := auth.NewToken()
	store.CreateSession(ctx, h, "u7", "bench", time.Hour)
	return &Server{Lib: lib, HLS: transcode.NewManager(t.TempDir()), DB: store, Web: fstest.MapFS{}}, tok
}

// 50 000 Titel, 20 Benutzer: /api/home und eine Bibliotheksseite unter 100 ms (Ziel auf dem Pi).
// Läuft nur mit FLIMMER_BENCH=1 und ohne -race, weil es Zeiten misst.
func TestLibraryPerformance(t *testing.T) {
	if os.Getenv("FLIMMER_BENCH") == "" {
		t.Skip("FLIMMER_BENCH=1 setzen")
	}
	start := time.Now()
	s, tok := bigLibrary(t, 50_000, 20)
	t.Logf("Testdaten: %d Titel in %s", len(s.Lib.All()), time.Since(start).Round(time.Millisecond))
	h := s.Handler()
	profile := `{"containers":["mp4","mkv"],"video":["h264","hevc"],"audio":["aac","ac3","eac3"]}`
	measure := func(path string) (time.Duration, int) {
		var times []time.Duration
		size := 0
		for range 7 {
			req := httptest.NewRequest("POST", path, strings.NewReader(profile))
			req.Header.Set("Authorization", "Bearer "+tok)
			rec := httptest.NewRecorder()
			t0 := time.Now()
			h.ServeHTTP(rec, req)
			times = append(times, time.Since(t0))
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: %d %s", path, rec.Code, rec.Body)
			}
			size = rec.Body.Len()
		}
		slices.Sort(times)
		return times[len(times)/2], size
	}
	for _, c := range []struct {
		path  string
		limit time.Duration
	}{
		{"/api/home", 100 * time.Millisecond},
		{"/api/library?offset=0&limit=100", 100 * time.Millisecond},
		{"/api/library?offset=40000&limit=100", 100 * time.Millisecond},
		{"/api/library", 0}, // alles auf einmal: nur zur Info
	} {
		d, size := measure(c.path)
		t.Logf("%-40s Median %6.1f ms  %7d KB", c.path, float64(d.Microseconds())/1000, size/1024)
		if c.limit > 0 && d > c.limit {
			t.Errorf("%s: %s > %s", c.path, d, c.limit)
		}
	}
}

// Die heißen Abfragen müssen einen Index nutzen, nie die ganze Tabelle lesen.
func TestQueryPlans(t *testing.T) {
	s, _ := bigLibrary(t, 200, 2)
	for _, q := range []string{
		"SELECT item_id, pos, duration, watched, updated_at FROM progress WHERE user_id = 'u1'",
		"SELECT pos, duration, watched, updated_at FROM progress WHERE user_id = 'u1' AND item_id = 'x'",
		"SELECT u.id FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.hash = 'x'",
		"SELECT audio, subtitle FROM series_prefs WHERE user_id = 'u1' AND series = 'x'",
		"SELECT data FROM keyframes WHERE item_id = 'x' AND size = 1 AND mtime = 1",
		"SELECT json FROM meta WHERE item_id = 'x'",
	} {
		rows, err := s.DB.Query("EXPLAIN QUERY PLAN " + q)
		if err != nil {
			t.Fatal(err)
		}
		var plan []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			rows.Scan(&id, &parent, &unused, &detail)
			plan = append(plan, detail)
		}
		rows.Close()
		for _, p := range plan {
			if strings.HasPrefix(p, "SCAN") && !strings.Contains(p, "USING") {
				t.Errorf("%s\n  → %v", q, plan)
			}
		}
	}
}
