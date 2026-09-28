# Testbericht: Flimmer auf dem Spiel-NAS

Stand 28.09.2026, Flimmer `d1cd832`. Deploy mit `scripts/deploy-nas.sh`.

## Umgebung

| | |
|---|---|
| Hardware | Intel Celeron N3060 (Braswell, 2 Kerne, 1,6 GHz), Intel HD 400, 3,2 GB RAM, davon ~600 MB frei, 3,2 GB im Swap |
| System | Ubuntu 26.04, Grundlast ohne Flimmer: Load 4–5 |
| Nebenan | Jellyfin auf Port 8096, weitere Container |
| Flimmer | Port 8097, alles unter `~/flimmer` (Binary, statisches ffmpeg von BtbN, Daten, VA-Treiber), ohne root |
| Bibliothek | 16 Titel (14 Filme, 2 Episoden), alle MP4 mit H.264 und AAC in SD (640×480 bis 720×576), 13,7 GB |
| Zugang | Admin `admin`. Das Passwort liegt nur lokal in `~/.config/flimmer-nas-admin` auf dem Entwicklungsrechner. |

## Ergebnisse

### Speicher und Scan

| Messung | Wert | Ziel |
|---|---|---|
| RSS im Leerlauf, ohne Anmeldung | **11–15 MB** | < 50 MB ✅ |
| RSS direkt nach der ersten Anmeldung (argon2id, 19 MiB) | 50–64 MB | |
| RSS 2 min danach (Go gibt den Speicher zurück) | **34 MB** | < 50 MB ✅ |
| Jellyfin zum Vergleich | 24 MB RSS + 64 MB im Swap | |
| Erster Scan (Header-Probe, 16 Dateien) | 18 s | |
| Keyframe-Index (im Hintergrund, pro Film) | 7–14 s, zusammen ~2,5 min | |
| `/api/library` (16 Titel) | 3–4 ms | |

### Ampel

Mit dem LG-webOS-Profil und mit dem Chrome-Profil: **16 × 🟢 Direct Play**. Die Bibliothek besteht nur aus H.264/AAC-MP4, deshalb muss nichts umgewandelt werden. Range-Anfragen liefern 206 mit rund 80 MB/s.

### Wiedergabe-Pfade (per Testprofil erzwungen)

| Pfad | Start | Echtzeit-Faktor | Spulen in die Mitte | ffmpeg-RSS |
|---|---|---|---|---|
| Remux (Direct Stream) | 0,5 s | 177× | 0,7 s | 58 MB |
| Nur Ton (Transcode Audio) | 3,6 s | 4,6× | 2,7 s | 136 MB |
| Voll-Transcode, Software (libx264) | 7,4 s | **0,44×** ❌ | 26 s | 231 MB |
| Voll-Transcode, **VAAPI** | 2,2 s | **5,4×** ✅ | 3,1 s | 75 MB |

Encoder-Test mit 1080p-H.264 (3-s-Clip):
- Software: 0,27×
- VAAPI: 2,4× im Dauerbetrieb, 3,1× mit Verkleinern auf 720p

### Last: drei Streams gleichzeitig (60 s)

| Stream | Ergebnis |
|---|---|
| Direct Play (Range-Häppchen wie ein Player) | 27,5 Mbit/s, keine Fehler |
| Remux | 67× Echtzeit |
| Nur Ton | 13× Echtzeit |

Dabei belegte Flimmer 20 MB und ffmpeg 210 MB. Die Systemlast lag bei 10 auf 2 Kernen, das NAS ist also auch ohne Flimmer schon stark ausgelastet. Trotzdem lief jeder Stream deutlich schneller als Echtzeit.

### Metadaten

Nicht messbar: Es gibt keine NFO-Dateien und keinen TMDB-Schlüssel (der Projekt-Key kommt erst mit dem Release-Build). Alle Titel haben deshalb `source: filename`. Die Dateinamen werden sauber erkannt, Titel und Jahr stimmen bei 16/16.

## Gefundene und behobene Fehler

