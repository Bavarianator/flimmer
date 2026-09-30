// Detailseiten im Jellyfin-Aufbau: Film (auch Folge), Serie und Staffel (Entwurf Film-Detail, Serie-Detail,
// Staffel, Handy-Film, Handy-Serie, TV-Detail). Lädt nach; screens/DetailFilm.tsx usw. sind nur Hüllen.
import type { ComponentChildren } from 'preact'
import { useEffect, useState } from 'preact/hooks'
import {
  alleGesehen,
  anteil,
  anzeigeTitel,
  bibliothek,
  bild,
  details,
  fortsetzenAb,
  jahr,
  naechsteFolge,
  serien,
  serienInfo,
  setFavorit,
  useDaten,
  useGecacht,
  vorladen,
  type Details as DetailDaten,
  type Item,
  type Person,
  type Sammlung,
  type Serie,
} from '../lib/api'
import { geraet, istTV } from '../lib/device'
import { Gruppe, fokusStart, useFokus } from '../lib/focus'
import { anzahl, dauer, ergaenze, folge, t, uhr } from '../lib/i18n'
import { back, go, pfad } from '../lib/router'
import { AktionenMenue, technik, zufaellig } from './Aktionen'
import { eintraege, sortiere, zeigeGenre, EintragKarte, type Eintrag } from './Bibliothek'
import { Button } from './Button'
import { AmpelZeile, Badges, Bild, Fortschritt, Karte, tonFuer } from './Karte'
import { useMenue } from './Menue'
import { Reihe } from './Reihe'
import { Seite } from './Seite'
import { SkeletonHero } from './Skeleton'
import { Tabs } from './Tabs'
import { SerienKarte } from './TitelKarte'
import { Fehler, Leer } from './Zustand'
import '../screens/browse.css'

ergaenze({
  'det.film': 'Film',
  'det.folge': 'Folge',
  'det.serie': 'Serie',
  'det.endet': 'Endet um {uhr}',
  'det.hinzu': 'Hinzugefügt am {datum}',
  'det.mehr': 'Mehr anzeigen',
  'det.weniger': 'Weniger anzeigen',
  'det.genres': 'Genres',
  'det.regie': 'Regie',
  'det.buch': 'Drehbuch',
  'det.studio': 'Studio',
  'det.sender': 'Sender',
  'det.tags': 'Tags',
  'det.szenen': 'Szenen',
  'det.kapitel': 'Kapitel {n}',
  'det.szene': 'Szene {n}',
  'det.musik': 'Musik',
  'det.land': 'Land',
  'det.besetzung': 'Besetzung & Mitwirkende',
  'det.sammlungen': 'Sammlungen',
  'det.teil': 'Teil von: {name}',
  'det.aehnlich': 'Mehr wie dieses',
  'det.naechstes': 'Als Nächstes',
  'det.staffeln': 'Staffeln',
  'det.fortsetzen.folge': '{folge} fortsetzen',
  'det.laufend': 'Fortlaufend',
  'det.beendet': 'Beendet',
  'det.heute': 'heute',
  'det.fsk': 'Freigegeben ab {n} Jahren',
  'det.gesehen.von': '{g} von {n} Folgen gesehen',
  'det.staffel.fuss': '{n} Folgen · {g} gesehen · {b} begonnen',
  'det.folgen': 'Folgen von {staffel}',
  'det.staffel.gesehen': 'Staffel als gesehen markieren',
  'det.zur.serie': 'Zur Serie',
  'rolle.director': 'Regie',
  'rolle.writer': 'Drehbuch',
  'rolle.producer': 'Produktion',
  'rolle.composer': 'Musik',
})

// ---------- Hilfen ----------
function endetUm(sek: number) {
  const d = new Date(Date.now() + sek * 1000)
  return d.getHours() + ':' + (d.getMinutes() < 10 ? '0' : '') + d.getMinutes()
}

function datum(iso?: string) {
  if (!iso) return ''
  const d = new Date(iso)
  return isNaN(d.getTime()) ? '' : d.toLocaleDateString('de-DE', { day: 'numeric', month: 'long', year: 'numeric' })
}

function statusText(s?: string) {
  if (!s) return ''
  return /end|cancel/i.test(s) ? t('det.beendet') : t('det.laufend')
}

export function initialen(name: string) {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .map((w) => w.charAt(0))
    .slice(0, 2)
    .join('')
    .toUpperCase()
}

function Fsk({ n }: { n?: number | null }) {
  if (n == null) return null
  return (
    <span class="fsk" title={t('det.fsk', { n })}>
      {'FSK ' + n}
    </span>
  )
}

