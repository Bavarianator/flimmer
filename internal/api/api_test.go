package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/flimmer-media/flimmer/internal/db"
	"github.com/flimmer-media/flimmer/internal/optimize"
	"github.com/flimmer-media/flimmer/internal/scan"
	"github.com/flimmer-media/flimmer/internal/setup"
	"github.com/flimmer-media/flimmer/internal/share"
	"github.com/flimmer-media/flimmer/internal/transcode"
)

// Einrichtung → Anmeldung → Bibliothek → Medien-Token (ohne Cookie) → Fortschritt → Home → Drossel.
func TestEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg fehlt")
	}
	media, data := t.TempDir(), t.TempDir()
	clip := filepath.Join(media, "Testfilm (2024).mkv")
	if b, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=8",
		"-g", "25", "-c:v", "libx264", clip).CombinedOutput(); err != nil {
		t.Fatalf("testclip: %v %s", err, b)
	}
	store, err := db.Open(filepath.Join(data, "flimmer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	lib := scan.NewLibrary(store.DB, nil)
	s := &Server{Lib: lib, HLS: transcode.NewManager(t.TempDir()), DB: store, CacheDir: data,
		Web: fstest.MapFS{"index.html": {Data: []byte("ui")}}, Pages: setup.FS()}
	s.FFmpeg.Store(true)
	s.Share = share.New(share.Options{DB: store, BaseURL: func() (string, bool) { return "http://flimmer", false },
		Login: s.GuestLogin, UserID: func(r *http.Request) string { return UserFrom(r).ID }})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	jar, _ := cookiejar.New(nil)
	browser := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	anon := &http.Client{CheckRedirect: browser.CheckRedirect}
	call := func(c *http.Client, method, path string, body any) (*http.Response, []byte) {
		t.Helper()
		var r io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			r = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, srv.URL+path, r)
		res, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return res, b
	}

	// Vor der Einrichtung: Startseite leitet um, API verlangt Anmeldung und sagt „setup“.
	if res, _ := call(browser, "GET", "/", nil); res.StatusCode != http.StatusFound || res.Header.Get("Location") != "/setup" {
		t.Fatalf("/ ohne Admin: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	if res, b := call(browser, "POST", "/api/library", map[string]any{}); res.StatusCode != 401 || !strings.Contains(string(b), `"setup":true`) {
		t.Fatalf("library ohne Login: %d %s", res.StatusCode, b)
	}
	if res, _ := call(browser, "GET", "/setup", nil); res.StatusCode != 200 {
		t.Fatalf("/setup: %d", res.StatusCode)
	}
	if res, b := call(browser, "POST", "/api/setup", map[string]any{"name": "Anna", "password": "geheim", "dirs": []string{media}}); res.StatusCode != 200 {
		t.Fatalf("setup: %d %s", res.StatusCode, b)
	}
	if res, _ := call(anon, "POST", "/api/setup", map[string]any{"name": "Mallory", "password": "boese", "dirs": []string{}}); res.StatusCode != 400 {
		t.Fatalf("zweites Setup erlaubt: %d", res.StatusCode)
	}
	if err := lib.Scan(context.Background()); err != nil { // SetDirs weckt nur Run, das hier nicht läuft
		t.Fatal(err)
	}

	// Angemeldet: Bibliothek mit Direct Play, Wiedergabe per HLS über ein Medien-Token im Pfad.
	var items []struct {
		ID     string `json:"id"`
		Method string `json:"method"`
	}
	res, b := call(browser, "POST", "/api/library", map[string]any{"containers": []string{"mkv"}, "video": []string{"h264"}})
	json.Unmarshal(b, &items)
	if res.StatusCode != 200 || len(items) != 1 || items[0].Method != "direct-play" {
		t.Fatalf("library: %d %s", res.StatusCode, b)
	}
	id := items[0].ID
	var play struct {
		URL string `json:"url"`
	}
	_, b = call(browser, "POST", "/api/items/"+id+"/play", map[string]any{"video": []string{"h264"}})
	json.Unmarshal(b, &play)
	if !strings.HasPrefix(play.URL, "/api/m/") || !strings.HasSuffix(play.URL, "index.m3u8") {
		t.Fatalf("play-URL: %s", b)
	}
	res, b = call(anon, "GET", play.URL, nil)
	if n := strings.Count(string(b), ".ts"); res.StatusCode != 200 || n < 2 {
		t.Fatalf("playlist ohne Cookie: %d mit %d Segmenten\n%s", res.StatusCode, n, b)
	}
	if res, _ := call(anon, "GET", strings.Replace(play.URL, "/copy/copy/", "/aac-night/copy/", 1), nil); res.StatusCode != 200 {
		t.Fatalf("Nachtmodus-Playlist: %d", res.StatusCode)
	}
	if res, _ := call(anon, "GET", strings.Replace(play.URL, "/items/", "x/items/", 1), nil); res.StatusCode != 401 {
		t.Fatalf("manipuliertes Token: %d", res.StatusCode)
	}
	req, _ := http.NewRequest("GET", srv.URL+strings.Replace(play.URL, "/hls/-1/copy/copy/index.m3u8", "/file", 1), nil)
	req.Header.Set("Range", "bytes=0-99")
	if res, err := anon.Do(req); err != nil || res.StatusCode != http.StatusPartialContent {
		t.Fatalf("range: %v %v", err, res.Status)
	}

	// Vorbereitete MP4-Version: Das Original (MKV) würde nur umverpackt, die MP4 läuft direkt → play nimmt /o/.
	optDir := t.TempDir()
	if b, err := exec.Command("ffmpeg", "-v", "error", "-i", clip, "-c", "copy", filepath.Join(optDir, id+".mp4")).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, b)
	}
	s.Optimizer.Store(optimize.New(optimize.Options{Dir: optDir}))
	var opt struct {
		URL       string `json:"url"`
		Optimized bool   `json:"optimized"`
	}
	_, b = call(browser, "POST", "/api/items/"+id+"/play", map[string]any{"containers": []string{"mp4"}, "video": []string{"h264"}})
	json.Unmarshal(b, &opt)
	if !opt.Optimized || !strings.HasSuffix(opt.URL, "/o/file") {
		t.Fatalf("optimierte Version nicht gewählt: %s", b)
	}
	req, _ = http.NewRequest("GET", srv.URL+opt.URL, nil)
	req.Header.Set("Range", "bytes=0-99")
	if res, err := anon.Do(req); err != nil || res.StatusCode != http.StatusPartialContent {
		t.Fatalf("optimierte Datei: %v %v", err, res.Status)
	}
	s.Optimizer.Store(nil)

	// Fortschritt → Weiterschauen.
	if res, b := call(browser, "POST", "/api/items/"+id+"/progress", map[string]any{"pos": 4}); res.StatusCode != 200 {
		t.Fatalf("progress: %d %s", res.StatusCode, b)
	}
	var rows []struct {
		ID    string `json:"id"`
		Items []struct {
			ID       string  `json:"id"`
			Progress float64 `json:"progress"`
		} `json:"items"`
	}
	_, b = call(browser, "POST", "/api/home", map[string]any{})
	json.Unmarshal(b, &rows)
	if len(rows) == 0 || rows[0].ID != "continue" || rows[0].Items[0].Progress != 4 {
		t.Fatalf("home: %s", b)
	}

	// Sicherung herunterladen und wieder einspielen; Datenmüll wird abgelehnt.
	res, b = call(browser, "GET", "/api/settings/backup", nil)
	if res.StatusCode != 200 || !bytes.HasPrefix(b, []byte("SQLite format 3")) {
		t.Fatalf("backup: %d %.20q", res.StatusCode, b)
	}
	if res, _ := call(anon, "GET", "/api/settings/backup", nil); res.StatusCode != 401 {
		t.Fatalf("backup ohne Login: %d", res.StatusCode)
	}
	req, _ = http.NewRequest("POST", srv.URL+"/api/settings/restore", strings.NewReader("kein sqlite"))
	if res, _ := browser.Do(req); res.StatusCode != 400 {
		t.Fatalf("restore mit Müll: %d", res.StatusCode)
	}
	req, _ = http.NewRequest("POST", srv.URL+"/api/settings/restore", bytes.NewReader(b))
	if res, err := browser.Do(req); err != nil || res.StatusCode != 204 {
		t.Fatalf("restore: %v %v", err, res.Status)
	}
	if lib.Get(id) == nil {
		t.Fatal("Bibliothek nach Restore leer")
	}

	// Gast mit Einladung für einen anderen Titel: sieht nichts, kommt nicht an diesen Film, steht nicht in der
	// Profilauswahl; nach dem Widerruf ist Schluss.
	var inv struct {
		URL    string              `json:"url"`
		Invite struct{ ID string } `json:"invite"`
	}
	other := filepath.Join(media, "Anderer Film (2020).mkv")
	if b, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=160x120:rate=25:duration=2",
		"-c:v", "libx264", other).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, b)
	}
	if err := lib.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	otherID := scan.ID(other)
	res, b = call(browser, "POST", "/api/invites", map[string]any{"items": []string{otherID}, "hours": 24})
	json.Unmarshal(b, &inv)
	_, token, _ := strings.Cut(inv.URL, "#")
	if res.StatusCode != 200 || token == "" {
		t.Fatalf("Einladung: %d %s", res.StatusCode, b)
	}
	gjar, _ := cookiejar.New(nil)
	guest := &http.Client{Jar: gjar}
	if res, b := call(guest, "POST", "/api/invites/redeem", map[string]any{"token": token, "name": "Gast"}); res.StatusCode != 200 {
		t.Fatalf("einlösen: %d %s", res.StatusCode, b)
	}
	if _, b := call(guest, "POST", "/api/library", map[string]any{}); !strings.Contains(string(b), otherID) || strings.Contains(string(b), id) {
		t.Fatalf("Gast sieht falsche Bibliothek: %s", b)
	}
	if _, b := call(guest, "POST", "/api/home", map[string]any{}); strings.Contains(string(b), id) {
		t.Fatalf("Gast sieht fremden Film auf der Startseite: %s", b)
	}
	if res, _ := call(guest, "POST", "/api/items/"+id+"/play", map[string]any{}); res.StatusCode != 404 {
		t.Fatalf("Gast darf fremden Film abspielen: %d", res.StatusCode)
	}
	if _, b := call(anon, "GET", "/api/users", nil); strings.Contains(string(b), "Gast") {
		t.Fatalf("Gast in der Profilauswahl: %s", b)
	}
	if res, _ := call(browser, "DELETE", "/api/invites/"+inv.Invite.ID, nil); res.StatusCode != 204 {
		t.Fatalf("widerrufen: %d", res.StatusCode)
	}
	if res, _ := call(guest, "POST", "/api/library", map[string]any{}); res.StatusCode != 401 {
		t.Fatalf("Gast nach Widerruf: %d", res.StatusCode)
	}

	// Falsches Passwort wird gedrosselt.
	var users []struct{ ID string }
	_, b = call(anon, "GET", "/api/users", nil)
	json.Unmarshal(b, &users)
	codes := ""
	for range 6 {
		res, _ := call(anon, "POST", "/api/login", map[string]any{"user": users[0].ID, "password": "falsch"})
		codes += res.Status[:3] + " "
	}
	if !strings.HasSuffix(codes, "429 ") {
		t.Fatalf("keine Drossel: %s", codes)
	}
}

