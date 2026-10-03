// Dashboard unter /dashboard[/:bereich] (docs/umbau-jellyfin.md). Nur für Admins; der Rahmen zeigt die Dashboard-Seitenleiste.
import type { ComponentType } from 'preact'
import { useEffect } from 'preact/hooks'
import { neuEinlesen } from '../../lib/api-admin'
import { istTV } from '../../lib/device'
import { Gruppe, fokusStart } from '../../lib/focus'
import { ergaenze, t } from '../../lib/i18n'
import { useRoute } from '../../lib/router'
import { Button } from '../../components/Button'
import { Seite, useIch } from '../../components/Seite'
import { Hinweis, Toast, melde, meldeFehler } from './teile'
import { Uebersicht } from './Uebersicht'
import { Allgemein } from './Allgemein'
import { Benutzer } from './Benutzer'
import { Einladungen } from './Einladungen'
import { Bibliotheken } from './Bibliotheken'
import { Metadaten } from './Metadaten'
import { Umwandlung } from './Umwandlung'
import { Geraete } from './Geraete'
import { LiveTVAdmin } from './LiveTV'
import { Netzwerk } from './Netzwerk'
import { Aufgaben } from './Aufgaben'
import { Sicherung } from './Sicherung'
import { Statistik } from './Statistik'
import { Mediathek } from './Mediathek'

ergaenze(
  { 'dash.scannen': 'Bibliothek scannen', 'dash.scan.gestartet': 'Die Bibliothek wird neu eingelesen.' },
  { 'dash.scannen': 'Scan library', 'dash.scan.gestartet': 'The library is being rescanned.' },
)

const BEREICHE: Record<string, [string, ComponentType]> = {
  '': ['nav.uebersicht', Uebersicht],
  statistik: ['nav.statistik', Statistik],
  allgemein: ['nav.allgemein', Allgemein],
  benutzer: ['nav.benutzer', Benutzer],
  einladungen: ['nav.einladungen', Einladungen],
  bibliotheken: ['nav.bibliotheken', Bibliotheken],
  metadaten: ['nav.metadaten', Metadaten],
  mediathek: ['nav.mediathek', Mediathek],
  umwandlung: ['nav.umwandlung', Umwandlung],
  geraete: ['nav.geraete-aktivitaeten', Geraete],
  livetv: ['nav.livetv-aufnahmen', LiveTVAdmin],
  netzwerk: ['nav.netzwerk', Netzwerk],
  aufgaben: ['nav.aufgaben', Aufgaben],
  sicherung: ['nav.sicherung', Sicherung],
}

export function Dashboard() {
  const b = useRoute()[1] || ''
  const bereich = BEREICHE[b] ? b : ''
  const [titel, Inhalt] = BEREICHE[bereich]
  const n = useIch()
  useEffect(() => {
    if (istTV()) fokusStart('inhalt')
  }, [bereich])
  return (
    <Seite
      admin
      bereich={bereich ? 'dash-' + bereich : 'dash'}
      titel={t(titel)}
      aktionen={
        <Button fokusKey="dash-scan" icon="neustart" onPress={() => neuEinlesen().then(() => melde(t('dash.scan.gestartet')), meldeFehler)}>
          {t('dash.scannen')}
        </Button>
      }
    >
      <div class="admin rand">
        {n && !n.admin ? (
          <Hinweis text={t('einst.nuradmin')} />
        ) : (
          <Gruppe fokusKey="inhalt">{n && <Inhalt key={bereich} />}</Gruppe>
        )}
      </div>
      <Toast />
    </Seite>
  )
}
