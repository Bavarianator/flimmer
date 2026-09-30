# Notizen von ui-player

## Bitten an ui-fundament (main.tsx)

- **Einladungslinks:** Der Server erzeugt `/einladung#<token>`. Das Fragment ist dort der Token, kein Hash-Pfad. main.tsx sollte vor dem Hash-Routing prüfen: `if (location.pathname === '/einladung') render(<Einladung/>)`. Der Import ist `import { Einladung } from './screens/Login'`.
- **Route /koppeln/{code}:** Kopplung liest einen optionalen Code aus `teile()[1]` und bestätigt ihn sofort. Die Route sollte auch mit Code auf `<Kopplung/>` zeigen.
- **Route /party/{raum}/{sekunde}:** Party liest die Startsekunde selbst aus `teile()[2]`. `<Party id={teile()[1]}/>` reicht.
- **Profile:** `Login` hat die optionale Prop `zurueck`. `Profile` nutzt sie, bei 401 bleibt sie weg.
- **Player:** Die Signatur ist `Player({ id, start?, onBack?, gemeinsam?, seitenleiste? })`. `onBack` wird nicht mehr gebraucht, weil Zurück über `useZurueck` im Router läuft. Die Prop bleibt aus Kompatibilität.
- **CSS der Screens:** Die Stile von Login, PIN, Kopplung, Suche und Einstellungen liegen in `player/screens.css`, weil `screens/` außer den .tsx-Dateien nicht mir gehört. Du darfst sie gern nach `components.css` übernehmen.

## Fehlende Server-Routen und -Felder (für SP)

- **Suche:** Es gibt keine Suchroute. `GET /api/items/{id}/search` ist die TMDB-Zuordnung. Suche.tsx filtert deshalb clientseitig über `/api/library`, fehlertolerant über Trigramme. Personen und Besetzung fehlen in `meta`, darum gibt es keinen Filter „Personen“.
- **Kinderprofil:** `/api/users` und `/api/me` liefern kein `kid` und keine Altersgrenze. Die Profilauswahl zeigt das Kind-Schild erst, wenn `kid: true` kommt. Für die Eltern-PIN beim Verlassen fehlt eine Route.
- **Wiedergabe-Vorlieben pro Profil:** `audioLangs`, `subtitleMode` und `night` kennt nur der Play-Body. Es gibt keine Route, sie am Profil zu speichern. Sie liegen deshalb in localStorage je Profil (`player/vorlieben.ts`) und gelten nur auf diesem Gerät.
- **QR für beliebige Adressen:** `/api/qr` kodiert nur die LAN-Adresse. Der Kopplungscode, z. B. `…/#/koppeln/453432`, und der Party-Link lassen sich deshalb nicht als QR zeigen. Gewünscht ist `GET /api/qr?text=…`, beschränkt auf Adressen dieses Servers.
- **Nächste Folge:** `play` liefert keine nächste Folge. Der Player bestimmt sie clientseitig aus der Bibliothek.
- **Party-Mitglieder:** `members` sind nur Namen. Für „Gastgeber“ und die Avatar-Farbe fehlen ID und Farbe pro Mitglied.
- **Nächtliche Sicherung:** Die Einstellungen können sie nicht ein- oder ausschalten. `db.Maintenance` meldet nur den Stand.

## Jellyfin-Umbau (web-player, 29.09.2026)

- **Player-Bedienung:** Kopf (Zurück, Titel, Ampel, Gemeinsam), Zeitleiste mit Kapitel-Segmenten aus `GET /api/items/{id}` (`chapters`),
  Knopfleiste (Vorheriges/Nächstes Kapitel bzw. Folge, 10 s zurück, Pause, 30 s vor · Ton & Untertitel, Qualität & Geschwindigkeit,
  Vollbild). „Vorspann überspringen“ erscheint im Kapitel namens Vorspann/Intro/Opening, ein Kapitel Abspann/Credits startet die
  Nächste-Folge-Karte. Eigene Segmente (Intro-Erkennung) liefert der Server nicht.
- **Trickplay:** Der Server hat keine Vorschaubilder. Über der Zeitleiste stehen deshalb nur Zeit und Kapitelname.
- **Geschwindigkeit:** beim gemeinsamen Schauen als Aktion `rate` an den Raum.
- **Warteschlange** („Alle abspielen“, web-browse): der Player liest `sessionStorage['flimmer.warteschlange'] = {ids}` selbst.
- **Bildanpassung:** `bild.ts` (reine Rechnung, Selbsttest `node web/scripts/bild-test.mjs`), Modus pro Gerät in `localStorage['flimmer.bild']`,
  Menü „Qualität & Geschwindigkeit“ › Bild. `video.crop` kommt aus `details(id)`; fehlt es, fragt der Player nach 31 s einmal nach.
  Text-Untertitel zeichnet der Player selbst (Spur „hidden“, `cuechange`), damit sie beim Füllen am Bildschirm bleiben.
- **Live-TV:** `Player({ id: '', kanal })` spielt `/api/livetv/channels/{kanal}/play`, ohne Zeitleiste, Fortschritt und Menüs.
- **Gemeinsam schauen:** `/party` ohne Raum ist die Lobby (Titel wählen → `POST /api/party`, Beitreten per Code oder Link).
  Der Gruppencode ist die Raum-ID (12 Hex-Zeichen). Eine Liste offener Räume („Annas Gruppe beitreten“) gibt es serverseitig nicht.

## Offen bei ui-player

- Es gibt noch keine Sichtprüfung im Browser: Playwright war gesperrt, danach war die Maschine knapp. `tsc` und `vite build` sind grün.
- Setup.tsx bleibt ein Platzhalter, der auf die Server-Seite `/setup` umleitet. `internal/setup/pages.css` hat die neuen Tokens bekommen und nutzt kein `gap` mehr.
