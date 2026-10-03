package api

import (
	"testing"
	"time"
)

func TestBeatSehzeit(t *testing.T) {
	var s streams
	schritt := func(vorher time.Duration, paused bool) float64 {
		if st := s.m["k"]; st != nil {
			st.beat = time.Now().Add(-vorher)
		}
		sek, _ := s.beat("k", stream{}, 0, 100, paused)
		return sek
	}
	if sek := schritt(0, false); sek != 0 {
		t.Fatalf("erster Herzschlag: %v", sek)
	}
	if sek := schritt(10*time.Second, false); sek < 9.9 || sek > 11 {
		t.Fatalf("10 s gesehen: %v", sek)
	}
	if sek := schritt(10*time.Second, true); sek != 0 {
		t.Fatalf("pausiert: %v", sek)
	}
	if sek := schritt(10*time.Second, false); sek != 0 {
		t.Fatalf("nach der Pause: %v", sek)
	}
	if sek := schritt(40*time.Second, false); sek != 0 {
		t.Fatalf("Lücke über 30 s: %v", sek)
	}
}
