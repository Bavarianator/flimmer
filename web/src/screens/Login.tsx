// Profilauswahl (screens/01), PIN (02), TV-Kopplung als Leiste unten (01) und Gast-Einladung.
// Vertrag: export function Login(p: { onDone: () => void }) – erscheint bei 401.
// Einladung: Links haben die Form /einladung#<token>; main.tsx rendert dafür <Einladung/> (siehe NOTIZEN.md).
import { useEffect, useState } from 'preact/hooks'
import { api, nutzer, type Nutzer } from '../lib/api'
import { istTV } from '../lib/device'
import { fokusBald, Gruppe, useFokus, usePausierteNavigation } from '../lib/focus'
import { ergaenze, t } from '../lib/i18n'
import { useZurueck } from '../lib/router'
import { Button } from '../components/Button'
import { avatarFarbe } from '../components/Avatar'
import { Icon } from '../components/Icon'
import { Wortmarke } from '../components/Wortmarke'
import { Fehler } from '../components/Zustand'
import { profile, setToken } from '../profile'
import { Tastatur } from './SucheTastatur'
import { KoppelCode, useKoppelCode } from './KopplungTV'
import '../player/screens.css'

ergaenze(
  {
    'login.wer': 'Wer schaut?',
    'login.pin': 'PIN für {name}',
    'login.passwort': 'Passwort für {name}',
    'login.anmelden': 'Anmelden',
    'login.falsch': 'Das stimmt nicht. Bitte noch einmal.',
    'login.zuviel': 'Zu viele Versuche – bitte eine Minute warten.',
    'login.pin.hinweis': 'Oder die Zifferntasten der Fernbedienung · Zurück = anderes Profil',
    'login.buchstaben': 'Buchstaben',
    'login.ziffern': 'Ziffern',
    'login.mitpin': '{name}, mit PIN',
    'login.kind': 'Kind',
    'login.kinderprofil': '{name}, Kinderprofil',
    'login.handy.titel': 'Mit dem Handy anmelden',
    'login.handy.text': 'Flimmer am Handy öffnen, Profil › Fernseher koppeln, Code eingeben – oder den QR-Code scannen.',
    'einladung.titel': 'Du bist eingeladen',
    'einladung.text': 'Wie sollen dich die anderen sehen? Der Name erscheint beim gemeinsamen Schauen.',
    'einladung.name': 'Dein Name',
    'einladung.los': 'Los geht’s',
    'einladung.ungueltig': 'Diese Einladung gilt nicht mehr. Frag nach einem neuen Link.',
  },
  {
    'login.wer': 'Who’s watching?',
    'login.pin': 'PIN for {name}',
    'login.passwort': 'Password for {name}',
    'login.anmelden': 'Sign in',
    'login.falsch': 'That is not right. Please try again.',
    'login.zuviel': 'Too many attempts – please wait a minute.',
    'login.pin.hinweis': 'Or use the number keys on the remote · Back = other profile',
    'login.buchstaben': 'Letters',
    'login.ziffern': 'Digits',
    'login.mitpin': '{name}, with PIN',
    'login.kind': 'Kid',
    'login.kinderprofil': '{name}, kids profile',
    'login.handy.titel': 'Sign in with your phone',
    'login.handy.text': 'Open Flimmer on your phone, Profile › Pair TV, enter the code – or scan the QR code.',
    'einladung.titel': 'You are invited',
    'einladung.text': 'How should the others see you? The name appears when watching together.',
    'einladung.name': 'Your name',
    'einladung.los': 'Let’s go',
    'einladung.ungueltig': 'This invitation is no longer valid. Ask for a new link.',
  },
)

// Kinderprofil: kid kommt noch nicht vom Server (/api/users), siehe NOTIZEN.md.
type Profil = Nutzer & { kid?: boolean }

function Kachel({ u, onPick }: { u: Profil; onPick: (u: Profil) => void }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: 'profil-' + u.id, onPress: () => onPick(u) })
  const label = u.kid ? t('login.kinderprofil', { name: u.name }) : u.hasPassword ? t('login.mitpin', { name: u.name }) : u.name
  return (
    <button ref={f.ref} {...f.dom} type="button" aria-label={label} class={'fl-avatar' + (f.fokus ? ' ist-fokus' : '')}>
      <span class="kachel" style={{ background: avatarFarbe(u.color) }}>
        {u.name.charAt(0)}
        {u.hasPassword && (
          <span class="zusatz">
            <Schloss />
          </span>
        )}
        {u.kid && <span class="kind">{t('login.kind')}</span>}
      </span>
      <span class="name eine-zeile">{u.name}</span>
    </button>
  )
}

