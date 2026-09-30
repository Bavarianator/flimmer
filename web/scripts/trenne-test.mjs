// Selbsttest der Silbentrennung (src/lib/trenne.ts), ohne Test-Framework: node scripts/trenne-test.mjs
import assert from 'node:assert/strict'
import { trenne } from '../src/lib/trenne.ts'

const faelle = {
  LEBENSRETTER: 'LEBENS·RET·TER',
  SEENOTRETTER: 'SEE·NOT·RET·TER',
  PROGRAMMFÜHRER: 'PRO·GRAMM·FÜH·RER',
  TAGESSCHAU: 'TAGES·SCHAU',
  WIRTSCHAFT: 'WIRT·SCHAFT',
  GOLDSTÄNDER: 'GOLD·STÄN·DER',
  KULTURZEIT: 'KUL·TUR·ZEIT',
  NACHRICHTEN: 'NACH·RICH·TEN',
  GLÜCKLICHER: 'GLÜCK·LICHER',
  KROKODIL: 'KROKODIL', // unter 10 Zeichen: bleibt
  'Die Lebensretter von Murnau – Einsatz': 'Die Lebens·ret·ter von Murnau – Einsatz',
}
for (const [ein, soll] of Object.entries(faelle)) assert.equal(trenne(ein).replace(/­/g, '·'), soll, ein)
console.log('trenne: ' + Object.keys(faelle).length + ' Fälle ok')
