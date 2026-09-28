package ratelimit

import (
	"net"
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	var l Limiter
	now := time.Now()
	a, b := net.ParseIP("2001:db8::1"), net.ParseIP("2001:db8::ffff") // gleiches /64
	for i := range 20 {
		if !l.Allow(a, now) {
			t.Fatalf("Anfrage %d abgelehnt", i)
		}
	}
	if l.Allow(b, now) {
		t.Fatal("gleiches /64 muss mitgezählt werden")
	}
	if !l.Allow(net.ParseIP("203.0.113.7"), now) {
		t.Fatal("anderer Absender betroffen")
	}
	if !l.Allow(a, now.Add(time.Second)) || l.Allow(a, now.Add(time.Second)) {
		t.Fatal("nach 1 s genau ein neues Token erwartet")
	}
	strict := Limiter{Burst: 2, Rate: 0.1}
	ip := net.ParseIP("198.51.100.1")
	if !strict.Allow(ip, now) || !strict.Allow(ip, now) || strict.Allow(ip, now) || strict.Allow(ip, now.Add(5*time.Second)) {
		t.Fatal("eigene Burst/Rate nicht beachtet")
	}
}
