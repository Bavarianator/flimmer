package remote

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeIGD ist ein Router mit UPnP-IGD: SSDP auf Loopback, Beschreibung und SOAP per httptest.
func fakeIGD(t *testing.T) (actions *[]string, mu *sync.Mutex) {
	actions, mu = new([]string), new(sync.Mutex)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			io.WriteString(w, `<?xml version="1.0"?><root xmlns="urn:schemas-upnp-org:device-1-0"><device>
<deviceType>urn:schemas-upnp-org:device:InternetGatewayDevice:1</deviceType><deviceList><device>
<deviceType>urn:schemas-upnp-org:device:WANDevice:1</deviceType><deviceList><device>
<serviceList><service><serviceType>urn:schemas-upnp-org:service:WANIPConnection:1</serviceType>
<controlURL>/ctl</controlURL></service></serviceList></device></deviceList></device></deviceList></device></root>`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		action := strings.Split(strings.Trim(r.Header.Get("SOAPAction"), `"`), "#")[1]
		mu.Lock()
		*actions = append(*actions, action)
		mu.Unlock()
		if action == "AddPortMapping" && !strings.Contains(string(body), "<NewInternalClient>127.0.0.1</NewInternalClient>") {
			w.WriteHeader(500)
			io.WriteString(w, "<errorCode>402</errorCode>")
			return
		}
		if action == "AddPortMapping" && strings.Contains(string(body), "<NewLeaseDuration>3600<") {
			w.WriteHeader(500)
			io.WriteString(w, "<UPnPError><errorCode>725</errorCode><errorDescription>OnlyPermanentLeasesSupported</errorDescription></UPnPError>")
			return
		}
		io.WriteString(w, `<s:Envelope><s:Body><u:R><NewExternalIPAddress>100.64.1.2</NewExternalIPAddress></u:R></s:Body></s:Envelope>`)
	}))
	t.Cleanup(srv.Close)

	udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { udp.Close() })
	go func() {
		buf := make([]byte, 2048)
		for {
			n, from, err := udp.ReadFrom(buf)
			if err != nil {
				return
			}
			if strings.Contains(string(buf[:n]), "InternetGatewayDevice:1") {
				udp.WriteTo([]byte("HTTP/1.1 200 OK\r\nST: x\r\nLOCATION: "+srv.URL+"/desc.xml\r\n\r\n"), from)
			}
		}
	}()
	old := ssdpAddr
	ssdpAddr = udp.LocalAddr().String()
	t.Cleanup(func() { ssdpAddr = old })
	return actions, mu
}

