// Serie-Detail (/serie/:name). Umsetzung in components/Details.tsx, lädt nach.
import { spaeter } from '../components/Spaeter'

export const DetailSerie = spaeter<{ name: string }>(() => import('../components/Details').then((m) => m.DetailSerie))
