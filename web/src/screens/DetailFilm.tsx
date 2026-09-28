import { useEffect, useState } from 'preact/hooks'
import { Abspielknoepfe } from '../components/Abspielen'
import { Fortschritt } from '../components/Karte'
import { Button } from '../components/Button'
import { Hero } from '../components/Hero'
import { Seite } from '../components/Seite'
import { SkeletonHero } from '../components/Skeleton'
import { Fehler, Leer } from '../components/Zustand'
import { anteil, anzeigeTitel, bibliothek, fortsetzenAb, bild, jahr, setGesehen, titelAus, useDaten, type Item } from '../lib/api'
import { Gruppe, fokusStart } from '../lib/focus'
import { dauer, t } from '../lib/i18n'
import { back } from '../lib/router'

export function GesehenKnopf({ it }: { it: Item }) {
  const [g, setG] = useState(!!it.watched)
  return (
    <Button
      fokusKey="gesehen"
      icon="haken"
      onPress={() => {
        setG(!g)
        setGesehen(it.id, !g).catch(() => setG(g))
      }}
    >
      {g ? t('knopf.ungesehen') : t('knopf.gesehen')}
    </Button>
  )
}

// Fortschritt als Haarlinie mit Restzeit und Ende („endet um 21:31“).
export function Restzeit({ it }: { it: Item }) {
  const a = anteil(it)
  if (!a) return null
  const rest = Math.max(0, it.duration - fortsetzenAb(it.id))
  const ende = new Date(Date.now() + rest * 1000)
  const uhrzeit = ende.getHours() + ':' + (ende.getMinutes() < 10 ? '0' : '') + ende.getMinutes()
  return (
    <div class="restzeit">
      <Fortschritt anteil={a} />
      <div class="fl-2">{t('zeit.noch', { dauer: dauer(rest) }) + ' · ' + t('zeit.endet', { uhr: uhrzeit })}</div>
    </div>
  )
}

export function DetailFilm({ id }: { id: string }) {
  const alle = useDaten<Item[]>('bibliothek', () => bibliothek(), { merken: true })
  const it = (alle.daten && alle.daten.filter((x) => x.id === id)[0]) || titelAus(id)
  const da = !!it
  useEffect(() => {
    if (da) fokusStart('abspielen')
  }, [da, id])

  if (!it) {
    if (alle.fehler) return <Seite><Fehler fehler={alle.fehler} nochmal={alle.neu} /></Seite>
    if (!alle.laedt) return <Seite><Leer titel={t('leer.unbekannt.titel')} text={t('leer.unbekannt.text')} aktion={{ label: t('knopf.zurueck'), onPress: back }} /></Seite>
    return <Seite><SkeletonHero /></Seite>
  }
  const m = it.meta || {}
  return (
    <Seite>
      <Hero
        class="detail"
        titel={anzeigeTitel(it)}
        label={m.originalTitle && m.originalTitle !== anzeigeTitel(it) ? m.originalTitle : undefined}
        fakten={[jahr(it), it.duration > 0 && dauer(it.duration), !!m.rating && t('bewertung', { n: m.rating.toFixed(1) }), m.genres && m.genres.slice(0, 3).join(', ')]}
        text={m.overview}
        bild={bild(it, 'backdrop', 1280)}
        farbe={it.color}
        ampel={it.light}
        ampelLang
      >
        <Gruppe fokusKey="knoepfe">
          <Abspielknoepfe it={it} extra={<GesehenKnopf key={it.id} it={it} />} />
        </Gruppe>
        <Restzeit it={it} />
      </Hero>
    </Seite>
  )
}
