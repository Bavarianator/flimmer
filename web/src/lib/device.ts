// Geräteklasse und Thema. Setzt html.tv / html.dt / html.hd (Typo-Skala und Maße aus tokens.css)
// und data-theme. Zum Testen am Rechner: ?geraet=tv an die URL hängen. Dauerhaft: Einstellungen › Anzeige › Ansicht
// (localStorage), z. B. für den LG-Browser, der sich als Desktop-Chrome meldet (kein „Web0S“ im User-Agent).
import { isTV, isTizen, isWebOS } from '../profile'

export { isTV, isTizen, isWebOS }

export type Geraet = 'tv' | 'dt' | 'hd'
export type Thema = 'kino' | 'hell' | 'system'

export function geraetWahl(): Geraet | 'auto' {
  try {
    const g = localStorage.getItem('flimmer.geraet')
    return g === 'tv' || g === 'dt' || g === 'hd' ? g : 'auto'
  } catch {
    return 'auto'
  }
}

// Neu laden: Fokus-System und Seiten lesen die Geräteklasse beim Start.
export function setGeraetWahl(g: Geraet | 'auto') {
  try {
    if (g === 'auto') localStorage.removeItem('flimmer.geraet')
    else localStorage.setItem('flimmer.geraet', g)
  } catch {}
  location.reload()
}

const wahl = geraetWahl()
const erzwungen = ((/[?&]geraet=(tv|dt|hd)/.exec(location.search) || [])[1] as Geraet | undefined) || (wahl === 'auto' ? undefined : wahl)

function messen(): Geraet {
  if (erzwungen) return erzwungen
  if (isTV) return 'tv'
  return window.innerWidth < 600 ? 'hd' : 'dt'
}

export let geraet: Geraet = messen()
export const istTV = () => geraet === 'tv'
export const istHandy = () => geraet === 'hd'
// Touch-Gerät ohne Fernbedienung: kein Scale, natives Wischen.
export const touch = 'ontouchstart' in window

const lauscher: ((g: Geraet) => void)[] = []
export function beiGeraetWechsel(fn: (g: Geraet) => void) {
  lauscher.push(fn)
  return () => lauscher.splice(lauscher.indexOf(fn), 1)
}

function anwenden() {
  const h = document.documentElement
  h.className = h.className.replace(/\b(tv|dt|hd)\b/g, '').trim() + ' ' + geraet
  const t = thema()
  const hell = geraet !== 'tv' && (t === 'hell' || (t === 'system' && !!window.matchMedia && matchMedia('(prefers-color-scheme: light)').matches))
  h.setAttribute('data-theme', hell ? 'hell' : 'kino')
  // TV: Die Maße sind für 1920 × 1080 gebaut. TV-Browser mit eigenem Layout-Viewport (LG: 960) skalieren damit
  // auf jede Bildschirmgröße; Browser ohne Viewport-Unterstützung ignorieren die Angabe.
  const vp = document.querySelector('meta[name=viewport]')
  if (vp) vp.setAttribute('content', geraet === 'tv' ? 'width=1920' : 'width=device-width, initial-scale=1')
}

export function thema(): Thema {
  try {
    return (localStorage.getItem('flimmer.thema') as Thema) || 'kino'
  } catch {
    return 'kino'
  }
}

export function setThema(t: Thema) {
  try {
    localStorage.setItem('flimmer.thema', t)
  } catch {}
  anwenden()
}

anwenden()
window.addEventListener('resize', () => {
  const g = messen()
  if (g === geraet) return
  geraet = g
  anwenden()
  lauscher.slice().forEach((fn) => fn(g))
})
