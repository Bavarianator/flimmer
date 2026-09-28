# UI-Rework – Bestandsaufnahme (Phase 1)

Stand: 2026-09-28, Code-Stand `9d54f2b` plus die offenen Änderungen im Arbeitsbaum. Session: flimmer-ui-rework.

## Aufbau des Tests

- Server lokal aus dem Arbeitsbaum gebaut (`npm run build`, `go build ./cmd/server`) und mit einem leeren Datenordner im Scratch gestartet (Port 8231, `-https 0`). Das Spiel-NAS wurde nicht angefasst.
- Bibliothek: `scripts/testmedia.sh` (Direkt, Remux mit SRT+ASS, Tonwandel DTS, HEVC 10 bit, HDR10, Testserie) plus 8 Filme und die Serie „Dark“ (2 Staffeln, 6 Folgen) als lavfi-Clips. Kein TMDB-Schlüssel, also überall Platzhalter statt Poster.
- Konten über `/setup`: Admin „Chris“ mit Passwort, dazu „Mia“ (ohne Passwort) und „Oma Hilde“ (mit Passwort). Fortschritt bei Metropolis, Das Boot, Remux, Dark S1E1 (95 %) und S1E2 (30 %).
- Headless-Chromium über Playwright in drei Größen: **TV 1920 × 1080** (User-Agent eines LG-webOS-Geräts, damit `isTV` greift), **Desktop 1440 × 900**, **Handy 390 × 844** (Touch, mobil).
- Zustände Laden, leer und Fehler über abgefangene Anfragen erzeugt (`/api/library` verzögert, leer, abgebrochen; `/api/items/*/play` mit 500).
- Das Skript liegt nicht im Repo; es folgt dem Muster von `docs/oma-test.js`.

## Screenshots

Unter [`bestand/`](bestand/), Dateiname `<Nr>-<Screen>-<Größe>.png`:

| Nr. | Screen / Zustand |
|---|---|
| 01–05 | Setup: Willkommen, Konto, Ordner, ffmpeg, Fertig |
| 10–13 | Profilauswahl (TV mit Kopplungscode), Passwort, falsches Passwort, Server weg |
| 20–23 | Start, Start ganze Seite, D-Pad-Fokus (TV), Start im Profil von Mia |
| 30–34 | Detail Film (Fortsetzen, neu, rote Ampel), Serie, Serie ganze Seite |
| 40–43 | Player mit Overlay, Ton-/Untertitelmenü, Pause (TV), Wiedergabefehler |
| 50–51 | Kopplung am Handy, falscher Code |
| 60–61 | Einstellungen (`/settings`), ganze Seite |
| 70–74 | Start lädt, Detail lädt, leere Bibliothek, Server nicht erreichbar, unbekannter Titel |

**Lücke:** Der TV-Durchlauf ist vollständig. Desktop fehlen 13 und 70–74, Handy alles außer Setup. Der Lauf wurde auf Bitte des Dirigenten angehalten (Maschine ausgelastet, APK-Build hatte Vorrang). Die Zustände sehen auf allen Größen gleich aus: ein Satz Text, kein Button. Der Rest wird nachgereicht, sobald Playwright wieder laufen darf.

Hinweis zum Player: Das Video bleibt in den Screenshots schwarz. Chrome for Testing hat keine H.264-Decoder, das liegt am Testaufbau und nicht an Flimmer.

## Befund: Schwachstellen

### 1. Auf dem TV nicht lesbar, im 10-Fuß-Abstand viel zu klein

Die UI hat eine Schriftgröße für alle Geräte. Auf 1920 × 1080 gibt es keine eigene Skala:
- Kopfzeile 14 px, Legende 13 px, Kartenuntertitel ca. 14 px, Beschreibung 17 px, Reihentitel 20 px.
- Das Ziel sind mindestens 24 px. Fast alles liegt darunter, außer Titeln und dem Kopplungscode.
- Auch die Ampel-Punkte sind mit 10–12 px auf dem TV kaum zu sehen.

### 2. Teile der Oberfläche sind mit dem D-Pad nicht erreichbar

- „Gerät neu testen“, „Fernseher koppeln“, „Einstellungen“ und „Profil wechseln“ sind einfache Links in der Kopfzeile. Sie sind nicht bei `norigin-spatial-navigation` registriert, der Fokus kommt also nicht hin. Auf dem TV kann man das Profil nicht wechseln, ohne die App neu zu starten.
- `/settings` ist eine eigene Server-Seite (`internal/setup/settings.html`). Sie ist ein Desktop-Formular ohne Fokus-System. Auf dem TV ist sie praktisch nicht bedienbar und nur über einen fokuslosen Link erreichbar.
- Auf dem Desktop blendet der eigene Player das Overlay nach 4 s aus. Danach bleiben nur die nativen `<video controls>`, und das Ton-/Untertitelmenü ist nicht mehr erreichbar. Im Test ließ sich der Button nicht klicken.

