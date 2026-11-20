import { useCallback, useEffect, useMemo, useState } from 'react'
import { AgGridReact } from 'ag-grid-react'
import type { ColDef, SelectionChangedEvent } from 'ag-grid-community'
import { api, errorText, loadSaveHistory, storeSaveHistory, type HistoryEntry } from '../lib/api'
import { gridTheme, Icon } from '../components/ui'
import TrendChart, { type Point } from '../components/TrendChart'
import { fmtDate } from '../components/SheetBrowser'

interface Props {
  notify: (m: string) => void
  onOpen: (id: string) => void
  onCompare: (beforeId: string, afterId: string) => void
}

const n0 = (v: number) => Math.round(v).toLocaleString('es-ES')

export default function HistoryPage({ notify, onOpen, onCompare }: Props) {
  const [entries, setEntries] = useState<HistoryEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [dir, setDir] = useState('')
  const [selected, setSelected] = useState<HistoryEntry[]>([])
  const [autoSave, setAutoSave] = useState(loadSaveHistory)
  const [confirmDelete, setConfirmDelete] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try { setEntries(await api.listHistory()) } catch (e) { notify(errorText(e)) } finally { setLoading(false) }
  }, [notify])
  useEffect(() => { load(); api.historyDir().then(setDir).catch(() => { /* informative only */ }) }, [load])

  // trends: completed scans only, oldest first
  const series = useMemo(() => {
    const ok = entries.filter(e => !e.cancelled).slice().reverse()
    const pts = (f: (e: HistoryEntry) => number): Point[] => ok.map(e => ({ t: new Date(e.startedAt), v: f(e) }))
    const cur = ok.find(e => e.currency)?.currency ?? 'USD'
    const hasReal = ok.some(e => e.costReal)
    return {
      cur, hasReal,
      vms: pts(e => e.vms), vcpu: pts(e => e.vcpu), cost: pts(e => (hasReal ? e.costReal : e.costList)), high: pts(e => e.high),
    }
  }, [entries])

  const older = (e: HistoryEntry) => {
    const i = entries.findIndex(x => x.id === e.id)
    return entries.slice(i + 1).find(x => !x.cancelled)
  }

  const remove = async () => {
    try {
      for (const e of selected) await api.deleteHistory(e.id)
      notify(selected.length === 1 ? 'Escaneo eliminado del historial' : `${selected.length} escaneos eliminados del historial`)
      setConfirmDelete(false); setSelected([])
      await load()
    } catch (e) { notify(errorText(e)) }
  }

  const columnDefs = useMemo<ColDef<HistoryEntry>[]>(() => [
    { field: 'startedAt', headerName: 'Fecha', width: 160, valueFormatter: p => fmtDate(p.value), sort: 'desc' },
    { field: 'profile', headerName: 'Perfil', width: 140 },
    { headerName: 'Proyectos', width: 110, type: 'rightAligned', valueGetter: p => p.data!.projectsOk === p.data!.projects ? p.data!.projects : `${p.data!.projectsOk}/${p.data!.projects}` },
    { field: 'vms', headerName: 'VMs', width: 90, type: 'rightAligned' },
    { field: 'running', headerName: 'Encendidas', width: 115, type: 'rightAligned' },
    { field: 'vcpu', headerName: 'vCPU', width: 90, type: 'rightAligned' },
    { field: 'orphans', headerName: 'Discos huérfanos', width: 140, type: 'rightAligned' },
    { field: 'costList', headerName: 'Coste lista/mes', width: 140, type: 'rightAligned', valueFormatter: p => p.value ? `${n0(p.value)} ${p.data!.currency}` : '' },
    { field: 'costReal', headerName: 'Coste real/mes', width: 135, type: 'rightAligned', valueFormatter: p => p.value ? `${n0(p.value)} ${p.data!.currency}` : '' },
    { field: 'high', headerName: 'Altas', width: 85, type: 'rightAligned' },
    { field: 'medium', headerName: 'Medias', width: 90, type: 'rightAligned' },
    { field: 'low', headerName: 'Bajas', width: 85, type: 'rightAligned' },
    { field: 'version', headerName: 'Versión', width: 95 },
    { headerName: 'Estado', width: 110, valueGetter: p => p.data!.cancelled ? 'Parcial' : 'Completo' },
  ], [])

  const one = selected.length === 1 ? selected[0] : null
  const two = selected.length === 2 ? [...selected].sort((a, b) => a.startedAt.localeCompare(b.startedAt)) : null

  return (
    <div className="page fill">
      <div className="page-head">
        <div className="grow">
          <h1 className="headline-s">Historial</h1>
          <p className="body-m muted" style={{ margin: '4px 0 0' }}>
            {entries.length} escaneos guardados{dir && <> en <span className="mono selectable">{dir}</span></>} · se conservan los 100 más recientes
          </p>
        </div>
        <label className="row body-m" style={{ gap: 8, cursor: 'pointer' }}>
          <input type="checkbox" checked={autoSave} onChange={e => { setAutoSave(e.target.checked); storeSaveHistory(e.target.checked) }}
            style={{ width: 18, height: 18, accentColor: 'var(--md-primary)' }} />
          Guardar cada escaneo automáticamente
        </label>
      </div>

      {series.vms.length >= 2 && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: 12, flexShrink: 0 }}>
          <TrendChart title="VMs" points={series.vms} format={n0} />
          <TrendChart title="vCPU" points={series.vcpu} format={n0} />
          <TrendChart title={`${series.hasReal ? 'Coste real' : 'Coste lista'}/mes (${series.cur})`} points={series.cost} format={n0} />
          <TrendChart title="Hallazgos de severidad alta" points={series.high} format={n0} />
        </div>
      )}

      <div className="toolbar">
        <span className="body-m muted">
          {selected.length === 0 ? 'Selecciona un escaneo para abrirlo o compararlo con el anterior, o dos para compararlos entre sí.'
            : `${selected.length} seleccionado${selected.length > 1 ? 's' : ''}`}
        </span>
        <span className="spacer" />
        {one && <>
          <button className="btn outlined" onClick={() => onOpen(one.id)}><Icon name="table_view" />Abrir en Inventario</button>
          {older(one) && <button className="btn outlined" onClick={() => onCompare(older(one)!.id, one.id)}><Icon name="compare_arrows" />Comparar con el anterior</button>}
        </>}
        {two && <button className="btn filled" onClick={() => onCompare(two[0].id, two[1].id)}><Icon name="compare_arrows" />Comparar</button>}
        {selected.length > 0 && (confirmDelete
          ? <><span className="body-m">¿Eliminar {selected.length === 1 ? 'este escaneo' : `${selected.length} escaneos`}?</span>
              <button className="btn text" onClick={remove}>Eliminar</button>
              <button className="btn text" onClick={() => setConfirmDelete(false)}>Cancelar</button></>
          : <button className="icon-btn" onClick={() => setConfirmDelete(true)} aria-label="Eliminar del historial" title="Eliminar del historial"><Icon name="delete" /></button>)}
      </div>

      <div className="grid-wrap">
        {!loading && entries.length === 0 ? (
          <div className="empty"><Icon name="history" /><span className="title-m">Aún no hay escaneos en el historial</span>
            <span className="body-m">{autoSave ? 'Cada escaneo que hagas se guardará aquí automáticamente.' : 'Activa "Guardar cada escaneo automáticamente" para empezar a acumular historial.'}</span></div>
        ) : (
          <AgGridReact<HistoryEntry>
            theme={gridTheme}
            rowData={entries}
            columnDefs={columnDefs}
            defaultColDef={{ sortable: true, resizable: true }}
            getRowId={p => p.data.id}
            loading={loading}
            rowSelection={{ mode: 'multiRow', checkboxes: true, headerCheckbox: false }}
            onSelectionChanged={(e: SelectionChangedEvent<HistoryEntry>) => { setSelected(e.api.getSelectedRows()); setConfirmDelete(false) }}
            onRowDoubleClicked={e => e.data && onOpen(e.data.id)}
          />
        )}
      </div>
    </div>
  )
}
