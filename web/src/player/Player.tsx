import { useEffect, useRef, useState } from 'preact/hooks'
import type Hls from 'hls.js/light'
import { api, getToken, isTV, profile } from '../profile'
import { probeSignal } from '../probe'
import { BACK_KEYS, fmtTime, library, resumePos } from '../data'
import { usePausedNav } from '../ui'

interface Subtitle {
  index: number
  language?: string
  title?: string
  format: 'vtt' | 'pgs'
  url?: string
}
interface AudioTrack {
  index: number
  language?: string
  title?: string
  codec: string
  channels?: number
  default?: boolean
}
interface Plan {
  method: string
  light: 'green' | 'yellow' | 'red'
  reasons?: string[]
  url: string
  title: string
  duration: number
  audioIndex: number
  audio: AudioTrack[] | null
  subtitles: Subtitle[] | null
  prefs?: { audio?: string; subtitle?: string } // pro Serie gemerkt
}

const methodLabel: Record<string, string> = {
  'direct-play': 'Direkt abgespielt',
  'direct-stream': 'Remux (Video + Ton kopiert)',
  'transcode-audio': 'Nur der Ton wird umgewandelt',
  transcode: 'Wird umgewandelt',
}

const langs: Record<string, string> = {
  ger: 'Deutsch', deu: 'Deutsch', de: 'Deutsch', eng: 'Englisch', en: 'Englisch', fre: 'Französisch', fra: 'Französisch',
  spa: 'Spanisch', ita: 'Italienisch', jpn: 'Japanisch', tur: 'Türkisch', pol: 'Polnisch', rus: 'Russisch', und: 'Unbekannt',
}
const lang = (c?: string) => (c ? langs[c] || c.toUpperCase() : '')

function subUrl(id: string, s: Subtitle) {
  return s.url || `/api/items/${id}/subs/${s.index}.${s.format === 'pgs' ? 'sup' : 'vtt'}`
}

function subLabel(s: Subtitle) {
  return [lang(s.language), s.title, s.format === 'pgs' ? '(Bild)' : ''].filter(Boolean).join(' · ') || `Spur ${s.index}`
}

function audioLabel(a: AudioTrack) {
  const ch = a.channels ? (a.channels > 2 ? a.channels - 1 + '.1' : a.channels === 1 ? 'Mono' : 'Stereo') : ''
  return [lang(a.language) || 'Spur ' + a.index, a.title, [a.codec.toUpperCase(), ch].filter(Boolean).join(' ')].filter(Boolean).join(' · ')
}

// Fortschritt an den Server; die lokale Kopie hilft, falls der Server gerade nicht erreichbar ist.
function saveProgress(id: string, pos: number, duration: number, audio: string, subtitle: string, beacon = false) {
  if (pos < 5) return
  try {
    localStorage.setItem('pos:' + id, String(Math.floor(pos)))
  } catch {}
  const body = JSON.stringify({ pos: Math.floor(pos), duration: Math.floor(duration), audio, subtitle })
  const url = `/api/items/${id}/progress`
  // sendBeacon überlebt das Schließen der App, kann aber keinen Bearer-Header (TVs) – dann fetch mit keepalive.
  if (beacon && navigator.sendBeacon && !getToken() && navigator.sendBeacon(url, body)) return
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (getToken()) headers.Authorization = 'Bearer ' + getToken()
  fetch(url, { method: 'POST', headers, body, credentials: 'same-origin', keepalive: true }).catch(() => {})
}

type Panel = { col: 0 | 1; row: number } | null

