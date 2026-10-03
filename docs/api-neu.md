# Neue Server-Routen (Umbau nach Jellyfin-Vorbild)

Stand 29.09.2026. Vertrag: `docs/umbau-jellyfin.md`, Abschnitt „Neue Server-Endpunkte“. Hier steht, was der Server
tatsächlich liefert, mit kurzen Beispielen. Alle Routen brauchen eine Anmeldung (Session, Bearer-Token) und liefern JSON.
**(A)** heißt nur für Admins (sonst 403). Gäste (Einladungen) sehen nur freigegebene Titel, wie bei `/api/library`.

Fehler: 400 bei ungültiger Eingabe (Text auf Deutsch), 404 bei Unbekanntem oder nicht Freigegebenem, 409 bei Konflikt.
Unbekannte Pfade unter `/api/` liefern 404 (nicht mehr `index.html`).

## Listen-Schlüssel

Favoriten, Sammlungen und Wiedergabelisten speichern **Schlüssel**: eine Titel-ID oder `serie:<Name>`. `<Name>` ist der
Serienname wie im Feld `series` der Titel (aus dem Dateinamen), nicht der Anzeigename aus den Metadaten. In URLs wird er
kodiert (`encodeURIComponent`).

## Favoriten (pro Profil)

```
GET    /api/favorites        → ["4b9b5d8402a0", "serie:Breaking Bad"]   neueste zuerst
PUT    /api/favorites/{key}  → 204   404, wenn der Titel/die Serie nicht existiert oder nicht sichtbar ist
DELETE /api/favorites/{key}  → 204   auch wenn er nicht gemerkt war
```

`GET` lässt Einträge weg, die das Profil gerade nicht sieht (Datei entfernt, Gast). Gespeichert bleiben sie trotzdem.

## Startseite

`POST /api/home` (Body: Geräteprofil wie bisher) liefert statt `recent` zwei Reihen:

```json
[{"id":"continue","title":"Weiterschauen","items":[…]},
 {"id":"nextup","title":"Als Nächstes","items":[…]},
 {"id":"recent-movies","title":"Kürzlich hinzugefügt in Filme","items":[…]},
 {"id":"recent-series","title":"Kürzlich hinzugefügt in Serien","items":[…]}]
```

`recent-series` enthält je Serie die neueste Folge. Leere Reihen fehlen. Je Reihe höchstens 20 Einträge.

## Details

GET-Routen haben keinen Body. Das Geräteprofil für Ampel und `method` kommt deshalb aus `?device=<id>` (gespeichert über
`PUT /api/devices/{device}/profile`), ohne Parameter gilt ein leeres Profil.

### `GET /api/items/{id}?device=`

Ein `libraryItem` wie in `/api/library` und dazu:

```json
{"id":"4b9b5d8402a0","title":"Heat","year":1995,"size":1,"duration":600,"light":"green","method":"direct-play",
 "meta":{…},"poster":"/api/images/…","backdrop":"…","color":"#223344","progress":0,"watched":false,
 "tagline":"A Los Angeles Crime Saga","studios":["Warner Bros."],"countries":["United States of America"],
 "people":[{"name":"Michael Mann","role":"Director","kind":"director"},
           {"name":"Al Pacino","role":"Vincent Hanna","kind":"actor","image":"/api/images/person/Al%20Pacino/profile?w=300&v=f967c4d36168"}],
 "tags":["heist"],"path":"/m/Heat (1995).mkv","container":"mkv","bitrate":1000000,
 "video":{"codec":"h264","width":1920,"height":1080,"hdr":"hdr10","fps":23.976},
 "audio":[{"index":1,"codec":"aac","lang":"ger","channels":6,"default":true}],
 "subs":[{"index":2,"codec":"subrip","lang":"ger","forced":true},
         {"index":0,"codec":"srt","lang":"eng","title":"Englisch","external":true}],
 "chapters":[{"start":0,"name":"Anfang"},{"start":312.5,"name":"Banküberfall"}],
 "locked":["title"]}
```

