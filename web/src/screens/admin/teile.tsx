// Bausteine für Dashboard, Benutzer-Einstellungen und Metadaten-Editor: Zeile, Feld, Auswahl, Ladezustand, Toast.
import type { ComponentChildren } from 'preact'
import { useEffect, useRef, useState } from 'preact/hooks'
import { getCurrentFocusKey } from '@noriginmedia/norigin-spatial-navigation'
import type { Zustand } from '../../lib/api'
import { fehlt } from '../../lib/api-admin'
import { istTV } from '../../lib/device'
import { fokusBald, useFokus } from '../../lib/focus'
import { ergaenze, t } from '../../lib/i18n'
import { Chip } from '../../components/Button'
import { Icon, type IconName } from '../../components/Icon'
import { Fehler } from '../../components/Zustand'
import './admin.css'

ergaenze(
  {
    'einst.gespeichert': 'Gespeichert',
    'einst.speichern': 'Speichern',
    'einst.verwerfen': 'Verwerfen',
    'einst.nie': 'noch nie',
    'einst.jetzt': 'gerade eben',
    'einst.vor.min': 'vor {n} Min.',
    'einst.vor.std': 'vor {n} Std.',
    'einst.gestern': 'gestern, {zeit}',
    'einst.tage': '{n} Tagen',
    'einst.tag': '1 Tag',
    'einst.fehlt.titel': 'Noch nicht verfügbar',
    'einst.fehlt.text': 'Der Server liefert diesen Bereich noch nicht. Nach einem Update erscheint er hier.',
    'einst.kopiert': 'In die Zwischenablage kopiert',
    'einst.kopieren': 'Kopieren',
    'einst.nuradmin': 'Diese Einstellungen darf nur ein Admin ändern.',
    'methode.direct': 'Direktes Abspielen',
    'methode.direct-play': 'Direktes Abspielen',
    'methode.remux': 'Neu verpackt, ohne Umwandlung',
    'methode.direct-stream': 'Neu verpackt, ohne Umwandlung',
    'methode.transcode-audio': 'Nur der Ton wird umgewandelt',
    'methode.transcode': 'Wird umgewandelt',
  },
  {
    'einst.gespeichert': 'Saved',
    'einst.speichern': 'Save',
    'einst.verwerfen': 'Discard',
    'einst.nie': 'never',
    'einst.jetzt': 'just now',
    'einst.vor.min': '{n} min ago',
    'einst.vor.std': '{n} h ago',
    'einst.gestern': 'yesterday, {zeit}',
    'einst.tage': '{n} days',
    'einst.tag': '1 day',
    'einst.fehlt.titel': 'Not available yet',
    'einst.fehlt.text': 'The server does not provide this section yet. It will appear here after an update.',
    'einst.kopiert': 'Copied to the clipboard',
    'einst.kopieren': 'Copy',
    'einst.nuradmin': 'Only an admin can change these settings.',
  },
)

// ---------- Toast ----------
let toastSetzen: (s: string) => void = () => {}
export function melde(s: string) {
  toastSetzen(s)
}
export function fehlerText(e: unknown) {
  return String((e && (e as Error).message) || e).replace(/^\d{3} /, '')
}
export const meldeFehler = (e: unknown) => melde(fehlerText(e))
export const methode = (m: string) => t('methode.' + m) === 'methode.' + m ? m : t('methode.' + m)

export function Toast() {
  const [s, setS] = useState('')
  // Der zuletzt eingehängte Toast zeigt die Meldungen (Dialog über dem Dashboard), danach wieder der vorige.
  useEffect(() => {
    const vorher = toastSetzen
    toastSetzen = setS
    return () => {
      toastSetzen = vorher
    }
  }, [])
  useEffect(() => {
    if (!s) return
    const x = setTimeout(() => setS(''), 3000)
    return () => clearTimeout(x)
  }, [s])
  return s ? (
    <div class="fl-toast admin-toast" role="status">
      {s}
    </div>
  ) : null
}

// Chromium 53 kennt navigator.clipboard nicht.
export function kopieren(text: string) {
  const ta = document.createElement('textarea')
  ta.value = text
  document.body.appendChild(ta)
  ta.select()
  try {
    document.execCommand('copy')
    melde(t('einst.kopiert'))
  } catch {}
  document.body.removeChild(ta)
}

