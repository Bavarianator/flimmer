// Fokus-System auf Basis von norigin-spatial-navigation.
// - useFokus: ein fokussierbares Element (Klasse ist-fokus statt :focus-visible)
// - Gruppe: Container mit eigenem Fokus-Kontext (Reihe, Knopfleiste, Raster)
// - TV: Die Reihe fährt per translateX (.schiene), die Seite per translateY (.seite-schiene).
// - Fokus-Gedächtnis: pro Reihe (norigin, saveLastFocusedChild) und pro Seite (Pfad → Fokus-Schlüssel),
//   damit „Zurück“ wieder auf der zuletzt gewählten Karte landet.
import { createElement, type ComponentChildren } from 'preact'
import {
  FocusContext,
  doesFocusableExist,
  init,
  pause,
  resume,
  setFocus,
  useFocusable,
  type UseFocusableConfig,
} from '@noriginmedia/norigin-spatial-navigation'
import { useEffect } from 'preact/hooks'
import { geraet, istTV } from './device'
import { aktuell } from './router'

export { FocusContext, pause, resume, setFocus }

export function starteFokus() {
  // Auf dem TV kein DOM-Fokus: node.focus() würde die per transform verschobenen Schienen scrollen.
  // Auf Desktop/Handy bekommt das Element echten Fokus (Screenreader, native Scroll-Nachführung).
  init({ throttle: 100, shouldFocusDOMNode: geraet !== 'tv' })
  // Die D-Pad-Navigation (Listener auf window) blockiert Enter/←/→ auch in Eingabefeldern.
  // Hier auf document abfangen, damit Formulare abschicken und der Cursor im Text wandern kann.
  document.addEventListener('keydown', (e) => {
    const t = e.target as HTMLElement
    if ((t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.tagName === 'SELECT') && [13, 37, 39].indexOf(e.keyCode) >= 0) e.stopPropagation()
  })
}

// ---------- Fokus-Gedächtnis pro Seite ----------
const gedaechtnis: Record<string, string> = {}

// Nach dem Laden einer Seite aufrufen: stellt den gemerkten Fokus wieder her, sonst `ersatz`.
// Preact führt Effekte der Eltern vor denen der Kinder aus; deshalb erst im nächsten Tick.
export function fokusStart(ersatz: string) {
  const pfad = aktuell()
  setTimeout(() => {
    if (aktuell() !== pfad) return
    const k = gedaechtnis[pfad]
    setFocus(k && doesFocusableExist(k) ? k : ersatz)
  }, 0)
}

export function fokusBald(k: string) {
  setTimeout(() => setFocus(k), 0)
}

// ---------- Nachführen auf dem TV ----------
function oben(el: HTMLElement, bis: HTMLElement): number {
  let y = 0
  for (let n: HTMLElement | null = el; n && n !== bis; n = n.offsetParent as HTMLElement | null) y += n.offsetTop
  return y
}

export function folge(node: HTMLElement) {
  if (!istTV()) return
  const schiene = node.closest('.schiene') as HTMLElement | null
  if (schiene && schiene.parentElement) {
    const max = Math.max(0, schiene.scrollWidth - schiene.parentElement.clientWidth)
    const x = Math.min(max, Math.max(0, node.offsetLeft - (schiene.firstElementChild as HTMLElement).offsetLeft))
    schiene.style.transform = schiene.style.webkitTransform = 'translateX(' + -x + 'px)'
  }
  const seite = node.closest('.seite-schiene') as HTMLElement | null
  if (seite) {
    const ziel = (node.closest('.reihe') as HTMLElement | null) || node
    const max = Math.max(0, seite.scrollHeight - window.innerHeight)
    const y = Math.min(max, Math.max(0, oben(ziel, seite) - window.innerHeight * 0.42))
    seite.style.transform = seite.style.webkitTransform = 'translateY(' + -y + 'px)'
  }
}

// ---------- Hooks ----------
export interface FokusOptionen {
  fokusKey?: string
  onPress?: () => void
  onFocus?: () => void
  onArrow?: (richtung: string) => boolean // false = Navigation unterbinden
  aus?: boolean // nicht fokussierbar
}

export function useFokus<E extends HTMLElement = HTMLElement>(o: FokusOptionen) {
  const f = useFocusable<object, E>({
    focusKey: o.fokusKey,
    focusable: !o.aus,
    onEnterPress: o.onPress,
    onArrowPress: o.onArrow ? (d: string) => o.onArrow!(d) : undefined,
    onFocus: (l) => {
      folge(l.node as HTMLElement)
      gedaechtnis[aktuell()] = f.focusKey
      if (o.onFocus) o.onFocus()
    },
  })
  // Maus/Magic-Remote-Zeiger (TV) und Tab-Taste (Desktop) führen den D-Pad-Fokus mit.
  const dom = {
    onMouseEnter: istTV() ? () => f.focusSelf() : undefined,
    onFocus: () => {
      if (!f.focused) f.focusSelf()
    },
    onClick: o.onPress,
  }
  return { ref: f.ref, fokus: f.focused, fokusKey: f.focusKey, fokusSelbst: f.focusSelf, dom }
}

// Container mit eigenem Fokus-Kontext. merken=true: Rückkehr in die Gruppe landet auf dem zuletzt
// fokussierten Kind (Fokus-Gedächtnis pro Reihe).
export function Gruppe(p: {
  fokusKey?: string
  class?: string
  tag?: string
  grenze?: boolean // Fokus verlässt die Gruppe nicht (Menüs, Sheets)
  bevorzugt?: string
  label?: string
  children: ComponentChildren
}) {
  const cfg: UseFocusableConfig = {
    focusKey: p.fokusKey,
    saveLastFocusedChild: true,
    trackChildren: false,
    isFocusBoundary: !!p.grenze,
    preferredChildFocusKey: p.bevorzugt,
  }
  const { ref, focusKey } = useFocusable(cfg)
  return createElement(
    FocusContext.Provider,
    { value: focusKey },
    createElement(p.tag || 'div', { ref, class: p.class, 'aria-label': p.label, role: p.label ? 'group' : undefined }, p.children),
  )
}

// Solange Texteingabe oder eigene Tastensteuerung aktiv ist, darf die D-Pad-Navigation Enter und
// Pfeile nicht abfangen.
export function usePausierteNavigation(aktiv = true) {
  useEffect(() => {
    if (!aktiv) return
    pause()
    return () => resume()
  }, [aktiv])
}
