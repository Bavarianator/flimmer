// Wiedergabe-Vorlieben pro Profil: Sprachkette, Untertitel-Modus, Nachtmodus.
// Der Server kennt (noch) keine Route dafür; sie reisen im Body von /api/items/{id}/play mit und liegen
// bis dahin in localStorage, getrennt nach Profil (siehe NOTIZEN.md).
import { gecacht, type Nutzer } from '../lib/api'
import { sprache } from '../lib/i18n'

export interface Vorlieben {
  audioLangs: string[] // z. B. ["de", "en"]
  subtitleMode: '' | 'always' | 'off' // "" = automatisch
  night: boolean
}

function schluessel() {
  const ich = gecacht<Nutzer>('ich')
  return 'flimmer.vorlieben.' + (ich ? ich.id : 'geraet')
}

export function vorlieben(): Vorlieben {
  const std: Vorlieben = { audioLangs: sprache === 'en' ? ['en'] : ['de', 'en'], subtitleMode: '', night: false }
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
