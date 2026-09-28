package remote

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

// Request ist eine vom Server signierte Nachricht an das Relay (und die Antwort auf dessen Ping).
// Kind trennt die Zwecke, damit eine Signatur nicht für etwas anderes wiederverwendet werden kann.
type Request struct {
	ID   string            `json:"id"`
	Pub  ed25519.PublicKey `json:"pub"`
	Data string            `json:"data"` // register: URL, ping: Nonce, acme: TXT-Wert, pair: leer
	TS   int64             `json:"ts"`
	Sig  []byte            `json:"sig"`
}

// MaxSkew ist die erlaubte Uhrabweichung signierter Nachrichten.
const MaxSkew = 5 * time.Minute

// ServerID leitet die Server-ID aus dem öffentlichen Schlüssel ab: 16 Zeichen, taugt als DNS-Label.
func ServerID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return strings.ToLower(base32.StdEncoding.EncodeToString(sum[:10]))
}

func message(kind, id, data string, ts int64) []byte {
	return []byte("flimmer-v1\n" + kind + "\n" + id + "\n" + data + "\n" + strconv.FormatInt(ts, 10))
}

// Sign erzeugt eine signierte Nachricht.
func Sign(kind string, key ed25519.PrivateKey, data string) Request {
	pub := key.Public().(ed25519.PublicKey)
	r := Request{ID: ServerID(pub), Pub: pub, Data: data, TS: time.Now().Unix()}
	r.Sig = ed25519.Sign(key, message(kind, r.ID, data, r.TS))
	return r
}

// Verify prüft Signatur, ID und Zeitstempel.
func (r Request) Verify(kind string, now time.Time) error {
	if len(r.Pub) != ed25519.PublicKeySize || r.ID != ServerID(r.Pub) {
		return errors.New("ID passt nicht zum Schlüssel")
	}
	if d := now.Sub(time.Unix(r.TS, 0)); d > MaxSkew || d < -MaxSkew {
		return errors.New("Zeitstempel zu alt oder Uhr falsch gestellt")
	}
	if !ed25519.Verify(r.Pub, message(kind, r.ID, r.Data, r.TS), r.Sig) {
		return errors.New("Signatur ungültig")
	}
	return nil
}

// LoadKey liest den Server-Schlüssel oder legt ihn beim ersten Start an (nur für den Besitzer lesbar).
func LoadKey(path string) (ed25519.PrivateKey, error) {
	seed, err := os.ReadFile(path)
	if err == nil && len(seed) == ed25519.SeedSize {
		return ed25519.NewKeyFromSeed(seed), nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		return nil, errors.New(path + ": Schlüsseldatei beschädigt")
	}
	seed = make([]byte, ed25519.SeedSize)
	rand.Read(seed)
	if err := os.WriteFile(path, seed, 0o600); err != nil {
		return nil, err
	}
	return ed25519.NewKeyFromSeed(seed), nil
}
