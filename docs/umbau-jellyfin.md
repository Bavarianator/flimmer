# Umbau nach Jellyfin-Vorbild

Stand 29.09.2026. Vorlage ist der Entwurf „Flimmer im Jellyfin-Aufbau“ mit 62 Ansichten:
https://claude.ai/artifact/N3dKuAEzXrpnPZjJpEi2Rt (privat, gehört dem Nutzer).

Die Entwurfsdateien liegen lokal unter
`/tmp/claude-1000/-home-bayernator-Vault/b3cad322-4384-41e5-b056-7a3c80e03421/scratchpad/canvas/project/<Name>.dc.html`.
Eine Datei ist eine Ansicht (HTML mit Inline-Stilen), die Daten stehen im `renderVals()` am Ende.
Die Dateien sind nur Vorlage. In den Code wird nichts davon kopiert, denn die Inline-Stile nutzen `gap` und CSS-Grid.

## Regeln für alle

- **Web läuft auch auf alten TVs.** webOS 4 hat Chromium 53. Deshalb gilt:
  - kein CSS-Grid und kein `gap` bei Flexbox, Abstände über `margin`
  - kein `inset`, kein `aspect-ratio`, keine CSS-Verschachtelung, kein `:is()` oder `:where()`
  - Flexbox mit `-webkit-`-Präfixen wie in `design/komponenten.css`
- **Größenlimit Web:** Das Start-JS bleibt unter 80 KB gzip (`node scripts/size-check.mjs` nach `npm run build`). Seltene Seiten lädt `spaeter()` in `main.tsx` nach.
- **Design:** Farben, Schriften und Abstände kommen nur aus `design/tokens.css`. Neue Farben gibt es nicht, auch keine Akzentfarbe. Radius 2 px, Dialoge 4 px. Fokus ist die Klasse `ist-fokus` mit Doppelring.
- **Jede Seite ist auf drei Geräten bedienbar:** Desktop (Maus/Tastatur), Handy (Touch, `html.hd`, < 600 px) und TV (D-Pad, `html.tv`). Für Fokus und Navigation dienen `useFokus` und `Gruppe` aus `lib/focus.ts`.
- **Texte:** Deutsch, mit echten Umlauten und „…“-Anführungszeichen. Eigene Texte meldet ein Modul mit `ergaenze()` aus `lib/i18n.ts` an. Englisch darf zunächst fehlen, dann greift Deutsch.
- **Builds laufen nacheinander**, weil die Maschine nur 7 GB RAM hat. Jeder Build wird so aufgerufen:
  `flock /tmp/flimmer-build.lock nice -n 10 <befehl>`
  Das gilt für `go build`, `go test`, `npm run build`, `npx tsc` und `./gradlew`.
- **Kein git add, kein git commit.** Im Arbeitsbaum liegen fremde, nicht committete Änderungen früherer Sessions, und die bleiben unangetastet. Über Commits entscheidet der Nutzer.
- **Nur eigene Dateien ändern** (siehe Besitz). Wer eine fremde Datei braucht, schickt dem Besitzer eine Nachricht (SendMessage) oder hinterlässt `// TODO(<besitzer>): …`.
- **Keine neuen Abhängigkeiten**, weder npm noch Go-Module noch Gradle.

## Besitz

| Wer | Dateien |
|---|---|
| lead (Hauptsession) | `web/src/main.tsx`, `web/src/lib/router.ts`, `web/src/components/Seite.tsx`, `web/src/components/Menue.tsx`, `web/src/components/rahmen.css`, `web/src/components/Icon.tsx`, dieses Dokument |
| server | `internal/**`, `cmd/**`, `docs/api-neu.md` |
| web-browse | `web/src/screens/{Start,Filme,Serien,DetailFilm,DetailSerie,Staffel,Favoriten,Sammlungen,Listen,Person,Suche,SucheTastatur}.tsx`, `web/src/components/*` außer den Lead-Dateien, `web/src/lib/api.ts`, `web/src/screens/browse.css` |
| web-admin | `web/src/screens/Einstellungen.tsx`, `web/src/screens/einstellungen/**`, `web/src/screens/admin/**`, `web/src/lib/api-admin.ts` |
| web-player | `web/src/player/**`, `web/src/screens/{Party,Kopplung,KopplungTV,Login,Profile,Setup,LiveTV}.tsx`, `web/src/screens/livetv.css` |
| android | `apps/android/**` |

