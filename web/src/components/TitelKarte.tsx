import { anteil, anzeigeTitel, bild, fortsetzenAb, jahr, naechsteFolge, type Item, type Serie } from '../lib/api'
import { anzahl, dauer, folge, t } from '../lib/i18n'
import { go, pfad } from '../lib/router'
import { Karte } from './Karte'

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
export function TitelKarte({ it, reihe, breit, onFocus }: { it: Item; reihe: string; breit?: boolean; onFocus?: (it: Item) => void }) {
  const name = it.series || anzeigeTitel(it)
  return (
    <Karte
      fokusKey={reihe + '-' + it.id}
      titel={name}
      unter={unterzeile(it)}
      bild={bild(it, breit ? 'backdrop' : 'poster', breit ? 640 : 360)}
      farbe={it.color}
      fuss={[String(jahr(it) || ''), it.duration > 0 && !it.series ? Math.round(it.duration / 60) + ' ' + t('min.kurz') : '']}
      breit={breit}
      fortschritt={anteil(it)}
      ampel={it.light}
      gesehen={it.watched}
      onPress={() => go(zielVon(it))}
      onFocus={onFocus && (() => onFocus(it))}
    />
  )
}

export function SerienKarte({ s, reihe, onFocus }: { s: Serie; reihe: string; onFocus?: (it: Item) => void }) {
  const n = naechsteFolge(s)
  return (
    <Karte
      fokusKey={reihe + '-' + s.name}
      titel={s.name}
      unter={anzahl(s.staffeln.length, 'staffeln.1', 'staffeln') + ' · ' + anzahl(s.anzahl, 'folgen.1', 'folgen')}
      bild={bild(s.erste, 'poster', 360)}
      farbe={s.erste.color}
      fuss={[t('karte.serie'), '']}
      ampel={n.light}
      gesehen={s.staffeln.every((x) => x.folgen.every((f) => !!f.watched))}
      onPress={() => go(pfad('serie', s.name))}
      onFocus={onFocus && (() => onFocus(n))}
    />
  )
}
