// Filme- und Serien-Bibliothek mit Tabs (Entwurf Filme, Filme-Filter, Serien, Handy-Bibliothek, TV-Bibliothek).
// Sortieren und Filtern laufen im Client über /api/library, die Bibliothek ist klein.
// Lädt nach (screens/Filme.tsx, Serien.tsx sind nur Hüllen), damit das Start-JS klein bleibt.
import { useEffect, useState } from 'preact/hooks'
import {
  bibliothek,
  favoriten,
  fortsetzenAb,
  gecacht,
  jahr,
  anzeigeTitel,
  naechsteFolge,
  sammlungenApi,
  serien,
  useDaten,
  useGecacht,
  type Item,
  type Nutzer,
  type Sammlung,
  type Serie,
} from '../lib/api'
import { geraet } from '../lib/device'
import { Gruppe, fokusBald, fokusStart, useFokus } from '../lib/focus'
import { anzahl, ergaenze, t } from '../lib/i18n'
import { go, pfad, useRoute } from '../lib/router'
import { AuswahlLeiste, NameDialog, spieleAlle, useAuswahl, zufaellig } from './Aktionen'
import { Button, Chip } from './Button'
import { Menue, Sheet, useMenue, type MenueEintrag } from './Menue'
import { Raster, Reihe } from './Reihe'
import { Seite, type SeitenTab } from './Seite'
import { SkeletonKarten } from './Skeleton'
import { Tabs } from './Tabs'
import { SerienKarte, TitelKarte } from './TitelKarte'
import { Fehler, Leer } from './Zustand'
import '../screens/browse.css'

ergaenze({
  'bib.vorschlaege': 'Vorschläge',
  'bib.favoriten': 'Favoriten',
  'bib.sammlungen': 'Sammlungen',
  'bib.genres': 'Genres',
  'bib.studios': 'Studios',
  'bib.folgen': 'Folgen',
  'bib.bereich': '{von}–{bis} von {n}',
  'bib.von': '{n} von {alle}',
  'bib.alle.abspielen': 'Alle abspielen',
  'bib.zufall': 'Zufällig',
  'bib.sortieren': 'Sortieren',
  'bib.sortieren.nach': 'Sortieren: {feld}',
  'bib.filter': 'Filter',
  'bib.filter.n': 'Filter ({n})',
  'bib.sortfilter': 'Sortieren & Filtern',
  'bib.neue.sammlung': 'Neue Sammlung',
  'bib.auswahl': 'Mehrfachauswahl',
  'bib.seite.zurueck': 'Vorherige Seite',
  'bib.seite.weiter': 'Nächste Seite',
  'bib.seite': 'Seite {n}',
  'bib.pro.seite': '{n} pro Seite · {s} Seiten',
  'bib.springen': 'Zu {z} springen',
  'bib.mehr': 'Weitere anzeigen',
  'bib.zuruecksetzen': 'Zurücksetzen',
  'bib.anzeigen': '{n} Titel anzeigen',
  'bib.nichts': 'Keine Treffer',
  'bib.nichts.text': 'Kein Titel passt zu diesen Filtern.',
  'sort.name': 'Name',
  'sort.neu': 'Hinzugefügt',
  'sort.jahr': 'Erscheinungsjahr',
  'sort.bewertung': 'Bewertung',
  'sort.dauer': 'Laufzeit',
  'sort.auf': 'Aufsteigend',
  'sort.ab': 'Absteigend',
  'f.status': 'Status',
  'f.gespielt': 'Gespielt',
  'f.ungespielt': 'Ungespielt',
  'f.fortsetzbar': 'Fortsetzbar',
  'f.favoriten': 'Favoriten',
  'f.direkt': 'Läuft direkt auf diesem Gerät',
  'f.direkt.text': 'Nur Titel mit grüner Ampel',
  'f.direkt.kurz': 'Direkt abspielbar',
  'f.ungesehen': 'Ungesehen',
  'f.genres': 'Genres',
  'f.fsk': 'Altersfreigabe',
  'f.jahre': 'Jahre',
  'f.studios': 'Studios',
  'vor.bewertet': 'Gut bewertet',
  'vor.weil': 'Weil du „{titel}“ gesehen hast',
  'vor.kurz': 'Für zwischendurch',
  'vor.neu': 'Neu hinzugefügt',
  'vor.weiter': 'Weiterschauen',
})

