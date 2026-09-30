// Kontextmenü „…“ an Karten und Detailseiten, Mehrfachauswahl-Leiste und die Dialoge dazu
// (Zu Sammlung / Wiedergabeliste hinzufügen, Medieninformation, Name eingeben).
// Gezeigt wird nur, was wirklich geht: keine Warteschlange, kein Löschen, kein Teilen (siehe Bericht).
import { useEffect, useRef, useState } from 'preact/hooks'
import {
  alleGesehen,
  api,
  bibliothek,
  details,
  gecacht,
  listenApi,
  naechsteFolge,
  sammlungenApi,
  serien,
  setFavorit,
  useGecacht,
  warteschlange,
  type Details,
  type Item,
  type Nutzer,
  type Sammlung,
  type Wiedergabeliste,
} from '../lib/api'
import { geraet } from '../lib/device'
import { Gruppe, useFokus } from '../lib/focus'
import { ergaenze, t } from '../lib/i18n'
import { go, pfad } from '../lib/router'
import { Button } from './Button'
import { Icon } from './Icon'
import { Dialog, Menue, type Anker, type MenueEintrag } from './Menue'
import { spaeter } from './Spaeter'

// Metadaten-Editor von web-admin, nur für Admins und erst beim Öffnen geladen.
const MetadatenEditor = spaeter(() => import('../screens/admin/MetadatenEditor').then((m) => m.MetadatenEditor))

ergaenze({
  'akt.gemeinsam': 'Gemeinsam schauen starten',
  'akt.auswaehlen': 'Auswählen',
  'akt.zu.sammlung': 'Zu Sammlung hinzufügen',
  'akt.zu.liste': 'Zu Wiedergabeliste hinzufügen',
  'akt.download': 'Herunterladen',
  'akt.info': 'Medieninformation',
  'akt.meta': 'Metadaten bearbeiten',
  'akt.abbrechen': 'Abbrechen',
  'akt.hinzufuegen': 'Hinzufügen',
  'akt.neue.sammlung': 'Neue Sammlung …',
  'akt.neue.liste': 'Neue Wiedergabeliste …',
  'akt.name': 'Name',
  'akt.wird': '{titel} wird hinzugefügt.',
  'akt.werden': '{n} Titel werden hinzugefügt.',
  'akt.keine.sammlung': 'Noch keine Sammlung. Gib unten einen Namen ein.',
  'akt.keine.liste': 'Noch keine Wiedergabeliste. Gib unten einen Namen ein.',
  'akt.fehler': 'Das hat nicht geklappt: {grund}',
  'akt.ausgewaehlt': '{n} ausgewählt',
  'akt.alle': 'Alles auswählen',
  'akt.zu.sammlung.kurz': 'Zu Sammlung',
  'akt.zu.liste.kurz': 'Zu Wiedergabeliste',
  'akt.gesehen.kurz': 'Als gesehen',
  'akt.ungesehen.kurz': 'Als ungesehen',
  'akt.auswahl.ende': 'Auswahl beenden',
  'akt.speichern': 'Speichern',
  'info.datei': 'Datei',
  'info.video': 'Video',
  'info.ton': 'Ton',
  'info.ut': 'Untertitel',
  'info.pfad': 'Pfad',
  'info.extern': 'extern',
  'info.erzwungen': 'erzwungen',
  'info.standard': 'Standard',
})

// ---------- Hilfen ----------
// „Alle abspielen“: Warteschlange merken, ersten Titel starten (der Player nimmt die nächste id daraus).
export function spieleAlle(its: Item[]) {
  if (!its.length) return
  warteschlange(its.map((x) => x.id))
  go(pfad('watch', its[0].id))
}

export function zufaellig(its: Item[]) {
  if (its.length) go(pfad('watch', its[Math.floor(Math.random() * its.length)].id))
}

function gemeinsam(id: string) {
  api<{ id: string }>('/api/party', { mediaId: id }).then((r) => go(pfad('party', r.id)), () => {})
}

