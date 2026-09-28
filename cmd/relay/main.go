// Command relay ist der Rendezvous-Dienst für Flimmer-Fernzugriff, ohne Video-Traffic:
// Adressbuch (signiert gemeldete Server-Adressen), Pairing-Codes, Erreichbarkeitstest per Rückruf
// und – sobald ein DNS-Anbieter angebunden ist – ACME DNS-01 für <id>.flimmer.direct. Ablauf: docs/fernzugriff.md.
package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/Bavarianator/flimmer/internal/ratelimit"
	"github.com/Bavarianator/flimmer/internal/remote"
	_ "github.com/ncruces/go-sqlite3/driver"
)

func main() {
	addr := flag.String("addr", ":8098", "Adresse, auf der das Relay lauscht")
	dbPath := flag.String("db", "relay.db", "SQLite-Datei")
	zone := flag.String("zone", "flimmer.direct", "DNS-Zone für <id>.<zone>")
	dev := flag.Bool("dev", false, "Rückrufe auch an private Adressen erlauben (nur zum Testen im LAN)")
	proxy := flag.String("trusted-proxy", "", "IP oder Netz (CIDR) des Reverse-Proxys; nur von dort wird X-Forwarded-For ausgewertet")
	flag.Parse()

	var trusted netip.Prefix
	if *proxy != "" {
		var err error
		if trusted, err = netip.ParsePrefix(*proxy); err != nil {
			a, err2 := netip.ParseAddr(*proxy)
			if err2 != nil {
				log.Fatalf("-trusted-proxy: %v", err)
			}
			trusted = netip.PrefixFrom(a, a.BitLen())
		}
	}

	db, err := openDB(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	rl := &relay{db: db, zone: *zone, allowPrivate: *dev, trusted: trusted}
	log.Printf("Relay lauscht auf %s", *addr)
	srv := &http.Server{Addr: *addr, Handler: rl.routes(), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

// DNS setzt Records in der Zone. Noch ohne Implementierung; ohne Anbieter antwortet /v1/acme mit 501.
type DNS interface {
	// SetTXT setzt (value != "") oder löscht (value == "") den TXT-Record fqdn.
	SetTXT(ctx context.Context, fqdn, value string) error
	// SetAddr zeigt fqdn (A bzw. AAAA) auf ip.
	SetAddr(ctx context.Context, fqdn string, ip net.IP) error
}

type relay struct {
	db           *sql.DB
	zone         string
	dns          DNS
	allowPrivate bool
	trusted      netip.Prefix // Reverse-Proxy, dem X-Forwarded-For geglaubt wird
	limiter      ratelimit.Limiter
	now          func() time.Time
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "file:"+url.PathEscape(path)+"?_pragma=busy_timeout(10000)&_pragma=foreign_keys(1)&_pragma=journal_mode(wal)")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS servers(id TEXT PRIMARY KEY, url TEXT NOT NULL, reachable INTEGER NOT NULL, seen INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS pairings(code TEXT PRIMARY KEY, id TEXT NOT NULL REFERENCES servers(id), expires INTEGER NOT NULL);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (rl *relay) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/register", rl.register)
	mux.HandleFunc("GET /v1/servers/{id}", rl.lookup)
	mux.HandleFunc("POST /v1/pair", rl.pair)
	mux.HandleFunc("GET /v1/pair/{code}", rl.resolve)
	mux.HandleFunc("POST /v1/acme", rl.acme)
	// 20 Anfragen am Stück, dann 1 pro Sekunde – bremst auch das Raten von Pairing-Codes.
	return http.MaxBytesHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.limiter.Allow(rl.clientIP(r), rl.clock()) {
			w.Header().Set("Retry-After", "10")
			fail(w, http.StatusTooManyRequests, "zu viele Anfragen, bitte kurz warten")
			return
		}
		mux.ServeHTTP(w, r)
	}), 8<<10)
}

