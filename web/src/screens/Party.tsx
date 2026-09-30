// Gemeinsam schauen: /party ohne Raum = „Neue Gruppe erstellen / beitreten“, /party/{raum}[/{startsekunde}] = Raum.
// Vertrag internal/party: POST /api/party {mediaId} → {id}; GET /api/party/{id} → {state, members, eventsUrl}; SSE mit hello,
// state, members, chat, reaction; POST /api/party/{id}/actions {member, type, pos, rate, mediaId, buffering, text}.
// Uhrensync: Median aus 5 Messungen gegen /api/time; Drift < 0,3 s nichts, bis 1 s playbackRate ±5 %, sonst Sprung.
import { useEffect, useRef, useState } from 'preact/hooks'
import { anzeigeTitel, api, bibliothek, bild, offeneGruppen, startseite, type Item, type OffeneGruppe } from '../lib/api'
import { istTV } from '../lib/device'
import { fokusStart } from '../lib/focus'
import { ergaenze, folge, t } from '../lib/i18n'
import { back, go, pfad, teile } from '../lib/router'
import { Seite } from '../components/Seite'
import { Fehler, Leer } from '../components/Zustand'
import { MiniAvatar } from '../components/Avatar'
import { Button } from '../components/Button'
import { Icon } from '../components/Icon'
import { Karte } from '../components/Karte'
import { Reihe } from '../components/Reihe'
import { Player } from '../player/Player'
import { messeVersatz, type Gemeinsam } from '../player/gemeinsam'
import type { PartyState } from '../player/sync'
import { GruppenReihe } from './Start'
import { Tastatur } from './SucheTastatur'
import '../player/screens.css'

