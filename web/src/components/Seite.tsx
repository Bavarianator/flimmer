// Rahmen jeder Seite im Jellyfin-Aufbau (docs/umbau-jellyfin.md):
// - Desktop: Seitenleiste links (Hauptmenü bzw. Dashboard-Menü), Kopfzeile oben mit Titel, Tabs, Suche, Avatar-Menü
// - Handy: Kopfzeile mit Menü-Knopf, Schublade von links, wischbare Tabs unter der Kopfzeile
// - TV: Icon-Schiene links, die beim Fokus aufklappt; Titel und Tabs stehen oben im Inhalt
import type { ComponentChildren } from 'preact'
import { useEffect, useState } from 'preact/hooks'
import { FocusContext, useFocusable } from '@noriginmedia/norigin-spatial-navigation'
import { abmelden, api, geraeteTest, holen, ich, type Nutzer } from '../lib/api'
import { beiGeraetWechsel, geraet, type Geraet } from '../lib/device'
import { Gruppe, useFokus } from '../lib/focus'
import { ergaenze, t } from '../lib/i18n'
import { go, pfad, useZurueck } from '../lib/router'
import { MiniAvatar } from './Avatar'
import { Icon, type IconName } from './Icon'
import { Menue, useMenue, type MenueEintrag } from './Menue'
import { Wortmarke } from './Wortmarke'
import './rahmen.css'

ergaenze(
  {
    'nav.startseite': 'Startseite',
    'nav.favoriten': 'Favoriten',
    'nav.medien': 'Medien',
    'nav.musik': 'Musik',
    'nav.livetv': 'Live-TV',
    'nav.sammlungen': 'Sammlungen',
    'nav.listen': 'Wiedergabelisten',
    'nav.administration': 'Administration',
    'nav.dashboard': 'Dashboard',
    'nav.metadaten': 'Metadaten-Manager',
    'nav.benutzer': 'Benutzer',
    'nav.abmelden': 'Abmelden',
    'nav.zurueck-flimmer': 'Zurück zu Flimmer',
    'nav.uebersicht': 'Übersicht',
    'nav.statistik': 'Statistik',
    'nav.mediathek': 'Mediathek',
    'nav.server': 'Server',
    'nav.allgemein': 'Allgemein & Branding',
    'nav.einladungen': 'Einladungen',
    'nav.bibliotheken': 'Bibliotheken',
    'nav.wiedergabe': 'Wiedergabe',
    'nav.umwandlung': 'Umwandlung & Trickplay',
    'nav.geraete': 'Geräte',
    'nav.geraete-aktivitaeten': 'Geräte & Aktivitäten',
    'nav.livetv-aufnahmen': 'Live-TV & Aufnahmen',
    'nav.erweitert': 'Erweitert',
    'nav.netzwerk': 'Netzwerk & Fernzugriff',
    'nav.aufgaben': 'Geplante Aufgaben',
    'nav.sicherung': 'Sicherung & Protokoll',
    'nav.gemeinsam': 'Gemeinsam schauen',
    'nav.suchfeld': 'Filme, Serien, Personen …',
    'nav.menue-oeffnen': 'Menü öffnen',
    'nav.benutzermenue': 'Benutzermenü von {name}',
    'nav.verbunden': 'Server verbunden',
    'nav.getrennt': 'Keine Verbindung',
    'nav.scan': 'Bibliothek wird gescannt …',
    'nav.schnellverbindung': 'Schnellverbindung',
    'nav.admin': 'Admin',
  },
  {
    'nav.startseite': 'Home',
    'nav.favoriten': 'Favorites',
    'nav.medien': 'Media',
    'nav.musik': 'Music',
    'nav.livetv': 'Live TV',
    'nav.sammlungen': 'Collections',
    'nav.listen': 'Playlists',
    'nav.administration': 'Administration',
    'nav.dashboard': 'Dashboard',
    'nav.metadaten': 'Metadata manager',
    'nav.benutzer': 'User',
    'nav.abmelden': 'Sign out',
    'nav.zurueck-flimmer': 'Back to Flimmer',
    'nav.uebersicht': 'Overview',
    'nav.statistik': 'Statistics',
    'nav.mediathek': 'Public broadcasters',
    'nav.server': 'Server',
    'nav.allgemein': 'General & branding',
    'nav.einladungen': 'Invites',
    'nav.bibliotheken': 'Libraries',
    'nav.wiedergabe': 'Playback',
    'nav.umwandlung': 'Transcoding & trickplay',
    'nav.geraete': 'Devices',
    'nav.geraete-aktivitaeten': 'Devices & activity',
    'nav.livetv-aufnahmen': 'Live TV & recordings',
    'nav.erweitert': 'Advanced',
    'nav.netzwerk': 'Network & remote access',
    'nav.aufgaben': 'Scheduled tasks',
    'nav.sicherung': 'Backup & logs',
    'nav.gemeinsam': 'Watch together',
    'nav.suchfeld': 'Movies, shows, people …',
    'nav.menue-oeffnen': 'Open menu',
    'nav.benutzermenue': 'User menu of {name}',
    'nav.verbunden': 'Server connected',
    'nav.getrennt': 'Not connected',
    'nav.scan': 'Scanning library …',
    'nav.schnellverbindung': 'Quick connect',
    'nav.admin': 'Admin',
  },
)

