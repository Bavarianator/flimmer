// Einstellungen (screens/14), Route /einstellungen[/{bereich}].
// Links die Bereiche, rechts der Inhalt. „Dieses Gerät“ darf jeder, alles andere nur Admins (Routen adminOnly).
// Formen der Routen wie in internal/api: /api/settings, /api/users, /api/setup/dirs, /api/settings/review,
// /api/remote, /api/invites, /api/settings/optimize, /api/settings/backup|restore, /api/diagnostics.
import type { ComponentChildren } from 'preact'
import { useEffect, useRef, useState } from 'preact/hooks'
import { api, geraeteTest, ich, type Nutzer } from '../lib/api'
import { istTV, setThema, thema, type Thema } from '../lib/device'
import { fokusBald, Gruppe, useFokus } from '../lib/focus'
import { ergaenze, setSprache, sprache, t } from '../lib/i18n'
import { go, teile } from '../lib/router'
import { Button, Chip } from '../components/Button'
import { Icon } from '../components/Icon'
import { AmpelPunkt } from '../components/Karte'
import { Seite } from '../components/Seite'
import { Fehler } from '../components/Zustand'
import { loadProbe } from '../probe'
import { setVorlieben, vorlieben } from '../player/vorlieben'
import '../player/spuren' // Texte player.* (Nachtmodus, Methoden) für „Dieses Gerät“ und die Diagnose
import '../player/screens.css'

