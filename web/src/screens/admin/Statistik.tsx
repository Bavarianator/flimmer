// Dashboard › Statistik: Sehzeit (wer, was, wann, wie), Bibliothek, Speicher und Downloads aus /api/admin/stats.
import { useState } from 'preact/hooks'
import { useDaten } from '../../lib/api'
import { statistik, type Statistik as Daten } from '../../lib/api-admin'
import { dauer, ergaenze, t } from '../../lib/i18n'
import { go } from '../../lib/router'
import { MiniAvatar } from '../../components/Avatar'
import { Abschnitt, Balken, Geladen, Messwert, Wahl, Zeile, groesse, methode, vor } from './teile'

ergaenze({
  'stat.tage': '{n} Tage',
  'stat.jahr': '1 Jahr',
  'stat.sehzeit': 'Sehzeit',
  'stat.profile': 'Aktive Profile',
  'stat.gesehen': 'Zu Ende gesehen',
  'stat.gesehen.text': '{n} von {m} Titeln',
  'stat.downloads': 'Downloads',
  'stat.protag': 'Sehzeit pro Tag',
  'stat.prowoche': 'Sehzeit pro Woche',
  'stat.spitze': 'Am meisten: {wert} am {wann}',
  'stat.tageszeit': 'Tageszeit',
  'stat.tageszeit.spitze': 'Am meisten wird um {h} Uhr geschaut.',
  'stat.uhr': '{h} Uhr',
  'stat.leer': 'In diesem Zeitraum wurde nichts geschaut.',
  'stat.nutzer': 'Profile',
  'stat.nutzer.titel': '{n} Titel',
  'stat.top': 'Meistgesehen',
  'stat.top.nutzer': '{n} Profile',
  'stat.top.nutzer1': '1 Profil',
  'stat.art': 'Wiedergabeart',
  'stat.unbekannt': 'Unbekannt',
  'stat.clients': 'Geräte und Apps',
  'stat.bibliothek': 'Bibliothek',
  'stat.zugang': 'Zugang pro Monat',
  'stat.zugang.text': 'Nach Dateidatum, {n} Titel in 12 Monaten',
  'stat.aufloesung': 'Auflösung',
  'stat.codecs': 'Video-Codecs',
  'stat.hdr': 'Davon HDR',
  'stat.anzahl': '{n} Titel',
  'stat.speicher': 'Speicher',
  'stat.belegt': '{frei} frei von {gesamt}',
  'stat.prognose': 'Bei etwa {zuwachs} pro Monat (Schnitt der letzten 3 Monate) reicht der Platz noch rund {monate} Monate.',
  'stat.prognose.lang': 'Bei etwa {zuwachs} pro Monat reicht der Platz noch mehrere Jahre.',
  'stat.prognose.keine': 'In den letzten 3 Monaten kam nichts dazu.',
  'stat.groesste': 'Größte Titel',
  'stat.quellen': 'Downloads nach Quelle',
  'stat.quelle.text': '{n} geladen · {g}',
  'stat.quelle.fehler': '{n} fehlgeschlagen',
  'stat.letzte': 'Letzte Downloads',
  'stat.keine.downloads': 'Noch keine Downloads.',
  'quelle.mediathek': 'Mediathek',
  'quelle.abo': 'Mediathek-Abo',
  'quelle.link': 'Von Link',
  'quelle.upload': 'Hochgeladen',
  'dl.wartet': 'Wartet',
  'dl.laeuft': 'Lädt',
  'dl.fertig': 'Fertig',
  'dl.fehler': 'Fehler',
  'dl.abgebrochen': 'Abgebrochen',
})

const MONATE = ['Jan', 'Feb', 'Mär', 'Apr', 'Mai', 'Jun', 'Jul', 'Aug', 'Sep', 'Okt', 'Nov', 'Dez']
const tagLabel = (iso: string) => {
  const [, m, d] = iso.split('-')
  return +d + '.' + +m + '.'
}
const monatLabel = (iso: string) => MONATE[+iso.split('-')[1] - 1]
const stunden = (sek: number) => (sek ? dauer(sek) : '0')
export const quelle = (q: string) => (t('quelle.' + q) === 'quelle.' + q ? q : t('quelle.' + q))

