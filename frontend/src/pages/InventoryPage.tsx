import { useCallback, useMemo, useState } from 'react'
import { api, errorText, type ExportFormat, type InventorySummary } from '../lib/api'
import { Icon, Menu } from '../components/ui'
import SheetBrowser, { fmtDate, type SheetView } from '../components/SheetBrowser'
import { num, t } from '../lib/i18n'

interface Props { inventory: InventorySummary; notify: (m: string) => void }

const HIDE_KEY = 'gcetools.hideManaged'

// The page receives a summary; each tab's rows are fetched when it is opened
// (see SheetBrowser), so a large inventory never crosses the bridge whole.
export default function InventoryPage({ inventory, notify }: Props) {
  const [hideManaged, setHideManaged] = useState(() => { try { return localStorage.getItem(HIDE_KEY) === '1' } catch { return false } })
  const toggleManaged = (v: boolean) => { setHideManaged(v); try { localStorage.setItem(HIDE_KEY, v ? '1' : '0') } catch { /* ignore */ } }
  const hide = hideManaged && inventory.managed > 0
  const sheets = useMemo<SheetView[]>(() => inventory.sheets.map(s => ({
    name: s.name, title: s.title, description: s.description, columns: s.columns, count: hide ? s.rowsUnmanaged : s.rows,
  })), [inventory, hide])
  // A new function (another inventory, or the managed filter toggled) makes
  // SheetBrowser drop the rows it had loaded.
  const loadRows = useCallback((name: string) => api.sheetRows(name, hide), [inventory, hide])
  const [exporting, setExporting] = useState(false)

  const stats = useMemo(() => {
    const st = inventory.stats
    const cur = inventory.meta.currency
    return [
      { k: 'VMs', v: st.vms },
      { k: t('Encendidas'), v: st.running },
      { k: 'vCPU', v: st.vcpu },
      { k: 'RAM (GB)', v: Math.round(st.memoryGB) },
      { k: t('Disco total (GB)'), v: Math.round(st.diskGB) },
      { k: t('Discos huérfanos'), v: st.orphans, alert: true },
      { k: t('Hallazgos altos'), v: st.high, alert: true },
      ...(st.hasSecurity ? [{ k: t('Expuestas a Internet'), v: st.exposed, alert: true }] : []),
      ...(cur ? [{ k: t('Coste/mes lista ({0})', cur), v: Math.round(st.costList) }] : []),
      ...(st.hasCostReal ? [{ k: t('Coste real/mes {0} ({1})', st.costRealMonth, st.costRealCurrency), v: Math.round(st.costReal) }] : []),
      ...(st.hasSavings ? [{ k: t('Ahorro estimado/mes ({0})', st.savingsCurrency), v: Math.round(st.savings) }] : []),
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

      {inventory.managed > 0 && (
        <label className="row body-m" style={{ gap: 10, cursor: 'pointer', alignSelf: 'flex-start' }}>
          <input type="checkbox" checked={hideManaged} onChange={e => toggleManaged(e.target.checked)}
            style={{ width: 18, height: 18, accentColor: 'var(--md-primary)' }} />
          {t('Ocultar las {0} VMs gestionadas por GKE, Dataproc, Dataflow o Batch', num(inventory.managed))}
        </label>
      )}

      <SheetBrowser key={m.startedAt} sheets={sheets} loadRows={loadRows} />
    </div>
  )
}
