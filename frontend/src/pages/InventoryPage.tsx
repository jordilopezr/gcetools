import { useMemo, useState } from 'react'
import { api, errorText, type ExportFormat, type Inventory } from '../lib/api'
import { Icon, Menu } from '../components/ui'
import SheetBrowser, { fmtDate } from '../components/SheetBrowser'

interface Props { inventory: Inventory; notify: (m: string) => void }

const primarySheets = ['gInfo', 'gHealth', 'gCost', 'gCostReal', 'gRecommend', 'gPerf', 'gTools', 'gDisks', 'gDisk', 'gNetwork', 'gSnapshot', 'gImage', 'gCPU', 'gMemory', 'gLicense', 'gMIG', 'gTemplate', 'gReservation', 'gCommitment', 'gSoleTenant', 'gQuota', 'gVPC', 'gSubnet', 'gFirewall', 'gAddress', 'gProject', 'gErrors', 'gMetaData']

export default function InventoryPage({ inventory, notify }: Props) {
  const sheets = useMemo(() => {
    const order = (n: string) => { const i = primarySheets.indexOf(n); return i < 0 ? 99 : i }
    return [...inventory.sheets].sort((a, b) => order(a.name) - order(b.name))
  }, [inventory])
  const [exporting, setExporting] = useState(false)

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
      ...(rows('gCostReal').length ? [{
        k: `Coste real/mes ${String(rows('gCostReal')[0]?.month ?? '')} (${String(rows('gCostReal')[0]?.currency ?? '')})`,
        v: Math.round(Number(rows('gCostReal').find(r => r.project === 'TOTAL')?.net) || 0),
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

  const doReport = async () => {
    setExporting(true)
    try {
      const path = await api.exportReport()
      if (path) notify(`Informe guardado en ${path}`)
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
            <button onClick={() => { close(); doReport() }}><Icon name="summarize" /><div><div>Informe ejecutivo (HTML)</div><div className="sub">Resumen de una página, imprimible a PDF</div></div></button>
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

      <SheetBrowser key={m.startedAt} sheets={sheets} />
    </div>
  )
}
