package remote

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/acme"
)

// fakeACME spricht genug RFC 8555, um x/crypto/acme ein Zertifikat auszustellen. Die DNS-01-Prüfung
// schaut in dns (vom Fake-Relay befüllt), statt echtes DNS zu fragen. JWS-Signaturen prüft er nicht.
type fakeACME struct {
	url     string
	dns     map[string]string
	accKey  string // Pfad zu account.key, um den erwarteten TXT-Wert zu berechnen
	domain  string
	authzOK bool
	invalid bool
	orders  int
	certPEM []byte
	ca      *x509.Certificate
	caKey   *ecdsa.PrivateKey
}

func (f *fakeACME) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Replay-Nonce", rand.Text())
	if r.Method == "HEAD" {
		return
	}
	var jws struct{ Payload string }
	json.NewDecoder(r.Body).Decode(&jws)
	payload, _ := base64.RawURLEncoding.DecodeString(jws.Payload)
	status := func() string {
		switch {
		case f.authzOK:
			return "valid"
		case f.invalid:
			return "invalid"
		}
		return "pending"
	}
	order := func() map[string]any {
		st := "pending"
		if f.authzOK {
			st = "ready"
		}
		if f.certPEM != nil {
			st = "valid"
		}
		return map[string]any{"status": st, "identifiers": []any{map[string]string{"type": "dns", "value": f.domain}},
			"authorizations": []string{f.url + "/authz/1"}, "finalize": f.url + "/finalize/1", "certificate": f.url + "/cert/1"}
	}
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/dir":
		json.NewEncoder(w).Encode(map[string]string{"newNonce": f.url + "/nonce", "newAccount": f.url + "/acct", "newOrder": f.url + "/order"})
	case "/acct":
		w.Header().Set("Location", f.url+"/acct/1")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"status":"valid"}`)
	case "/order":
		f.orders++
		f.authzOK, f.invalid, f.certPEM = false, false, nil
		w.Header().Set("Location", f.url+"/order/1")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(order())
	case "/order/1":
		w.Header().Set("Location", f.url+"/order/1")
		json.NewEncoder(w).Encode(order())
	case "/authz/1":
		json.NewEncoder(w).Encode(map[string]any{"status": status(), "identifier": map[string]string{"type": "dns", "value": f.domain},
			"challenges": []any{map[string]string{"type": "dns-01", "url": f.url + "/chal/1", "token": "tok123", "status": status()}}})
	case "/chal/1":
		key, _ := loadECKey(f.accKey)
		want, _ := (&acme.Client{Key: key}).DNS01ChallengeRecord("tok123")
		f.authzOK = f.dns["_acme-challenge."+f.domain] == want
		f.invalid = !f.authzOK
		json.NewEncoder(w).Encode(map[string]string{"type": "dns-01", "url": f.url + "/chal/1", "token": "tok123", "status": status()})
	case "/finalize/1":
		var p struct{ CSR string }
		json.Unmarshal(payload, &p)
		der, _ := base64.RawURLEncoding.DecodeString(p.CSR)
		csr, err := x509.ParseCertificateRequest(der)
		if err != nil || !f.authzOK || len(csr.DNSNames) != 1 || csr.DNSNames[0] != f.domain {
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, `{"type":"urn:ietf:params:acme:error:unauthorized"}`)
			return
		}
		tpl := &x509.Certificate{SerialNumber: big.NewInt(2), DNSNames: csr.DNSNames, NotBefore: time.Now().Add(-time.Hour),
			NotAfter: time.Now().Add(90 * 24 * time.Hour), ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
		leaf, _ := x509.CreateCertificate(rand.Reader, tpl, f.ca, csr.PublicKey, f.caKey)
		f.certPEM = append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf}),
			pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.ca.Raw})...)
		w.Header().Set("Location", f.url+"/order/1")
		json.NewEncoder(w).Encode(order())
	case "/cert/1":
		w.Header().Set("Content-Type", "application/pem-certificate-chain")
		w.Write(f.certPEM)
	default:
		http.NotFound(w, r)
	}
}

func TestACME(t *testing.T) {
	dir := t.TempDir()
	dns := map[string]string{}
	var dnsMu sync.Mutex

	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)
		if r.URL.Path != "/v1/acme" || req.Verify("acme", time.Now()) != nil {
			http.Error(w, `{"error":"nein"}`, http.StatusUnauthorized)
			return
		}
		dnsMu.Lock()
		dns["_acme-challenge."+req.ID+".flimmer.direct"] = req.Data
		dnsMu.Unlock()
		io.WriteString(w, `{"ok":true}`)
	}))
	defer relay.Close()

	r, err := New(Options{Port: 8097, KeyFile: filepath.Join(dir, "remote.key"), RelayURL: relay.URL,
		TLSPort: 8443, TLSDir: filepath.Join(dir, "tls")})
	if err != nil {
		t.Fatal(err)
	}

	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Fake CA"}, IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(1, 0, 0)}
	caDER, _ := x509.CreateCertificate(rand.Reader, caTpl, caTpl, &caKey.PublicKey, caKey)
	ca, _ := x509.ParseCertificate(caDER)
	fa := &fakeACME{dns: dns, accKey: filepath.Join(dir, "tls", "account.key"), domain: r.Domain(), ca: ca, caKey: caKey}
	acmeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		dnsMu.Lock()
		defer dnsMu.Unlock()
		fa.ServeHTTP(w, req)
	}))
	defer acmeSrv.Close()
	fa.url = acmeSrv.URL
	r.opts.ACMEURL = acmeSrv.URL + "/dir"

	// Vorher: selbst signiertes Zertifikat, damit der Rückruf des Relays schon klappt.
	c, err := r.TLSConfig().GetCertificate(nil)
	if err != nil || c.Leaf.Issuer.CommonName != r.Domain() {
		t.Fatalf("Fallback: %v %v", err, c)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := r.ensureCert(ctx); err != nil {
		t.Fatal(err)
	}
	c, _ = r.TLSConfig().GetCertificate(nil)
	if c.Leaf.Issuer.CommonName != "Fake CA" || c.Leaf.VerifyHostname(r.Domain()) != nil {
		t.Fatalf("Zertifikat: %v %v", c.Leaf.Issuer, c.Leaf.DNSNames)
	}
	if v := dns["_acme-challenge."+r.Domain()]; v != "" {
		t.Fatalf("TXT nicht gelöscht: %q", v)
	}

	// Noch 90 Tage gültig: keine neue Bestellung, auch nach Neustart (von Platte geladen).
	r2, _ := New(r.opts)
	if err := r2.ensureCert(ctx); err != nil || fa.orders != 1 {
		t.Fatalf("Erneuerung zu früh: %v, %d Aufträge", err, fa.orders)
	}
	c2, _ := r2.TLSConfig().GetCertificate(nil)
	if c2.Leaf.SerialNumber.Cmp(c.Leaf.SerialNumber) != 0 {
		t.Fatal("Zertifikat nicht von Platte geladen")
	}
	// Relay setzt falschen TXT → Fehler statt Zertifikat.
	r3, _ := New(Options{Port: 1, KeyFile: filepath.Join(dir, "other.key"), RelayURL: relay.URL, TLSPort: 8443,
		TLSDir: filepath.Join(dir, "tls3"), ACMEURL: acmeSrv.URL + "/dir"})
	if err := r3.ensureCert(ctx); err == nil || !strings.Contains(err.Error(), "ACME") {
		t.Fatalf("anderer Name muss scheitern: %v", err)
	}
}
