import type { ComponentChildren } from 'preact'
import type { Ampel } from '../lib/api'
import { AmpelZeile, tonFuer } from './Karte'

// Hero: Kulisse mit Schleier aus saal (kein Blur), Titel in Plakat-Versalien, Fakten, Ampel,
// Beschreibung und Knöpfe. Ein neues Bild blendet per opacity über (key am img).
export function Hero(p: {
  titel: string
  label?: string // Zeile über dem Titel, z. B. „Weiterschauen“
  fakten?: (string | number | undefined | false)[]
  ampel?: Ampel
  ampelLang?: boolean // ganzer Satz statt Kurzform, eigene Zeile (Detailseiten)
  text?: string
  bild?: string
  farbe?: string
  class?: string
  children?: ComponentChildren
}) {
  const fakten = (p.fakten || []).filter(Boolean)
  return (
    <div class={'fl-hero held' + (p.class ? ' ' + p.class : '')}>
      <div class="kulisse" style={{ background: p.farbe || tonFuer(p.titel) }} aria-hidden="true">
        {p.bild && <img key={p.bild} src={p.bild} alt="" onLoad={(e) => ((e.target as HTMLElement).className = 'da')} />}
      </div>
      <div class="schleier" aria-hidden="true" />
      <div class="inhalt">
        {p.label && <span class="fl-label">{p.label}</span>}
        <h1>{p.titel}</h1>
        {(fakten.length > 0 || (p.ampel && !p.ampelLang)) && (
          <div class="fakten fl-reihe-flex fl-wrap">
            {fakten.map((f, i) => (
              <span key={i}>{f}</span>
            ))}
            {p.ampel && !p.ampelLang && <AmpelZeile stufe={p.ampel} kurz />}
          </div>
        )}
        {p.ampel && p.ampelLang && <AmpelZeile stufe={p.ampel} />}
        {p.text && <p class="held-text">{p.text}</p>}
        {p.children}
      </div>
    </div>
  )
}
