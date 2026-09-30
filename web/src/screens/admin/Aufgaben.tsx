// Dashboard › Geplante Aufgaben: nach Gruppe, mit letztem Lauf, nächstem Termin und „Jetzt starten“.
// Auslöser bearbeiten (Entwurf) gibt es nicht; die Zeiten legt der Server fest.
import { useDaten } from '../../lib/api'
import { aufgaben, aufgabeStarten, type Task } from '../../lib/api-admin'
import { dauer, ergaenze, t } from '../../lib/i18n'
import { Button } from '../../components/Button'
import { Abschnitt, Balken, Geladen, datum, meldeFehler, useNachladen, vor } from './teile'

ergaenze({
  'auf.leer': 'Keine Aufgaben.',
  'auf.laeuft': 'Läuft · {n} %',
  'auf.zuletzt': 'Zuletzt {zeit}',
  'auf.fehler': 'Fehlgeschlagen {zeit}',
  'auf.dauer': 'Dauer {d}',
  'auf.naechste': 'Nächster Lauf {zeit}',
  'auf.nie': 'Noch nie gelaufen',
  'auf.starten': '{name} jetzt starten',
  'auf.start': 'Starten',
})

export function Aufgaben() {
  const z = useDaten('admin.aufgaben', aufgaben)
  const laeuft = (z.daten || []).some((a) => a.running)
  useNachladen(z.neu, laeuft ? 2000 : 30000)
  return (
    <Geladen z={z}>
      {(l) => {
        if (!l.length) return <p class="t-text leise">{t('auf.leer')}</p>
        const gruppen: string[] = []
        for (const a of l) if (gruppen.indexOf(a.group) < 0) gruppen.push(a.group)
        return (
          <>
            {gruppen.map((g) => (
              <Abschnitt key={g} titel={g}>
                {l
                  .filter((a) => a.group === g)
                  .map((a) => (
                    <AufgabeZeile key={a.id} a={a} neu={z.neu} />
                  ))}
              </Abschnitt>
            ))}
          </>
        )
      }}
    </Geladen>
  )
}

function AufgabeZeile({ a, neu }: { a: Task; neu: () => void }) {
  const stand = a.running
    ? t('auf.laeuft', { n: Math.round((a.progress || 0) * 100) })
    : a.lastRun
      ? (a.lastResult === 'error' ? t('auf.fehler', { zeit: vor(a.lastRun) }) : t('auf.zuletzt', { zeit: vor(a.lastRun) })) + (a.duration ? ' · ' + t('auf.dauer', { d: dauer(a.duration) }) : '')
      : t('auf.nie')
  return (
    <div class={'fl-zeile admin-aufgabe' + (a.lastResult === 'error' && !a.running ? ' admin-fehler' : '')}>
      <span class="text">
        <b>{a.name}</b>
        <small>{a.description}</small>
        <small class="admin-aufgabe-stand">
          {stand}
          {a.lastResult === 'error' && a.lastError && !a.running ? ' · ' + a.lastError : ''}
          {a.next && !a.running ? ' · ' + t('auf.naechste', { zeit: datum(a.next) }) : ''}
        </small>
        {a.running && <Balken anteil={a.progress || 0} />}
      </span>
      <span class="admin-rechts">
        <Button fokusKey={'auf-' + a.id} icon="abspielen" label={t('auf.starten', { name: a.name })} aus={a.running} onPress={() => aufgabeStarten(a.id).then(neu, meldeFehler)} />
      </span>
    </div>
  )
}
