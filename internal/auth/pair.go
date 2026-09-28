package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// Pairing koppelt einen TV ohne Tastatur (angelehnt an RFC 8628): Der TV zeigt einen 6-stelligen Code,
// ein angemeldetes Handy bestätigt ihn, der TV holt sich dann mit seinem geheimen Poll-Schlüssel das Token.
type Pairing struct {
	mu      sync.Mutex
	pending map[string]*pair // Code → Kopplung
}

type pair struct {
	secret  string
	device  string
	expires time.Time
	userID  string // gesetzt, sobald bestätigt
}

const pairTTL = 10 * time.Minute

// Start legt einen Code an. secret bekommt nur der TV.
func (p *Pairing) Start(device string) (code, secret string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.expire()
	if p.pending == nil {
		p.pending = map[string]*pair{}
	}
	for {
		n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000))
		code = fmt.Sprintf("%06d", n)
		if p.pending[code] == nil {
			break
		}
	}
	secret, _ = NewToken()
	p.pending[code] = &pair{secret: secret, device: device, expires: time.Now().Add(pairTTL)}
	return code, secret
}

// Confirm ordnet den Code einem Benutzer zu (vom angemeldeten Handy aus).
func (p *Pairing) Confirm(code, userID string) (device string, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.expire()
	pr := p.pending[code]
	if pr == nil || pr.userID != "" {
		return "", false
	}
	pr.userID = userID
	return pr.device, true
}

// Poll liefert den Benutzer, sobald bestätigt; danach ist der Code verbraucht.
func (p *Pairing) Poll(code, secret string) (userID, device string, done, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.expire()
	pr := p.pending[code]
	if pr == nil || pr.secret != secret {
		return "", "", false, false
	}
	if pr.userID == "" {
		return "", "", false, true
	}
	delete(p.pending, code)
	return pr.userID, pr.device, true, true
}

func (p *Pairing) expire() {
	for c, pr := range p.pending {
		if time.Now().After(pr.expires) {
			delete(p.pending, c)
		}
	}
}
