// Suche (screens/07), Route /suche.
// Der Server hat noch keine Suchroute: gefiltert wird im Client über die geladene Bibliothek (siehe NOTIZEN.md).
// Tippfehler: Findet die Wortsuche nichts, ranken Trigramme ähnliche Titel („Meintest du …?“).
// TV: Bildschirmtastatur links, Treffer rechts; Zurück löscht ein Zeichen, → springt zu den Treffern.
import { useEffect, useState } from 'preact/hooks'
import { anzeigeTitel, bibliothek, serien, type Item, type Serie } from '../lib/api'
import { istTV } from '../lib/device'
import { fokusBald } from '../lib/focus'
import { ergaenze, t } from '../lib/i18n'
import { useZurueck } from '../lib/router'
import { Chip } from '../components/Button'
import { Icon } from '../components/Icon'
import { Raster } from '../components/Reihe'
import { Seite } from '../components/Seite'
import { SkeletonKarten } from '../components/Skeleton'
import { SerienKarte, TitelKarte } from '../components/TitelKarte'
import { Fehler, Leer } from '../components/Zustand'
import { Tastatur } from './SucheTastatur'
import '../player/screens.css'

ergaenze(
  {
    'suche.feld': 'Titel, Genre oder Jahr',
    'suche.alles': 'Alles',
    'suche.filme': 'Filme',
    'suche.serien': 'Serien',
    'suche.direkt': 'Direkt abspielbar',
    'suche.treffer': 'Titel',
    'suche.meintest': 'Meintest du „{titel}“? Keine Treffer für „{q}“. Ähnlich:',
    'suche.nichts': 'Nichts gefunden für „{q}“.',
    'suche.leer': 'Tippe los – Treffer erscheinen schon beim Tippen.',
    'suche.tv.hinweis': 'Zurück löscht ein Zeichen · → springt zu den Treffern',
  },
  {
    'suche.feld': 'Title, genre or year',
    'suche.alles': 'All',
    'suche.filme': 'Movies',
    'suche.serien': 'Shows',
    'suche.direkt': 'Plays directly',
    'suche.treffer': 'Titles',
    'suche.meintest': 'Did you mean “{titel}”? No results for “{q}”. Similar:',
    'suche.nichts': 'Nothing found for “{q}”.',
    'suche.leer': 'Start typing – results appear as you type.',
    'suche.tv.hinweis': 'Back deletes a character · → jumps to the results',
  },
)

type Filter = 'alles' | 'filme' | 'serien' | 'direkt'

interface Eintrag {
  titel: string
  text: string // normalisiert: Titel, Originaltitel, Jahr, Genres
  film?: Item
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
  for (const it of alle) if (!it.series) out.push({ titel: anzeigeTitel(it), text: text(it, anzeigeTitel(it)), film: it })
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
  const bewertet = liste.map((e) => ({ e, s: aehnlich(nq, norm(e.titel)) })).filter((x) => x.s > 0.25)
  bewertet.sort((a, b) => b.s - a.s)
  return { treffer: bewertet.slice(0, 12).map((x) => x.e), aehnlich: true }
}

function gemerkt(): string {
  try {
    return sessionStorage.getItem('flimmer.suche') || ''
  } catch {
    return ''
  }
}

export function Suche() {
  const [q, setQ] = useState(gemerkt())
  const [filter, setFilter] = useState<Filter>('alles')
  const [liste, setListe] = useState<Eintrag[] | null>(null)
  const [fehler, setFehler] = useState('')
  const [versuch, setVersuch] = useState(0)
  const tv = istTV()

  useEffect(() => {
    setFehler('')
    bibliothek().then((alle) => setListe(eintraege(alle)), (e) => setFehler(String((e && e.message) || e)))
  }, [versuch])
  useEffect(() => {
    try {
      sessionStorage.setItem('flimmer.suche', q)
    } catch {}
  }, [q])
  useEffect(() => {
    if (tv) fokusBald('tastatur-a')
  }, [])
  // TV: Zurück löscht erst ein Zeichen, dann geht es wie gewohnt zurück.
  useZurueck(() => {
    if (!q) return false
    setQ(q.slice(0, -1))
    return true
  }, tv)

  const passt = (e: Eintrag) =>
    filter === 'alles' || (filter === 'filme' && !!e.film) || (filter === 'serien' && !!e.serie) || (filter === 'direkt' && (e.film ? e.film.light === 'green' : !!e.serie && e.serie.erste.light === 'green'))
  const erg = liste ? suchen(liste, q) : { treffer: [], aehnlich: false }
  const treffer = erg.treffer.filter(passt)

  const ergebnis = fehler ? (
    <Fehler fehler={fehler} nochmal={() => setVersuch(versuch + 1)} />
  ) : !liste ? (
    <SkeletonKarten n={tv ? 6 : 8} />
  ) : !q.trim() ? (
    <p class="t-text leise suche-info">{t('suche.leer')}</p>
  ) : !treffer.length ? (
    <Leer titel={t('suche.nichts', { q })} />
  ) : (
    <>
      <p class="t-text leise suche-info">
        {erg.aehnlich ? t('suche.meintest', { titel: treffer[0].titel, q }) : t('suche.treffer') + ' · ' + treffer.length}
      </p>
      <Raster fokusKey="suche-treffer">
        {treffer.slice(0, 60).map((e) => (e.serie ? <SerienKarte key={'s' + e.titel} s={e.serie} reihe="suche" /> : <TitelKarte key={e.film!.id} it={e.film!} reihe="suche" />))}
      </Raster>
    </>
  )

  const chips = (
    <div class="zeile zeile-umbruch suche-filter" role="group" aria-label={t('nav.suche')}>
      {(['alles', 'filme', 'serien', 'direkt'] as Filter[]).map((f) => (
        <Chip key={f} fokusKey={'suche-filter-' + f} an={filter === f} onPress={() => setFilter(f)}>
          {t('suche.' + f)}
        </Chip>
      ))}
    </div>
  )

  return (
    <Seite bereich="suche">
      <div class="suche rand">
        <div class="zeile suche-seite">
          <div class="suche-links">
            {tv ? (
              <>
                <div class={"fl-feld suche-wert" + (q ? "" : " suche-wert-leer")} aria-live="polite">
                  <Icon name="suche" />
                  <span class="wachse eine-zeile">{q || t('suche.feld')}</span>
                </div>
                <Tastatur wert={q} setWert={setQ} max={60} />
                <p class="t-klein leise suche-tv-hinweis">{t('suche.tv.hinweis')}</p>
              </>
            ) : (
              <label class="fl-feld suche-feld">
                <Icon name="suche" />
                <input class="wachse" type="search" autoFocus placeholder={t('suche.feld')} aria-label={t('nav.suche')} value={q} onInput={(e) => setQ((e.target as HTMLInputElement).value)} />
              </label>
            )}
            {!tv && chips}
          </div>
          <div class="suche-treffer wachse">
            {tv && chips}
            {ergebnis}
          </div>
        </div>
      </div>
    </Seite>
  )
}