Icons: Wer ein neues Icon braucht, schreibt den Pfad aus SPEC 5 in `Icon.tsx`, also in die Lead-Datei. Das geht ohne Absprache, solange ein Eintrag nur ergänzt und nichts geändert wird.

## Rahmen (lead)

`<Seite>` bleibt der Rahmen jeder Seite, bekommt aber neue Props:

```ts
Seite({
  bereich,           // markiert den Eintrag in der Seitenleiste:
                     // 'start' | 'favoriten' | 'filme' | 'serien' | 'musik' | 'livetv' | 'sammlungen' | 'listen' | 'suche'
                     // | 'einstellungen' | Dashboard: 'dash' | 'dash-allgemein' | 'dash-benutzer' | … (siehe Menü unten)
  titel?,            // Überschrift in der Kopfzeile (Desktop/Handy)
  tabs?,             // { id, label, pfad }[]: Bibliotheks-Tabs in der Kopfzeile
  tab?,              // aktiver Tab
  admin?,            // true: Dashboard-Seitenleiste statt Hauptmenü
  ueberHero?,        // Kopfzeile transparent über einem Hero (Detailseiten)
  ohneKopf?, class?, children,
})
```

- **Desktop:** links die Seitenleiste (248 px) mit Wortmarke, Menü und Serverstatus unten. Oben die Kopfzeile (64 px) mit Titel, Tabs, Suchfeld, „Gemeinsam schauen“, „Wiedergabe auf anderem Gerät“ und dem Avatar-Menü.
- **Handy:** Kopfzeile mit 56 px (Menü-Knopf, Titel, Cast, Suche, Avatar). Das Menü ist eine Schublade (Drawer, 312 px). Die Tabs sitzen darunter und lassen sich wischen.
- **TV:** links eine Schiene nur mit Icons, die beim Fokus aufklappt. Oben keine Kopfzeile, die Tabs stehen im Inhalt.

Hauptmenü: Startseite, Favoriten · MEDIEN: Filme, Serien, Musik, Live-TV, Sammlungen, Wiedergabelisten · ADMINISTRATION (nur Admin): Dashboard, Metadaten-Manager · BENUTZER: Einstellungen, Abmelden.

Dashboard-Menü (`admin`): Übersicht · SERVER: Allgemein & Branding, Benutzer, Einladungen · BIBLIOTHEKEN: Bibliotheken, Metadaten-Manager · WIEDERGABE: Umwandlung & Trickplay · GERÄTE: Geräte & Aktivitäten, Live-TV & Aufnahmen · ERWEITERT: Netzwerk & Fernzugriff, Geplante Aufgaben, Sicherung & Protokoll. Oben steht „Zurück zu Flimmer“.

Plugins aus dem Entwurf werden nicht gebaut: Flimmer hat kein Plugin-System. Musik folgt in Phase 2.

`Menue.tsx` (lead, fertig) enthält drei Überlagerungen. Alle hängen per Portal an `<body>`, halten den Fokus in sich, geben ihn beim Schließen an den Auslöser zurück und schließen mit der Zurück-Taste.