ergaenze(
  {
    'einst.titel': 'Einstellungen',
    'einst.geraet': 'Dieses Gerät',
    'einst.profile': 'Profile',
    'einst.bibliothek': 'Bibliothek',
    'einst.server': 'Server',
    'einst.fernzugriff': 'Fernzugriff',
    'einst.einladungen': 'Einladungen',
    'einst.nachts': 'Nachts vorbereiten',
    'einst.sicherung': 'Sicherung',
    'einst.diagnose': 'Diagnose',
    'einst.gespeichert': 'Gespeichert',
    'einst.speichern': 'Speichern',
    'einst.nuradmin': 'Diese Einstellungen darf nur ein Admin ändern.',
    // Dieses Gerät
    'einst.thema': 'Erscheinungsbild',
    'einst.thema.kino': 'Dunkel',
    'einst.thema.hell': 'Hell',
    'einst.thema.system': 'Wie das System',
    'einst.sprache': 'Sprache der Oberfläche',
    'einst.tonsprache': 'Bevorzugte Tonsprache',
    'einst.tonsprache.text': 'Flimmer wählt diese Tonspur, wenn es sie gibt',
    'einst.ut': 'Untertitel',
    'einst.ut.auto': 'Automatisch',
    'einst.ut.auto.text': 'Nur wenn du die Sprache nicht verstehst, sonst nur erzwungene',
    'einst.ut.always': 'Immer',
    'einst.ut.off': 'Nur erzwungene',
    'einst.nacht.text': 'Dialoge klarer, Explosionen leiser – für alle Titel auf diesem Profil',
    'einst.test': 'Gerät neu testen',
    'einst.test.text': 'Spielt kurze Clips ab und misst, was dieses Gerät wirklich kann',
    'einst.test.laeuft': 'Wird getestet …',
    'einst.test.fertig': 'Fertig: {n} Formate laufen direkt',
    'einst.koppeln.text': 'Einen Fernseher mit dem Code auf seinem Bildschirm anmelden',
    'einst.profil.text': 'Anderes Profil auf diesem Gerät',
    'einst.abmelden': 'Abmelden',
    // Profile
    'einst.neu': 'Neues Profil',
    'einst.name': 'Name',
    'einst.pin': 'PIN oder Passwort',
    'einst.pin.neu': 'Neue PIN',
    'einst.pin.aendern': 'PIN ändern',
    'einst.pin.weg': 'PIN entfernen',
    'einst.admin': 'Admin',
    'einst.mitpin': 'mit PIN',
    'einst.ohnepin': 'ohne PIN (nur im Heimnetz)',
    'einst.loeschen': 'Löschen',
    'einst.loeschen.sicher': 'Wirklich löschen?',
    'einst.anlegen': 'Anlegen',
    // Bibliothek
    'einst.ordner': 'Medienordner',
    'einst.ordner.leer': 'Noch kein Ordner. Wähle unten einen aus.',
    'einst.ordner.hinzu': 'Ordner hinzufügen',
    'einst.ordner.hier': 'Diesen Ordner nehmen',
    'einst.ordner.hoch': 'Eine Ebene höher',
    'einst.entfernen': 'Entfernen',
    'einst.scan': 'Jetzt neu einlesen',
    'einst.scan.laeuft': 'Liest ein … {n} Titel gefunden',
    'einst.scan.fertig': '{n} Titel in der Bibliothek',
    'einst.tmdb': 'TMDB-Schlüssel',
    'einst.tmdb.text': 'Für Poster und Beschreibungen. Ohne eigenen Schlüssel gilt der eingebaute, falls vorhanden.',
    'einst.tmdb.gesetzt': 'Eigener Schlüssel ist gesetzt',
    'einst.pruefen': 'Bitte prüfen',
    'einst.pruefen.text': 'Diese Titel hat Flimmer nicht sicher erkannt',
    'einst.pruefen.leer': 'Alles erkannt.',
    'einst.suchen': 'Suchen',
    'einst.nehmen': 'Übernehmen',
    // Server
    'einst.servername': 'Name des Servers',
    'einst.adresse': 'Adresse im Heimnetz',
    'einst.version': 'Version',
    'einst.update': 'Nach Updates suchen',
    'einst.update.da': 'Version {v} ist erschienen',
    'einst.ffmpeg': 'ffmpeg',
    'einst.ffmpeg.ok': 'Installiert – Umwandeln ist möglich',
    'einst.ffmpeg.fehlt': 'Fehlt – ohne ffmpeg läuft nur, was das Gerät direkt kann',
    'einst.ffmpeg.laden': 'ffmpeg herunterladen',
    'einst.ffmpeg.laedt': 'Wird geladen … {n} %',
    'einst.serversprache': 'Sprache für Titel und Beschreibungen',
    // Fernzugriff
    'einst.remote': 'Von unterwegs schauen',
    'einst.remote.text': 'Filme laufen direkt von hier zu dir, über keinen fremden Server',
    'einst.remote.nicht': 'Fernzugriff ist in diesem Build nicht verfügbar.',
    'einst.remote.pruefen': 'Erneut prüfen',
    'einst.remote.prueft': 'Prüft … (bis 30 s)',
    'einst.remote.code': 'Code für die App',
    'einst.remote.code.text': 'Die Android-App verbindet sich damit auch von unterwegs',
    'einst.remote.code.knopf': 'Code erzeugen',
    'einst.remote.code.gilt': 'Gilt bis {zeit} Uhr',
    'einst.remote.erreichbar': 'Erreichbar',
    'einst.remote.nichterreichbar': 'Nicht erreichbar',
    // Einladungen
    'einst.einl.leer': 'Noch keine Einladungen.',
    'einst.einl.nur': 'Nur {was}',
    'einst.einl.alles': 'Alle Bibliotheken',
    'einst.einl.bis': 'läuft ab am {datum}',
    'einst.einl.genutzt': '{n} von {max} genutzt',
    'einst.einl.genutzt.frei': '{n}-mal genutzt',
    'einst.einl.widerrufen': 'Widerrufen',
    'einst.einl.neu': 'Neue Einladung',
    'einst.einl.neu.text': 'Link und QR-Code, befristet, mit Auswahl der Bibliotheken',
    'einst.einl.fuer': 'Für wen?',
    'einst.einl.tage': 'Gültig (Tage)',
    'einst.einl.max': 'Höchstens (0 = unbegrenzt)',
    'einst.einl.erstellen': 'Erstellen',
    'einst.einl.link': 'Diesen Link weitergeben:',
    // Nachts vorbereiten
    'einst.opt': 'Titel nachts vorbereiten',
    'einst.opt.text': 'Wandelt schwierige Titel vorab um, damit sie abends direkt laufen',
    'einst.opt.von': 'Von (Uhr)',
    'einst.opt.bis': 'Bis (Uhr)',
    'einst.opt.frei': 'Mindestens frei (GB)',
    'einst.opt.stand': '{done} fertig · {pending} warten · Fenster {fenster}',
    'einst.opt.jetzt': 'Gerade: {titel} ({n} %)',
    'einst.opt.wartet': 'Startet, sobald der Server bereit ist.',
    // Sicherung
    'einst.backup.laden': 'Sicherung herunterladen',
    'einst.backup.laden.text': 'Profile, Fortschritt und Einstellungen als eine Datei',
    'einst.backup.stand': 'Letzte automatische Sicherung: {zeit}',
    'einst.backup.einspielen': 'Sicherung einspielen',
    'einst.backup.einspielen.text': 'Prüft Version und Zustand, bevor etwas ersetzt wird',
    'einst.backup.datei': 'Datei wählen',
    'einst.backup.ok': 'Sicherung eingespielt',
    // Diagnose
    'einst.diag.jetzt': 'Jetzt',
    'einst.diag.umwandeln': 'Umwandeln ({hw})',
    'einst.diag.streams': 'Streams',
    'einst.diag.platz': 'frei im Cache',
    'einst.diag.laufend': 'Laufende Wiedergaben',
    'einst.diag.keine': 'Gerade schaut niemand.',
    'einst.diag.zuletzt': 'Zuletzt',
    'einst.diag.db': 'Datenbank',
    'einst.diag.db.text': 'Letzte Prüfung {zeit} · {ergebnis}',
    'einst.diag.log': 'Protokoll anzeigen',
    'einst.diag.log.text': 'Die letzten Zeilen des Servers',
    'einst.diag.kopieren': 'Diagnose kopieren',
    'einst.diag.kopieren.text': 'Für einen Fehlerbericht: Status und Protokoll',
    'einst.diag.kopiert': 'In die Zwischenablage kopiert',
    'einst.anzeigen': 'Anzeigen',
    'einst.ausblenden': 'Ausblenden',
    'einst.nie': 'noch nie',
  },
  {
    'einst.titel': 'Settings',
    'einst.geraet': 'This device',
    'einst.profile': 'Profiles',
    'einst.bibliothek': 'Library',
    'einst.server': 'Server',
    'einst.fernzugriff': 'Remote access',
    'einst.einladungen': 'Invitations',
    'einst.nachts': 'Prepare overnight',
    'einst.sicherung': 'Backup',
    'einst.diagnose': 'Diagnostics',
    'einst.gespeichert': 'Saved',
    'einst.speichern': 'Save',
    'einst.nuradmin': 'Only an admin can change these settings.',
    'einst.thema': 'Appearance',
    'einst.thema.kino': 'Dark',
    'einst.thema.hell': 'Light',
    'einst.thema.system': 'Match system',
    'einst.sprache': 'Interface language',
    'einst.tonsprache': 'Preferred audio language',
    'einst.tonsprache.text': 'Flimmer picks this audio track when available',
    'einst.ut': 'Subtitles',
    'einst.ut.auto': 'Automatic',
    'einst.ut.auto.text': 'Only when you do not understand the language, otherwise forced only',
    'einst.ut.always': 'Always',
    'einst.ut.off': 'Forced only',
    'einst.nacht.text': 'Clearer dialogue, quieter explosions – for all titles on this profile',
    'einst.test': 'Test device again',
    'einst.test.text': 'Plays short clips and measures what this device can really do',
    'einst.test.laeuft': 'Testing …',
    'einst.test.fertig': 'Done: {n} formats play directly',
    'einst.koppeln.text': 'Sign in a TV with the code on its screen',
    'einst.profil.text': 'Another profile on this device',
    'einst.abmelden': 'Sign out',
    'einst.neu': 'New profile',
    'einst.name': 'Name',
    'einst.pin': 'PIN or password',
    'einst.pin.neu': 'New PIN',
    'einst.pin.aendern': 'Change PIN',
    'einst.pin.weg': 'Remove PIN',
    'einst.mitpin': 'with PIN',
    'einst.ohnepin': 'without PIN (home network only)',
    'einst.loeschen': 'Delete',
    'einst.loeschen.sicher': 'Really delete?',
    'einst.anlegen': 'Create',
    'einst.ordner': 'Media folders',
    'einst.ordner.leer': 'No folder yet. Pick one below.',
    'einst.ordner.hinzu': 'Add folder',
    'einst.ordner.hier': 'Use this folder',
    'einst.ordner.hoch': 'Up one level',
    'einst.entfernen': 'Remove',
    'einst.scan': 'Rescan now',
    'einst.scan.laeuft': 'Scanning … {n} titles found',
    'einst.scan.fertig': '{n} titles in the library',
    'einst.tmdb': 'TMDB key',
    'einst.tmdb.text': 'For posters and descriptions. Without your own key the built-in one is used, if any.',
    'einst.tmdb.gesetzt': 'Your own key is set',
    'einst.pruefen': 'Please check',
    'einst.pruefen.text': 'Flimmer was not sure about these titles',
    'einst.pruefen.leer': 'Everything recognised.',
    'einst.suchen': 'Search',
    'einst.nehmen': 'Use',
    'einst.servername': 'Server name',
    'einst.adresse': 'Home network address',
    'einst.version': 'Version',
    'einst.update': 'Check for updates',
    'einst.update.da': 'Version {v} is available',
    'einst.ffmpeg.ok': 'Installed – conversion is possible',
    'einst.ffmpeg.fehlt': 'Missing – without ffmpeg only what the device plays directly works',
    'einst.ffmpeg.laden': 'Download ffmpeg',
    'einst.ffmpeg.laedt': 'Downloading … {n} %',
    'einst.serversprache': 'Language for titles and descriptions',
    'einst.remote': 'Watch on the go',
    'einst.remote.text': 'Movies stream directly from here to you, through no third-party server',
    'einst.remote.nicht': 'Remote access is not available in this build.',
    'einst.remote.pruefen': 'Check again',
    'einst.remote.prueft': 'Checking … (up to 30 s)',
    'einst.remote.code': 'Code for the app',
    'einst.remote.code.text': 'The Android app uses it to connect on the go',
    'einst.remote.code.knopf': 'Create code',
    'einst.remote.code.gilt': 'Valid until {zeit}',
    'einst.remote.erreichbar': 'Reachable',
    'einst.remote.nichterreichbar': 'Not reachable',
    'einst.einl.leer': 'No invitations yet.',
    'einst.einl.nur': 'Only {was}',
    'einst.einl.alles': 'All libraries',
    'einst.einl.bis': 'expires {datum}',
    'einst.einl.genutzt': '{n} of {max} used',
    'einst.einl.genutzt.frei': 'used {n} times',
    'einst.einl.widerrufen': 'Revoke',
    'einst.einl.neu': 'New invitation',
    'einst.einl.neu.text': 'Link and QR code, time-limited, with a choice of libraries',
    'einst.einl.fuer': 'For whom?',
    'einst.einl.tage': 'Valid (days)',
    'einst.einl.max': 'At most (0 = unlimited)',
    'einst.einl.erstellen': 'Create',
    'einst.einl.link': 'Share this link:',
    'einst.opt': 'Prepare titles overnight',
    'einst.opt.text': 'Converts difficult titles in advance so they play directly in the evening',
    'einst.opt.von': 'From (hour)',
    'einst.opt.bis': 'To (hour)',
    'einst.opt.frei': 'Keep free (GB)',
    'einst.opt.stand': '{done} done · {pending} waiting · window {fenster}',
    'einst.opt.jetzt': 'Now: {titel} ({n} %)',
    'einst.opt.wartet': 'Starts as soon as the server is ready.',
    'einst.backup.laden': 'Download backup',
    'einst.backup.laden.text': 'Profiles, progress and settings in one file',
    'einst.backup.stand': 'Last automatic backup: {zeit}',
    'einst.backup.einspielen': 'Restore backup',
    'einst.backup.einspielen.text': 'Checks version and integrity before replacing anything',
    'einst.backup.datei': 'Choose file',
    'einst.backup.ok': 'Backup restored',
    'einst.diag.jetzt': 'Now',
    'einst.diag.umwandeln': 'Conversion ({hw})',
    'einst.diag.platz': 'free in cache',
    'einst.diag.laufend': 'Current playback',
    'einst.diag.keine': 'Nobody is watching right now.',
    'einst.diag.zuletzt': 'Recently',
    'einst.diag.db': 'Database',
    'einst.diag.db.text': 'Last check {zeit} · {ergebnis}',
    'einst.diag.log': 'Show log',
    'einst.diag.log.text': 'The last lines of the server',
    'einst.diag.kopieren': 'Copy diagnostics',
    'einst.diag.kopieren.text': 'For a bug report: status and log',
    'einst.diag.kopiert': 'Copied to the clipboard',
    'einst.anzeigen': 'Show',
    'einst.ausblenden': 'Hide',
    'einst.nie': 'never',
  },
)

