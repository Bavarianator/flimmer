// Gemeinsam schauen: Uhrenabgleich und Drift-Korrektur. Reine Logik ohne DOM, damit sie testbar bleibt.

// Zustand eines Raums (internal/party State).
export interface PartyState {
  mediaId: string
  pos: number // Sekunden zum Zeitpunkt serverTs
  serverTs: number // Unix-ms
  rate: number
  paused: boolean
  waiting: string[]
  host: string
}

// Median der Uhrversätze (Server minus Client, ms) aus mehreren Messungen – ein Ausreißer (WLAN-Hänger) zählt nicht.
export function median(xs: number[]): number {
  const s = xs.slice().sort((a, b) => a - b)
  const m = s.length >> 1
  return s.length % 2 ? s[m] : (s[m - 1] + s[m]) / 2
}

// Ein Versatz aus einer Messung: t0/t1 = lokale Zeit vor/nach der Anfrage, server = Antwort von /api/time.
export function offsetOf(t0: number, server: number, t1: number): number {
  return server - (t0 + t1) / 2
}

// Soll-Position des Raums zum Server-Zeitpunkt now (ms).
export function expected(st: PartyState, now: number): number {
  return st.paused ? st.pos : st.pos + ((now - st.serverTs) / 1000) * (st.rate || 1)
}

// Korrektur für eine gemessene Abweichung drift = lokal − soll (Sekunden):
// unter 0,3 s nichts, bis 1 s sanft über playbackRate ±5 %, darüber springen.
export type Fix = { kind: 'none'; rate: number } | { kind: 'rate'; rate: number } | { kind: 'seek'; to: number }

export function correct(drift: number, target: number, rate = 1): Fix {
  const a = Math.abs(drift)
  if (a < 0.3) return { kind: 'none', rate }
  if (a <= 1) return { kind: 'rate', rate: rate * (drift > 0 ? 0.95 : 1.05) }
  return { kind: 'seek', to: Math.max(0, target) }
}

// Selbsttest: `npx tsx web/src/player/sync.ts` (oder im Browser-Konsolenimport) – wirft bei Fehlern.
export function selfCheck() {
  const ok = (c: boolean, m: string) => {
    if (!c) throw new Error('sync: ' + m)
  }
  ok(median([5, 1, 100, 3, 4]) === 4, 'median ungerade')
  ok(median([1, 2, 3, 4]) === 2.5, 'median gerade')
  ok(offsetOf(1000, 5100, 1200) === 4000, 'offset')
  const st: PartyState = { mediaId: 'x', pos: 10, serverTs: 0, rate: 1, paused: false, waiting: [], host: 'h' }
  ok(expected(st, 2000) === 12, 'expected läuft')
  ok(expected({ ...st, paused: true }, 2000) === 10, 'expected pausiert')
  ok(correct(0.2, 12).kind === 'none', 'klein')
  const r = correct(-0.5, 12)
  ok(r.kind === 'rate' && r.rate === 1.05, 'hinterher → schneller')
  const r2 = correct(0.5, 12)
  ok(r2.kind === 'rate' && r2.rate === 0.95, 'voraus → langsamer')
  const s = correct(3, 12)
  ok(s.kind === 'seek' && s.to === 12, 'seek')
  return true
}