function Schloss() {
  return (
    <svg class="icon fl-icon" viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <path d="M6 11h12v9H6zM8.5 11V8a3.5 3.5 0 0 1 7 0v3" />
    </svg>
  )
}

// Profilauswahl. zurueck: nur beim Profilwechsel (/profile) – bei 401 gibt es kein Zurück.
export function Login(p: { onDone: () => void; zurueck?: () => void }) {
  const [liste, setListe] = useState<Profil[] | null>(null)
  const [fehler, setFehler] = useState('')
  const [versuch, setVersuch] = useState(0)
  const [wahl, setWahl] = useState<Profil | null>(null)
  const kopplung = useKoppelCode(istTV(), p.onDone)

  useEffect(() => {
    setFehler('')
    nutzer().then(setListe, (e) => setFehler(String((e && e.message) || e)))
  }, [versuch])
  useEffect(() => {
    if (liste && liste.length && !wahl) fokusBald('profil-' + liste[0].id)
  }, [liste, wahl])

  const anmelden = (u: Profil, passwort?: string) =>
    api<{ token: string }>('/api/login', { user: u.id, password: passwort, device: profile.name }).then((r) => {
      if (r.token) setToken(r.token) // TVs und App-Hüllen senden Bearer; Browser nutzen zusätzlich das Cookie
      p.onDone()
    })

  if (wahl) return <Pin u={wahl} anmelden={(pw) => anmelden(wahl, pw)} zurueck={() => setWahl(null)} />

  return (
    <main class="seite">
      <div class="rand login-kopf">
        <Wortmarke />
      </div>
      {fehler ? (
        <Fehler fehler={fehler} nochmal={() => setVersuch(versuch + 1)} />
      ) : (
        <div class="profile rand">
          <h1 class="t-titel">{t('login.wer')}</h1>
          <Gruppe fokusKey="profile" class="profile-liste">
            {(liste || []).map((u) => (
              <Kachel key={u.id} u={u} onPick={(x) => (x.hasPassword ? setWahl(x) : anmelden(x).catch(() => setWahl(x)))} />
            ))}
          </Gruppe>
        </div>
      )}
      {p.zurueck && <ZurueckTaste fn={p.zurueck} />}
      {kopplung && (
        <div class="login-koppeln zeile rand">
          <div class="login-koppeln-innen zeile wachse">
            <img class="kopplung-qr" src="/api/qr" alt="" width={200} height={200} />
            <div class="wachse login-koppeln-text">
              <p class="t-karte">{t('login.handy.titel')}</p>
              <p class="t-klein leise">{t('login.handy.text')}</p>
            </div>
            <KoppelCode code={kopplung.code} />
          </div>
        </div>
      )}
    </main>
  )
}

function ZurueckTaste({ fn }: { fn: () => void }) {
  useZurueck(() => {
    fn()
    return true
  })
  return null
}

