import { useEffect } from 'preact/hooks'
import { Raster } from '../components/Reihe'
import { Seite } from '../components/Seite'
import { SkeletonKarten } from '../components/Skeleton'
import { SerienKarte } from '../components/TitelKarte'
import { Fehler, Leer } from '../components/Zustand'
import { serien, useSeiten } from '../lib/api'
import { fokusStart } from '../lib/focus'
import { t } from '../lib/i18n'

// Serien entstehen aus den Folgen; damit Staffel- und Folgenzahl stimmen, werden alle Seiten
// nacheinander geholt. Das Raster zeigt schon, was da ist.
export function Serien() {
  const s = useSeiten(500)
  useEffect(() => {
    if (!s.fertig && !s.fehler) s.mehr()
  }, [s.items.length, s.fertig])
  const liste = serien(s.items)
  const da = liste.length > 0
  useEffect(() => {
    if (da) fokusStart('serien-' + liste[0].name)
  }, [da])

  return (
    <Seite bereich="serien">
      <div class="seitenkopf rand">
        <h1>
          {t('nav.serien')}
          {s.fertig && <small>{' · ' + liste.length}</small>}
        </h1>
      </div>
      {s.fehler && !da ? (
        <Fehler fehler={s.fehler} nochmal={s.mehr} />
      ) : s.fertig && !da ? (
        <Leer titel={t('leer.serien')} />
      ) : (
        <Raster fokusKey="raster-serien">
          {liste.map((x) => (
            <SerienKarte key={x.name} s={x} reihe="serien" />
          ))}
          {!s.fertig && <SkeletonKarten n={da ? 4 : 12} />}
        </Raster>
      )}
    </Seite>
  )
}

