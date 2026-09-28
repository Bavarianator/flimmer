// Bibliothek einmal laden und zwischen den Seiten teilen.
import { api, profile } from './profile'

export interface Meta {
  title?: string
  overview?: string
  year?: number
  rating?: number
  genres?: string[]
  poster?: boolean | string
  backdrop?: boolean | string
  source?: string
}

export interface Item {
  id: string
  title: string
  year?: number
  series?: string
  season?: number
  episode?: number
  duration: number
  light: 'green' | 'yellow' | 'red'
  method: string
  meta?: Meta
  poster?: string // fertige URL vom Server (inkl. ?t=)
  backdrop?: string
  color?: string // dominante Bildfarbe als Platzhalter
  progress?: number // Sekunden
  watched?: boolean
}

let cache: Promise<Item[]> | null = null

export function library(reload = false): Promise<Item[]> {
  if (!cache || reload) {
    cache = api<Item[] | null>('/api/library', profile).then((r) => {
      for (const it of r || []) byId[it.id] = it
      return r || []
    })
    cache.catch(() => (cache = null))
  }
  return cache
}

export function displayTitle(it: Item) {
  return (it.meta && it.meta.title) || it.title
}

const byId: Record<string, Item> = {}

// Serverseitiger Fortschritt, solange es ihn gibt; sonst der lokal gemerkte.
export function resumePos(id: string): number {
  const it = byId[id]
  if (it && it.progress !== undefined) return it.watched ? 0 : it.progress
  try {
    return Number(localStorage.getItem('pos:' + id) || 0)
  } catch {
    return 0
  }
}

export function image(it: Item, kind: 'poster' | 'backdrop', w: number) {
  const u = it[kind]
  if (u) return /[?&]w=\d+/.test(u) ? u.replace(/([?&]w=)\d+/, '$1' + w) : u + (u.indexOf('?') < 0 ? '?' : '&') + 'w=' + w
  return it.poster === '' || it[kind] === '' ? '' : `/api/images/${it.id}/${kind}?w=${w}`
}

export interface Series {
  name: string
  seasons: { season: number; episodes: Item[] }[]
  first: Item
}

export function groupSeries(items: Item[]): Series[] {
  const by: Record<string, Series> = {}
  const order: string[] = []
  for (const it of items) {
    if (!it.series) continue
    let s = by[it.series]
    if (!s) {
      s = by[it.series] = { name: it.series, seasons: [], first: it }
      order.push(it.series)
    }
    let season = s.seasons.filter((x) => x.season === (it.season || 0))[0]
    if (!season) {
      season = { season: it.season || 0, episodes: [] }
      s.seasons.push(season)
    }
    season.episodes.push(it)
  }
  return order.map((n) => {
    const s = by[n]
    s.seasons.sort((a, b) => a.season - b.season)
    for (const x of s.seasons) x.episodes.sort((a, b) => (a.episode || 0) - (b.episode || 0))
    s.first = s.seasons[0].episodes[0]
    return s
  })
}

// Nächste Episode nach der zuletzt angefangenen – für „Weiterschauen“ bei Serien.
export function nextUp(s: Series): Item {
  const all = s.seasons.reduce<Item[]>((acc, x) => acc.concat(x.episodes), [])
  let last = -1
  all.forEach((e, i) => {
    if (resumePos(e.id) > 0) last = i
  })
  if (last < 0) return all[0]
  const e = all[last]
  return resumePos(e.id) > e.duration * 0.9 && all[last + 1] ? all[last + 1] : e
}

export const lightText = {
  green: 'Läuft direkt – ohne Umwandlung',
  yellow: 'Läuft flüssig – der Server wandelt einen Teil um',
  red: 'Muss umgewandelt werden – dieser Server ist dafür knapp',
}

export function fmtTime(sec: number) {
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  const mm = h ? String(m).padStart(2, '0') : String(m)
  return (h ? h + ':' : '') + mm + ':' + String(s).padStart(2, '0')
}

export const BACK_KEYS = [461, 10009, 8, 27] // webOS, Tizen, Backspace, Escape

export interface HomeRow {
  id: string
  title: string
  items: Item[]
}

// Startseite vom Server (/api/home); fehlt die Route noch, werden die Reihen lokal gebildet.
export async function home(): Promise<HomeRow[]> {
  const items = await library()
  try {
    const rows = await api<HomeRow[]>('/api/home', profile)
    for (const r of rows) for (const it of r.items) byId[it.id] = it
    return rows.filter((r) => r.items && r.items.length)
  } catch (e) {
    if (e instanceof LoginError) throw e
  }
  const cont = items.filter((i) => resumePos(i.id) > 0)
  return cont.length ? [{ id: 'continue', title: 'Weiterschauen', items: cont }] : []
}

export class LoginError extends Error {
  setup: boolean
  constructor(setup: boolean) {
    super('login')
    this.setup = setup
  }
}