- **Kontextmenü `<Menue>`:**
  - Signatur: `<Menue eintraege={MenueEintrag[]} anker={element | {x, y}} titel? onZu />`
  - `MenueEintrag` = `{ label, icon?, unter?, onPress?, gefahr?, an?, trenner? }`
  - Auf dem Handy erscheint es als Blatt von unten.
  - Hilfe: `const m = useMenue()`, dann `m.auf(knopfElement)` oder `onContextMenu={m.rechtsklick}`. Solange `m.offen` gilt, wird `<Menue anker={m.anker} onZu={m.zu} …/>` gerendert.
- **Seitenblatt `<Sheet>`:** `<Sheet titel onZu fuss? erster?>…</Sheet>` erscheint rechts, auf dem Handy von unten. Einsatz: Sortieren & Filtern.
- **Dialog `<Dialog>`:** `<Dialog titel onZu knoepfe? breit? erster?>…</Dialog>` erscheint in der Mitte. `breit` = 1000 px, z. B. für den Metadaten-Editor.

Die Kopfzeile verhält sich so:
- Das Suchfeld öffnet mit Enter `/suche/<begriff>`. Die Suche liest den Begriff aus `useRoute()[1]`.
- „Gemeinsam schauen“ öffnet `/party` ohne ID. web-player zeigt dort „Neue Gruppe erstellen / beitreten“.
- `aktionen` sind Knöpfe rechts in der Kopfzeile, etwa „Bibliothek scannen“ im Dashboard. Dafür `<Button>` verwenden, er passt sich in der Kopfzeile an.
- Das Avatar-Menü bietet: Profil wechseln, Einstellungen, (Admin: Dashboard, Metadaten-Manager), Schnellverbindung (`/einstellungen/schnellverbindung`), Fernseher koppeln, Gerät neu testen, Abmelden.

`useGeraet()` aus `Seite.tsx` rendert neu, wenn das Gerät zwischen Handy und Desktop wechselt, `useIch()` liefert das Profil.

Für alle neuen Routen liegen Platzhalter-Dateien bereit, die der Besitzer ersetzt: `Favoriten.tsx`, `Staffel.tsx` (Props `name`, `staffel`), `Person.tsx` (`name`), `Sammlungen.tsx` (`Sammlungen`, `Sammlung({id})`), `Listen.tsx` (`Listen`, `Liste({id})`), `LiveTV.tsx` und `admin/Dashboard.tsx` (`Dashboard`, liest den Bereich aus `useRoute()[1]`).

## Routen

| Pfad | Seite | Besitz |
|---|---|---|
| `/` | Startseite | web-browse |
| `/favoriten` | Favoriten | web-browse |
| `/filme`, `/filme/vorschlaege`, `/filme/genres`, `/filme/studios` | Filme-Bibliothek mit Tabs | web-browse |
| `/serien`, `/serien/demnaechst`, `/serien/genres`, `/serien/folgen` | Serien-Bibliothek | web-browse |
| `/film/:id` | Film-Detail | web-browse |
| `/serie/:name`, `/serie/:name/:staffel` | Serie, Staffel | web-browse |
| `/person/:name` | Person | web-browse |
| `/sammlungen`, `/sammlung/:id` | Sammlungen | web-browse |
| `/listen`, `/liste/:id` | Wiedergabelisten | web-browse |
| `/suche` | Suche | web-browse |
| `/livetv`, `/livetv/programm`, `/livetv/kanaele` | Live-TV | web-player |
| `/watch/:id/:start?`, `/party/:id`, `/koppeln`, `/profile`, `/setup` | wie bisher | web-player |
| `/einstellungen`, `/einstellungen/:bereich` | Benutzer-Einstellungen: profil, anzeige, startseite, wiedergabe, untertitel, schnellverbindung | web-admin |
| `/dashboard`, `/dashboard/:bereich` | Dashboard: allgemein, benutzer, einladungen, bibliotheken, metadaten, umwandlung, geraete, livetv, netzwerk, aufgaben, sicherung | web-admin |

Die alten Pfade `/item/:id`, `/series/:name` und `/pair` bleiben gültig.

