// Wiedergabelisten (/listen) und eine Liste (/liste/:id), Entwurf Wiedergabelisten. Pro Profil.
// Sortieren per „Nach oben/unten“ im Zeilenmenü (Ziehen fehlt: auf dem TV nicht bedienbar), Entf entfernt.
import { useEffect, useState } from 'preact/hooks'
import { AktionenMenue, NameDialog, spieleAlle, zufaellig } from '../components/Aktionen'
import { Button } from '../components/Button'
import { InfoKopf } from '../components/Details'
import { Bild, Karte, tonFuer } from '../components/Karte'
import { Menue, useMenue, type MenueEintrag } from '../components/Menue'
import { Raster } from '../components/Reihe'
import { Seite, useIch } from '../components/Seite'
import { SkeletonKarten } from '../components/Skeleton'
import { Fehler, Leer } from '../components/Zustand'
import { anzeigeTitel, bibliothek, bild, jahr, listenApi, useDaten, type Item, type Wiedergabeliste } from '../lib/api'
import { geraet } from '../lib/device'
import { Gruppe, fokusBald, fokusStart, useFokus } from '../lib/focus'
import { anzahl, dauer, ergaenze, folge, t } from '../lib/i18n'
import { back, go, pfad } from '../lib/router'
import { listenTitel } from './Sammlungen'
import './browse.css'

ergaenze({
  'liste.titel': 'Wiedergabeliste',
  'liste.von': 'Wiedergabeliste · von {name}',
  'liste.neu': 'Neue Wiedergabeliste',
  'liste.leer': 'Noch keine Wiedergabelisten',
  'liste.leer.text': 'Lege eine Liste an und füge Titel über „…“ → „Zu Wiedergabeliste hinzufügen“ hinzu.',
  'liste.fehlt': 'Wiedergabelisten gibt es auf diesem Server noch nicht',
  'liste.leer.titel': 'Diese Liste ist leer. Füge Titel über „…“ an einer Karte hinzu.',
  'liste.umbenennen': 'Umbenennen',
  'liste.loeschen': 'Liste löschen',
  'liste.hoch': 'Nach oben',
  'liste.runter': 'Nach unten',
  'liste.entfernen': 'Aus Liste entfernen',
  'liste.hinweis': 'Reihenfolge über „…“ ändern · Entf entfernt',
  'liste.nr': 'Nr.',
  'liste.spalte.titel': 'Titel',
  'liste.spalte.info': 'Info',
  'liste.spalte.dauer': 'Dauer',
  'liste.unbekannt': 'Diese Wiedergabeliste gibt es nicht mehr.',
})

function Mosaik({ its }: { its: Item[] }) {
  const vier = its.slice(0, 4)
  if (vier.length < 4) return <Bild src={vier[0] ? bild(vier[0], 'poster', 480) : ''} titel={vier[0] ? vier[0].series || anzeigeTitel(vier[0]) : '♪'} farbe={vier[0] ? vier[0].color : undefined} />
  return (
    <div class="mosaik">
      {vier.map((x) => (
        <div key={x.id} class="mosaik-teil bildbox">
          <Bild src={bild(x, 'poster', 240)} titel={x.series || anzeigeTitel(x)} farbe={x.color} />
        </div>
      ))}
    </div>
  )
}

function listenItems(l: Wiedergabeliste, alle: Item[]): Item[] {
  return listenTitel(l.items, alle).map((x) => x.it || x.serie!.erste)
}

