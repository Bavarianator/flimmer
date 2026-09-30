// Benutzer-Einstellungen unter /einstellungen[/:bereich]: Profil, Anzeige, Startseite, Wiedergabe, Untertitel,
// Schnellverbindung. Desktop und TV: Menü links, Inhalt rechts. Handy: /einstellungen zeigt das Menü, ein Bereich die Seite.
// Admin-Teile (Profile, Bibliothek, Server, Fernzugriff, Einladungen, Sicherung, Diagnose) liegen im Dashboard.
import { useEffect, useState } from 'preact/hooks'
import { abmelden, geraeteTest } from '../lib/api'
import { koppeln } from '../lib/api-admin'
import { geraetWahl, istTV, setGeraetWahl, setThema, thema, type Geraet, type Thema } from '../lib/device'
import { Gruppe, fokusBald, useFokus } from '../lib/focus'
import { ergaenze, setSprache, sprache, t } from '../lib/i18n'
import { go, useRoute } from '../lib/router'
import { Button } from '../components/Button'
import { MiniAvatar } from '../components/Avatar'
import { Icon, type IconName } from '../components/Icon'
import { Seite, useGeraet, useIch } from '../components/Seite'
import { loadProbe } from '../probe'
import { setVorlieben, vorlieben, type Vorlieben } from '../player/vorlieben'
import { Abschnitt, Feld, Hinweis, Toast, Wahl, Zeile, melde } from './admin/teile'
import { START_REIHEN, setStartVorlieben, startVorlieben } from './einstellungen/startseite'

ergaenze({
  'einst.titel': 'Einstellungen',
  'einst.profil': 'Profil',
  'einst.anzeige': 'Anzeige',
  'einst.startseite': 'Startseite',
  'einst.wiedergabe': 'Wiedergabe',
  'einst.untertitel': 'Untertitel',
  'einst.schnellverbindung': 'Schnellverbindung',
  'einst.konto': 'Konto',
  'einst.app': 'App',
  'einst.admin': 'Admin',
  'einst.benutzer': 'Benutzer',
  // Profil
  'einst.profil.wechseln.text': 'Anderes Profil auf diesem Gerät',
  'einst.profil.verwalten': 'Profile und PINs verwalten',
  'einst.profil.verwalten.text': 'Im Dashboard unter „Benutzer“',
  'einst.profil.pin': 'PIN ändern',
  'einst.profil.pin.text': 'Das darf nur ein Admin. Frag den, der Flimmer eingerichtet hat.',
  'einst.koppeln.text': 'Einen Fernseher mit dem Code auf seinem Bildschirm anmelden',
  // Anzeige
  'einst.thema': 'Erscheinungsbild',
  'einst.thema.text': 'Gilt nur auf diesem Gerät.',
  'einst.thema.kino': 'Dunkel',
  'einst.thema.hell': 'Hell',
  'einst.thema.system': 'Wie das System',
  'einst.ansicht': 'Ansicht',
  'einst.ansicht.text': 'Gilt nur auf diesem Gerät. Wähle „Fernseher“, wenn der Browser des Fernsehers die große Ansicht nicht von selbst zeigt.',
  'einst.ansicht.auto': 'Automatisch',
  'einst.ansicht.tv': 'Fernseher',
  'einst.ansicht.dt': 'Computer',
  'einst.ansicht.hd': 'Handy',
  'einst.sprache': 'Sprache der Oberfläche',
  'einst.sprache.text': 'Titel und Beschreibungen kommen in der Sprache, die der Admin für den Server gewählt hat.',
  // Startseite
  'einst.start.text': 'Wähle, welche Reihen du auf der Startseite siehst – und in welcher Reihenfolge. Gilt für dein Profil auf diesem Gerät.',
  'einst.start.reihen': 'Reihen der Startseite',
  'einst.start.an': 'Sichtbar',
  'einst.start.aus': 'Ausgeblendet',
  'einst.start.hoch': '{name} nach oben',
  'einst.start.runter': '{name} nach unten',
  'einst.reihe.continue': 'Weiterschauen',
  'einst.reihe.nextup': 'Als Nächstes',
  'einst.reihe.recent-movies': 'Kürzlich hinzugefügt in Filme',
  'einst.reihe.recent-series': 'Kürzlich hinzugefügt in Serien',
  // Wiedergabe
  'einst.wg.text': 'Gilt für dein Profil auf diesem Gerät.',
  'einst.tonsprache': 'Bevorzugte Tonsprache',
  'einst.tonsprache.text': 'Flimmer wählt zuerst die Tonspur in dieser Sprache.',
  'einst.qualitaet': 'Maximale Qualität',
  'einst.qualitaet.text': 'Größere Bilder rechnet der Server beim Umwandeln herunter.',
  'einst.qualitaet.auto': 'Auto',
  'einst.nacht': 'Nachtmodus',
  'einst.nacht.text': 'Laute Stellen leiser, leise Dialoge lauter – für alle Titel',
  'einst.geraet': 'Dieses Gerät',
  'einst.test': 'Gerät neu testen',
  'einst.test.text': 'Spielt kurze Clips ab und misst, was dieses Gerät wirklich kann',
  'einst.test.laeuft': 'Wird getestet …',
  'einst.test.fertig': '{n} Formate laufen direkt',
  // Untertitel
  'einst.ut.text': 'Legt fest, wann Untertitel von selbst erscheinen. Im Player kannst du jederzeit umschalten.',
  'einst.ut.modus': 'Untertitelmodus',
  'einst.ut.auto': 'Intelligent',
  'einst.ut.auto.text': 'Zeigt Untertitel, wenn der Ton nicht in deiner Sprache ist, sonst nur erzwungene.',
  'einst.ut.always': 'Immer',
  'einst.ut.always.text': 'Zeigt immer Untertitel in deiner Sprache.',
  'einst.ut.off': 'Nur erzwungene',
  'einst.ut.off.text': 'Zeigt nur Untertitel für fremdsprachige Stellen.',
  'einst.ut.sprache': 'Sprache der Untertitel',
  'einst.ut.sprache.text': 'Folgt der bevorzugten Tonsprache unter „Wiedergabe“.',
  // Schnellverbindung
  'einst.sv.text': 'Ein anderes Gerät zeigt einen 6-stelligen Code? Gib ihn hier ein – es meldet sich dann als dein Profil an, ohne dass du ein Passwort tippst.',
  'einst.sv.code': 'Code',
  'einst.sv.knopf': 'Anmelden',
  'einst.sv.ok': '„{geraet}“ ist jetzt angemeldet.',
  'einst.sv.falsch': 'Diesen Code kennt der Server nicht, oder er ist abgelaufen.',
})

