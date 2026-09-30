// Metadaten-Editor: Dialog (Kontextmenü „Metadaten bearbeiten“) und Inhalt des Metadaten-Managers.
// Speichern über PUT /api/items/{id}/meta, Identifizieren über /api/items/{id}/search|identify.
import { useEffect, useState } from 'preact/hooks'
import { vergessen } from '../../lib/api'
import { details, fehlt, identifizieren, kandidaten as kandidatenLaden, metaSpeichern, type Details, type Kandidat, type MetaAenderung, type Person } from '../../lib/api-admin'
import { fokusBald } from '../../lib/focus'
import { ergaenze, t } from '../../lib/i18n'
import { Button, Chip } from '../../components/Button'
import { Dialog } from '../../components/Menue'
import { Tabs } from '../../components/Tabs'
import { Fehler } from '../../components/Zustand'
import { Feld, Toast, Wahl, Zeile, datum, fehlerText, melde, meldeFehler } from './teile'

ergaenze({
  'meta.titel': 'Metadaten bearbeiten',
  'meta.tab.meta': 'Metadaten',
  'meta.tab.ident': 'Identifizieren',
  'meta.pfad': 'Pfad',
  'meta.f.title': 'Titel',
  'meta.f.originalTitle': 'Originaltitel',
  'meta.f.sortTitle': 'Sortiertitel',
  'meta.f.year': 'Jahr',
  'meta.f.rating': 'Community-Bewertung',
  'meta.f.age': 'Altersfreigabe',
  'meta.f.tagline': 'Leitsatz',
  'meta.f.overview': 'Übersicht',
  'meta.f.genres': 'Genres',
  'meta.f.studios': 'Studios',
  'meta.f.tags': 'Tags',
  'meta.f.people': 'Personen',
  'meta.hinzu': 'Hinzufügen …',
  'meta.hinzufuegen': 'Hinzufügen',
  'meta.entfernen': '{name} entfernen',
  'meta.person.name': 'Name',
  'meta.person.rolle': 'Rolle',
  'meta.kind.actor': 'Darsteller',
  'meta.kind.director': 'Regie',
  'meta.kind.writer': 'Drehbuch',
  'meta.kind.producer': 'Produktion',
  'meta.sperren': 'Felder sperren',
  'meta.sperren.text': 'Gesperrte Felder überschreibt Flimmer beim Aktualisieren nicht.',
  'meta.alle.sperren': 'Metadaten sperren',
  'meta.quelle': 'Quelle: {q}',
  'meta.speichern.text': 'Änderungen speichert Flimmer in der Datenbank.',
  'meta.abbrechen': 'Abbrechen',
  'meta.fehlt': 'Der Server kann Metadaten noch nicht speichern.',
  'meta.suchen': 'Suchen',
  'meta.ident.text': 'Wähle den passenden Treffer. Flimmer lädt danach Titel, Beschreibung und Bilder neu.',
  'meta.ident.leer': 'Keine Treffer. Versuch einen anderen Suchbegriff.',
  'meta.ident.ok': 'Neu zugeordnet',
})

const FSK: [number, string][] = [[-1, '–'], [0, 'FSK 0'], [6, 'FSK 6'], [12, 'FSK 12'], [16, 'FSK 16'], [18, 'FSK 18']]
const SPERRBAR = ['title', 'overview', 'genres', 'people', 'studios', 'tags', 'age', 'year', 'tagline']
const ARTEN: Person['kind'][] = ['actor', 'director', 'writer', 'producer']

interface Form {
  title: string
  originalTitle: string
  sortTitle: string
  year: string
  rating: string
  age: number
  tagline: string
  overview: string
  genres: string[]
  studios: string[]
  tags: string[]
  people: Person[]
  locked: string[]
}

function formAus(d: Details): Form {
  const m = d.meta || {}
  return {
    title: m.title || d.title || '',
    originalTitle: m.originalTitle || '',
    sortTitle: m.sortTitle || '',
    year: String(m.year || d.year || ''),
    rating: m.rating ? String(m.rating).replace('.', ',') : '',
    age: m.age === undefined || m.age === null ? -1 : m.age,
    tagline: d.tagline || m.tagline || '',
    overview: m.overview || '',
    genres: m.genres || [],
    studios: d.studios || m.studios || [],
    tags: d.tags || m.tags || [],
    people: d.people || m.people || [],
    locked: d.locked || [],
  }
}