// Beschreibung mit Leitsatz; lange Texte klappen auf.
function Text({ text, leitsatz }: { text?: string; leitsatz?: string }) {
  const [lang, setLang] = useState(false)
  if (!text && !leitsatz) return null
  return (
    <div class="detail-text">
      {leitsatz && <p class="leitsatz">„{leitsatz}“</p>}
      {text && <p class={lang ? '' : 'kurz'}>{text}</p>}
      {text && text.length > 240 && !istTV() && (
        <button type="button" class="mehr-text" onClick={() => setLang(!lang)}>
          {t(lang ? 'det.weniger' : 'det.mehr')}
        </button>
      )}
    </div>
  )
}

function InfoZeile({ k, children }: { k: string; children: ComponentChildren }) {
  return (
    <div class="info-paar">
      <span class="fl-label">{k}</span>
      <span class="wachse">{children}</span>
    </div>
  )
}

function Verweis({ text, onPress }: { text: string; onPress: () => void }) {
  return (
    <a
      href="#"
      class="verweis"
      onClick={(e) => {
        e.preventDefault()
        onPress()
      }}
    >
      {text}
    </a>
  )
}

function Liste<T>({ l, bau }: { l: T[]; bau: (x: T) => ComponentChildren }) {
  return (
    <>
      {l.map((x, i) => (
        <span key={i}>
          {i > 0 && ', '}
          {bau(x)}
        </span>
      ))}
    </>
  )
}

// Fakten rechts neben (Desktop) bzw. unter der Beschreibung (Handy, TV). Leere Felder fallen weg;
// ohne ein einziges Feld gibt es keinen Block. Laufzeit, FSK und Bewertung stehen schon in der Zeile unter dem Titel.
function Fakten(p: { art: 'filme' | 'serien'; genres?: string[]; people?: Person[]; studios?: string[]; countries?: string[]; tags?: string[] }) {
  const wer = (kind: string) => (p.people || []).filter((x) => x.kind === kind).slice(0, 3)
  const person = (x: Person) => <Verweis text={x.name} onPress={() => go(pfad('person', x.name))} />
  const zeilen: ComponentChildren[] = []
  const zeile = (k: string, inhalt: ComponentChildren) => zeilen.push(<InfoZeile key={k} k={t(k)}>{inhalt}</InfoZeile>)
  if (p.genres && p.genres.length) zeile('det.genres', <Liste l={p.genres} bau={(g) => <Verweis text={g} onPress={() => zeigeGenre(p.art, g)} />} />)
  if (wer('director').length) zeile('det.regie', <Liste l={wer('director')} bau={person} />)
  if (wer('writer').length) zeile('det.buch', <Liste l={wer('writer')} bau={person} />)
  if (wer('composer').length) zeile('det.musik', <Liste l={wer('composer')} bau={person} />)
  if (p.studios && p.studios.length) zeile(p.art === 'serien' ? 'det.sender' : 'det.studio', p.studios.slice(0, 3).join(', '))
  if (p.countries && p.countries.length) zeile('det.land', p.countries.slice(0, 3).join(', '))
  if (p.tags && p.tags.length) zeile('det.tags', p.tags.slice(0, 6).map((x) => <span key={x} class="tag">{x}</span>))
  if (!zeilen.length) return null
  return <div class="detail-info">{zeilen}</div>
}

// IMDb/TMDB als kleine Verweise in der Faktenzeile (nicht auf dem TV: dort nicht bedienbar).
function Links({ imdb, tmdb }: { imdb?: string; tmdb?: string }) {
  if (geraet === 'tv' || (!imdb && !tmdb)) return null
  return (
    <>
      {imdb && <a class="verweis link" href={'https://www.imdb.com/title/' + imdb + '/'} target="_blank" rel="noopener noreferrer">IMDb</a>}
      {tmdb && <a class="verweis link" href={'https://www.themoviedb.org/' + tmdb} target="_blank" rel="noopener noreferrer">TMDB</a>}
    </>
  )
}

// „Chapter 1“, „Kapitel 01“, „Chapter 1.“, „00:12:03.000“ oder leer → „Szene 1“.
export function szenenName(name: string | undefined, i: number): { titel: string; eigen: boolean } {
  const n = (name || '').trim()
  if (!n || /^(chapter|kapitel|chapitre|cap[ií]tulo|capitolo|scene|szene|hoofdstuk)?\s*\d+\.?$/i.test(n) || /^\d{1,2}(:\d{2}){1,2}([.,]\d+)?$/.test(n))
    return { titel: t('det.szene', { n: i + 1 }), eigen: false }
  return { titel: n, eigen: true }
}

