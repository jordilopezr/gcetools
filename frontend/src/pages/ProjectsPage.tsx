import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AgGridReact } from 'ag-grid-react'
import type { ColDef, GridApi, SelectionChangedEvent } from 'ag-grid-community'
import { api, errorText, type Modules, type Project } from '../lib/api'
import { gridTheme, Icon } from '../components/ui'
import type { ScanState } from '../App'

interface Props {
  scan: ScanState
  onScanStart: () => void
  notify: (m: string) => void
}

const stepLabel: Record<string, string> = {
  disks: 'discos', instances: 'instancias', snapshots: 'snapshots', migs: 'MIGs', templates: 'templates',
  networks: 'VPCs', subnets: 'subredes', firewalls: 'firewall', addresses: 'IPs', done: 'completado',
  images: 'imágenes', reservations: 'reservas', commitments: 'commitments', soletenant: 'nodos sole-tenant',
  quotas: 'cuotas', osconfig: 'inventario de SO', monitoring: 'métricas', recommender: 'recomendaciones',
}

export default function ProjectsPage({ scan, onScanStart, notify }: Props) {
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState('')
  const [manual, setManual] = useState('')
  const [selected, setSelected] = useState<string[]>([])
  const [concurrency, setConcurrency] = useState(8)
  const [modules, setModules] = useState<Modules>({ osconfig: true, monitoring: true, recommender: true, metricsDays: 30, pricing: true, currency: 'USD' })
  const toggle = (k: 'osconfig' | 'monitoring' | 'recommender' | 'pricing') => setModules(m => ({ ...m, [k]: !m[k] }))
  const gridApi = useRef<GridApi<Project> | null>(null)

  const load = useCallback(async (q: string) => {
    setLoading(true); setError(null)
    try { setProjects(await api.listProjects(q)) } catch (e) { setError(errorText(e)) } finally { setLoading(false) }
  }, [])
  useEffect(() => { load('') }, [load])

  const addManual = () => {
    const ids = manual.split(/[\s,;]+/).map(s => s.trim()).filter(Boolean)
    const known = new Set(projects.map(p => p.id))
    const extra = ids.filter(id => !known.has(id)).map(id => ({ id, name: '(añadido manualmente)', number: '', parent: '', state: '', created: '', labels: null }))
    setProjects(p => [...extra, ...p]); setManual('')
    setTimeout(() => gridApi.current?.forEachNode(n => { if (ids.includes(n.data!.id)) n.setSelected(true) }))
  }

  const start = async () => {
    try {
      onScanStart()
      await api.startScan(selected, { concurrency, health: { stoppedDays: 30, snapshotDays: 90 }, modules })
    } catch (e) { notify(errorText(e)) }
  }

  const columnDefs = useMemo<ColDef<Project>[]>(() => [
    { field: 'id', headerName: 'Project ID', flex: 2, minWidth: 200 },
    { field: 'name', headerName: 'Nombre', flex: 2, minWidth: 180 },
    { field: 'number', headerName: 'Número', width: 150 },
    { field: 'parent', headerName: 'Padre', width: 170 },
    { headerName: 'Labels', flex: 2, minWidth: 160, valueGetter: p => Object.entries(p.data?.labels ?? {}).map(([k, v]) => `${k}=${v}`).join('; ') },
  ], [])

  const pct = scan.progress && scan.progress.total ? Math.round((scan.progress.done / scan.progress.total) * 100) : 0

  return (
    <div className="page fill">
      <div className="page-head">
        <div className="grow">
          <h1 className="headline-s">Proyectos</h1>
          <p className="body-m muted" style={{ margin: '4px 0 0' }}>
            {loading ? 'Buscando proyectos…' : `${projects.length} proyectos visibles · ${selected.length} seleccionados`}
          </p>
        </div>
        <div className="field with-icon" style={{ width: 340 }}>
          <span className="lead"><Icon name="filter_alt" /></span>
          <input id="q" value={query} placeholder=" " spellCheck={false} onChange={e => setQuery(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') load(query) }} />
          <label htmlFor="q">Filtro Resource Manager (parent:folders/123)</label>
        </div>
        <button className="btn outlined" onClick={() => load(query)} disabled={loading}><Icon name="refresh" />Buscar</button>
      </div>

      {error && <div className="banner error"><Icon name="error" /><div className="selectable">{error}<br />
        <span className="body-s">Si no tienes permiso para listar proyectos, añádelos manualmente por ID.</span></div></div>}

      {(scan.running || scan.log.length > 0) && (
        <div className="card elevated col">
          <div className="row">
            <Icon name={scan.running ? 'radar' : 'task_alt'} />
            <span className="title-m">{scan.running ? 'Escaneando…' : 'Último escaneo'}</span>
            <span className="spacer" />
            {scan.progress && <span className="body-m muted">
              {scan.progress.done}/{scan.progress.total} proyectos · {scan.progress.vms} VMs · {scan.progress.errors} errores
            </span>}
            {scan.running && <button className="btn text" onClick={() => api.cancelScan()}><Icon name="stop_circle" />Cancelar</button>}
          </div>
          <div className={`progress ${scan.running && !scan.progress ? 'indeterminate' : ''}`}><div style={{ width: `${pct}%` }} /></div>
          {scan.running && scan.progress && scan.progress.step !== 'done' && (
            <span className="body-s muted">Leyendo {stepLabel[scan.progress.step] ?? scan.progress.step} en <span className="mono">{scan.progress.project}</span></span>
          )}
          {scan.log.length > 0 && (
            <div className="log">
              {scan.log.slice(0, 50).map((l, i) => (
                <div className="log-row" key={i}>
                  <Icon size="sm" name={l.status === 'OK' ? 'check_circle' : 'warning'} className={l.status === 'OK' ? '' : 'muted'} />
                  <span className="mono">{l.project}</span>
                  <span className={`badge ${l.status === 'OK' ? 'ok' : 'sev-Media'}`}>{l.status}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      <div className="row" style={{ gap: 8 }}>
        <span className="label-l muted" style={{ marginRight: 4 }}>Módulos opcionales</span>
        {([
          ['osconfig', 'memory', 'SO y agentes (OS Config)'],
          ['monitoring', 'monitoring', 'Métricas CPU/RAM'],
          ['recommender', 'lightbulb', 'Recomendaciones'],
          ['pricing', 'payments', 'Coste estimado'],
        ] as const).map(([k, icon, label]) => (
          <button key={k} className={`chip ${modules[k] ? 'selected' : ''}`} onClick={() => toggle(k)} aria-pressed={modules[k]}>
            <Icon name={modules[k] ? 'check' : icon} size="sm" />{label}
          </button>
        ))}
        {modules.monitoring && (
          <select value={modules.metricsDays} onChange={e => setModules(m => ({ ...m, metricsDays: Number(e.target.value) }))}
            aria-label="Días de métricas"
            style={{ height: 32, borderRadius: 8, border: '1px solid var(--md-outline-variant)', background: 'transparent', color: 'inherit', padding: '0 8px' }}>
            {[7, 14, 30, 42].map(n => <option key={n} value={n}>{n} días</option>)}
          </select>
        )}
        {modules.pricing && (
          <select value={modules.currency} onChange={e => setModules(m => ({ ...m, currency: e.target.value }))}
            aria-label="Moneda"
            style={{ height: 32, borderRadius: 8, border: '1px solid var(--md-outline-variant)', background: 'transparent', color: 'inherit', padding: '0 8px' }}>
            {['USD', 'EUR', 'CLP', 'MXN', 'COP', 'BRL', 'GBP'].map(c => <option key={c} value={c}>{c}</option>)}
          </select>
        )}
        <span className="body-s muted">Si la API no está habilitada en un proyecto, el módulo se omite sin marcarlo como error.</span>
      </div>

      <div className="toolbar">
        <div className="search">
          <Icon name="search" />
          <input value={filter} onChange={e => setFilter(e.target.value)} placeholder="Filtrar la lista" aria-label="Filtrar proyectos" />
        </div>
        <div className="search" style={{ minWidth: 320 }}>
          <Icon name="add" />
          <input value={manual} onChange={e => setManual(e.target.value)} placeholder="Añadir IDs manualmente (separados por comas)"
            onKeyDown={e => { if (e.key === 'Enter' && manual.trim()) addManual() }} aria-label="Añadir proyectos por ID" />
        </div>
        <span className="spacer" />
        <label className="row body-m" style={{ gap: 8 }}>
          Paralelismo
          <select value={concurrency} onChange={e => setConcurrency(Number(e.target.value))}
            style={{ height: 36, borderRadius: 8, border: '1px solid var(--md-outline)', background: 'transparent', color: 'inherit', padding: '0 8px' }}>
            {[2, 4, 8, 16, 32].map(n => <option key={n} value={n}>{n}</option>)}
          </select>
        </label>
        <button className="btn filled" disabled={selected.length === 0 || scan.running} onClick={start}>
          <Icon name="play_arrow" />Escanear {selected.length > 0 ? `(${selected.length})` : ''}
        </button>
      </div>

      <div className="grid-wrap">
        <AgGridReact<Project>
          theme={gridTheme}
          rowData={projects}
          columnDefs={columnDefs}
          defaultColDef={{ sortable: true, filter: true, resizable: true }}
          quickFilterText={filter}
          getRowId={p => p.data.id}
          loading={loading}
          rowSelection={{ mode: 'multiRow', checkboxes: true, headerCheckbox: true, selectAll: 'filtered' }}
          onGridReady={e => { gridApi.current = e.api }}
          onSelectionChanged={(e: SelectionChangedEvent<Project>) => setSelected(e.api.getSelectedRows().map(r => r.id))}
          overlayNoRowsTemplate="No hay proyectos visibles con esta identidad"
        />
      </div>
    </div>
  )
}
