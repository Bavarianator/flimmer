// Film-Detail (/film/:id, auch für einzelne Folgen). Umsetzung in components/Details.tsx, lädt nach.
import { spaeter } from '../components/Spaeter'

export const DetailFilm = spaeter<{ id: string }>(() => import('../components/Details').then((m) => m.DetailFilm))
