# UI-Rework – Design (Phase 2)

Stand: 2026-09-28. Session: flimmer-ui-rework. Status: **Die visuelle Richtung ist noch nicht gewählt. Code in `web/src` entsteht erst nach der Freigabe.**

## Artefakte

| Was | Link / Ort |
|---|---|
| Designsystem „Flimmer“ | https://claude.ai/artifact/4u41G18sE1BuLkX1eUAk2p |
| Zwei neue visuelle Richtungen (Cover und Startseite TV) | [richtungen/](richtungen/README.md) |
| Bestandsaufnahme | [bestand.md](bestand.md) |

Das Artefakt ist privat. Wer außer dem Nutzer es sehen soll, braucht eine Freigabe über das Teilen-Menü der Seite.

## Verlauf

1. **Erster Entwurf** (im Artefakt, Version 3):
   - Aufbau: Markenbuch, Tokens für Kino und Hell, drei Typo-Skalen, 27 Komponenten, Chromium-53-Regeln, Ampel über die Form, Sprache.
   - Optik: Neon-Cyan, Bricolage Grotesque, Pillen, Verläufe.
   - Nutzer-Urteil: Der Aufbau ist „Weltklasse“ und bleibt. Die Optik ist „AI slop“ und wird ersetzt.
2. **Zwei neue Richtungen**, siehe `richtungen/`:
   - **A · Kinemathek**: warmes Schwarz und Papierweiß, Serife für Titel, IBM Plex für die UI.
   - **B · Plakat**: Schwarz und Weiß, schmale Archivo-Versalien, keine Rundungen.
   - Der Nutzer wählt eine davon.
3. **Nach der Wahl**:
   - Im bestehenden Artefakt tauschen: `tokens.json`, `bundle.css`, Vorschauen, Cover, Wortmarke und Icons. Aufbau und Dateien bleiben.
   - Danach die Screens in TV 1920, Desktop 1440 und Handy 390 als HTML und PNG unter `screens/` anlegen.
   - Der Entwurf der Screens im alten Stil ist verworfen und nicht eingecheckt.

## Designentscheidungen, die in jeder Richtung bleiben

- **Chromium 53**:
  - Flexbox mit margin, kein Grid und kein `gap`.
  - `padding-top` statt `aspect-ratio`.
  - Deckende Flächen statt `backdrop-filter`.
  - Klasse `ist-fokus` statt `:focus-visible`.
- **Bewegung**: nur `transform` und `opacity`. `prefers-reduced-motion` wird respektiert.
- **TV**:
  - Schrift mindestens 24 px.
  - Sicherheitsrand 96 × 54 px.
  - Die Reihe verschiebt sich per `translateX`, die Seite per `translateY`.
  - Navigationsleiste links.
- **Desktop**: Kopfzeile mit Bereichen. **Handy**: Tab-Leiste unten, Sheets statt Modals.
- **Ampel**: Die Form trägt die Bedeutung (voll, halb, Ring bzw. Rahmen), gedeckt und klein, immer mit Text-Alternative.
- **Funktionsgleichheit**: Ampel, Fortschritt, Ton- und Untertitelmenü mit Nachtmodus (API: `night`, `subtitleMode`, `subtitleIndex`, `forced`/`sdh`, `notes[]`), Kopplungscode mit QR, Profil-PIN und Kinderprofil, Gemeinsam-schauen-Leiste mit Chat, Suche mit Filtern, Einstellungen mit Diagnose, Sicherung, Fernzugriff und Einladungen.
