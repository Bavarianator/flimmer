// Dashboard › Geräte & Aktivitäten: angemeldete Geräte (abmelden) und das Aktivitätsprotokoll mit Filter.
import { useState } from 'preact/hooks'
import { useDaten } from '../../lib/api'
import { aktivitaeten, geraetAbmelden, geraete, type Activity } from '../../lib/api-admin'
import { ergaenze, t } from '../../lib/i18n'
import { Button } from '../../components/Button'
import { AKTIVITAET_ICON } from './Uebersicht'
import { Abschnitt, Geladen, Wahl, Zeile, meldeFehler, useNachladen, vor } from './teile'

ergaenze({
  'ger.geraete': 'Geräte',
  'ger.anzahl': '{n} Geräte',
  'ger.leer': 'Noch kein Gerät angemeldet.',
  'ger.dieses': 'dieses Gerät',
  'ger.abmelden': 'Abmelden',
  'ger.abmelden.sicher': 'Wirklich abmelden?',
  'ger.text': 'Ein abgemeldetes Gerät muss sich neu anmelden oder neu gekoppelt werden.',
  'ger.akt': 'Aktivitäten',
  'ger.f.alle': 'Alle',
  'ger.f.play': 'Wiedergabe',
  'ger.f.login': 'Anmeldung',
  'ger.f.system': 'System',
  'ger.f.error': 'Fehler',
  'ger.akt.leer': 'Keine Einträge.',
  'ger.system': 'System',
})

type Filter = '' | 'play' | 'login' | 'system' | 'error'
const SYSTEM: Activity['kind'][] = ['scan', 'task', 'backup', 'invite', 'user']

export function Geraete() {
  const z = useDaten('admin.geraete', geraete)
  const [sicher, setSicher] = useState('')
  return (
    <>
      <Abschnitt titel={t('ger.geraete')} text={t('ger.text')}>
        <Geladen z={z}>
          {(l) => (
            <>
              {!l.length && <p class="t-text leise">{t('ger.leer')}</p>}
              {l.map((d) => (
                <Zeile
                  key={d.id}
                  icon={/android|ios|handy|phone/i.test(d.client || '') ? 'handy' : /cast/i.test(d.client || '') ? 'cast' : 'fernseher'}
                  titel={d.name + (d.current ? ' · ' + t('ger.dieses') : '')}
                  unter={[d.client, d.user, d.ip].filter(Boolean).join(' · ')}
                  wert={vor(d.lastSeen)}
                  rechts={
                    d.current ? undefined : (
                      <Button
                        fokusKey={'ger-weg-' + d.id}
                        variante="geist"
                        onPress={() => (sicher === d.id ? geraetAbmelden(d.id).then(z.neu, meldeFehler) : setSicher(d.id))}
                      >
                        {t(sicher === d.id ? 'ger.abmelden.sicher' : 'ger.abmelden')}
                      </Button>
                    )
                  }
                />
              ))}
            </>
          )}
        </Geladen>
      </Abschnitt>
      <Aktivitaeten />
    </>
  )
}

function Aktivitaeten() {
  const z = useDaten('admin.aktivitaeten', () => aktivitaeten(200))
  const [f, setF] = useState<Filter>('')
  useNachladen(z.neu, 15000)
  const passt = (a: Activity) => !f || (f === 'system' ? SYSTEM.indexOf(a.kind) >= 0 : a.kind === f)
  return (
    <Abschnitt titel={t('ger.akt')}>
      <Wahl
        fk="ger-filter"
        an={f}
        onWahl={setF}
        werte={(['', 'play', 'login', 'system', 'error'] as Filter[]).map((x) => [x, t('ger.f.' + (x || 'alle'))] as [Filter, string])}
      />
      <Geladen z={z}>
        {(l) => {
          const liste = l.filter(passt)
          return (
            <>
              {!liste.length && <p class="t-text leise">{t('ger.akt.leer')}</p>}
              {liste.map((a, i) => (
                <Zeile key={i} icon={AKTIVITAET_ICON[a.kind] || 'info'} class={a.kind === 'error' ? 'admin-fehler' : undefined} titel={a.text} unter={a.user || t('ger.system')} wert={vor(a.time)} />
              ))}
            </>
          )
        }}
      </Geladen>
    </Abschnitt>
  )
}
