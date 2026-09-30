// Dashboard › Live-TV: Senderliste (M3U oder HDHomeRun-lineup.json), Programm (XMLTV), Bildumwandlung, Kanäle.
// DVR und Kanalzuordnung von Hand gibt es nicht (docs/livetv-vpn.md, „Bewusst weggelassen“).
import { useState } from 'preact/hooks'
import { useDaten } from '../../lib/api'
import { kanaele, liveTV, liveTVNeu, liveTVSetzen, liveTVVorlagen, type LiveTVVorlage } from '../../lib/api-admin'
import { ergaenze, t } from '../../lib/i18n'
import { Button } from '../../components/Button'
import { Abschnitt, Feld, Geladen, Hinweis, Wahl, Zeile, melde, meldeFehler, uhrzeit, vor } from './teile'

ergaenze({
  'ltv.schnell': 'Schnell einrichten',
  'ltv.schnell.text': 'Ein Knopf trägt Sender und Programm ein. Adressen braucht es dafür nicht.',
  'ltv.vorlage.frei': 'Freie Sender (ARD, ZDF, 3sat, arte …)',
  'ltv.vorlage.frei.text': '{n} öffentlich-rechtliche Sender übers Internet, mit Programm. Meist nur in Deutschland abrufbar.',
  'ltv.vorlage.fritz': 'FRITZ!Box mit Kabel-TV',
  'ltv.vorlage.fritz.text': '{n} Sender aus deinem Kabelanschluss gefunden, mit Programm.',
  'ltv.vorlage.fritz.fehlt': 'Keine FRITZ!Box mit Kabel-TV im Heimnetz gefunden.',
  'ltv.vorlage.an': 'Aktiv',
  'ltv.vorlage.nehmen': 'Übernehmen',
  'ltv.eigene': 'Eigene Quelle',
  'ltv.quellen': 'Senderliste & Programm',
  'ltv.quelle': 'Senderliste (M3U oder HDHomeRun)',
  'ltv.quelle.text': 'Adresse einer M3U-Liste, z. B. von Tvheadend, oder http://<HDHomeRun>/lineup.json. Auch ein Dateipfad geht.',
  'ltv.epg': 'Programmführer (XMLTV)',
  'ltv.epg.text': 'XMLTV-Adresse oder -Datei, auch .xml.gz. Die Zuordnung läuft über tvg-id, sonst über den Sendernamen.',
  'ltv.gesetzt': 'Gespeichert: {wert}',
  'ltv.video': 'Bild',
  'ltv.video.copy': 'Kopieren (schnell)',
  'ltv.video.h264': 'In H.264 umwandeln',
  'ltv.video.text': 'Kopieren braucht kaum Leistung, aber SD-Sender (MPEG-2) laufen dann nicht im Browser.',
  'ltv.stand': '{k} Kanäle · {p} Sendungen · aktualisiert {zeit}',
  'ltv.neu': 'Programmdaten jetzt aktualisieren',
  'ltv.neu.ok': 'Wird neu geladen …',
  'ltv.kanaele': 'Kanäle',
  'ltv.kanaele.leer': 'Noch keine Kanäle.',
  'ltv.ohneepg': 'kein Programm zugeordnet',
  'ltv.jetzt': '{zeit} {titel}',
  'ltv.remux': 'Live-TV wird ohne Umweg nach HLS verpackt. Höchstens 3 Kanäle laufen gleichzeitig.',
})

