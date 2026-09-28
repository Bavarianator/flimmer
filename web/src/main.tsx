import { render } from 'preact'
import { useEffect, useState } from 'preact/hooks'
import { init } from '@noriginmedia/norigin-spatial-navigation'
import { Home } from './Home'
import { Detail, SeriesPage } from './Detail'
import { Login, PairConfirm } from './Login'
import { Player } from './player/Player'
import { LoginError, library } from './data'
import { back, current, go } from './route'
import './style.css'

init({ throttle: 100 })

// Die D-Pad-Navigation (Listener auf window) blockiert Enter/←/→ auch in Eingabefeldern – selbst pausiert.
// Hier auf document abfangen, damit Formulare abschicken und der Cursor im Text wandern kann.
document.addEventListener('keydown', (e) => {
  const t = e.target as HTMLElement
  if ((t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.tagName === 'SELECT') && [13, 37, 39].indexOf(e.keyCode) >= 0) e.stopPropagation()
})

function App() {
  const [path, setPath] = useState(current())
  const [needLogin, setNeedLogin] = useState(false)

  useEffect(() => {
    const on = () => setPath(current())
    window.addEventListener('hashchange', on)
    // Jede Seite lädt über library(); ein 401 dort schaltet auf die Profilauswahl.
    const onErr = (e: PromiseRejectionEvent) => {
      if (e.reason instanceof LoginError) {
        e.preventDefault()
        if (e.reason.setup) location.href = '/setup'
        else setNeedLogin(true)
      }
    }
    window.addEventListener('unhandledrejection', onErr)
    library().catch((e) => {
      if (e instanceof LoginError) {
        if (e.setup) location.href = '/setup'
        else setNeedLogin(true)
      }
    })
    return () => {
      window.removeEventListener('hashchange', on)
      window.removeEventListener('unhandledrejection', onErr)
    }
  }, [])

  if (needLogin)
    return (
      <Login
        onDone={() => {
          setNeedLogin(false)
          library(true)
          go('/')
        }}
      />
    )
  switch (path[0]) {
    case 'watch':
      return <Player key={path[1]} id={path[1]} start={path[2] ? Number(path[2]) : undefined} onBack={back} />
    case 'item':
      return <Detail key={path[1]} id={path[1]} />
    case 'series':
      return <SeriesPage key={path[1]} name={path[1]} />
    case 'pair':
      return <PairConfirm />
  }
  return <Home />
}

render(<App />, document.getElementById('app')!)
