// Suche (/suche/<begriff>), Entwurf Suche und Handy-Suche. Der Begriff steht in der Route (useRoute()[1]),
// die Kopfzeile öffnet sie mit Enter. Gesucht wird im Client über die geladene Bibliothek: sofort beim Tippen,
// mit Tippfehler-Toleranz (Trigramme, „Meintest du …?“). Personen fehlen: dafür gibt es keinen Suchendpunkt.
// TV: Bildschirmtastatur links, Treffer rechts; Zurück löscht ein Zeichen.
import { useEffect, useState } from 'preact/hooks'
import { Chip } from '../components/Button'
import { Icon } from '../components/Icon'
import { Karte, tonFuer } from '../components/Karte'
import { Raster } from '../components/Reihe'
import { Seite } from '../components/Seite'
import { SkeletonKarten } from '../components/Skeleton'
import { SerienKarte, TitelKarte } from '../components/TitelKarte'
import { Fehler, Leer } from '../components/Zustand'
import { anzeigeTitel, bibliothek, sammlungenApi, serien, useDaten, useGecacht, type Item, type Sammlung, type Serie } from '../lib/api'
import { istTV } from '../lib/device'
import { Gruppe, fokusBald, useFokus } from '../lib/focus'
import { ergaenze, t } from '../lib/i18n'
import { go, pfad, useRoute, useZurueck } from '../lib/router'
import { Tastatur } from './SucheTastatur'
import './browse.css'

ergaenze(
  {
    'suche.feld': 'Filme, Serien, Folgen …',
    'suche.alles': 'Alles',
    'suche.filme': 'Filme',
    'suche.serien': 'Serien',
    'suche.folgen': 'Folgen',
    'suche.sammlungen': 'Sammlungen',
    'suche.direkt': 'Direkt abspielbar',
    'suche.treffer': '{n} Treffer für „{q}“',
    'suche.meintest': 'Meintest du „{titel}“? Keine Treffer für „{q}“. Ähnlich:',
    'suche.nichts': 'Nichts gefunden für „{q}“.',
    'suche.leer': 'Tippe los – Treffer erscheinen schon beim Tippen.',
    'suche.tv.hinweis': 'Zurück löscht ein Zeichen · → springt zu den Treffern',
    'suche.zuletzt': 'Zuletzt gesucht',
    'suche.verlauf.weg': 'Löschen',
  },
  {
    'suche.feld': 'Movies, shows, episodes …',
    'suche.alles': 'All',
    'suche.filme': 'Movies',
    'suche.serien': 'Shows',
    'suche.folgen': 'Episodes',
    'suche.sammlungen': 'Collections',
    'suche.direkt': 'Plays directly',
    'suche.treffer': '{n} results for “{q}”',
    'suche.meintest': 'Did you mean “{titel}”? No results for “{q}”. Similar:',
    'suche.nichts': 'Nothing found for “{q}”.',
    'suche.leer': 'Start typing – results appear as you type.',
    'suche.tv.hinweis': 'Back deletes a character · → jumps to the results',
    'suche.zuletzt': 'Recent searches',
    'suche.verlauf.weg': 'Clear',
  },
)

type Filter = 'alles' | 'filme' | 'serien' | 'folgen' | 'direkt'

interface Eintrag {
  titel: string
  text: string // normalisiert: Titel, Originaltitel, Jahr, Genres (Folgen: plus Serie)
  film?: Item
  folge?: Item
  serie?: Serie
}

// Kleinbuchstaben, Umlaute gefaltet, Satzzeichen weg: „Der Herr der Ringe: Die Gefährten“ → „der herr der ringe die gefahrten“
export function norm(s: string): string {
  return s
    .toLowerCase()
    .replace(/ä/g, 'a')
    .replace(/ö/g, 'o')
    .replace(/ü/g, 'u')
    .replace(/ß/g, 'ss')
    .replace(/[^a-z0-9]+/g, ' ')
    .trim()
}

function trigramme(s: string): string[] {
  const x = '  ' + s + ' '
  const out: string[] = []
  for (let i = 0; i < x.length - 2; i++) out.push(x.slice(i, i + 3))
  return out
}

// Dice-Koeffizient über Trigramme, 0 … 1.
export function aehnlich(a: string, b: string): number {
  const ta = trigramme(a)
  const tb = trigramme(b)
  if (!ta.length || !tb.length) return 0
  const rest = tb.slice()
  let gleich = 0
  for (const g of ta) {
    const i = rest.indexOf(g)
    if (i >= 0) {
      gleich++
      rest.splice(i, 1)
    }
  }
  return (2 * gleich) / (ta.length + tb.length)
}

