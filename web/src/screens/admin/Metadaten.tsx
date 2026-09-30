// Dashboard › Metadaten-Manager: Titel suchen und filtern (ohne Poster, unsicher erkannt), links die Liste,
// rechts der Editor. Handy und TV öffnen den Editor als Dialog.
import { useState } from 'preact/hooks'
import { anzeigeTitel, bibliothek, useDaten, type Item } from '../../lib/api'
import { META_AUFGABE, aufgabeStarten, unsicher } from '../../lib/api-admin'
import { ergaenze, folge, t } from '../../lib/i18n'
import { Button, Chip } from '../../components/Button'
import { useGeraet } from '../../components/Seite'
import { Editor, MetadatenEditor } from './MetadatenEditor'
import { Feld, Geladen, Hinweis, Zeile, melde, meldeFehler } from './teile'

ergaenze({
  'mm.unsicher': '{n} Titel wurden unsicher zugeordnet. Bitte prüfen.',
  'mm.suchen': 'Titel suchen …',
  'mm.ohneposter': 'Nur ohne Poster',
  'mm.nurunsicher': 'Nur unsichere Treffer ({n})',
  'mm.filme': 'Filme',
  'mm.serien': 'Serien',
  'mm.mehr': '… {n} weitere Titel. Grenze die Suche ein.',
  'mm.leer': 'Nichts gefunden.',
  'mm.waehlen': 'Wähle links einen Titel, um seine Metadaten zu bearbeiten.',
  'mm.aktualisieren': 'Metadaten aktualisieren',
  'mm.aktualisieren.ok': 'Läuft im Hintergrund. Den Stand zeigt „Geplante Aufgaben“.',
  'mm.aktualisieren.text': 'Holt Personen, Studios, Leitsatz, Länder und Filmreihen für alle Titel nach. Gesperrte Felder bleiben unberührt.',
})

const GRENZE = 200

function name(it: Item) {
  return it.series ? it.series + ' · ' + folge(it.season, it.episode) + ' · ' + anzeigeTitel(it) : anzeigeTitel(it) + (it.year ? ' (' + it.year + ')' : '')
}

export function Metadaten() {
  const g = useGeraet()
  const bib = useDaten('bibliothek', () => bibliothek(true))
  const rev = useDaten('admin.unsicher', unsicher)
  const [q, setQ] = useState('')
  const [ohnePoster, setOhnePoster] = useState(false)
  const [nurUnsicher, setNurUnsicher] = useState(false)
  const [art, setArt] = useState<'' | 'film' | 'serie'>('')
  const [wahl, setWahl] = useState<{ id: string; ident: boolean } | null>(null)
  const unsichereIds: Record<string, boolean> = {}
  for (const u of rev.daten || []) unsichereIds[u.id] = true
  const nUnsicher = (rev.daten || []).length
  const inline = g === 'dt'
  const neuLaden = () => {
    bib.neu()
    rev.neu()
  }

  return (
    <>
      {nUnsicher > 0 && !nurUnsicher && <Hinweis icon="fehler" text={t('mm.unsicher', { n: nUnsicher })} />}
      <div class="admin-formzeile">
        <Feld fk="mm-q" wert={q} setWert={setQ} label={t('mm.suchen')} breit />
        <Button fokusKey="mm-aktualisieren" icon="neustart" onPress={() => aufgabeStarten(META_AUFGABE).then(() => melde(t('mm.aktualisieren.ok')), meldeFehler)}>
          {t('mm.aktualisieren')}
        </Button>
      </div>
      <p class="t-klein leise admin-absatz">{t('mm.aktualisieren.text')}</p>
      <div class="admin-chips">
        <Chip fokusKey="mm-film" an={art === 'film'} onPress={() => setArt(art === 'film' ? '' : 'film')}>
          {t('mm.filme')}
        </Chip>
        <Chip fokusKey="mm-serie" an={art === 'serie'} onPress={() => setArt(art === 'serie' ? '' : 'serie')}>
          {t('mm.serien')}
        </Chip>
        <Chip fokusKey="mm-poster" an={ohnePoster} onPress={() => setOhnePoster(!ohnePoster)}>
          {t('mm.ohneposter')}
        </Chip>
        <Chip fokusKey="mm-unsicher" icon="fehler" an={nurUnsicher} onPress={() => setNurUnsicher(!nurUnsicher)}>
          {t('mm.nurunsicher', { n: nUnsicher })}
        </Chip>
      </div>
      <Geladen z={bib}>
        {(items) => {
          const such = q.trim().toLowerCase()
          const treffer = items.filter(
            (it) =>
              (!art || (art === 'serie') === !!it.series) &&
              (!ohnePoster || !it.poster) &&
              (!nurUnsicher || unsichereIds[it.id] || (it.meta && it.meta.uncertain)) &&
              (!such || name(it).toLowerCase().indexOf(such) >= 0),
          )
          const liste = (
            <div class="fl-gruppe admin-liste">
              {!treffer.length && <p class="t-text leise">{t('mm.leer')}</p>}
              {treffer.slice(0, GRENZE).map((it) => {
                const u = !!unsichereIds[it.id] || !!(it.meta && it.meta.uncertain)
                return (
                  <Zeile
                    key={it.id}
                    fk={'mm-' + it.id}
                    icon={u ? 'fehler' : it.series ? 'serie' : 'film'}
                    class={u ? 'admin-warn' : undefined}
                    an={!!wahl && wahl.id === it.id}
                    titel={<span class="eine-zeile">{name(it)}</span>}
                    onPress={() => setWahl({ id: it.id, ident: u })}
                  />
                )
              })}
              {treffer.length > GRENZE && <p class="t-klein leise">{t('mm.mehr', { n: treffer.length - GRENZE })}</p>}
            </div>
          )
          if (!inline)
            return (
              <>
                {liste}
                {wahl && <MetadatenEditor key={wahl.id} id={wahl.id} onZu={() => (setWahl(null), neuLaden())} />}
              </>
            )
          return (
            <div class="admin-spalten">
              {liste}
              <div class="admin-detail">
                {wahl ? <Editor key={wahl.id} id={wahl.id} start={wahl.ident ? 'ident' : 'meta'} onGespeichert={neuLaden} /> : <p class="t-text leise">{t('mm.waehlen')}</p>}
              </div>
            </div>
          )
        }}
      </Geladen>
    </>
  )
}