function aenderung(f: Form): MetaAenderung {
  return {
    title: f.title.trim(),
    originalTitle: f.originalTitle.trim(),
    sortTitle: f.sortTitle.trim(),
    year: Number(f.year) || 0,
    rating: Number(f.rating.replace(',', '.')) || 0,
    age: f.age < 0 ? null : f.age,
    tagline: f.tagline.trim(),
    overview: f.overview.trim(),
    genres: f.genres,
    studios: f.studios,
    tags: f.tags,
    people: f.people,
    locked: f.locked,
  }
}

// Dialog für das Kontextmenü (web-browse).
export function MetadatenEditor({ id, onZu }: { id: string; onZu: () => void }) {
  return (
    <Dialog titel={t('meta.titel')} onZu={onZu} breit>
      <Editor id={id} fertig={onZu} fokus />
      <Toast />
    </Dialog>
  )
}

// Inhalt: Tabs „Metadaten“ und „Identifizieren“. start = 'ident' öffnet gleich die Zuordnung (unsichere Titel).
export function Editor(p: { id: string; fertig?: () => void; start?: 'meta' | 'ident'; onGespeichert?: () => void; fokus?: boolean }) {
  const { id, fertig, start = 'meta', onGespeichert } = p
  const [d, setD] = useState<Details | null>(null)
  const [fehler, setFehler] = useState('')
  const [tab, setTab] = useState(start)
  const laden = () =>
    details(id).then(
      (x) => {
        setD(x)
        setFehler('')
      },
      (e) => setFehler(fehlerText(e)),
    )
  useEffect(() => {
    setD(null)
    setTab(start)
    laden()
  }, [id])
  // Im Dialog nach dem Laden ins erste Feld (TV: sonst hinge der Fokus am Schließen-Knopf).
  useEffect(() => {
    if (d && p.fokus) fokusBald(tab === 'meta' ? 'meta-title' : 'meta-q')
  }, [!!d])
  if (fehler) return <Fehler fehler={fehler} nochmal={laden} />
  if (!d) return <div class="admin-laedt" aria-busy="true" />
  return (
    <div class="admin-editor">
      <p class="t-klein leise admin-absatz eine-zeile">{d.title + (d.year ? ' (' + d.year + ')' : '')}</p>
      <Tabs
        fokusKey="meta-tabs"
        aktiv={tab}
        tabs={[
          { id: 'meta', label: t('meta.tab.meta') },
          { id: 'ident', label: t('meta.tab.ident') },
        ]}
        onWahl={(x) => setTab(x as 'meta' | 'ident')}
      />
      {tab === 'meta' ? (
        <Formular key={id} d={d} fertig={fertig} onGespeichert={onGespeichert} />
      ) : (
        <Identifizieren
          d={d}
          fertig={() => {
            vergessen()
            laden()
            setTab('meta')
            if (onGespeichert) onGespeichert()
          }}
        />
      )}
    </div>
  )
}

