// Dashboard › Umwandlung: erkannte Hardware-Beschleunigung und „Nachts vorbereiten“.
// Flimmer wählt Encoder und Qualität selbst; im Dashboard lassen sich Encoder und Tempo erzwingen.
import { useEffect, useState } from 'preact/hooks'
import { useDaten } from '../../lib/api'
import { diagnose, einstellungen, einstellungenSetzen, optimieren, optimierenVorgemerkt, optimierenWeg, type OptWarteschlange, type Settings } from '../../lib/api-admin'
import { ergaenze, t } from '../../lib/i18n'
import { Button } from '../../components/Button'
import { Abschnitt, Feld, Geladen, Messwert, Wahl, Zeile, melde, meldeFehler } from './teile'

ergaenze(
  {
    'umw.hw': 'Hardwarebeschleunigung',
    'umw.hw.text': 'Flimmer testet beim Start, welche Beschleunigung der Server hat, und nimmt die schnellste.',
    'umw.encoder': 'Encoder',
    'umw.tempo': 'Tempo (1080p)',
    'umw.kerne': 'Prozessorkerne',
    'umw.ffmpeg.fehlt': 'ffmpeg fehlt – Umwandeln ist nicht möglich.',
    'umw.hw.erzwingen': 'Encoder erzwingen',
    'umw.hw.auto': 'Auto',
    'umw.hw.sw': 'CPU (libx264)',
    'umw.hw.erzwingen.text': 'Automatisch lässt Flimmer messen und wählt selbst. Erzwungene Encoderverfahren: geht der Test daneben (Gerät fehlt, Codec fehlt), bleibt es bei der automatischen Wahl.',
    'umw.hw.tempo': 'Festes Tempo (×)',
    'umw.hw.tempo.text': 'Ampel und Nacht-Sampling rechnen mit diesem Wert, statt mit dem gemessenen. 0 = messen.',
    'umw.opt': 'Nachts vorbereiten',
    'umw.opt.titel': 'Titel nachts vorbereiten',
    'umw.opt.text': 'Wandelt schwierige Titel vorab um, damit sie abends direkt laufen.',
    'umw.opt.von': 'Von (Uhr)',
    'umw.opt.bis': 'Bis (Uhr)',
    'umw.opt.frei': 'Mindestens frei (GB)',
    'umw.opt.stand': '{done} fertig · {pending} warten · Fenster {fenster}',
    'umw.opt.jetzt': 'Gerade: {titel} ({n} %)',
    'umw.opt.wartet': 'Startet, sobald der Server bereit ist.',
    'umw.opt.art': 'Im Menü vorgemerkt',
    'umw.opt.art.text': 'Titel, die du im Menü (…) am Film oder bei der Folge mit „Umwandeln“ vorgemerkt hast. Sie kommen zusätzlich immer dran, auch wenn kein Gerät Rot zeigt – ab {fenster}.',
    'umw.opt.art.weg': 'Vormerkung entfernen',
    'umw.opt.art.leer': 'Noch nichts vorgemerkt. Öffne das Menü (…) am Titel und wähle „Umwandeln“.',
  },
  {
    'umw.hw': 'Hardware acceleration',
    'umw.hw.text': 'Flimmer tests the hardware of the server at startup and picks the fastest option.',
    'umw.encoder': 'Encoder',
    'umw.tempo': 'Speed (1080p)',
    'umw.kerne': 'Processor cores',
    'umw.ffmpeg.fehlt': 'ffmpeg is missing – converting is not possible.',
    'umw.hw.erzwingen': 'Force encoder',
    'umw.hw.auto': 'Auto',
    'umw.hw.sw': 'CPU (libx264)',
    'umw.hw.erzwingen.text': 'Automatic means Flimmer measures and chooses. Forced encoder: if the test fails (device or codec missing), the automatic choice keeps the server working.',
    'umw.hw.tempo': 'Fixed speed (×)',
    'umw.hw.tempo.text': 'The traffic light and night preparation assume this value instead of the measured one. 0 = measure.',
    'umw.opt': 'Prepare overnight',
    'umw.opt.titel': 'Prepare titles overnight',
    'umw.opt.text': 'Converts difficult titles in advance so they play directly in the evening.',
    'umw.opt.von': 'From (o’clock)',
    'umw.opt.bis': 'To (o’clock)',
    'umw.opt.frei': 'At least free (GB)',
    'umw.opt.stand': '{done} done · {pending} waiting · window {fenster}',
    'umw.opt.jetzt': 'Currently: {titel} ({n} %)',
    'umw.opt.wartet': 'Starts as soon as the server is ready.',
    'umw.opt.art': 'Queued in menus',
    'umw.opt.art.text': 'Titles queued with “Convert” in the (…) menu of a film or episode. They always get converted too, even when no device shows red – starting at {fenster}.',
    'umw.opt.art.weg': 'Remove from queue',
    'umw.opt.art.leer': 'Nothing queued yet. Open the (…) menu of a title and choose “Convert”.',
  },
)

