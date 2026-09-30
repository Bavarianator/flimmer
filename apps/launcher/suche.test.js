// Prüft die Serversuche des Starters ohne Fernseher: node apps/launcher/suche.test.js
// Das Skript aus index.html läuft mit nachgebauten Browser- und TV-Schnittstellen; server = Adressen, die antworten.
const fs = require('fs')
const vm = require('vm')
const assert = require('assert')

const html = fs.readFileSync(__dirname + '/index.html', 'utf8')
const code = html.slice(html.indexOf('<script>') + 8, html.indexOf('</script>'))

function lauf({ server = [], tvIP = null, gespeichert = null }) {
  return new Promise((erledigt) => {
    let aus = false
    const fertig = (r) => aus || ((aus = true), erledigt(r))
    const el = {}
    const $ = (id) => (el[id] = el[id] || { id, textContent: '', className: 'hidden', value: '', focus() {} })
    const anfragen = []
    class XHR {
      open(_, url) { this.url = url.replace('/api/users', '') }
      send() {
        anfragen.push(this.url)
        setImmediate(() => (server.includes(this.url) ? ((this.status = 200), this.onload()) : this.onerror()))
      }
    }
    const PalmServiceBridge = tvIP &&
      function () {
        this.call = () => setImmediate(() => this.onservicecallback(JSON.stringify({ wifi: { ipAddress: tvIP } })))
      }
    const location = {
      set href(h) { fertig({ ziel: h, anfragen, el }) },
    }
    const ctx = {
      location, XMLHttpRequest: XHR, setTimeout, setImmediate,
      document: { getElementById: $, addEventListener() {} },
      localStorage: { getItem: () => gespeichert, setItem() {} },
    }
    if (PalmServiceBridge) ctx.PalmServiceBridge = PalmServiceBridge
    ctx.window = ctx // im Browser ist window der globale Bereich
    vm.runInNewContext(code, ctx)
    // manuell(): Eingabe sichtbar, keine Weiterleitung
    const warte = () => aus || (el.manuell && el.manuell.className === '' ? fertig({ ziel: null, anfragen, el }) : setTimeout(warte, 5))
    warte()
  })
}

;(async () => {
  let r = await lauf({ server: ['http://flimmer.local:8097'] })
  assert.strictEqual(r.ziel, 'http://flimmer.local:8097/', 'flimmer.local auf 8097 (8096 belegt)')

  r = await lauf({ server: ['http://192.168.55.190:8097'], tvIP: '192.168.55.30' })
  assert.strictEqual(r.ziel, 'http://192.168.55.190:8097/', 'Suche im eigenen Netz')
  assert.ok(!r.anfragen.some((u) => u.includes('192.168.55.0:') || u.includes('.255:')), 'nur .1 bis .254')

  r = await lauf({ tvIP: '10.0.0.7' })
  assert.strictEqual(r.ziel, null, 'nichts gefunden: Eingabe von Hand')
  assert.strictEqual(r.anfragen.filter((u) => u.startsWith('http://10.0.0.')).length, 254 * 2, 'alle Adressen, beide Ports')
  assert.match(r.el.hint.textContent, /Kein Server gefunden/)

  r = await lauf({ server: ['http://192.168.1.5:8096'], gespeichert: 'http://192.168.1.5:8096' })
  assert.deepStrictEqual([r.ziel, r.anfragen.length], ['http://192.168.1.5:8096/', 1], 'gespeicherte Adresse zuerst')
  console.log('ok: Starter findet den Server')
  process.exit(0) // offene 3-s-Zeitgeber aus tvIP nicht abwarten
})().catch((e) => { console.error(e.message); process.exit(1) })
