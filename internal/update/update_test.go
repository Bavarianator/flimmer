package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, tt := range []struct {
		latest, current string
		want            bool
	}{
		{"v0.2.0", "v0.1.9", true},
		{"v0.10.0", "v0.9.0", true}, // numerisch, nicht alphabetisch
		{"v1.0", "v0.99.99", true},
		{"v0.1.0", "v0.1.0", false},
		{"v0.1.0", "v0.2.0", false},
		{"v0.2.0-rc1", "v0.1.0", true},
		{"v0.2.0", "dev", false},
		{"kaputt", "v0.1.0", false},
	} {
		if got := Newer(tt.latest, tt.current); got != tt.want {
			t.Errorf("Newer(%q, %q) = %v", tt.latest, tt.current, got)
		}
	}
}

func fake(t *testing.T, status int, body string) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("GitHub verlangt einen User-Agent")
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	old, oldV := API, Version
	t.Cleanup(func() { API, Version = old, oldV })
	API = srv.URL
}

func TestChecker(t *testing.T) {
	fake(t, 200, `{"tag_name":"v0.3.0","html_url":"https://github.com/Bavarianator/flimmer/releases/tag/v0.3.0","published_at":"2026-10-01T12:00:00Z"}`)
	var c Checker

	Version = "v0.3.0"
	c.check(context.Background())
	if c.Available() != nil {
		t.Error("gleiche Version darf keinen Hinweis geben")
	}

	Version = "v0.2.1"
	c.check(context.Background())
	if r := c.Available(); r == nil || r.Version != "v0.3.0" || r.URL == "" || r.Published.Year() != 2026 {
		t.Errorf("Hinweis fehlt: %+v", r)
	}
}

func TestKeinReleaseUndFehler(t *testing.T) {
	fake(t, 404, `{"message":"Not Found"}`)
	if r, err := Latest(context.Background()); r != nil || err != nil {
		t.Errorf("404: %v %v", r, err)
	}
	fake(t, 403, `{"message":"rate limit"}`)
	var c Checker
	Version = "v0.1.0"
	c.check(context.Background()) // darf nur loggen
	if c.Available() != nil {
		t.Error("Fehler darf keinen Hinweis erzeugen")
	}
}

func TestDevPrueftNie(t *testing.T) {
	Version = "dev"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	(&Checker{}).Run(ctx, func() bool { t.Error("dev-Build fragt enabled ab"); return true })
}
