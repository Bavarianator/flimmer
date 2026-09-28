// Schicht 3 der Geräteerkennung: winzige Clips wirklich abspielen. TVs melden per canPlayType oft
// Unsinn (z. B. „DTS ja“ und dann Stille) – was hier gemessen wird, gewinnt.
import type { Profile } from './profile'

export interface ProbeResult {
  ua: string
  played: string[] // z. B. "video:hevc", "audio:dts", "container:mkv"
  failed: string[]
}

const clips: [string, string][] = [
  ['video:h264', 'h264.mp4'],
  ['video:hevc', 'hevc.mp4'],
  ['video:hevc10', 'hevc10.mp4'],
  ['video:av1', 'av1.mp4'],
  ['video:vp9', 'vp9.webm'],
  ['container:mkv', 'mkv.mkv'],
  ['container:ts', 'ts.ts'],
  ['audio:aac', 'aac.mp4'],
  ['audio:ac3', 'ac3.mp4'],
  ['audio:eac3', 'eac3.mp4'],
  ['audio:mp3', 'mp3.mp4'],
  ['audio:opus', 'opus.mp4'],
  ['audio:flac', 'flac.mp4'],
  ['audio:dts', 'dts.mkv'],
  ['audio:truehd', 'truehd.mkv'],
]

type Decoded = HTMLVideoElement & { webkitAudioDecodedByteCount?: number; webkitVideoDecodedByteCount?: number }

// Ergebnis: true = läuft, false = läuft nicht, null = nicht messbar (dann bleibt Schicht 1/2 gültig).
function playClip(url: string, kind: string, signal: { aborted: boolean }): Promise<boolean | null> {
  return new Promise((resolve) => {
    const v = document.createElement('video') as Decoded
    v.muted = true // Autoplay-Regeln; dekodiert wird trotzdem
    v.setAttribute('playsinline', '')
    v.style.cssText = 'position:fixed;left:-10px;top:-10px;width:1px;height:1px;opacity:0;pointer-events:none'
    let done = false
    const finish = (r: boolean | null) => {
      if (done) return
      done = true
      clearTimeout(timer)
      v.removeAttribute('src')
      v.load()
      v.remove()
      resolve(r)
    }
    const timer = setTimeout(() => finish(false), 2500) // 1-s-Clips: was bis dahin nicht läuft, läuft nicht
    v.onerror = () => finish(false)
    v.ontimeupdate = () => {
      if (signal.aborted) return finish(null)
      if (v.currentTime < 0.3) return
      if (kind === 'audio') {
        // Ohne diese Chromium-Zähler lässt sich stummes Scheitern nicht erkennen.
        finish(v.webkitAudioDecodedByteCount === undefined ? null : v.webkitAudioDecodedByteCount > 0)
      } else {
        finish(v.videoWidth > 0 && (v.webkitVideoDecodedByteCount === undefined || v.webkitVideoDecodedByteCount > 0))
      }
    }
    document.body.appendChild(v)
    v.src = new URL('probe/' + url, location.href).href
    const p = v.play()
    if (p) p.catch(() => {}) // Fehler kommen über onerror/Timeout
  })
}

const KEY = 'flimmer.probe'

export function loadProbe(): ProbeResult | null {
  try {
    const r: ProbeResult = JSON.parse(localStorage.getItem(KEY) || 'null')
    return r && r.ua === navigator.userAgent ? r : null // neue Firmware/Browser → neu testen
  } catch {
    return null
  }
}

// Ein Gerät hat oft nur einen Hardware-Decoder: der Test bricht ab, sobald die Wiedergabe startet.
export const probeSignal = { aborted: false }

export async function runProbe(): Promise<ProbeResult | null> {
  probeSignal.aborted = false
  const r: ProbeResult = { ua: navigator.userAgent, played: [], failed: [] }
  for (const [key, url] of clips) {
    const ok = await playClip(url, key.split(':')[0], probeSignal)
    if (probeSignal.aborted) return null
    if (ok === true) r.played.push(key)
    if (ok === false) r.failed.push(key)
  }
  try {
    localStorage.setItem(KEY, JSON.stringify(r))
  } catch {}
  return r
}

export function applyProbe(p: Profile, r: ProbeResult | null): Profile {
  if (!r) return p
  const out: Profile = { ...p, containers: p.containers.slice(), video: p.video.slice(), audio: p.audio.slice() }
  const lists: Record<string, string[]> = { video: out.video, audio: out.audio, container: out.containers }
  for (const key of r.played) {
    const [kind, name] = key.split(':')
    if (lists[kind] && lists[kind].indexOf(name) < 0) lists[kind].push(name)
  }
  for (const key of r.failed) {
    const [kind, name] = key.split(':')
    const l = lists[kind]
    if (l && l.indexOf(name) >= 0) l.splice(l.indexOf(name), 1)
  }
  return out
}
