# Tailscale/NetBird und Live-TV

Beides ist bewusst leicht gehalten: keine neue Abhängigkeit, kein Zugriff auf die VPN-Daemons, kein DVR.

## VPN: Tailscale oder NetBird (`internal/vpn`)

Beide Dienste hängen den Rechner mit einer IPv4-Adresse aus `100.64.0.0/10` in ein privates Netz, und Flimmer lauscht ohnehin
auf allen Schnittstellen. Deshalb erkennt Flimmer nur die Adresse und zeigt sie an. Erkannt werden Schnittstellen namens
`tailscale*` (Linux, Windows), `wt<N>` (NetBird) und `utun<N>` (macOS, dort ist der Anbieter nicht unterscheidbar). Die
100.64-Adresse eines Mobilfunk-Routers auf `eth0` zählt nicht.

`GET /api/vpn` (alle angemeldeten Nutzer außer Gästen; die Android-App merkt sich die Adresse für unterwegs):

```json
{"addrs":[{"provider":"tailscale","interface":"tailscale0","ip":"100.101.102.103","url":"http://100.101.102.103:8096"}],
 "hint":"Geräte im selben Netz erreichen Flimmer unter http://100.101.102.103:8096. HTTP ist hier unkritisch, weil das VPN den Verkehr verschlüsselt."}
```

Ohne Netz ist `addrs` leer, und `hint` sagt auf Deutsch, was fehlt. Das Android-Manifest erlaubt HTTP (`usesCleartextTraffic`),
die App kommt also mit der 100.x-Adresse zurecht.

Grenzen: Nur Geräte im selben Tailscale-/NetBird-Netz kommen so an den Server. Einladungslinks für Gäste bleiben beim
Fernzugriff über das Relay (`docs/fernzugriff.md`). MagicDNS-Namen kennt Flimmer nicht (dafür bräuchte es
`tailscale status --json`), es gilt die IP.

### Docker

Im Standard-Bridge-Netz sieht der Container die Schnittstelle des Hosts nicht. Zwei Wege:

- `network_mode: host` (in `deploy/compose.yml` vorbereitet). Dann läuft der Tailscale-/NetBird-Daemon auf dem Host.
- Sidecar: Flimmer teilt sich das Netz mit dem VPN-Container. `ports:` wandern dann an den Sidecar.
  *Nicht auf echter Hardware getestet.*

```yaml
services:
  tailscale:
    image: tailscale/tailscale
    environment: { TS_AUTHKEY: "${TS_AUTHKEY}", TS_STATE_DIR: /var/lib/tailscale, TS_USERSPACE: "false" }
    cap_add: [NET_ADMIN]
    devices: ["/dev/net/tun:/dev/net/tun"]
    volumes: ["ts-state:/var/lib/tailscale"]
    ports: ["8096:8096"]
  flimmer:
    network_mode: service:tailscale     # statt ports:
```

`TS_USERSPACE=false` ist wichtig, sonst gibt es keine Schnittstelle. NetBird ist analog (`netbirdio/netbird`, `NB_SETUP_KEY`,
`cap_add: [NET_ADMIN, SYS_ADMIN, SYS_RESOURCE]`).

## Fernseher finden den Server (`internal/discovery/mdns.go`)

Flimmer antwortet per mDNS auf `flimmer.local` (nur A-Eintrag, Port 5353, verträgt sich mit Avahi). Der Starter für LG
und Samsung (`apps/launcher`) fragt zuerst die zuletzt genutzte Adresse ab, dann `flimmer.local` auf 8096 und 8097, dann
alle Adressen im eigenen /24-Netz (IP des Fernsehers über Luna bzw. `tizen.systeminfo`). Erst wenn nichts antwortet, fragt
er nach der Adresse. Im Docker-Bridge-Netz kommt Multicast nicht an, dafür braucht es `network_mode: host`.
*Auf echten Fernsehern ungetestet; die Suche ist mit `node apps/launcher/suche.test.js` nachgestellt.*

## Live-TV und Programm (`internal/livetv`)

Quellen:

| Was | Format |
|---|---|
| Kanäle | M3U-Liste (URL oder Datei), z. B. Tvheadend `/playlist/channels.m3u`, xTeVe, IPTV-Anbieter, oder die `lineup.json` eines HDHomeRun (`http://<ip>/lineup.json`) |
| Programm | XMLTV (URL oder Datei, auch `.xml.gz`), Zuordnung über `tvg-id`, sonst über den Anzeigenamen |

Die Einstellungen liegen als Schlüssel `livetv_source`, `livetv_epg`, `livetv_video` in der Tabelle `settings`. Kanäle und
Programm werden beim Start, danach alle 6 Stunden und bei jeder Änderung neu geladen. Ein Fehler beim Programm lässt die Kanäle
stehen. Von den Sendungen bleiben nur die der nächsten 72 Stunden im Speicher.

**Zugangsdaten:** IPTV-Anbieter legen Benutzer und Passwort meist in die Stream-Adresse. Sie erscheinen nirgends in der API, in
Fehlermeldungen oder im Log; Status und Fehler zeigen nur `Schema://Rechner`. Clients bekommen nie die Quelladresse, nur
Flimmer-HLS. Nur `http`, `https`, `rtsp`, `rtmp`, `udp` und `rtp` gehen an ffmpeg (kein `file:` aus einer fremden Liste).