export function Listen() {
  const listen = useDaten('listen', listenApi.alle)
  const alle = useDaten<Item[]>('bibliothek', () => bibliothek(), { merken: true })
  const [neu, setNeu] = useState(false)
  const l = listen.daten
  useEffect(() => {
    if (l && l.length) fokusStart('listen-' + l[0].id)
  }, [!!(l && l.length)])
  let inhalt: preact.ComponentChildren
  if (listen.fehler && !l) inhalt = <Fehler fehler={listen.fehler} nochmal={listen.neu} />
  else if (l === null) inhalt = <Leer icon="liste" titel={t('liste.fehlt')} text={t('samm.fehlt.text')} />
  else if (!l || !alle.daten)
    inhalt = (
      <div class="raster rand">
        <SkeletonKarten n={8} />
      </div>
    )
  else if (!l.length) inhalt = <Leer icon="liste" titel={t('liste.leer')} text={t('liste.leer.text')} aktion={{ label: t('liste.neu'), onPress: () => setNeu(true) }} />
  else
    inhalt = (
      <Raster fokusKey="listen">
        {l.map((x) => {
          const its = listenItems(x, alle.daten!)
          const gesamt = its.reduce((a, i) => a + (i.duration || 0), 0)
          return (
            <Karte
              key={x.id}
              fokusKey={'listen-' + x.id}
              titel={x.name}
              unter={[anzahl(x.items.length, 'samm.titel.1', 'samm.titel.n'), gesamt > 0 && dauer(gesamt)].filter(Boolean).join(' · ')}
              bild={its[0] ? bild(its[0], 'poster', 360) : ''}
              farbe={its[0] ? its[0].color : tonFuer(x.name)}
              onPress={() => go(pfad('liste', x.id))}
            />
          )
        })}
      </Raster>
    )
  return (
    <Seite
      bereich="listen"
      titel={t('nav.listen')}
      class="browse"
      aktionen={l && l.length > 0 ? <Button fokusKey="liste-neu" icon="plus" onPress={() => setNeu(true)}>{t('liste.neu')}</Button> : undefined}
    >
      {inhalt}
      {neu && <NameDialog titel={t('liste.neu')} onZu={() => setNeu(false)} onOk={(name) => listenApi.neu({ name, items: [] }).then((x) => go(pfad('liste', x.id)))} />}
    </Seite>
  )
}

// Eine Zeile: Bild, Nr., Titel, Info, Dauer, „…“. OK spielt ab, Entf entfernt.
function Zeile(p: { it: Item; nr: number; extra: MenueEintrag[]; entfernen: () => void }) {
  const { it } = p
  const f = useFokus<HTMLButtonElement>({ fokusKey: 'lz-' + p.nr, onPress: () => go(pfad('watch', it.id)) })
  const m = useMenue()
  const info = it.series ? it.series + ' · ' + folge(it.season, it.episode) : String(jahr(it) || '')
  return (
    <div class={'listen-zeile' + (f.fokus ? ' ist-fokus' : '')} onContextMenu={m.rechtsklick}>
      <button
        ref={f.ref}
        {...f.dom}
        type="button"
        class="lz-knopf"
        aria-label={[p.nr + '.', anzeigeTitel(it), info].join(' ')}
        onKeyDown={(e) => {
          if (e.keyCode === 46) p.entfernen()
        }}
      >
        <span class="lz-nr">{p.nr}</span>
        <span class="lz-bild bildbox">
          <Bild src={bild(it, 'backdrop', 240)} titel={it.series || anzeigeTitel(it)} farbe={it.color} />
        </span>
        <span class="lz-titel eine-zeile">{anzeigeTitel(it)}</span>
        <span class="lz-info eine-zeile leise">{info}</span>
        <span class="lz-dauer leise">{it.duration > 0 ? dauer(it.duration) : ''}</span>
      </button>
      <Button fokusKey={'lzm-' + p.nr} variante="geist" icon="mehr" label={t('knopf.mehr.menue')} onPress={m.auf} />
      {m.offen && <AktionenMenue ziel={{ it }} anker={m.anker} onZu={m.zu} extra={p.extra} />}
    </div>
  )
}

