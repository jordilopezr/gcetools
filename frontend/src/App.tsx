import { useCallback, useEffect, useRef, useState } from 'react'
import { api, CURRENT, errorText, type AboutInfo, type InventorySummary, type Progress, type SessionInfo } from './lib/api'
import { Icon, Snackbar, Menu } from './components/ui'
import appIcon from './assets/gcetools-icon-small.svg' // simplified variant for small sizes
import ConnectPage from './pages/ConnectPage'
import ProjectsPage from './pages/ProjectsPage'
import InventoryPage from './pages/InventoryPage'
import AboutPage, { type UpdateState } from './pages/AboutPage'
import ComparePage, { type CompareState } from './pages/ComparePage'
import HistoryPage from './pages/HistoryPage'
import { getLang, setLang as storeLang, t, type Lang } from './lib/i18n'

type Page = 'connect' | 'projects' | 'inventory' | 'compare' | 'history' | 'about'
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
  const [inventory, setInventory] = useState<InventorySummary | null>(null)
  const [scan, setScan] = useState<ScanState>({ running: false, progress: null, log: [] })
  const [snack, setSnack] = useState<string | null>(null)
  const [theme, setTheme] = useState<Theme>(loadTheme)
  const [about, setAbout] = useState<AboutInfo | null>(null)
  const [update, setUpdate] = useState<UpdateState>({ status: 'idle' })
  const [autoCheck, setAutoCheckState] = useState(loadAutoCheck)
  const [compare, setCompare] = useState<CompareState>({ before: null, after: null, result: null })
  const [lang, setLangState] = useState<Lang>(getLang)

  // Pages are keyed by language: switching remounts them so every t() runs
  // again; App keeps the session, inventory and comparison.
  const setLang = async (l: Lang) => {
    storeLang(l)
    try { await api.setLanguage(l) } catch { /* older backend: UI only */ }
    if (inventory) { try { setInventory(await api.inventorySummary() ?? inventory) } catch { /* keep */ } }
    setLangState(l)
  }
  useEffect(() => { api.setLanguage(getLang()).catch(() => { /* older backend */ }) }, [])

  const setAutoCheck = (v: boolean) => {
    setAutoCheckState(v)
    try { localStorage.setItem('autoUpdateCheck', String(v)) } catch { /* storage unavailable */ }
  }

  const checkUpdate = useCallback(async (silent: boolean) => {
    setUpdate(u => ({ ...u, status: 'checking' }))
    try {
      const info = await api.checkUpdate()
      setUpdate({ status: 'done', info })
      if (info.available && silent) setSnack(t('Hay una versión nueva de gceTools: {0}', info.latest))
    } catch (e) {
      setUpdate({ status: 'error', error: errorText(e) })
    }
  }, [])

  useEffect(() => { api.appInfo().then(setAbout).catch(() => { /* non-fatal */ }) }, [])
  useEffect(() => {
    if (!autoCheck) return
    const timer = setTimeout(() => checkUpdate(true), 2500)
    return () => clearTimeout(timer)
  }, []) // eslint-disable-line react-hooks/exhaustive-deps -- once at startup

  useEffect(() => {
    const root = document.documentElement
    if (theme === 'system') root.removeAttribute('data-theme')
    else root.setAttribute('data-theme', theme)
    try { localStorage.setItem('theme', theme) } catch { /* storage unavailable */ }
  }, [theme])

  const scanWarning = useRef<string | null>(null)
  useEffect(() => {
    const offW = api.onWarning(msg => { scanWarning.current = msg })
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
        setInventory(await api.inventorySummary())
        setPage('inventory')
        const done = meta.cancelled ? t('Escaneo cancelado: se muestran resultados parciales') : t('Escaneo completado')
        setSnack(scanWarning.current ? `${done}. ${scanWarning.current}` : done)
      } catch (e) { setSnack(errorText(e)) } finally { scanWarning.current = null }
    })
    const offE = api.onError(msg => { setScan(s => ({ ...s, running: false })); setSnack(msg) })
    return () => { offW(); offP(); offD(); offE() }
  }, [])

  const notify = useCallback((m: string) => setSnack(m), [])

  const openHistory = async (id: string) => {
    try { setInventory(await api.openHistory(id)); setPage('inventory') } catch (e) { setSnack(errorText(e)) }
  }
  const compareHistory = async (beforeId: string, afterId: string) => {
    try {
      const [before, after] = await Promise.all([api.historySnapshot(beforeId), api.historySnapshot(afterId)])
      setCompare({ before, after, result: await api.compare(before.path, after.path) })
      setPage('compare')
    } catch (e) { setSnack(errorText(e)) }
  }

  const disconnect = async () => {
    await api.disconnect()
    setSession(null); setInventory(null); setScan({ running: false, progress: null, log: [] }); setPage('connect')
    setCompare(c => c.after?.path === CURRENT ? { ...c, after: null, result: null } : c)
  }

  const nav: { id: Page; icon: string; label: string; enabled: boolean }[] = [
    { id: 'connect', icon: 'key', label: t('Conexión'), enabled: true },
    { id: 'projects', icon: 'folder_open', label: t('Proyectos'), enabled: !!session },
    { id: 'inventory', icon: 'table_view', label: t('Inventario'), enabled: !!inventory },
    { id: 'compare', icon: 'compare_arrows', label: t('Comparar'), enabled: true },
    { id: 'history', icon: 'history', label: t('Historial'), enabled: true },
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
          <span className="brand-sub">{t('Inventario de Compute Engine')}</span>
        </div>
        <div className="spacer" />
        {updateAvailable && (
          <button className="assist" style={{ border: 0, cursor: 'pointer', background: 'var(--md-primary-container)', color: 'var(--md-on-primary-container)' }}
            onClick={() => setPage('about')} title={t('Ver novedades y descargar')}>
            <span className="avatar" style={{ background: 'var(--md-primary)' }}><Icon name="arrow_upward" size="sm" /></span>
            {t('Versión {0} disponible', update.info!.latest)}
          </button>
        )}
        {api.mock && <span className="badge sev-Media" title={t('Ejecutándose fuera de Wails con datos simulados')}>{t('Modo demo')}</span>}
        <Menu trigger={open => (
          <button className="icon-btn" onClick={open} aria-label={t('Tema')} title={t('Tema')}><Icon name={themeIcon} /></button>
        )}>
          {close => (['system', 'light', 'dark'] as Theme[]).map(th => (
            <button key={th} onClick={() => { setTheme(th); close() }}>
              <Icon name={th === 'dark' ? 'dark_mode' : th === 'light' ? 'light_mode' : 'contrast'} />
              <span>{{ system: t('Según el sistema'), light: t('Claro'), dark: t('Oscuro') }[th]}</span>
              {theme === th && <><span className="spacer" /><Icon name="check" size="sm" /></>}
            </button>
          ))}
        </Menu>
        <Menu trigger={open => (
          <button className="icon-btn" onClick={open} aria-label="Idioma / Language" title="Idioma / Language"><Icon name="translate" /></button>
        )}>
          {close => (['es', 'en'] as Lang[]).map(l => (
            <button key={l} onClick={() => { close(); if (l !== lang) setLang(l) }}>
              <span style={{ width: 24, fontWeight: 600 }}>{l.toUpperCase()}</span>
              <span>{l === 'es' ? 'Español' : 'English'}</span>{/* i18n-ignore: native language names */}
              {lang === l && <><span className="spacer" /><Icon name="check" size="sm" /></>}
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
                <div className="sub">{{ adc: t('Credenciales de gcloud (ADC)'), sa: t('Clave de service account'), impersonate: t('Impersonación') }[session.method] ?? session.method}</div>
              </div>
              <hr />
              <button onClick={() => { close(); disconnect() }}><Icon name="logout" /><span>{t('Desconectar')}</span></button>
            </>}
          </Menu>
        )}
      </header>

      <nav className="rail" aria-label={t('Navegación')}>
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
            {updateAvailable && <span className="rail-dot" aria-label={t('Actualización disponible')} />}
          </span>
          <span className="rail-label">{t('Acerca de')}</span>
        </button>
      </nav>

      <main className="main" key={lang}>
        {page === 'connect' && <ConnectPage session={session} notify={notify}
          onConnected={s => { setSession(s); setInventory(null); setPage('projects') }} />}
        {page === 'projects' && session && <ProjectsPage session={session} scan={scan} notify={notify}
          onScanStart={() => setScan({ running: true, progress: null, log: [] })} />}
        {page === 'inventory' && inventory && <InventoryPage inventory={inventory} notify={notify} />}
        {page === 'compare' && <ComparePage state={compare} setState={setCompare} hasInventory={!!inventory} notify={notify} />}
        {page === 'history' && <HistoryPage notify={notify} onOpen={openHistory} onCompare={compareHistory} />}
        {page === 'about' && <AboutPage about={about} update={update} onCheck={() => checkUpdate(false)}
          autoCheck={autoCheck} setAutoCheck={setAutoCheck} />}
      </main>

      <Snackbar message={snack} onClose={() => setSnack(null)} />
    </div>
  )
}