type Bereich = 'geraet' | 'profile' | 'bibliothek' | 'server' | 'fernzugriff' | 'einladungen' | 'nachts' | 'sicherung' | 'diagnose'
const BEREICHE: { id: Bereich; icon: Parameters<typeof Icon>[0]['name']; admin: boolean }[] = [
  { id: 'geraet', icon: 'fernseher', admin: false },
  { id: 'profile', icon: 'profil', admin: true },
  { id: 'bibliothek', icon: 'film', admin: true },
  { id: 'server', icon: 'einstellungen', admin: true },
  { id: 'fernzugriff', icon: 'start', admin: true },
  { id: 'einladungen', icon: 'mehr', admin: true },
  { id: 'nachts', icon: 'neustart', admin: true },
  { id: 'sicherung', icon: 'haken', admin: true },
  { id: 'diagnose', icon: 'info', admin: true },
]

interface Settings {
  serverName: string
  language: string
  dirs: string[]
  tmdbKey: boolean
  ffmpeg: { ok: boolean; canDownload: boolean; hint: string; download: { running: boolean; percent: number; error?: string } }
  lanUrl: string
  updateCheck: boolean
  remote: boolean
  optimize: { off: boolean; from: number; to: number; minFreeGB: number }
  remoteAvailable: boolean
  update: { version: string; url: string } | null
  version: string
}

// ---------- kleine Bausteine ----------

let toastSetzen: (s: string) => void = () => {}
function melde(s: string) {
  toastSetzen(s)
}
function fehlerText(e: unknown) {
  const m = String((e && (e as Error).message) || e)
  return m.replace(/^\d{3} /, '')
}

function datum(s: string | number) {
  const d = new Date(s)
  if (!d.getTime() || d.getFullYear() < 2000) return t('einst.nie')
  const zwei = (n: number) => (n < 10 ? '0' : '') + n
  return zwei(d.getDate()) + '.' + zwei(d.getMonth() + 1) + '.' + d.getFullYear() + ', ' + d.getHours() + ':' + zwei(d.getMinutes())
}

function groesse(b: number) {
  if (b > 1e9) return (b / 1e9).toFixed(1).replace('.', ',') + ' GB'
  return Math.max(0.1, b / 1e6).toFixed(1).replace('.', ',') + ' MB'
}

// Eine Zeile (.fl-zeile): Icon, Titel mit Unterzeile, rechts Wert, Schalter oder Knopf. Mit onPress ist die ganze Zeile bedienbar.
function Zeile(p: { fk?: string; icon?: Parameters<typeof Icon>[0]['name']; titel: string; unter?: string; schalter?: boolean; rechts?: ComponentChildren; onPress?: () => void }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: p.fk, onPress: p.onPress, aus: !p.onPress })
  const inhalt = (
    <>
      {p.icon && <Icon name={p.icon} class="fl-icon" />}
      <span class="text">
        <b>{p.titel}</b>
        {p.unter && <small>{p.unter}</small>}
      </span>
      {p.schalter !== undefined && (
        <span class={'fl-schalter' + (p.schalter ? ' an' : '')} role="switch" aria-checked={p.schalter}>
          <i />
        </span>
      )}
      {p.rechts}
    </>
  )
  if (!p.onPress) return <div class="fl-zeile">{inhalt}</div>
  return (
    <button ref={f.ref} {...f.dom} type="button" class={'fl-zeile einst-zeile-knopf' + (f.fokus ? ' ist-fokus' : '')}>
      {inhalt}
    </button>
  )
}

// Eingabefeld, das auch per D-Pad erreichbar ist: OK setzt den echten Fokus (öffnet die TV-Tastatur).
function Feld(p: { fk: string; wert: string; setWert: (w: string) => void; label: string; typ?: string; breit?: boolean; onEnter?: () => void }) {
  const input = useRef<HTMLInputElement>(null)
  const f = useFokus<HTMLLabelElement>({ fokusKey: p.fk, onPress: () => input.current && input.current.focus() })
  return (
    <label ref={f.ref} class={'fl-feld einst-feld' + (p.breit ? ' einst-feld-breit' : '') + (f.fokus ? ' ist-fokus' : '')}>
      <span class="nur-sr">{p.label}</span>
      <input
        ref={input}
        class="wachse"
        type={p.typ || 'text'}
        placeholder={p.label}
        value={p.wert}
        onInput={(e) => p.setWert((e.target as HTMLInputElement).value)}
        onKeyDown={(e) => e.keyCode === 13 && p.onEnter && p.onEnter()}
        onFocus={() => !f.fokus && f.fokusSelbst()}
      />
    </label>
  )
}

function Gruppenkopf({ titel }: { titel: string }) {
  return <h3>{titel}</h3>
}

