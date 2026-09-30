// Live-TV (Entwurf „LiveTV“, „TV-Guide“): Tabs Programme (/livetv), Programmführer (/livetv/programm), Kanäle
// (/livetv/kanaele); /livetv/kanal/{id} spielt einen Sender im Player. Daten aus internal/livetv (docs/livetv-vpn.md):
// GET /api/livetv/channels (mit „läuft jetzt“/„danach“), GET /api/livetv/guide?hours=, POST …/channels/{id}/play.
// Aufnahmen gibt es nicht (kein DVR), deshalb keine Aufnahme-Knöpfe.
import { useEffect, useRef, useState } from 'preact/hooks'
import { api, useDaten } from '../lib/api'
import { istTV } from '../lib/device'
import { Gruppe, fokusStart, useFokus } from '../lib/focus'
import { dauer, ergaenze, t } from '../lib/i18n'
import { go, pfad, useRoute } from '../lib/router'
import { Seite, useIch } from '../components/Seite'
import { Karte } from '../components/Karte'
import { Reihe } from '../components/Reihe'
import { Chip, Button } from '../components/Button'
import { Dialog } from '../components/Menue'
import { Fehler, Leer } from '../components/Zustand'
import { SkeletonReihe } from '../components/Skeleton'
import { Player } from '../player/Player'
import './livetv.css'

ergaenze(
  {
    'livetv.titel': 'Live-TV',
    'livetv.programme': 'Programme',
    'livetv.fuehrer': 'Programmführer',
    'livetv.kanaele': 'Kanäle',
    'livetv.jetzt': 'Läuft jetzt',
    'livetv.danach': 'Gleich danach',
    'livetv.kanal': 'Kanal',
    'livetv.alle': 'Alle',
    'livetv.bis': 'bis {uhr}',
    'livetv.ab': 'ab {uhr}',
    'livetv.noch': 'noch {dauer}',
    'livetv.einschalten': 'Einschalten',
    'livetv.kein.programm': 'Kein Programm',
    'livetv.leer': 'Noch keine Sender eingerichtet',
    'livetv.leer.admin': 'Trage im Dashboard eine M3U-Liste oder einen HDHomeRun ein, dazu ein XMLTV-Programm.',
    'livetv.leer.nutzer': 'Bitte den Admin, im Dashboard Sender einzurichten.',
    'livetv.einrichten': 'Live-TV einrichten',
    'livetv.gesperrt': 'Live-TV ist für dieses Konto nicht freigegeben',
    'livetv.gesperrt.text': 'Gastkonten sehen kein Live-TV.',
    'livetv.kein.fuehrer': 'Kein Programm geladen',
    'livetv.kein.fuehrer.text': 'Die Sender laufen, aber es gibt keine XMLTV-Daten für die nächsten Stunden.',
    'livetv.jetzt.linie': 'Jetzt, {uhr} Uhr',
    'livetv.zeitraum': '{n} KANÄLE · {von}–{bis}',
  },
  {
    'livetv.titel': 'Live TV',
    'livetv.programme': 'Programs',
    'livetv.fuehrer': 'Guide',
    'livetv.kanaele': 'Channels',
    'livetv.jetzt': 'On now',
    'livetv.danach': 'Up next',
    'livetv.kanal': 'Channel',
    'livetv.alle': 'All',
    'livetv.bis': 'until {uhr}',
    'livetv.ab': 'from {uhr}',
    'livetv.noch': '{dauer} left',
    'livetv.einschalten': 'Watch',
    'livetv.kein.programm': 'No guide data',
    'livetv.leer': 'No channels set up yet',
    'livetv.leer.admin': 'Add an M3U list or an HDHomeRun in the dashboard, plus an XMLTV guide.',
    'livetv.leer.nutzer': 'Ask the admin to set up channels in the dashboard.',
    'livetv.einrichten': 'Set up Live TV',
    'livetv.gesperrt': 'Live TV is not enabled for this account',
    'livetv.gesperrt.text': 'Guest accounts cannot watch Live TV.',
    'livetv.kein.fuehrer': 'No guide loaded',
    'livetv.kein.fuehrer.text': 'The channels work, but there is no XMLTV data for the next hours.',
    'livetv.jetzt.linie': 'Now, {uhr}',
    'livetv.zeitraum': '{n} CHANNELS · {von}–{bis}',
  },
)

interface Sendung {
  start: string
  stop: string
  title: string
  desc?: string
}
interface Kanal {
  id: string
  number?: string
  name: string
  logo?: string
  group?: string
  now: Sendung | null
  next: Sendung | null
}
interface Fuehrer {
  from: string
  to: string
  channels: { id: string; programs: Sendung[] }[]
}

