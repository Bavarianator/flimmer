// Wiedergabe-Vorlieben pro Profil: Sprachkette, Untertitel-Modus, Nachtmodus, Qualität.
// Der Server kennt (noch) keine Route dafür; sie reisen im Body von /api/items/{id}/play mit und liegen
// bis dahin in localStorage, getrennt nach Profil (siehe NOTIZEN.md).
import { gecacht, type Nutzer } from '../lib/api'
import { sprache } from '../lib/i18n'
import { BILD_MODI, type BildModus } from './bild'

export interface Vorlieben {
  audioLangs: string[] // z. B. ["de", "en"]
  subtitleMode: '' | 'always' | 'off' // "" = automatisch
  night: boolean
  maxHeight: number // Qualitätswahl: höchstens so viele Bildzeilen (1080/720/480), 0 = automatisch
}

function schluessel() {
  const ich = gecacht<Nutzer>('ich')
  return 'flimmer.vorlieben.' + (ich ? ich.id : 'geraet')
}

export function vorlieben(): Vorlieben {
  const std: Vorlieben = { audioLangs: sprache === 'en' ? ['en'] : ['de', 'en'], subtitleMode: '', night: false, maxHeight: 0 }
  try {
    const v = JSON.parse(localStorage.getItem(schluessel()) || 'null')
    return v ? { ...std, ...v } : std
  } catch {
    return std
  }
}

export function setVorlieben(teil: Partial<Vorlieben>) {
  try {
    localStorage.setItem(schluessel(), JSON.stringify({ ...vorlieben(), ...teil }))
  } catch {}
}

// Bildanpassung: pro Gerät (nicht pro Profil), weil sie vom Bildschirm abhängt. Standard „Automatisch“.
export function bildModus(): BildModus {
  try {
    const m = localStorage.getItem('flimmer.bild') as BildModus | null
    return m && BILD_MODI.indexOf(m) >= 0 ? m : 'auto'
  } catch {
    return 'auto'
  }
}

export function setBildModus(m: BildModus) {
  try {
    localStorage.setItem('flimmer.bild', m)
  } catch {}
}