// Bereiche der Seitenleiste; der Dashboard-Bereich beginnt mit „dash“.
export type Bereich = string

interface Punkt {
  id: string
  label: string // i18n-Schlüssel
  pfad: string
  icon: IconName
  admin?: boolean
}
type Eintrag = Punkt | { kopf: string; admin?: boolean }

// Musik kommt in Phase 2 dazu.
const HAUPT: Eintrag[] = [
  { id: 'start', label: 'nav.startseite', pfad: '/', icon: 'start' },
  { id: 'favoriten', label: 'nav.favoriten', pfad: '/favoriten', icon: 'herz' },
  { kopf: 'nav.medien' },
  { id: 'filme', label: 'nav.filme', pfad: '/filme', icon: 'film' },
  { id: 'serien', label: 'nav.serien', pfad: '/serien', icon: 'serie' },
  { id: 'livetv', label: 'nav.livetv', pfad: '/livetv', icon: 'live' },
  { id: 'sammlungen', label: 'nav.sammlungen', pfad: '/sammlungen', icon: 'sammlung' },
  { id: 'listen', label: 'nav.listen', pfad: '/listen', icon: 'liste' },
  { kopf: 'nav.administration', admin: true },
  { id: 'dash', label: 'nav.dashboard', pfad: '/dashboard', icon: 'dashboard', admin: true },
  { id: 'dash-metadaten', label: 'nav.metadaten', pfad: '/dashboard/metadaten', icon: 'bearbeiten', admin: true },
  { kopf: 'nav.benutzer' },
  { id: 'einstellungen', label: 'nav.einstellungen', pfad: '/einstellungen', icon: 'einstellungen' },
  { id: 'abmelden', label: 'nav.abmelden', pfad: '', icon: 'abmelden' },
]

const DASHBOARD: Eintrag[] = [
  { id: 'dash', label: 'nav.uebersicht', pfad: '/dashboard', icon: 'dashboard' },
  { id: 'dash-statistik', label: 'nav.statistik', pfad: '/dashboard/statistik', icon: 'statistik' },
  { kopf: 'nav.server' },
  { id: 'dash-allgemein', label: 'nav.allgemein', pfad: '/dashboard/allgemein', icon: 'einstellungen' },
  { id: 'dash-benutzer', label: 'nav.benutzer', pfad: '/dashboard/benutzer', icon: 'gemeinsam' },
  { id: 'dash-einladungen', label: 'nav.einladungen', pfad: '/dashboard/einladungen', icon: 'einladung' },
  { kopf: 'nav.bibliotheken' },
  { id: 'dash-bibliotheken', label: 'nav.bibliotheken', pfad: '/dashboard/bibliotheken', icon: 'bibliothek' },
  { id: 'dash-metadaten', label: 'nav.metadaten', pfad: '/dashboard/metadaten', icon: 'bearbeiten' },
  { id: 'dash-mediathek', label: 'nav.mediathek', pfad: '/dashboard/mediathek', icon: 'hochladen' },
  { kopf: 'nav.wiedergabe' },
  { id: 'dash-umwandlung', label: 'nav.umwandlung', pfad: '/dashboard/umwandlung', icon: 'abspielen' },
  { kopf: 'nav.geraete' },
  { id: 'dash-geraete', label: 'nav.geraete-aktivitaeten', pfad: '/dashboard/geraete', icon: 'fernseher' },
  { id: 'dash-livetv', label: 'nav.livetv-aufnahmen', pfad: '/dashboard/livetv', icon: 'live' },
  { kopf: 'nav.erweitert' },
  { id: 'dash-netzwerk', label: 'nav.netzwerk', pfad: '/dashboard/netzwerk', icon: 'netzwerk' },
  { id: 'dash-aufgaben', label: 'nav.aufgaben', pfad: '/dashboard/aufgaben', icon: 'aufgaben' },
  { id: 'dash-sicherung', label: 'nav.sicherung', pfad: '/dashboard/sicherung', icon: 'sicherung' },
]