function herunterladen(id: string) {
  const a = document.createElement('a')
  a.href = '/api/items/' + encodeURIComponent(id) + '/file'
  a.setAttribute('download', '')
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

function admin() {
  const n = gecacht<Nutzer>('ich')
  return !!(n && n.admin)
}

// ---------- Mehrfachauswahl ----------
export interface Auswahl {
  an: boolean
  keys: string[]
  hat: (k: string) => boolean
  um: (k: string) => void
  start: (k?: string) => void
  setze: (keys: string[]) => void
  ende: () => void
}

export function useAuswahl(): Auswahl {
  const [keys, setKeys] = useState<string[] | null>(null)
  const k = keys || []
  return {
    an: !!keys,
    keys: k,
    hat: (x) => k.indexOf(x) >= 0,
    um: (x) => setKeys(k.indexOf(x) >= 0 ? k.filter((y) => y !== x) : k.concat(x)),
    start: (x) => setKeys(x ? [x] : []),
    setze: (x) => setKeys(x),
    ende: () => setKeys(null),
  }
}

// ---------- Kontextmenü ----------
export interface Ziel {
  it: Item // Film/Folge; bei Serien eine Folge (für Bilder)
  serie?: boolean // "serie:<Name>" statt der Folge
}

type Art = '' | 'sammlung' | 'liste' | 'info' | 'meta'

// Menü plus Folge-Dialog. anker: Knopf oder Mausposition. extra: seiteneigene Einträge (z. B. „Aus Liste entfernen“).
export function AktionenMenue(p: { ziel: Ziel; anker: Anker; onZu: () => void; aw?: Auswahl; extra?: MenueEintrag[] }) {
  const [art, setArt] = useState<Art>('')
  const gewaehlt = useRef(false)
  const fav = useGecacht<string[]>('favoriten') || []
  const samm = useGecacht<Sammlung[] | null>('sammlungen')
  const listen = useGecacht<Wiedergabeliste[] | null>('listen')
  const { it, serie } = p.ziel
  const key = serie && it.series ? 'serie:' + it.series : it.id
  const titel = serie && it.series ? it.series : (it.meta && it.meta.title) || it.title
  const dialog = (a: Art) => () => {
    gewaehlt.current = true
    setArt(a)
  }

  if (art === 'sammlung' || art === 'liste') return <ZuListe art={art} keys={[key]} titel={titel} onZu={p.onZu} />
  if (art === 'info') return <MedienInfo id={it.id} onZu={p.onZu} />
  if (art === 'meta') return <MetadatenEditor id={it.id} onZu={p.onZu} />

  const e: MenueEintrag[] = []
  const abspielen = () =>
    serie
      ? bibliothek().then((alle) => {
          const s = serien(alle).filter((x) => x.name === it.series)[0]
          if (s) go(pfad('watch', naechsteFolge(s).id))
        })
      : go(pfad('watch', it.id))
  e.push({ label: t('akt.abspielen'), icon: 'abspielen', onPress: abspielen })
  if (!serie) e.push({ label: t('akt.gemeinsam'), icon: 'gemeinsam', onPress: () => gemeinsam(it.id) })
  // Gesehen und Favorit sind am Desktop Knöpfe auf der Karte; TV und Handy haben kein Hover.
  if (geraet !== 'dt') {
    const g = serie ? false : !!it.watched
    e.push({ label: t(g ? 'akt.ungesehen' : 'akt.gesehen'), icon: 'haken', onPress: () => alleGesehen([key], !g) })
    const f = fav.indexOf(key) >= 0
    e.push({ label: t(f ? 'akt.fav.weg' : 'akt.fav'), icon: 'herz', onPress: () => setFavorit(key, !f).catch(() => {}) })
  }
  e.push({ trenner: true })
  if (p.aw) e.push({ label: t('akt.auswaehlen'), icon: 'haken', onPress: () => p.aw!.start(key) })
  if (admin() && samm !== null) e.push({ label: t('akt.zu.sammlung'), icon: 'sammlung', onPress: dialog('sammlung') })
  if (listen !== null) e.push({ label: t('akt.zu.liste'), icon: 'liste', onPress: dialog('liste') })
  if (!serie && geraet !== 'tv') e.push({ label: t('akt.download'), icon: 'download', onPress: () => herunterladen(it.id) })
  if (p.extra && p.extra.length) e.push({ trenner: true }, ...p.extra)
  if (!serie) {
    e.push({ trenner: true }, { label: t('akt.info'), icon: 'info', onPress: dialog('info') })
    if (admin()) e.push({ label: t('akt.meta'), icon: 'bearbeiten', onPress: dialog('meta') })
  }

  // Punkt ruft erst onZu, dann onPress: kurz warten, ob ein Dialog übernimmt.
  return <Menue anker={p.anker} titel={titel} eintraege={e} onZu={() => setTimeout(() => gewaehlt.current || p.onZu(), 0)} />
}

// ---------- Dialoge ----------
function Zeile(p: { fk: string; label: string; an: boolean; onPress: () => void }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: p.fk, onPress: p.onPress })
  return (
    <button ref={f.ref} {...f.dom} type="button" role="radio" aria-checked={p.an} class={'menue-punkt wahl-zeile' + (f.fokus ? ' ist-fokus' : '')}>
      <span class="fl-grow eine-zeile">{p.label}</span>
      {p.an && <Icon name="haken" class="an" />}
    </button>
  )
}

