import { useCallback, useEffect, useState } from 'react'
import { api, errorText, type Inventory, type Progress, type SessionInfo } from './lib/api'
import { Icon, Snackbar, Menu } from './components/ui'
import ConnectPage from './pages/ConnectPage'
import ProjectsPage from './pages/ProjectsPage'
import InventoryPage from './pages/InventoryPage'

type Page = 'connect' | 'projects' | 'inventory'
type Theme = 'system' | 'light' | 'dark'

export interface ScanState {
  running: boolean
  progress: Progress | null
  log: { project: string; status: string }[]
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
  }

  const nav: { id: Page; icon: string; label: string; enabled: boolean }[] = [
    { id: 'connect', icon: 'key', label: 'Conexión', enabled: true },
    { id: 'projects', icon: 'folder_open', label: 'Proyectos', enabled: !!session },
    { id: 'inventory', icon: 'table_view', label: 'Inventario', enabled: !!inventory },
  ]
  const themeIcon = theme === 'dark' ? 'dark_mode' : theme === 'light' ? 'light_mode' : 'contrast'

  return (
    <div className="shell">
      <header className="topbar">
        <div className="brand">
          <div className="brand-mark"><Icon name="dns" filled /></div>
          <div>
            <div className="brand-name">gceTools</div>
          </div>
          <span className="brand-sub">Inventario de Compute Engine</span>
        </div>
        <div className="spacer" />
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
      </nav>

      <main className="main">
        {page === 'connect' && <ConnectPage session={session} notify={notify}
          onConnected={s => { setSession(s); setInventory(null); setPage('projects') }} />}
        {page === 'projects' && session && <ProjectsPage scan={scan} notify={notify}
          onScanStart={() => setScan({ running: true, progress: null, log: [] })} />}
        {page === 'inventory' && inventory && <InventoryPage inventory={inventory} notify={notify} />}
      </main>

      <Snackbar message={snack} onClose={() => setSnack(null)} />
    </div>
  )
}
