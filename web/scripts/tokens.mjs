// Erzeugt aus src/design/tokens.json die CSS-Variablen (src/design/tokens.css) und die
// Kotlin-Tokens für die Android-App. Aufruf: node scripts/tokens.mjs
// Android: Tokens.kt wird nur neu angelegt; existiert sie schon (von ST gepflegt), landet die
// frische Fassung unter web/dist-tokens/Tokens.kt zum Abgleich.
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const web = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const t = JSON.parse(readFileSync(resolve(web, 'src/design/tokens.json'), 'utf8'))
const themes = t.color.themes.map((x) => x.id)
const flat = ['spacing', 'radius', 'shadow', 'dauer', 'kurve', 'masse'].flatMap((g) => t[g].tokens)
const val = (v, theme) => (typeof v === 'object' ? v[theme] : v)

// ---------- CSS ----------
const css = ['/* Erzeugt von scripts/tokens.mjs aus tokens.json – nicht von Hand ändern. */']
for (const f of t.type.fonts) {
  // Chromium 53 kennt keine variablen Achsen: statt der -var-Datei statische Instanzen 400/500/600.
  const statisch = /-var\./.test(f.file) ? ['400', '500', '600'].map((w) => ({ w, file: f.file.replace('-var.', '-' + w + '.') })) : [{ w: f.weight, file: f.file }]
  for (const x of statisch)
    css.push(`@font-face { font-family: '${f.family}'; font-weight: ${x.w}; font-style: ${f.style || 'normal'}; font-display: swap; src: url('/${x.file}') format('woff2'); }`)
}

const vars = (theme) => {
  const out = t.color.tokens.map((c) => `--${c.name}: ${val(c.value, theme)};`)
  for (const x of flat) if (typeof x.value === 'object') out.push(`--${x.name}: ${val(x.value, theme)};`)
  return out
}
const fixed = flat.filter((x) => typeof x.value !== 'object').map((x) => `--${x.name}: ${x.value};`)
for (const [k, v] of Object.entries(t.type.families)) fixed.push(`--font-${k}: ${v};`)
css.push(`:root, [data-theme="${themes[0]}"] {\n  ${vars(themes[0]).concat(fixed).join('\n  ')}\n}`)
for (const th of themes.slice(1)) css.push(`[data-theme="${th}"] {\n  ${vars(th).join('\n  ')}\n}`)

// Typo und Maße je Geräteklasse. Stilnamen tragen das Gerät als Präfix (tv-plakat, dt-text, hd-label).
// --t-<rolle> zeigt auf die Skala des aktuellen Geräts: dt ist Standard, schmale Fenster bekommen hd,
// device.ts setzt html.tv / html.dt / html.hd.
const masse = Object.fromEntries(t.masse.tokens.map((x) => [x.name, x.value]))
const rand = { tv: '96px', dt: '64px', hd: '16px' }
const stile = t.type.groups.flatMap((g) => g.styles.map((s) => ({ ...s, family: s.family || g.family || 'sans' })))
const geraete = ['tv', 'dt', 'hd']
const scale = (g) => {
  const out = []
  for (const s of stile.filter((x) => x.name.indexOf(g + '-') === 0)) {
    const rolle = s.name.slice(g.length + 1)
    out.push(`--t-${rolle}: ${s.fontWeight} ${s.fontSize}/${s.lineHeight} var(--font-${s.family});`, `--t-${rolle}-ls: ${s.letterSpacing || 'normal'};`)
  }
  out.push(`--karte-poster: ${masse['karte-poster-' + g]};`, `--karte-breit: ${masse['karte-breit-' + g]};`)
  out.push(`--fokus-ring: ${masse[g === 'tv' ? 'fokus-ring-tv' : 'fokus-ring-dt']};`, `--rand: ${rand[g]};`)
  return out.join('\n  ')
}
css.push(`:root {\n  ${scale('dt')}\n}`)
css.push(`@media (max-width: 599px) {\n  :root {\n  ${scale('hd')}\n  }\n}`)
for (const g of geraete) css.push(`html.${g} {\n  ${scale(g)}\n}`)
writeFileSync(resolve(web, 'src/design/tokens.css'), css.join('\n') + '\n')

// ---------- Kotlin ----------
const camel = (s) => s.replace(/-(\w)/g, (_, c) => c.toUpperCase()).replace(/^\w/, (c) => c.toUpperCase())
const color = (v) => {
  const m = /^rgba\((\d+),\s*(\d+),\s*(\d+),\s*([\d.]+)\)$/.exec(v)
  if (m) return `Color(${m[1]}, ${m[2]}, ${m[3]}, ${Math.round(Number(m[4]) * 255)})`
  return `Color(0xFF${v.slice(1).toUpperCase()})`
}
const px = (v) => parseFloat(v)
const kt = ['// Erzeugt von web/scripts/tokens.mjs aus web/src/design/tokens.json – Quelle ist die JSON-Datei.', 'package io.flimmer.app.ui', '', 'import androidx.compose.ui.graphics.Color', 'import androidx.compose.ui.unit.dp', 'import androidx.compose.ui.unit.sp', '', 'object Tokens {']
for (const th of themes) {
  kt.push(`    object ${camel(th)} {`)
  for (const c of t.color.tokens) kt.push(`        val ${camel(c.name)} = ${color(val(c.value, th))}`)
  kt.push('    }')
}
kt.push('    object Abstand {')
for (const x of t.spacing.tokens) kt.push(`        val ${camel(x.name)} = ${px(x.value)}.dp`)
kt.push('    }', '    object Radius {')
for (const x of t.radius.tokens) kt.push(`        val ${camel(x.name)} = ${px(x.value)}.dp`)
kt.push('    }', '    object Masse {')
for (const x of t.masse.tokens) kt.push(`        val ${camel(x.name)} = ${px(x.value)}.dp`)
kt.push('    }', '    object Dauer {')
for (const x of t.dauer.tokens) kt.push(x.name === 'fokus-scale' ? `        const val FokusScale = ${x.value}f` : `        const val ${camel(x.name)} = ${px(x.value)}`)
kt.push('    }')
for (const g of geraete) {
  kt.push(`    object Typo${camel(g)} {`)
  for (const st of stile.filter((x) => x.name.indexOf(g + '-') === 0)) kt.push(`        val ${camel(st.name.slice(g.length + 1))} = ${px(st.fontSize)}.sp`)
  kt.push('    }')
}
kt.push('}', '')
const android = resolve(web, '../apps/android/app/src/main/java/io/flimmer/app/ui/Tokens.kt')
const target = existsSync(dirname(android)) && !existsSync(android) ? android : resolve(web, 'dist-tokens/Tokens.kt')
mkdirSync(dirname(target), { recursive: true })
writeFileSync(target, kt.join('\n'))
console.log('tokens.css und ' + target + ' geschrieben')
