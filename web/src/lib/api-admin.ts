// Datenfunktionen für Dashboard, Benutzer-Einstellungen und Metadaten-Editor.
// Formen wie im Vertrag (docs/umbau-jellyfin.md, docs/livetv-vpn.md) und in internal/api.
import { api, type Ampel, type Item, type Meta, type Nutzer } from './api'
import { deviceId } from '../profile'

const k = encodeURIComponent

// 202/204 ohne Body: api() scheitert dann an res.json(). Das ist hier Erfolg.
function leer(p: Promise<unknown>): Promise<void> {
  return p.then(
    () => {},
    (e) => {
      if (e instanceof SyntaxError) return
      throw e
    },
  )
}

// Go liefert leere Listen als null.
const liste = <T>(p: Promise<T[] | null>): Promise<T[]> => p.then((x) => x || [])

// Endpunkt fehlt (noch): 404, bzw. 405, wenn nur die Methode unbekannt ist.
export const fehlt = (fehler: string) => /^40[45]\b/.test(fehler)

// ---------- Typen ----------
export interface Session {
  id: string
  user: string
  userColor: number | string
  device: string
  client: string
  itemId: string
  title: string
  position: number
  duration: number
  paused: boolean
  method: 'direct' | 'remux' | 'transcode'
  light: Ampel
  reason?: string
  since: string
}

export interface Activity {
  time: string
  kind: 'login' | 'play' | 'scan' | 'error' | 'invite' | 'backup' | 'task' | 'user'
  user?: string
  text: string
}

export interface Overview {
  server: { name: string; version: string; os: string; arch: string; uptime: number; started: string; update?: { version: string; url: string } }
  library: { movies: number; series: number; episodes: number; sizeBytes: number; lastScan?: string }
  disks: { path: string; free: number; total: number }[]
  sessions: Session[]
  activity: Activity[]
}

export interface Device {
  id: string
  name: string
  client?: string
  user: string
  userId: string
  lastSeen: string
  ip?: string // client und ip nur bei Sessions nach dem Umbau
  current?: boolean
}

export interface Task {
  id: string
  name: string
  group: string
  description: string
  lastRun?: string
  lastResult?: 'ok' | 'error'
  lastError?: string
  duration?: number
  next?: string
  running: boolean
  progress?: number
}

export interface LogZeile {
  time: string
  level: string
  msg: string
  attrs?: Record<string, unknown>
}

export interface Vpn {
  addrs: { provider: string; interface: string; ip: string; url: string }[]
  hint: string
}

export interface LiveTVStatus {
  source: string
  epg: string
  video: 'copy' | 'h264'
  channels: number
  programs: number
  updated: string
  error: string
}

export interface Sendung {
  start: string
  stop: string
  title: string
  desc?: string
}

export interface Kanal {
  id: string
  number?: string
  name: string
  logo?: string
  group?: string
  now: Sendung | null
  next: Sendung | null
}

export interface Settings {
  serverName: string
  language: string
  dirs: string[]
  tmdbKey: boolean
  ffmpeg: { ok: boolean; canDownload: boolean; hint: string; download: { running: boolean; percent: number; error?: string } }
  lanUrl: string
  updateCheck: boolean
  remote: boolean
  optimize: { off: boolean; from: number; to: number; minFreeGB: number }
  remoteAvailable: boolean
  update: { version: string; url: string } | null
  version: string
}

export interface Einladung {
  id: string
  note: string
  scope: { libraries?: string[]; items?: string[] }
  expires: string
  maxUses: number
  uses: number
  guests: number
  created?: string
}

export interface Remote {
  method: string
  publicUrl: string
  reachable: boolean
  hint: string
}

export interface Optimize {
  waiting?: boolean
  on?: boolean
  window?: string
  current?: { title: string; percent: number }
  done?: number
  pending?: number
  lastError?: string
}

export interface Diag {
  version: string
  os: string
  cpus: number
  ffmpeg: string
  hw: string
  hwSpeed: number
  diskFree: number
  diskTotal: number
  cacheDir: string
  scan: { scanning: boolean; found: number }
  active: { user: string; title: string; device: string; method: string; light: Ampel; reasons: string[] | null }[] | null
  recent: Diag['active']
  log: string[] | null
  db: { checkedAt: string; integrity: string; sizeBytes: number; backupAt: string; backupFile: string }
}

export interface Unsicher {
  id: string
  file: string
  meta: { title?: string; year?: number } | null
  guess: string
}

export interface Kandidat {
  tmdbId: number
  title: string
  year?: number
}

export interface Person {
  name: string
  role?: string
  kind: 'actor' | 'director' | 'writer' | 'producer'
  image?: string
}

export interface MetaVoll extends Meta {
  sortTitle?: string
  tagline?: string
  studios?: string[]
  tags?: string[]
  people?: Person[]
  age?: number | null
  tmdbId?: number
  imdbId?: string
}

export interface Details extends Item {
  meta?: MetaVoll | null
  tagline?: string
  studios?: string[]
  countries?: string[]
  people?: Person[]
  tags?: string[]
  path?: string
  container?: string
  locked?: string[]
}

