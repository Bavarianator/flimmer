// Bildanpassung (docs/umbau-jellyfin.md, „Bildanpassung im Player“): reine Rechnung ohne DOM, damit sie testbar bleibt.
// Das <video> behält sein volles Bild (object-fit: fill) und wird so gesetzt und skaliert, dass die Mitte des sichtbaren
// Ausschnitts R in der Bildschirmmitte liegt; der Rahmen schneidet mit overflow: hidden ab.
// Selbsttest: node web/scripts/bild-test.mjs

export type BildModus = 'auto' | 'einpassen' | 'fuellen' | 'strecken'
export const BILD_MODI: BildModus[] = ['auto', 'einpassen', 'fuellen', 'strecken']

// Eingebrannte Balken: sichtbarer Ausschnitt als Anteile des Bildes (0..1), vom Server (video.crop).
export interface Crop {
  x: number
  y: number
  w: number
  h: number
}

export interface Lage {
  links: number
  oben: number
  breite: number
  hoehe: number
  modus: BildModus // tatsächlich gewählt; bei auto: fuellen oder einpassen
}

// Automatisch vergrößert, bis höchstens so viel vom Ausschnitt verloren geht: ganz füllen, sonst so weit es geht (schmalere Balken).
export const AUTO_VERLUST = 0.12

// W×H Bildschirm, vw×vh Video (Anzeigegröße, videoWidth/videoHeight), crop optional.
export function bildLage(modus: BildModus, W: number, H: number, vw: number, vh: number, crop?: Crop | null): Lage {
  const c = modus === 'einpassen' || !crop ? { x: 0, y: 0, w: 1, h: 1 } : crop
  const rw = c.w * vw
  const rh = c.h * vh
  let m = modus
  const fuellen = Math.max(W / rw, H / rh)
  const einpassen = Math.min(W / rw, H / rh)
  let sx: number
  let sy: number
  if (m === 'strecken') {
    sx = W / rw
    sy = H / rh
  } else if (m === 'auto') {
    // zwischen Einpassen und Füllen ragt nur eine Seite hinaus: sichtbarer Anteil = einpassen / s
    sx = sy = Math.min(fuellen, einpassen / (1 - AUTO_VERLUST))
    m = sx === fuellen ? 'fuellen' : 'einpassen'
  } else {
    sx = sy = m === 'fuellen' ? fuellen : einpassen
  }
  return {
    links: W / 2 - (c.x * vw + rw / 2) * sx,
    oben: H / 2 - (c.y * vh + rh / 2) * sy,
    breite: vw * sx,
    hoehe: vh * sy,
    modus: m,
  }
}
