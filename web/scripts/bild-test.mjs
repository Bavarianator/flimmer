// Selbsttest der Bildanpassung (src/player/bild.ts), ohne Test-Framework: node scripts/bild-test.mjs
// Node ≥ 23.6 liest die .ts-Datei direkt (Type-Stripping).
import assert from 'node:assert/strict'
import { bildLage } from '../src/player/bild.ts'

const nah = (a, b, m) => assert.ok(Math.abs(a - b) < 0.01, `${m}: ${a} ≠ ${b}`)

// Die vier Beispiele aus docs/umbau-jellyfin.md (Automatisch, Balken eingebrannt → crop = Bildinhalt).
// Video 1920×1080 mit eingebranntem 2,39:1-Bild: crop.h = (1920/2.39)/1080.
const scope = { x: 0, y: (1 - 1920 / 2.39 / 1080) / 2, w: 1, h: 1920 / 2.39 / 1080 }
const flat = { x: 0, y: (1 - 1920 / 1.85 / 1080) / 2, w: 1, h: 1920 / 1.85 / 1080 }
const faelle = [
  ['2,39:1 auf 20:9-Handy', 2400, 1080, scope, 'fuellen'],
  ['1,85:1 auf 16:9', 1920, 1080, flat, 'fuellen'],
  ['2,39:1 auf 16:9-TV', 1920, 1080, scope, 'einpassen'],
  ['16:9 auf 20:9-Handy', 2400, 1080, null, 'einpassen'],
]
for (const [name, W, H, crop, erwartet] of faelle) {
  const l = bildLage('auto', W, H, 1920, 1080, crop)
  assert.equal(l.modus, erwartet, name)
  // Die Mitte des Ausschnitts liegt in der Bildschirmmitte.
  const c = crop || { x: 0, y: 0, w: 1, h: 1 }
  nah(l.links + (c.x + c.w / 2) * l.breite, W / 2, name + ' Mitte x')
  nah(l.oben + (c.y + c.h / 2) * l.hoehe, H / 2, name + ' Mitte y')
}

// Einpassen ignoriert crop: 16:9 auf 16:9 füllt genau.
let l = bildLage('einpassen', 1920, 1080, 1920, 1080, scope)
nah(l.links, 0, 'einpassen links'), nah(l.oben, 0, 'einpassen oben'), nah(l.breite, 1920, 'einpassen breite')
// 2,39:1 eingebrannt, Füllen auf 16:9: Höhe des Ausschnitts = Bildschirmhöhe.
l = bildLage('fuellen', 1920, 1080, 1920, 1080, scope)
nah(scope.h * l.hoehe, 1080, 'füllen höhe')
// Strecken: Ausschnitt deckt den Bildschirm in beiden Richtungen genau.
l = bildLage('strecken', 1000, 1000, 1920, 1080, scope)
nah(scope.w * l.breite, 1000, 'strecken breite'), nah(scope.h * l.hoehe, 1000, 'strecken höhe')
nah(l.oben + scope.y * l.hoehe, 0, 'strecken oben')

console.log('bild-test: ok')
