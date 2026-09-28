# UI-Abnahme (Schritt 5)

Stand: 2026-09-28. Getestet gegen einen lokalen Server mit der Testmatrix (MP4/H.264, MKV/HEVC10+DTS,
MKV/H.264+AC3, MKV mit zwei Tonspuren + SRT).

## Chromium 53 (ältestes Ziel: webOS 4)

Docker-Image `selenium/standalone-chrome:2.53.1` (Chrome 53.0.2785.143), gesteuert per WebDriver,
User-Agent eines LG-TVs, Bedienung ausschließlich über Tasten (Pfeile, Enter, Escape = Zurück).

| Ablauf | Ergebnis |
|---|---|
| Profilauswahl hat Fokus, Enter → Passwort, Enter → angemeldet | ✅ |
| Startseite: Reihen (Neu hinzugefügt, Filme, Serien), Fokus auf erster Karte | ✅ |
| ↓ → wechselt Reihe/Karte, Enter öffnet Serie | ✅ |
| Serienseite: Fokus auf „Abspielen“, Enter startet | ✅ |
| Wiedergabe über hls.js (Chrome 53 hat kein natives HLS, kein AC3 → Server wandelt nur Ton) | ✅ läuft |
| ↑ öffnet Ton/Untertitel-Menü, → ↓ Enter schaltet Untertitel ein | ✅ |
| ↑ ↓ Enter wechselt auf englische Tonspur, Wiedergabe läuft an derselben Stelle weiter | ✅ |
| Fortsetzen an serverseitig gespeicherter Position | ✅ |
| Escape (= Zurück-Taste) führt zurück zur Startseite | ✅ |

Nachstellen: `docker run -d --network host --shm-size=1g selenium/standalone-chrome:2.53.1`,
danach WebDriver auf `http://localhost:4444/wd/hub` (Skripte liegen nicht im Repo).

## Gefundene und behobene Fehler

- Enter im Passwortfeld wurde von der D-Pad-Navigation verschluckt (Formular nie abgeschickt).
  Fix: Enter/←/→ in Eingabefeldern werden vor der Navigation abgefangen (`main.tsx`).
- Kein Anfangsfokus: Preact führt Eltern-Effekte vor Kind-Effekten aus → `focusSoon()` (`ui.tsx`).
- Untertitel-Cues nach Tonwechsel doppelt → `<track>` wird pro Plan neu eingehängt.

## Offen

- **webOS-Emulator:** nicht getestet – braucht webOS Studio + VirtualBox, auf dieser Maschine nicht
  vorhanden. Nächster Schritt: echter LG-TV im Developer Mode (`apps/webos`).
- Samsung Tizen: kommt mit Schritt 9.
