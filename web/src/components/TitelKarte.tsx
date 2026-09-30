import { useEffect } from 'preact/hooks'
import { alleGesehen, anteil, anzeigeTitel, bild, fortsetzenAb, jahr, naechsteFolge, setFavorit, useGecacht, vorladen, type Item, type Serie } from '../lib/api'
import { geraet } from '../lib/device'
import { anzahl, dauer, ergaenze, folge, t } from '../lib/i18n'
import { go, pfad } from '../lib/router'
import type { Auswahl } from './Aktionen'
import { Icon } from './Icon'
import { Karte } from './Karte'
import { useMenue, type Anker, type MenueEintrag } from './Menue'
import { spaeter } from './Spaeter'

ergaenze({
  'akt.abspielen': 'Abspielen',
  'akt.gesehen': 'Als gesehen markieren',
  'akt.ungesehen': 'Als ungesehen markieren',
  'akt.fav': 'Zu Favoriten hinzufügen',
  'akt.fav.weg': 'Aus Favoriten entfernen',
  'karte.ungesehen': '{n} ungesehene Folgen',
  'knopf.mehr.menue': 'Mehr',
})

// Menü und Dialoge laden erst beim ersten Öffnen.
const AktionenMenue = spaeter(() => import('./Aktionen').then((m) => m.AktionenMenue))

// Hover-Knöpfe am Desktop: Abspielen in der Mitte, rechts unten Gesehen, Favorit und „…“.
function Schnell(p: { k: string; gesehen: boolean; spielen: () => void; onMehr: (a: Anker) => void }) {
  const fav = useGecacht<string[]>('favoriten') || []
  const f = fav.indexOf(p.k) >= 0
  return (
    <>
      <span class="schleier" />
      <button type="button" tabIndex={-1} class="spielen" aria-label={t('akt.abspielen')} onClick={p.spielen}>
        <Icon name="abspielen" />
      </button>
      <span class="klein">
        <button type="button" tabIndex={-1} class={p.gesehen ? 'an' : ''} aria-label={t(p.gesehen ? 'akt.ungesehen' : 'akt.gesehen')} title={t(p.gesehen ? 'akt.ungesehen' : 'akt.gesehen')} onClick={() => alleGesehen([p.k], !p.gesehen)}>
          <Icon name="haken" />
        </button>
        <button type="button" tabIndex={-1} class={'fav' + (f ? ' an' : '')} aria-label={t(f ? 'akt.fav.weg' : 'akt.fav')} title={t(f ? 'akt.fav.weg' : 'akt.fav')} onClick={() => setFavorit(p.k, !f).catch(() => {})}>
          <Icon name="herz" />
        </button>
        <button type="button" tabIndex={-1} aria-label={t('knopf.mehr.menue')} title={t('knopf.mehr.menue')} onClick={(e) => p.onMehr(e.currentTarget as HTMLElement)}>
          <Icon name="mehr" />
        </button>
      </span>
    </>
  )
}

interface KartenExtras {
  aw?: Auswahl // Mehrfachauswahl der Seite
  extra?: MenueEintrag[] // seiteneigene Menüeinträge
  unter?: string // eigene zweite Zeile
}

export function zielVon(it: Item): string {
  return it.series ? pfad('serie', it.series) : pfad('film', it.id)
}

// Zweite Zeile: Folge bzw. Restzeit, sonst Jahr und Laufzeit.
export function unterzeile(it: Item): string {
  const rest = it.duration - fortsetzenAb(it.id)
  const noch = fortsetzenAb(it.id) > 0 && rest > 60 ? t('zeit.noch', { dauer: dauer(rest) }) : ''
  if (it.series) return [folge(it.season, it.episode), noch || anzeigeTitel(it)].join(' · ')
  return noch || [jahr(it), it.duration > 0 ? dauer(it.duration) : ''].filter(Boolean).join(' · ')
}

// Karte für einen Titel aus der Bibliothek (Film oder Folge).
export function TitelKarte({ it, reihe, breit, onFocus, aw, extra, unter }: { it: Item; reihe: string; breit?: boolean; onFocus?: (it: Item) => void } & KartenExtras) {
  const name = it.series || anzeigeTitel(it)
  const m = useMenue()
  useEffect(vorladen, [])
  const wahl = !!aw && aw.an
  const spielen = () => go(pfad('watch', it.id))
  return (
    <Karte
      fokusKey={reihe + '-' + it.id}
      titel={name}
      unter={unter || unterzeile(it)}
      bild={bild(it, breit ? 'backdrop' : 'poster', breit ? 640 : 360)}
      farbe={it.color}
      fuss={[String(jahr(it) || ''), it.duration > 0 && !it.series ? Math.round(it.duration / 60) + ' ' + t('min.kurz') : '']}
      breit={breit}
      fortschritt={anteil(it)}
      ampel={it.light}
      gesehen={it.watched}
      gewaehlt={wahl ? aw!.hat(it.id) : undefined}
      onKontext={wahl ? undefined : m.rechtsklick}
      ueber={geraet === 'dt' && <Schnell k={it.id} gesehen={!!it.watched} spielen={spielen} onMehr={m.auf} />}
      onPress={wahl ? () => aw!.um(it.id) : () => go(zielVon(it))}
      onFocus={onFocus && (() => onFocus(it))}
    >
      {m.offen && <AktionenMenue ziel={{ it }} anker={m.anker} onZu={m.zu} aw={aw} extra={extra} />}
    </Karte>
  )
}

export function SerienKarte({ s, reihe, onFocus, aw, extra, unter }: { s: Serie; reihe: string; onFocus?: (it: Item) => void } & KartenExtras) {
  const n = naechsteFolge(s)
  const m = useMenue()
  useEffect(vorladen, [])
  const k = 'serie:' + s.name
  const wahl = !!aw && aw.an
  const offen = s.staffeln.reduce((a, x) => a + x.folgen.filter((f) => !f.watched).length, 0)
  const gesehen = !offen
  return (
    <Karte
      fokusKey={reihe + '-' + s.name}
      titel={s.name}
      unter={unter || anzahl(s.staffeln.length, 'staffeln.1', 'staffeln') + ' · ' + anzahl(s.anzahl, 'folgen.1', 'folgen')}
      bild={bild(s.erste, 'poster', 360)}
      farbe={s.erste.color}
      fuss={[t('karte.serie'), '']}
      ampel={n.light}
      gesehen={gesehen}
      zahl={offen}
      gewaehlt={wahl ? aw!.hat(k) : undefined}
      onKontext={wahl ? undefined : m.rechtsklick}
      ueber={geraet === 'dt' && <Schnell k={k} gesehen={gesehen} spielen={() => go(pfad('watch', n.id))} onMehr={m.auf} />}
      onPress={wahl ? () => aw!.um(k) : () => go(pfad('serie', s.name))}
      onFocus={onFocus && (() => onFocus(n))}
    >
      {m.offen && <AktionenMenue ziel={{ it: s.erste, serie: true }} anker={m.anker} onZu={m.zu} aw={aw} extra={extra} />}
    </Karte>
  )
}
