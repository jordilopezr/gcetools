import { useMemo, useState } from 'react'
import { AgGridReact } from 'ag-grid-react'
import type { ColDef, ICellRendererParams } from 'ag-grid-community'
import { api, errorText, type Column, type ExportFormat, type Inventory, type Row } from '../lib/api'
import { gridTheme, Icon, Menu } from '../components/ui'

interface Props { inventory: Inventory; notify: (m: string) => void }

const pad = (n: number) => String(n).padStart(2, '0')
function fmtDate(v: unknown): string {
  if (!v) return ''
  const d = new Date(String(v))
  if (isNaN(d.getTime())) return ''
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const statusClass: Record<string, string> = { RUNNING: 'ok', OK: 'ok', TERMINATED: 'sev-Baja', STOPPING: 'sev-Media', SUSPENDED: 'sev-Media' }
function BadgeCell(p: ICellRendererParams<Row>) {
  const v = String(p.value ?? '')
  if (!v) return null
  const cls = p.colDef?.field === 'severity' ? `sev-${v}` : statusClass[v] ?? 'sev-Media'
  return <span className={`badge ${cls}`}>{v}</span>
}

function toColDef(c: Column): ColDef<Row> {
  const base: ColDef<Row> = { field: c.key, headerName: c.title, minWidth: 90, width: Math.max(110, Math.min(320, (c.width ?? 16) * 8)) }
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

const primarySheets = ['gInfo', 'gHealth', 'gCost', 'gRecommend', 'gPerf', 'gTools', 'gDisks', 'gDisk', 'gNetwork', 'gSnapshot', 'gImage', 'gCPU', 'gMemory', 'gLicense', 'gMIG', 'gTemplate', 'gReservation', 'gCommitment', 'gSoleTenant', 'gQuota', 'gVPC', 'gSubnet', 'gFirewall', 'gAddress', 'gProject', 'gErrors', 'gMetaData']

export default function InventoryPage({ inventory, notify }: Props) {
  const sheets = useMemo(() => {
    const order = (n: string) => { const i = primarySheets.indexOf(n); return i < 0 ? 99 : i }
    return [...inventory.sheets].sort((a, b) => order(a.name) - order(b.name))
  }, [inventory])
  const [active, setActive] = useState(sheets[0]?.name ?? 'gInfo')
  const [filter, setFilter] = useState('')
  const [exporting, setExporting] = useState(false)
  const sheet = sheets.find(s => s.name === active) ?? sheets[0]
  const columnDefs = useMemo(() => sheet?.columns.map(toColDef) ?? [], [sheet])

  const stats = useMemo(() => {
    const rows = (n: string) => inventory.sheets.find(s => s.name === n)?.rows ?? []
    const vms = rows('gInfo')
    const sum = (k: string) => vms.reduce((a, r) => a + (Number(r[k]) || 0), 0)
    return [
      { k: 'VMs', v: vms.length },
      { k: 'Encendidas', v: vms.filter(r => r.status === 'RUNNING').length },
      { k: 'vCPU', v: sum('vcpu') },
      { k: 'RAM (GB)', v: Math.round(sum('memoryGB')) },
      { k: 'Disco total (GB)', v: Math.round(rows('gDisks').reduce((a, r) => a + (Number(r.sizeGB) || 0), 0)) },
      { k: 'Discos huérfanos', v: rows('gDisks').filter(r => r.orphan === true).length, alert: true },
      { k: 'Hallazgos altos', v: rows('gHealth').filter(r => r.severity === 'Alta').length, alert: true },
      ...(inventory.meta.currency ? [{
        k: `Coste/mes lista (${inventory.meta.currency})`,
        v: Math.round(rows('gCost').filter(r => r.project !== 'TOTAL').reduce((a, r) => a + (Number(r.total) || 0), 0)),
      }] : []),
      ...(rows('gRecommend').length ? [{
        k: `Ahorro estimado/mes (${String(rows('gRecommend').find(r => r.currency)?.currency ?? 'USD')})`,
        v: Math.round(rows('gRecommend').reduce((a, r) => a + Math.max(0, Number(r.savingsMonthly) || 0), 0)),
      }] : []),
    ]
  }, [inventory])

  const doExport = async (f: ExportFormat, sep = ',') => {
    setExporting(true)
    try {
      const path = await api.exportInventory(f, sep)
      if (path) notify(`Exportado a ${path}`)
    } catch (e) { notify(errorText(e)) } finally { setExporting(false) }
  }

  const m = inventory.meta
  const duration = m.startedAt && m.finishedAt ? Math.round((new Date(m.finishedAt).getTime() - new Date(m.startedAt).getTime()) / 1000) : 0

  return (
    <div className="page fill">
      <div className="page-head">
        <div className="grow">
          <h1 className="headline-s">Inventario</h1>
          <p className="body-m muted" style={{ margin: '4px 0 0' }}>
            {fmtDate(m.startedAt)} · {m.projects} proyectos ({m.projectsOk} OK) · {duration}s · <span className="selectable">{m.identity}</span>
            {m.cancelled && <> · <span className="badge sev-Media">Parcial</span></>}
          </p>
        </div>
        <Menu trigger={open => (
          <button className="btn filled" onClick={open} disabled={exporting}><Icon name="download" />{exporting ? 'Exportando…' : 'Exportar'}</button>
        )}>
          {close => <>
            <button onClick={() => { close(); doExport('xlsx') }}><Icon name="table" /><div><div>Excel (.xlsx)</div><div className="sub">Una hoja por pestaña, con filtros</div></div></button>
            <button onClick={() => { close(); doExport('csv', ',') }}><Icon name="csv" /><div><div>CSV (coma)</div><div className="sub">ZIP con un CSV por pestaña</div></div></button>
            <button onClick={() => { close(); doExport('csv', ';') }}><Icon name="csv" /><div><div>CSV (punto y coma)</div><div className="sub">Para Excel con configuración regional española</div></div></button>
            <hr />
            <button onClick={() => { close(); doExport('json') }}><Icon name="data_object" /><div><div>Snapshot JSON</div><div className="sub">Para comparar escaneos</div></div></button>
          </>}
        </Menu>
      </div>

      <div className="stats">
        {stats.map(s => (
          <div key={s.k} className={`stat ${s.alert && s.v > 0 ? 'alert' : ''}`}>
            <div className="v">{s.v.toLocaleString('es-ES')}</div>
            <div className="k">{s.k}</div>
          </div>
        ))}
      </div>

      <div className="sheet-tabs" role="tablist" aria-label="Pestañas del inventario">
        {sheets.map(s => (
          <button key={s.name} role="tab" aria-selected={s.name === active} title={s.description}
            className={`chip ${s.name === active ? 'selected' : ''}`} onClick={() => setActive(s.name)}>
            {s.name === active && <Icon name="check" size="sm" />}
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
            <span className="body-m">No se encontraron recursos de este tipo en los proyectos escaneados.</span></div>
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
    </div>
  )
}
