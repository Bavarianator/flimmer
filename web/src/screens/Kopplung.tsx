// Kopplung (screens/16), Route /koppeln[/{code}].
// Handy/Desktop: Code vom Fernseher eingeben, nach der sechsten Ziffer wird ohne Knopf gekoppelt.
// TV: Code groß in Plex Mono mit Ablaufbalken und QR-Code; Zurück = Profil auswählen.
import { useEffect, useState } from 'preact/hooks'
import { api } from '../lib/api'
import { istTV } from '../lib/device'
import { usePausierteNavigation } from '../lib/focus'
import { ergaenze, t } from '../lib/i18n'
import { back, go, teile } from '../lib/router'
import { Seite } from '../components/Seite'
import { Icon } from '../components/Icon'
import { Wortmarke } from '../components/Wortmarke'
import { KoppelCode, useKoppelCode } from './KopplungTV'
import '../player/screens.css'

ergaenze(
  {
    'koppeln.titel': 'Fernseher koppeln',
    'koppeln.text': 'Gib den 6-stelligen Code ein, den der Fernseher zeigt. Er meldet sich dann als dein Profil an.',
    'koppeln.tipp': 'Tipp: Kamera auf den QR-Code am Fernseher halten – dann geht es ohne Tippen.',
    'koppeln.knopf': 'Fernseher koppeln',
    'koppeln.ok': '„{geraet}“ ist jetzt angemeldet.',
    'koppeln.falsch': 'Diesen Code kennt der Server nicht, oder er ist abgelaufen. Der Fernseher zeigt nach Ablauf einen neuen.',
    'koppeln.code': 'Code vom Fernseher',
    'koppeln.tv.titel': 'Fernseher mit dem Handy anmelden',
    'koppeln.tv.text': 'Niemand muss hier ein Passwort tippen.',
    'koppeln.tv.1': 'Flimmer am Handy öffnen – oder den QR-Code scannen',
    'koppeln.tv.2': 'Profil › Fernseher koppeln',
    'koppeln.tv.3': 'Diesen Code eingeben',
    'koppeln.tv.neu': 'Neuer Code in {n} Min.',
    'koppeln.tv.zurueck': 'Zurück = Profil auswählen',
  },
  {
    'koppeln.titel': 'Pair TV',
    'koppeln.text': 'Enter the 6-digit code shown on the TV. It will then sign in as your profile.',
    'koppeln.tipp': 'Tip: point your camera at the QR code on the TV – no typing needed.',
    'koppeln.knopf': 'Pair TV',
    'koppeln.ok': '“{geraet}” is now signed in.',
    'koppeln.falsch': 'The server does not know this code, or it has expired. The TV shows a new one when it expires.',
    'koppeln.code': 'Code from the TV',
    'koppeln.tv.titel': 'Sign in the TV with your phone',
    'koppeln.tv.text': 'Nobody has to type a password here.',
    'koppeln.tv.1': 'Open Flimmer on your phone – or scan the QR code',
    'koppeln.tv.2': 'Profile › Pair TV',
    'koppeln.tv.3': 'Enter this code',
    'koppeln.tv.neu': 'New code in {n} min',
    'koppeln.tv.zurueck': 'Back = choose profile',
  },
)

export function Kopplung() {
  return istTV() ? <KopplungTV onDone={() => go('/', { ersetzen: true })} /> : <KopplungHandy />
}

