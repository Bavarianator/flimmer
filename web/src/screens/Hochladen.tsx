// Videos hochladen (/hochladen): nur mit Recht „Hochladen“, das der Admin unter Dashboard › Benutzer vergibt.
// Jede Datei geht einzeln als Body an POST /api/upload?name=… (XHR, weil fetch keinen Upload-Fortschritt kennt).
// Der Server legt sie unter <daten>/uploads ab und scannt danach selbst.
import { useState } from 'preact/hooks'
import { Icon } from '../components/Icon'
import { Seite, useIch } from '../components/Seite'
import { ergaenze, t } from '../lib/i18n'
import { getToken } from '../profile'
import './browse.css'

ergaenze(
  {
    'hoch.text': 'Filme und Videos kommen in den Upload-Ordner von Flimmer und erscheinen nach dem Scan in der Bibliothek. Serienfolgen erkennt Flimmer am Namen, z. B. „Serie S01E02.mkv“.',
    'hoch.waehlen': 'Dateien wählen',
    'hoch.fertig': 'fertig',
    'hoch.nicht': 'Hochladen ist für dein Profil nicht freigegeben. Ein Admin kann es unter Dashboard › Benutzer erlauben.',
  },
  {
    'hoch.text': 'Movies and videos go to Flimmer’s upload folder and appear in the library after the scan. Episodes are recognised by name, e.g. “Show S01E02.mkv”.',
    'hoch.waehlen': 'Choose files',
    'hoch.fertig': 'done',
    'hoch.nicht': 'Uploading is not enabled for your profile. An admin can allow it under Dashboard › Users.',
  },
)

type Stand = { id: number; name: string; anteil: number; fehler?: string; fertig?: boolean }
let zaehler = 0

function senden(f: File, fortschritt: (a: number) => void): Promise<void> {
  return new Promise((ok, nein) => {
    const x = new XMLHttpRequest()
    x.open('POST', '/api/upload?name=' + encodeURIComponent(f.name))
    const tok = getToken()
    if (tok) x.setRequestHeader('Authorization', 'Bearer ' + tok)
    x.upload.onprogress = (e) => e.lengthComputable && fortschritt(e.loaded / e.total)
    x.onload = () => (x.status < 300 ? ok() : nein(new Error(x.responseText.trim() || String(x.status))))
    x.onerror = () => nein(new Error(t('player.fehler.netz')))
    x.send(f)
  })
}

export function Hochladen() {
  const ich = useIch()
  const [liste, setListe] = useState<Stand[]>([])
  // Nacheinander: das NAS schreibt eine Datei zur Zeit, der Fortschritt bleibt ehrlich.
  const start = async (files: File[]) => {
    const neu = files.map((f) => ({ id: ++zaehler, name: f.name, anteil: 0 }))
    setListe((l) => l.concat(neu))
    for (let i = 0; i < files.length; i++) {
      const setze = (s: Partial<Stand>) => setListe((l) => l.map((x) => (x.id === neu[i].id ? { ...x, ...s } : x)))
      await senden(files[i], (a) => setze({ anteil: a })).then(
        () => setze({ anteil: 1, fertig: true }),
        (e: Error) => setze({ fehler: e.message }),
      )
    }
  }
  return (
    <Seite titel={t('nav.hochladen')}>
      <div class="rand hochladen">
        <p class="t-text leise">{t('hoch.text')}</p>
        {ich && !ich.upload ? (
          <p class="t-text">{t('hoch.nicht')}</p>
        ) : (
          <label class="fl-btn primaer">
            <Icon name="hochladen" />
            {t('hoch.waehlen')}
            <input
              type="file"
              multiple
              hidden
              accept="video/*,.mkv,.m4v,.mov,.avi,.ts,.m2ts,.webm,.wmv,.mpg"
              onChange={(e) => {
                const el = e.target as HTMLInputElement
                if (el.files && el.files.length) start(Array.prototype.slice.call(el.files))
                el.value = '' // dieselbe Datei nochmal wählbar
              }}
            />
          </label>
        )}
        {liste.map((x) => (
          <div key={x.id} class="hochladen-datei">
            <div class="zeile">
              <span class="wachse eine-zeile">{x.name}</span>
              <span class={x.fehler ? 'fehlertext' : 'leise'}>{x.fehler || (x.fertig ? t('hoch.fertig') : Math.round(x.anteil * 100) + ' %')}</span>
            </div>
            <div class="fl-fortschritt" aria-hidden="true">
              <i style={{ transform: 'scaleX(' + x.anteil + ')', webkitTransform: 'scaleX(' + x.anteil + ')' }} />
            </div>
          </div>
        ))}
      </div>
    </Seite>
  )
}
