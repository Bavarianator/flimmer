import { render, type ComponentType } from 'preact'
import { useEffect, useState } from 'preact/hooks'
import './design/tokens.css'
import './design/base.css'
import './design/komponenten.css'
import './components/components.css'
import { setLoginHandler } from './lib/api'
import './lib/device' // setzt html.tv/dt/hd und das Thema vor dem ersten Rendern
import { starteFokus } from './lib/focus'
import { go, useRoute } from './lib/router'
import { Start } from './screens/Start'
import { Filme } from './screens/Filme'
import { Serien } from './screens/Serien'
import { DetailFilm } from './screens/DetailFilm'
import { DetailSerie } from './screens/DetailSerie'
import { Login } from './screens/Login'
import { Profile } from './screens/Profile'

starteFokus()

// Seltene Seiten werden erst bei Bedarf geladen (Initial-JS unter 80 KB gzip).
function spaeter<P>(laden: () => Promise<ComponentType<P>>): ComponentType<P> {
  let K: ComponentType<P> | null = null
  return (p: P) => {
    const [, neu] = useState(0)
    useEffect(() => {
      if (!K) laden().then((k) => ((K = k), neu(1)))
    }, [])
    return K ? <K {...(p as P & object)} /> : null
  }
}
const Player = spaeter(() => import('./player/Player').then((m) => m.Player))
const Suche = spaeter(() => import('./screens/Suche').then((m) => m.Suche))
const Einstellungen = spaeter(() => import('./screens/Einstellungen').then((m) => m.Einstellungen))
const Kopplung = spaeter(() => import('./screens/Kopplung').then((m) => m.Kopplung))
const Party = spaeter(() => import('./screens/Party').then((m) => m.Party))
const Setup = spaeter(() => import('./screens/Setup').then((m) => m.Setup))

function App() {
  const r = useRoute()
  const [login, setLogin] = useState(false)
  useEffect(() => {
    // Jede Anfrage mit 401 schaltet auf die Profilauswahl (bzw. die Einrichtung, wenn noch niemand da ist).
    setLoginHandler((setup) => {
      if (setup) go('/setup', { ersetzen: true })
      else setLogin(true)
    })
  }, [])

  if (login && r[0] !== 'setup')
    return (
      <Login
        onDone={() => {
          setLogin(false)
          location.reload() // ponytail: frischer Start nach dem Login, statt alle Caches einzeln zu leeren
        }}
      />
    )
  // Alte Pfade (item, series, pair) bleiben gültig, damit Lesezeichen und Einladungen weiter gehen.
  switch (r[0]) {
    case 'filme':
      return <Filme />
    case 'serien':
      return <Serien />
    case 'film':
    case 'item':
      return <DetailFilm key={r[1]} id={r[1]} />
    case 'serie':
    case 'series':
      return <DetailSerie key={r[1]} name={r[1]} />
    case 'watch':
      return <Player key={r[1] + '/' + r[2]} id={r[1]} start={r[2] ? Number(r[2]) : undefined} />
    case 'suche':
      return <Suche />
    case 'einstellungen':
      return <Einstellungen />
    case 'koppeln':
    case 'pair':
      return <Kopplung />
    case 'profile':
      return <Profile />
    case 'party':
      return <Party key={r[1]} id={r[1]} />
    case 'setup':
      return <Setup />
  }
  return <Start />
}

render(<App />, document.getElementById('app')!)