export function Eingabe(p: { fk: string; wert: string; setWert: (w: string) => void; label: string; onEnter?: () => void }) {
  const f = useFokus<HTMLInputElement>({ fokusKey: p.fk })
  return (
    <label class={'fl-feld dialog-feld' + (f.fokus ? ' ist-fokus' : '')}>
      <input
        ref={f.ref}
        {...f.dom}
        onClick={undefined}
        class="wachse"
        type="text"
        aria-label={p.label}
        placeholder={p.label}
        value={p.wert}
        onInput={(e) => p.setWert((e.target as HTMLInputElement).value)}
        onKeyDown={(e) => {
          if (e.keyCode === 13 && p.onEnter) p.onEnter()
        }}
      />
    </label>
  )
}

// Zu Sammlung bzw. Wiedergabeliste hinzufügen: vorhandene wählen oder neue anlegen.
export function ZuListe(p: { art: 'sammlung' | 'liste'; keys: string[]; titel?: string; onZu: () => void }) {
  const a = p.art === 'sammlung' ? sammlungenApi : listenApi
  const [liste, setListe] = useState<{ id: string; name: string; auto?: boolean }[] | null>(null)
  const [wahl, setWahl] = useState('')
  const [name, setName] = useState('')
  const [fehler, setFehler] = useState('')
  useEffect(() => {
    a.alle().then((l) => {
      const x = ((l || []) as { id: string; name: string; auto?: boolean }[]).filter((y) => !y.auto && y.id.indexOf('auto-') !== 0)
      setListe(x)
      if (x.length) setWahl(x[0].id)
    }, (e) => setFehler(String(e.message || e)))
  }, [])
  const los = () => {
    const fertig = wahl && !name.trim() ? a.aendern(wahl, { add: p.keys }) : name.trim() ? a.neu({ name: name.trim(), items: p.keys }) : null
    if (fertig) fertig.then(p.onZu, (e: Error) => setFehler(String(e.message || e)))
  }
  const sam = p.art === 'sammlung'
  return (
    <Dialog
      titel={t(sam ? 'akt.zu.sammlung' : 'akt.zu.liste')}
      onZu={p.onZu}
      erster="zul-0"
      knoepfe={
        <>
          <Button fokusKey="zul-ab" variante="geist" onPress={p.onZu}>
            {t('akt.abbrechen')}
          </Button>
          <Button fokusKey="zul-ok" variante="primaer" aus={!wahl && !name.trim()} onPress={los}>
            {t('akt.hinzufuegen')}
          </Button>
        </>
      }
    >
      <p class="t-klein leise dialog-hinweis">{p.keys.length === 1 && p.titel ? t('akt.wird', { titel: p.titel }) : t('akt.werden', { n: p.keys.length })}</p>
      {liste && !liste.length && <p class="t-klein leise dialog-hinweis">{t(sam ? 'akt.keine.sammlung' : 'akt.keine.liste')}</p>}
      <Gruppe fokusKey="zul-liste" class="wahl-liste">
        {(liste || []).map((x, i) => (
          <Zeile key={x.id} fk={'zul-' + i} label={x.name} an={wahl === x.id && !name.trim()} onPress={() => (setWahl(x.id), setName(''))} />
        ))}
      </Gruppe>
      <Eingabe fk={liste && liste.length ? 'zul-name' : 'zul-0'} wert={name} setWert={setName} label={t(sam ? 'akt.neue.sammlung' : 'akt.neue.liste')} onEnter={los} />
      {fehler && <p class="t-klein fehlertext dialog-hinweis">{t('akt.fehler', { grund: fehler })}</p>}
    </Dialog>
  )
}

// Name eingeben (neue Sammlung, neue Liste, umbenennen).
export function NameDialog(p: { titel: string; wert?: string; onZu: () => void; onOk: (name: string) => Promise<unknown> }) {
  const [name, setName] = useState(p.wert || '')
  const [fehler, setFehler] = useState('')
  const los = () => name.trim() && p.onOk(name.trim()).then(p.onZu, (e: Error) => setFehler(String(e.message || e)))
  return (
    <Dialog
      titel={p.titel}
      onZu={p.onZu}
      erster="nd-name"
      knoepfe={
        <>
          <Button fokusKey="nd-ab" variante="geist" onPress={p.onZu}>
            {t('akt.abbrechen')}
          </Button>
          <Button fokusKey="nd-ok" variante="primaer" aus={!name.trim()} onPress={los}>
            {t('akt.speichern')}
          </Button>
        </>
      }
    >
      <Eingabe fk="nd-name" wert={name} setWert={setName} label={t('akt.name')} onEnter={los} />
      {fehler && <p class="t-klein fehlertext dialog-hinweis">{t('akt.fehler', { grund: fehler })}</p>}
    </Dialog>
  )
}

