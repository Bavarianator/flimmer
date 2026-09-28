// Geräteklasse und Thema. Setzt html.tv / html.dt / html.hd (Typo-Skala und Maße aus tokens.css)
// und data-theme. Zum Testen am Rechner: ?geraet=tv an die URL hängen.
import { isTV, isTizen, isWebOS } from '../profile'

export { isTV, isTizen, isWebOS }

export type Geraet = 'tv' | 'dt' | 'hd'
export type Thema = 'kino' | 'hell' | 'system'

const erzwungen = (/[?&]geraet=(tv|dt|hd)/.exec(location.search) || [])[1] as Geraet | undefined

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
