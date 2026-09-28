// Wiedergabeplan (POST /api/items/{id}/play), Spur-Beschriftungen und die Texte des Players.
import { ergaenze, sprache, t } from '../lib/i18n'

export interface Untertitel {
  index: number
  language?: string
  title?: string
  format: 'vtt' | 'pgs'
  url?: string
  forced?: boolean
  sdh?: boolean
}

export interface Tonspur {
  index: number
  language?: string
  title?: string
  codec: string // nur anzeigen, nicht auswerten
  channels?: number
  default?: boolean
}

export interface Plan {
  method: 'direct-play' | 'direct-stream' | 'transcode-audio' | 'transcode'
  light: 'green' | 'yellow' | 'red'
  reasons?: string[] // warum umgewandelt wird
  notes?: string[] // dezente Hinweise ohne Einfluss auf die Methode
  url: string
  title: string
  duration: number
  resume?: number
  audioIndex: number
  audioCodec?: string
  videoCodec?: string
  audio: Tonspur[] | null
  subtitles: Untertitel[] | null
  subtitleIndex?: number // -1 = keine
  prefs?: { audio?: string; subtitle?: string } // pro Serie gemerkt
  optimized?: boolean
}

ergaenze(
  {
    'player.methode.direct-play': 'Läuft direkt',
    'player.methode.direct-stream': 'Läuft direkt, neu verpackt',
    'player.methode.transcode-audio': 'Nur der Ton wird umgewandelt',
    'player.methode.transcode': 'Wird umgewandelt',
    'player.optimiert': 'vorbereitete Fassung',
    'player.warum': 'Warum umgewandelt?',
    'player.warum.titel': 'Darum wandelt der Server um',
    'player.ton': 'Ton',
    'player.untertitel': 'Untertitel',
    'player.aus': 'Aus',
    'player.nachtmodus': 'Nachtmodus',
    'player.nachtmodus.text': 'Dialoge klarer, Explosionen leiser',
    'player.an': 'An',
    'player.erzwungen': 'Erzwungen',
    'player.sdh': 'SDH',
    'player.bild': 'Bild',
    'player.spur': 'Spur {n}',
    'player.mono': 'Mono',
    'player.stereo': 'Stereo',
    'player.menue': 'Ton & Untertitel',
    'player.naechste': 'Nächste Folge',
    'player.abspielen': 'Abspielen',
    'player.pause': 'Pause',
    'player.zurueck10': '10 Sekunden zurück',
    'player.vor10': '10 Sekunden vor',
    'player.endet': 'endet um {zeit}',
    'player.zeitleiste': 'Zeitleiste',
    'player.laedt': 'Wird geladen …',
    'player.fehler.titel': 'Das lässt sich gerade nicht abspielen',
    'player.fehler.hls': 'Dieses Gerät kann den Stream nicht abspielen.',
    'player.fehler.netz': 'Die Verbindung zum Server ist abgerissen.',
    'player.fehler.server': 'Der Server konnte die Wiedergabe nicht vorbereiten.',
    'player.naechste.jetzt': 'Jetzt ansehen',
    'player.naechste.in': 'Nächste Folge in {n} s',
    'player.naechste.abspann': 'Abspann',
    'player.gemeinsam.kurz': 'Gemeinsam',
    'player.gemeinsam': 'Gemeinsam schauen',
    'player.reaktionen': 'Reaktionen',
  },
  {
    'player.methode.direct-play': 'Plays directly',
    'player.methode.direct-stream': 'Plays directly, repackaged',
    'player.methode.transcode-audio': 'Only the audio is converted',
    'player.methode.transcode': 'Being converted',
    'player.optimiert': 'prepared version',
    'player.warum': 'Why converted?',
    'player.warum.titel': 'Why the server converts',
    'player.ton': 'Audio',
    'player.untertitel': 'Subtitles',
    'player.aus': 'Off',
    'player.nachtmodus': 'Night mode',
    'player.nachtmodus.text': 'Clearer dialogue, quieter explosions',
    'player.an': 'On',
    'player.erzwungen': 'Forced',
    'player.spur': 'Track {n}',
    'player.menue': 'Audio & subtitles',
    'player.naechste': 'Next episode',
    'player.abspielen': 'Play',
    'player.pause': 'Pause',
    'player.zurueck10': 'Back 10 seconds',
    'player.vor10': 'Forward 10 seconds',
    'player.endet': 'ends at {zeit}',
    'player.zeitleiste': 'Timeline',
    'player.laedt': 'Loading …',
    'player.fehler.titel': 'This cannot be played right now',
    'player.fehler.hls': 'This device cannot play the stream.',
    'player.fehler.netz': 'The connection to the server was lost.',
    'player.fehler.server': 'The server could not prepare playback.',
    'player.naechste.jetzt': 'Watch now',
    'player.naechste.in': 'Next episode in {n} s',
    'player.naechste.abspann': 'Credits',
    'player.gemeinsam.kurz': 'Together',
    'player.gemeinsam': 'Watch together',
    'player.reaktionen': 'Reactions',
  },
)