// Kopf aller Detailseiten: Kulisse, Label, Titel in Plakat-Versalien, Fakten, Knöpfe, Infospalte, Text.
function Kopf(p: { label: string; titel: string; fakten: ComponentChildren[]; unter?: string; bild?: string; farbe?: string; knoepfe: ComponentChildren; info?: ComponentChildren; text?: string; leitsatz?: string; technik?: string }) {
  return (
    <div class={'fl-hero held detail-held' + (p.bild ? '' : ' ohne-bild')}>
      <div class="kulisse" style={{ background: p.farbe || tonFuer(p.titel) }} aria-hidden="true">
        {p.bild && <img key={p.bild} src={p.bild} alt="" onLoad={(e) => ((e.target as HTMLElement).className = 'da')} />}
      </div>
      <div class="schleier" aria-hidden="true" />
      <div class="inhalt detail-inhalt">
        <span class="fl-label">{p.label}</span>
        <h1>{p.titel}</h1>
        <div class="fakten detail-fakten">{p.fakten.filter(Boolean).map((f, i) => <span key={i}>{f}</span>)}</div>
        {p.unter && <div class="detail-unter">{p.unter}</div>}
        <Gruppe fokusKey="knoepfe" class="detail-knoepfe">
          {p.knoepfe}
        </Gruppe>
        {(p.text || p.leitsatz || p.info || p.technik) && (
          <div class="detail-unten">
            <div class="detail-textblock">
              <Text text={p.text} leitsatz={p.leitsatz} />
              {p.technik && <div class="detail-technik">{p.technik}</div>}
            </div>
            {p.info}
          </div>
        )}
      </div>
    </div>
  )
}

// „Mehr wie dieses“ gibt es immer: gleiche Genres, sonst gleiche Sammlung bzw. Jahrzehnt, sonst zuletzt hinzugefügt.
function mehrWie(andere: Eintrag[], genres: string[], jahrzehnt: number, sammlung: string[]): Eintrag[] {
  const n = (e: Eintrag) => e.genres.filter((g) => genres.indexOf(g) >= 0).length
  const nachGenre = andere.filter((e) => n(e) > 0).sort((a, b) => n(b) - n(a) || b.rating - a.rating)
  const nachNaehe = andere.filter((e) => n(e) === 0 && (sammlung.indexOf(e.key) >= 0 || (jahrzehnt > 0 && Math.floor(e.jahr / 10) * 10 === jahrzehnt)))
  const neu = sortiere(andere, 'neu', true)
  const out: Eintrag[] = []
  for (const e of nachGenre.concat(nachNaehe, neu)) if (out.indexOf(e) < 0 && out.push(e) >= 16) break
  return out
}

// Runde Icon-Knöpfe der Kopfzeile: Gesehen, Favorit, Mehr.
function GesehenIcon({ keys, an }: { keys: string[]; an: boolean }) {
  return <Button fokusKey="k-gesehen" icon="haken" variante={an ? 'primaer' : 'sekundaer'} label={t(an ? 'akt.ungesehen' : 'akt.gesehen')} onPress={() => alleGesehen(keys, !an)} />
}

function FavIcon({ k }: { k: string }) {
  const fav = useGecacht<string[]>('favoriten') || []
  const an = fav.indexOf(k) >= 0
  return (
    <span class={an ? 'herz-an' : ''}>
      <Button fokusKey="k-fav" icon="herz" label={t(an ? 'akt.fav.weg' : 'akt.fav')} onPress={() => setFavorit(k, !an).catch(() => {})} />
    </span>
  )
}

function MehrIcon({ it, serie }: { it: Item; serie?: boolean }) {
  const m = useMenue()
  return (
    <>
      <Button fokusKey="k-mehr" icon="mehr" label={t('knopf.mehr.menue')} onPress={m.auf} />
      {m.offen && <AktionenMenue ziel={{ it, serie }} anker={m.anker} onZu={m.zu} />}
    </>
  )
}

// Abspielen bzw. Fortsetzen mit Fortschritt darunter, dann Von vorn.
function Spielen({ it, label }: { it: Item; label?: string }) {
  const pos = fortsetzenAb(it.id)
  const a = anteil(it)
  return (
    <>
      <span class="spielen-block">
        <Button fokusKey="abspielen" variante="primaer" icon="abspielen" onPress={() => go(pfad('watch', it.id))}>
          {label || (pos > 0 ? t('knopf.fortsetzen', { zeit: uhr(pos) }) : t('knopf.abspielen'))}
        </Button>
        {a > 0 && <Fortschritt anteil={a} />}
      </span>
      {pos > 0 && (
        <Button fokusKey="vonvorn" icon="neustart" onPress={() => go(pfad('watch', it.id, 0))}>
          {t('knopf.vonvorn')}
        </Button>
      )}
    </>
  )
}

