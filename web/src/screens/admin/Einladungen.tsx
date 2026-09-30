// Dashboard › Einladungen: neue Einladung (Link + QR), Liste der aktiven Einladungen.
// Rechte (Downloads, Fernzugriff) und die Gästeliste aus dem Entwurf kennt der Server (noch) nicht.
import { useState } from 'preact/hooks'
import { useDaten } from '../../lib/api'
import { einladungen, einladungNeu, einladungWeg, einstellungen } from '../../lib/api-admin'
import { ergaenze, t } from '../../lib/i18n'
import { Button, Chip } from '../../components/Button'
import { Abschnitt, Feld, Geladen, Wahl, Zeile, datum, kopieren, meldeFehler } from './teile'

ergaenze({
  'einl.neu': 'Neue Einladung',
  'einl.notiz': 'Notiz',
  'einl.notiz.text': 'Nur für dich sichtbar – so erkennst du die Einladung in der Liste wieder.',
  'einl.bibliotheken': 'Bibliotheken',
  'einl.alle': 'Alle Bibliotheken',
  'einl.gueltig': 'Gültig für',
  'einl.tag': '1 Tag',
  'einl.tage': '{n} Tage',
  'einl.max': 'Max. Einlösungen (0 = unbegrenzt)',
  'einl.erstellen': 'Einladung erstellen',
  'einl.erstellt': 'Einladung erstellt',
  'einl.erstellt.text': 'Schick diesen Link weiter. Er wird nur jetzt vollständig angezeigt – später siehst du in der Liste nur noch die Notiz.',
  'einl.aktiv': 'Aktive Einladungen',
  'einl.leer': 'Noch keine Einladungen.',
  'einl.nur': 'Nur {was}',
  'einl.titel': '{n} Titel',
  'einl.bis': 'läuft ab am {datum}',
  'einl.von': 'erstellt am {datum}',
  'einl.genutzt': '{n} / {max} eingelöst',
  'einl.genutzt.frei': '{n}-mal eingelöst',
  'einl.gaeste': '{n} Gäste',
  'einl.widerrufen': 'Widerrufen',
})

const name = (p: string) => p.split(/[\\/]/).filter(Boolean).pop() || p

export function Einladungen() {
  const liste = useDaten('admin.einladungen', einladungen)
  const s = useDaten('admin.einstellungen', einstellungen)
  const [notiz, setNotiz] = useState('')
  const [tage, setTage] = useState(14)
  const [max, setMax] = useState('1')
  const [libs, setLibs] = useState<string[]>([])
  const [neu, setNeu] = useState<{ url: string; qr?: string; hint?: string } | null>(null)
  const dirs = (s.daten && s.daten.dirs) || []
  const erstellen = () =>
    einladungNeu({ note: notiz, libraries: libs, hours: tage * 24, maxUses: Math.max(0, Number(max) || 0) }).then((r) => {
      setNeu(r)
      setNotiz('')
      liste.neu()
    }, meldeFehler)
  return (
    <>
      <div class="admin-spalten">
        <Abschnitt titel={t('einl.neu')} class="admin-detail">
          <Feld fk="einl-notiz" wert={notiz} setWert={setNotiz} label={t('einl.notiz')} zeigeLabel breit />
          <p class="t-klein leise admin-absatz">{t('einl.notiz.text')}</p>
          {dirs.length > 1 && (
            <>
              <h3>{t('einl.bibliotheken')}</h3>
              <div class="admin-chips">
                <Chip fokusKey="einl-lib-alle" an={!libs.length} onPress={() => setLibs([])}>
                  {t('einl.alle')}
                </Chip>
                {dirs.map((d) => (
                  <Chip key={d} fokusKey={'einl-lib-' + d} an={libs.indexOf(d) >= 0} onPress={() => setLibs(libs.indexOf(d) >= 0 ? libs.filter((x) => x !== d) : libs.concat(d))}>
                    {name(d)}
                  </Chip>
                ))}
              </div>
            </>
          )}
          <h3>{t('einl.gueltig')}</h3>
          <Wahl fk="einl-tage" werte={[1, 7, 14, 30].map((n) => [n, n === 1 ? t('einl.tag') : t('einl.tage', { n })] as [number, string])} an={tage} onWahl={setTage} />
          <Feld fk="einl-max" wert={max} setWert={setMax} label={t('einl.max')} typ="number" zeigeLabel />
          <div class="admin-knoepfe">
            <Button fokusKey="einl-erstellen" variante="primaer" icon="einladung" onPress={erstellen}>
              {t('einl.erstellen')}
            </Button>
          </div>
        </Abschnitt>
        {neu && (
          <Abschnitt titel={t('einl.erstellt')} text={t('einl.erstellt.text')} class="admin-liste admin-einladung">
            <p class="admin-link">{neu.url}</p>
            <Button fokusKey="einl-kopieren" icon="link" onPress={() => kopieren(neu.url)}>
              {t('einst.kopieren')}
            </Button>
            {neu.qr && <img class="admin-qr" src={neu.qr} alt="" width={180} height={180} />}
            {neu.hint && <p class="t-klein leise">{neu.hint}</p>}
          </Abschnitt>
        )}
      </div>

      <Abschnitt titel={t('einl.aktiv')}>
        <Geladen z={liste}>
          {(l) => (
            <>
              {!l.length && <p class="t-text leise">{t('einl.leer')}</p>}
              {l.map((e) => {
                const was = (e.scope.libraries || []).map(name).concat((e.scope.items || []).length ? [t('einl.titel', { n: (e.scope.items || []).length })] : [])
                return (
                  <Zeile
                    key={e.id}
                    icon="einladung"
                    titel={e.note || e.id}
                    unter={[
                      was.length ? t('einl.nur', { was: was.join(', ') }) : t('einl.alle'),
                      e.created ? t('einl.von', { datum: datum(e.created, false) }) : '',
                      t('einl.bis', { datum: datum(e.expires, false) }),
                      e.maxUses ? t('einl.genutzt', { n: e.uses, max: e.maxUses }) : t('einl.genutzt.frei', { n: e.uses }),
                      e.guests ? t('einl.gaeste', { n: e.guests }) : '',
                    ]
                      .filter(Boolean)
                      .join(' · ')}
                    rechts={
                      <Button fokusKey={'einl-weg-' + e.id} onPress={() => einladungWeg(e.id).then(liste.neu, meldeFehler)}>
                        {t('einl.widerrufen')}
                      </Button>
                    }
                  />
                )
              })}
            </>
          )}
        </Geladen>
      </Abschnitt>
    </>
  )
}
