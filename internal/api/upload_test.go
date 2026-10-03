package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Bavarianator/flimmer/internal/auth"
	"github.com/Bavarianator/flimmer/internal/db"
	"github.com/Bavarianator/flimmer/internal/scan"
	"github.com/Bavarianator/flimmer/internal/setup"
	"github.com/Bavarianator/flimmer/internal/share"
	"github.com/Bavarianator/flimmer/internal/transcode"
)

// Recht „Hochladen“: ohne Freigabe 403, Admin gibt frei, gleicher Name wird „(2)“, Pfade und Nicht-Videos 400.
func TestUpload(t *testing.T) {
	data := t.TempDir()
	store, err := db.Open(filepath.Join(data, "flimmer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.CreateUser(t.Context(), db.User{ID: "a", Name: "Anna", Admin: true, PassHash: auth.HashPassword("geheim")})
	store.CreateUser(t.Context(), db.User{ID: "b", Name: "Ben"})
	s := &Server{Lib: scan.NewLibrary(store.DB, nil), HLS: transcode.NewManager(t.TempDir()), DB: store, CacheDir: data,
		UploadDir: filepath.Join(data, "uploads"), Web: fstest.MapFS{"index.html": {Data: []byte("ui")}}, Pages: setup.FS()}
	s.Share = share.New(share.Options{DB: store, BaseURL: func() (string, bool) { return "http://flimmer", false },
		Login: s.GuestLogin, UserID: func(r *http.Request) string { return UserFrom(r).ID }})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	call := func(tok, method, path, body string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	login := func(user, pass string) string {
		_, b := call("", "POST", "/api/login", `{"user":"`+user+`","password":"`+pass+`"}`)
		var r struct{ Token string }
		json.Unmarshal([]byte(b), &r)
		if r.Token == "" {
			t.Fatalf("login %s: %s", user, b)
		}
		return r.Token
	}
	anna, ben := login("a", "geheim"), login("b", "")

	if code, _ := call(ben, "POST", "/api/upload?name=Film.mkv", "x"); code != http.StatusForbidden {
		t.Fatalf("ohne Recht: %d", code)
	}
	if code, _ := call(ben, "POST", "/api/upload/link", `{"url":"https://example.com/a.mp4"}`); code != http.StatusForbidden {
		t.Fatalf("Link ohne Recht: %d", code)
	}
	if code, b := call(anna, "PUT", "/api/users/b", `{"upload":true}`); code != 200 || !strings.Contains(b, `"upload":true`) {
		t.Fatalf("Recht vergeben: %d %s", code, b)
	}
	for _, want := range []string{"Film.mkv", "Film (2).mkv"} {
		if code, b := call(ben, "POST", "/api/upload?name=Film.mkv", "video"); code != 200 || !strings.Contains(b, want) {
			t.Fatalf("upload: %d %s, erwartet %s", code, b, want)
		}
		if got, _ := os.ReadFile(filepath.Join(data, "uploads", want)); string(got) != "video" {
			t.Fatalf("%s: %q", want, got)
		}
	}
	for _, name := range []string{"../boese.mkv", "skript.sh", ".versteckt.mkv"} {
		if code, _ := call(ben, "POST", "/api/upload?name="+name, "x"); code != http.StatusBadRequest && name != "../boese.mkv" {
			t.Fatalf("%s: %d", name, code)
		}
	}
	if _, err := os.Stat(filepath.Join(data, "boese.mkv")); err == nil {
		t.Fatal("Upload außerhalb des Upload-Ordners")
	}
	if left, _ := filepath.Glob(filepath.Join(data, "uploads", ".upload-*")); len(left) != 0 {
		t.Fatalf("Reste: %v", left)
	}

	for _, link := range []string{"file:///etc/passwd", "/etc/passwd", "-o /tmp/x", "ftp://host/a.mp4"} {
		if code, _ := call(ben, "POST", "/api/upload/link", `{"url":"`+link+`"}`); code != http.StatusBadRequest {
			t.Fatalf("Link %s: %d", link, code)
		}
	}

	// Echter Download per yt-dlp von einer direkten Datei-URL (lokal, ohne Internet).
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skip("yt-dlp fehlt")
	}
	quelle := t.TempDir()
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=d=1:s=160x120", "-pix_fmt", "yuv420p",
		filepath.Join(quelle, "Clip.mp4")).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg: %v %s", err, out)
	}
	// /geheim/download: nur mit Cookie, ohne Dateiendung und Video-Typ – wie ein SharePoint-download.aspx.
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(quelle)))
	mux.HandleFunc("/geheim/download", func(w http.ResponseWriter, r *http.Request) {
		if c, _ := r.Cookie("flimmer"); c == nil || c.Value != "ok" {
			http.Error(w, "Anmeldung nötig", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, filepath.Join(quelle, "Clip.mp4"))
	})
	datei := httptest.NewServer(mux)
	defer datei.Close()
	laden := func(link, name, cookies string) linkJob {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"url": datei.URL + link, "name": name, "cookies": cookies})
		code, b := call(ben, "POST", "/api/upload/link", string(body))
		var j linkJob
		if json.Unmarshal([]byte(b), &j); code != 200 {
			t.Fatalf("Link %s: %d %s", link, code, b)
		}
		for range 300 {
			_, b := call(ben, "GET", "/api/upload/link", "")
			var jobs []linkJob
			json.Unmarshal([]byte(b), &jobs)
			for _, x := range jobs {
				if x.ID == j.ID && (x.Fertig || x.Fehler != "") {
					return x
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatalf("Link %s: kein Ende", link)
		return j
	}
	cookies := "# Netscape HTTP Cookie File\n127.0.0.1\tFALSE\t/\tFALSE\t0\tflimmer\tok\n"
	// Der Name kommt vom Lesezeichen „An Flimmer“; % darf yt-dlp nicht als Platzhalter lesen.
	for _, c := range []struct{ link, name, cookies, datei string }{
		{"/Clip.mp4", "", "", "Clip.mp4"},
		{"/geheim/download", "", "", ""},
		{"/geheim/download", "", cookies, "download.mkv"},
		{"/geheim/download?tempauth=x", "../Besprechung 100%.mp4", cookies, "Besprechung 100%.mkv"},
	} {
		j := laden(c.link, c.name, c.cookies)
		if c.datei == "" {
			if j.Fertig || j.Fehler == "" {
				t.Fatalf("%s ohne Cookies: %+v", c.link, j)
			}
			continue
		}
		if _, err := os.Stat(filepath.Join(data, "uploads", j.Name)); err != nil || j.Name != c.datei {
			t.Fatalf("%s: %+v %v", c.link, j, err)
		}
	}
	if _, err := os.Stat(filepath.Join(data, "link")); !os.IsNotExist(err) {
		t.Fatalf("Cookies/Reste nicht aufgeräumt: %v", err)
	}
	if _, b := call(anna, "GET", "/api/upload/link", ""); b != "[]\n" {
		t.Fatalf("fremde Aufträge sichtbar: %q", b)
	}
}
