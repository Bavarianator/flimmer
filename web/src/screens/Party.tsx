// Gemeinsam schauen (Route /party/{raum}[/{startsekunde}]).
// Vertrag internal/party: GET /api/party/{id} → {state, members, eventsUrl}; SSE mit hello, state, members,
// chat, reaction; POST /api/party/{id}/actions {member, type, pos, rate, mediaId, buffering, text}.
// Uhrensync: Median aus 5 Messungen gegen /api/time; Drift < 0,3 s nichts, bis 1 s playbackRate ±5 %, sonst Sprung.
import { useEffect, useRef, useState } from 'preact/hooks'
import { api } from '../lib/api'
import { istTV } from '../lib/device'
import { ergaenze, t } from '../lib/i18n'
import { back, teile } from '../lib/router'
import { Seite } from '../components/Seite'
import { Fehler, Leer } from '../components/Zustand'
import { MiniAvatar } from '../components/Avatar'
import { Player } from '../player/Player'
import { messeVersatz, type Gemeinsam } from '../player/gemeinsam'
import type { PartyState } from '../player/sync'

ergaenze(
  {
    'party.titel': 'Gemeinsam schauen',
    'party.weg': 'Diesen Raum gibt es nicht mehr',
    'party.weg.text': 'Räume verschwinden 30 Minuten, nachdem alle gegangen sind. Starte im Player einen neuen.',
    'party.einladen': 'Zum Mitschauen diesen Link teilen:',
    'party.chat': 'Nachricht schreiben',
    'party.senden': 'Senden',
    'party.leer': 'Noch keine Nachrichten. Reaktionen gibt es mit ↑ im Menü.',
    'party.raum': 'RAUM {code}',
    'party.laedt': 'lädt …',
    'party.synchron': 'synchron',
    'party.aktion.play': '{wer} spielt ab',
    'party.aktion.pause': '{wer} pausiert',
    'party.aktion.seek': '{wer} springt',
    'party.aktion.media': '{wer} wechselt den Titel',
  },
  {
    'party.titel': 'Watch together',
    'party.weg': 'This room no longer exists',
    'party.weg.text': 'Rooms disappear 30 minutes after everyone has left. Start a new one in the player.',
    'party.einladen': 'Share this link to join:',
    'party.chat': 'Write a message',
    'party.senden': 'Send',
    'party.leer': 'No messages yet. Reactions are in the menu (↑).',
    'party.raum': 'ROOM {code}',
    'party.laedt': 'loading …',
    'party.synchron': 'in sync',
    'party.aktion.play': '{wer} plays',
    'party.aktion.pause': '{wer} paused',
    'party.aktion.seek': '{wer} skipped',
    'party.aktion.media': '{wer} changed the title',
  },
)

// Emoji nur hier – die einzige Ausnahme im Designsystem.
const REAKTIONEN = ['😂', '😱', '❤️', '👏', '🍿']

interface Nachricht {
  from: string
  member?: string
  text: string
  ts: number
  info?: boolean // Systemzeile („Chris pausiert“)
}

export function Party({ id }: { id: string }) {
  const [zustand, setZustand] = useState<PartyState | null>(null)
  const [versatz, setVersatz] = useState(0)
  const [leute, setLeute] = useState<string[]>([])
  const [chat, setChat] = useState<Nachricht[]>([])
  const [fliegend, setFliegend] = useState<{ n: number; text: string }[]>([])
  const [fehler, setFehler] = useState('')
  const [weg, setWeg] = useState(false)
  const [versuch, setVersuch] = useState(0)
  const mitglied = useRef('')
  const gestartet = useRef(false)

  const aktion = (typ: string, daten?: Record<string, unknown>) => {
    if (!mitglied.current) return
    api<void>(`/api/party/${id}/actions`, { member: mitglied.current, type: typ, ...(daten || {}) }).catch(() => {})
  }

  useEffect(() => {
    let es: EventSource | undefined
    let aus = false
    setFehler('')
    const neu = (n: Nachricht) => setChat((c) => c.concat(n).slice(-60))
    Promise.all([api<{ state: PartyState; members: string[]; eventsUrl: string }>(`/api/party/${id}`), messeVersatz()]).then(
      ([r, off]) => {
        if (aus) return
        setVersatz(off)
        setZustand(r.state)
        setLeute(r.members || [])
        es = new EventSource(r.eventsUrl)
        const an = (name: string, fn: (d: any) => void) => es!.addEventListener(name, (e) => fn(JSON.parse((e as MessageEvent).data)))
        an('hello', (d) => {
          mitglied.current = d.member
          setZustand(d.state)
          // Wer den Raum aus dem Player heraus anlegt, bringt seine Stelle mit.
          const ab = Number(teile()[2])
          if (!gestartet.current && ab > 0 && d.state.pos === 0) {
            gestartet.current = true
            aktion('seek', { pos: ab })
            aktion('play')
          }
        })
        an('state', (d) => {
          setZustand(d.state)
          const k = 'party.aktion.' + d.action
          if (d.by && t(k) !== k) neu({ from: '', text: t(k, { wer: d.by }), ts: Date.now(), info: true })
        })
        an('members', (d) => setLeute(d.members || []))
        an('chat', (d) => neu(d))
        an('reaction', (d) => {
          const n = Date.now() + Math.random()
          setFliegend((f) => f.concat({ n, text: d.text }).slice(-12))
          setTimeout(() => setFliegend((f) => f.filter((x) => x.n !== n)), 3000)
        })
        // EventSource verbindet sich selbst neu (retry: 2000); CLOSED heißt: Raum weg oder abgemeldet.
        es.onerror = () => es && es.readyState === 2 && setWeg(true)
      },
      (e) => {
        if (aus) return
        const m = String((e && e.message) || e)
        if (m.indexOf('404') === 0) setWeg(true)
        else setFehler(m)
      },
    )
    return () => {
      aus = true
      if (es) es.close()
    }
  }, [id, versuch])

  if (weg)
    return (
      <Seite>
        <Leer titel={t('party.weg')} text={t('party.weg.text')} aktion={{ label: t('knopf.zurueck'), onPress: back }} />
      </Seite>
    )
  if (fehler)
    return (
      <Seite>
        <Fehler fehler={fehler} nochmal={() => setVersuch(versuch + 1)} />
      </Seite>
    )
  if (!zustand) return <div class="pl" />

  const g: Gemeinsam = { zustand, versatz, aktion, reaktionen: REAKTIONEN, leute: leute.length }
  return (
    <Player
      key={zustand.mediaId}
      id={zustand.mediaId}
      start={0}
      gemeinsam={g}
      seitenleiste={
        <ChatLeiste id={id} leute={leute} chat={chat} wartet={zustand.waiting} fliegend={fliegend} senden={(text) => aktion('chat', { text })} reagieren={(r) => aktion('reaction', { text: r })} />
      }
    />
  )
}

