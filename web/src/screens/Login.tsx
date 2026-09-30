// Profilauswahl (screens/01), PIN (02), TV-Kopplung als Leiste unten (01) und Gast-Einladung.
// Vertrag: export function Login(p: { onDone: () => void }) – erscheint bei 401.
// Einladung: Links haben die Form /einladung#<token>; main.tsx rendert dafür <Einladung/> (siehe NOTIZEN.md).
import { useEffect, useState } from 'preact/hooks'
import { api, nutzer, type Nutzer } from '../lib/api'
import { istHandy, istTV } from '../lib/device'
import { fokusBald, Gruppe, useFokus, usePausierteNavigation } from '../lib/focus'
import { ergaenze, t } from '../lib/i18n'
import { useZurueck } from '../lib/router'
import { Button } from '../components/Button'
import { avatarFarbe } from '../components/Avatar'
import { Icon } from '../components/Icon'
import { Bildmarke } from '../components/Seite'
import { Wortmarke } from '../components/Wortmarke'
import { Fehler } from '../components/Zustand'
import { Seite } from '../components/Seite'
import { profile, setToken } from '../profile'
import { Tastatur } from './SucheTastatur'
import { useKoppelCode, type KoppelZustand } from './KopplungTV'
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
    'login.wer.text': 'Wähle dein Profil. Für Profile mit Schloss brauchst du eine PIN.',
    'login.admin': 'Admin',
    'login.geschuetzt': 'PIN geschützt',
    'login.kind.lang': 'Kinderprofil',
    'login.schnell': 'Schnellverbindung',
    'login.schnell.text': 'Gib diesen Code in einer angemeldeten Flimmer-App unter Avatar-Menü › Schnellverbindung ein.',
    'login.schnell.gilt': 'Code gilt noch {zeit} Min.',
    'login.schnell.knopf': 'Schnellverbindung verwenden',
    'login.scannen': 'Mit Handy scannen',
    'login.scannen.text': 'Mit der Handy-Kamera oder in der Flimmer-App scannen und bestätigen – dieses Gerät meldet sich dann an.',
    'login.tv.hinweis': 'QR-Code scannen oder am Handy: Avatar-Menü › Schnellverbindung.',
    'login.manuell': 'Mit Name und Passwort',
    'login.manuell.titel': 'Anmelden',
    'login.manuell.text': 'Für Konten, die nicht in der Profilauswahl stehen, zum Beispiel nach einer Einladung.',
    'login.name': 'Name',
    'login.pw': 'Passwort',
    'login.fuss': 'Privater Medienserver. Zugang nur nach Einladung.',
    'einladung.titel': 'Du bist eingeladen',
    'einladung.text': 'Wie sollen dich die anderen sehen? Der Name erscheint beim gemeinsamen Schauen.',
    'einladung.name': 'Dein Name',
    'einladung.passwort': 'Passwort (mindestens 4 Zeichen)',
    'einladung.passwort.text': 'Mit Name und Passwort meldest du dich später wieder an – im Browser, in der Flimmer-App und auch von unterwegs.',
    'einladung.vergeben': 'Diesen Namen gibt es schon. Bitte wähle einen anderen.',
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
    'login.wer.text': 'Choose your profile. Profiles with a lock need a PIN.',
    'login.admin': 'Admin',
    'login.geschuetzt': 'PIN protected',
    'login.kind.lang': 'Kids profile',
    'login.schnell': 'Quick connect',
    'login.schnell.text': 'Enter this code in a signed-in Flimmer app under avatar menu › Quick connect.',
    'login.schnell.gilt': 'Code valid for {zeit} min',
    'login.schnell.knopf': 'Use quick connect',
    'login.scannen': 'Scan with your phone',
    'login.scannen.text': 'Scan with your phone camera or the Flimmer app and confirm – this device then signs in.',
    'login.tv.hinweis': 'Scan the QR code or on your phone: avatar menu › Quick connect.',
    'login.manuell': 'With name and password',
    'login.manuell.titel': 'Sign in',
    'login.manuell.text': 'For accounts not shown in the profile list, for example after an invitation.',
    'login.name': 'Name',
    'login.pw': 'Password',
    'login.fuss': 'Private media server. Access by invitation only.',
    'einladung.titel': 'You are invited',
    'einladung.text': 'How should the others see you? The name appears when watching together.',
    'einladung.name': 'Your name',
    'einladung.passwort': 'Password (at least 4 characters)',
    'einladung.passwort.text': 'With your name and password you can sign in again later – in the browser, in the Flimmer app and on the go.',
    'einladung.vergeben': 'This name is already taken. Please choose another one.',
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
      <span class="unter eine-zeile">{u.kid ? t('login.kind.lang') : u.hasPassword ? t('login.geschuetzt') : u.admin ? t('login.admin') : ''}</span>
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
  const [schnell, setSchnell] = useState(false) // Handy: Schnellverbindung erst auf Knopfdruck
  const [manuell, setManuell] = useState(false)
  // Schnellverbindung beim Anmelden (401) und immer auf dem TV; beim Profilwechsel am Handy/Desktop nicht nötig.
  const kopplung = useKoppelCode(istTV() || !p.zurueck, p.onDone)

  useEffect(() => {
    setFehler('')
    nutzer().then(setListe, (e) => setFehler(String((e && e.message) || e)))
  }, [versuch])
  useEffect(() => {
    if (liste && liste.length && !wahl) fokusBald('profil-' + liste[0].id)
  }, [liste, wahl])

  // user: Profil-ID oder Name (der Server nimmt beides)
  const anmelden = (user: string, passwort?: string) =>
    api<{ token: string }>('/api/login', { user, password: passwort, device: profile.name }).then((r) => {
      if (r.token) setToken(r.token) // TVs und App-Hüllen senden Bearer; Browser nutzen zusätzlich das Cookie
      p.onDone()
    })

  if (wahl) return <Pin u={wahl} anmelden={(pw) => anmelden(wahl.id, pw)} zurueck={() => setWahl(null)} />
  if (manuell) return <Manuell anmelden={anmelden} zurueck={() => setManuell(false)} />

  const tv = istTV()
  const hd = istHandy()
  return (
    <Seite ohneKopf class="login">
      <LoginKopf />
      {fehler ? (
        <Fehler fehler={fehler} nochmal={() => setVersuch(versuch + 1)} />
      ) : (
        <div class="login-haupt rand zeile">
          <section class="login-wer wachse" aria-labelledby="login-wer">
            <h1 id="login-wer" class="t-titel">
              {t('login.wer')}
            </h1>
            {!tv && <p class="t-text leise login-wer-text">{t('login.wer.text')}</p>}
            <Gruppe fokusKey="profile" class="profile-liste">
              {(liste || []).map((u) => (
                <Kachel key={u.id} u={u} onPick={(x) => (x.hasPassword ? setWahl(x) : anmelden(x.id).catch(() => setWahl(x)))} />
              ))}
            </Gruppe>
            {!tv && (
              <Gruppe fokusKey="login-aktionen" class="zeile login-aktionen">
                {hd && kopplung && !schnell && (
                  <Button fokusKey="login-schnell" icon="schluessel" onPress={() => setSchnell(true)}>
                    {t('login.schnell.knopf')}
                  </Button>
                )}
                <Button fokusKey="login-manuell" variante="geist" icon="profil" onPress={() => setManuell(true)}>
                  {t('login.manuell')}
                </Button>
                {p.zurueck && (
                  <Button fokusKey="login-zurueck" variante="geist" icon="zurueck" onPress={p.zurueck}>
                    {t('knopf.zurueck')}
                  </Button>
                )}
              </Gruppe>
            )}
          </section>
          {kopplung && !tv && (!hd || schnell) && <Schnell z={kopplung} />}
        </div>
      )}
      {p.zurueck && <ZurueckTaste fn={p.zurueck} />}
      {tv && kopplung && (
        <footer class="login-tv-fuss rand zeile">
          <span class="wachse" />
          <div class="login-tv-code">
            <p class="zeile leise">
              <Icon name="fernseher" />
              {t('login.handy.titel')}
            </p>
            <p class="fl-zahl login-tv-ziffern">{kopplung.code.slice(0, 3) + ' ' + kopplung.code.slice(3)}</p>
            <p class="t-klein leise">{t('login.tv.hinweis')}</p>
          </div>
          <img class="kopplung-qr login-tv-qr" src={'/api/pair/' + kopplung.code + '/qr'} alt="" width={200} height={200} />
        </footer>
      )}
      {!tv && <footer class="login-fuss rand t-klein leise">{t('login.fuss')}</footer>}
    </Seite>
  )
}

