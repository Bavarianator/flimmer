# Flimmer

Ein schlanker Open-Source-Medienserver für Filme und Serien, als Alternative zu Jellyfin und Plex.

**Wiedergabe soll auf jedem Gerät zuverlässig laufen.** Flimmer entscheidet pro Titel und Gerät, ob die Datei direkt abgespielt, nur umverpackt oder transkodiert wird. Ziel ist, dass so wenig wie möglich neu kodiert wird. Deshalb werden Untertitel nie eingebrannt, Segmente liegen auf echten Keyframes, und wenn nur der Ton nicht passt, wird nur der Ton umgewandelt. Eine Ampel zeigt vorher an, wie gut ein Titel auf dem Gerät läuft.

Weitere Ziele:
- **Wenig Ressourcen:** eine einzige Binary ohne Abhängigkeiten außer ffmpeg. Läuft auch auf dem Raspberry Pi.
- **Alle Geräte:** eine Web-UI für Browser, LG webOS, Samsung Tizen, Android (TV) und iOS.
- **Gemeinsam schauen und einfaches Teilen** sind geplant (siehe [Architektur](docs/architektur.md)).

> Status: früher Prototyp. Benutzer, Metadaten (TMDB/NFO) und die meisten Apps fehlen noch. Für LG-TVs gibt es einen webOS-Starter.

## Schnellstart

### Docker

```sh
docker run -d --name flimmer -p 8096:8096 \
  -v /pfad/zu/filmen:/media:ro \
  -v flimmer-data:/data \
  ghcr.io/flimmer-media/flimmer
```

Selbst bauen (Multi-Arch):

```sh
docker buildx build -f deploy/Dockerfile --platform linux/amd64,linux/arm64,linux/arm/v7 -t flimmer .
```

### Binary

Voraussetzung: `ffmpeg` und `ffprobe` im `PATH`.

```sh
flimmer -media /pfad/zu/filmen,/pfad/zu/serien -data ~/.cache/flimmer
```

Danach im Browser `http://localhost:8096` öffnen. Als Dienst unter Linux: siehe [`deploy/flimmer.service`](deploy/flimmer.service).

| Flag | Standard | Bedeutung |
|---|---|---|
| `-media` | – (Pflicht) | Medienordner, mehrere mit Komma getrennt |
| `-data` | Benutzer-Cache-Ordner/flimmer | Cache und Transcode-Zwischendateien |
| `-addr` | `:8096` | Adresse, auf der der Server lauscht |

### Aus dem Quellcode

Du brauchst Go 1.27+, Node 22 und ffmpeg.

```sh
(cd web && npm ci && npm run build)   # UI nach web/dist bauen, wird in die Binary eingebettet
go run ./cmd/server -media /pfad/zu/filmen
go test ./...
```

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
