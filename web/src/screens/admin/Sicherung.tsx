// Dashboard › Sicherung & Protokoll: Sicherung laden/einspielen, Serverprotokoll mit Stufenfilter, Diagnose.
// Liste mehrerer Sicherungen und API-Schlüssel aus dem Entwurf kennt der Server (noch) nicht.
import { useRef, useState } from 'preact/hooks'
import { useDaten } from '../../lib/api'
import { diagnose, fehlt, protokoll, type Diag } from '../../lib/api-admin'
import { ergaenze, t } from '../../lib/i18n'
import { Button } from '../../components/Button'
import { AmpelPunkt } from '../../components/Karte'
import { Abschnitt, Geladen, Messwert, Wahl, Zeile, datum, groesse, kopieren, melde, meldeFehler, methode, uhrzeit, useNachladen } from './teile'

ergaenze({
  'sich.sicherungen': 'Sicherungen',
  'sich.laden': 'Sicherung herunterladen',
  'sich.laden.text': 'Profile, Fortschritt und Einstellungen als eine Datei',
  'sich.stand': 'Letzte automatische Sicherung: {zeit} · {groesse}',
  'sich.einspielen': 'Sicherung einspielen …',
  'sich.einspielen.text': 'Prüft Version und Zustand, bevor etwas ersetzt wird',
  'sich.ok': 'Sicherung eingespielt',
  'sich.protokoll': 'Protokoll',
  'sich.alle': 'Alle',
  'sich.pause': 'Pausieren',
  'sich.weiter': 'Fortsetzen',
  'sich.leer': 'Keine Einträge.',
  'sich.diagnose': 'Diagnose',
  'sich.streams': 'Laufende Streams',
  'sich.hw': 'Hardware-Beschleunigung',
  'sich.db': 'Datenbank',
  'sich.frei': 'Freier Speicher',
  'sich.laufend': 'Laufende Wiedergaben',
  'sich.keine': 'Gerade schaut niemand.',
  'sich.db.text': 'Letzte Prüfung {zeit} · {ergebnis}',
  'sich.kopieren': 'Diagnose kopieren',
  'sich.kopieren.text': 'Kopiert Version, Hardware, Zustand und Protokoll als Text – zum Einfügen in eine Fehlermeldung.',
})

export function Sicherung() {
  const d = useDaten('admin.diagnose', diagnose)
  return (
    <>
      <Sicherungen d={d.daten} />
      <Protokoll diagLog={(d.daten && d.daten.log) || []} />
      <Abschnitt titel={t('sich.diagnose')}>
        <Geladen z={d}>{(x) => <Diagnose d={x} />}</Geladen>
      </Abschnitt>
    </>
  )
}

function Sicherungen({ d }: { d?: Diag }) {
  const datei = useRef<HTMLInputElement>(null)
  const [laeuft, setLaeuft] = useState(false)
  const einspielen = (f: File) => {
    setLaeuft(true)
    fetch('/api/settings/restore', { method: 'POST', body: f, credentials: 'same-origin' })
      .then((r) => (r.ok ? melde(t('sich.ok')) : r.text().then(melde)))
      .catch(meldeFehler)
      .then(() => setLaeuft(false))
  }
  const db = d && d.db
  return (
    <Abschnitt titel={t('sich.sicherungen')}>
      <Zeile
        fk="sich-laden"
        icon="download"
        titel={t('sich.laden')}
        unter={db && db.backupAt ? t('sich.stand', { zeit: datum(db.backupAt), groesse: groesse(db.sizeBytes) }) : t('sich.laden.text')}
        onPress={() => (location.href = '/api/settings/backup')}
      />
      <Zeile fk="sich-einspielen" icon="sicherung" titel={t('sich.einspielen')} unter={t('sich.einspielen.text')} onPress={() => !laeuft && datei.current && datei.current.click()} />
      <input
        ref={datei}
        type="file"
        accept=".db,application/octet-stream"
        class="nur-sr"
        tabIndex={-1}
        onChange={(e) => {
          const f = (e.target as HTMLInputElement).files
          if (f && f[0]) einspielen(f[0])
        }}
      />
    </Abschnitt>
  )
}

const STUFEN: [string, string][] = [
  ['', 'sich.alle'],
  ['info', 'INF'],
  ['warn', 'WRN'],
  ['error', 'ERR'],
]