// Kopf ohne Navigation: Bildmarke und Wortmarke (Profilauswahl, PIN, Einladung, Kopplung).
export function LoginKopf({ zurueck }: { zurueck?: () => void }) {
  return (
    <header class="login-kopf rand zeile">
      {zurueck && (
        <button type="button" class="kopf-knopf login-zurueck" aria-label={t('knopf.zurueck')} title={t('knopf.zurueck')} onClick={zurueck}>
          <Icon name="zurueck" />
        </button>
      )}
      <Bildmarke />
      <Wortmarke />
    </header>
  )
}

// Schnellverbindung (Entwurf „Anmeldung“, rechts): Code mit Ablauf, darunter QR für das Handy.
function Schnell({ z }: { z: KoppelZustand }) {
  const [jetzt, setJetzt] = useState(Date.now())
  useEffect(() => {
    const x = setInterval(() => setJetzt(Date.now()), 1000)
    return () => clearInterval(x)
  }, [])
  const rest = Math.max(0, z.bis - jetzt)
  const s = Math.floor(rest / 1000)
  const anteil = Math.min(1, rest / 600000)
  return (
    <aside class="login-schnell" aria-labelledby="login-schnell">
      <h2 id="login-schnell" class="zeile">
        <Icon name="schluessel" />
        {t('login.schnell')}
      </h2>
      <p class="leise">{t('login.schnell.text')}</p>
      <div class="login-code fl-zahl" aria-label={z.code.split('').join(' ')}>
        {z.code.slice(0, 3) + ' ' + z.code.slice(3)}
      </div>
      <p class="zeile leise login-gilt">
        <Icon name="uhr" />
        {t('login.schnell.gilt', { zeit: Math.floor(s / 60) + ':' + (s % 60 < 10 ? '0' : '') + (s % 60) })}
      </p>
      <div class="login-ablauf">
        <i style={{ transform: 'scaleX(' + anteil + ')', webkitTransform: 'scaleX(' + anteil + ')' }} />
      </div>
      <div class="zeile login-qr">
        <img class="kopplung-qr" src={'/api/pair/' + z.code + '/qr'} alt="" width={168} height={168} />
        <span>
          <b>{t('login.scannen')}</b>
          <span class="leise">{t('login.scannen.text')}</span>
        </span>
      </div>
    </aside>
  )
}

