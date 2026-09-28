import { useEffect, useRef, useState } from 'preact/hooks'
import type Hls from 'hls.js/light'
import { api, getToken, isTV, profile } from '../profile'
import { probeSignal } from '../probe'
import { BACK_KEYS, library, resumePos } from '../data'

interface Subtitle { index: number; language?: string; title?: string; format: 'vtt' | 'pgs'; url?: string }
interface Plan {
  method: string
  light: 'green' | 'yellow' | 'red'
  reasons?: string[]
  url: string
  title: string
  duration: number
  subtitles: Subtitle[] | null
  prefs?: { audio?: string; subtitle?: string } // pro Serie gemerkt
}

const methodLabel: Record<string, string> = {
  'direct-play': 'Direkt abgespielt',
  'direct-stream': 'Remux (Video + Ton kopiert)',
  'transcode-audio': 'Nur Ton wird umgewandelt',
  transcode: 'Wird transkodiert',
}

function subUrl(id: string, s: Subtitle) {
  return s.url || `/api/items/${id}/subs/${s.index}.${s.format === 'pgs' ? 'sup' : 'vtt'}`
}

// Fortschritt an den Server; ohne Route/Anmeldung bleibt die lokale Kopie.
function saveProgress(id: string, pos: number, duration: number, subtitle: string, beacon = false) {
  if (pos < 5) return
  try {
    localStorage.setItem('pos:' + id, String(Math.floor(pos)))
  } catch {}
  const body = JSON.stringify({ pos: Math.floor(pos), duration: Math.floor(duration), subtitle })
  const url = `/api/items/${id}/progress`
  // sendBeacon überlebt das Schließen der App, kann aber keinen Bearer-Header (TVs) – dann fetch mit keepalive.
  if (beacon && navigator.sendBeacon && !getToken() && navigator.sendBeacon(url, body)) return
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (getToken()) headers.Authorization = 'Bearer ' + getToken()
  fetch(url, { method: 'POST', headers, body, credentials: 'same-origin', keepalive: true }).catch(() => {})
}

export function Player({ id, start, onBack }: { id: string; start?: number; onBack: () => void }) {
  const video = useRef<HTMLVideoElement>(null)
  const [plan, setPlan] = useState<Plan | null>(null)
  const [error, setError] = useState('')
  const [sub, setSub] = useState(-1)
  const [osd, setOsd] = useState(true)
  const subLangRef = useRef('off')

  // Plan holen und Quelle setzen.
  useEffect(() => {
    let hls: Hls | undefined
    let cancelled = false
    probeSignal.aborted = true // Geräte-Test gibt den (oft einzigen) Hardware-Decoder frei
    const v = video.current!
    const resume = start !== undefined ? start : resumePos(id)
    api<Plan>(`/api/items/${id}/play`, profile)
      .then(async (p) => {
        if (cancelled) return
        setPlan(p)
        const want = p.prefs && p.prefs.subtitle
        const pre = want && want !== 'off' && (p.subtitles || []).filter((s) => s.language === want)[0]
        if (pre) setSub(pre.index)
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
          hls.on(HlsJs.Events.ERROR, (_e, d) => d.fatal && setError(`Wiedergabefehler: ${d.details}`))
          hls.loadSource(p.url)
          hls.attachMedia(v)
        }
        v.play().catch(() => {}) // Autoplay kann blockiert sein, dann startet der Nutzer selbst
      })
      .catch((e) => setError(String(e)))
    const save = setInterval(() => !v.paused && saveProgress(id, v.currentTime, v.duration || 0, subLangRef.current), 10000)
    const onHide = () => saveProgress(id, v.currentTime, v.duration || 0, subLangRef.current, true)
    window.addEventListener('pagehide', onHide)
    document.addEventListener('visibilitychange', onHide)
    return () => {
      cancelled = true
      clearInterval(save)
      onHide()
      window.removeEventListener('pagehide', onHide)
      document.removeEventListener('visibilitychange', onHide)
      library(true) // Fortschritt/„Weiterschauen“ auf der Startseite aktualisieren
      hls?.destroy()
      v.removeAttribute('src')
      v.load()
    }
  }, [id])

  // PGS-Untertitel als Canvas-Overlay – niemals einbrennen (das erzwingt Transcoding und war ein Hauptgrund fürs Stocken).
  useEffect(() => {
    const s = plan?.subtitles?.find((x) => x.index === sub)
    subLangRef.current = s ? s.language || 'und' : 'off'
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

  // Fernbedienung: OK = Pause, links/rechts = 10 s spulen, oben/unten = 60 s, Zurück = Bibliothek.
  useEffect(() => {
    let hide: number
    const poke = () => {
      setOsd(true)
      clearTimeout(hide)
      hide = window.setTimeout(() => setOsd(false), 4000)
    }
    const onKey = (e: KeyboardEvent) => {
      const v = video.current!
      poke()
      const seek = (d: number) => (v.currentTime = Math.max(0, Math.min(v.duration || Infinity, v.currentTime + d)))
      switch (e.keyCode) {
        case 13: case 32: case 415: case 19: v.paused ? v.play() : v.pause(); break
        case 37: case 412: seek(-10); break
        case 39: case 417: seek(10); break
        case 38: seek(60); break
        case 40: seek(-60); break
        default:
          if (BACK_KEYS.indexOf(e.keyCode) >= 0) onBack()
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

  const subs = plan?.subtitles || []
  return (
    <div class="player">
      <video ref={video} controls={!isTV} playsInline crossOrigin="anonymous">
        {subs.filter((s) => s.format === 'vtt').map((s) => (
          <track id={'sub' + s.index} kind="subtitles" srcLang={s.language} label={label(s)} src={subUrl(id, s)} />
        ))}
      </video>
      <div class={'osd' + (osd ? ' visible' : '')}>
        <button class="back" onClick={onBack}>← Zurück</button>
        {plan && (
          <div class="info">
            <h2>{plan.title}</h2>
            <span class={'light ' + plan.light} />
            <span>{methodLabel[plan.method]}</span>
            {plan.reasons && plan.reasons.length > 0 && <small> · {plan.reasons.join(' · ')}</small>}
          </div>
        )}
        {subs.length > 0 && (
          <select value={sub} onChange={(e) => setSub(Number((e.target as HTMLSelectElement).value))}>
            <option value={-1}>Untertitel aus</option>
            {subs.map((s) => <option value={s.index}>{label(s)}</option>)}
          </select>
        )}
      </div>
      {error && <div class="error">{error}</div>}
    </div>
  )
}

function label(s: Subtitle) {
  return [s.language?.toUpperCase(), s.title, s.format === 'pgs' ? '(Bild)' : ''].filter(Boolean).join(' ') || `Spur ${s.index}`
}
