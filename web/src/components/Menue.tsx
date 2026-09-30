// Überlagerungen wie in Jellyfin: Kontextmenü („…“ bzw. Rechtsklick), Seitenblatt (Sortieren & Filtern)
// und Dialog. Alle hängen per Portal an <body>, weil auf dem TV .seite-schiene per transform verschoben
// wird und position: fixed darin nicht mehr am Bildschirm hängt.
// Der Fokus bleibt in der Überlagerung (Gruppe mit grenze) und kehrt beim Schließen zum Auslöser zurück.
import type { ComponentChildren } from 'preact'
import { createPortal } from 'preact/compat'
import { useEffect, useLayoutEffect, useRef, useState } from 'preact/hooks'
import { getCurrentFocusKey } from '@noriginmedia/norigin-spatial-navigation'
import { geraet } from '../lib/device'
import { Gruppe, fokusBald, setFocus, useFokus } from '../lib/focus'
import { ergaenze, t } from '../lib/i18n'
import { useZurueck } from '../lib/router'
import { Icon, type IconName } from './Icon'

ergaenze({ 'knopf.schliessen': 'Schließen' }, { 'knopf.schliessen': 'Close' })

let naechste = 0

// Merkt beim Öffnen den fokussierten Auslöser, setzt den Fokus in die Überlagerung und gibt ihn beim
// Schließen zurück. Die Zurück-Taste schließt.
function useUeberlagerung(onZu: () => void, erster: string) {
  const vorher = useRef('')
  useEffect(() => {
    vorher.current = getCurrentFocusKey()
    fokusBald(erster)
    return () => {
      const k = vorher.current
      if (k) setTimeout(() => setFocus(k), 0)
    }
  }, [])
  useZurueck(() => (onZu(), true))
}

export interface MenueEintrag {
  label?: string
  icon?: IconName
  unter?: string // zweite Zeile, z. B. Gerätetyp
  onPress?: () => void
  gefahr?: boolean // Löschen & Co.: rot
  an?: boolean // Häkchen rechts (Auswahl)
  trenner?: boolean
}

function Punkt({ e, fk, onZu }: { e: MenueEintrag; fk: string; onZu: () => void }) {
  const f = useFokus<HTMLButtonElement>({
    fokusKey: fk,
    onPress: () => {
      onZu()
      if (e.onPress) e.onPress()
    },
  })
  return (
    <button ref={f.ref} {...f.dom} type="button" role="menuitem" class={'menue-punkt' + (e.gefahr ? ' gefahr' : '') + (f.fokus ? ' ist-fokus' : '')}>
      {e.icon ? <Icon name={e.icon} /> : <span class="fl-icon" />}
      <span class="fl-grow">
        {e.label}
        {e.unter && <small>{e.unter}</small>}
      </span>
      {e.an && <Icon name="haken" class="an" />}
    </button>
  )
}

export type Anker = HTMLElement | { x: number; y: number }

// Kontextmenü an einem Knopf (anker = Element) oder am Mauszeiger (anker = {x, y}).
// Handy: als Blatt von unten, mit Titel.
export function Menue(p: { eintraege: MenueEintrag[]; anker: Anker; onZu: () => void; titel?: string }) {
  const id = useRef('menue' + ++naechste).current
  const box = useRef<HTMLDivElement>(null)
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null)
  const unten = geraet === 'hd'
  useUeberlagerung(p.onZu, id + '-0')

  useLayoutEffect(() => {
    if (unten || !box.current) return
    const b = box.current.getBoundingClientRect()
    const a = 'x' in p.anker ? { left: p.anker.x, right: p.anker.x, top: p.anker.y, bottom: p.anker.y } : p.anker.getBoundingClientRect()
    let left = a.left
    if (left + b.width > innerWidth - 8) left = Math.max(8, a.right - b.width)
    let top = a.bottom + 4
    if (top + b.height > innerHeight - 8) top = Math.max(8, a.top - b.height - 4)
    setPos({ left, top })
  }, [])

  let n = 0
  return createPortal(
    <div class={'ueberlagerung' + (unten ? ' dunkel' : '')} onClick={p.onZu} onContextMenu={(e) => (e.preventDefault(), p.onZu())}>
      <div
        ref={box}
        role="menu"
        aria-label={p.titel}
        class={'fl-menue' + (unten ? ' unten' : '')}
        style={unten ? undefined : pos ? { left: pos.left + 'px', top: pos.top + 'px' } : { visibility: 'hidden' }}
        onClick={(e) => e.stopPropagation()}
      >
        {unten && <div class="griff" />}
        {unten && p.titel && <div class="menue-titel eine-zeile">{p.titel}</div>}
        <Gruppe fokusKey={id} grenze>
          {p.eintraege.map((e, i) => (e.trenner ? <div key={i} class="trenner" role="separator" /> : <Punkt key={i} e={e} fk={id + '-' + n++} onZu={p.onZu} />))}
        </Gruppe>
      </div>
    </div>,
    document.body,
  )
}

