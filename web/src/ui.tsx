// Gemeinsame Bausteine: Karten, Reihen, Bilder mit Platzhalter.
import type { ComponentChildren } from 'preact'
import { useEffect, useState } from 'preact/hooks'
import { FocusContext, useFocusable } from '@noriginmedia/norigin-spatial-navigation'
import { BACK_KEYS } from './data'

function hue(s: string) {
  let h = 0
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) % 360
  return h
}

// Bild vom Server; fehlt es, bleibt der aus dem Namen erzeugte Farbverlauf mit Initiale sichtbar.
export function Art({ src, name, children }: { src: string; name: string; children?: ComponentChildren }) {
  const [ok, setOk] = useState(true)
  const h = hue(name)
  return (
    <div class="art" style={{ background: `linear-gradient(160deg, hsl(${h},55%,38%), hsl(${(h + 60) % 360},60%,16%))` }}>
      {ok ? <img src={src} alt="" loading="lazy" onError={() => setOk(false)} /> : <span class="initial">{name.charAt(0)}</span>}
      {children}
    </div>
  )
}

export function Card(props: {
  focusKey: string
  wide?: boolean
  img: string
  name: string
  title: string
  sub?: string
  light?: string
  progress?: number
  onOpen: () => void
}) {
  const { ref, focused } = useFocusable({
    onEnterPress: props.onOpen,
    focusKey: props.focusKey,
    onFocus: ({ node }: { node?: HTMLElement }) => node && node.scrollIntoView && node.scrollIntoView({ block: 'nearest', inline: 'nearest' }),
  })
  return (
    <button ref={ref} class={'card' + (props.wide ? ' wide' : '') + (focused ? ' focused' : '')} onClick={props.onOpen}>
      <div class="frame">
        <Art src={props.img} name={props.name}>
          {props.light && <span class={'light ' + props.light} />}
          {props.progress !== undefined && props.progress > 0 && (
            <span class="progress">
              <span style={{ width: Math.min(100, props.progress * 100) + '%' }} />
            </span>
          )}
        </Art>
      </div>
      <div class="meta">
        <strong>{props.title}</strong>
        {props.sub && <small>{props.sub}</small>}
      </div>
    </button>
  )
}

export function Row({ title, children }: { title: string; children: ComponentChildren }) {
  const { ref, focusKey } = useFocusable({ focusKey: 'row-' + title })
  return (
    <FocusContext.Provider value={focusKey}>
      <section ref={ref}>
        <h2>{title}</h2>
        <div class="cards">{children}</div>
      </section>
    </FocusContext.Provider>
  )
}

export function FocusButton({ focusKey, onPress, children, primary }: { focusKey: string; onPress: () => void; children: ComponentChildren; primary?: boolean }) {
  const { ref, focused } = useFocusable({ focusKey, onEnterPress: onPress })
  return (
    <button ref={ref} class={'btn' + (primary ? ' primary' : '') + (focused ? ' focused' : '')} onClick={onPress}>
      {children}
    </button>
  )
}

// Zurück-Taste der Fernbedienung (und Escape) führt eine Seite zurück.
export function useBack(onBack: () => void) {
  useEffect(() => {
    const on = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement
      if (e.keyCode === 8 && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA')) return
      if (BACK_KEYS.indexOf(e.keyCode) >= 0) {
        e.preventDefault()
        onBack()
      }
    }
    window.addEventListener('keydown', on)
    return () => window.removeEventListener('keydown', on)
  }, [onBack])
}