function eintraege(alle: Item[]): Eintrag[] {
  const out: Eintrag[] = []
  const text = (it: Item, titel: string) => norm([titel, it.title, it.meta && it.meta.originalTitle, it.year, it.meta && it.meta.year, ((it.meta && it.meta.genres) || []).join(' ')].filter(Boolean).join(' '))
  for (const it of alle) {
    if (!it.series) out.push({ titel: anzeigeTitel(it), text: text(it, anzeigeTitel(it)), film: it })
    else out.push({ titel: anzeigeTitel(it), text: norm(anzeigeTitel(it) + ' ' + it.series), folge: it })
  }
  for (const s of serien(alle)) out.push({ titel: s.name, text: text(s.erste, s.name), serie: s })
  return out
}

// Treffer: jedes Wort der Anfrage kommt vor; Titelanfang vor Titelmitte vor Rest. Sonst ähnliche Titel.
export function suchen(liste: Eintrag[], q: string): { treffer: Eintrag[]; aehnlich: boolean } {
  const w = norm(q).split(' ').filter(Boolean)
  if (!w.length) return { treffer: [], aehnlich: false }
  const rang = (e: Eintrag) => {
    const tt = norm(e.titel)
    return tt.indexOf(w.join(' ')) === 0 ? 0 : tt.indexOf(w[0]) >= 0 ? 1 : 2
  }
  const treffer = liste.filter((e) => w.every((x) => e.text.indexOf(x) >= 0)).sort((a, b) => rang(a) - rang(b) || a.titel.localeCompare(b.titel))
  if (treffer.length) return { treffer, aehnlich: false }
  const nq = norm(q)
  const bewertet = liste.filter((e) => !e.folge).map((e) => ({ e, s: aehnlich(nq, norm(e.titel)) })).filter((x) => x.s > 0.25)
  bewertet.sort((a, b) => b.s - a.s)
  return { treffer: bewertet.slice(0, 12).map((x) => x.e), aehnlich: true }
}

// ---------- Verlauf „Zuletzt gesucht“ (localStorage) ----------
function verlauf(neu?: string[]): string[] {
  try {
    if (neu) localStorage.setItem('flimmer.suchverlauf', JSON.stringify(neu))
    return neu || JSON.parse(localStorage.getItem('flimmer.suchverlauf') || '[]')
  } catch {
    return neu || []
  }
}

function VerlaufPunkt(p: { q: string; i: number; onPress: () => void }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: 'verlauf-' + p.i, onPress: p.onPress })
  return (
    <button ref={f.ref} {...f.dom} type="button" class={'verlauf-punkt' + (f.fokus ? ' ist-fokus' : '')}>
      <Icon name="uhr" />
      <span class="eine-zeile">{p.q}</span>
    </button>
  )
}

function Abschnitt(p: { titel: string; n: number; children: preact.ComponentChildren }) {
  return (
    <section class="abschnitt">
      <h2 class="abschnitt-titel">
        {p.titel}
        <small>{p.n}</small>
      </h2>
      {p.children}
    </section>
  )
}

