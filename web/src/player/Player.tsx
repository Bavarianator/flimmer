// Player: Wiedergabe (Direct Play, natives HLS oder hls.js mit TV-Puffergrenzen), PGS-Overlay,
// Ton-/Untertitelmenü mit Nachtmodus, eigene Zeitleiste, Nächste-Folge-Karte, Fortschritt alle 10 s.
// Tastatur/Fernbedienung steuert der Player selbst (D-Pad-Navigation ist pausiert):
//   Standard:     OK = Pause, ←/→ = 10 s (gehalten 30 s), ↑ = Ton & Untertitel, ↓ = Knopfleiste
//   Knopfleiste:  ←/→ = Knopf, OK = drücken, ↑ oder Zurück = zurück zum Standard
//   Menü:         ↑/↓ = Eintrag, ←/→ = Spalte, OK = wählen, Zurück = zu
import type { ComponentChildren } from 'preact'
import { useEffect, useRef, useState } from 'preact/hooks'
import type Hls from 'hls.js/light'
import { getToken, profile } from '../profile'
import { probeSignal } from '../probe'
import { anzeigeTitel, api, bild, bibliothek, fortsetzenAb, serien, titelAus, vergessen, type Item } from '../lib/api'
import { istTV, istHandy } from '../lib/device'
import { usePausierteNavigation } from '../lib/focus'
import { folge, t, uhr } from '../lib/i18n'
import { back, go, pfad, useZurueck } from '../lib/router'
import { Icon } from '../components/Icon'
import { AmpelPunkt } from '../components/Karte'
import { startUntertitel, tonName, untertitelName, untertitelUrl, type Plan } from './spuren'
import { setVorlieben, vorlieben } from './vorlieben'
import { useGleichlauf, type Gemeinsam } from './gemeinsam'
import './player.css'

// Fortschritt an den Server; die lokale Kopie hilft, falls der Server gerade nicht erreichbar ist.
function speichern(id: string, pos: number, dauer: number, ton: string, ut: string, beacon = false) {
  if (pos < 5) return
  try {
    localStorage.setItem('pos:' + id, String(Math.floor(pos)))
  } catch {}
  const it = titelAus(id)
  if (it) it.progress = Math.floor(pos)
  const body = JSON.stringify({ pos: Math.floor(pos), duration: Math.floor(dauer), audio: ton, subtitle: ut })
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

// Player-eigene Linien-Icons (24er-Raster, 2 px) für Sprünge und „Gemeinsam“.
const PFADE = {
  zurueck10: 'M4 12a8 8 0 1 0 2.3-5.7M4 4v4h4M9.5 10v5M13.75 10a1.75 1.75 0 0 1 1.75 1.75v1.5a1.75 1.75 0 0 1-3.5 0v-1.5a1.75 1.75 0 0 1 1.75-1.75z',
  vor10: 'M20 12a8 8 0 1 1-2.3-5.7M20 4v4h-4M9.5 10v5M13.75 10a1.75 1.75 0 0 1 1.75 1.75v1.5a1.75 1.75 0 0 1-3.5 0v-1.5a1.75 1.75 0 0 1 1.75-1.75z',
  gemeinsam: 'M9 12.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7zM17 12.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5zM3 19.5a6 6 0 0 1 12 0M14.5 15.5a4.5 4.5 0 0 1 6.5 4',
}
function Linie({ d }: { d: string }) {
  return (
    <svg class="icon fl-icon" viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">
      <path d={d} />
    </svg>
  )
}

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
  icon: ComponentChildren
  rund?: boolean
  gross?: boolean
  tun: () => void
}

type Menue = { col: number; row: number } | null