function useLaden<T>(url: string, abh: unknown[] = []) {
  const [daten, setDaten] = useState<T | null>(null)
  const [fehler, setFehler] = useState('')
  const neu = () =>
    api<T>(url).then(
      (d) => {
        setDaten(d)
        setFehler('')
      },
      (e) => setFehler(fehlerText(e)),
    )
  useEffect(() => {
    neu()
  }, abh)
  return { daten, fehler, neu, setDaten }
}

function speichern(teil: Partial<Record<string, unknown>>) {
  return api<void>('/api/settings', teil, 'PUT').then(
    () => melde(t('einst.gespeichert')),
    (e) => {
      melde(fehlerText(e))
      throw e
    },
  )
}

// ---------- Seite ----------

export function Einstellungen() {
  const [n, setN] = useState<Nutzer | null>(null)
  const [bereich, setBereich] = useState<Bereich>(((teile()[1] as Bereich) || 'geraet') as Bereich)
  const [toast, setToast] = useState('')
  toastSetzen = setToast
  useEffect(() => {
    ich().then(setN, () => {})
    fokusBald('einst-' + bereich)
  }, [])
  useEffect(() => {
    if (!toast) return
    const x = setTimeout(() => setToast(''), 3000)
    return () => clearTimeout(x)
  }, [toast])
  const sichtbar = BEREICHE.filter((b) => !b.admin || (n && n.admin))
  const waehle = (b: Bereich) => {
    setBereich(b)
    go('/einstellungen/' + b, { ersetzen: true }) // Zurück verlässt die Einstellungen, nicht jeden Bereich einzeln
  }

  return (
    <Seite>
      <div class="einst rand">
        <div class="zeile einst-seite">
          <nav class="einst-nav" aria-label={t('einst.titel')}>
            <h1 class="t-titel einst-titel">{t('einst.titel')}</h1>
            <Gruppe fokusKey="einst-nav" bevorzugt={'einst-' + bereich}>
              {sichtbar.map((b) => (
                <NavPunkt key={b.id} id={b.id} icon={b.icon} aktiv={b.id === bereich} waehle={waehle} />
              ))}
            </Gruppe>
          </nav>
          <section class="einst-inhalt wachse" aria-live="polite">
            <h2 class="t-reihe einst-bereich-titel">{t('einst.' + bereich)}</h2>
            <Gruppe fokusKey="einst-inhalt">
              {bereich === 'geraet' && <Geraet n={n} />}
              {bereich !== 'geraet' && n && !n.admin && <p class="t-text leise">{t('einst.nuradmin')}</p>}
              {n && n.admin && bereich === 'profile' && <Profile />}
              {n && n.admin && bereich === 'bibliothek' && <Bibliothek />}
              {n && n.admin && bereich === 'server' && <Server />}
              {n && n.admin && bereich === 'fernzugriff' && <Fernzugriff />}
              {n && n.admin && bereich === 'einladungen' && <Einladungen />}
              {n && n.admin && bereich === 'nachts' && <Nachts />}
              {n && n.admin && bereich === 'sicherung' && <Sicherung />}
              {n && n.admin && bereich === 'diagnose' && <Diagnose />}
            </Gruppe>
          </section>
        </div>
      </div>
      {toast && (
        <div class="fl-toast einst-toast" role="status">
          {toast}
        </div>
      )}
    </Seite>
  )
}

function NavPunkt({ id, icon, aktiv, waehle }: { id: Bereich; icon: Parameters<typeof Icon>[0]['name']; aktiv: boolean; waehle: (b: Bereich) => void }) {
  // Auf dem TV zeigt schon der Fokus den Bereich (screens/14), OK springt in den Inhalt.
  const f = useFokus<HTMLButtonElement>({ fokusKey: 'einst-' + id, onPress: () => (waehle(id), fokusBald('einst-inhalt')), onFocus: () => istTV() && !aktiv && waehle(id) })
  return (
    <button ref={f.ref} {...f.dom} type="button" aria-current={aktiv ? 'page' : undefined} class={'einst-punkt zeile fokusierbar' + (aktiv ? ' einst-punkt-aktiv' : '') + (f.fokus ? ' ist-fokus' : '')}>
      <Icon name={icon} />
      <span>{t('einst.' + id)}</span>
    </button>
  )
}

// ---------- Dieses Gerät (jeder) ----------

const TONSPRACHEN = ['de', 'en', 'fr', 'es', 'it', 'ja', 'tr']
const SPRACHNAMEN: Record<string, string> = { de: 'Deutsch', en: 'English', fr: 'Français', es: 'Español', it: 'Italiano', ja: '日本語', tr: 'Türkçe' }

function Geraet({ n }: { n: Nutzer | null }) {
  const [v, setV] = useState(vorlieben())
  const [th, setTh] = useState<Thema>(thema())
  const [test, setTest] = useState('')
  const aendern = (teil: Partial<typeof v>) => {
    setVorlieben(teil)
    setV(vorlieben())
  }
  const erste = v.audioLangs[0] || 'de'
  return (
    <>
      {!istTV() && (
        <div class="fl-gruppe">
          <Gruppenkopf titel={t('einst.thema')} />
          <div class="zeile zeile-umbruch">
            {(['kino', 'hell', 'system'] as Thema[]).map((x) => (
              <Chip key={x} fokusKey={'thema-' + x} an={th === x} onPress={() => (setThema(x), setTh(x))}>
                {t('einst.thema.' + x)}
              </Chip>
            ))}
          </div>
        </div>
      )}
      <div class="fl-gruppe">
        <Gruppenkopf titel={t('einst.sprache')} />
        <div class="zeile zeile-umbruch">
          {(['de', 'en'] as const).map((x) => (
            <Chip key={x} fokusKey={'sprache-' + x} an={sprache === x} onPress={() => sprache !== x && setSprache(x)}>
              {SPRACHNAMEN[x]}
            </Chip>
          ))}
        </div>
      </div>
      <div class="fl-gruppe">
        <Gruppenkopf titel={t('einst.tonsprache')} />
        <p class="t-klein leise einst-absatz">{t('einst.tonsprache.text')}</p>
        <div class="zeile zeile-umbruch">
          {TONSPRACHEN.map((x) => (
            <Chip key={x} fokusKey={'ton-' + x} an={erste === x} onPress={() => aendern({ audioLangs: x === 'en' ? ['en'] : [x, 'en'] })}>
              {SPRACHNAMEN[x]}
            </Chip>
          ))}
        </div>
      </div>
      <div class="fl-gruppe">
        <Gruppenkopf titel={t('einst.ut')} />
        <p class="t-klein leise einst-absatz">{t('einst.ut.auto.text')}</p>
        <div class="zeile zeile-umbruch">
          {(
            [
              ['', 'auto'],
              ['always', 'always'],
              ['off', 'off'],
            ] as const
          ).map(([wert, k]) => (
            <Chip key={k} fokusKey={'ut-' + k} an={v.subtitleMode === wert} onPress={() => aendern({ subtitleMode: wert })}>
              {t('einst.ut.' + k)}
            </Chip>
          ))}
        </div>
        <Zeile fk="nacht" icon="ton" titel={t('player.nachtmodus')} unter={t('einst.nacht.text')} schalter={v.night} onPress={() => aendern({ night: !v.night })} />
      </div>
      <div class="fl-gruppe">
        <Zeile
          fk="geraetetest"
          icon="neustart"
          titel={t('einst.test')}
          unter={test || t('einst.test.text')}
          onPress={() => {
            setTest(t('einst.test.laeuft'))
            geraeteTest.starten().then(() => {
              const r = loadProbe()
              setTest(r ? t('einst.test.fertig', { n: r.played.length }) : '')
            })
          }}
        />
        {!istTV() && <Zeile fk="koppeln" icon="fernseher" titel={t('nav.koppeln')} unter={t('einst.koppeln.text')} onPress={() => go('/koppeln')} />}
        <Zeile fk="profilwechsel" icon="profil" titel={t('nav.profil')} unter={n ? n.name + ' · ' + t('einst.profil.text') : t('einst.profil.text')} onPress={() => go('/profile')} />
      </div>
    </>
  )
}