// Name und Passwort: für Gäste mit Passwort (stehen nicht in der Profilauswahl), gerade von außerhalb.
function Manuell({ anmelden, zurueck }: { anmelden: (user: string, pw: string) => Promise<void>; zurueck: () => void }) {
  const [name, setName] = useState('')
  const [pw, setPw] = useState('')
  const [fehler, setFehler] = useState('')
  usePausierteNavigation()
  useZurueck(() => {
    zurueck()
    return true
  })
  const los = () => {
    setFehler('')
    anmelden(name.trim(), pw).catch((e) => {
      setPw('')
      setFehler(String((e && e.message) || e).indexOf('429') === 0 ? t('login.zuviel') : t('login.falsch'))
    })
  }
  return (
    <Seite ohneKopf class="login">
      <LoginKopf zurueck={zurueck} />
      <div class="pin rand">
        <h1 class="t-titel">{t('login.manuell.titel')}</h1>
        <p class="t-text leise pin-text-absatz">{t('login.manuell.text')}</p>
        <form
          class="pin-form"
          onSubmit={(e) => {
            e.preventDefault()
            if (name.trim() && pw) los()
          }}
        >
          <input class="feld" autoFocus autoComplete="username" maxLength={40} placeholder={t('login.name')} aria-label={t('login.name')} value={name} onInput={(e) => setName((e.target as HTMLInputElement).value)} />
          <input class="feld" type="password" autoComplete="current-password" placeholder={t('login.pw')} aria-label={t('login.pw')} value={pw} onInput={(e) => setPw((e.target as HTMLInputElement).value)} />
          {fehler && <p class="t-text fehlertext pin-fehler" role="alert">{fehler}</p>}
          <div class="zeile pin-aktionen">
            <button class="fl-btn primaer" type="submit" disabled={!name.trim() || !pw}>
              {t('login.anmelden')}
            </button>
            <button class="fl-btn" type="button" onClick={zurueck}>
              {t('knopf.zurueck')}
            </button>
          </div>
        </form>
      </div>
    </Seite>
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
    <Seite ohneKopf class="login">
      <LoginKopf zurueck={zurueck} />
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
    </Seite>
  )
}

// Gast mit Einladungslink: Name wählen, POST /api/invites/redeem meldet an (Cookie).
export function Einladung() {
  const token = location.hash.replace(/^#/, '')
  const [name, setName] = useState('')
  const [pw, setPw] = useState('')
  const [fehler, setFehler] = useState('')
  usePausierteNavigation()
  const bereit = !!name.trim() && pw.length >= 4 && !!token
  const los = () =>
    api<{ token: string }>('/api/invites/redeem', { token, name: name.trim(), password: pw }).then(
      (r) => {
        if (r.token) setToken(r.token)
        location.replace('/#/')
      },
      (e) => {
        const m = String((e && e.message) || e)
        const code = m.slice(0, 3)
        setFehler(code === '410' ? t('einladung.ungueltig') : code === '409' ? t('einladung.vergeben') : code === '429' ? t('login.zuviel') : m)
      },
    )
  return (
    <Seite ohneKopf class="login">
      <LoginKopf />
      <div class="pin rand">
        <h1 class="t-titel">{t('einladung.titel')}</h1>
        <p class="t-text leise pin-text-absatz">{t('einladung.text')}</p>
        <form
          class="pin-form"
          onSubmit={(e) => {
            e.preventDefault()
            if (bereit) los()
          }}
        >
          <input class="feld" autoFocus autoComplete="username" maxLength={40} placeholder={t('einladung.name')} aria-label={t('einladung.name')} value={name} onInput={(e) => setName((e.target as HTMLInputElement).value)} />
          <input class="feld" type="password" autoComplete="new-password" placeholder={t('einladung.passwort')} aria-label={t('einladung.passwort')} value={pw} onInput={(e) => setPw((e.target as HTMLInputElement).value)} />
          <p class="t-klein leise pin-hinweis">{t('einladung.passwort.text')}</p>
          {fehler && <p class="t-text fehlertext pin-fehler" role="alert">{fehler}</p>}
          <div class="zeile pin-aktionen">
            <button class="fl-btn primaer" type="submit" disabled={!bereit}>
              {t('einladung.los')}
            </button>
          </div>
        </form>
      </div>
    </Seite>
  )
}