## Neue Server-Endpunkte (Vertrag zwischen server und web-*)

Alle Endpunkte liefern JSON und brauchen eine Anmeldung. (A) heißt: nur für Admins. Profile mit FSK-Grenze sehen nur freigegebene Titel, wie bei `/api/library`.

### Favoriten (pro Profil)
```
GET    /api/favorites          → string[]      Schlüssel: Item-ID oder "serie:<Name>"
PUT    /api/favorites/{key}    → 204           {key} URL-kodiert
DELETE /api/favorites/{key}    → 204
```

### Details
```
GET /api/items/{id} → libraryItem (wie in /api/library) plus
  { tagline?, studios?: string[], countries?: string[], people?: Person[], tags?: string[],
    path? (nur Admin), size, container, bitrate?,
    video?: { codec, width, height, hdr?: string, fps? },
    audio: Stream[], subs: Stream[], chapters?: { start: number, name: string }[],
    locked?: string[] }
Person = { name, role?, kind: 'actor'|'director'|'writer'|'producer', image?: string /* URL */ }
Stream = { index, codec, lang?, title?, channels?, default?, forced?, external? }

GET /api/series/{name} → { name, overview?, year?, endYear?, status?, genres?, studios?, people?, age?, rating?,
                           poster?, backdrop?, color? }
GET /api/people/{name} → { name, image?, bio?, items: libraryItem[] /* Filme; pro Serie die erste Folge */,
                           roles: { id: string, role: string }[] }
PUT /api/items/{id}/meta (A) → body { title?, originalTitle?, sortTitle?, year?, overview?, tagline?, genres?, tags?,
                                     studios?, age?, rating?, people?, locked?: string[] } → wie GET /api/items/{id}
```
Personen, Studios, Leitsatz (tagline) und Länder kommen aus TMDB (`credits`) und aus der NFO (`<actor>`, `<director>`, `<studio>`). Bilder von Personen laufen wie Poster über den Bild-Cache (`/api/images/person/<name>/profile` o. ä.).

### Sammlungen (sichtbar für alle, bearbeiten nur Admin)
```
GET    /api/collections           → Collection[]  { id, name, overview?, items: string[] /* IDs oder "serie:<Name>" */, auto?: boolean }
POST   /api/collections (A)       { name, overview?, items? } → Collection
PUT    /api/collections/{id} (A)  { name?, overview?, add?: string[], remove?: string[] } → Collection
DELETE /api/collections/{id} (A)  → 204
```
Filmreihen aus TMDB (`belongs_to_collection`) erscheinen automatisch mit `auto: true`.

### Wiedergabelisten (pro Profil)
```
GET    /api/playlists          → Playlist[] { id, name, items: string[] }
POST   /api/playlists          { name, items? } → Playlist
PUT    /api/playlists/{id}     { name?, add?, remove?, items? /* neue Reihenfolge */ } → Playlist
DELETE /api/playlists/{id}     → 204
```

### Startseite
`POST /api/home` liefert statt `recent` zwei Reihen: `recent-movies` („Kürzlich hinzugefügt in Filme“) und `recent-series` („Kürzlich hinzugefügt in Serien“, je Serie die neueste Folge).