### 3. Chromium-53-Regeln sind auf den Server-Seiten verletzt

- `internal/setup/pages.css` und `settings.html` nutzen `gap` bei Flexbox, 5-mal in `pages.css` und 5-mal inline in `settings.html`. Auf webOS 4 und Tizen 4 fallen diese Abstände weg: Buttons kleben aneinander, Status-Punkte kleben am Text.
- Die Web-UI selbst hält die Regeln ein. Ihre Fokus-Stile nutzen zusätzlich `:focus-visible`, das Chrome 53 ignoriert. Das schadet nicht, weil `.focused` dasselbe tut.

### 4. Kein Leer-, Lade- oder Fehlerzustand, der weiterhilft

- **Laden:** nur „Lade Bibliothek …“ bzw. „Lade …“ als grauer Text, keine Skeletons. Beim Nachladen springt das Layout.
- **Fehler:** Die Startseite zeigt „Server nicht erreichbar: Failed to fetch“ als Rohtext über der Kopfzeile. Es gibt keinen Button „Erneut versuchen“ und keinen Hinweis, was zu tun ist.
- **Player-Fehler:** „500 Transcoder nicht verfügbar“ im Rohformat mit HTTP-Code, ohne Handlung.
- **Leer:** „Noch keine Filme gefunden. In den Einstellungen einen Medienordner hinzufügen.“ Für Nicht-Admins gibt es die Einstellungen nicht, einen Link auch nicht.
- **Unbekannter Titel** (`#/item/unbekannt`): bleibt für immer bei „Lade …“.

### 5. Ohne Poster wirkt alles gleich und billig

- Ohne TMDB-Schlüssel, also beim ersten Start und auf dem NAS, bekommen alle Filme einen zufälligen Farbverlauf mit einer Initiale. Die Startseite zeigt eine Wand aus „D D D D G H L“.
- Sechs Filme fangen mit „D“ an und sind nicht zu unterscheiden.
- Die Verläufe sind grell gesättigt (Grün, Violett, Rot) und wirken nicht wie Kino.
- Episoden ohne Standbild zeigen einen Verlauf ohne Titel.

### Weitere Befunde

- **Ampel:** Sie unterscheidet sich nur durch die Farbe. Die Legende steht klein am Seitenende, auf dem TV unter dem sichtbaren Bereich. Grün und Rot sind bei Rot-Grün-Schwäche schwer zu trennen. Die Gründe im Player sind Technik-Sprache („Video-Codec h264 wird vom Gerät nicht unterstützt · Ton aac …“).
- **Weiterschauen bei Serien:** Die Karte heißt nur „Dark“ und zeigt „S1 E2 · Episode 2“. Unter „Neu hinzugefügt“ erscheint „Dark“ mehrmals (S2 E2), jeweils als eigene Karte.
- **Serienseite:** keine Staffelwahl, sondern Staffeln als Reihen untereinander. Episoden ohne Titel und Inhalt, gesehene Episoden ohne Haken (S1E1 bei 95 % sieht aus wie ungesehen). „Staffel als gesehen markieren“ fehlt.
- **Detailseite:** keine Besetzung, keine Tonspuren und Untertitel vorab, kein „Als gesehen markieren“, keine Merkliste. Das Hero-Bild ist das hochskalierte Standbild (pixelig).
- **Profilauswahl:** Profile mit Passwort und Kinderprofile sehen gleich aus. Es gibt kein Schloss, kein „Kind“ und kein „Profil hinzufügen“. Der Titel „Wer schaut?“ hat einen blauen Verlauf, die Profilnamen sind klein (17 px).
- **Kopfzeile:** Das Gerät („LG webOS“) steht wie ein Nutzername da. „Gerät neu testen“ ist ein Entwickler-Werkzeug auf der Startseite. Während des Tests steht dort lange „Gerät wird getestet …“ (siehe `docs/oma-test.md`).
- **Navigation:** Es gibt keine Bereiche (Filme, Serien, Suche, Merkliste). Alles ist eine lange Startseite. Suche und Filter fehlen ganz.
- **Player:** Oben stehen Titel, Methode und Gründe in einer Zeile, unten eine dünne Leiste. Es fehlen Restzeit, „endet um“, Spul-Vorschau und Play/Pause-Knöpfe für Maus und Touch. Die Tonspur heißt „Spur 1 · AAC Mono“, wenn keine Sprache gesetzt ist. Nächste Episode, Nachtmodus und Gemeinsam schauen fehlen.
- **Handy:** Die Kopfzeile bricht in mehrere Zeilen um. Tippziele wie „Profil wechseln“ sind kleiner als 44 px. Das Setup auf dem Handy ist dagegen ordentlich.
- **Einstellungen:** eine lange Seite mit zehn Karten ohne Gliederung. Es gibt keine Einladungen (`internal/share`) und keine Bereiche pro Gerät oder Profil. Die Diagnose ist ein Rohprotokoll.
- **Wortmarke:** Die Schrift „Flimmer“ mit Verlauf von Weiß nach Blau ist eine Systemschrift und wirkt beliebig.
- **Fokus:** Der Ring (3 px, `#7c9cff`) verschwindet auf blauen und hellen Postern. Buttons skalieren und verschieben dabei die Nachbarn (`scale` am Button selbst ohne Platz).
- **`scrollIntoView`** bei jedem Fokuswechsel ruckelt auf schwachen TVs. Die Reihe scrollt nativ statt per `transform`.