function eintraege(admin: boolean, n: Nutzer | null): Eintrag[] {
  const liste = admin ? DASHBOARD : HAUPT
  return liste.filter((e) => !e.admin || (n && n.admin))
}

function abmeldenUndNeu() {
  abmelden().then(
    () => location.reload(),
    () => location.reload(),
  )
}

// Angemeldetes Profil (aus dem Cache sofort, dann frisch).
export function useIch() {
  const [n, setN] = useState<Nutzer | null>(null)
  useEffect(() => {
    ich().then(setN, () => {})
  }, [])
  return n
}

// Handy ↔ Desktop wechselt beim Drehen oder Ändern der Fenstergröße.
export function useGeraet(): Geraet {
  const [g, setG] = useState(geraet)
  useEffect(() => beiGeraetWechsel(setG), [])
  return g
}

function NavPunkt({ e, an, onWahl }: { e: Punkt; an: boolean; onWahl?: () => void }) {
  const los = () => {
    if (onWahl) onWahl()
    if (e.id === 'abmelden') abmeldenUndNeu()
    else go(e.pfad)
  }
  const f = useFokus<HTMLAnchorElement>({ fokusKey: 'nav-' + e.id, onPress: los })
  return (
    <a
      ref={f.ref}
      {...f.dom}
      href={e.pfad ? '#' + e.pfad : undefined}
      onClick={(ev) => {
        ev.preventDefault()
        los()
      }}
      aria-current={an ? 'page' : undefined}
      class={'nav-punkt' + (an ? ' an' : '') + (f.fokus ? ' ist-fokus' : '')}
    >
      <Icon name={e.icon} />
      <span class="text eine-zeile">{t(e.label)}</span>
    </a>
  )
}

function NavListe({ admin, bereich, n, onWahl }: { admin: boolean; bereich: string; n: Nutzer | null; onWahl?: () => void }) {
  return (
    <nav class="leiste-nav" aria-label={t('nav.menue')}>
      {admin && <NavPunkt e={{ id: 'zurueck', label: 'nav.zurueck-flimmer', pfad: '/', icon: 'zurueck' }} an={false} onWahl={onWahl} />}
      {eintraege(admin, n).map((e) =>
        'kopf' in e ? (
          <div key={e.kopf} class="nav-kopf">
            {t(e.kopf)}
          </div>
        ) : (
          <NavPunkt key={e.id} e={e} an={e.id === bereich} onWahl={onWahl} />
        ),
      )}
    </nav>
  )
}

// Bildmarke: Block-F, heller Block mit ausgestanztem F (brand/block-f).
export function Bildmarke() {
  return (
    <svg class="bildmarke" viewBox="0 0 640 640" aria-hidden="true" focusable="false">
      <path d="M0 0 h640 v640 h-640 Z M160 80 L480 80 L480 160 L240 160 L240 240 L400 240 L400 320 L240 320 L240 560 L160 560 Z" fill="currentColor" fill-rule="evenodd" />
      <path d="M160 320 L240 320 L240 560 L160 560 Z" style="fill:var(--linie-stark)" />
    </svg>
  )
}

function Marke({ admin }: { admin: boolean }) {
  return (
    <div class="leiste-marke">
      <Bildmarke />
      <span class="marke-text">
        <Wortmarke />
        {admin && <span class="fl-label">{t('nav.dashboard')}</span>}
      </span>
    </div>
  )
}

function Status() {
  const [s, setS] = useState<'' | 'ok' | 'scan' | 'weg'>('')
  useEffect(() => {
    holen('status', () => api<{ scanning: boolean }>('/api/status'), { maxAlter: 30000 }).then(
      (x) => setS(x.scanning ? 'scan' : 'ok'),
      () => setS('weg'),
    )
  }, [])
  if (!s) return null
  return (
    <div class="leiste-status" role="status">
      <span class={'fl-ampel ' + (s === 'weg' ? 'rot' : s === 'scan' ? 'gelb' : 'gruen')} />
      <span class="text">{t(s === 'weg' ? 'nav.getrennt' : s === 'scan' ? 'nav.scan' : 'nav.verbunden')}</span>
    </div>
  )
}

// Desktop: feste Seitenleiste.
function Leiste({ admin, bereich, n }: { admin: boolean; bereich: string; n: Nutzer | null }) {
  return (
    <Gruppe fokusKey="leiste" tag="aside" class="leiste">
      <Marke admin={admin} />
      <NavListe admin={admin} bereich={bereich} n={n} />
      <Status />
    </Gruppe>
  )
}