// ---------- Profile ----------

function Profile() {
  const { daten, fehler, neu } = useLaden<Nutzer[]>('/api/users')
  const [name, setName] = useState('')
  const [pin, setPin] = useState('')
  const [admin, setAdmin] = useState(false)
  const [offen, setOffen] = useState('') // Profil-ID mit offenem PIN-Feld
  const [neuePin, setNeuePin] = useState('')
  const [loeschen, setLoeschen] = useState('')
  if (fehler) return <Fehler fehler={fehler} nochmal={neu} />
  const anlegen = () =>
    api<Nutzer>('/api/users', { name, password: pin || undefined, admin, color: Math.floor(Math.random() * 360) }).then(
      () => {
        setName('')
        setPin('')
        setAdmin(false)
        melde(t('einst.gespeichert'))
        neu()
      },
      (e) => melde(fehlerText(e)),
    )
  const aendern = (u: Nutzer, teil: Record<string, unknown>) =>
    api<Nutzer>('/api/users/' + u.id, teil, 'PUT').then(
      () => {
        setOffen('')
        setNeuePin('')
        melde(t('einst.gespeichert'))
        neu()
      },
      (e) => melde(fehlerText(e)),
    )
  return (
    <>
      <div class="fl-gruppe">
        {(daten || []).map((u) => (
          <div key={u.id} class="einst-profil">
            <Zeile
              icon="profil"
              titel={u.name + (u.admin ? ' · ' + t('einst.admin') : '')}
              unter={u.hasPassword ? t('einst.mitpin') : t('einst.ohnepin')}
              rechts={
                <span class="zeile einst-knoepfe">
                  <Button fokusKey={'pin-' + u.id} onPress={() => setOffen(offen === u.id ? '' : u.id)}>
                    {t('einst.pin.aendern')}
                  </Button>
                  <Button
                    fokusKey={'del-' + u.id}
                    variante="geist"
                    onPress={() => {
                      if (loeschen !== u.id) return setLoeschen(u.id)
                      api<void>('/api/users/' + u.id, undefined, 'DELETE').then(neu, (e) => melde(fehlerText(e)))
                    }}
                  >
                    {t(loeschen === u.id ? 'einst.loeschen.sicher' : 'einst.loeschen')}
                  </Button>
                </span>
              }
            />
            {offen === u.id && (
              <div class="zeile zeile-umbruch einst-unterform">
                <Feld fk={'pinfeld-' + u.id} wert={neuePin} setWert={setNeuePin} label={t('einst.pin.neu')} typ="password" onEnter={() => aendern(u, { password: neuePin })} />
                <Button fokusKey={'pinok-' + u.id} variante="primaer" onPress={() => aendern(u, { password: neuePin })}>
                  {t('einst.speichern')}
                </Button>
                {u.hasPassword && !u.admin && (
                  <Button fokusKey={'pinweg-' + u.id} variante="geist" onPress={() => aendern(u, { password: '' })}>
                    {t('einst.pin.weg')}
                  </Button>
                )}
              </div>
            )}
          </div>
        ))}
      </div>
      <div class="fl-gruppe">
        <Gruppenkopf titel={t('einst.neu')} />
        <div class="zeile zeile-umbruch">
          <Feld fk="neu-name" wert={name} setWert={setName} label={t('einst.name')} />
          <Feld fk="neu-pin" wert={pin} setWert={setPin} label={t('einst.pin')} typ="password" />
        </div>
        <Zeile fk="neu-admin" titel={t('einst.admin')} schalter={admin} onPress={() => setAdmin(!admin)} />
        <Button fokusKey="neu-anlegen" variante="primaer" aus={!name.trim()} onPress={anlegen}>
          {t('einst.anlegen')}
        </Button>
      </div>
    </>
  )
}

// ---------- Bibliothek ----------

function Bibliothek() {
  const s = useLaden<Settings>('/api/settings')
  const [status, setStatus] = useState<{ scanning: boolean; found: number } | null>(null)
  const [ordner, setOrdner] = useState<{ path: string; parent: string; dirs: { name: string; path: string }[] } | null>(null)
  const [tmdb, setTmdb] = useState('')
  useEffect(() => {
    let x = 0
    const hole = () =>
      api<{ scanning: boolean; found: number }>('/api/status').then((st) => {
        setStatus(st)
        if (st.scanning) x = window.setTimeout(hole, 1500)
      }, () => {})
    hole()
    return () => clearTimeout(x)
  }, [s.daten])
  if (s.fehler) return <Fehler fehler={s.fehler} nochmal={s.neu} />
  if (!s.daten) return null
  const dirs = s.daten.dirs
  const setDirs = (d: string[]) => speichern({ dirs: d }).then(s.neu, () => {})
  const blaettern = (path: string) => api<typeof ordner>('/api/setup/dirs?path=' + encodeURIComponent(path)).then(setOrdner, (e) => melde(fehlerText(e)))
  return (
    <>
      <div class="fl-gruppe">
        <Gruppenkopf titel={t('einst.ordner')} />
        {!dirs.length && <p class="t-text leise">{t('einst.ordner.leer')}</p>}
        {dirs.map((d) => (
          <Zeile
            key={d}
            icon="film"
            titel={d}
            rechts={
              <Button fokusKey={'dir-weg-' + d} variante="geist" onPress={() => setDirs(dirs.filter((x) => x !== d))}>
                {t('einst.entfernen')}
              </Button>
            }
          />
        ))}
        {!ordner ? (
          <Button fokusKey="dir-hinzu" icon="mehr" onPress={() => blaettern('')}>
            {t('einst.ordner.hinzu')}
          </Button>
        ) : (
          <div class="einst-ordner">
            <p class="wert einst-absatz">{ordner.path || '…'}</p>
            <div class="zeile zeile-umbruch">
              {ordner.path && (
                <Button fokusKey="dir-nehmen" variante="primaer" onPress={() => (setOrdner(null), setDirs(dirs.concat(ordner.path)))}>
                  {t('einst.ordner.hier')}
                </Button>
              )}
              {ordner.path && (
                <Button fokusKey="dir-hoch" icon="hoch" onPress={() => blaettern(ordner.parent)}>
                  {t('einst.ordner.hoch')}
                </Button>
              )}
              <Button fokusKey="dir-zu" variante="geist" onPress={() => setOrdner(null)}>
                {t('knopf.zurueck')}
              </Button>
            </div>
            {ordner.dirs.map((d) => (
              <Zeile key={d.path} fk={'dir-' + d.path} icon="weiter" titel={d.name} unter={ordner.path ? undefined : d.path} onPress={() => blaettern(d.path)} />
            ))}
          </div>
        )}
      </div>
      <div class="fl-gruppe">
        <Zeile
          fk="scan"
          icon="neustart"
          titel={t('einst.scan')}
          unter={status ? t(status.scanning ? 'einst.scan.laeuft' : 'einst.scan.fertig', { n: status.found }) : undefined}
          onPress={() => api<void>('/api/rescan', {}).then(() => setStatus({ scanning: true, found: status ? status.found : 0 }), (e) => melde(fehlerText(e)))}
        />
      </div>
      <div class="fl-gruppe">
        <Gruppenkopf titel={t('einst.tmdb')} />
        <p class="t-klein leise einst-absatz">{s.daten.tmdbKey ? t('einst.tmdb.gesetzt') : t('einst.tmdb.text')}</p>
        <div class="zeile zeile-umbruch">
          <Feld fk="tmdb" wert={tmdb} setWert={setTmdb} label={t('einst.tmdb')} breit onEnter={() => speichern({ tmdbKey: tmdb }).then(() => (setTmdb(''), s.neu()), () => {})} />
          <Button fokusKey="tmdb-ok" onPress={() => speichern({ tmdbKey: tmdb }).then(() => (setTmdb(''), s.neu()), () => {})}>
            {t('einst.speichern')}
          </Button>
          {s.daten.tmdbKey && (
            <Button fokusKey="tmdb-weg" variante="geist" onPress={() => speichern({ tmdbKey: '' }).then(s.neu, () => {})}>
              {t('einst.entfernen')}
            </Button>
          )}
        </div>
      </div>
      <Pruefen />
    </>
  )
}

