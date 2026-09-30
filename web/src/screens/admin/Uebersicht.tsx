// Dashboard › Übersicht: Server, Kennzahlen, aktive Geräte, laufende Aufgaben, letzte Aktivitäten.
import { useDaten } from '../../lib/api'
import { aufgaben, uebersicht, type Activity, type Session } from '../../lib/api-admin'
import { dauer, ergaenze, t } from '../../lib/i18n'
import { go } from '../../lib/router'
import { Button } from '../../components/Button'
import { MiniAvatar } from '../../components/Avatar'
import { AmpelPunkt } from '../../components/Karte'
import type { IconName } from '../../components/Icon'
import { Abschnitt, Balken, Geladen, Messwert, Zeile, groesse, methode, seit, useNachladen, vor } from './teile'

ergaenze({
  'dash.verbunden': 'Verbunden',
  'dash.version': 'Version',
  'dash.system': 'Betriebssystem',
  'dash.laeuft': 'Läuft seit',
  'dash.update': 'Update verfügbar: {v}',
  'dash.aktualisieren': 'Aktualisieren',
  'dash.filme': 'Filme',
  'dash.serien': 'Serien',
  'dash.folgen': '{n} Folgen',
  'dash.gescannt': 'Zuletzt gescannt {zeit}',
  'dash.speicher': 'Speicher',
  'dash.belegt': '{frei} frei von {gesamt}',
  'dash.aktiv': 'Aktive Geräte',
  'dash.aktiv.leer': 'Gerade schaut niemand.',
  'dash.noch': 'noch {zeit}',
  'dash.pausiert': 'pausiert',
  'dash.aufgaben': 'Laufende Aufgaben',
  'dash.aktivitaeten': 'Letzte Aktivitäten',
  'dash.alle': 'Alle anzeigen',
  'dash.keine.aktivitaet': 'Noch nichts passiert.',
})

export const AKTIVITAET_ICON: Record<Activity['kind'], IconName> = {
  login: 'profil',
  play: 'abspielen',
  scan: 'neustart',
  error: 'fehler',
  invite: 'einladung',
  backup: 'sicherung',
  task: 'aufgaben',
  user: 'gemeinsam',
}

export function Uebersicht() {
  const z = useDaten('admin.uebersicht', uebersicht)
  useNachladen(z.neu, 10000)
  return (
    <Geladen z={z}>
      {(o) => (
        <>
          <section class="admin-karte admin-server fl-reihe-flex">
            <div class="fl-grow">
              <b class="admin-server-name">{o.server.name}</b>
              <span class="fl-ampel-zeile">
                <span class="fl-ampel gruen" />
                {t('dash.verbunden')}
              </span>
            </div>
            <Messwert label={t('dash.version')} wert={o.server.version} />
            <Messwert label={t('dash.system')} wert={o.server.os + ' · ' + o.server.arch} />
            <Messwert label={t('dash.laeuft')} wert={seit(o.server.uptime)} />
            {o.server.update && (
              <span class="admin-update fl-reihe-flex">
                <span class="t-klein">{t('dash.update', { v: o.server.update.version })}</span>
                <Button fokusKey="dash-update" icon="download" onPress={() => window.open(o.server.update!.url, '_blank')}>
                  {t('dash.aktualisieren')}
                </Button>
              </span>
            )}
          </section>

          <section class="admin-kennzahlen">
            <Messwert label={t('dash.filme')} wert={String(o.library.movies)} />
            <Messwert label={t('dash.serien')} wert={String(o.library.series)} />
            <span class="fl-messwert admin-messwert">
              <small class="fl-label">{t('dash.folgen', { n: '' }).trim()}</small>
              <b>{o.library.episodes}</b>
              {o.library.lastScan && <small>{t('dash.gescannt', { zeit: vor(o.library.lastScan) })}</small>}
            </span>
            {o.disks.map((d) => (
              <span key={d.path} class="fl-messwert admin-messwert">
                <small class="fl-label">{t('dash.speicher')}</small>
                <b>{groesse(d.total - d.free)}</b>
                <small class="eine-zeile">{t('dash.belegt', { frei: groesse(d.free), gesamt: groesse(d.total) })}</small>
                <Balken anteil={d.total ? 1 - d.free / d.total : 0} />
              </span>
            ))}
          </section>

          <Abschnitt
            titel={t('dash.aktiv')}
            zusatz={
              <Button fokusKey="dash-geraete" variante="geist" onPress={() => go('/dashboard/geraete')}>
                {t('dash.alle')}
              </Button>
            }
          >
            {!o.sessions.length && <p class="t-text leise">{t('dash.aktiv.leer')}</p>}
            <div class="admin-sitzungen">
              {o.sessions.map((s) => (
                <SitzungKarte key={s.id} s={s} />
              ))}
            </div>
          </Abschnitt>

          <LaufendeAufgaben />

          <Abschnitt
            titel={t('dash.aktivitaeten')}
            zusatz={
              <Button fokusKey="dash-aktiv-alle" variante="geist" onPress={() => go('/dashboard/geraete')}>
                {t('dash.alle')}
              </Button>
            }
          >
            {!o.activity.length && <p class="t-text leise">{t('dash.keine.aktivitaet')}</p>}
            {o.activity.map((a, i) => (
              <Zeile key={i} icon={AKTIVITAET_ICON[a.kind] || 'info'} class={a.kind === 'error' ? 'admin-fehler' : undefined} titel={a.text} wert={vor(a.time)} />
            ))}
          </Abschnitt>
        </>
      )}
    </Geladen>
  )
}

function SitzungKarte({ s }: { s: Session }) {
  const anteil = s.duration ? s.position / s.duration : 0
  return (
    <article class="admin-karte admin-sitzung">
      <div class="fl-reihe-flex">
        <MiniAvatar n={{ name: s.user, color: s.userColor }} />
        <span class="fl-grow admin-sitzung-wer">
          <b class="eine-zeile">{s.user}</b>
          <small class="eine-zeile">{s.device + (s.client ? ' · ' + s.client : '')}</small>
        </span>
      </div>
      <b class="admin-sitzung-titel eine-zeile">{s.title}</b>
      <span class="fl-ampel-zeile t-klein">
        <AmpelPunkt stufe={s.light} />
        <span class="eine-zeile">{methode(s.method) + (s.reason ? ' · ' + s.reason : '')}</span>
      </span>
      <Balken anteil={anteil} />
      <small class="leise">{s.paused ? t('dash.pausiert') : t('dash.noch', { zeit: dauer(Math.max(0, s.duration - s.position)) })}</small>
    </article>
  )
}

// Laufende Aufgaben aus /api/tasks; fehlt der Endpunkt, bleibt der Abschnitt weg.
function LaufendeAufgaben() {
  const z = useDaten('admin.aufgaben', aufgaben)
  const laufend = (z.daten || []).filter((a) => a.running)
  useNachladen(z.neu, 3000, laufend.length > 0)
  if (!laufend.length) return null
  return (
    <Abschnitt titel={t('dash.aufgaben')}>
      {laufend.map((a) => (
        <div key={a.id} class="fl-zeile admin-aufgabe-lauf">
          <span class="text">
            <b>{a.name}</b>
            <small>{a.description}</small>
            <Balken anteil={a.progress || 0} />
          </span>
          <span class="wert">{Math.round((a.progress || 0) * 100)} %</span>
        </div>
      ))}
    </Abschnitt>
  )
}