export function Suche() {
  const r = useRoute()
  const q = r[1] || ''
  const setQ = (x: string) => go(x ? pfad('suche', x) : '/suche', { ersetzen: true })
  const alle = useDaten<Item[]>('bibliothek', () => bibliothek(), { merken: true })
  const samm = useGecacht<Sammlung[] | null>('sammlungen')
  const [filter, setFilter] = useState<Filter>('alles')
  const tv = istTV()
  const liste = alle.daten ? eintraege(alle.daten) : null
  const zuletzt = verlauf()

  useEffect(() => {
    sammlungenApi.alle().catch(() => {})
    if (tv) fokusBald('tastatur-a')
  }, [])
  // Nach kurzer Pause landet der Begriff im Verlauf.
  useEffect(() => {
    const x = q.trim()
    if (x.length < 2) return
    const h = setTimeout(() => verlauf([x].concat(verlauf().filter((y) => y !== x)).slice(0, 8)), 1500)
    return () => clearTimeout(h)
  }, [q])
  // TV: Zurück löscht erst ein Zeichen, dann geht es wie gewohnt zurück.
  useZurueck(() => {
    if (!q) return false
    setQ(q.slice(0, -1))
    return true
  }, tv)

  const erg = liste ? suchen(liste, q) : { treffer: [], aehnlich: false }
  const direkt = (e: Eintrag) => (e.film || e.folge || (e.serie && e.serie.erste))!.light === 'green'
  const passt = (e: Eintrag) => filter !== 'direkt' || direkt(e)
  const treffer = erg.treffer.filter(passt)
  const filme = filter === 'alles' || filter === 'filme' || filter === 'direkt' ? treffer.filter((e) => e.film) : []
  const reihen = filter === 'alles' || filter === 'serien' || filter === 'direkt' ? treffer.filter((e) => e.serie) : []
  const folgen = filter === 'alles' || filter === 'folgen' || filter === 'direkt' ? treffer.filter((e) => e.folge) : []
  const nq = norm(q)
  const sammlungen = samm && nq && filter === 'alles' ? samm.filter((s) => norm(s.name).indexOf(nq) >= 0) : []
  const n = filme.length + reihen.length + folgen.length + sammlungen.length

  const ergebnis = alle.fehler && !liste ? (
    <Fehler fehler={alle.fehler} nochmal={alle.neu} />
  ) : !liste ? (
    <div class="raster">
      <SkeletonKarten n={tv ? 6 : 8} />
    </div>
  ) : !q.trim() ? (
    <p class="t-text leise su-info">{t('suche.leer')}</p>
  ) : !n ? (
    <Leer titel={t('suche.nichts', { q })} />
  ) : (
    <>
      <p class="t-text leise su-info">{erg.aehnlich ? t('suche.meintest', { titel: treffer[0].titel, q }) : t('suche.treffer', { n, q })}</p>
      {filme.length > 0 && (
        <Abschnitt titel={t('suche.filme')} n={filme.length}>
          <Raster fokusKey="su-filme">
            {filme.slice(0, 40).map((e) => (
              <TitelKarte key={e.film!.id} it={e.film!} reihe="su-f" />
            ))}
          </Raster>
        </Abschnitt>
      )}
      {reihen.length > 0 && (
        <Abschnitt titel={t('suche.serien')} n={reihen.length}>
          <Raster fokusKey="su-serien">
            {reihen.slice(0, 40).map((e) => (
              <SerienKarte key={e.serie!.name} s={e.serie!} reihe="su-s" />
            ))}
          </Raster>
        </Abschnitt>
      )}
      {folgen.length > 0 && (
        <Abschnitt titel={t('suche.folgen')} n={folgen.length}>
          <Raster fokusKey="su-folgen">
            {folgen.slice(0, 24).map((e) => (
              <TitelKarte key={e.folge!.id} it={e.folge!} reihe="su-e" breit />
            ))}
          </Raster>
        </Abschnitt>
      )}
      {sammlungen.length > 0 && (
        <Abschnitt titel={t('suche.sammlungen')} n={sammlungen.length}>
          <Raster fokusKey="su-samm">
            {sammlungen.map((s) => (
              <Karte key={s.id} fokusKey={'su-samm-' + s.id} titel={s.name} farbe={tonFuer(s.name)} onPress={() => go(pfad('sammlung', s.id))} />
            ))}
          </Raster>
        </Abschnitt>
      )}
    </>
  )

  const chips = (
    <Gruppe fokusKey="su-filter" class="su-filter" label={t('nav.suche')}>
      {(['alles', 'filme', 'serien', 'folgen', 'direkt'] as Filter[]).map((f) => (
        <Chip key={f} fokusKey={'suche-filter-' + f} an={filter === f} onPress={() => setFilter(f)}>
          {f === 'direkt' && <span class="fl-ampel gruen" />}
          {(f === 'direkt' ? ' ' : '') + t('suche.' + f)}
        </Chip>
      ))}
    </Gruppe>
  )

  const verlaufListe = zuletzt.length > 0 && (
    <aside class="su-verlauf">
      <div class="su-verlauf-kopf">
        <span class="fl-label">{t('suche.zuletzt')}</span>
        <button type="button" class="mehr-text" onClick={() => (verlauf([]), setQ(''))}>
          {t('suche.verlauf.weg')}
        </button>
      </div>
      <Gruppe fokusKey="su-verlauf">
        {zuletzt.map((x, i) => (
          <VerlaufPunkt key={x} q={x} i={i} onPress={() => setQ(x)} />
        ))}
      </Gruppe>
    </aside>
  )

  return (
    <Seite bereich="suche" titel={t('nav.suche')} class="browse su">
      <div class="su-seite rand">
        {tv ? (
          <div class="su-links">
            <div class={'fl-feld su-wert' + (q ? '' : ' leer')} aria-live="polite">
              <Icon name="suche" />
              <span class="wachse eine-zeile">{q || t('suche.feld')}</span>
            </div>
            <Tastatur wert={q} setWert={setQ} max={60} />
            <p class="t-klein leise su-hinweis">{t('suche.tv.hinweis')}</p>
          </div>
        ) : (
          <div class="su-oben">
            <label class="fl-feld su-feld">
              <Icon name="suche" />
              <input class="wachse" type="search" autoFocus placeholder={t('suche.feld')} aria-label={t('nav.suche')} value={q} onInput={(e) => setQ((e.target as HTMLInputElement).value)} />
            </label>
            {chips}
          </div>
        )}
        <div class="su-rechts">
          {tv && chips}
          {!q.trim() && !tv && verlaufListe}
          {ergebnis}
          {!q.trim() && tv && verlaufListe}
        </div>
      </div>
    </Seite>
  )
}
