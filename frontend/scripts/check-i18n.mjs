// Checks the UI translations (no dependencies):
//  - every t('…') / tc('ctx', '…') literal key has an entry in src/lib/i18n.en.ts
//    (exit 1 if not), with the same {0}, {1}… placeholders;
//  - warns about Spanish-looking text that is not wrapped in t(): JSX text and
//    string literals with á é í ó ú ñ ¿ ¡. A line containing "i18n-ignore" is
//    skipped on purpose (e.g. language names shown in their own language).
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const src = join(root, 'src')
const dictFile = join(src, 'lib', 'i18n.en.ts')
const skip = new Set([join(src, 'lib', 'mock.ts'), dictFile])
const spanish = /[áéíóúñÁÉÍÓÚÑ¿¡]/

function walk(dir) {
  return readdirSync(dir).flatMap(n => {
    const p = join(dir, n)
    if (statSync(p).isDirectory()) return walk(p)
    return /\.(ts|tsx)$/.test(n) && !n.endsWith('.d.ts') && !skip.has(p) ? [p] : []
  })
}

function unescape(raw) {
  return raw.replace(/\\(u\{[0-9a-fA-F]+\}|u[0-9a-fA-F]{4}|x[0-9a-fA-F]{2}|.)/gs, (_, e) => {
    if (e[0] === 'u') return String.fromCodePoint(parseInt(e.replace(/[u{}]/g, ''), 16))
    if (e[0] === 'x' && e.length === 3) return String.fromCharCode(parseInt(e.slice(1), 16))
    return { n: '\n', t: '\t', r: '\r', 0: '\0' }[e] ?? e
  })
}

/**
 * Minimal JS/TS scanner: returns the string literals (with offsets and whether
 * a template literal has ${} parts) and the source with comments and string
 * contents blanked out, so JSX text can be found with a regex.
 */
function scan(code) {
  const strings = []
  const out = code.split('')
  const blank = (a, b) => { for (let i = a; i < b; i++) if (out[i] !== '\n') out[i] = ' ' }
  const stack = [] // brace depths of open ${ } inside template literals
  let i = 0
  const readTemplate = (start, from) => {
    // reads a template chunk from `from` until ` or ${
    let j = from
    while (j < code.length && code[j] !== '`' && !(code[j] === '$' && code[j + 1] === '{')) {
      if (code[j] === '\\') j++
      j++
    }
    const parts = code.slice(from, j)
    return { end: j, parts, start }
  }
  while (i < code.length) {
    const c = code[i], d = code[i + 1]
    if (c === '/' && d === '/') { const e = code.indexOf('\n', i); const end = e < 0 ? code.length : e; blank(i, end); i = end; continue }
    if (c === '/' && d === '*') { const e = code.indexOf('*/', i + 2); const end = e < 0 ? code.length : e + 2; blank(i, end); i = end; continue }
    if (c === "'" || c === '"') {
      let j = i + 1
      while (j < code.length && code[j] !== c && code[j] !== '\n') { if (code[j] === '\\') j++; j++ }
      strings.push({ start: i, end: j + 1, quote: c, raw: code.slice(i + 1, j), dynamic: false })
      blank(i + 1, j); i = j + 1; continue
    }
    if (c === '`' || (c === '}' && stack.length && stack[stack.length - 1].depth === 0)) {
      let start = i
      let tpl // current template literal being collected
      if (c === '}') { const top = stack.pop(); tpl = top.tpl; start = tpl.start } else { tpl = { start: i, raw: '', dynamic: false } }
      const r = readTemplate(start, i + 1)
      tpl.raw += r.parts
      blank(i + 1, r.end)
      if (code[r.end] === '`') {
        strings.push({ start: tpl.start, end: r.end + 1, quote: '`', raw: tpl.raw, dynamic: tpl.dynamic })
        i = r.end + 1; continue
      }
      // ${ … }: back to code until the matching }
      tpl.dynamic = true; tpl.raw += '${}'
      stack.push({ depth: 0, tpl })
      i = r.end + 2; continue
    }
    if (stack.length) {
      if (c === '{') stack[stack.length - 1].depth++
      else if (c === '}') stack[stack.length - 1].depth--
    }
    i++
  }
  return { strings, blanked: out.join('') }
}

