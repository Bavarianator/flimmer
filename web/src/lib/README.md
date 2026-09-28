# Web-UI: Schnittstellen (von ui-fundament)

Stand: 2026-09-28. Wer hier etwas braucht, das fehlt: im eigenen Code einen `// TODO(ui-fundament): …`-Kommentar setzen und im Abschlussbericht nennen. Diese Dateien gehören ui-fundament.

## Aufbau

- `design/tokens.json`: Quelle der Tokens, eine Kopie von `docs/ui-rework/tokens.json`. Daraus erzeugt `node scripts/tokens.mjs` die Datei `design/tokens.css` und `Tokens.kt` für Android.
- `design/komponenten.css`: die Komponenten-Stile aus `docs/ui-rework/screens/flimmer.css`, also `.fl-btn`, `.fl-karte`, `.fl-spurmenue`, `.fl-naechste`, `.fl-gemeinsam`, `.fl-panel`, `.fl-zeitleiste`, `.fl-avatar`, `.fl-pin`, `.fl-code`, `.fl-schalter`, `.fl-zeile`, `.fl-gruppe`, `.fl-sheet`, `.fl-modal`, `.fl-toast` usw. Statt `.fl-tv` bzw. `.fl-hd` gilt `html.tv` bzw. `html.hd`. Die HTML-Vorlagen in `docs/ui-rework/screens/` lassen sich damit direkt nachbauen.
- `design/base.css`: Reset, Schrift-Rollen `.t-plakat .t-titel .t-reihe .t-karte .t-text .t-klein .t-label .t-code`, Helfer `.leise .eine-zeile .nur-sr .zeile .wachse .rand`.
- `components/components.css`: Seitenlayout (Seite, Kopf, Hero, Reihe, Raster, Serie).
- CSS-Variablen je Gerät:
  - `--t-plakat`, `--t-titel` usw. als `font`-Kurzform, dazu `--t-…-ls`
  - `--rand` (TV 96, Desktop 64, Handy 16)
  - `--karte-poster`, `--karte-breit`, `--fokus-ring`
- Fokus ist die Klasse `ist-fokus`, kein `:focus-visible`. Der Doppelring kommt aus komponenten.css.

## lib/device.ts
- `geraet: 'tv' | 'dt' | 'hd'`, `istTV()`, `istHandy()`, `touch`, `beiGeraetWechsel(fn)`.
- Setzt `html.tv`, `html.dt` oder `html.hd` sowie `data-theme` (`kino` oder `hell`; der TV bleibt immer `kino`). Zum Testen am Rechner: `?geraet=tv`.
- `thema()` und `setThema('kino' | 'hell' | 'system')`: für die Einstellungen.
- Reicht `isTV`, `isWebOS` und `isTizen` aus `profile.ts` weiter.

## lib/api.ts
- Typen: `Item`, `Meta`, `HomeRow`, `Nutzer`, `Serie { name, staffeln: { nummer, folgen }[], erste, anzahl }` und `Ampel = 'green' | 'yellow' | 'red'`.
- `api<T>(pfad, body?, methode?)`: ohne body ein GET, sonst POST. Wirft `LoginError` bei 401 und ruft vorher den Login-Handler auf; main.tsx zeigt dann die Profilauswahl.
- `anfrage<T>(…)` wie `api`, liefert zusätzlich `headers`.
- `setLoginHandler(fn)`: nur main.tsx.
- Daten, alle mit Cache:
  - `bibliothek(neu?)`, `startseite()` (POST /api/home) und `ich()` (GET /api/me)
  - `nutzer()`, `abmelden()` und `setGesehen(id, bool)`
- Paginierung:
  - `seite(offset, limit)` liefert `{ items, total }`.
  - Hook `useSeiten(groesse)` liefert `{ items, fertig, fehler, mehr() }`.
- Cache (stale-while-revalidate):
  - `holen(k, laden, { merken, maxAlter })`, `vergessen(praefix)` und `gecacht(k)`
  - Hook `useDaten(k, laden, { merken })` liefert `{ daten, fehler, laedt, neu() }`.
  - `merken` legt den Eintrag zusätzlich in localStorage ab.
- Hilfen:
  - `anzeigeTitel(it)`, `jahr(it)` und `bild(it, 'poster' | 'backdrop', breite)`; `bild` liefert `''` ohne Bild.
  - `fortsetzenAb(id)`, `anteil(it)` (0 bis 1), `serien(items)`, `naechsteFolge(serie)` und `titelAus(id)`
- `geraeteTest.starten()`, `.laeuft()` und `.wennNoetig()`: der Probe-Test aus probe.ts. Danach werden die Ampeln neu geladen.

## lib/router.ts
- Hash-Routing: `go(pfad, { ersetzen? })`, `back()`, `kannZurueck()`, `pfad('serie', name)` (kodiert), `useRoute()` (liefert `string[]`) und `aktuell()`.
- Routen:
  - Browse: `/`, `/filme`, `/serien`, `/film/:id`, `/serie/:name`, `/watch/:id/:start?`
  - Weitere Screens: `/suche`, `/einstellungen`, `/koppeln`, `/profile`, `/party/:id`, `/setup`
  - Alte Pfade bleiben gültig: `/item/:id`, `/series/:name`, `/pair`.