export type Art = 'filme' | 'serien' | 'folgen'
type Feld = 'name' | 'neu' | 'jahr' | 'bewertung' | 'dauer'

export interface Eintrag {
  key: string // Item-ID oder "serie:<Name>"
  titel: string
  sort: string
  jahr: number
  added: string
  rating: number
  dauer: number
  genres: string[]
  studios: string[]
  age?: number | null
  it: Item // Film, Folge bzw. die nächste Folge der Serie
  serie?: Serie
  gesehen: boolean
  begonnen: boolean
}

interface Filter {
  status: string[]
  direkt: boolean
  genres: string[]
  fsk: number[]
  jahre: number[]
}

const leer = (): Filter => ({ status: [], direkt: false, genres: [], fsk: [], jahre: [] })
// Filter und Sortierung überleben den Wechsel zwischen Seiten (z. B. Genre-Link auf der Detailseite).
const filterVon: Record<Art, Filter> = { filme: leer(), serien: leer(), folgen: leer() }
const sortVon: Record<Art, { feld: Feld; ab: boolean }> = { filme: { feld: 'name', ab: false }, serien: { feld: 'name', ab: false }, folgen: { feld: 'neu', ab: true } }

// Genre-Link (Detailseite, Genres-Tab): Bibliothek mit diesem Genre öffnen.
export function zeigeGenre(art: 'filme' | 'serien', genre: string) {
  filterVon[art] = { ...leer(), genres: [genre] }
  go('/' + art)
}

// „Der blaue Engel“ steht unter B wie in Jellyfin.
function ohneArtikel(s: string) {
  return s.replace(/^(der|die|das|the|a|an|ein|eine|le|la|les)\s+/i, '').replace(/^[^\wÄÖÜäöü]+/, '')
}

function buchstabe(e: Eintrag) {
  const z = e.sort.charAt(0).toUpperCase().replace('Ä', 'A').replace('Ö', 'O').replace('Ü', 'U')
  return z >= 'A' && z <= 'Z' ? z : '#'
}

function ausItem(it: Item, art: Art): Eintrag {
  const m = it.meta || {}
  const titel = art === 'folgen' ? it.series || anzeigeTitel(it) : anzeigeTitel(it)
  const nr = (n?: number) => ('000' + (n || 0)).slice(-3)
  return {
    key: it.id,
    titel,
    sort: art === 'folgen' ? ohneArtikel(titel) + ' ' + nr(it.season) + nr(it.episode) : ohneArtikel(m.sortTitle || titel),
    jahr: jahr(it) || 0,
    added: it.added || '',
    rating: m.rating || 0,
    dauer: it.duration || 0,
    genres: m.genres || [],
    studios: m.studios || [],
    age: m.age,
    it,
    gesehen: !!it.watched,
    begonnen: !it.watched && fortsetzenAb(it.id) > 0,
  }
}

function ausSerie(s: Serie): Eintrag {
  const folgen = s.staffeln.reduce<Item[]>((a, x) => a.concat(x.folgen), [])
  const m = s.erste.meta || {}
  const jahre = folgen.map((f) => jahr(f) || 0).filter(Boolean)
  const gesehen = folgen.every((f) => !!f.watched)
  return {
    key: 'serie:' + s.name,
    titel: s.name,
    sort: ohneArtikel(s.name),
    jahr: jahre.length ? Math.min.apply(null, jahre) : 0,
    added: folgen.reduce((a, f) => (f.added && f.added > a ? f.added : a), ''),
    rating: m.rating || 0,
    dauer: folgen.reduce((a, f) => a + (f.duration || 0), 0),
    genres: m.genres || [],
    studios: m.studios || [],
    age: m.age,
    it: naechsteFolge(s),
    serie: s,
    gesehen,
    begonnen: !gesehen && folgen.some((f) => f.watched || fortsetzenAb(f.id) > 0),
  }
}

export function eintraege(alle: Item[], art: Art): Eintrag[] {
  if (art === 'serien') return serien(alle).map(ausSerie)
  return alle.filter((i) => (art === 'filme' ? !i.series : !!i.series)).map((i) => ausItem(i, art))
}

