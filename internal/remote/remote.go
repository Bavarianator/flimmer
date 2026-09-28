// Package remote macht den Server von außen erreichbar: öffentliche IPv6 erkennen, sonst Portfreigabe per
// UPnP-IGD, PCP oder NAT-PMP (mit Lease-Erneuerung und Entfernen beim Beenden), CGNAT erkennen und beim
// Rendezvous-Dienst (cmd/relay) die Adresse signiert melden. Das Relay prüft die Erreichbarkeit durch einen Rückruf.
package remote

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Status ist das Ergebnis für die Einstellungsseite.
type Status struct {
	Method    string `json:"method"`    // ipv6, upnp, pcp, nat-pmp oder none
	PublicURL string `json:"publicUrl"` // leer, wenn keine öffentliche Adresse gefunden wurde
	Reachable bool   `json:"reachable"` // vom Relay per Rückruf bestätigt
	Hint      string `json:"hint"`      // deutscher, handlungsleitender Hinweis
}

type Options struct {
	Port     int    // lokaler HTTP-Port des Servers
	KeyFile  string // z. B. <data>/remote.key, wird beim ersten Start angelegt
	RelayURL string // z. B. https://relay.flimmer.direct; leer = keine Prüfung von außen
}

type Remote struct {
	opts Options
	key  ed25519.PrivateKey
	gw   func() (string, error) // "ip:5351" für PCP/NAT-PMP; Tests setzen einen Fake ein

	checkMu sync.Mutex // serialisiert Check
	m       *mapping

	mu sync.Mutex
	st Status
}

// mapping ist eine eingerichtete Portfreigabe.
type mapping struct {
	method  string
	extIP   net.IP // WAN-Adresse laut Router
	extPort int
	lease   time.Duration // 0 = dauerhaft
	renew   func(context.Context) error
	remove  func(context.Context) error
}

func New(opts Options) (*Remote, error) {
	key, err := LoadKey(opts.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("Fernzugriff: %w", err)
	}
	gw := func() (string, error) {
		ip, err := gateway()
		if err != nil {
			return "", err
		}
		return net.JoinHostPort(ip.String(), "5351"), nil
	}
	return &Remote{opts: opts, key: key, gw: gw, st: Status{Method: "none", Hint: "Noch nicht geprüft."}}, nil
}

// ID ist die Server-ID beim Relay (und später <id>.flimmer.direct).
func (r *Remote) ID() string { return ServerID(r.key.Public().(ed25519.PublicKey)) }

func (r *Remote) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.st
}

// Run prüft sofort und dann regelmäßig (spätestens zur halben Lease-Zeit), bis ctx endet.
// Danach wird die Portfreigabe im Router wieder entfernt. Nur starten, wenn der Admin Fernzugriff eingeschaltet hat.
func (r *Remote) Run(ctx context.Context) {
	for {
		r.Check(ctx)
		wait := 30 * time.Minute
		r.checkMu.Lock()
		if r.m != nil && r.m.lease > 0 && r.m.lease/2 < wait {
			wait = r.m.lease / 2
		}
		r.checkMu.Unlock()
		select {
		case <-ctx.Done():
			r.checkMu.Lock()
			defer r.checkMu.Unlock()
			if r.m != nil {
				c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				if err := r.m.remove(c); err != nil {
					log.Printf("Fernzugriff: Portfreigabe entfernen: %v", err)
				}
				r.m = nil
			}
			return
		case <-time.After(wait):
		}
	}
}

// Check richtet bei Bedarf die Portfreigabe ein bzw. erneuert sie, meldet die Adresse beim Relay und
// lässt sie dort prüfen. Ohne laufendes Run verfällt eine neue Freigabe mit ihrer Lease.
func (r *Remote) Check(ctx context.Context) Status {
	r.checkMu.Lock()
	defer r.checkMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if r.m != nil {
		if err := r.m.renew(ctx); err != nil {
			log.Printf("Fernzugriff: Freigabe erneuern: %v", err)
			r.m = nil
		}
	}
	var errs []error
	if r.m == nil {
		r.m, errs = r.tryMap(ctx)
	}

	in := input{port: r.opts.Port, v6: publicIPv6(), m: r.m, mapErrs: errs, relay: r.opts.RelayURL != ""}
	for _, u := range in.candidates() {
		ok, observed, err := r.register(ctx, u)
		if err != nil {
			in.relayErr = err
			break
		}
		in.observed = observed
		if ok {
			in.reachableURL = u
			break
		}
	}
	st := in.status()
	r.mu.Lock()
	r.st = st
	r.mu.Unlock()
	return st
}

