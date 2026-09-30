// Startseite pro Profil: welche Reihen erscheinen und in welcher Reihenfolge. Der Server kennt dafür (noch)
// keine Route; die Wahl liegt wie die Wiedergabe-Vorlieben in localStorage, getrennt nach Profil.
// Start.tsx (web-browse) ruft startReihen(rows) auf die Antwort von POST /api/home an.
import { gecacht, type Nutzer } from '../../lib/api'

export interface StartVorlieben {
  reihenfolge: string[] // Reihen-IDs aus /api/home, unbekannte hängen hinten an
  aus: string[]
}

// Reihen, die der Server heute liefert (internal/api/library.go, home).
export const START_REIHEN = ['continue', 'nextup', 'recent-movies', 'recent-series']

function schluessel() {
  const ich = gecacht<Nutzer>('ich')
  return 'flimmer.startseite.' + (ich ? ich.id : 'geraet')
}

export function startVorlieben(): StartVorlieben {
  try {
    const v = JSON.parse(localStorage.getItem(schluessel()) || 'null')
    if (v && v.reihenfolge && v.aus) return v
  } catch {}
  return { reihenfolge: START_REIHEN.slice(), aus: [] }
}

export function setStartVorlieben(v: StartVorlieben) {
  try {
    localStorage.setItem(schluessel(), JSON.stringify(v))
  } catch {}
}

// Filtert und sortiert die Reihen der Startseite nach der Wahl des Profils.
export function startReihen<T extends { id: string }>(rows: T[]): T[] {
  const v = startVorlieben()
  const rang = (id: string) => {
    const i = v.reihenfolge.indexOf(id)
    return i < 0 ? 999 : i
  }
  return rows.filter((r) => v.aus.indexOf(r.id) < 0).sort((a, b) => rang(a.id) - rang(b.id))
}
