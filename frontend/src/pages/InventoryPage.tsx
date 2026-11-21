import { useMemo, useState } from 'react'
import { api, errorText, type ExportFormat, type Inventory } from '../lib/api'
import { Icon, Menu } from '../components/ui'
import SheetBrowser, { fmtDate } from '../components/SheetBrowser'
import { num, sevKey, t } from '../lib/i18n'

interface Props { inventory: Inventory; notify: (m: string) => void }

const primarySheets = ['gInfo', 'gHealth', 'gCost', 'gCostReal', 'gRecommend', 'gPerf', 'gTools', 'gDisks', 'gDisk', 'gNetwork', 'gSnapshot', 'gImage', 'gCPU', 'gMemory', 'gLicense', 'gMIG', 'gTemplate', 'gReservation', 'gCommitment', 'gSoleTenant', 'gQuota', 'gVPC', 'gSubnet', 'gFirewall', 'gAddress', 'gProject', 'gErrors', 'gMetaData']

// VMs created by a Google service; MIG members are the user's own and stay visible.
const serviceManaged = new Set(['GKE', 'Dataproc', 'Dataflow', 'Batch'])
// Column holding the VM name in each per-VM sheet.
const vmColumn: Record<string, string> = { gInfo: 'name', gCPU: 'name', gMemory: 'name', gDisk: 'vm', gNetwork: 'vm', gPerf: 'vm', gTools: 'vm', gLicense: 'vm', gHealth: 'resource' }
const HIDE_KEY = 'gcetools.hideManaged'

