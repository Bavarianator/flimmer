// TV-Kopplung: Code holen (POST /api/pair), anzeigen, pollen bis das Handy bestätigt (GET /api/pair/{code}).
import { useEffect, useState } from 'preact/hooks'
import { api } from '../lib/api'
import { profile, setToken } from '../profile'

export interface KoppelZustand {
  code: string
  bis: number // Ablauf (ms)
}

export function useKoppelCode(aktiv: boolean, onDone: () => void): KoppelZustand | null {
  const [z, setZ] = useState<KoppelZustand | null>(null)
  useEffect(() => {
    if (!aktiv) return
    let stop = false
    let timer = 0
    const start = () =>
      api<{ code: string; secret: string; expiresIn: number }>('/api/pair', { device: profile.name }).then(
        (p) => {
          if (stop) return
          setZ({ code: p.code, bis: Date.now() + p.expiresIn * 1000 })
          const poll = () =>
            fetch(`/api/pair/${p.code}?secret=${encodeURIComponent(p.secret)}`).then(
              (r) => {
                if (stop) return
                if (r.status === 200)
                  return r.json().then((d: { token: string }) => {
                    setToken(d.token)
                    onDone()
                  })
                if (r.status === 404) return start() // abgelaufen → neuer Code
                timer = window.setTimeout(poll, 2000)
              },
              () => (timer = window.setTimeout(poll, 5000)),
            )
          poll()
        },
        () => (timer = window.setTimeout(start, 10000)),
      )
    start()
    return () => {
      stop = true
      clearTimeout(timer)
    }
  }, [aktiv])
  return z
}

// „453 432“ in zwei Gruppen, Plex Mono.
export function KoppelCode({ code, gross }: { code: string; gross?: boolean }) {
  return (
    <div class={'fl-code' + (gross ? ' kopplung-code-gross' : '')} aria-label={code.split('').join(' ')}>
      <span>{code.slice(0, 3)}</span>
      <span>{code.slice(3)}</span>
    </div>
  )
}