// Farbton je Name, damit dieselbe Person überall dieselbe Avatar-Farbe hat.
function farbton(name: string) {
  let h = 0
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) % 360
  return h
}

function uhrzeit(ms: number) {
  const d = new Date(ms)
  return d.getHours() + ':' + (d.getMinutes() < 10 ? '0' : '') + d.getMinutes()
}

function ChatLeiste(p: {
  id: string
  leute: string[]
  chat: Nachricht[]
  wartet: string[]
  fliegend: { n: number; text: string }[]
  senden: (t: string) => void
  reagieren: (r: string) => void
}) {
  const [text, setText] = useState('')
  const liste = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const l = liste.current
    if (l) l.scrollTop = l.scrollHeight
  }, [p.chat.length])
  const link = location.origin + location.pathname + '#/party/' + p.id
  const wartet = p.wartet || []
  return (
    <aside class="fl-panel fl-gemeinsam party" aria-label={t('party.titel')}>
      <div class="kopf">
        <b>{t('party.titel')}</b>
        <span class="fl-zahl">{t('party.raum', { code: p.id.slice(0, 4).toUpperCase() })}</span>
      </div>
      <div class="leute">
        {p.leute.map((n) => (
          <div class="person" key={n}>
            <MiniAvatar n={{ name: n, color: farbton(n) }} class="mini" />
            <span class="eine-zeile">{n}</span>
            <small class={wartet.indexOf(n) >= 0 ? 'warte' : ''}>{t(wartet.indexOf(n) >= 0 ? 'party.laedt' : 'party.synchron')}</small>
          </div>
        ))}
      </div>
      <p class="party-link">
        {t('party.einladen')} <span class="fl-zahl">{link}</span>
      </p>
      <div class="chat" ref={liste} role="log" aria-live="polite">
        {!p.chat.length && <p class="nachricht party-info">{t('party.leer')}</p>}
        {p.chat.map((n, i) =>
          n.info ? (
            <p key={i} class="nachricht party-info">{n.text}</p>
          ) : (
            <p key={i} class="nachricht">
              <b>{n.from}</b>
              {n.text}
              <time>{uhrzeit(n.ts)}</time>
            </p>
          ),
        )}
      </div>
      <div class="reaktionen">
        {REAKTIONEN.map((r) => (
          <button key={r} type="button" onClick={() => p.reagieren(r)} aria-label={r}>
            {r}
          </button>
        ))}
      </div>
      {!istTV() && (
        <form
          class="zeile party-form"
          onSubmit={(e) => {
            e.preventDefault()
            if (text.trim()) p.senden(text.trim())
            setText('')
          }}
        >
          <input class="eingabe wachse" maxLength={500} placeholder={t('party.chat')} aria-label={t('party.chat')} value={text} onInput={(e) => setText((e.target as HTMLInputElement).value)} />
          <button type="submit" class="fl-btn pl-glas">
            {t('party.senden')}
          </button>
        </form>
      )}
      <div class="party-flug" aria-hidden="true">
        {p.fliegend.map((f) => (
          <span key={f.n} class="party-emoji" style={{ left: 10 + ((f.n * 37) % 60) + '%' }}>
            {f.text}
          </span>
        ))}
      </div>
    </aside>
  )
}