export function Statistik() {
  const [tage, setTage] = useState(30)
  const z = useDaten('admin.stats.' + tage, () => statistik(tage))
  return (
    <>
      <Wahl
        fk="stat-tage"
        werte={[
          [7, t('stat.tage', { n: 7 })],
          [30, t('stat.tage', { n: 30 })],
          [90, t('stat.tage', { n: 90 })],
          [365, t('stat.jahr')],
        ]}
        an={tage}
        onWahl={setTage}
      />
      <Geladen z={z}>{(d) => <Inhalt d={d} tage={tage} />}</Geladen>
    </>
  )
}

function Inhalt({ d, tage }: { d: Daten; tage: number }) {
  const w = d.wiedergabe
  const b = d.bibliothek
  const downloads = d.downloads.quellen.reduce((s, q) => s + q.anzahl, 0)
  // Ab 90 Tagen Wochen statt Tage, sonst werden die Säulen zu dünn.
  const woche = tage > 90
  const reihe: { tag: string; sekunden: number }[] = []
  w.proTag.forEach((p, i) => {
    if (!woche || i % 7 === 0) reihe.push({ ...p })
    else reihe[reihe.length - 1].sekunden += p.sekunden
  })
  const spitzeTag = reihe.reduce((a, p) => (p.sekunden > a.sekunden ? p : a), reihe[0])
  const spitzeStunde = w.proStunde.indexOf(Math.max(...w.proStunde))
  const sehzeit = w.methoden.reduce((s, m) => s + m.sekunden, 0) || 1
  const clientZeit = w.clients.reduce((s, c) => s + c.sekunden, 0) || 1
  const maxNutzer = Math.max(1, ...w.nutzer.map((n) => n.sekunden))
  const maxTitel = Math.max(1, ...w.titel.map((x) => x.sekunden))
  const mitVideo = b.aufloesung.reduce((s, a) => s + a.anzahl, 0) || 1
  const maxCodec = Math.max(1, ...b.codecs.map((c) => c.anzahl))
  const maxGroesse = Math.max(1, ...b.groesste.map((g) => g.bytes))
  const zugang = b.proMonat.reduce((s, m) => s + m.anzahl, 0)
  const sp = d.speicher
  const oeffnen = (x: { id: string; serie?: string }) => go(x.serie ? '/serie/' + encodeURIComponent(x.serie) : '/film/' + x.id)

  return (
    <>
      <section class="admin-kennzahlen">
        <Messwert label={t('stat.sehzeit')} wert={stunden(w.sekunden)} />
        <Messwert label={t('stat.profile')} wert={String(w.nutzer.length)} />
        <span class="fl-messwert admin-messwert">
          <small class="fl-label">{t('stat.gesehen')}</small>
          <b>{b.titel ? Math.round((b.gesehen / b.titel) * 100) + ' %' : '–'}</b>
          <small>{t('stat.gesehen.text', { n: b.gesehen, m: b.titel })}</small>
        </span>
        <Messwert label={t('stat.downloads')} wert={String(downloads)} />
      </section>

      {!w.sekunden ? (
        <Abschnitt titel={t('stat.sehzeit')} text={t('stat.leer')} />
      ) : (
        <>
          <Abschnitt
            titel={woche ? t('stat.prowoche') : t('stat.protag')}
            text={t('stat.spitze', { wert: stunden(spitzeTag.sekunden), wann: tagLabel(spitzeTag.tag) })}
          >
            <Saeulen
              werte={reihe.map((p) => p.sekunden)}
              name={(i) => tagLabel(reihe[i].tag)}
              format={stunden}
              achse={[tagLabel(reihe[0].tag), tagLabel(reihe[Math.floor(reihe.length / 2)].tag), tagLabel(reihe[reihe.length - 1].tag)]}
            />
          </Abschnitt>

          <Abschnitt titel={t('stat.tageszeit')} text={t('stat.tageszeit.spitze', { h: spitzeStunde })}>
            <Saeulen werte={w.proStunde} name={(i) => t('stat.uhr', { h: i })} format={stunden} achse={['0', '6', '12', '18', '23']} />
          </Abschnitt>

          <div class="admin-zwei">
            <Abschnitt titel={t('stat.nutzer')}>
              {w.nutzer.map((n) => (
                <Zeile
                  key={n.name}
                  titel={
                    <span class="fl-reihe-flex">
                      <MiniAvatar n={{ name: n.name, color: n.farbe }} class="admin-stat-avatar" />
                      {n.name}
                    </span>
                  }
                  unter={
                    <>
                      {t('stat.nutzer.titel', { n: n.titel })}
                      <Balken anteil={n.sekunden / maxNutzer} />
                    </>
                  }
                  wert={stunden(n.sekunden)}
                />
              ))}
            </Abschnitt>

            <Abschnitt titel={t('stat.top')}>
              {w.titel.map((x) => (
                <Zeile
                  key={x.id}
                  fk={'stat-top-' + x.id}
                  titel={<span class="eine-zeile">{x.titel}</span>}
                  unter={
                    <>
                      {x.nutzer === 1 ? t('stat.top.nutzer1') : t('stat.top.nutzer', { n: x.nutzer })}
                      <Balken anteil={x.sekunden / maxTitel} />
                    </>
                  }
                  wert={stunden(x.sekunden)}
                  onPress={x.titel !== '(entfernt)' ? () => oeffnen(x) : undefined}
                />
              ))}
            </Abschnitt>

            <Abschnitt titel={t('stat.art')}>
              {w.methoden.map((m) => (
                <Zeile
                  key={m.name}
                  titel={m.name ? methode(m.name) : t('stat.unbekannt')}
                  unter={<Balken anteil={m.sekunden / sehzeit} />}
                  wert={Math.round((m.sekunden / sehzeit) * 100) + ' %'}
                />
              ))}
            </Abschnitt>

            <Abschnitt titel={t('stat.clients')}>
              {w.clients.map((c) => (
                <Zeile
                  key={c.name}
                  titel={c.name || t('stat.unbekannt')}
                  unter={<Balken anteil={c.sekunden / clientZeit} />}
                  wert={Math.round((c.sekunden / clientZeit) * 100) + ' %'}
                />
              ))}
            </Abschnitt>
          </div>
        </>
      )}

      <Abschnitt titel={t('stat.zugang')} text={t('stat.zugang.text', { n: zugang })}>
        <Saeulen
          werte={b.proMonat.map((m) => m.bytes)}
          name={(i) => monatLabel(b.proMonat[i].monat) + ' · ' + t('stat.anzahl', { n: b.proMonat[i].anzahl })}
          format={groesse}
          achse={b.proMonat.map((m) => monatLabel(m.monat))}
        />
      </Abschnitt>

      <div class="admin-zwei">
        <Abschnitt titel={t('stat.aufloesung')}>
          {b.aufloesung.map((a) => (
            <Zeile
              key={a.name}
              titel={a.name}
              unter={
                <>
                  {t('stat.anzahl', { n: a.anzahl })}
                  <Balken anteil={a.anzahl / mitVideo} />
                </>
              }
              wert={a.bytes ? groesse(a.bytes) : '–'}
            />
          ))}
          <Zeile titel={t('stat.hdr')} wert={t('stat.anzahl', { n: b.hdr })} />
        </Abschnitt>

        <Abschnitt titel={t('stat.codecs')}>
          {b.codecs.map((c) => (
            <Zeile key={c.name} titel={c.name} unter={<Balken anteil={c.anzahl / maxCodec} />} wert={t('stat.anzahl', { n: c.anzahl })} />
          ))}
        </Abschnitt>

        <Abschnitt titel={t('stat.speicher')} text={sp.monateBisVoll === undefined ? t('stat.prognose.keine') : prognose(sp.zuwachsMonat, sp.monateBisVoll)}>
          <section class="admin-kennzahlen">
            {sp.laufwerke.map((l) => (
              <span key={l.path} class="fl-messwert admin-messwert" title={l.path}>
                <small class="fl-label eine-zeile">{l.path}</small>
                <b>{groesse(l.total - l.free)}</b>
                <small class="eine-zeile">{t('stat.belegt', { frei: groesse(l.free), gesamt: groesse(l.total) })}</small>
                <Balken anteil={l.total ? 1 - l.free / l.total : 0} />
              </span>
            ))}
          </section>
        </Abschnitt>

        <Abschnitt titel={t('stat.groesste')}>
          {b.groesste.map((g) => (
            <Zeile
              key={g.id}
              fk={'stat-gross-' + g.id}
              titel={<span class="eine-zeile">{g.titel}</span>}
              unter={<Balken anteil={g.bytes / maxGroesse} />}
              wert={groesse(g.bytes)}
              onPress={() => oeffnen(g)}
            />
          ))}
        </Abschnitt>

        <Abschnitt titel={t('stat.quellen')}>
          {!d.downloads.quellen.length && <p class="t-text leise">{t('stat.keine.downloads')}</p>}
          {d.downloads.quellen.map((q) => (
            <Zeile
              key={q.quelle}
              class={q.fehler && !q.anzahl ? 'admin-fehler' : undefined}
              titel={quelle(q.quelle)}
              unter={t('stat.quelle.text', { n: q.anzahl, g: groesse(q.bytes) }) + (q.fehler ? ' · ' + t('stat.quelle.fehler', { n: q.fehler }) : '')}
              wert={String(q.anzahl)}
            />
          ))}
        </Abschnitt>

        <Abschnitt titel={t('stat.letzte')}>
          {!d.downloads.letzte.length && <p class="t-text leise">{t('stat.keine.downloads')}</p>}
          {d.downloads.letzte.map((x) => (
            <Zeile
              key={x.id}
              class={x.status === 'fehler' ? 'admin-fehler' : undefined}
              titel={<span class="eine-zeile">{x.titel}</span>}
              unter={quelle(x.quelle) + ' · ' + t('dl.' + x.status) + (x.fehler ? ' · ' + x.fehler : '')}
              wert={vor(x.ende || x.erstellt)}
            />
          ))}
        </Abschnitt>
      </div>
    </>
  )
}

function prognose(zuwachs: number, monate: number) {
  if (monate > 60) return t('stat.prognose.lang', { zuwachs: groesse(zuwachs) })
  return t('stat.prognose', { zuwachs: groesse(zuwachs), monate: Math.max(0, Math.round(monate)) })
}

// Säulen einer Reihe: eine Farbe, Höhe relativ zum Höchstwert, Wert als Tooltip; die Zusammenfassung steht im Text darüber.
function Saeulen({ werte, name, format, achse }: { werte: number[]; name: (i: number) => string; format: (v: number) => string; achse: string[] }) {
  const max = Math.max(0, ...werte)
  return (
    <div class="admin-diagramm">
      <div class="admin-saeulen" role="img" aria-label={werte.map((v, i) => name(i) + ': ' + format(v)).join(', ')}>
        {werte.map((v, i) => (
          <span key={i} class="admin-saeule" title={name(i) + ': ' + format(v)}>
            <i style={{ height: max && v ? Math.max(2, (v / max) * 100) + '%' : '0' }} />
          </span>
        ))}
      </div>
      <div class="admin-achse fl-reihe-flex">
        {achse.map((a, i) => (
          <span key={i}>{a}</span>
        ))}
      </div>
    </div>
  )
}