type Bereich = 'profil' | 'anzeige' | 'startseite' | 'wiedergabe' | 'untertitel' | 'schnellverbindung'
const BEREICHE: { id: Bereich; icon: IconName }[] = [
  { id: 'profil', icon: 'profil' },
  { id: 'schnellverbindung', icon: 'schluessel' },
  { id: 'anzeige', icon: 'fernseher' },
  { id: 'startseite', icon: 'start' },
  { id: 'wiedergabe', icon: 'abspielen' },
  { id: 'untertitel', icon: 'untertitel' },
]
// Alte Bereiche (vor dem Umbau) → neue Ziele
const ALT: Record<string, string> = {
  geraet: '/einstellungen/wiedergabe',
  profile: '/dashboard/benutzer',
  bibliothek: '/dashboard/bibliotheken',
  server: '/dashboard/allgemein',
  fernzugriff: '/dashboard/netzwerk',
  einladungen: '/dashboard/einladungen',
  nachts: '/dashboard/umwandlung',
  sicherung: '/dashboard/sicherung',
  diagnose: '/dashboard/sicherung',
}

const abmeldenUndNeu = () => abmelden().then(() => location.reload(), () => location.reload())

export function Einstellungen() {
  const g = useGeraet()
  const n = useIch()
  const roh = useRoute()[1] || ''
  const handy = g === 'hd'
  const bereich = (BEREICHE.filter((b) => b.id === roh).length ? roh : handy ? '' : 'profil') as Bereich | ''
  useEffect(() => {
    if (ALT[roh]) go(ALT[roh], { ersetzen: true })
  }, [roh])
  useEffect(() => {
    fokusBald(bereich && !handy ? 'einst-' + bereich : 'inhalt')
  }, [])
  const waehle = (b: Bereich) => go('/einstellungen/' + b, { ersetzen: !handy }) // Desktop: Zurück verlässt die Einstellungen

  const inhalt = bereich && (
    <Gruppe fokusKey="inhalt" class="einst-inhalt">
      {!handy && <h2 class="einst-titel">{t('einst.' + bereich)}</h2>}
      {bereich === 'profil' && <Profil />}
      {bereich === 'anzeige' && <Anzeige />}
      {bereich === 'startseite' && <Startseite />}
      {bereich === 'wiedergabe' && <Wiedergabe />}
      {bereich === 'untertitel' && <Untertitel />}
      {bereich === 'schnellverbindung' && <Schnellverbindung />}
    </Gruppe>
  )

  return (
    <Seite bereich="einstellungen" titel={handy && bereich ? t('einst.' + bereich) : t('einst.titel')}>
      <div class="admin rand">
        {handy ? (
          bereich ? (
            inhalt
          ) : (
            <Gruppe fokusKey="inhalt">
              <Menue n={n} waehle={waehle} />
            </Gruppe>
          )
        ) : (
          <div class="admin-spalten">
            <Gruppe fokusKey="einst-nav" tag="nav" class="admin-liste einst-nav" bevorzugt={'einst-' + bereich} label={t('einst.titel')}>
              {n && (
                <div class="einst-wer fl-reihe-flex">
                  <MiniAvatar n={n} class="admin-avatar gross" />
                  <span class="fl-grow">
                    <b class="eine-zeile">{n.name}</b>
                    <small>{n.admin ? t('einst.admin') : t('einst.benutzer')}</small>
                  </span>
                </div>
              )}
              {BEREICHE.map((b) => (
                <NavPunkt key={b.id} b={b} aktiv={b.id === bereich} waehle={waehle} />
              ))}
            </Gruppe>
            <div class="admin-detail">{inhalt}</div>
          </div>
        )}
      </div>
      <Toast />
    </Seite>
  )
}

