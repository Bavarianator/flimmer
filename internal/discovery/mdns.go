package discovery

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strings"
)

// mDNS (RFC 6762): Flimmer antwortet auf „flimmer.local“ mit seiner LAN-Adresse. So findet der Starter auf LG und
// Samsung den Server ohne IP-Eingabe (Web-Apps können kein SSDP, der Fernseher löst .local-Namen aber selbst auf).
// ponytail: nur A-Einträge für genau diesen Namen; laufen zwei Flimmer im selben Netz, antworten beide.

// Host ist der feste Name im Heimnetz.
const Host = "flimmer.local"

var mdnsAddr = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

// StartMDNS beantwortet Anfragen nach Host, bis ctx endet. Port 5353 teilt es sich mit Avahi & Co. (SO_REUSEADDR).
func StartMDNS(ctx context.Context) error {
	conn, err := net.ListenMulticastUDP("udp4", nil, mdnsAddr)
	if err != nil {
		return fmt.Errorf("mDNS: %w", err)
	}
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	go func() {
		buf := make([]byte, 1500)
		for {
			n, from, err := conn.ReadFromUDP(buf)
			if err != nil {
				return // geschlossen
			}
			ip := localIPFor(from)
			if ip == nil {
				continue
			}
			legacy := from.Port != mdnsAddr.Port // einfache Resolver fragen von einem beliebigen Port
			resp, unicast := mdnsAnswer(buf[:n], ip.To4(), legacy)
			if resp == nil {
				continue
			}
			to := mdnsAddr
			if legacy || unicast {
				to = from
			}
			if _, err := conn.WriteToUDP(resp, to); err != nil {
				log.Printf("mDNS: %v", err)
			}
		}
	}()
	return nil
}

// mdnsAnswer baut die Antwort auf eine Frage nach Host (Typ A oder ANY). unicast = der Fragende wünscht eine
// direkte Antwort (QU-Bit). nil bei Antworten, fremden Namen und kaputten Paketen.
func mdnsAnswer(msg []byte, ip net.IP, legacy bool) ([]byte, bool) {
	if len(msg) < 12 || msg[2]&0x80 != 0 || len(ip) != 4 {
		return nil, false
	}
	off := 12
	for range int(binary.BigEndian.Uint16(msg[4:])) {
		name, next, ok := readName(msg, off)
		if !ok || next+4 > len(msg) {
			return nil, false
		}
		qtype, qclass := binary.BigEndian.Uint16(msg[next:]), binary.BigEndian.Uint16(msg[next+2:])
		off = next + 4
		if strings.EqualFold(name, Host) && (qtype == 1 || qtype == 255) && qclass&0x7fff == 1 {
			return mdnsReply(msg[:2], ip, legacy), qclass&0x8000 != 0
		}
	}
	return nil, false
}

func mdnsReply(id []byte, ip net.IP, legacy bool) []byte {
	name := []byte("\x07flimmer\x05local\x00")
	b := make([]byte, 12, 64)
	ttl, class := uint32(120), uint16(0x8001) // Cache-Flush: der Name gehört nur Flimmer
	if legacy {                               // §6.7: Kennung und Frage zurückgeben, kurze TTL, ohne Cache-Flush
		copy(b, id)
		b[5] = 1
		b = append(b, name...)
		b = binary.BigEndian.AppendUint16(b, 1)
		b = binary.BigEndian.AppendUint16(b, 1)
		ttl, class = 10, 1
	}
	b[2], b[7] = 0x84, 1 // Antwort, autoritativ; ein Eintrag
	b = append(b, name...)
	b = binary.BigEndian.AppendUint16(b, 1) // A
	b = binary.BigEndian.AppendUint16(b, class)
	b = binary.BigEndian.AppendUint32(b, ttl)
	b = binary.BigEndian.AppendUint16(b, 4)
	return append(b, ip...)
}

// readName liest einen DNS-Namen ab off, auch mit Kompressionszeigern, und liefert die Position danach.
func readName(msg []byte, off int) (string, int, bool) {
	var parts []string
	next := -1
	for jumps := 0; jumps < 16; {
		if off >= len(msg) {
			return "", 0, false
		}
		switch l := int(msg[off]); {
		case l == 0:
			if next < 0 {
				next = off + 1
			}
			return strings.Join(parts, "."), next, true
		case l&0xc0 == 0xc0:
			if off+1 >= len(msg) {
				return "", 0, false
			}
			if next < 0 {
				next = off + 2
			}
			off = int(binary.BigEndian.Uint16(msg[off:]) & 0x3fff)
			jumps++
		case l > 63 || off+1+l > len(msg):
			return "", 0, false
		default:
			parts = append(parts, string(msg[off+1:off+1+l]))
			off += 1 + l
		}
	}
	return "", 0, false // Zeigerschleife
}
