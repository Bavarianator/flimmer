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
// Profilauswahl/PIN laden ebenfalls nach: so bleibt player/screens.css aus dem Start-CSS heraus.
const Login = spaeter(() => import('./screens/Login').then((m) => m.Login))
const Einladung = spaeter(() => import('./screens/Login').then((m) => m.Einladung))
const Profile = spaeter(() => import('./screens/Profile').then((m) => m.Profile))
const Setup = spaeter(() => import('./screens/Setup').then((m) => m.Setup))
// Jellyfin-Umbau (docs/umbau-jellyfin.md)
const Favoriten = spaeter(() => import('./screens/Favoriten').then((m) => m.Favoriten))
const Staffel = spaeter(() => import('./screens/Staffel').then((m) => m.Staffel))
const Person = spaeter(() => import('./screens/Person').then((m) => m.Person))
const Sammlungen = spaeter(() => import('./screens/Sammlungen').then((m) => m.Sammlungen))
const Sammlung = spaeter(() => import('./screens/Sammlungen').then((m) => m.Sammlung))
const Listen = spaeter(() => import('./screens/Listen').then((m) => m.Listen))
const Liste = spaeter(() => import('./screens/Listen').then((m) => m.Liste))
const LiveTV = spaeter(() => import('./screens/LiveTV').then((m) => m.LiveTV))
const Dashboard = spaeter(() => import('./screens/admin/Dashboard').then((m) => m.Dashboard))
const Hochladen = spaeter(() => import('./screens/Hochladen').then((m) => m.Hochladen))

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
  // Unterseiten (Tabs, Einstellungs- und Dashboard-Bereiche) liest jede Seite selbst aus useRoute().
  switch (r[0]) {
    case 'favoriten':
      return <Favoriten />
    case 'filme':
      return <Filme />
    case 'serien':
      return <Serien />
    case 'film':
    case 'item':
      return <DetailFilm key={r[1]} id={r[1]} />
    case 'serie':
    case 'series':
      if (r[2]) return <Staffel key={r[1] + '/' + r[2]} name={r[1]} staffel={r[2]} />
      return <DetailSerie key={r[1]} name={r[1]} />
    case 'person':
      return <Person key={r[1]} name={r[1]} />
    case 'sammlungen':
      return <Sammlungen />
    case 'sammlung':
      return <Sammlung key={r[1]} id={r[1]} />
    case 'listen':
      return <Listen />
    case 'liste':
      return <Liste key={r[1]} id={r[1]} />
    case 'livetv':
      return <LiveTV />
    case 'dashboard':
      return <Dashboard />
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
    case 'hochladen':
      return <Hochladen key={r[1]} link={r[1]} name={r[2]} />
  }
  return <Start />
}

// Einladungslinks: /einladung#<token> – das Fragment ist der Token, kein Hash-Pfad.
render(location.pathname === '/einladung' ? <Einladung /> : <App />, document.getElementById('app')!)