### Dashboard (A)
```
GET    /api/admin/overview  → { server: { name, version, os, arch, uptime /* s */, started, update?: { version, url } },
                                library: { movies, series, episodes, sizeBytes, lastScan? },
                                disks: { path, free, total }[], sessions: Session[], activity: Activity[] /* letzte 20 */ }
GET    /api/sessions        → Session[] { id, user, userColor, device, client, itemId, title, position, duration, paused,
                                          method: 'direct'|'remux'|'transcode', light, reason?, since }
GET    /api/activity?limit= → Activity[] { time, kind: 'login'|'play'|'scan'|'error'|'invite'|'backup'|'task'|'user',
                                            user?, text }
GET    /api/devices         → Device[] { id, name, client, user, lastSeen, ip?, current?: boolean }
DELETE /api/devices/{id}    → 204 (Gerät abmelden)
GET    /api/tasks           → Task[] { id, name, group, description, lastRun?, lastResult?: 'ok'|'error', lastError?,
                                       duration?, next?, running, progress? /* 0..1 */ }
POST   /api/tasks/{id}/run  → 202
GET    /api/logs?level=&limit= → { time, level, msg, attrs? }[]
GET    /api/vpn             → vpn.Status (docs/livetv-vpn.md)
/api/livetv/*               → die Handler aus internal/livetv, Pfade wie in docs/livetv-vpn.md
```
Sitzungen entstehen aus den `/progress`-Herzschlägen der letzten 60 s, also im Speicher und ohne Tabelle. Aktivitäten und Protokoll liegen in je einem Ringpuffer im Speicher.

Aufgaben: Bibliothek scannen, Metadaten aktualisieren, Nachts vorbereiten (optimize), Sicherung erstellen, Datenbank pflegen, Bild-Cache aufräumen, Live-TV-Programm laden.

Die bestehenden Routen bleiben unverändert: `/api/settings`, `/api/settings/review`, `/api/rescan`, `/api/diagnostics`, `/api/invites`, `/api/remote`, `/api/settings/backup|restore|optimize`, `/api/users`, `/api/items/{id}/search|identify`.

### Nachträge vom Server (verbindlich, Details in docs/api-neu.md)

1. **Geräteprofil:** GET-Routen ohne Body, also `/api/items/{id}` und `/api/people/{name}`, lesen das Geräteprofil aus `?device=<id>`, dem mit `PUT /api/devices/{device}/profile` gespeicherten Profil. Ohne den Parameter gilt ein leeres Profil.
2. **Fortschritt:** `POST /api/items/{id}/progress` nimmt optional `"paused": true` an. Player schicken auch im Pausenzustand Herzschläge, sonst verschwindet die Sitzung nach 60 s.
3. **Serien:** `GET /api/series/{name}` liefert zusätzlich `title` (Anzeigename aus den Metadaten), aber nur, wenn er vom Schlüssel abweicht. `name` bleibt der Schlüssel aus dem Pfad.
4. **Personen:**
   - Das Bild liegt unter `/api/images/person/{name}/profile?w=&v=`, die fertige URL steht in `people[].image`.
   - `meta` in Listen enthält keine `people` mehr. Personen gibt es nur in den Details.
5. **Metadaten ändern:** Fehlt bei `PUT /api/items/{id}/meta` das Feld `locked`, werden die geänderten Felder automatisch gesperrt. `"locked": []` entsperrt alles.
6. **Untertitel:** Externe Untertitel stehen in `subs` mit `external: true`. Ihr `index` zählt nur innerhalb der externen Dateien.
7. **Sammlungen:**
   - Automatische Sammlungen haben die IDs `auto-<tmdbId>` bzw. `auto-set-<hash>` und lassen sich nicht bearbeiten.
   - `PUT /api/collections/{id}` nimmt zusätzlich `items` an und ersetzt damit die ganze Liste.
8. **Rollen:** In `/api/people` ist `roles[].id` bei Serien die ID der ersten Folge, also dieselbe ID wie in `items`.
9. **Geräte:** `/api/devices` liefert zusätzlich `userId`. `client` und `ip` gibt es nur für Sessions, die nach dem Umbau angelegt wurden.
10. **Fehlerfall:** Unbekannte Pfade unter `/api/` liefern 404 statt `index.html`.
11. **Bestand:** Personen, Studios, Leitsatz, Länder und Filmreihen kommen bei schon erfassten Titeln erst nach der Aufgabe „Metadaten aktualisieren“ (`POST /api/tasks/meta/run`). Deshalb gilt ein leerer Wert als normal und ist kein Fehler.

