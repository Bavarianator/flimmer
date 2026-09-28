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

## Offen bei ui-player

- Es gibt noch keine Sichtprüfung im Browser: Playwright war gesperrt, danach war die Maschine knapp. `tsc` und `vite build` sind grün.
- Setup.tsx bleibt ein Platzhalter, der auf die Server-Seite `/setup` umleitet. `internal/setup/pages.css` hat die neuen Tokens bekommen und nutzt kein `gap` mehr.