function PersonKarte({ p, reihe }: { p: Person; reihe: string }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: reihe + '-' + p.name + p.role, onPress: () => go(pfad('person', p.name)) })
  const rolle = p.kind === 'actor' ? p.role : t('rolle.' + p.kind)
  return (
    <button ref={f.ref} {...f.dom} type="button" class={'fl-karte person-karte' + (f.fokus ? ' ist-fokus' : '')} aria-label={[p.name, rolle].filter(Boolean).join(', ')}>
      <div class="rahmen" style={{ background: tonFuer(p.name) }}>
        {p.image ? <Bild src={p.image} titel={p.name} /> : <span class="initialen">{initialen(p.name)}</span>}
      </div>
      <div class="meta" aria-hidden="true">
        <b>{p.name}</b>
        {rolle && <small>{rolle}</small>}
      </div>
    </button>
  )
}

function Besetzung({ people }: { people?: Person[] }) {
  if (!people || !people.length) return null
  // Regie und Buch zuerst, dann die Darsteller in der Reihenfolge der Quelle.
  const l = people.filter((x) => x.kind === 'director').concat(people.filter((x) => x.kind !== 'director'))
  return (
    <Reihe fokusKey="reihe-besetzung" titel={t('det.besetzung')}>
      {l.slice(0, 30).map((x, i) => (
        <PersonKarte key={i} p={x} reihe="besetzung" />
      ))}
    </Reihe>
  )
}