// Hilfe für Aufrufer: const m = useMenue(); <Button onPress={() => m.auf(knopf)} /> {m.offen && <Menue anker={m.anker} … onZu={m.zu} />}
export function useMenue() {
  const [anker, setAnker] = useState<Anker | null>(null)
  return {
    offen: !!anker,
    anker: anker as Anker,
    auf: (a: Anker) => setAnker(a),
    zu: () => setAnker(null),
    // Rechtsklick auf einer Karte
    rechtsklick: (e: MouseEvent) => {
      e.preventDefault()
      setAnker({ x: e.clientX, y: e.clientY })
    },
  }
}

function Schliessen({ fk, onZu }: { fk: string; onZu: () => void }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: fk, onPress: onZu })
  return (
    <button ref={f.ref} {...f.dom} type="button" aria-label={t('knopf.schliessen')} title={t('knopf.schliessen')} class={'fl-btn geist rund' + (f.fokus ? ' ist-fokus' : '')}>
      <Icon name="schliessen" />
    </button>
  )
}

// Seitenblatt rechts (Desktop, TV) bzw. von unten (Handy): Sortieren & Filtern, Details, Auswahl.
// fuss: Knopfzeile unten (z. B. „Zurücksetzen“ und „68 Filme anzeigen“).
export function Sheet(p: { titel: string; onZu: () => void; children: ComponentChildren; fuss?: ComponentChildren; erster?: string }) {
  const id = useRef('sheet' + ++naechste).current
  useUeberlagerung(p.onZu, p.erster || id + '-zu')
  return createPortal(
    <div class="ueberlagerung dunkel" onClick={p.onZu}>
      <div role="dialog" aria-modal="true" aria-label={p.titel} class="fl-blatt" onClick={(e) => e.stopPropagation()}>
        <Gruppe fokusKey={id} grenze class="blatt-innen">
          <div class="blatt-kopf fl-reihe-flex">
            <h2 class="fl-grow">{p.titel}</h2>
            <Schliessen fk={id + '-zu'} onZu={p.onZu} />
          </div>
          <div class="blatt-inhalt">{p.children}</div>
          {p.fuss && <div class="blatt-fuss fl-reihe-flex">{p.fuss}</div>}
        </Gruppe>
      </div>
    </div>,
    document.body,
  )
}

// Dialog in der Mitte, z. B. „Zu Sammlung hinzufügen“. knoepfe: rechts unten, der Hauptknopf zuletzt.
export function Dialog(p: { titel: string; onZu: () => void; children: ComponentChildren; knoepfe?: ComponentChildren; erster?: string; breit?: boolean }) {
  const id = useRef('dialog' + ++naechste).current
  useUeberlagerung(p.onZu, p.erster || id + '-zu')
  return createPortal(
    <div class="ueberlagerung dunkel mitte" onClick={p.onZu}>
      <div role="dialog" aria-modal="true" aria-label={p.titel} class={'fl-dialog' + (p.breit ? ' breit' : '')} onClick={(e) => e.stopPropagation()}>
        <Gruppe fokusKey={id} grenze>
          <div class="blatt-kopf fl-reihe-flex">
            <h2 class="fl-grow">{p.titel}</h2>
            <Schliessen fk={id + '-zu'} onZu={p.onZu} />
          </div>
          <div class="dialog-inhalt">{p.children}</div>
          {p.knoepfe && <div class="dialog-knoepfe">{p.knoepfe}</div>}
        </Gruppe>
      </div>
    </div>,
    document.body,
  )
}
