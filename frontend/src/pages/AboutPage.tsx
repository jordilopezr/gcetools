import { api, type AboutInfo, type UpdateInfo } from '../lib/api'
import { Icon } from '../components/ui'
import appIcon from '../assets/gcetools-icon.svg'

export interface UpdateState {
  status: 'idle' | 'checking' | 'done' | 'error'
  info?: UpdateInfo
  error?: string
}

interface Props {
  about: AboutInfo | null
  update: UpdateState
  onCheck: () => void
  autoCheck: boolean
  setAutoCheck: (v: boolean) => void
}

const credits: [string, string, string][] = [
  ['Wails', 'MIT', 'https://wails.io'],
  ['AG Grid Community', 'MIT', 'https://www.ag-grid.com'],
  ['React', 'MIT', 'https://react.dev'],
  ['excelize', 'BSD-3-Clause', 'https://github.com/qax-os/excelize'],
  ['Google API Go Client', 'BSD-3-Clause', 'https://github.com/googleapis/google-api-go-client'],
  ['Roboto Flex', 'OFL-1.1', 'https://fonts.google.com/specimen/Roboto+Flex'],
  ['Material Symbols', 'Apache-2.0', 'https://fonts.google.com/icons'],
]

const apiCosts: [string, string, string][] = [
  ['Compute Engine', 'VMs, discos, red, snapshots…', 'Gratis (solo cuotas por minuto)'],
  ['Resource Manager', 'Listar proyectos', 'Gratis'],
  ['Cloud Billing Catalog', 'Coste estimado (caché de 24 h)', 'Gratis'],
  ['Recommender', 'Recomendaciones', 'Gratis'],
  ['Cloud Asset Inventory', 'Descubrir proyectos con Compute', 'Gratis'],
  ['Cloud Monitoring', 'Métricas de CPU y RAM', '0,50 USD por millón de series; 1 millón gratis al mes'],
  ['OS Config', 'SO y agentes', 'Leer es gratis (VM Manager cobra por agente activo, no por gceTools)'],
  ['BigQuery', 'Coste real (opcional, desactivado por defecto)', 'Por datos leídos (1 TiB gratis al mes); gceTools estima antes y respeta un límite de GB'],
  ['GitHub', 'Buscar actualizaciones', 'Gratis'],
]

function fmtDate(iso?: string) {
  if (!iso) return ''
  const d = new Date(iso)
  return isNaN(d.getTime()) ? '' : d.toLocaleDateString('es-ES', { year: 'numeric', month: 'long', day: 'numeric' })
}