// TV: schmale Icon-Schiene; hat sie den Fokus, klappt sie mit Beschriftung auf.
function SchieneTV({ admin, bereich, n }: { admin: boolean; bereich: string; n: Nutzer | null }) {
  const { ref, focusKey, hasFocusedChild } = useFocusable({
    focusKey: 'leiste',
    trackChildren: true,
    saveLastFocusedChild: false,
    preferredChildFocusKey: 'nav-' + bereich,
  })
  return (
    <FocusContext.Provider value={focusKey}>
      <aside ref={ref} class={'schiene-tv' + (hasFocusedChild ? ' offen' : '')}>
        <Marke admin={admin} />
        <NavListe admin={admin} bereich={bereich} n={n} />
      </aside>
    </FocusContext.Provider>
  )
}

// Handy: Schublade von links mit Profilblock oben.
function Schublade({ admin, bereich, n, onZu }: { admin: boolean; bereich: string; n: Nutzer | null; onZu: () => void }) {
  useZurueck(() => (onZu(), true))
  return (
    <>
      <div class="schublade-scrim" onClick={onZu} />
      <Gruppe fokusKey="schublade" tag="aside" class="schublade" grenze>
        {n && (
          <div class="schublade-profil fl-reihe-flex">
            <MiniAvatar n={n} />
            <span class="fl-grow">
              <b class="eine-zeile">{n.name}</b>
              <small>{n.admin ? t('nav.admin') : ''}</small>
            </span>
          </div>
        )}
        <NavListe admin={admin} bereich={bereich} n={n} onWahl={onZu} />
        <Status />
      </Gruppe>
    </>
  )
}

function KopfKnopf({ fk, icon, label, onPress }: { fk: string; icon: IconName; label: string; onPress: () => void }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: fk, onPress })
  return (
    <button ref={f.ref} {...f.dom} type="button" aria-label={label} title={label} class={'kopf-knopf' + (f.fokus ? ' ist-fokus' : '')}>
      <Icon name={icon} />
    </button>
  )
}

function avatarMenue(n: Nutzer): MenueEintrag[] {
  const m: MenueEintrag[] = [
    { label: t('nav.profil'), icon: 'wechseln', onPress: () => go('/profile') },
    { label: t('nav.einstellungen'), icon: 'einstellungen', onPress: () => go('/einstellungen') },
  ]
  if (n.admin)
    m.push(
      { trenner: true },
      { label: t('nav.dashboard'), icon: 'dashboard', onPress: () => go('/dashboard') },
      { label: t('nav.metadaten'), icon: 'bearbeiten', onPress: () => go('/dashboard/metadaten') },
    )
  m.push(
    { trenner: true },
    { label: t('nav.schnellverbindung'), icon: 'schluessel', onPress: () => go('/einstellungen/schnellverbindung') },
    { label: t('nav.koppeln'), icon: 'fernseher', onPress: () => go('/koppeln') },
    ...(n.upload && geraet !== 'tv' ? [{ label: t('nav.hochladen'), icon: 'hochladen' as const, onPress: () => go('/hochladen') }] : []), // TV: kein Dateisystem
    { label: t('nav.geraetetest'), icon: 'neustart', onPress: () => geraeteTest.starten() },
    { trenner: true },
    { label: t('nav.abmelden'), icon: 'abmelden', onPress: abmeldenUndNeu },
  )
  return m
}

function AvatarKnopf({ n }: { n: Nutzer }) {
  const m = useMenue()
  const f = useFokus<HTMLButtonElement>({ fokusKey: 'kopf-avatar', onPress: () => f.ref.current && m.auf(f.ref.current) })
  const label = t('nav.benutzermenue', { name: n.name })
  return (
    <>
      <button ref={f.ref} {...f.dom} type="button" aria-label={label} aria-haspopup="menu" title={label} class={'kopf-knopf' + (f.fokus ? ' ist-fokus' : '')}>
        <MiniAvatar n={n} />
      </button>
      {m.offen && <Menue anker={m.anker} titel={n.name} eintraege={avatarMenue(n)} onZu={m.zu} />}
    </>
  )
}

// Suchfeld der Kopfzeile: Enter öffnet /suche/<begriff>.
function Suchfeld() {
  const [q, setQ] = useState('')
  const f = useFokus<HTMLInputElement>({ fokusKey: 'kopf-suche' })
  return (
    <label class={'kopf-suche' + (f.fokus ? ' ist-fokus' : '')}>
      <Icon name="suche" />
      <input
        ref={f.ref}
        {...f.dom}
        onClick={undefined}
        type="search"
        aria-label={t('nav.suche')}
        placeholder={t('nav.suchfeld')}
        value={q}
        onInput={(e) => setQ((e.target as HTMLInputElement).value)}
        onKeyDown={(e) => {
          if (e.keyCode === 13 && q.trim()) go(pfad('suche', q.trim()))
        }}
      />
    </label>
  )
}

