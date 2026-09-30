// Dashboard › Umwandlung: erkannte Hardware-Beschleunigung und „Nachts vorbereiten“.
// Flimmer wählt Encoder und Qualität selbst; die Stellschrauben aus dem Entwurf (Preset, CRF, Tone-Mapping) gibt es nicht.
import { useEffect, useState } from 'preact/hooks'
import { useDaten } from '../../lib/api'
import { diagnose, einstellungen, einstellungenSetzen, optimieren, type Settings } from '../../lib/api-admin'
import { ergaenze, t } from '../../lib/i18n'
import { Button } from '../../components/Button'
import { Abschnitt, Feld, Geladen, Messwert, Zeile, melde, meldeFehler } from './teile'

ergaenze({
  'umw.hw': 'Hardwarebeschleunigung',
  'umw.hw.text': 'Flimmer testet beim Start, welche Beschleunigung der Server hat, und nimmt die schnellste.',
  'umw.encoder': 'Encoder',
  'umw.tempo': 'Tempo (1080p)',
  'umw.kerne': 'Prozessorkerne',
  'umw.ffmpeg.fehlt': 'ffmpeg fehlt – Umwandeln ist nicht möglich.',
  'umw.opt': 'Nachts vorbereiten',
  'umw.opt.titel': 'Titel nachts vorbereiten',
  'umw.opt.text': 'Wandelt schwierige Titel vorab um, damit sie abends direkt laufen.',
  'umw.opt.von': 'Von (Uhr)',
  'umw.opt.bis': 'Bis (Uhr)',
  'umw.opt.frei': 'Mindestens frei (GB)',
  'umw.opt.stand': '{done} fertig · {pending} warten · Fenster {fenster}',
  'umw.opt.jetzt': 'Gerade: {titel} ({n} %)',
  'umw.opt.wartet': 'Startet, sobald der Server bereit ist.',
})

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
      </Abschnitt>
      <Nachts />
    </>
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