const sprachen: Record<string, [string, string]> = {
  de: ['Deutsch', 'German'],
  en: ['Englisch', 'English'],
  fr: ['Französisch', 'French'],
  es: ['Spanisch', 'Spanish'],
  it: ['Italienisch', 'Italian'],
  ja: ['Japanisch', 'Japanese'],
  tr: ['Türkisch', 'Turkish'],
  pl: ['Polnisch', 'Polish'],
  ru: ['Russisch', 'Russian'],
  nl: ['Niederländisch', 'Dutch'],
  sv: ['Schwedisch', 'Swedish'],
  da: ['Dänisch', 'Danish'],
  ko: ['Koreanisch', 'Korean'],
  zh: ['Chinesisch', 'Chinese'],
}
const drei: Record<string, string> = { ger: 'de', deu: 'de', eng: 'en', fre: 'fr', fra: 'fr', spa: 'es', ita: 'it', jpn: 'ja', tur: 'tr', pol: 'pl', rus: 'ru', dut: 'nl', nld: 'nl', swe: 'sv', dan: 'da', kor: 'ko', chi: 'zh', zho: 'zh' }

export function sprachname(c?: string): string {
  if (!c || c === 'und') return ''
  const k = drei[c.toLowerCase()] || c.toLowerCase()
  const n = sprachen[k]
  return n ? n[sprache === 'en' ? 1 : 0] : c.toUpperCase()
}

export function untertitelName(s: Untertitel): string {
  const teile = [sprachname(s.language) || t('player.spur', { n: s.index }), s.title]
  if (s.forced) teile.push(t('player.erzwungen'))
  if (s.sdh) teile.push(t('player.sdh'))
  if (s.format === 'pgs') teile.push(t('player.bild'))
  return uniq(teile).join(' · ')
}

export function tonName(a: Tonspur): string {
  const ch = a.channels ? (a.channels > 2 ? a.channels - 1 + '.1' : a.channels === 1 ? t('player.mono') : t('player.stereo')) : ''
  return uniq([sprachname(a.language) || t('player.spur', { n: a.index }), a.title, [a.codec.toUpperCase(), ch].filter(Boolean).join(' ')]).join(' · ')
}

// Titel wie „Deutsch“ neben der Sprache „Deutsch“ nicht doppelt zeigen.
function uniq(xs: (string | undefined)[]): string[] {
  const out: string[] = []
  for (const x of xs) if (x && out.map((o) => o.toLowerCase()).indexOf(x.toLowerCase()) < 0) out.push(x)
  return out
}

export function untertitelUrl(id: string, s: Untertitel) {
  return s.url || `/api/items/${id}/subs/${s.index}.${s.format === 'pgs' ? 'sup' : 'vtt'}`
}

// Welche Untertitelspur startet: gemerkte Wahl der Serie, sonst der Vorschlag des Servers (subtitleIndex).
export function startUntertitel(p: Plan): number {
  const subs = p.subtitles || []
  const gemerkt = p.prefs && p.prefs.subtitle
  if (gemerkt === 'off') return subs.filter((s) => s.index === p.subtitleIndex && s.forced).length ? (p.subtitleIndex as number) : -1
  if (gemerkt) {
    const s = subs.filter((x) => x.language === gemerkt && !x.forced)[0]
    if (s) return s.index
  }
  return p.subtitleIndex === undefined ? -1 : p.subtitleIndex
}
