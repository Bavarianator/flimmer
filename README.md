# Flimmer

Ein schlanker Open-Source-Medienserver für Filme und Serien, als Alternative zu Jellyfin und Plex.

**Wiedergabe soll auf jedem Gerät zuverlässig laufen.** Flimmer entscheidet pro Titel und Gerät, ob die Datei direkt abgespielt, nur umverpackt oder transkodiert wird. Ziel ist, dass so wenig wie möglich neu kodiert wird. Deshalb werden Untertitel nie eingebrannt, Segmente liegen auf echten Keyframes, und wenn nur der Ton nicht passt, wird nur der Ton umgewandelt. Eine Ampel zeigt vorher an, wie gut ein Titel auf dem Gerät läuft.

Weitere Ziele:
- **Wenig Ressourcen:** eine einzige Binary ohne Abhängigkeiten außer ffmpeg. Läuft auch auf dem Raspberry Pi.
- **Alle Geräte:** eine Web-UI für Browser, LG webOS, Samsung Tizen, Android (TV) und iOS.
- **Gemeinsam schauen und einfaches Teilen:** synchron schauen mit Freunden, Einladungslinks mit Registrierung, Anmeldung am Fernseher per QR-Code.

> Status: frühe Version. Server, Web-UI, Android-App (Handy und TV) und LG-Starter laufen; fertige Releases folgen.

## Installation

Du brauchst nur [Docker](https://docs.docker.com/get-docker/). Ein Befehl lädt Flimmer und startet es. `/pfad/zu/filmen` ersetzt du durch deinen Medienordner:

```sh
curl -fsSL https://raw.githubusercontent.com/Bavarianator/flimmer/main/deploy/install.sh | sh -s -- /pfad/zu/filmen
```

Das Skript lädt das fertige Image, ffmpeg ist schon drin. Flimmer startet danach mit dem Rechner von selbst. Unter Linux nutzt es das Host-Netz, damit Apps und Fernseher den Server im Heimnetz finden, und die Grafikkarte (`/dev/dri`) zum Umwandeln, falls vorhanden.

- **Aktualisieren:** denselben Befehl noch einmal ausführen. Einstellungen und Konten bleiben im Volume `flimmer-data`.
- **Anderer Port**, z. B. weil Jellyfin schon 8096 belegt: `… | FLIMMER_PORT=8097 sh -s -- /pfad/zu/filmen`
- **Mit Docker Compose:** `curl -fsSL https://raw.githubusercontent.com/Bavarianator/flimmer/main/deploy/compose.remote.yml | MEDIA_DIR=/pfad/zu/filmen docker compose -f - up -d --pull always`

### Erste Schritte

1. Die Adresse öffnen, die das Skript anzeigt (z. B. `http://192.168.1.20:8096`), und das Admin-Konto anlegen. Der Medienordner heißt im Container `/media`.
2. **Fernseher anmelden:** Der Fernseher zeigt einen Code und einen QR-Code. Scanne den QR-Code mit der Handy-Kamera oder in der Flimmer-App, dann ist der Fernseher angemeldet, ganz ohne Tippen.
3. **Freunde einladen:** Unter Dashboard › Einladungen einen Link erstellen und verschicken. Wer ihn öffnet, wählt Name und Passwort und kann sich danach überall anmelden, mit Fernzugriff auch von unterwegs.

| Aufgabe | Befehl |
|---|---|
| Logs ansehen | `docker logs -f flimmer` |
| Stoppen / starten | `docker stop flimmer` / `docker start flimmer` |
| Entfernen | `docker rm -f flimmer` (Einstellungen löschen: `docker volume rm flimmer-data`) |

## Ohne Docker

Voraussetzung ist ein Build aus dem Quellcode (siehe unten: Go, Node und ffmpeg). Danach:

```sh
./flimmer -media /pfad/zu/filmen
```

Automatisch mit dem Rechner starten: `flimmer install`, wieder entfernen mit `flimmer uninstall`. Dafür sind keine Admin-Rechte nötig. Unter Linux wird dabei eine systemd-User-Unit angelegt, unter macOS ein LaunchAgent, unter Windows ein Eintrag im Autostart. Unter Linux und macOS muss ffmpeg installiert sein, z. B. mit `sudo apt install ffmpeg` bzw. `brew install ffmpeg`.

### Als Systemdienst (Linux-Server)

Für einen eigenen Benutzer und Härtung gibt es [`deploy/flimmer.service`](deploy/flimmer.service). Die Anleitung steht in der Datei.

| Flag | Standard | Bedeutung |
|---|---|---|
| `-media` | – | Medienordner, mehrere mit Komma getrennt. Alternativ in der Einrichtung wählen. |
| `-data` | Benutzer-Cache-Ordner/flimmer | Einstellungen, Cache, Poster |
| `-addr` | `:8096` | Adresse, auf der der Server lauscht |

### Aus dem Quellcode

Du brauchst Go 1.27+, Node 22 und ffmpeg.

Docker-Image selbst bauen: `docker build -f deploy/Dockerfile -t flimmer .` bzw. `MEDIA_DIR=/pfad/zu/filmen docker compose -f deploy/compose.yml up -d --build`.

```sh
(cd web && npm ci && npm run build)   # UI nach web/dist bauen, wird in die Binary eingebettet
go build -o flimmer ./cmd/server      # erzeugt die Datei ./flimmer
go run ./cmd/server -media /pfad/zu/filmen
go test ./...
```

Ein Release entsteht durch einen Tag, z. B. `git tag v0.1.0 && git push --tags`. Die CI baut daraus die Binaries für alle Plattformen, das Windows-ZIP und die Docker-Images `:v0.1.0` und `:latest`. Den TMDB-Projekt-Key liest sie aus dem Repo-Secret `TMDB_KEY`.

### LG-TV (webOS)

Die App in `apps/webos` fragt nur nach der Server-Adresse und lädt die Oberfläche dann direkt vom Server. Neue Server-Versionen erreichen den TV deshalb ohne App-Update.

1. Auf dem TV die App „Developer Mode“ installieren, einschalten und den TV mit `ares-setup-device` einrichten ([Anleitung](https://webostv.developer.lge.com/develop/getting-started/developer-mode-app)).
2. Paketieren und installieren:
   ```sh
   ares-package apps/webos
   ares-install io.flimmer.app_0.1.0_all.ipk
   ```
3. In der App die Server-Adresse eingeben, z. B. `192.168.178.20`. Der Port 8096 wird automatisch ergänzt.

### Passwort vergessen

Das Passwort lässt sich per Befehl neu setzen, auch bei laufendem Server (ohne `-user` ist der erste Admin gemeint):

```sh
./flimmer resetpw -data /pfad/zum/datenordner NEUES-PASSWORT          # ohne Docker
docker exec flimmer flimmer resetpw -data /data NEUES-PASSWORT        # Docker
```

## Tests

```sh
go test ./...          # Unit- und Tabellentests
scripts/smoke.sh       # E2E: erzeugt Testclips, startet den Server, prüft Direct Play, HLS und Untertitel
```

## Dateinamen

Flimmer erkennt Filme und Episoden am Dateinamen:

```
Filme/Blade Runner 2049 (2017).mkv
Serien/Dark/Staffel 1/Dark S01E01.mkv
Serien/Dark/Season 2/S02E03.mkv          # Serienname kommt aus dem Ordner
```

## Lizenz

[AGPL-3.0](LICENSE)
