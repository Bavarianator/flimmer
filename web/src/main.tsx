import { render } from 'preact'
import { useEffect, useState } from 'preact/hooks'
import { init } from '@noriginmedia/norigin-spatial-navigation'
import { Library } from './Library'
import { Player } from './player/Player'
import './style.css'

init({ throttle: 100 })

function route() {
  const m = location.hash.match(/^#\/watch\/(\w+)/)
  return m ? m[1] : ''
}

function App() {
  const [watching, setWatching] = useState(route())
  useEffect(() => {
    const on = () => setWatching(route())
    window.addEventListener('hashchange', on)
    return () => window.removeEventListener('hashchange', on)
  }, [])
  if (watching) return <Player id={watching} onBack={() => (location.hash = '#/')} />
  return <Library onOpen={(id) => (location.hash = '#/watch/' + id)} />
}

render(<App />, document.getElementById('app')!)
