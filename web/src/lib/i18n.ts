// Alle Texte der Oberfläche. Deutsch ist Standard, Englisch die zweite Sprache.
// t('schluessel', { name: 'x' }) ersetzt {name}. Fehlt ein Schlüssel im Englischen, gilt Deutsch.

const de: Record<string, string> = {
  'nav.start': 'Start',
  'nav.filme': 'Filme',
  'nav.serien': 'Serien',
  'nav.suche': 'Suche',
  'nav.einstellungen': 'Einstellungen',
  'nav.koppeln': 'Fernseher koppeln',
  'nav.hochladen': 'Videos hochladen',
  'nav.profil': 'Profil wechseln',
  'nav.geraetetest': 'Gerät neu testen',
  'nav.geraetetest.laeuft': 'Gerät wird getestet …',
  'nav.menue': 'Menü',

  'reihe.continue': 'Weiterschauen',
  'reihe.nextup': 'Als Nächstes',
  'reihe.recent': 'Neu hinzugefügt',
  'reihe.filme': 'Filme',
  'reihe.serien': 'Serien',

  'knopf.abspielen': 'Abspielen',
  'knopf.fortsetzen': 'Fortsetzen · {zeit}',
  'knopf.vonvorn': 'Von vorn',
  'knopf.details': 'Details',
  'knopf.allefolgen': 'Alle Folgen',
  'knopf.nochmal': 'Nochmal versuchen',
  'knopf.zurueck': 'Zurück',
  'knopf.mehr': 'Mehr laden',
  'knopf.gesehen': 'Als gesehen markieren',
  'knopf.ungesehen': 'Als ungesehen markieren',

  'ampel.green': 'Läuft direkt – ohne Umwandlung',
  'ampel.yellow': 'Läuft flüssig – der Server wandelt einen Teil um',
  'ampel.red': 'Muss umgewandelt werden – dieser Server ist dafür knapp',
  'ampel.kurz.green': 'läuft direkt',
  'ampel.kurz.yellow': 'wandelt teilweise um',
  'ampel.kurz.red': 'Server knapp',
  'ampel.legende': 'Geprüft für dieses Gerät',

  'karte.gesehen': 'Gesehen',
  'karte.film': 'Film',
  'karte.serie': 'Serie',

  'zeit.std': '{h} Std.',
  'zeit.min': '{m} Min.',
  'zeit.noch': 'Noch {dauer}',
  'min.kurz': 'Min.',
  'zeit.endet': 'endet um {uhr}',
  'folge.kurz': 'S{s} E{e}',
  'staffel': 'Staffel {n}',
  'staffel.specials': 'Specials',
  'staffeln': '{n} Staffeln',
  'staffeln.1': '1 Staffel',
  'folgen': '{n} Folgen',
  'folgen.1': '1 Folge',
  'als.naechstes': 'Als Nächstes: {folge}',
  'bewertung': '★ {n}',
  'titel.anzahl': '{n} Titel',
  'filter.neu': 'Neu hinzugefügt',
  'filter.ungesehen': 'Ungesehen',
  'filter.direkt': 'Direkt abspielbar',
  'filter.passen': '{n} passen',

  'leer.bibliothek.titel': 'Noch nichts zu sehen',
  'leer.bibliothek.text': 'Flimmer hat noch keine Filme gefunden. Füge in den Einstellungen einen Medienordner hinzu.',
  'leer.filme': 'Keine Filme in der Bibliothek.',
  'leer.serien': 'Keine Serien in der Bibliothek.',
  'leer.unbekannt.titel': 'Titel nicht gefunden',
  'leer.unbekannt.text': 'Dieser Titel ist nicht mehr in der Bibliothek. Vielleicht wurde die Datei verschoben.',
  'fehler.titel': 'Der Server antwortet nicht',
  'fehler.text': 'Läuft der Rechner mit Flimmer, und ist dieses Gerät im selben Netz? ({grund})',
  'laden': 'Lade …',
  'platzhalter': 'Dieser Bereich wird gerade neu gebaut.',
}

// Schlüssel sind freie Strings, damit andere Module eigene Texte per ergaenze() anmelden können.
type Schluessel = string

