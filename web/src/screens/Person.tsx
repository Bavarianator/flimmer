// Person (/person/:name), Entwurf Person: Porträt, Biografie und Filmografie aus der eigenen Bibliothek.
import { useEffect, useState } from 'preact/hooks'
import { InfoKopf, initialen } from '../components/Details'
import { Bild, tonFuer } from '../components/Karte'
import { Raster } from '../components/Reihe'
import { Seite } from '../components/Seite'
import { SkeletonKarten } from '../components/Skeleton'
import { Tabs } from '../components/Tabs'
import { SerienKarte, TitelKarte } from '../components/TitelKarte'
import { Fehler, Leer } from '../components/Zustand'
import { bibliothek, jahr, person, serien, useDaten, type Item, type PersonInfo } from '../lib/api'
import { fokusStart } from '../lib/focus'
import { anzahl, ergaenze, t } from '../lib/i18n'
import { back } from '../lib/router'
import './browse.css'

ergaenze({
  'person.label': 'Person',
  'person.filme': 'Filme',
  'person.serien': 'Serien',
  'person.fehlt': 'Zu dieser Person gibt es noch keine Seite',
  'person.fehlt.text': 'Der Server kennt Personen erst nach dem nächsten Update bzw. dem nächsten Metadaten-Abgleich.',
  'person.anzahl': '{n} Titel in deiner Bibliothek',
  'person.anzahl.1': '1 Titel in deiner Bibliothek',
})

export function Person({ name }: { name: string }) {
  const p = useDaten<PersonInfo | null>('person:' + name, () => person(name))
  const alle = useDaten<Item[]>('bibliothek', () => bibliothek(), { merken: true })
  const [tab, setTab] = useState('filme')
  const d = p.daten
  // Titel aus der Bibliothek bevorzugen: dort stimmen Ampel und Fortschritt für dieses Gerät.
  const byId: Record<string, Item> = {}
  for (const it of alle.daten || []) byId[it.id] = it
  const items = d ? (d.items || []).map((x) => byId[x.id] || x) : []
  const rolle: Record<string, string> = {}
  for (const r of (d && d.roles) || []) rolle[r.id] = rolle[r.id] ? rolle[r.id] + ', ' + r.role : r.role
  const filme = items.filter((x) => !x.series).sort((a, b) => (jahr(a) || 0) - (jahr(b) || 0))
  const namen = items.filter((x) => !!x.series).map((x) => x.series!)
  const reihen = serien(alle.daten || []).filter((s) => namen.indexOf(s.name) >= 0)
  const rolleSerie: Record<string, string> = {}
  for (const x of items) if (x.series) rolleSerie[x.series] = rolle[x.id] || ''
  const zeige = tab === 'serien' && reihen.length ? 'serien' : filme.length ? 'filme' : 'serien'
  const da = !!d
  useEffect(() => {
    if (da) fokusStart(filme.length ? 'person-' + filme[0].id : reihen.length ? 'person-' + reihen[0].name : 'zustand-aktion')
  }, [da])

  let inhalt: preact.ComponentChildren
  if (p.fehler && !d) inhalt = <Fehler fehler={p.fehler} nochmal={p.neu} />
  else if (d === null) inhalt = <Leer icon="profil" titel={t('person.fehlt')} text={t('person.fehlt.text')} aktion={{ label: t('knopf.zurueck'), onPress: back }} />
  else if (!d)
    inhalt = (
      <div class="raster rand">
        <SkeletonKarten n={8} />
      </div>
    )
  else
    inhalt = (
      <>
        <InfoKopf
          rund
          bildTeil={d.image ? <Bild src={d.image} titel={d.name} /> : <span class="initialen" style={{ background: tonFuer(d.name) }}>{initialen(d.name)}</span>}
          label={t('person.label')}
          titel={d.name}
          zeilen={<div class="info-zeile2 leise">{anzahl(filme.length + reihen.length, 'person.anzahl.1', 'person.anzahl')}</div>}
          text={d.bio}
        />
        {filme.length > 0 && reihen.length > 0 && (
          <div class="rand staffel-tabs">
            <Tabs fokusKey="person-tabs" aktiv={zeige} onWahl={setTab} tabs={[{ id: 'filme', label: t('person.filme') }, { id: 'serien', label: t('person.serien') }]} />
          </div>
        )}
        <Raster fokusKey="person-titel">
          {zeige === 'filme'
            ? filme.map((it) => <TitelKarte key={it.id} it={it} reihe="person" unter={[jahr(it), rolle[it.id]].filter(Boolean).join(' · ')} />)
            : reihen.map((s) => <SerienKarte key={s.name} s={s} reihe="person" unter={rolleSerie[s.name] || undefined} />)}
        </Raster>
      </>
    )
  return (
    <Seite bereich="" titel={name} class="browse">
      {inhalt}
    </Seite>
  )
}