## Bildanpassung im Player (Nachtrag 29.09.)

Das Videobild passt sich automatisch an den Bildschirm an. Web-Player und Android verhalten sich gleich.

**Server** (`GET /api/items/{id}`): liefert optional `video.crop = { x, y, w, h }` als Anteile des Bildes (0..1), wenn im Video schwarze Balken eingebrannt sind.
- **Erkennung:** ffmpeg `cropdetect` an drei Stellen (10 %, 50 %, 80 % der Laufzeit, je etwa 2 s). Der Server nimmt die Vereinigung der drei Rechtecke, bleibt also vorsichtig.
- **Keine Balken:** Deckt das Rechteck in beiden Richtungen mindestens 98 % ab, gibt es kein `crop`.
- **Dunkle Szenen:** Ist die Fläche kleiner als 50 %, gilt das als dunkle Szene, und es gibt ebenfalls kein `crop`.
- **Laufzeit und Speicher:** Die Erkennung läuft im Hintergrund beim ersten Abruf, der erste Abruf kommt ohne `crop` zurück. Das Ergebnis bleibt dauerhaft gespeichert.
- **Anteile:** Anteile statt Pixel, damit anamorphe Videos (SAR ≠ 1) und umgewandelte HLS-Streams mit anderer Auflösung stimmen.

**Modi** (pro Gerät gespeichert, Standard „Automatisch“):
- **Automatisch:** schneidet eingebrannte Balken weg (`crop`). Kostet Ausfüllen höchstens 12 % des Bildes, wird ausgefüllt, sonst eingepasst. Beispiele:
  - 2,39:1 auf einem 20:9-Handy: ausfüllen
  - 1,85:1 auf 16:9: ausfüllen
  - 2,39:1 auf einem 16:9-TV: einpassen, mit Balken
  - 16:9 auf einem 20:9-Handy: einpassen
- **Einpassen:** Das ganze Bild ist sichtbar, `crop` wird ignoriert.
- **Füllen:** Das Bild füllt den Bildschirm, die Ränder werden abgeschnitten (nach `crop`).
- **Strecken:** Das Bild wird verzerrt auf den ganzen Bildschirm gezogen (nach `crop`).

**Rechnung**, für alle Clients gleich: R ist der sichtbare Ausschnitt (`crop` oder das ganze Bild), W×H der Bildschirm.
- Skalierung:
  - Einpassen: `s = min(W/Rw, H/Rh)`
  - Füllen: `s = max(W/Rw, H/Rh)`
  - Strecken: `sx = W/Rw`, `sy = H/Rh`
- Platzierung: Das ganze Video wird so gesetzt, dass die Mitte von R in der Bildschirmmitte liegt. Was darüber hinausragt, schneidet der Rahmen ab.
- Neu gerechnet wird bei jeder Größen- oder Drehungsänderung.

**Bedienung:**
- **Web:** Im Player-Menü „Qualität & Geschwindigkeit“ gibt es den Abschnitt „Bild“.
- **Android:** gleiches Menü. Auf dem Handy wechselt ein Zwei-Finger-Zoom (Pinch) zwischen „Automatisch“ und „Füllen“.

## Volle Detailseiten ohne TMDB-Schlüssel (Nachtrag 29.09.)

Auf dem NAS gibt es keinen TMDB-Schlüssel. Die Daten kommen schlüsselfrei (Wikidata/Wikipedia bzw. TVmaze) und die Detailseiten sind deshalb leer. Geplant ist:

**Server:**
1. **Wikidata-Ergänzung bei Filmen** (`internal/meta/keyless.go`), mit deutschen Labels. Übernommen werden:
   - Genres (P136)
   - Regie (P57), Drehbuch (P58), Musik (P86), Produktion (P162)
   - Besetzung (P161, höchstens 20, Rollenname aus dem Qualifier P453, falls vorhanden)
   - Studio (P272) und Land (P495)
   - Personenbilder aus Wikimedia Commons (P18), ausgeliefert über den Bild-Cache wie `people[].image`
   - Wikipedia-Beschreibung: bis zu drei Absätze der Einleitung statt nur einem
   - Titel mit Quelle `wikidata` bekommen die neuen Felder beim nächsten „Metadaten aktualisieren“.