function passt(e: Eintrag, f: Filter, fav: string[]) {
  const st = f.status
  if (
    st.length &&
    !((st.indexOf('gespielt') >= 0 && e.gesehen) ||
      (st.indexOf('ungespielt') >= 0 && !e.gesehen) ||
      (st.indexOf('fortsetzbar') >= 0 && e.begonnen) ||
      (st.indexOf('favoriten') >= 0 && fav.indexOf(e.key) >= 0))
  )
    return false
  if (f.direkt && e.it.light !== 'green') return false
  if (f.genres.length && !e.genres.some((g) => f.genres.indexOf(g) >= 0)) return false
  if (f.fsk.length && (e.age == null || f.fsk.indexOf(e.age) < 0)) return false
  if (f.jahre.length && f.jahre.indexOf(Math.floor(e.jahr / 10) * 10) < 0) return false
  return true
}

function anzahlFilter(f: Filter) {
  return f.status.length + (f.direkt ? 1 : 0) + f.genres.length + f.fsk.length + f.jahre.length
}

export function sortiere(l: Eintrag[], feld: Feld, ab: boolean): Eintrag[] {
  const nachName = (a: Eintrag, b: Eintrag) => a.sort.localeCompare(b.sort, 'de')
  const cmp: Record<Feld, (a: Eintrag, b: Eintrag) => number> = {
    name: nachName,
    neu: (a, b) => (a.added < b.added ? -1 : a.added > b.added ? 1 : 0),
    jahr: (a, b) => a.jahr - b.jahr,
    bewertung: (a, b) => a.rating - b.rating,
    dauer: (a, b) => a.dauer - b.dauer,
  }
  return l.slice().sort((a, b) => (ab ? -1 : 1) * cmp[feld](a, b) || nachName(a, b))
}

// Häufigste Werte zuerst (Genres, Studios).
function haeufig(l: Eintrag[], von: (e: Eintrag) => (string | number)[]): (string | number)[] {
  const n: Record<string, number> = {}
  const wert: Record<string, string | number> = {}
  for (const e of l)
    for (const g of von(e)) {
      n[g] = (n[g] || 0) + 1
      wert[g] = g
    }
  return Object.keys(n)
    .sort((a, b) => n[b] - n[a])
    .map((k) => wert[k])
}

export function EintragKarte({ e, reihe, aw, breit, onFocus }: { e: Eintrag; reihe: string; aw?: ReturnType<typeof useAuswahl>; breit?: boolean; onFocus?: () => void }) {
  return e.serie ? <SerienKarte s={e.serie} reihe={reihe} aw={aw} onFocus={onFocus} /> : <TitelKarte it={e.it} reihe={reihe} aw={aw} breit={breit} onFocus={onFocus} />
}

function Buchstabe(p: { z: string; an: boolean; aus: boolean; onPress: () => void }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: 'abc-' + p.z, onPress: p.onPress, aus: p.aus })
  return (
    <button ref={f.ref} {...f.dom} type="button" disabled={p.aus} aria-label={t('bib.springen', { z: p.z })} class={'abc-z' + (p.an ? ' an' : '') + (f.fokus ? ' ist-fokus' : '')}>
      {p.z}
    </button>
  )
}

const ABC = '#ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('')
const PRO_SEITE = 40

