# Registro del cliente OAuth de gceTools (mantenedor)

Guía para registrar gceTools en Google y que "Iniciar sesión con Google" funcione en las releases sin que cada usuario cree su propio cliente. Se hace una sola vez.

## 1. Proyecto

Crea un proyecto dedicado (p. ej. `gcetools-app`) en una cuenta propia, **fuera de cualquier organización de cliente**. Así el consentimiento puede ser de tipo *Externo* y el proyecto no depende de ningún tercero.

Con "Iniciar sesión con Google", Google carga la cuota y comprueba la habilitación de las APIs en el **proyecto del cliente OAuth**, no en los proyectos que se escanean. Por eso hay que habilitar en él todas las APIs que usa gceTools:

```bash
gcloud services enable --project gcetools-app \
  cloudresourcemanager.googleapis.com compute.googleapis.com monitoring.googleapis.com \
  osconfig.googleapis.com recommender.googleapis.com cloudasset.googleapis.com \
  cloudbilling.googleapis.com bigquery.googleapis.com
```

Ninguna tiene coste por estar habilitada. Las cuotas por minuto de ese proyecto se comparten entre todos los usuarios de la app.

## 2. Pantalla de consentimiento (Google Auth Platform)

*APIs y servicios → Pantalla de consentimiento de OAuth* (o *Google Auth Platform*):

| Campo | Valor |
|---|---|
| Tipo de usuario | **Externo** |
| Nombre de la app | gceTools |
| Logo | `assets/brand/png/gcetools-icon-128.png` (120×120 recomendado) |
| Email de asistencia | el tuyo |
| Página principal | https://gcetools.jordilopezr.com |
| Política de privacidad | https://gcetools.jordilopezr.com/privacidad (publica ahí [PRIVACY.md](../PRIVACY.md)) |
| Condiciones del servicio | opcional |
| Dominios autorizados | `jordilopezr.com` (hay que verificarlo en Google Search Console) |
| Scopes | `openid`, `.../auth/userinfo.email`, `.../auth/cloud-platform.read-only`, `.../auth/compute.readonly`, `.../auth/monitoring.read`, `.../auth/cloud-billing.readonly` |

## 3. Cliente OAuth

*Credenciales → Crear credenciales → ID de cliente de OAuth → App de escritorio*, con el nombre "gceTools desktop". Guarda el **ID de cliente** y el **secreto**.

En los clientes de escritorio Google trata el secreto como **público**: va dentro del binario y la seguridad la aporta PKCE. No lo subas al repositorio de todas formas, para que el escaneo de secretos no lo marque. Se inyecta en la compilación.

## 4. Secrets en GitHub

En *Settings → Secrets and variables → Actions* del repositorio:

| Tipo | Nombre | Valor |
|---|---|---|
| Secret | `GCETOOLS_OAUTH_CLIENT_ID` | ID de cliente (`….apps.googleusercontent.com`) |
| Secret | `GCETOOLS_OAUTH_CLIENT_SECRET` | secreto del cliente (`GOCSPX-…`) |
| Variable | `GCETOOLS_OAUTH_PROJECT` | ID del proyecto (p. ej. `gcetools-app`) |

El workflow los inyecta con `-ldflags` en la app y en `gcetools-cli`. Sin ellos, la build no incluye el cliente y solo funciona el cliente propio de cada organización.

Para probar en local antes de una release:

```bash
wails build -tags webkit2_41 -ldflags "-X gcetools/internal/auth.BuiltinClientID=<id> -X gcetools/internal/auth.BuiltinClientSecret=<secreto> -X gcetools/internal/auth.BuiltinClientProject=<proyecto>"
```

## 5. Modo de prueba y verificación

Mientras la app esté en **Prueba**:

- Solo pueden iniciar sesión los usuarios de prueba añadidos en la consola (hasta 100).
- Los tokens de refresco **caducan a los 7 días**, así que hay que volver a iniciar sesión cada semana.

Para pasarla a **Producción** sin esos límites, `cloud-platform.read-only` y `compute.readonly` son scopes **sensibles** y Google exige verificarla. No hace falta la evaluación de seguridad (CASA), que solo se pide para scopes restringidos. Para la verificación necesitas:

1. El dominio verificado en Search Console y la política de privacidad publicada en él.
2. La **justificación del scope**, por ejemplo:

   > gceTools es una aplicación de escritorio de código abierto que genera un inventario de solo lectura de los recursos de Google Compute Engine del usuario (máquinas virtuales, discos, redes, snapshots, métricas y costes) y lo exporta a Excel/CSV, al estilo de RVTools en VMware. Pide solo scopes de lectura: compute.readonly para leer Compute Engine, monitoring.read para las métricas de CPU y memoria, cloud-billing.readonly para el catálogo de precios y cloud-platform.read-only para Resource Manager y BigQuery (exportación de facturación). Compute Engine, Monitoring y Cloud Billing no aceptan cloud-platform.read-only, por eso se piden por separado. La app nunca modifica recursos y procesa los datos solo en el equipo del usuario, sin servidores propios.

3. Un **vídeo de demostración** (YouTube, puede estar como "no listado") que muestre:
   - abrir gceTools y pulsar "Iniciar sesión con Google";
   - la pantalla de consentimiento completa, con el nombre de la app y el scope visibles en la barra de direcciones (`client_id`);
   - el uso de los datos: lista de proyectos, escaneo, pestañas del inventario y exportación;
   - "Cerrar sesión".
4. Responder a los correos del equipo de verificación de Google. Suele tardar entre días y algunas semanas.

## 6. Organizaciones que bloquean apps externas

Algunas organizaciones de Google Workspace o Cloud Identity bloquean las apps no configuradas que piden scopes de Google Cloud. Sus administradores pueden autorizar gceTools por su ID de cliente en *Consola de administración → Seguridad → Acceso y control de datos → Controles de API*. Si no, cada organización puede usar su propio cliente interno: ver la sección "Iniciar sesión con Google" del README.
