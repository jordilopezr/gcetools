// Tabs + search + AG Grid for any list of sheets. Used by the inventory and
// by the comparison page (whose result is also a list of sheets).
import { useMemo, useState } from 'react'
import { AgGridReact } from 'ag-grid-react'
import type { ColDef, ICellRendererParams } from 'ag-grid-community'
import type { Column, Row, Sheet } from '../lib/api'
import { gridTheme, Icon } from './ui'

const pad = (n: number) => String(n).padStart(2, '0')
export function fmtDate(v: unknown): string {
  if (!v) return ''
  const d = new Date(String(v))
  if (isNaN(d.getTime())) return String(v)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const statusClass: Record<string, string> = { RUNNING: 'ok', OK: 'ok', TERMINATED: 'sev-Baja', STOPPING: 'sev-Media', SUSPENDED: 'sev-Media' }
function BadgeCell(p: ICellRendererParams<Row>) {
  const v = String(p.value ?? '')
  if (!v) return null
  const cls = p.colDef?.field === 'severity' ? `sev-${v}` : statusClass[v] ?? 'sev-Media'
  return <span className={`badge ${cls}`}>{v}</span>
}

function RelevantCell(p: ICellRendererParams<Row>) {
  if (p.data?.relevant !== true) return null
  return <span className="badge sev-Alta" title={String(p.data?.reason ?? '')}><Icon name="priority_high" size="sm" />Sí</span>
}

function toColDef(c: Column): ColDef<Row> {
  const base: ColDef<Row> = { field: c.key, headerName: c.title, minWidth: 90, width: Math.max(110, Math.min(320, (c.width ?? 16) * 8)) }
  if (c.key === 'relevant' && c.type === 'bool') return { ...base, width: 120, cellRenderer: RelevantCell, valueGetter: p => p.data?.relevant === true ? 'Sí' : 'No' }
  switch (c.type) {
    case 'int':
    case 'float':
      return { ...base, type: 'rightAligned', filter: 'agNumberColumnFilter' }
    case 'gb':
      return { ...base, type: 'rightAligned', filter: 'agNumberColumnFilter', valueFormatter: p => p.value == null ? '' : Number(p.value).toLocaleString('es-ES', { maximumFractionDigits: 2 }) }
    case 'money':
      return { ...base, type: 'rightAligned', filter: 'agNumberColumnFilter', valueFormatter: p => p.value == null ? '' : Number(p.value).toLocaleString('es-ES', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) }
    case 'bool':
      return { ...base, width: 110, valueGetter: p => { const v = p.data?.[c.key]; return v === true ? 'Sí' : v === false ? 'No' : '' } }
    case 'datetime':
      return { ...base, width: 150, valueGetter: p => fmtDate(p.data?.[c.key]) }
  }
  if (['status', 'scanStatus', 'severity'].includes(c.key)) return { ...base, cellRenderer: BadgeCell }
  return base
}

interface Props {
  sheets: Sheet[]
  emptyText?: string
}

export default function SheetBrowser({ sheets, emptyText = 'No se encontraron recursos de este tipo en los proyectos escaneados.' }: Props) {
  const [active, setActive] = useState(sheets[0]?.name ?? '')
  const [filter, setFilter] = useState('')
  const sheet = sheets.find(s => s.name === active) ?? sheets[0]
  const columnDefs = useMemo(() => sheet?.columns.map(toColDef) ?? [], [sheet])

  return <>
    <div className="sheet-tabs" role="tablist" aria-label="Pestañas">
      {sheets.map(s => (
        <button key={s.name} role="tab" aria-selected={s.name === sheet?.name} title={s.description}
          className={`chip ${s.name === sheet?.name ? 'selected' : ''}`} onClick={() => setActive(s.name)}>
          {s.name === sheet?.name && <Icon name="check" size="sm" />}
          {s.name}<span className="count">{s.rows.length}</span>
        </button>
      ))}
    </div>

    <div className="toolbar">
      <div className="col" style={{ gap: 0 }}>
        <span className="title-m">{sheet?.title}</span>
        <span className="body-s muted">{sheet?.description}</span>
      </div>
      <span className="spacer" />
      <div className="search">
        <Icon name="search" />
        <input value={filter} onChange={e => setFilter(e.target.value)} placeholder={`Buscar en ${sheet?.name ?? ''}`} aria-label="Buscar en la pestaña" />
        {filter && <button className="icon-btn" style={{ width: 28, height: 28 }} onClick={() => setFilter('')} aria-label="Limpiar"><Icon name="close" size="sm" /></button>}
      </div>
    </div>

    <div className="grid-wrap">
      {sheet && sheet.rows.length === 0 && !filter ? (
        <div className="empty"><Icon name="inbox" /><span className="title-m">Sin datos en {sheet.name}</span>
          <span className="body-m">{emptyText}</span></div>
      ) : (
        <AgGridReact<Row>
          key={sheet?.name}
          theme={gridTheme}
          rowData={sheet?.rows ?? []}
          columnDefs={columnDefs}
          defaultColDef={{ sortable: true, filter: true, resizable: true }}
          quickFilterText={filter}
          enableCellTextSelection
          ensureDomOrder
          tooltipShowDelay={400}
          pagination={false}
        />
      )}
    </div>
  </>
}