- `path` nur für Admins.
- `container` ist die Kurzform (`mkv`, `mp4`, `ts` …).
- `people`: Regie, Buch, Musik und Produktion zuerst, danach bis zu 20 Darsteller in TMDB-Reihenfolge. `kind` ist
  `actor|director|writer|composer|producer` (**`composer` neu gegenüber dem Vertrag**, für „Musik“). Bei Folgen kommen
  Personen, Sender und Länder von der Serie.
- **Ohne TMDB-Schlüssel** sind die Felder genauso gefüllt: Filme aus Wikidata (Genres P136, Regie P57, Drehbuch P58,
  Musik P86, Produktion P162, Besetzung P161 mit Rolle aus P453/P4633, Studio P272, Land P495, Personenbilder aus Commons
  P18) mit bis zu drei Absätzen der deutschen Wikipedia-Einleitung; Serien aus TVmaze (Genres, Sender bzw.
  Streamingdienst, Besetzung mit Fotos, Status und Endjahr). Pro Film sind das sechs kleine Anfragen, höchstens eine
  Minute. Schon erfasste Titel bekommen die Felder mit „Metadaten aktualisieren“.
- Externe Untertitel (Dateien neben dem Video) haben `external: true`. Ihr `index` zählt in der Liste der externen Dateien,
  nicht in den Spuren der Videodatei.
- `fps` und `chapters` liest der Server beim ersten Abruf per ffprobe aus dem Dateikopf (danach aus dem Speicher). Ohne
  ffmpeg fehlen sie.
- `video.crop` (Bildanpassung, siehe `docs/umbau-jellyfin.md`): sichtbares Bild ohne eingebrannte schwarze Balken, in
  Anteilen des ganzen Bildes, z. B. `{"x":0,"y":0.1222,"w":1,"h":0.7556}` für 2,39:1 in 1920×1080. Fehlt es, gibt es
  keine Balken oder die Erkennung ist noch nicht gelaufen: Der erste Abruf eines Titels startet sie im Hintergrund
  (ffmpeg `cropdetect` bei 10, 50 und 80 % der Laufzeit, je 2 s, Vereinigung der Rechtecke, mit `nice`, höchstens eine
  gleichzeitig), ein späterer Abruf liefert das Ergebnis. Kein `crop`, wenn das Rechteck in beiden Richtungen ≥ 98 %
  abdeckt oder weniger als 50 % Fläche hat (dunkle Szene). Das Ergebnis, auch „keine Balken“, bleibt in der Datenbank
  (Tabelle `crops`), bis die Datei neu geprobt wird.
- `meta` enthält in Listen (`/api/library`, `/api/home`, Suche) keine `people`, damit die Antworten klein bleiben.
- `chapters[].image`: Standbild kurz nach Kapitelbeginn (`/api/images/{id}/frame?t=…&w=300`), siehe „Standbilder“.
- `backdrop` (auch in `/api/library`): Filme ohne Hintergrundbild bekommen `/api/images/{id}/backdrop?w=1280`, ein
  Standbild aus dem Video. Der Server zieht dafür vier Kandidaten bei 15, 20, 25 und 30 % der Laufzeit und nimmt das
  hellste (keine dunkle Szene im Kopf der Seite).

### `GET /api/series/{name}`

```json
{"name":"Breaking Bad","title":"Breaking Bad","overview":"Ein Chemielehrer …","year":2008,"endYear":2013,"status":"Ended",
 "genres":["Drama"],"studios":["AMC"],"people":[{"name":"Bryan Cranston","role":"Walter White","kind":"actor","image":"…"}],
 "age":16,"rating":8.9,"poster":"/api/images/…/poster?w=300&v=…","backdrop":"/api/images/serie%3ABreaking%20Bad/backdrop?w=1280&v=…",
 "color":"#1b2a1f"}
```

`name` ist der Schlüssel aus dem Pfad. **Neu gegenüber dem Vertrag:** `title` (Anzeigename aus den Metadaten), nur wenn er
vom Schlüssel abweicht. `endYear` nur bei beendeten Serien. Die Folgen kommen weiter aus `/api/library`.

### `GET /api/people/{name}?device=`

