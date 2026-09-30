// Favoriten (Entwurf Favoriten): Filme, Serien und Folgen, die das Profil mit dem Herz markiert hat.
// Personen, Alben, Lieder und Sammlungen fehlen: der Vertrag kennt nur Item-IDs und "serie:<Name>".
import { useEffect } from 'preact/hooks'
import { AuswahlLeiste, useAuswahl } from '../components/Aktionen'
import { Raster } from '../components/Reihe'
import { Seite } from '../components/Seite'
import { SkeletonKarten } from '../components/Skeleton'
import { SerienKarte, TitelKarte } from '../components/TitelKarte'
import { Fehler, Leer } from '../components/Zustand'
import { bibliothek, favoriten, serien, useDaten, type Item } from '../lib/api'
import { fokusStart } from '../lib/focus'
import { ergaenze, t } from '../lib/i18n'
import './browse.css'

ergaenze({
  'fav.filme': 'Filme',
  'fav.serien': 'Serien',
  'fav.folgen': 'Folgen',
  'fav.leer': 'Noch keine Favoriten',
  'fav.leer.text': 'Markiere Filme, Serien und Folgen mit dem Herz. Sie erscheinen dann hier.',
})

function Abschnitt(p: { titel: string; n: number; children: preact.ComponentChildren }) {
  return (
    <section class="abschnitt">
      <h2 class="abschnitt-titel rand">
        {p.titel}
        <small>{p.n}</small>
      </h2>
      {p.children}
    </section>
  )
}

export function Favoriten() {
  const fav = useDaten<string[]>('favoriten', favoriten)
  const alle = useDaten<Item[]>('bibliothek', () => bibliothek(), { merken: true })
  const aw = useAuswahl()
  const keys = fav.daten
  const liste = alle.daten
  const byId: Record<string, Item> = {}
  for (const it of liste || []) byId[it.id] = it
  const ids = (keys || []).filter((k) => k.indexOf('serie:') !== 0).map((k) => byId[k]).filter(Boolean)
  const filme = ids.filter((it) => !it.series)
  const folgen = ids.filter((it) => !!it.series)
  const namen = (keys || []).filter((k) => k.indexOf('serie:') === 0).map((k) => k.slice(6))
  const reihen = serien(liste || []).filter((s) => namen.indexOf(s.name) >= 0)
  const da = filme.length + folgen.length + reihen.length > 0
  useEffect(() => {
    if (da) fokusStart(filme.length ? 'fav-filme-' + filme[0].id : reihen.length ? 'fav-serien-' + reihen[0].name : 'fav-folgen-' + folgen[0].id)
  }, [da])

  let inhalt: preact.ComponentChildren
  if ((fav.fehler && !keys) || (alle.fehler && !liste)) inhalt = <Fehler fehler={fav.fehler || alle.fehler} nochmal={() => (fav.neu(), alle.neu())} />
  else if (!keys || !liste)
    inhalt = (
      <div class="raster rand">
        <SkeletonKarten n={8} />
      </div>
    )
  else if (!da) inhalt = <Leer icon="herz" titel={t('fav.leer')} text={t('fav.leer.text')} />
  else
    inhalt = (
      <>
        {filme.length > 0 && (
          <Abschnitt titel={t('fav.filme')} n={filme.length}>
            <Raster fokusKey="fav-filme">
              {filme.map((it) => (
                <TitelKarte key={it.id} it={it} reihe="fav-filme" aw={aw} />
              ))}
            </Raster>
          </Abschnitt>
        )}
        {reihen.length > 0 && (
          <Abschnitt titel={t('fav.serien')} n={reihen.length}>
            <Raster fokusKey="fav-serien">
              {reihen.map((s) => (
                <SerienKarte key={s.name} s={s} reihe="fav-serien" aw={aw} />
              ))}
            </Raster>
          </Abschnitt>
        )}
        {folgen.length > 0 && (
          <Abschnitt titel={t('fav.folgen')} n={folgen.length}>
            <Raster fokusKey="fav-folgen">
              {folgen.map((it) => (
                <TitelKarte key={it.id} it={it} reihe="fav-folgen" breit aw={aw} />
              ))}
            </Raster>
          </Abschnitt>
        )}
        <AuswahlLeiste aw={aw} alle={filme.map((x) => x.id).concat(reihen.map((s) => 'serie:' + s.name), folgen.map((x) => x.id))} />
      </>
    )
  return (
    <Seite bereich="favoriten" titel={t('nav.favoriten')} class="browse">
      {inhalt}
    </Seite>
  )
}