// ---------- Film (auch einzelne Folge) ----------
export function DetailFilm({ id }: { id: string }) {
  const alle = useDaten<Item[]>('bibliothek', () => bibliothek(), { merken: true })
  const det = useDaten<DetailDaten | null>('detail:' + id, () => details(id))
  const samm = useGecacht<Sammlung[] | null>('sammlungen')
  const liste = alle.daten || []
  const it: Item | undefined = liste.filter((x) => x.id === id)[0] || det.daten || undefined
  const da = !!it
  useEffect(() => {
    if (da) fokusStart('abspielen')
  }, [da, id])
  useEffect(vorladen, [])

  if (!it) {
    if ((alle.fehler && !alle.daten) || det.fehler) return <Seite><Fehler fehler={alle.fehler || det.fehler} nochmal={alle.neu} /></Seite>
    if (alle.daten && !alle.laedt && !det.laedt) return <Seite><Leer titel={t('leer.unbekannt.titel')} text={t('leer.unbekannt.text')} aktion={{ label: t('knopf.zurueck'), onPress: back }} /></Seite>
    return <Seite ueberHero><SkeletonHero /></Seite>
  }
  const d = det.daten
  const m = it.meta || {}
  const genres = m.genres || []
  const rest = Math.max(0, it.duration - fortsetzenAb(it.id))
  const folgeText = it.series ? folge(it.season, it.episode) : ''
  const label = (it.series ? [t('det.folge'), it.series, folgeText] : [t('det.film'), genres.slice(0, 2).join(', ')]).filter(Boolean).join(' · ').toUpperCase()
  // Alle Sammlungen mit diesem Titel (Filmreihen aus TMDB und eigene).
  const drin = samm ? samm.filter((s) => s.items.indexOf(it.id) >= 0) : []
  // Folge: andere Serien; Film: andere Filme.
  const andere = it.series ? eintraege(liste, 'serien').filter((e) => e.serie!.name !== it.series) : eintraege(liste, 'filme').filter((e) => e.key !== it.id)
  const aehnlich = mehrWie(andere, genres, Math.floor((jahr(it) || 0) / 10) * 10, drin.reduce<string[]>((a, x) => a.concat(x.items), []))
  return (
    <Seite bereich={it.series ? 'serien' : 'filme'} titel={istTV() ? undefined : it.series || t('nav.filme')} ueberHero class="browse detail">
      <Kopf
        label={label}
        titel={anzeigeTitel(it)}
        bild={bild(it, 'backdrop', 1280)}
        farbe={it.color}
        fakten={[
          jahr(it),
          it.duration > 0 && dauer(it.duration),
          m.age != null && <Fsk n={m.age} />,
          !!m.rating && '★\u00a0' + m.rating.toFixed(1).replace('.', ','),
          it.duration > 0 && t('det.endet', { uhr: endetUm(rest) }),
          <AmpelZeile stufe={it.light} kurz />,
          (m.imdbId || (m.tmdbId && !it.series)) && geraet !== 'tv' && <Links imdb={m.imdbId} tmdb={m.tmdbId && !it.series ? 'movie/' + m.tmdbId : undefined} />,
        ]}
        unter={it.added ? t('det.hinzu', { datum: datum(it.added) }) : undefined}
        knoepfe={
          <>
            <Spielen it={it} />
            <GesehenIcon keys={[it.id]} an={!!it.watched} />
            <FavIcon k={it.id} />
            <MehrIcon it={it} />
            {it.series && (
              <Button fokusKey="k-serie" icon="serie" variante="geist" onPress={() => go(pfad('serie', it.series!))}>
                {t('det.zur.serie')}
              </Button>
            )}
          </>
        }
        info={<Fakten art={it.series ? 'serien' : 'filme'} genres={genres} people={d ? d.people : undefined} studios={(d && d.studios) || m.studios} countries={(d && d.countries) || m.countries} tags={(d && d.tags) || m.tags} />}
        text={m.overview}
        leitsatz={(d && d.tagline) || m.tagline}
        technik={d ? technik(d) : undefined}
      />
      <div class="detail-teile">
        {d && d.chapters && d.chapters.length > 1 && (
          <Reihe fokusKey="reihe-szenen" titel={t('det.szenen')}>
            {d.chapters.map((c, i) => {
              const sz = szenenName(c.name, i)
              return (
                <Karte
                  key={i}
                  fokusKey={'szene-' + i}
                  breit
                  titel={sz.titel}
                  unter={(sz.eigen ? t('det.szene', { n: i + 1 }) + ' · ' : '') + uhr(c.start)}
                  bild={c.image}
                  farbe={tonFuer(anzeigeTitel(it) + i)}
                  onPress={() => go(pfad('watch', it.id, Math.floor(c.start)))}
                />
              )
            })}
          </Reihe>
        )}
        <Besetzung people={d ? d.people : undefined} />
        {drin.length > 0 && (
          <section class="fl-reihe reihe">
            <h2>{t('det.sammlungen')}</h2>
            <Gruppe fokusKey="reihe-sammlung">
              {drin.map((x) => (
                <Button key={x.id} fokusKey={'zur-sammlung-' + x.id} icon="sammlung" onPress={() => go(pfad('sammlung', x.id))}>
                  {t('det.teil', { name: x.name })}
                </Button>
              ))}
            </Gruppe>
          </section>
        )}
        {aehnlich.length > 0 && (
          <Reihe fokusKey="reihe-aehnlich" titel={t('det.aehnlich')}>
            {aehnlich.map((e) => (
              <EintragKarte key={e.key} e={e} reihe="aehnlich" />
            ))}
          </Reihe>
        )}
      </div>
    </Seite>
  )
}

// ---------- Serie ----------
function zeitraum(s: Serie, info?: { year?: number; endYear?: number; status?: string } | null) {
  const jahre = s.staffeln.reduce<number[]>((a, x) => a.concat(x.folgen.map((f) => jahr(f) || 0).filter(Boolean)), [])
  const von = (info && info.year) || (jahre.length ? Math.min.apply(null, jahre) : 0)
  const bis = info && info.status && !/end|cancel/i.test(info.status) ? t('det.heute') : (info && info.endYear) || (jahre.length ? Math.max.apply(null, jahre) : 0)
  return !von ? '' : !bis || bis === von ? String(von) : von + ' – ' + bis
}

function useSerie(name: string) {
  const alle = useDaten<Item[]>('bibliothek', () => bibliothek(), { merken: true })
  const info = useDaten('serie:' + name, () => serienInfo(name))
  const liste = alle.daten || []
  const s = serien(liste).filter((x) => x.name === name)[0]
  return { alle, info: info.daten, s, liste }
}

function serieFehlt(alle: { daten?: Item[]; fehler: string; laedt: boolean; neu: () => void }) {
  if (alle.fehler && !alle.daten) return <Seite bereich="serien"><Fehler fehler={alle.fehler} nochmal={alle.neu} /></Seite>
  if (alle.daten && !alle.laedt) return <Seite bereich="serien"><Leer titel={t('leer.unbekannt.titel')} text={t('leer.unbekannt.text')} aktion={{ label: t('knopf.zurueck'), onPress: back }} /></Seite>
  return <Seite bereich="serien" ueberHero><SkeletonHero /></Seite>
}