// ---------- Formate ----------
const zwei = (n: number) => (n < 10 ? '0' : '') + n
export function datum(s: string | number | undefined, mitZeit = true) {
  const d = new Date(s || 0)
  if (!d.getTime() || d.getFullYear() < 2000) return t('einst.nie')
  return zwei(d.getDate()) + '.' + zwei(d.getMonth() + 1) + '.' + d.getFullYear() + (mitZeit ? ', ' + d.getHours() + ':' + zwei(d.getMinutes()) : '')
}
export function uhrzeit(s: string) {
  const d = new Date(s)
  return zwei(d.getHours()) + ':' + zwei(d.getMinutes())
}
// „vor 26 Min.“, „gestern, 19:12“, sonst Datum
export function vor(s: string | undefined) {
  const d = new Date(s || 0)
  if (!d.getTime() || d.getFullYear() < 2000) return t('einst.nie')
  const min = Math.round((Date.now() - d.getTime()) / 60000)
  if (min < 1) return t('einst.jetzt')
  if (min < 60) return t('einst.vor.min', { n: min })
  if (min < 12 * 60) return t('einst.vor.std', { n: Math.round(min / 60) })
  const gestern = new Date()
  gestern.setDate(gestern.getDate() - 1)
  if (d.toDateString() === gestern.toDateString()) return t('einst.gestern', { zeit: uhrzeit(s!) })
  return datum(s)
}
export function groesse(b: number) {
  if (b >= 1e12) return (b / 1e12).toFixed(1).replace('.', ',') + ' TB'
  if (b >= 1e9) return (b / 1e9).toFixed(1).replace('.', ',') + ' GB'
  return Math.max(0.1, b / 1e6).toFixed(1).replace('.', ',') + ' MB'
}
// Laufzeit eines Servers: „6 Tagen“ bzw. „3 Std. 12 Min.“
export function seit(sek: number) {
  const tage = Math.floor(sek / 86400)
  if (tage > 1) return t('einst.tage', { n: tage })
  if (tage === 1) return t('einst.tag')
  const h = Math.floor(sek / 3600)
  const m = Math.round((sek % 3600) / 60)
  return (h ? t('zeit.std', { h }) + ' ' : '') + t('zeit.min', { m })
}

// ---------- Bausteine ----------
// Eine Zeile (.fl-zeile): Icon, Titel mit Unterzeile, rechts Wert, Schalter oder Knöpfe. Mit onPress ist die ganze Zeile bedienbar.
export function Zeile(p: {
  fk?: string
  icon?: IconName
  titel: ComponentChildren
  unter?: ComponentChildren
  schalter?: boolean
  wert?: ComponentChildren
  rechts?: ComponentChildren
  an?: boolean // gewählt (Listen mit Auswahl)
  class?: string
  onPress?: () => void
}) {
  // Auf dem TV ist jede Zeile fokussierbar, sonst käme das D-Pad nicht an ihr vorbei (die Seite fährt dem Fokus nach).
  const f = useFokus<HTMLButtonElement & HTMLDivElement>({ fokusKey: p.fk, onPress: p.onPress, aus: !p.onPress && (!istTV() || !!p.rechts) })
  const cls = 'fl-zeile' + (p.an ? ' admin-an' : '') + (p.class ? ' ' + p.class : '') + (f.fokus ? ' ist-fokus' : '')
  const inhalt = (
    <>
      {p.icon && <Icon name={p.icon} class="fl-icon" />}
      <span class="text">
        <b>{p.titel}</b>
        {p.unter && <small>{p.unter}</small>}
      </span>
      {p.wert !== undefined && <span class="wert">{p.wert}</span>}
      {p.schalter !== undefined && (
        <span class={'fl-schalter' + (p.schalter ? ' an' : '')} role="switch" aria-checked={p.schalter}>
          <i />
        </span>
      )}
      {p.rechts && <span class="admin-rechts">{p.rechts}</span>}
    </>
  )
  if (!p.onPress)
    return (
      <div ref={f.ref} class={cls}>
        {inhalt}
      </div>
    )
  return (
    <button ref={f.ref} {...f.dom} type="button" aria-current={p.an ? 'true' : undefined} class={cls + ' admin-zeile-knopf'}>
      {inhalt}
    </button>
  )
}

