package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/flimmer-media/flimmer/internal/remote"
)

type fakeDNS map[string]string

func (f fakeDNS) SetTXT(_ context.Context, fqdn, v string) error { f[fqdn] = v; return nil }
func (f fakeDNS) SetAddr(_ context.Context, fqdn string, ip net.IP) error {
	f[fqdn] = ip.String()
	return nil
}

func post(t *testing.T, url string, body any, out any) int {
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(out)
	return resp.StatusCode
}

func TestRelay(t *testing.T) {
	dir := t.TempDir()
	db, err := openDB(filepath.Join(dir, "relay.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	dns := fakeDNS{}
	rl := &relay{db: db, zone: "flimmer.direct", allowPrivate: true}
	relaySrv := httptest.NewServer(rl.routes())
	defer relaySrv.Close()

	// Ein Flimmer-Server, der nur den Ping-Handler hat.
	keyFile := filepath.Join(dir, "remote.key")
	rem, err := remote.New(remote.Options{Port: 1, KeyFile: keyFile})
	if err != nil {
		t.Fatal(err)
	}
	key, _ := remote.LoadKey(keyFile)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/remote/ping", rem.PingHandler)
	flimmer := httptest.NewServer(mux)
	defer flimmer.Close()

	// ACME vor der Registrierung: abgelehnt.
	var e struct{ Error string }
	if c := post(t, relaySrv.URL+"/v1/acme", remote.Sign("acme", key, "tok"), &e); c != http.StatusConflict {
		t.Fatalf("acme vor register: %d %s", c, e.Error)
	}

	var reg struct {
		Reachable bool
		Observed  string
	}
	if c := post(t, relaySrv.URL+"/v1/register", remote.Sign("register", key, flimmer.URL), &reg); c != 200 || !reg.Reachable || reg.Observed != "127.0.0.1" {
		t.Fatalf("register: %d %+v", c, reg)
	}

	// Falsche Signatur.
	bad := remote.Sign("register", key, flimmer.URL)
	bad.Data = "http://127.0.0.1:1"
	if c := post(t, relaySrv.URL+"/v1/register", bad, &e); c != http.StatusUnauthorized {
		t.Fatalf("manipuliert: %d", c)
	}

	// Anderer Server hinter derselben Adresse: Rückruf beweist die falsche ID → nicht erreichbar.
	other, _ := remote.LoadKey(filepath.Join(dir, "other.key"))
	if c := post(t, relaySrv.URL+"/v1/register", remote.Sign("register", other, flimmer.URL), &reg); c != 200 || reg.Reachable {
		t.Fatalf("fremde ID: %d %+v", c, reg)
	}

	resp, _ := http.Get(relaySrv.URL + "/v1/servers/" + rem.ID())
	var srv struct{ URL string }
	json.NewDecoder(resp.Body).Decode(&srv)
	resp.Body.Close()
	if srv.URL != flimmer.URL {
		t.Fatalf("lookup: %+v", srv)
	}

	var pair struct{ Code string }
	if c := post(t, relaySrv.URL+"/v1/pair", remote.Sign("pair", key, ""), &pair); c != 200 || len(pair.Code) != 9 {
		t.Fatalf("pair: %d %+v", c, pair)
	}
	for i, want := range []int{200, 404} { // einmalig
		resp, _ := http.Get(relaySrv.URL + "/v1/pair/" + pair.Code)
		var res struct{ ID, URL string }
		json.NewDecoder(resp.Body).Decode(&res)
		resp.Body.Close()
		if resp.StatusCode != want || (i == 0 && (res.ID != rem.ID() || res.URL != flimmer.URL)) {
			t.Fatalf("resolve %d: %d %+v", i, resp.StatusCode, res)
		}
	}

	if c := post(t, relaySrv.URL+"/v1/acme", remote.Sign("acme", key, "tok"), &e); c != http.StatusNotImplemented {
		t.Fatalf("acme ohne DNS: %d", c)
	}
	rl.dns = dns
	if c := post(t, relaySrv.URL+"/v1/acme", remote.Sign("acme", key, "tok"), &e); c != 200 || dns["_acme-challenge."+rem.ID()+".flimmer.direct"] != "tok" {
		t.Fatalf("acme: %d %v", c, dns)
	}
}

func TestCheckTarget(t *testing.T) {
	rl := &relay{}
	from := net.ParseIP("203.0.113.7")
	for raw, ok := range map[string]bool{
		"http://203.0.113.7:8097":       true,
		"http://[2001:db8::1]:8097":     true,
		"http://198.51.100.1:8097":      false, // fremde IPv4
		"http://192.168.1.2:8097":       false,
		"http://127.0.0.1:8097":         false,
		"http://example.com:8097":       false,
		"http://203.0.113.7:8097/admin": false,
		"file:///etc/passwd":            false,
		"http://203.0.113.7":            false,
	} {
		if _, err := rl.checkTarget(raw, from); (err == nil) != ok {
			t.Errorf("%s: %v", raw, err)
		}
	}
}
