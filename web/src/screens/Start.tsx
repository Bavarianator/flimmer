// Startseite (Entwurf Main, Handy-Start, TV-Start): Meine Medien, Weiterschauen, Als Nächstes,
// Kürzlich hinzugefügt in Filme bzw. Serien. Der TV zeigt oben den fokussierten Titel.
import { useEffect, useRef, useState } from 'preact/hooks'
import { Hero } from '../components/Hero'
import { Icon, type IconName } from '../components/Icon'
import { Karte, tonFuer, trenne } from '../components/Karte'
import { Reihe } from '../components/Reihe'
import { Seite } from '../components/Seite'
import { SkeletonReihe } from '../components/Skeleton'
import { SerienKarte, TitelKarte } from '../components/TitelKarte'
import { Fehler, Leer } from '../components/Zustand'
import {
  anzeigeTitel,
  bibliothek,
  bild,
  geraeteTest,
  jahr,
  listenApi,
  offeneGruppen,
  sammlungenApi,
  serien,
  startseite,
  titelAus,
  useDaten,
  type HomeRow,
  type Item,
  type OffeneGruppe,
} from '../lib/api'
import { istTV } from '../lib/device'
import { Gruppe, fokusStart, useFokus } from '../lib/focus'
import { anzahl, dauer, ergaenze, folge, t } from '../lib/i18n'
import { go, pfad } from '../lib/router'
import { startReihen } from './einstellungen/startseite'
import './browse.css'

ergaenze({
  'start.medien': 'Meine Medien',
  'start.gruppen': 'Gemeinsam schauen – jetzt offen',
  'start.gruppe.unter': '{wer} · {n} dabei',
  'reihe.recent-movies': 'Kürzlich hinzugefügt in Filme',
  'reihe.recent-series': 'Kürzlich hinzugefügt in Serien',
  'anz.filme': '{n} Filme',
  'anz.filme.1': '1 Film',
  'anz.serien': '{n} Serien',
  'anz.serien.1': '1 Serie',
  'anz.sammlungen': '{n} Sammlungen',
  'anz.sammlungen.1': '1 Sammlung',
  'anz.listen': '{n} Listen',
  'anz.listen.1': '1 Liste',
})

const breiteReihen = ['continue', 'nextup']
const ziele: Record<string, string> = { nextup: '/serien', 'recent-movies': '/filme', 'recent-series': '/serien' }

function StartHero({ it, label }: { it: Item; label: string }) {
  const m = it.meta || {}
  return (
    <Hero
      class="start-held"
      titel={it.series || anzeigeTitel(it)}
      label={label}
      fakten={[it.series ? folge(it.season, it.episode) + ' · ' + anzeigeTitel(it) : jahr(it), m.age != null && 'FSK ' + m.age, !!m.rating && '★\u00a0' + m.rating.toFixed(1).replace('.', ','), it.duration > 0 && dauer(it.duration)]}
      ampel={it.light}
      text={m.overview}
      bild={bild(it, 'backdrop', 1280)}
      farbe={it.color}
    />
  )
}

// Kachel unter „Meine Medien“.
function Kachel(p: { id: string; name: string; zahl?: string; icon: IconName; ziel: string }) {
  const f = useFokus<HTMLAnchorElement>({ fokusKey: 'kachel-' + p.id, onPress: () => go(p.ziel) })
  return (
    <a
      ref={f.ref}
      {...f.dom}
      href={'#' + p.ziel}
      onClick={(e) => (e.preventDefault(), go(p.ziel))}
      class={'kachel' + (f.fokus ? ' ist-fokus' : '')}
      style={{ background: tonFuer(p.name) }}
    >
      <Icon name={p.icon} />
      <span class="kachel-name">{trenne(p.name)}</span>
      {p.zahl && <span class="kachel-zahl">{p.zahl}</span>}
    </a>
  )
}

