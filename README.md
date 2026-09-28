# Flimmer

Ein schlanker Open-Source-Medienserver für Filme und Serien, als Alternative zu Jellyfin und Plex.

**Wiedergabe soll auf jedem Gerät zuverlässig laufen.** Flimmer entscheidet pro Titel und Gerät, ob die Datei direkt abgespielt, nur umverpackt oder transkodiert wird. Ziel ist, dass so wenig wie möglich neu kodiert wird. Deshalb werden Untertitel nie eingebrannt, Segmente liegen auf echten Keyframes, und wenn nur der Ton nicht passt, wird nur der Ton umgewandelt. Eine Ampel zeigt vorher an, wie gut ein Titel auf dem Gerät läuft.

Weitere Ziele:
- **Wenig Ressourcen:** eine einzige Binary ohne Abhängigkeiten außer ffmpeg. Läuft auch auf dem Raspberry Pi.
- **Alle Geräte:** eine Web-UI für Browser, LG webOS, Samsung Tizen, Android (TV) und iOS.
- **Gemeinsam schauen und einfaches Teilen** sind geplant (siehe [Architektur](docs/architektur.md)).

> Status: frühe Version. Benutzer, Metadaten (NFO/TMDB) und Hardware-Transcoding sind da; Apps außer dem LG-Starter fehlen noch.

## Schnellstart in 3 Schritten

1. **Herunterladen:** Die passende Datei aus den [Releases](https://github.com/flimmer-media/flimmer/releases/latest) holen und entpacken.
   - Windows: `…-windows-amd64.zip`
   - Mac mit Apple-Chip: `…-darwin-arm64.tar.gz`
   - Linux-PC/NAS: `…-linux-amd64.tar.gz`
   - Raspberry Pi 4/5: `…-linux-arm64.tar.gz`
   - Raspberry Pi 2/3: `…-linux-armv7.tar.gz`
2. **Starten:** `flimmer` doppelklicken bzw. im Terminal `./flimmer` ausführen.
   - Unter Windows wird ffmpeg bei Bedarf automatisch geladen.
   - Unter Linux und macOS muss ffmpeg installiert sein, z. B. mit `sudo apt install ffmpeg` bzw. `brew install ffmpeg`.
3. **Browser:** Die Einrichtung öffnet sich unter `http://localhost:8096`. Dort Medienordner wählen, fertig. TV und Handy erreichen den Server unter der angezeigten Adresse oder per QR-Code.

Automatisch mit dem Rechner starten: `flimmer install`, wieder entfernen mit `flimmer uninstall`. Dafür sind keine Admin-Rechte nötig. Unter Linux wird dabei eine systemd-User-Unit angelegt, unter macOS ein LaunchAgent, unter Windows ein Eintrag im Autostart.

### Docker

```sh
docker run -d --name flimmer -p 8096:8096 \
  -v /pfad/zu/filmen:/media:ro \
  -v flimmer-data:/data \
  ghcr.io/flimmer-media/flimmer
```

Oder mit Compose: [`deploy/compose.yml`](deploy/compose.yml) anpassen und dann `docker compose -f deploy/compose.yml up -d` ausführen. `:latest` ist das letzte Release, `:edge` der aktuelle Stand von `main`.

### Als Systemdienst (Linux-Server)

Für einen eigenen Benutzer und Härtung gibt es [`deploy/flimmer.service`](deploy/flimmer.service). Die Anleitung steht in der Datei.

| Flag | Standard | Bedeutung |
|---|---|---|
| `-media` | – | Medienordner, mehrere mit Komma getrennt. Alternativ in der Einrichtung wählen. |
| `-data` | Benutzer-Cache-Ordner/flimmer | Einstellungen, Cache, Poster |
| `-addr` | `:8096` | Adresse, auf der der Server lauscht |

### Aus dem Quellcode

Du brauchst Go 1.27+, Node 22 und ffmpeg.

```sh
(cd web && npm ci && npm run build)   # UI nach web/dist bauen, wird in die Binary eingebettet
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
