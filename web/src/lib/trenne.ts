// Chromium 53 trennt nicht selbst: lange Wörter (ab 10 Zeichen) bekommen weiche Trennstellen (U+00AD), sonst laufen
// Versalien-Titel auf Platzhaltern über den Rand. Getrennt wird vor dem Silbenanfang vor jedem Vokal: sch, ch, ck, ph, th,
// Konsonant + l/r (Pro-gramm), st/sp am Ende einer längeren Konsonantengruppe (Gold-stän-der), sonst ein Konsonant
// (Lebens-ret-ter). Mindestens 3 Zeichen je Stück, spätestens nach 8 Zeichen. Selbsttest: node scripts/trenne-test.mjs
// ponytail: Silbenregeln statt Wörterbuch; Zusammensetzungen wie „Bei-spiel“ trennt es falsch („Beis-piel“).
const vokal = /[aeiouäöüy]/i
const paar = /^(ch|ck|ph|th|[bcdfgkpt][lr])$/

export function trenne(titel: string): string {
  return titel.replace(/[^\s\-–]{10,}/g, (w) => {
    const v = (k: number) => vokal.test(w[k] || '')
    const erlaubt: boolean[] = [] // erlaubt[p]: Trennung vor Zeichen p
    let gruppe = -1 // Beginn der laufenden Konsonantengruppe
    let vokalDa = false
    for (let j = 0; j < w.length; j++) {
      if (!v(j)) {
        if (gruppe < 0) gruppe = j
        continue
      }
      if (gruppe >= 0 && vokalDa) {
        const g = w.slice(gruppe, j).toLowerCase()
        let n = /sch$/.test(g) ? 3 : paar.test(g.slice(-2)) || (g.length > 2 && /s[tp]$/.test(g)) ? 2 : 1
        n = Math.min(n, g.length)
        if (/^c[kh]$/i.test(w.substr(j - n - 1, 2))) n-- // ck/ch nicht zerreißen (Glück-lich)
        if (n > 0) erlaubt[j - n] = true
        if (n === 2 && !/^(ch|ck|ph|th)$/.test(g.slice(-2))) erlaubt[j - 1] = true // zu nah? dann Not-ret-ter, Fens-ter
      }
      vokalDa = true
      gruppe = -1
    }
    let out = ''
    let seit = 0
    for (let i = 0; i < w.length; i++) {
      out += w[i]
      seit++
      if (w.length - i - 1 >= 3 && ((seit >= 3 && erlaubt[i + 1]) || seit >= 8)) {
        out += '­'
        seit = 0
      }
    }
    return out
  })
}