export function LiveTVAdmin() {
  const z = useDaten('admin.livetv', liveTV)
  const v = useDaten('admin.livetv.vorlagen', liveTVVorlagen)
  const k = useDaten('admin.kanaele', kanaele)
  const [quelle, setQuelle] = useState('')
  const [epg, setEpg] = useState('')
  const setzen = (teil: { source?: string; epg?: string; video?: string }) =>
    liveTVSetzen(teil).then(() => {
      // Der Server zeigt danach nur Schema://Rechner (ohne Zugangsdaten), die Felder bleiben deshalb leer.
      if (teil.source) setQuelle('')
      if (teil.epg) setEpg('')
      melde(t('einst.gespeichert'))
      setTimeout(() => (z.neu(), k.neu(), v.neu()), 1500)
    }, meldeFehler)
  return (
    <Geladen z={z}>
      {(s) => (
        <>
          <Abschnitt titel={t('ltv.schnell')} text={t('ltv.schnell.text')}>
            <Geladen z={v}>
              {(l) => (
                <>
                  {l.map((p) => (
                    <Vorlage key={p.id} p={p} onNehmen={() => setzen({ source: p.source, epg: p.epg })} />
                  ))}
                </>
              )}
            </Geladen>
          </Abschnitt>
          <Abschnitt
            titel={t('ltv.eigene')}
            zusatz={
              <Button fokusKey="ltv-neu" icon="neustart" onPress={() => liveTVNeu().then(() => (melde(t('ltv.neu.ok')), setTimeout(() => (z.neu(), k.neu()), 2000)), meldeFehler)}>
                {t('ltv.neu')}
              </Button>
            }
          >
            {s.error && <Hinweis icon="fehler" text={s.error} />}
            <p class="t-klein leise admin-absatz">{t('ltv.stand', { k: s.channels, p: s.programs, zeit: vor(s.updated) })}</p>
            <div class="admin-formzeile">
              <Feld fk="ltv-quelle" wert={quelle} setWert={setQuelle} label={t('ltv.quelle')} zeigeLabel breit onEnter={() => quelle && setzen({ source: quelle })} />
              <Button fokusKey="ltv-quelle-ok" aus={!quelle} onPress={() => setzen({ source: quelle })}>
                {t('einst.speichern')}
              </Button>
            </div>
            <p class="t-klein leise admin-absatz">{s.source ? t('ltv.gesetzt', { wert: s.source === FREI ? t('ltv.vorlage.frei') : s.source }) : t('ltv.quelle.text')}</p>
            <div class="admin-formzeile">
              <Feld fk="ltv-epg" wert={epg} setWert={setEpg} label={t('ltv.epg')} zeigeLabel breit onEnter={() => epg && setzen({ epg })} />
              <Button fokusKey="ltv-epg-ok" aus={!epg} onPress={() => setzen({ epg })}>
                {t('einst.speichern')}
              </Button>
            </div>
            <p class="t-klein leise admin-absatz">{s.epg ? t('ltv.gesetzt', { wert: s.epg }) : t('ltv.epg.text')}</p>
            <h3>{t('ltv.video')}</h3>
            <Wahl
              fk="ltv-video"
              an={s.video || 'copy'}
              onWahl={(video) => setzen({ video })}
              werte={[
                ['copy', t('ltv.video.copy')],
                ['h264', t('ltv.video.h264')],
              ]}
            />
            <p class="t-klein leise admin-absatz">{t('ltv.video.text')}</p>
          </Abschnitt>
          <Abschnitt titel={t('ltv.kanaele')} text={t('ltv.remux')}>
            <Geladen z={k}>
              {(l) => (
                <>
                  {!l.length && <p class="t-text leise">{t('ltv.kanaele.leer')}</p>}
                  {l.map((c) => (
                    <Zeile
                      key={c.id}
                      icon="live"
                      class={c.now ? undefined : 'admin-warn'}
                      titel={(c.number ? c.number + ' · ' : '') + c.name}
                      unter={c.now ? t('ltv.jetzt', { zeit: uhrzeit(c.now.start), titel: c.now.title }) : t('ltv.ohneepg')}
                      wert={c.group}
                    />
                  ))}
                </>
              )}
            </Geladen>
          </Abschnitt>
        </>
      )}
    </Geladen>
  )
}

const FREI = 'flimmer:freie-sender'

function Vorlage({ p, onNehmen }: { p: LiveTVVorlage; onNehmen: () => void }) {
  const da = p.channels > 0
  const aktiv = p.active
  return (
    <Zeile
      fk={'ltv-vorlage-' + p.id}
      icon={p.id === 'fritz' ? 'fernseher' : 'live'}
      titel={t('ltv.vorlage.' + p.id)}
      unter={da ? t('ltv.vorlage.' + p.id + '.text', { n: p.channels }) : t('ltv.vorlage.fritz.fehlt')}
      class={da ? undefined : 'admin-warn'}
      wert={aktiv ? t('ltv.vorlage.an') : da ? t('ltv.vorlage.nehmen') : undefined}
      an={aktiv}
      onPress={da && !aktiv ? onNehmen : undefined}
    />
  )
}
