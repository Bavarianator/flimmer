# Changelog

## v0.1.0 – 28.09.2026 (interner Release)

Erste lauffähige Version. Server, Web-UI und TV-Starter laufen auf dem Heim-NAS, mehr dazu im [NAS-Testbericht](docs/testbericht-nas.md).

### Wiedergabe: „läuft garantiert“
- Für jeden Titel und jedes Gerät wird die Wiedergabe-Art gewählt: Direct Play, Remux (HLS), nur Ton umwandeln oder voll umwandeln. Eine Ampel zeigt vorher, wie gut ein Titel läuft.
- Der Keyframe-Index sorgt für eine sofort vollständige HLS-Playlist und präzises Spulen, auch wenn das Video nur kopiert wird.
- Hardware-Transcoding (VAAPI, QSV, NVENC, VideoToolbox, V4L2) wird per echtem Testencode erkannt. Ein Speed-Wächter schaltet auf 720p, statt ruckeln zu lassen.
- 4K/HDR:
  - Erkannt werden HDR10, HDR10+, HLG und Dolby Vision (Profil und Basisschicht).
  - DV 8.1 und 7 laufen als HDR10, DV 8.4 als HLG.
  - Auf SDR-Geräten wird per Tone-Mapping (hable) nach BT.709 umgerechnet, 4K dabei auf höchstens 1080p verkleinert.
- Untertitel werden nie eingebrannt: Text wird zu WebVTT, PGS zeichnet der Client selbst.
  - Sprachkette ger = deu = de.
  - Forced-Automatik: Volle Untertitel kommen nur, wenn man die Tonsprache nicht versteht.
- Ton:
  - Wahl der Tonspur und gemerkte Sprache pro Serie.
  - Stereo-Downmix mit lauter Sprache und Nachtmodus.
- Hintergrund-Optimierung: Nachts entsteht eine MP4-Version roter Titel, auf der GPU mit Rückfall auf Software.

### Server und Betrieb
- Eine Binary ohne Pflicht-Flags, SQLite ohne CGO, Einrichtung im Browser. Unter Windows, Linux und macOS wird ffmpeg bei Bedarf automatisch geladen.
- Benutzer mit argon2id, Profilauswahl, TV-Kopplung per Code, Fortschritt und „Weiterschauen“.
- Metadaten aus Kodi-NFO und TMDB, Bilder lokal skaliert, „Falsch erkannt?“.
- Autostart ohne root mit `flimmer install`, SSDP-Discovery, QR-Code, Diagnose, Backup, Update-Hinweis.
- Fernzugriff: IPv6/UPnP/PCP, Rendezvous-Dienst `flimmer-relay`, HTTPS per ACME, Einladungen.

### Clients
- Web-UI (Preact) für Browser, LG webOS und Samsung Tizen, per D-Pad bedienbar, getestet ab Chromium 53.
- Geräteerkennung in drei Schichten, zuletzt mit echten Probe-Clips.

### Messwerte (Celeron N3060)
- RAM im Leerlauf 11–15 MB.
- VAAPI-Transcode 5,4× Echtzeit, in Software 0,44×.
- Hintergrund-Optimierung von 1080p MPEG-2 mit 1,6× Echtzeit.

### Bekannte Grenzen
- Dolby Vision Profil 5 auf Geräten ohne DV: Die Farben können abweichen (dafür bräuchte es libplacebo).
- Tone-Mapping läuft in Software. 4K-HDR kann auf schwachen Servern deshalb nur die Hintergrund-Optimierung.
- Metadaten ohne TMDB-Key nur aus NFO und Dateiname. Der Release-Build braucht das Secret `TMDB_KEY`.
