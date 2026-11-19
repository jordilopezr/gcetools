import { useMemo, useState } from 'react'
import { api, CURRENT, errorText, type ExportFormat, type Inventory, type SnapshotInfo } from '../lib/api'
import { Icon, Menu } from '../components/ui'
import SheetBrowser, { fmtDate } from '../components/SheetBrowser'

export interface CompareState {
  before: SnapshotInfo | null
  after: SnapshotInfo | null
  result: Inventory | null
}

interface Props {
  state: CompareState
  setState: (s: CompareState) => void
  hasInventory: boolean
  notify: (m: string) => void
}

function SnapshotCard({ label, hint, snap, onPick, onCurrent, canCurrent, busy }: {
  label: string; hint: string; snap: SnapshotInfo | null
  onPick: () => void; onCurrent?: () => void; canCurrent: boolean; busy: boolean
}) {
  return (
    <div className="card outlined col" style={{ flex: 1, minWidth: 260, gap: 10 }}>
      <span className="label-l muted">{label}</span>
      {snap ? (
        <div className="row" style={{ gap: 12 }}>
          <span className="method-icon"><Icon name={snap.path === CURRENT ? 'table_view' : 'data_object'} /></span>
          <div className="col" style={{ gap: 2, minWidth: 0 }}>
            <span className="title-m selectable" style={{ overflowWrap: 'anywhere' }}>{snap.name}</span>
            <span className="body-s muted">
              {fmtDate(snap.startedAt)} · {snap.projects} proyectos · {snap.vms} VMs{snap.version ? ` · v${snap.version}` : ''}
            </span>
            {snap.identity && <span className="body-s muted selectable">{snap.identity}</span>}
          </div>
        </div>
      ) : <span className="body-m muted">{hint}</span>}
      <div className="row" style={{ gap: 8, flexWrap: 'wrap' }}>
        <button className="btn outlined" onClick={onPick} disabled={busy}><Icon name="folder_open" />{snap ? 'Cambiar' : 'Elegir snapshot JSON'}</button>
        {onCurrent && canCurrent && snap?.path !== CURRENT && (
          <button className="btn text" onClick={onCurrent} disabled={busy}><Icon name="table_view" />Usar el inventario actual</button>
        )}
      </div>
    </div>
  )
}

export default function ComparePage({ state, setState, hasInventory, notify }: Props) {
  const [busy, setBusy] = useState(false)
  const { before, after, result } = state

  const pick = async (side: 'before' | 'after') => {
    try {
      const s = await api.pickSnapshot()
      if (s) setState({ ...state, [side]: s, result: null })
    } catch (e) { notify(errorText(e)) }
  }
  const useCurrent = async () => {
    const s = await api.currentSnapshot()
    if (s) setState({ ...state, after: s, result: null })
  }
  const run = async () => {
    if (!before || !after) return
    setBusy(true)
    try {
      setState({ ...state, result: await api.compare(before.path, after.path) })
    } catch (e) { notify(errorText(e)) } finally { setBusy(false) }
  }
  const doExport = async (f: ExportFormat, sep = ',') => {
    try {
      const path = await api.exportDiff(f, sep)
      if (path) notify(`Exportado a ${path}`)
    } catch (e) { notify(errorText(e)) }
  }

  const stats = useMemo(() => {
    if (!result) return []
    const rows = result.sheets.find(s => s.name === 'Resumen')?.rows ?? []
    const sum = (k: string) => rows.reduce((a, r) => a + (Number(r[k]) || 0), 0)
    const meta = result.sheets.find(s => s.name === 'Metadatos')?.rows ?? []
    const cost = meta.find(r => r.property === 'Diferencia de coste/mes')?.value as string | undefined
    return [
      { k: 'Altas', v: sum('added').toLocaleString('es-ES') },
      { k: 'Bajas', v: sum('removed').toLocaleString('es-ES') },
      { k: 'Recursos modificados', v: sum('modified').toLocaleString('es-ES') },
      { k: 'Cambios relevantes', v: sum('relevant').toLocaleString('es-ES'), alert: sum('relevant') > 0 },
      ...(cost ? [{ k: 'Diferencia de coste/mes', v: cost }] : []),
    ]
  }, [result])

  const noChanges = result && stats.slice(0, 3).every(s => s.v === '0')

  return (
    <div className={`page ${result ? 'fill' : ''}`}>
      <div className="page-head">
        <div className="grow">
          <h1 className="headline-s">Comparar inventarios</h1>
          <p className="body-m muted" style={{ margin: '4px 0 0' }}>
            Altas, bajas y cambios entre dos snapshots JSON (Inventario → Exportar → Snapshot JSON). No necesita conexión a Google Cloud.
          </p>
        </div>
        {result && (
          <Menu trigger={open => <button className="btn filled" onClick={open}><Icon name="download" />Exportar</button>}>
            {close => <>
              <button onClick={() => { close(); doExport('xlsx') }}><Icon name="table" /><div><div>Excel (.xlsx)</div><div className="sub">Resumen, Altas, Bajas, Cambios y Metadatos</div></div></button>
              <button onClick={() => { close(); doExport('csv', ',') }}><Icon name="csv" /><div><div>CSV (coma)</div><div className="sub">ZIP con un CSV por pestaña</div></div></button>
              <button onClick={() => { close(); doExport('csv', ';') }}><Icon name="csv" /><div><div>CSV (punto y coma)</div><div className="sub">Para Excel con configuración regional española</div></div></button>
            </>}
          </Menu>
        )}
      </div>

      <div className="row" style={{ gap: 12, alignItems: 'stretch', flexWrap: 'wrap' }}>
        <SnapshotCard label="Anterior (A)" hint="El inventario de referencia, por ejemplo el del mes pasado." snap={before}
          onPick={() => pick('before')} canCurrent={false} busy={busy} />
        <div className="row" style={{ alignSelf: 'center' }}><Icon name="arrow_forward" /></div>
        <SnapshotCard label="Nuevo (B)" hint="El inventario más reciente: otro snapshot o el escaneo que tienes abierto." snap={after}
          onPick={() => pick('after')} onCurrent={useCurrent} canCurrent={hasInventory} busy={busy} />
        <div className="row" style={{ alignSelf: 'center' }}>
          <button className="btn filled" onClick={run} disabled={!before || !after || busy}>
            <Icon name="compare_arrows" />{busy ? 'Comparando…' : 'Comparar'}
          </button>
        </div>
      </div>

      {result && <>
        <div className="stats">
          {stats.map(s => (
            <div key={s.k} className={`stat ${s.alert ? 'alert' : ''}`}>
              <div className="v">{s.v}</div>
              <div className="k">{s.k}</div>
            </div>
          ))}
        </div>
        {noChanges && (
          <div className="banner info"><Icon name="check_circle" />
            <span className="body-m">No hay diferencias en los recursos comparados. Revisa la pestaña Metadatos para ver qué se pudo comparar.</span>
          </div>
        )}
        <SheetBrowser key={`${before?.path}|${after?.path}|${result.meta.finishedAt}`} sheets={result.sheets}
          emptyText="Sin diferencias de este tipo." />
      </>}
    </div>
  )
}
