import type { Nutzer } from '../lib/api'

// Profilfarbe: Der Server speichert einen Farbton (0–360) oder eine CSS-Farbe; angezeigt wird eine der
// sechs gedeckten Avatar-Farben aus den Tokens.
export function avatarFarbe(c: Nutzer['color']): string {
  if (typeof c === 'number') return 'var(--avatar-' + ((Math.floor((((c % 360) + 360) % 360) / 60) % 6) + 1) + ')'
  return c || 'var(--avatar-3)'
}

// Kleine Kachel mit Initiale in Plakat-Versalien (Kopfzeile, Gemeinsam schauen).
export function MiniAvatar({ n, class: cls }: { n: Pick<Nutzer, 'name' | 'color'>; class?: string }) {
  return (
    <span class={'fl-mini-avatar' + (cls ? ' ' + cls : '')} style={{ background: avatarFarbe(n.color) }} aria-hidden="true">
      {n.name.charAt(0).toUpperCase()}
    </span>
  )
}
