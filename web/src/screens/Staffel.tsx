// Staffel (/serie/:name/:staffel). Umsetzung in components/Details.tsx.
import { StaffelSeite } from '../components/Details'

export function Staffel(p: { name: string; staffel: string }) {
  return <StaffelSeite name={p.name} staffel={p.staffel} />
}
