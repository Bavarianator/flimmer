package remote

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/acme"
)

// renewBefore: so lange vor Ablauf wird das Zertifikat erneuert.
const renewBefore = 30 * 24 * time.Hour

// Domain ist der öffentliche Name dieses Servers.
func (r *Remote) Domain() string { return r.ID() + "." + r.opts.Zone }

// TLSConfig ist für den zweiten (HTTPS-)Listener. Bis ein ACME-Zertifikat da ist, antwortet er mit einem
// selbst signierten; das reicht für den Rückruf des Relays, weil der Ping ohnehin signiert ist.
func (r *Remote) TLSConfig() *tls.Config {
	return &tls.Config{GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return r.currentCert() }}
}

func (r *Remote) currentCert() (*tls.Certificate, error) {
	r.certMu.Lock()
	defer r.certMu.Unlock()
	if r.cert != nil {
		return r.cert, nil
	}
	if c, err := tls.LoadX509KeyPair(r.tlsFile("cert.pem"), r.tlsFile("key.pem")); err == nil {
		r.cert = &c
		return r.cert, nil
	}
	c, err := selfSigned(r.Domain())
	if err != nil {
		return nil, err
	}
	r.cert = c
	return c, nil
}

func (r *Remote) tlsFile(name string) string { return filepath.Join(r.opts.TLSDir, name) }

// ensureCert holt per ACME DNS-01 ein Zertifikat für Domain(), wenn keines da ist oder es in weniger als
// 30 Tagen abläuft. Den TXT-Record setzt das Relay (POST /v1/acme); Zertifikat und Schlüssel bleiben hier.
func (r *Remote) ensureCert(ctx context.Context) error {
	if c, err := tls.LoadX509KeyPair(r.tlsFile("cert.pem"), r.tlsFile("key.pem")); err == nil &&
		time.Until(c.Leaf.NotAfter) > renewBefore && c.Leaf.VerifyHostname(r.Domain()) == nil {
		return nil
	}
	if err := os.MkdirAll(r.opts.TLSDir, 0o700); err != nil {
		return err
	}
	accKey, err := loadECKey(r.tlsFile("account.key"))
	if err != nil {
		return err
	}
	client := &acme.Client{Key: accKey, DirectoryURL: r.opts.ACMEURL}
	if _, err := client.Register(ctx, &acme.Account{}, acme.AcceptTOS); err != nil && !errors.Is(err, acme.ErrAccountAlreadyExists) {
		return fmt.Errorf("ACME-Konto: %w", err)
	}
	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(r.Domain()))
	if err != nil {
		return fmt.Errorf("ACME-Auftrag: %w", err)
	}
	for _, u := range order.AuthzURLs {
		if err := r.authorize(ctx, client, u); err != nil {
			return err
		}
	}
	if order, err = client.WaitOrder(ctx, order.URI); err != nil {
		return fmt.Errorf("ACME-Auftrag: %w", err)
	}
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{DNSNames: []string{r.Domain()}}, certKey)
	if err != nil {
		return err
	}
	der, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil {
		return fmt.Errorf("ACME-Zertifikat: %w", err)
	}
	var chain []byte
	for _, d := range der {
		chain = append(chain, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: d})...)
	}
	keyDER, _ := x509.MarshalPKCS8PrivateKey(certKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	c, err := tls.X509KeyPair(chain, keyPEM)
	if err != nil {
		return err
	}
	if err := os.WriteFile(r.tlsFile("key.pem"), keyPEM, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(r.tlsFile("cert.pem"), chain, 0o600); err != nil {
		return err
	}
	r.certMu.Lock()
	r.cert = &c
	r.certMu.Unlock()
	return nil
}

func (r *Remote) authorize(ctx context.Context, client *acme.Client, u string) error {
	z, err := client.GetAuthorization(ctx, u)
	if err != nil {
		return fmt.Errorf("ACME-Autorisierung: %w", err)
	}
	if z.Status == acme.StatusValid {
		return nil
	}
	var chal *acme.Challenge
	for _, c := range z.Challenges {
		if c.Type == "dns-01" {
			chal = c
		}
	}
	if chal == nil {
		return errors.New("ACME-Server bietet kein DNS-01 an")
	}
	txt, err := client.DNS01ChallengeRecord(chal.Token)
	if err != nil {
		return err
	}
	var ok struct{}
	if err := r.relayCall(ctx, "/v1/acme", Sign("acme", r.key, txt), &ok); err != nil {
		return err
	}
	defer r.relayCall(context.WithoutCancel(ctx), "/v1/acme", Sign("acme", r.key, ""), &ok) // TXT wieder löschen
	if _, err := client.Accept(ctx, chal); err != nil {
		return fmt.Errorf("ACME-Prüfung: %w", err)
	}
	if _, err := client.WaitAuthorization(ctx, z.URI); err != nil {
		return fmt.Errorf("ACME-Prüfung: %w", err)
	}
	return nil
}

func loadECKey(path string) (crypto.Signer, error) {
	if b, err := os.ReadFile(path); err == nil {
		p, _ := pem.Decode(b)
		if p == nil {
			return nil, errors.New(path + ": beschädigt")
		}
		k, err := x509.ParsePKCS8PrivateKey(p.Bytes)
		if err != nil {
			return nil, err
		}
		s, ok := k.(crypto.Signer)
		if !ok {
			return nil, errors.New(path + ": kein Signaturschlüssel")
		}
		return s, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, _ := x509.MarshalPKCS8PrivateKey(k)
	return k, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600)
}

func selfSigned(name string) (*tls.Certificate, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	tpl := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: name}, DNSNames: []string{name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(1, 0, 0),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &k.PublicKey, k)
	if err != nil {
		return nil, err
	}
	leaf, _ := x509.ParseCertificate(der)
	return &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: k, Leaf: leaf}, nil
}
