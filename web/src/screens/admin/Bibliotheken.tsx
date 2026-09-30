// Dashboard › Bibliotheken: Medienordner (hinzufügen per Ordner-Browser, entfernen) und neu einlesen.
// Inhaltstypen, Echtzeitüberwachung und Dienste pro Bibliothek aus dem Entwurf kennt der Server (noch) nicht.
import { useEffect, useState } from 'preact/hooks'
import { useDaten } from '../../lib/api'
import { einstellungen, einstellungenSetzen, neuEinlesen, ordner as ordnerLaden, scanStatus } from '../../lib/api-admin'
import { fokusBald } from '../../lib/focus'
import { ergaenze, t } from '../../lib/i18n'
import { Button } from '../../components/Button'
import { Abschnitt, Geladen, Zeile, melde, meldeFehler } from './teile'

ergaenze({
  'bib.text': 'Ordner, aus denen Flimmer Medien einliest. Filme und Serien erkennt Flimmer am Ordner- und Dateinamen.',
  'bib.ordner': 'Medienordner',
  'bib.leer': 'Noch kein Ordner. Füge unten einen hinzu.',
  'bib.hinzu': 'Ordner hinzufügen',
  'bib.hier': 'Diesen Ordner nehmen',
  'bib.hoch': 'Eine Ebene höher',
  'bib.abbrechen': 'Abbrechen',
  'bib.entfernen': 'Entfernen',
  'bib.scan': 'Jetzt neu einlesen',
  'bib.scan.laeuft': 'Liest ein … {n} Titel gefunden',
  'bib.scan.fertig': '{n} Titel in der Bibliothek',
})

type Ordner = { path: string; parent: string; dirs: { name: string; path: string }[] }

export function Bibliotheken() {
  const s = useDaten('admin.einstellungen', einstellungen)
  const [status, setStatus] = useState<{ scanning: boolean; found: number } | null>(null)
  const [ordner, setOrdner] = useState<Ordner | null>(null)
  useEffect(() => {
    let x = 0
    const hole = () =>
      scanStatus().then((st) => {
        setStatus(st)
        if (st.scanning) x = window.setTimeout(hole, 1500)
      }, () => {})
    hole()
    return () => clearTimeout(x)
  }, [s.daten, status && status.scanning])
  const blaettern = (path: string) =>
    ordnerLaden(path).then((o) => {
      setOrdner(o)
      fokusBald('bib-browser')
    }, meldeFehler)
  return (
    <Geladen z={s}>
      {(d) => {
        const setDirs = (dirs: string[]) =>
          einstellungenSetzen({ dirs }).then(() => {
            melde(t('einst.gespeichert'))
            s.neu()
            setStatus({ scanning: true, found: status ? status.found : 0 })
          }, meldeFehler)
        return (
          <>
            <p class="t-text leise admin-absatz">{t('bib.text')}</p>
            <Abschnitt titel={t('bib.ordner')}>
              {!d.dirs.length && <p class="t-text leise">{t('bib.leer')}</p>}
              {d.dirs.map((p) => (
                <Zeile
                  key={p}
                  icon="bibliothek"
                  titel={p.split(/[\\/]/).filter(Boolean).pop() || p}
                  unter={p}
                  rechts={
                    <Button fokusKey={'bib-weg-' + p} variante="geist" onPress={() => setDirs(d.dirs.filter((x) => x !== p))}>
                      {t('bib.entfernen')}
                    </Button>
                  }
                />
              ))}
              {!ordner ? (
                <div class="admin-knoepfe">
                  <Button fokusKey="bib-hinzu" icon="plus" onPress={() => blaettern('')}>
                    {t('bib.hinzu')}
                  </Button>
                </div>
              ) : (
                <div class="admin-karte admin-browser">
                  <p class="admin-link">{ordner.path || '…'}</p>
                  <div class="admin-knoepfe">
                    {ordner.path && (
                      <Button fokusKey="bib-browser" variante="primaer" onPress={() => (setOrdner(null), setDirs(d.dirs.concat(ordner.path)))}>
                        {t('bib.hier')}
                      </Button>
                    )}
                    {ordner.path && (
                      <Button fokusKey="bib-hoch" icon="hoch" onPress={() => blaettern(ordner.parent)}>
                        {t('bib.hoch')}
                      </Button>
                    )}
                    <Button fokusKey={ordner.path ? 'bib-zu' : 'bib-browser'} variante="geist" onPress={() => (setOrdner(null), fokusBald('bib-hinzu'))}>
                      {t('bib.abbrechen')}
                    </Button>
                  </div>
                  {ordner.dirs.map((x) => (
                    <Zeile key={x.path} fk={'bib-dir-' + x.path} icon="weiter" titel={x.name} unter={ordner.path ? undefined : x.path} onPress={() => blaettern(x.path)} />
                  ))}
                </div>
              )}
            </Abschnitt>
            <Abschnitt>
              <Zeile
                fk="bib-scan"
                icon="neustart"
                titel={t('bib.scan')}
                unter={status ? t(status.scanning ? 'bib.scan.laeuft' : 'bib.scan.fertig', { n: status.found }) : undefined}
                onPress={() => neuEinlesen().then(() => setStatus({ scanning: true, found: status ? status.found : 0 }), meldeFehler)}
              />
            </Abschnitt>
          </>
        )
      }}
    </Geladen>
  )
}
