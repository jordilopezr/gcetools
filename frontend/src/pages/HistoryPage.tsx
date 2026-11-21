import { useCallback, useEffect, useMemo, useState } from 'react'
import { AgGridReact } from 'ag-grid-react'
import type { ColDef, SelectionChangedEvent } from 'ag-grid-community'
import { api, errorText, loadSaveHistory, storeSaveHistory, type HistoryEntry } from '../lib/api'
import { gridTheme, Icon } from '../components/ui'
import TrendChart, { type Point } from '../components/TrendChart'
import { fmtDate } from '../components/SheetBrowser'
import { locale, t, tc } from '../lib/i18n'

interface Props {
  notify: (m: string) => void
  onOpen: (id: string) => void
  onCompare: (beforeId: string, afterId: string) => void
}

const n0 = (v: number) => Math.round(v).toLocaleString(locale())

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
      notify(selected.length === 1 ? t('Escaneo eliminado del historial') : t('{0} escaneos eliminados del historial', selected.length))
      setConfirmDelete(false); setSelected([])
      await load()
    } catch (e) { notify(errorText(e)) }
  }

  const columnDefs = useMemo<ColDef<HistoryEntry>[]>(() => [
    { field: 'startedAt', headerName: t('Fecha'), width: 160, valueFormatter: p => fmtDate(p.value), sort: 'desc' },
    { field: 'profile', headerName: t('Perfil'), width: 140 },
    { headerName: t('Proyectos'), width: 110, type: 'rightAligned', valueGetter: p => p.data!.projectsOk === p.data!.projects ? p.data!.projects : `${p.data!.projectsOk}/${p.data!.projects}` },
    { field: 'vms', headerName: 'VMs', width: 90, type: 'rightAligned' },
    { field: 'running', headerName: t('Encendidas'), width: 115, type: 'rightAligned' },
    { field: 'vcpu', headerName: 'vCPU', width: 90, type: 'rightAligned' },
    { field: 'orphans', headerName: t('Discos huérfanos'), width: 140, type: 'rightAligned' },
    { field: 'costList', headerName: t('Coste lista/mes'), width: 140, type: 'rightAligned', valueFormatter: p => p.value ? `${n0(p.value)} ${p.data!.currency}` : '' },
    { field: 'costReal', headerName: t('Coste real/mes'), width: 135, type: 'rightAligned', valueFormatter: p => p.value ? `${n0(p.value)} ${p.data!.currency}` : '' },
    { field: 'high', headerName: tc('severity', 'Altas'), width: 85, type: 'rightAligned' },
    { field: 'medium', headerName: tc('severity', 'Medias'), width: 90, type: 'rightAligned' },
    { field: 'low', headerName: tc('severity', 'Bajas'), width: 85, type: 'rightAligned' },
    { field: 'version', headerName: t('Versión'), width: 95 },
    { headerName: t('Estado'), width: 110, valueGetter: p => p.data!.cancelled ? t('Parcial') : t('Completo') },
  ], [])

  const one = selected.length === 1 ? selected[0] : null
  const two = selected.length === 2 ? [...selected].sort((a, b) => a.startedAt.localeCompare(b.startedAt)) : null

  return (
    <div className="page fill">
      <div className="page-head">
        <div className="grow">
          <h1 className="headline-s">{t('Historial')}</h1>
          <p className="body-m muted" style={{ margin: '4px 0 0' }}>
            {t('{0} escaneos guardados', entries.length)}{dir && <> {t('en')} <span className="mono selectable">{dir}</span></>} · {t('se conservan los 100 más recientes')}
          </p>
        </div>
        <label className="row body-m" style={{ gap: 8, cursor: 'pointer' }}>
          <input type="checkbox" checked={autoSave} onChange={e => { setAutoSave(e.target.checked); storeSaveHistory(e.target.checked) }}
            style={{ width: 18, height: 18, accentColor: 'var(--md-primary)' }} />
          {t('Guardar cada escaneo automáticamente')}
        </label>
      </div>

      {series.vms.length >= 2 && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: 12, flexShrink: 0 }}>
          <TrendChart title="VMs" points={series.vms} format={n0} />
          <TrendChart title="vCPU" points={series.vcpu} format={n0} />
          <TrendChart title={series.hasReal ? t('Coste real/mes ({0})', series.cur) : t('Coste lista/mes ({0})', series.cur)} points={series.cost} format={n0} />
          <TrendChart title={t('Hallazgos de severidad alta')} points={series.high} format={n0} />
        </div>
      )}

      <div className="toolbar">
        <span className="body-m muted">
          {selected.length === 0 ? t('Selecciona un escaneo para abrirlo o compararlo con el anterior, o dos para compararlos entre sí.')
            : selected.length > 1 ? t('{0} seleccionados', selected.length) : t('{0} seleccionado', selected.length)}
        </span>
        <span className="spacer" />
        {one && <>
          <button className="btn outlined" onClick={() => onOpen(one.id)}><Icon name="table_view" />{t('Abrir en Inventario')}</button>
          {older(one) && <button className="btn outlined" onClick={() => onCompare(older(one)!.id, one.id)}><Icon name="compare_arrows" />{t('Comparar con el anterior')}</button>}
        </>}
        {two && <button className="btn filled" onClick={() => onCompare(two[0].id, two[1].id)}><Icon name="compare_arrows" />{t('Comparar')}</button>}
        {selected.length > 0 && (confirmDelete
          ? <><span className="body-m">{selected.length === 1 ? t('¿Eliminar este escaneo?') : t('¿Eliminar {0} escaneos?', selected.length)}</span>
              <button className="btn text" onClick={remove}>{t('Eliminar')}</button>
              <button className="btn text" onClick={() => setConfirmDelete(false)}>{t('Cancelar')}</button></>
          : <button className="icon-btn" onClick={() => setConfirmDelete(true)} aria-label={t('Eliminar del historial')} title={t('Eliminar del historial')}><Icon name="delete" /></button>)}
      </div>

      <div className="grid-wrap">
        {!loading && entries.length === 0 ? (
          <div className="empty"><Icon name="history" /><span className="title-m">{t('Aún no hay escaneos en el historial')}</span>
            <span className="body-m">{autoSave ? t('Cada escaneo que hagas se guardará aquí automáticamente.') : t('Activa "Guardar cada escaneo automáticamente" para empezar a acumular historial.')}</span></div>
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