function KopplungHandy() {
  const vorgabe = (teile()[1] || '').replace(/\D/g, '').slice(0, 6)
  const [code, setCode] = useState(vorgabe)
  const [ok, setOk] = useState('')
  const [fehler, setFehler] = useState('')
  const [laeuft, setLaeuft] = useState(false)
  usePausierteNavigation()
  const koppeln = (c: string) => {
    if (c.length !== 6 || laeuft) return
    setLaeuft(true)
    setFehler('')
    setOk('')
    api<{ device: string }>(`/api/pair/${c}/confirm`, {}).then(
      (r) => {
        setLaeuft(false)
        setOk(t('koppeln.ok', { geraet: r.device }))
      },
      () => {
        setLaeuft(false)
        setFehler(t('koppeln.falsch'))
      },
    )
  }
  useEffect(() => koppeln(vorgabe), []) // Code aus dem QR-Link sofort bestätigen
  return (
    <Seite>
      <div class="koppeln-seite rand">
        <div class="koppeln-karte">
          <h1 class="t-titel">{t('koppeln.titel')}</h1>
          <p class="t-text leise koppeln-absatz">{t('koppeln.text')}</p>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              koppeln(code)
            }}
          >
            <input
              class="feld koppeln-eingabe"
              inputMode="numeric"
              autoComplete="one-time-code"
              autoFocus
              maxLength={7}
              placeholder="000 000"
              aria-label={t('koppeln.code')}
              value={code.length > 3 ? code.slice(0, 3) + ' ' + code.slice(3) : code}
              onInput={(e) => {
                const c = (e.target as HTMLInputElement).value.replace(/\D/g, '').slice(0, 6)
                setCode(c)
                if (c.length === 6) koppeln(c) // nach der sechsten Ziffer ohne Knopf
              }}
            />
            <p class="t-klein leiser koppeln-absatz">{t('koppeln.tipp')}</p>
            <button class="fl-btn primaer voll" type="submit" disabled={code.length !== 6 || laeuft}>
              {t('koppeln.knopf')}
            </button>
          </form>
          {ok && (
            <p class="toast-inline koppeln-ok zeile" role="status">
              <Icon name="haken" />
              <span>{ok}</span>
            </p>
          )}
          {fehler && (
            <p class="t-text fehlertext koppeln-absatz" role="alert">
              {fehler}
            </p>
          )}
        </div>
      </div>
    </Seite>
  )
}

// TV-Ansicht, auch aus der Profilauswahl erreichbar. onDone: angemeldet.
export function KopplungTV({ onDone }: { onDone: () => void }) {
  const z = useKoppelCode(true, onDone)
  const [jetzt, setJetzt] = useState(Date.now())
  useEffect(() => {
    const x = setInterval(() => setJetzt(Date.now()), 10000)
    return () => clearInterval(x)
  }, [])
  const rest = z ? Math.max(0, z.bis - jetzt) : 0
  const anteil = Math.min(1, rest / 600000)
  return (
    <main class="seite koppeln-tv">
      <div class="rand login-kopf">
        <Wortmarke />
      </div>
      <div class="koppeln-tv-inhalt rand">
        <h1 class="t-titel">{t('koppeln.tv.titel')}</h1>
        <p class="t-text leise koppeln-tv-unter">{t('koppeln.tv.text')}</p>
        <div class="zeile koppeln-tv-reihe">
          <ol class="koppeln-schritte">
            {[1, 2, 3].map((n) => (
              <li key={n} class="zeile t-text">
                <span class="koppeln-nr t-zahl">{n}</span>
                <span>{t('koppeln.tv.' + n)}</span>
              </li>
            ))}
          </ol>
          <div class="koppeln-tv-code">
            {z ? <KoppelCode code={z.code} gross /> : <div class="fl-code">— — —</div>}
            <div class="koppeln-ablauf">
              <i style={{ transform: 'scaleX(' + anteil + ')', webkitTransform: 'scaleX(' + anteil + ')' }} />
            </div>
            <p class="t-klein leise">{t('koppeln.tv.neu', { n: Math.max(1, Math.ceil(rest / 60000)) })}</p>
          </div>
          <img class="kopplung-qr" src="/api/qr" alt="" width={240} height={240} />
        </div>
      </div>
      <p class="koppeln-tv-fuss t-klein leise">{t('koppeln.tv.zurueck')}</p>
      <button class="nur-sr" type="button" onClick={back}>
        {t('knopf.zurueck')}
      </button>
    </main>
  )
}