2. **TVmaze bei Serien:** Genres, Sender und Besetzung mit Fotos (`embed=cast`).
3. **Standbilder aus dem Video:**
   - `GET /api/images/{id}/frame?t=<sekunden>&w=` liefert ein JPEG-Einzelbild (ffmpeg, `-ss` vor `-i`). Es wird beim ersten Abruf erzeugt und im Bild-Cache gespeichert. Höchstens zwei Erzeugungen laufen gleichzeitig, mit nice.
   - Hat ein Film kein Hintergrundbild, zeigt `backdrop` in Bibliothek und Details auf ein Standbild bei etwa 20 % der Laufzeit (abgedunkelte Szenen meiden, z. B. mehrere Kandidaten, das hellste gewinnt).
   - `chapters[].image` ist die Frame-URL wenige Sekunden nach Kapitelbeginn.

**Web (web-browse):**
- Kapitel mit Allerweltsnamen („Chapter 1“, „Kapitel 01“, „Chapter 1.“) heißen „Szene 1“, dazu die Startzeit. `chapters[].image` wird als Vorschaubild gezeigt.
- Die Detailseite ist ohne Lücken aufgebaut. Rechts bzw. unter der Beschreibung stehen alle vorhandenen Fakten: Genres, Regie, Drehbuch, Musik, Studio, Land, Laufzeit, FSK, Bewertung, Tags. Leere Felder fallen weg, ohne einen einsamen „Links“-Block.
- „Mehr wie dieses“ gibt es immer: gleiche Genres, sonst gleiche Sammlung oder Jahrzehnt, sonst zuletzt hinzugefügt.

## Android: alles, was das Web kann, nativ (Nachtrag 29.09.)

Nutzerwunsch: In der App fehlt nichts, was im Web geht. Das gilt vor allem für das Dashboard, und alles wird nativ gebaut (Compose, kein WebView). Die Arbeit ist auf drei Agenten mit getrennten Paketen aufgeteilt:

| Agent | Paket bzw. Dateien | Inhalt |
|---|---|---|
| android (Hauptagent) | alles Bestehende in `io.flimmer.app` und `io.flimmer.app.ui`: MainActivity, Rahmen, Detail, Api.kt, PlayerActivity, Party.kt … | volle Detailseiten; Player mit Kapiteln in der Zeitleiste, Nachtmodus, „Vorspann überspringen“, Nächste-Folge-Karte, Geschwindigkeit und Live-Modus; Blatt „Sortieren & Filtern“ und Studios-Tab; Sammlungen und Listen verwalten (anlegen, umbenennen, löschen, entfernen, Reihenfolge); Mehrfachauswahl; **Einbinden** aller neuen Screens in Menü und Navigation |
| android-admin | `io.flimmer.app.admin` (neue Dateien unter `app/src/main/java/io/flimmer/app/admin/`, Tests unter `test/.../admin/`) | Dashboard mit allen 12 Bereichen wie im Web, Metadaten-Manager, Metadaten-Editor mit „Identifizieren“ |
| android-mehr | `io.flimmer.app.livetv`, `io.flimmer.app.einstellungen`, `io.flimmer.app.gemeinsam` | Live-TV (Programme, Programmführer, Kanäle), Benutzer-Einstellungen (Profil, Anzeige, Startseite, Wiedergabe, Untertitel, Schnellverbindung), Lobby „Gemeinsam schauen“ (erstellen, per Code beitreten) |

