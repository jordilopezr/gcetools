import { useCallback, useEffect, useState } from 'react'
import { api, CURRENT, errorText, type AboutInfo, type Inventory, type Progress, type SessionInfo } from './lib/api'
import { Icon, Snackbar, Menu } from './components/ui'
import appIcon from './assets/gcetools-icon-small.svg' // simplified variant for small sizes
import ConnectPage from './pages/ConnectPage'
import ProjectsPage from './pages/ProjectsPage'
import InventoryPage from './pages/InventoryPage'
import AboutPage, { type UpdateState } from './pages/AboutPage'
import ComparePage, { type CompareState } from './pages/ComparePage'

type Page = 'connect' | 'projects' | 'inventory' | 'compare' | 'about'
type Theme = 'system' | 'light' | 'dark'

export interface ScanState {
  running: boolean
  progress: Progress | null
  log: { project: string; status: string }[]
}

function loadAutoCheck(): boolean {
  try { return localStorage.getItem('autoUpdateCheck') !== 'false' } catch { return true }
}

function loadTheme(): Theme {
  try { return (localStorage.getItem('theme') as Theme) || 'system' } catch { return 'system' }
}

export default function App() {
  const [page, setPage] = useState<Page>('connect')
  const [session, setSession] = useState<SessionInfo | null>(null)
  const [inventory, setInventory] = useState<Inventory | null>(null)
  const [scan, setScan] = useState<ScanState>({ running: false, progress: null, log: [] })
  const [snack, setSnack] = useState<string | null>(null)
  const [theme, setTheme] = useState<Theme>(loadTheme)
  const [about, setAbout] = useState<AboutInfo | null>(null)
  const [update, setUpdate] = useState<UpdateState>({ status: 'idle' })
  const [autoCheck, setAutoCheckState] = useState(loadAutoCheck)
  const [compare, setCompare] = useState<CompareState>({ before: null, after: null, result: null })

  const setAutoCheck = (v: boolean) => {
    setAutoCheckState(v)
    try { localStorage.setItem('autoUpdateCheck', String(v)) } catch { /* storage unavailable */ }
  }

  const checkUpdate = useCallback(async (silent: boolean) => {
    setUpdate(u => ({ ...u, status: 'checking' }))
    try {
      const info = await api.checkUpdate()
      setUpdate({ status: 'done', info })
      if (info.available && silent) setSnack(`Hay una versión nueva de gceTools: ${info.latest}`)
    } catch (e) {
      setUpdate({ status: 'error', error: errorText(e) })
    }
  }, [])

  useEffect(() => { api.appInfo().then(setAbout).catch(() => { /* non-fatal */ }) }, [])
  useEffect(() => {
    if (!autoCheck) return
    const t = setTimeout(() => checkUpdate(true), 2500)
    return () => clearTimeout(t)
  }, []) // eslint-disable-line react-hooks/exhaustive-deps -- once at startup

  useEffect(() => {
    const root = document.documentElement
    if (theme === 'system') root.removeAttribute('data-theme')
    else root.setAttribute('data-theme', theme)
    try { localStorage.setItem('theme', theme) } catch { /* storage unavailable */ }
  }, [theme])

  useEffect(() => {
    const offP = api.onProgress(p => setScan(s => ({
      running: true,
      progress: p,
      log: p.step === 'done' ? [{ project: p.project, status: p.message }, ...s.log].slice(0, 500) : s.log,
    })))
    const offD = api.onDone(async meta => {
      setScan(s => ({ ...s, running: false }))
      // a comparison against the previous in-memory inventory is now stale
      setCompare(c => c.after?.path === CURRENT ? { ...c, after: null, result: null } : c)
      try {
        setInventory(await api.getInventory())
        setPage('inventory')
        setSnack(meta.cancelled ? 'Escaneo cancelado: se muestran resultados parciales' : 'Escaneo completado')
      } catch (e) { setSnack(errorText(e)) }
    })
    const offE = api.onError(msg => { setScan(s => ({ ...s, running: false })); setSnack(msg) })
    return () => { offP(); offD(); offE() }
  }, [])

  const notify = useCallback((m: string) => setSnack(m), [])

  const disconnect = async () => {
    await api.disconnect()
    setSession(null); setInventory(null); setScan({ running: false, progress: null, log: [] }); setPage('connect')
    setCompare(c => c.after?.path === CURRENT ? { ...c, after: null, result: null } : c)
  }

  const nav: { id: Page; icon: string; label: string; enabled: boolean }[] = [
    { id: 'connect', icon: 'key', label: 'Conexión', enabled: true },
    { id: 'projects', icon: 'folder_open', label: 'Proyectos', enabled: !!session },
    { id: 'inventory', icon: 'table_view', label: 'Inventario', enabled: !!inventory },
    { id: 'compare', icon: 'compare_arrows', label: 'Comparar', enabled: true },
  ]
  const updateAvailable = update.status === 'done' && !!update.info?.available
  const themeIcon = theme === 'dark' ? 'dark_mode' : theme === 'light' ? 'light_mode' : 'contrast'

  return (
    <div className="shell">
      <header className="topbar">
        <div className="brand">
          <img className="brand-logo" src={appIcon} alt="" width={36} height={36} />
          <div>
            <div className="brand-name">gceTools</div>
          </div>
          <span className="brand-sub">Inventario de Compute Engine</span>
        </div>
        <div className="spacer" />
        {updateAvailable && (
          <button className="assist" style={{ border: 0, cursor: 'pointer', background: 'var(--md-primary-container)', color: 'var(--md-on-primary-container)' }}
            onClick={() => setPage('about')} title="Ver novedades y descargar">
            <span className="avatar" style={{ background: 'var(--md-primary)' }}><Icon name="arrow_upward" size="sm" /></span>
            Versión {update.info!.latest} disponible
          </button>
        )}
        {api.mock && <span className="badge sev-Media" title="Ejecutándose fuera de Wails con datos simulados">Modo demo</span>}
        <Menu trigger={open => (
          <button className="icon-btn" onClick={open} aria-label="Tema" title="Tema"><Icon name={themeIcon} /></button>
        )}>
          {close => (['system', 'light', 'dark'] as Theme[]).map(t => (
            <button key={t} onClick={() => { setTheme(t); close() }}>
              <Icon name={t === 'dark' ? 'dark_mode' : t === 'light' ? 'light_mode' : 'contrast'} />
              <span>{{ system: 'Según el sistema', light: 'Claro', dark: 'Oscuro' }[t]}</span>
              {theme === t && <><span className="spacer" /><Icon name="check" size="sm" /></>}
            </button>
          ))}
        </Menu>
        {session && (
          <Menu trigger={open => (
            <button className="assist" style={{ border: 0, cursor: 'pointer' }} onClick={open}>
              <span className="avatar">{session.identity.charAt(0).toUpperCase()}</span>
              <span className="selectable">{session.identity}</span>
              <Icon name="arrow_drop_down" size="sm" />
            </button>
          )}>
            {close => <>
              <div style={{ padding: '8px 16px' }}>
                <div className="title-s selectable">{session.identity}</div>
                <div className="sub">{{ adc: 'Credenciales de gcloud (ADC)', sa: 'Clave de service account', impersonate: 'Impersonación' }[session.method] ?? session.method}</div>
              </div>
              <hr />
              <button onClick={() => { close(); disconnect() }}><Icon name="logout" /><span>Desconectar</span></button>
            </>}
          </Menu>
        )}
      </header>

      <nav className="rail" aria-label="Navegación">
        {nav.map(n => (
          <button key={n.id} className={`rail-item ${page === n.id ? 'active' : ''}`} disabled={!n.enabled}
            onClick={() => setPage(n.id)} aria-current={page === n.id ? 'page' : undefined}>
            <span className="rail-pill"><Icon name={n.icon} /></span>
            <span className="rail-label">{n.label}</span>
          </button>
        ))}
        <span className="spacer" />
        <button className={`rail-item ${page === 'about' ? 'active' : ''}`} onClick={() => setPage('about')}
          aria-current={page === 'about' ? 'page' : undefined} style={{ marginBottom: 16 }}>
          <span className="rail-pill" style={{ position: 'relative' }}>
            <Icon name="info" />
            {updateAvailable && <span className="rail-dot" aria-label="Actualización disponible" />}
          </span>
          <span className="rail-label">Acerca de</span>
        </button>
      </nav>

      <main className="main">
        {page === 'connect' && <ConnectPage session={session} notify={notify}
          onConnected={s => { setSession(s); setInventory(null); setPage('projects') }} />}
        {page === 'projects' && session && <ProjectsPage session={session} scan={scan} notify={notify}
          onScanStart={() => setScan({ running: true, progress: null, log: [] })} />}
        {page === 'inventory' && inventory && <InventoryPage inventory={inventory} notify={notify} />}
        {page === 'compare' && <ComparePage state={compare} setState={setCompare} hasInventory={!!inventory} notify={notify} />}
        {page === 'about' && <AboutPage about={about} update={update} onCheck={() => checkUpdate(false)}
          autoCheck={autoCheck} setAutoCheck={setAutoCheck} />}
      </main>

      <Snackbar message={snack} onClose={() => setSnack(null)} />
    </div>
  )
}