// RFC 3339 von Go, ohne Nanosekunden (ältere Chromium-Parser mögen mehr als drei Nachkommastellen nicht).
const ms = (s: string) => new Date(s.replace(/\.\d+/, '')).getTime()
const hm = (x: number) => {
  const d = new Date(x)
  return (d.getHours() < 10 ? '0' : '') + d.getHours() + ':' + (d.getMinutes() < 10 ? '0' : '') + d.getMinutes()
}
const anteil = (s: Sendung, jetzt: number) => Math.max(0, Math.min(1, (jetzt - ms(s.start)) / (ms(s.stop) - ms(s.start))))
const einschalten = (k: Kanal) => go(pfad('livetv', 'kanal', k.id))

const TABS = [
  { id: 'programme', label: 'livetv.programme', pfad: '/livetv' },
  { id: 'programm', label: 'livetv.fuehrer', pfad: '/livetv/programm' },
  { id: 'kanaele', label: 'livetv.kanaele', pfad: '/livetv/kanaele' },
]

export function LiveTV() {
  const r = useRoute()
  if (r[1] === 'kanal' && r[2]) return <Sender key={r[2]} id={r[2]} />
  const tab = r[1] === 'programm' || r[1] === 'kanaele' ? r[1] : 'programme'
  return <Uebersicht tab={tab} />
}

// Vollbild-Player für einen Sender; Unterzeile = laufende Sendung (aus dem Kanal-Cache, falls da).
function Sender({ id }: { id: string }) {
  const k = useDaten('livetv-kanaele', () => api<Kanal[]>('/api/livetv/channels')).daten
  const kanal = k ? k.filter((x) => x.id === id)[0] : undefined
  return <Player id="" kanal={id} unter={kanal && kanal.now ? kanal.now.title + ' · ' + t('livetv.bis', { uhr: hm(ms(kanal.now.stop)) }) : kanal ? kanal.name : ''} />
}

function Uebersicht({ tab }: { tab: string }) {
  const ich = useIch()
  const k = useDaten('livetv-kanaele', () => api<Kanal[]>('/api/livetv/channels'))
  const tabs = TABS.map((x) => ({ ...x, label: t(x.label) }))
  let inhalt
  if (k.fehler) {
    const code = k.fehler.slice(0, 3)
    inhalt =
      code === '403' ? (
        <Leer titel={t('livetv.gesperrt')} text={t('livetv.gesperrt.text')} icon="live" />
      ) : code === '404' ? (
        <Einrichten admin={!!(ich && ich.admin)} />
      ) : (
        <Fehler fehler={k.fehler} nochmal={k.neu} />
      )
  } else if (!k.daten) inhalt = <SkeletonReihe breit />
  else if (!k.daten.length) inhalt = <Einrichten admin={!!(ich && ich.admin)} />
  else if (tab === 'programm') inhalt = <Programmfuehrer kanaele={k.daten} />
  else if (tab === 'kanaele') inhalt = <Kanaele kanaele={k.daten} />
  else inhalt = <Programme kanaele={k.daten} />
  return (
    <Seite bereich="livetv" titel={t('livetv.titel')} tabs={tabs} tab={tab} class="livetv">
      {inhalt}
    </Seite>
  )
}

function Einrichten({ admin }: { admin: boolean }) {
  return (
    <Leer
      titel={t('livetv.leer')}
      text={t(admin ? 'livetv.leer.admin' : 'livetv.leer.nutzer')}
      icon="live"
      aktion={admin ? { label: t('livetv.einrichten'), onPress: () => go('/dashboard/livetv') } : undefined}
    />
  )
}

// Programme: was jetzt läuft und was danach kommt, als Karten je Sender.
function Programme({ kanaele }: { kanaele: Kanal[] }) {
  const jetzt = Date.now()
  const laufen = kanaele.filter((k) => k.now)
  const danach = kanaele.filter((k) => k.next)
  useEffect(() => fokusStart(laufen.length ? 'lt-jetzt-' + laufen[0].id : 'lt-alle-' + kanaele[0].id), [])
  const karte = (k: Kanal, s: Sendung | null, vorsilbe: string, jetztLaeuft: boolean) => (
    <Karte
      key={k.id}
      fokusKey={vorsilbe + k.id}
      breit
      titel={s ? s.title : k.name}
      unter={[k.number, k.name, s && t(jetztLaeuft ? 'livetv.bis' : 'livetv.ab', { uhr: hm(ms(jetztLaeuft ? s.stop : s.start)) })].filter(Boolean).join(' · ')}
      bild={k.logo}
      fortschritt={s && jetztLaeuft ? anteil(s, jetzt) : undefined}
      onPress={() => einschalten(k)}
    />
  )
  return (
    <div class="livetv-programme">
      {laufen.length > 0 && (
        <Reihe titel={t('livetv.jetzt')} fokusKey="lt-jetzt">
          {laufen.map((k) => karte(k, k.now, 'lt-jetzt-', true))}
        </Reihe>
      )}
      {danach.length > 0 && (
        <Reihe titel={t('livetv.danach')} fokusKey="lt-danach">
          {danach.map((k) => karte(k, k.next, 'lt-danach-', false))}
        </Reihe>
      )}
      {!laufen.length && (
        <Reihe titel={t('livetv.kanaele')} fokusKey="lt-alle">
          {kanaele.map((k) => karte(k, null, 'lt-alle-', false))}
        </Reihe>
      )}
    </div>
  )
}

