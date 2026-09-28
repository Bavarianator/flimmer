// Package auth: Passwort-Hashes, Session-Tokens, signierte Medien-Tokens, Login-Drossel und TV-Kopplung.
// Nur Standardbibliothek.
package auth

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ponytail: PBKDF2 aus der Stdlib statt argon2id (x/crypto); Iterationen nach OWASP 2023.
const iterations = 600_000

var b64 = base64.RawURLEncoding

// HashPassword liefert "pbkdf2-sha256$<iter>$<salt>$<hash>".
func HashPassword(pw string) string {
	salt := make([]byte, 16)
	rand.Read(salt)
	key, _ := pbkdf2.Key(sha256.New, pw, salt, iterations, 32)
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", iterations, b64.EncodeToString(salt), b64.EncodeToString(key))
}

func CheckPassword(hash, pw string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err1 := strconv.Atoi(parts[1])
	salt, err2 := b64.DecodeString(parts[2])
	want, err3 := b64.DecodeString(parts[3])
	if err1 != nil || err2 != nil || err3 != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, pw, salt, iter, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

// NewToken liefert ein zufälliges Session-Token und den Hash, der gespeichert wird.
func NewToken() (token, hash string) {
	b := make([]byte, 32)
	rand.Read(b)
	token = b64.EncodeToString(b)
	return token, HashToken(token)
}

func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// MediaToken signiert userID und Ablauf, weil <video src> und TVs keine Header oder Cookies mitschicken.
// Format: <userID>.<unix-ablauf>.<hmac>, URL-sicher.
func MediaToken(secret []byte, userID string, ttl time.Duration) string {
	payload := userID + "." + strconv.FormatInt(time.Now().Add(ttl).Unix(), 10)
	return payload + "." + sign(secret, payload)
}

// CheckMediaToken liefert die Benutzer-ID, wenn Signatur und Ablauf stimmen.
func CheckMediaToken(secret []byte, tok string) (string, bool) {
	i := strings.LastIndexByte(tok, '.')
	if i < 0 || !hmac.Equal([]byte(tok[i+1:]), []byte(sign(secret, tok[:i]))) {
		return "", false
	}
	user, exp, ok := strings.Cut(tok[:i], ".")
	sec, err := strconv.ParseInt(exp, 10, 64)
	if !ok || err != nil || time.Now().Unix() > sec {
		return "", false
	}
	return user, true
}

func sign(secret []byte, s string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(s))
	return b64.EncodeToString(m.Sum(nil)[:18])
}

// Limiter drosselt Fehlversuche pro Schlüssel (IP): höchstens max im Zeitfenster.
type Limiter struct {
	Max    int
	Window time.Duration

	mu    sync.Mutex
	fails map[string][]time.Time
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key)) < l.Max
}

func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fails == nil {
		l.fails = map[string][]time.Time{}
	}
	l.fails[key] = append(l.recent(key), time.Now())
}

func (l *Limiter) recent(key string) []time.Time {
	var out []time.Time
	for _, t := range l.fails[key] {
		if time.Since(t) < l.Window {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		delete(l.fails, key) // Map wächst nicht mit jeder IP, die je vorbeikam
	}
	return out
}
