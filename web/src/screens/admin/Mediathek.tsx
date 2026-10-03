// Dashboard › Mediathek: Sendungen von ARD, ZDF, arte, 3sat … suchen (MediathekViewWeb), laden und als Abo speichern.
// Der Server lädt nacheinander nach <uploads>/Mediathek; Abos prüft er alle 6 Stunden (Aufgabe „mediathek“).
import { useEffect, useState } from 'preact/hooks'
import { useDaten, type Zustand } from '../../lib/api'
import {
  aboNeu,
  aboWeg,
  abos,
  aufgabeStarten,
  mediathekAbbrechen,
  mediathekDownloads,
  mediathekLaden,
  mediathekSuche,
  type Download,
  type Treffer,
} from '../../lib/api-admin'
import { dauer, ergaenze, t } from '../../lib/i18n'
import { Button, Chip } from '../../components/Button'
import { Abschnitt, Balken, Feld, Geladen, Wahl, Zeile, datum, fehlerText, groesse, melde, meldeFehler, useNachladen, vor } from './teile'
import { quelle } from './Statistik'

ergaenze({
  'mt.text': 'Frei verfügbare Sendungen der öffentlich-rechtlichen Mediatheken. Der Server lädt sie nacheinander in die Bibliothek (Ordner „Mediathek“ bei den Uploads).',
  'mt.suche': 'Titel oder Thema',
  'mt.suchen': 'Suchen',
  'mt.alle.sender': 'Alle Sender',
  'mt.alle.laengen': 'Alle Längen',
  'mt.ab': 'ab {n} Min.',
  'mt.schnell.filme': 'Spielfilme',
  'mt.schnell.dokus': 'Dokus',
  'mt.schnell.tatort': 'Tatort',
  'mt.schnell.kinder': 'Kinder',
  'mt.treffer': 'Treffer',
  'mt.treffer.zahl': '{n} Treffer',
  'mt.keine': 'Nichts gefunden.',
  'mt.mehr': 'Mehr laden',
  'mt.laden': '„{titel}“ laden',
  'mt.eingereiht': '„{titel}“ steht in der Warteschlange.',
  'mt.abo': 'Als Abo speichern',
  'mt.abo.neu': 'Abo „{text}“ angelegt. Neue Sendungen der letzten 7 Tage werden geladen.',
  'mt.abo.ohne': 'Für ein Abo braucht es einen Suchbegriff.',
  'mt.abos': 'Abos',
  'mt.abos.text': 'Abos laden neue Sendungen der letzten 7 Tage automatisch, jede nur einmal. Auf dem Laufwerk bleiben immer 10 GB frei.',
  'mt.abos.leer': 'Noch keine Abos. Suche etwas und tippe auf „Als Abo speichern“.',
  'mt.abo.weg': 'Abo „{text}“ löschen',
  'mt.pruefen': 'Jetzt prüfen',
  'mt.geprueft': 'Abos werden geprüft.',
  'mt.downloads': 'Downloads',
  'mt.downloads.leer': 'Noch nichts geladen.',
  'mt.abbrechen': '„{titel}“ abbrechen',
  'mt.entfernen': '„{titel}“ aus der Liste entfernen',
})

interface Suche {
  q: string
  sender: string
  min: number
}

const SENDER = ['ARD', 'ZDF', 'ARTE.DE', '3Sat', 'PHOENIX', 'KiKA', 'BR', 'HR', 'MDR', 'NDR', 'RBB', 'SWR', 'WDR']
const SCHNELL: [string, Suche][] = [
  ['mt.schnell.filme', { q: 'film', sender: '', min: 70 }],
  ['mt.schnell.dokus', { q: 'doku', sender: '', min: 40 }],
  ['mt.schnell.tatort', { q: 'tatort', sender: '', min: 80 }],
  ['mt.schnell.kinder', { q: '', sender: 'KiKA', min: 20 }],
]

