// Dashboard › Benutzer: Liste links, Bearbeiten rechts (Handy: darunter). Der Server kennt Name, Admin und PIN;
// Altersfreigabe, Tags und Zugangsplan aus dem Entwurf gibt es (noch) nicht.
import { useEffect, useState } from 'preact/hooks'
import { useDaten, type Nutzer } from '../../lib/api'
import { benutzer, benutzerLoeschen, benutzerNeu, benutzerSetzen } from '../../lib/api-admin'
import { fokusBald } from '../../lib/focus'
import { ergaenze, t } from '../../lib/i18n'
import { Button } from '../../components/Button'
import { MiniAvatar } from '../../components/Avatar'
import { Abschnitt, Feld, Geladen, Zeile, melde, meldeFehler } from './teile'

ergaenze({
  'ben.neu': 'Benutzer hinzufügen',
  'ben.anzahl': '{n} Benutzer · {a} Administrator',
  'ben.admin': 'Administrator',
  'ben.benutzer': 'Benutzer',
  'ben.mitpin': 'mit PIN',
  'ben.ohnepin': 'ohne PIN (nur im Heimnetz)',
  'ben.bearbeiten': '{name} bearbeiten',
  'ben.anlegen.titel': 'Neuer Benutzer',
  'ben.name': 'Name',
  'ben.pin': 'PIN oder Passwort',
  'ben.pin.neu': 'Neue PIN',
  'ben.pin.text': 'Mindestens 4 Zeichen. Admins brauchen immer eine PIN. Ohne PIN kommt das Profil nur im Heimnetz an.',
  'ben.pin.weg': 'PIN entfernen',
  'ben.admin.text': 'Darf das Dashboard öffnen und alle Einstellungen ändern.',
  'ben.upload': 'Darf Videos hochladen',
  'ben.upload.text': 'Über das Profilmenü › Videos hochladen, in den Upload-Ordner von Flimmer.',
  'ben.anlegen': 'Anlegen',
  'ben.loeschen': 'Benutzer löschen',
  'ben.loeschen.sicher': 'Wirklich löschen?',
  'ben.gaeste': 'Gäste aus Einladungen erscheinen nicht in dieser Liste.',
})

export function Benutzer() {
  const z = useDaten('admin.benutzer', benutzer)
  const [wahl, setWahl] = useState('')
  const liste = z.daten || []
  const u = liste.filter((x) => x.id === wahl)[0]
  useEffect(() => {
    if (!wahl && liste.length) setWahl(liste[0].id)
  }, [liste.length])
  return (
    <Geladen z={z}>
      {(l) => (
        <div class="admin-spalten">
          <div class="admin-liste">
            <Button fokusKey="ben-neu" icon="plus" onPress={() => (setWahl('neu'), fokusBald('ben-name'))}>
              {t('ben.neu')}
            </Button>
            <div class="fl-gruppe">
              {l.map((x) => (
                <Zeile
                  key={x.id}
                  fk={'ben-' + x.id}
                  an={x.id === wahl}
                  titel={
                    <span class="fl-reihe-flex">
                      <MiniAvatar n={x} class="admin-avatar" />
                      <span class="eine-zeile">{x.name}</span>
                    </span>
                  }
                  unter={(x.admin ? t('ben.admin') : t('ben.benutzer')) + ' · ' + (x.hasPassword ? t('ben.mitpin') : t('ben.ohnepin'))}
                  onPress={() => setWahl(x.id)}
                />
              ))}
            </div>
            <p class="t-klein leise">
              {t('ben.anzahl', { n: l.length, a: l.filter((x) => x.admin).length })}. {t('ben.gaeste')}
            </p>
          </div>
          <div class="admin-detail">
            {wahl === 'neu' ? (
              <Neu
                fertig={(id) => {
                  z.neu()
                  setWahl(id)
                }}
              />
            ) : (
              u && <Bearbeiten key={u.id} u={u} neu={z.neu} weg={() => (setWahl(''), z.neu())} />
            )}
          </div>
        </div>
      )}
    </Geladen>
  )
}

