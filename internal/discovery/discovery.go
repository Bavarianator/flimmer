// Package discovery macht den Server im Heimnetz auffindbar: SSDP-Antworten für Apps (Android, iOS, Desktop)
// und ein QR-Code mit der LAN-Adresse fürs Handy.
// Hinweis: Web-Apps auf webOS/Tizen können kein UDP senden; dort bleibt Adresse eintippen bzw. QR/Kopplung.
package discovery

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"strconv"
	"strings"

	"rsc.io/qr"
)

// ST ist der Suchtyp, nach dem Flimmer-Apps fragen.
const ST = "urn:flimmer-media:service:flimmer:1"

var ssdpAddr = &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}

// Start beantwortet SSDP-Suchen, bis ctx endet. Ist Port 1900 nicht nutzbar (z. B. Rechte, kein Multicast),
// kommt ein Fehler zurück – der Server läuft ohne Discovery weiter.
func Start(ctx context.Context, name string, port int) error {
	conn, err := net.ListenMulticastUDP("udp4", nil, ssdpAddr) // setzt SO_REUSEADDR: verträgt sich mit anderen DLNA-Servern
	if err != nil {
		return fmt.Errorf("discovery: %w", err)
	}
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	go serve(conn, name, port)
	return nil
}

func serve(conn net.PacketConn, name string, port int) {
	buf := make([]byte, 2048)
	for {
		n, from, err := conn.ReadFrom(buf)
		if err != nil {
			return // geschlossen
		}
		addr, ok := from.(*net.UDPAddr)
		if !ok || !wanted(buf[:n]) {
			continue
		}
		ip := localIPFor(addr)
		if ip == nil {
			continue
		}
		if _, err := conn.WriteTo(response(name, "http://"+net.JoinHostPort(ip.String(), strconv.Itoa(port))), addr); err != nil {
			log.Printf("discovery: %v", err)
		}
	}
}

// wanted prüft, ob das Paket ein M-SEARCH nach Flimmer oder nach allem ist.
func wanted(pkt []byte) bool {
	r := textproto.NewReader(bufio.NewReader(bytes.NewReader(pkt)))
	line, err := r.ReadLine()
	if err != nil || !strings.HasPrefix(line, "M-SEARCH ") {
		return false
	}
	h, err := r.ReadMIMEHeader()
	if err != nil && len(h) == 0 {
		return false
	}
	st := h.Get("St")
	return strings.Contains(h.Get("Man"), "ssdp:discover") && (st == ST || st == "ssdp:all")
}

func response(name, url string) []byte {
	host, _ := os.Hostname()
	h := sha1.Sum([]byte(host + name))
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x", h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
	return []byte("HTTP/1.1 200 OK\r\n" +
		"CACHE-CONTROL: max-age=1800\r\n" +
		"EXT:\r\n" +
		"LOCATION: " + url + "/\r\n" +
		"SERVER: Flimmer UPnP/1.1\r\n" +
		"ST: " + ST + "\r\n" +
		"USN: uuid:" + uuid + "::" + ST + "\r\n" +
		"X-FLIMMER-NAME: " + strings.NewReplacer("\r", "", "\n", "").Replace(name) + "\r\n" +
		"\r\n")
}

// localIPFor liefert die eigene Adresse, über die der Fragende erreichbar ist (richtiges Interface bei mehreren Netzen).
func localIPFor(remote *net.UDPAddr) net.IP {
	c, err := net.DialUDP("udp4", nil, remote) // UDP-„Dial“ sendet nichts, fragt nur die Routing-Tabelle
	if err != nil {
		return nil
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP
}

// LANURL rät die Adresse, unter der Handy und TV den Server erreichen.
// ponytail: Heuristik – 192.168.x vor 10.x vor 172.16/12, virtuelle und VPN-Interfaces ausgelassen;
// bei exotischen Netzen zeigt die Einrichtung ohnehin alle Adressen zur Auswahl.
func LANURL(port int) string {
	best, rank := "", 0
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || virtual(iface.Name) {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if !ok || n.IP.To4() == nil || !n.IP.IsPrivate() {
				continue
			}
			r := 1
			switch ip := n.IP.To4(); {
			case ip[0] == 192:
				r = 3
			case ip[0] == 10:
				r = 2
			}
			if r > rank {
				best, rank = n.IP.String(), r
			}
		}
	}
	if best == "" {
		best = "localhost"
	}
	return "http://" + net.JoinHostPort(best, strconv.Itoa(port))
}

func virtual(name string) bool {
	for _, p := range []string{"docker", "br-", "veth", "virbr", "vnet", "tun", "tap", "wg", "tailscale", "zt", "utun", "vmnet", "vboxnet", "proton"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// QR liefert ein PNG mit dem QR-Code für url.
func QR(url string) ([]byte, error) {
	c, err := qr.Encode(url, qr.M)
	if err != nil {
		return nil, err
	}
	c.Scale = 8
	return c.PNG(), nil
}

// QRHandler: GET /api/qr → PNG mit der LAN-Adresse (für Einrichtung und Einstellungen).
func QRHandler(port int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		png, err := QR(LANURL(port))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-cache") // Adresse kann sich ändern (DHCP)
		w.Write(png)
	}
}
