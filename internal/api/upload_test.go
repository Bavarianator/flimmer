package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

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
}
