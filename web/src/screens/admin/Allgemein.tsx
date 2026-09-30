// Dashboard › Allgemein: Servername, Metadatensprache, TMDB-Schlüssel, Updates, ffmpeg.
// Branding, Pfade und Leistung aus dem Entwurf kennt der Server (noch) nicht.
import { useEffect, useState } from 'preact/hooks'
import { useDaten } from '../../lib/api'
import { einstellungen, einstellungenSetzen, ffmpegLaden } from '../../lib/api-admin'
import { ergaenze, t } from '../../lib/i18n'
import { Button } from '../../components/Button'
import { Abschnitt, Feld, Geladen, Wahl, Zeile, melde, meldeFehler } from './teile'

ergaenze(
  {
    'allg.server': 'Server',
    'allg.name': 'Servername',
    'allg.name.text': 'Erscheint bei der Anmeldung, in den Apps und im Netzwerk.',
    'allg.adresse': 'Adresse im Heimnetz',
    'allg.metasprache': 'Metadatensprache',
    'allg.metasprache.text': 'Sprache für Titel, Inhaltsangaben und Bilder.',
    'allg.dienste': 'Metadaten-Dienste',
    'allg.tmdb': 'TMDB-Schlüssel',
    'allg.tmdb.text': 'Ohne eigenen Schlüssel nutzt Flimmer den eingebauten, sonst TVmaze und Wikidata.',
    'allg.tmdb.gesetzt': 'Eigener Schlüssel ist gesetzt.',
    'allg.entfernen': 'Entfernen',
    'allg.updates': 'Updates',
    'allg.version': 'Version',
    'allg.update.da': 'Version {v} ist erschienen',
    'allg.update.aktuell': 'Aktuell',
    'allg.update.suchen': 'Nach Updates suchen',
    'allg.update.suchen.text': 'Fragt einmal am Tag nach einer neuen Version. Installiert nichts von selbst.',
    'allg.herunterladen': 'Herunterladen',
    'allg.ffmpeg.ok': 'Installiert – Umwandeln ist möglich',
    'allg.ffmpeg.fehlt': 'Fehlt – ohne ffmpeg läuft nur, was das Gerät direkt kann',
    'allg.ffmpeg.laden': 'ffmpeg herunterladen',
    'allg.ffmpeg.laedt': 'Wird geladen … {n} %',
  },
  {
    'allg.server': 'Server',
    'allg.name': 'Server name',
    'allg.metasprache': 'Metadata language',
    'allg.tmdb': 'TMDB key',
    'allg.updates': 'Updates',
    'allg.version': 'Version',
  },
)

const SPRACHEN: [string, string][] = [
  ['de', 'Deutsch'],
  ['en', 'English'],
]

export function Allgemein() {
  const z = useDaten('admin.einstellungen', einstellungen)
  const [name, setName] = useState('')
  const [tmdb, setTmdb] = useState('')
  const d = z.daten
  useEffect(() => {
    if (d) setName(d.serverName)
  }, [d && d.serverName])
  // Während ffmpeg lädt, den Fortschritt nachfragen.
  useEffect(() => {
    if (!d || !d.ffmpeg.download.running) return
    const x = setTimeout(z.neu, 2000)
    return () => clearTimeout(x)
  }, [d])
  const setzen = (teil: Record<string, unknown>) =>
    einstellungenSetzen(teil).then(() => {
      melde(t('einst.gespeichert'))
      z.neu()
    }, meldeFehler)
  return (
    <Geladen z={z}>
      {(d) => {
        const ff = d.ffmpeg
        return (
          <>
            <Abschnitt titel={t('allg.server')}>
              <div class="admin-formzeile">
                <Feld fk="allg-name" wert={name} setWert={setName} label={t('allg.name')} zeigeLabel onEnter={() => setzen({ serverName: name })} />
                <Button fokusKey="allg-name-ok" aus={name === d.serverName} onPress={() => setzen({ serverName: name })}>
                  {t('einst.speichern')}
                </Button>
              </div>
              <p class="t-klein leise admin-absatz">{t('allg.name.text')}</p>
              <Zeile icon="netzwerk" titel={t('allg.adresse')} wert={d.lanUrl} />
            </Abschnitt>

            <Abschnitt titel={t('allg.metasprache')} text={t('allg.metasprache.text')}>
              <Wahl fk="allg-sprache" werte={SPRACHEN} an={d.language} onWahl={(x) => setzen({ language: x })} />
            </Abschnitt>

            <Abschnitt titel={t('allg.dienste')} text={d.tmdbKey ? t('allg.tmdb.gesetzt') : t('allg.tmdb.text')}>
              <div class="admin-formzeile">
                <Feld fk="allg-tmdb" wert={tmdb} setWert={setTmdb} label={t('allg.tmdb')} breit onEnter={() => tmdb && setzen({ tmdbKey: tmdb }).then(() => setTmdb(''))} />
                <Button fokusKey="allg-tmdb-ok" aus={!tmdb} onPress={() => setzen({ tmdbKey: tmdb }).then(() => setTmdb(''))}>
                  {t('einst.speichern')}
                </Button>
                {d.tmdbKey && (
                  <Button fokusKey="allg-tmdb-weg" variante="geist" onPress={() => setzen({ tmdbKey: '' })}>
                    {t('allg.entfernen')}
                  </Button>
                )}
              </div>
            </Abschnitt>

            <Abschnitt titel={t('allg.updates')}>
              <Zeile
                icon="info"
                titel={t('allg.version') + ' ' + d.version}
                unter={d.update ? t('allg.update.da', { v: d.update.version }) : t('allg.update.aktuell')}
                rechts={
                  d.update ? (
                    <Button fokusKey="allg-update" icon="download" onPress={() => window.open(d.update!.url, '_blank')}>
                      {t('allg.herunterladen')}
                    </Button>
                  ) : undefined
                }
              />
              <Zeile fk="allg-updatecheck" icon="neustart" titel={t('allg.update.suchen')} unter={t('allg.update.suchen.text')} schalter={d.updateCheck} onPress={() => setzen({ updateCheck: !d.updateCheck })} />
            </Abschnitt>

            <Abschnitt titel="ffmpeg">
              <Zeile
                icon={ff.ok ? 'haken' : 'fehler'}
                titel="ffmpeg"
                unter={ff.ok ? t('allg.ffmpeg.ok') : ff.download.running ? t('allg.ffmpeg.laedt', { n: ff.download.percent }) : ff.download.error || ff.hint || t('allg.ffmpeg.fehlt')}
                rechts={
                  !ff.ok && ff.canDownload && !ff.download.running ? (
                    <Button fokusKey="allg-ffmpeg" variante="primaer" onPress={() => ffmpegLaden().then(z.neu, meldeFehler)}>
                      {t('allg.ffmpeg.laden')}
                    </Button>
                  ) : undefined
                }
              />
            </Abschnitt>
          </>
        )
      }}
    </Geladen>
  )
}