### Einrichten per Knopf

Im Dashboard unter Live-TV steht „Schnell einrichten“ über den eigenen Adressen:

- **Freie Sender:** 18 öffentlich-rechtliche Sender (ARD, ZDF, dritte Programme …) als Internet-Stream. Die Liste ist
  eingebettet (`internal/livetv/freie-sender.m3u`, Quelle `flimmer:freie-sender`), das Programm kommt von epgshare01
  (ein Dritter). Die Streams sind meist nur in Deutschland abrufbar. Ändert ein Sender seine Adresse, hilft erst ein Update.
- **FRITZ!Box mit Kabel-TV:** Flimmer fragt `http://fritz.box/dvb/m3u/tvhd.m3u` und `192.168.178.1` ab (höchstens 2 s)
  und bietet die Liste nur an, wenn dort wirklich Sender liegen. Programm wie oben, Zuordnung über den Namen („Das Erste HD“
  passt zu „Das Erste“). *Nicht mit echter FRITZ!Box getestet.*

HLS-Quellen mit mehreren Qualitäten spielt Flimmer in der besten ab (`video: "h264"`: höchstens 720p).

### Wiedergabe

Pro Kanal läuft ein ffmpeg, der die Quelle nach HLS remuxt (2-s-Segmente, 6 in der Playlist). Alle Zuschauer eines Kanals
teilen ihn. Er endet 30 s nach dem letzten Abruf. Höchstens 3 Kanäle gleichzeitig (sonst 503).

- `video: "copy"` (Standard): Bild wird kopiert. Schneller Start, kaum Last, aber MPEG-2 (SD-Sender) läuft im Browser nicht.
- `video: "h264"`: `yadif` (nur für als interlaced markierte Bilder) und x264 in Software. Auf einem 2-Kern-NAS nur für einen Kanal.
- Ton: erste Tonspur, AAC Stereo.

### Routen und JSON

| Route | Zugriff | Antwort |
|---|---|---|
| `GET /api/livetv` | Admin | `{source, epg, video, channels, programs, updated, error}` |
| `PUT /api/livetv` | Admin | Body `{source?, epg?, video?}`, `""` löscht, fehlend bleibt; 202, lädt neu |
| `POST /api/livetv/refresh` | Admin | 202 |
| `GET /api/livetv/vorlagen` | Admin | `[{id, source, epg, channels, active}]`, `id` = `frei` oder `fritz`, `channels` 0 = nicht gefunden |
| `GET /api/livetv/channels` | Live-TV-Konto | `[{id, number?, name, logo?, group?, now:{start,stop,title,desc?}\|null, next:{…}\|null}]` |
| `GET /api/livetv/guide?hours=6&channel=<id>` | Live-TV-Konto | `{from, to, channels:[{id, programs:[{start,stop,title,desc?}]}]}`, `hours` 1–48; nur Kanäle mit Sendungen |
| `POST /api/livetv/channels/{id}/play` | Live-TV-Konto | `{url, title, live:true}`; startet den Kanal sofort |
| `GET /api/livetv/channels/{id}/{file}` | Live-TV-Konto | `index.m3u8` (wartet aufs erste Segment, höchstens 20 s) und `s<N>.ts` |

Zeiten sind RFC 3339. Fehler: 403 (Konto darf kein Live-TV), 404 (unbekannter Kanal), 503 (alle Plätze belegt), 502 (Quelle
öffnet nicht, Details im Log).

### Einhängen (`internal/api`, `cmd/server/main.go`)

```go
tv := livetv.New(store.DB, filepath.Join(cacheDir, "livetv"))
tv.Allow = func(r *http.Request) bool { u := UserFrom(r); return u != nil && !s.isGuest(r.Context(), u.ID) }
tv.URL = func(r *http.Request, id string) string {
	return "/api/m/" + auth.MediaToken(s.secret(), UserFrom(r).ID, mediaTTL) + "/livetv/channels/" + id + "/index.m3u8"
}
go tv.Run(ctx)
// Routen: adminOnly(tv.StatusHandler), adminOnly(tv.ConfigHandler), adminOnly(tv.RefreshHandler),
// tv.ChannelsHandler, tv.GuideHandler, tv.PlayHandler, tv.FileHandler; vpn.Handler(port) mit adminOnly.
```

`Allow` fehlt = niemand darf (Gäste sehen nie Live-TV). Damit der Player ohne Header auskommt, muss `authenticate` im
Medien-Token-Zweig zusätzlich den Pfad-Präfix `livetv/` zulassen.

### Bewusst weggelassen

- DVR, Timeshift, Tuner-Verwaltung mit Priorität, Favoriten, HDHomeRun-/SAT>IP-Suche (Adresse von Hand eintragen).
- Hardware-Encoder und Tonspurwahl (siehe `ponytail:`-Kommentar in `stream.go`).
- Programm bleibt im Speicher und wird bei jedem Start neu geladen; bei sehr großen XMLTV-Dateien dauert das einige Sekunden.
- Umschaltzeit auf dem LG ist noch nicht gemessen. Im Test mit lokaler Quelle (GOP 1 s, Rechner unter Last): 2,4 bis 2,7 s bis zur ersten Playlist, mit Kopie wie mit x264.
