# Zwei Richtungen für das neue Flimmer-Design

Nach dem Nutzer-Feedback zum ersten Entwurf (Neon-Cyan, Trend-Schrift, Pillen und Verläufe wirkten generisch) gibt es zwei neue visuelle Richtungen. Beide behalten den Aufbau des Designsystems, die Chromium-53-Regeln, die Ampel-Logik und die Sprache. Die Standbilder in den Vorschauen sind gezeichnete Platzhalter für echte Film-Standbilder.

| | Cover | Startseite TV 1920 |
|---|---|---|
| **A · Kinemathek** | [richtung-a-cover.png](richtung-a-cover.png) | [richtung-a-start-tv.png](richtung-a-start-tv.png) |
| **B · Plakat** | [richtung-b-cover.png](richtung-b-cover.png) | [richtung-b-start-tv.png](richtung-b-start-tv.png) |

## A · Kinemathek

Warmes Schwarz (#11100e) mit Papierweiß (#ece7dd) und drei warmen Grautönen, ohne Akzentfarbe: Die primäre Handlung ist eine papierweiße Fläche, und der Fokus ist ein ruhiger papierweißer 3-px-Rahmen mit Scale 1.04. Titel und Wortmarke stehen in der Serife Source Serif 4, die UI in IBM Plex Sans und Zahlen in IBM Plex Mono; die Hierarchie entsteht über Größensprünge (96 px Titel, 24 px Metadaten). Radien 2 px, Haarlinien statt Kästen, die Ampel ist ein kleiner gedeckter Punkt (Salbei, Ocker, Ziegel) neben dem Text, und Platzhalter ohne Bild sind ruhige Tonflächen mit gesetztem Titel wie ein Plakatentwurf.

## B · Plakat

Neutrales Schwarz (#0c0c0c) und gebrochenes Weiß (#f3f2ee), strikt ohne Farbe in der UI, mit 0-px-Ecken und Versalien-Labels mit Sperrung. Alles ist in einer Familie gesetzt, Archivo: schmale Versalien (Breite 62, Gewicht 800) für Titel und Wortmarke wie auf einem Kinoaushang, normale Breite für die UI. Die Wortmarke „FLIMMER“ teilt ein durchlaufender Bildstrich wie ein Filmsteg. Der Fokus ist ein weißer 3-px-Rahmen mit Scale 1.05, die Ampel sind kleine gedeckte Quadrate (voll, halb, Rahmen).

## Gemeinsam

- Beide halten Kontrast AA ein: Text ≥ 7:1, Metadaten ≥ 5:1, Ampel-Marken ≥ 3:1 auf dem Grund.
- Die TV-Schrift ist nie kleiner als 24 px.
- Das Layout hält Chromium 53 ein: Flexbox mit margin, kein Grid, kein gap, kein Blur, kein aspect-ratio.
- Animiert werden nur transform und opacity.
- Das helle Thema entsteht in beiden Richtungen durch Tausch: Papier bzw. Weiß als Grund, warmes bzw. neutrales Schwarz als Text.