// Kanäle: Liste mit Nummer, Logo, Name und laufender Sendung.
function Kanaele({ kanaele }: { kanaele: Kanal[] }) {
  const jetzt = Date.now()
  useEffect(() => fokusStart('lt-k-' + kanaele[0].id), [])
  return (
    <Gruppe fokusKey="lt-kanaele" class="livetv-kanaele rand">
      {kanaele.map((k) => (
        <KanalZeile key={k.id} k={k} jetzt={jetzt} />
      ))}
    </Gruppe>
  )
}

function KanalZeile({ k, jetzt }: { k: Kanal; jetzt: number }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: 'lt-k-' + k.id, onPress: () => einschalten(k) })
  return (
    <button ref={f.ref} {...f.dom} type="button" class={'livetv-kanal zeile' + (f.fokus ? ' ist-fokus' : '')}>
      <span class="livetv-nr fl-zahl">{k.number}</span>
      <span class="livetv-logo">{k.logo ? <img src={k.logo} alt="" /> : <span class="fl-zahl">{k.name.slice(0, 3)}</span>}</span>
      <span class="wachse livetv-kanal-text">
        <b class="eine-zeile">{k.name}</b>
        <span class="eine-zeile leise">
          {k.now ? k.now.title + ' · ' + t('livetv.bis', { uhr: hm(ms(k.now.stop)) }) : t('livetv.kein.programm')}
          {k.next ? ' · ' + t('livetv.danach') + ': ' + k.next.title : ''}
        </span>
        {k.now && (
          <span class="livetv-balken">
            <i style={{ transform: 'scaleX(' + anteil(k.now, jetzt) + ')', webkitTransform: 'scaleX(' + anteil(k.now, jetzt) + ')' }} />
          </span>
        )}
      </span>
      {k.group && <span class="livetv-gruppe fl-label">{k.group}</span>}
    </button>
  )
}