// Eingabefeld, das auch per D-Pad erreichbar ist: OK setzt den echten Fokus (öffnet die TV-Tastatur).
export function Feld(p: {
  fk: string
  wert: string
  setWert: (w: string) => void
  label: string
  typ?: string
  breit?: boolean
  zeigeLabel?: boolean // Beschriftung über dem Feld (Formulare), sonst nur Platzhalter
  mehrzeilig?: boolean
  nurLesen?: boolean
  onEnter?: () => void
}) {
  const input = useRef<HTMLInputElement & HTMLTextAreaElement>(null)
  const f = useFokus<HTMLLabelElement>({ fokusKey: p.fk, onPress: () => input.current && input.current.focus() })
  const attr = {
    ref: input,
    class: 'wachse',
    placeholder: p.label,
    value: p.wert,
    readOnly: p.nurLesen,
    onInput: (e: Event) => p.setWert((e.target as HTMLInputElement).value),
    onFocus: () => !f.fokus && f.fokusSelbst(),
  }
  return (
    <label ref={f.ref} class={'admin-feld' + (p.breit ? ' breit' : '') + (p.mehrzeilig ? ' mehrzeilig' : '')}>
      <span class={p.zeigeLabel ? 'admin-feld-label' : 'nur-sr'}>{p.label}</span>
      <span class={'fl-feld' + (f.fokus ? ' ist-fokus' : '')}>
        {p.mehrzeilig ? (
          <textarea {...attr} rows={4} />
        ) : (
          <input {...attr} type={p.typ || 'text'} onKeyDown={(e) => e.keyCode === 13 && p.onEnter && p.onEnter()} />
        )}
      </span>
    </label>
  )
}

// Auswahl aus wenigen Werten als Chips (statt <select>, das auf webOS mit dem D-Pad schlecht geht).
export function Wahl<W extends string | number>(p: { fk: string; werte: [W, string][]; an: W; onWahl: (w: W) => void }) {
  return (
    <div class="admin-chips">
      {p.werte.map(([w, label]) => (
        <Chip key={String(w)} fokusKey={p.fk + '-' + w} an={p.an === w} onPress={() => p.an !== w && p.onWahl(w)}>
          {label}
        </Chip>
      ))}
    </div>
  )
}

// Abschnitt mit Kopf (.fl-gruppe h3); rechts im Kopf optional Knöpfe.
export function Abschnitt(p: { titel?: string; zusatz?: ComponentChildren; text?: string; children?: ComponentChildren; class?: string }) {
  return (
    <section class={'fl-gruppe' + (p.class ? ' ' + p.class : '')}>
      {(p.titel || p.zusatz) && (
        <div class="admin-kopf fl-reihe-flex">
          <h3 class="fl-grow">{p.titel}</h3>
          {p.zusatz}
        </div>
      )}
      {p.text && <p class="t-klein leise admin-absatz">{p.text}</p>}
      {p.children}
    </section>
  )
}

export function Messwert({ wert, label, anteil }: { wert: string; label: string; anteil?: number }) {
  return (
    <span class="fl-messwert admin-messwert">
      <small class="fl-label">{label}</small>
      <b>{wert}</b>
      {anteil !== undefined && <Balken anteil={anteil} />}
    </span>
  )
}

export function Balken({ anteil }: { anteil: number }) {
  return (
    <span class="admin-balken" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(anteil * 100)}>
      <i style={{ width: Math.max(0, Math.min(100, anteil * 100)) + '%' }} />
    </span>
  )
}

export function Hinweis({ titel, text, icon = 'info' }: { titel?: string; text: string; icon?: IconName }) {
  return (
    <div class="admin-hinweis fl-reihe-flex" role="status">
      <Icon name={icon} />
      <span class="fl-grow">
        {titel && <b>{titel}</b>}
        {text}
      </span>
    </div>
  )
}

// Ladezustand für useDaten: fehlt der Endpunkt, ein ruhiger Hinweis statt Fehlerseite.
// Auf dem TV landet der Fokus nach dem Laden im Inhalt, falls er noch am leeren Container hängt.
export function Geladen<T>({ z, children }: { z: Zustand<T>; children: (d: T) => ComponentChildren }) {
  const da = z.daten !== undefined
  useEffect(() => {
    if (da && getCurrentFocusKey() === 'inhalt') fokusBald('inhalt')
  }, [da])
  if (z.fehler && !da) return fehlt(z.fehler) ? <Hinweis titel={t('einst.fehlt.titel')} text={t('einst.fehlt.text')} /> : <Fehler fehler={z.fehler} nochmal={z.neu} />
  if (!da) return <div class="admin-laedt" aria-busy="true" />
  return <>{children(z.daten as T)}</>
}

// Wiederholt laden, solange bedingung gilt (Scan, Aufgaben, Downloads).
export function useNachladen(neu: () => void, ms: number, bedingung = true) {
  useEffect(() => {
    if (!bedingung) return
    const x = setInterval(neu, ms)
    return () => clearInterval(x)
  }, [bedingung, ms])
}