export interface SeitenTab {
  id: string
  label: string
  pfad: string
}

function KopfTab({ x, an }: { x: SeitenTab; an: boolean }) {
  const f = useFokus<HTMLAnchorElement>({ fokusKey: 'tab-' + x.id, onPress: () => go(x.pfad, { ersetzen: true }) })
  return (
    <a
      ref={f.ref}
      {...f.dom}
      href={'#' + x.pfad}
      onClick={(ev) => {
        ev.preventDefault()
        go(x.pfad, { ersetzen: true })
      }}
      role="tab"
      aria-selected={an}
      class={'kopf-tab' + (an ? ' an' : '') + (f.fokus ? ' ist-fokus' : '')}
    >
      {x.label}
    </a>
  )
}

function KopfTabs({ tabs, tab, class: cls }: { tabs: SeitenTab[]; tab?: string; class: string }) {
  return (
    <Gruppe fokusKey="kopf-tabs" class={cls} bevorzugt={'tab-' + tab}>
      <div role="tablist" class="kopf-tabs-innen">
        {tabs.map((x) => (
          <KopfTab key={x.id} x={x} an={x.id === tab} />
        ))}
      </div>
    </Gruppe>
  )
}

export interface SeitenProps {
  bereich?: Bereich
  titel?: string
  tabs?: SeitenTab[]
  tab?: string
  admin?: boolean // Dashboard-Seitenleiste
  ueberHero?: boolean // Kopfzeile transparent über dem Hero (Detailseiten)
  aktionen?: ComponentChildren // Knöpfe rechts in der Kopfzeile, z. B. „Bibliothek scannen“
  ohneKopf?: boolean
  class?: string
  children: ComponentChildren
}

export function Seite(p: SeitenProps) {
  const g = useGeraet()
  const n = useIch()
  const [schublade, setSchublade] = useState(false)
  const bereich = p.bereich || ''
  const admin = !!p.admin
  const tv = g === 'tv'
  const handy = g === 'hd'
  const rahmen = !p.ohneKopf

  return (
    <div class={'rahmen' + (rahmen ? '' : ' ohne')}>
      {rahmen && !handy && (tv ? <SchieneTV admin={admin} bereich={bereich} n={n} /> : <Leiste admin={admin} bereich={bereich} n={n} />)}
      <main class={'seite' + (p.class ? ' ' + p.class : '')}>
        <div class="seite-schiene">
          {rahmen && !tv && (
            <>
              <Gruppe fokusKey="kopf" tag="header" class={'kopfzeile' + (p.ueberHero ? ' ueber-hero' : '')}>
                {handy && <KopfKnopf fk="kopf-menue" icon="menue" label={t('nav.menue-oeffnen')} onPress={() => setSchublade(true)} />}
                {p.titel && <h1 class="kopf-titel eine-zeile">{p.titel}</h1>}
                {!handy && p.tabs && <KopfTabs tabs={p.tabs} tab={p.tab} class="kopf-tabs" />}
                <span class="fl-grow" />
                {p.aktionen}
                {!handy && !admin && bereich !== 'suche' && <Suchfeld />}
                {handy && !admin && <KopfKnopf fk="kopf-suche" icon="suche" label={t('nav.suche')} onPress={() => go('/suche')} />}
                {!handy && !admin && <KopfKnopf fk="kopf-gemeinsam" icon="gemeinsam" label={t('nav.gemeinsam')} onPress={() => go('/party')} />}
                {n && <AvatarKnopf n={n} />}
              </Gruppe>
              {handy && p.tabs && <KopfTabs tabs={p.tabs} tab={p.tab} class="kopf-tabs kopf-tabs-hd" />}
            </>
          )}
          {rahmen && tv && (p.titel || p.tabs) && (
            <div class="tv-kopf">
              {p.titel && <h1 class="tv-titel">{p.titel}</h1>}
              {p.tabs && <KopfTabs tabs={p.tabs} tab={p.tab} class="kopf-tabs" />}
            </div>
          )}
          {p.children}
        </div>
      </main>
      {rahmen && handy && schublade && <Schublade admin={admin} bereich={bereich} n={n} onZu={() => setSchublade(false)} />}
    </div>
  )
}