// Sortieren & Filtern als Seitenblatt.
function FilterBlatt(p: { art: Art; alle: Eintrag[]; f: Filter; setF: (f: Filter) => void; sort: { feld: Feld; ab: boolean }; setSort: (s: { feld: Feld; ab: boolean }) => void; treffer: number; onZu: () => void }) {
  const [tab, setTab] = useState('filter')
  const { f, setF } = p
  const um = (k: 'status' | 'genres' | 'fsk' | 'jahre', w: string | number) => {
    const l = f[k] as (string | number)[]
    setF({ ...f, [k]: l.indexOf(w) >= 0 ? l.filter((x) => x !== w) : l.concat(w) })
  }
  const felder: Feld[] = p.art === 'serien' ? ['name', 'neu', 'jahr', 'bewertung'] : ['name', 'neu', 'jahr', 'bewertung', 'dauer']
  const genres = haeufig(p.alle, (e) => e.genres) as string[]
  const fsk = (haeufig(p.alle, (e) => (e.age == null ? [] : [e.age])) as number[]).sort((a, b) => a - b)
  const jahre = (haeufig(p.alle, (e) => (e.jahr ? [Math.floor(e.jahr / 10) * 10] : [])) as number[]).sort((a, b) => a - b)
  const gruppe = (k: string, titel: string, kinder: preact.ComponentChildren) => (
    <section class="blatt-gruppe">
      <h3 class="fl-label">{titel}</h3>
      <Gruppe fokusKey={'fb-' + k} class="blatt-chips">
        {kinder}
      </Gruppe>
    </section>
  )
  return (
    <Sheet
      titel={t('bib.sortfilter')}
      onZu={p.onZu}
      erster="fbt-filter"
      fuss={
        <>
          <Button fokusKey="fb-reset" variante="geist" onPress={() => setF(leer())}>
            {t('bib.zuruecksetzen')}
          </Button>
          <span class="fl-grow" />
          <Button fokusKey="fb-ok" variante="primaer" onPress={p.onZu}>
            {t('bib.anzeigen', { n: p.treffer })}
          </Button>
        </>
      }
    >
      <Tabs fokusKey="fbt" aktiv={tab} onWahl={setTab} tabs={[{ id: 'sortieren', label: t('bib.sortieren') }, { id: 'filter', label: t('bib.filter') }]} />
      {tab === 'sortieren' ? (
        <>
          {gruppe(
            'feld',
            t('bib.sortieren'),
            felder.map((x) => (
              <Chip key={x} fokusKey={'fb-feld-' + x} an={p.sort.feld === x} icon={p.sort.feld === x ? 'haken' : undefined} onPress={() => p.setSort({ ...p.sort, feld: x })}>
                {t('sort.' + x)}
              </Chip>
            )),
          )}
          {gruppe(
            'richtung',
            ' ',
            [false, true].map((ab) => (
              <Chip key={String(ab)} fokusKey={'fb-ab-' + ab} an={p.sort.ab === ab} icon={p.sort.ab === ab ? 'haken' : undefined} onPress={() => p.setSort({ ...p.sort, ab })}>
                {t(ab ? 'sort.ab' : 'sort.auf')}
              </Chip>
            )),
          )}
        </>
      ) : (
        <>
          {gruppe(
            'status',
            t('f.status'),
            ['gespielt', 'ungespielt', 'fortsetzbar', 'favoriten'].map((x) => (
              <Chip key={x} fokusKey={'fb-st-' + x} an={f.status.indexOf(x) >= 0} icon={f.status.indexOf(x) >= 0 ? 'haken' : undefined} onPress={() => um('status', x)}>
                {t('f.' + x)}
              </Chip>
            )),
          )}
          <Gruppe fokusKey="fb-direkt">
            <DirektZeile an={f.direkt} onPress={() => setF({ ...f, direkt: !f.direkt })} />
          </Gruppe>
          {genres.length > 0 &&
            gruppe(
              'genres',
              t('f.genres'),
              genres.map((g) => (
                <Chip key={g} fokusKey={'fb-g-' + g} an={f.genres.indexOf(g) >= 0} icon={f.genres.indexOf(g) >= 0 ? 'haken' : undefined} onPress={() => um('genres', g)}>
                  {g}
                </Chip>
              )),
            )}
          {fsk.length > 0 &&
            gruppe(
              'fsk',
              t('f.fsk'),
              fsk.map((g) => (
                <Chip key={g} fokusKey={'fb-fsk-' + g} an={f.fsk.indexOf(g) >= 0} icon={f.fsk.indexOf(g) >= 0 ? 'haken' : undefined} onPress={() => um('fsk', g)}>
                  {'FSK ' + g}
                </Chip>
              )),
            )}
          {jahre.length > 1 &&
            gruppe(
              'jahre',
              t('f.jahre'),
              jahre.map((g) => (
                <Chip key={g} fokusKey={'fb-j-' + g} an={f.jahre.indexOf(g) >= 0} icon={f.jahre.indexOf(g) >= 0 ? 'haken' : undefined} onPress={() => um('jahre', g)}>
                  {g + 'er'}
                </Chip>
              )),
            )}
        </>
      )}
    </Sheet>
  )
}