function Formular({ d, fertig, onGespeichert }: { d: Details; fertig?: () => void; onGespeichert?: () => void }) {
  const [f, setF] = useState<Form>(() => formAus(d))
  const [laeuft, setLaeuft] = useState(false)
  // Geänderte Felder gleich sperren, wie es der Server ohne locked täte – hier sichtbar und abwählbar.
  const setze = (teil: Partial<Form>) => {
    const locked = teil.locked || f.locked.concat(Object.keys(teil).filter((k) => SPERRBAR.indexOf(k) >= 0 && f.locked.indexOf(k) < 0))
    setF({ ...f, ...teil, locked })
  }
  const text = (k: 'title' | 'originalTitle' | 'sortTitle' | 'year' | 'rating' | 'tagline', breit?: boolean, typ?: string) => (
    <Feld fk={'meta-' + k} wert={f[k]} setWert={(w) => setze({ [k]: w } as Partial<Form>)} label={t('meta.f.' + k)} zeigeLabel breit={breit} typ={typ} />
  )
  const speichern = () => {
    setLaeuft(true)
    metaSpeichern(d.id, aenderung(f)).then(
      () => {
        setLaeuft(false)
        melde(t('einst.gespeichert'))
        vergessen()
        if (onGespeichert) onGespeichert()
        if (fertig) fertig()
      },
      (e) => {
        setLaeuft(false)
        melde(fehlt(String(e && (e as Error).message)) ? t('meta.fehlt') : fehlerText(e))
      },
    )
  }
  const alle = SPERRBAR.every((k) => f.locked.indexOf(k) >= 0)
  return (
    <>
      <div class="admin-form">
        {d.path && <Feld fk="meta-pfad" wert={d.path} setWert={() => {}} label={t('meta.pfad')} zeigeLabel breit nurLesen />}
        {text('title', true)}
        {text('originalTitle')}
        {text('sortTitle')}
        {text('year', false, 'number')}
        {text('rating')}
        <div class="admin-feld breit">
          <span class="admin-feld-label">{t('meta.f.age')}</span>
          <Wahl fk="meta-age" werte={FSK} an={f.age} onWahl={(age) => setze({ age })} />
        </div>
        {text('tagline', true)}
        <Feld fk="meta-overview" wert={f.overview} setWert={(overview) => setze({ overview })} label={t('meta.f.overview')} zeigeLabel breit mehrzeilig />
        <ListeFeld fk="meta-genres" label={t('meta.f.genres')} werte={f.genres} setWerte={(genres) => setze({ genres })} />
        <ListeFeld fk="meta-studios" label={t('meta.f.studios')} werte={f.studios} setWerte={(studios) => setze({ studios })} />
        <ListeFeld fk="meta-tags" label={t('meta.f.tags')} werte={f.tags} setWerte={(tags) => setze({ tags })} />
        {(d.meta && (d.meta.tmdbId || d.meta.imdbId)) ? (
          <p class="t-klein leise admin-feld breit">
            {[d.meta.tmdbId ? 'TMDB ' + d.meta.tmdbId : '', d.meta.imdbId ? 'IMDb ' + d.meta.imdbId : ''].filter(Boolean).join(' · ')}
          </p>
        ) : null}
      </div>
      <Personen werte={f.people} setWerte={(people) => setze({ people })} />
      <div class="fl-gruppe">
        <h3>{t('meta.sperren')}</h3>
        <p class="t-klein leise admin-absatz">{t('meta.sperren.text')}</p>
        <div class="admin-chips">
          {SPERRBAR.map((k) => (
            <Chip key={k} fokusKey={'meta-sperre-' + k} icon={f.locked.indexOf(k) >= 0 ? 'schloss' : undefined} an={f.locked.indexOf(k) >= 0} onPress={() => setze({ locked: f.locked.indexOf(k) >= 0 ? f.locked.filter((x) => x !== k) : f.locked.concat(k) })}>
              {t('meta.f.' + k)}
            </Chip>
          ))}
        </div>
        <Zeile fk="meta-alle" icon="schloss" titel={t('meta.alle.sperren')} schalter={alle} onPress={() => setze({ locked: alle ? [] : SPERRBAR.slice() })} />
      </div>
      <div class="admin-fuss fl-reihe-flex">
        <span class="fl-grow t-klein leise">
          {d.meta && d.meta.source ? t('meta.quelle', { q: d.meta.source.toUpperCase() }) + (d.added ? ' · ' + datum(d.added, false) : '') + ' · ' : ''}
          {t('meta.speichern.text')}
        </span>
        {fertig && (
          <Button fokusKey="meta-abbrechen" variante="geist" onPress={fertig}>
            {t('meta.abbrechen')}
          </Button>
        )}
        <Button fokusKey="meta-speichern" variante="primaer" aus={laeuft || !f.title.trim()} onPress={speichern}>
          {t('einst.speichern')}
        </Button>
      </div>
    </>
  )
}