**Schnittstellen:**
- **Einstiege:** Jedes Modul stellt reine `@Composable`-Einstiege bereit. Der Hauptagent hängt sie in Menü und Navigation ein.
  - admin: `DashboardScreen(api: ApiClient, bereich: String, tv: Boolean, onBereich: (String) -> Unit, onZurueck: () -> Unit)`, `MetadatenEditor(api: ApiClient, id: String, onZu: () -> Unit)`
  - livetv: `LiveTVScreen(api: ApiClient, tv: Boolean, onKanal: (url: String, titel: String) -> Unit)`
  - einstellungen: `EinstellungenScreen(api: ApiClient, bereich: String?, tv: Boolean, onBereich: (String?) -> Unit)`
  - gemeinsam: `GemeinsamLobby(api: ApiClient, tv: Boolean, onRaum: (raumId: String) -> Unit)`
- **Server-Zugriff:** über `ApiClient.roh(method, path, body)` (in Api.kt ergänzt). Datenklassen und JSON-Decoding bleiben im eigenen Paket.
- **Bausteine:** Farben, Schriften und Bausteine kommen aus `ui/Tokens.kt`, `ui/Components.kt` und `ui/Menues.kt`. Die Module nutzen sie nur und ändern sie nicht; fehlt etwas, sagt das Modul dem Hauptagenten Bescheid.
- **Live-Wiedergabe:** `onKanal` startet die PlayerActivity mit den Intent-Extras `live_url` (fertige HLS-URL aus `POST /api/livetv/channels/{id}/play`) und `live_titel`. Die PlayerActivity (Hauptagent) spielt das ohne Zeitleiste und Fortschritt ab.
- **Admin-Rechte:** Das Dashboard gibt es nur für Admins. Die Menüeinträge „Dashboard“ und „Metadaten-Manager“ stehen unter ADMINISTRATION, wie im Web.

**Build:**
- Mit `ANDROID_HOME=/home/bayernator/Android/Sdk flock /tmp/flimmer-build.lock nice -n 10 ./gradlew assembleDebug testDebugUnitTest`, ohne `--no-daemon`, damit der gemeinsame Gradle-Daemon die Builds beschleunigt.
- Baue nicht nach jeder Kleinigkeit, denn die anderen warten auf die Sperre.
- Bricht der Build wegen fremder Dateien ab, schreibst du deren Besitzer an (SendMessage) und reparierst nicht selbst.

## Phasen

Stand 29.09. abends:
- Phase 1 und 2 sind fertig.
  - Server-Tests sind grün, der Web-Build liegt im Budget (45,7 KB, legacy 104,4 KB), Android baut und die Unit-Tests laufen.
  - Durchlauf gegen einen echten Testserver: 24 Seiten auf Desktop, Handy und TV, ohne JS-Fehler.
- Offen:
  - Musik (Phase 3)
  - Test auf dem NAS
  - Punkte ohne Server-Grundlage, siehe unten
- Ohne Server-Grundlage bewusst weggelassen:
  - Plugins
  - Abspiel-Warteschlange („Als Nächstes“, „Zur Warteschlange“)
  - Löschen, Bilder bearbeiten und Untertitel bearbeiten im Kontextmenü
  - Serien-Termine („Demnächst“)
  - Trickplay und Intro-Erkennung
  - Aufnahmen (DVR)
  - Branding und Ports im Dashboard

1. **Rahmen:** Web-Rahmen (lead), Server-Endpunkte (server) und Android-Umbau (android) laufen gleichzeitig.
2. **Web-Seiten:** web-browse, web-admin und web-player arbeiten auf dem Rahmen.
3. **Musik:** Scan von Audiodateien, Künstler/Alben/Titel, Warteschlange, Mini-Player. Server, Web und Android kommen in dieser Reihenfolge dran.
4. **Abnahme:** jede Seite auf Desktop, Handy (390 px) und TV (`?geraet=tv`) gegen den Entwurf prüfen, danach Test auf dem NAS.
