// Videos hochladen (/hochladen): nur mit Recht „Hochladen“, das der Admin unter Dashboard › Benutzer vergibt.
// Jede Datei geht einzeln als Body an POST /api/upload?name=… (XHR, weil fetch keinen Upload-Fortschritt kennt).
// Der Server legt sie unter <daten>/uploads ab und scannt danach selbst.
// Links lädt der Server selbst per yt-dlp (POST /api/upload/link), die Seite fragt den Stand jede Sekunde ab.
// Für Links mit Anmeldung (Teams der Firma …): das Lesezeichen „An Flimmer“ öffnet #/hochladen/<link>/<name>.
// Auf einer SharePoint/Stream-Seite nimmt es statt der Seitenadresse einen Link mit Zugangstoken aus g_fileInfo
// (wie yt-dlps SharePoint-Extraktor): den Original-Download mit tempauth, sonst das DASH-Manifest auf svc.ms.
// Notlösung bleibt eine cookies.txt; sie bleibt nur, solange die Seite offen ist.
import { useEffect, useState } from 'preact/hooks'
import { Eingabe } from '../components/Aktionen'
import { Button } from '../components/Button'
import { Icon } from '../components/Icon'
import { Seite, useIch } from '../components/Seite'
import { api } from '../lib/api'
import { ergaenze, t } from '../lib/i18n'
import { getToken } from '../profile'
import './browse.css'

ergaenze(
  {
    'hoch.text': 'Filme und Videos kommen in den Upload-Ordner von Flimmer und erscheinen nach dem Scan in der Bibliothek. Serienfolgen erkennt Flimmer am Namen, z. B. „Serie S01E02.mkv“.',
    'hoch.waehlen': 'Dateien wählen',
    'hoch.fertig': 'fertig',
    'hoch.nicht': 'Hochladen ist für dein Profil nicht freigegeben. Ein Admin kann es unter Dashboard › Benutzer erlauben.',
    'hoch.link': 'Link zu einem Video (YouTube, Teams, Mediathek, …)',
    'hoch.linkLaden': 'Von Link laden',
    'hoch.cookies': 'cookies.txt anhängen',
    'hoch.cookiesText': 'Klappt ein Link mit Anmeldung trotzdem nicht: die Cookies der Seite als cookies.txt exportieren (z. B. Erweiterung „Get cookies.txt LOCALLY“) und hier anhängen. Flimmer nutzt sie nur für deine Downloads und löscht sie danach.',
    'hoch.lesezeichen': 'An Flimmer',
    'hoch.lesezeichenText': 'Am leichtesten: Zieh „An Flimmer“ in die Lesezeichenleiste deines Browsers. Dann auf einer Video-Seite draufklicken – YouTube, Mediathek oder eine Teams-Aufnahme, die im Browser offen ist („In Stream öffnen“). Du bist dort ja schon angemeldet, Flimmer bekommt den Link fertig zum Laden.',
  },
  {
    'hoch.text': 'Movies and videos go to Flimmer’s upload folder and appear in the library after the scan. Episodes are recognised by name, e.g. “Show S01E02.mkv”.',
    'hoch.waehlen': 'Choose files',
    'hoch.fertig': 'done',
    'hoch.nicht': 'Uploading is not enabled for your profile. An admin can allow it under Dashboard › Users.',
    'hoch.link': 'Link to a video (YouTube, Teams, Vimeo, …)',
    'hoch.linkLaden': 'Download from link',
    'hoch.cookies': 'Attach cookies.txt',
    'hoch.cookiesText': 'If a link that needs a login still fails: export the site’s cookies as cookies.txt (e.g. extension “Get cookies.txt LOCALLY”) and attach it here. Flimmer uses it only for your downloads and deletes it afterwards.',
    'hoch.lesezeichen': 'To Flimmer',
    'hoch.lesezeichenText': 'Easiest: drag “To Flimmer” to your browser’s bookmarks bar. Then click it on a video page – YouTube, Vimeo or a Teams recording open in the browser (“Open in Stream”). You’re already signed in there, so Flimmer gets a ready-to-download link.',
  },
)

