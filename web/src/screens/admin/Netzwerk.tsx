// Dashboard › Netzwerk & Fernzugriff: Adresse im Heimnetz, Fernzugriff über das Relay (Status, Prüfen, App-Code)
// und erkannte VPNs (Tailscale/NetBird). Ports, HTTPS und IP-Filter aus dem Entwurf stellt Flimmer nicht ein.
import { useState } from 'preact/hooks'
import { useDaten } from '../../lib/api'
import { appCode, einstellungen, einstellungenSetzen, fernzugriff, fernzugriffPruefen, vpn, type Remote } from '../../lib/api-admin'
import { ergaenze, t } from '../../lib/i18n'
import { Button } from '../../components/Button'
import { Abschnitt, Geladen, Zeile, kopieren, melde, meldeFehler, uhrzeit } from './teile'

ergaenze({
  'netz.netz': 'Heimnetz',
  'netz.adresse': 'Adresse im Heimnetz',
  'netz.fern': 'Fernzugriff',
  'netz.remote': 'Von unterwegs schauen',
  'netz.remote.text': 'Filme laufen direkt von hier zu dir, über keinen fremden Server.',
  'netz.remote.nicht': 'Fernzugriff ist in diesem Build nicht verfügbar.',
  'netz.erreichbar': 'Von unterwegs erreichbar',
  'netz.nicht': 'Nicht erreichbar',
  'netz.pruefen': 'Fernzugriff prüfen',
  'netz.prueft': 'Prüft … (bis 30 s)',
  'netz.code': 'Code für die App',
  'netz.code.text': 'Die Android-App verbindet sich damit auch von unterwegs.',
  'netz.code.knopf': 'Code erzeugen',
  'netz.code.gilt': 'Gilt bis {zeit} Uhr',
  'netz.vpn': 'VPN',
  'netz.vpn.leer': 'Kein VPN erkannt.',
  'netz.vpn.kopieren': 'Adresse kopieren',
})

const ANBIETER: Record<string, string> = { tailscale: 'Tailscale', netbird: 'NetBird', vpn: 'VPN' }

export function Netzwerk() {
  const s = useDaten('admin.einstellungen', einstellungen)
  return (
    <>
      <Geladen z={s}>
        {(d) => (
          <>
            <Abschnitt titel={t('netz.netz')}>
              <Zeile
                icon="netzwerk"
                titel={t('netz.adresse')}
                unter={d.lanUrl}
                rechts={
                  <Button fokusKey="netz-lan" icon="link" onPress={() => kopieren(d.lanUrl)}>
                    {t('einst.kopieren')}
                  </Button>
                }
              />
            </Abschnitt>
            <Abschnitt titel={t('netz.fern')}>
              {d.remoteAvailable ? (
                <Fern an={d.remote} umschalten={() => einstellungenSetzen({ remote: !d.remote }).then(() => (melde(t('einst.gespeichert')), s.neu()), meldeFehler)} />
              ) : (
                <p class="t-text leise">{t('netz.remote.nicht')}</p>
              )}
            </Abschnitt>
          </>
        )}
      </Geladen>
      <Vpn />
    </>
  )
}

function Fern({ an, umschalten }: { an: boolean; umschalten: () => void }) {
  const st = useDaten('admin.remote', fernzugriff)
  const [prueft, setPrueft] = useState(false)
  const [code, setCode] = useState<{ code: string; expires: string } | null>(null)
  const x: Remote | undefined = st.daten
  return (
    <>
      <Zeile fk="netz-remote" icon="fernzugriff" titel={t('netz.remote')} unter={t('netz.remote.text')} schalter={an} onPress={umschalten} />
      {x && (
        <Zeile
          icon={x.reachable ? 'haken' : 'fehler'}
          titel={x.reachable ? t('netz.erreichbar') : t('netz.nicht')}
          unter={[x.publicUrl, x.hint || (x.method && x.method !== 'none' ? x.method.toUpperCase() : '')].filter(Boolean).join(' · ')}
          rechts={
            <Button
              fokusKey="netz-pruefen"
              icon="neustart"
              aus={prueft}
              onPress={() => {
                setPrueft(true)
                fernzugriffPruefen()
                  .then(() => st.neu(), meldeFehler)
                  .then(() => setPrueft(false))
              }}
            >
              {t(prueft ? 'netz.prueft' : 'netz.pruefen')}
            </Button>
          }
        />
      )}
      <Zeile
        icon="schluessel"
        titel={t('netz.code')}
        unter={code ? t('netz.code.gilt', { zeit: uhrzeit(code.expires) }) : t('netz.code.text')}
        rechts={
          code ? (
            <span class="admin-appcode">{code.code}</span>
          ) : (
            <Button fokusKey="netz-code" onPress={() => appCode().then(setCode, meldeFehler)}>
              {t('netz.code.knopf')}
            </Button>
          )
        }
      />
    </>
  )
}

function Vpn() {
  const z = useDaten('admin.vpn', vpn)
  return (
    <Abschnitt titel={t('netz.vpn')}>
      <Geladen z={z}>
        {(v) => (
          <>
            {!v.addrs.length && <p class="t-text leise">{t('netz.vpn.leer')}</p>}
            {v.addrs.map((a) => (
              <Zeile
                key={a.interface + a.ip}
                icon="netzwerk"
                titel={ANBIETER[a.provider] || a.provider}
                unter={a.url + ' · ' + a.interface}
                rechts={
                  <Button fokusKey={'netz-vpn-' + a.ip} icon="link" onPress={() => kopieren(a.url)}>
                    {t('netz.vpn.kopieren')}
                  </Button>
                }
              />
            ))}
            {v.hint && <p class="t-klein leise admin-absatz">{v.hint}</p>}
          </>
        )}
      </Geladen>
    </Abschnitt>
  )
}