func (r *Remote) tryMap(ctx context.Context) (*mapping, []error) {
	var errs []error
	sub := func() (context.Context, context.CancelFunc) { return context.WithTimeout(ctx, 5*time.Second) }
	c, cancel := sub()
	m, err := upnpMap(c, r.opts.Port)
	cancel()
	if err == nil {
		return m, nil
	}
	errs = append(errs, err)
	gw, err := r.gw()
	if err != nil {
		return nil, append(errs, err)
	}
	for _, f := range []func(context.Context, string, int) (*mapping, error){pcpMap, pmpMap} {
		c, cancel := sub()
		m, err := f(c, gw, r.opts.Port)
		cancel()
		if err == nil {
			return m, nil
		}
		errs = append(errs, err)
	}
	return nil, errs
}

// register meldet u beim Relay; das Relay ruft u zurück und sagt, ob es ankam und von welcher IP wir kamen.
func (r *Remote) register(ctx context.Context, u string) (reachable bool, observed net.IP, err error) {
	var out struct {
		Reachable bool   `json:"reachable"`
		Observed  string `json:"observed"`
	}
	if err := r.relayCall(ctx, "/v1/register", Sign("register", r.key, u), &out); err != nil {
		return false, nil, err
	}
	return out.Reachable, net.ParseIP(out.Observed), nil
}

// PairCode holt beim Relay einen einmaligen Code, mit dem ein Gerät diesen Server findet.
func (r *Remote) PairCode(ctx context.Context) (code string, expires time.Time, err error) {
	var out struct {
		Code    string    `json:"code"`
		Expires time.Time `json:"expires"`
	}
	err = r.relayCall(ctx, "/v1/pair", Sign("pair", r.key, ""), &out)
	return out.Code, out.Expires, err
}

