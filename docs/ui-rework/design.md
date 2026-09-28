# UI-Rework – Design (Phase 2)

Stand: 2026-09-28. Session: ui-design (vorher flimmer-ui-rework). Status: **Richtung „Mischung“ ist gewählt und im Designsystem umgesetzt.**

## Artefakte und Dateien

| Was | Ort |
|---|---|
| Designsystem „Flimmer“ (Version 5, Richtung Mischung) | https://claude.ai/artifact/4u41G18sE1BuLkX1eUAk2p |
| Tokens als Datei (Kopie aus dem Artefakt) | [tokens.json](tokens.json) |
| Screens: 17 Screens in TV, Desktop und Handy (HTML + PNG) | [screens/](screens/README.md) |
| Kontrollbilder der Mischung | [richtungen/mischung-cover.png](richtungen/mischung-cover.png), [richtungen/mischung-start-tv.png](richtungen/mischung-start-tv.png) |
| Die zwei Richtungen zur Wahl | [richtungen/](richtungen/README.md) |
| Bestandsaufnahme | [bestand.md](bestand.md) |

Das Artefakt ist privat. Andere sehen es erst nach einer Freigabe über das Teilen-Menü.

## Verlauf

1. **Erster Entwurf** (Version 3 des Artefakts):
   - Aufbau: Markenbuch, Tokens, 27 Komponenten, Chromium-53-Regeln, Ampel, Sprache. Der Aufbau bleibt.
   - Die Optik wurde verworfen: Neon-Cyan, Bricolage Grotesque, Pillen, Verläufe.
2. **Zwei Richtungen**: A „Kinemathek“ und B „Plakat“ (siehe `richtungen/`).
3. **Nutzer-Entscheidung „Mischung“**:
   - Das Grundgerüst kommt von A.
   - Wortmarke, Hero-Titel und bildlose Platzhalter bekommen die schmalen Plakat-Versalien von B.
   - Die Serife fällt weg.

## Zentrale Designentscheidungen

- **Farben**:
  - Warmes Schwarz `#11100e` als Grund, Papierweiß `#ece7dd` als Text.
  - Dazu drei warme Grautöne, keine Akzentfarbe. Hervorhebung ist Helligkeit: Primärbutton, Fokus und Fortschritt sind papierweiß.
  - Die Ampel ist gedeckt (Salbei `#8fae86`, Ocker `#cfae5c`, Ziegel `#d4826f`) und klein. Die Form trägt die Bedeutung: voller Punkt, halber Punkt, Ring.
  - Das Papier-Thema (hell) tauscht die Rollen: Grund `#f3efe6`, Text Tinte `#16140f`.
- **Typografie**, zwei Familien:
  - Archivo in Breite 62 und Gewicht 800, als Versalien: nur Wortmarke, Hero- und Detailtitel (TV 144 px, Desktop 104 px, Handy 56 px), bildlose Platzhalter und Avatar-Initialen.
  - IBM Plex Sans für die ganze UI in normaler Schreibung.
  - IBM Plex Mono für Zeiten, Codes, Messwerte und kleine Kicker.
  - Auf dem TV nie unter 24 px.
- **Form**:
  - Ecken 2 px, Haarlinien statt Kästen.
  - Fokus: papierweißer Rahmen, 3 px auf dem TV, mit 3 px dunklem Abstand. Karten skalieren um 1.04. Kein Glow.
- **Layout TV vs. Handy**:
  - TV: ruhige Textnavigation oben im 96/54-px-Sicherheitsrand, Hero mit Plakat-Titel. Die Reihen fahren per `translateX`, die Seite per `translateY`.
  - Handy: Tab-Leiste unten, Hero als Standbild mit dem Titel darauf, Reihen zum Wischen, Sheets statt Modals.
  - Desktop: Kopfzeile mit Suche und Avatar-Menü.
- **Chromium 53**:
  - Nur Flexbox mit margin, kein Grid, kein `gap`, kein `aspect-ratio`, kein Blur.
  - Die Plakat-Schrift kommt als statische woff2-Instanz, weil Chromium 53 keine variablen Achsen kann.
  - Animationen nur mit `transform` und `opacity`.
- **Funktionsgleichheit**: Ampel, Fortschritt, Ton-/Untertitelmenü mit Nachtmodus (`night`, `subtitleMode`, `subtitleIndex`, `forced`/`sdh`, `notes[]`), Kopplungscode mit QR, PIN und Kinderprofil, Gemeinsam-schauen-Leiste mit Chat, Suche mit Filtern, Einstellungen mit Diagnose, Sicherung, Fernzugriff und Einladungen.

## Entscheidungen des Dirigenten (2026-09-28)

- Die TV-Navigation liegt oben als Textnavigation.
- Der TV bleibt immer im Kino-Thema, das Papier-Thema gibt es nur auf Desktop und Handy.
- `/setup` und `/settings` stellt ui-player auf die Tokens um.
- Das Artefakt bleibt privat.
