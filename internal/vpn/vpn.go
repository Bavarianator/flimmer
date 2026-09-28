// Package vpn erkennt, ob der Rechner in einem Tailscale- oder NetBird-Netz hängt, und nennt die Adresse,
// unter der Flimmer dort erreichbar ist. Es gibt keine Anbindung an die Daemons: Beide vergeben IPv4-Adressen
// aus 100.64.0.0/10 auf einer eigenen Schnittstelle, und Flimmer lauscht ohnehin auf allen Schnittstellen.
package vpn

import (
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
)

type Addr struct {
	Provider  string `json:"provider"` // tailscale, netbird oder vpn (macOS: nicht unterscheidbar)
	Interface string `json:"interface"`
	IP        string `json:"ip"`
	URL       string `json:"url"`
}

type Status struct {
	Addrs []Addr `json:"addrs"`
	Hint  string `json:"hint"`
}

var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// provider ordnet eine Schnittstelle zu. Nur bekannte Namen zählen, sonst hielte man die CGNAT-Adresse
// eines Mobilfunk-Routers für ein VPN.
func provider(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.HasPrefix(n, "tailscale"): // Linux tailscale0, Windows „Tailscale“
		return "tailscale"
	case strings.HasPrefix(n, "wt") && len(n) > 2 && n[2] >= '0' && n[2] <= '9': // NetBird wt0
		return "netbird"
	case strings.HasPrefix(n, "utun"): // macOS: beide nutzen utunN
		return "vpn"
	}
	return ""
}

// Find liefert die VPN-Adressen aus einer Liste von Schnittstellen (Name → IPv4-Adressen).
func Find(ifaces map[string][]netip.Addr, port int) []Addr {
	var out []Addr
	for name, ips := range ifaces {
		p := provider(name)
		if p == "" {
			continue
		}
		for _, ip := range ips {
			if ip.Is4() && cgnat.Contains(ip) {
				out = append(out, Addr{p, name, ip.String(), "http://" + net.JoinHostPort(ip.String(), strconv.Itoa(port))})
			}
		}
	}
	return out
}

// Detect liest die laufenden Schnittstellen des Rechners.
func Detect(port int) Status {
	m := map[string][]netip.Addr{}
	ifs, _ := net.Interfaces()
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if p, err := netip.ParsePrefix(a.String()); err == nil {
				m[ifc.Name] = append(m[ifc.Name], p.Addr())
			}
		}
	}
	st := Status{Addrs: Find(m, port)}
	if st.Addrs == nil {
		st.Addrs = []Addr{}
		st.Hint = "Kein Tailscale- oder NetBird-Netz gefunden. Der Rechner muss im Netz angemeldet sein. " +
			"Läuft Flimmer in Docker, gehört der Container ins Netz des Hosts oder eines Tailscale/NetBird-Containers (docs/livetv-vpn.md)."
		return st
	}
	st.Hint = "Geräte im selben Netz erreichen Flimmer unter " + st.Addrs[0].URL +
		". HTTP ist hier unkritisch, weil das VPN den Verkehr verschlüsselt."
	return st
}

// Handler beantwortet GET mit dem Status. port ist der HTTP-Port des Servers (nicht der HTTPS-Port des Fernzugriffs).
func Handler(port int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Detect(port))
	})
}