export function Start() {
  const start = useDaten<HomeRow[]>('start', startseite, { merken: true })
  const alle = useDaten<Item[]>('bibliothek', () => bibliothek(), { merken: true })
  const samm = useDaten('sammlungen', sammlungenApi.alle)
  const listen = useDaten('listen', listenApi.alle)
  const gruppen = useDaten('gruppen', offeneGruppen)
  const rows = startReihen(start.daten || []) // Reihenfolge und Auswahl aus Einstellungen › Startseite
  const items = alle.daten
  const tv = istTV()

  // TV: Der Hero zeigt den fokussierten Titel; kurz verzögert, damit schnelles Scrollen flüssig bleibt.
  const [held, setHeld] = useState<{ it: Item; label: string } | null>(null)
  const timer = useRef(0)
  const zeige = (label: string) => (it: Item) => {
    clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setHeld({ it, label }), 250)
  }
  useEffect(() => () => clearTimeout(timer.current), [])

  // TV: auf die Reihen warten, sonst landet der Anfangsfokus je nach Ladezeit unten auf „Meine Medien“.
  const bereit = tv ? !!start.daten || (!!start.fehler && !!items) : !!items || rows.length > 0
  useEffect(() => {
    if (bereit) fokusStart(tv && rows.length ? 'reihe-' + rows[0].id : 'kachel-filme')
  }, [bereit])
  useEffect(() => geraeteTest.wennNoetig(), [])

  const seite = (inhalt: preact.ComponentChildren) => (
    <Seite bereich="start" titel={istTV() ? undefined : t('nav.startseite')} class="start browse">
      {inhalt}
    </Seite>
  )
  const fehler = !items && !start.daten && (alle.fehler || start.fehler)
  if (fehler) return seite(<Fehler fehler={fehler} nochmal={() => (alle.neu(), start.neu())} />)
  if (items && !items.length && !rows.length) return seite(<Leer titel={t('leer.bibliothek.titel')} text={t('leer.bibliothek.text')} />)
  if (!bereit)
    return seite(
      <>
        <SkeletonReihe breit n={4} />
        <SkeletonReihe n={8} />
      </>,
    )

  const liste = items || []
  const reihen = serien(liste)
  const nachName: Record<string, (typeof reihen)[0]> = {}
  for (const s of reihen) nachName[s.name] = s
  const filme = liste.filter((i) => !i.series).length
  const erstes = rows.length ? rows[0].items[0] : liste[0]
  const h = held || (erstes && { it: erstes, label: rows.length ? t('reihe.' + rows[0].id) : '' })
  const titelVon = (r: HomeRow) => (t('reihe.' + r.id) === 'reihe.' + r.id ? r.title : t('reihe.' + r.id))

  // TV (Entwurf TV-Start): erst die Reihen unter dem Hero, „Meine Medien“ danach.
  const medien = (
    <section class="fl-reihe medien">
      <h2>{t('start.medien')}</h2>
      <Gruppe fokusKey="medien" class="kacheln">
        <Kachel id="filme" name={t('nav.filme')} zahl={items ? anzahl(filme, 'anz.filme.1', 'anz.filme') : ''} icon="film" ziel="/filme" />
        <Kachel id="serien" name={t('nav.serien')} zahl={items ? anzahl(reihen.length, 'anz.serien.1', 'anz.serien') : ''} icon="serie" ziel="/serien" />
        <Kachel id="livetv" name={t('nav.livetv')} icon="live" ziel="/livetv" />
        {samm.daten && <Kachel id="sammlungen" name={t('nav.sammlungen')} zahl={anzahl(samm.daten.length, 'anz.sammlungen.1', 'anz.sammlungen')} icon="sammlung" ziel="/sammlungen" />}
        {listen.daten && <Kachel id="listen" name={t('nav.listen')} zahl={anzahl(listen.daten.length, 'anz.listen.1', 'anz.listen')} icon="liste" ziel="/listen" />}
      </Gruppe>
    </section>
  )
  return seite(
    <>
      {tv && h && <StartHero it={h.it} label={h.label} />}
      {!tv && medien}
      {rows.map((r, i) => {
        const titel = titelVon(r)
        return [
          <Reihe key={r.id} fokusKey={'reihe-' + r.id} titel={titel} ziel={ziele[r.id]}>
            {r.id === 'recent-series'
              ? r.items.map((it) => (nachName[it.series!] ? <SerienKarte key={it.id} s={nachName[it.series!]} reihe={r.id} onFocus={zeige(titel)} /> : <TitelKarte key={it.id} it={it} reihe={r.id} onFocus={zeige(titel)} />))
              : r.items.map((it) => <TitelKarte key={it.id} it={it} reihe={r.id} breit={breiteReihen.indexOf(r.id) >= 0} onFocus={zeige(titel)} />)}
          </Reihe>,
          i === 0 && <GruppenReihe key="gruppen" gruppen={gruppen.daten || []} />, // hinter der ersten Reihe: der Hero bleibt oben
        ]
      })}
      {!rows.length && <GruppenReihe gruppen={gruppen.daten || []} />}
      {!start.daten && <SkeletonReihe n={8} />}
      {tv && medien}
    </>,
  )
}

// Offene „Gemeinsam schauen“-Gruppen (Startseite und Lobby). Leer = nichts.
export function GruppenReihe({ gruppen }: { gruppen: OffeneGruppe[] }) {
  if (!gruppen.length) return null
  return (
    <Reihe titel={t('start.gruppen')} fokusKey="reihe-gruppen">
      {gruppen.map((g) => {
        const it = titelAus(g.mediaId)
        return (
          <Karte
            key={g.id}
            fokusKey={'gruppe-' + g.id}
            breit
            titel={it ? it.series || anzeigeTitel(it) : t('nav.gemeinsam')}
            unter={t('start.gruppe.unter', { wer: g.host, n: g.members.length })}
            bild={it ? bild(it, 'backdrop', 480) : undefined}
            farbe={it && it.color}
            onPress={() => go(pfad('party', g.id))}
          />
        )
      })}
    </Reihe>
  )
}