export function Mediathek() {
  const [q, setQ] = useState('film')
  const [suche, setSuche] = useState<Suche>({ q: 'film', sender: '', min: 70 })
  const [treffer, setTreffer] = useState<Treffer[]>([])
  const [gesamt, setGesamt] = useState(0)
  const [offset, setOffset] = useState(0)
  const [laedt, setLaedt] = useState(true)
  const [fehler, setFehler] = useState('')
  const [geladen, setGeladen] = useState<Record<string, boolean>>({})
  const dl = useDaten('admin.mediathek.downloads', mediathekDownloads)
  const ab = useDaten('admin.mediathek.abos', abos)

  const hole = (s: Suche, off: number) => {
    setLaedt(true)
    setFehler('')
    mediathekSuche(s.q, s.sender, s.min, off).then(
      (r) => {
        setTreffer((alt) => (off ? alt.concat(r.treffer) : r.treffer))
        setGesamt(r.gesamt)
        setOffset(off)
        setLaedt(false)
      },
      (e) => {
        setFehler(fehlerText(e))
        setLaedt(false)
      },
    )
  }
  useEffect(() => hole(suche, 0), [suche])
  const suchen = (teil: Partial<Suche>) => {
    const s = { ...suche, q, ...teil }
    if (teil.q !== undefined) setQ(teil.q)
    setSuche(s)
  }

  const laden = (x: Treffer) =>
    mediathekLaden(x).then(
      (d) => {
        setGeladen((g) => ({ ...g, [x.id]: true }))
        melde(t('mt.eingereiht', { titel: d.titel }))
        dl.neu()
      },
      meldeFehler,
    )
  const abo = () => {
    if (!suche.q.trim()) return melde(t('mt.abo.ohne'))
    aboNeu({ text: suche.q, sender: suche.sender, minMinuten: suche.min }).then((a) => {
      melde(t('mt.abo.neu', { text: a.text }))
      ab.neu()
      setTimeout(dl.neu, 3000)
    }, meldeFehler)
  }

  return (
    <>
      <p class="t-klein leise admin-absatz">{t('mt.text')}</p>
      <div class="admin-formzeile">
        <Feld fk="mt-q" wert={q} setWert={setQ} label={t('mt.suche')} breit onEnter={() => suchen({})} />
        <Button fokusKey="mt-suchen" variante="primaer" icon="suche" onPress={() => suchen({})}>
          {t('mt.suchen')}
        </Button>
        <Button fokusKey="mt-abo" icon="plus" onPress={abo}>
          {t('mt.abo')}
        </Button>
      </div>
      <div class="admin-chips">
        {SCHNELL.map(([label, s]) => (
          <Chip key={label} fokusKey={'mt-schnell-' + label} an={s.q === suche.q && s.sender === suche.sender && s.min === suche.min} onPress={() => suchen(s)}>
            {t(label)}
          </Chip>
        ))}
      </div>
      <Wahl fk="mt-sender" werte={[['', t('mt.alle.sender')] as [string, string]].concat(SENDER.map((s) => [s, s.replace('.DE', '')]))} an={suche.sender} onWahl={(sender) => suchen({ sender })} />
      <Wahl
        fk="mt-min"
        werte={[
          [0, t('mt.alle.laengen')],
          [20, t('mt.ab', { n: 20 })],
          [45, t('mt.ab', { n: 45 })],
          [70, t('mt.ab', { n: 70 })],
        ]}
        an={suche.min}
        onWahl={(min) => suchen({ min })}
      />

      <Downloads z={dl} />

      <Abschnitt titel={t('mt.treffer')} zusatz={!laedt && gesamt ? <span class="t-klein leise">{t('mt.treffer.zahl', { n: gesamt })}</span> : undefined}>
        {fehler && <p class="t-text admin-fehler">{fehler}</p>}
        {!laedt && !fehler && !treffer.length && <p class="t-text leise">{t('mt.keine')}</p>}
        {treffer.map((x) => (
          <Zeile
            key={x.id}
            titel={<span class="eine-zeile">{x.titel}</span>}
            unter={
              <>
                <span class="eine-zeile">
                  {[x.sender, x.thema, datum(x.zeit * 1000, false), x.dauer ? dauer(x.dauer) : '', x.groesse ? groesse(x.groesse) : ''].filter(Boolean).join(' · ')}
                </span>
                {x.beschreibung && <span class="eine-zeile leise">{x.beschreibung}</span>}
              </>
            }
            rechts={<Button fokusKey={'mt-laden-' + x.id} icon="download" label={t('mt.laden', { titel: x.titel })} aus={geladen[x.id]} onPress={() => laden(x)} />}
          />
        ))}
        {laedt && <div class="admin-laedt" aria-busy="true" />}
        {!laedt && offset + 30 < gesamt && (
          <Button fokusKey="mt-mehr" variante="geist" onPress={() => hole(suche, offset + 30)}>
            {t('mt.mehr')}
          </Button>
        )}
      </Abschnitt>

      <Abschnitt
        titel={t('mt.abos')}
        text={t('mt.abos.text')}
        zusatz={
          <Button
            fokusKey="mt-pruefen"
            variante="geist"
            onPress={() =>
              aufgabeStarten('mediathek').then(() => {
                melde(t('mt.geprueft'))
                setTimeout(dl.neu, 4000)
              }, meldeFehler)
            }
          >
            {t('mt.pruefen')}
          </Button>
        }
      >
        <Geladen z={ab}>
          {(l) =>
            !l.length ? (
              <p class="t-text leise">{t('mt.abos.leer')}</p>
            ) : (
              l.map((a) => (
                <Zeile
                  key={a.id}
                  icon="kalender"
                  titel={a.text}
                  unter={[a.sender ? a.sender.replace('.DE', '') : t('mt.alle.sender'), a.minMinuten ? t('mt.ab', { n: a.minMinuten }) : '', vor(a.erstellt)].filter(Boolean).join(' · ')}
                  rechts={<Button fokusKey={'mt-abo-' + a.id} icon="loeschen" label={t('mt.abo.weg', { text: a.text })} onPress={() => aboWeg(a.id).then(ab.neu, meldeFehler)} />}
                />
              ))
            )
          }
        </Geladen>
      </Abschnitt>
    </>
  )
}

