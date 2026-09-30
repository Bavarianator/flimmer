package discovery

import (
	"bytes"
	"net"
	"testing"
)

func query(id uint16, questions ...[]byte) []byte {
	b := []byte{byte(id >> 8), byte(id), 0, 0, 0, byte(len(questions)), 0, 0, 0, 0, 0, 0}
	for _, q := range questions {
		b = append(b, q...)
	}
	return b
}

func TestMDNSAnswer(t *testing.T) {
	ip := net.IPv4(192, 168, 55, 204).To4()
	a := []byte("\x07FLIMMER\x05local\x00\x00\x01\x00\x01") // Groß/klein egal

	resp, uni := mdnsAnswer(query(0, a), ip, false)
	if resp == nil || uni || resp[2] != 0x84 || resp[5] != 0 || resp[7] != 1 || !bytes.HasSuffix(resp, ip) {
		t.Fatalf("A: % x %v", resp, uni)
	}
	if _, uni := mdnsAnswer(query(0, []byte("\x07flimmer\x05local\x00\x00\x01\x80\x01")), ip, false); !uni {
		t.Error("QU-Bit nicht erkannt")
	}
	// Einfacher Resolver (anderer Port): Kennung und Frage kommen zurück.
	resp, _ = mdnsAnswer(query(0x1234, a), ip, true)
	if resp == nil || resp[0] != 0x12 || resp[1] != 0x34 || resp[5] != 1 || !bytes.HasSuffix(resp, ip) {
		t.Fatalf("legacy: % x", resp)
	}
	// Zweite Frage mit Zeiger auf „local“ aus der ersten (Offset 12+4 = 16).
	other := []byte("\x03foo\x05local\x00\x00\x01\x00\x01")
	if resp, _ := mdnsAnswer(query(0, other, []byte("\x07flimmer\xc0\x10\x00\x01\x00\x01")), ip, false); resp == nil {
		t.Error("Kompressionszeiger")
	}
	for name, msg := range map[string][]byte{
		"fremder Name":   query(0, other),
		"AAAA":           query(0, []byte("\x07flimmer\x05local\x00\x00\x1c\x00\x01")),
		"Antwort":        append([]byte{0, 0, 0x84, 0}, query(0, a)[4:]...),
		"abgeschnitten":  query(0, a)[:20],
		"Zeigerschleife": query(0, []byte("\xc0\x0c\x00\x01\x00\x01")),
	} {
		if resp, _ := mdnsAnswer(msg, ip, false); resp != nil {
			t.Errorf("%s: sollte nil sein", name)
		}
	}
}
