# Fernzugriff

Flimmer versucht, den Server ohne Router-Gefrickel von außen erreichbar zu machen. Video läuft immer direkt
zwischen Gerät und Server. Der Rendezvous-Dienst (`cmd/relay`) vermittelt nur Adressen.

## Was der Server macht (`internal/remote`)

Der Server prüft beim Einschalten und danach alle 30 Minuten (bzw. zur halben Lease-Zeit):

1. **IPv6:** Hat der Rechner eine öffentliche IPv6-Adresse, ist sie der erste Kandidat. Das ist wichtig bei
   DS-Lite, wo es keine eigene IPv4 gibt. Der Router muss eingehende Verbindungen trotzdem erlauben
   (FRITZ!Box: „IPv6-Portfreigabe“).
2. **Portfreigabe im Router**, in dieser Reihenfolge:
   - UPnP-IGD (SSDP-Suche, dann SOAP `AddPortMapping`, Lease 1 h; lässt der Router nur dauerhafte Freigaben zu, wird Lease 0 genutzt),
   - PCP (RFC 6887, `MAP`),
   - NAT-PMP (RFC 6886).
   Die Freigabe wird regelmäßig erneuert und beim Beenden wieder entfernt.
3. **Meldung beim Relay:** Die Kandidaten-URL wird mit dem Server-Schlüssel signiert gemeldet. Das Relay ruft
   sie zurück (`GET /api/remote/ping?nonce=…`), und der Server antwortet signiert. Erst dann gilt der Server als erreichbar.
4. **CGNAT-Erkennung:** Meldet der Router eine WAN-Adresse aus `100.64.0.0/10` oder weicht sie von der IP ab,
   die das Relay sieht, hat der Anschluss keine eigene IPv4. Ist die WAN-Adresse privat, hängt der Router
   hinter einem weiteren Router (doppeltes NAT).

Das Ergebnis ist `Status{method, publicUrl, reachable, hint}`. Der Hinweis sagt auf Deutsch, was zu tun ist.

### HTTPS

Mit `TLSPort` gesetzt, gibt Flimmer nur den HTTPS-Port nach außen frei. HTTP bleibt im Heimnetz, damit
Passwörter nie unverschlüsselt durchs Internet gehen. Der zweite Listener nutzt `rem.TLSConfig()`:

- Solange es kein Zertifikat gibt, antwortet er mit einem selbst signierten. Das reicht für den Rückruf des Relays,
  weil die Identität über den signierten Ping bewiesen wird.