// PIN bzw. Passwort. TV: Ziffernblock plus Zifferntasten der Fernbedienung, bei 4 Ziffern geht es los;
// wer ein Passwort mit Buchstaben hat, schaltet auf die Buchstaben-Tastatur. Sonst ein normales Feld.
function Pin({ u, anmelden, zurueck }: { u: Profil; anmelden: (pw: string) => Promise<void>; zurueck: () => void }) {
  const [wert, setWert] = useState('')
  const [fehler, setFehler] = useState('')
  const [art, setArt] = useState<'ziffern' | 'text'>('ziffern')
  const tv = istTV()
  useZurueck(() => {
    zurueck()
    return true
  })
  usePausierteNavigation(!tv)
  useEffect(() => {
    if (tv) fokusBald('pin-5')
  }, [art])
  const los = (pw: string) => {
    setFehler('')
    anmelden(pw).catch((e) => {
      setWert('')
      setFehler(String((e && e.message) || e).indexOf('429') === 0 ? t('login.zuviel') : t('login.falsch'))
    })
  }
  const setze = (w: string) => {
    setWert(w)
    if (art === 'ziffern' && w.length === 4 && /^\d{4}$/.test(w)) los(w)
  }

  return (
    <main class="seite">
      <div class="rand login-kopf">
        <Wortmarke />
      </div>
      <div class="pin rand">
        <h1 class="t-titel">{t(tv ? 'login.pin' : 'login.passwort', { name: u.name })}</h1>
        {tv ? (
          <>
            <div class="fl-pin pin-felder" aria-live="polite" aria-label={wert.length + ' / 4'}>
              {art === 'ziffern' ? (
                [0, 1, 2, 3].map((i) => <i key={i} class={(i < wert.length ? 'voll' : '') + (i === wert.length ? ' jetzt' : '')} />)
              ) : (
                <span class="pin-text">{wert.replace(/./g, '•') || ' '}</span>
              )}
            </div>
            {fehler && <p class="t-text fehlertext pin-fehler" role="alert">{fehler}</p>}
            <div class="pin-block">
              <Tastatur art={art === 'ziffern' ? 'ziffern' : 'text'} fokusKey="pin" wert={wert} setWert={setze} max={art === 'ziffern' ? 4 : 64} onFertig={() => los(wert)} />
            </div>
            <div class="zeile pin-aktionen">
              {art === 'text' && (
                <Button fokusKey="pin-ok" variante="primaer" onPress={() => los(wert)}>
                  {t('login.anmelden')}
                </Button>
              )}
              <Button fokusKey="pin-art" variante="geist" onPress={() => (setWert(''), setArt(art === 'ziffern' ? 'text' : 'ziffern'))}>
                {t(art === 'ziffern' ? 'login.buchstaben' : 'login.ziffern')}
              </Button>
            </div>
            <p class="t-klein leise">{t('login.pin.hinweis')}</p>
          </>
        ) : (
          <form
            class="pin-form"
            onSubmit={(e) => {
              e.preventDefault()
              los(wert)
            }}
          >
            <input class="feld pin-feld" type="password" autoFocus autoComplete="current-password" aria-label={t('login.passwort', { name: u.name })} value={wert} onInput={(e) => setWert((e.target as HTMLInputElement).value)} />
            {fehler && <p class="t-text fehlertext pin-fehler" role="alert">{fehler}</p>}
            <div class="zeile pin-aktionen">
              <button class="fl-btn primaer" type="submit">
                {t('login.anmelden')}
              </button>
              <button class="fl-btn" type="button" onClick={zurueck}>
                {t('knopf.zurueck')}
              </button>
            </div>
          </form>
        )}
      </div>
    </main>
  )
}

// Gast mit Einladungslink: Name wählen, POST /api/invites/redeem meldet an (Cookie).
export function Einladung() {
  const token = location.hash.replace(/^#/, '')
  const [name, setName] = useState('')
  const [fehler, setFehler] = useState('')
  usePausierteNavigation()
  const los = () =>
    api<{ id: string; name: string }>('/api/invites/redeem', { token, name: name.trim() }).then(
      () => location.replace('/#/'),
      (e) => {
        const m = String((e && e.message) || e)
        setFehler(m.indexOf('410') === 0 ? t('einladung.ungueltig') : m.indexOf('429') === 0 ? t('login.zuviel') : m)
      },
    )
  return (
    <main class="seite">
      <div class="rand login-kopf">
        <Wortmarke />
      </div>
      <div class="pin rand">
        <h1 class="t-titel">{t('einladung.titel')}</h1>
        <p class="t-text leise pin-text-absatz">{t('einladung.text')}</p>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (name.trim()) los()
          }}
        >
          <input class="feld" autoFocus maxLength={40} placeholder={t('einladung.name')} aria-label={t('einladung.name')} value={name} onInput={(e) => setName((e.target as HTMLInputElement).value)} />
          {fehler && <p class="t-text fehlertext pin-fehler" role="alert">{fehler}</p>}
          <div class="zeile pin-aktionen">
            <button class="fl-btn primaer" type="submit" disabled={!name.trim() || !token}>
              {t('einladung.los')}
            </button>
          </div>
        </form>
      </div>
    </main>
  )
}