- Zurück-Tasten (461, 10009, 27 und 8 außerhalb von Feldern) laufen über einen Stapel.
  - Mit `useZurueck(fn, aktiv?)` meldet man einen Handler an; der zuletzt angemeldete kommt zuerst dran.
  - Gibt `fn` `true` zurück, ist die Taste erledigt. Sonst geht es eine Seite zurück.
  - Auf `/` ohne Verlauf bleibt die Taste bei der TV-Hülle.

## lib/focus.ts (norigin-spatial-navigation)
- `useFokus({ fokusKey, onPress, onFocus, onArrow, aus })` liefert `{ ref, fokus, fokusKey, fokusSelbst, dom }`.
  - `dom` gehört auf das Element (`{...f.dom}`): Klick, Tab-Fokus und Magic-Remote-Zeiger.
  - Bei `fokus` die Klasse `ist-fokus` setzen.
- `<Gruppe fokusKey class tag grenze bevorzugt label>`: Container mit Fokus-Gedächtnis. Mit `grenze` verlässt der Fokus die Gruppe nicht, gedacht für Menüs und Sheets.
- `fokusStart(ersatz)`: nach dem Laden einer Seite aufrufen. Stellt den gemerkten Fokus dieser Route wieder her, sonst `ersatz`.
- `fokusBald(key)`, `setFocus`, `pause` und `resume`.
- `usePausierteNavigation(aktiv?)`: für Texteingabe und eigene Tastensteuerung.
- TV: `.schiene` fährt per translateX, `.seite-schiene` per translateY. Das erledigt `folge()` automatisch im onFocus von `useFokus`.

## lib/i18n.ts
- `t(schluessel, { var })`, `sprache` und `setSprache('de' | 'en')`.
- **`ergaenze(de, en?)`**: eigene Texte eines Moduls anmelden, ohne i18n.ts zu ändern.
- Formate:
  - `dauer(sek)` ergibt „2 Std. 29 Min.“, `uhr(sek)` ergibt „1:12:04“.
  - `folge(s, e)` ergibt „S1 E3“, `anzahl(n, einsKey, vieleKey)` wählt Einzahl oder Mehrzahl.

## components/
Alle Komponenten sind fokussierbar über `lib/focus`, Props auf Deutsch.

**Bedienung**
- `Button { fokusKey?, onPress, variante?: 'primaer' | 'sekundaer' | 'geist', icon?, label?, aus?, voll?, children? }`: ohne children entsteht ein runder Icon-Knopf, dann ist `label` Pflicht.
- `Chip { fokusKey?, an?, icon?, onPress, children }`
- `Tabs { tabs: { id, label }[], aktiv, onWahl, fokusKey, label? }`

**Karten und Bilder**
- `Karte { fokusKey, titel, unter?, bild?, farbe?, fuss?: [links, rechts], breit?, fortschritt?, ampel?, gesehen?, onPress, onFocus? }`
- `TitelKarte { it, reihe, breit?, onFocus? }`, `SerienKarte { s, reihe, onFocus? }`, `zielVon(it)` und `unterzeile(it)`
- `AmpelPunkt { stufe }`, `AmpelZeile { stufe, kurz? }`, `Badges { ampel?, gesehen? }`, `Fortschritt { anteil }` und `Bild { src?, titel, farbe?, fuss? }`
- `Bild` lädt lazy per IntersectionObserver. Ohne Bild zeigt es eine Tonfläche mit dem Titel in Plakat-Versalien.

**Seitenaufbau**
- `Reihe { titel, fokusKey, zusatz?, children }` und `Raster { fokusKey, children }`
- `Hero { titel, label?, fakten?, ampel?, ampelLang?, text?, bild?, farbe?, class?, children? }`
- `Seite { bereich?: 'start' | 'filme' | 'serien' | 'suche', ohneKopf?, class?, children }`: Kopfzeile bzw. Tab-Leiste und TV-Seitenschiene.
- `useIch()` liefert das angemeldete Profil.
- `Abspielknoepfe { it, extra? }` mit den Fokus-Schlüsseln `abspielen` und `vonvorn`.

**Zustände und Kleinteile**
- `SkeletonKarten { n?, breit? }`, `SkeletonReihe` und `SkeletonHero`
- `Leer { titel, text?, icon?, aktion?: { label, onPress } }` und `Fehler { fehler, nochmal? }`
- `Icon { name, label?, class? }` mit den Namen in Icon.tsx
- `Wortmarke`, `MiniAvatar { n }` und `avatarFarbe(color)`

## Screens von ui-player
main.tsx importiert diese Exporte: `Login({ onDone })` und `Profile()` direkt, alle anderen werden bei Bedarf nachgeladen: `Kopplung()`, `Einstellungen()`, `Setup()`, `Suche()`, `Party({ id })` und `Player({ id, start? })`. Einstellungen.tsx und Setup.tsx sind noch Platzhalter von ui-fundament, die ui-player ersetzt. Die alten Dateien Login.tsx, ui.tsx, route.ts, data.ts, Home.tsx, Detail.tsx und style.css sind entfernt.
