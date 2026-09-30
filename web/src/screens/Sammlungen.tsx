// Sammlungen (/sammlungen) und eine Sammlung (/sammlung/:id), Entwurf Sammlungen.
// Sichtbar für alle, bearbeiten nur Admins. Filmreihen aus TMDB (auto) lassen sich nicht ändern.
import { useEffect, useState } from 'preact/hooks'
import { AuswahlLeiste, NameDialog, spieleAlle, useAuswahl, zufaellig } from '../components/Aktionen'
import { Button } from '../components/Button'
import { InfoKopf } from '../components/Details'
import { Bild, Karte, tonFuer } from '../components/Karte'
import { Menue, useMenue, type MenueEintrag } from '../components/Menue'
import { Raster, Reihe } from '../components/Reihe'
import { Seite, useIch } from '../components/Seite'
import { SkeletonKarten } from '../components/Skeleton'
import { SerienKarte, TitelKarte } from '../components/TitelKarte'
import { Fehler, Leer } from '../components/Zustand'
import { bibliothek, bild, jahr, sammlungenApi, serien, useDaten, type Item, type Sammlung, type Serie } from '../lib/api'
import { fokusStart } from '../lib/focus'
import { anzahl, dauer, ergaenze, t } from '../lib/i18n'
import { back, go, pfad } from '../lib/router'
import './browse.css'

ergaenze({
  'samm.titel': 'Sammlung',
  'samm.neu': 'Neue Sammlung',
  'samm.leer': 'Noch keine Sammlungen',
  'samm.leer.text': 'Fasse Filme zu Sammlungen zusammen, z. B. alle Filme einer Reihe. Filmreihen aus TMDB erscheinen von selbst.',
  'samm.fehlt': 'Sammlungen gibt es auf diesem Server noch nicht',
  'samm.fehlt.text': 'Nach dem nächsten Server-Update erscheinen sie hier.',
  'samm.titel.n': '{n} Titel',
  'samm.titel.1': '1 Titel',
  'samm.nach.jahr': 'nach Erscheinungsjahr',
  'samm.weitere': 'Weitere Sammlungen',
  'samm.alle': 'Alle {n} Sammlungen',
  'samm.umbenennen': 'Umbenennen',
  'samm.loeschen': 'Sammlung löschen',
  'samm.entfernen': 'Aus Sammlung entfernen',
  'samm.auto': 'Filmreihe',
  'samm.unbekannt': 'Diese Sammlung gibt es nicht mehr.',
  'samm.auswahl': 'Titel auswählen',
})

// Titel einer Liste: Item-IDs als Film/Folge, "serie:<Name>" als Serie.
export function listenTitel(keys: string[], alle: Item[]): { key: string; it?: Item; serie?: Serie }[] {
  const byId: Record<string, Item> = {}
  for (const it of alle) byId[it.id] = it
  const reihen = serien(alle)
  return keys
    .map((k) => (k.indexOf('serie:') === 0 ? { key: k, serie: reihen.filter((s) => s.name === k.slice(6))[0] } : { key: k, it: byId[k] }))
    .filter((x) => x.it || x.serie)
}

function erstesBild(keys: string[], alle: Item[]) {
  const x = listenTitel(keys.slice(0, 4), alle)[0]
  return x ? (x.it || x.serie!.erste) : undefined
}

function SammlungKarte({ s, alle, reihe }: { s: Sammlung; alle: Item[]; reihe: string }) {
  const it = erstesBild(s.items, alle)
  return (
    <Karte
      fokusKey={reihe + '-' + s.id}
      titel={s.name}
      unter={anzahl(s.items.length, 'samm.titel.1', 'samm.titel.n') + (s.auto ? ' · ' + t('samm.auto') : '')}
      bild={it ? bild(it, 'poster', 360) : ''}
      farbe={it ? it.color : tonFuer(s.name)}
      fuss={[anzahl(s.items.length, 'samm.titel.1', 'samm.titel.n').toUpperCase(), '']}
      onPress={() => go(pfad('sammlung', s.id))}
    />
  )
}