type fakeLib []*scan.Item

func (f fakeLib) Get(id string) *scan.Item {
	for _, it := range f {
		if it.ID == id {
			return it
		}
	}
	return nil
}

func (f fakeLib) Episodes(series string) (out []*scan.Item) {
	for _, it := range f {
		if it.Series == series {
			out = append(out, it)
		}
	}
	return out
}

func (f fakeLib) Newest() []*scan.Item {
	out := slices.Clone(f)
	slices.SortStableFunc(out, func(a, b *scan.Item) int { return b.Added.Compare(a.Added) })
	return out
}

func TestHomeRows(t *testing.T) {
	now := time.Now()
	ep := func(id string, e int) *scan.Item { return &scan.Item{ID: id, Series: "Dark", Season: 1, Episode: e} }
	all := fakeLib{ep("e1", 1), ep("e2", 2), ep("e3", 3), {ID: "film", Title: "Heat", Added: now}}
	prog := map[string]db.Progress{
		"e1":   {Watched: true, Updated: now.Add(-time.Hour)},
		"film": {Pos: 600, Updated: now},
		"weg":  {Pos: 100, Updated: now}, // Datei nicht mehr da
	}
	cont, next, recent := homeRows(all, prog)
	if len(cont) != 1 || cont[0].ID != "film" || len(next) != 1 || next[0].ID != "e2" || len(recent) != 2 || recent[0].ID != "film" {
		t.Fatalf("cont=%v next=%v recent=%v", ids(cont), ids(next), ids(recent))
	}
}

func ids(list []*scan.Item) (out []string) {
	for _, it := range list {
		out = append(out, it.ID)
	}
	return out
}