function spurText(s: Details['audio'][0]) {
  return [s.lang && s.lang.toUpperCase(), s.title, s.codec.toUpperCase(), s.channels && s.channels + ' ch', s.default && t('info.standard'), s.forced && t('info.erzwungen'), s.external && t('info.extern')]
    .filter(Boolean)
    .join(' · ')
}

export function technik(d: Details): string {
  const v = d.video
  return [
    d.container && d.container.toUpperCase(),
    v && v.codec.toUpperCase(),
    v && v.width + '×' + v.height,
    v && v.hdr,
    d.audio[0] && d.audio[0].codec.toUpperCase() + (d.audio[0].channels ? ' ' + (d.audio[0].channels > 2 ? d.audio[0].channels - 1 + '.1' : d.audio[0].channels + '.0') : ''),
    d.subs.length && d.subs.length + ' ' + t('info.ut'),
  ]
    .filter(Boolean)
    .join(' · ')
}

function InfoZeile({ k, v }: { k: string; v?: string | number | false }) {
  if (!v) return null
  return (
    <div class="info-zeile">
      <span class="fl-label info-k">{k}</span>
      <span class="wachse">{v}</span>
    </div>
  )
}

export function MedienInfo(p: { id: string; onZu: () => void }) {
  const [d, setD] = useState<Details | null | undefined>(undefined)
  useEffect(() => {
    details(p.id).then(setD, () => setD(null))
  }, [p.id])
  return (
    <Dialog titel={t('akt.info')} onZu={p.onZu} breit>
      {d === undefined && <div class="fl-skel zeile" style={{ width: '60%' }} />}
      {d === null && <p class="leise">{t('leer.unbekannt.text')}</p>}
      {d && (
        <div class="medien-info t-klein">
          <InfoZeile k={t('info.datei')} v={[d.container && d.container.toUpperCase(), d.size && (d.size / 1e9).toFixed(2) + ' GB', d.bitrate && Math.round(d.bitrate / 1e6) + ' Mbit/s'].filter(Boolean).join(' · ')} />
          {d.video && <InfoZeile k={t('info.video')} v={[d.video.codec.toUpperCase(), d.video.width + '×' + d.video.height, d.video.hdr, d.video.fps && Math.round(d.video.fps * 100) / 100 + ' fps'].filter(Boolean).join(' · ')} />}
          {d.audio.map((s, i) => (
            <InfoZeile key={'a' + i} k={i ? ' ' : t('info.ton')} v={spurText(s)} />
          ))}
          {d.subs.map((s, i) => (
            <InfoZeile key={'u' + i} k={i ? ' ' : t('info.ut')} v={spurText(s)} />
          ))}
          <InfoZeile k={t('info.pfad')} v={d.path} />
        </div>
      )}
    </Dialog>
  )
}

// ---------- Mehrfachauswahl-Leiste ----------
export function AuswahlLeiste(p: { aw: Auswahl; alle: string[] }) {
  const [art, setArt] = useState<'' | 'sammlung' | 'liste'>('')
  const samm = useGecacht<Sammlung[] | null>('sammlungen')
  const listen = useGecacht<Wiedergabeliste[] | null>('listen')
  if (!p.aw.an) return null
  const k = p.aw.keys
  const n = k.length
  return (
    <>
      <Gruppe fokusKey="auswahl" class="auswahl-leiste" label={t('akt.ausgewaehlt', { n })}>
        <Button fokusKey="auswahl-zu" icon="schliessen" variante="geist" label={t('akt.auswahl.ende')} onPress={p.aw.ende} />
        <span class="auswahl-zahl">{t('akt.ausgewaehlt', { n })}</span>
        <Button fokusKey="auswahl-alle" variante="geist" onPress={() => p.aw.setze(p.alle)}>
          {t('akt.alle')}
        </Button>
        {admin() && samm !== null && (
          <Button fokusKey="auswahl-samm" variante="geist" aus={!n} onPress={() => setArt('sammlung')}>
            {t('akt.zu.sammlung.kurz')}
          </Button>
        )}
        {listen !== null && (
          <Button fokusKey="auswahl-liste" variante="geist" aus={!n} onPress={() => setArt('liste')}>
            {t('akt.zu.liste.kurz')}
          </Button>
        )}
        <Button fokusKey="auswahl-gesehen" variante="geist" aus={!n} onPress={() => alleGesehen(k, true).then(p.aw.ende)}>
          {t('akt.gesehen.kurz')}
        </Button>
        <Button fokusKey="auswahl-ungesehen" variante="geist" aus={!n} onPress={() => alleGesehen(k, false).then(p.aw.ende)}>
          {t('akt.ungesehen.kurz')}
        </Button>
      </Gruppe>
      {art && <ZuListe art={art} keys={k} onZu={() => (setArt(''), p.aw.ende())} />}
    </>
  )
}
