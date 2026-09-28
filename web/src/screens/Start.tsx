import { useEffect, useRef, useState } from 'preact/hooks'
import { Abspielknoepfe } from '../components/Abspielen'
import { Button } from '../components/Button'
import { Hero } from '../components/Hero'
import { AmpelZeile } from '../components/Karte'
import { Reihe } from '../components/Reihe'
import { Seite } from '../components/Seite'
import { SkeletonHero, SkeletonReihe } from '../components/Skeleton'
import { SerienKarte, TitelKarte, zielVon } from '../components/TitelKarte'
import { Fehler, Leer } from '../components/Zustand'
import { anzeigeTitel, bibliothek, bild, geraeteTest, jahr, serien, startseite, useDaten, type HomeRow, type Item } from '../lib/api'
import { istTV } from '../lib/device'
import { Gruppe, fokusStart } from '../lib/focus'
import { dauer, folge, t } from '../lib/i18n'
import { go } from '../lib/router'

const breiteReihen = ['continue', 'nextup']

function StartHero({ it, label }: { it: Item; label: string }) {
  const m = it.meta || {}
  return (
    <Hero
      titel={it.series || anzeigeTitel(it)}
      label={label}
      fakten={[it.series ? folge(it.season, it.episode) + ' · ' + anzeigeTitel(it) : jahr(it), it.duration > 0 && dauer(it.duration), m.genres && m.genres.slice(0, 2).join(', ')]}
      ampel={it.light}
      text={m.overview}
      bild={bild(it, 'backdrop', 1280)}
      farbe={it.color}
    >
      {/* TV: Der Fokus liegt auf den Karten, der Hero zeigt nur den fokussierten Titel. */}
      {!istTV() && (
        <Gruppe fokusKey="hero">
          <Abspielknoepfe it={it} extra={<Button fokusKey="details" onPress={() => go(zielVon(it))}>{t('knopf.details')}</Button>} />
        </Gruppe>
      )}
    </Hero>
  )
}

// „Dieses Gerät“: Ampel-Legende, Geräte-Test und Kopplung (vorher in der Kopfzeile).
function Fuss() {
  const [laeuft, setLaeuft] = useState(geraeteTest.laeuft())
  const testen = () => {
    setLaeuft(true)
    geraeteTest.starten().then(() => setLaeuft(false))
  }
  return (
    <footer class="fuss rand t-klein">
      <div class="fl-reihe-flex fl-wrap">
        <AmpelZeile stufe="green" kurz />
        <AmpelZeile stufe="yellow" kurz />
        <AmpelZeile stufe="red" kurz />
        <span>{t('ampel.legende')}</span>
      </div>
      <Gruppe fokusKey="fuss" class="fuss-knoepfe">
        <Button fokusKey="fuss-test" variante="geist" icon="neustart" aus={laeuft} onPress={testen}>
          {laeuft ? t('nav.geraetetest.laeuft') : t('nav.geraetetest')}
        </Button>
        {!istTV() && (
          <Button fokusKey="fuss-koppeln" variante="geist" icon="fernseher" onPress={() => go('/koppeln')}>
            {t('nav.koppeln')}
          </Button>
        )}
      </Gruppe>
    </footer>
  )
}

export function Start() {
  const start = useDaten<HomeRow[]>('start', startseite, { merken: true })
  const alle = useDaten<Item[]>('bibliothek', () => bibliothek(), { merken: true })
  const rows = start.daten || []
  const items = alle.daten
  const erstes = rows.length ? rows[0].items[0] : items && items[0]

  // Der Hero zeigt den fokussierten Titel; kurz verzögert, damit schnelles Scrollen flüssig bleibt.
  const [held, setHeld] = useState<{ it: Item; label: string } | null>(null)
  const timer = useRef(0)
  const zeige = (label: string) => (it: Item) => {
    clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setHeld({ it, label }), 250)
  }
  useEffect(() => () => clearTimeout(timer.current), [])

  const bereit = !!erstes
  useEffect(() => {
    const erste = rows.length ? 'reihe-' + rows[0].id : items && items.some((i) => !i.series) ? 'reihe-filme' : 'reihe-serien'
    if (bereit) fokusStart(istTV() ? erste : 'abspielen')
  }, [bereit])
  useEffect(() => geraeteTest.wennNoetig(), [])

  const fehler = !items && !start.daten && (alle.fehler || start.fehler)
  if (fehler)
    return (
      <Seite bereich="start">
        <Fehler fehler={fehler} nochmal={() => (alle.neu(), start.neu())} />
      </Seite>
    )
  if (items && !items.length && !rows.length)
    return (
      <Seite bereich="start">
        <Leer titel={t('leer.bibliothek.titel')} text={t('leer.bibliothek.text')} />
        <Fuss />
      </Seite>
    )
  if (!erstes)
    return (
      <Seite bereich="start">
        <SkeletonHero />
        <SkeletonReihe breit n={5} />
        <SkeletonReihe n={8} />
      </Seite>
    )

  const filme = (items || []).filter((i) => !i.series)
  const reihen = serien(items || [])
  const heroLabel = rows.length ? t(('reihe.' + rows[0].id) as 'reihe.continue') : t('reihe.filme')
  const h = held || { it: erstes, label: heroLabel }
  return (
    <Seite bereich="start" class="start">
      <StartHero it={h.it} label={h.label} />
      {rows.map((r) => {
        const titel = t('reihe.' + r.id) === 'reihe.' + r.id ? r.title : t('reihe.' + r.id)
        return (
          <Reihe key={r.id} fokusKey={'reihe-' + r.id} titel={titel}>
            {r.items.map((it) => (
              <TitelKarte key={it.id} it={it} reihe={r.id} breit={breiteReihen.indexOf(r.id) >= 0} onFocus={zeige(titel)} />
            ))}
          </Reihe>
        )
      })}
      {!items && <SkeletonReihe n={8} />}
      {filme.length > 0 && (
        <Reihe fokusKey="reihe-filme" titel={t('reihe.filme')}>
          {filme.slice(0, 30).map((it) => (
            <TitelKarte key={it.id} it={it} reihe="filme" onFocus={zeige(t('reihe.filme'))} />
          ))}
        </Reihe>
      )}
      {reihen.length > 0 && (
        <Reihe fokusKey="reihe-serien" titel={t('reihe.serien')}>
          {reihen.slice(0, 30).map((s) => (
            <SerienKarte key={s.name} s={s} reihe="serien" onFocus={zeige(t('reihe.serien'))} />
          ))}
        </Reihe>
      )}
      <Fuss />
    </Seite>
  )
}
