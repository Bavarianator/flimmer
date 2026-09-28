import { applyProbe, loadProbe, type ProbeResult } from './probe'

// Ermittelt, was dieses Gerät abspielen kann. Der Server entscheidet damit pro Titel über
// Direct Play / Remux / Ton-Transcoding und zeigt die Ampel passend zu genau diesem Gerät.

export interface Profile {
  name: string
  containers: string[]
  video: string[]
  audio: string[]
  nativeHls: boolean
  maxBitrate: number
}

const ua = navigator.userAgent
export const isWebOS = /Web0S|webOS/i.test(ua)
export const isTizen = /Tizen/i.test(ua)
export const isTV = isWebOS || isTizen || /SMART-TV|SmartTV|AFT|BRAVIA/i.test(ua)

const videoTypes: Record<string, string> = {
  h264: 'video/mp4; codecs="avc1.640028"',
  hevc: 'video/mp4; codecs="hvc1.1.6.L120.90"',
  hevc10: 'video/mp4; codecs="hvc1.2.4.L120.90"',
  av1: 'video/mp4; codecs="av01.0.08M.08"',
  vp9: 'video/webm; codecs="vp9"',
}

const audioTypes: Record<string, string[]> = {
  aac: ['audio/mp4; codecs="mp4a.40.2"'],
  ac3: ['audio/mp4; codecs="ac-3"'],
  eac3: ['audio/mp4; codecs="ec-3"'],
  mp3: ['audio/mpeg'],
  opus: ['audio/webm; codecs="opus"', 'audio/mp4; codecs="opus"'],
  flac: ['audio/flac', 'audio/mp4; codecs="fLaC"'],
}

function can(type: string): boolean {
  const v = document.createElement('video')
  return v.canPlayType(type) !== ''
}

export function detectProfile(): Profile {
  const p: Profile = {
    name: isWebOS ? 'LG webOS' : isTizen ? 'Samsung Tizen' : 'Browser',
    containers: ['mp4'],
    video: Object.keys(videoTypes).filter((k) => can(videoTypes[k])),
    audio: Object.keys(audioTypes).filter((k) => audioTypes[k].some(can)),
    nativeHls: can('application/vnd.apple.mpegurl'),
    maxBitrate: 0,
  }
  if (can('video/webm')) p.containers.push('webm')
  // Schicht 1 (statische Regeln): TVs spielen MKV/TS nativ, melden es aber nicht per canPlayType.
  if (isWebOS || isTizen) {
    p.containers.push('mkv', 'ts')
    for (const a of ['aac', 'ac3', 'eac3', 'mp3']) if (p.audio.indexOf(a) < 0) p.audio.push(a)
  }
  return p
}

// Aktuelles Profil: Schicht 1+2 (detectProfile) plus gemessene Schicht 3 (Probe-Clips).
export let profile = applyProbe(detectProfile(), loadProbe())

export function setProbe(r: ProbeResult) {
  profile = applyProbe(detectProfile(), r)
  // Server merkt sich die Messung pro Gerät (Diagnose, Neuinstallation der App). Fehler sind egal.
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (getToken()) headers.Authorization = 'Bearer ' + getToken()
  fetch(`/api/devices/${deviceId()}/profile`, {
    method: 'PUT',
    credentials: 'same-origin',
    headers,
    body: JSON.stringify({ name: profile.name, probe: r }),
  }).catch(() => {})
}

export function deviceId(): string {
  let id = ''
  try {
    id = localStorage.getItem('flimmer.device') || ''
    if (!id) {
      id = Math.random().toString(36).slice(2) + Date.now().toString(36)
      localStorage.setItem('flimmer.device', id)
    }
  } catch {}
  return id || 'unbekannt'
}

export async function api<T>(path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  const token = getToken()
  if (token) headers.Authorization = 'Bearer ' + token // TVs nach Kopplung; Browser nutzen das Cookie
  const res = await fetch(path, {
    credentials: 'same-origin',
    method: body === undefined ? 'GET' : 'POST',
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (res.status === 401) {
    let setup = false
    try {
      setup = !!(await res.json()).setup
    } catch {}
    const { LoginError } = await import('./data')
    throw new LoginError(setup)
  }
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`)
  return res.status === 204 ? (undefined as T) : res.json()
}

export function getToken(): string {
  try {
    return localStorage.getItem('flimmer.token') || ''
  } catch {
    return ''
  }
}

export function setToken(t: string) {
  try {
    if (t) localStorage.setItem('flimmer.token', t)
    else localStorage.removeItem('flimmer.token')
  } catch {}
}
