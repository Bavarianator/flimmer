import { useEffect, useState } from 'preact/hooks'
import { FocusContext, useFocusable } from '@noriginmedia/norigin-spatial-navigation'
import { displayTitle, fmtTime, groupSeries, image, library, lightText, nextUp, resumePos, type Item, type Series } from './data'
import { Art, FocusButton, Row, focusSoon, useBack } from './ui'
import { ItemCard } from './Home'
import { back, go } from './route'

function Hero({ it, title, children }: { it: Item; title: string; children: preact.ComponentChildren }) {
  const m = it.meta || {}
  const facts = [
    m.year || it.year,
    it.duration ? Math.round(it.duration / 60) + ' min' : '',
    m.rating ? '★ ' + m.rating.toFixed(1) : '',
    m.genres && m.genres.slice(0, 3).join(', '),
  ].filter(Boolean)
  return (
    <div class="hero">
      <div class="backdrop" style={{ background: it.color || undefined }}>
        <Art src={image(it, 'backdrop', 1280)} name={title} />
      </div>
      <div class="hero-body">
        <h1>{title}</h1>
        <p class="facts">{facts.join(' · ')}</p>
        <p class="light-line">
          <span class={'light ' + it.light} />
          {lightText[it.light]}
        </p>
        {m.overview && <p class="overview">{m.overview}</p>}
        {children}
      </div>
    </div>
  )
}

function PlayButtons({ it }: { it: Item }) {
  const pos = resumePos(it.id)
  return (
    <div class="buttons">
      {pos > 0 ? (
        <>
          <FocusButton primary focusKey="play" onPress={() => go('/watch/' + it.id)}>▶ Fortsetzen ab {fmtTime(pos)}</FocusButton>
          <FocusButton focusKey="restart" onPress={() => go('/watch/' + it.id + '/0')}>Von vorn</FocusButton>
        </>
      ) : (
        <FocusButton primary focusKey="play" onPress={() => go('/watch/' + it.id)}>▶ Abspielen</FocusButton>
      )}
    </div>
  )
}

export function Detail({ id }: { id: string }) {
  const [it, setIt] = useState<Item | null>(null)
  const { ref, focusKey } = useFocusable({ focusKey: 'detail' })
  useBack(back)
  useEffect(() => {
    library().then((all) => setIt(all.filter((x) => x.id === id)[0] || null))
  }, [id])
  useEffect(() => {
    if (it) focusSoon('play')
  }, [it])
  return (
    <FocusContext.Provider value={focusKey}>
      <main ref={ref} class="page detail">
        {it ? (
          <Hero it={it} title={displayTitle(it)}>
            <PlayButtons it={it} />
          </Hero>
        ) : (
          <p class="empty">Lade …</p>
        )}
      </main>
    </FocusContext.Provider>
  )
}

export function SeriesPage({ name }: { name: string }) {
  const [s, setS] = useState<Series | null>(null)
  const { ref, focusKey } = useFocusable({ focusKey: 'series' })
  useBack(back)
  useEffect(() => {
    library().then((all) => setS(groupSeries(all).filter((x) => x.name === name)[0] || null))
  }, [name])
  useEffect(() => {
    if (s) focusSoon('play')
  }, [s])
  const next = s && nextUp(s)
  return (
    <FocusContext.Provider value={focusKey}>
      <main ref={ref} class="page detail">
        {!s || !next ? <p class="empty">Lade …</p> : <>
        <Hero it={next} title={s.name}>
          <p class="next">
            Als Nächstes: S{next.season} E{next.episode} · {displayTitle(next)}
          </p>
          <PlayButtons it={next} />
        </Hero>
        {s.seasons.map((x) => (
          <Row key={x.season} title={x.season ? 'Staffel ' + x.season : 'Specials'}>
            {x.episodes.map((e) => (
              <EpisodeCard key={e.id} it={e} />
            ))}
          </Row>
        ))}
        </>}
      </main>
    </FocusContext.Provider>
  )
}

function EpisodeCard({ it }: { it: Item }) {
  return <ItemCard it={{ ...it, series: undefined, title: `${it.episode}. ${displayTitle(it)}`, meta: undefined }} row={'s' + it.season} wide />
}