| Befund | Behoben in |
|---|---|
| Nach dem Spulen passten die Segmentgrenzen nicht zur Playlist (6-s-Segment enthielt 2 s), Stocken | `f3e9246` (Streaming) |
| Der Speed-Wächter skalierte SD-Quellen beim „Wechsel auf 720p“ hoch | `55d3fc6` |
| Zielton war AAC, auch wenn das Gerät kein AAC kann | `ff52257` |
| hwaccel maß VAAPI mit 0,5×, weil die GPU-Initialisierung im kurzen Testclip mitzählte | `d1cd832` |
| Der Smoke-Test war seit Einführung der Anmeldung rot (401) | `cd76c84` |
| Login nur per Benutzer-ID, nicht per Name | `69789f6` (Setup) |

## VAAPI ohne root

Dem NAS fehlen libva und der Intel-Treiber. Das statische BtbN-ffmpeg lädt `libva-drm.so.2` zur Laufzeit und bricht ohne ab. Die Lösung ohne Systemänderung:

1. Die Pakete `libva2`, `libva-drm2` und `i965-va-driver-shaders` per `apt-get download` als Benutzer holen und nach `~/flimmer/va` entpacken.
2. Flimmer mit `LD_LIBRARY_PATH` und `LIBVA_DRIVERS_PATH` starten.
3. Dazu kommt die Gruppe `render` für den Nutzer `mo`.

Das Skript erledigt Schritt 1 und 2 automatisch, wenn das System keinen eigenen Treiber hat. Wichtig ist das Paket **`-shaders`**: Mit dem normalen `i965-va-driver` bricht `h264_vaapi` auf Braswell mit `intel_enc_hw_context_init: Assertion 'encoder_context->mfc_context' failed` ab.

Ergebnis: Das Software-Transcoding ist rund **12× langsamer** als VAAPI. Echtzeit-Transcoding ist auf diesem NAS nur mit VAAPI möglich.

## Offen

- **Ampel beim Start zu pessimistisch.** `hwaccel.Detect` läuft parallel zum Start-Scan und misst dadurch 0,9× statt 2,4×. Transcodes erscheinen deshalb Rot statt Gelb. Die Messung nach dem Scan zu verschieben ist bei der Setup-Session angefragt.
- **LG-TV-Test (10 min mit Spulen, Vergleich mit Jellyfin)** braucht den Nutzer am Gerät. Weil die Bibliothek komplett Direct Play ist, sind hier keine Unterschiede zu erwarten. Jellyfin stockt typischerweise beim Transcoding, und das entfällt.
- **Metadaten-Trefferquote** erst mit TMDB-Schlüssel (Repo-Secret `TMDB_KEY` bzw. `FLIMMER_TMDB_KEY`).
- **4K/HDR:** Auf dem Celeron ist 4K-Tone-Mapping in Echtzeit ausgeschlossen, schon 1080p in Software schafft nur 0,27×. Solche Titel gehören in die nächtliche Hintergrund-Optimierung (`internal/optimize`).
- **VAAPI-Treiber im Produkt.** Das Deploy-Skript löst es für Debian und Ubuntu. Für alle anderen wäre ein ffmpeg-Build mit mitgelieferten Treibern nötig, wie ihn jellyfin-ffmpeg nutzt.

## Reproduzieren

```sh
scripts/deploy-nas.sh              # baut HEAD, deployt, startet (Port 8097)
scripts/deploy-nas.sh stop|remove  # stoppen bzw. ~/flimmer auf dem NAS entfernen
```

## Nachtrag: Hintergrund-Optimierung mit VAAPI

Testclip `~/flimmer/testmedia/Rot (2025).ts`, außerhalb der Jellyfin-Bibliothek. Es ist eine DVB-typische Aufnahme: 60 s MPEG-2 1080p mit 12 Mbit/s und MP2-Ton. Für einen H.264/AAC-Browser ist das 🔴.

| Lauf | Dauer | Echtzeit-Faktor | Ergebnis |
|---|---|---|---|
| VAAPI (hwaccel.Detect: 1,89×) | 37 s | 1,6× | MP4 H.264 1080p, danach 🟢 Direct Play |
| GPU absichtlich kaputt (Render-Node existiert nicht) | 194 s | 0,31× | Rückfall auf Software greift, ebenfalls 🟢 |

Die Dauer enthält Start, Probe und `nice`/`ionice`. Mit VAAPI schafft das NAS nachts rund 1,6 Stunden Film pro Stunde, in Software nur 0,3.

Nebenbefund: `Status().Done` blieb nach dem fertigen Job 0, obwohl `Lookup` die Version findet. Das ist an vault-b8 gemeldet.
