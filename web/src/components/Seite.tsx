import type { ComponentChildren } from 'preact'
import { useEffect, useState } from 'preact/hooks'
import { ich, type Nutzer } from '../lib/api'
import { geraet } from '../lib/device'
import { Gruppe, useFokus } from '../lib/focus'
import { t } from '../lib/i18n'
import { go } from '../lib/router'
import { MiniAvatar } from './Avatar'
import { Icon, type IconName } from './Icon'
import { Wortmarke } from './Wortmarke'

export type Bereich = 'start' | 'filme' | 'serien' | 'suche' | ''

// „Merkliste“ aus dem Design fehlt noch: dafür gibt es keine API (bei SP anfragen).
const bereiche: { id: Exclude<Bereich, ''>; pfad: string; icon: IconName }[] = [
  { id: 'start', pfad: '/', icon: 'start' },
  { id: 'filme', pfad: '/filme', icon: 'film' },
  { id: 'serien', pfad: '/serien', icon: 'serie' },
  { id: 'suche', pfad: '/suche', icon: 'suche' },
]

function NavLink({ b, an }: { b: (typeof bereiche)[0]; an: boolean }) {
  const f = useFokus<HTMLAnchorElement>({ fokusKey: 'nav-' + b.id, onPress: () => go(b.pfad) })
  return (
    <a ref={f.ref} {...f.dom} onClick={undefined} href={'#' + b.pfad} aria-current={an ? 'page' : undefined} class={(an ? 'an' : '') + (f.fokus ? ' ist-fokus' : '')}>
      {b.id === 'suche' && <Icon name="suche" />}
      {b.id === 'suche' ? ' ' : ''}
      {t('nav.' + b.id)}
    </a>
  )
}

function Profil({ n }: { n: Nutzer }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: 'nav-profil', onPress: () => go('/profile') })
  return (
    <button ref={f.ref} {...f.dom} type="button" class={'kopf-profil fl-reihe-flex' + (f.fokus ? ' ist-fokus' : '')} aria-label={t('nav.profil') + ': ' + n.name}>
      {geraet !== 'hd' && <span class="fl-2 kopf-name">{n.name}</span>}
      <MiniAvatar n={n} />
    </button>
  )
}

function KopfKnopf({ icon, label, pfad, fk }: { icon: IconName; label: string; pfad: string; fk: string }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: fk, onPress: () => go(pfad) })
  return (
    <button ref={f.ref} {...f.dom} type="button" aria-label={label} title={label} class={'fl-btn geist rund' + (f.fokus ? ' ist-fokus' : '')}>
      <Icon name={icon} />
    </button>
  )
}

// Angemeldetes Profil (aus dem Cache sofort, dann frisch).
export function useIch() {
  const [n, setN] = useState<Nutzer | null>(null)
  useEffect(() => {
    ich().then(setN, () => {})
  }, [])
  return n
}

// Kopfzeile: ruhige Textnavigation oben (TV, Desktop). Handy: Wortmarke oben, Tab-Leiste unten.
function Kopf({ aktiv }: { aktiv: Bereich }) {
  const n = useIch()
  const handy = geraet === 'hd'
  return (
    <>
      <Gruppe fokusKey="nav" tag="header" class="fl-kopf kopf">
        <span class="wm">
          <Wortmarke />
        </span>
        {!handy && (
          <nav class="nav" aria-label={t('nav.menue')}>
            {bereiche.map((b) => (
              <NavLink key={b.id} b={b} an={b.id === aktiv} />
            ))}
          </nav>
        )}
        <span class="rechts">
          {handy && <KopfKnopf fk="nav-suche" icon="suche" label={t('nav.suche')} pfad="/suche" />}
          {n && n.admin && <KopfKnopf fk="nav-einstellungen" icon="einstellungen" label={t('nav.einstellungen')} pfad="/einstellungen" />}
          {n && <Profil n={n} />}
        </span>
      </Gruppe>
      {handy && (
        <nav class="fl-tabbar" aria-label={t('nav.menue')}>
          {bereiche.map((b) => (
            <a key={b.id} href={'#' + b.pfad} class={b.id === aktiv ? 'an' : ''} aria-current={b.id === aktiv ? 'page' : undefined}>
              <Icon name={b.icon} />
              {t('nav.' + b.id)}
            </a>
          ))}
        </nav>
      )}
    </>
  )
}

// Rahmen jeder Browse-Seite. Auf dem TV fährt .seite-schiene per translateY dem Fokus nach.
export function Seite({ bereich = '', children, ohneKopf, class: cls }: { bereich?: Bereich; children: ComponentChildren; ohneKopf?: boolean; class?: string }) {
  return (
    <main class={'seite' + (cls ? ' ' + cls : '')}>
      <div class="seite-schiene">
        {!ohneKopf && <Kopf aktiv={bereich} />}
        {children}
      </div>
    </main>
  )
}