func (rl *relay) clock() time.Time {
	if rl.now != nil {
		return rl.now()
	}
	return time.Now()
}

// signed liest und prüft eine signierte Anfrage; bei Fehler ist die Antwort schon geschrieben.
func (rl *relay) signed(w http.ResponseWriter, r *http.Request, kind string) (remote.Request, bool) {
	var req remote.Request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "ungültige Anfrage")
		return req, false
	}
	if err := req.Verify(kind, rl.clock()); err != nil {
		fail(w, http.StatusUnauthorized, err.Error())
		return req, false
	}
	return req, true
}

// register: Server meldet seine öffentliche URL; das Relay ruft sie zurück und merkt sich das Ergebnis.
func (rl *relay) register(w http.ResponseWriter, r *http.Request) {
	req, ok := rl.signed(w, r, "register")
	if !ok {
		return
	}
	observed := rl.clientIP(r)
	target, err := rl.checkTarget(req.Data, observed)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	reachable := rl.callback(r.Context(), req.Data, req.ID)
	_, err = rl.db.ExecContext(r.Context(), `INSERT INTO servers(id, url, reachable, seen) VALUES(?,?,?,?)
ON CONFLICT(id) DO UPDATE SET url=excluded.url, reachable=excluded.reachable, seen=excluded.seen`,
		req.ID, req.Data, reachable, rl.clock().Unix())
	if err != nil {
		fail(w, http.StatusInternalServerError, "Speichern fehlgeschlagen")
		return
	}
	if reachable && rl.dns != nil {
		if err := rl.dns.SetAddr(r.Context(), req.ID+"."+rl.zone, target); err != nil {
			log.Printf("DNS %s: %v", req.ID, err)
		}
	}
	writeJSON(w, map[string]any{"reachable": reachable, "observed": observed.String()})
}

// checkTarget verhindert, dass das Relay als Werkzeug gegen fremde Netze dient: nur http(s) auf eine
// öffentliche IP-Literal-Adresse, bei IPv4 genau die, von der die Anfrage kam.
func (rl *relay) checkTarget(raw string, observed net.IP) (net.IP, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Port() == "" || u.Path != "" || u.User != nil {
		return nil, errors.New("URL muss die Form http://IP:Port haben")
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil {
		return nil, errors.New("URL muss eine IP-Adresse enthalten")
	}
	if rl.allowPrivate {
		return ip, nil
	}
	if !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return nil, errors.New("Adresse ist nicht öffentlich")
	}
	if ip.To4() != nil && observed.To4() != nil && !ip.Equal(observed) {
		return nil, errors.New("IPv4-Adresse passt nicht zum Absender")
	}
	return ip, nil
}

var callbackClient = &http.Client{
	Timeout: 5 * time.Second,
	// Das Zertifikat ist anfangs selbst signiert und auf den Namen ausgestellt, nicht auf die IP.
	// Die Identität beweist die signierte Ping-Antwort, nicht TLS.
	Transport:     &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// callback ruft <url>/api/remote/ping auf und prüft, dass dort derselbe Server signiert antwortet.
func (rl *relay) callback(ctx context.Context, base, id string) bool {
	nonce := rand.Text()
	req, err := http.NewRequestWithContext(ctx, "GET", base+"/api/remote/ping?nonce="+nonce, nil)
	if err != nil {
		return false
	}
	resp, err := callbackClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var pong remote.Request
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&pong) != nil {
		return false
	}
	return pong.ID == id && pong.Data == nonce && pong.Verify("ping", rl.clock()) == nil
}

func (rl *relay) lookup(w http.ResponseWriter, r *http.Request) {
	var u string
	var reachable bool
	var seen int64
	err := rl.db.QueryRowContext(r.Context(), "SELECT url, reachable, seen FROM servers WHERE id=?", r.PathValue("id")).Scan(&u, &reachable, &seen)
	if err != nil {
		fail(w, http.StatusNotFound, "Server unbekannt")
		return
	}
	writeJSON(w, map[string]any{"id": r.PathValue("id"), "url": u, "reachable": reachable, "seen": time.Unix(seen, 0)})
}

const pairAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // ohne 0/O und 1/I

// pair gibt einem registrierten Server einen einmaligen Code (10 Minuten gültig).
func (rl *relay) pair(w http.ResponseWriter, r *http.Request) {
	req, ok := rl.signed(w, r, "pair")
	if !ok {
		return
	}
	b := make([]byte, 8)
	rand.Read(b)
	for i := range b {
		b[i] = pairAlphabet[b[i]%32]
	}
	code := string(b)
	exp := rl.clock().Add(10 * time.Minute)
	rl.db.ExecContext(r.Context(), "DELETE FROM pairings WHERE expires < ?", rl.clock().Unix())
	if _, err := rl.db.ExecContext(r.Context(), "INSERT INTO pairings(code, id, expires) VALUES(?,?,?)", code, req.ID, exp.Unix()); err != nil {
		fail(w, http.StatusConflict, "Server erst registrieren")
		return
	}
	writeJSON(w, map[string]any{"code": code[:4] + "-" + code[4:], "expires": exp})
}

// resolve löst einen Code einmalig in Server-ID und URL auf.
func (rl *relay) resolve(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.ReplaceAll(r.PathValue("code"), "-", ""))
	var id, u string
	err := rl.db.QueryRowContext(r.Context(), "DELETE FROM pairings WHERE code=? AND expires >= ? RETURNING id", code, rl.clock().Unix()).Scan(&id)
	if err == nil {
		err = rl.db.QueryRowContext(r.Context(), "SELECT url FROM servers WHERE id=?", id).Scan(&u)
	}
	if err != nil {
		fail(w, http.StatusNotFound, "Code unbekannt oder abgelaufen")
		return
	}
	writeJSON(w, map[string]any{"id": id, "url": u})
}

// acme setzt für die ACME-DNS-01-Prüfung den TXT-Record _acme-challenge.<id>.<zone>. Das Zertifikat und
// sein Schlüssel entstehen auf dem Server; das Relay sieht nur den TXT-Wert. Leerer Wert löscht den Record.
func (rl *relay) acme(w http.ResponseWriter, r *http.Request) {
	req, ok := rl.signed(w, r, "acme")
	if !ok {
		return
	}
	var reachable bool
	if rl.db.QueryRowContext(r.Context(), "SELECT reachable FROM servers WHERE id=?", req.ID).Scan(&reachable) != nil || !reachable {
		fail(w, http.StatusConflict, "Server muss erst registriert und erreichbar sein")
		return
	}
	if rl.dns == nil {
		fail(w, http.StatusNotImplemented, "kein DNS-Anbieter konfiguriert")
		return
	}
	if len(req.Data) > 128 {
		fail(w, http.StatusBadRequest, "TXT-Wert zu lang")
		return
	}
	if err := rl.dns.SetTXT(r.Context(), "_acme-challenge."+req.ID+"."+rl.zone, req.Data); err != nil {
		fail(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// clientIP ist die Absender-IP; X-Forwarded-For zählt nur, wenn die Verbindung vom vertrauten Proxy kommt
// (dann gilt der letzte Eintrag, den der Proxy selbst angehängt hat).
func (rl *relay) clientIP(r *http.Request) net.IP {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	if a, ok := netip.AddrFromSlice(ip); ok && rl.trusted.IsValid() && rl.trusted.Contains(a.Unmap()) {
		if xff := r.Header.Values("X-Forwarded-For"); len(xff) > 0 {
			parts := strings.Split(xff[len(xff)-1], ",")
			if fwd := net.ParseIP(strings.TrimSpace(parts[len(parts)-1])); fwd != nil {
				return fwd
			}
		}
	}
	return ip
}

func fail(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