const placeholders = s => [...s.matchAll(/\{(\d+)\}/g)].map(m => m[1]).sort().join(',')
const lineOf = (code, idx) => code.slice(0, idx).split('\n').length

// --- dictionary --------------------------------------------------------------
const dictSrc = readFileSync(dictFile, 'utf8')
const dict = new Map()
const errors = []
const warnings = []
{
  const { strings } = scan(dictSrc)
  // entries are `'key': 'value',` pairs: a string followed by ':' then a string
  for (let k = 0; k + 1 < strings.length; k++) {
    const a = strings[k], b = strings[k + 1]
    if (/^\s*:\s*$/.test(dictSrc.slice(a.end, b.start))) {
      const key = unescape(a.raw), val = unescape(b.raw)
      if (dict.has(key)) errors.push(`i18n.en.ts: duplicate key ${JSON.stringify(key)}`)
      dict.set(key, val)
      if (placeholders(key) !== placeholders(val)) errors.push(`i18n.en.ts: placeholders differ for ${JSON.stringify(key)} → ${JSON.stringify(val)}`)
      k++
    }
  }
}

// --- sources -----------------------------------------------------------------
const used = new Set()
const missing = new Map() // key -> first location
for (const file of walk(src)) {
  const code = readFileSync(file, 'utf8')
  const rel = relative(root, file)
  const lines = code.split('\n')
  const ignored = idx => lines[lineOf(code, idx) - 1].includes('i18n-ignore')
  const { strings, blanked } = scan(code)
  const keyIdx = new Set()

  strings.forEach((s, n) => {
    const before = blanked.slice(Math.max(0, s.start - 200), s.start)
    let key = null
    if (/(?<![\w.$])t\(\s*$/.test(before)) key = unescape(s.raw)
    else if (n > 0 && /(?<![\w.$])tc\(\s*$/.test(blanked.slice(Math.max(0, strings[n - 1].start - 200), strings[n - 1].start))
      && /^\s*,\s*$/.test(code.slice(strings[n - 1].end, s.start))) key = `${unescape(strings[n - 1].raw)}|${unescape(s.raw)}`
    if (key === null) return
    keyIdx.add(n)
    if (s.dynamic) { warnings.push(`${rel}:${lineOf(code, s.start)}: t() with a dynamic template literal can't be checked`); return }
    used.add(key)
    if (!dict.has(key) && !missing.has(key)) missing.set(key, `${rel}:${lineOf(code, s.start)}`)
  })
  // the context argument of tc() is not UI text
  strings.forEach((s, n) => { if (keyIdx.has(n + 1) && /(?<![\w.$])tc\(\s*$/.test(blanked.slice(Math.max(0, s.start - 200), s.start))) keyIdx.add(n) })

  strings.forEach((s, n) => {
    if (keyIdx.has(n) || !spanish.test(s.raw) || ignored(s.start)) return
    warnings.push(`${rel}:${lineOf(code, s.start)}: untranslated string ${s.quote}${s.raw}${s.quote}`)
  })
  if (file.endsWith('.tsx')) {
    for (const m of blanked.matchAll(/>([^<>{}]*)</g)) {
      const text = code.slice(m.index + 1, m.index + 1 + m[1].length)
      if (spanish.test(m[1]) && !ignored(m.index)) warnings.push(`${rel}:${lineOf(code, m.index)}: untranslated JSX text ${JSON.stringify(text.trim())}`)
    }
  }
}

for (const [k, where] of missing) errors.push(`${where}: missing English for ${JSON.stringify(k)}`)
const unused = [...dict.keys()].filter(k => !used.has(k))
for (const k of unused) warnings.push(`i18n.en.ts: unused key ${JSON.stringify(k)}`)

for (const w of warnings) console.warn(`warning: ${w}`)
for (const e of errors) console.error(`error: ${e}`)
console.log(`i18n: ${used.size} keys used, ${dict.size} in the English dictionary, ${errors.length} errors, ${warnings.length} warnings`)
process.exit(errors.length ? 1 : 0)
