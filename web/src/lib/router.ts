// Hash-Router mit Zurück-Stapel. Hash-Routing läuft auf jedem TV-Browser und in App-Hüllen ohne
// Server-Rewrites. Zurück-Tasten: webOS 461, Tizen 10009, Escape 27, Backspace 8 (nicht in Feldern).
import { useEffect, useState } from 'preact/hooks'

export const ZURUECK_TASTEN = [461, 10009, 27, 8]

export function aktuell(): string {
  return location.hash.replace(/^#/, '') || '/'
}

// '/serie/Dark' → ['serie', 'Dark']
export function teile(pfad = aktuell()): string[] {
  return pfad
    .replace(/^\/+/, '')
    .split('/')
    .filter((x, i) => x || i === 0)
    .map(decodeURIComponent)
}

// Eigener Stapel, weil history.length auf TVs nichts über die App verrät.
const stapel: string[] = [aktuell()]
let ersetzt = false

export function go(pfad: string, opt: { ersetzen?: boolean } = {}) {
  if (pfad === aktuell()) return
  if (opt.ersetzen) {
    ersetzt = true
    location.replace('#' + pfad)
  } else location.hash = '#' + pfad
}

export function kannZurueck(): boolean {
  return stapel.length > 1
}

export function back() {
  if (kannZurueck()) history.back()
  else go('/', { ersetzen: true })
}

// Baut einen Pfad mit kodierten Teilen: pfad('serie', 'Haus des Geldes') → '/serie/Haus%20des%20Geldes'
export function pfad(...t: (string | number)[]): string {
  return '/' + t.map((x) => encodeURIComponent(String(x))).join('/')
}

const lauscher: ((p: string) => void)[] = []
window.addEventListener('hashchange', () => {
  const p = aktuell()
  if (ersetzt) {
    ersetzt = false
    stapel[stapel.length - 1] = p
  } else if (stapel.length > 1 && stapel[stapel.length - 2] === p) stapel.pop()
  else stapel.push(p)
  lauscher.slice().forEach((fn) => fn(p))
})

export function useRoute(): string[] {
  const [p, setP] = useState(aktuell())
  useEffect(() => {
    lauscher.push(setP)
    return () => {
      lauscher.splice(lauscher.indexOf(setP), 1)
    }
  }, [])
  return teile(p)
}

// ---------- Zurück-Taste ----------
// Handler bilden einen Stapel: der zuletzt angemeldete (z. B. ein offenes Menü) kommt zuerst dran.
// Gibt er true zurück, ist die Taste erledigt; sonst geht es eine Seite zurück.
type Handler = () => boolean | void
const handler: Handler[] = []

export function useZurueck(fn: Handler, aktiv = true) {
  useEffect(() => {
    if (!aktiv) return
    handler.push(fn)
    return () => {
      handler.splice(handler.lastIndexOf(fn), 1)
    }
  }, [fn, aktiv])
}

function istFeld(t: EventTarget | null) {
  const n = (t as HTMLElement | null) && (t as HTMLElement).tagName
  return n === 'INPUT' || n === 'TEXTAREA' || n === 'SELECT'
}

window.addEventListener('keydown', (e) => {
  const k = e.keyCode
  if (ZURUECK_TASTEN.indexOf(k) < 0) return
  if (k === 8 && istFeld(e.target)) return
  const top = handler[handler.length - 1]
  if (top) {
    e.preventDefault()
    if (top() === true) return
  }
  if (!kannZurueck() && aktuell() === '/') return // Startseite: Taste der TV-Hülle überlassen (App beenden)
  e.preventDefault()
  back()
})