interface Unsicher {
  id: string
  file: string
  meta: { title?: string; year?: number } | null
  guess: string
}
interface Kandidat {
  tmdbId: number
  title: string
  year?: number
}

function Pruefen() {
  const { daten, neu } = useLaden<Unsicher[]>('/api/settings/review')
  const [offen, setOffen] = useState('')
  const [q, setQ] = useState('')
  const [kandidaten, setKandidaten] = useState<Kandidat[]>([])
  const suche = (id: string, text: string) => api<Kandidat[]>('/api/items/' + id + '/search?q=' + encodeURIComponent(text)).then(setKandidaten, (e) => melde(fehlerText(e)))
  return (
    <div class="fl-gruppe">
      <Gruppenkopf titel={t('einst.pruefen')} />
      <p class="t-klein leise einst-absatz">{daten && !daten.length ? t('einst.pruefen.leer') : t('einst.pruefen.text')}</p>
      {(daten || []).slice(0, 50).map((e) => (
        <div key={e.id}>
          <Zeile
            fk={'pruef-' + e.id}
            icon="info"
            titel={(e.meta && e.meta.title) || e.guess}
            unter={e.file}
            onPress={() => {
              setOffen(offen === e.id ? '' : e.id)
              setQ(e.guess)
              setKandidaten([])
              if (offen !== e.id) suche(e.id, e.guess)
            }}
          />
          {offen === e.id && (
            <div class="einst-unterform">
              <div class="zeile zeile-umbruch">
                <Feld fk={'pruef-q-' + e.id} wert={q} setWert={setQ} label={t('einst.suchen')} breit onEnter={() => suche(e.id, q)} />
                <Button fokusKey={'pruef-suchen-' + e.id} icon="suche" onPress={() => suche(e.id, q)}>
                  {t('einst.suchen')}
                </Button>
              </div>
              {kandidaten.map((k) => (
                <Zeile
                  key={k.tmdbId}
                  fk={'kand-' + k.tmdbId}
                  icon="haken"
                  titel={k.title + (k.year ? ' (' + k.year + ')' : '')}
                  onPress={() =>
                    api<void>('/api/items/' + e.id + '/identify', { tmdbId: k.tmdbId }).then(
                      () => {
                        melde(t('einst.gespeichert'))
                        setOffen('')
                        neu()
                      },
                      (x) => melde(fehlerText(x)),
                    )
                  }
                />
              ))}
            </div>
          )}
        </div>
      ))}
    </div>
  )
}

// ---------- Server ----------

function Server() {
  const s = useLaden<Settings>('/api/settings')
  const [name, setName] = useState('')
  useEffect(() => {
    if (s.daten) setName(s.daten.serverName)
  }, [s.daten])
  // Während ffmpeg lädt, den Fortschritt nachfragen.
  useEffect(() => {
    if (!s.daten || !s.daten.ffmpeg.download.running) return
    const x = setTimeout(s.neu, 2000)
    return () => clearTimeout(x)
  }, [s.daten])
  if (s.fehler) return <Fehler fehler={s.fehler} nochmal={s.neu} />
  if (!s.daten) return null
  const d = s.daten
  const ff = d.ffmpeg
  return (
    <>
      <div class="fl-gruppe">
        <Gruppenkopf titel={t('einst.servername')} />
        <div class="zeile zeile-umbruch">
          <Feld fk="servername" wert={name} setWert={setName} label={t('einst.servername')} onEnter={() => speichern({ serverName: name }).catch(() => {})} />
          <Button fokusKey="servername-ok" onPress={() => speichern({ serverName: name }).catch(() => {})}>
            {t('einst.speichern')}
          </Button>
        </div>
        <Zeile icon="fernseher" titel={t('einst.adresse')} rechts={<span class="wert">{d.lanUrl}</span>} />
      </div>
      <div class="fl-gruppe">
        <Gruppenkopf titel={t('einst.serversprache')} />
        <div class="zeile zeile-umbruch">
          {(['de', 'en'] as const).map((x) => (
            <Chip key={x} fokusKey={'serversprache-' + x} an={d.language === x} onPress={() => speichern({ language: x }).then(s.neu, () => {})}>
              {SPRACHNAMEN[x]}
            </Chip>
          ))}
        </div>
      </div>
      <div class="fl-gruppe">
        <Zeile
          icon={ff.ok ? 'haken' : 'fehler'}
          titel={t('einst.ffmpeg')}
          unter={ff.ok ? t('einst.ffmpeg.ok') : ff.download.running ? t('einst.ffmpeg.laedt', { n: ff.download.percent }) : ff.download.error || ff.hint || t('einst.ffmpeg.fehlt')}
          rechts={
            !ff.ok && ff.canDownload && !ff.download.running ? (
              <Button fokusKey="ffmpeg-laden" variante="primaer" onPress={() => api<void>('/api/setup/ffmpeg', {}).then(s.neu, (e) => melde(fehlerText(e)))}>
                {t('einst.ffmpeg.laden')}
              </Button>
            ) : undefined
          }
        />
        <Zeile icon="info" titel={t('einst.version')} rechts={<span class="wert">{d.version}</span>} unter={d.update ? t('einst.update.da', { v: d.update.version }) : undefined} />
        <Zeile fk="updates" icon="neustart" titel={t('einst.update')} schalter={d.updateCheck} onPress={() => speichern({ updateCheck: !d.updateCheck }).then(s.neu, () => {})} />
      </div>
    </>
  )
}

// ---------- Fernzugriff ----------

