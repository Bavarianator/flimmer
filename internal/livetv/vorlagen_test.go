package livetv

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Die eingebettete Liste lädt ohne Netz, jeder Sender hat Namen, Nummer, eigene ID und eine https-Adresse.
func TestFreeSource(t *testing.T) {
	chs, err := fetch(context.Background(), FreeSource, time.Second, parseChannels)
	if err != nil || len(chs) != 18 {
		t.Fatalf("%v %d", err, len(chs))
	}
	ids := map[string]bool{}
	for _, c := range chs {
		if c.Number == "" || ids[c.ID] || !strings.HasPrefix(c.url, "https://") {
			t.Errorf("%+v", c)
		}
		ids[c.ID] = true
	}
}

// Eine FRITZ!Box-Liste (#EXTVLCOPT, rtsp, „HD“ im Namen, keine tvg-id) findet ihr Programm über den Namen.
func TestFritzNames(t *testing.T) {
	const fritz = "#EXTM3U\n#EXTINF:0,Das Erste HD\n#EXTVLCOPT:network-caching=1000\n" +
		"rtsp://192.168.178.1:554/?avm=1&freq=330&bw=8&msys=dvbc&mtype=256qam&sr=6900&specinv=1&pids=0,16,17,18,20,5100\n"
	chs, err := parseChannels(strings.NewReader(fritz))
	if err != nil || len(chs) != 1 {
		t.Fatalf("%v %+v", err, chs)
	}
	g, err := parseXMLTV(strings.NewReader(`<tv><channel id="Das.Erste.de"><display-name>Das Erste</display-name></channel></tv>`), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	tv := &TV{}
	tv.set(chs, g, "")
	if tv.chs[0].epg != "Das.Erste.de" {
		t.Errorf("Zuordnung: %q", tv.chs[0].epg)
	}
}

// Die Vorlagen melden die FRITZ!Box nur, wenn unter ihrer Adresse wirklich eine Kanalliste liegt.
func TestPresetsHandler(t *testing.T) {
	box := serve(t, map[string]string{"/dvb/m3u/tvhd.m3u": "#EXTM3U\n#EXTINF:0,ZDF HD\nrtsp://box/1\n#EXTINF:0,arte HD\nrtsp://box/2\n"})
	keine := serve(t, map[string]string{"/dvb/m3u/tvhd.m3u": "<html>Anmeldung</html>"})
	tv := newTV(t)
	old := fritzLists
	t.Cleanup(func() { fritzLists = old })

	get := func() (out []preset) {
		w := httptest.NewRecorder()
		tv.PresetsHandler(w, httptest.NewRequest("GET", "/", nil))
		json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	fritzLists = []string{keine + "/dvb/m3u/tvhd.m3u", box + "/dvb/m3u/tvhd.m3u"}
	p := get()
	if len(p) != 2 || p[0].ID != "frei" || p[0].Source != FreeSource || p[0].Channels != 18 || p[0].EPG != EPGDE {
		t.Fatalf("frei: %+v", p)
	}
	if p[1].ID != "fritz" || p[1].Channels != 2 || p[1].Source != fritzLists[1] || p[0].Active || p[1].Active {
		t.Errorf("fritz: %+v", p[1])
	}
	if err := tv.save(context.Background(), Config{Source: fritzLists[1]}); err != nil {
		t.Fatal(err)
	}
	if p := get(); p[0].Active || !p[1].Active {
		t.Errorf("aktiv: %+v", p)
	}
	fritzLists = []string{keine + "/dvb/m3u/tvhd.m3u"}
	if p := get(); p[1].Channels != 0 {
		t.Errorf("ohne FRITZ!Box: %+v", p[1])
	}
}

func TestConfigAcceptsFreeSource(t *testing.T) {
	tv := newTV(t)
	w := httptest.NewRecorder()
	tv.ConfigHandler(w, httptest.NewRequest("PUT", "/", strings.NewReader(`{"source":"`+FreeSource+`","epg":"`+EPGDE+`"}`)))
	if w.Code != 202 || tv.config(context.Background()).Source != FreeSource {
		t.Fatalf("%d %+v", w.Code, tv.config(context.Background()))
	}
}

// Aus der Hauptliste kommt die beste Variante (h264: höchstens 720p); relative Adressen werden aufgelöst.
func TestBestVariant(t *testing.T) {
	base := serve(t, map[string]string{"/live/master.m3u8": "#EXTM3U\n" +
		"#EXT-X-STREAM-INF:BANDWIDTH=2147200,AVERAGE-BANDWIDTH=1460800,RESOLUTION=640x360\nv360/index.m3u8\n" +
		"#EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=9999999,RESOLUTION=1920x1080,URI=\"iframe.m3u8\"\n" +
		"#EXT-X-STREAM-INF:BANDWIDTH=8500800,AVERAGE-BANDWIDTH=5640800,RESOLUTION=1920x1080\nv1080/index.m3u8\n" +
		"#EXT-X-STREAM-INF:BANDWIDTH=5491200,RESOLUTION=1280x720\nhttps://cdn.example/v720.m3u8\n"})
	if got, a := bestVariant(base+"/live/master.m3u8", "copy"); got != base+"/live/v1080/index.m3u8" || a != "" {
		t.Errorf("copy: %s %s", got, a)
	}
	if got, _ := bestVariant(base+"/live/master.m3u8", "h264"); got != "https://cdn.example/v720.m3u8" {
		t.Errorf("h264: %s", got)
	}
	for _, src := range []string{"rtsp://box/1", base + "/fehlt.m3u8", "http://tv.local/stream.ts"} {
		if got, a := bestVariant(src, "copy"); got != src || a != "" {
			t.Errorf("%s -> %s %s", src, got, a)
		}
	}
}

// ZDF/ARD führen den Ton getrennt (#EXT-X-MEDIA TYPE=AUDIO): Ohne die Tonspur liefe der Sender stumm.
func TestBestVariantAudio(t *testing.T) {
	base := serve(t, map[string]string{"/zdf/master.m3u8": "#EXTM3U\n" +
		`#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="a",NAME="Audio-Deskription",LANGUAGE="deu",URI="/zdf/6.m3u8"` + "\n" +
		`#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="a",NAME="TV Ton",LANGUAGE="deu",DEFAULT=YES,URI="/zdf/4.m3u8"` + "\n" +
		`#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="b",NAME="Original",LANGUAGE="mul",URI="/zdf/b5.m3u8"` + "\n" +
		`#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="b",NAME="Deutsch",LANGUAGE="deu",URI="/zdf/b4.m3u8"` + "\n" +
		`#EXT-X-STREAM-INF:CODECS="avc1.4d401f,mp4a.40.2",BANDWIDTH=1173371,AUDIO="a",RESOLUTION=640x360` + "\n1/1.m3u8\n" +
		`#EXT-X-STREAM-INF:CODECS="avc1.640028,mp4a.40.2",BANDWIDTH=4504154,AUDIO="a",RESOLUTION=1280x720` + "\n3/3.m3u8\n",
		"/ohne/master.m3u8": "#EXTM3U\n" +
			`#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="b",NAME="Original",LANGUAGE="mul",URI="/zdf/b5.m3u8"` + "\n" +
			`#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="b",NAME="Deutsch",LANGUAGE="deu",URI="/zdf/b4.m3u8"` + "\n" +
			`#EXT-X-STREAM-INF:BANDWIDTH=900000,AUDIO="b",RESOLUTION=640x360` + "\nv.m3u8\n"})
	if v, a := bestVariant(base+"/zdf/master.m3u8", "copy"); v != base+"/zdf/3/3.m3u8" || a != base+"/zdf/4.m3u8" {
		t.Errorf("Standardspur: %s %s", v, a)
	}
	if _, a := bestVariant(base+"/ohne/master.m3u8", "copy"); a != base+"/zdf/b4.m3u8" {
		t.Errorf("ohne Standardspur Deutsch: %s", a)
	}
	args := strings.Join(ffmpegArgs("http://v/3.m3u8", "http://v/4.m3u8", "/tmp/x", "copy"), " ")
	if !strings.Contains(args, "-i http://v/4.m3u8") || !strings.Contains(args, "-map 1:a:0") {
		t.Errorf("zweite Quelle fehlt: %s", args)
	}
}
