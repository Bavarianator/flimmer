import { useEffect, useState } from 'preact/hooks'
import { Abspielknoepfe } from '../components/Abspielen'
import { Hero } from '../components/Hero'
import { Badges, Bild, Fortschritt } from '../components/Karte'
import { Seite } from '../components/Seite'
import { SkeletonHero } from '../components/Skeleton'
import { Tabs } from '../components/Tabs'
import { Fehler, Leer } from '../components/Zustand'
import { anteil, anzeigeTitel, bibliothek, bild, fortsetzenAb, naechsteFolge, serien, useDaten, type Item } from '../lib/api'
import { Gruppe, fokusStart, useFokus } from '../lib/focus'
import { anzahl, dauer, folge, t } from '../lib/i18n'
import { back, go, pfad } from '../lib/router'
import { GesehenKnopf } from './DetailFilm'

function zwei(n?: number) {
  return n === undefined ? '' : (n < 10 ? '0' : '') + n
}

// Eine Folge in der Liste: OK spielt sie ab (ab dem gemerkten Fortschritt).
function Folge({ it }: { it: Item }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: 'folge-' + it.id, onPress: () => go(pfad('watch', it.id)) })
  const m = it.meta || {}
  const rest = it.duration - fortsetzenAb(it.id)
  const a = anteil(it)
  return (
    <button
      ref={f.ref}
      {...f.dom}
      type="button"
      class={'fl-episode' + (f.fokus ? ' ist-fokus' : '')}
      aria-label={[folge(it.season, it.episode), anzeigeTitel(it), t('ampel.' + it.light), it.watched && t('karte.gesehen')].filter(Boolean).join(', ')}
    >
      <div class="vorschau">
        <Bild src={bild(it, 'backdrop', 480)} titel={it.episode ? zwei(it.episode) : anzeigeTitel(it)} farbe={it.color} />
        <Badges ampel={it.light} gesehen={it.watched} />
      </div>
      <div class="fl-grow" aria-hidden="true">
        <h3>
          {it.episode !== undefined && <span class="nr">{zwei(it.episode)}</span>}
          {anzeigeTitel(it)}
        </h3>
        <p class="folge-dauer">{a > 0 && rest > 60 ? t('zeit.noch', { dauer: dauer(rest) }) : it.duration > 0 ? dauer(it.duration) : ''}</p>
        {m.overview && <p class="folge-text">{m.overview}</p>}
        <Fortschritt anteil={a} />
      </div>
    </button>
  )
}

export function DetailSerie({ name }: { name: string }) {
  const alle = useDaten<Item[]>('bibliothek', () => bibliothek(), { merken: true })
  const s = alle.daten ? serien(alle.daten).filter((x) => x.name === name)[0] : undefined
  const next = s && naechsteFolge(s)
  const [staffel, setStaffel] = useState<number | null>(null)
  const da = !!next
  useEffect(() => {
    if (da) fokusStart('abspielen')
  }, [da, name])

  if (!s || !next) {
    if (alle.fehler && !alle.daten) return <Seite bereich="serien"><Fehler fehler={alle.fehler} nochmal={alle.neu} /></Seite>
    if (alle.daten && !alle.laedt) return <Seite bereich="serien"><Leer titel={t('leer.unbekannt.titel')} text={t('leer.unbekannt.text')} aktion={{ label: t('knopf.zurueck'), onPress: back }} /></Seite>
    return <Seite bereich="serien"><SkeletonHero /></Seite>
  }
  const gewaehlt = staffel === null ? next.season || 0 : staffel
  const st = s.staffeln.filter((x) => x.nummer === gewaehlt)[0] || s.staffeln[0]
  const m = next.meta || {}
  const jahre = s.staffeln.reduce<number[]>((acc, x) => acc.concat(x.folgen.map((f) => (f.meta && f.meta.year) || f.year || 0).filter(Boolean)), [])
  const zeitraum = jahre.length ? (Math.min.apply(null, jahre) === Math.max.apply(null, jahre) ? String(jahre[0]) : Math.min.apply(null, jahre) + '–' + Math.max.apply(null, jahre)) : ''
  const staffelName = (n: number) => (n ? t('staffel', { n }) : t('staffel.specials'))
  return (
    <Seite bereich="serien" class="serie">
      <div class="serie-spalten fl-reihe-flex rand">
        <div class="serie-info">
          <Hero
            titel={s.name}
            fakten={[zeitraum, anzahl(s.staffeln.length, 'staffeln.1', 'staffeln'), anzahl(s.anzahl, 'folgen.1', 'folgen')]}
            ampel={next.light}
            text={m.overview}
            bild={bild(next, 'backdrop', 1280)}
            farbe={next.color}
          >
            <p class="fl-2 als-naechstes">{t('als.naechstes', { folge: folge(next.season, next.episode) + ' · ' + anzeigeTitel(next) })}</p>
            <Gruppe fokusKey="knoepfe">
              <Abspielknoepfe it={next} extra={<GesehenKnopf key={next.id} it={next} />} />
            </Gruppe>
          </Hero>
        </div>
        <section class="serie-folgen" aria-label={staffelName(st.nummer)}>
          {s.staffeln.length > 1 ? (
            <Tabs
              fokusKey="staffeln"
              aktiv={String(st.nummer)}
              onWahl={(id) => setStaffel(Number(id))}
              tabs={s.staffeln.map((x) => ({ id: String(x.nummer), label: staffelName(x.nummer) }))}
            />
          ) : (
            <div class="fl-tabs">
              <span class="fl-tab an">{staffelName(st.nummer)}</span>
            </div>
          )}
          <Gruppe fokusKey={'folgen-' + st.nummer} class="folgen">
            {st.folgen.map((e) => (
              <Folge key={e.id} it={e} />
            ))}
          </Gruppe>
        </section>
      </div>
    </Seite>
  )
}