// Liste als Chips (Drücken entfernt) mit Eingabe zum Ergänzen (Enter oder +).
function ListeFeld({ fk, label, werte, setWerte }: { fk: string; label: string; werte: string[]; setWerte: (w: string[]) => void }) {
  const [neu, setNeu] = useState('')
  const hinzu = () => {
    const w = neu.trim()
    if (w && werte.indexOf(w) < 0) setWerte(werte.concat(w))
    setNeu('')
  }
  return (
    <div class="admin-feld breit">
      <span class="admin-feld-label">{label}</span>
      <div class="admin-chips">
        {werte.map((w) => (
          <Chip key={w} fokusKey={fk + '-' + w} icon="schliessen" onPress={() => setWerte(werte.filter((x) => x !== w))}>
            <span class="nur-sr">{t('meta.entfernen', { name: w })}</span>
            <span aria-hidden="true">{w}</span>
          </Chip>
        ))}
      </div>
      <div class="admin-formzeile">
        <Feld fk={fk + '-neu'} wert={neu} setWert={setNeu} label={t('meta.hinzu')} onEnter={hinzu} />
        <Button fokusKey={fk + '-plus'} icon="plus" label={t('meta.hinzufuegen')} aus={!neu.trim()} onPress={hinzu} />
      </div>
    </div>
  )
}

function Personen({ werte, setWerte }: { werte: Person[]; setWerte: (w: Person[]) => void }) {
  const [name, setName] = useState('')
  const [rolle, setRolle] = useState('')
  const [art, setArt] = useState<Person['kind']>('actor')
  const hinzu = () => {
    if (!name.trim()) return
    setWerte(werte.concat({ name: name.trim(), role: rolle.trim() || undefined, kind: art }))
    setName('')
    setRolle('')
    fokusBald('meta-person-name')
  }
  return (
    <div class="fl-gruppe">
      <h3>
        {t('meta.f.people')} · {werte.length}
      </h3>
      {werte.map((p, i) => (
        <Zeile
          key={p.name + i}
          icon="profil"
          titel={p.name}
          unter={t('meta.kind.' + p.kind) + (p.role ? ' · „' + p.role + '“' : '')}
          rechts={<Button fokusKey={'meta-person-weg-' + i} icon="schliessen" label={t('meta.entfernen', { name: p.name })} onPress={() => setWerte(werte.filter((_, j) => j !== i))} />}
        />
      ))}
      <div class="admin-formzeile">
        <Feld fk="meta-person-name" wert={name} setWert={setName} label={t('meta.person.name')} onEnter={hinzu} />
        <Feld fk="meta-person-rolle" wert={rolle} setWert={setRolle} label={t('meta.person.rolle')} onEnter={hinzu} />
      </div>
      <div class="admin-formzeile">
        <Wahl fk="meta-person-art" werte={ARTEN.map((a) => [a, t('meta.kind.' + a)] as [Person['kind'], string])} an={art} onWahl={setArt} />
        <Button fokusKey="meta-person-hinzu" icon="plus" aus={!name.trim()} onPress={hinzu}>
          {t('meta.hinzufuegen')}
        </Button>
      </div>
    </div>
  )
}

function Identifizieren({ d, fertig }: { d: Details; fertig: () => void }) {
  const [q, setQ] = useState((d.meta && d.meta.title) || d.title)
  const [liste, setListe] = useState<Kandidat[] | null>(null)
  const suche = () => kandidatenLaden(d.id, q).then(setListe, meldeFehler)
  useEffect(() => {
    suche()
    fokusBald('meta-q')
  }, [])
  return (
    <div class="fl-gruppe admin-ident">
      <p class="t-klein leise admin-absatz">{t('meta.ident.text')}</p>
      <div class="admin-formzeile">
        <Feld fk="meta-q" wert={q} setWert={setQ} label={t('meta.suchen')} breit onEnter={suche} />
        <Button fokusKey="meta-suchen" icon="suche" onPress={suche}>
          {t('meta.suchen')}
        </Button>
      </div>
      {liste && !liste.length && <p class="t-text leise">{t('meta.ident.leer')}</p>}
      {(liste || []).map((k) => (
        <Zeile
          key={k.tmdbId}
          fk={'meta-kand-' + k.tmdbId}
          icon="haken"
          titel={k.title + (k.year ? ' (' + k.year + ')' : '')}
          wert={'TMDB ' + k.tmdbId}
          onPress={() =>
            identifizieren(d.id, k.tmdbId).then(() => {
              melde(t('meta.ident.ok'))
              fertig()
            }, meldeFehler)
          }
        />
      ))}
    </div>
  )
}
