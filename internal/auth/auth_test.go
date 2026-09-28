package auth

import (
	"testing"
	"time"
)

func TestPassword(t *testing.T) {
	h := HashPassword("geheim")
	if !CheckPassword(h, "geheim") || CheckPassword(h, "Geheim") || CheckPassword("murks", "geheim") {
		t.Fatal("Passwortprüfung falsch")
	}
}

func TestMediaToken(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	tok := MediaToken(secret, "anna", time.Hour)
	if u, ok := CheckMediaToken(secret, tok); !ok || u != "anna" {
		t.Fatalf("gültiges Token abgelehnt: %q %v", u, ok)
	}
	if _, ok := CheckMediaToken(secret, "bob"+tok[4:]); ok {
		t.Fatal("fremder Benutzer akzeptiert")
	}
	if _, ok := CheckMediaToken([]byte("anderer-schluessel"), tok); ok {
		t.Fatal("falscher Schlüssel akzeptiert")
	}
	if _, ok := CheckMediaToken(secret, MediaToken(secret, "anna", -time.Second)); ok {
		t.Fatal("abgelaufenes Token akzeptiert")
	}
}

func TestLimiter(t *testing.T) {
	l := &Limiter{Max: 2, Window: time.Minute}
	l.Fail("ip")
	l.Fail("ip")
	if l.Allow("ip") || !l.Allow("andere") {
		t.Fatal("Drossel greift falsch")
	}
}

func TestPairing(t *testing.T) {
	var p Pairing
	code, secret := p.Start("LG TV")
	if _, _, done, ok := p.Poll(code, secret); done || !ok {
		t.Fatal("unbestätigt schon fertig")
	}
	if _, _, _, ok := p.Poll(code, "falsch"); ok {
		t.Fatal("falscher Poll-Schlüssel akzeptiert")
	}
	if _, ok := p.Confirm(code, "anna"); !ok {
		t.Fatal("Bestätigen fehlgeschlagen")
	}
	if u, dev, done, _ := p.Poll(code, secret); !done || u != "anna" || dev != "LG TV" {
		t.Fatal("Kopplung nicht abgeschlossen")
	}
	if _, _, _, ok := p.Poll(code, secret); ok {
		t.Fatal("Code zweimal nutzbar")
	}
}