function staffelName(n: number) {
  return n ? t('staffel', { n }) : t('staffel.specials')
}

function StaffelKarte({ s, nr, folgen }: { s: Serie; nr: number; folgen: Item[] }) {
  const offen = folgen.filter((f) => !f.watched).length
  const jahre = folgen.map((f) => jahr(f) || 0).filter(Boolean)
  return (
    <Karte
      fokusKey={'staffel-' + nr}
      titel={staffelName(nr)}
      unter={[jahre.length ? Math.min.apply(null, jahre) : '', anzahl(folgen.length, 'folgen.1', 'folgen')].filter(Boolean).join(' · ')}
      bild={bild(s.erste, 'poster', 360)}
      farbe={s.erste.color}
      fuss={[s.name, '']}
      gesehen={!offen}
      zahl={offen < folgen.length ? offen : 0}
      onPress={() => go(pfad('serie', s.name, nr))}
    />
  )
}

// Große Karte „Als Nächstes“ mit Beschreibung und Knöpfen.
function NaechsteFolge({ it }: { it: Item }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: 'naechste-bild', onPress: () => go(pfad('watch', it.id)) })
  const m = it.meta || {}
  return (
    <section class="fl-reihe reihe naechste rand">
      <h2>{t('det.naechstes')}</h2>
      <Gruppe fokusKey="reihe-naechste" class="naechste-zeile">
        <button ref={f.ref} {...f.dom} type="button" class={'naechste-bild bildbox' + (f.fokus ? ' ist-fokus' : '')} aria-label={folge(it.season, it.episode) + ' ' + anzeigeTitel(it)}>
          <Bild src={bild(it, 'backdrop', 640)} titel={it.series || anzeigeTitel(it)} farbe={it.color} />
          <Badges ampel={it.light} />
          <Fortschritt anteil={anteil(it)} />
        </button>
        <div class="naechste-text">
          <span class="fl-label">{t('staffel', { n: it.season || 0 }) + ' · ' + t('det.folge') + ' ' + (it.episode || '')}</span>
          <h3>{(it.episode ? it.episode + '. ' : '') + anzeigeTitel(it)}</h3>
          <span class="leise t-klein">{[it.duration > 0 && dauer(it.duration), it.duration > 0 && t('det.endet', { uhr: endetUm(Math.max(0, it.duration - fortsetzenAb(it.id))) })].filter(Boolean).join(' · ')}</span>
          {m.overview && <p class="folge-text">{m.overview}</p>}
        </div>
      </Gruppe>
    </section>
  )
}

export function DetailSerie({ name }: { name: string }) {
  const { alle, info, s, liste } = useSerie(name)
  const da = !!s
  useEffect(() => {
    if (da) fokusStart('abspielen')
  }, [da, name])
  useEffect(vorladen, [])
  if (!s) return serieFehlt(alle)
  const next = naechsteFolge(s)
  const folgen = s.staffeln.reduce<Item[]>((a, x) => a.concat(x.folgen), [])
  const m = next.meta || {}
  const genres = (info && info.genres) || m.genres || []
  const alt = info && info.age != null ? info.age : m.age
  const rating = (info && info.rating) || m.rating
  const studios = (info && info.studios) || m.studios || []
  const gesehen = folgen.every((f) => !!f.watched)
  const begonnen = fortsetzenAb(next.id) > 0
  const jahre = folgen.map((f) => jahr(f) || 0).filter(Boolean)
  const aehnlich = mehrWie(
    eintraege(liste, 'serien').filter((e) => e.serie!.name !== name),
    genres,
    Math.floor(((info && info.year) || (jahre.length ? Math.min.apply(null, jahre) : 0)) / 10) * 10,
    [],
  )
  return (
    <Seite bereich="serien" titel={istTV() ? undefined : t('nav.serien')} ueberHero class="browse detail">
      <Kopf
        label={[t('det.serie'), genres.slice(0, 2).join(', ')].filter(Boolean).join(' · ').toUpperCase()}
        titel={(info && info.title) || s.name}
        bild={(info && info.backdrop) || bild(next, 'backdrop', 1280)}
        farbe={(info && info.color) || next.color}
        fakten={[
          zeitraum(s, info),
          anzahl(s.staffeln.filter((x) => x.nummer).length || 1, 'staffeln.1', 'staffeln'),
          alt != null && <Fsk n={alt} />,
          !!rating && '★\u00a0' + rating.toFixed(1).replace('.', ','),
          info && statusText(info.status),
          <AmpelZeile stufe={next.light} kurz />,
        ]}
        knoepfe={
          <>
            <Spielen it={next} label={begonnen ? t('det.fortsetzen.folge', { folge: folge(next.season, next.episode) }) : t('knopf.abspielen') + ' · ' + folge(next.season, next.episode)} />
            <Button fokusKey="k-zufall" icon="zufall" onPress={() => zufaellig(folgen)}>
              {t('bib.zufall')}
            </Button>
            <GesehenIcon keys={['serie:' + s.name]} an={gesehen} />
            <FavIcon k={'serie:' + s.name} />
            <MehrIcon it={next} serie />
          </>
        }
        info={<Fakten art="serien" genres={genres} people={info ? info.people : undefined} studios={studios} tags={m.tags} />}
        text={(info && info.overview) || undefined}
      />
      <div class="detail-teile">
        <NaechsteFolge it={next} />
        <Reihe fokusKey="reihe-staffeln" titel={t('det.staffeln')}>
          {s.staffeln.map((x) => (
            <StaffelKarte key={x.nummer} s={s} nr={x.nummer} folgen={x.folgen} />
          ))}
        </Reihe>
        <Besetzung people={info ? info.people : undefined} />
        {aehnlich.length > 0 && (
          <Reihe fokusKey="reihe-aehnlich" titel={t('det.aehnlich')}>
            {aehnlich.map((e) => (
              <SerienKarte key={e.key} s={e.serie!} reihe="aehnlich" />
            ))}
          </Reihe>
        )}
      </div>
    </Seite>
  )
}

