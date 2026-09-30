package livetv

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Bavarianator/flimmer/internal/db"
)

const m3u = "\xef\xbb\xbf#EXTM3U\n" +
	`#EXTINF:-1 tvg-id="das.erste" tvg-logo="http://x/l.png" group-title="Voll, Gruppe" tvg-chno="1",Das Erste HD` + "\nhttp://u:secret@tv.local/1\n" +
	"#EXTINF:-1,ZDF\nhttp://tv.local/2\n" +
	"#EXTINF:-1,ZDF\nhttp://tv.local/2b\n" + // gleicher Name: eigene ID
	"#EXTINF:-1,Böse\nfile:///etc/passwd\n" + // Schema nicht erlaubt
	"#EXTINF:-1,Ohne Adresse\n"

func TestParseChannels(t *testing.T) {
	chs, err := parseChannels(strings.NewReader(m3u))
	if err != nil || len(chs) != 3 {
		t.Fatalf("%v %+v", err, chs)
	}
	if c := chs[0]; c.Name != "Das Erste HD" || c.Group != "Voll, Gruppe" || c.Number != "1" || c.epg != "das.erste" || c.Logo == "" {
		t.Errorf("Attribute: %+v", c)
	}
	if chs[1].ID == chs[2].ID || chs[1].ID == chs[0].ID {
		t.Error("IDs müssen eindeutig sein")
	}
	if b, _ := json.Marshal(chs[0]); strings.Contains(string(b), "secret") {
		t.Error("Adresse darf nicht in der JSON-Ausgabe stehen")
	}

	hd, err := parseChannels(strings.NewReader(`[{"GuideNumber":"2.1","GuideName":"ZDF","URL":"http://hd:5004/auto/v2.1"}]`))
	if err != nil || len(hd) != 1 || hd[0].Number != "2.1" {
		t.Errorf("HDHomeRun: %v %+v", err, hd)
	}
	if _, err := parseChannels(strings.NewReader("<html>")); err == nil {
		t.Error("kein M3U muss ein Fehler sein")
	}
}

func TestParseXMLTV(t *testing.T) {
	now := time.Date(2026, 9, 28, 20, 0, 0, 0, time.UTC)
	in := `<?xml version="1.0" encoding="ISO-8859-1"?><tv>
<channel id="das.erste"><display-name>Das Erste HD</display-name></channel>
<programme start="20260928170000 +0000" stop="20260928183000 +0000" channel="das.erste"><title>Gestern</title></programme>
<programme start="20260928200000 +0000" stop="20260928211500 +0000" channel="das.erste"><title>Tagesschau K` + "\xf6ln" + `</title><desc>Nachrichten</desc></programme>
<programme start="20260928221500 +0200" stop="20260929000000 +0200" channel="das.erste"><title>Sp` + "\xe4" + `ter</title></programme>
<programme start="20261101000000 +0000" stop="20261101010000 +0000" channel="das.erste"><title>Zu weit weg</title></programme>
</tv>`
	g, err := parseXMLTV(strings.NewReader(in), now)
	if err != nil {
		t.Fatal(err)
	}
	p := g.progs["das.erste"]
	if len(p) != 2 || p[0].Title != "Tagesschau Köln" || p[1].Title != "Später" || p[0].Desc != "Nachrichten" {
		t.Fatalf("%+v", p)
	}
	if !p[1].Start.Equal(time.Date(2026, 9, 28, 20, 15, 0, 0, time.UTC)) { // +0200 wird umgerechnet
		t.Errorf("Zeitzone: %v", p[1].Start)
	}
	if g.names[nameKey("Das Erste")] != "das.erste" { // XMLTV „Das Erste HD“ passt auch zu „Das Erste“
		t.Error("Anzeigename fehlt")
	}
}

func newTV(t *testing.T) *TV {
	t.Helper()
	store, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	tv := New(store.DB, t.TempDir())
	tv.Allow = func(*http.Request) bool { return true }
	tv.URL = func(_ *http.Request, id string) string { return "/api/m/tok/livetv/channels/" + id + "/index.m3u8" }
	return tv
}