- Ist der Server erreichbar, holt `ensureCert` per ACME DNS-01 ein Zertifikat für `<id>.flimmer.direct`
  (`golang.org/x/crypto/acme`, Let's Encrypt). Unter `<data>/tls/` liegen dann `account.key`, `key.pem` und `cert.pem`.
- Erneuert wird, sobald die Restlaufzeit unter 30 Tage fällt. Die Prüfung läuft bei jedem `Check`, also etwa alle 30 Minuten.
- `publicUrl` wird dann `https://<id>.flimmer.direct:<port>`.

Server-Schlüssel: `<data>/remote.key` (ed25519-Seed, Rechte 0600). Die Server-ID sind die ersten 80 Bit von
SHA-256(öffentlicher Schlüssel), base32-kodiert, also 16 Zeichen, die als DNS-Label taugen.

### Routen (eingehängt in `internal/api`)

| Route | Zugriff | Handler |
|---|---|---|
| `GET /api/remote` | Admin | `StatusHandler`: letzter Status |
| `POST /api/remote/check` | Admin | `CheckHandler`: „Erreichbarkeit testen“, dauert bis zu 30 s |
| `POST /api/remote/pair` | Admin | `PairHandler`: `{code, expires}` für eine Einladung |
| `GET /api/remote/ping` | **ohne Anmeldung** | `PingHandler`: Rückruf des Relays |

`Run(ctx)` läuft nur, wenn der Admin den Fernzugriff eingeschaltet hat. Automatisch Ports im Router zu öffnen
passiert nie ungefragt.

## Wenn nichts davon klappt (CGNAT)

- **IPv6** nutzen: Mit DS-Lite ist das meist der einfachste Weg. Unterwegs braucht dann auch das Gerät IPv6.
- Beim Anbieter eine **öffentliche IPv4** anfragen (oft kostenlos oder gegen kleinen Aufpreis).
- **Tailscale/Headscale:** Server und Geräte kommen in ein privates Netz, und Flimmer ist dann unter der Tailscale-IP erreichbar.
- **WireGuard-VPS als Brücke:** Ein kleiner VPS mit öffentlicher IP leitet den Port per WireGuard an den Server weiter.
- **Cloudflare Tunnel ist kein Standard:** Die Nutzungsbedingungen verbieten Video-Traffic über den Tunnel.

## Rendezvous-Dienst `cmd/relay`

```
go run ./cmd/relay -addr :8098 -db relay.db -zone flimmer.direct [-trusted-proxy 10.0.0.2] [-dev]
```

- **Rate-Limit:** Token-Bucket pro IPv4-Adresse bzw. IPv6-/64, 20 Anfragen am Stück, danach 1 pro Sekunde (sonst 429).
- **`-trusted-proxy`** (IP oder CIDR): Nur Verbindungen von dort dürfen per `X-Forwarded-For` eine andere Absender-IP
  angeben. Es zählt der letzte Eintrag. Ohne das Flag wird der Header ignoriert.
- **Body-Limit:** 8 KiB für jede Anfrage, 4 KiB für signierte JSON-Anfragen und Ping-Antworten.

Das Relay ist eine einzelne Binary und speichert in SQLite. Es sieht kein Video. Alle schreibenden Anfragen
sind `remote.Request`-JSON `{id, pub, data, ts, sig}` mit einer ed25519-Signatur über
`flimmer-v1\n<art>\n<id>\n<data>\n<ts>` (erlaubte Uhrabweichung: ±5 min).

| Route | Signiert | Zweck |
|---|---|---|
| `POST /v1/register` | `register`, data = URL | Adresse melden. Antwort: `{reachable, observed}` |
| `GET /v1/servers/{id}` | – | Adressbuch: `{id, url, reachable, seen}` |
| `POST /v1/pair` | `pair` | Einmal-Code `ABCD-EFGH`, 10 min gültig |
| `GET /v1/pair/{code}` | – | Code einmalig einlösen: `{id, url}` |
| `POST /v1/acme` | `acme`, data = TXT-Wert | `_acme-challenge.<id>.<zone>` setzen (leer = löschen) |

Schutz gegen Missbrauch als Scanner: Gemeldet werden darf nur `http(s)://IP:Port` mit einer öffentlichen IP,
bei IPv4 genau die Absender-IP. Weiterleitungen folgt das Relay nicht. `-dev` hebt die Prüfung für Tests im LAN auf.

Der Rückruf läuft auch über HTTPS mit selbst signiertem Zertifikat (TLS-Prüfung aus). Die Identität beweist die Signatur.

### ACME DNS-01 für `<id>.flimmer.direct` (vorbereitet, noch ohne Domain)

Das Zertifikat und sein Schlüssel entstehen **auf dem Server**. Das Relay setzt nur DNS-Records.

1. Der Server registriert sich und ist erreichbar. Danach setzt das Relay `<id>.flimmer.direct` per `DNS.SetAddr` auf die gemeldete IP.
2. Der Server bestellt bei Let's Encrypt ein Zertifikat für `<id>.flimmer.direct` und bekommt die DNS-01-Challenge.
3. Der Server schickt den Key-Authorization-Digest signiert an `POST /v1/acme`. Das Relay setzt ihn per `DNS.SetTXT`,
   aber nur, wenn der Server registriert und erreichbar ist.
4. Let's Encrypt prüft den TXT-Record, und der Server holt das Zertifikat ab. Danach löscht er den Record wieder mit einem leeren Wert.
5. Die Verlängerung läuft genauso, rund 30 Tage vor Ablauf.

Es fehlt noch eine Implementierung von `DNS` für den Anbieter der Zone (z. B. per RFC 2136 oder eine Anbieter-API).
`SetTXT` darf erst zurückkehren, wenn die autoritativen Nameserver den Record ausliefern. Ohne DNS-Anbieter
antwortet `/v1/acme` mit 501.

Die Tests (`go test ./internal/remote -run TestACME`) laufen gegen einen Fake-ACME-Server. Gegen
[Pebble](https://github.com/letsencrypt/pebble), den Test-CA von Let's Encrypt, geht es so:

```
# Pebble mit eigenem DNS-Server für die Challenges (pebble-challtestsrv)
docker run -d --name challtest -p 8055:8055 -p 8053:8053/udp ghcr.io/letsencrypt/pebble-challtestsrv -defaultIPv4 127.0.0.1
docker run -d --name pebble --network host -e PEBBLE_VA_NOSLEEP=1 ghcr.io/letsencrypt/pebble -dnsserver 127.0.0.1:8053
# Relay mit einer DNS-Implementierung, die challtestsrv befüllt (POST :8055/set-txt {"host","value"}),
# Server mit ACMEURL https://localhost:14000/dir; Pebbles CA-Zertifikat muss dem Server vertraut sein
# (SSL_CERT_FILE=pebble.minica.pem).
```
