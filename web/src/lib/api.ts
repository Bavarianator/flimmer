// Datenschicht: typisierte API, 401 → Login, Paginierung, kleiner stale-while-revalidate-Cache.
import { useEffect, useRef, useState } from 'preact/hooks'
import { getToken, profile, setProbe } from '../profile'
import { loadProbe, runProbe } from '../probe'

// ---------- Typen (Formen wie in internal/api) ----------
export type Ampel = 'green' | 'yellow' | 'red'

export interface Meta {
  title?: string
  originalTitle?: string
  overview?: string
  year?: number
  rating?: number
  genres?: string[]
  poster?: string
  backdrop?: string
  source?: string
  uncertain?: boolean
}

export interface Item {
  id: string
  title: string
  year?: number
  series?: string
  season?: number
  episode?: number
  size?: number
  added?: string
  duration: number
  light: Ampel
  method: string
  meta?: Meta | null
  poster?: string // fertige URL vom Server (mit ?w= und ?v=), leer = kein Bild
  backdrop?: string
  color?: string // dominante Bildfarbe
  progress?: number // Sekunden
  watched?: boolean
}

export interface HomeRow {
  id: string // continue | nextup | recent
  title: string
  items: Item[]
}

export interface Nutzer {
  id: string
  name: string
  color: number | string
  admin: boolean
  hasPassword: boolean
}

export interface Serie {
  name: string
  staffeln: { nummer: number; folgen: Item[] }[]
  erste: Item
  anzahl: number
}

// ---------- Kern ----------
export class LoginError extends Error {
  setup: boolean
  constructor(setup: boolean) {
    super('login')
    this.setup = setup
  }
}

let beiLogin: (setup: boolean) => void = (setup) => {
  if (setup) location.href = '/setup'
}
// main.tsx meldet hier, wie auf ein 401 reagiert wird (Profilauswahl zeigen).
export function setLoginHandler(fn: (setup: boolean) => void) {
  beiLogin = fn
}

export interface Antwort<T> {
  daten: T
  headers: Headers
}