export function Umwandlung() {
  const d = useDaten('admin.diagnose', diagnose)
  return (
    <>
      <Abschnitt titel={t('umw.hw')} text={t('umw.hw.text')}>
        <Geladen z={d}>
          {(x) =>
            x.ffmpeg ? (
              <div class="admin-kennzahlen">
                <Messwert label={t('umw.encoder')} wert={x.hw || 'CPU'} />
                <Messwert label={t('umw.tempo')} wert={x.hwSpeed ? x.hwSpeed.toFixed(1).replace('.', ',') + '×' : '–'} />
                <Messwert label={t('umw.kerne')} wert={String(x.cpus)} />
                <Messwert label="ffmpeg" wert={x.ffmpeg.split(' ')[0]} />
              </div>
            ) : (
              <Zeile icon="fehler" titel={t('umw.ffmpeg.fehlt')} />
            )
          }
        </Geladen>
        <HwSetzen neu={() => d.neu()} />
      </Abschnitt>
      <Nachts />
      <Vorgemerkt />
    </>
  )
}

// Erzwungenes Encoder-Verfahren und festes Tempo: in flimmer.db gespeichert, nach dem Speichern misst der
// Server neu (HwRecheck), ohne Neustart. Die Messwerte oben zeigen danach die frische Wahl samt angewandtem Tempo.
function HwSetzen({ neu }: { neu: () => void }) {
  const s = useDaten('admin.einstellungen', einstellungen)
  const [enc, setEnc] = useState('auto')
  const [tempo, setTempo] = useState('')
  useEffect(() => {
    if (!s.daten) return
    setEnc(s.daten.hw.encoder || 'auto')
    setTempo(s.daten.hw.speed ? String(s.daten.hw.speed).replace('.', ',') : '')
  }, [s.daten])
  const sichern = () => {
    const speed = Number(tempo.replace(',', '.'))
    if (tempo && (!isFinite(speed) || speed < 0.5 || speed > 100)) {
      meldeFehler(new Error(t('umw.hw.tempo.text')))
      return
    }
    einstellungenSetzen({ hw: { encoder: enc, speed: speed || 0 } }).then(() => {
      melde(t('einst.gespeichert'))
      s.neu()
      neu()
    }, meldeFehler)
  }
  return (
    <>
      <p class="t-klein leise admin-absatz">{t('umw.hw.erzwingen')}</p>
      <Geladen z={s}>
        {() => (
          <>
            <p class="t-klein leise admin-absatz">{t('umw.hw.erzwingen.text')}</p>
            <Wahl
              fk="umw-hw"
              werte={[
                ['auto', t('umw.hw.auto')],
                ['vaapi', 'VAAPI'],
                ['qsv', 'QSV'],
                ['nvenc', 'NVENC'],
                ['videotoolbox', 'VideoToolbox'],
                ['v4l2m2m', 'V4L2'],
                ['software', t('umw.hw.sw')],
              ]}
              an={enc}
              onWahl={(w) => setEnc(w)}
            />
            <div class="admin-formzeile">
              <Feld fk="umw-hw-tempo" wert={tempo} setWert={setTempo} label={t('umw.hw.tempo')} typ="number" zeigeLabel />
              <div>
                <p class="nur-sr">{t('umw.hw.tempo.text')}</p>
                <Button fokusKey="umw-hw-ok" onPress={sichern}>
                  {t('einst.speichern')}
                </Button>
              </div>
            </div>
          </>
        )}
      </Geladen>
    </>
  )
}

