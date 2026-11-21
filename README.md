# gceTools

[![build](https://github.com/jordilopezr/gcetools/actions/workflows/build.yml/badge.svg)](https://github.com/jordilopezr/gcetools/actions/workflows/build.yml) · Licencia GPL-3.0 · [gcetools.jordilopezr.com](https://gcetools.jordilopezr.com) · **[English](README.en.md)**

Inventario de Google Compute Engine al estilo RVTools: aplicación de escritorio para Windows, Linux y macOS, más un modo de línea de comandos.
Lee todos los proyectos a los que llega la identidad y exporta a XLSX (una hoja por pestaña), CSV (ZIP) o JSON.

- **Solo lectura:** pide únicamente el scope `cloud-platform.read-only`.
- **Autenticación:** inicio de sesión con Google sin gcloud (sesión guardada en el llavero del sistema), credenciales de gcloud (ADC), clave JSON de service account (solo en memoria) o impersonación de service account.
- **Idiomas:** español e inglés (interfaz, inventario, informe y CLI).
- **Stack:** Go + [Wails v2](https://wails.io) + React/TypeScript, con interfaz Material 3 y AG Grid Community.

## Pestañas (MVP)

| Pestaña | RVTools | Contenido |
|---|---|---|
| gInfo | vInfo | Una fila por VM: machine type, vCPU, RAM, estado, IPs, SO, discos, SA, labels, Shielded VM, MIG, servicio que la gestiona (GKE, Dataproc…) |
| gCPU / gMemory | vCPU / vMemory | Detalle de CPU y memoria |
| gDisk | vDisk | Discos conectados a cada VM |
| gNetwork | vNetwork | NICs, alias IP, IP externa, proyecto de la VPC (Shared VPC) |
| gDisks | vDatastore | Todos los discos, incluidos los huérfanos |
| gSnapshot | vSnapshot | Snapshots con antigüedad y tamaño almacenado |
| gLicense | vLicense | Licencias de SO y SQL Server por disco |
| gMIG / gTemplate | vCluster | MIGs, autoscaling, health checks, templates |
| gVPC / gSubnet / gFirewall / gAddress | vSwitch / dvPort | Red |
| gImage | vCD | Imágenes custom y machine images, con antigüedad y deprecación |
| gReservation / gCommitment | — | Reservas (con % sin usar) y CUDs (días hasta el vencimiento) |
| gSoleTenant | vHost | Nodos sole-tenant y VMs alojadas |
| gQuota | — | Cuotas regionales con uso > 0 y % de consumo |
| gTools * | vTools | SO real, kernel y versión del agente (OS Config) |
| gPerf * | — | CPU y RAM: media, p95 y máximo en N días (Cloud Monitoring; la RAM requiere Ops Agent) |
| gRecommend * | — | Recomendaciones activas: rightsizing, recursos idle y CUDs, con ahorro mensual |
| gCost * | — | Coste mensual estimado por proyecto: compute y GPU, licencias, discos, snapshots, imágenes, IPs sin uso |
| gProject | vSource | Resumen por proyecto y resultado del escaneo |
| gHealth | vHealth | Hallazgos: discos huérfanos, firewall abierto, SA por defecto, VMs paradas, discos sin snapshot schedule, templates con imágenes deprecadas o antiguas… |
| gErrors / gMetaData | vMetaData | Errores por proyecto e información del escaneo |

\* Módulos opcionales. Se activan en la pantalla de Proyectos o con `--modules`. Si la API está deshabilitada o falta el permiso, el módulo se omite: el proyecto queda como "OK (sin …)" y el motivo se registra en gErrors.

### Iniciar sesión con Google

Para usar gceTools sin instalar Google Cloud CLI: **Conexión → Iniciar sesión con Google**. Se abre el navegador, aceptas el permiso y la sesión queda guardada; al volver a abrir la app ya estás conectado.

- Se solicita solo `cloud-platform.read-only` (más tu email, para mostrar la cuenta). Flujo OAuth para apps instaladas con PKCE y redirección a `127.0.0.1`.
- El token de refresco se guarda **solo en el llavero del sistema** (Credential Manager en Windows, Keychain en macOS, Secret Service en Linux), nunca en archivos. "Cerrar sesión" lo borra; también puedes revocar el acceso en [tu cuenta de Google](https://myaccount.google.com/permissions).
- gceTools no envía tus datos a ningún servidor propio: habla directamente con las APIs de Google. Ver [PRIVACY.md](PRIVACY.md).
- En consola: `gcetools-cli login` y luego `gcetools-cli scan --auth oauth`; `gcetools-cli logout` cierra la sesión.

**Si tu organización bloquea apps externas** (políticas de Google Workspace o Cloud Identity), hay dos salidas:

- Que un administrador autorice gceTools en *Consola de administración → Seguridad → Acceso y control de datos → Controles de API*.
- O usar un cliente OAuth propio: en un proyecto de la organización, *APIs y servicios → Pantalla de consentimiento de OAuth* con tipo **Interno**, y *Credenciales → ID de cliente de OAuth* de tipo **App de escritorio**. Se reparte el JSON y cada usuario lo carga en **Conexión → Opciones avanzadas → Usar el cliente OAuth de mi organización** (o `gcetools-cli login --client client_secret_….json`). Con un cliente propio, la cuota de las APIs opcionales (Recommender, Cloud Asset, Cloud Billing) se carga a ese proyecto, que debe tenerlas habilitadas.

Las compilaciones hechas desde el código fuente no incluyen el cliente de gceTools (se inyecta al publicar las releases); en ellas solo funciona el cliente propio.

### Idioma

La interfaz está en español e inglés: el selector está en la barra superior (icono de traducción) y por defecto se usa el idioma del sistema.

- Los escaneos, las exportaciones XLSX/CSV y el informe se generan en el idioma activo: títulos de pestañas y columnas, reglas de salud, estados y metadatos. Los nombres de las pestañas (`gInfo`, `gDisks`…) no cambian, para no romper scripts que lean los archivos.
- Al abrir un escaneo del historial o un snapshot JSON, las cabeceras se muestran en el idioma activo; los textos de los hallazgos quedan en el idioma en que se escaneó.
- En consola: `--lang es|en` (o la variable `GCETOOLS_LANG`); si no se indica, se usa el idioma del sistema.

### VMs gestionadas por GKE, Dataproc, Dataflow o Batch

gInfo indica en **Gestionada por** qué servicio creó cada VM (GKE, Dataproc, Dataflow, Batch o MIG) y, para GKE y Dataproc, el **clúster** y el **node pool**. Se detecta por los labels y metadatos que ponen esos servicios.

- Las reglas de salud que no tienen sentido en VMs desechables se omiten para ellas: VM parada, VM parada sin discos, deletion protection, snapshot schedule y Ops Agent. El resto (IP pública, service account por defecto, Secure Boot…) se sigue evaluando.
- En **Inventario**, la casilla **Ocultar las VMs gestionadas** quita los nodos de GKE, Dataproc, Dataflow y Batch de las pestañas por VM (gInfo, gCPU, gMemory, gDisk, gNetwork, gPerf, gTools, gLicense y gHealth). Los miembros de MIGs propios siguen visibles.

### Modo rápido con Cloud Asset Inventory

En organizaciones con muchos proyectos, el **modo rápido** (Proyectos → **Modo rápido (Cloud Asset Inventory)**, o `--fast`) lee VMs, discos, snapshots, imágenes, machine images, templates, MIGs, autoscalers, VPCs, subredes, firewalls e IPs de Cloud Asset Inventory en vez de la Compute API:

- **Sin ámbito:** una lectura por proyecto en lugar de una docena de llamadas.
- **Con una organización o carpeta** en el ámbito de detección (o `--discover`): una sola lectura paginada para todos los proyectos.
- Reservas, commitments, sole-tenant, cuotas y los módulos opcionales (OS Config, Monitoring, Recommender) se siguen leyendo por proyecto, porque CAI no los incluye o no tiene esos datos.
- **Retraso:** CAI puede tardar unos minutos en reflejar cambios recientes; gMetaData indica que el escaneo usó el modo rápido.
- Si CAI falla en un proyecto (permisos, API deshabilitada), ese proyecto se lee con la Compute API y el motivo queda en gErrors.
- Necesita `roles/cloudasset.viewer` en el ámbito y la Cloud Asset API habilitada en el quota project (o en el proyecto de la service account o del cliente OAuth).

```bash
gcetools-cli scan --discover organizations/123456 --fast --out inventario.xlsx
```

### Historial y tendencias

Cada escaneo hecho desde la app se guarda automáticamente (se desactiva en **Historial**) como JSON comprimido en la carpeta de datos del usuario: `~/.local/share/gcetools/history` en Linux, `%LocalAppData%\gcetools\history` en Windows y `~/Library/Application Support/gcetools/history` en macOS. Se conservan los 100 más recientes.

- **Historial** muestra la lista de escaneos y gráficas de evolución de VMs, vCPU, coste (real si existe, si no el estimado) y hallazgos de severidad alta.
- **Abrir en Inventario** carga un escaneo antiguo; **Comparar con el anterior** (o seleccionando dos) abre la comparación sin exportar nada a mano.
- En consola: `gcetools-cli scan --save-history` guarda también en el mismo historial (útil en tareas programadas).

### Informe ejecutivo

**Inventario → Exportar → Informe ejecutivo (HTML)** genera una página autocontenida (sin scripts ni recursos externos) que se abre en el navegador y se imprime a PDF: totales, coste estimado o real, ahorro de Recommender, los 10 hallazgos principales, coste por proyecto, proyectos no leídos y, si hay historial, los cambios desde el escaneo anterior. En consola: `gcetools-cli scan --report informe.html`.

### Perfiles y descubrimiento con Cloud Asset Inventory

- **Perfiles:** en Proyectos → **Guardar perfil** se guardan el filtro, los proyectos seleccionados, los módulos, la moneda, el paralelismo, el ámbito de descubrimiento y el método de autenticación. Se aplican desde el selector "Perfil" o en consola con `--profile`. No guardan secretos: de una clave de service account solo se guarda la ruta del archivo. Se almacenan en `profiles.json` dentro de la carpeta de configuración del usuario (`~/.config/gcetools` en Linux, `%AppData%\gcetools` en Windows, `~/Library/Application Support/gcetools` en macOS), con permisos 0600.
- **Detectar proyectos con Compute:** una sola consulta a Cloud Asset Inventory sobre una organización o carpeta (`organizations/123`, `folders/456`) devuelve qué proyectos tienen VMs, discos, snapshots, imágenes, templates, reservas o commitments, y solo esos se seleccionan. En organizaciones con cientos o miles de proyectos evita escanear los que no usan Compute Engine. Necesita `roles/cloudasset.viewer` en ese ámbito y la Cloud Asset API habilitada en el quota project (o en el proyecto de la service account).

```bash
gcetools-cli scan --discover organizations/123456 --out inventario.json
gcetools-cli scan --profile produccion --out inventario.xlsx
gcetools-cli profiles
```

### Comparar inventarios

Exporta cada escaneo como **Snapshot JSON** y compáralos en la página **Comparar** (no necesita conexión a Google Cloud) o con `gcetools-cli diff`. El informe tiene cinco pestañas:

| Pestaña | Contenido |
|---|---|
| Resumen | Altas, bajas y cambios por tipo de recurso |
| Altas / Bajas | Recursos nuevos y eliminados, con un resumen de cada uno |
| Cambios | Un campo por fila: valor anterior y valor nuevo |
| Metadatos | Inventarios comparados, diferencia de coste/mes y lo que no se pudo comparar |

- **Relevantes primero:** VMs redimensionadas, cambios de service account o de Shielded VM, IPs públicas nuevas, reglas de firewall abiertas a Internet, discos que quedan huérfanos, hallazgos nuevos de severidad alta y cambios en commitments.
- **Sin falsos positivos por permisos:** un recurso solo se compara si su colector funcionó en los dos escaneos; si un proyecto falló en uno de ellos, aparece en Metadatos como "No comparado" en lugar de como cientos de bajas.
- **Sin ruido:** se ignoran fechas de último arranque y parada, antigüedades, uso de cuotas, métricas y los snapshots creados por schedule (rotan a diario). Los costes solo se comparan si los dos inventarios usan la misma moneda.

```bash
gcetools-cli diff --out cambios.xlsx anterior.json nuevo.json
gcetools-cli diff --fail-on-relevant base.json hoy.json   # código de salida 3 si hay cambios relevantes
```

### Reglas de salud configurables

Cada regla de gHealth tiene un ID estable (`gcetools-cli rules` las lista). En **Proyectos → Reglas de salud** se pueden desactivar y ajustar sus umbrales (días de parada, antigüedad de snapshots e imágenes, CPU, RAM, cuotas, vencimiento de CUD); la configuración se guarda en el perfil.

Para exceptuar recursos concretos sin desactivar la regla, usa labels en la VM, el disco, el template o el proyecto:

| Label | Efecto |
|---|---|
| `gcetools-ignore=<id>` | Omite esa regla para el recurso |
| `gcetools-ignore-<id>=true` | Igual, y permite combinar varias reglas |
| `gcetools-ignore=all` | Omite todas las reglas para el recurso |

Por ejemplo, un proyecto con el label `gcetools-ignore=snapshot-schedule` no genera avisos de discos sin snapshot schedule (útil si los backups los hace Veeam o Backup and DR). gMetaData indica cuántos hallazgos se ocultaron y qué reglas estaban desactivadas.

```bash
gcetools-cli rules
gcetools-cli scan --disable-rules snapshot-schedule,no-labels
```

### Coste real (BigQuery Billing Export)

Opcional. Con la [exportación de facturación a BigQuery](https://cloud.google.com/billing/docs/how-to/export-data-bigquery) activada, gceTools lee el coste **neto** de Compute Engine de un mes (con CUD, SUD, créditos y descuentos) y lo pone junto al estimado de lista:

- **gCostReal:** por proyecto, coste bruto, créditos, neto, estimado de lista y diferencia. Con la exportación **detallada** (`gcp_billing_export_resource_v1_…`) separa lo atribuido a VMs y discos de lo no atribuido (red/egress, IPs, snapshots, commitments, recursos ya borrados).
- **gInfo y gDisks:** columna "Coste real mes (BigQuery)" por VM y por disco (solo con la exportación detallada).
- **Coste de la consulta:** es la única función de gceTools que puede costar dinero, porque BigQuery cobra los datos leídos (1 TiB gratis al mes por cuenta de facturación). gceTools filtra por las particiones del mes, hace antes una estimación gratuita (*dry run*) y no ejecuta la consulta si leería más del límite (50 GB por defecto). gMetaData muestra los GB leídos.
- **Permisos:** `roles/bigquery.jobUser` en el proyecto de la tabla y `roles/bigquery.dataViewer` en el dataset.

```bash
gcetools-cli scan --billing-table finops-prj.billing.gcp_billing_export_resource_v1_XXXXXX_XXXXXX_XXXXXX --billing-month 202610
```

### Coste estimado

El módulo `pricing` descarga el catálogo público de Compute Engine (Cloud Billing Catalog API). Lo guarda en caché 24 horas en `~/.cache/gcetools/` o en su equivalente en Windows y macOS, y calcula precios de lista on-demand a 730 h/mes en la moneda elegida (`--currency USD|EUR|CLP…`). Añade columnas de coste a gInfo, gDisks, gSnapshot, gImage, gAddress y gProject, y genera la pestaña gCost.

- **VM encendida:** compute + GPU + licencias + discos.
- **VM parada:** solo discos.
- **No incluye:** descuentos por uso sostenido (SUD), CUDs, precios negociados, red/egress ni licencias BYOL/BYOS.
- **Licencias de pago tarifadas:** Windows Server, SQL Server, RHEL (también for SAP), SLES (también for SAP) y Ubuntu Pro (también FIPS y Minimal Pro; por tramo de vCPU más RAM). Container-Optimized OS y las distribuciones libres no tienen coste de licencia.
- **Lo que no encuentra precio** aparece en gInfo (columna "Notas de coste") y en gMetaData ("Sin tarifa").
- **Validación con el catálogo real:**

  ```bash
  GCETOOLS_CATALOG=~/.cache/gcetools/compute-skus-USD.json go test ./internal/pricing -run Real -v
  ```

## Instalación

| Sistema | Paquete | Requisitos |
|---|---|---|
| Windows 10/11 (x64 o ARM64) | `gcetools-…-setup.exe` (instalador) o `.zip` portable | WebView2 Runtime (ya incluido en Windows 10/11 actualizados; si falta, la app ofrece instalarlo) |
| macOS 11+ (Intel y Apple Silicon) | `.dmg` (arrastrar a Aplicaciones) o `.zip` con `gcetools-cli`; binario universal | — |
| Debian 12+ / Ubuntu 22.04+ (x64) | `.deb` | Dependencias resueltas por `apt` |
| Fedora 36+ / RHEL 9+ / openSUSE (x64) | `.rpm` | Dependencias resueltas por `dnf` / `zypper` |
| Cualquier Linux x64 | `.AppImage` o `.tar.gz` | GTK 3 y WebKitGTK 4.1 instalados (vienen con GNOME en las distros actuales) |

Cada paquete incluye también `gcetools-cli`, la versión de consola para tareas programadas.

En Linux:

```bash
sudo apt install ./gcetools-0.7.0-linux-amd64.deb
```

```bash
sudo dnf install ./gcetools-0.7.0-linux-x86_64.rpm
```

```bash
chmod +x gcetools-0.7.0-linux-x86_64.AppImage && ./gcetools-0.7.0-linux-x86_64.AppImage
```

El `.deb` y el `.rpm` instalan `gcetools` y `gcetools-cli` en `/usr/bin` y añaden gceTools al menú de aplicaciones. El AppImage no incluye GTK ni WebKitGTK: usa los del sistema.

**Mientras los binarios no estén firmados:**
- **Windows:** SmartScreen avisa de "editor desconocido". Hay que pulsar "Más información" y luego "Ejecutar de todas formas".
- **macOS:** Gatekeeper bloquea la app. Se abre con clic derecho → Abrir, o quitando la cuarentena:

  ```bash
  xattr -dr com.apple.quarantine gceTools.app
  ```

**Firma:**
- **macOS:** el workflow firma y notariza solo si existen los secrets `APPLE_CERT_P12`, `APPLE_CERT_PASSWORD`, `APPLE_SIGN_IDENTITY`, `APPLE_ID`, `APPLE_TEAM_ID` y `APPLE_APP_PASSWORD`. Hace falta una cuenta Apple Developer (99 USD/año).
- **Windows:** firma gratuita para proyectos open source de [SignPath Foundation](https://signpath.org) (ver [CODE_SIGNING_POLICY.md](CODE_SIGNING_POLICY.md)). Solo se firman las releases (tags `v*`) y solo si existen el secret `SIGNPATH_API_TOKEN` y las variables `SIGNPATH_ORGANIZATION_ID`, `SIGNPATH_PROJECT_SLUG` y `SIGNPATH_SIGNING_POLICY_SLUG`. Se firma en dos rondas: primero `gcetools.exe` y `gcetools-cli.exe`, luego el instalador, regenerado con el ejecutable ya firmado. Las configuraciones de artefacto de SignPath están en `.signpath/artifact-configurations/`. Cada firma se aprueba a mano en SignPath. El editor que muestra Windows es "SignPath Foundation".

## Builds multiplataforma

El workflow [.github/workflows/build.yml](.github/workflows/build.yml) ejecuta los tests y compila en runners nativos:

| Plataforma | Qué genera |
|---|---|
| Linux | `.deb`, `.rpm`, `.AppImage` y `.tar.gz` (script [build/linux/package.sh](build/linux/package.sh), con nfpm y appimagetool) |
| Windows | `.zip` e instalador NSIS |
| macOS | `.dmg` y `.zip` con la `.app` universal |

Cada build se publica como artefacto. Al subir un tag `v*` se crea además una GitHub Release.

Desde Linux también se puede compilar localmente para Windows, porque Wails no necesita CGO en Windows:

```bash
wails build -platform windows/amd64,windows/arm64
GOOS=windows CGO_ENABLED=0 go build -o build/bin/gcetools-cli.exe ./cmd/gcetools-cli
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o build/bin/gcetools-cli-macos ./cmd/gcetools-cli
```

La app gráfica de macOS solo se puede compilar en un Mac (o en el runner `macos-latest`), porque necesita el SDK de Apple.

## Requisitos de desarrollo

- Go 1.23+ y Node 22+ (CI usa Node 26)
- CLI de Wails: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Dependencias del sistema (`wails doctor` las comprueba):
  - **Linux:** `libgtk-3-dev` y `libwebkit2gtk-4.1-dev` (Debian 13 / Ubuntu 24.04 solo traen la 4.1, de ahí `-tags webkit2_41`)
  - **Windows:** WebView2 (viene con Windows 10/11)
  - **macOS:** Xcode Command Line Tools

## Uso

```bash
wails dev -tags webkit2_41                 # app con recarga en caliente
wails build -tags webkit2_41               # binario en build/bin/ (Linux)
wails build -platform windows/amd64        # en Windows
wails build -platform darwin/universal     # en macOS
go test ./...                              # tests del backend
```

Para trabajar solo en la interfaz, sin Wails ni credenciales:

```bash
npm --prefix frontend run dev              # abre http://localhost:5173 con datos simulados ("Modo demo")
```

### Línea de comandos (tareas programadas)

```bash
gcetools-cli scan --out inventario.xlsx
gcetools-cli scan --auth sa --key sa.json --query "parent:folders/123456" --out inv.zip --csv-sep ";"
gcetools-cli scan --auth impersonate --impersonate inventory@prj.iam.gserviceaccount.com --projects a,b
gcetools-cli scan --out "inventario-$(date +%F).json"     # snapshot para comparar después
gcetools-cli diff --out cambios.xlsx anterior.json nuevo.json
gcetools-cli scan --profile produccion --discover organizations/123456
gcetools-cli profiles
gcetools-cli scan --save-history --report "informe-$(date +%F).html"   # historial + informe para la tarea semanal
gcetools-cli login && gcetools-cli scan --auth oauth
gcetools-cli scan --discover organizations/123456 --fast           # modo rápido con Cloud Asset Inventory
gcetools-cli --lang en scan --out inventory.xlsx                    # inventario en inglés
gcetools-cli version
```

`gcetools-cli` viene en todos los paquetes y funciona igual en Windows, macOS y Linux. En Linux y macOS el binario de la app también acepta `gcetools scan …` y `gcetools diff …`; en Windows no, porque la app gráfica no escribe en la consola.

## Coste de uso de las APIs

gceTools solo hace lecturas. Casi todas las APIs que usa son gratuitas; pueden facturar Cloud Monitoring, con un tramo gratuito que en la práctica no se agota, y BigQuery si activas el coste real.

| API | Para qué | Coste |
|---|---|---|
| Compute Engine | Inventario de VMs, discos, red, snapshots… | Gratis (solo cuotas de peticiones por minuto) |
| Resource Manager | Listar proyectos | Gratis |
| Cloud Billing Catalog | Precios del coste estimado (caché de 24 h) | Gratis |
| Recommender | Recomendaciones | Gratis |
| Cloud Asset Inventory | Descubrir proyectos con Compute y modo rápido | Gratis (con cuotas por minuto por proyecto) |
| Cloud Monitoring | Métricas de CPU y RAM (gPerf) | 0,50 USD por millón de series devueltas; el primer millón al mes por cuenta de facturación es gratis |
| BigQuery | Coste real (opcional, desactivado por defecto) | Por datos leídos (1 TiB gratis al mes); gceTools estima antes y respeta un límite de GB |
| OS Config | SO y agentes (gTools) | Leer es gratis. VM Manager cobra por VM con agente activo (0,003 USD/h, 100 VMs gratis por cuenta de facturación), pero eso depende de tener el agente, no de gceTools |
| GitHub | Buscar actualizaciones | Gratis |

Como referencia, un escaneo pide unas 2 series de Monitoring por VM (CPU y RAM): con 100 VMs harían falta unos 5.000 escaneos al mes para salir del tramo gratuito. Con credenciales de usuario, lo que se factura va al quota project de ADC. Precios de noviembre de 2026: consulta [Monitoring](https://cloud.google.com/monitoring/pricing), [VM Manager](https://cloud.google.com/compute/vm-manager/pricing) y [Cloud Asset Inventory](https://cloud.google.com/asset-inventory/pricing).

## Permisos IAM

- Mínimos: `roles/compute.viewer` y `roles/browser` (a nivel de organización o carpeta para tener visión global).
- gTools: `roles/osconfig.inventoryViewer`, con la API de OS Config habilitada en el proyecto y el agente en las VMs.
- gPerf: `roles/monitoring.viewer`.
- Descubrimiento y modo rápido con Cloud Asset Inventory: `roles/cloudasset.viewer` en la organización, carpeta o proyecto.
- Coste real: `roles/bigquery.jobUser` en el proyecto de la tabla de facturación y `roles/bigquery.dataViewer` en su dataset.
- gRecommend: `roles/recommender.computeViewer`. Con credenciales de usuario, Google exige un quota project: se usa el `quota_project_id` de ADC (`gcloud auth application-default set-quota-project <proyecto>`), que debe tener la Recommender API habilitada.
- Sin `resourcemanager.projects.list` se pueden añadir proyectos a mano por ID.

## Estructura

```
main.go, app.go           Arranque de Wails + métodos expuestos al frontend
internal/auth             ADC / clave JSON / impersonación / inicio de sesión con Google
internal/core             Descubrimiento de proyectos, escaneo concurrente, HTTP con rate-limit y reintentos
internal/collect          Un colector por tipo de recurso (aggregatedList o Cloud Asset Inventory) + esquema de pestañas
internal/i18n             Traducciones (el texto en español es la clave; inglés en en_*.go)
internal/health           Reglas de gHealth
internal/diff             Comparación de dos inventarios (altas, bajas y cambios)
internal/pricing          Catálogo de Cloud Billing y cálculo de coste estimado
internal/export           XLSX (excelize streaming), CSV en ZIP, JSON
internal/cli              Comandos `scan`, `diff`, `profiles` y `version`
internal/profile          Perfiles de escaneo guardados
internal/billing          Coste real desde la exportación de facturación a BigQuery
internal/history          Historial local de escaneos
internal/report           Informe ejecutivo en HTML
frontend/                 React + TS, tokens Material 3, AG Grid
assets/brand/             Identidad visual: SVG maestros, iconos por plataforma, favicon y recursos web
```

Para añadir una pestaña: define la hoja en `internal/collect/schema.go`, escribe su colector y regístralo en `Project()` (collect.go). La UI y los exportadores la muestran sin más cambios.

Para traducir: envuelve el texto con `i18n.T("…")` o `i18n.Tf("…", args)` en Go y con `t('…')` en el frontend, y añade la versión en inglés a `internal/i18n/en_*.go` o a `frontend/src/lib/i18n.en.ts`. `go test ./internal/i18n` y `npm --prefix frontend run check:i18n` fallan si falta alguna.

## Pendiente

- Firma de los ejecutables de Windows
- Pestaña de postura de seguridad (gSecurity)

## Actualizaciones

La app comprueba al iniciarse si hay una release más reciente en GitHub; la comprobación se puede desactivar en "Acerca de". En la línea de comandos: `gcetools-cli version`. La consulta es un GET anónimo a la API pública de GitHub y no envía datos del usuario ni del inventario.

## Publicar una versión

1. Actualiza la versión en `internal/core/core.go`, `wails.json` y `frontend/package.json`. El CI inyecta igualmente la versión del tag en el binario.
2. Crea el tag y súbelo:

   ```bash
   git tag v0.2.0
   git push origin v0.2.0
   ```

3. El workflow publica la GitHub Release con los paquetes de Linux, Windows y macOS.

## Apoyar el proyecto

gceTools es gratuito. Si te resulta útil, puedes [invitarme a un café](https://buymeacoffee.com/jordimlopezr).

## Licencia

Copyright © 2026 Jordi Lopez Reyes.

Este programa es software libre: puedes redistribuirlo y/o modificarlo bajo los términos de la **GNU General Public License v3.0** publicada por la Free Software Foundation. Se distribuye con la esperanza de que sea útil, pero **sin ninguna garantía**. Consulta el archivo [LICENSE](LICENSE) para más detalles.
