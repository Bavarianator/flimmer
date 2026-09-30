// Player: Wiedergabe (Direct Play, natives HLS oder hls.js mit TV-Puffergrenzen), PGS-Overlay,
// Bedienung im Jellyfin-Aufbau: Kopf (Zurück, Titel, Ampel), Zeitleiste mit Kapiteln, Knopfleiste
// (Vorherige, 10 s zurück, Pause, 30 s vor, Nächste · Ton & Untertitel, Qualität & Tempo, Vollbild),
// „Vorspann überspringen“ (Kapitel namens Vorspann/Intro), Nächste-Folge-Karte, Fortschritt alle 10 s.
// Live-TV: Prop kanal statt id, dann ohne Zeitleiste, Fortschritt und Menüs.
// Tastatur/Fernbedienung steuert der Player selbst (D-Pad-Navigation ist pausiert):
//   Standard:     OK = Pause (bzw. Vorspann überspringen), ←/→ = 10 s (gehalten 30 s), ↑ = Ton & Untertitel, ↓ = Knopfleiste
//   Knopfleiste:  ←/→ = Knopf, OK = drücken, ↑ oder Zurück = zurück zum Standard
//   Menü:         ↑/↓ = Eintrag, ←/→ = Spalte, OK = wählen, Zurück = zu
import type { ComponentChildren } from 'preact'
import { useEffect, useRef, useState } from 'preact/hooks'
import type Hls from 'hls.js/light'
import { getToken, profile } from '../profile'
import { probeSignal } from '../probe'
import { anzeigeTitel, api, bild, bibliothek, details, fortsetzenAb, serien, titelAus, vergessen, type Item } from '../lib/api'
import { istTV, istHandy } from '../lib/device'
import { usePausierteNavigation } from '../lib/focus'
import { dauer, folge, t, uhr } from '../lib/i18n'
import { back, go, pfad, useZurueck } from '../lib/router'
import { Icon, type IconName } from '../components/Icon'
import { AmpelPunkt } from '../components/Karte'
import { startUntertitel, tonName, untertitelName, untertitelUrl, type Plan } from './spuren'
import { bildModus, setBildModus, setVorlieben, vorlieben } from './vorlieben'
import { BILD_MODI, bildLage, type BildModus, type Crop, type Lage } from './bild'
import { useGleichlauf, type Gemeinsam } from './gemeinsam'
import './player.css'

// Fortschritt an den Server; die lokale Kopie hilft, falls der Server gerade nicht erreichbar ist.
function speichern(id: string, pos: number, dauer: number, ton: string, ut: string, beacon = false, pausiert = false) {
  if (pos < 5) return
  try {
    localStorage.setItem('pos:' + id, String(Math.floor(pos)))
  } catch {}
  const it = titelAus(id)
  if (it) it.progress = Math.floor(pos)
  // paused: Herzschlag auch in der Pause, sonst verschwindet die Sitzung nach 60 s aus dem Dashboard.
  const body = JSON.stringify({ pos: Math.floor(pos), duration: Math.floor(dauer), audio: ton, subtitle: ut, paused: pausiert || undefined })
  const url = `/api/items/${id}/progress`
  // sendBeacon überlebt das Schließen der App, kann aber keinen Bearer-Header (TVs) – dann fetch mit keepalive.
  if (beacon && navigator.sendBeacon && !getToken() && navigator.sendBeacon(url, body)) return
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (getToken()) headers.Authorization = 'Bearer ' + getToken()
  fetch(url, { method: 'POST', headers, body, credentials: 'same-origin', keepalive: true }).catch(() => {})
}

// „22:41“ – Uhrzeit, zu der der Titel bei normaler Geschwindigkeit endet.
function endetUm(rest: number): string {
  const d = new Date(Date.now() + rest * 1000)
  return d.getHours() + ':' + (d.getMinutes() < 10 ? '0' : '') + d.getMinutes()
}

// Raum anlegen und hinein; die Startstelle reist im Pfad mit (/party/{raum}/{sekunden}).
function gemeinsamStarten(id: string, pos: number) {
  api<{ id: string }>('/api/party', { mediaId: id }).then((r) => go(pfad('party', r.id, Math.floor(pos)), { ersetzen: true }), () => {})
}

// „Alle abspielen“ (web-browse): sessionStorage 'flimmer.warteschlange' = {ids}. Steht die id darin, gilt diese Reihenfolge.
function warteschlange(id: string): string[] | null {
  try {
    const ids = (JSON.parse(sessionStorage.getItem('flimmer.warteschlange') || 'null') || {}).ids as string[] | undefined
    return ids && ids.indexOf(id) >= 0 ? ids : null
  } catch {
    return null
  }
}

interface Kapitel {
  start: number
  name: string
}
const VORSPANN = /vorspann|intro|opening|titelsequenz/i
const ABSPANN = /abspann|credits|outro|ending/i
const TEMPI = [0.5, 0.75, 1, 1.25, 1.5, 2]

interface Eintrag {
  label: string
  unter?: string // kleine zweite Zeile
  aktiv: boolean
  schalter?: boolean // als Schalter statt Haken zeigen
  trenner?: boolean // Linie davor
  tun: () => void
}
interface Spalte {
  titel: string
  eintraege: Eintrag[]
}
interface Knopf {
  id: string
  label: string
  icon: IconName
  zahl?: string // „10“/„30“ im Spul-Pfeil
  gross?: boolean
  rechts?: boolean
  offen?: boolean
  tun: () => void
}

// spuren = Ton & Untertitel (↑), einst = Qualität & Geschwindigkeit
type Menue = { art: 'spuren' | 'einst'; col: number; row: number } | null