function Neu({ fertig }: { fertig: (id: string) => void }) {
  const [name, setName] = useState('')
  const [pin, setPin] = useState('')
  const [admin, setAdmin] = useState(false)
  const anlegen = () =>
    benutzerNeu({ name, password: pin || undefined, admin, color: Math.floor(Math.random() * 360) }).then((u) => {
      melde(t('einst.gespeichert'))
      fertig(u.id)
    }, meldeFehler)
  return (
    <Abschnitt titel={t('ben.anlegen.titel')}>
      <div class="admin-formzeile">
        <Feld fk="ben-name" wert={name} setWert={setName} label={t('ben.name')} zeigeLabel />
        <Feld fk="ben-pin" wert={pin} setWert={setPin} label={t('ben.pin')} typ="password" zeigeLabel />
      </div>
      <p class="t-klein leise admin-absatz">{t('ben.pin.text')}</p>
      <Zeile fk="ben-admin" icon="schluessel" titel={t('ben.admin')} unter={t('ben.admin.text')} schalter={admin} onPress={() => setAdmin(!admin)} />
      <div class="admin-knoepfe">
        <Button fokusKey="ben-anlegen" variante="primaer" aus={!name.trim()} onPress={anlegen}>
          {t('ben.anlegen')}
        </Button>
      </div>
    </Abschnitt>
  )
}

function Bearbeiten({ u, neu, weg }: { u: Nutzer; neu: () => void; weg: () => void }) {
  const [name, setName] = useState(u.name)
  const [pin, setPin] = useState('')
  const [sicher, setSicher] = useState(false)
  const setzen = (teil: Parameters<typeof benutzerSetzen>[1]) =>
    benutzerSetzen(u.id, teil).then(() => {
      melde(t('einst.gespeichert'))
      setPin('')
      neu()
    }, meldeFehler)
  return (
    <Abschnitt
      titel={t('ben.bearbeiten', { name: u.name })}
      zusatz={<MiniAvatar n={u} class="admin-avatar gross" />}
    >
      <div class="admin-formzeile">
        <Feld fk="ben-name" wert={name} setWert={setName} label={t('ben.name')} zeigeLabel onEnter={() => setzen({ name })} />
        <Button fokusKey="ben-name-ok" aus={!name.trim() || name === u.name} onPress={() => setzen({ name })}>
          {t('einst.speichern')}
        </Button>
      </div>
      <Zeile fk="ben-admin" icon="schluessel" titel={t('ben.admin')} unter={t('ben.admin.text')} schalter={u.admin} onPress={() => setzen({ admin: !u.admin })} />
      {!u.admin && <Zeile fk="ben-upload" icon="hochladen" titel={t('ben.upload')} unter={t('ben.upload.text')} schalter={!!u.upload} onPress={() => setzen({ upload: !u.upload })} />}
      <div class="admin-formzeile">
        <Feld fk="ben-pin" wert={pin} setWert={setPin} label={t('ben.pin.neu')} typ="password" zeigeLabel onEnter={() => pin && setzen({ password: pin })} />
        <Button fokusKey="ben-pin-ok" aus={!pin} onPress={() => setzen({ password: pin })}>
          {t('einst.speichern')}
        </Button>
        {u.hasPassword && !u.admin && (
          <Button fokusKey="ben-pin-weg" variante="geist" onPress={() => setzen({ password: '' })}>
            {t('ben.pin.weg')}
          </Button>
        )}
      </div>
      <p class="t-klein leise admin-absatz">{t('ben.pin.text')}</p>
      <div class="admin-knoepfe">
        <Button fokusKey="ben-loeschen" icon="loeschen" variante="geist" onPress={() => (sicher ? benutzerLoeschen(u.id).then(weg, meldeFehler) : setSicher(true))}>
          {t(sicher ? 'ben.loeschen.sicher' : 'ben.loeschen')}
        </Button>
      </div>
    </Abschnitt>
  )
}
