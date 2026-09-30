import type { ComponentChildren } from 'preact'
import { useRef } from 'preact/hooks'
import { geraet } from '../lib/device'
import { Gruppe } from '../lib/focus'
import { ergaenze, t } from '../lib/i18n'
import { Icon } from './Icon'

ergaenze({ 'reihe.zurueck': 'Zurück blättern', 'reihe.weiter': 'Weiter blättern' }, { 'reihe.zurueck': 'Scroll back', 'reihe.weiter': 'Scroll forward' })

// Horizontale Reihe. TV: .schiene fährt per translateX (lib/focus.ts folge), .spur ohne Scrollbalken.
// Desktop/Handy: .spur scrollt nativ (Wischen), am Desktop zusätzlich mit Blätterknöpfen.
// ziel: Überschrift als Link („Kürzlich hinzugefügt in Filme ›“).
export function Reihe(p: { titel: string; fokusKey: string; zusatz?: string; ziel?: string; onZiel?: () => void; children: ComponentChildren }) {
  const spur = useRef<HTMLDivElement>(null)
  const blaettern = (r: number) => {
    const s = spur.current
    if (s) s.scrollLeft += r * s.clientWidth * 0.8
  }
  return (
    <section class="fl-reihe reihe">
      <div class="reihe-kopf">
        <h2>
          {p.ziel ? (
            <a href={'#' + p.ziel} class="reihe-link" onClick={p.onZiel && ((e) => (e.preventDefault(), p.onZiel!()))}>
              {p.titel}
              <Icon name="weiter" />
            </a>
          ) : (
            p.titel
          )}
          {p.zusatz && <small>{p.zusatz}</small>}
        </h2>
        {geraet === 'dt' && (
          <span class="reihe-blaettern">
            <button type="button" tabIndex={-1} aria-label={t('reihe.zurueck')} onClick={() => blaettern(-1)}>
              <Icon name="zurueck" />
            </button>
            <button type="button" tabIndex={-1} aria-label={t('reihe.weiter')} onClick={() => blaettern(1)}>
              <Icon name="weiter" />
            </button>
          </span>
        )}
      </div>
      <div class="spur" ref={spur}>
        <Gruppe fokusKey={p.fokusKey} class="schiene" label={p.titel}>
          {p.children}
        </Gruppe>
      </div>
    </section>
  )
}

// Raster (Filme, Serien): Karten umbrechen, Abstände per margin.
export function Raster(p: { fokusKey: string; children: ComponentChildren }) {
  return (
    <Gruppe fokusKey={p.fokusKey} class="raster rand">
      {p.children}
    </Gruppe>
  )
}