function DirektZeile(p: { an: boolean; onPress: () => void }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: 'fb-direkt-an', onPress: p.onPress })
  return (
    <button ref={f.ref} {...f.dom} type="button" role="switch" aria-checked={p.an} class={'direkt-zeile' + (f.fokus ? ' ist-fokus' : '')}>
      <span class="fl-ampel gruen" />
      <span class="fl-grow">
        <b>{t('f.direkt')}</b>
        <small>{t('f.direkt.text')}</small>
      </span>
      <span class={'fl-schalter' + (p.an ? ' an' : '')}>
        <i />
      </span>
    </button>
  )
}

// Raster mit Werkzeugleiste, Seiten (Desktop) bzw. wachsender Liste (TV, Handy) und Buchstabenleiste.
function Liste({ art, alle }: { art: Art; alle: Eintrag[] }) {
  const [f, setF0] = useState(filterVon[art])
  const [sort, setSort0] = useState(sortVon[art])
  const [seite, setSeite] = useState(0)
  const [limit, setLimit] = useState(60)
  const [blatt, setBlatt] = useState(false)
  const [neu, setNeu] = useState(false)
  const sm = useMenue()
  const aw = useAuswahl()
  const fav = useGecacht<string[]>('favoriten') || []
  const samm = useGecacht<Sammlung[] | null>('sammlungen')
  const ich = gecacht<Nutzer>('ich')
  const g = geraet
  const blaettern = g === 'dt'
  const setF = (x: Filter) => (setF0((filterVon[art] = x)), setSeite(0))
  const setSort = (x: { feld: Feld; ab: boolean }) => (setSort0((sortVon[art] = x)), setSeite(0))
  const blaettere = (s: number) => (setSeite(s), window.scrollTo(0, 0))
  useEffect(() => {
    if (f.status.indexOf('favoriten') >= 0) favoriten().catch(() => {})
  }, [f.status.length])

  const liste = sortiere(
    alle.filter((e) => passt(e, f, fav)),
    sort.feld,
    sort.ab,
  )
  const n = liste.length
  const seiten = Math.max(1, Math.ceil(n / PRO_SEITE))
  const von = blaettern ? seite * PRO_SEITE : 0
  const sichtbar = liste.slice(von, blaettern ? von + PRO_SEITE : limit)
  const reihe = 'bib-' + art
  useEffect(() => {
    if (sichtbar.length) fokusStart(reihe + '-' + (sichtbar[0].serie ? sichtbar[0].serie.name : sichtbar[0].key))
  }, [art])

  const karteKey = (e: Eintrag) => reihe + '-' + (e.serie ? e.serie.name : e.key)
  const springe = (z: string) => {
    const i = liste.map(buchstabe).indexOf(z)
    if (i < 0) return
    if (blaettern) setSeite(Math.floor(i / PRO_SEITE))
    else if (i >= limit) setLimit(i + 60)
    fokusBald(karteKey(liste[i]))
  }
  const titelZum = (e: Eintrag) => (e.serie ? e.serie.staffeln.reduce<Item[]>((a, x) => a.concat(x.folgen), []) : [e.it])
  const alleTitel = () => liste.reduce<Item[]>((a, e) => a.concat(titelZum(e)), [])
  const aktiv = anzahlFilter(f)
  const feldLabel = t('sort.' + sort.feld) + (sort.ab ? ' ↓' : ' ↑')
  const sortEintraege: MenueEintrag[] = (['name', 'neu', 'jahr', 'bewertung', 'dauer'] as Feld[])
    .filter((x) => art !== 'serien' || x !== 'dauer')
    .map((x): MenueEintrag => ({ label: t('sort.' + x), an: sort.feld === x, onPress: () => setSort({ ...sort, feld: x }) }))
    .concat({ trenner: true }, { label: t('sort.auf'), an: !sort.ab, onPress: () => setSort({ ...sort, ab: false }) }, { label: t('sort.ab'), an: sort.ab, onPress: () => setSort({ ...sort, ab: true }) })
  const hd = g === 'hd'
  const ohneText = hd || g === 'tv'
  const zahl = blaettern ? t('bib.bereich', { von: n ? von + 1 : 0, bis: Math.min(n, von + PRO_SEITE), n }) : aktiv ? t('bib.von', { n, alle: alle.length }) : anzahl(n, 'titel.anzahl', 'titel.anzahl')
  const buchstaben = sort.feld === 'name' && n > 20
  const vorhanden: Record<string, number> = {}
  liste.forEach((e, i) => vorhanden[buchstabe(e)] === undefined && (vorhanden[buchstabe(e)] = i))
  const jetzt = sichtbar.length ? buchstabe(sichtbar[0]) : ''

  return (
    <>
      <Gruppe fokusKey="werkzeug" class="werkzeug rand" label={t('nav.' + (art === 'folgen' ? 'serien' : art))}>
        <span class="werkzeug-zahl">{zahl}</span>
        {blaettern && (
          <>
            <Button fokusKey="wz-zurueck" icon="zurueck" label={t('bib.seite.zurueck')} aus={seite === 0} onPress={() => blaettere(seite - 1)} />
            <Button fokusKey="wz-weiter" icon="weiter" label={t('bib.seite.weiter')} aus={seite >= seiten - 1} onPress={() => blaettere(seite + 1)} />
            <span class="werkzeug-strich" />
          </>
        )}
        {!hd && (
          <Button fokusKey="wz-alle" variante="primaer" icon="abspielen" aus={!n} onPress={() => spieleAlle(alleTitel())}>
            {t('bib.alle.abspielen')}
          </Button>
        )}
        <Button fokusKey="wz-zufall" variante="geist" icon="zufall" label={t('bib.zufall')} aus={!n} onPress={() => zufaellig(alleTitel())}>
          {ohneText ? undefined : t('bib.zufall')}
        </Button>
        <span class="fl-grow" />
        <Button fokusKey="wz-sort" variante="geist" icon="sortieren" label={t('bib.sortieren.nach', { feld: feldLabel })} onPress={sm.auf}>
          {ohneText ? undefined : t('bib.sortieren.nach', { feld: feldLabel })}
        </Button>
        <Button fokusKey="wz-filter" variante={aktiv ? 'sekundaer' : 'geist'} icon="filter" label={aktiv ? t('bib.filter.n', { n: aktiv }) : t('bib.filter')} onPress={() => setBlatt(true)}>
          {ohneText ? (aktiv ? String(aktiv) : undefined) : aktiv ? t('bib.filter.n', { n: aktiv }) : t('bib.filter')}
        </Button>
        {art === 'filme' && !!ich && ich.admin && samm !== null && samm !== undefined && !hd && (
          <Button fokusKey="wz-samm" variante="geist" icon="sammlung" label={t('bib.neue.sammlung')} onPress={() => setNeu(true)} />
        )}
        {g !== 'tv' && <Button fokusKey="wz-wahl" variante={aw.an ? 'sekundaer' : 'geist'} icon="haken" label={t('bib.auswahl')} onPress={() => (aw.an ? aw.ende() : aw.start())} />}
      </Gruppe>
      {hd && (
        <Gruppe fokusKey="schnellfilter" class="schnellfilter rand">
          <Chip fokusKey="sf-ungesehen" an={f.status.indexOf('ungespielt') >= 0} onPress={() => setF({ ...f, status: f.status.indexOf('ungespielt') >= 0 ? f.status.filter((x) => x !== 'ungespielt') : f.status.concat('ungespielt') })}>
            {t('f.ungesehen')}
          </Chip>
          <Chip fokusKey="sf-direkt" an={f.direkt} onPress={() => setF({ ...f, direkt: !f.direkt })}>
            <span class="fl-ampel gruen" /> {t('f.direkt.kurz')}
          </Chip>
        </Gruppe>
      )}
      {buchstaben && g === 'tv' && (
        <Gruppe fokusKey="abc" class="abc abc-quer rand">
          {ABC.map((z) => (
            <Buchstabe key={z} z={z} an={z === jetzt} aus={vorhanden[z] === undefined} onPress={() => springe(z)} />
          ))}
        </Gruppe>
      )}
      <div class={'bib-flaeche' + (buchstaben && g !== 'tv' ? ' mit-abc' : '')}>
        {!n ? (
          <Leer titel={t('bib.nichts')} text={t('bib.nichts.text')} aktion={{ label: t('bib.zuruecksetzen'), onPress: () => setF(leer()) }} />
        ) : (
          <Raster fokusKey={'raster-' + art}>
            {sichtbar.map((e, i) => (
              <EintragKarte key={e.key} e={e} reihe={reihe} aw={aw} breit={art === 'folgen'} onFocus={!blaettern && i > sichtbar.length - 12 && sichtbar.length < n ? () => setLimit(limit + 60) : undefined} />
            ))}
          </Raster>
        )}
        {buchstaben && g !== 'tv' && (
          <Gruppe fokusKey="abc" class="abc abc-hoch">
            {ABC.map((z) => (
              <Buchstabe key={z} z={z} an={z === jetzt} aus={vorhanden[z] === undefined} onPress={() => springe(z)} />
            ))}
          </Gruppe>
        )}
      </div>
      {blaettern && seiten > 1 && (
        <Gruppe fokusKey="seiten" class="seiten rand">
          <Button fokusKey="sz-zurueck" icon="zurueck" label={t('bib.seite.zurueck')} aus={seite === 0} onPress={() => blaettere(seite - 1)} />
          {seitenZahlen(seite, seiten).map((z, i) =>
            z < 0 ? (
              <span key={'x' + i} class="seiten-luecke">…</span>
            ) : (
              <Button key={z} fokusKey={'sz-' + z} variante={z === seite ? 'sekundaer' : 'geist'} label={t('bib.seite', { n: z + 1 })} onPress={() => blaettere(z)}>
                {String(z + 1)}
              </Button>
            ),
          )}
          <Button fokusKey="sz-weiter" icon="weiter" label={t('bib.seite.weiter')} aus={seite >= seiten - 1} onPress={() => blaettere(seite + 1)} />
          <span class="fl-grow" />
          <span class="leiser t-klein">{t('bib.pro.seite', { n: PRO_SEITE, s: seiten })}</span>
        </Gruppe>
      )}
      {!blaettern && sichtbar.length < n && (
        <Gruppe fokusKey="mehr-gruppe" class="rand">
          <Button fokusKey="mehr" onPress={() => setLimit(limit + 60)}>
            {t('bib.mehr')}
          </Button>
        </Gruppe>
      )}
      {sm.offen && <Menue anker={sm.anker} titel={t('bib.sortieren')} eintraege={sortEintraege} onZu={sm.zu} />}
      {blatt && <FilterBlatt art={art} alle={alle} f={f} setF={setF} sort={sort} setSort={setSort} treffer={n} onZu={() => setBlatt(false)} />}
      {neu && <NameDialog titel={t('bib.neue.sammlung')} onZu={() => setNeu(false)} onOk={(name) => sammlungenApi.neu({ name }).then((s) => go(pfad('sammlung', s.id)))} />}
      <AuswahlLeiste aw={aw} alle={liste.map((e) => e.key)} />
    </>
  )
}

