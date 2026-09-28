// Package ratelimit ist ein Token-Bucket pro Absender: je IPv4-Adresse bzw. IPv6-/64 (sonst umgeht man
// es mit einer neuen Adresse aus dem eigenen Präfix).
package ratelimit

import (
	"net"
	"net/netip"
	"sync"
	"time"
)

type Limiter struct {
	Burst float64 // Anfragen am Stück, 0 = 20
	Rate  float64 // danach pro Sekunde, 0 = 1

	mu sync.Mutex
	m  map[netip.Addr]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

// Allow verbraucht ein Token für ip und sagt, ob die Anfrage durch darf.
func (l *Limiter) Allow(ip net.IP, now time.Time) bool {
	burst, rate := l.Burst, l.Rate
	if burst == 0 {
		burst = 20
	}
	if rate == 0 {
		rate = 1
	}
	a, _ := netip.AddrFromSlice(ip)
	a = a.Unmap()
	if a.Is6() {
		a = netip.PrefixFrom(a, 64).Masked().Addr()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.m == nil {
		l.m = map[netip.Addr]*bucket{}
	}
	if len(l.m) > 100_000 { // volle Buckets vergessen; ponytail: O(n) Aufräumen, das nur unter Last läuft
		for k, b := range l.m {
			if now.Sub(b.last).Seconds()*rate >= burst {
				delete(l.m, k)
			}
		}
	}
	b := l.m[a]
	if b == nil {
		b = &bucket{tokens: burst, last: now}
		l.m[a] = b
	}
	b.tokens = min(burst, b.tokens+now.Sub(b.last).Seconds()*rate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