const en: Record<string, string> = {
  'nav.start': 'Home',
  'nav.filme': 'Movies',
  'nav.serien': 'Shows',
  'nav.suche': 'Search',
  'nav.einstellungen': 'Settings',
  'nav.koppeln': 'Pair TV',
  'nav.hochladen': 'Upload videos',
  'nav.profil': 'Switch profile',
  'nav.geraetetest': 'Test device again',
  'nav.geraetetest.laeuft': 'Testing device …',
  'nav.menue': 'Menu',
  'reihe.continue': 'Continue watching',
  'reihe.nextup': 'Up next',
  'reihe.recent': 'Recently added',
  'reihe.filme': 'Movies',
  'reihe.serien': 'Shows',
  'knopf.abspielen': 'Play',
  'knopf.fortsetzen': 'Resume · {zeit}',
  'knopf.vonvorn': 'From the start',
  'knopf.details': 'Details',
  'knopf.allefolgen': 'All episodes',
  'knopf.nochmal': 'Try again',
  'knopf.zurueck': 'Back',
  'knopf.mehr': 'Load more',
  'knopf.gesehen': 'Mark as watched',
  'knopf.ungesehen': 'Mark as unwatched',
  'ampel.green': 'Plays directly – no conversion',
  'ampel.yellow': 'Plays smoothly – the server converts part of it',
  'ampel.red': 'Needs conversion – this server is short on power',
  'ampel.kurz.green': 'plays directly',
  'ampel.kurz.yellow': 'partly converted',
  'ampel.kurz.red': 'server busy',
  'ampel.legende': 'Checked for this device',
  'karte.gesehen': 'Watched',
  'karte.film': 'Movie',
  'karte.serie': 'Show',
  'zeit.std': '{h} hr',
  'zeit.min': '{m} min',
  'zeit.noch': '{dauer} left',
  'min.kurz': 'min',
  'zeit.endet': 'ends at {uhr}',
  'staffel': 'Season {n}',
  'staffeln': '{n} seasons',
  'staffeln.1': '1 season',
  'folgen': '{n} episodes',
  'folgen.1': '1 episode',
  'als.naechstes': 'Up next: {folge}',
  'titel.anzahl': '{n} titles',
  'filter.neu': 'Recently added',
  'filter.ungesehen': 'Unwatched',
  'filter.direkt': 'Plays directly',
  'filter.passen': '{n} match',
  'leer.bibliothek.titel': 'Nothing to watch yet',
  'leer.bibliothek.text': 'Flimmer has not found any movies. Add a media folder in the settings.',
  'leer.filme': 'No movies in the library.',
  'leer.serien': 'No shows in the library.',
  'leer.unbekannt.titel': 'Title not found',
  'leer.unbekannt.text': 'This title is no longer in the library. The file may have moved.',
  'fehler.titel': 'The server is not responding',
  'fehler.text': 'Is the computer running Flimmer, and is this device on the same network? ({grund})',
  'laden': 'Loading …',
  'platzhalter': 'This area is being rebuilt.',
}

// Eigene Texte eines Moduls (z. B. Player, Einstellungen) anmelden, ohne diese Datei zu ändern.
export function ergaenze(d: Record<string, string>, e?: Record<string, string>) {
  for (const k in d) de[k] = d[k]
  if (e) for (const k in e) en[k] = e[k]
}

function gespeichert(): string {
  try {
    return localStorage.getItem('flimmer.sprache') || ''
  } catch {
    return ''
  }
}

export let sprache: 'de' | 'en' = (gespeichert() || navigator.language || 'de').slice(0, 2) === 'en' ? 'en' : 'de'

export function setSprache(s: 'de' | 'en') {
  try {
    localStorage.setItem('flimmer.sprache', s)
  } catch {}
  location.reload() // ponytail: Neuladen statt reaktivem Umschalten, reicht für eine seltene Einstellung
}

export function t(k: Schluessel, v?: Record<string, string | number>): string {
  let s = (sprache === 'en' && en[k]) || de[k] || k
  if (v) for (const n in v) s = s.split('{' + n + '}').join(String(v[n]))
  return s
}

// „2 Std. 29 Min.“ bzw. „34 Min.“
export function dauer(sec: number): string {
  const h = Math.floor(sec / 3600)
  const m = Math.round((sec % 3600) / 60)
  if (!h) return t('zeit.min', { m: Math.max(1, m) })
  return t('zeit.std', { h }) + (m ? ' ' + t('zeit.min', { m }) : '')
}

// „1:12:04“ bzw. „4:05“
export function uhr(sec: number): string {
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  const zwei = (n: number) => (n < 10 ? '0' : '') + n
  return (h ? h + ':' + zwei(m) : String(m)) + ':' + zwei(s)
}

export function folge(s?: number, e?: number): string {
  return t('folge.kurz', { s: s || 0, e: e || 0 })
}

export function anzahl(n: number, eins: Schluessel, viele: Schluessel): string {
  return n === 1 ? t(eins) : t(viele, { n })
}