// 1 2 3 4 … 11: erste, letzte und die Nachbarn der aktuellen Seite; -1 = Lücke.
function seitenZahlen(s: number, n: number): number[] {
  const out: number[] = []
  for (let i = 0; i < n; i++) if (i === 0 || i === n - 1 || Math.abs(i - s) <= 1 || (s < 3 && i < 4)) out.push(i)
  return out.reduce<number[]>((a, i) => (a.length && i - a[a.length - 1] > 1 ? a.concat(-1, i) : a.concat(i)), [])
}

// Vorschläge: ohne Server-Logik aus Bewertung, Genres und Zeitpunkt.
function Vorschlaege({ art, alle }: { art: Art; alle: Eintrag[] }) {
  const offen = alle.filter((e) => !e.gesehen)
  const reihen: { id: string; titel: string; l: Eintrag[] }[] = []
  reihen.push({ id: 'weiter', titel: t('vor.weiter'), l: alle.filter((e) => e.begonnen) })
  reihen.push({ id: 'bewertet', titel: t('vor.bewertet'), l: sortiere(offen.filter((e) => e.rating >= 7), 'bewertung', true) })
  const vorbild = sortiere(alle.filter((e) => e.gesehen && e.genres.length), 'neu', true)[0]
  if (vorbild) reihen.push({ id: 'weil', titel: t('vor.weil', { titel: vorbild.titel }), l: sortiere(offen.filter((e) => e.genres.some((g) => vorbild.genres.indexOf(g) >= 0)), 'bewertung', true) })
  if (art === 'filme') reihen.push({ id: 'kurz', titel: t('vor.kurz'), l: offen.filter((e) => e.dauer > 0 && e.dauer <= 95 * 60) })
  reihen.push({ id: 'neu', titel: t('vor.neu'), l: sortiere(alle, 'neu', true) })
  const da = reihen.filter((r) => r.l.length)
  useEffect(() => {
    if (da.length) fokusStart('vor-' + da[0].id + '-' + (da[0].l[0].serie ? da[0].l[0].serie.name : da[0].l[0].key))
  }, [])
  return (
    <>
      {da.map((r) => (
        <Reihe key={r.id} fokusKey={'reihe-vor-' + r.id} titel={r.titel}>
          {r.l.slice(0, 24).map((e) => (
            <EintragKarte key={e.key} e={e} reihe={'vor-' + r.id} />
          ))}
        </Reihe>
      ))}
    </>
  )
}