// Vorgemerkte Titel (Menü „Umwandeln“): die kommen unabhängig von der Gerätempel immer dran.
function Vorgemerkt() {
  const s = useDaten('admin.einstellungen', einstellungen)
  const q = useDaten('admin.optimieren.warteschlange', optimierenVorgemerkt)
  useEffect(() => {
    const f = () => q.neu()
    window.addEventListener('focus', f)
    return () => window.removeEventListener('focus', f)
  }, [])
  const fenster = s.daten ? (s.daten.optimize.off ? '' : `${s.daten.optimize.from || 2}–${s.daten.optimize.to || 6} Uhr`) : ''
  const weg = (e: OptWarteschlange) => optimierenWeg(e.id).then(() => q.neu(), meldeFehler)
  return (
    <Abschnitt titel={t('umw.opt.art')} text={fenster ? t('umw.opt.art.text', { fenster }) : ''}>
      <Geladen z={q}>
        {(liste) =>
          liste.length ? (
            <>
              {liste.map((e, i) => (
                <Zeile
                  key={e.id}
                  fk={'umw-q-' + i}
                  icon="film"
                  titel={e.title}
                  unter={e.since}
                  rechts={
                    <Button fokusKey={'umw-q-weg-' + i} variante="geist" onPress={() => weg(e)}>
                      {t('umw.opt.art.weg')}
                    </Button>
                  }
                />
              ))}
            </>
          ) : (
            <p class="t-klein leise admin-absatz">{t('umw.opt.art.leer')}</p>
          )
        }
      </Geladen>
    </Abschnitt>
  )
}

function Nachts() {
  const s = useDaten('admin.einstellungen', einstellungen)
  const st = useDaten('admin.optimieren', optimieren)
  const [von, setVon] = useState('')
  const [bis, setBis] = useState('')
  const [frei, setFrei] = useState('')
  useEffect(() => {
    if (!s.daten) return
    const o = s.daten.optimize
    const std = !o.from && !o.to
    setVon(String(std ? 2 : o.from))
    setBis(String(std ? 6 : o.to))
    setFrei(String(o.minFreeGB || 20))
  }, [s.daten])
  return (
    <Abschnitt titel={t('umw.opt')}>
      <Geladen z={s}>
        {(e) => {
          const o = e.optimize
          const sichern = (teil: Partial<Settings['optimize']>) =>
            einstellungenSetzen({ optimize: { off: o.off, from: Number(von) || 0, to: Number(bis) || 0, minFreeGB: Number(frei) || 0, ...teil } }).then(() => {
              melde(t('einst.gespeichert'))
              s.neu()
              st.neu()
            }, meldeFehler)
          const x = st.daten
          return (
            <>
              <Zeile fk="umw-opt" icon="uhr" titel={t('umw.opt.titel')} unter={t('umw.opt.text')} schalter={!o.off} onPress={() => sichern({ off: !o.off })} />
              {x && (
                <p class="t-text leise admin-absatz">
                  {x.waiting
                    ? t('umw.opt.wartet')
                    : t('umw.opt.stand', { done: x.done || 0, pending: x.pending || 0, fenster: x.window || '' }) +
                      (x.current ? ' · ' + t('umw.opt.jetzt', { titel: x.current.title, n: Math.round(x.current.percent) }) : '')}
                </p>
              )}
              {x && x.lastError && <p class="t-klein admin-fehler admin-absatz">{x.lastError}</p>}
              <div class="admin-formzeile">
                <Feld fk="umw-von" wert={von} setWert={setVon} label={t('umw.opt.von')} typ="number" zeigeLabel />
                <Feld fk="umw-bis" wert={bis} setWert={setBis} label={t('umw.opt.bis')} typ="number" zeigeLabel />
                <Feld fk="umw-frei" wert={frei} setWert={setFrei} label={t('umw.opt.frei')} typ="number" zeigeLabel />
                <Button fokusKey="umw-ok" onPress={() => sichern({})}>
                  {t('einst.speichern')}
                </Button>
              </div>
            </>
          )
        }}
      </Geladen>
    </Abschnitt>
  )
}