function Fernzugriff() {
  const s = useLaden<Settings>('/api/settings')
  const [st, setSt] = useState<{ method: string; publicUrl: string; reachable: boolean; hint: string } | null>(null)
  const [prueft, setPrueft] = useState(false)
  const [code, setCode] = useState<{ code: string; expires: string } | null>(null)
  useEffect(() => {
    if (s.daten && s.daten.remoteAvailable) api<typeof st>('/api/remote').then(setSt, () => {})
  }, [s.daten])
  if (s.fehler) return <Fehler fehler={s.fehler} nochmal={s.neu} />
  if (!s.daten) return null
  if (!s.daten.remoteAvailable) return <p class="t-text leise">{t('einst.remote.nicht')}</p>
  const an = s.daten.remote
  return (
    <div class="fl-gruppe">
      <Zeile fk="remote" icon="start" titel={t('einst.remote')} unter={t('einst.remote.text')} schalter={an} onPress={() => speichern({ remote: !an }).then(s.neu, () => {})} />
      {st && (
        <Zeile
          titel={st.reachable ? t('einst.remote.erreichbar') + (st.publicUrl ? ' · ' + st.publicUrl : '') : t('einst.remote.nichterreichbar')}
          unter={st.hint || (st.method && st.method !== 'none' ? st.method.toUpperCase() : undefined)}
          rechts={
            <Button
              fokusKey="remote-pruefen"
              aus={prueft}
              onPress={() => {
                setPrueft(true)
                api<typeof st>('/api/remote/check', {})
                  .then(setSt, (e) => melde(fehlerText(e)))
                  .then(() => setPrueft(false))
              }}
            >
              {t(prueft ? 'einst.remote.prueft' : 'einst.remote.pruefen')}
            </Button>
          }
        />
      )}
      <Zeile
        icon="fernseher"
        titel={t('einst.remote.code')}
        unter={code ? t('einst.remote.code.gilt', { zeit: datum(code.expires) }) : t('einst.remote.code.text')}
        rechts={
          code ? (
            <span class="fl-code">{code.code}</span>
          ) : (
            <Button fokusKey="remote-code" onPress={() => api<{ code: string; expires: string }>('/api/remote/pair', {}).then(setCode, (e) => melde(fehlerText(e)))}>
              {t('einst.remote.code.knopf')}
            </Button>
          )
        }
      />
    </div>
  )
}

// ---------- Einladungen ----------

interface Einladung {
  id: string
  note: string
  scope: { libraries?: string[]; items?: string[] }
  expires: string
  maxUses: number
  uses: number
  guests: number
}

function Einladungen() {
  const liste = useLaden<Einladung[]>('/api/invites')
  const s = useLaden<Settings>('/api/settings')
  const [notiz, setNotiz] = useState('')
  const [tage, setTage] = useState('7')
  const [max, setMax] = useState('0')
  const [libs, setLibs] = useState<string[]>([])
  const [neu, setNeu] = useState<{ url: string; qr?: string; hint?: string } | null>(null)
  if (liste.fehler) return <Fehler fehler={liste.fehler} nochmal={liste.neu} />
  const name = (p: string) => p.split(/[\\/]/).filter(Boolean).pop() || p
  const erstellen = () =>
    api<{ url: string; qr?: string; hint?: string }>('/api/invites', { note: notiz, libraries: libs, hours: Math.max(1, Number(tage) || 7) * 24, maxUses: Number(max) || 0 }).then(
      (r) => {
        setNeu(r)
        setNotiz('')
        liste.neu()
      },
      (e) => melde(fehlerText(e)),
    )
  return (
    <>
      <div class="fl-gruppe">
        {liste.daten && !liste.daten.length && <p class="t-text leise">{t('einst.einl.leer')}</p>}
        {(liste.daten || []).map((e) => {
          const was = (e.scope.libraries || []).map(name).concat((e.scope.items || []).length ? [(e.scope.items || []).length + ' Titel'] : [])
          return (
            <Zeile
              key={e.id}
              icon="profil"
              titel={e.note || e.id}
              unter={[
                was.length ? t('einst.einl.nur', { was: '„' + was.join('“, „') + '“' }) : t('einst.einl.alles'),
                t('einst.einl.bis', { datum: datum(e.expires) }),
                e.maxUses ? t('einst.einl.genutzt', { n: e.uses, max: e.maxUses }) : t('einst.einl.genutzt.frei', { n: e.uses }),
              ].join(' · ')}
              rechts={
                <Button fokusKey={'einl-weg-' + e.id} onPress={() => api<void>('/api/invites/' + e.id, undefined, 'DELETE').then(liste.neu, (x) => melde(fehlerText(x)))}>
                  {t('einst.einl.widerrufen')}
                </Button>
              }
            />
          )
        })}
      </div>
      <div class="fl-gruppe">
        <Gruppenkopf titel={t('einst.einl.neu')} />
        <p class="t-klein leise einst-absatz">{t('einst.einl.neu.text')}</p>
        <div class="zeile zeile-umbruch">
          <Feld fk="einl-notiz" wert={notiz} setWert={setNotiz} label={t('einst.einl.fuer')} />
          <Feld fk="einl-tage" wert={tage} setWert={setTage} label={t('einst.einl.tage')} typ="number" />
          <Feld fk="einl-max" wert={max} setWert={setMax} label={t('einst.einl.max')} typ="number" />
        </div>
        {s.daten && s.daten.dirs.length > 1 && (
          <div class="zeile zeile-umbruch">
            {s.daten.dirs.map((d) => (
              <Chip key={d} fokusKey={'einl-lib-' + d} an={libs.indexOf(d) >= 0} onPress={() => setLibs(libs.indexOf(d) >= 0 ? libs.filter((x) => x !== d) : libs.concat(d))}>
                {name(d)}
              </Chip>
            ))}
          </div>
        )}
        <Button fokusKey="einl-erstellen" variante="primaer" onPress={erstellen}>
          {t('einst.einl.erstellen')}
        </Button>
        {neu && (
          <div class="zeile einst-einladung">
            {neu.qr && <img class="einladung-qr" src={neu.qr} alt="" width={180} height={180} />}
            <div class="wachse">
              <p class="t-klein leise">{t('einst.einl.link')}</p>
              <p class="wert einst-link">{neu.url}</p>
              {neu.hint && <p class="t-klein leise">{neu.hint}</p>}
            </div>
          </div>
        )}
      </div>
    </>
  )
}

// ---------- Nachts vorbereiten ----------

