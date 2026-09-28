package remote

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const pmpLease = 2 * time.Hour

// pmpMap gibt den Port per NAT-PMP (RFC 6886) frei; gw ist "ip:5351".
func pmpMap(ctx context.Context, gw string, port int) (*mapping, error) {
	resp, err := udpExchange(ctx, gw, []byte{0, 0}, func(b []byte) bool { return len(b) >= 12 && b[0] == 0 && b[1] == 128 })
	if err != nil {
		return nil, fmt.Errorf("NAT-PMP: %w", err)
	}
	if rc := binary.BigEndian.Uint16(resp[2:]); rc != 0 {
		return nil, fmt.Errorf("NAT-PMP: Router meldet Fehler %d", rc)
	}
	m := &mapping{method: "nat-pmp", extIP: net.IP(resp[8:12]).To16(), lease: pmpLease}
	req := func(ctx context.Context, lease time.Duration, ext int) error {
		b := make([]byte, 12)
		b[1] = 2 // TCP
		binary.BigEndian.PutUint16(b[4:], uint16(port))
		binary.BigEndian.PutUint16(b[6:], uint16(ext))
		binary.BigEndian.PutUint32(b[8:], uint32(lease.Seconds()))
		resp, err := udpExchange(ctx, gw, b, func(b []byte) bool {
			return len(b) >= 16 && b[0] == 0 && b[1] == 130 && binary.BigEndian.Uint16(b[8:]) == uint16(port)
		})
		if err != nil {
			return fmt.Errorf("NAT-PMP: %w", err)
		}
		if rc := binary.BigEndian.Uint16(resp[2:]); rc != 0 {
			return fmt.Errorf("NAT-PMP: Router meldet Fehler %d", rc)
		}
		if lease > 0 {
			m.extPort = int(binary.BigEndian.Uint16(resp[10:]))
			m.lease = time.Duration(binary.BigEndian.Uint32(resp[12:])) * time.Second
		}
		return nil
	}
	if err := req(ctx, pmpLease, port); err != nil {
		return nil, err
	}
	m.renew = func(ctx context.Context) error { return req(ctx, pmpLease, m.extPort) }
	m.remove = func(ctx context.Context) error { return req(ctx, 0, 0) }
	return m, nil
}

// pcpMap gibt den Port per PCP (RFC 6887, MAP-Opcode) frei.
func pcpMap(ctx context.Context, gw string, port int) (*mapping, error) {
	host, _, _ := net.SplitHostPort(gw)
	local, err := localIPTo(host)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, 12)
	rand.Read(nonce)
	m := &mapping{method: "pcp", lease: pmpLease}
	req := func(ctx context.Context, lease time.Duration, ext int, extIP net.IP) error {
		b := make([]byte, 60)
		b[0], b[1] = 2, 1 // Version 2, MAP
		binary.BigEndian.PutUint32(b[4:], uint32(lease.Seconds()))
		copy(b[8:24], local.To16())
		copy(b[24:36], nonce)
		b[36] = 6 // TCP
		binary.BigEndian.PutUint16(b[40:], uint16(port))
		binary.BigEndian.PutUint16(b[42:], uint16(ext))
		copy(b[44:60], extIP.To16())
		resp, err := udpExchange(ctx, gw, b, func(r []byte) bool {
			return len(r) >= 60 && r[1] == 0x81 && bytes.Equal(r[24:36], nonce) || len(r) >= 4 && r[0] != 2
		})
		if err != nil {
			return fmt.Errorf("PCP: %w", err)
		}
		if resp[0] != 2 {
			return errors.New("PCP: Router kann kein PCP")
		}
		if rc := resp[3]; rc != 0 {
			return fmt.Errorf("PCP: Router meldet Fehler %d", rc)
		}
		if lease > 0 {
			m.lease = time.Duration(binary.BigEndian.Uint32(resp[4:])) * time.Second
			m.extPort = int(binary.BigEndian.Uint16(resp[42:]))
			m.extIP = net.IP(append([]byte(nil), resp[44:60]...))
		}
		return nil
	}
	if err := req(ctx, pmpLease, port, net.IPv4zero); err != nil {
		return nil, err
	}
	m.renew = func(ctx context.Context) error { return req(ctx, pmpLease, m.extPort, m.extIP) }
	m.remove = func(ctx context.Context) error { return req(ctx, 0, m.extPort, m.extIP) }
	return m, nil
}

// udpExchange sendet req und wartet mit Wiederholungen (250 ms, verdoppelnd) auf eine passende Antwort.
func udpExchange(ctx context.Context, addr string, req []byte, ok func([]byte) bool) ([]byte, error) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	buf := make([]byte, 1100)
	for wait := 250 * time.Millisecond; wait <= 2*time.Second; wait *= 2 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := conn.Write(req); err != nil {
			return nil, err
		}
		conn.SetReadDeadline(time.Now().Add(wait))
		for {
			n, err := conn.Read(buf)
			if err != nil {
				var ne net.Error
				if errors.As(err, &ne) && ne.Timeout() {
					break
				}
				return nil, errors.New("keine Antwort vom Router") // z. B. ICMP „Port unreachable“
			}
			if ok(buf[:n]) {
				return buf[:n], nil
			}
		}
	}
	return nil, errors.New("keine Antwort vom Router")
}

// gateway liefert das Standard-Gateway (IPv4).
func gateway() (net.IP, error) {
	if f, err := os.Open("/proc/net/route"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			fs := strings.Fields(sc.Text())
			if len(fs) > 2 && fs[1] == "00000000" {
				v, err := strconv.ParseUint(fs[2], 16, 32)
				if err == nil && v != 0 {
					ip := make(net.IP, 4)
					binary.LittleEndian.PutUint32(ip, uint32(v))
					return ip, nil
				}
			}
		}
	}
	// ponytail: außerhalb von Linux wird „.1“ im eigenen /24 geraten; echte Routing-Tabelle (sysctl/iphlpapi) wenn das oft danebenliegt
	local, err := localIPTo("192.0.2.1")
	if err != nil {
		return nil, err
	}
	ip := local.To4()
	if ip == nil {
		return nil, errors.New("keine IPv4-Adresse im Heimnetz")
	}
	return net.IPv4(ip[0], ip[1], ip[2], 1), nil
}

// localIPTo liefert die eigene Adresse, über die host erreicht wird (UDP-Dial sendet nichts).
func localIPTo(host string) (net.IP, error) {
	c, err := net.Dial("udp", net.JoinHostPort(host, "9"))
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP, nil
}
