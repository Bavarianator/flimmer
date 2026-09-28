# Architektur

Flimmer besteht aus einer Go-Binary, die ffmpeg/ffprobe als Unterprozess nutzt und die Web-UI eingebettet mitbringt. Die Web-UI ist eine einzige Oberfläche für alle Plattformen: Browser, webOS, Tizen sowie Android und iOS über Capacitor.

```
Medienordner ──► scan ──► probe (ffprobe + Keyframe-Index) ──► Bibliothek (Speicher + JSON-Cache)
                                                                     │
Client ──► api ──► playback.Decide(Media, Geräteprofil) ──► Plan ────┤
                                                                     ▼
                          Direct Play: Originaldatei per HTTP-Range
                          sonst:       transcode → HLS-Segmente (ffmpeg on demand)
```

## Pakete

| Paket | Aufgabe |
|---|---|
| `cmd/server` | Einstiegspunkt, Flags, verdrahtet alles |
| `internal/scan` | Durchsucht die Medienordner und erkennt Film oder Episode am Dateinamen. Probe-Ergebnisse werden anhand von Größe und mtime gecacht. |
| `internal/probe` | Liest per ffprobe Container, Streams, Sprachen und alle Keyframe-Zeitstempel |
| `internal/playback` | Die reine Funktion `Decide` ohne Seiteneffekte ermittelt Methode, Ampel und Untertitel-Formate. Abgesichert durch Tabellentests. |
| `internal/transcode` | Schneidet Segmente auf Keyframes und erzeugt die HLS-Playlist. Ein ffmpeg-Prozess pro Wiedergabe, gedrosselt und nach Leerlauf beendet. |
| `internal/api` | HTTP-API mit Router aus der Standardbibliothek (Go-1.22-Patterns) |
| `web` | UI, gebaut nach `web/dist` und per `go:embed` eingebettet |

## Wiedergabe-Entscheidung

| Methode | Wann | Ampel |
|---|---|---|
| Direct Play | Container, Video, Ton und Bitrate passen zum Gerät | 🟢 |
| Direct Stream | Nur der Container passt nicht → HLS, alles kopiert | 🟢 |
| Nur Ton transkodieren | Video passt, der Ton nicht (z. B. DTS/TrueHD → EAC3/AAC) | 🟡 |
| Transkodieren | Video passt nicht → H.264 | 🔴 |

Untertitel werden nie eingebrannt, weil das die häufigste Ursache für unnötiges Transkodieren ist:
- Text-Untertitel (SRT/ASS) wandelt der Server in WebVTT um.
- PGS zeichnet der Client selbst als Overlay.

## Warum Keyframe-Segmente

Der Scan speichert alle Keyframe-Zeitstempel. Daraus steht die HLS-Playlist sofort vollständig fest, auch wenn das Video nur kopiert wird. Spulen springt deshalb an jede Stelle, ohne dass ffmpeg vorher bis dorthin laufen muss. Die ersten drei Segmente sind kürzer (ca. 2 s), damit die Wiedergabe schnell startet.

## Geplant

- SQLite (CGO-frei) für Benutzer und Weiterschauen
- Geräteprofile in drei Schichten: Profil-Datenbank, API-Abfrage im Client, echter Probe-Test mit Testclips
- Hardware-Transcoding (VAAPI/QSV/NVENC/VideoToolbox)
- Fernzugriff: IPv6 bzw. UPnP, dazu ein Rendezvous-Dienst ohne Video-Traffic
- Gemeinsam schauen über SSE + POST
- Live-TV/DVR
