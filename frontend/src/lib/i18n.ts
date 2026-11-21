// UI translations. Spanish is the source language: the Spanish text is the
// key (t('Proyectos')), so components stay readable and a missing English
// entry falls back to Spanish. Placeholders are positional: t('{0} VMs', n).
//
// The language lives here (module state) and in App (React state): App calls
// setLang and remounts the pages, so every t() call re-runs. The backend is
// told too, so its messages, new scans and exports match the UI.
import { en } from './i18n.en'

export type Lang = 'es' | 'en'
const KEY = 'gcetools.lang'

function detect(): Lang {
  try {
    const saved = localStorage.getItem(KEY)
    if (saved === 'es' || saved === 'en') return saved
  } catch { /* storage unavailable */ }
  const nav = typeof navigator !== 'undefined' ? navigator.language : ''
  return nav.toLowerCase().startsWith('es') ? 'es' : 'en'
}

let current: Lang = detect()

export function getLang(): Lang { return current }

/** Switches the language and remembers it; App re-renders the pages. */
export function setLang(l: Lang) {
  current = l
  try { localStorage.setItem(KEY, l) } catch { /* storage unavailable */ }
  if (typeof document !== 'undefined') document.documentElement.lang = l
}

/** Translates s and fills {0}, {1}… with args. */
export function t(s: string, ...args: (string | number)[]): string {
  const text = current === 'en' ? (en[s] ?? s) : s
  return args.length ? text.replace(/\{(\d+)\}/g, (m, i) => (i < args.length ? String(args[Number(i)]) : m)) : text
}

/**
 * Translates a Spanish text whose English depends on context (e.g. "Altas"
 * is "Added" in a comparison but "High" as a severity count). The dictionary
 * key is "ctx|s"; Spanish shows s unchanged.
 */
export function tc(ctx: string, s: string, ...args: (string | number)[]): string {
  const key = `${ctx}|${s}`
  return t(current === 'en' && key in en ? key : s, ...args)
}

/** Locale for numbers and dates in the current language. */
export function locale(): string { return current === 'en' ? 'en-US' : 'es-ES' }

/** Formats a number with the current locale. */
export function num(v: number, opts?: Intl.NumberFormatOptions): string { return v.toLocaleString(locale(), opts) }

/**
 * Severity values come from the scan in the language it ran in ("Alta" or
 * "High"); styles and counters use the Spanish value as the key.
 */
export function sevKey(v: unknown): string {
  const s = String(v ?? '')
  return ({ High: 'Alta', Medium: 'Media', Low: 'Baja' } as Record<string, string>)[s] ?? s
}

if (typeof document !== 'undefined') document.documentElement.lang = current