function useDatenSammlungen() {
  const samm = useDaten('sammlungen', sammlungenApi.alle)
  const alle = useDaten<Item[]>('bibliothek', () => bibliothek(), { merken: true })
  return { samm, alle }
}

export function Sammlungen() {
  const { samm, alle } = useDatenSammlungen()
  const n = useIch()
  const [neu, setNeu] = useState(false)
  const l = samm.daten
  useEffect(() => {
    if (l && l.length) fokusStart('samm-' + l[0].id)
  }, [!!(l && l.length)])
  const admin = !!n && n.admin && !!l
  let inhalt: preact.ComponentChildren
  if (samm.fehler && !l) inhalt = <Fehler fehler={samm.fehler} nochmal={samm.neu} />
  else if (l === null) inhalt = <Leer icon="sammlung" titel={t('samm.fehlt')} text={t('samm.fehlt.text')} />
  else if (!l || !alle.daten)
    inhalt = (
      <div class="raster rand">
        <SkeletonKarten n={8} />
      </div>
    )
  else if (!l.length) inhalt = <Leer icon="sammlung" titel={t('samm.leer')} text={t('samm.leer.text')} aktion={admin ? { label: t('samm.neu'), onPress: () => setNeu(true) } : undefined} />
  else
    inhalt = (
      <Raster fokusKey="samm">
        {l
          .slice()
          .sort((a, b) => a.name.localeCompare(b.name, 'de'))
          .map((s) => (
            <SammlungKarte key={s.id} s={s} alle={alle.daten!} reihe="samm" />
          ))}
      </Raster>
    )
  return (
    <Seite
      bereich="sammlungen"
      titel={t('nav.sammlungen')}
      class="browse"
      aktionen={admin && l && l.length > 0 ? <Button fokusKey="samm-neu" icon="plus" onPress={() => setNeu(true)}>{t('samm.neu')}</Button> : undefined}
    >
      {inhalt}
      {neu && <NameDialog titel={t('samm.neu')} onZu={() => setNeu(false)} onOk={(name) => sammlungenApi.neu({ name }).then((s) => go(pfad('sammlung', s.id)))} />}
    </Seite>
  )
}

