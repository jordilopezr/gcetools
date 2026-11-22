import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AgGridReact } from 'ag-grid-react'
import type { ColDef, GridApi, SelectionChangedEvent } from 'ag-grid-community'
import { api, errorText, loadSaveHistory, type Footprint, type HealthOptions, type HealthRule, type Modules, type Profile, type Project, type SessionInfo } from '../lib/api'
import { gridTheme, Icon } from '../components/ui'
import type { ScanState } from '../App'
import { locale, sevKey, t } from '../lib/i18n'

interface Props {
  session: SessionInfo
  scan: ScanState
  onScanStart: () => void
  notify: (m: string) => void
}

// built on each call so the labels follow the current language
const stepLabel = (): Record<string, string> => ({
  disks: t('discos'), instances: t('instancias'), snapshots: 'snapshots', migs: 'MIGs', templates: 'templates',
  networks: 'VPCs', subnets: t('subredes'), firewalls: 'firewall', addresses: 'IPs', done: t('completado'),
  images: t('imágenes'), reservations: t('reservas'), commitments: 'commitments', soletenant: t('nodos sole-tenant'),
  quotas: t('cuotas'), osconfig: t('inventario de SO'), monitoring: t('métricas'), recommender: t('recomendaciones'),
})

const defaultHealth: HealthOptions = { stoppedDays: 30, snapshotDays: 90, imageDays: 180, cpuLowPct: 10, cpuHighPct: 90, memHighPct: 90, quotaPct: 80, cudDays: 60, disabled: [] }
const paramUnit = (p: string) => (p.endsWith('Days') ? t('días') : p.endsWith('Pct') ? '%' : '')
type NumericHealth = Exclude<keyof HealthOptions, 'disabled'>
const fpTotal = (f?: Footprint) => (f ? f.vms + f.disks + f.other : 0)
const selectStyle = { height: 36, borderRadius: 8, border: '1px solid var(--md-outline)', background: 'transparent', color: 'inherit', padding: '0 8px' }