```json
{"name":"Al Pacino","image":"/api/images/person/Al%20Pacino/profile?w=300&v=…","bio":"Alfredo James Pacino …",
 "items":[…libraryItem…],"roles":[{"id":"4b9b5d8402a0","role":"Vincent Hanna"}]}
```

`items`: Filme und je Serie die erste Folge (sortiert wie die Bibliothek). `roles[].id` ist die ID aus `items`, bei
Serien also die der ersten Folge. `bio` kommt von TMDB (deutsch, sonst englisch), nur mit TMDB-Schlüssel. 404, wenn
niemand dieses Namens in sichtbaren Titeln vorkommt.

### Personenbilder

`GET /api/images/person/{name}/profile?w=300&v=…` liefert JPEG über den Bild-Cache (wie Poster, ohne Anmeldung, weil TVs
bei `<img>` keinen Header schicken). Die URL steht fertig in `people[].image`.

### Standbilder

`GET /api/images/{id}/frame?t=<Sekunden>&w=<Breite>` liefert ein JPEG-Einzelbild (ohne Anmeldung, wie Poster).
Beim ersten Abruf zieht ffmpeg das Bild (`-ss` vor `-i`, mit `nice`, höchstens zwei gleichzeitig), danach kommt es aus
dem Bild-Cache (Aufgabe „Bild-Cache aufräumen“ räumt nach 30 Tagen auf). `t` wird auf 5 s gerundet, frühestens 5 s
und vor dem Ende; der Cache pro Film bleibt so begrenzt. 400 bei `t` außerhalb der Laufzeit, 404 ohne Video.

### `PUT /api/items/{id}/meta` (A)

Body: beliebige Teilmenge aus `title, originalTitle, sortTitle, year, overview, tagline, genres, tags, studios, age, rating,
people` und optional `locked`. Antwort wie `GET /api/items/{id}`.

```json
{"title":"Heat (Director's Cut)","age":16,"people":[{"name":"Al Pacino","role":"Vincent Hanna","kind":"actor"}]}
```

- Ohne `locked` werden die geänderten Felder **automatisch gesperrt** (zur bisherigen Sperrliste hinzugefügt). Mit
  `locked` gilt genau diese Liste, `[]` entsperrt alles.
- Gesperrte Felder überstehen „Metadaten aktualisieren“, NFO-Änderungen und „Falsch erkannt?“.
- `null` löscht ein Feld. `age` muss 0, 6, 12, 16 oder 18 sein, `rating` 0–10.
- Personenbilder kommen nie vom Client: Bekannte Namen behalten ihr Bild, neue haben keins. `kind` außer
  `actor|director|writer|composer|producer` wird zu `actor`.
- Titel ohne Metadaten (nur Dateiname) gelten danach als „manuell“ und werden nicht mehr automatisch abgeglichen.

## Sammlungen (sichtbar für alle, bearbeiten nur Admin)

```
GET    /api/collections          → Collection[]
POST   /api/collections (A)      {name, overview?, items?}            → Collection
PUT    /api/collections/{id} (A) {name?, overview?, add?, remove?, items?} → Collection
DELETE /api/collections/{id} (A) → 204
```

```json
[{"id":"a1b2c3d4e5f60718","name":"Lieblinge","overview":"Für Regentage","items":["4b9b5d8402a0","serie:Breaking Bad"]},
 {"id":"auto-2344","name":"Matrix Filmreihe","items":["…1999…","…2003…"],"auto":true}]
```

- Filmreihen (`auto: true`) entstehen aus TMDB `belongs_to_collection` und aus NFO `<set>`, ab zwei sichtbaren Filmen,
  sortiert nach Jahr. IDs: `auto-<TMDB-ID>`, bei NFO-Reihen ohne ID `auto-set-<hash>`. Sie lassen sich nicht bearbeiten
  (PUT/DELETE → 404).
- Unbekannte oder nicht sichtbare Einträge in `items`/`add` → 400. Doppelte zählen einmal. `items` im PUT ersetzt die
  Liste (neue Reihenfolge).
- Gäste sehen nur Sammlungen mit mindestens einem erlaubten Eintrag, und nur diese Einträge.