export function Player({ id, start, onBack }: { id: string; start?: number; onBack: () => void }) {
  const video = useRef<HTMLVideoElement>(null)
  const [plan, setPlan] = useState<Plan | null>(null)
  const [error, setError] = useState('')
  const [audioTrack, setAudioTrack] = useState(0) // 0 = automatisch (Server nimmt gemerkte Sprache/Standard)
  const [sub, setSub] = useState(-1)
  const [osd, setOsd] = useState(true)
  const [panel, setPanel] = useState<Panel>(null)
  const [time, setTime] = useState({ t: 0, d: 0, paused: false, waiting: false })
  const resumeAt = useRef(start !== undefined ? start : resumePos(id))
  const langRef = useRef({ audio: '', sub: 'off' })
  usePausedNav() // eigene Tastensteuerung

  // Plan holen und Quelle setzen – erneut, wenn eine andere Tonspur gewählt wird.
  useEffect(() => {
    let hls: Hls | undefined
    let cancelled = false
    probeSignal.aborted = true // Geräte-Test gibt den (oft einzigen) Hardware-Decoder frei
    const v = video.current!
    const resume = resumeAt.current
    setError('')
    api<Plan>(`/api/items/${id}/play`, { ...profile, audioTrack })
      .then(async (p) => {
        if (cancelled) return
        setPlan(p)
        const a = (p.audio || []).filter((x) => x.index === p.audioIndex)[0]
        langRef.current.audio = (a && a.language) || ''
        if (!audioTrack) {
          const want = p.prefs && p.prefs.subtitle
          const pre = want && want !== 'off' ? (p.subtitles || []).filter((s) => s.language === want)[0] : null
          if (pre) setSub(pre.index)
        }
        const isHls = p.url.indexOf('.m3u8') > 0
        if (!isHls || profile.nativeHls) {
          // Direct Play und natives HLS: die Videopipeline des Geräts puffert selbst – auf TVs der stabilste Weg.
          v.src = p.url
          if (resume > 0) v.currentTime = resume
        } else {
          const { default: HlsJs } = await import('hls.js/light')
          if (cancelled) return
          if (!HlsJs.isSupported()) throw new Error('Dieses Gerät kann kein HLS abspielen')
          // Puffer begrenzen: große Remux-Segmente sprengen sonst das MSE-Kontingent von TVs (→ Stocken).
          hls = new HlsJs({
            startPosition: resume,
            maxBufferLength: isTV ? 20 : 30,
            maxMaxBufferLength: isTV ? 30 : 120,
            backBufferLength: isTV ? 10 : 30,
            maxBufferSize: (isTV ? 60 : 120) * 1000 * 1000,
          })
          hls.on(HlsJs.Events.ERROR, (_e: unknown, d: { fatal: boolean; details: string }) => d.fatal && setError(`Wiedergabefehler: ${d.details}`))
          hls.loadSource(p.url)
          hls.attachMedia(v)
        }
        v.play().catch(() => {}) // Autoplay kann blockiert sein, dann startet der Nutzer selbst
      })
      .catch((e) => setError(String(e.message || e)))
    const save = () => saveProgress(id, v.currentTime, v.duration || 0, langRef.current.audio, langRef.current.sub)
    const tick = setInterval(() => !v.paused && save(), 10000)
    const onHide = () => saveProgress(id, v.currentTime, v.duration || 0, langRef.current.audio, langRef.current.sub, true)
    window.addEventListener('pagehide', onHide)
    document.addEventListener('visibilitychange', onHide)
    return () => {
      cancelled = true
      clearInterval(tick)
      resumeAt.current = v.currentTime || resume
      save()
      window.removeEventListener('pagehide', onHide)
      document.removeEventListener('visibilitychange', onHide)
      hls?.destroy()
      v.removeAttribute('src')
      v.load()
    }
  }, [id, audioTrack])

  // Beim Verlassen: Startseite („Weiterschauen“) neu laden.
  useEffect(() => () => void library(true), [])

  // Zeitanzeige
  useEffect(() => {
    const v = video.current!
    const upd = () => setTime({ t: v.currentTime, d: v.duration || (plan ? plan.duration : 0), paused: v.paused, waiting: v.readyState < 3 && !v.paused })
    const evs = ['timeupdate', 'play', 'pause', 'waiting', 'playing', 'durationchange']
    evs.forEach((e) => v.addEventListener(e, upd))
    return () => evs.forEach((e) => v.removeEventListener(e, upd))
  }, [plan])

  // PGS-Untertitel als Canvas-Overlay – niemals einbrennen (das erzwingt Transcoding und war ein Hauptgrund fürs Stocken).
  useEffect(() => {
    const s = (plan?.subtitles || []).filter((x) => x.index === sub)[0]
    langRef.current.sub = s ? s.language || 'und' : 'off'
    if (!s || s.format !== 'pgs') return
    let renderer: { dispose(): void } | undefined
    let cancelled = false
    Promise.all([import('libpgs'), import('libpgs/dist/libpgs.worker.js?url')]).then(([lib, worker]) => {
      if (cancelled) return
      renderer = new lib.PgsRenderer({ workerUrl: worker.default, video: video.current!, subUrl: subUrl(id, s) })
    })
    return () => {
      cancelled = true
      renderer?.dispose()
    }
  }, [plan, sub])

  // Text-Untertitel: nur die gewählte Spur anzeigen.
  useEffect(() => {
    const tracks = video.current?.textTracks
    if (!tracks) return
    for (let i = 0; i < tracks.length; i++) tracks[i].mode = tracks[i].id === 'sub' + sub ? 'showing' : 'hidden'
  }, [sub, plan])

  // Menü-Einträge: Spalte 0 = Ton, Spalte 1 = Untertitel (-1 = aus).
  const audios = (plan && plan.audio) || []
  const subs = (plan && plan.subtitles) || []
  const cols: number[][] = [audios.map((a) => a.index), [-1].concat(subs.map((s) => s.index))]
  const choose = (col: number, idx: number) => {
    if (col === 0) {
      if (plan && idx !== plan.audioIndex) setAudioTrack(idx)
    } else setSub(idx)
  }

  // Fernbedienung:
  //   ohne Menü: OK = Pause, ←/→ = 10 s, ↑ = Ton/Untertitel-Menü, ↓ = Leiste zeigen, Zurück = Seite verlassen
  //   im Menü:   ↑/↓ = Eintrag, ←/→ = Spalte, OK = wählen, Zurück = Menü schließen
  const st = useRef({ panel, cols, choose, onBack })
  st.current = { panel, cols, choose, onBack }
  useEffect(() => {
    let hide: number
    const poke = () => {
      setOsd(true)
      clearTimeout(hide)
      hide = window.setTimeout(() => !st.current.panel && setOsd(false), 4000)
    }
    const onKey = (e: KeyboardEvent) => {
      const v = video.current!
      const { panel, cols, choose, onBack } = st.current
      const k = e.keyCode
      poke()
      if (panel) {
        const col = cols[panel.col]
        if (k === 38) setPanel({ col: panel.col, row: Math.max(0, panel.row - 1) })
        else if (k === 40) setPanel({ col: panel.col, row: Math.min(col.length - 1, panel.row + 1) })
        else if ((k === 37 || k === 39) && cols[1 - panel.col].length) setPanel({ col: (1 - panel.col) as 0 | 1, row: 0 })
        else if (k === 13) {
          choose(panel.col, col[panel.row])
          setPanel(null)
        } else if (BACK_KEYS.indexOf(k) >= 0) setPanel(null)
        else return
        e.preventDefault()
        return
      }
      const seek = (d: number) => (v.currentTime = Math.max(0, Math.min(v.duration || Infinity, v.currentTime + d)))
      switch (k) {
        case 13: case 32: case 415: case 19: case 463: v.paused ? v.play() : v.pause(); break
        case 37: case 412: seek(-10); break
        case 39: case 417: seek(10); break
        case 38: if (cols[0].length > 1 || cols[1].length > 1) setPanel({ col: cols[0].length > 1 ? 0 : 1, row: 0 }); break
        case 40: break
        default:
          if (BACK_KEYS.indexOf(k) >= 0) onBack()
          return
      }
      e.preventDefault()
    }
    poke()
    window.addEventListener('keydown', onKey)
    window.addEventListener('mousemove', poke)
    return () => {
      window.removeEventListener('keydown', onKey)
      window.removeEventListener('mousemove', poke)
      clearTimeout(hide)
    }
  }, [])

  const selected = [plan ? plan.audioIndex : -1, sub]
  const hasMenu = audios.length > 1 || subs.length > 0
  return (
    <div class="player">
      <video ref={video} controls={!isTV} playsInline crossOrigin="anonymous" onClick={() => !isTV && video.current && (video.current.paused ? video.current.play() : video.current.pause())}>
        {subs
          .filter((s) => s.format === 'vtt')
          .map((s) => (
            // key mit URL: nach Tonwechsel neu einhängen, sonst hängt Chrome die Cues doppelt an
            <track key={s.index + (plan ? plan.url : '')} id={'sub' + s.index} kind="subtitles" srcLang={s.language} label={subLabel(s)} src={subUrl(id, s)} />
          ))}
      </video>
      <div class={'osd' + (osd || panel ? ' visible' : '')}>
        <button class="back" onClick={onBack}>← Zurück</button>
        {plan && (
          <div class="info">
            <h2>{plan.title}</h2>
            <span class={'light ' + plan.light} />
            <span>{methodLabel[plan.method]}</span>
            {plan.reasons && plan.reasons.length > 0 && <small> · {plan.reasons.join(' · ')}</small>}
          </div>
        )}
        {hasMenu && (
          <button class="back" onClick={() => setPanel(panel ? null : { col: audios.length > 1 ? 0 : 1, row: 0 })}>
            Ton & Untertitel
          </button>
        )}
      </div>
      {isTV && (
        <div class={'bar' + (osd || panel || time.paused ? ' visible' : '')}>
          <span>{fmtTime(time.t)}</span>
          <div class="track">
            <div style={{ width: (time.d ? (time.t / time.d) * 100 : 0) + '%' }} />
          </div>
          <span>{fmtTime(time.d)}</span>
          {hasMenu && <small>↑ Ton & Untertitel</small>}
        </div>
      )}
      {time.paused && isTV && !panel && <div class="big-state">❚❚</div>}
      {panel && (
        <div class="tracks">
          {[
            { title: 'Ton', items: audios.map((a) => ({ idx: a.index, label: audioLabel(a) })) },
            { title: 'Untertitel', items: [{ idx: -1, label: 'Aus' }].concat(subs.map((s) => ({ idx: s.index, label: subLabel(s) }))) },
          ].map((c, ci) => (
            <div class="col" key={c.title}>
              <h3>{c.title}</h3>
              {c.items.map((x, ri) => (
                <button
                  key={x.idx}
                  class={(panel.col === ci && panel.row === ri ? 'focused ' : '') + (selected[ci] === x.idx ? 'active' : '')}
                  onClick={() => {
                    choose(ci, x.idx)
                    setPanel(null)
                  }}
                >
                  {selected[ci] === x.idx ? '✓ ' : ''}
                  {x.label}
                </button>
              ))}
            </div>
          ))}
        </div>
      )}
      {error && <div class="error">{error}</div>}
    </div>
  )
}