// Programmführer: links die Kanäle, rechts das Zeitraster (5 px pro Minute, Desktop), Linie für „jetzt“.
// Auf dem TV fährt das Raster beim Fokus waagerecht mit (scrollLeft), senkrecht folgt die Seite (lib/focus).
function Programmfuehrer({ kanaele }: { kanaele: Kanal[] }) {
  const g = useDaten('livetv-fuehrer', () => api<Fuehrer>('/api/livetv/guide?hours=12'))
  const [gruppe, setGruppe] = useState('')
  const [wahl, setWahl] = useState<{ k: Kanal; s: Sendung } | null>(null)
  const raster = useRef<HTMLDivElement>(null)
  const tv = istTV()
  const ppm = tv ? 8 : 5 // Pixel pro Minute
  const gruppen = kanaele.map((k) => k.group || '').filter((x, i, a) => x && a.indexOf(x) === i)
  useEffect(() => {
    if (g.daten) fokusStart('lt-fuehrer-knopf')
  }, [!!g.daten])

  if (g.fehler) return <Fehler fehler={g.fehler} nochmal={g.neu} />
  if (!g.daten) return <SkeletonReihe breit />
  const von = Math.floor(ms(g.daten.from) / 1800000) * 1800000 // auf die halbe Stunde abgerundet
  const bis = ms(g.daten.to)
  const jetzt = Date.now()
  const px = (x: number) => Math.round(((x - von) / 60000) * ppm)
  const sendungen: Record<string, Sendung[]> = {}
  for (const c of g.daten.channels) sendungen[c.id] = c.programs
  const liste = kanaele.filter((k) => !gruppe || k.group === gruppe)
  if (!g.daten.channels.length) return <Leer titel={t('livetv.kein.fuehrer')} text={t('livetv.kein.fuehrer.text')} icon="kalender" />
  const marken: number[] = []
  for (let x = von; x < bis; x += 1800000) marken.push(x)
  const breite = px(bis)
  // TV: kein DOM-Fokus, also das Raster selbst nachführen (eine halbe Stunde links sichtbar lassen).
  const folgen = (links: number) => {
    const r = raster.current
    if (r && tv) r.scrollLeft = Math.max(0, links - 30 * ppm)
  }

  return (
    <div class="livetv-fuehrer rand">
      <Gruppe fokusKey="lt-filter" class="zeile livetv-filter">
        <Chip fokusKey="lt-fuehrer-knopf" an={!gruppe} onPress={() => setGruppe('')}>
          {t('livetv.alle')}
        </Chip>
        {gruppen.map((x) => (
          <Chip key={x} fokusKey={'lt-g-' + x} an={gruppe === x} onPress={() => setGruppe(x)}>
            {x}
          </Chip>
        ))}
        <span class="wachse" />
        <span class="fl-label">{t('livetv.zeitraum', { n: liste.length, von: hm(von), bis: hm(bis) })}</span>
      </Gruppe>
      <div class="livetv-gitter zeile">
        <div class="livetv-spalte">
          <div class="livetv-kopfzelle fl-label">{t('livetv.kanal')}</div>
          {liste.map((k) => (
            <div key={k.id} class="livetv-zeilenkopf zeile">
              <span class="livetv-nr fl-zahl">{k.number}</span>
              <span class="wachse eine-zeile">{k.name}</span>
            </div>
          ))}
        </div>
        <div class="livetv-raster wachse" ref={raster}>
          <Gruppe fokusKey="lt-raster" class="livetv-flaeche">
            <div style={{ width: breite + 'px', height: 40 + liste.length * (tv ? 96 : 64) + 'px' }} class="livetv-innen">
              {marken.map((x) => (
                <span key={x} class="livetv-marke" style={{ left: px(x) + 'px' }}>
                  <span class="fl-zahl">{hm(x)}</span>
                </span>
              ))}
              {liste.map((k, i) =>
                (sendungen[k.id] || []).map((s) => (
                  <Block
                    key={k.id + s.start}
                    s={s}
                    k={k}
                    links={Math.max(0, px(ms(s.start)))}
                    breite={Math.max(ppm * 5, px(Math.min(ms(s.stop), bis)) - Math.max(0, px(ms(s.start))) - 4)}
                    oben={40 + i * (tv ? 96 : 64) + 6}
                    laeuft={ms(s.start) <= jetzt && jetzt < ms(s.stop)}
                    onFocus={folgen}
                    onPress={() => setWahl({ k, s })}
                  />
                )),
              )}
              <span class="livetv-jetzt" style={{ left: px(jetzt) + 'px' }} aria-label={t('livetv.jetzt.linie', { uhr: hm(jetzt) })}>
                <span class="fl-zahl">{hm(jetzt)}</span>
              </span>
            </div>
          </Gruppe>
        </div>
      </div>
      {wahl && <SendungDialog k={wahl.k} s={wahl.s} onZu={() => setWahl(null)} />}
    </div>
  )
}

function Block(p: { s: Sendung; k: Kanal; links: number; breite: number; oben: number; laeuft: boolean; onFocus: (x: number) => void; onPress: () => void }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: 'lt-b-' + p.k.id + '-' + p.s.start, onPress: p.onPress, onFocus: () => p.onFocus(p.links) })
  return (
    <button
      ref={f.ref}
      {...f.dom}
      type="button"
      class={'livetv-block' + (p.laeuft ? ' laeuft' : '') + (f.fokus ? ' ist-fokus' : '')}
      style={{ left: p.links + 'px', top: p.oben + 'px', width: p.breite + 'px' }}
      aria-label={p.s.title + ', ' + p.k.name + ', ' + hm(ms(p.s.start)) + '–' + hm(ms(p.s.stop))}
    >
      <span class="eine-zeile">{p.s.title}</span>
      <small class="eine-zeile fl-zahl">
        {hm(ms(p.s.start))}–{hm(ms(p.s.stop))}
      </small>
    </button>
  )
}

function SendungDialog({ k, s, onZu }: { k: Kanal; s: Sendung; onZu: () => void }) {
  const jetzt = Date.now()
  const laeuft = ms(s.start) <= jetzt && jetzt < ms(s.stop)
  return (
    <Dialog
      titel={s.title}
      onZu={onZu}
      erster={laeuft ? 'lt-einschalten' : undefined}
      knoepfe={
        laeuft && (
          <Button fokusKey="lt-einschalten" variante="primaer" icon="abspielen" onPress={() => einschalten(k)}>
            {t('livetv.einschalten')}
          </Button>
        )
      }
    >
      <p class="leise">
        {hm(ms(s.start))}–{hm(ms(s.stop))} · {[k.number, k.name].filter(Boolean).join(' ')}
      </p>
      {s.desc && <p class="livetv-text">{s.desc}</p>}
      {laeuft && (
        <div class="zeile livetv-rest">
          <span class="livetv-balken wachse">
            <i style={{ transform: 'scaleX(' + anteil(s, jetzt) + ')', webkitTransform: 'scaleX(' + anteil(s, jetzt) + ')' }} />
          </span>
          <span class="fl-label">{t('livetv.noch', { dauer: dauer((ms(s.stop) - jetzt) / 1000) })}</span>
        </div>
      )}
    </Dialog>
  )
}