function Downloads({ z }: { z: Zustand<Download[]> }) {
  const aktiv = (z.daten || []).some((d) => d.status === 'wartet' || d.status === 'laeuft')
  useNachladen(z.neu, 2000, aktiv)
  if (!z.daten || !z.daten.length) return null
  return (
    <Abschnitt titel={t('mt.downloads')}>
      {z.daten.map((d) => {
        const offen = d.status === 'wartet' || d.status === 'laeuft'
        return (
          <Zeile
            key={d.id}
            class={d.status === 'fehler' ? 'admin-fehler' : undefined}
            titel={<span class="eine-zeile">{d.titel}</span>}
            unter={
              <>
                {[quelle(d.quelle), d.sender, t('dl.' + d.status) + (d.status === 'laeuft' && d.anteil ? ' ' + Math.round(d.anteil * 100) + ' %' : ''), d.bytes ? groesse(d.bytes) : '', d.fehler]
                  .filter(Boolean)
                  .join(' · ')}
                {d.status === 'laeuft' && <Balken anteil={d.anteil || 0} />}
              </>
            }
            rechts={
              <Button
                fokusKey={'mt-dl-' + d.id}
                icon={offen ? 'stopp' : 'schliessen'}
                label={t(offen ? 'mt.abbrechen' : 'mt.entfernen', { titel: d.titel })}
                onPress={() => mediathekAbbrechen(d.id).then(z.neu, meldeFehler)}
              />
            }
          />
        )
      })}
    </Abschnitt>
  )
}
