// Profil wechseln (Route /profile): dieselbe Profilauswahl wie beim Anmelden, mit Zurück.
// Nach dem Wechsel fliegen die gemerkten Daten des alten Profils raus, dann lädt die App neu.
import { Login } from './Login'
import { back } from '../lib/router'

function frisch() {
  try {
    for (const k of ['bibliothek', 'start', 'ich']) localStorage.removeItem('flimmer.cache.' + k)
  } catch {}
  location.replace(location.pathname + '#/')
  location.reload()
}

export function Profile() {
  return <Login onDone={frisch} zurueck={back} />
}
