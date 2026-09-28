import { useEffect, useState } from 'preact/hooks'
import { FocusContext, useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation'
import { profile, setProbe } from './profile'
import { loadProbe, runProbe } from './probe'
import { displayTitle, groupSeries, home, image, library, nextUp, resumePos, type HomeRow, type Item } from './data'
import { Card, Row } from './ui'
import { go } from './route'

function sub(it: Item) {
  if (it.series) return `S${it.season} E${it.episode} · ${displayTitle(it)}`
  const y = (it.meta && it.meta.year) || it.year
  return [y, it.duration > 0 ? Math.round(it.duration / 60) + ' min' : ''].filter(Boolean).join(' · ')
}

export function ItemCard({ it, row, wide }: { it: Item; row: string; wide?: boolean }) {
  const pos = resumePos(it.id)
  return (
    <Card
      focusKey={row + '-' + it.id}
      wide={wide}
      img={image(it, wide ? 'backdrop' : 'poster', wide ? 480 : 300)}
      name={it.series || displayTitle(it)}
      title={it.series || displayTitle(it)}
      sub={sub(it)}
      light={it.light}
      progress={it.duration ? pos / it.duration : 0}
      onOpen={() => go(it.series ? '/series/' + encodeURIComponent(it.series) : '/item/' + it.id)}
    />
  )
}

export function Home() {
  const [items, setItems] = useState<Item[] | null>(null)
  const [rows, setRows] = useState<HomeRow[]>([])
  const [error, setError] = useState('')
  const [probing, setProbing] = useState(false)
  const { ref, focusKey } = useFocusable({ focusKey: 'home' })

  const load = (reload: boolean) =>
    library(reload)
      .then((all) => {
        setItems(all)
        return home().then(setRows)
      })
      .catch((e) => setError(String(e.message || e)))
  const probe = () => {
    setProbing(true)
    runProbe().then((r) => {
      setProbing(false)
      if (r) {
        setProbe(r)
        load(true) // Ampeln mit gemessenem Profil neu berechnen
      }
    })
  }
  useEffect(() => {
    load(false)
    if (!loadProbe()) probe() // einmal pro Gerät/Firmware im Hintergrund
  }, [])
  useEffect(() => {
    if (items && items.length) setFocus('home')
  }, [items])

  const empty = error
    ? 'Server nicht erreichbar: ' + error
    : !items
      ? 'Lade Bibliothek …'
      : !items.length
        ? 'Noch keine Filme gefunden. In den Einstellungen einen Medienordner hinzufügen.'
        : ''
  const movies = (items || []).filter((i) => !i.series)
  const series = groupSeries(items || [])
  return (
    <FocusContext.Provider value={focusKey}>
      <main ref={ref} class="page">
        {empty && <p class="empty">{empty}</p>}
        <header>
          <h1>Flimmer</h1>
          <span class="device">
            {profile.name}
            {' · '}
            <button class="link" onClick={probe} disabled={probing}>
              {probing ? 'Gerät wird getestet …' : 'Gerät neu testen'}
            </button>
          </span>
        </header>
        {rows.map((r) => (
          <Row key={r.id} title={r.title}>
            {r.items.map((it) => <ItemCard key={it.id} it={it} row={r.id} wide />)}
          </Row>
        ))}
        {movies.length > 0 && (
          <Row title="Filme">
            {movies.map((it) => <ItemCard key={it.id} it={it} row="movies" />)}
          </Row>
        )}
        {series.length > 0 && (
          <Row title="Serien">
            {series.map((s) => {
              const n = nextUp(s)
              const count = s.seasons.reduce((a, x) => a + x.episodes.length, 0)
              return (
                <Card
                  key={s.name}
                  focusKey={'series-' + s.name}
                  img={image(s.first, 'poster', 300)}
                  name={s.name}
                  title={s.name}
                  sub={`${s.seasons.length} ${s.seasons.length === 1 ? 'Staffel' : 'Staffeln'} · ${count} ${count === 1 ? 'Folge' : 'Folgen'}`}
                  light={n.light}
                  onOpen={() => go('/series/' + encodeURIComponent(s.name))}
                />
              )
            })}
          </Row>
        )}
        {!empty && <footer>
          <span class="light green" /> läuft direkt <span class="light yellow" /> Server wandelt teilweise um{' '}
          <span class="light red" /> Server ist knapp – geprüft für dieses Gerät
        </footer>}
      </main>
    </FocusContext.Provider>
  )
}
