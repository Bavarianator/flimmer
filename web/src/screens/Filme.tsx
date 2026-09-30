// Filme-Bibliothek mit Tabs (/filme, /filme/vorschlaege, /filme/genres, /filme/studios).
// Die Umsetzung liegt in components/Bibliothek.tsx und lädt nach, damit das Start-JS klein bleibt.
import { spaeter } from '../components/Spaeter'

const B = spaeter(() => import('../components/Bibliothek').then((m) => m.Bibliothek))

export function Filme() {
  return <B art="filme" />
}
