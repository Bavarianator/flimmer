// Bildschirmtastatur für TVs: Suche (Buchstaben) und PIN/Code (Ziffern). Jede Taste ist per D-Pad erreichbar;
// Ziffern- und Buchstabentasten der Fernbedienung bzw. einer echten Tastatur tippen direkt mit.
import { useEffect } from 'preact/hooks'
import { Gruppe, useFokus } from '../lib/focus'
import { ergaenze, t } from '../lib/i18n'
import '../player/screens.css'

ergaenze(
  { 'tastatur.leer': 'Leerzeichen', 'tastatur.loeschen': 'Löschen', 'tastatur.alles': 'Alles löschen', 'tastatur.fertig': 'Fertig' },
  { 'tastatur.leer': 'Space', 'tastatur.loeschen': 'Delete', 'tastatur.alles': 'Clear', 'tastatur.fertig': 'Done' },
)

const BUCHSTABEN = 'abcdefghijklmnopqrstuvwxyzäöüß1234567890'.split('')
const ZIFFERN = '123456789'.split('')

function Taste(p: { fk: string; label: string; aria?: string; breit?: boolean; onPress: () => void }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: p.fk, onPress: p.onPress })
  return (
    <button
      ref={f.ref}
      {...f.dom}
      type="button"
      aria-label={p.aria}
      class={'fl-taste' + (p.breit ? ' breit' : '') + (f.fokus ? ' ist-fokus' : '')}
    >
      {p.label}
    </button>
  )
}

export function Tastatur(p: {
  wert: string
  setWert: (w: string) => void
  art?: 'text' | 'ziffern'
  max?: number
  fokusKey?: string
  onFertig?: () => void
  direkt?: boolean // echte Tasten (Ziffern, Buchstaben, Backspace) mitschreiben
}) {
  const fk = p.fokusKey || 'tastatur'
  const tippe = (z: string) => {
    if (p.max && p.wert.length >= p.max) return
    p.setWert(p.wert + z)
  }
  const loeschen = () => p.setWert(p.wert.slice(0, -1))

  useEffect(() => {
    if (p.direkt === false) return
    const on = (e: KeyboardEvent) => {
      const ziel = e.target as HTMLElement
      if (ziel && (ziel.tagName === 'INPUT' || ziel.tagName === 'TEXTAREA')) return
      const k = e.keyCode
      if (k >= 48 && k <= 57) tippe(String(k - 48))
      else if (k >= 96 && k <= 105) tippe(String(k - 96)) // Ziffernblock
      else if (p.art !== 'ziffern' && e.key && e.key.length === 1 && /[\wäöüß ]/i.test(e.key)) tippe(e.key.toLowerCase())
      else if (k === 8 && p.wert) loeschen()
      else return
      e.preventDefault()
      e.stopPropagation() // Backspace nicht als Zurück werten
    }
    window.addEventListener('keydown', on, true)
    return () => window.removeEventListener('keydown', on, true)
  }, [p.wert, p.art])

  if (p.art === 'ziffern')
    return (
      <Gruppe fokusKey={fk} class="fl-tasten tastatur-ziffern" label={t('tastatur.fertig')}>
        {ZIFFERN.map((z) => (
          <Taste key={z} fk={fk + '-' + z} label={z} onPress={() => tippe(z)} />
        ))}
        <Taste fk={fk + '-loeschen'} label="⌫" aria={t('tastatur.loeschen')} onPress={loeschen} />
        <Taste fk={fk + '-0'} label="0" onPress={() => tippe('0')} />
        <Taste fk={fk + '-fertig'} label="OK" aria={t('tastatur.fertig')} onPress={() => p.onFertig && p.onFertig()} />
      </Gruppe>
    )

  return (
    <Gruppe fokusKey={fk} class="fl-tasten tastatur" label={t('nav.suche')}>
      {BUCHSTABEN.map((z) => (
        <Taste key={z} fk={fk + '-' + z} label={z} onPress={() => tippe(z)} />
      ))}
      <Taste fk={fk + '-leer'} label="␣" aria={t('tastatur.leer')} breit onPress={() => tippe(' ')} />
      <Taste fk={fk + '-loeschen'} label="⌫" aria={t('tastatur.loeschen')} breit onPress={loeschen} />
      <Taste fk={fk + '-alles'} label={t('tastatur.alles')} breit onPress={() => p.setWert('')} />
    </Gruppe>
  )
}
