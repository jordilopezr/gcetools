import { useEffect, useState } from 'react'
import { api, errorText, type ADCStatus, type AuthMethod, type KeyInfo, type SessionInfo } from '../lib/api'
import { Icon } from '../components/ui'

interface Props {
  session: SessionInfo | null
  onConnected: (s: SessionInfo) => void
  notify: (m: string) => void
}

const methods: { id: AuthMethod; icon: string; title: string; text: string }[] = [
  { id: 'adc', icon: 'person', title: 'Mi cuenta de Google', text: 'Usa las credenciales de gcloud (Application Default Credentials). Verás los proyectos a los que tienes acceso.' },
  { id: 'sa', icon: 'key', title: 'Service account (JSON)', text: 'Carga la clave JSON de una service account. La clave solo se mantiene en memoria.' },
  { id: 'impersonate', icon: 'badge', title: 'Impersonar service account', text: 'Tu cuenta obtiene tokens de corta duración de una SA. No requiere claves; ideal si la organización las bloquea.' },
]

export default function ConnectPage({ session, onConnected, notify }: Props) {
  const [method, setMethod] = useState<AuthMethod>('adc')
  const [adc, setAdc] = useState<ADCStatus | null>(null)
  const [key, setKey] = useState<KeyInfo | null>(null)
  const [target, setTarget] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => { api.detectADC().then(setAdc).catch(() => setAdc({ found: false, identity: '' })) }, [])

  const pickKey = async () => {
    try {
      const k = await api.pickKey()
      if (k) { setKey(k); setError(null) }
    } catch (e) { setError(errorText(e)) }
  }

  const canConnect = method === 'adc' ? !!adc?.found : method === 'sa' ? !!key : target.includes('@')

  const connect = async () => {
    setBusy(true); setError(null)
    try {
      const s = await api.connect(method, target)
      notify(`Conectado como ${s.identity}`)
      onConnected(s)
    } catch (e) { setError(errorText(e)) } finally { setBusy(false) }
  }

  return (
    <div className="page" style={{ maxWidth: 1080 }}>
      <div className="col" style={{ gap: 4 }}>
        <h1 className="headline-s">Conectar con Google Cloud</h1>
        <p className="body-m muted" style={{ margin: 0 }}>
          Elige con qué identidad leer el inventario. El alcance depende de los permisos IAM de esa identidad.
        </p>
      </div>

      {session && (
        <div className="banner info">
          <Icon name="check_circle" className="" />
          <div>Sesión activa como <strong className="selectable">{session.identity}</strong>. Conectar de nuevo la reemplaza.</div>
        </div>
      )}

      <div className="method-grid" role="radiogroup" aria-label="Método de autenticación">
        {methods.map(m => (
          <button key={m.id} role="radio" aria-checked={method === m.id}
            className={`method ${method === m.id ? 'selected' : ''}`} onClick={() => setMethod(m.id)}>
            <div className="row" style={{ justifyContent: 'space-between' }}>
              <span className="method-icon"><Icon name={m.icon} /></span>
              <Icon name={method === m.id ? 'radio_button_checked' : 'radio_button_unchecked'}
                className={method === m.id ? '' : 'muted'} />
            </div>
            <span className="title-m">{m.title}</span>
            <span className="body-m muted">{m.text}</span>
          </button>
        ))}
      </div>

      <div className="card outlined col">
        {method === 'adc' && (adc === null ? (
          <div className="row muted"><Icon name="hourglass_empty" /> Buscando credenciales…</div>
        ) : adc.found ? (
          <div className="row"><span className="badge ok">Detectadas</span><span className="selectable">{adc.identity}</span></div>
        ) : (
          <div className="col" style={{ gap: 8 }}>
            <div className="row"><span className="badge sev-Alta">No encontradas</span></div>
            <span className="body-m">Inicia sesión una vez con Google Cloud CLI y vuelve a abrir esta pantalla:</span>
            <code className="inline selectable" style={{ alignSelf: 'flex-start' }}>gcloud auth application-default login</code>
          </div>
        ))}

        {method === 'sa' && (
          <div className="row">
            <button className="btn tonal" onClick={pickKey}><Icon name="upload_file" />Seleccionar archivo JSON</button>
            {key ? (
              <div className="col" style={{ gap: 0 }}>
                <span className="title-s selectable">{key.clientEmail}</span>
                <span className="body-s muted">Proyecto de la SA: {key.projectId} · key id {key.keyId.slice(0, 8)}…</span>
              </div>
            ) : <span className="muted">Ningún archivo seleccionado</span>}
          </div>
        )}

        {method === 'impersonate' && (
          <div className="col" style={{ gap: 8 }}>
            <div className="field" style={{ maxWidth: 560 }}>
              <input id="sa" value={target} onChange={e => setTarget(e.target.value)} placeholder=" " spellCheck={false}
                onKeyDown={e => { if (e.key === 'Enter' && canConnect) connect() }} />
              <label htmlFor="sa">Email de la service account</label>
              <span className="support">Tu cuenta necesita roles/iam.serviceAccountTokenCreator sobre ella.</span>
            </div>
            {!adc?.found && <span className="body-s" style={{ color: 'var(--md-error)' }}>La impersonación requiere credenciales ADC (gcloud auth application-default login).</span>}
          </div>
        )}
      </div>

      {error && <div className="banner error"><Icon name="error" /><div className="selectable">{error}</div></div>}

      <div className="row">
        <button className="btn filled" disabled={!canConnect || busy} onClick={connect}>
          <Icon name={busy ? 'progress_activity' : 'login'} />{busy ? 'Conectando…' : 'Conectar'}
        </button>
        <span className="body-s muted row" style={{ gap: 6 }}>
          <Icon name="lock" size="sm" /> Solo lectura: se solicita únicamente el scope <code className="inline">cloud-platform.read-only</code>
        </span>
      </div>

      <details className="card filled">
        <summary className="title-s" style={{ cursor: 'pointer' }}>Permisos IAM recomendados</summary>
        <ul className="body-m" style={{ margin: '12px 0 0', paddingLeft: 20, lineHeight: '24px' }}>
          <li><code className="inline">roles/compute.viewer</code> en organización, carpeta o proyecto: inventario de Compute.</li>
          <li><code className="inline">roles/browser</code>: descubrir proyectos y su jerarquía.</li>
          <li>Opcionales (fase 2): <code className="inline">roles/osconfig.inventoryViewer</code>, <code className="inline">roles/monitoring.viewer</code>, <code className="inline">roles/recommender.computeViewer</code>, <code className="inline">roles/cloudasset.viewer</code>.</li>
        </ul>
      </details>
    </div>
  )
}