func (r *Remote) relayCall(ctx context.Context, path string, body Request, out any) error {
	if r.opts.RelayURL == "" {
		return errors.New("kein Rendezvous-Dienst eingestellt")
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimSuffix(r.opts.RelayURL, "/")+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Rendezvous-Dienst nicht erreichbar: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e struct{ Error string }
		json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("Rendezvous-Dienst: %s", e.Error)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// publicIPv6 liefert eine global erreichbare IPv6-Adresse dieses Rechners (keine ULA, kein Link-Local).
// ponytail: nimmt die erste, evtl. eine Privacy-Adresse; die Anmeldung beim Relay alle 30 min fängt den Wechsel ab
func publicIPv6() net.IP {
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if ok && n.IP.To4() == nil && n.IP.IsGlobalUnicast() && !n.IP.IsPrivate() {
			return n.IP
		}
	}
	return nil
}

var cgnatNet = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// input sammelt alles, woraus status() Methode und Hinweis ableitet (reine Funktion, tabellengetestet).
type input struct {
	port         int
	v6           net.IP
	m            *mapping
	mapErrs      []error
	relay        bool
	relayErr     error
	observed     net.IP // öffentliche IP laut Relay
	reachableURL string
}

// candidates: IPv6 zuerst (DS-Lite!), dann die Router-Freigabe.
func (in input) candidates() []string {
	if !in.relay {
		return nil
	}
	var c []string
	if in.v6 != nil {
		c = append(c, "http://"+net.JoinHostPort(in.v6.String(), strconv.Itoa(in.port)))
	}
	if in.m != nil && in.m.extIP != nil && !in.m.extIP.IsUnspecified() {
		c = append(c, "http://"+net.JoinHostPort(in.m.extIP.String(), strconv.Itoa(in.m.extPort)))
	}
	return c
}

func (in input) status() Status {
	p := strconv.Itoa(in.port)
	st := Status{Method: "none"}
	if in.m != nil {
		st.Method = in.m.method
	}
	if in.v6 != nil && (in.m == nil || strings.Contains(in.reachableURL, "[")) {
		st.Method = "ipv6"
	}
	if cs := in.candidates(); len(cs) > 0 {
		st.PublicURL = cs[0]
	}
	if in.reachableURL != "" {
		st.PublicURL, st.Reachable = in.reachableURL, true
	}
	var wan net.IP
	if in.m != nil {
		wan = in.m.extIP.To4()
	}
	switch {
	case st.Reachable && st.Method == "ipv6":
		st.Hint = "Von außen erreichbar über IPv6. Aus Netzen ohne IPv6 (manche Mobilfunk- und Hotel-WLANs) klappt es nicht."
	case st.Reachable:
		st.Hint = "Von außen erreichbar. Port " + strconv.Itoa(in.m.extPort) + " ist per " + strings.ToUpper(in.m.method) + " im Router freigegeben."
	case wan != nil && cgnatNet.Contains(wan), wan != nil && in.observed != nil && in.observed.To4() != nil && !in.observed.Equal(wan) && !wan.IsPrivate():
		st.Hint = "Dein Anschluss hat keine eigene öffentliche IPv4-Adresse (Carrier-Grade-NAT, z. B. DS-Lite). Portfreigaben im Router helfen deshalb nicht. " +
			"Möglichkeiten: IPv6 nutzen (siehe unten), beim Anbieter eine öffentliche IPv4 anfragen oder Tailscale bzw. einen WireGuard-VPS als Brücke einrichten (docs/fernzugriff.md)."
	case wan != nil && wan.IsPrivate():
		st.Hint = "Dein Router hängt hinter einem weiteren Router (seine Internet-Adresse " + wan.String() + " ist privat). " +
			"Gib Port " + p + " auch im vorderen Router frei oder schalte diesen in den Bridge-Modus."
	case in.v6 != nil && in.m == nil && in.relay && in.relayErr == nil:
		st.Hint = "Dieser Rechner hat eine öffentliche IPv6-Adresse, aber der Router lässt keine Verbindungen herein. " +
			"FRITZ!Box: Internet → Freigaben → Gerät hinzufügen → „IPv6-Portfreigabe“ für Port " + p + " aktivieren. " +
			"Zusätzlich UPnP bzw. „Selbstständige Portfreigaben“ erlauben, damit auch IPv4 klappt."
	case in.m == nil:
		st.Hint = "Der Router erlaubt keine automatische Portfreigabe. Schalte UPnP ein (FRITZ!Box: Internet → Freigaben → Gerät → " +
			"„Selbstständige Portfreigaben für dieses Gerät erlauben“) oder gib Port " + p + " von Hand auf diesen Rechner frei."
		if len(in.mapErrs) > 0 {
			st.Hint += " (Details: " + errors.Join(in.mapErrs...).Error() + ")"
		}
	case !in.relay:
		st.Hint = "Portfreigabe eingerichtet, aber nicht von außen geprüft: Es ist kein Rendezvous-Dienst eingestellt."
	case in.relayErr != nil:
		st.Hint = "Portfreigabe eingerichtet, aber die Prüfung von außen ging nicht: " + in.relayErr.Error()
	default:
		st.Hint = "Die Portfreigabe ist eingerichtet, aber von außen kommt nichts an. Prüfe die Firewall dieses Rechners für Port " + p + "."
	}
	return st
}

// StatusHandler: GET, nur Admin. Liefert den letzten Status als JSON.
func (r *Remote) StatusHandler(w http.ResponseWriter, _ *http.Request) { writeJSON(w, r.Status()) }

// CheckHandler: POST, nur Admin („Erreichbarkeit testen“). Prüft neu (dauert bis zu 30 s).
func (r *Remote) CheckHandler(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, r.Check(req.Context()))
}

// PairHandler: POST, nur Admin. Liefert {code, expires} für die Einladung eines Geräts von außen.
func (r *Remote) PairHandler(w http.ResponseWriter, req *http.Request) {
	code, exp, err := r.PairCode(req.Context())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"code": code, "expires": exp})
}

// PingHandler: GET ?nonce=…, OHNE Anmeldung. Das Relay ruft ihn beim Erreichbarkeitstest auf;
// die signierte Antwort beweist, dass hinter der Adresse wirklich dieser Server steht.
func (r *Remote) PingHandler(w http.ResponseWriter, req *http.Request) {
	nonce := req.URL.Query().Get("nonce")
	if len(nonce) == 0 || len(nonce) > 64 {
		http.Error(w, "nonce fehlt", http.StatusBadRequest)
		return
	}
	writeJSON(w, Sign("ping", r.key, nonce))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
