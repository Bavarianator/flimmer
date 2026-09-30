// Bundle-Budget: misst, was dist/index.html beim Start lädt (gzip), und endet bei Überschreitung mit Exit 1.
// Aufruf nach `vite build`: node scripts/size-check.mjs
import { readFileSync } from 'node:fs'
import { gzipSync } from 'node:zlib'

const dist = new URL('../dist/', import.meta.url)
const budget = { modern: 80, legacy: 120, css: 20 } // KB gzip; legacy = Chrome 53 inkl. Polyfills

const html = readFileSync(new URL('index.html', dist), 'utf8')
const gz = (f) => gzipSync(readFileSync(new URL(f.replace(/^\//, ''), dist))).length
const alle = (re) => [...html.matchAll(re)].map((m) => m[1])

// Legacy-Chunks laden ihre Abhängigkeiten per System.register([...]) – die statischen Importe mitzählen.
function legacyKette(f, seen = new Set()) {
  if (seen.has(f)) return seen
  seen.add(f)
  const js = readFileSync(new URL(f.replace(/^\//, ''), dist), 'utf8')
  const m = js.match(/System\.register\(\[([^\]]*)\]/)
  for (const d of m ? m[1].match(/[\w.-]+\.js/g) || [] : []) legacyKette('assets/' + d, seen)
  return seen
}

const gruppen = {
  modern: alle(/<script type="module"[^>]*src="([^"]+)"/g).concat(alle(/<link rel="modulepreload"[^>]*href="([^"]+)"/g)),
  legacy: [...alle(/id="vite-legacy-polyfill" src="([^"]+)"/g), ...alle(/id="vite-legacy-entry" data-src="([^"]+)"/g).flatMap((e) => [...legacyKette(e)])],
  css: alle(/<link rel="stylesheet"[^>]*href="([^"]+)"/g),
}

let ok = true
for (const [name, dateien] of Object.entries(gruppen)) {
  const kb = dateien.reduce((s, f) => s + gz(f), 0) / 1024
  const zuViel = kb > budget[name]
  ok = ok && !zuViel
  console.log(`${zuViel ? '✗' : '✓'} ${name.padEnd(6)} ${kb.toFixed(1).padStart(6)} KB gzip (Budget ${budget[name]} KB)  ${dateien.map((f) => f.split('/').pop()).join(', ')}`)
}
process.exit(ok ? 0 : 1)