## Wiedergabelisten (pro Profil)

```
GET    /api/playlists          → [{"id":"…","name":"Abend","items":["…","…"]}]
POST   /api/playlists          {name, items?}                  → Playlist
PUT    /api/playlists/{id}     {name?, add?, remove?, items?}  → Playlist
DELETE /api/playlists/{id}     → 204
```

`items` im PUT setzt die neue Reihenfolge (ersetzt die Liste), danach wirken `add` (hinten anhängen) und `remove`.
Fremde Listen gibt es für niemanden (404). Gäste dürfen eigene Listen anlegen.

## Dashboard (A)

### `GET /api/admin/overview`

```json
{"server":{"name":"NAS","version":"v0.4.0","os":"linux","arch":"amd64","uptime":86400,"started":"2026-09-28T18:00:00+02:00",
           "update":{"version":"v0.5.0","url":"https://github.com/Bavarianator/flimmer/releases/tag/v0.5.0"}},
 "library":{"movies":412,"series":37,"episodes":1840,"sizeBytes":8123456789012,"lastScan":"2026-09-29T17:45:00+02:00"},
 "disks":[{"path":"/srv/media","free":1200000000000,"total":4000000000000}],
 "sessions":[…Session…],"activity":[…letzte 20…]}
```

`update` nur, wenn es eine neuere Version gibt. `disks`: jeder Medienordner und der Cache-Ordner.

### `GET /api/sessions`

```json
[{"id":"anna|4b9b5d8402a0","user":"anna","userColor":120,"device":"Wohnzimmer-TV","client":"LG webOS",
  "itemId":"4b9b5d8402a0","title":"Heat","position":1204.5,"duration":10200,"paused":false,
  "method":"transcode","light":"yellow","reason":"Codec hevc10 nicht unterstützt","since":"2026-09-29T20:01:00+02:00"}]
```

Eine Sitzung lebt, solange in den letzten 60 s ein `POST /api/items/{id}/progress` kam (nur im Speicher).
**Neu gegenüber dem Vertrag:** Der progress-Body nimmt optional `"paused": true` an. Player sollen auch pausiert
weiter Herzschläge schicken, sonst verschwindet die Sitzung nach 60 s. `device`, `method`, `light` und `reason` stammen
vom letzten `play` dieses Titels (nach einem Neustart des Servers bis zum nächsten `play` leer). `client` kommt aus
dem User-Agent.

### `GET /api/activity?limit=50`

```json
[{"time":"2026-09-29T20:01:00+02:00","kind":"play","user":"anna","text":"Spielt „Heat“"},
 {"time":"2026-09-29T19:00:00+02:00","kind":"scan","text":"Bibliothek gescannt: 2289 Titel"}]
```

Neueste zuerst, `limit` höchstens 500. Ringpuffer im Speicher (leer nach Neustart). Arten: `login` (Anmeldung,
TV-Kopplung), `play`, `scan`, `error` (Scan/Aufgabe gescheitert), `invite` (erstellt, zurückgenommen, eingelöst),
`backup` (erstellt, heruntergeladen, eingespielt), `task`, `user` (angelegt, geändert, gelöscht, Gerät abgemeldet).

### Geräte

```
GET    /api/devices       → [{"id":"9f86d081884c7d65","userId":"anna","user":"anna","name":"Pixel 8","client":"Android-App",
                              "ip":"192.168.1.40","lastSeen":"2026-09-29T19:00:00+02:00","current":true}]
DELETE /api/devices/{id}  → 204   meldet das Gerät ab (Token ungültig)
```

Ein Gerät ist eine angemeldete Session. `name` ist der Gerätename aus Anmeldung bzw. Kopplung, `id` sind die ersten
16 Zeichen des Token-Hashes (nie das Token). `client` und `ip` gibt es erst für Sessions ab diesem Stand. `lastSeen`
wird höchstens stündlich aktualisiert.

### Aufgaben

```
GET  /api/tasks          → Task[]
POST /api/tasks/{id}/run → 202 (läuft im Hintergrund), 409 wenn sie schon läuft, 404 unbekannt
```