export default function AboutPage({ about, update, onCheck, autoCheck, setAutoCheck }: Props) {
  const host = (u?: string) => (u ? u.replace(/^https?:\/\//, '') : '')
  const u = update.info

  return (
    <div className="page" style={{ maxWidth: 880 }}>
      <div className="row" style={{ gap: 20 }}>
        <img src={appIcon} alt="Logotipo de gceTools" width={88} height={88} style={{ flexShrink: 0 }} />
        <div className="col" style={{ gap: 2 }}>
          <h1 className="headline-s">gceTools</h1>
          <span className="body-m muted">Inventario de Google Compute Engine al estilo RVTools</span>
          <span className="row" style={{ gap: 8, marginTop: 6 }}>
            <span className="badge ok">Versión {about?.version ?? '…'}</span>
            <span className="badge sev-Baja">Licencia {about?.license ?? 'GPL-3.0'}</span>
          </span>
        </div>
      </div>

      <div className="method-grid">
        <button className="method" onClick={() => about && api.openURL(about.website)}>
          <span className="method-icon"><Icon name="language" /></span>
          <span className="title-m">Sitio web</span>
          <span className="body-m muted">Documentación, novedades y descargas</span>
          <span className="label-l" style={{ color: 'var(--md-primary)' }}>{host(about?.website)} ↗</span>
        </button>
        <button className="method" onClick={() => about && api.openURL(about.donate)}>
          <span className="method-icon" style={{ background: 'var(--md-warning-container)', color: 'var(--md-on-warning-container)' }}>
            <Icon name="local_cafe" />
          </span>
          <span className="title-m">Invítame a un café</span>
          <span className="body-m muted">gceTools es gratuito y de código abierto. Si te ahorra tiempo, puedes apoyar su desarrollo.</span>
          <span className="label-l" style={{ color: 'var(--md-primary)' }}>{host(about?.donate)} ↗</span>
        </button>
        <button className="method" onClick={() => about && api.openURL(about.licenseUrl)}>
          <span className="method-icon"><Icon name="balance" /></span>
          <span className="title-m">Licencia GPL-3.0</span>
          <span className="body-m muted">Software libre: puedes usarlo, estudiarlo, modificarlo y redistribuirlo bajo los términos de la GNU GPL v3.</span>
          <span className="label-l" style={{ color: 'var(--md-primary)' }}>gnu.org/licenses/gpl-3.0 ↗</span>
        </button>
      </div>

      <div className="card outlined col">
        <div className="row">
          <Icon name="system_update" />
          <span className="title-m">Actualizaciones</span>
          <span className="spacer" />
          <button className="btn outlined" onClick={onCheck} disabled={update.status === 'checking'}>
            <Icon name="refresh" />{update.status === 'checking' ? 'Comprobando…' : 'Buscar actualizaciones'}
          </button>
        </div>

        {update.status === 'done' && u?.available && (
          <div className="banner info" style={{ background: 'var(--md-primary-container)', color: 'var(--md-on-primary-container)' }}>
            <Icon name="new_releases" filled />
            <div className="col" style={{ gap: 6, flex: 1 }}>
              <span className="title-s">Hay una versión nueva: {u.latest} (tienes {u.current})</span>
              {u.published && <span className="body-s">Publicada el {fmtDate(u.published)}</span>}
              {u.notes && <pre className="body-s selectable" style={{ whiteSpace: 'pre-wrap', margin: 0, fontFamily: 'inherit', maxHeight: 160, overflow: 'auto' }}>{u.notes}</pre>}
            </div>
            <button className="btn filled" onClick={() => api.openURL(u.url)}><Icon name="download" />Descargar</button>
          </div>
        )}
        {update.status === 'done' && u && !u.available && (
          <div className="row body-m"><Icon name="check_circle" /> Tienes la última versión ({u.current}).</div>
        )}
        {update.status === 'error' && (
          <div className="row body-m muted"><Icon name="cloud_off" /> No se pudo comprobar: {update.error}</div>
        )}

        <label className="row body-m" style={{ gap: 10, cursor: 'pointer' }}>
          <input type="checkbox" checked={autoCheck} onChange={e => setAutoCheck(e.target.checked)}
            style={{ width: 18, height: 18, accentColor: 'var(--md-primary)' }} />
          Comprobar automáticamente al iniciar
        </label>
        <span className="body-s muted">
          La comprobación consulta la última release publicada en GitHub. No envía ningún dato tuyo ni de tu inventario.
        </span>
      </div>

      <details className="card filled">
        <summary className="title-s" style={{ cursor: 'pointer' }}>Coste de uso de las APIs de Google Cloud</summary>
        <p className="body-m" style={{ margin: '12px 0 8px' }}>
          gceTools solo hace lecturas. Casi todas las APIs son gratuitas; pueden facturar Cloud Monitoring, con un tramo gratuito que en la práctica no se agota, y BigQuery si activas el coste real.
        </p>
        <table className="body-m" style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr className="label-l muted" style={{ textAlign: 'left' }}>
              <th style={{ padding: '6px 8px 6px 0' }}>API</th><th style={{ padding: '6px 8px' }}>Para qué</th><th style={{ padding: '6px 0 6px 8px' }}>Coste</th>
            </tr>
          </thead>
          <tbody>
            {apiCosts.map(([apiName, use, cost]) => (
              <tr key={apiName} style={{ borderTop: '1px solid var(--md-outline-variant)', verticalAlign: 'top' }}>
                <td style={{ padding: '6px 8px 6px 0', whiteSpace: 'nowrap' }}>{apiName}</td>
                <td style={{ padding: '6px 8px' }} className="muted">{use}</td>
                <td style={{ padding: '6px 0 6px 8px' }}>{cost}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <p className="body-s muted" style={{ margin: '10px 0 0' }}>
          Un escaneo pide unas 2 series de Monitoring por VM: con 100 VMs harían falta unos 5.000 escaneos al mes para salir del tramo gratuito.
          Con credenciales de usuario, lo facturable va al quota project de ADC. Precios de noviembre de 2026; consulta la{' '}
          <a href="https://cloud.google.com/monitoring/pricing" onClick={e => { e.preventDefault(); api.openURL('https://cloud.google.com/monitoring/pricing') }} style={{ color: 'var(--md-primary)' }}>página de precios de Monitoring</a>.
        </p>
      </details>

      <details className="card filled">
        <summary className="title-s" style={{ cursor: 'pointer' }}>Componentes de terceros</summary>
        <ul className="body-m" style={{ margin: '12px 0 0', paddingLeft: 20, lineHeight: '26px' }}>
          {credits.map(([name, lic, url]) => (
            <li key={name}>
              <a href={url} onClick={e => { e.preventDefault(); api.openURL(url) }} style={{ color: 'var(--md-primary)' }}>{name}</a>
              <span className="muted"> · {lic}</span>
            </li>
          ))}
        </ul>
      </details>
    </div>
  )
}
