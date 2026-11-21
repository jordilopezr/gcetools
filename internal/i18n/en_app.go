package i18n

// English for the app and CLI side: app.go, main.go, internal/cli,
// internal/auth, internal/profile, internal/history and internal/update.

func init() {
	add(map[string]string{
		// ---------- CLI: help and usage blocks ----------
		`gceTools — inventario de Google Compute Engine

Comandos:
  scan      escanea proyectos y exporta el inventario (gcetools scan --help)
  diff      compara dos snapshots JSON (gcetools diff --help)
  profiles  lista los perfiles de escaneo guardados
  rules     lista las reglas de gHealth y cómo exceptuar recursos
  login     inicia sesión con Google sin gcloud (la sesión queda en el llavero)
  logout    cierra la sesión de Google guardada en el llavero
  version   muestra la versión y si hay una más reciente

Opción global:
  --lang es|en  idioma de los mensajes y de los archivos generados
                (también GCETOOLS_LANG; por defecto, el idioma del sistema)
`: `gceTools — Google Compute Engine inventory

Commands:
  scan      scan projects and export the inventory (gcetools scan --help)
  diff      compare two JSON snapshots (gcetools diff --help)
  profiles  list the saved scan profiles
  rules     list the gHealth rules and how to exempt resources
  login     sign in with Google without gcloud (the session stays in the keyring)
  logout    sign out of the Google session stored in the keyring
  version   show the version and whether a newer one exists

Global option:
  --lang es|en  language of messages and generated files
                (also GCETOOLS_LANG; default: the system language)
`,

		`gcetools scan — inventario de Compute Engine sin interfaz gráfica

Uso:
  gcetools scan [opciones]

Ejemplos:
  gcetools scan --out inventario.xlsx
  gcetools scan --auth sa --key ./sa.json --query "parent:folders/123456" --out inv.zip
  gcetools scan --auth impersonate --impersonate inventory@prj.iam.gserviceaccount.com --projects a,b
  gcetools scan --discover organizations/123456 --out inventario.json
  gcetools scan --discover organizations/123456 --fast --out inventario.xlsx
  gcetools scan --profile produccion --out inventario.xlsx
  gcetools scan --save-history --report informe.html --out inventario.xlsx
  gcetools scan --billing-table finops-prj.billing.gcp_billing_export_resource_v1_XXXX --out inventario.xlsx

Con --profile se usan los valores guardados en la app (Proyectos → Perfil);
las opciones que pases explícitamente tienen prioridad.
Idioma de los mensajes y del inventario: --lang es|en (o GCETOOLS_LANG).

Opciones:
`: `gcetools scan — headless Compute Engine inventory

Usage:
  gcetools scan [options]

Examples:
  gcetools scan --out inventario.xlsx
  gcetools scan --auth sa --key ./sa.json --query "parent:folders/123456" --out inv.zip
  gcetools scan --auth impersonate --impersonate inventory@prj.iam.gserviceaccount.com --projects a,b
  gcetools scan --discover organizations/123456 --out inventario.json
  gcetools scan --discover organizations/123456 --fast --out inventario.xlsx
  gcetools scan --profile produccion --out inventario.xlsx
  gcetools scan --save-history --report informe.html --out inventario.xlsx
  gcetools scan --billing-table finops-prj.billing.gcp_billing_export_resource_v1_XXXX --out inventario.xlsx

--profile uses the values saved in the app (Projects → Profile);
options you pass explicitly take precedence.
Language of messages and inventory: --lang es|en (or GCETOOLS_LANG).

Options:
`,

		`gcetools diff — compara dos snapshots JSON de gceTools

Uso:
  gcetools diff [opciones] anterior.json nuevo.json

Genera un informe con las pestañas Resumen, Altas, Bajas, Cambios y Metadatos.
Los cambios relevantes (VMs redimensionadas, reglas abiertas a Internet, discos
que quedan huérfanos, cambios de service account…) aparecen primero.

Ejemplos:
  gcetools diff lunes.json viernes.json
  gcetools diff --out cambios.xlsx --fail-on-relevant base.json hoy.json

Códigos de salida: 0 correcto · 1 error · 3 hay cambios relevantes (con --fail-on-relevant)

Opciones:
`: `gcetools diff — compare two gceTools JSON snapshots

Usage:
  gcetools diff [options] before.json after.json

Writes a report with the Summary, Added, Removed, Changes and Metadata tabs.
Relevant changes (resized VMs, firewall rules open to the internet, disks
left orphaned, service account changes…) come first.

Examples:
  gcetools diff lunes.json viernes.json
  gcetools diff --out cambios.xlsx --fail-on-relevant base.json hoy.json

Exit codes: 0 success · 1 error · 3 relevant changes found (with --fail-on-relevant)

Options:
`,

		`gcetools login — inicia sesión con Google sin gcloud

Uso:
  gcetools login [--client client_secret_….json]

Abre el navegador para iniciar sesión con Google. El token se guarda en el
llavero del sistema; después: gcetools scan --auth oauth
Si tu organización bloquea apps externas, usa su propio cliente OAuth
("App de escritorio") con --client (ver docs/manual.md).

Opciones:
`: `gcetools login — sign in with Google without gcloud

Usage:
  gcetools login [--client client_secret_….json]

Opens your browser to sign in with Google. The token is stored in the
system keyring; then: gcetools scan --auth oauth
If your organization blocks third-party apps, use its own OAuth client
("Desktop app") with --client (see docs/manual.en.md).

Options:
`,

		`
Desactivar en un escaneo:   gcetools scan --disable-rules snapshot-schedule,no-labels
Excepción por recurso:      label gcetools-ignore=<id> (o gcetools-ignore-<id>=true, o gcetools-ignore=all)
                            en la VM, el disco, el template o el proyecto
`: `
Disable in a scan:          gcetools scan --disable-rules snapshot-schedule,no-labels
Exempt a resource:          label gcetools-ignore=<id> (or gcetools-ignore-<id>=true, or gcetools-ignore=all)
                            on the VM, disk, template or project
`,

		// ---------- CLI: flag help ----------
		"método: adc | sa | impersonate | oauth (tras gcetools login)":                                            "method: adc | sa | impersonate | oauth (after gcetools login)",
		"ruta a la clave JSON de la service account (--auth sa)":                                                  "path to the service account JSON key (--auth sa)",
		"email de la service account a impersonar (--auth impersonate)":                                           "email of the service account to impersonate (--auth impersonate)",
		"IDs de proyecto separados por comas (por defecto: todos los visibles)":                                   "comma-separated project IDs (default: every visible project)",
		`filtro de Resource Manager, p.ej. "parent:folders/123"`:                                                  `Resource Manager filter, e.g. "parent:folders/123"`,
		"archivo de salida (.xlsx, .zip/.csv, .json)":                                                             "output file (.xlsx, .zip/.csv, .json)",
		"xlsx | csv | json (por defecto según la extensión)":                                                      "xlsx | csv | json (default: from the file extension)",
		"separador CSV (usa ';' para Excel en español)":                                                           "CSV separator (use ';' for Excel in locales with decimal commas)",
		"proyectos en paralelo":                                                                                   "projects scanned in parallel",
		"peticiones por segundo máximas":                                                                          "maximum requests per second",
		"módulos opcionales separados por comas (\"none\" para desactivarlos)":                                    "comma-separated optional modules (\"none\" to disable them)",
		"días de métricas de Cloud Monitoring":                                                                    "days of Cloud Monitoring metrics",
		"moneda de los costes estimados (USD, EUR, CLP…)":                                                         "currency of the estimated costs (USD, EUR, CLP…)",
		"escanea solo proyectos con recursos de Compute según Cloud Asset Inventory (organizations/N, folders/N)": "scan only projects with Compute resources according to Cloud Asset Inventory (organizations/N, folders/N)",
		"perfil guardado (gcetools profiles los lista)":                                                           "saved profile (gcetools profiles lists them)",
		"tabla de exportación de facturación en BigQuery (proyecto.dataset.tabla) para el coste real":             "BigQuery billing export table (project.dataset.table) for the actual cost",
		"mes de facturación AAAAMM (por defecto, el mes anterior)":                                                "billing month YYYYMM (default: the previous month)",
		"no ejecutar la consulta de BigQuery si leería más GB que esto":                                           "skip the BigQuery query if it would read more GB than this",
		"genera además el informe ejecutivo en HTML en esta ruta":                                                 "also write the HTML executive report to this path",
		"guarda también el resultado en el historial local (el mismo que usa la app)":                             "also save the result in the local history (the same one the app uses)",
		"modo rápido: lee VMs, discos, snapshots, imágenes, templates, MIGs y redes de Cloud Asset Inventory (con --discover, una sola lectura para todo el ámbito)": "fast mode: read VMs, disks, snapshots, images, templates, MIGs and networks from Cloud Asset Inventory (with --discover, a single read for the whole scope)",
		"reglas de gHealth a desactivar, separadas por comas (gcetools rules las lista)":                                                                             "comma-separated gHealth rules to disable (gcetools rules lists them)",
		"termina con código 3 si hay cambios relevantes":                                                                                                             "exit with code 3 if there are relevant changes",
		"JSON del cliente OAuth propio de tu organización (opcional; solo si bloquea apps externas)":                                                                 "your organization's own OAuth client JSON (optional; only if it blocks third-party apps)",
		"olvida también el cliente OAuth de la organización":                                                                                                         "also forget the organization's OAuth client",

		// ---------- CLI: output and errors ----------
		"regla desconocida %q (gcetools rules las lista)": "unknown rule %q (gcetools rules lists them)",
		"Perfil: %s\n":                 "Profile: %s\n",
		"no se pudo leer la clave: %v": "could not read the key: %v",
		"Conectado como %s (%s)\n":     "Connected as %s (%s)\n",
		"Cloud Asset Inventory: %d de %d proyectos tienen recursos de Compute\n":                 "Cloud Asset Inventory: %d of %d projects have Compute resources\n",
		"Cloud Asset Inventory: %d proyectos con recursos de Compute en %s\n":                    "Cloud Asset Inventory: %d projects with Compute resources in %s\n",
		"no hay proyectos que escanear":                                                          "no projects to scan",
		"Escaneando %d proyectos…\n":                                                             "Scanning %d projects…\n",
		"módulo desconocido %q (osconfig, monitoring, recommender, pricing)":                     "unknown module %q (osconfig, monitoring, recommender, pricing)",
		"--csv-sep debe ser un único carácter":                                                   "--csv-sep must be a single character",
		"error al exportar: %v":                                                                  "export failed: %v",
		"actual":                                                                                 "current",
		"aviso: no se pudo generar el informe: %v\n":                                             "warning: could not generate the report: %v\n",
		"aviso: no se pudo guardar el informe: %v\n":                                             "warning: could not save the report: %v\n",
		"Informe ejecutivo: %s\n":                                                                "Executive report: %s\n",
		"aviso: no se pudo guardar en el historial: %v\n":                                        "warning: could not save to the history: %v\n",
		"Guardado en el historial: %s\n":                                                         "Saved to the history: %s\n",
		"Inventario: %d VMs, %d errores → %s\n":                                                  "Inventory: %d VMs, %d errors → %s\n",
		"gceTools %s · licencia %s · %s\n":                                                       "gceTools %s · license %s · %s\n",
		"No se pudo comprobar si hay versiones nuevas: %v\n":                                     "Could not check for new versions: %v\n",
		"Hay una versión nueva: %s → %s\n":                                                       "A new version is available: %s → %s\n",
		"Estás en la última versión.":                                                            "You are on the latest version.",
		"No hay perfiles guardados. Créalos en la app (Proyectos → Guardar perfil) o edita %s\n": "No saved profiles. Create them in the app (Projects → Save profile) or edit %s\n",
		"%d proyectos":                 "%d projects",
		"filtro %s":                    "filter %s",
		"todos los proyectos visibles": "every visible project",
		"descubrimiento CAI en %s":     "CAI discovery in %s",
		"modo rápido":                  "fast mode",
		" [umbral: %s]":                " [threshold: %s]",
		"%d altas, %d bajas, %d recursos modificados (%d cambios relevantes) → %s\n":            "%d added, %d removed, %d resources modified (%d relevant changes) → %s\n",
		"Abre este enlace para iniciar sesión (se intenta abrir solo):\n\n  %s\n\nEsperando…\n": "Open this link to sign in (it should open automatically):\n\n  %s\n\nWaiting…\n",
		"Sesión iniciada como %s. Usa: gcetools scan --auth oauth\n":                            "Signed in as %s. Use: gcetools scan --auth oauth\n",
		"Sesión de Google cerrada.": "Signed out of Google.",

		// ---------- app: dialogs, labels and errors ----------
		"Selecciona la clave JSON de la service account":          "Select the service account JSON key",
		"Clave JSON (*.json)":                                     "JSON key (*.json)",
		"selecciona primero un archivo de clave JSON":             "select a JSON key file first",
		"selecciona al menos un proyecto":                         "select at least one project",
		"ya hay un escaneo en curso":                              "a scan is already running",
		"No se pudo guardar en el historial: %v":                  "Could not save to the history: %v",
		"no hay inventario que exportar":                          "there is no inventory to export",
		"Exportar inventario":                                     "Export inventory",
		"no hay comparación que exportar":                         "there is no comparison to export",
		"Exportar comparación":                                    "Export comparison",
		"formato no soportado":                                    "unsupported format",
		"no hay sesión: conéctate primero":                        "not connected: connect first",
		"Elegir snapshot JSON de gceTools":                        "Choose a gceTools JSON snapshot",
		"Snapshot JSON (*.json)":                                  "JSON snapshot (*.json)",
		"Inventario actual":                                       "Current inventory",
		"no hay inventario actual: escanea primero":               "there is no current inventory: run a scan first",
		"no se pudo leer el escaneo %s del historial: %w":         "could not read scan %s from the history: %w",
		"Historial %s":                                            "History %s",
		"snapshot no cargado: vuelve a elegirlo":                  "snapshot not loaded: choose it again",
		"no hay inventario: escanea o abre uno del historial":     "there is no inventory: run a scan or open one from the history",
		"Guardar informe ejecutivo":                               "Save executive report",
		"gcetools-informe-%s.html":                                "gcetools-report-%s.html",
		"Actual":                                                  "Current",
		"Cliente OAuth de tu organización (client_secret_….json)": "Your organization's OAuth client (client_secret_….json)",
		"Inventario de Compute Engine al estilo RVTools":          "RVTools-style Compute Engine inventory",

		// ---------- auth ----------
		"el archivo no es un JSON válido: %w":                                                        "the file is not valid JSON: %w",
		"tipo de credencial %q no soportado: se esperaba \"service_account\"":                        "unsupported credential type %q: expected \"service_account\"",
		"la clave no contiene client_email o private_key":                                            "the key has no client_email or private_key",
		"no se encontraron credenciales ADC (ejecuta `gcloud auth application-default login`): %w":   "Application Default Credentials (ADC) not found (run `gcloud auth application-default login`): %w",
		"clave de service account inválida: %w":                                                      "invalid service account key: %w",
		"indica el email de la service account a impersonar":                                         "enter the email of the service account to impersonate",
		"la impersonación necesita credenciales ADC: %w":                                             "impersonation needs Application Default Credentials (ADC): %w",
		"no se pudo impersonar %s: %w":                                                               "could not impersonate %s: %w",
		"método de autenticación desconocido: %q":                                                    "unknown authentication method: %q",
		"la sesión de Google ha caducado o se revocó el acceso: vuelve a iniciar sesión en Conexión": "the Google session has expired or access was revoked: sign in again on the Connection page",
		"la sesión de gcloud ha caducado o tu organización exige volver a autenticarse. Ejecuta `gcloud auth application-default login` y vuelve a conectar": "the gcloud session has expired or your organization requires you to reauthenticate. Run `gcloud auth application-default login` and connect again",
		"no se pudo obtener un token: %w":                  "could not get a token: %w",
		"Usuario (ADC)":                                    "User (ADC)",
		"Usuario de gcloud":                                "gcloud user",
		"Cuenta de servicio del entorno (metadata server)": "Environment service account (metadata server)",
		"es un cliente de tipo \"Aplicación web\": crea uno de tipo \"App de escritorio\"":                    "this is a \"Web application\" client: create a \"Desktop app\" one",
		"es una clave de service account: úsala en la opción \"Service account (JSON)\"":                      "this is a service account key: use it with the \"Service account (JSON)\" option",
		"no es un cliente OAuth de Google (falta la sección \"installed\")":                                   "not a Google OAuth client (the \"installed\" section is missing)",
		"llavero del sistema no disponible: %w":                                                               "system keyring unavailable: %w",
		"no se pudo guardar en el llavero del sistema: %w":                                                    "could not save to the system keyring: %w",
		"esta compilación no incluye el cliente OAuth de gceTools: carga el cliente OAuth de tu organización": "this build does not include the gceTools OAuth client: load your organization's OAuth client",
		"estado inválido": "invalid state",
		"se agotó el tiempo de espera del inicio de sesión (5 minutos)": "sign-in timed out (5 minutes)",
		"inicio de sesión cancelado":                                    "sign-in cancelled",
		"se denegó el acceso en la pantalla de consentimiento":          "access was denied on the consent screen",
		"tu organización no permite esta app: pide a tu administrador que la autorice o usa el cliente OAuth de tu organización (opciones avanzadas)": "your organization does not allow this app: ask your administrator to allow it or use your organization's OAuth client (advanced options)",
		"Google devolvió el error %q":                  "Google returned the error %q",
		"no se pudo completar el inicio de sesión: %w": "could not complete the sign-in: %w",
		"Google no devolvió un token de refresco; revoca el acceso de la app en tu cuenta de Google y vuelve a intentarlo": "Google did not return a refresh token; remove the app's access in your Google Account and try again",
		"no has iniciado sesión con Google en gceTools":                                                                    "you have not signed in with Google in gceTools",

		// OAuth loopback page
		"Sesión iniciada": "Signed in",
		"Ya puedes cerrar esta pestaña y volver a gceTools.": "You can close this tab and return to gceTools.",
		"Inicio de sesión no completado":                     "Sign-in not completed",
		"No se completó el inicio de sesión":                 "Sign-in was not completed",
		"Vuelve a gceTools e inténtalo de nuevo.":            "Return to gceTools and try again.",
		"gceTools solo tiene acceso de lectura a Google Cloud. Puedes revocarlo en cualquier momento desde «Cerrar sesión».": "gceTools only has read access to Google Cloud. You can revoke it at any time with “Sign out”.",

		// ---------- profiles, history, update ----------
		"%s está dañado: %w":                                                  "%s is corrupted: %w",
		"no existe el perfil %q (no hay perfiles guardados)":                  "profile %q does not exist (no saved profiles)",
		"no existe el perfil %q (disponibles: %s)":                            "profile %q does not exist (available: %s)",
		"el perfil necesita un nombre":                                        "the profile needs a name",
		"el nombre del perfil es demasiado largo (máximo 60 caracteres)":      "the profile name is too long (60 characters max)",
		"ámbito de descubrimiento no válido %q":                               "invalid discovery scope %q",
		"no se encontró la carpeta de datos del usuario":                      "user data folder not found",
		"identificador de historial no válido: %q":                            "invalid history ID: %q",
		"no se pudo consultar GitHub: %w":                                     "could not reach GitHub: %w",
		"no hay releases publicadas todavía (o el repositorio no es público)": "no releases published yet (or the repository is not public)",
		"GitHub limitó las consultas; inténtalo más tarde":                    "GitHub rate-limited the request; try again later",
		"GitHub respondió %s":                                                 "GitHub responded %s",
	})
}
