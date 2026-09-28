// Gemeinsam schauen im Player: Uhrenabgleich mit dem Server und Gleichlauf des <video> mit dem Raum.
import { useEffect } from 'preact/hooks'
import { correct, expected, median, offsetOf, type PartyState } from './sync'

export interface Gemeinsam {
  zustand: PartyState | null
  versatz: number // Server-Uhr minus eigene Uhr, ms
  aktion: (typ: string, daten?: Record<string, unknown>) => void
  reaktionen?: string[] // erscheinen als eigene Spalte im Ton-/Untertitelmenü
  leute?: number // Anzahl im Raum, steht am Knopf „Gemeinsam · 3“
}

// Median aus fünf Messungen gegen /api/time (NTP-artig, ein Ausreißer zählt nicht).
export async function messeVersatz(n = 5): Promise<number> {
  const xs: number[] = []
  for (let i = 0; i < n; i++) {
    const t0 = Date.now()
    try {
      const r = await fetch('/api/time', { cache: 'no-store', credentials: 'same-origin' })
      const { now } = await r.json()
      xs.push(offsetOf(t0, now, Date.now()))
    } catch {}
  }
  return xs.length ? median(xs) : 0
}

// Hält das Video am Raum: unter 0,3 s nichts, bis 1 s über playbackRate, darüber Sprung.
// Gesendet wird hier nur „buffering“; play/pause/seek schickt der Player bei Nutzeraktionen.
export function useGleichlauf(video: { current: HTMLVideoElement | null }, g: Gemeinsam | undefined) {
  const st = g && g.zustand
  const versatz = g ? g.versatz : 0
  useEffect(() => {
    const v = video.current
    if (!g || !st || !v) return
    const takt = () => {
      if (v.readyState < 1) return
      if (st.paused !== v.paused) st.paused ? v.pause() : v.play().catch(() => {})
      const soll = expected(st, Date.now() + versatz)
      const fix = correct(v.currentTime - soll, soll, st.rate || 1)
      if (fix.kind === 'seek' || (st.paused && fix.kind === 'rate')) v.currentTime = soll
      else v.playbackRate = fix.rate
    }
    takt()
    const id = setInterval(takt, 1000)
    v.addEventListener('loadedmetadata', takt)
    return () => {
      clearInterval(id)
      v.removeEventListener('loadedmetadata', takt)
    }
  }, [st, versatz])

  useEffect(() => {
    const v = video.current
    if (!g || !v) return
    let puffert = false
    const melde = (b: boolean) => {
      if (b === puffert) return
      puffert = b
      g.aktion('buffering', { buffering: b })
    }
    const an = () => melde(true)
    const aus = () => melde(false)
    v.addEventListener('waiting', an)
    v.addEventListener('playing', aus)
    v.addEventListener('canplay', aus)
    return () => {
      v.removeEventListener('waiting', an)
      v.removeEventListener('playing', aus)
      v.removeEventListener('canplay', aus)
    }
  }, [!!g])
}