export default function ProjectsPage({ session, scan, onScanStart, notify }: Props) {
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
  // These APIs only accept the full cloud-platform scope, which only gcloud
  // (ADC) tokens carry.
  const fullScopeOnly = (k: string) => !session.fullScope && (k === 'osconfig' || k === 'recommender' || k === 'fast')
  const [health, setHealth] = useState<HealthOptions>(defaultHealth)
  const [rules, setRules] = useState<HealthRule[]>([])
  const [showRules, setShowRules] = useState(false)
  const [showBilling, setShowBilling] = useState(false)
  const months = useMemo(() => {
    const out: { v: string; label: string }[] = []
    const d = new Date()
    for (let i = 1; i <= 6; i++) {
      const m = new Date(d.getFullYear(), d.getMonth() - i, 1)
      out.push({ v: `${m.getFullYear()}${String(m.getMonth() + 1).padStart(2, '0')}`, label: m.toLocaleDateString(locale(), { month: 'long', year: 'numeric' }) })
    }
    return out
  }, [])
  useEffect(() => { api.healthRules().then(setRules).catch(() => { /* panel stays empty */ }) }, [])
  const toggleRule = (id: string) => setHealth(h => ({
    ...h, disabled: h.disabled.includes(id) ? h.disabled.filter(x => x !== id) : [...h.disabled, id],
  }))
  const rulesByCategory = useMemo(() => {
    const m = new Map<string, HealthRule[]>()
    for (const r of rules) m.set(r.category, [...(m.get(r.category) ?? []), r])
    return [...m.entries()]
  }, [rules])
  const gridApi = useRef<GridApi<Project> | null>(null)

  // profiles and Cloud Asset Inventory discovery
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [profileName, setProfileName] = useState('')
  const [saving, setSaving] = useState<string | null>(null) // name typed while saving
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [scope, setScope] = useState('')
  const [footprint, setFootprint] = useState<Record<string, Footprint> | null>(null)
  const [discovering, setDiscovering] = useState(false)
  const pending = useRef<string[] | 'all' | null>(null) // selection to apply once rows render

  const load = useCallback(async (q: string): Promise<Project[]> => {
    setLoading(true); setError(null)
    try { const list = await api.listProjects(q); setProjects(list); return list } catch (e) { setError(errorText(e)); return [] } finally { setLoading(false) }
  }, [])
  useEffect(() => { load('') }, [load])
  useEffect(() => { api.listProfiles().then(setProfiles).catch(e => notify(errorText(e))) }, [notify])

  useEffect(() => {
    const want = pending.current
    if (!want) return
    pending.current = null
    setTimeout(() => gridApi.current?.forEachNode(n => n.setSelected(want === 'all' || want.includes(n.data!.id))))
  }, [projects])

  // Suggest the organization (or first folder) the visible projects hang from.
  const suggestion = useMemo(() => {
    const parents = projects.map(p => p.parent).filter(Boolean)
    return parents.find(p => p.startsWith('organizations/')) ?? parents.find(p => p.startsWith('folders/')) ?? ''
  }, [projects])
  useEffect(() => { if (!scope && suggestion) setScope(suggestion) }, [suggestion]) // eslint-disable-line react-hooks/exhaustive-deps

  const runDiscover = async (sc: string, restrictTo?: string[]) => {
    setDiscovering(true)
    try {
      const fps = await api.discover(sc.trim())
      const map = Object.fromEntries(fps.map(f => [f.projectId, f]))
      const withCompute = fps.filter(f => fpTotal(f) > 0).map(f => f.projectId)
      pending.current = restrictTo?.length ? withCompute.filter(id => restrictTo.includes(id)) : withCompute
      setFootprint(map)
      setProjects(cur => {
        const known = new Set(cur.map(p => p.id))
        const extra = fps.filter(f => !known.has(f.projectId)).map(f => ({
          id: f.projectId, name: t('(encontrado por Cloud Asset Inventory)'), number: f.number, parent: '', state: '', created: '', labels: null,
        }))
        return [...cur, ...extra]
      })
      notify(t('Cloud Asset Inventory: {0} proyectos con recursos de Compute en {1}', withCompute.length, sc.trim()))
    } catch (e) { notify(errorText(e)) } finally { setDiscovering(false) }
  }

  const applyProfile = async (name: string) => {
    setProfileName(name); setConfirmDelete(false)
    const p = profiles.find(x => x.name === name)
    if (!p) return
    const q = p.query ?? ''
    setQuery(q); setModules(p.modules); setConcurrency(p.concurrency || 8)
    setHealth({ ...defaultHealth, ...p.health, disabled: p.health?.disabled ?? [] }); setFootprint(null)
    if (p.discover) setScope(p.discover)
    const ids = p.projects ?? []
    pending.current = ids.length ? ids : 'all'
    const list = await load(q)
    const missing = ids.filter(id => !list.some(x => x.id === id))
    if (missing.length) {
      pending.current = ids
      setProjects(cur => [...missing.map(id => ({ id, name: t('(del perfil)'), number: '', parent: '', state: '', created: '', labels: null })), ...cur])
    }
    if (p.discover) await runDiscover(p.discover, ids)
  }

  const saveProfile = async (name: string) => {
    const withCompute = footprint ? Object.values(footprint).filter(f => fpTotal(f) > 0).map(f => f.projectId).sort() : null
    const sameAsDiscovery = withCompute && JSON.stringify([...selected].sort()) === JSON.stringify(withCompute)
    const p: Profile = {
      name: name.trim(), query, projects: selected.length === projects.length || sameAsDiscovery ? [] : selected,
      discover: footprint ? scope.trim() : '', concurrency, modules, health,
      auth: session.method, impersonate: session.method === 'impersonate' ? session.identity : '',
    }
    try {
      await api.saveProfile(p)
      setProfiles(await api.listProfiles()); setProfileName(p.name); setSaving(null)
      notify(t('Perfil "{0}" guardado', p.name))
    } catch (e) { notify(errorText(e)) }
  }

  const deleteProfile = async () => {
    try {
      await api.deleteProfile(profileName)
      setProfiles(await api.listProfiles()); notify(t('Perfil "{0}" eliminado', profileName)); setProfileName(''); setConfirmDelete(false)
    } catch (e) { notify(errorText(e)) }
  }

  const addManual = () => {
    const ids = manual.split(/[\s,;]+/).map(s => s.trim()).filter(Boolean)
    const known = new Set(projects.map(p => p.id))
    const extra = ids.filter(id => !known.has(id)).map(id => ({ id, name: t('(añadido manualmente)'), number: '', parent: '', state: '', created: '', labels: null }))
    setProjects(p => [...extra, ...p]); setManual('')
    setTimeout(() => gridApi.current?.forEachNode(n => { if (ids.includes(n.data!.id)) n.setSelected(true) }))
  }

  const start = async () => {
    try {
      onScanStart()
      const sc = scope.trim()
      const fastScope = modules.fast && /^(organizations|folders)\/\d+$/.test(sc) ? sc : ''
      await api.startScan(selected, { concurrency, health, modules: { ...modules, fastScope }, saveHistory: loadSaveHistory(), profile: profileName })
    } catch (e) { notify(errorText(e)) }
  }

  const columnDefs = useMemo<ColDef<Project>[]>(() => [
    { field: 'id', headerName: 'Project ID', flex: 2, minWidth: 200 },
    { field: 'name', headerName: t('Nombre'), flex: 2, minWidth: 180 },
    { field: 'number', headerName: t('Número'), width: 150 },
    { field: 'parent', headerName: t('Padre'), width: 170 },
    ...(footprint ? [
      { colId: 'caiVms', headerName: 'VMs (CAI)', width: 125, type: 'rightAligned', valueGetter: p => footprint[p.data!.id]?.vms ?? 0, sort: 'desc' },
      { colId: 'caiRunning', headerName: t('Encendidas'), width: 115, type: 'rightAligned', valueGetter: p => footprint[p.data!.id]?.running ?? 0 },
      { colId: 'caiDisks', headerName: t('Discos'), width: 100, type: 'rightAligned', valueGetter: p => footprint[p.data!.id]?.disks ?? 0 },
      { colId: 'caiOther', headerName: t('Otros'), width: 95, type: 'rightAligned', headerTooltip: t('Snapshots, imágenes, templates, reservas y commitments'), valueGetter: p => footprint[p.data!.id]?.other ?? 0 },
    ] as ColDef<Project>[] : []),
    { colId: 'labels', headerName: 'Labels', flex: 2, minWidth: 160, valueGetter: p => Object.entries(p.data?.labels ?? {}).map(([k, v]) => `${k}=${v}`).join('; ') },
  ], [footprint])

  const pct = scan.progress && scan.progress.total ? Math.round((scan.progress.done / scan.progress.total) * 100) : 0

  return (
    <div className="page fill">
      <div className="page-head">
        <div className="grow">
          <h1 className="headline-s">{t('Proyectos')}</h1>
          <p className="body-m muted" style={{ margin: '4px 0 0' }}>
            {loading ? t('Buscando proyectos…') : t('{0} proyectos visibles · {1} seleccionados', projects.length, selected.length)}
          </p>
        </div>
        <div className="field with-icon" style={{ width: 340 }}>
          <span className="lead"><Icon name="filter_alt" /></span>
          <input id="q" value={query} placeholder=" " spellCheck={false} onChange={e => setQuery(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') load(query) }} />
          <label htmlFor="q">{t('Filtro Resource Manager (parent:folders/123)')}</label>
        </div>
        <button className="btn outlined" onClick={() => load(query)} disabled={loading}><Icon name="refresh" />{t('Buscar')}</button>
      </div>

      <div className="row" style={{ gap: 8, flexWrap: 'wrap' }}>
        <span className="label-l muted" style={{ marginRight: 4 }}>{t('Perfil')}</span>
        <select value={profileName} onChange={e => applyProfile(e.target.value)} aria-label={t('Perfil de escaneo')} style={selectStyle}>
          <option value="">{t('Sin perfil')}</option>
          {profiles.map(p => <option key={p.name} value={p.name}>{p.name}</option>)}
        </select>
        {saving === null ? <>
          <button className="btn text" onClick={() => setSaving(profileName)} title={t('Guarda proyectos, filtro, módulos y descubrimiento para repetir el escaneo')}><Icon name="bookmark_add" />{t('Guardar perfil')}</button>
          {profileName && (confirmDelete
            ? <><span className="body-m">{t('¿Eliminar "{0}"?', profileName)}</span>
                <button className="btn text" onClick={deleteProfile}>{t('Eliminar')}</button>
                <button className="btn text" onClick={() => setConfirmDelete(false)}>{t('Cancelar')}</button></>
            : <button className="icon-btn" onClick={() => setConfirmDelete(true)} aria-label={t('Eliminar perfil')} title={t('Eliminar perfil')}><Icon name="delete" /></button>)}
        </> : <>
          <div className="search" style={{ minWidth: 220 }}>
            <Icon name="bookmark" />
            <input autoFocus value={saving} onChange={e => setSaving(e.target.value)} placeholder={t('Nombre del perfil')} aria-label={t('Nombre del perfil')} maxLength={60}
              onKeyDown={e => { if (e.key === 'Enter' && saving.trim()) saveProfile(saving); if (e.key === 'Escape') setSaving(null) }} />
          </div>
          <button className="btn filled" disabled={!saving.trim()} onClick={() => saveProfile(saving)}>{t('Guardar')}</button>
          <button className="btn text" onClick={() => setSaving(null)}>{t('Cancelar')}</button>
        </>}
        <span className="spacer" />
        <div className="search" style={{ minWidth: 300 }} title={t('Cloud Asset Inventory: una sola consulta para saber qué proyectos tienen VMs, discos, snapshots o imágenes. Requiere roles/cloudasset.viewer en ese ámbito.')}>
          <Icon name="travel_explore" />
          <input value={scope} onChange={e => setScope(e.target.value)} placeholder={t('organizations/123 o folders/456')} aria-label={t('Ámbito de Cloud Asset Inventory')}
            onKeyDown={e => { if (e.key === 'Enter' && scope.trim()) runDiscover(scope) }} spellCheck={false} />
        </div>
        <button className="btn outlined" disabled={!scope.trim() || discovering || !session.fullScope} onClick={() => runDiscover(scope)}
          title={session.fullScope ? undefined : t('Solo con gcloud (ADC)')}>
          <Icon name="travel_explore" />{discovering ? t('Detectando…') : t('Detectar proyectos con Compute')}
        </button>
      </div>

      {error && <div className="banner error"><Icon name="error" /><div className="selectable">{error}<br />
        <span className="body-s">{t('Si no tienes permiso para listar proyectos, añádelos manualmente por ID.')}</span></div></div>}

      {(scan.running || scan.log.length > 0) && (
        <div className="card elevated col">
          <div className="row">
            <Icon name={scan.running ? 'radar' : 'task_alt'} />
            <span className="title-m">{scan.running ? t('Escaneando…') : t('Último escaneo')}</span>
            <span className="spacer" />
            {scan.progress && <span className="body-m muted">
              {t('{0}/{1} proyectos · {2} VMs · {3} errores', scan.progress.done, scan.progress.total, scan.progress.vms, scan.progress.errors)}
            </span>}
            {scan.running && <button className="btn text" onClick={() => api.cancelScan()}><Icon name="stop_circle" />{t('Cancelar')}</button>}
          </div>
          <div className={`progress ${scan.running && !scan.progress ? 'indeterminate' : ''}`}><div style={{ width: `${pct}%` }} /></div>
          {scan.running && scan.progress && scan.progress.step !== 'done' && (
            <span className="body-s muted">{t('Leyendo {0} en', stepLabel()[scan.progress.step] ?? scan.progress.step)} <span className="mono">{scan.progress.project}</span></span>
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
        <span className="label-l muted" style={{ marginRight: 4 }}>{t('Módulos opcionales')}</span>
        {([
          ['osconfig', 'memory', t('SO y agentes (OS Config)')],
          ['monitoring', 'monitoring', t('Métricas CPU/RAM')],
          ['recommender', 'lightbulb', t('Recomendaciones')],
          ['pricing', 'payments', t('Coste estimado')],
        ] as const).map(([k, icon, label]) => {
          const off = fullScopeOnly(k)
          const on = modules[k] && !off
          return (
            <button key={k} className={`chip ${on ? 'selected' : ''}`} onClick={() => toggle(k)} aria-pressed={on}
              disabled={off} title={off ? t('Solo con gcloud (ADC)') : undefined}>
              <Icon name={on ? 'check' : icon} size="sm" />{label}
            </button>
          )
        })}
        {modules.monitoring && (
          <select value={modules.metricsDays} onChange={e => setModules(m => ({ ...m, metricsDays: Number(e.target.value) }))}
            aria-label={t('Días de métricas')}
            style={{ height: 32, borderRadius: 8, border: '1px solid var(--md-outline-variant)', background: 'transparent', color: 'inherit', padding: '0 8px' }}>
            {[7, 14, 30, 42].map(n => <option key={n} value={n}>{t('{0} días', n)}</option>)}
          </select>
        )}
        {modules.pricing && (
          <select value={modules.currency} onChange={e => setModules(m => ({ ...m, currency: e.target.value }))}
            aria-label={t('Moneda')}
            style={{ height: 32, borderRadius: 8, border: '1px solid var(--md-outline-variant)', background: 'transparent', color: 'inherit', padding: '0 8px' }}>
            {['USD', 'EUR', 'CLP', 'MXN', 'COP', 'BRL', 'GBP'].map(c => <option key={c} value={c}>{c}</option>)}
          </select>
        )}
        <span className="body-s muted">{t('Si la API no está habilitada en un proyecto, el módulo se omite sin marcarlo como error.')}</span>
        <span className="spacer" />
        <button className={`chip ${modules.fast && !fullScopeOnly('fast') ? 'selected' : ''}`} onClick={() => setModules(m => ({ ...m, fast: !m.fast }))}
          aria-pressed={!!modules.fast && !fullScopeOnly('fast')} disabled={fullScopeOnly('fast')}
          title={fullScopeOnly('fast') ? t('Solo con gcloud (ADC)') : t('Lee VMs, discos, snapshots, imágenes, templates, MIGs y redes de Cloud Asset Inventory: una consulta por proyecto, o una sola para toda la organización o carpeta indicada en el ámbito de detección. Puede reflejar cambios con unos minutos de retraso. Necesita roles/cloudasset.viewer.')}>
          <Icon name={modules.fast && !fullScopeOnly('fast') ? 'check' : 'bolt'} size="sm" />{t('Modo rápido (Cloud Asset Inventory)')}
        </button>
        <button className={`chip ${modules.billingTable ? 'selected' : ''}`} onClick={() => setShowBilling(v => !v)} aria-expanded={showBilling}
          title={t('Coste neto real del mes (con CUD, SUD y descuentos) desde la exportación de facturación a BigQuery')}>
          <Icon name={modules.billingTable ? 'check' : 'receipt_long'} size="sm" />{t('Coste real (BigQuery)')}
          <Icon name={showBilling ? 'expand_less' : 'expand_more'} size="sm" />
        </button>
        <button className={`chip ${showRules ? 'selected' : ''}`} onClick={() => setShowRules(v => !v)} aria-expanded={showRules}>
          <Icon name="rule" size="sm" />{t('Reglas de salud')}{health.disabled.length > 0 ? ` · ${t('{0} desactivadas', health.disabled.length)}` : ''}
          <Icon name={showRules ? 'expand_less' : 'expand_more'} size="sm" />
        </button>
      </div>

      {showBilling && (
        <div className="card outlined col" style={{ gap: 10 }}>
          <div className="row" style={{ gap: 12, flexWrap: 'wrap' }}>
            <div className="search" style={{ minWidth: 460, flex: 1 }}>
              <Icon name="table" />
              <input value={modules.billingTable ?? ''} spellCheck={false} aria-label={t('Tabla de exportación de facturación')}
                onChange={e => setModules(m => ({ ...m, billingTable: e.target.value.trim() }))}
                placeholder={t('proyecto.dataset.gcp_billing_export_resource_v1_XXXXXX_XXXXXX_XXXXXX')} />
            </div>
            <label className="row body-m" style={{ gap: 8 }}>{t('Mes')}
              <select value={modules.billingMonth ?? ''} onChange={e => setModules(m => ({ ...m, billingMonth: e.target.value }))} style={selectStyle}>
                <option value="">{t('Mes anterior (al escanear)')}</option>
                {months.map(m => <option key={m.v} value={m.v}>{m.label}</option>)}
              </select>
            </label>
            <label className="row body-m" style={{ gap: 8 }}>{t('Límite')}
              <input type="number" min={1} max={10000} value={modules.billingMaxGB || 50}
                onChange={e => setModules(m => ({ ...m, billingMaxGB: Math.max(1, Number(e.target.value) || 50) }))}
                aria-label={t('Límite de GB leídos')} style={{ ...selectStyle, width: 80 }} /> GB
            </label>
            {modules.billingTable && <button className="btn text" onClick={() => setModules(m => ({ ...m, billingTable: '' }))}>{t('Desactivar')}</button>}
          </div>
          <span className="body-s muted">
            {t('Añade la columna "Coste real mes" en gInfo y gDisks (con la exportación')} <b>{t('detallada')}</b>{t(') y la pestaña gCostReal con el neto frente al estimado.')}
            {' '}{t('Es el único módulo con coste: BigQuery cobra los datos leídos (1 TiB gratis al mes). gceTools hace antes una estimación gratuita y no ejecuta la consulta si supera el límite.')}
            {' '}{t('Permisos:')} <span className="mono">roles/bigquery.jobUser</span> {t('en el proyecto de la tabla y')} <span className="mono">roles/bigquery.dataViewer</span> {t('en el dataset.')}
          </span>
        </div>
      )}

      {showRules && (
        <div className="card outlined col" style={{ gap: 12, maxHeight: 360, overflow: 'auto' }}>
          <div className="row" style={{ gap: 8 }}>
            <span className="title-s">{t('Reglas de gHealth')}</span>
            <span className="body-s muted">
              {t('Se guardan en el perfil. Para exceptuar un recurso concreto, ponle el label')} <span className="mono">gcetools-ignore=&lt;id&gt;</span> {t('(o')} <span className="mono">gcetools-ignore=all</span>{t(') en la VM, el disco, el template o el proyecto.')}
            </span>
            <span className="spacer" />
            <button className="btn text" onClick={() => setHealth(defaultHealth)}><Icon name="restart_alt" />{t('Valores por defecto')}</button>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(420px, 1fr))', gap: '4px 24px' }}>
            {rulesByCategory.map(([cat, list]) => (
              <div key={cat} className="col" style={{ gap: 2 }}>
                <span className="label-l muted" style={{ marginTop: 6 }}>{cat}</span>
                {list.map(r => {
                  const on = !health.disabled.includes(r.id)
                  const param = r.param as NumericHealth | undefined
                  return (
                    <div key={r.id} className="row" style={{ gap: 8, minHeight: 34 }}>
                      <label className="row body-m" style={{ gap: 8, cursor: 'pointer', flex: 1, minWidth: 0 }} title={t('ID: {0} · excepción en: {1}', r.id, r.labels)}>
                        <input type="checkbox" checked={on} onChange={() => toggleRule(r.id)}
                          style={{ width: 18, height: 18, accentColor: 'var(--md-primary)', flexShrink: 0 }} />
                        <span style={{ opacity: on ? 1 : 0.55 }}>{r.title}</span>
                        <span className={`badge sev-${sevKey(r.severity)}`}>{r.severity}</span>
                        <span className="mono body-s muted">{r.id}</span>
                      </label>
                      {param && (
                        <label className="row body-s" style={{ gap: 4 }}>
                          <input type="number" min={1} max={param.endsWith('Pct') ? 100 : 3650} value={health[param]} disabled={!on}
                            onChange={e => { const v = Number(e.target.value); if (v > 0) setHealth(h => ({ ...h, [param]: v })) }}
                            aria-label={t('Umbral de {0}', r.title)}
                            style={{ width: 64, height: 28, borderRadius: 6, border: '1px solid var(--md-outline-variant)', background: 'transparent', color: 'inherit', padding: '0 6px' }} />
                          {paramUnit(param)}
                        </label>
                      )}
                    </div>
                  )
                })}
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="toolbar">
        <div className="search">
          <Icon name="search" />
          <input value={filter} onChange={e => setFilter(e.target.value)} placeholder={t('Filtrar la lista')} aria-label={t('Filtrar proyectos')} />
        </div>
        <div className="search" style={{ minWidth: 320 }}>
          <Icon name="add" />
          <input value={manual} onChange={e => setManual(e.target.value)} placeholder={t('Añadir IDs manualmente (separados por comas)')}
            onKeyDown={e => { if (e.key === 'Enter' && manual.trim()) addManual() }} aria-label={t('Añadir proyectos por ID')} />
        </div>
        <span className="spacer" />
        <label className="row body-m" style={{ gap: 8 }}>
          {t('Paralelismo')}
          <select value={concurrency} onChange={e => setConcurrency(Number(e.target.value))} style={selectStyle}>
            {[2, 4, 8, 16, 32].map(n => <option key={n} value={n}>{n}</option>)}
          </select>
        </label>
        <button className="btn filled" disabled={selected.length === 0 || scan.running} onClick={start}>
          <Icon name="play_arrow" />{t('Escanear')} {selected.length > 0 ? `(${selected.length})` : ''}
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
          overlayNoRowsTemplate={t('No hay proyectos visibles con esta identidad')}
        />
      </div>
    </div>
  )
}