func TestUPnP(t *testing.T) {
	actions, mu := fakeIGD(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m, err := upnpMap(ctx, 8097)
	if err != nil {
		t.Fatal(err)
	}
	if m.method != "upnp" || m.extIP.String() != "100.64.1.2" || m.extPort != 8097 || m.lease != 0 {
		t.Fatalf("mapping = %+v", m)
	}
	if err := m.remove(ctx); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	got := strings.Join(*actions, ",")
	mu.Unlock()
	if got != "GetExternalIPAddress,AddPortMapping,AddPortMapping,DeletePortMapping" {
		t.Fatalf("Aktionen: %s", got)
	}
}

// fakePMP antwortet auf NAT-PMP; pcp=true versteht zusätzlich PCP.
func fakePMP(t *testing.T, pcp bool) (addr string, deleted chan bool) {
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	deleted = make(chan bool, 1)
	go func() {
		buf := make([]byte, 1100)
		for {
			n, from, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			b := buf[:n]
			var out []byte
			switch {
			case b[0] == 2 && pcp && n == 60:
				out = append([]byte{2, 0x81, 0, 0}, b[4:]...)
				binary.BigEndian.PutUint16(out[42:], 40000)
				copy(out[44:], net.ParseIP("203.0.113.7").To16())
				if binary.BigEndian.Uint32(b[4:]) == 0 {
					deleted <- true
				}
			case b[0] != 0:
				out = []byte{0, b[1] | 0x80, 0, 1} // UNSUPP_VERSION
			case b[1] == 0:
				out = []byte{0, 128, 0, 0, 0, 0, 0, 1, 192, 168, 178, 99}
			case b[1] == 2:
				out = make([]byte, 16)
				out[1] = 130
				copy(out[8:10], b[4:6])
				binary.BigEndian.PutUint16(out[10:], 40001)
				copy(out[12:16], b[8:12])
				if binary.BigEndian.Uint32(b[8:]) == 0 {
					deleted <- true
				}
			}
			conn.WriteTo(out, from)
		}
	}()
	return conn.LocalAddr().String(), deleted
}

func TestNATPMP(t *testing.T) {
	gw, deleted := fakePMP(t, false)
	ctx := context.Background()
	if _, err := pcpMap(ctx, gw, 8097); err == nil || !strings.Contains(err.Error(), "kein PCP") {
		t.Fatalf("PCP gegen reinen NAT-PMP-Router: %v", err)
	}
	m, err := pmpMap(ctx, gw, 8097)
	if err != nil {
		t.Fatal(err)
	}
	if m.extIP.String() != "192.168.178.99" || m.extPort != 40001 || m.lease != pmpLease {
		t.Fatalf("mapping = %+v", m)
	}
	if err := m.renew(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.remove(ctx); err != nil || !<-deleted {
		t.Fatal(err)
	}
}

func TestPCP(t *testing.T) {
	gw, deleted := fakePMP(t, true)
	ctx := context.Background()
	m, err := pcpMap(ctx, gw, 8097)
	if err != nil {
		t.Fatal(err)
	}
	if m.extIP.String() != "203.0.113.7" || m.extPort != 40000 || m.lease != pmpLease {
		t.Fatalf("mapping = %+v", m)
	}
	if err := m.remove(ctx); err != nil || !<-deleted {
		t.Fatal(err)
	}
}

func TestNoRouter(t *testing.T) {
	conn, _ := net.ListenPacket("udp4", "127.0.0.1:0")
	addr := conn.LocalAddr().String()
	conn.Close() // niemand antwortet
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := pmpMap(ctx, addr, 8097); err == nil {
		t.Fatal("Fehler erwartet")
	}
}

func TestStatusHints(t *testing.T) {
	up := &mapping{method: "upnp", extIP: net.ParseIP("203.0.113.7"), extPort: 8097}
	v6 := net.ParseIP("2001:db8::5")
	for _, c := range []struct {
		name   string
		in     input
		method string
		reach  bool
		hint   string
	}{
		{"v6 erreichbar", input{v6: v6, relay: true, reachableURL: "http://[2001:db8::5]:8097"}, "ipv6", true, "über IPv6"},
		{"v6 geblockt", input{v6: v6, relay: true}, "ipv6", false, "IPv6-Portfreigabe"},
		{"upnp erreichbar", input{m: up, relay: true, reachableURL: "http://203.0.113.7:8097"}, "upnp", true, "per UPNP"},
		{"cgnat 100.64", input{m: &mapping{method: "upnp", extIP: net.ParseIP("100.72.1.1")}, relay: true}, "upnp", false, "Carrier-Grade-NAT"},
		{"cgnat Abweichung", input{m: up, relay: true, observed: net.ParseIP("198.51.100.1")}, "upnp", false, "Carrier-Grade-NAT"},
		{"doppeltes NAT", input{m: &mapping{method: "nat-pmp", extIP: net.ParseIP("192.168.1.20")}, relay: true}, "nat-pmp", false, "weiteren Router"},
		{"kein upnp", input{mapErrs: []error{errors.New("x")}, relay: true}, "none", false, "UPnP ein"},
		{"firewall", input{m: up, relay: true, observed: net.ParseIP("203.0.113.7")}, "upnp", false, "Firewall"},
		{"ohne relay", input{m: up}, "upnp", false, "nicht von außen geprüft"},
	} {
		st := c.in.status()
		if st.Method != c.method || st.Reachable != c.reach || !strings.Contains(st.Hint, c.hint) {
			t.Errorf("%s: %+v", c.name, st)
		}
	}
}

func TestSignVerify(t *testing.T) {
	key, err := LoadKey(filepath.Join(t.TempDir(), "remote.key"))
	if err != nil {
		t.Fatal(err)
	}
	r := Sign("register", key, "http://203.0.113.7:8097")
	if err := r.Verify("register", time.Now()); err != nil {
		t.Fatal(err)
	}
	if r.Verify("acme", time.Now()) == nil {
		t.Fatal("andere Art darf nicht gelten")
	}
	if r.Verify("register", time.Now().Add(time.Hour)) == nil {
		t.Fatal("alter Zeitstempel darf nicht gelten")
	}
	r.Data = "http://evil"
	if r.Verify("register", time.Now()) == nil {
		t.Fatal("veränderte Daten dürfen nicht gelten")
	}
	if len(r.ID) != 16 {
		t.Fatalf("ID %q", r.ID)
	}
}
