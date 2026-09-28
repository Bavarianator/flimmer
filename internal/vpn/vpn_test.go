package vpn

import (
	"net/netip"
	"testing"
)

func TestFind(t *testing.T) {
	ip := func(s string) []netip.Addr { return []netip.Addr{netip.MustParseAddr(s)} }
	got := Find(map[string][]netip.Addr{
		"tailscale0": ip("100.101.102.103"),
		"wt0":        ip("100.64.0.7"),
		"utun4":      ip("100.90.1.2"),
		"eth0":       ip("100.72.0.5"),    // CGNAT des Anbieters, kein VPN
		"tailscale1": ip("192.168.1.2"),   // falscher Bereich
		"wt":         ip("100.100.100.1"), // kein wtN
	}, 8096)
	want := map[string]string{"tailscale0": "tailscale", "wt0": "netbird", "utun4": "vpn"}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for _, a := range got {
		if want[a.Interface] != a.Provider {
			t.Errorf("%s: Anbieter %q", a.Interface, a.Provider)
		}
	}
	if u := Find(map[string][]netip.Addr{"wt0": ip("100.64.0.7")}, 8096)[0].URL; u != "http://100.64.0.7:8096" {
		t.Errorf("URL %s", u)
	}
}
