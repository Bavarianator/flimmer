# Oma-Test (Abnahme Schritt „Einrichtung im Browser“)

**Frage:** Spielt jemand ohne Technikwissen mit leerem Datenordner in unter 3 Minuten den ersten Film, ohne Terminal?

**Ergebnis (2026-09-28, Stand 69789f6 + früher ffmpeg-Download): ja, 148 s.** Keine Fehler im Server-Log.

## Aufbau

- Frische Umgebung: leeres `XDG_CONFIG_HOME`/`XDG_CACHE_HOME`, `PATH` **ohne ffmpeg** (nur `tar` und `xz` zum Entpacken).
- Bibliothek: ein Film (MP4, H.264/AAC, 90 s) und eine Episode (MKV, H.264/AC3).
- Headless-Chromium (Playwright, Chrome for Testing) klickt wie ein Mensch: [`oma-test.js`](oma-test.js).
- Rechner: AMD A4-9125 (2 Kerne), also ungefähr Pi/NAS-Klasse; Heim-DSL.

## Ablauf mit Zeiten

| Zeit | Schritt |
|---|---|
| 11,7 s | Browser offen, Startseite leitet auf `/setup` um |
| 13,4 s | ffmpeg-Download läuft schon im Hintergrund |
| 19,0 s | Servername, Name, Passwort eingegeben |
| 22,3 s | Ordner gewählt, „2 Videos“ gezählt |
| 23,8 s | ffmpeg-Schritt: „wird heruntergeladen … 19 %“ |
| 124,8 s | ffmpeg geprüft (SHA-256), entpackt, „installiert“ |
| 130,9 s | „Fertig – los geht's“ → Bibliothek |
| 136,3 s | Bibliothek zeigt „Neu hinzugefügt“, Filme, Serien |
| 139,1 s | Detailseite |
| **148,0 s** | **Film läuft** (Direct Play, `currentTime` > 2 s) |

## Was der Test verbessert hat

- **Erster Lauf: 262,7 s, also Ziel verfehlt.** 193 s davon entfielen auf den ffmpeg-Download (~100 MB) und das Entpacken, weil der Download erst im letzten Schritt startete.
- **Behoben:** Die Einrichtung startet den Download jetzt beim Öffnen, sofern ffmpeg fehlt und ladbar ist, mit sichtbarem Hinweis. Der Download läuft, während die Person tippt.
- Mit vorhandenem ffmpeg (Docker, Paketmanager) dauert der ganze Ablauf etwa 30 s.

## Beobachtungen für die UI (an ST)

- Die Kopfzeile zeigt nach dem Start länger „Gerät wird getestet …“ (Probe-Clips laufen). Das blockiert nichts, wirkt aber unfertig.
- Ohne TMDB-Key: Filme zeigen den Farbverlauf mit Anfangsbuchstaben, Episoden das ffmpeg-Standbild. Beides ist gewollt. Das Standbild erscheint erst nach dem ersten Erzeugen (einige Sekunden).

## Wiederholen

```sh
cd web && npm run build && cd .. && go build -o /tmp/flimmer ./cmd/server
mkdir -p /tmp/oma/tools && ln -s "$(which tar)" "$(which xz)" /tmp/oma/tools/
env -i HOME=/tmp/oma PATH=/tmp/oma/tools XDG_CONFIG_HOME=/tmp/oma/c XDG_CACHE_HOME=/tmp/oma/k /tmp/flimmer -addr 127.0.0.1:8231 &
npm i playwright-core   # in einem Arbeitsordner; Browser: npx playwright install chromium
S=/tmp/oma BASE=http://127.0.0.1:8231 MEDIA=/pfad/zu/testfilmen CHROME=/pfad/zu/chrome node docs/oma-test.js
```