const KURZ: Record<string, string> = { DEBUG: 'DBG', INFO: 'INF', WARN: 'WRN', ERROR: 'ERR' }

// /api/logs (level = Mindeststufe); fehlt der Endpunkt, zeigt es die letzten Zeilen aus /api/diagnostics.
function Protokoll({ diagLog }: { diagLog: string[] }) {
  const [stufe, setStufe] = useState('')
  const [pause, setPause] = useState(false)
  const z = useDaten('admin.logs.' + stufe, () => protokoll(stufe, 300))
  useNachladen(z.neu, 5000, !pause && !(z.fehler && fehlt(z.fehler)))
  const ohne = !!z.fehler && fehlt(z.fehler)
  const zeilen = ohne ? diagLog.slice(-300) : (z.daten || []).map((l) => uhrzeit(l.time) + ' [' + (KURZ[l.level.toUpperCase()] || l.level) + '] ' + l.msg + (l.attrs ? ' ' + JSON.stringify(l.attrs) : ''))
  return (
    <Abschnitt
      titel={t('sich.protokoll')}
      zusatz={
        <>
          {!ohne && (
            <Button fokusKey="sich-pause" variante="geist" icon={pause ? 'abspielen' : 'pause'} onPress={() => setPause(!pause)}>
              {t(pause ? 'sich.weiter' : 'sich.pause')}
            </Button>
          )}
          <Button fokusKey="sich-log-kopieren" variante="geist" icon="link" onPress={() => kopieren(zeilen.join('\n'))}>
            {t('einst.kopieren')}
          </Button>
        </>
      }
    >
      {!ohne && <Wahl fk="sich-stufe" werte={STUFEN.map(([w, l]) => [w, l.indexOf('.') > 0 ? t(l) : l] as [string, string])} an={stufe} onWahl={setStufe} />}
      {z.fehler && !ohne ? (
        <p class="t-text admin-fehler">{z.fehler}</p>
      ) : (
        <pre class="admin-protokoll" role="log" tabIndex={0}>
          {zeilen.length ? zeilen.join('\n') : t('sich.leer')}
        </pre>
      )}
    </Abschnitt>
  )
}

function Diagnose({ d }: { d: Diag }) {
  const aktiv = d.active || []
  const text = JSON.stringify({ ...d, log: undefined }, null, 2) + '\n\n' + (d.log || []).join('\n')
  return (
    <>
      <div class="admin-kennzahlen">
        <Messwert wert={String(aktiv.length)} label={t('sich.streams')} />
        <Messwert wert={(d.hw || 'CPU') + (d.hwSpeed ? ' · ' + d.hwSpeed.toFixed(1).replace('.', ',') + '×' : '')} label={t('sich.hw')} />
        <Messwert wert={groesse(d.db.sizeBytes) + (d.db.integrity ? ' · ' + d.db.integrity : '')} label={t('sich.db')} />
        <Messwert wert={groesse(d.diskFree)} label={t('sich.frei')} anteil={d.diskTotal ? 1 - d.diskFree / d.diskTotal : undefined} />
      </div>
      <h3>{t('sich.laufend')}</h3>
      {!aktiv.length && <p class="t-text leise">{t('sich.keine')}</p>}
      {aktiv.map((a, i) => (
        <Zeile
          key={i}
          icon="abspielen"
          titel={'„' + a.title + '“ · ' + a.user + ' · ' + a.device}
          unter={methode(a.method) + (a.reasons && a.reasons.length ? ': ' + a.reasons.join(' · ') : '')}
          wert={<AmpelPunkt stufe={a.light} />}
        />
      ))}
      <Zeile icon={d.ffmpeg ? 'haken' : 'fehler'} titel={(d.ffmpeg || 'ffmpeg –') + ' · ' + (d.hw || 'CPU')} unter={d.os + ' · ' + d.cpus + ' CPUs · ' + d.version} />
      <Zeile
        icon={d.db.integrity === 'ok' || !d.db.integrity ? 'haken' : 'fehler'}
        titel={t('sich.db')}
        unter={t('sich.db.text', { zeit: datum(d.db.checkedAt), ergebnis: d.db.integrity || '–' })}
        wert={groesse(d.db.sizeBytes)}
      />
      <Zeile fk="sich-diag" icon="protokoll" titel={t('sich.kopieren')} unter={t('sich.kopieren.text')} onPress={() => kopieren(text)} />
    </>
  )
}