type Stand = { id: number; name: string; anteil: number; fehler?: string; fertig?: boolean }
type LinkJob = { id: number; url: string; name?: string; anteil: number; fehler?: string; fertig?: boolean }
let zaehler = 0

// Lesezeichen „An Flimmer“ (Bookmarklet). Bewusst ES5 als Text: es läuft auf fremden Seiten, nicht in unserem Bundle.
const lesezeichen = (ziel: string) =>
  'javascript:(function(){var f=window.g_fileInfo,l=location.href,n="";try{if(f&&f[".transformUrl"]){n=f.name||f.title||"";' +
  'if(/tempauth=/.test(f.downloadUrl||""))l=f.downloadUrl;else{var t=new URL(f[".transformUrl"]),' +
  'm=new URL("../videomanifest",t.origin+t.pathname+"/");m.search=t.search;var q=m.searchParams;' +
  'q.set("cTag",f[".ctag"]);q.set("action","Access");q.set("part","index");q.set("format","dash");l=m.href}}}catch(e){}' +
  'var u=' + JSON.stringify(ziel + '#/hochladen/') + '+encodeURIComponent(l)+"/"+encodeURIComponent(n);' +
  'window.open(u)||(location.href=u)})()'

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

export function Hochladen(p: { link?: string; name?: string }) {
  const ich = useIch()
  const [liste, setListe] = useState<Stand[]>([])
  const [links, setLinks] = useState<LinkJob[]>([])
  const [link, setLink] = useState(p.link || '')
  const [name, setName] = useState(p.name || '') // vom Lesezeichen, gilt nur für dessen Link
  const [linkFehler, setLinkFehler] = useState('')
  const [cookies, setCookies] = useState<{ name: string; text: string } | null>(null)
  const linksHolen = () => api<LinkJob[]>('/api/upload/link').then(setLinks, () => {})
  useEffect(() => void linksHolen(), [])
  useEffect(() => {
    if (!links.some((j) => !j.fertig && !j.fehler)) return
    const z = setTimeout(linksHolen, 1000)
    return () => clearTimeout(z)
  }, [links])
  const linkLaden = () => {
    if (!link.trim()) return
    setLinkFehler('')
    api('/api/upload/link', { url: link.trim(), name, cookies: cookies?.text }).then(
      () => (setLink(''), setName(''), linksHolen()),
      (e: Error) => setLinkFehler(e.message.replace(/^\d+ /, '')),
    )
  }
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
        {ich?.upload && (
          <div class="hochladen-link">
            <Eingabe fk="hoch-link" wert={link} setWert={(w) => (setLink(w), setName(''))} label={t('hoch.link')} onEnter={linkLaden} />
            <Button fokusKey="hoch-link-los" icon="hochladen" onPress={linkLaden} aus={!link.trim()}>
              {t('hoch.linkLaden')}
            </Button>
            <label class="fl-btn">
              <Icon name="hochladen" />
              {cookies ? cookies.name : t('hoch.cookies')}
              <input
                type="file"
                hidden
                accept=".txt,text/plain"
                onChange={(e) => {
                  const el = e.target as HTMLInputElement
                  const f = el.files && el.files[0]
                  if (f) f.text().then((text) => setCookies({ name: f.name, text }))
                  el.value = ''
                }}
              />
            </label>
            {linkFehler && <p class="fehlertext">{linkFehler}</p>}
            <p class="t-text leise">
              <a class="fl-btn" href={lesezeichen(location.origin + location.pathname)} onClick={(e) => e.preventDefault()}>
                <Icon name="hochladen" />
                {t('hoch.lesezeichen')}
              </a>
            </p>
            <p class="t-text leise">{t('hoch.lesezeichenText')}</p>
            <p class="t-text leise">{t('hoch.cookiesText')}</p>
          </div>
        )}
        {liste.concat(links.map((j) => ({ ...j, id: -j.id, name: j.name || j.url }))).map((x) => (
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