export function Player(p: {
  id: string
  start?: number
  onBack?: () => void // Zurück läuft über den Router (useZurueck); bleibt für den Aufrufer-Vertrag
  gemeinsam?: Gemeinsam
  seitenleiste?: ComponentChildren
}) {
  const { id, gemeinsam } = p
  const video = useRef<HTMLVideoElement>(null)
  const [plan, setPlan] = useState<Plan | null>(null)
  const [fehler, setFehler] = useState<{ text: string; detail: string } | null>(null)
  const [versuch, setVersuch] = useState(0)
  const [tonspur, setTonspur] = useState(0) // 0 = automatisch (Server nimmt Sprachkette/gemerkte Sprache)
  const [nacht, setNacht] = useState(vorlieben().night)
  const [ut, setUt] = useState(-2) // -2 = noch nicht gewählt, -1 = aus
  const [osd, setOsd] = useState(true)
  const [menue, setMenue] = useState<Menue>(null)
  const [reihe, setReihe] = useState<number | null>(null) // Fokus in der Knopfleiste (D-Pad)
  const [warum, setWarum] = useState(false)
  const [leiste, setLeiste] = useState(true) // Gemeinsam-Leiste sichtbar
  const [zeit, setZeit] = useState({ t: 0, d: 0, b: 0, paused: true, waiting: true })
  const [titel, setTitel] = useState<Item | undefined>(titelAus(id))
  const [naechste, setNaechste] = useState<Item | null>(null)
  const [karte, setKarte] = useState<'aus' | 'zeigen' | 'weg'>('aus')
  const [countdown, setCountdown] = useState(10)
  const resumeAt = useRef(p.start !== undefined ? p.start : fortsetzenAb(id))
  const sprachen = useRef({ ton: '', ut: 'off' })
  const tv = istTV()
  usePausierteNavigation()
  useGleichlauf(video, gemeinsam)

  // Titel und nächste Folge aus der (meist schon gecachten) Bibliothek.
  useEffect(() => {
    bibliothek().then((alle) => {
      const it = titelAus(id)
      setTitel(it)
      if (!it || !it.series) return
      const s = serien(alle).filter((x) => x.name === it.series)[0]
      const folgen = s ? s.staffeln.reduce<Item[]>((a, x) => a.concat(x.folgen), []) : []
      const i = folgen.map((x) => x.id).indexOf(id)
      setNaechste(i >= 0 && folgen[i + 1] ? folgen[i + 1] : null)
    }, () => {})
  }, [id])

  // Plan holen und Quelle setzen – erneut bei anderer Tonspur, Nachtmodus oder „Nochmal versuchen“.
  useEffect(() => {
    let hls: Hls | undefined
    let weg = false
    probeSignal.aborted = true // Geräte-Test gibt den (oft einzigen) Hardware-Decoder frei
    const v = video.current!
    const ab = resumeAt.current
    const vl = vorlieben()
    setFehler(null)
    api<Plan>(`/api/items/${id}/play`, { ...profile, audioTrack: tonspur, audioLangs: vl.audioLangs, subtitleMode: vl.subtitleMode, night: nacht })
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
            startPosition: ab,
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
      .catch((e) => !weg && setFehler({ text: t('player.fehler.server'), detail: String((e && e.message) || e) }))
    const sichern = () => speichern(id, v.currentTime, v.duration || 0, sprachen.current.ton, sprachen.current.ut)
    const takt = setInterval(() => !v.paused && sichern(), 10000)
    const beacon = () => speichern(id, v.currentTime, v.duration || 0, sprachen.current.ton, sprachen.current.ut, true)
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
  }, [id, tonspur, nacht, versuch])

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
      setZeit({ t: v.currentTime, d: v.duration || (plan ? plan.duration : 0), b, paused: v.paused, waiting: v.readyState < 3 && !v.paused })
    }
    const evs = ['timeupdate', 'play', 'pause', 'waiting', 'playing', 'durationchange', 'loadeddata', 'progress']
    evs.forEach((e) => v.addEventListener(e, upd))
    return () => evs.forEach((e) => v.removeEventListener(e, upd))
  }, [plan])

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

  // Text-Untertitel: nur die gewählte Spur anzeigen.
  useEffect(() => {
    const tracks = video.current && video.current.textTracks
    if (!tracks) return
    for (let i = 0; i < tracks.length; i++) tracks[i].mode = tracks[i].id === 'ut' + ut ? 'showing' : 'hidden'
  }, [ut, plan])

  // Nächste-Folge-Karte: im Abspann (letzte 30 s bzw. 10 %), zählt 10 s herunter.
  const rest = Math.max(0, zeit.d - zeit.t)
  const ende = zeit.d > 0 && rest <= Math.min(30, zeit.d * 0.1)
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

  const weiter = () => {
    if (!naechste) return
    if (gemeinsam) gemeinsam.aktion('media', { mediaId: naechste.id })
    else go(pfad('watch', naechste.id), { ersetzen: true }) // Zurück führt dann zur Detailseite, nicht zur alten Folge
  }

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
    const v = video.current!
    v.currentTime = Math.max(0, Math.min((v.duration || Infinity) - 1, ziel))
    if (gemeinsam) gemeinsam.aktion('seek', { pos: v.currentTime })
  }

  // Menü: Ton (mit Nachtmodus), Untertitel, beim gemeinsamen Schauen Reaktionen.
  const audios = (plan && plan.audio) || []
  const subs = (plan && plan.subtitles) || []
  const nachtUmschalten = () => {
    setVorlieben({ night: !nacht })
    setNacht(!nacht)
  }
  const spalten: Spalte[] = [
    {
      titel: t('player.ton'),
      eintraege: audios
        .map<Eintrag>((a) => ({ label: tonName(a), aktiv: !!plan && a.index === plan.audioIndex, tun: () => plan && a.index !== plan.audioIndex && setTonspur(a.index) }))
        .concat({ label: t('player.nachtmodus'), unter: t('player.nachtmodus.text'), aktiv: nacht, schalter: true, trenner: audios.length > 0, tun: nachtUmschalten }),
    },
  ]
  if (subs.length)
    spalten.push({
      titel: t('player.untertitel'),
      eintraege: [{ label: t('player.aus'), aktiv: ut < 0, tun: () => setUt(-1) } as Eintrag].concat(
        subs.map((s) => ({ label: untertitelName(s), aktiv: s.index === ut, tun: () => setUt(s.index) })),
      ),
    })
  if (gemeinsam && gemeinsam.reaktionen)
    spalten.push({ titel: t('player.reaktionen'), eintraege: gemeinsam.reaktionen.map((r) => ({ label: r, aktiv: false, tun: () => gemeinsam.aktion('reaction', { text: r }) })) })
  const menueAuf = () => setMenue({ col: subs.length && audios.length <= 1 ? 1 : 0, row: 0 })

  // Knopfleiste unten; auf dem TV per ↓ erreichbar.
  const knoepfe: Knopf[] = [
    { id: 'z10', label: t('player.zurueck10'), icon: <Linie d={PFADE.zurueck10} />, rund: true, tun: () => springe(zeit.t - 10) },
    { id: 'play', label: t(zeit.paused ? 'player.abspielen' : 'player.pause'), icon: <Icon name={zeit.paused ? 'abspielen' : 'pause'} class="fl-icon" />, rund: true, gross: true, tun: umschalten },
    { id: 'v10', label: t('player.vor10'), icon: <Linie d={PFADE.vor10} />, rund: true, tun: () => springe(zeit.t + 10) },
    { id: 'menue', label: t('player.menue'), icon: <Icon name="untertitel" class="fl-icon" />, tun: () => (menue ? setMenue(null) : menueAuf()) },
    gemeinsam
      ? { id: 'leiste', label: t('player.gemeinsam') + (gemeinsam.leute ? ' · ' + gemeinsam.leute : ''), icon: <Linie d={PFADE.gemeinsam} />, tun: () => setLeiste(!leiste) }
      : { id: 'gemeinsam', label: t('player.gemeinsam.kurz'), icon: <Linie d={PFADE.gemeinsam} />, tun: () => gemeinsamStarten(id, video.current ? video.current.currentTime : 0) },
  ]
  if (plan && plan.reasons && plan.reasons.length) knoepfe.push({ id: 'warum', label: t('player.warum'), icon: <Icon name="info" class="fl-icon" />, tun: () => setWarum(true) })
  const playIndex = 1

  // Zurück: erst Erklärung, Menü, Knopfleiste, Karte schließen; sonst eine Seite zurück (Router).
  const nochmal = () => setVersuch(versuch + 1)
  const st = useRef({ menue, reihe, warum, karte, fehler, spalten, knoepfe, umschalten, springe, weiter, menueAuf, nochmal })
  st.current = { menue, reihe, warum, karte, fehler, spalten, knoepfe, umschalten, springe, weiter, menueAuf, nochmal }
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
    let verstecken = 0
    const wecken = () => {
      setOsd(true)
      clearTimeout(verstecken)
      verstecken = window.setTimeout(() => {
        const s = st.current
        if (s.menue || s.warum) return
        setOsd(false)
        setReihe(null)
      }, 5000)
    }
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
        if (k === 38) setMenue({ col: m.col, row: Math.max(0, m.row - 1) })
        else if (k === 40) setMenue({ col: m.col, row: Math.min(col.eintraege.length - 1, m.row + 1) })
        else if (k === 37 || k === 39) {
          const c = Math.max(0, Math.min(s.spalten.length - 1, m.col + (k === 37 ? -1 : 1)))
          setMenue({ col: c, row: Math.min(m.row, s.spalten[c].eintraege.length - 1) })
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
      const v = video.current!
      const schritt = e.repeat ? 30 : 10
      switch (k) {
        case 13: case 32: case 415: case 19: case 463: case 10252: s.umschalten(); break // 10252 = Samsung Play/Pause
        case 413: back(); break // Stopp
        case 37: case 412: s.springe(v.currentTime - schritt); break
        case 39: case 417: s.springe(v.currentTime + schritt); break
        case 38: s.menueAuf(); break
        case 40: setReihe(playIndex); break
        default: return
      }
      e.preventDefault()
    }
    wecken()
    window.addEventListener('keydown', onKey)
    window.addEventListener('mousemove', wecken)
    window.addEventListener('touchstart', wecken)
    return () => {
      window.removeEventListener('keydown', onKey)
      window.removeEventListener('mousemove', wecken)
      window.removeEventListener('touchstart', wecken)
      clearTimeout(verstecken)
    }
  }, [])

  const sichtbar = osd || !!menue || zeit.paused || warum || reihe !== null
  const anteil = zeit.d ? Math.min(1, zeit.t / zeit.d) : 0
  const puffer = zeit.d ? Math.min(1, zeit.b / zeit.d) : 0
  const it = titel
  const kopf = it && it.series ? it.series : it ? anzeigeTitel(it) : plan ? plan.title : ''
  const unter = it && it.series ? folge(it.season, it.episode) + ' · ' + it.title : ''
  // Auf dem TV zeigt der Play-Knopf den Fokus, solange OK Pause/Abspielen bedeutet.
  const fokusKnopf = reihe !== null ? reihe : tv ? playIndex : -1
  const klickLeiste = (e: MouseEvent) => {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    if (zeit.d) springe(((e.clientX - r.left) / r.width) * zeit.d)
  }
  const vollbild = () => {
    const d = document as Document & { webkitFullscreenElement?: Element; webkitExitFullscreen?: () => void }
    const el = document.documentElement as HTMLElement & { webkitRequestFullscreen?: () => void }
    if (d.fullscreenElement || d.webkitFullscreenElement) d.exitFullscreen ? d.exitFullscreen() : d.webkitExitFullscreen && d.webkitExitFullscreen()
    else if (el.requestFullscreen) el.requestFullscreen().catch(() => {})
    else if (el.webkitRequestFullscreen) el.webkitRequestFullscreen()
  }
  const scale = (x: number) => ({ transform: 'scaleX(' + x + ')', webkitTransform: 'scaleX(' + x + ')' })
  const mitLeiste = !!p.seitenleiste && leiste

  return (
    <div class={'pl' + (sichtbar ? ' pl-sichtbar' : '') + (mitLeiste ? ' pl-mit-leiste' : '')}>
      <video ref={video} playsInline crossOrigin="anonymous" onClick={() => (istHandy() ? setOsd(!osd) : umschalten())} onDblClick={vollbild}>
        {subs
          .filter((s) => s.format === 'vtt')
          .map((s) => (
            // key mit URL: nach Tonwechsel neu einhängen, sonst hängt Chrome die Cues doppelt an
            <track key={s.index + (plan ? plan.url : '')} id={'ut' + s.index} kind="subtitles" srcLang={s.language} label={untertitelName(s)} src={untertitelUrl(id, s)} />
          ))}
      </video>

      <div class="pl-oben zeile">
        <button class="fl-btn rund pl-glas" type="button" onClick={() => back()} aria-label={t('knopf.zurueck')}>
          <Icon name="zurueck" class="fl-icon" />
        </button>
        <div class="pl-titel wachse">
          <h1 class="pl-titel-text eine-zeile">{kopf}</h1>
          {unter && <p class="pl-unter eine-zeile">{unter}</p>}
          {plan && (
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
          {plan && plan.notes && plan.notes.map((n) => <p class="pl-hinweis" key={n}>{n}</p>)}
        </div>
      </div>

      <div class="pl-unten">
        <div class="pl-bahn" role="slider" tabIndex={-1} aria-label={t('player.zeitleiste')} aria-valuemin={0} aria-valuemax={Math.round(zeit.d)} aria-valuenow={Math.round(zeit.t)} aria-valuetext={uhr(zeit.t)} onClick={klickLeiste}>
          <div class="pl-bahn-grund" />
          <div class="pl-bahn-puffer" style={scale(puffer)} />
          <div class="pl-bahn-voll" style={scale(anteil)} />
          <div class="pl-bahn-kopf-schiene" style={{ transform: 'translateX(' + anteil * 100 + '%)', webkitTransform: 'translateX(' + anteil * 100 + '%)' }}>
            <div class="pl-bahn-kopf" />
          </div>
        </div>
        <div class="zeile pl-zeiten">
          <span class="pl-zeit">{uhr(zeit.t)}</span>
          <span class="wachse" />
          <span class="pl-zeit pl-leise">
            −{uhr(rest)}
            {zeit.d > 0 ? ' · ' + t('player.endet', { zeit: endetUm(rest) }) : ''}
          </span>
        </div>
        <div class="zeile pl-knoepfe">
          {knoepfe.map((k, i) => (
            <button
              key={k.id}
              type="button"
              class={'fl-btn pl-glas' + (k.rund ? ' rund' : '') + (k.gross ? ' pl-gross primaer' : '') + (i === fokusKnopf ? ' ist-fokus' : '')}
              aria-label={k.rund ? k.label : undefined}
              aria-expanded={k.id === 'menue' ? !!menue : k.id === 'leiste' ? leiste : undefined}
              onMouseDown={(e) => e.preventDefault()} // Maus hinterlässt keinen DOM-Fokus (sonst doppelt mit Enter)
              onClick={k.tun}
            >
              {k.icon}
              {!k.rund && k.label}
            </button>
          ))}
        </div>
      </div>

      {zeit.waiting && !fehler && <div class="pl-laedt" role="status">{t('player.laedt')}</div>}

      {menue && (
        <div class="fl-panel fl-spurmenue pl-menue" role="dialog" aria-label={t('player.menue')}>
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
                    onMouseEnter={() => setMenue({ col: ci, row: ri })}
                  >
                    {!x.schalter && <Icon name="haken" class="fl-icon" />}
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
        <div class="fl-panel fl-naechste pl-naechste" role="dialog" aria-label={t('player.naechste')}>
          <div class="vorschau" style={naechste.backdrop ? { backgroundImage: 'url(' + bild(naechste, 'backdrop', 480) + ')' } : undefined}>
            <div class="zaehler">
              <i style={scale((10 - countdown) / 10)} />
            </div>
          </div>
          <div class="fl-grow">
            <small>{t('player.naechste.in', { n: countdown })}</small>
            <b class="eine-zeile">{folge(naechste.season, naechste.episode) + ' · ' + naechste.title}</b>
            <button type="button" class="fl-btn primaer ist-fokus" onClick={weiter}>
              <Icon name="abspielen" class="fl-icon" />
              {t('player.naechste.jetzt')}
            </button>
            <button type="button" class="fl-btn" onClick={() => setKarte('weg')}>
              {t('player.naechste.abspann')}
            </button>
          </div>
        </div>
      )}

      {fehler && (
        <div class="pl-fehler" role="alert">
          <Icon name="fehler" />
          <h2>{t('player.fehler.titel')}</h2>
          <p>{fehler.text}</p>
          {fehler.detail && <p class="pl-zeit pl-leise">{fehler.detail}</p>}
          <div class="zeile">
            <button type="button" class="fl-btn primaer ist-fokus" onClick={nochmal}>
              <Icon name="neustart" class="fl-icon" />
              {t('knopf.nochmal')}
            </button>
            <button type="button" class="fl-btn" onClick={() => back()}>
              {t('knopf.zurueck')}
            </button>
          </div>
        </div>
      )}

      {mitLeiste && p.seitenleiste}
    </div>
  )
}
