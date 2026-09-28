import type { ComponentChildren } from 'preact'
import { Gruppe } from '../lib/focus'

// Horizontale Reihe. TV: .schiene fährt per translateX (lib/focus.ts folge), .spur ohne Scrollbalken.
// Desktop/Handy: .spur scrollt nativ (Wischen), siehe components.css.
export function Reihe(p: { titel: string; fokusKey: string; zusatz?: string; children: ComponentChildren }) {
  return (
    <section class="fl-reihe reihe">
      <h2>
        {p.titel}
        {p.zusatz && <small>{p.zusatz}</small>}
      </h2>
      <div class="spur">
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