export function Sammlung({ id }: { id: string }) {
  const { samm, alle } = useDatenSammlungen()
  const n = useIch()
  const m = useMenue()
  const aw = useAuswahl()
  const [umbenennen, setUmbenennen] = useState(false)
  const s = samm.daten ? samm.daten.filter((x) => x.id === id)[0] : undefined
  const da = !!s && !!alle.daten
  useEffect(() => {
    if (da) fokusStart('k-alle')
  }, [da])
  if (!s || !alle.daten) {
    const f = samm.fehler || alle.fehler
    return (
      <Seite bereich="sammlungen" titel={t('nav.sammlungen')} class="browse">
        {f ? <Fehler fehler={f} nochmal={samm.neu} /> : samm.daten === null || (samm.daten && !samm.laedt) ? <Leer titel={t('leer.unbekannt.titel')} text={t('samm.unbekannt')} aktion={{ label: t('knopf.zurueck'), onPress: back }} /> : <div class="raster rand"><SkeletonKarten n={8} /></div>}
      </Seite>
    )
  }
  const titel = listenTitel(s.items, alle.daten)
  titel.sort((a, b) => (jahr(a.it || a.serie!.erste) || 0) - (jahr(b.it || b.serie!.erste) || 0))
  const its = titel.reduce<Item[]>((acc, x) => acc.concat(x.it ? [x.it] : x.serie!.staffeln.reduce<Item[]>((a, st) => a.concat(st.folgen), [])), [])
  const jahre = titel.map((x) => jahr(x.it || x.serie!.erste) || 0).filter(Boolean)
  const gesamt = its.reduce((a, x) => a + (x.duration || 0), 0)
  const darf = !!n && n.admin && !s.auto && s.id.indexOf('auto-') !== 0 // Filmreihen sind nicht bearbeitbar
  const erstes = erstesBild(s.items, alle.daten)
  const entfernen = (k: string): MenueEintrag[] => (darf ? [{ label: t('samm.entfernen'), icon: 'loeschen', onPress: () => sammlungenApi.aendern(s.id, { remove: [k] }) }] : [])
  const menue: MenueEintrag[] = darf
    ? [
        { label: t('samm.auswahl'), icon: 'haken', onPress: () => aw.start() },
        { label: t('samm.umbenennen'), icon: 'bearbeiten', onPress: () => setUmbenennen(true) },
        { label: t('samm.loeschen'), icon: 'loeschen', gefahr: true, onPress: () => sammlungenApi.loeschen(s.id).then(() => go('/sammlungen', { ersetzen: true })) },
      ]
    : [{ label: t('samm.auswahl'), icon: 'haken', onPress: () => aw.start() }]
  const weitere = (samm.daten || []).filter((x) => x.id !== s.id).slice(0, 12)
  return (
    <Seite bereich="sammlungen" titel={t('nav.sammlungen')} class="browse">
      <InfoKopf
        bildTeil={<Bild src={erstes ? bild(erstes, 'poster', 480) : ''} titel={s.name} farbe={erstes ? erstes.color : undefined} />}
        label={t('samm.titel') + (s.auto ? ' · ' + t('samm.auto') : '')}
        titel={s.name}
        zeilen={<div class="info-zeile2 leise">{[anzahl(titel.length, 'samm.titel.1', 'samm.titel.n'), jahre.length && Math.min.apply(null, jahre) + (Math.max.apply(null, jahre) > Math.min.apply(null, jahre) ? '–' + Math.max.apply(null, jahre) : ''), gesamt > 0 && dauer(gesamt)].filter(Boolean).join(' · ')}</div>}
        text={s.overview}
        knoepfe={
          <>
            <Button fokusKey="k-alle" variante="primaer" icon="abspielen" aus={!its.length} onPress={() => spieleAlle(its)}>
              {t('bib.alle.abspielen')}
            </Button>
            <Button fokusKey="k-zufall" icon="zufall" aus={!its.length} onPress={() => zufaellig(its)}>
              {t('bib.zufall')}
            </Button>
            <Button fokusKey="k-mehr" icon="mehr" label={t('knopf.mehr.menue')} onPress={m.auf} />
          </>
        }
      />
      <section class="abschnitt">
        <h2 class="abschnitt-titel rand">
          {t('nav.filme')}
          <small>{anzahl(titel.length, 'samm.titel.1', 'samm.titel.n') + ' · ' + t('samm.nach.jahr')}</small>
        </h2>
        {titel.length ? (
          <Raster fokusKey="samm-titel">
            {titel.map((x) => (x.serie ? <SerienKarte key={x.key} s={x.serie} reihe="st" aw={aw} extra={entfernen(x.key)} /> : <TitelKarte key={x.key} it={x.it!} reihe="st" aw={aw} extra={entfernen(x.key)} />))}
          </Raster>
        ) : (
          <p class="rand leise">{t('samm.leer.text')}</p>
        )}
      </section>
      {weitere.length > 0 && (
        <Reihe fokusKey="reihe-weitere" titel={t('samm.weitere')} ziel="/sammlungen">
          {weitere.map((x) => (
            <SammlungKarte key={x.id} s={x} alle={alle.daten!} reihe="weitere" />
          ))}
        </Reihe>
      )}
      {m.offen && <Menue anker={m.anker} titel={s.name} eintraege={menue} onZu={m.zu} />}
      {umbenennen && <NameDialog titel={t('samm.umbenennen')} wert={s.name} onZu={() => setUmbenennen(false)} onOk={(name) => sammlungenApi.aendern(s.id, { name })} />}
      <AuswahlLeiste aw={aw} alle={titel.map((x) => x.key)} />
    </Seite>
  )
}