func serve(t *testing.T, files map[string]string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := files[r.URL.Path]; ok {
			io.WriteString(w, body)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// Kanäle ohne tvg-id finden ihr Programm über den Anzeigenamen; Fehler im EPG lassen die Kanäle stehen.
func TestRefreshAndHandlers(t *testing.T) {
	tv := newTV(t)
	now := time.Now().UTC()
	f := func(d time.Duration) string { return now.Add(d).Format("20060102150405 -0700") }
	base := serve(t, map[string]string{
		"/list.m3u": "#EXTM3U\n#EXTINF:-1,ZDF\nhttp://tv.local/2\n",
		"/epg.xml": `<tv><channel id="zdf.de"><display-name>zdf</display-name></channel>
<programme start="` + f(-time.Hour) + `" stop="` + f(30*time.Minute) + `" channel="zdf.de"><title>Läuft</title></programme>
<programme start="` + f(30*time.Minute) + `" stop="` + f(2*time.Hour) + `" channel="zdf.de"><title>Danach</title></programme></tv>`,
	})
	do := func(h http.HandlerFunc, method, target, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		r.SetPathValue("id", strings.TrimSuffix(strings.TrimPrefix(target, "/c/"), "/play"))
		h(w, r)
		return w
	}

	if w := do(tv.ConfigHandler, "PUT", "/", `{"source":"file:///etc/passwd"}`); w.Code != 400 {
		t.Errorf("file:// muss abgelehnt werden: %d", w.Code)
	}
	if w := do(tv.ConfigHandler, "PUT", "/", `{"source":"`+base+`/list.m3u","epg":"`+base+`/epg.xml"}`); w.Code != 202 {
		t.Fatalf("PUT: %d", w.Code)
	}
	if err := tv.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}

	var chs []nowNext
	json.Unmarshal(do(tv.ChannelsHandler, "GET", "/", "").Body.Bytes(), &chs)
	if len(chs) != 1 || chs[0].Now == nil || chs[0].Now.Title != "Läuft" || chs[0].Next == nil || chs[0].Next.Title != "Danach" {
		t.Fatalf("%+v", chs)
	}
	var gd struct {
		Channels []struct{ Programs []Program }
	}
	json.Unmarshal(do(tv.GuideHandler, "GET", "/?hours=1", "").Body.Bytes(), &gd)
	if len(gd.Channels) != 1 || len(gd.Channels[0].Programs) != 2 {
		t.Errorf("Guide: %+v", gd)
	}
	if b := do(tv.StatusHandler, "GET", "/", "").Body.String(); strings.Contains(b, "list.m3u") || !strings.Contains(b, `"channels":1`) {
		t.Errorf("Status verrät den Pfad oder zählt falsch: %s", b)
	}

	// Ohne Freigabe (Gast) und mit kaputtem EPG
	tv.Allow = nil
	if w := do(tv.ChannelsHandler, "GET", "/", ""); w.Code != 403 {
		t.Errorf("ohne Allow muss 403 kommen: %d", w.Code)
	}
	tv.Allow = func(*http.Request) bool { return true }
	do(tv.ConfigHandler, "PUT", "/", `{"epg":"`+base+`/fehlt.xml"}`)
	if err := tv.Refresh(context.Background()); err == nil || strings.Contains(err.Error(), "fehlt.xml") == false && strings.Contains(err.Error(), "404") == false {
		t.Errorf("EPG-Fehler erwartet: %v", err)
	}
	if len(tv.chs) != 1 {
		t.Error("Kanäle müssen trotz EPG-Fehler bleiben")
	}
}

// Echter ffmpeg-Lauf: eine Quelle per HTTP → Playlist mit Segment, Idle-Abbau räumt auf.
func TestPlayEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg fehlt")
	}
	src := filepath.Join(t.TempDir(), "quelle.ts")
	if b, err := exec.Command("ffmpeg", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=6",
		"-f", "lavfi", "-i", "sine=duration=6", "-c:v", "libx264", "-g", "25", "-c:a", "mp2", "-f", "mpegts", src).CombinedOutput(); err != nil {
		t.Skipf("Testquelle: %v %s", err, b)
	}
	data, _ := os.ReadFile(src)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()

	tv := newTV(t)
	tv.set([]Channel{{ID: "abc", Name: "Test", url: srv.URL + "/live.ts"}}, nil, "")
	for _, mode := range []string{"copy", "h264"} {
		tv.DB.Exec("INSERT INTO settings(key, value) VALUES('livetv_video', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", mode)
		start := time.Now()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.SetPathValue("id", "abc")
		r.SetPathValue("file", "index.m3u8")
		tv.FileHandler(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "s0.ts") {
			t.Fatalf("%s: %d %s", mode, w.Code, w.Body.String())
		}
		t.Logf("%s: erste Playlist nach %v", mode, time.Since(start))

		w = httptest.NewRecorder()
		r.SetPathValue("file", "s0.ts")
		tv.FileHandler(w, r)
		if w.Code != 200 || w.Body.Len() < 1000 || w.Header().Get("Content-Type") != "video/mp2t" {
			t.Errorf("%s: Segment %d, %d Bytes", mode, w.Code, w.Body.Len())
		}
		tv.reapIdle(0)
		select {
		case <-tv.stream("abc").done:
		case <-time.After(5 * time.Second):
			t.Fatal("ffmpeg endet nicht")
		}
	}
}