function NavPunkt({ b, aktiv, waehle }: { b: (typeof BEREICHE)[number]; aktiv: boolean; waehle: (b: Bereich) => void }) {
  // TV: schon der Fokus zeigt den Bereich, OK springt in den Inhalt.
  const f = useFokus<HTMLButtonElement>({
    fokusKey: 'einst-' + b.id,
    onPress: () => (waehle(b.id), fokusBald('inhalt')),
    onFocus: () => istTV() && !aktiv && waehle(b.id),
  })
  return (
    <button ref={f.ref} {...f.dom} type="button" aria-current={aktiv ? 'page' : undefined} class={'nav-punkt einst-punkt' + (aktiv ? ' an' : '') + (f.fokus ? ' ist-fokus' : '')}>
      <Icon name={b.icon} />
      <span class="text eine-zeile">{t('einst.' + b.id)}</span>
    </button>
  )
}

// Handy: Übersicht wie Handy-Einstellungen.dc.html
function Menue({ n, waehle }: { n: ReturnType<typeof useIch>; waehle: (b: Bereich) => void }) {
  const zeile = (b: Bereich, icon: IconName, unter?: string) => <Zeile key={b} fk={'einst-' + b} icon={icon} titel={t('einst.' + b)} unter={unter} onPress={() => waehle(b)} />
  return (
    <>
      {n && (
        <div class="einst-wer fl-reihe-flex">
          <MiniAvatar n={n} class="admin-avatar gross" />
          <span class="fl-grow">
            <b class="eine-zeile">{n.name}</b>
            <small>{n.admin ? t('einst.admin') : t('einst.benutzer')}</small>
          </span>
        </div>
      )}
      <Abschnitt titel={t('einst.konto')}>
        {zeile('profil', 'profil')}
        {zeile('schnellverbindung', 'schluessel', t('einst.sv.text').split(' – ')[0])}
        <Zeile fk="einst-wechseln" icon="wechseln" titel={t('nav.profil')} onPress={() => go('/profile')} />
      </Abschnitt>
      <Abschnitt titel={t('einst.app')}>
        {zeile('anzeige', 'fernseher')}
        {zeile('startseite', 'start')}
        {zeile('wiedergabe', 'abspielen')}
        {zeile('untertitel', 'untertitel')}
      </Abschnitt>
      {n && n.admin && (
        <Abschnitt titel={t('nav.administration')}>
          <Zeile fk="einst-dash" icon="dashboard" titel={t('nav.dashboard')} onPress={() => go('/dashboard')} />
          <Zeile fk="einst-meta" icon="bearbeiten" titel={t('nav.metadaten')} onPress={() => go('/dashboard/metadaten')} />
        </Abschnitt>
      )}
      <Abschnitt>
        <Zeile fk="einst-abmelden" icon="abmelden" titel={t('nav.abmelden')} onPress={abmeldenUndNeu} />
      </Abschnitt>
    </>
  )
}

