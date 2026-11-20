import { useEffect, useState } from 'react'
import { api, errorText, type ADCStatus, type AuthMethod, type KeyInfo, type OAuthStatus, type SessionInfo } from '../lib/api'
import { Icon } from '../components/ui'

interface Props {
  session: SessionInfo | null
  onConnected: (s: SessionInfo) => void
  notify: (m: string) => void
}

const methods: { id: AuthMethod; icon: string; title: string; text: string }[] = [
  { id: 'oauth', icon: 'account_circle', title: 'Iniciar sesión con Google', text: 'Sin instalar gcloud: inicias sesión en el navegador y la sesión queda guardada en el llavero del sistema.' },
  { id: 'adc', icon: 'terminal', title: 'Credenciales de gcloud', text: 'Usa las credenciales de gcloud (Application Default Credentials). Verás los proyectos a los que tienes acceso.' },
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
  const [oauth, setOauth] = useState<OAuthStatus | null>(null)
  const [signingIn, setSigningIn] = useState(false)

  useEffect(() => {
    api.detectADC().then(setAdc).catch(() => setAdc({ found: false, identity: '' }))
    api.oauthStatus().then(o => { setOauth(o); if (o.account) setMethod('oauth') }).catch(() => setOauth(null))
  }, [])

  const pickClient = async () => {
    try { setOauth(await api.oauthPickClient()); setError(null) } catch (e) { setError(errorText(e)) }
  }
  const signIn = async () => {
    setSigningIn(true); setError(null)
    try {
      const o = await api.oauthLogin()
      setOauth(o)
      if (o.account) await connectWith('oauth')
    } catch (e) { setError(errorText(e)); setOauth(await api.oauthStatus()) } finally { setSigningIn(false) }
  }
  const useBuiltin = async () => {
    try { setOauth(await api.oauthUseBuiltin()) } catch (e) { setError(errorText(e)) }
  }
  const signOut = async (all: boolean) => {
    try { setOauth(await api.oauthLogout(all)) } catch (e) { setError(errorText(e)) }
  }

  const pickKey = async () => {
    try {
      const k = await api.pickKey()
      if (k) { setKey(k); setError(null) }
    } catch (e) { setError(errorText(e)) }
  }

  const canConnect = method === 'adc' ? !!adc?.found : method === 'sa' ? !!key : method === 'oauth' ? !!oauth?.account : target.includes('@')

  const connect = () => connectWith(method)
  const connectWith = async (m: AuthMethod) => {
    setBusy(true); setError(null)
    try {
      const s = await api.connect(m, target)
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

      <div className="method-grid" role="radiogroup" aria-label="Método de autenticación" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))' }}>
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
        {method === 'oauth' && (oauth?.error ? (
          <div className="col" style={{ gap: 6 }}>
            <div className="row"><span className="badge sev-Alta">Llavero no disponible</span></div>
            <span className="body-m selectable">{oauth.error}</span>
            <span className="body-s muted">gceTools solo guarda la sesión en el llavero del sistema (en Linux, un servicio Secret Service como GNOME Keyring o KWallet). Usa otro método mientras tanto.</span>
          </div>
        ) : (
          <div className="col" style={{ gap: 12 }}>
            {!oauth?.client ? (
              <span className="body-m">
                Esta compilación no incluye el cliente OAuth de gceTools. Carga el <b>cliente OAuth de tu organización</b> (tipo "App de escritorio", archivo <span className="mono">client_secret_….json</span>).
              </span>
            ) : oauth.account ? (
              <div className="row" style={{ gap: 10, flexWrap: 'wrap' }}>
                <span className="badge ok">Sesión guardada</span>
                <span className="title-s selectable">{oauth.account}</span>
                <span className="spacer" />
                <button className="btn text" onClick={() => signOut(false)}><Icon name="logout" />Cerrar sesión</button>
              </div>
            ) : signingIn ? (
              <div className="row" style={{ gap: 10 }}>
                <Icon name="progress_activity" />
                <span className="body-m">Completa el inicio de sesión en el navegador que se ha abierto…</span>
                <button className="btn text" onClick={() => api.oauthCancel()}>Cancelar</button>
              </div>
            ) : (
              <div className="row" style={{ gap: 12, flexWrap: 'wrap' }}>
                <button className="btn filled" onClick={signIn}><Icon name="account_circle" />Iniciar sesión con Google</button>
                <span className="body-s muted">Se abrirá el navegador. gceTools solo pide permiso de <b>lectura</b> sobre Google Cloud.</span>
              </div>
            )}
            <details open={!oauth?.builtin || oauth?.custom}>
              <summary className="body-s muted" style={{ cursor: 'pointer' }}>
                Opciones avanzadas{oauth?.custom ? ` · usando el cliente OAuth de ${oauth.projectId || 'tu organización'}` : ''}
              </summary>
              <div className="col" style={{ gap: 8, marginTop: 8 }}>
                <span className="body-s muted">
                  Si tu organización no permite apps externas, un administrador puede crear un cliente OAuth propio (tipo "App de escritorio", consentimiento <b>Interno</b>) y repartir su JSON.
                </span>
                <div className="row" style={{ gap: 8, flexWrap: 'wrap' }}>
                  <button className="btn tonal" onClick={pickClient}><Icon name="upload_file" />{oauth?.custom ? 'Cambiar cliente de la organización' : 'Usar el cliente OAuth de mi organización'}</button>
                  {oauth?.custom && oauth.builtin && <button className="btn text" onClick={useBuiltin}>Volver al cliente de gceTools</button>}
                  {oauth?.custom && !oauth.builtin && <button className="btn text" onClick={() => signOut(true)}>Olvidar cliente</button>}
                  <button className="btn text" onClick={() => api.openURL('https://github.com/jordilopezr/gcetools#iniciar-sesi%C3%B3n-con-google')}><Icon name="help" />Ayuda</button>
                </div>
              </div>
            </details>
          </div>
        ))}

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