function Nachts() {
  const s = useLaden<Settings>('/api/settings')
  const st = useLaden<{ waiting?: boolean; on?: boolean; window?: string; current?: { title: string; percent: number }; done?: number; pending?: number; lastError?: string }>('/api/settings/optimize')
  const [von, setVon] = useState('')
  const [bis, setBis] = useState('')
  const [frei, setFrei] = useState('')
  useEffect(() => {
    if (!s.daten) return
    const o = s.daten.optimize
    const std = !o.from && !o.to
    setVon(String(std ? 2 : o.from))
    setBis(String(std ? 6 : o.to))
    setFrei(String(o.minFreeGB || 20))
  }, [s.daten])
  if (s.fehler) return <Fehler fehler={s.fehler} nochmal={s.neu} />
  if (!s.daten) return null
  const o = s.daten.optimize
  const sichern = (teil: Partial<Settings['optimize']>) =>
    speichern({ optimize: { off: o.off, from: Number(von) || 0, to: Number(bis) || 0, minFreeGB: Number(frei) || 0, ...teil } }).then(() => (s.neu(), st.neu()), () => {})
  const d = st.daten
  return (
    <div class="fl-gruppe">
      <Zeile fk="opt-an" icon="neustart" titel={t('einst.opt')} unter={t('einst.opt.text')} schalter={!o.off} onPress={() => sichern({ off: !o.off })} />
      {d && (
        <p class="t-text leise einst-absatz">
          {d.waiting
            ? t('einst.opt.wartet')
            : t('einst.opt.stand', { done: d.done || 0, pending: d.pending || 0, fenster: d.window || '' }) + (d.current ? ' · ' + t('einst.opt.jetzt', { titel: d.current.title, n: Math.round(d.current.percent) }) : '')}
        </p>
      )}
      {d && d.lastError && <p class="t-klein fehlertext einst-absatz">{d.lastError}</p>}
      <div class="zeile zeile-umbruch">
        <Feld fk="opt-von" wert={von} setWert={setVon} label={t('einst.opt.von')} typ="number" />
        <Feld fk="opt-bis" wert={bis} setWert={setBis} label={t('einst.opt.bis')} typ="number" />
        <Feld fk="opt-frei" wert={frei} setWert={setFrei} label={t('einst.opt.frei')} typ="number" />
        <Button fokusKey="opt-ok" onPress={() => sichern({})}>
          {t('einst.speichern')}
        </Button>
      </div>
    </div>
  )
}

// ---------- Sicherung ----------

function Sicherung() {
  const diag = useLaden<{ db: { backupAt: string; backupFile: string; sizeBytes: number } }>('/api/diagnostics')
  const datei = useRef<HTMLInputElement>(null)
  const [laeuft, setLaeuft] = useState(false)
  const einspielen = (f: File) => {
    setLaeuft(true)
    fetch('/api/settings/restore', { method: 'POST', body: f, credentials: 'same-origin' })
      .then((r) => (r.ok ? melde(t('einst.backup.ok')) : r.text().then((x) => melde(x))))
      .catch((e) => melde(fehlerText(e)))
      .then(() => setLaeuft(false))
  }
  const db = diag.daten && diag.daten.db
  return (
    <div class="fl-gruppe">
      <Zeile
        fk="backup-laden"
        icon="runter"
        titel={t('einst.backup.laden')}
        unter={db ? t('einst.backup.stand', { zeit: datum(db.backupAt) }) + ' · ' + groesse(db.sizeBytes) : t('einst.backup.laden.text')}
        onPress={() => (location.href = '/api/settings/backup')}
      />
      <Zeile fk="backup-einspielen" icon="hoch" titel={t('einst.backup.einspielen')} unter={t('einst.backup.einspielen.text')} onPress={() => !laeuft && datei.current && datei.current.click()} />
      <input ref={datei} type="file" accept=".db,application/octet-stream" class="nur-sr" tabIndex={-1} onChange={(e) => {
        const f = (e.target as HTMLInputElement).files
        if (f && f[0]) einspielen(f[0])
      }} />
    </div>
  )
}

// ---------- Diagnose ----------

interface Diag {
  version: string
  os: string
  cpus: number
  ffmpeg: string
  hw: string
  hwSpeed: number
  diskFree: number
  diskTotal: number
  cacheDir: string
  scan: { scanning: boolean; found: number }
  active: { user: string; title: string; device: string; method: string; light: 'green' | 'yellow' | 'red'; reasons: string[] | null }[] | null
  recent: Diag['active']
  log: string[] | null
  db: { checkedAt: string; integrity: string; sizeBytes: number }
}

function Diagnose() {
  const { daten: d, fehler, neu } = useLaden<Diag>('/api/diagnostics')
  const [log, setLog] = useState(false)
  useEffect(() => {
    const x = setInterval(neu, 10000)
    return () => clearInterval(x)
  }, [])
  if (fehler) return <Fehler fehler={fehler} nochmal={neu} />
  if (!d) return null
  const aktiv = d.active || []
  const text = JSON.stringify({ ...d, log: undefined }, null, 2) + '\n\n' + (d.log || []).join('\n')
  const kopieren = () => {
    const ta = document.createElement('textarea') // Chromium 53 kennt navigator.clipboard nicht
    ta.value = text
    document.body.appendChild(ta)
    ta.select()
    try {
      document.execCommand('copy')
      melde(t('einst.diag.kopiert'))
    } catch {}
    ta.remove()
  }
  const methode = (m: string) => t('player.methode.' + m)
  return (
    <>
      <div class="fl-gruppe">
        <Gruppenkopf titel={t('einst.diag.jetzt')} />
        <div class="fl-zeile einst-messwerte">
          <Messwert wert={d.hwSpeed ? d.hwSpeed.toFixed(1).replace('.', ',') + '×' : '–'} label={t('einst.diag.umwandeln', { hw: d.hw || 'CPU' })} />
          <Messwert wert={String(aktiv.length)} label={t('einst.diag.streams')} />
          <Messwert wert={groesse(d.diskFree)} label={t('einst.diag.platz')} />
          <Messwert wert={String(d.scan.found)} label={t('titel.anzahl', { n: '' }).trim()} />
        </div>
      </div>
      <div class="fl-gruppe">
        <Gruppenkopf titel={t('einst.diag.laufend')} />
        {!aktiv.length && <p class="t-text leise">{t('einst.diag.keine')}</p>}
        {aktiv.map((a, i) => (
          <Zeile
            key={i}
            icon="abspielen"
            titel={'„' + a.title + '“ · ' + a.user + ' · ' + a.device}
            unter={methode(a.method) + (a.reasons && a.reasons.length ? ': ' + a.reasons.join(' · ') : '')}
            rechts={<span class="wert" title={t('ampel.' + a.light)}><AmpelPunkt stufe={a.light} /></span>}
          />
        ))}
      </div>
      <div class="fl-gruppe">
        <Gruppenkopf titel={t('einst.server')} />
        <Zeile icon={d.ffmpeg ? 'haken' : 'fehler'} titel={(d.ffmpeg || 'ffmpeg –') + ' · ' + (d.hw || 'CPU')} unter={d.os + ' · ' + d.cpus + ' CPUs · ' + d.version} />
        <Zeile
          icon={d.db.integrity === 'ok' || !d.db.integrity ? 'haken' : 'fehler'}
          titel={t('einst.diag.db')}
          unter={t('einst.diag.db.text', { zeit: datum(d.db.checkedAt), ergebnis: d.db.integrity || '–' })}
          rechts={<span class="wert">{groesse(d.db.sizeBytes)}</span>}
        />
        <Zeile fk="diag-log" icon="info" titel={t('einst.diag.log')} unter={t('einst.diag.log.text')} rechts={<span class="wert">{t(log ? 'einst.ausblenden' : 'einst.anzeigen')}</span>} onPress={() => setLog(!log)} />
        {log && <pre class="protokoll">{(d.log || []).slice(-200).join('\n')}</pre>}
        <Zeile fk="diag-kopieren" icon="mehr" titel={t('einst.diag.kopieren')} unter={t('einst.diag.kopieren.text')} onPress={kopieren} />
      </div>
    </>
  )
}

function Messwert({ wert, label }: { wert: string; label: string }) {
  return (
    <span class="fl-messwert">
      <b>{wert}</b>
      <small>{label}</small>
    </span>
  )
}