// ---------- Profil ----------
function Profil() {
  const n = useIch()
  return (
    <>
      <Abschnitt>
        <Zeile fk="profil-wechseln" icon="wechseln" titel={t('nav.profil')} unter={(n ? n.name + ' · ' : '') + t('einst.profil.wechseln.text')} onPress={() => go('/profile')} />
        {n && n.admin ? (
          <Zeile fk="profil-verwalten" icon="gemeinsam" titel={t('einst.profil.verwalten')} unter={t('einst.profil.verwalten.text')} onPress={() => go('/dashboard/benutzer')} />
        ) : (
          <Zeile icon="schloss" titel={t('einst.profil.pin')} unter={t('einst.profil.pin.text')} />
        )}
        {!istTV() && <Zeile fk="profil-koppeln" icon="fernseher" titel={t('nav.koppeln')} unter={t('einst.koppeln.text')} onPress={() => go('/koppeln')} />}
        <Zeile fk="profil-abmelden" icon="abmelden" titel={t('nav.abmelden')} onPress={abmeldenUndNeu} />
      </Abschnitt>
    </>
  )
}

// ---------- Anzeige ----------
function Anzeige() {
  const [th, setTh] = useState<Thema>(thema())
  return (
    <>
      {!istTV() && (
        <Abschnitt titel={t('einst.thema')} text={t('einst.thema.text')}>
          <Wahl
            fk="thema"
            an={th}
            onWahl={(x) => (setThema(x), setTh(x))}
            werte={(['kino', 'hell', 'system'] as Thema[]).map((x) => [x, t('einst.thema.' + x)] as [Thema, string])}
          />
        </Abschnitt>
      )}
      <Abschnitt titel={t('einst.ansicht')} text={t('einst.ansicht.text')}>
        <Wahl
          fk="ansicht"
          an={geraetWahl()}
          onWahl={setGeraetWahl}
          werte={(['auto', 'tv', 'dt', 'hd'] as (Geraet | 'auto')[]).map((x) => [x, t('einst.ansicht.' + x)] as [Geraet | 'auto', string])}
        />
      </Abschnitt>
      <Abschnitt titel={t('einst.sprache')} text={t('einst.sprache.text')}>
        <Wahl
          fk="sprache"
          an={sprache}
          onWahl={setSprache}
          werte={[
            ['de', 'Deutsch'],
            ['en', 'English'],
          ]}
        />
      </Abschnitt>
    </>
  )
}

// ---------- Startseite ----------
function Startseite() {
  const [v, setV] = useState(startVorlieben())
  const setze = (neu: typeof v) => {
    setStartVorlieben(neu)
    setV(neu)
  }
  const liste = v.reihenfolge.concat(START_REIHEN.filter((x) => v.reihenfolge.indexOf(x) < 0))
  const schiebe = (i: number, d: number) => {
    const neu = liste.slice()
    const x = neu.splice(i, 1)[0]
    neu.splice(i + d, 0, x)
    setze({ ...v, reihenfolge: neu })
  }
  return (
    <Abschnitt titel={t('einst.start.reihen')} text={t('einst.start.text')}>
      {liste.map((id, i) => {
        const aus = v.aus.indexOf(id) >= 0
        const name = t('einst.reihe.' + id)
        return (
          <Zeile
            key={id}
            icon={aus ? 'schliessen' : 'haken'}
            class={aus ? 'leise' : undefined}
            titel={String(i + 1).replace(/^(\d)$/, '0$1') + ' · ' + name}
            rechts={
              <>
                <Button fokusKey={'start-an-' + id} variante={aus ? 'sekundaer' : 'geist'} onPress={() => setze({ ...v, aus: aus ? v.aus.filter((x) => x !== id) : v.aus.concat(id) })}>
                  {t(aus ? 'einst.start.aus' : 'einst.start.an')}
                </Button>
                <Button fokusKey={'start-hoch-' + id} icon="hoch" label={t('einst.start.hoch', { name })} aus={i === 0} onPress={() => schiebe(i, -1)} />
                <Button fokusKey={'start-runter-' + id} icon="runter" label={t('einst.start.runter', { name })} aus={i === liste.length - 1} onPress={() => schiebe(i, 1)} />
              </>
            }
          />
        )
      })}
    </Abschnitt>
  )
}

// ---------- Wiedergabe ----------
const TONSPRACHEN = ['de', 'en', 'fr', 'es', 'it', 'ja', 'tr']
const SPRACHNAMEN: Record<string, string> = { de: 'Deutsch', en: 'English', fr: 'Français', es: 'Español', it: 'Italiano', ja: '日本語', tr: 'Türkçe' }