export function Liste({ id }: { id: string }) {
  const listen = useDaten('listen', listenApi.alle)
  const alle = useDaten<Item[]>('bibliothek', () => bibliothek(), { merken: true })
  const n = useIch()
  const m = useMenue()
  const [umbenennen, setUmbenennen] = useState(false)
  const l = listen.daten ? listen.daten.filter((x) => x.id === id)[0] : undefined
  const da = !!l && !!alle.daten
  useEffect(() => {
    if (da) fokusStart('k-alle')
  }, [da])
  if (!l || !alle.daten) {
    const f = listen.fehler || alle.fehler
    return (
      <Seite bereich="listen" titel={t('nav.listen')} class="browse">
        {f ? <Fehler fehler={f} nochmal={listen.neu} /> : listen.daten === null || (listen.daten && !listen.laedt) ? <Leer titel={t('leer.unbekannt.titel')} text={t('liste.unbekannt')} aktion={{ label: t('knopf.zurueck'), onPress: back }} /> : <div class="raster rand"><SkeletonKarten n={8} /></div>}
      </Seite>
    )
  }
  const titel = listenTitel(l.items, alle.daten)
  const its = titel.map((x) => x.it || x.serie!.erste)
  const gesamt = its.reduce((a, x) => a + (x.duration || 0), 0)
  const keys = titel.map((x) => x.key)
  const neuOrdnen = (von: number, nach: number) => {
    if (nach < 0 || nach >= keys.length) return
    const k = keys.slice()
    k.splice(nach, 0, k.splice(von, 1)[0])
    listenApi.aendern(l.id, { items: k }).then(() => fokusBald('lz-' + (nach + 1)), () => {})
  }
  const entfernen = (i: number) => listenApi.aendern(l.id, { remove: [keys[i]] }).catch(() => {})
  const menue: MenueEintrag[] = [
    { label: t('liste.umbenennen'), icon: 'bearbeiten', onPress: () => setUmbenennen(true) },
    { label: t('liste.loeschen'), icon: 'loeschen', gefahr: true, onPress: () => listenApi.loeschen(l.id).then(() => go('/listen', { ersetzen: true })) },
  ]
  return (
    <Seite bereich="listen" titel={t('nav.listen')} class="browse">
      <InfoKopf
        bildTeil={<Mosaik its={its} />}
        label={n ? t('liste.von', { name: n.name }) : t('liste.titel')}
        titel={l.name}
        zeilen={<div class="info-zeile2 leise">{[anzahl(its.length, 'samm.titel.1', 'samm.titel.n'), gesamt > 0 && dauer(gesamt)].filter(Boolean).join(' · ')}</div>}
        knoepfe={
          <>
            <Button fokusKey="k-alle" variante="primaer" icon="abspielen" aus={!its.length} onPress={() => spieleAlle(its)}>
              {t('bib.alle.abspielen')}
            </Button>
            <Button fokusKey="k-zufall" icon="zufall" aus={!its.length} onPress={() => zufaellig(its)}>
              {t('bib.zufall')}
            </Button>
            <Button fokusKey="k-bearbeiten" icon="bearbeiten" label={t('liste.umbenennen')} onPress={() => setUmbenennen(true)} />
            <Button fokusKey="k-mehr" icon="mehr" label={t('knopf.mehr.menue')} onPress={m.auf} />
          </>
        }
      />
      {its.length ? (
        <section class="rand listen-tabelle">
          {geraet === 'dt' && (
            <div class="listen-kopf fl-label" aria-hidden="true">
              <span class="lz-nr">{t('liste.nr')}</span>
              <span class="lz-bild" />
              <span class="lz-titel">{t('liste.spalte.titel')}</span>
              <span class="lz-info">{t('liste.spalte.info')}</span>
              <span class="lz-dauer">{t('liste.spalte.dauer')}</span>
            </div>
          )}
          <Gruppe fokusKey="listen-zeilen">
            {its.map((it, i) => (
              <Zeile
                key={keys[i]}
                it={it}
                nr={i + 1}
                entfernen={() => entfernen(i)}
                extra={[
                  i > 0 && { label: t('liste.hoch'), icon: 'hoch', onPress: () => neuOrdnen(i, i - 1) },
                  i < its.length - 1 && { label: t('liste.runter'), icon: 'runter', onPress: () => neuOrdnen(i, i + 1) },
                  { label: t('liste.entfernen'), icon: 'loeschen', onPress: () => entfernen(i) },
                ].filter(Boolean) as MenueEintrag[]}
              />
            ))}
          </Gruppe>
          {geraet === 'dt' && <p class="leiser t-klein listen-hinweis">{t('liste.hinweis')}</p>}
        </section>
      ) : (
        <p class="rand leise">{t('liste.leer.titel')}</p>
      )}
      {m.offen && <Menue anker={m.anker} titel={l.name} eintraege={menue} onZu={m.zu} />}
      {umbenennen && <NameDialog titel={t('liste.umbenennen')} wert={l.name} onZu={() => setUmbenennen(false)} onOk={(name) => listenApi.aendern(l.id, { name })} />}
    </Seite>
  )
}