export interface MetaAenderung {
  title?: string
  originalTitle?: string
  sortTitle?: string
  year?: number
  overview?: string
  tagline?: string
  genres?: string[]
  tags?: string[]
  studios?: string[]
  age?: number | null
  rating?: number
  people?: Person[]
  locked: string[] // immer mitschicken: fehlt es, sperrt der Server die geänderten Felder; [] entsperrt alles
}

// Aufgabe „Metadaten aktualisieren“: füllt Personen, Studios, Leitsatz, Länder und Filmreihen im Bestand nach.
export const META_AUFGABE = 'meta'

// ---------- Dashboard (neu, Agent „server“) ----------
export const uebersicht = () =>
  api<Overview>('/api/admin/overview').then((o) => ({ ...o, disks: o.disks || [], sessions: o.sessions || [], activity: o.activity || [] }))
export const sitzungen = () => liste(api<Session[] | null>('/api/sessions'))
export const aktivitaeten = (limit = 100) => liste(api<Activity[] | null>('/api/activity?limit=' + limit))
export const geraete = () => liste(api<Device[] | null>('/api/devices'))
export const geraetAbmelden = (id: string) => leer(api('/api/devices/' + k(id), undefined, 'DELETE'))
export const aufgaben = () => liste(api<Task[] | null>('/api/tasks'))
export const aufgabeStarten = (id: string) => leer(api('/api/tasks/' + k(id) + '/run', {}))
export const protokoll = (level = '', limit = 500) => liste(api<LogZeile[] | null>('/api/logs?level=' + k(level) + '&limit=' + limit))
export const vpn = () => api<Vpn>('/api/vpn').then((v) => ({ ...v, addrs: v.addrs || [] }))
export const liveTV = () => api<LiveTVStatus>('/api/livetv')
export const liveTVSetzen = (teil: { source?: string; epg?: string; video?: string }) => leer(api('/api/livetv', teil, 'PUT'))
export const liveTVNeu = () => leer(api('/api/livetv/refresh', {}))
export const kanaele = () => liste(api<Kanal[] | null>('/api/livetv/channels'))
// Vorlagen für die Einrichtung per Knopf. channels = 0: nicht gefunden (FRITZ!Box). Übernehmen mit liveTVSetzen.
export interface LiveTVVorlage {
  id: 'frei' | 'fritz'
  source: string
  epg: string
  channels: number
  active: boolean
}
export const liveTVVorlagen = () => api<LiveTVVorlage[]>('/api/livetv/vorlagen')
// ?device= wählt das Geräteprofil (Ampel, Methode); ohne gilt ein leeres Profil.
export const details = (id: string) => api<Details>('/api/items/' + k(id) + '?device=' + k(deviceId()))
export const metaSpeichern = (id: string, teil: MetaAenderung) => api<Details>('/api/items/' + k(id) + '/meta', teil, 'PUT')

// ---------- bestehende Routen ----------
export const einstellungen = () => api<Settings>('/api/settings')
export const einstellungenSetzen = (teil: Record<string, unknown>) => api<Settings>('/api/settings', teil, 'PUT')
export const neuEinlesen = () => leer(api('/api/rescan', {}))
export const scanStatus = () => api<{ scanning: boolean; found: number }>('/api/status')
export const ordner = (path: string) => api<{ path: string; parent: string; dirs: { name: string; path: string }[] }>('/api/setup/dirs?path=' + k(path))
export const ffmpegLaden = () => leer(api('/api/setup/ffmpeg', {}))
export const benutzer = () => liste(api<Nutzer[] | null>('/api/users'))
export const benutzerNeu = (b: { name: string; password?: string; admin: boolean; color: number }) => api<Nutzer>('/api/users', b)
export const benutzerSetzen = (id: string, teil: { name?: string; password?: string; admin?: boolean; color?: number; upload?: boolean }) => api<Nutzer>('/api/users/' + k(id), teil, 'PUT')
export const benutzerLoeschen = (id: string) => leer(api('/api/users/' + k(id), undefined, 'DELETE'))
export const einladungen = () => liste(api<Einladung[] | null>('/api/invites'))
export const einladungNeu = (b: { note: string; libraries: string[]; hours: number; maxUses: number }) => api<{ url: string; qr?: string; hint?: string }>('/api/invites', b)
export const einladungWeg = (id: string) => leer(api('/api/invites/' + k(id), undefined, 'DELETE'))
export const fernzugriff = () => api<Remote>('/api/remote')
export const fernzugriffPruefen = () => api<Remote>('/api/remote/check', {})
export const appCode = () => api<{ code: string; expires: string }>('/api/remote/pair', {})
export const optimieren = () => api<Optimize>('/api/settings/optimize')
export const diagnose = () => api<Diag>('/api/diagnostics')
export const unsicher = () => liste(api<Unsicher[] | null>('/api/settings/review'))
export const kandidaten = (id: string, q: string) => liste(api<Kandidat[] | null>('/api/items/' + k(id) + '/search?q=' + k(q)))
export const identifizieren = (id: string, tmdbId: number) => leer(api('/api/items/' + k(id) + '/identify', { tmdbId }))
export const koppeln = (code: string) => api<{ device: string }>('/api/pair/' + k(code) + '/confirm', {})