function useVorlieben(): [Vorlieben, (teil: Partial<Vorlieben>) => void] {
  const [v, setV] = useState(vorlieben())
  return [
    v,
    (teil) => {
      setVorlieben(teil)
      setV(vorlieben())
      melde(t('einst.gespeichert'))
    },
  ]
}

function Wiedergabe() {
  const [v, aendern] = useVorlieben()
  const [test, setTest] = useState('')
  const probe = loadProbe()
  return (
    <>
      <p class="t-klein leise admin-absatz">{t('einst.wg.text')}</p>
      <Abschnitt titel={t('einst.tonsprache')} text={t('einst.tonsprache.text')}>
        <Wahl fk="ton" an={v.audioLangs[0] || 'de'} onWahl={(x) => aendern({ audioLangs: x === 'en' ? ['en'] : [x, 'en'] })} werte={TONSPRACHEN.map((x) => [x, SPRACHNAMEN[x]] as [string, string])} />
      </Abschnitt>
      <Abschnitt titel={t('einst.qualitaet')} text={t('einst.qualitaet.text')}>
        <Wahl
          fk="qualitaet"
          an={v.maxHeight || 0}
          onWahl={(maxHeight) => aendern({ maxHeight })}
          werte={[
            [0, t('einst.qualitaet.auto')],
            [1080, '1080p'],
            [720, '720p'],
            [480, '480p'],
          ]}
        />
      </Abschnitt>
      <Abschnitt>
        <Zeile fk="nacht" icon="ton" titel={t('einst.nacht')} unter={t('einst.nacht.text')} schalter={v.night} onPress={() => aendern({ night: !v.night })} />
      </Abschnitt>
      <Abschnitt titel={t('einst.geraet')}>
        <Zeile
          fk="geraetetest"
          icon="neustart"
          titel={t('einst.test')}
          unter={test || (probe ? t('einst.test.fertig', { n: probe.played.length }) : t('einst.test.text'))}
          onPress={() => {
            setTest(t('einst.test.laeuft'))
            geraeteTest.starten().then(() => {
              const r = loadProbe()
              setTest(r ? t('einst.test.fertig', { n: r.played.length }) : '')
            })
          }}
        />
      </Abschnitt>
    </>
  )
}

// ---------- Untertitel ----------
function Untertitel() {
  const [v, aendern] = useVorlieben()
  const modi: [Vorlieben['subtitleMode'], string][] = [
    ['', 'auto'],
    ['always', 'always'],
    ['off', 'off'],
  ]
  return (
    <>
      <p class="t-klein leise admin-absatz">{t('einst.ut.text')}</p>
      <Abschnitt titel={t('einst.ut.modus')}>
        {modi.map(([w, k]) => (
          <Zeile key={k} fk={'ut-' + k} icon={v.subtitleMode === w ? 'haken' : 'punkt'} an={v.subtitleMode === w} titel={t('einst.ut.' + k)} unter={t('einst.ut.' + k + '.text')} onPress={() => aendern({ subtitleMode: w })} />
        ))}
      </Abschnitt>
      <Abschnitt titel={t('einst.ut.sprache')}>
        <Zeile fk="ut-sprache" icon="ton" titel={SPRACHNAMEN[v.audioLangs[0] || 'de']} unter={t('einst.ut.sprache.text')} onPress={() => go('/einstellungen/wiedergabe', { ersetzen: true })} />
      </Abschnitt>
    </>
  )
}

// ---------- Schnellverbindung ----------
function Schnellverbindung() {
  const [code, setCode] = useState('')
  const [ok, setOk] = useState('')
  const [fehler, setFehler] = useState('')
  const los = () => {
    const c = code.replace(/\D/g, '')
    if (c.length !== 6) return
    setOk('')
    setFehler('')
    koppeln(c).then(
      (r) => {
        setOk(t('einst.sv.ok', { geraet: r.device }))
        setCode('')
      },
      () => setFehler(t('einst.sv.falsch')),
    )
  }
  return (
    <Abschnitt text={t('einst.sv.text')}>
      <div class="admin-formzeile">
        <Feld fk="sv-code" wert={code} setWert={(w) => setCode(w.replace(/\D/g, '').slice(0, 6))} label={t('einst.sv.code')} typ="tel" zeigeLabel onEnter={los} />
        <Button fokusKey="sv-los" variante="primaer" aus={code.length !== 6} onPress={los}>
          {t('einst.sv.knopf')}
        </Button>
      </div>
      {ok && <Hinweis icon="haken" text={ok} />}
      {fehler && <Hinweis icon="fehler" text={fehler} />}
    </Abschnitt>
  )
}
