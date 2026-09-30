# Flimmer — Logo-Richtlinien (Block-F)

## 1. Das Zeichen
Ein heller Block mit ausgestanztem F, die untere Hälfte des F im Schatten. Zeichen und Wortmarke sind aus Pixelblöcken auf einem Raster gebaut (Symbol 8×8 Zellen, Wortmarke 5 Zeilen x-Höhe plus 2 Zeilen Oberlänge). Die Wortmarke ist zweigeteilt: „flim“ grau, „mer“ hell. Sie ist aus Pfaden gebaut, es gibt keine Schrift und keine Lizenzfrage.

| Datei | Wofür |
|---|---|
| `flimmer-symbol-dunkel.svg` / `-hell.svg` | Zeichen auf dunklem / hellem Grund (durchsichtig) |
| `flimmer-symbol-klein.svg` | unter 24 px: Favicon, Tab, kleine Listen (ohne Grauton, deckend) |
| `flimmer-quer-*.svg`, `flimmer-hoch-*.svg` | Zeichen plus Wortmarke, quer oder gestapelt |
| `flimmer-wortmarke-*.svg` | nur der Schriftzug |
| `flimmer-app-icon.svg` | App-Icon, Profilbild (dunkle Kachel) |
| `export/` | Einfarbig schwarz und weiß (SVG, PNG) |
| `web/` | Favicon, Apple-Touch-Icon, PWA-Icons, `head-snippet.html`, `site.webmanifest` |
| `praesentation.html` | Präsentationsblatt mit Beispielen (im Browser öffnen) |

`dunkel` heißt „für dunklen Untergrund“, `hell` „für hellen Untergrund“.

## 2. Freiraum
Rund um das Logo bleibt **2 × x** frei. x ist eine Zelle des Zeichens, also ein Achtel der Zeichenbreite. Der Freiraum wächst mit dem Logo, nie mit festem Abstand arbeiten.

## 3. Mindestgrößen
| Version | Bildschirm | Druck |
|---|---|---|
| Quer | 96 px breit | 25 mm |
| Gestapelt | 64 px breit | 20 mm |
| Zeichen | 24 px (darunter `-klein`) | 6 mm |
| Klein-Zeichen | 16 px | – |

## 4. Farben
| Name | HEX | RGB | CMYK (Näherung) | Einsatz |
|---|---|---|---|---|
| Saal | `#11100e` | 17 16 14 | 0 6 18 93 | dunkler Grund, Kachel |
| Papier | `#ece7dd` | 236 231 221 | 0 2 6 7 | Block und „mer“ auf Dunkel |
| Grau | `#78716a` | 120 113 106 | 0 6 12 53 | „flim“, Schatten im F |
| Tinte | `#16140f` | 22 20 15 | 0 9 32 91 | Block und „mer“ auf Hell |

Die CMYK-Werte sind gerechnet, keine Pantone-Zuordnung. Für Druck bitte ein Andruckmuster nehmen. Ocker (`#cfae5c`) gehört nicht zu diesem Logo. Er bleibt dem älteren Flacker-F und dem Cursor-Entwurf vorbehalten.

**Kontrast:** Papier auf Saal 15,4 : 1, Tinte auf Papier 14,9 : 1. Das Grau erreicht auf Saal 4,0 : 1, auf Papier 3,9 : 1, auf Weiß 4,5 : 1. Es ist deshalb nur für große Schrift und Zeichenteile gedacht, nie für Fließtext.

**Freigegebene Paare:** Papier-Block auf Saal · Tinte-Block auf Papier oder Weiß · Weiß auf Saal · Schwarz auf Weiß (Dateien in `export/`).
Auf Fotos die einfarbige Version auf ruhiger Fläche oder in einer Kachel.

## 5. Nicht
Nicht strecken, drehen oder umfärben · keine Schatten, Verläufe oder Effekte · Zeichen und Wortmarke nicht neu anordnen · die Wortmarke nicht in einer Schrift nachtippen · das Grau nicht durch Farbe ersetzen · Klein-Zeichen nicht über 48 px verwenden.

## 6. Neu erzeugen
```sh
python3 brand/entwuerfe/make_kit.py        # SVGs in brand/block-f/
```
Danach die Exporte mit `export_variants.py` aus dem Logo-Design-Skill, wie in der Übergabe beschrieben.

## 7. Was noch offen ist
- **Nicht eingebaut:** Web-UI (`web/`), Android-Launcher und `brand/svg/` sind unverändert. Die alten Logos bleiben, bis du sie ersetzt.
- **Alte Lockups kaputt:** `brand/make.py` setzt die Wortmarke falsch. `wordmark()` liefert Pfade oberhalb der Grundlinie, das Skript geht vom Gegenteil aus. Deshalb sind `brand/svg/flimmer-lockup-*.svg` und das PNG dazu oben abgeschnitten.
- **Nicht geprüft:** `praesentation.html` und die Vorschau-Blätter habe ich nicht als Bild gesehen. Auf diesem Rechner ist kein Chromium für Screenshots da. Das Präsentationsblatt bitte einmal im Browser durchsehen.
- **Marke:** Die Form ist an den Stil von OpenCode angelehnt. Vor einer öffentlichen Veröffentlichung eine Markenrecherche machen (DPMA, EUIPO, Bildersuche).
- **Nicht gemacht:** Eine dünner gezeichnete Version für Weiß auf Schwarz. Das Zeichen besteht aus großen Flächen, dort wirkt der Effekt kaum.
