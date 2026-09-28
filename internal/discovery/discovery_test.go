package discovery

import (
	"bytes"
	"context"
	"image/png"
	"net"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func search(st string) []byte {
	return []byte("M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: " + st + "\r\n\r\n")
}

func TestWanted(t *testing.T) {
	for _, tt := range []struct {
		pkt  []byte
		want bool
	}{
		{search(ST), true},
		{search("ssdp:all"), true},
		{search("urn:schemas-upnp-org:device:MediaRenderer:1"), false}, // fremde Geräte ignorieren
		{[]byte("NOTIFY * HTTP/1.1\r\nNT: " + ST + "\r\n\r\n"), false},
		{[]byte("Müll"), false},
	} {
		if got := wanted(tt.pkt); got != tt.want {
			t.Errorf("%q: %v, will %v", tt.pkt, got, tt.want)
		}
	}
}

// Ende-zu-Ende über Loopback-Unicast: dieselbe Schleife wie im Multicast-Betrieb, nur ohne Netz-Abhängigkeit in der CI.
func TestServe(t *testing.T) {
	srv, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	go serve(srv, "Wohnzimmer\r\nX-Böse: 1", 8096)

	c, err := net.DialUDP("udp4", nil, srv.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Write(search("urn:andere:sache"))
	c.Write(search(ST))
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 2048)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	resp := string(buf[:n])
	for _, want := range []string{"HTTP/1.1 200 OK\r\n", "LOCATION: http://127.0.0.1:8096/\r\n", "ST: " + ST + "\r\n", "X-FLIMMER-NAME: WohnzimmerX-Böse: 1\r\n"} {
		if !strings.Contains(resp, want) {
			t.Errorf("fehlt %q in\n%s", want, resp)
		}
	}
	// Nur eine Antwort: die fremde Suche wurde ignoriert.
	c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if n, err := c.Read(buf); err == nil {
		t.Errorf("unerwartete zweite Antwort: %s", buf[:n])
	}
}

func TestStartBeendetSichMitContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	if err := Start(ctx, "Test", 8096); err != nil {
		t.Skipf("kein Multicast in dieser Umgebung: %v", err) // darf den Server nie aufhalten
	}
	cancel()
}

func TestQR(t *testing.T) {
	rec := httptest.NewRecorder()
	QRHandler(8096)(rec, httptest.NewRequest("GET", "/api/qr", nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil || img.Bounds().Dx() < 100 {
		t.Errorf("kein brauchbares PNG: %v %v", err, img)
	}
	if u := LANURL(8096); !strings.HasPrefix(u, "http://") || !strings.HasSuffix(u, ":8096") {
		t.Errorf("LANURL = %s", u)
	}
}

// Echter Multicast-Weg: FLIMMER_SSDP=1 go test -run TestEchtesSSDP ./internal/discovery
func TestEchtesSSDP(t *testing.T) {
	if os.Getenv("FLIMMER_SSDP") == "" {
		t.Skip("nur mit FLIMMER_SSDP=1")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := Start(ctx, "Test", 8096); err != nil {
		t.Fatal(err)
	}
	c, err := net.ListenUDP("udp4", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.WriteTo(search(ST), ssdpAddr)
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 2048)
	n, _, err := c.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", buf[:n])
}
