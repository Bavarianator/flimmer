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
  // TODO(v0.1): Schicht 3 – echte Probe-Clips abspielen und Ergebnis gewinnen lassen.
  return p
}

export const profile = detectProfile()

export async function api<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method: body === undefined ? 'GET' : 'POST',
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`)
  return res.json()
}
