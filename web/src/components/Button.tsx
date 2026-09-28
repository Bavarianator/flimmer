import type { ComponentChildren } from 'preact'
import { useFokus } from '../lib/focus'
import { Icon, type IconName } from './Icon'

// Varianten wie im Designsystem: primaer (papierweiße Fläche, eine pro Blick), sekundaer (Haarlinie),
// geist (nur Schrift). Ohne children entsteht ein runder Icon-Knopf; dann ist label Pflicht.
export function Button(p: {
  fokusKey?: string
  onPress: () => void
  variante?: 'primaer' | 'sekundaer' | 'geist'
  icon?: IconName
  label?: string
  aus?: boolean
  voll?: boolean // volle Breite (Handy)
  children?: ComponentChildren
}) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: p.fokusKey, onPress: p.aus ? undefined : p.onPress })
  const v = p.variante && p.variante !== 'sekundaer' ? ' ' + p.variante : ''
  return (
    <button
      ref={f.ref}
      {...f.dom}
      onClick={p.aus ? undefined : p.onPress}
      type="button"
      disabled={p.aus}
      aria-label={p.label}
      title={p.children ? undefined : p.label}
      class={'fl-btn' + v + (p.children ? '' : ' rund') + (p.voll ? ' voll' : '') + (p.aus ? ' aus' : '') + (f.fokus ? ' ist-fokus' : '')}
    >
      {p.icon && <Icon name={p.icon} />}
      {p.children}
    </button>
  )
}

// Chip: Filter oder Auswahl, an = gewählt.
export function Chip(p: { fokusKey?: string; an?: boolean; icon?: IconName; onPress: () => void; children: ComponentChildren }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: p.fokusKey, onPress: p.onPress })
  return (
    <button ref={f.ref} {...f.dom} type="button" aria-pressed={!!p.an} class={'fl-chip' + (p.an ? ' an' : '') + (f.fokus ? ' ist-fokus' : '')}>
      {p.icon && <Icon name={p.icon} />}
      {p.children}
    </button>
  )
}
