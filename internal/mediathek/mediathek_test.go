package mediathek

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Bavarianator/flimmer/internal/db"
	"github.com/Bavarianator/flimmer/internal/scan"
)

func TestName(t *testing.T) {
	for _, c := range []struct {
		t    Treffer
		want string
	}{
		{Treffer{Thema: "Spielfilm", Titel: "Die Spionin - Spielfilm, Norwegen/Schweden/Belgien 2019"}, "Die Spionin (2019)"},
		{Treffer{Thema: "Tatort", Titel: "Cash (2024) (klare Sprache)"}, "Tatort - Cash (2024)"},
		{Treffer{Thema: "Spielfilm-Highlights", Titel: "DogMan"}, "DogMan"},
		{Treffer{Thema: "Babylon Berlin", Titel: "Folge 3 (S01/E03)"}, "Babylon Berlin S01E03 Folge 3"},
		{Treffer{Thema: "Doku", Titel: "Wer/Was: Teil 1?"}, "Doku - Wer Was Teil 1"},
	} {
		if got := Name(c.t); got != c.want {
			t.Errorf("Name(%q) = %q, will %q", c.t.Titel, got, c.want)
		}
	}
	for titel, variante := range map[string]bool{"Der Räuber Hotzenplotz (AD)": true, "Kundschafter des Friedens 2 (mit Untertitel)": true,
		"Das Mädchen mit den goldenen Händen (Originalversion mit Untertitel)": true,
		"Tatort - Audiodeskription": true, "Englisch für Anfänger": false, "Der Räuber Hotzenplotz": false, "Das ADAC-Magazin": false} {
		if reVariante.MatchString(titel) != variante {
			t.Errorf("Variante %q: will %v", titel, variante)
		}
	}
	// So ordnet der Scan die Namen ein.
	if it := scan.Parse("/x/Babylon Berlin S01E03 Folge 3.mp4"); it.Series != "Babylon Berlin" || it.Episode != 3 {
		t.Errorf("Folge: %+v", it)
	}
	if it := scan.Parse("/x/Tatort - Cash (2024).mp4"); it.Title != "Tatort - Cash" || it.Year != 2024 {
		t.Errorf("Film: %+v", it)
	}
}

func TestSucheLadenAbo(t *testing.T) {
	video := strings.Repeat("x", 100000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/video.mp4" {
			io.WriteString(w, video)
			return
		}
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), `"query":"spionin"`) {
			t.Errorf("Anfrage %s", b)
		}
		jetzt := time.Now().Unix()
		io.WriteString(w, `{"result":{"results":[
			{"id":"a","channel":"3Sat","topic":"Spielfilm","title":"Die Spionin - Spielfilm, Norwegen 2019","timestamp":`+strconv.FormatInt(jetzt, 10)+`,"duration":6306,"size":null,"url_video":"`+r.Host+`","url_video_hd":"http://`+r.Host+`/video.mp4"},
			{"id":"b","channel":"3Sat","topic":"Spielfilm","title":"Die Spionin (Audiodeskription)","timestamp":`+strconv.FormatInt(jetzt, 10)+`,"duration":6306,"size":1,"url_video":"http://x/v.mp4","url_video_hd":""},
			{"id":"c","channel":"ORF","topic":"Spielfilm","title":"Die Spionin","timestamp":`+strconv.FormatInt(jetzt, 10)+`,"duration":6306,"size":1,"url_video":"http://x/v.m3u8","url_video_hd":""},
			{"id":"d","channel":"ARD","topic":"Spielfilm","title":"Die Spionin - Spielfilm, Norwegen 2019","timestamp":`+strconv.FormatInt(jetzt, 10)+`,"duration":6306,"size":1,"url_video":"http://x/v.mp4","url_video_hd":""}
		],"queryInfo":{"totalResults":4}},"err":null}`)
	}))
	defer srv.Close()
	api = srv.URL

	ctx := context.Background()
	treffer, gesamt, err := Suche(ctx, Anfrage{Text: "spionin"})
	if err != nil || gesamt != 4 || len(treffer) != 2 || treffer[0].ID != "a" {
		t.Fatalf("Suche: %v %d %+v", err, gesamt, treffer)
	}

	d, err := db.Open(filepath.Join(t.TempDir(), "flimmer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	dir := t.TempDir()
	m := New(d, dir)
	gescannt := make(chan struct{}, 1)
	m.Rescan = func() { gescannt <- struct{}{} }
	ctx, stopp := context.WithCancel(ctx)
	defer stopp()
	go m.Run(ctx)

	if _, err := m.Laden(ctx, treffer[0], "mediathek", "u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Laden(ctx, treffer[0], "mediathek", "u1"); err != db.ErrSchonDa {
		t.Fatalf("doppelt: %v", err)
	}
	select {
	case <-gescannt:
	case <-time.After(10 * time.Second):
		t.Fatal("Download nicht fertig")
	}
	b, err := os.ReadFile(filepath.Join(dir, "Die Spionin (2019).mp4"))
	if err != nil || len(b) != len(video) {
		t.Fatalf("Datei: %v %d", err, len(b))
	}
	if reste, _ := filepath.Glob(filepath.Join(dir, ".*.part")); len(reste) > 0 {
		t.Fatalf("Zwischendateien: %v", reste)
	}
	l, _ := m.Downloads(ctx)
	if len(l) != 1 || l[0].Status != "fertig" || l[0].Bytes != int64(len(video)) {
		t.Fatalf("Downloads: %+v", l)
	}

	// Abo: „d“ ist derselbe Film auf einem anderen Sender und wird nicht noch einmal geladen.
	if _, err := d.AddAbo(ctx, db.Abo{Text: "spionin"}); err != nil {
		t.Fatal(err)
	}
	if err := m.PruefeAbos(ctx); err != nil {
		t.Fatal(err)
	}
	if l, _ := m.Downloads(ctx); len(l) != 1 {
		t.Fatalf("Abo hat doppelt eingereiht: %+v", l)
	}
}
