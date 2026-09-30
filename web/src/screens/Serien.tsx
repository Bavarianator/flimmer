// Serien-Bibliothek mit Tabs (/serien, /serien/vorschlaege, /serien/genres, /serien/folgen).
// Die Umsetzung liegt in components/Bibliothek.tsx und lädt nach, damit das Start-JS klein bleibt.
import { spaeter } from '../components/Spaeter'

const B = spaeter(() => import('../components/Bibliothek').then((m) => m.Bibliothek))

export function Serien() {
  return <B art="serien" />
}