export function Player(p: {
  id: string
  kanal?: string // Live-TV: Kanal-ID, dann spielt /api/livetv/channels/{kanal}/play
  start?: number
  onBack?: () => void // Zurück läuft über den Router (useZurueck); bleibt für den Aufrufer-Vertrag
  gemeinsam?: Gemeinsam
  seitenleiste?: (zu: () => void) => ComponentChildren // Gemeinsam: Leiste rechts, zu = ausblenden
  unter?: string // Live-TV: laufende Sendung
}) {
  const { id, gemeinsam, kanal } = p
  const live = !!kanal
  const video = useRef<HTMLVideoElement>(null)
  const [plan, setPlan] = useState<Plan | null>(null)
  const [fehler, setFehler] = useState<{ text: string; detail: string } | null>(null)
  const [versuch, setVersuch] = useState(0)
  const [tonspur, setTonspur] = useState(0) // 0 = automatisch (Server nimmt Sprachkette/gemerkte Sprache)
  const [nacht, setNacht] = useState(vorlieben().night)
  const [qualitaet, setQualitaet] = useState(vorlieben().maxHeight) // 0 = automatisch
  const [tempo, setTempo] = useState(1)
  const [ut, setUt] = useState(-2) // -2 = noch nicht gewählt, -1 = aus
  const [osd, setOsd] = useState(true)
  const [zeigerWeg, setZeigerWeg] = useState(false) // Mauszeiger ausgeblendet
  const [menue, setMenue] = useState<Menue>(null)
  const [reihe, setReihe] = useState<number | null>(null) // Fokus in der Knopfleiste (D-Pad)
  const [warum, setWarum] = useState(false)
  const [leiste, setLeiste] = useState(true) // Gemeinsam-Leiste sichtbar
  const [zeit, setZeit] = useState({ t: 0, d: 0, b: 0, paused: true, waiting: true })
  const [titel, setTitel] = useState<Item | undefined>(titelAus(id))
  const [naechste, setNaechste] = useState<Item | null>(null)
  const [vorige, setVorige] = useState<Item | null>(null)
  const [kapitel, setKapitel] = useState<Kapitel[]>([])
  const [quelleHoehe, setQuelleHoehe] = useState(0) // Bildhöhe der Quelle, 0 = unbekannt
  const [zeiger, setZeiger] = useState<number | null>(null) // Maus über der Zeitleiste (0..1)
  const [karte, setKarte] = useState<'aus' | 'zeigen' | 'weg'>('aus')
  const [countdown, setCountdown] = useState(10)
  const [bildM, setBildM] = useState<BildModus>(bildModus())
  const [crop, setCrop] = useState<Crop | null>(null)
  const [lage, setLage] = useState<Lage | null>(null) // null = noch keine Videogröße: CSS (contain)
  const [utZeilen, setUtZeilen] = useState<string[]>([])
  const resumeAt = useRef(p.start !== undefined ? p.start : fortsetzenAb(id))
  const sprachen = useRef({ ton: '', ut: 'off' })
  const tv = istTV()
  usePausierteNavigation()
  useGleichlauf(video, gemeinsam)

  // Titel, vorige und nächste Folge (bzw. Warteschlange) aus der (meist schon gecachten) Bibliothek.
  useEffect(() => {
    if (live) return
    bibliothek().then((alle) => {
      const it = titelAus(id)
      setTitel(it)
      const ws = warteschlange(id)
      let liste: Item[] = []
      if (ws) liste = ws.map(titelAus).filter((x): x is Item => !!x)
      else if (it && it.series) {
        const s = serien(alle).filter((x) => x.name === it.series)[0]
        liste = s ? s.staffeln.reduce<Item[]>((a, x) => a.concat(x.folgen), []) : []
      }
      const i = liste.map((x) => x.id).indexOf(id)
      setVorige(i > 0 ? liste[i - 1] : null)
      setNaechste(i >= 0 && liste[i + 1] ? liste[i + 1] : null)
    }, () => {})
    // Kapitel (ffprobe) aus den Details (mit ?device=, Cache geteilt mit der Detailseite); ohne Kapitel bleibt die Zeitleiste ein Stück.
    // Dazu video.crop (eingebrannte Balken). Der Server erkennt sie beim ersten Abruf im Hintergrund, deshalb ohne crop
    // einmal nachfragen, sobald der Cache (30 s) abgelaufen ist. ponytail: ein Nachversuch, reicht für die Erkennung.
    setKapitel([])
    setCrop(null)
    setQuelleHoehe(0)
    let nochmal = 0
    const holeDetails = (erst: boolean) =>
      details(id).then((d) => {
        if (erst) setKapitel((d && d.chapters) || [])
        if (erst && d && d.video) setQuelleHoehe(d.video.height)
        const c = d && d.video ? d.video.crop : undefined
        if (c) setCrop(c)
        else if (erst) nochmal = window.setTimeout(() => holeDetails(false), 31000)
      }, () => {})
    holeDetails(true)
    return () => clearTimeout(nochmal)
  }, [id])

  // Plan holen und Quelle setzen – erneut bei anderer Tonspur, Nachtmodus, Qualität oder „Nochmal versuchen“.
  useEffect(() => {
    let hls: Hls | undefined
    let weg = false
    probeSignal.aborted = true // Geräte-Test gibt den (oft einzigen) Hardware-Decoder frei
    const v = video.current!
    const ab = resumeAt.current
    const vl = vorlieben()
    setFehler(null)
    const laden = kanal
      ? api<{ url: string; title: string }>(`/api/livetv/channels/${encodeURIComponent(kanal)}/play`, {}).then(
          (r) => ({ url: r.url, title: r.title, method: 'direct-stream', light: 'green', duration: 0, audioIndex: 0, audio: null, subtitles: null }) as Plan,
        )
      : api<Plan>(`/api/items/${id}/play`, { ...profile, audioTrack: tonspur, audioLangs: vl.audioLangs, subtitleMode: vl.subtitleMode, night: nacht, maxHeight: qualitaet })
    laden
      .then(async (pl) => {
        if (weg) return
        setPlan(pl)
        const a = (pl.audio || []).filter((x) => x.index === pl.audioIndex)[0]
        sprachen.current.ton = (a && a.language) || ''
        setUt((alt) => (alt === -2 ? startUntertitel(pl) : alt))
        const istHls = pl.url.indexOf('.m3u8') > 0
        if (!istHls || profile.nativeHls) {
          // Direct Play und natives HLS: die Videopipeline des Geräts puffert selbst – auf TVs der stabilste Weg.
          v.src = pl.url
          if (ab > 0) v.currentTime = ab
        } else {
          const { default: HlsJs } = await import('hls.js/light')
          if (weg) return
          if (!HlsJs.isSupported()) throw new Error(t('player.fehler.hls'))
          // Puffer begrenzen: große Remux-Segmente sprengen sonst das MSE-Kontingent von TVs (→ Stocken).
          hls = new HlsJs({
            startPosition: live ? -1 : ab,
            maxBufferLength: tv ? 20 : 30,
            maxMaxBufferLength: tv ? 30 : 120,
            backBufferLength: tv ? 10 : 30,
            maxBufferSize: (tv ? 60 : 120) * 1000 * 1000,
          })
          hls.on(HlsJs.Events.ERROR, (_e: unknown, d: { fatal: boolean; details: string; type?: string }) => {
            if (d.fatal) setFehler({ text: t(d.type === 'networkError' ? 'player.fehler.netz' : 'player.fehler.hls'), detail: d.details })
          })
          hls.loadSource(pl.url)
          hls.attachMedia(v)
        }
        if (!gemeinsam) v.play().catch(() => {}) // Autoplay kann blockiert sein, dann startet der Nutzer selbst
      })
      .catch((e) => !weg && setFehler({ text: t(live ? 'player.fehler.live' : 'player.fehler.server'), detail: String((e && e.message) || e) }))
    const sichern = () => !live && speichern(id, v.currentTime, v.duration || 0, sprachen.current.ton, sprachen.current.ut, false, v.paused)
    const takt = setInterval(sichern, 10000)
    const beacon = () => !live && speichern(id, v.currentTime, v.duration || 0, sprachen.current.ton, sprachen.current.ut, true)
    const sicht = () => document.visibilityState === 'hidden' && beacon()
    window.addEventListener('pagehide', beacon)
    document.addEventListener('visibilitychange', sicht)
    return () => {
      weg = true
      clearInterval(takt)
      resumeAt.current = v.currentTime || ab
      sichern()
      window.removeEventListener('pagehide', beacon)
      document.removeEventListener('visibilitychange', sicht)
      if (hls) hls.destroy()
      v.removeAttribute('src')
      v.load()
    }
  }, [id, kanal, tonspur, nacht, qualitaet, versuch])

  // Beim Verlassen: Start („Weiterschauen“) und Bibliothek frisch laden.
  useEffect(
    () => () => {
      vergessen('start')
      vergessen('bibliothek')
    },
    [],
  )

  // Zeitanzeige samt Pufferstand
  useEffect(() => {
    const v = video.current!
    const upd = () => {
      let b = 0
      for (let i = 0; i < v.buffered.length; i++) if (v.buffered.start(i) <= v.currentTime + 0.5) b = Math.max(b, v.buffered.end(i))
      const d = isFinite(v.duration) ? v.duration : 0 // Live: Infinity
      setZeit({ t: v.currentTime, d: d || (plan ? plan.duration : 0), b, paused: v.paused, waiting: v.readyState < 3 && !v.paused })
    }
    const evs = ['timeupdate', 'play', 'pause', 'waiting', 'playing', 'durationchange', 'loadeddata', 'progress']
    evs.forEach((e) => v.addEventListener(e, upd))
    return () => evs.forEach((e) => v.removeEventListener(e, upd))
  }, [plan])

  // Geschwindigkeit; ein neues src setzt playbackRate zurück, deshalb auch defaultPlaybackRate.
  // Beim gemeinsamen Schauen regelt useGleichlauf die Rate (Raum-Tempo plus Drift-Korrektur).
  useEffect(() => {
    const v = video.current!
    if (gemeinsam) return
    v.defaultPlaybackRate = v.playbackRate = tempo
  }, [tempo, plan])

  // PGS-Untertitel als Canvas-Overlay – niemals einbrennen (das erzwingt Transcoding und war ein Hauptgrund fürs Stocken).
  useEffect(() => {
    const s = ((plan && plan.subtitles) || []).filter((x) => x.index === ut)[0]
    sprachen.current.ut = s ? (s.forced ? 'off' : s.language || 'und') : 'off'
    if (!s || s.format !== 'pgs') return
    let renderer: { dispose(): void } | undefined
    let weg = false
    Promise.all([import('libpgs'), import('libpgs/dist/libpgs.worker.js?url')]).then(([lib, worker]) => {
      if (weg) return
      renderer = new lib.PgsRenderer({ workerUrl: worker.default, video: video.current!, subUrl: untertitelUrl(id, s) })
    })
    return () => {
      weg = true
      if (renderer) renderer.dispose()
    }
  }, [plan, ut])

  // Bildanpassung: das <video> behält sein volles Bild (object-fit: fill) und wird gesetzt und skaliert (bild.ts);
  // .pl schneidet mit overflow: hidden ab. Neu gerechnet bei Größe, Drehung, Metadaten, Modus und crop.
  useEffect(() => {
    const v = video.current!
    const rahmen = v.parentElement!
    const rechne = () =>
      setLage(v.videoWidth && v.videoHeight ? bildLage(bildM, rahmen.clientWidth, rahmen.clientHeight, v.videoWidth, v.videoHeight, crop) : null)
    rechne()
    window.addEventListener('resize', rechne)
    window.addEventListener('orientationchange', rechne)
    v.addEventListener('loadedmetadata', rechne)
    v.addEventListener('resize', rechne) // andere Auflösung, z. B. nach Qualitätswechsel
    return () => {
      window.removeEventListener('resize', rechne)
      window.removeEventListener('orientationchange', rechne)
      v.removeEventListener('loadedmetadata', rechne)
      v.removeEventListener('resize', rechne)
    }
  }, [bildM, crop])

  // Text-Untertitel: Die gewählte Spur läuft „hidden“, ihre Zeilen zeichnet der Player selbst am unteren
  // Bildschirmrand. Der Browser würde sie im (beim Füllen größeren) Videorahmen zeichnen, also teils außerhalb.
  useEffect(() => {
    const tracks = video.current && video.current.textTracks
    setUtZeilen([])
    if (!tracks) return
    let an: TextTrack | null = null
    for (let i = 0; i < tracks.length; i++) {
      const ja = tracks[i].id === 'ut' + ut
      tracks[i].mode = ja ? 'hidden' : 'disabled'
      if (ja) an = tracks[i]
    }
    const tr = an
    if (!tr) return
    const neu = () => {
      let z: string[] = []
      const cues = tr.activeCues
      for (let i = 0; cues && i < cues.length; i++)
        z = z.concat(
          (cues[i] as VTTCue).text
            .replace(/<[^>]*>/g, '')
            .replace(/&nbsp;/g, ' ')
            .replace(/&lt;/g, '<')
            .replace(/&gt;/g, '>')
            .replace(/&amp;/g, '&')
            .split('\n'),
        )
      setUtZeilen(z)
    }
    tr.addEventListener('cuechange', neu)
    return () => tr.removeEventListener('cuechange', neu)
  }, [ut, plan])

  // Kapitel: Ende = Start des nächsten; Vorspann zum Überspringen, Abspann startet die Nächste-Folge-Karte.
  const kap = kapitel.map((k, i) => ({ start: k.start, ende: kapitel[i + 1] ? kapitel[i + 1].start : zeit.d, name: k.name && !/^(chapter|kapitel)\s*\d+$/i.test(k.name) ? k.name : t('player.kapitel', { n: i + 1 }) }))
  const jetzt = kap.filter((k) => zeit.t >= k.start && zeit.t < k.ende)[0]
  const vorspann = jetzt && VORSPANN.test(jetzt.name) && jetzt.ende - zeit.t > 2 ? jetzt : null
  const abspann = kap.filter((k) => ABSPANN.test(k.name) && k.start > zeit.d / 2)[0]

  // Nächste-Folge-Karte: ab dem Abspann-Kapitel, sonst in den letzten 30 s bzw. 10 %; zählt 10 s herunter.
  const rest = Math.max(0, zeit.d - zeit.t)
  const ende = zeit.d > 0 && (abspann ? zeit.t >= abspann.start : rest <= Math.min(30, zeit.d * 0.1))
  useEffect(() => {
    if (naechste && ende && karte === 'aus') {
      setKarte('zeigen')
      setCountdown(10)
    }
  }, [ende, naechste])
  useEffect(() => {
    if (karte !== 'zeigen' || zeit.paused) return
    if (countdown <= 0) {
      weiter()
      return
    }
    const x = setTimeout(() => setCountdown(countdown - 1), 1000)
    return () => clearTimeout(x)
  }, [karte, countdown, zeit.paused])
  useEffect(() => {
    const v = video.current!
    const zuEnde = () => {
      if (naechste && karte !== 'weg') weiter()
      else if (!gemeinsam) back()
    }
    v.addEventListener('ended', zuEnde)
    return () => v.removeEventListener('ended', zuEnde)
  }, [naechste, karte])

  const zuTitel = (x: Item) => {
    if (gemeinsam) gemeinsam.aktion('media', { mediaId: x.id })
    else go(pfad('watch', x.id), { ersetzen: true }) // Zurück führt dann zur Detailseite, nicht zur alten Folge
  }
  const weiter = () => naechste && zuTitel(naechste)

  // Wiedergabe steuern; beim gemeinsamen Schauen geht jede Aktion auch an den Raum.
  const umschalten = () => {
    const v = video.current!
    if (v.paused) {
      v.play().catch(() => {})
      if (gemeinsam) gemeinsam.aktion('play')
    } else {
      v.pause()
      if (gemeinsam) gemeinsam.aktion('pause')
    }
  }
  const springe = (ziel: number) => {
    if (live) return
    const v = video.current!
    v.currentTime = Math.max(0, Math.min((v.duration || Infinity) - 1, ziel))
    if (gemeinsam) gemeinsam.aktion('seek', { pos: v.currentTime })
  }
  // Vorheriges/nächstes Kapitel, an den Rändern die vorige/nächste Folge.
  const zurueckKapitel = () => {
    const i = jetzt ? kap.indexOf(jetzt) : -1
    if (jetzt && zeit.t - jetzt.start > 3) springe(jetzt.start)
    else if (i > 0) springe(kap[i - 1].start)
    else if (vorige && zeit.t < 3) zuTitel(vorige)
    else springe(0)
  }
  const vorKapitel = () => {
    const n = kap.filter((k) => k.start > zeit.t + 1)[0]
    if (n) springe(n.start)
    else weiter()
  }
  const setzeTempo = (x: number) => {
    setTempo(x)
    if (gemeinsam) gemeinsam.aktion('rate', { rate: x })
  }

  // Menüs: Ton (mit Nachtmodus), Untertitel und beim gemeinsamen Schauen Reaktionen; bzw. Qualität und Tempo.
  const audios = (plan && plan.audio) || []
  const subs = (plan && plan.subtitles) || []
  const nachtUmschalten = () => {
    setVorlieben({ night: !nacht })
    setNacht(!nacht)
  }
  const spuren: Spalte[] = [
    {
      titel: t('player.ton'),
      eintraege: audios
        .map<Eintrag>((a) => ({ label: tonName(a), aktiv: !!plan && a.index === plan.audioIndex, tun: () => plan && a.index !== plan.audioIndex && setTonspur(a.index) }))
        .concat({ label: t('player.nachtmodus'), unter: t('player.nachtmodus.text'), aktiv: nacht, schalter: true, trenner: audios.length > 0, tun: nachtUmschalten }),
    },
  ]
  if (subs.length)
    spuren.push({
      titel: t('player.untertitel'),
      eintraege: [{ label: t('player.aus'), aktiv: ut < 0, tun: () => setUt(-1) } as Eintrag].concat(
        subs.map((s) => ({ label: untertitelName(s), aktiv: s.index === ut, tun: () => setUt(s.index) })),
      ),
    })
  if (gemeinsam && gemeinsam.reaktionen)
    spuren.push({ titel: t('player.reaktionen'), eintraege: gemeinsam.reaktionen.map((r) => ({ label: r, aktiv: false, tun: () => gemeinsam.aktion('reaction', { text: r }) })) })
  const raumTempo = gemeinsam && gemeinsam.zustand ? gemeinsam.zustand.rate || 1 : tempo
  const einst: Spalte[] = [
    // Qualität: kleinere Stufen wandelt der Server auf diese Höhe um (nur wenn die Quelle größer ist).
    {
      titel: t('player.qualitaet'),
      // nur Stufen unter der Quelle; eine 720p-Datei auf „1080p“ zu stellen ändert nichts
      eintraege: [0, 1080, 720, 480].filter((h) => !h || !quelleHoehe || h < quelleHoehe).map<Eintrag>((h) => ({
        label: h ? h + 'p' : t('player.qualitaet.auto'),
        unter: h === 0 ? t('player.qualitaet.auto.text') : h === 480 ? t('player.qualitaet.sparen') : undefined,
        aktiv: qualitaet === h || (!h && !!quelleHoehe && qualitaet >= quelleHoehe), // Grenze über der Quelle = Original
        tun: () => {
          if (h === qualitaet) return
          setVorlieben({ maxHeight: h })
          setQualitaet(h)
        },
      })),
    },
    {
      titel: t('player.tempo'),
      eintraege: TEMPI.map<Eintrag>((x) => ({
        label: String(x).replace('.', t('player.komma')) + '×',
        unter: x === 1 ? t('player.tempo.normal') : undefined,
        aktiv: raumTempo === x,
        tun: () => setzeTempo(x),
      })),
    },
    {
      titel: t('player.bildanpassung'),
      eintraege: BILD_MODI.map<Eintrag>((m) => ({
        label: t('player.bild.' + m),
        unter: m === 'auto' ? t('player.bild.auto.text') : undefined,
        aktiv: bildM === m,
        tun: () => {
          setBildModus(m)
          setBildM(m)
        },
      })),
    },
  ]
  const spalten = menue && menue.art === 'einst' ? einst : spuren
  const menueAuf = (art: 'spuren' | 'einst' = 'spuren') => {
    if (live) return
    setMenue({ art, col: art === 'spuren' && subs.length && audios.length <= 1 ? 1 : 0, row: 0 })
  }
  const menueUm = (art: 'spuren' | 'einst') => (menue && menue.art === art ? setMenue(null) : menueAuf(art))

  const vollbild = () => {
    const d = document as Document & { webkitFullscreenElement?: Element; webkitExitFullscreen?: () => void }
    const el = document.documentElement as HTMLElement & { webkitRequestFullscreen?: () => void }
    if (d.fullscreenElement || d.webkitFullscreenElement) d.exitFullscreen ? d.exitFullscreen() : d.webkitExitFullscreen && d.webkitExitFullscreen()
    else if (el.requestFullscreen) el.requestFullscreen().catch(() => {})
    else if (el.webkitRequestFullscreen) el.webkitRequestFullscreen()
  }
  const gemeinsamKnopf: Knopf | null = live
    ? null
    : gemeinsam
      ? { id: 'leiste', label: t('player.gemeinsam') + (gemeinsam.leute ? ' · ' + gemeinsam.leute : ''), icon: 'gemeinsam', rechts: true, offen: leiste, tun: () => setLeiste(!leiste) }
      : { id: 'gemeinsam', label: t('player.gemeinsam'), icon: 'gemeinsam', rechts: true, tun: () => gemeinsamStarten(id, video.current ? video.current.currentTime : 0) }

  // Knopfleiste: links Transport, rechts Menüs. Auf dem TV per ↓ erreichbar, ←/→ laufen über alle.
  const mitKapiteln = kap.length > 1
  const knoepfe: Knopf[] = []
  if (!live && (mitKapiteln || vorige))
    knoepfe.push({ id: 'vorher', label: t(mitKapiteln ? 'player.kapitel.vorher' : 'player.vorherige'), icon: 'vorherige', tun: zurueckKapitel })
  if (!live) knoepfe.push({ id: 'z10', label: t('player.zurueck10'), icon: 'neustart', zahl: '10', tun: () => springe(zeit.t - 10) })
  const playIndex = knoepfe.length
  knoepfe.push({ id: 'play', label: t(zeit.paused ? 'player.abspielen' : 'player.pause'), icon: zeit.paused ? 'abspielen' : 'pause', gross: true, tun: umschalten })
  if (!live) knoepfe.push({ id: 'v30', label: t('player.vor30'), icon: 'vorspulen', zahl: '30', tun: () => springe(zeit.t + 30) })
  if (!live && (mitKapiteln || naechste))
    knoepfe.push({ id: 'nach', label: t(mitKapiteln ? 'player.kapitel.nach' : 'player.naechster'), icon: 'naechste', tun: vorKapitel })
  if (!live) {
    knoepfe.push({ id: 'spuren', label: t('player.menue'), icon: 'untertitel', rechts: true, offen: !!menue && menue.art === 'spuren', tun: () => menueUm('spuren') })
    knoepfe.push({ id: 'einst', label: t('player.einst'), icon: 'einstellungen', rechts: true, offen: !!menue && menue.art === 'einst', tun: () => menueUm('einst') })
  }
  if (tv && gemeinsamKnopf) knoepfe.push(gemeinsamKnopf) // Desktop/Handy: oben im Kopf
  if (tv && plan && plan.reasons && plan.reasons.length) knoepfe.push({ id: 'warum', label: t('player.warum'), icon: 'info', rechts: true, tun: () => setWarum(true) })
  if (!tv) knoepfe.push({ id: 'voll', label: t('player.vollbild'), icon: 'vollbild', rechts: true, tun: vollbild })

  // Zurück: erst Erklärung, Menü, Knopfleiste, Karte schließen; sonst eine Seite zurück (Router).
  const nochmal = () => setVersuch(versuch + 1)
  const ueberspringen = () => vorspann && springe(vorspann.ende)
  const st = useRef({ menue, reihe, warum, karte, fehler, spalten, knoepfe, umschalten, springe, weiter, menueAuf, nochmal, vorspann, ueberspringen, vollbild, playIndex })
  st.current = { menue, reihe, warum, karte, fehler, spalten, knoepfe, umschalten, springe, weiter, menueAuf, nochmal, vorspann, ueberspringen, vollbild, playIndex }
  const zurueck = useRef(() => {
    const s = st.current
    if (s.warum) return setWarum(false), true
    if (s.menue) return setMenue(null), true
    if (s.reihe !== null) return setReihe(null), true
    if (s.karte === 'zeigen') return setKarte('weg'), true
    return false
  }).current
  useZurueck(zurueck)

  useEffect(() => {
    // Ausblenden: nach 3 s ohne Mausbewegung (Taste/Touch: 5 s) Bedienung weg und Zeiger weg (cursor: none).
    // Steht die Maus über Kopf, Knopfleiste, Menü oder Leiste, bleibt alles sichtbar. In der Pause und bei offenem
    // Menü bleibt die Bedienung, der Zeiger über dem Bild verschwindet trotzdem.
    // TV: Der Zeiger der Magic Remote verschwindet von selbst ohne mouseleave; dort hält „über der Leiste“ nichts fest.
    let verstecken = 0
    let ueber = false
    let mx = -1
    let my = -1
    const wecken = (ms = 5000) => {
      setOsd(true)
      clearTimeout(verstecken)
      verstecken = window.setTimeout(() => {
        if (ueber && !istTV()) return wecken(3000)
        setZeigerWeg(true)
        const s = st.current
        if (s.menue || s.warum) return
        setOsd(false)
        setReihe(null)
      }, ms)
    }
    // Chrome schickt beim Cursor-Wechsel ein mousemove ohne Bewegung; nur echte Positionsänderungen zählen.
    const maus = (e: MouseEvent) => {
      if (e.clientX === mx && e.clientY === my) return
      mx = e.clientX
      my = e.clientY
      const z = e.target as Element | null
      ueber = !!(z && z.closest && z.closest('.pl-oben, .pl-unten, .pl-menue, .pl-blatt, .pl-naechste, .pl-ueberspringen, .party'))
      setZeigerWeg(false)
      wecken(3000)
    }
    const tipp = () => wecken()
    const onKey = (e: KeyboardEvent) => {
      const ziel = e.target as HTMLElement
      if (ziel && (ziel.tagName === 'INPUT' || ziel.tagName === 'TEXTAREA')) return // Chat-Feld
      const s = st.current
      const k = e.keyCode
      wecken()
      if (s.warum) {
        if (k === 13) setWarum(false)
        return
      }
      if (s.fehler) {
        if (k === 13) {
          e.preventDefault()
          s.nochmal()
        }
        return
      }
      if (s.menue) {
        const m = s.menue
        const col = s.spalten[m.col]
        if (k === 38) setMenue({ ...m, row: Math.max(0, m.row - 1) })
        else if (k === 40) setMenue({ ...m, row: Math.min(col.eintraege.length - 1, m.row + 1) })
        else if (k === 37 || k === 39) {
          const c = Math.max(0, Math.min(s.spalten.length - 1, m.col + (k === 37 ? -1 : 1)))
          setMenue({ ...m, col: c, row: Math.min(m.row, s.spalten[c].eintraege.length - 1) })
        } else if (k === 13) {
          const x = col.eintraege[m.row]
          x.tun()
          if (!x.schalter) setMenue(null) // Schalter lassen das Menü offen
        } else return
        e.preventDefault()
        return
      }
      if (s.karte === 'zeigen' && k === 13) {
        e.preventDefault()
        return s.weiter()
      }
      if (s.reihe !== null) {
        if (k === 37) setReihe(Math.max(0, s.reihe - 1))
        else if (k === 39) setReihe(Math.min(s.knoepfe.length - 1, s.reihe + 1))
        else if (k === 38) setReihe(null)
        else if (k === 13) s.knoepfe[s.reihe].tun()
        else if (k !== 40) return
        e.preventDefault()
        return
      }
      // Auf Desktop/Handy bedient Enter/Leertaste einen per Tab fokussierten Knopf selbst.
      if ((k === 13 || k === 32) && ziel && ziel.tagName === 'BUTTON') return
      // Im Vorspann hat „Vorspann überspringen“ den Fokus: OK springt.
      if (k === 13 && s.vorspann) {
        e.preventDefault()
        return s.ueberspringen()
      }
      const v = video.current!
      const schritt = e.repeat ? 30 : 10
      switch (k) {
        case 13: case 32: case 415: case 19: case 463: case 10252: s.umschalten(); break // 10252 = Samsung Play/Pause
        case 413: back(); break // Stopp
        case 37: case 412: s.springe(v.currentTime - schritt); break
        case 39: case 417: s.springe(v.currentTime + schritt); break
        case 38: s.menueAuf(); break
        case 40: setReihe(s.playIndex); break
        case 70: if (!istTV()) s.vollbild(); break // F
        default: return
      }
      e.preventDefault()
    }
    wecken()
    window.addEventListener('keydown', onKey)
    window.addEventListener('mousemove', maus)
    window.addEventListener('touchstart', tipp)
    window.addEventListener('mousedown', tipp) // TV: OK der Magic Remote bei ruhendem Zeiger
    return () => {
      window.removeEventListener('keydown', onKey)
      window.removeEventListener('mousemove', maus)
      window.removeEventListener('touchstart', tipp)
      window.removeEventListener('mousedown', tipp)
      clearTimeout(verstecken)
    }
  }, [])

  const sichtbar = osd || !!menue || zeit.paused || warum || reihe !== null
  const anteil = zeit.d ? Math.min(1, zeit.t / zeit.d) : 0
  const it = titel
  const kopf = it && it.series ? it.series : it ? anzeigeTitel(it) : plan ? plan.title : ''
  const unter = live ? p.unter || '' : it && it.series ? folge(it.season, it.episode) + ' · ' + it.title : ''
  // Auf dem TV zeigt der Play-Knopf den Fokus, solange OK Pause/Abspielen bedeutet (im Vorspann: Überspringen).
  const fokusKnopf = reihe !== null ? reihe : tv && !vorspann ? playIndex : -1
  const stelle = (e: MouseEvent) => {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    return Math.max(0, Math.min(1, (e.clientX - r.left) / r.width))
  }
  const scale = (x: number) => ({ transform: 'scaleX(' + x + ')', webkitTransform: 'scaleX(' + x + ')' })
  const schieb = (x: number) => ({ transform: 'translateX(' + x * 100 + '%)', webkitTransform: 'translateX(' + x * 100 + '%)' })
  const mitLeiste = !!p.seitenleiste && leiste
  // Segmente der Zeitleiste: je Kapitel eins (2 px Lücke), sonst eins über die ganze Länge.
  const segmente = mitKapiteln ? kap : [{ start: 0, ende: zeit.d, name: '' }]
  const anteilIn = (x: number, s: { start: number; ende: number }) => (s.ende > s.start ? Math.max(0, Math.min(1, (x - s.start) / (s.ende - s.start))) : 0)
  // Vorschau über der Zeitleiste: Maus (Desktop) bzw. aktuelle Stelle (TV, beim Spulen).
  const vorschauAn = zeiger !== null ? zeiger : tv && sichtbar && zeit.d ? anteil : null
  const vorschauZeit = vorschauAn !== null ? vorschauAn * zeit.d : 0
  const vorschauKap = mitKapiteln ? kap.filter((k) => vorschauZeit >= k.start && vorschauZeit < k.ende)[0] : null

  const knopf = (k: Knopf, i: number) => (
    <button
      key={k.id}
      type="button"
      class={'pl-knopf' + (k.gross ? ' pl-gross' : '') + (k.offen ? ' an' : '') + (i === fokusKnopf ? ' ist-fokus' : '')}
      aria-label={k.label}
      title={k.label}
      aria-expanded={k.offen !== undefined ? k.offen : undefined}
      onMouseDown={(e) => e.preventDefault()} // Maus hinterlässt keinen DOM-Fokus (sonst doppelt mit Enter)
      onClick={k.tun}
    >
      <Icon name={k.icon} />
      {k.zahl && (
        <span class="pl-knopf-zahl" aria-hidden="true">
          {k.zahl}
        </span>
      )}
    </button>
  )
  const links = knoepfe.filter((k) => !k.rechts)

  return (
    <div class={'pl' + (sichtbar ? ' pl-sichtbar' : '') + (mitLeiste ? ' pl-mit-leiste' : '') + (zeigerWeg ? ' pl-zeiger-weg' : '')}>
      <video
        ref={video}
        playsInline
        crossOrigin="anonymous"
        style={lage ? { left: lage.links + 'px', top: lage.oben + 'px', width: lage.breite + 'px', height: lage.hoehe + 'px', objectFit: 'fill' } : undefined}
        // Handy: Tippen blendet ein/aus. TV: Klick (Magic Remote) weckt nur die Bedienung, pausiert nicht.
        onClick={() => (istHandy() ? setOsd(!osd) : !tv && umschalten())}
        onDblClick={vollbild}
      >
        {subs
          .filter((s) => s.format === 'vtt')
          .map((s) => (
            // key mit URL: nach Tonwechsel neu einhängen, sonst hängt Chrome die Cues doppelt an
            <track key={s.index + (plan ? plan.url : '')} id={'ut' + s.index} kind="subtitles" srcLang={s.language} label={untertitelName(s)} src={untertitelUrl(id, s)} />
          ))}
      </video>
      {utZeilen.length > 0 && (
        <div class="pl-ut" aria-hidden="true">
          {utZeilen.map((z, i) => (
            <span key={i}>
              <span class="pl-ut-zeile">{z}</span>
              <br />
            </span>
          ))}
        </div>
      )}

      <header class="pl-oben zeile">
        <button class="pl-knopf" type="button" onClick={() => back()} aria-label={t('knopf.zurueck')} title={t('knopf.zurueck')}>
          <Icon name="zurueck" />
        </button>
        <div class="pl-titel wachse">
          <h1 class="pl-titel-text eine-zeile">{kopf}</h1>
          {unter && <p class="pl-unter eine-zeile">{unter}</p>}
          {plan && !live && (
            <p class="pl-methode fl-ampel-zeile">
              <AmpelPunkt stufe={plan.light} />
              <span>
                {t('player.methode.' + plan.method)}
                {plan.optimized ? ' · ' + t('player.optimiert') : ''}
              </span>
              {plan.reasons && plan.reasons.length > 0 && !tv && (
                <button type="button" class="pl-link" onClick={() => setWarum(true)}>
                  {t('player.warum')}
                </button>
              )}
            </p>
          )}
          {live && <p class="pl-methode fl-label">{t('player.live')}</p>}
          {plan && plan.notes && plan.notes.map((n) => <p class="pl-hinweis" key={n}>{n}</p>)}
        </div>
        {tv && <span class="pl-uhr fl-zahl">{endetUm(0)}</span>}
        {!tv && gemeinsamKnopf && knopf(gemeinsamKnopf, -1)}
      </header>

      {vorspann && !menue && karte !== 'zeigen' && (
        <button type="button" class={'pl-ueberspringen' + (tv && reihe === null ? ' ist-fokus' : '')} onClick={ueberspringen}>
          {t('player.vorspann')}
          <Icon name="weiter" />
        </button>
      )}

      <div class="pl-unten">
        {!live && (
          <div
            class="pl-bahn"
            role="slider"
            tabIndex={-1}
            aria-label={t('player.zeitleiste')}
            aria-valuemin={0}
            aria-valuemax={Math.round(zeit.d)}
            aria-valuenow={Math.round(zeit.t)}
            aria-valuetext={uhr(zeit.t) + ' / ' + uhr(zeit.d)}
            onClick={(e) => zeit.d && springe(stelle(e) * zeit.d)}
            onMouseMove={tv ? undefined : (e) => setZeiger(stelle(e))}
            onMouseLeave={() => setZeiger(null)}
          >
            <div class="pl-segmente">
              {segmente.map((s, i) => (
                <div key={i} class="pl-seg" style={{ flex: s.ende - s.start + ' 1 0px', webkitFlex: s.ende - s.start + ' 1 0px' }}>
                  <i class="pl-seg-puffer" style={scale(anteilIn(zeit.b, s))} />
                  <i class="pl-seg-voll" style={scale(anteilIn(zeit.t, s))} />
                </div>
              ))}
            </div>
            <div class="pl-schiene" style={schieb(anteil)}>
              <span class="pl-bahn-kopf" />
            </div>
            {vorschauAn !== null && (
              <div class="pl-schiene" style={schieb(vorschauAn)}>
                {zeiger !== null && <span class="pl-zeiger" />}
                {/* Links am Anfang, rechts am Ende bündig: bleibt immer über der Leiste statt halb aus dem Bild. */}
                <span class="pl-vorschau fl-zahl" style={{ transform: 'translateX(' + -vorschauAn * 100 + '%)', webkitTransform: 'translateX(' + -vorschauAn * 100 + '%)' }}>
                  {uhr(vorschauZeit)}
                  {vorschauKap ? ' · ' + vorschauKap.name : ''}
                </span>
              </div>
            )}
          </div>
        )}
        <div class="zeile pl-leiste">
          <div class="zeile pl-links">{links.map((k, i) => knopf(k, i))}</div>
          {!live && (
            <div class="zeile pl-zeitinfo">
              <span class="pl-zeit">
                {uhr(zeit.t)}
                <span class="pl-leise"> / {uhr(zeit.d)}</span>
              </span>
              {zeit.d > 0 && <span class="pl-endet pl-leise">{t('player.endet', { zeit: endetUm(rest / (gemeinsam ? raumTempo : tempo)) })}</span>}
            </div>
          )}
          <span class="wachse" />
          <div class="zeile pl-rechts">{knoepfe.slice(links.length).map((k, i) => knopf(k, links.length + i))}</div>
        </div>
      </div>

      {zeit.waiting && !fehler && <div class="pl-laedt" role="status">{t('player.laedt')}</div>}

      {menue && (
        <div class="fl-panel fl-spurmenue pl-menue" role="dialog" aria-label={t(menue.art === 'einst' ? 'player.einst' : 'player.menue')}>
          {spalten.map((c, ci) => (
            <div class="spalte" key={c.titel}>
              <h4>{c.titel}</h4>
              {c.eintraege.map((x, ri) => (
                <div key={x.label}>
                  {x.trenner && <div class="trenner" />}
                  <button
                    type="button"
                    class={'eintrag' + (menue.col === ci && menue.row === ri ? ' ist-fokus' : '') + (x.aktiv && !x.schalter ? ' gewaehlt' : '')}
                    aria-pressed={x.aktiv}
                    onClick={() => {
                      x.tun()
                      if (!x.schalter) setMenue(null)
                    }}
                    onMouseEnter={() => setMenue({ ...menue, col: ci, row: ri })}
                  >
                    {!x.schalter && <Icon name="haken" />}
                    <span class="wachse">
                      {x.label}
                      {x.unter && <small>{x.unter}</small>}
                    </span>
                    {x.schalter && (
                      <span class={'fl-schalter' + (x.aktiv ? ' an' : '')}>
                        <i />
                      </span>
                    )}
                  </button>
                </div>
              ))}
            </div>
          ))}
        </div>
      )}

      {warum && plan && (
        <div class="fl-panel pl-blatt" role="dialog" aria-label={t('player.warum.titel')}>
          <h2>{t('player.warum.titel')}</h2>
          <ul>
            {(plan.reasons || []).map((r) => <li key={r}>{r}</li>)}
          </ul>
          <p class="pl-zeit pl-leise">
            {[plan.videoCodec && 'Video ' + plan.videoCodec, plan.audioCodec && t('player.ton') + ' ' + plan.audioCodec].filter(Boolean).join(' · ')}
          </p>
          <button type="button" class="fl-btn primaer ist-fokus" onClick={() => setWarum(false)}>
            {t('knopf.zurueck')}
          </button>
        </div>
      )}

      {karte === 'zeigen' && naechste && (
        <section class="fl-panel pl-naechste" role="dialog" aria-labelledby="pl-naechste-titel">
          <div class="pl-n-bild" style={naechste.backdrop ? { backgroundImage: 'url(' + bild(naechste, 'backdrop', 480) + ')' } : undefined}>
            {!naechste.backdrop && <div class="pl-n-serie">{naechste.series || anzeigeTitel(naechste)}</div>}
            {naechste.series && <span class="pl-n-folge fl-zahl">{folge(naechste.season, naechste.episode)}</span>}
            <span class="pl-n-ampel">
              <AmpelPunkt stufe={naechste.light} />
            </span>
            <div class="pl-n-zaehler">
              <i style={scale((10 - countdown) / 10)} />
            </div>
          </div>
          <div class="pl-n-text">
            <span id="pl-naechste-titel" class="fl-label">
              {t('player.naechste.als')}
            </span>
            <b class="eine-zeile">{naechste.series ? folge(naechste.season, naechste.episode) + ' · ' + naechste.title : anzeigeTitel(naechste)}</b>
            {naechste.duration > 0 && (
              <span class="pl-leise">
                {dauer(naechste.duration)} · {t('player.endet', { zeit: endetUm(naechste.duration + countdown) })}
              </span>
            )}
            <div class="zeile pl-n-countdown">
              <span class="pl-n-zahl fl-zahl">{countdown}</span>
              <span>
                {t(naechste.series ? 'player.naechste.in' : 'player.naechste.titel.in', { n: countdown })}
                <small>{t('player.naechste.auto')}</small>
              </span>
            </div>
            <div class="zeile pl-n-knoepfe">
              <button type="button" class="fl-btn primaer ist-fokus wachse" onClick={weiter}>
                <Icon name="abspielen" />
                {t('player.naechste.jetzt')}
              </button>
              <button type="button" class="fl-btn" onClick={() => setKarte('weg')}>
                {t('player.naechste.verbergen')}
              </button>
            </div>
          </div>
        </section>
      )}

      {fehler && (
        <div class="pl-fehler" role="alert">
          <Icon name="fehler" />
          <h2>{t('player.fehler.titel')}</h2>
          <p>{fehler.text}</p>
          {fehler.detail && <p class="pl-zeit pl-leise">{fehler.detail}</p>}
          <div class="zeile">
            <button type="button" class="fl-btn primaer ist-fokus" onClick={nochmal}>
              <Icon name="neustart" />
              {t('knopf.nochmal')}
            </button>
            <button type="button" class="fl-btn" onClick={() => back()}>
              {t('knopf.zurueck')}
            </button>
          </div>
        </div>
      )}

      {mitLeiste && p.seitenleiste && p.seitenleiste(() => setLeiste(false))}
    </div>
  )
}