// ---------- Kopf ohne Kulisse: Bild links, Text rechts (Staffel, Sammlung, Liste, Person) ----------
export function InfoKopf(p: { bildTeil: ComponentChildren; rund?: boolean; label: string; titel: string; zeilen?: ComponentChildren; text?: string; knoepfe?: ComponentChildren }) {
  return (
    <div class="info-kopf rand">
      <div class={'info-bild bildbox' + (p.rund ? ' person' : '')}>{p.bildTeil}</div>
      <div class="info-text">
        <span class="fl-label">{p.label}</span>
        <h1 class="info-titel">{p.titel}</h1>
        {p.zeilen}
        <Text text={p.text} />
        {p.knoepfe && (
          <Gruppe fokusKey="knoepfe" class="detail-knoepfe">
            {p.knoepfe}
          </Gruppe>
        )}
      </div>
    </div>
  )
}

// ---------- Staffel ----------
function zwei(n?: number) {
  return n === undefined ? '' : (n < 10 ? '0' : '') + n
}

function FolgeZeile({ it }: { it: Item }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: 'folge-' + it.id, onPress: () => go(pfad('watch', it.id)) })
  const m = it.meta || {}
  const pos = fortsetzenAb(it.id)
  const a = anteil(it)
  const menue = useMenue()
  const fav = useGecacht<string[]>('favoriten') || []
  const istFav = fav.indexOf(it.id) >= 0
  const info = [
    it.duration > 0 && (a > 0 && it.duration - pos > 60 ? t('zeit.noch', { dauer: dauer(it.duration - pos) }) : dauer(it.duration)),
    it.added && datum(it.added),
    it.duration > 0 && t('det.endet', { uhr: endetUm(Math.max(0, it.duration - pos)) }),
  ]
    .filter(Boolean)
    .join(' · ')
  return (
    <Gruppe fokusKey={'zeile-' + it.id} class="folge-zeile" tag="div">
      <button
        ref={f.ref}
        {...f.dom}
        type="button"
        class={'fl-episode' + (f.fokus ? ' ist-fokus' : '')}
        onContextMenu={menue.rechtsklick}
        aria-label={[folge(it.season, it.episode), anzeigeTitel(it), t('ampel.' + it.light), it.watched && t('karte.gesehen')].filter(Boolean).join(', ')}
      >
        <div class="vorschau">
          <Bild src={bild(it, 'backdrop', 480)} titel={it.episode ? zwei(it.episode) : anzeigeTitel(it)} farbe={it.color} />
          <Badges ampel={it.light} gesehen={it.watched} />
          <Fortschritt anteil={a} />
        </div>
        <div class="fl-grow" aria-hidden="true">
          <h3>
            {it.episode !== undefined && <span class="nr">{zwei(it.episode)}</span>}
            {anzeigeTitel(it)}
          </h3>
          <p class="folge-dauer">{info}</p>
          {m.overview && <p class="folge-text">{m.overview}</p>}
        </div>
      </button>
      {!istTV() && (
        <span class="folge-knoepfe">
          <Button fokusKey={'fg-' + it.id} icon="haken" variante={it.watched ? 'primaer' : 'geist'} label={t(it.watched ? 'akt.ungesehen' : 'akt.gesehen')} onPress={() => alleGesehen([it.id], !it.watched)} />
          <span class={istFav ? 'herz-an' : ''}>
            <Button fokusKey={'ff-' + it.id} icon="herz" variante="geist" label={t(istFav ? 'akt.fav.weg' : 'akt.fav')} onPress={() => setFavorit(it.id, !istFav).catch(() => {})} />
          </span>
          <Button fokusKey={'fm-' + it.id} icon="mehr" variante="geist" label={t('knopf.mehr.menue')} onPress={menue.auf} />
        </span>
      )}
      {menue.offen && <AktionenMenue ziel={{ it }} anker={menue.anker} onZu={menue.zu} />}
    </Gruppe>
  )
}

