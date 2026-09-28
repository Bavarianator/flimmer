import { useEffect, useState } from 'preact/hooks'
import { FocusContext, useFocusable, setFocus } from '@noriginmedia/norigin-spatial-navigation'
import { api, profile, setProbe } from './profile'
import { loadProbe, runProbe } from './probe'

export interface Item {
  id: string
  title: string
  year?: number
  series?: string
  season?: number
  episode?: number
  duration: number
  light: 'green' | 'yellow' | 'red'
  method: string
}

const lightText = {
  green: 'Läuft direkt',
  yellow: 'Ton wird umgewandelt – läuft flüssig',
  red: 'Muss transkodiert werden',
}

function hue(s: string) {
  let h = 0
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) % 360
  return h
}

function Card({ item, onOpen }: { item: Item; onOpen: (id: string) => void }) {
  const name = item.series || item.title
  const { ref, focused } = useFocusable({ onEnterPress: () => onOpen(item.id), focusKey: 'item-' + item.id })
  const h = hue(name)
  return (
    <button
      ref={ref}
      class={'card' + (focused ? ' focused' : '')}
      onClick={() => onOpen(item.id)}
      title={lightText[item.light]}
    >
      {/* ponytail: generiertes Cover; TMDB-Poster kommen mit internal/meta */}
      <div class="poster" style={{ background: `linear-gradient(160deg, hsl(${h},55%,38%), hsl(${(h + 60) % 360},60%,16%))` }}>
        <span class="initial">{name.charAt(0)}</span>
        <span class={'light ' + item.light} />
      </div>
      <div class="meta">
        <strong>{name}</strong>
        <small>
          {item.series ? `S${item.season} E${item.episode} · ${item.title}` : item.year || ''}
          {item.duration > 0 && ` · ${Math.round(item.duration / 60)} min`}
        </small>
      </div>
    </button>
  )
}

function Row({ title, items, onOpen }: { title: string; items: Item[]; onOpen: (id: string) => void }) {
  const { ref, focusKey } = useFocusable({ focusKey: 'row-' + title })
  return (
    <FocusContext.Provider value={focusKey}>
      <section ref={ref}>
        <h2>{title}</h2>
        <div class="cards">{items.map((it) => <Card key={it.id} item={it} onOpen={onOpen} />)}</div>
      </section>
    </FocusContext.Provider>
  )
}

export function Library({ onOpen }: { onOpen: (id: string) => void }) {
  const [items, setItems] = useState<Item[] | null>(null)
  const [error, setError] = useState('')
  const { ref, focusKey } = useFocusable({ focusKey: 'library' })

  const [probing, setProbing] = useState(false)
  const load = () => api<Item[] | null>('/api/library', profile).then((r) => setItems(r || []), (e) => setError(String(e)))
  const probe = () => {
    setProbing(true)
    runProbe().then((r) => {
      setProbing(false)
      if (r) {
        setProbe(r)
        load() // Ampeln mit gemessenem Profil neu berechnen
      }
    })
  }
  useEffect(() => {
    load()
    if (!loadProbe()) probe() // einmal pro Gerät/Firmware im Hintergrund
  }, [])
  useEffect(() => {
    if (items && items.length) setFocus('item-' + items[0].id)
  }, [items])

  if (error) return <p class="empty">Server nicht erreichbar: {error}</p>
  if (!items) return <p class="empty">Lade Bibliothek …</p>
  if (!items.length) return <p class="empty">Noch keine Filme gefunden. Prüfe den Medienordner (-media).</p>

  const resume = items.filter((i) => Number(localStorage.getItem('pos:' + i.id) || 0) > 0)
  const movies = items.filter((i) => !i.series)
  const episodes = items.filter((i) => i.series)
  return (
    <FocusContext.Provider value={focusKey}>
      <main ref={ref} class="library">
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
        {resume.length > 0 && <Row title="Weiterschauen" items={resume} onOpen={onOpen} />}
        {movies.length > 0 && <Row title="Filme" items={movies} onOpen={onOpen} />}
        {episodes.length > 0 && <Row title="Serien" items={episodes} onOpen={onOpen} />}
        <footer>
          <span class="light green" /> läuft direkt <span class="light yellow" /> nur Ton wird umgewandelt{' '}
          <span class="light red" /> Transcoding nötig – geprüft für dieses Gerät
        </footer>
      </main>
    </FocusContext.Provider>
  )
}
