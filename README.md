# gceTools

Inventario de Google Compute Engine al estilo RVTools: aplicación de escritorio para Windows, Linux y macOS, más un modo de línea de comandos.
Lee todos los proyectos a los que llega la identidad y exporta a XLSX (una hoja por pestaña), CSV (ZIP) o JSON.

- **Solo lectura:** pide únicamente el scope `cloud-platform.read-only`.
- **Autenticación:** credenciales de gcloud (ADC), clave JSON de service account (solo en memoria) o impersonación de service account.
- **Stack:** Go + [Wails v2](https://wails.io) + React/TypeScript, con interfaz Material 3 y AG Grid Community.

## Pestañas (MVP)

| Pestaña | RVTools | Contenido |
|---|---|---|
| gInfo | vInfo | Una fila por VM: machine type, vCPU, RAM, estado, IPs, SO, discos, SA, labels, Shielded VM, MIG… |
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
| gHealth | vHealth | Hallazgos: discos huérfanos, firewall abierto, SA por defecto, VMs paradas… |
| gErrors / gMetaData | vMetaData | Errores por proyecto e información del escaneo |

\* Módulos opcionales. Se activan en la pantalla de Proyectos o con `--modules`. Si la API está deshabilitada o falta el permiso, el módulo se omite: el proyecto queda como "OK (sin …)" y el motivo se registra en gErrors.

### Coste estimado

El módulo `pricing` descarga el catálogo público de Compute Engine (Cloud Billing Catalog API). Lo guarda en caché 24 horas en `~/.cache/gcetools/` o en su equivalente en Windows y macOS, y calcula precios de lista on-demand a 730 h/mes en la moneda elegida (`--currency USD|EUR|CLP…`). Añade columnas de coste a gInfo, gDisks, gSnapshot, gImage, gAddress y gProject, y genera la pestaña gCost.

- **VM encendida:** compute + GPU + licencias + discos.
- **VM parada:** solo discos.
- **No incluye:** descuentos por uso sostenido (SUD), CUDs, precios negociados, red/egress ni licencias BYOL/BYOS.
- **Licencias de pago tarifadas:** Windows Server, SQL Server, RHEL (también for SAP) y SLES (también for SAP). Ubuntu Pro todavía no.
- **Lo que no encuentra precio** aparece en gInfo (columna "Notas de coste") y en gMetaData ("Sin tarifa").
- **Validación con el catálogo real:**

  ```bash
  GCETOOLS_CATALOG=~/.cache/gcetools/compute-skus-USD.json go test ./internal/pricing -run Real -v
  ```

## Instalación

| Sistema | Paquete | Requisitos |
|---|---|---|
| Windows 10/11 (x64 o ARM64) | `gcetools-…-setup.exe` (instalador) o `.zip` portable | WebView2 Runtime (ya incluido en Windows 10/11 actualizados; si falta, la app ofrece instalarlo) |
| macOS 11+ (Intel y Apple Silicon) | `gceTools.app` en `.zip`, binario universal | — |
| Linux x64 | `.tar.gz` | `libgtk-3-0` y `libwebkit2gtk-4.1-0` |

Cada paquete incluye también `gcetools-cli`, la versión de consola para tareas programadas.

**Mientras los binarios no estén firmados:**
- **Windows:** SmartScreen avisa de "editor desconocido". Hay que pulsar "Más información" y luego "Ejecutar de todas formas".
- **macOS:** Gatekeeper bloquea la app. Se abre con clic derecho → Abrir, o quitando la cuarentena:

  ```bash
  xattr -dr com.apple.quarantine gceTools.app
  ```

**Firma:**
- **macOS:** el workflow firma y notariza solo si existen los secrets `APPLE_CERT_P12`, `APPLE_CERT_PASSWORD`, `APPLE_SIGN_IDENTITY`, `APPLE_ID`, `APPLE_TEAM_ID` y `APPLE_APP_PASSWORD`. Hace falta una cuenta Apple Developer (99 USD/año).
- **Windows:** se recomienda Azure Trusted Signing o un certificado EV, que todavía no están integrados.

## Builds multiplataforma

El workflow [.github/workflows/build.yml](.github/workflows/build.yml) ejecuta los tests y compila en runners nativos:

| Plataforma | Qué genera |
|---|---|
| Linux | `.tar.gz` |
| Windows | `.zip` e instalador NSIS |
| macOS | `.app` universal en `.zip` |

Cada build se publica como artefacto. Al subir un tag `v*` se crea además una GitHub Release.

Desde Linux también se puede compilar localmente para Windows, porque Wails no necesita CGO en Windows:

```bash
wails build -platform windows/amd64,windows/arm64
GOOS=windows CGO_ENABLED=0 go build -o build/bin/gcetools-cli.exe ./cmd/gcetools-cli
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o build/bin/gcetools-cli-macos ./cmd/gcetools-cli
```

La app gráfica de macOS solo se puede compilar en un Mac (o en el runner `macos-latest`), porque necesita el SDK de Apple.

## Requisitos de desarrollo

- Go 1.23+ y Node 20+
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
gcetools scan --out inventario.xlsx
gcetools scan --auth sa --key sa.json --query "parent:folders/123456" --out inv.zip --csv-sep ";"
gcetools scan --auth impersonate --impersonate inventory@prj.iam.gserviceaccount.com --projects a,b
```

En Windows el binario de la GUI no escribe en la consola. Para tareas programadas compila `go build -o gcetools-cli.exe ./cmd/gcetools-cli`.

## Permisos IAM

- Mínimos: `roles/compute.viewer` y `roles/browser` (a nivel de organización o carpeta para tener visión global).
- gTools: `roles/osconfig.inventoryViewer`, con la API de OS Config habilitada en el proyecto y el agente en las VMs.
- gPerf: `roles/monitoring.viewer`.
- gRecommend: `roles/recommender.computeViewer`. Con credenciales de usuario, Google exige un quota project: se usa el `quota_project_id` de ADC (`gcloud auth application-default set-quota-project <proyecto>`), que debe tener la Recommender API habilitada.
- Sin `resourcemanager.projects.list` se pueden añadir proyectos a mano por ID.

## Estructura

```
main.go, app.go           Arranque de Wails + métodos expuestos al frontend
internal/auth             ADC / clave JSON / impersonación
internal/core             Descubrimiento de proyectos, escaneo concurrente, HTTP con rate-limit y reintentos
internal/collect          Un colector por tipo de recurso (aggregatedList) + esquema de pestañas
internal/health           Reglas de gHealth
internal/export           XLSX (excelize streaming), CSV en ZIP, JSON
internal/cli              Comando `scan`
frontend/                 React + TS, tokens Material 3, AG Grid
```

Para añadir una pestaña: define la hoja en `internal/collect/schema.go`, escribe su colector y regístralo en `Project()` (collect.go). La UI y los exportadores la muestran sin más cambios.

## Pendiente

- Inicio de sesión OAuth en el navegador sin gcloud (necesita un OAuth client ID de la organización)
- Guardar perfiles con el llavero del sistema
- Cloud Asset Inventory para organizaciones grandes
- Comparar dos snapshots JSON
- Firma de Windows (Azure Trusted Signing o EV), DMG para macOS y AppImage o .deb para Linux
