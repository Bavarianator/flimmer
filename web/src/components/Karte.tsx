import { useEffect, useRef, useState } from 'preact/hooks'
import type { Ampel } from '../lib/api'
import { useFokus } from '../lib/focus'
import { t } from '../lib/i18n'
import { Icon } from './Icon'

const farbwort: Record<Ampel, string> = { green: 'gruen', yellow: 'gelb', red: 'rot' }

// Ampel als kleiner gedeckter Punkt; die Form trägt die Bedeutung (voll, halb, Ring).
export function AmpelPunkt({ stufe }: { stufe: Ampel }) {
  return <span class={'fl-ampel ' + farbwort[stufe]} aria-hidden="true" />
}

// Punkt mit Text daneben. kurz = „Läuft direkt“, sonst der ganze Satz.
export function AmpelZeile({ stufe, kurz }: { stufe: Ampel; kurz?: boolean }) {
  return (
    <div class="fl-ampel-zeile">
      <AmpelPunkt stufe={stufe} />
      {t((kurz ? 'ampel.kurz.' : 'ampel.') + stufe)}
    </div>
  )
}

// Badges oben rechts auf Bildern: Gesehen-Haken und Ampel, auf dunklem Grund (badge-grund).
export function Badges({ ampel, gesehen }: { ampel?: Ampel; gesehen?: boolean }) {
  if (!ampel && !gesehen) return null
  return (
    <div class="oben">
      {gesehen && (
        <span class="fl-badge" title={t('karte.gesehen')}>
          <Icon name="haken" class="haken" />
        </span>
      )}
      {ampel && (
        <span class="fl-badge" title={t('ampel.' + ampel)}>
          <AmpelPunkt stufe={ampel} />
        </span>
      )}
    </div>
  )
}

export function Fortschritt({ anteil }: { anteil: number }) {
  if (!(anteil > 0)) return null
  const tf = 'scaleX(' + Math.min(1, anteil) + ')'
  return (
    <div class="fl-fortschritt" aria-hidden="true">
      <i style={{ transform: tf, webkitTransform: tf }} />
    </div>
  )
}

// Gedeckter Ton für Platzhalter ohne item.color, aus dem Titel abgeleitet (warme Grautöne).
const toene = ['#3b3a33', '#3a4046', '#4a3a2c', '#33392f', '#40353a', '#2f3a3a']
export function tonFuer(s: string): string {
  let h = 0
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0
  return toene[Math.abs(h) % toene.length]
}

// Ein IntersectionObserver für alle Bilder: lädt erst kurz bevor das Bild sichtbar wird
// (loading="lazy" kennt Chromium 53 nicht). Ohne IO wird sofort geladen.
let io: IntersectionObserver | null = null
const wartend = new Map<Element, () => void>()
function beobachte(el: Element, fn: () => void) {
  if (!('IntersectionObserver' in window)) return fn()
  if (!io)
    io = new IntersectionObserver(
      (es) => {
        for (const e of es)
          if (e.isIntersecting || e.intersectionRatio > 0) {
            const f = wartend.get(e.target)
            wartend.delete(e.target)
            io!.unobserve(e.target)
            if (f) f()
          }
      },
      { rootMargin: '400px 800px' },
    )
  wartend.set(el, fn)
  io.observe(el)
  return () => {
    wartend.delete(el)
    if (io) io.unobserve(el)
  }
}

// Chromium 53 trennt nicht selbst: lange Wörter (ab 10 Zeichen) bekommen weiche Trennstellen (U+00AD)
// nach den Silbenregeln V|KV und VK|KV, spätestens nach 8 Zeichen, mindestens 3 Zeichen vom Wortrand. Sonst laufen
// Versalien-Titel auf Platzhaltern über den Rand.
const vokal = /[aeiouäöüyAEIOUÄÖÜY]/
export function trenne(titel: string): string {
  return titel.replace(/[^\s\-–]{10,}/g, (w) => {
    let out = ''
    let seit = 0
    for (let i = 0; i < w.length; i++) {
      out += w[i]
      seit++
      const rest = w.length - i - 1
      const v = (k: number) => vokal.test(w[k] || '')
      const silbe = (v(i) && !v(i + 1) && v(i + 2)) || (v(i - 1) && !v(i) && !v(i + 1) && v(i + 2)) // V|CV, VC|CV
      if (rest >= 3 && ((seit >= 3 && silbe) || seit >= 8)) {
        out += '\u00ad'
        seit = 0
      }
    }
    return out
  })
}

// Bild in fester Box (.bild der Karte bzw. .vorschau der Episode). Ohne Bild: Tonfläche mit dem Titel
// in Plakat-Versalien und Jahr/Laufzeit am Fuß.
export function Bild({ src, titel, farbe, fuss }: { src?: string; titel: string; farbe?: string; fuss?: [string?, string?] }) {
  const ref = useRef<HTMLDivElement>(null)
  const [sichtbar, setSichtbar] = useState(false)
  const [ok, setOk] = useState(true)
  useEffect(() => (src && ref.current ? beobachte(ref.current, () => setSichtbar(true)) : undefined), [src])
  if (!src || !ok)
    return (
      <div class="fl-platzhalter" style={{ background: farbe || tonFuer(titel) }}>
        <div class="t">{trenne(titel)}</div>
        {fuss && (fuss[0] || fuss[1]) && (
          <div class="f">
            {fuss[0]}
            <span>{fuss[1]}</span>
          </div>
        )}
      </div>
    )
  return (
    <div ref={ref} class="bild" style={{ background: farbe || 'var(--flaeche-2)' }}>
      {sichtbar && <img src={src} alt="" onError={() => setOk(false)} onLoad={(e) => ((e.target as HTMLElement).className = 'da')} />}
    </div>
  )
}

export interface KarteProps {
  fokusKey: string
  titel: string
  unter?: string // zweite Zeile unter dem Titel
  bild?: string // URL; leer = Tonfläche mit Titel
  farbe?: string // dominante Bildfarbe als Grund
  fuss?: [string?, string?] // Fuß der Tonfläche: links (Jahr), rechts (Laufzeit)
  breit?: boolean // 16:9 statt Poster 2:3
  fortschritt?: number // 0..1
  ampel?: Ampel
  gesehen?: boolean
  onPress: () => void
  onFocus?: () => void
}

export function Karte(p: KarteProps) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: p.fokusKey, onPress: p.onPress, onFocus: p.onFocus })
  const label = [p.titel, p.unter, p.ampel && t('ampel.' + p.ampel), p.gesehen && t('karte.gesehen')].filter(Boolean).join(', ')
  return (
    <button ref={f.ref} {...f.dom} type="button" aria-label={label} class={'fl-karte' + (p.breit ? ' breit' : '') + (f.fokus ? ' ist-fokus' : '')}>
      <div class="rahmen">
        <Bild src={p.bild} titel={p.titel} farbe={p.farbe} fuss={p.fuss} />
        <Badges ampel={p.ampel} gesehen={p.gesehen} />
      </div>
      <Fortschritt anteil={p.fortschritt || 0} />
      <div class="meta" aria-hidden="true">
        <b>{p.titel}</b>
        {p.unter && <small>{p.unter}</small>}
      </div>
    </button>
  )
}