```json
[{"id":"meta","name":"Metadaten aktualisieren","group":"Bibliothek","description":"…","lastRun":"2026-09-29T03:00:00+02:00",
  "lastResult":"ok","duration":412.3,"running":true,"progress":0.42}]
```

| id | Name | Gruppe | `next` |
|---|---|---|---|
| `scan` | Bibliothek scannen | Bibliothek | – |
| `meta` | Metadaten aktualisieren | Bibliothek | – (nur von Hand) |
| `optimize` | Nachts vorbereiten | Wiedergabe | Beginn des Nachtfensters |
| `db` | Datenbank pflegen | Wartung | 7 Tage nach der letzten Prüfung |
| `backup` | Sicherung erstellen | Wartung | 3 Uhr |
| `images` | Bild-Cache aufräumen | Wartung | – (nur von Hand) |
| `livetv` | Live-TV-Programm laden | Live-TV | 6 h nach dem letzten Laden |
| `mediathek` | Mediathek-Abos prüfen | Bibliothek | 6 h nach der letzten Prüfung |

Der letzte Lauf je Aufgabe steht in der Datenbank. Läufe, die ohne die Aufgabe passierten (Scan-Schleife, Nacht-Backup,
wöchentliche Prüfung, Live-TV alle 6 h), zählen auch. `lastError` nur bei `lastResult: "error"`, `progress` (0..1) nur,
wenn die Aufgabe ihn kennt (Metadaten, Vorbereiten). „Metadaten aktualisieren“ ist nötig, damit schon erfasste Titel
Personen, Studios, Leitsatz, Länder und Filmreihen bekommen. „Nachts vorbereiten“ läuft von Hand auch außerhalb des
Fensters und endet, sobald jemand etwas abspielt.

### `GET /api/logs?level=info&limit=200`

```json
[{"time":"2026-09-29T20:01:00.123+02:00","level":"error","msg":"Aufgabe fehlgeschlagen","attrs":{"aufgabe":"Sicherung erstellen","fehler":"…"}},
 {"time":"2026-09-29T20:00:59+02:00","level":"info","msg":"play \"Heat\" für anna: transcode ([…])"}]
```

Neueste zuerst, `level` ist die Mindeststufe (`debug|info|warn|error`, Standard `info`), `limit` höchstens 1000.
Alles aus `log.Printf` hat die Stufe `info`; nur neuer Code (`slog.Warn/Error`) setzt andere Stufen.

### VPN und Live-TV

```
GET /api/vpn  → vpn.Status (angemeldet, Gäste 403), siehe docs/livetv-vpn.md
/api/livetv, /api/livetv/refresh (A); /api/livetv/channels, /guide, /channels/{id}/play, /channels/{id}/{file}
```

Pfade und JSON wie in `docs/livetv-vpn.md`. Gäste bekommen 403. Die abspielbare URL aus `play` trägt ein Medien-Token im
Pfad (`/api/m/<token>/livetv/channels/<id>/index.m3u8`), der Player braucht also keinen Header.

### Statistik: `GET /api/admin/stats?tage=7|30|90|365` (A)

```json
{"wiedergabe": {"sekunden": 5400, "proTag": [{"tag":"2026-10-03","sekunden":5400}], "proStunde": [0, …, 5400, …],
                "nutzer": [{"name":"Anna","farbe":220,"sekunden":5400,"titel":2}],
                "titel": [{"id":"75c3…","titel":"Heat","serie":"","sekunden":5400,"nutzer":1}],
                "methoden": [{"name":"direct","sekunden":5000},{"name":"transcode","sekunden":400}],
                "clients": [{"name":"Chrome","sekunden":5400}]},
 "bibliothek": {"proMonat": [{"monat":"2026-10","anzahl":3,"bytes":4500000000}], "aufloesung": [{"name":"4K","anzahl":1,"bytes":…}, …],
                "codecs": [{"name":"h264","anzahl":120}], "hdr": 4, "titel": 300, "gesehen": 80,
                "groesste": [{"id":"…","titel":"…","serie":"","bytes":…}]},
 "speicher":   {"laufwerke": [{"path":"/media","free":…,"total":…}], "zuwachsMonat": 25000000000, "monateBisVoll": 9.4},
 "downloads":  {"quellen": [{"quelle":"mediathek","anzahl":3,"bytes":…,"fehler":0}], "letzte": [Download, …]}}
```

