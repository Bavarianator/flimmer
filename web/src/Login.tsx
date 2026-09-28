import { useEffect, useState } from 'preact/hooks'
import { FocusContext, useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation'
import { api, isTV, profile, setToken } from './profile'
import { FocusButton, useBack } from './ui'
import { back, go } from './route'

interface User {
  id: string
  name: string
  color: string | number // Farbton (0–360) oder CSS-Farbe
  admin: boolean
  hasPassword: boolean
}

function UserTile({ u, onPick }: { u: User; onPick: (u: User) => void }) {
  const { ref, focused } = useFocusable({ focusKey: 'user-' + u.id, onEnterPress: () => onPick(u) })
  return (
    <button ref={ref} class={'user' + (focused ? ' focused' : '')} onClick={() => onPick(u)}>
      <span class="avatar" style={{ background: typeof u.color === 'number' ? `hsl(${u.color},55%,45%)` : u.color || '#7c9cff' }}>
        {u.name.charAt(0).toUpperCase()}
      </span>
      <span>{u.name}</span>
    </button>
  )
}

// Profilauswahl im Netflix-Stil. Auf TVs zusätzlich Kopplung per Code, damit niemand ein Passwort
// mit der Fernbedienung tippen muss.
export function Login({ onDone }: { onDone: () => void }) {
  const [users, setUsers] = useState<User[] | null>(null)
  const [pick, setPick] = useState<User | null>(null)
  const [pw, setPw] = useState('')
  const [error, setError] = useState('')
  const [pair, setPair] = useState<{ code: string; secret: string } | null>(null)
  const { ref, focusKey } = useFocusable({ focusKey: 'login' })

  useEffect(() => {
    api<User[]>('/api/users').then(setUsers, (e) => setError(String(e.message || e)))
  }, [])
  useEffect(() => {
    if (users && users.length) setFocus('user-' + users[0].id)
  }, [users])

  // TV-Kopplung: Code anzeigen, pollen bis am Handy bestätigt.
  useEffect(() => {
    if (!isTV) return
    let stop = false
    let timer = 0
    const start = () =>
      api<{ code: string; secret: string; expiresIn: number }>('/api/pair', { device: profile.name }).then((p) => {
        if (stop) return
        setPair(p)
        const poll = () =>
          fetch(`/api/pair/${p.code}?secret=${encodeURIComponent(p.secret)}`).then((r) => {
            if (stop) return
            if (r.status === 200) return r.json().then((d: { token: string }) => (setToken(d.token), onDone()))
            if (r.status === 404) return start() // abgelaufen → neuer Code
            timer = window.setTimeout(poll, 2000)
          }, () => (timer = window.setTimeout(poll, 5000)))
        poll()
      }, () => {})
    start()
    return () => {
      stop = true
      clearTimeout(timer)
    }
  }, [])

  const login = (u: User, password?: string) =>
    api<{ token: string }>('/api/login', { user: u.id, password, device: profile.name }).then(
      (r) => {
        if (isTV && r.token) setToken(r.token)
        onDone()
      },
      (e) => setError(String(e.message || e).indexOf('429') === 0 ? 'Zu viele Versuche – bitte kurz warten.' : 'Das hat nicht geklappt.'),
    )
  const onPick = (u: User) => {
    setError('')
    if (u.hasPassword) {
      setPick(u)
      setPw('')
    } else login(u)
  }

  if (pick)
    return (
      <main class="page login">
        <form
          onSubmit={(e) => {
            e.preventDefault()
            login(pick, pw)
          }}
        >
          <h1>Hallo {pick.name}</h1>
          <input type="password" autoFocus placeholder="Passwort" value={pw} onInput={(e) => setPw((e.target as HTMLInputElement).value)} />
          <div class="buttons">
            <button class="btn primary" type="submit">Anmelden</button>
            <button class="btn" type="button" onClick={() => setPick(null)}>Zurück</button>
          </div>
          {error && <p class="error-text">{error}</p>}
        </form>
      </main>
    )

  return (
    <FocusContext.Provider value={focusKey}>
      <main ref={ref} class="page login">
        <h1>Wer schaut?</h1>
        {!users && !error && <p class="empty">Lade …</p>}
        <div class="users">{users && users.map((u) => <UserTile key={u.id} u={u} onPick={onPick} />)}</div>
        {error && <p class="error-text">{error}</p>}
        {pair && (
          <div class="pair">
            <p>Oder am Handy anmelden und unter „Fernseher koppeln“ diesen Code eingeben:</p>
            <p class="code">{pair.code.replace(/(\d{3})(\d{3})/, '$1 $2')}</p>
          </div>
        )}
      </main>
    </FocusContext.Provider>
  )
}

// Handy-Seite: Code vom Fernseher bestätigen.
export function PairConfirm() {
  const [code, setCode] = useState('')
  const [msg, setMsg] = useState('')
  useBack(back)
  const confirm = () =>
    api<{ device: string }>(`/api/pair/${code.replace(/\D/g, '')}/confirm`, {}).then(
      (r) => setMsg(`„${r.device}“ ist jetzt angemeldet.`),
      () => setMsg('Code unbekannt oder abgelaufen.'),
    )
  return (
    <main class="page login">
      <form
        onSubmit={(e) => {
          e.preventDefault()
          confirm()
        }}
      >
        <h1>Fernseher koppeln</h1>
        <p class="muted">Den 6-stelligen Code eingeben, den der Fernseher zeigt.</p>
        <input inputMode="numeric" autoFocus maxLength={7} placeholder="123 456" value={code} onInput={(e) => setCode((e.target as HTMLInputElement).value)} />
        <div class="buttons">
          <button class="btn primary" type="submit">Koppeln</button>
          <FocusButton focusKey="pair-back" onPress={() => go('/')}>Fertig</FocusButton>
        </div>
        {msg && <p class="muted">{msg}</p>}
      </form>
    </main>
  )
}