ergaenze(
  {
    'party.titel': 'Gemeinsam schauen',
    'party.weg': 'Diesen Raum gibt es nicht mehr',
    'party.weg.text': 'Räume verschwinden 30 Minuten, nachdem alle gegangen sind. Starte hier einen neuen.',
    'party.chat': 'Nachricht …',
    'party.chat.label': 'Nachricht an die Gruppe',
    'party.senden': 'Senden',
    'party.leer': 'Noch keine Nachrichten.',
    'party.laedt': 'puffert …',
    'party.synchron': 'bereit',
    'party.personen': '{n} PERSONEN',
    'party.person': '1 PERSON',
    'party.mitglieder': 'Mitglieder',
    'party.code': 'Gruppencode',
    'party.kopieren': 'Einladungslink kopieren',
    'party.kopiert': 'Link kopiert',
    'party.alle.synchron': 'Alle synchron',
    'party.wartet': 'Warten auf {wer} …',
    'party.anhalten': 'Wiedergabe für alle anhalten',
    'party.fortsetzen': 'Wiedergabe für alle fortsetzen',
    'party.verlassen': 'Gruppe verlassen',
    'party.schliessen': 'Bereich schließen',
    'party.aktion.play': '{wer} spielt ab',
    'party.aktion.pause': '{wer} pausiert',
    'party.aktion.seek': '{wer} springt',
    'party.aktion.media': '{wer} wechselt den Titel',
    'party.lobby.text': 'Alle sehen dasselbe – synchron auf jedem Gerät. Pausieren, Spulen und Reaktionen gelten für die ganze Gruppe.',
    'party.neu': 'Neue Gruppe erstellen',
    'party.neu.zusatz': 'Was wollt ihr schauen?',
    'party.neu.leer': 'Öffne einen Film oder eine Folge und wähle im Player „Gemeinsam schauen“.',
    'party.beitreten': 'Gruppe beitreten',
    'party.beitreten.text': 'Gib den Gruppencode oder den Einladungslink ein, den du bekommen hast.',
    'party.beitreten.feld': 'Gruppencode oder Link',
    'party.beitreten.knopf': 'Beitreten',
    'party.beitreten.falsch': 'Das ist kein gültiger Gruppencode. Er hat 12 Zeichen, z. B. 3F9A 0C21 B7E4.',
  },
  {
    'party.titel': 'Watch together',
    'party.weg': 'This room no longer exists',
    'party.weg.text': 'Rooms disappear 30 minutes after everyone has left. Start a new one here.',
    'party.chat': 'Message …',
    'party.chat.label': 'Message to the group',
    'party.senden': 'Send',
    'party.leer': 'No messages yet.',
    'party.laedt': 'buffering …',
    'party.synchron': 'ready',
    'party.personen': '{n} PEOPLE',
    'party.person': '1 PERSON',
    'party.mitglieder': 'Members',
    'party.code': 'Group code',
    'party.kopieren': 'Copy invite link',
    'party.kopiert': 'Link copied',
    'party.alle.synchron': 'Everyone in sync',
    'party.wartet': 'Waiting for {wer} …',
    'party.anhalten': 'Pause for everyone',
    'party.fortsetzen': 'Resume for everyone',
    'party.verlassen': 'Leave group',
    'party.schliessen': 'Close panel',
    'party.aktion.play': '{wer} plays',
    'party.aktion.pause': '{wer} paused',
    'party.aktion.seek': '{wer} skipped',
    'party.aktion.media': '{wer} changed the title',
    'party.lobby.text': 'Everyone sees the same thing – in sync on every device. Pause, seek and reactions apply to the whole group.',
    'party.neu': 'Create a new group',
    'party.neu.zusatz': 'What do you want to watch?',
    'party.neu.leer': 'Open a movie or an episode and choose “Watch together” in the player.',
    'party.beitreten': 'Join a group',
    'party.beitreten.text': 'Enter the group code or the invite link you received.',
    'party.beitreten.feld': 'Group code or link',
    'party.beitreten.knopf': 'Join',
    'party.beitreten.falsch': 'That is not a valid group code. It has 12 characters, e.g. 3F9A 0C21 B7E4.',
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

// Raum-IDs sind 12 Hex-Zeichen; angezeigt in Vierergruppen.
const codeAus = (s: string) => {
  const m = s.toLowerCase().replace(/[\s-]/g, '').match(/[0-9a-f]{12}/)
  return m ? m[0] : ''
}
const codeZeigen = (id: string) => id.toUpperCase().replace(/(.{4})(?=.)/g, '$1 ')

export function Party({ id }: { id?: string }) {
  return id ? <Raum id={id} /> : <Lobby />
}

// „Neue Gruppe erstellen / beitreten“ (Route /party).
function Lobby() {
  const [titel, setTitel] = useState<Item[] | null>(null)
  const [code, setCode] = useState('')
  const [fehler, setFehler] = useState('')
  const [offen, setOffen] = useState<OffeneGruppe[]>([])
  const tv = istTV()
  useEffect(() => {
    // Offene Gruppen der anderen; alle 15 s neu, damit neue ohne Neuladen erscheinen. Bibliothek für Titel und Bild.
    const holen = () => Promise.all([offeneGruppen(), bibliothek()]).then(([g]) => setOffen(g), () => {})
    holen()
    const x = setInterval(holen, 15000)
    return () => clearInterval(x)
  }, [])
  useEffect(() => {
    // Kandidaten: Weiterschauen, Nächste Folgen, zuletzt hinzugefügt – ohne Doppelte.
    startseite().then(
      (rows) => {
        const ids: Record<string, boolean> = {}
        const alle: Item[] = []
        for (const r of rows) for (const it of r.items) if (!ids[it.id] && alle.length < 16) (ids[it.id] = true), alle.push(it)
        setTitel(alle)
      },
      () => setTitel([]),
    )
  }, [])
  useEffect(() => {
    if (titel) fokusStart(titel.length ? 'party-neu-' + titel[0].id : 'party-code')
  }, [!!titel])
  const erstellen = (it: Item) =>
    api<{ id: string }>('/api/party', { mediaId: it.id }).then(
      (r) => go(pfad('party', r.id, Math.floor(it.progress || 0))),
      (e) => setFehler(String((e && e.message) || e)),
    )
  const beitreten = () => {
    const c = codeAus(code)
    if (!c) return setFehler(t('party.beitreten.falsch'))
    setFehler('')
    go(pfad('party', c))
  }

  return (
    <Seite titel={t('party.titel')} class="party-lobby">
      <p class="rand t-text leise party-lobby-text">{t('party.lobby.text')}</p>
      <GruppenReihe gruppen={offen} />
      {titel && titel.length > 0 ? (
        <Reihe titel={t('party.neu')} zusatz={t('party.neu.zusatz')} fokusKey="party-neu">
          {titel.map((it) => (
            <Karte
              key={it.id}
              fokusKey={'party-neu-' + it.id}
              breit
              titel={it.series || anzeigeTitel(it)}
              unter={it.series ? folge(it.season, it.episode) + ' · ' + it.title : it.year ? String(it.year) : undefined}
              bild={bild(it, 'backdrop', 480)}
              farbe={it.color}
              ampel={it.light}
              fortschritt={it.duration ? (it.progress || 0) / it.duration : 0}
              onPress={() => erstellen(it)}
            />
          ))}
        </Reihe>
      ) : (
        titel && (
          <section class="rand party-block">
            <h2 class="t-reihe">{t('party.neu')}</h2>
            <p class="t-klein leise">{t('party.neu.leer')}</p>
          </section>
        )
      )}
      <section class="rand party-block">
        <h2 class="t-reihe">{t('party.beitreten')}</h2>
        <p class="t-klein leise party-block-text">{t('party.beitreten.text')}</p>
        {tv ? (
          <>
            <div class="feld party-code-feld fl-zahl" aria-live="polite">
              {codeZeigen(code) || <span class="leise">{t('party.beitreten.feld')}</span>}
            </div>
            <div class="party-tastatur">
              <Tastatur fokusKey="party-code" wert={code} setWert={(w) => setCode(w.replace(/[^0-9a-f]/gi, '').slice(0, 12))} max={12} onFertig={beitreten} direkt />
            </div>
            <Button fokusKey="party-beitreten" variante="primaer" onPress={beitreten}>
              {t('party.beitreten.knopf')}
            </Button>
          </>
        ) : (
          <form
            class="zeile party-code-form"
            onSubmit={(e) => {
              e.preventDefault()
              beitreten()
            }}
          >
            <input
              class="feld wachse"
              autoComplete="off"
              spellcheck={false}
              placeholder={t('party.beitreten.feld')}
              aria-label={t('party.beitreten.feld')}
              value={code}
              onInput={(e) => setCode((e.target as HTMLInputElement).value)}
            />
            <button class="fl-btn primaer" type="submit" disabled={!code.trim()}>
              {t('party.beitreten.knopf')}
            </button>
          </form>
        )}
        {fehler && (
          <p class="t-text fehlertext party-block-text" role="alert">
            {fehler}
          </p>
        )}
      </section>
    </Seite>
  )
}

function Raum({ id }: { id: string }) {
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
          // Wer den Raum anlegt, bringt seine Stelle mit.
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
      <Seite titel={t('party.titel')}>
        <Leer titel={t('party.weg')} text={t('party.weg.text')} icon="gemeinsam" aktion={{ label: t('party.neu'), onPress: () => go('/party', { ersetzen: true }) }} />
      </Seite>
    )
  if (fehler)
    return (
      <Seite titel={t('party.titel')}>
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
      seitenleiste={(zu) => (
        <ChatLeiste
          id={id}
          leute={leute}
          chat={chat}
          wartet={zustand.waiting}
          pausiert={zustand.paused}
          fliegend={fliegend}
          zu={zu}
          aktion={aktion}
        />
      )}
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

// execCommand statt navigator.clipboard: das gibt es nur in sicheren Kontexten, Flimmer läuft im LAN meist über http.
function kopieren(text: string): boolean {
  const f = document.createElement('textarea')
  f.value = text
  f.setAttribute('readonly', '')
  f.style.position = 'fixed'
  f.style.opacity = '0'
  document.body.appendChild(f)
  f.select()
  let ok = false
  try {
    ok = document.execCommand('copy')
  } catch {}
  document.body.removeChild(f)
  return ok
}

// Leiste rechts (Entwurf „Gemeinsam“): Gruppe mit Code und Link, Mitglieder, Gleichlauf, Chat, Reaktionen, Verlassen.
function ChatLeiste(p: {
  id: string
  leute: string[]
  chat: Nachricht[]
  wartet: string[]
  pausiert: boolean
  fliegend: { n: number; text: string }[]
  zu: () => void
  aktion: (typ: string, daten?: Record<string, unknown>) => void
}) {
  const [text, setText] = useState('')
  const [kopiert, setKopiert] = useState(false)
  const liste = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const l = liste.current
    if (l) l.scrollTop = l.scrollHeight
  }, [p.chat.length])
  const link = location.origin + location.pathname + '#/party/' + p.id
  const wartet = p.wartet || []
  const tv = istTV()
  return (
    <aside class="party" aria-label={t('party.titel')}>
      <div class="party-kopf zeile">
        <h2 class="wachse">{t('party.titel')}</h2>
        {!tv && (
          <button type="button" class="pl-knopf" aria-label={t('party.schliessen')} title={t('party.schliessen')} onClick={p.zu}>
            <Icon name="schliessen" />
          </button>
        )}
      </div>

      <div class="party-abschnitt">
        <div class="zeile">
          <span class="wachse party-gruppe">{t('party.code')}</span>
          <span class="fl-label">{p.leute.length === 1 ? t('party.person') : t('party.personen', { n: p.leute.length })}</span>
        </div>
        <div class="zeile party-code-zeile">
          <span class="party-code fl-zahl" title={t('party.code')}>
            {codeZeigen(p.id)}
          </span>
          {!tv && (
            <button type="button" class="party-knopf wachse" onClick={() => kopieren(link) && (setKopiert(true), setTimeout(() => setKopiert(false), 2000))}>
              <Icon name={kopiert ? 'haken' : 'link'} />
              {t(kopiert ? 'party.kopiert' : 'party.kopieren')}
            </button>
          )}
        </div>
        {tv && <p class="party-link fl-zahl">{link}</p>}
      </div>

      <div class="party-abschnitt">
        <div class="fl-label party-klein-kopf">{t('party.mitglieder')}</div>
        {p.leute.map((n) => {
          const w = wartet.indexOf(n) >= 0
          return (
            <div class="person zeile" key={n}>
              <MiniAvatar n={{ name: n, color: farbton(n) }} class="mini" />
              <span class="wachse eine-zeile">
                {n} <small>{t(w ? 'party.laedt' : 'party.synchron')}</small>
              </span>
              <span class={'fl-ampel ' + (w ? 'gelb' : 'gruen')} />
            </div>
          )
        })}
      </div>

      <div class="party-abschnitt">
        <p class="zeile party-sync">
          <span class={'fl-ampel ' + (wartet.length ? 'gelb' : 'gruen')} />
          {wartet.length ? t('party.wartet', { wer: wartet.join(', ') }) : t('party.alle.synchron')}
        </p>
        {!tv && (
          <button type="button" class="party-knopf voll" onClick={() => p.aktion(p.pausiert ? 'play' : 'pause')}>
            <Icon name={p.pausiert ? 'abspielen' : 'pause'} />
            {t(p.pausiert ? 'party.fortsetzen' : 'party.anhalten')}
          </button>
        )}
      </div>

      <div class="party-abschnitt chat" ref={liste} role="log" aria-live="polite">
        <div class="fl-label party-klein-kopf">Chat</div>
        {!p.chat.length && <p class="nachricht party-info">{t('party.leer')}</p>}
        {p.chat.map((n, i) =>
          n.info ? (
            <p key={i} class="nachricht party-info">
              {n.text}
            </p>
          ) : (
            <p key={i} class="nachricht zeile">
              <span class="wachse">
                <b style={{ color: 'hsl(' + farbton(n.from) + ',30%,70%)' }}>{n.from}</b> {n.text}
              </span>
              <time class="fl-zahl">{uhrzeit(n.ts)}</time>
            </p>
          ),
        )}
      </div>

      <div class="reaktionen zeile">
        {REAKTIONEN.map((r) => (
          <button key={r} type="button" class="wachse" onClick={() => p.aktion('reaction', { text: r })} aria-label={r}>
            {r}
          </button>
        ))}
      </div>
      {!tv && (
        <form
          class="zeile party-form"
          onSubmit={(e) => {
            e.preventDefault()
            if (text.trim()) p.aktion('chat', { text: text.trim() })
            setText('')
          }}
        >
          <input class="eingabe wachse" maxLength={500} placeholder={t('party.chat')} aria-label={t('party.chat.label')} value={text} onInput={(e) => setText((e.target as HTMLInputElement).value)} />
          <button type="submit" class="pl-knopf party-senden" aria-label={t('party.senden')} title={t('party.senden')}>
            <Icon name="weiter" />
          </button>
        </form>
      )}
      {!tv && (
        <button type="button" class="party-verlassen zeile" onClick={() => back()}>
          <Icon name="abmelden" />
          {t('party.verlassen')}
        </button>
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