// Rohform mit Headern (für X-Total-Count). body undefined = GET, sonst POST (oder methode).
export async function anfrage<T>(path: string, body?: unknown, methode?: string): Promise<Antwort<T>> {
  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  const token = getToken()
  if (token) headers.Authorization = 'Bearer ' + token // TVs nach Kopplung; Browser nutzen das Cookie
  const res = await fetch(path, {
    credentials: 'same-origin',
    method: methode || (body === undefined ? 'GET' : 'POST'),
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (res.status === 401) {
    let setup = false
    try {
      setup = !!(await res.json()).setup
    } catch {}
    beiLogin(setup)
    throw new LoginError(setup)
  }
  if (!res.ok) throw new Error(res.status + ' ' + (await res.text()))
  return { daten: res.status === 204 ? (undefined as T) : await res.json(), headers: res.headers }
}

export async function api<T>(path: string, body?: unknown, methode?: string): Promise<T> {
  return (await anfrage<T>(path, body, methode)).daten
}

// ---------- Cache: stale-while-revalidate ----------
// Ein Eintrag lebt im Speicher; mit merken=true zusätzlich in localStorage, damit der Start nach
// dem Einschalten des TVs sofort etwas zeigt.
interface Eintrag {
  daten?: unknown
  zeit: number
  laeuft?: Promise<unknown>
  lauscher: ((d: unknown) => void)[]
}
const cache: Record<string, Eintrag> = {}

function eintrag(k: string, merken: boolean): Eintrag {
  let e = cache[k]
  if (!e) {
    e = cache[k] = { zeit: 0, lauscher: [] }
    if (merken)
      try {
        const s = localStorage.getItem('flimmer.cache.' + k)
        if (s) e.daten = JSON.parse(s)
        // Fortschritt und Gesehen der gemerkten Titel auch für fortsetzenAb() bekannt machen.
        if (k === 'bibliothek') merke(e.daten as Item[])
        if (k === 'start') for (const r of e.daten as HomeRow[]) merke(r.items)
      } catch {}
  }
  return e
}

export function holen<T>(k: string, laden: () => Promise<T>, opt: { merken?: boolean; maxAlter?: number } = {}): Promise<T> {
  const e = eintrag(k, !!opt.merken)
  if (e.laeuft) return e.laeuft as Promise<T>
  if (e.daten !== undefined && Date.now() - e.zeit < (opt.maxAlter || 0)) return Promise.resolve(e.daten as T)
  e.laeuft = laden().then(
    (d) => {
      e.daten = d
      e.zeit = Date.now()
      e.laeuft = undefined
      if (opt.merken)
        try {
          localStorage.setItem('flimmer.cache.' + k, JSON.stringify(d))
        } catch {} // Speicher voll: dann eben nur im Speicher
      e.lauscher.slice().forEach((fn) => fn(d))
      return d
    },
    (err) => {
      e.laeuft = undefined
      throw err
    },
  )
  return e.laeuft as Promise<T>
}

export function vergessen(praefix = '') {
  for (const k in cache) if (k.indexOf(praefix) === 0) cache[k].zeit = 0
}

export function gecacht<T>(k: string): T | undefined {
  return cache[k] && (cache[k].daten as T)
}

export interface Zustand<T> {
  daten: T | undefined
  fehler: string
  laedt: boolean
  neu: () => void
}

// Hook: zeigt sofort, was unter k im Cache liegt, und lädt im Hintergrund frisch nach.
// laden ist meist ein Endpunkt, der selbst über holen(k, …) cacht; Updates kommen über den Cache.
export function useDaten<T>(k: string, laden: () => Promise<T>, opt: { merken?: boolean } = {}): Zustand<T> {
  const e = eintrag(k, !!opt.merken)
  const [daten, setDaten] = useState<T | undefined>(e.daten as T | undefined)
  const [fehler, setFehler] = useState('')
  const [laedt, setLaedt] = useState(true)
  const lade = useRef(laden)
  lade.current = laden
  const neu = () => {
    setFehler('')
    setLaedt(true)
    lade.current().then(
      (d) => {
        setDaten(d)
        setLaedt(false)
      },
      (err) => {
        setLaedt(false)
        if (!(err instanceof LoginError)) setFehler(String((err && err.message) || err))
      },
    )
  }
  useEffect(() => {
    const fn = (d: unknown) => setDaten(d as T)
    e.lauscher.push(fn)
    if (e.daten !== undefined) setDaten(e.daten as T)
    neu()
    return () => {
      e.lauscher.splice(e.lauscher.indexOf(fn), 1)
    }
  }, [k])
  return { daten, fehler, laedt, neu }
}

// ---------- Endpunkte ----------
const bekannt: Record<string, Item> = {}
function merke(items: Item[]) {
  for (const it of items) bekannt[it.id] = it
  return items
}

export function titelAus(id: string): Item | undefined {
  return bekannt[id]
}

// Ganze Bibliothek (Start, Detail). Die Ampel hängt am Geräteprofil, deshalb POST.
export function bibliothek(neu = false): Promise<Item[]> {
  if (neu) vergessen('bibliothek')
  return holen('bibliothek', () => api<Item[] | null>('/api/library', profile).then((r) => merke(r || [])), { merken: true, maxAlter: neu ? 0 : 30000 })
}

// Eine Seite der Bibliothek. total aus X-Total-Count.
export async function seite(offset: number, limit: number): Promise<{ items: Item[]; total: number }> {
  const r = await anfrage<Item[] | null>(`/api/library?offset=${offset}&limit=${limit}`, profile)
  const items = merke(r.daten || [])
  return { items, total: Number(r.headers.get('X-Total-Count')) || offset + items.length }
}

export function startseite(): Promise<HomeRow[]> {
  return holen('start', () => api<HomeRow[]>('/api/home', profile).then((rows) => rows.filter((r) => r.items && r.items.length && merke(r.items))), { merken: true })
}

export function ich(): Promise<Nutzer> {
  return holen('ich', () => api<Nutzer>('/api/me'), { merken: true, maxAlter: 60000 })
}

export function nutzer(): Promise<Nutzer[]> {
  return api<Nutzer[]>('/api/users')
}

export function abmelden(): Promise<void> {
  return api<void>('/api/logout', {}).finally(() => {
    for (const k in cache) delete cache[k]
    try {
      for (const k of ['bibliothek', 'start', 'ich']) localStorage.removeItem('flimmer.cache.' + k)
    } catch {}
  })
}

export function setGesehen(id: string, gesehen: boolean): Promise<void> {
  const it = bekannt[id]
  if (it) {
    it.watched = gesehen
    if (gesehen) it.progress = 0
  }
  vergessen('start')
  return api<void>(`/api/items/${id}/watched`, { watched: gesehen })
}

// ---------- Paginierung für Listen (Filme, Serien) ----------
// Lädt seitenweise; mehr() holt die nächste Seite. Liegt die ganze Bibliothek schon im Cache,
// wird sie ohne Netz verwendet.
export function useSeiten(groesse = 200) {
  const voll = gecacht<Item[]>('bibliothek')
  const [items, setItems] = useState<Item[]>(voll || [])
  const [fertig, setFertig] = useState(!!voll)
  const [fehler, setFehler] = useState('')
  const st = useRef({ offset: voll ? voll.length : 0, laeuft: false, fertig: !!voll })
  const mehr = () => {
    const s = st.current
    if (s.laeuft || s.fertig) return
    s.laeuft = true
    setFehler('')
    seite(s.offset, groesse).then(
      (r) => {
        s.laeuft = false
        s.offset += r.items.length
        s.fertig = !r.items.length || s.offset >= r.total
        setFertig(s.fertig)
        setItems((alt) => alt.concat(r.items))
      },
      (err) => {
        s.laeuft = false
        if (!(err instanceof LoginError)) setFehler(String((err && err.message) || err))
      },
    )
  }
  useEffect(mehr, [])
  return { items, fertig, fehler, mehr }
}

// ---------- Hilfen ----------
export function anzeigeTitel(it: Item): string {
  return (it.meta && it.meta.title) || it.title
}

export function jahr(it: Item): number | undefined {
  return (it.meta && it.meta.year) || it.year
}

// Serverseitiger Fortschritt, solange es ihn gibt; sonst der lokal gemerkte.
export function fortsetzenAb(id: string): number {
  const it = bekannt[id]
  if (it && it.progress !== undefined) return it.watched ? 0 : it.progress
  try {
    return Number(localStorage.getItem('pos:' + id) || 0)
  } catch {
    return 0
  }
}

// Bild-URL in passender Breite. '' = kein Bild (dann Tonfläche mit Titel).
export function bild(it: Item, art: 'poster' | 'backdrop', w: number): string {
  const u = it[art]
  if (!u) return ''
  return /[?&]w=\d+/.test(u) ? u.replace(/([?&]w=)\d+/, '$1' + w) : u + (u.indexOf('?') < 0 ? '?' : '&') + 'w=' + w
}

export function serien(items: Item[]): Serie[] {
  const by: Record<string, Serie> = {}
  const reihenfolge: string[] = []
  for (const it of items) {
    if (!it.series) continue
    let s = by[it.series]
    if (!s) {
      s = by[it.series] = { name: it.series, staffeln: [], erste: it, anzahl: 0 }
      reihenfolge.push(it.series)
    }
    const n = it.season || 0
    let st = s.staffeln.filter((x) => x.nummer === n)[0]
    if (!st) s.staffeln.push((st = { nummer: n, folgen: [] }))
    st.folgen.push(it)
    s.anzahl++
  }
  return reihenfolge.map((n) => {
    const s = by[n]
    s.staffeln.sort((a, b) => (a.nummer || 99999) - (b.nummer || 99999)) // Specials (0) ans Ende
    for (const x of s.staffeln) x.folgen.sort((a, b) => (a.episode || 0) - (b.episode || 0))
    s.erste = s.staffeln[0].folgen[0]
    return s
  })
}

// Nächste Folge nach der zuletzt angefangenen – für „Weiterschauen“ bei Serien.
export function naechsteFolge(s: Serie): Item {
  const alle = s.staffeln.reduce<Item[]>((acc, x) => acc.concat(x.folgen), [])
  let letzte = -1
  alle.forEach((e, i) => {
    if (fortsetzenAb(e.id) > 0 || e.watched) letzte = i
  })
  if (letzte < 0) return alle[0]
  const e = alle[letzte]
  const durch = e.watched || fortsetzenAb(e.id) > e.duration * 0.9
  return durch && alle[letzte + 1] ? alle[letzte + 1] : e
}

export function anteil(it: Item): number {
  const p = fortsetzenAb(it.id)
  return it.duration > 0 && p > 0 ? Math.min(1, p / it.duration) : 0
}

// ---------- Geräte-Test (Probe) ----------
// Spielt kleine Clips ab (probe.ts), meldet das Profil an den Server und lädt die Ampeln neu.
// Läuft einmal pro Gerät/Firmware still im Hintergrund, auf Wunsch erneut.
let testLaeuft: Promise<void> | null = null
export const geraeteTest = {
  laeuft: () => !!testLaeuft,
  starten(): Promise<void> {
    if (!testLaeuft)
      testLaeuft = runProbe()
        .then((r) => {
          testLaeuft = null
          if (!r) return
          setProbe(r)
          vergessen()
          return bibliothek(true).then(() => startseite()).then(() => {})
        })
        .catch(() => {
          testLaeuft = null
        })
    return testLaeuft
  },
  // Beim Start: nur wenn es für dieses Gerät noch keine Messung gibt.
  wennNoetig() {
    if (!loadProbe()) geraeteTest.starten()
  },
}