export default function InventoryPage({ inventory, notify }: Props) {
  const [hideManaged, setHideManaged] = useState(() => { try { return localStorage.getItem(HIDE_KEY) === '1' } catch { return false } })
  const toggleManaged = (v: boolean) => { setHideManaged(v); try { localStorage.setItem(HIDE_KEY, v ? '1' : '0') } catch { /* ignore */ } }
  const managed = useMemo(() => {
    const vms = inventory.sheets.find(s => s.name === 'gInfo')?.rows ?? []
    return new Set(vms.filter(r => serviceManaged.has(String(r.managedBy ?? ''))).map(r => `${r.project}/${r.name}`))
  }, [inventory])
  const sheets = useMemo(() => {
    const order = (n: string) => { const i = primarySheets.indexOf(n); return i < 0 ? 99 : i }
    const sorted = [...inventory.sheets].sort((a, b) => order(a.name) - order(b.name))
    if (!hideManaged || managed.size === 0) return sorted
    return sorted.map(s => {
      const col = vmColumn[s.name]
      return col ? { ...s, rows: s.rows.filter(r => !managed.has(`${r.project}/${r[col]}`)) } : s
    })
  }, [inventory, hideManaged, managed])
  const [exporting, setExporting] = useState(false)

  const stats = useMemo(() => {
    const rows = (n: string) => inventory.sheets.find(s => s.name === n)?.rows ?? []
    const vms = rows('gInfo')
    const sum = (k: string) => vms.reduce((a, r) => a + (Number(r[k]) || 0), 0)
    return [
      { k: 'VMs', v: vms.length },
      { k: t('Encendidas'), v: vms.filter(r => r.status === 'RUNNING').length },
      { k: 'vCPU', v: sum('vcpu') },
      { k: 'RAM (GB)', v: Math.round(sum('memoryGB')) },
      { k: t('Disco total (GB)'), v: Math.round(rows('gDisks').reduce((a, r) => a + (Number(r.sizeGB) || 0), 0)) },
      { k: t('Discos huérfanos'), v: rows('gDisks').filter(r => r.orphan === true).length, alert: true },
      { k: t('Hallazgos altos'), v: rows('gHealth').filter(r => sevKey(r.severity) === 'Alta').length, alert: true },
      ...(inventory.meta.currency ? [{
        k: t('Coste/mes lista ({0})', inventory.meta.currency),
        v: Math.round(rows('gCost').filter(r => r.project !== 'TOTAL').reduce((a, r) => a + (Number(r.total) || 0), 0)),
      }] : []),
      ...(rows('gCostReal').length ? [{
        k: t('Coste real/mes {0} ({1})', String(rows('gCostReal')[0]?.month ?? ''), String(rows('gCostReal')[0]?.currency ?? '')),
        v: Math.round(Number(rows('gCostReal').find(r => r.project === 'TOTAL')?.net) || 0),
      }] : []),
      ...(rows('gRecommend').length ? [{
        k: t('Ahorro estimado/mes ({0})', String(rows('gRecommend').find(r => r.currency)?.currency ?? 'USD')),
        v: Math.round(rows('gRecommend').reduce((a, r) => a + Math.max(0, Number(r.savingsMonthly) || 0), 0)),
      }] : []),
    ]
  }, [inventory])

  const doExport = async (f: ExportFormat, sep = ',') => {
    setExporting(true)
    try {
      const path = await api.exportInventory(f, sep)
      if (path) notify(t('Exportado a {0}', path))
    } catch (e) { notify(errorText(e)) } finally { setExporting(false) }
  }

  const doReport = async () => {
    setExporting(true)
    try {
      const path = await api.exportReport()
      if (path) notify(t('Informe guardado en {0}', path))
    } catch (e) { notify(errorText(e)) } finally { setExporting(false) }
  }

  const m = inventory.meta
  const duration = m.startedAt && m.finishedAt ? Math.round((new Date(m.finishedAt).getTime() - new Date(m.startedAt).getTime()) / 1000) : 0

  return (
    <div className="page fill">
      <div className="page-head">
        <div className="grow">
          <h1 className="headline-s">{t('Inventario')}</h1>
          <p className="body-m muted" style={{ margin: '4px 0 0' }}>
            {fmtDate(m.startedAt)} · {t('{0} proyectos ({1} OK)', m.projects, m.projectsOk)} · {duration}s · <span className="selectable">{m.identity}</span>
            {m.cancelled && <> · <span className="badge sev-Media">{t('Parcial')}</span></>}
          </p>
        </div>
        <Menu trigger={open => (
          <button className="btn filled" onClick={open} disabled={exporting}><Icon name="download" />{exporting ? t('Exportando…') : t('Exportar')}</button>
        )}>
          {close => <>
            <button onClick={() => { close(); doExport('xlsx') }}><Icon name="table" /><div><div>Excel (.xlsx)</div><div className="sub">{t('Una hoja por pestaña, con filtros')}</div></div></button>
            <button onClick={() => { close(); doExport('csv', ',') }}><Icon name="csv" /><div><div>{t('CSV (coma)')}</div><div className="sub">{t('ZIP con un CSV por pestaña')}</div></div></button>
            <button onClick={() => { close(); doExport('csv', ';') }}><Icon name="csv" /><div><div>{t('CSV (punto y coma)')}</div><div className="sub">{t('Para Excel con configuración regional española')}</div></div></button>
            <hr />
            <button onClick={() => { close(); doReport() }}><Icon name="summarize" /><div><div>{t('Informe ejecutivo (HTML)')}</div><div className="sub">{t('Resumen de una página, imprimible a PDF')}</div></div></button>
            <button onClick={() => { close(); doExport('json') }}><Icon name="data_object" /><div><div>Snapshot JSON</div><div className="sub">{t('Para comparar escaneos')}</div></div></button>
          </>}
        </Menu>
      </div>

      <div className="stats">
        {stats.map(s => (
          <div key={s.k} className={`stat ${s.alert && s.v > 0 ? 'alert' : ''}`}>
            <div className="v">{num(s.v)}</div>
            <div className="k">{s.k}</div>
          </div>
        ))}
      </div>

      {managed.size > 0 && (
        <label className="row body-m" style={{ gap: 10, cursor: 'pointer', alignSelf: 'flex-start' }}>
          <input type="checkbox" checked={hideManaged} onChange={e => toggleManaged(e.target.checked)}
            style={{ width: 18, height: 18, accentColor: 'var(--md-primary)' }} />
          {t('Ocultar las {0} VMs gestionadas por GKE, Dataproc, Dataflow o Batch', num(managed.size))}
        </label>
      )}

      <SheetBrowser key={m.startedAt} sheets={sheets} />
    </div>
  )
}