// Genres bzw. Studios: eine Reihe je Wert; die Überschrift öffnet die gefilterte Bibliothek.
function NachWert({ art, alle, von }: { art: 'filme' | 'serien'; alle: Eintrag[]; von: 'genres' | 'studios' }) {
  const werte = (haeufig(alle, (e) => e[von]) as string[]).slice(0, 20)
  useEffect(() => {
    if (werte.length) fokusStart('reihe-w-0')
  }, [])
  if (!werte.length) return <Leer titel={t('bib.nichts')} />
  return (
    <>
      {werte.map((w, i) => {
        const l = sortiere(alle.filter((e) => e[von].indexOf(w) >= 0), 'bewertung', true)
        return (
          <Reihe key={w} fokusKey={'reihe-w-' + i} titel={w} zusatz={String(l.length)} ziel={von === 'genres' ? '/' + art : undefined} onZiel={() => zeigeGenre(art, w)}>
            {l.slice(0, 24).map((e) => (
              <EintragKarte key={e.key} e={e} reihe={'w' + i} />
            ))}
          </Reihe>
        )
      })}
    </>
  )
}

export function Bibliothek({ art }: { art: 'filme' | 'serien' }) {
  const r = useRoute()
  const tab = r[1] || art
  const alle = useDaten<Item[]>('bibliothek', () => bibliothek(), { merken: true })
  const tabs: SeitenTab[] =
    art === 'filme'
      ? [
          { id: 'filme', label: t('nav.filme'), pfad: '/filme' },
          { id: 'vorschlaege', label: t('bib.vorschlaege'), pfad: '/filme/vorschlaege' },
          { id: 'favoriten', label: t('bib.favoriten'), pfad: '/favoriten' },
          { id: 'sammlungen', label: t('bib.sammlungen'), pfad: '/sammlungen' },
          { id: 'genres', label: t('bib.genres'), pfad: '/filme/genres' },
          { id: 'studios', label: t('bib.studios'), pfad: '/filme/studios' },
        ]
      : [
          { id: 'serien', label: t('nav.serien'), pfad: '/serien' },
          { id: 'vorschlaege', label: t('bib.vorschlaege'), pfad: '/serien/vorschlaege' },
          { id: 'genres', label: t('bib.genres'), pfad: '/serien/genres' },
          { id: 'folgen', label: t('bib.folgen'), pfad: '/serien/folgen' },
        ]
  // TODO(server): „Demnächst“ (Serien) braucht Ausstrahlungstermine; es gibt dafür keinen Endpunkt, der Tab fehlt.
  const d = alle.daten
  const liste = d ? eintraege(d, tab === 'folgen' ? 'folgen' : art) : null
  let inhalt: preact.ComponentChildren
  if (!liste) inhalt = alle.fehler ? <Fehler fehler={alle.fehler} nochmal={alle.neu} /> : <div class="raster rand"><SkeletonKarten n={12} /></div>
  else if (!liste.length) inhalt = <Leer titel={t(art === 'filme' ? 'leer.filme' : 'leer.serien')} />
  else if (tab === 'vorschlaege') inhalt = <Vorschlaege art={art} alle={liste} />
  else if (tab === 'genres' || tab === 'studios') inhalt = <NachWert art={art} alle={liste} von={tab} />
  else inhalt = <Liste key={tab} art={tab === 'folgen' ? 'folgen' : art} alle={liste} />
  return (
    <Seite bereich={art} titel={t('nav.' + art)} tabs={tabs} tab={tab} class="browse bibliothek">
      {inhalt}
    </Seite>
  )
}