export function StaffelSeite({ name, staffel }: { name: string; staffel: string }) {
  const { alle, info, s } = useSerie(name)
  const nr = Number(staffel) || 0
  const st = s && (s.staffeln.filter((x) => x.nummer === nr)[0] || s.staffeln[0])
  const da = !!st
  useEffect(() => {
    if (da) fokusStart('abspielen')
  }, [da, name, staffel])
  useEffect(vorladen, [])
  if (!s || !st) return serieFehlt(alle)
  const folgen = st.folgen
  const g = folgen.filter((f) => f.watched).length
  const b = folgen.filter((f) => !f.watched && fortsetzenAb(f.id) > 0).length
  const next = folgen.filter((f) => !f.watched)[0] || folgen[0]
  const m = next.meta || {}
  const genres = (info && info.genres) || m.genres || []
  const studios = (info && info.studios) || m.studios || []
  const jahre = folgen.map((f) => jahr(f) || 0).filter(Boolean)
  const gesamt = folgen.reduce((a, f) => a + (f.duration || 0), 0)
  const begonnen = fortsetzenAb(next.id) > 0
  return (
    <Seite bereich="serien" titel={s.name} class="browse staffel-seite">
      <InfoKopf
        bildTeil={<Bild src={bild(s.erste, 'poster', 480)} titel={s.name} farbe={s.erste.color} fuss={['ST. ' + nr, jahre.length ? String(Math.min.apply(null, jahre)) : '']} />}
        label={[t('det.serie'), genres[0]].filter(Boolean).join(' · ')}
        titel={staffelName(nr)}
        zeilen={
          <>
            <div class="info-zeile2">
              <Verweis text={s.name} onPress={() => go(pfad('serie', s.name))} />
              {' · ' + [jahre.length ? Math.min.apply(null, jahre) : '', anzahl(folgen.length, 'folgen.1', 'folgen'), gesamt > 0 && dauer(gesamt)].filter(Boolean).join(' · ')}
            </div>
            <div class="info-zeile2 leise">
              <Fsk n={info && info.age != null ? info.age : m.age} /> {[studios[0], t('det.gesehen.von', { g, n: folgen.length })].filter(Boolean).join(' · ')}
            </div>
          </>
        }
        knoepfe={
          <>
            <Spielen it={next} label={begonnen ? t('det.fortsetzen.folge', { folge: folge(next.season, next.episode) }) : t('knopf.abspielen') + ' · ' + folge(next.season, next.episode)} />
            <Button fokusKey="k-zufall" icon="zufall" onPress={() => zufaellig(folgen)}>
              {t('bib.zufall')}
            </Button>
            <Button fokusKey="k-gesehen" icon="haken" variante={g === folgen.length ? 'primaer' : 'sekundaer'} label={t('det.staffel.gesehen')} onPress={() => alleGesehen(folgen.map((f) => f.id), g !== folgen.length)} />
          </>
        }
      />
      {s.staffeln.length > 1 && (
        <div class="rand staffel-tabs">
          <Tabs fokusKey="staffeln" aktiv={String(st.nummer)} onWahl={(id) => go(pfad('serie', s.name, id), { ersetzen: true })} tabs={s.staffeln.map((x) => ({ id: String(x.nummer), label: staffelName(x.nummer) }))} />
        </div>
      )}
      <Gruppe fokusKey={'folgen-' + st.nummer} class="folgen-liste rand" label={t('det.folgen', { staffel: staffelName(st.nummer) })}>
        {folgen.map((e) => (
          <FolgeZeile key={e.id} it={e} />
        ))}
      </Gruppe>
      <p class="rand leiser t-klein staffel-fuss">{t('det.staffel.fuss', { n: folgen.length, g, b })}</p>
    </Seite>
  )
}