## Was gut ist und bleiben soll

- Die Ampel als Idee: Pro Gerät sieht man vor dem Start, ob ein Titel flüssig läuft.
- TV-Kopplung per 6-stelligem Code in der Profilauswahl, ohne Tastatur.
- Das Ton-/Untertitelmenü mit zwei Spalten ist per D-Pad schnell (↑ öffnet, ←/→ wechselt die Spalte).
- Das Setup ist klar in vier Schritte gegliedert, der ffmpeg-Download läuft im Hintergrund, und es funktioniert auf dem Handy.
- Das Weiterschauen landet an der richtigen Stelle, und der Fortschritt ist pro Profil getrennt.

## Nutzerwege

Diese sechs Wege muss das neue Design auf TV (nur D-Pad), Desktop und Handy tragen. Heute gezählte Schritte gelten für den TV.

| # | Weg | Heute (TV) | Ziel im neuen Design |
|---|---|---|---|
| 1 | **Film fortsetzen** | Start → erste Karte „Weiterschauen“ hat Fokus → OK öffnet die Detailseite → OK „Fortsetzen ab …“: 2× OK | Start öffnet mit Fokus auf „Weiterschauen“, der Hero zeigt den Titel. OK auf der Karte startet sofort ab der letzten Stelle; die Info-Taste bzw. ↑ öffnet die Details: **1× OK** |
| 2 | **Neue Episode schauen** | Start → ↓ zu „Serien“ → Serie → OK auf „Fortsetzen“ spielt die zuletzt angefangene Folge, nicht die neue; die neue Folge liegt in der Staffel-Reihe | „Weiterschauen“ zeigt die nächste Folge („S1 E3 · Titel“); „Neu“-Badge auf der Serie; Serienseite mit Staffel-Tabs und markierter nächster Folge; am Ende der Folge die Nächste-Folge-Karte |
| 3 | **Suche** | nicht vorhanden | ← öffnet die Leiste, „Suche“ → Bildschirmtastatur links, Treffer rechts schon beim Tippen, tippfehlertolerant; Filter-Chips darüber |
| 4 | **TV koppeln** | TV: Profilauswahl zeigt den Code. Handy: angemeldet → Kopfzeile „Fernseher koppeln“ → Code tippen → „Koppeln“ | TV zeigt Code **und QR-Code**; das Handy scannt oder tippt sechs Ziffern und koppelt nach der sechsten Ziffer ohne Button; Bestätigung auf beiden Geräten |
| 5 | **Profil wechseln** | per D-Pad nicht möglich (Link nicht fokussierbar) | ← Leiste → Avatar unten → Profilauswahl; Profile mit Schloss verlangen die PIN (Zifferntasten der Fernbedienung); Kinderprofil verlassen braucht die Eltern-PIN |
| 6 | **Untertitel einschalten** | im Player ↑ → → zur Spalte Untertitel → ↓ → OK: 4 Tasten | gleich schnell (↑ → ↓ OK), dazu „Automatisch“ als Standard pro Profil, Beschriftung „Erzwungen“/„SDH“, Nachtmodus im selben Menü; die Wahl wird pro Serie gemerkt |