Die Sehzeit zählen die `progress`-Herzschläge: Gezählt wird die Zeit seit dem vorigen Herzschlag, nicht nach einer Pause
und nicht nach Lücken über 30 s (Tabelle `watch`, pro Profil, Titel und Stunde). `proTag` hat genau `tage` Einträge, `proStunde`
24, beides in Ortszeit des Servers. `methoden[].name` ist `direct|remux|transcode` oder leer, wenn `play` fehlte. `proMonat`
umfasst 12 Monate nach Dateidatum. `zuwachsMonat` ist der Schnitt der letzten 90 Tage, `monateBisVoll` fehlt ohne Zuwachs.
`aufloesung` zählt nach Breite: ab 3200 = 4K, ab 1800 = 1080p, ab 1200 = 720p, sonst SD.

### Mediathek (A)

```
GET    /api/mediathek/suche?q=&sender=&min=&offset= → {"treffer": Treffer[], "gesamt": 1130}   (502, wenn MediathekViewWeb fehlt)
POST   /api/mediathek/laden {"treffer": Treffer}    → Download   (409: schon geladen oder in der Warteschlange)
GET    /api/mediathek/downloads                     → Download[] (Mediathek und Abos, neueste zuerst, höchstens 100)
DELETE /api/mediathek/downloads/{id}                → 204 (wartet/laeuft → abgebrochen; sonst aus der Liste, die Datei bleibt)
GET    /api/mediathek/abos                          → Abo[]
POST   /api/mediathek/abos {"text","sender","minMinuten"} → Abo (prüft danach sofort)
DELETE /api/mediathek/abos/{id}                     → 204 (geladene Sendungen bleiben)
```

```json
Treffer  {"id":"4sXH…","sender":"WDR","thema":"Tatort","titel":"Cash (2024)","beschreibung":"…","zeit":1790972400,
          "dauer":5338,"groesse":1188036608,"webseite":"https://www.ardmediathek.de/…","video":"https://…mp4"}
Download {"id":1,"quelle":"mediathek|abo|link|upload","titel":"Tatort - Cash (2024)","sender":"WDR","datei":"Tatort - Cash (2024).mp4",
          "bytes":1188036608,"status":"wartet|laeuft|fertig|fehler|abgebrochen","fehler":"…","erstellt":"…","ende":"…","anteil":0.42}
Abo      {"id":1,"text":"tatort","sender":"ARD","minMinuten":80,"erstellt":"…"}
```

Die Quelle ist MediathekViewWeb. `zeit` ist in Unix-Sekunden angegeben, `dauer` in Sekunden, `sender` ist der Kanalname von MediathekViewWeb (`ARD`, `ZDF`, `ARTE.DE`, `3Sat` …).
Die Suche filtert diese Treffer heraus:
- Fassungen mit Audiodeskription, Gebärdensprache, klarer Sprache, OV/OmU oder Untertiteln
- HLS-Streams (`.m3u8`, fast nur ORF und SRF)
- arte in anderen Sprachen als Deutsch

`gesamt` zählt vor dem Filter. Seiten haben 30 Treffer, „Mehr laden“ über `offset`.

Der Server lädt immer nur einen Download zur Zeit nach `<uploads>/Mediathek/<titel>.mp4`, und auf dem Laufwerk bleiben 10 GB frei. `titel` ist schon der Dateiname, den
der Scan einordnet:
- Film: „Titel (Jahr)“
- Reihe: „Thema - Titel (Jahr)“
- Folge: „Thema S01E03 Titel“

`anteil` gibt es nur beim laufenden Download. Abos laden Sendungen der letzten 7 Tage, jede nur einmal (ID) und jeden Titel nur
einmal (gleicher Film auf zwei Sendern). Link- und Upload-Downloads erscheinen nur in der Statistik.
