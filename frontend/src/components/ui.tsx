import { useEffect, useRef, useState, type ReactNode } from 'react'
import { themeQuartz } from 'ag-grid-community'

export function Icon({ name, filled, size, className = '' }: { name: string; filled?: boolean; size?: 'sm'; className?: string }) {
  return <span className={`icon ${filled ? 'filled' : ''} ${size ?? ''} ${className}`} aria-hidden="true">{name}</span>
}

export function Snackbar({ message, onClose }: { message: string | null; onClose: () => void }) {
  useEffect(() => {
    if (!message) return
    const t = setTimeout(onClose, 6000)
    return () => clearTimeout(t)
  }, [message, onClose])
  if (!message) return null
  return (
    <div className="snackbar" role="status">
      <span className="body-m">{message}</span>
      <button className="icon-btn" style={{ color: 'inherit' }} onClick={onClose} aria-label="Cerrar"><Icon name="close" size="sm" /></button>
    </div>
  )
}

export function Menu({ trigger, children }: { trigger: (open: () => void) => ReactNode; children: (close: () => void) => ReactNode }) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const h = (e: MouseEvent) => { if (!ref.current?.contains(e.target as Node)) setOpen(false) }
    const k = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false) }
    document.addEventListener('mousedown', h)
    document.addEventListener('keydown', k)
    return () => { document.removeEventListener('mousedown', h); document.removeEventListener('keydown', k) }
  }, [open])
  return (
    <div className="menu-anchor" ref={ref}>
      {trigger(() => setOpen(o => !o))}
      {open && <div className="menu" role="menu">{children(() => setOpen(false))}</div>}
    </div>
  )
}

/** AG Grid theme driven by the M3 CSS variables, so it follows light/dark automatically. */
export const gridTheme = themeQuartz.withParams({
  fontFamily: 'var(--md-font)',
  fontSize: 13,
  headerFontSize: 13,
  headerFontWeight: 500,
  accentColor: 'var(--md-primary)',
  backgroundColor: 'var(--md-surface-container-lowest)',
  foregroundColor: 'var(--md-on-surface)',
  textColor: 'var(--md-on-surface)',
  headerBackgroundColor: 'var(--md-surface-container)',
  headerTextColor: 'var(--md-on-surface-variant)',
  borderColor: 'var(--md-outline-variant)',
  rowHoverColor: 'color-mix(in srgb, var(--md-on-surface) 6%, transparent)',
  selectedRowBackgroundColor: 'color-mix(in srgb, var(--md-primary) 12%, transparent)',
  oddRowBackgroundColor: 'var(--md-surface-container-lowest)',
  wrapperBorder: false,
  wrapperBorderRadius: 0,
  rowHeight: 36,
  headerHeight: 40,
  spacing: 6,
  browserColorScheme: 'inherit',
})
