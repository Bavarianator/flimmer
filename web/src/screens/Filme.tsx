import { useEffect, useState } from 'preact/hooks'
import { Button, Chip } from '../components/Button'
import { Raster } from '../components/Reihe'
import { Seite } from '../components/Seite'
import { SkeletonKarten } from '../components/Skeleton'
import { TitelKarte } from '../components/TitelKarte'
import { Fehler, Leer } from '../components/Zustand'
import { useSeiten, type Item } from '../lib/api'
import { Gruppe, fokusStart } from '../lib/focus'
import { t } from '../lib/i18n'

// Filter, die ohne neue API gehen. Genre, Jahr, Bewertung und FSK brauchen Server-Filter (SP).
const filter: { id: string; icon?: 'plus' | 'haken' | 'abspielen'; test?: (i: Item) => boolean }[] = [
  { id: 'neu', icon: 'plus' },
  { id: 'ungesehen', test: (i) => !i.watched },
  { id: 'direkt', icon: 'abspielen', test: (i) => i.light === 'green' },
]

// Alle Filme als Raster, seitenweise vom Server (?offset&limit). Nähert sich der Fokus dem Ende,
// wird die nächste Seite geholt; per Maus/Touch gibt es „Mehr laden“. Ein aktiver Filter lädt alles.
export function Filme() {
  const s = useSeiten(200)
  const [an, setAn] = useState<Record<string, boolean>>({})
  const aktiv = Object.keys(an).filter((k) => an[k])
  const alle = s.items.filter((i) => !i.series)
  let filme = alle.filter((i) => filter.every((f) => !an[f.id] || !f.test || f.test(i)))
  if (an.neu) filme = filme.slice().sort((a, b) => (b.added || '').localeCompare(a.added || ''))
  // Seiten voller Folgen liefern kaum Filme, Filter brauchen alles: dann gleich weiterladen.
  useEffect(() => {
    if (!s.fertig && !s.fehler && (alle.length < 60 || aktiv.length)) s.mehr()
  }, [s.items.length, s.fertig, aktiv.length])
  const da = filme.length > 0
  useEffect(() => {
    if (alle.length) fokusStart('filme-' + alle[0].id)
  }, [alle.length > 0])

  return (
    <Seite bereich="filme">
      <div class="seitenkopf rand">
        <h1>
          {t('nav.filme')}
          {s.fertig && <small>{' · ' + alle.length + (aktiv.length ? ' · ' + t('filter.passen', { n: filme.length }) : '')}</small>}
        </h1>
        <Gruppe fokusKey="filter">
          {filter.map((f) => (
            <Chip key={f.id} fokusKey={'filter-' + f.id} an={!!an[f.id]} icon={f.icon} onPress={() => setAn({ ...an, [f.id]: !an[f.id] })}>
              {t('filter.' + f.id)}
            </Chip>
          ))}
        </Gruppe>
      </div>
      {s.fehler && !da ? (
        <Fehler fehler={s.fehler} nochmal={s.mehr} />
      ) : s.fertig && !da ? (
        <Leer titel={t('leer.filme')} />
      ) : (
        <Raster fokusKey="raster-filme">
          {filme.map((it, i) => (
            <TitelKarte key={it.id} it={it} reihe="filme" onFocus={i > filme.length - 16 ? s.mehr : undefined} />
          ))}
          {!s.fertig && <SkeletonKarten n={da ? 4 : 12} />}
        </Raster>
      )}
      {!s.fertig && da && (
        <Gruppe fokusKey="mehr-gruppe" class="rand">
          <Button fokusKey="mehr" onPress={s.mehr}>
            {t('knopf.mehr')}
          </Button>
        </Gruppe>
      )}
    </Seite>
  )
}
