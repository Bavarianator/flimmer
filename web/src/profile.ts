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
  // undefined = unbekannt (Server rechnet nichts um), [] = SDR, sonst Teilmenge von hdr10/hlg/hdr10+/dv
  hdr?: string[]
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
// HDR: Displays und Decoder melden das nur asynchron. Bis die Antwort da ist, bleibt das Feld
// weg (= unbekannt, altes Verhalten); danach gilt es für alle folgenden Anfragen.
let hdr: string[] | undefined = staticHdr()

function staticHdr(): string[] | undefined {
  // Auf TVs lügen die Web-APIs oft – dort gelten feste Regeln.
  if (isWebOS) return ['hdr10', 'hlg', 'dv']
  if (isTizen) return ['hdr10', 'hlg', 'hdr10+']
  if (!window.matchMedia) return undefined
  if (matchMedia('(dynamic-range: high)').matches) return undefined // wird unten per MediaCapabilities bestimmt
  return matchMedia('(dynamic-range: standard)').matches ? [] : undefined // weder noch: Browser kennt die Abfrage nicht
}

async function detectHdr(): Promise<string[] | undefined> {
  const mc = (navigator as Navigator & { mediaCapabilities?: MediaCapabilities }).mediaCapabilities
  if (!mc) return ['hdr10'] // Display kann HDR, Details unbekannt
  const probe = (extra: Record<string, string>) =>
    mc
      .decodingInfo({
        type: 'media-source',
        video: { contentType: 'video/mp4; codecs="hvc1.2.4.L153.B0"', width: 3840, height: 2160, bitrate: 20e6, framerate: 24, colorGamut: 'rec2020', ...extra },
      } as MediaDecodingConfiguration)
      .then((r) => r.supported, () => false)
  const [hdr10, hlg, hdr10plus] = await Promise.all([
    probe({ transferFunction: 'pq', hdrMetadataType: 'smpteSt2086' }),
    probe({ transferFunction: 'hlg' }),
    probe({ transferFunction: 'pq', hdrMetadataType: 'smpteSt2094-40' }),
  ])
  const out: string[] = []
  if (hdr10) out.push('hdr10')
  if (hlg) out.push('hlg')
  if (hdr10plus) out.push('hdr10+')
  return out
}

function withHdr(p: Profile): Profile {
  return hdr === undefined ? p : { ...p, hdr }
}

export let profile = withHdr(applyProbe(detectProfile(), loadProbe()))

if (hdr === undefined && typeof window.matchMedia === 'function' && matchMedia('(dynamic-range: high)').matches) {
  detectHdr().then((h) => {
    hdr = h
    profile = withHdr(profile)
  })
}

export function setProbe(r: ProbeResult) {
  profile = withHdr(applyProbe(detectProfile(), r))
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
