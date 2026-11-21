# Manual de gceTools

*Versión 0.7 · [English](manual.en.md)*

gceTools hace un inventario completo de Google Compute Engine al estilo de RVTools en VMware. Lee todos los proyectos a los que llega tu identidad y presenta la información en pestañas: VMs, discos, redes, snapshots, costes, hallazgos de buenas prácticas… Desde ahí se exporta a Excel, CSV, JSON o un informe HTML. Hay una aplicación de escritorio para Windows, macOS y Linux, y una versión de línea de comandos para tareas programadas.

Este manual explica cada pantalla, cada opción y cada comando.

## Índice

1. [Conceptos básicos](#1-conceptos-básicos)
2. [Instalación](#2-instalación)
3. [Primeros pasos](#3-primeros-pasos)
4. [La ventana de la aplicación](#4-la-ventana-de-la-aplicación)
5. [Conexión](#5-conexión)
6. [Proyectos y escaneo](#6-proyectos-y-escaneo)
7. [Inventario](#7-inventario)
8. [Referencia de pestañas](#8-referencia-de-pestañas)
9. [Reglas de salud (gHealth)](#9-reglas-de-salud-ghealth)
10. [Costes](#10-costes)
11. [Comparar inventarios](#11-comparar-inventarios)
12. [Historial](#12-historial)
13. [Informe ejecutivo](#13-informe-ejecutivo)
14. [Acerca de y actualizaciones](#14-acerca-de-y-actualizaciones)
15. [Idioma](#15-idioma)
16. [Línea de comandos](#16-línea-de-comandos)
17. [Permisos IAM y APIs](#17-permisos-iam-y-apis)
18. [Coste de uso de las APIs](#18-coste-de-uso-de-las-apis)
19. [Archivos y datos locales](#19-archivos-y-datos-locales)
20. [Privacidad y seguridad](#20-privacidad-y-seguridad)
21. [Solución de problemas](#21-solución-de-problemas)
22. [Preguntas frecuentes](#22-preguntas-frecuentes)
23. [Equivalencias con RVTools](#23-equivalencias-con-rvtools)

---

## 1. Conceptos básicos

- **Solo lectura.** gceTools pide únicamente el scope `cloud-platform.read-only`. No crea, modifica ni borra nada en Google Cloud.
- **Identidad.** El inventario incluye lo que puede ver la identidad con la que te conectas: tu cuenta de Google, tus credenciales de gcloud o una service account. Lo que no ve (por permisos IAM) no aparece, y gceTools lo indica en la pestaña gErrors.
- **Escaneo.** Leer los proyectos seleccionados. Cada escaneo produce un **inventario**: un conjunto de pestañas (gInfo, gDisks, gHealth…) más los metadatos del escaneo.
- **Módulos opcionales.** Además de la Compute API, gceTools puede leer OS Config (SO real de las VMs), Cloud Monitoring (CPU y RAM), Recommender (recomendaciones de ahorro), el catálogo de precios (coste estimado) y la exportación de facturación a BigQuery (coste real). Cada uno se activa por separado.
- **Snapshot JSON.** Un inventario guardado como archivo JSON. Sirve para comparar dos escaneos o para abrirlo más tarde.
- **Historial.** La app guarda cada escaneo automáticamente en tu equipo para ver tendencias y comparar sin exportar nada.
- **Nada sale de tu equipo.** gceTools no tiene servidores: habla directamente con las APIs de Google y guarda los resultados en tu ordenador.

## 2. Instalación

Descarga el paquete de tu sistema desde la [página de releases](https://github.com/jordilopezr/gcetools/releases/latest) o desde [gcetools.jordilopezr.com](https://gcetools.jordilopezr.com).

| Sistema | Paquete | Requisitos |
|---|---|---|
| Windows 10/11 (x64 o ARM64) | `gcetools-<versión>-windows-amd64-setup.exe` (instalador) o `.zip` portable | WebView2 Runtime (ya viene con Windows 10/11 actualizados; si falta, la app ofrece instalarlo) |
| macOS 11 o posterior (Intel y Apple Silicon) | `.dmg` o `.zip` (binario universal) | — |
| Debian 12+ / Ubuntu 22.04+ | `.deb` | `apt` resuelve las dependencias |
| Fedora 36+ / RHEL 9+ / openSUSE | `.rpm` | `dnf` / `zypper` resuelve las dependencias |
| Cualquier Linux x64 | `.AppImage` o `.tar.gz` | GTK 3 y WebKitGTK 4.1 instalados |

Todos los paquetes incluyen la aplicación (`gcetools`) y la versión de consola (`gcetools-cli`).

**Windows.** Ejecuta el instalador o descomprime el `.zip` donde quieras. Mientras los ejecutables no estén firmados, SmartScreen avisa de "editor desconocido": pulsa **Más información → Ejecutar de todas formas**.

**macOS.** Abre el `.dmg` y arrastra gceTools a Aplicaciones. Gatekeeper bloquea las apps sin firmar: ábrela la primera vez con **clic derecho → Abrir**, o quita la cuarentena:

```bash
xattr -dr com.apple.quarantine /Applications/gceTools.app
```

**Debian / Ubuntu:**

```bash
sudo apt install ./gcetools-0.7.0-linux-amd64.deb
```

**Fedora / RHEL / openSUSE:**

```bash
sudo dnf install ./gcetools-0.7.0-linux-x86_64.rpm
```

**AppImage:**

```bash
chmod +x gcetools-0.7.0-linux-x86_64.AppImage && ./gcetools-0.7.0-linux-x86_64.AppImage
```

El `.deb` y el `.rpm` instalan `gcetools` y `gcetools-cli` en `/usr/bin` y añaden gceTools al menú de aplicaciones.

**Desinstalar.** Como cualquier otra aplicación (Panel de control en Windows, papelera en macOS, `sudo apt remove gcetools` o `sudo dnf remove gcetools` en Linux). Los datos locales (historial, perfiles, caché de precios y la sesión del llavero) no se borran solos: están descritos en [Archivos y datos locales](#19-archivos-y-datos-locales).

## 3. Primeros pasos

1. Abre gceTools. Se muestra la pantalla **Conexión**.
2. Elige **Iniciar sesión con Google** y pulsa el botón: se abre el navegador, eliges tu cuenta y aceptas el permiso de solo lectura. Si ya usas gcloud, también puedes elegir **Credenciales de gcloud**.
3. Pulsa **Conectar**. La app pasa a **Proyectos** y lista los proyectos que ve tu cuenta.
4. Marca los proyectos que quieres inventariar (la casilla de la cabecera los marca todos) y pulsa **Escanear**.
5. Al terminar se abre **Inventario**. Revisa las cifras de la cabecera, la pestaña **gHealth** con los hallazgos y, si quieres, **Exportar → Excel (.xlsx)**.

Para tu cuenta basta con `roles/compute.viewer` y `roles/browser` en los proyectos, carpetas u organización. Los módulos opcionales necesitan algunos roles más: ver [Permisos IAM y APIs](#17-permisos-iam-y-apis).

## 4. La ventana de la aplicación

**Barra superior**, de izquierda a derecha:

- **Logo y nombre.**
- **Versión X disponible:** aparece cuando hay una release más reciente. Lleva a **Acerca de**, donde están las novedades y el enlace de descarga.
- **Modo demo:** solo aparece si la interfaz se ejecuta fuera de la app (en un navegador, para desarrollo), con datos simulados.
- **Tema** (icono de luna o sol): según el sistema, claro u oscuro.
- **Idioma** (icono de traducción): español o English. Ver [Idioma](#15-idioma).
- **Cuenta:** con una sesión activa muestra la identidad. Su menú indica el método de autenticación y permite **Desconectar**.

**Barra lateral de navegación:**

| Sección | Para qué | Disponible |
|---|---|---|
| Conexión | Elegir identidad y conectar | Siempre |
| Proyectos | Elegir qué escanear y con qué opciones | Con una sesión activa |
| Inventario | Ver y exportar el último inventario | Cuando hay un inventario cargado |
| Comparar | Comparar dos inventarios | Siempre |
| Historial | Escaneos anteriores y tendencias | Siempre |
| Acerca de | Versión, actualizaciones, costes de las APIs, licencias | Siempre (abajo) |

Los mensajes breves (escaneo completado, archivo exportado, errores) aparecen en una barra temporal en la parte inferior.

## 5. Conexión

Elige con qué identidad leer el inventario. Lo que veas depende de los permisos IAM de esa identidad. Hay cuatro métodos:

### 5.1 Iniciar sesión con Google

La opción recomendada: no hace falta instalar gcloud.

1. Selecciona **Iniciar sesión con Google** y pulsa el botón del mismo nombre.
2. Se abre tu navegador con la pantalla de Google. Elige la cuenta y acepta. gceTools pide solo `cloud-platform.read-only` y tu email (para mostrar con qué cuenta has entrado).
3. El navegador muestra "Sesión iniciada"; ya puedes cerrar esa pestaña. La app se conecta sola.

La sesión (un token de refresco) se guarda **en el llavero del sistema**: Credential Manager en Windows, Keychain en macOS o Secret Service (GNOME Keyring, KWallet) en Linux. Al volver a abrir gceTools ya estás conectado: aparece **Sesión guardada** con tu cuenta.

- **Cerrar sesión** borra el token del llavero. También puedes revocar el acceso desde [tu cuenta de Google](https://myaccount.google.com/permissions).
- **Cancelar**, mientras esperas al navegador, aborta el inicio de sesión.
- Técnicamente es el flujo OAuth para apps instaladas, con PKCE y redirección a `127.0.0.1`.

**Opciones avanzadas: el cliente OAuth de tu organización.** Algunas organizaciones de Google Workspace o Cloud Identity bloquean las apps externas que piden acceso a Google Cloud. Tienen dos salidas:

- **Autorizar gceTools:** un administrador la permite en *Consola de administración → Seguridad → Acceso y control de datos → Controles de API*.
- **Usar un cliente propio:** un administrador crea en un proyecto de la organización una pantalla de consentimiento de tipo **Interno** y un *ID de cliente de OAuth* de tipo **App de escritorio**, y reparte el JSON. Cada usuario lo carga en **Opciones avanzadas → Usar el cliente OAuth de mi organización**.
  - **Volver al cliente de gceTools** deja de usar el cliente propio.
  - Con un cliente propio, la cuota de las APIs se carga a ese proyecto, que debe tenerlas habilitadas (ver [APIs que habilitar](#173-apis-que-habilitar)).

> Mientras Google no termine de verificar gceTools, el inicio de sesión con el cliente incorporado solo funciona para los usuarios de prueba que haya añadido el mantenedor, y la sesión caduca cada 7 días. El resto de métodos funciona sin esa limitación.

**Llavero no disponible.** Si el sistema no tiene un servicio de llavero (por ejemplo, un Linux sin GNOME Keyring ni KWallet), la tarjeta lo indica y hay que usar otro método. gceTools nunca guarda el token en un archivo.

### 5.2 Credenciales de gcloud (ADC)

Usa las Application Default Credentials de Google Cloud CLI. Si no están, la pantalla muestra **No encontradas** y el comando que hay que ejecutar una vez:

```bash
gcloud auth application-default login
```

Vuelve a abrir la pantalla de Conexión y aparecerá **Detectadas** con tu cuenta.

El **quota project** de ADC es el proyecto al que Google carga las APIs que lo exigen, como Recommender o Cloud Asset Inventory. Se configura con:

```bash
gcloud auth application-default set-quota-project <proyecto>
```

### 5.3 Service account (JSON)

**Seleccionar archivo JSON** carga la clave de una service account. Se muestra su email, su proyecto y el inicio del key id. La clave **solo se mantiene en memoria**: no se copia ni se guarda, y se borra al desconectar o cerrar la app.

### 5.4 Impersonar service account

Tu cuenta obtiene tokens de corta duración de una service account, sin descargar claves. Es lo ideal si la organización prohíbe las claves.

- Escribe el email de la service account.
- Tu cuenta necesita `roles/iam.serviceAccountTokenCreator` sobre ella.
- Requiere credenciales ADC (`gcloud auth application-default login`).

### 5.5 Conectar

**Conectar** valida las credenciales y pasa a **Proyectos**. Si ya había una sesión, conectar de nuevo la reemplaza. El panel **Permisos IAM recomendados** resume los roles necesarios.

## 6. Proyectos y escaneo

### 6.1 La lista de proyectos

Al entrar se listan todos los proyectos activos que ve la identidad, con su Project ID, nombre, número y padre (carpeta u organización).

- **Filtro Resource Manager** (arriba): restringe la búsqueda con la sintaxis de Resource Manager y se aplica con **Buscar**. Ejemplos:
  - `parent:folders/123456`: proyectos directamente bajo una carpeta.
  - `labels.env:prod`: proyectos con el label `env=prod`.
  - `name:erp*`: nombre que empieza por "erp".
- **Filtrar la lista:** filtra en el momento los proyectos ya cargados, por cualquier texto.
- **Añadir IDs manualmente:** escribe Project IDs separados por comas o espacios y pulsa Intro. Sirve cuando la identidad no tiene permiso para listar proyectos (`resourcemanager.projects.list`) pero sí para leerlos.
- **Selección:** marca los proyectos a escanear; la casilla de la cabecera marca todos los visibles. El contador muestra visibles y seleccionados.
- La tabla se puede ordenar, filtrar por columna y redimensionar.

### 6.2 Detectar proyectos con Compute (Cloud Asset Inventory)

En organizaciones grandes muchos proyectos no usan Compute Engine. Escribe un ámbito (`organizations/123456` o `folders/456`) y pulsa **Detectar proyectos con Compute**.

- Una sola consulta a Cloud Asset Inventory devuelve qué proyectos tienen VMs, discos, snapshots, imágenes, templates, reservas o commitments.
- Solo se seleccionan esos, y la tabla gana columnas con lo que encontró en cada uno: VMs (CAI), discos y otros.
- Requiere `roles/cloudasset.viewer` en ese ámbito y la Cloud Asset API habilitada en el quota project.

### 6.3 Perfiles

Un perfil guarda una configuración de escaneo para repetirla con un clic (o con `--profile` en el CLI):

- el filtro, los proyectos seleccionados y el ámbito de detección;
- los módulos, días de métricas, moneda, modo rápido y coste real;
- las reglas de salud y sus umbrales;
- el paralelismo y el método de autenticación (para el CLI).

**Guardar perfil** pide un nombre; si ya existe, lo sobrescribe. El selector **Perfil** aplica uno guardado, y el icono de papelera lo elimina tras confirmar.

Los perfiles **no guardan secretos**: de una clave de service account solo se guarda la ruta del archivo. Se almacenan en `profiles.json`, en la carpeta de configuración del usuario.

### 6.4 Módulos opcionales

| Módulo | Qué añade | Notas |
|---|---|---|
| **SO y agentes (OS Config)** | Pestaña gTools: SO real, versión, kernel, hostname, versión del agente | Necesita la API de OS Config en el proyecto y el agente en las VMs |
| **Métricas CPU/RAM** | Pestaña gPerf: media, p95 y máximo de CPU y RAM; reglas cpu-low, cpu-high, mem-high, no-ops-agent | Selector de días: 7, 14, 30 o 42. La RAM requiere Ops Agent |
| **Recomendaciones** | Pestaña gRecommend: rightsizing, recursos idle y CUDs, con ahorro mensual | Con credenciales de usuario exige un quota project con la Recommender API |
| **Coste estimado** | Columnas de coste en varias pestañas y la pestaña gCost | Selector de moneda: USD, EUR, CLP, MXN, COP, BRL, GBP |

Si la API de un módulo está deshabilitada en un proyecto o falta el permiso, el módulo se omite **sin marcar el proyecto como error**. El proyecto queda como "OK (sin osconfig)" o similar, y el motivo se anota en gErrors.

### 6.5 Modo rápido (Cloud Asset Inventory)

El botón **Modo rápido (Cloud Asset Inventory)** cambia la forma de leer VMs, discos, snapshots, imágenes, machine images, templates, MIGs, autoscalers, VPCs, subredes, firewalls e IPs:

- **Sin ámbito de detección:** una consulta a Cloud Asset Inventory por proyecto, en lugar de una docena de llamadas a la Compute API.
- **Con una organización o carpeta en el ámbito de detección:** una sola consulta paginada para todos los proyectos.

Cuándo conviene: en escaneos de cientos de proyectos, o cuando la Compute API limita las peticiones.

Lo que hay que saber:

- **Se sigue leyendo por proyecto:** reservas, commitments, sole-tenant, cuotas, metadatos del proyecto (OS Login, claves SSH) y los módulos opcionales, porque CAI no los incluye o no tiene esos datos.
- **Retraso:** CAI puede tardar unos minutos en reflejar cambios recientes. gMetaData indica que el escaneo usó el modo rápido.
- **Respaldo:** si CAI falla en un proyecto (permisos, API deshabilitada), ese proyecto se lee con la Compute API y el motivo queda en gErrors.
- **Permisos:** `roles/cloudasset.viewer` en el ámbito y la Cloud Asset API habilitada en el quota project, en el proyecto de la service account o en el del cliente OAuth.

### 6.6 Coste real (BigQuery)

El botón **Coste real (BigQuery)** despliega la configuración del coste real. Explicación completa en [Costes](#102-coste-real-bigquery).

- **Tabla de exportación de facturación:** `proyecto.dataset.tabla`.
- **Mes:** el mes anterior al escaneo, o uno concreto.
- **Límite de GB leídos:** por defecto 50 GB.
- **Desactivar** quita la tabla y apaga el módulo.

### 6.7 Reglas de salud

El botón **Reglas de salud** despliega la lista de las 22 reglas, con su severidad, categoría, ID y dónde se lee la excepción por label.

- Desmarca una regla para **desactivarla** en este escaneo y en el perfil.
- Las reglas con umbral tienen un campo para ajustarlo: días de parada, antigüedad de snapshots e imágenes, % de CPU baja o saturada, % de RAM, % de cuota y días hasta el vencimiento de un CUD.
- **Valores por defecto** restaura la configuración original.

Detalle de cada regla en [Reglas de salud](#9-reglas-de-salud-ghealth).

### 6.8 Paralelismo y escaneo

- **Paralelismo:** cuántos proyectos se leen a la vez (8 por defecto). Más paralelismo acaba antes, pero hace más peticiones por segundo. gceTools limita las peticiones (20 por segundo) y reintenta los errores temporales.
- **Escanear (N):** empieza el escaneo de los N proyectos seleccionados.

Durante el escaneo se ve:

- una barra de progreso con proyectos terminados, VMs encontradas y errores;
- qué se está leyendo en cada momento ("Leyendo discos en …");
- un registro con el resultado de cada proyecto.

**Cancelar** detiene el escaneo y muestra lo leído hasta ese momento, marcado como **Parcial**.

Al terminar, la app abre el **Inventario** y, si el historial está activado, guarda el escaneo en él.

**Resultado por proyecto** (columna "Resultado escaneo" de gProject):

| Resultado | Significado |
|---|---|
| OK | Todo leído |
| OK (sin osconfig, monitoring…) | Leído; algún módulo opcional no estaba disponible |
| Parcial (N errores) | Algunos tipos de recurso no se pudieron leer (ver gErrors) |
| Compute API deshabilitada | El proyecto no tiene la Compute API activa |
| Sin permisos | La identidad no puede leer Compute en ese proyecto |
| No encontrado | El proyecto no existe o no es visible |
| Cancelado / No escaneado | El escaneo se canceló antes de llegar a él |

## 7. Inventario

### 7.1 Cabecera y cifras

La cabecera muestra la fecha del escaneo, los proyectos (cuántos OK), la duración y la identidad. **Parcial** indica un escaneo cancelado.

Las tarjetas de cifras:

- **VMs** y **Encendidas**.
- **vCPU**, **RAM (GB)** y **Disco total (GB)**.
- **Discos huérfanos**, **Hallazgos altos** y **Expuestas a Internet** (VMs con puertos sensibles abiertos, ver gSecurity): se resaltan si son mayores que cero.
- **Coste/mes lista**, si se activó el coste estimado.
- **Coste real/mes**, si se activó BigQuery.
- **Ahorro estimado/mes**, si se activaron las recomendaciones.

### 7.2 VMs gestionadas

Si el inventario tiene VMs creadas por GKE, Dataproc, Dataflow o Batch, aparece la casilla **Ocultar las N VMs gestionadas por GKE, Dataproc, Dataflow o Batch**.

- **Qué quita:** esas VMs de las pestañas por VM (gInfo, gCPU, gMemory, gDisk, gNetwork, gPerf, gTools, gLicense, gHealth y gSecurity).
- **Qué no quita:** los miembros de tus propios MIGs, que siguen visibles.
- La preferencia se recuerda entre sesiones.
- **Exportar** incluye siempre todas las VMs.

### 7.3 Pestañas y tabla

- **Chips de pestañas:** cada uno muestra el número de filas. Al pasar el ratón se ve la descripción.
- **Buscar en …:** filtra las filas de la pestaña activa por cualquier texto.
- **Tabla:** ordena pulsando una cabecera, filtra por columna con el icono de embudo y redimensiona arrastrando el borde. El texto de las celdas se puede seleccionar y copiar.
- **Celdas especiales:** estados y severidades se muestran como etiquetas de color; las casillas Sí/No como texto.
- **Formatos:** las fechas en formato `AAAA-MM-DD HH:MM`; los GB con 2 decimales; los importes con 2 decimales en la moneda del escaneo.

### 7.4 Exportar

El botón **Exportar** ofrece:

| Formato | Contenido |
|---|---|
| **Excel (.xlsx)** | Un libro con una hoja por pestaña, cabecera fija, filtros automáticos y anchos ajustados |
| **CSV (coma)** | Un ZIP con un CSV por pestaña |
| **CSV (punto y coma)** | Igual, separado por `;`: es lo que espera Excel con configuración regional española |
| **Informe ejecutivo (HTML)** | Resumen de una página, imprimible a PDF (ver [Informe ejecutivo](#13-informe-ejecutivo)) |
| **Snapshot JSON** | El inventario completo, para compararlo o abrirlo más tarde |

El nombre propuesto es `gcetools-AAAAMMDD-HHMM` con la fecha del escaneo. Las cabeceras y los textos se exportan en el idioma activo; los nombres de las hojas (`gInfo`…) son siempre los mismos.

## 8. Referencia de pestañas

Cada pestaña es una tabla. Las que dependen de un módulo opcional o del coste solo tienen datos si ese módulo estaba activado.

#### gInfo — Instancias

Una fila por VM (equivale a vInfo).

Proyecto · VM · ID · Zona · Región · Estado · Machine type · Familia · vCPU · RAM (GB) · CPU platform · Aprovisionamiento · Mantenimiento host · Auto restart · Creada · Último arranque · Última parada · IP interna · IP externa · VPC · Subred · SO (licencia) · Imagen · Nº discos · Disco boot (GB) · Discos total (GB) · GPUs · Service account · Scopes · Labels · Network tags · Deletion protection · Secure Boot · vTPM · Confidential · MIG · Gestionada por · Clúster (GKE/Dataproc) · Node pool GKE · Hostname · Self link · Compute+GPU/mes (si encendida) · Licencias/mes · Discos/mes · Coste/mes actual · Notas de coste · Coste real mes (BigQuery)

- **Aprovisionamiento:** STANDARD, SPOT o PREEMPTIBLE.
- **SO (licencia):** el sistema deducido de la licencia del disco de arranque; si no tiene, la imagen de origen. El SO real que reporta el agente está en gTools.
- **Gestionada por:** GKE, Dataproc, Dataflow, Batch o MIG, según los labels y metadatos que pone cada servicio. Para GKE y Dataproc se rellenan el clúster y, en GKE, el node pool.
- **Columnas de coste:** "Compute+GPU/mes" es lo que costaría encendida todo el mes; "Coste/mes actual" es compute más licencias más discos si está encendida, y solo discos si está parada.

#### gCPU — CPU

Detalle de CPU por VM (vCPU).

Proyecto · VM · Zona · Machine type · vCPU · Threads/core · Cores visibles · CPU platform · Min CPU platform · Custom · Shared core · Nested virt.

#### gMemory — Memoria

Detalle de memoria por VM (vMemory).

Proyecto · VM · Zona · Machine type · RAM (GB) · GB/vCPU · Custom · Extended memory

#### gDisk — Discos de VM

Discos conectados a cada VM (vDisk).

Proyecto · VM · Zona · Device · Disco · Boot · Auto delete · Modo · Interfaz · Tipo · Tamaño (GB) · IOPS aprov. · MB/s aprov. · Cifrado · Imagen origen · Licencias

**Cifrado** puede ser Google (claves de Google), CMEK (Cloud KMS) o CSEK (clave del cliente).

#### gNetwork — NICs

Interfaces de red por VM (vNetwork).

Proyecto · VM · Zona · NIC · Tipo NIC · VPC · Proyecto VPC · Subred · IP interna · Alias IP · IP externa · Tier · Stack · IPv6 · IP forwarding

**Proyecto VPC** es distinto del proyecto de la VM cuando se usa Shared VPC.

#### gDisks — Todos los discos

Inventario de discos, incluidos los huérfanos (vDatastore).

Proyecto · Disco · Ubicación · Tipo · Tamaño (GB) · Estado · VMs · Huérfano · Creado · Último attach · Último detach · IOPS aprov. · MB/s aprov. · Cifrado · Imagen origen · Snapshot origen · Snapshot schedule · Réplicas · Arquitectura · Labels · Coste/mes · Coste real mes (BigQuery)

- **Huérfano:** el disco no está conectado a ninguna VM.
- **Snapshot schedule:** las resource policies de snapshots programados del disco.
- **Réplicas:** las zonas de los discos regionales.

#### gSnapshot — Snapshots

Snapshots de disco (vSnapshot).

Proyecto · Snapshot · Disco origen · Tamaño disco (GB) · Almacenado (GB) · Estado · Creado · Antigüedad (días) · Tipo · Por schedule · Ubicación · Cifrado · Labels · Coste/mes

**Almacenado (GB)** es lo que ocupa realmente el snapshot, y lo que se factura. **Por schedule** indica si lo creó un snapshot schedule.

#### gLicense — Licencias

Licencias de SO y software en discos (vLicense).

Proyecto · VM · Zona · Disco · Categoría · Licencia · Proyecto licencia

**Categoría** agrupa las licencias: Windows, SQL Server, RHEL, SLES, Ubuntu Pro, Ubuntu, Debian, Rocky Linux, CentOS, Container-Optimized OS, Herramienta (licencias que añaden las herramientas de migración) u Otra.

#### gMIG — MIGs

Managed instance groups y autoscaling (vCluster).

Proyecto · MIG · Ubicación · Ámbito · Template · Target size · Estable · Autoscaling · Modo · Mín. · Máx. · CPU objetivo · Health check · Update policy · Nombre base

#### gTemplate — Templates

Instance templates.

Proyecto · Template · Creado · Machine type · Tipo disco · Disco (GB) · Imagen · Proyecto imagen · VPC · Service account · Aprovisionamiento · Labels

Si la imagen es una familia, se muestra como `family/<nombre>`.

#### gVPC — VPCs

Redes VPC (vSwitch).

Proyecto · VPC · Modo subredes · Routing · MTU · Nº subredes · Peerings · Creada · Descripción

#### gSubnet — Subredes

Subredes (dvPort).

Proyecto · Subred · Región · VPC · CIDR · Rangos secundarios · Gateway · Private Google Access · Flow logs · Purpose · Stack · IPv6

#### gFirewall — Firewall

Reglas de firewall VPC.

Proyecto · Regla · VPC · Dirección · Prioridad · Acción · Protocolos/puertos · Orígenes · Tags/SA origen · Destinos · Tags/SA destino · Deshabilitada · Logging · Abierta a Internet

**Abierta a Internet** marca las reglas de entrada que permiten tráfico desde `0.0.0.0/0` o `::/0` y están habilitadas.

#### gAddress — IPs

Direcciones IP reservadas.

Proyecto · Nombre · IP · Tipo · Ubicación · Estado · Usada por · Tier · Purpose · Creada · Coste/mes

#### gImage — Imágenes

Imágenes custom y machine images (vCD).

Proyecto · Nombre · Tipo · Familia · Estado · Deprecación · Tamaño disco (GB) · Almacenado (GB) · Creada · Antigüedad (días) · Origen · Licencias · Ubicación · Arquitectura · Labels · Coste/mes

#### gReservation — Reservas

Reservas de capacidad zonal.

Proyecto · Reserva · Zona · Machine type · Reservadas · En uso · Sin usar (%) · Solo específica · GPUs · Local SSD (GB) · Estado · Commitment · Compartida · Creada

#### gCommitment — Commitments

Committed use discounts (CUD) por recurso.

Proyecto · Commitment · Región · Plan · Tipo · Categoría · Estado · Inicio · Fin · Días restantes · vCPU · RAM (GB) · Otros recursos · Auto renovación

#### gSoleTenant — Sole-tenant

Nodos sole-tenant y VMs alojadas (vHost).

Proyecto · Node group · Zona · Node template · Nodo · Tipo de nodo · Estado · Nº VMs · VMs · CPU overcommit · Mantenimiento · Autoscaling

#### gQuota — Cuotas

Cuotas regionales de Compute con uso mayor que cero.

Proyecto · Región · Métrica · Uso · Límite · Uso (%)

#### gTools — SO y agentes *(módulo OS Config)*

Inventario del SO invitado vía OS Config (vTools).

Proyecto · VM · Zona · Sistema operativo · Familia SO · Versión · Kernel · Arquitectura · Hostname · Agente OS Config · Actualizado

#### gPerf — Rendimiento *(módulo Métricas)*

CPU y memoria (Cloud Monitoring); la memoria requiere Ops Agent.

Proyecto · VM · Zona · Estado · Machine type · vCPU · RAM (GB) · Días · CPU media (%) · CPU p95 (%) · CPU máx (%) · RAM media (%) · RAM p95 (%) · RAM máx (%) · Ops Agent · Muestras

Los valores se calculan sobre muestras horarias del periodo elegido. **p95** es el valor por debajo del cual está el 95 % del tiempo: es mejor indicador que la media para dimensionar. **Muestras** es el número de horas con datos.

#### gRecommend — Recomendaciones *(módulo Recomendaciones)*

Recomendaciones activas de Google Recommender.

Proyecto · Impacto · Tipo · Subtipo · Recurso · Ubicación · Descripción · Ahorro mensual · Moneda · Prioridad · Estado · Actualizada

#### gCost — Coste *(módulo Coste estimado)*

Coste mensual estimado a precio de lista, sin SUD, CUD ni descuentos.

Proyecto · VMs encendidas · Compute+GPU · Licencias · Discos · Snapshots · Imágenes · IPs sin uso · Total/mes · Moneda

La última fila, **TOTAL**, suma todos los proyectos.

#### gCostReal — Coste real *(coste real con BigQuery)*

Coste neto real de Compute Engine según la exportación de facturación a BigQuery.

Proyecto · Mes · Coste bruto · Créditos (CUD, SUD, descuentos) · Coste neto · Atribuido a VMs y discos · No atribuido (red, IPs, snapshots, commitments…) · Estimado lista · Neto − estimado · Diferencia (%) · Moneda

#### gProject — Proyectos

Proyectos escaneados (vSource).

Proyecto · Número · Nombre · Padre · Estado · Resultado escaneo · VMs · VMs encendidas · vCPU · RAM (GB) · Discos · Discos (GB) · Labels · Coste/mes · OS Login (proyecto) · Claves SSH de proyecto · Puerto serie (proyecto) · VMs expuestas a Internet

#### gHealth — Salud

Hallazgos de buenas prácticas (vHealth). Ver [Reglas de salud](#9-reglas-de-salud-ghealth).

Severidad · Categoría · Regla · ID de regla · Proyecto · Recurso · Ubicación · Detalle

Los hallazgos se ordenan por severidad: Alta, Media y Baja.

#### gSecurity — Seguridad

Postura de seguridad por VM: exposición a Internet, acceso e identidad. Una fila por VM.

Proyecto · VM · Zona · Estado · Gestionada por · Problemas · IP externa · Expuesta a Internet · Puertos sensibles abiertos · Reglas que los abren · Service account · SA por defecto · Scope cloud-platform · OS Login · Acepta claves SSH de proyecto · Claves SSH en la VM · Puerto serie · IP forwarding · Secure Boot · vTPM · Integrity monitoring · Confidential · Cifrado de discos · Notas

- **Expuesta a Internet:** la VM tiene IP externa y alguna regla de firewall VPC deja entrar desde `0.0.0.0/0` (o `::/0`) a un puerto sensible: SSH (22), RDP (3389), Telnet, FTP, VNC, SMB, RPC, NetBIOS, Docker (2375), SQL Server, Oracle, MySQL, PostgreSQL, MongoDB, Redis, Elasticsearch o Memcached.
- **Cómo se evalúa:** se cruzan las reglas de la VPC de cada NIC (también las del proyecto host de una Shared VPC) que aplican a la VM por network tag, service account o a todas las instancias. Para cada puerto gana la regla con mayor prioridad (número menor) y, a igual prioridad, DENY gana a ALLOW, como en GCP. **Reglas que los abren** indica qué reglas ALLOW deciden; las de otro proyecto aparecen como `proyecto/regla`.
- **Límites:** no se leen las políticas de firewall jerárquicas ni las network firewall policies, ni se tiene en cuenta el acceso a través de balanceadores. Si el proyecto host de la VPC no se escaneó, la exposición queda vacía y **Notas** lo indica.
- **OS Login, claves SSH y puerto serie:** combinan los metadatos de la VM con los del proyecto (los de la VM mandan). **Acepta claves SSH de proyecto** es Sí cuando el proyecto tiene claves, la VM no tiene `block-project-ssh-keys` y OS Login está desactivado. Una política de organización que exija OS Login no es visible para gceTools.
- **Cifrado de discos:** Google, CMEK o CSEK; si los discos de la VM usan varios tipos, aparecen todos.
- **Problemas:** cuenta cuántas de estas comprobaciones fallan: expuesta a Internet, SA por defecto, OS Login desactivado, acepta claves SSH de proyecto, puerto serie, IP forwarding, Secure Boot desactivado y vTPM o integrity monitoring desactivados. En las VMs gestionadas no cuentan OS Login ni IP forwarding, porque esos servicios los configuran así a propósito.

gProject añade, por proyecto, **OS Login (proyecto)**, **Claves SSH de proyecto**, **Puerto serie (proyecto)** y **VMs expuestas a Internet**.

#### gErrors — Errores

Proyectos o APIs que no se pudieron leer.

Proyecto · Colector · Tipo · Mensaje

- **Colector:** qué se intentaba leer (disks, instances, osconfig, monitoring, cloudasset, pricing, billing…).
- **Tipo:** `permission_denied` (falta un permiso), `api_disabled` (la API no está habilitada), `not_found` u `error` (cualquier otro).

#### gMetaData — Metadatos

Información del escaneo (vMetaData): herramienta y versión, identidad, método de autenticación, scope, módulos activados (incluido el modo rápido), quota project, moneda y base de los costes, elementos sin tarifa, coste real leído, reglas desactivadas, hallazgos ocultos por labels, inicio, fin y duración, proyectos seleccionados y OK, errores y si se canceló.

## 9. Reglas de salud (gHealth)

Cada regla tiene un **ID estable** que no cambia entre versiones ni idiomas. Se usa para desactivarla, para exceptuar recursos y en el CLI.

| ID | Regla | Categoría | Severidad | Umbral (por defecto) | Excepción en |
|---|---|---|---|---|---|
| `orphan-disk` | Disco huérfano | Coste | Alta | — | disco o proyecto |
| `stopped-vm` | VM parada más de N días | Coste | Media | stoppedDays (30) | VM o proyecto |
| `unused-ip` | IP externa reservada sin uso | Coste | Media | — | proyecto |
| `unused-reservation` | Reserva con capacidad sin usar | Coste | Media | — | proyecto |
| `cud-expiring` | Commitment (CUD) que vence pronto sin renovación | Coste | Media | cudDays (60) | proyecto |
| `cpu-low` | VM infrautilizada (CPU p95 baja) | Coste | Media | cpuLowPct (10 %) | VM o proyecto |
| `old-family` | Familia de máquina antigua (N1) | Coste | Baja | — | VM o proyecto |
| `old-snapshots` | Snapshots manuales antiguos | Coste | Baja | snapshotDays (90) | proyecto |
| `firewall-open` | Regla de firewall abierta a Internet | Seguridad | Alta / Media | — | proyecto |
| `default-sa` | Service account por defecto de Compute | Seguridad | Media / Alta | — | VM o proyecto |
| `public-ip` | VM con IP pública | Seguridad | Media | — | VM o proyecto |
| `no-secure-boot` | Secure Boot deshabilitado | Seguridad | Baja | — | VM o proyecto |
| `internet-exposed` | VM expuesta a Internet en puertos sensibles | Seguridad | Alta | — | VM o proyecto |
| `project-ssh-keys` | VM que acepta claves SSH de proyecto | Seguridad | Media | — | VM o proyecto |
| `serial-port` | Acceso por puerto serie habilitado | Seguridad | Media | — | VM o proyecto |
| `no-os-login` | OS Login deshabilitado | Seguridad | Baja | — | VM o proyecto |
| `ip-forward` | IP forwarding habilitado | Seguridad | Baja | — | VM o proyecto |
| `no-shielded-vm` | vTPM o integrity monitoring deshabilitado | Seguridad | Baja | — | VM o proyecto |
| `template-image` | Template con imagen obsoleta, deprecada o antigua | Seguridad / Operación | Alta, Media o Baja | imageDays (180) | template o proyecto |
| `snapshot-schedule` | VM con discos sin snapshot schedule | Protección de datos | Baja | — | VM o proyecto |
| `cpu-high` | CPU saturada | Rendimiento | Media | cpuHighPct (90 %) | VM o proyecto |
| `mem-high` | Memoria saturada | Rendimiento | Media | memHighPct (90 %) | VM o proyecto |
| `boot-pd-standard` | Disco de arranque pd-standard | Rendimiento | Baja | — | VM o proyecto |
| `quota` | Cuota regional casi agotada | Capacidad | Media / Alta | quotaPct (80 %) | proyecto |
| `no-labels` | VM sin labels | Gobierno | Baja | — | VM o proyecto |
| `stopped-no-disks` | VM parada sin discos | Gobierno | Baja | — | VM o proyecto |
| `no-deletion-protection` | Deletion protection deshabilitada | Operación | Baja | — | VM o proyecto |
| `no-ops-agent` | Sin Ops Agent | Operación | Baja | — | VM o proyecto |

### 9.1 Qué detecta cada regla

- **orphan-disk:** discos sin ninguna VM conectada. Siguen facturando. El detalle incluye tamaño, tipo, último detach y coste mensual.
- **stopped-vm:** VMs en estado TERMINATED desde hace más de N días. La VM no factura compute, pero sus discos sí; el detalle muestra ese coste.
- **unused-ip:** IPs externas reservadas que no usa nadie. Se facturan aunque no se usen.
- **unused-reservation:** reservas activas con parte de la capacidad sin usar. Se factura igual.
- **cud-expiring:** commitments activos que vencen en N días o menos y no se renuevan solos.
- **cpu-low:** VMs encendidas con al menos una semana de métricas y CPU p95 por debajo del umbral. Candidatas a un machine type menor.
- **old-family:** VMs N1. Las familias N2, N2D, N4 o C3 suelen ofrecer mejor precio y rendimiento.
- **old-snapshots:** snapshots creados a mano (no por schedule) de más de N días. Se agrupan **por proyecto** en un solo hallazgo, porque las herramientas de backup crean miles; el detalle en gSnapshot.
- **firewall-open:** reglas de entrada habilitadas que permiten tráfico desde `0.0.0.0/0` o `::/0`. Son **Alta** si abren SSH (22), RDP (3389), todos los protocolos o todos los puertos TCP; **Media** en el resto.
- **default-sa:** VMs con la service account por defecto de Compute (`…-compute@developer.gserviceaccount.com`). Es **Alta** si además tiene el scope `cloud-platform` (acceso completo a la API).
- **public-ip:** VMs con IP externa.
- **no-secure-boot:** VMs sin Secure Boot de Shielded VM.
- **internet-exposed:** VMs con IP externa y algún puerto sensible (SSH, RDP, bases de datos…) abierto a Internet por el firewall VPC, según el análisis de gSecurity. El detalle indica los puertos, la IP y las reglas.
- **project-ssh-keys:** VMs en las que entra cualquiera con una clave SSH en los metadatos del proyecto: OS Login desactivado y sin `block-project-ssh-keys`.
- **serial-port:** VMs con `serial-port-enable=TRUE` (en la VM o en el proyecto): permite entrar por la consola serie.
- **no-os-login:** VMs sin OS Login: el acceso SSH se gestiona con claves en metadatos en vez de con IAM. Si una política de organización ya exige OS Login, desactiva la regla.
- **ip-forward:** VMs con IP forwarding, que solo necesitan los appliances de red, NAT o VPN.
- **no-shielded-vm:** VMs con vTPM o integrity monitoring de Shielded VM desactivados.
- **template-image:** templates que crean VMs desde una imagen custom del escaneo que está **obsoleta o eliminada** (Alta: ya no se pueden crear VMs), **deprecada** (Media) o es más antigua que N días (Baja: probablemente sin parches). Las imágenes públicas y las familias no se evalúan.
- **snapshot-schedule:** VMs con discos sin snapshot schedule. Un hallazgo por VM, con la lista de discos. Si los backups se hacen con otra herramienta (Backup and DR, Veeam…), desactiva la regla o usa la excepción por label.
- **cpu-high / mem-high:** CPU o RAM p95 por encima del umbral. La RAM requiere Ops Agent.
- **boot-pd-standard:** discos de arranque pd-standard. pd-balanced es más rápido y apenas más caro.
- **quota:** cuotas regionales por encima del umbral; **Alta** a partir del 95 %.
- **no-labels:** VMs sin labels. Dificulta repartir costes.
- **stopped-no-disks:** VMs paradas sin discos. Suelen ser restos de migraciones.
- **no-deletion-protection:** VMs que se pueden borrar sin protección.
- **no-ops-agent:** VMs encendidas con métricas pero sin Ops Agent: no hay métricas de memoria ni de disco.

### 9.2 VMs gestionadas

Para las VMs de GKE, Dataproc, Dataflow, Batch y de MIGs no se evalúan siete reglas que no tienen sentido en máquinas desechables o que esos servicios configuran a propósito:

- `stopped-vm`
- `stopped-no-disks`
- `no-deletion-protection`
- `snapshot-schedule`
- `no-ops-agent`
- `no-os-login`
- `ip-forward` (los nodos de GKE lo necesitan)

El resto (IP pública, service account por defecto, Secure Boot…) sí se evalúa.

### 9.3 Desactivar reglas y exceptuar recursos

- **Desactivar una regla** en todo el escaneo: en **Proyectos → Reglas de salud**, o con `--disable-rules id1,id2` en el CLI.
- **Exceptuar un recurso** sin desactivar la regla: ponle un label al recurso (VM, disco o template) o a su proyecto, según la columna "Excepción en":

| Label | Efecto |
|---|---|
| `gcetools-ignore=<id>` | Omite esa regla |
| `gcetools-ignore-<id>=true` | Igual; permite combinar varias reglas en el mismo recurso |
| `gcetools-ignore=all` | Omite todas las reglas |

Ejemplo: el label `gcetools-ignore=snapshot-schedule` en un proyecto evita los avisos de discos sin snapshot schedule de ese proyecto.

gMetaData muestra qué reglas estaban desactivadas y cuántos hallazgos se ocultaron por labels.

## 10. Costes

### 10.1 Coste estimado

El módulo **Coste estimado** descarga el catálogo público de precios de Compute Engine (Cloud Billing Catalog API). El catálogo se guarda 24 horas en caché.

- **Cálculo:** precio de lista on-demand × 730 horas al mes, en la moneda elegida.
- **VM encendida:** compute + GPU + licencias de pago + discos.
- **VM parada:** solo discos.
- **También se calcula:** snapshots (por GB almacenado), imágenes, machine images e IPs sin uso.
- **Licencias de pago incluidas:** Windows Server, SQL Server, RHEL (también for SAP), SLES (también for SAP) y Ubuntu Pro (también FIPS y Minimal Pro). Container-Optimized OS y las distribuciones libres no tienen coste de licencia.
- **No incluye:** descuentos por uso sostenido (SUD), CUDs, precios negociados, red/egress ni licencias BYOL/BYOS. Es una referencia, no la factura.
- **Lo que no encuentra precio** aparece en la columna "Notas de coste" de gInfo y en "Sin tarifa" de gMetaData.

### 10.2 Coste real (BigQuery)

Si tu organización exporta la facturación a BigQuery ([cómo activarlo](https://cloud.google.com/billing/docs/how-to/export-data-bigquery)), gceTools lee el **coste neto real** de Compute Engine de un mes, con CUD, SUD, créditos y descuentos aplicados:

- **gCostReal:** por proyecto, coste bruto, créditos, neto, estimado de lista y diferencia.
- **Con la exportación detallada** (`gcp_billing_export_resource_v1_…`) además separa lo atribuido a VMs y discos de lo no atribuido (red, IPs, snapshots, commitments, recursos borrados), y rellena la columna "Coste real mes (BigQuery)" de gInfo y gDisks.

**Es la única función que puede costar dinero:** BigQuery cobra por datos leídos (1 TiB gratis al mes por cuenta de facturación). Para controlarlo, gceTools:

1. filtra por las particiones del mes;
2. hace antes una estimación gratuita (*dry run*);
3. no ejecuta la consulta si leería más del límite (50 GB por defecto).

gMetaData muestra los GB leídos.

**Permisos:** `roles/bigquery.jobUser` en el proyecto de la tabla y `roles/bigquery.dataViewer` en el dataset.

## 11. Comparar inventarios

La página **Comparar** muestra las altas, bajas y cambios entre dos inventarios. No necesita conexión a Google Cloud.

1. **Anterior (A):** pulsa **Elegir snapshot JSON** y selecciona el inventario de referencia, por ejemplo el del mes pasado.
2. **Nuevo (B):** elige otro snapshot JSON o **Usar el inventario actual** (el escaneo abierto).
3. La comparación se hace sola. **Cambiar** sustituye cualquiera de los dos.

También se puede comparar desde **Historial** sin exportar nada.

**Resultado:**

- **Cifras:** altas, bajas, recursos modificados, cambios relevantes y diferencia de coste mensual.
- **Pestañas:**
  - **Resumen:** altas, bajas y cambios por tipo de recurso.
  - **Altas / Bajas:** recursos nuevos y eliminados, con un resumen de cada uno.
  - **Cambios:** un campo por fila, con valor anterior y nuevo.
  - **Metadatos:** los inventarios comparados, la diferencia de coste y lo que no se pudo comparar.
- **Exportar:** a Excel, CSV o JSON.

**Cambios relevantes.** Se marcan y aparecen primero:

- VMs redimensionadas;
- cambios de service account o de Shielded VM;
- IPs públicas nuevas;
- reglas de firewall abiertas a Internet;
- discos que quedan huérfanos;
- hallazgos nuevos de severidad alta;
- cambios en commitments.

**Sin falsos positivos:**

- **Permisos:** un tipo de recurso solo se compara en un proyecto si se leyó bien en los dos escaneos. Si un proyecto falló en uno, aparece en Metadatos como "No comparado" en lugar de como cientos de bajas.
- **Ruido que se ignora:** fechas de último arranque y parada, antigüedades, uso de cuotas, métricas, gCPU/gMemory (repiten gInfo), gCost, gErrors, gMetaData y los snapshots creados por schedule (rotan a diario).
- **Costes:** solo se comparan si los dos inventarios usan la misma moneda.
- **Columnas:** se comparan solo las que existen en los dos inventarios, así que un snapshot de una versión anterior no genera cambios falsos por columnas nuevas.

> Compara inventarios hechos en el mismo idioma: los textos traducidos (severidad, categoría de los hallazgos…) cambian entre un escaneo en español y otro en inglés, y saldrían como cambios.

## 12. Historial

Cada escaneo de la app se guarda automáticamente en tu equipo como JSON comprimido. Se conservan los 100 más recientes; al guardar uno nuevo, se borra el más antiguo. La casilla **Guardar cada escaneo automáticamente** lo activa o desactiva. La pantalla indica la carpeta donde se guardan.

- **Gráficas de tendencia:** VMs, vCPU, coste (real si existe, si no el estimado) y hallazgos de severidad alta, con un punto por escaneo. Al pasar el ratón se ve el valor y la fecha.
- **Tabla:** fecha, perfil, proyectos, VMs, vCPU, coste, hallazgos por severidad y estado (completo o parcial).
- **Con un escaneo seleccionado:** **Abrir en Inventario** lo carga como si acabaras de escanearlo, para verlo o exportarlo. **Comparar con el anterior** lo compara con el escaneo previo.
- **Con dos seleccionados:** **Comparar** los compara entre sí.
- **Eliminar del historial** borra los seleccionados, tras confirmar.

Al abrir un escaneo antiguo, las cabeceras se muestran en el idioma activo.

## 13. Informe ejecutivo

**Inventario → Exportar → Informe ejecutivo (HTML)** genera una página autocontenida (sin scripts ni recursos externos). Se abre en cualquier navegador y se imprime a PDF. Está pensada para dirección o para clientes:

- totales del inventario;
- coste estimado o real del mes;
- ahorro potencial según Recommender;
- los 10 hallazgos principales de severidad alta o media;
- coste por proyecto (los 10 mayores);
- proyectos que no se pudieron leer;
- si hay historial, los cambios desde el escaneo anterior (hasta 10 cambios relevantes).

En el CLI: `gcetools-cli scan --report informe.html`.

## 14. Acerca de y actualizaciones

- **Versión y licencia** de la app.
- **Sitio web:** documentación, novedades y descargas.
- **Invítame a un café:** gceTools es gratuito; si te ahorra tiempo, puedes apoyar su desarrollo.
- **Licencia GPL-3.0:** enlace al texto completo.
- **Actualizaciones:**
  - **Buscar actualizaciones** consulta la última release de GitHub; si hay una nueva, muestra sus novedades y el botón **Descargar**.
  - **Comprobar automáticamente al iniciar** hace la consulta al abrir la app.
  - La consulta es anónima y no envía datos tuyos ni del inventario.
- **Coste de uso de las APIs de Google Cloud:** la tabla de [Coste de uso de las APIs](#18-coste-de-uso-de-las-apis).
- **Componentes de terceros:** bibliotecas usadas y sus licencias.

## 15. Idioma

gceTools está en español e inglés. El idioma se cambia con el icono de traducción de la barra superior; la primera vez se usa el idioma del sistema. El cambio es inmediato: el inventario abierto se mantiene.

- **Interfaz:** todo el texto de la app.
- **Datos que se generan:** títulos de pestañas y columnas, estados, reglas y detalles de gHealth, metadatos, informe, exportaciones y mensajes de error. Se generan en el idioma activo **en el momento del escaneo o de la exportación**.
- **Nombres de las hojas** (`gInfo`, `gDisks`…), **IDs de reglas** y valores técnicos de Google (RUNNING, READY…): no cambian, para no romper scripts que lean los archivos.
- **Inventarios antiguos:** al abrir uno del historial o un snapshot JSON, las cabeceras se muestran en el idioma activo; los textos de las filas (por ejemplo, los detalles de los hallazgos) quedan en el idioma en que se escaneó.
- **CLI:** opción `--lang es|en` o variable `GCETOOLS_LANG`. Si no se indica, usa el idioma del sistema (`LC_ALL`, `LC_MESSAGES`, `LANG`).

## 16. Línea de comandos

`gcetools-cli` hace lo mismo que la app sin interfaz gráfica: sirve para tareas programadas, servidores y pipelines. Viene en todos los paquetes y funciona igual en Windows, macOS y Linux. En Linux y macOS el binario de la app también acepta los comandos (`gcetools scan …`); en Windows no, porque la app gráfica no escribe en la consola.

```
gcetools-cli [--lang es|en] <comando> [opciones]
```

| Comando | Para qué |
|---|---|
| `scan` | Escanear proyectos y exportar el inventario |
| `diff` | Comparar dos snapshots JSON |
| `profiles` | Listar los perfiles guardados en la app |
| `rules` | Listar las reglas de gHealth y cómo exceptuar recursos |
| `login` | Iniciar sesión con Google sin gcloud |
| `logout` | Cerrar la sesión de Google guardada en el llavero |
| `version` | Mostrar la versión y si hay una más reciente |
| `help` | Ayuda general (cada comando acepta `--help`) |

**Opción global:** `--lang es|en` fija el idioma de los mensajes y de los archivos generados. También vale la variable `GCETOOLS_LANG`; por defecto se usa el idioma del sistema. Puede ir en cualquier posición.

**Códigos de salida:** 0 correcto; 1 error; 2 opciones no válidas; 3 hay cambios relevantes (solo `diff --fail-on-relevant`). Los mensajes de progreso y los errores van a la salida de error (stderr).

### 16.1 scan

```
gcetools-cli scan [opciones]
```

**Autenticación:**

| Opción | Descripción | Por defecto |
|---|---|---|
| `--auth` | `adc`, `sa`, `impersonate` u `oauth` (tras `gcetools-cli login`) | `adc` |
| `--key` | Ruta a la clave JSON de la service account (`--auth sa`) | — |
| `--impersonate` | Email de la service account a impersonar (`--auth impersonate`) | — |

**Qué proyectos:**

| Opción | Descripción | Por defecto |
|---|---|---|
| `--projects` | IDs de proyecto separados por comas | todos los visibles |
| `--query` | Filtro de Resource Manager, por ejemplo `"parent:folders/123"` | — |
| `--discover` | Escanea solo los proyectos con recursos de Compute según Cloud Asset Inventory (`organizations/N`, `folders/N`) | — |
| `--profile` | Usa un perfil guardado en la app; las opciones explícitas tienen prioridad | — |

**Qué leer:**

| Opción | Descripción | Por defecto |
|---|---|---|
| `--modules` | Módulos opcionales separados por comas: `osconfig`, `monitoring`, `recommender`, `pricing`; `none` los desactiva | los cuatro |
| `--metrics-days` | Días de métricas de Cloud Monitoring | 30 |
| `--currency` | Moneda del coste estimado (USD, EUR, CLP…) | USD |
| `--fast` | Modo rápido con Cloud Asset Inventory; con `--discover`, una sola lectura para todo el ámbito | desactivado |
| `--billing-table` | Tabla de exportación de facturación en BigQuery (`proyecto.dataset.tabla`) para el coste real | — |
| `--billing-month` | Mes de facturación `AAAAMM` | el mes anterior |
| `--billing-max-gb` | No ejecutar la consulta de BigQuery si leería más GB que esto | 50 |
| `--disable-rules` | Reglas de gHealth a desactivar, separadas por comas | — |

**Salida:**

| Opción | Descripción | Por defecto |
|---|---|---|
| `--out` | Archivo de salida: `.xlsx`, `.zip` o `.csv` (ZIP de CSV), `.json` | `gcetools-AAAAMMDD-HHMM.xlsx` |
| `--format` | `xlsx`, `csv` o `json`, si no quieres deducirlo de la extensión | según la extensión |
| `--csv-sep` | Separador CSV (usa `;` para Excel en español) | `,` |
| `--report` | Genera además el informe ejecutivo HTML en esta ruta | — |
| `--save-history` | Guarda también el resultado en el historial local (el mismo de la app) | desactivado |

**Rendimiento:**

| Opción | Descripción | Por defecto |
|---|---|---|
| `--concurrency` | Proyectos en paralelo | 8 |
| `--rps` | Peticiones por segundo máximas | 20 |

**Ejemplos:**

```bash
# Todo lo visible con ADC, a Excel
gcetools-cli scan --out inventario.xlsx

# Una carpeta con una service account, CSV para Excel en español
gcetools-cli scan --auth sa --key ./sa.json --query "parent:folders/123456" --out inv.zip --csv-sep ";"

# Dos proyectos, impersonando una service account
gcetools-cli scan --auth impersonate --impersonate inventory@prj.iam.gserviceaccount.com --projects a,b

# Organización completa, solo proyectos con Compute, en modo rápido
gcetools-cli scan --discover organizations/123456 --fast --out inventario.xlsx

# Con un perfil de la app, guardando en el historial y generando el informe
gcetools-cli scan --profile produccion --save-history --report informe.html --out inventario.xlsx

# Coste real de octubre desde la exportación de facturación
gcetools-cli scan --billing-table finops-prj.billing.gcp_billing_export_resource_v1_XXXXXX --billing-month 202610

# Rápido y sin módulos opcionales, en inglés
gcetools-cli --lang en scan --modules none --out inventory.json
```

### 16.2 diff

```
gcetools-cli diff [opciones] anterior.json nuevo.json
```

Compara dos snapshots JSON con las mismas reglas que la página Comparar.

| Opción | Descripción | Por defecto |
|---|---|---|
| `--out` | Archivo de salida (`.xlsx`, `.zip`/`.csv`, `.json`) | `gcetools-diff-AAAAMMDD-HHMM.xlsx` |
| `--format` | `xlsx`, `csv` o `json` | según la extensión |
| `--csv-sep` | Separador CSV | `,` |
| `--fail-on-relevant` | Termina con código 3 si hay cambios relevantes | desactivado |

Además del archivo, imprime en la consola el resumen y hasta 20 cambios relevantes, útil en los correos de cron.

```bash
gcetools-cli diff --out cambios.xlsx lunes.json viernes.json
gcetools-cli diff --fail-on-relevant base.json hoy.json   # para alertas o pipelines
```

### 16.3 login y logout

```bash
gcetools-cli login                                   # abre el navegador; la sesión queda en el llavero
gcetools-cli login --client client_secret_XXXX.json  # con el cliente OAuth propio de tu organización
gcetools-cli scan --auth oauth --out inventario.xlsx
gcetools-cli logout                                  # borra la sesión del llavero
```

La sesión es la misma que usa la app: si ya iniciaste sesión en la app, `--auth oauth` funciona sin `login`.

### 16.4 profiles, rules y version

```bash
gcetools-cli profiles   # perfiles guardados en la app y ruta de profiles.json
gcetools-cli rules      # las 22 reglas con su ID, severidad, categoría y umbral
gcetools-cli version    # versión instalada y si hay una más reciente
```

### 16.5 Tareas programadas

**Linux o macOS (cron), cada lunes a las 7:00:**

```bash
0 7 * * 1 /usr/bin/gcetools-cli scan --profile produccion --save-history --report "$HOME/informes/informe-$(date +\%F).html" --out "$HOME/informes/inventario-$(date +\%F).xlsx"
```

**Windows (Programador de tareas):** crea una tarea con la acción *Iniciar un programa*:

- **Programa:** `C:\Program Files\gceTools\gcetools-cli.exe` (o la carpeta donde lo instalaste).
- **Argumentos:** `scan --auth sa --key C:\gcetools\sa.json --profile produccion --out C:\informes\inventario.xlsx`

**Alertas de cambios:** guarda un snapshot JSON cada día y compáralo con el anterior. `--fail-on-relevant` devuelve 3 si hay cambios relevantes, lo que permite disparar un aviso desde el script o el pipeline:

```bash
gcetools-cli scan --auth sa --key sa.json --out hoy.json
gcetools-cli diff --fail-on-relevant --out cambios.xlsx ayer.json hoy.json || echo "Hay cambios relevantes"
```

En tareas programadas usa una service account (`--auth sa` o `--auth impersonate`): las sesiones de usuario pueden caducar o exigir volver a autenticarse.

## 17. Permisos IAM y APIs

### 17.1 Roles por función

| Función | Rol necesario | Dónde |
|---|---|---|
| Inventario básico | `roles/compute.viewer` | Organización, carpeta o proyecto |
| Listar proyectos y su jerarquía | `roles/browser` | Organización o carpeta |
| SO y agentes (gTools) | `roles/osconfig.inventoryViewer` | Proyecto |
| Métricas (gPerf) | `roles/monitoring.viewer` | Proyecto |
| Recomendaciones (gRecommend) | `roles/recommender.computeViewer` | Proyecto |
| Detectar proyectos y modo rápido | `roles/cloudasset.viewer` | Organización, carpeta o proyecto |
| Coste real | `roles/bigquery.jobUser` y `roles/bigquery.dataViewer` | Proyecto de la tabla y dataset |
| Impersonación | `roles/iam.serviceAccountTokenCreator` | Sobre la service account |

Para una visión global, asigna los roles a nivel de organización o carpeta. Sin `resourcemanager.projects.list` puedes añadir proyectos a mano por ID.

### 17.2 Quota project

Algunas APIs (Recommender, Cloud Asset Inventory, Cloud Billing Catalog) cargan la cuota a un proyecto "de consumo" en lugar de al proyecto que se lee:

- **Credenciales de gcloud:** el `quota_project_id` de ADC (`gcloud auth application-default set-quota-project <proyecto>`).
- **Service account:** su propio proyecto.
- **Iniciar sesión con Google:** el proyecto del cliente OAuth (el de gceTools, o el de tu organización si usas uno propio).

Ese proyecto debe tener habilitadas las APIs correspondientes.

### 17.3 APIs que habilitar

En los **proyectos que se escanean**:

- **Compute Engine API:** imprescindible.
- **OS Config API:** para gTools.
- **Cloud Monitoring API:** suele venir habilitada.

En el **quota project**:

- **Recommender API**, **Cloud Asset API** y **Cloud Billing API** (catálogo de precios), según los módulos que uses.
- **BigQuery API:** en el proyecto donde se ejecuta la consulta de facturación.

Si usas un **cliente OAuth propio**, su proyecto debe tener habilitadas todas las APIs que usa gceTools:

```bash
gcloud services enable --project <proyecto-del-cliente> cloudresourcemanager.googleapis.com compute.googleapis.com monitoring.googleapis.com osconfig.googleapis.com recommender.googleapis.com cloudasset.googleapis.com cloudbilling.googleapis.com bigquery.googleapis.com
```

## 18. Coste de uso de las APIs

gceTools solo hace lecturas. Casi todas las APIs son gratuitas.

| API | Para qué | Coste |
|---|---|---|
| Compute Engine | Inventario de VMs, discos, red, snapshots… | Gratis (solo cuotas por minuto) |
| Resource Manager | Listar proyectos | Gratis |
| Cloud Billing Catalog | Precios del coste estimado (caché de 24 h) | Gratis |
| Recommender | Recomendaciones | Gratis |
| Cloud Asset Inventory | Detectar proyectos y modo rápido | Gratis (con cuotas por minuto) |
| Cloud Monitoring | Métricas de CPU y RAM | 0,50 USD por millón de series; el primer millón al mes por cuenta de facturación es gratis |
| OS Config | SO y agentes | Leer es gratis (VM Manager cobra por VM con agente activo, no por gceTools) |
| BigQuery | Coste real (opcional, desactivado por defecto) | Por datos leídos (1 TiB gratis al mes); gceTools estima antes y respeta un límite de GB |
| GitHub | Buscar actualizaciones | Gratis |

Como referencia, un escaneo pide unas 2 series de Monitoring por VM: con 100 VMs harían falta unos 5.000 escaneos al mes para salir del tramo gratuito.

## 19. Archivos y datos locales

| Qué | Linux | Windows | macOS |
|---|---|---|---|
| Historial | `~/.local/share/gcetools/history` | `%LocalAppData%\gcetools\history` | `~/Library/Application Support/gcetools/history` |
| Perfiles (`profiles.json`, permisos 0600) | `~/.config/gcetools` | `%AppData%\gcetools` | `~/Library/Application Support/gcetools` |
| Caché del catálogo de precios | `~/.cache/gcetools` | `%LocalAppData%\gcetools` | `~/Library/Caches/gcetools` |
| Sesión de Google | Secret Service (servicio `gcetools`) | Credential Manager (`gcetools`) | Keychain (`gcetools`) |

- **Historial:** `index.json` más un archivo `<id>.json.gz` por escaneo.
- **Preferencias de la interfaz** (idioma, tema, guardar historial, ocultar VMs gestionadas, comprobar actualizaciones): se guardan en el almacenamiento local de la app.
- **Clave de service account cargada en la app:** solo en memoria; nunca se escribe en disco.

Para empezar de cero, borra esas carpetas y cierra la sesión de Google desde la app (o elimina la entrada `gcetools` del llavero).

## 20. Privacidad y seguridad

- **Sin servidores propios:** gceTools no envía datos a su autor ni a terceros. Solo se comunica con las APIs de Google y, para buscar actualizaciones, con la API pública de GitHub, de forma anónima y desactivable.
- **Sin rastreo:** no usa analíticas, telemetría, cookies ni publicidad.
- **Solo lectura:** el acceso a Google Cloud es de solo lectura.
- **Credenciales:** los tokens de sesión van al llavero del sistema; las claves de service account solo se mantienen en memoria; los perfiles no guardan secretos.
- **Código abierto:** GPL-3.0, auditable en [GitHub](https://github.com/jordilopezr/gcetools).

Política completa: [PRIVACY.md](../PRIVACY.md).

## 21. Solución de problemas

| Mensaje o síntoma | Causa | Solución |
|---|---|---|
| "La sesión de gcloud ha caducado o tu organización exige volver a autenticarse" | El token de ADC caducó o tu organización exige reautenticación periódica | `gcloud auth application-default login` y vuelve a conectar |
| "La sesión de Google ha caducado o se revocó el acceso" | Se revocó el acceso o la sesión caducó (mientras la app está en modo de prueba, cada 7 días) | **Iniciar sesión con Google** de nuevo |
| Google muestra "Acceso bloqueado" o "Esta app no está verificada" | La app aún no está verificada y tu cuenta no es usuaria de prueba, o tu organización bloquea apps externas | Usa otro método de conexión, pide al administrador que autorice gceTools o usa el cliente OAuth de tu organización |
| "Llavero no disponible" | No hay servicio de llavero (Linux sin GNOME Keyring o KWallet) | Instala y desbloquea uno, o usa otro método |
| No aparece ningún proyecto | La identidad no tiene `resourcemanager.projects.list` | Añade los proyectos por ID o pide `roles/browser` |
| "no se pudieron listar proyectos: … API has not been used in project N" | La Cloud Resource Manager API no está habilitada en el quota project (o en el del cliente OAuth) | Habilítala en ese proyecto y espera unos minutos |
| Proyecto "Compute API deshabilitada" | El proyecto no usa Compute Engine | Normal; para no escanearlo, usa **Detectar proyectos con Compute** |
| Proyecto "Sin permisos" | Falta `roles/compute.viewer` | Asigna el rol en el proyecto o en una carpeta superior |
| Proyecto "OK (sin osconfig)", "OK (sin recommender)"… | El módulo no está disponible en ese proyecto (API deshabilitada o falta un permiso) | El motivo exacto está en gErrors |
| gRecommend vacío con credenciales de usuario | Falta quota project, o no tiene la Recommender API | `gcloud auth application-default set-quota-project <proyecto>` y habilita la API en él |
| Error de Cloud Asset Inventory al detectar o en modo rápido | Falta `roles/cloudasset.viewer` en el ámbito o la API en el quota project | Asigna el rol o habilita la Cloud Asset API; el modo rápido recurre a la Compute API automáticamente |
| gPerf sin datos de RAM | Las VMs no tienen Ops Agent | Instala Ops Agent; la regla `no-ops-agent` las señala |
| Sin columnas de coste | El módulo Coste estimado está desactivado o no se pudo descargar el catálogo | Actívalo; el error, si lo hay, está en gErrors (colector `pricing`) |
| "la consulta leería X GB, más que el límite de N GB" | La consulta de coste real supera el límite configurado | Sube el límite, revisa el mes elegido o usa otra tabla |
| El escaneo es lento | Muchos proyectos o cuotas de la Compute API | Usa **Detectar proyectos con Compute**, el **modo rápido**, más paralelismo o menos módulos |
| Windows: "editor desconocido" | Los ejecutables aún no están firmados | **Más información → Ejecutar de todas formas** |
| macOS: "no se puede abrir porque es de un desarrollador no identificado" | Gatekeeper bloquea apps sin firmar | Clic derecho → Abrir, o `xattr -dr com.apple.quarantine /Applications/gceTools.app` |
| Linux: el AppImage no arranca | Faltan GTK 3 o WebKitGTK 4.1 | Instálalos (`libwebkit2gtk-4.1-0`) o usa el `.deb`/`.rpm` |
| El CSV se abre en una sola columna en Excel | Excel en español espera `;` | Exporta como **CSV (punto y coma)** o usa `--csv-sep ";"` |

Si el problema persiste, abre una incidencia en [GitHub](https://github.com/jordilopezr/gcetools/issues) con la versión (`gcetools-cli version`) y el contenido de gErrors, sin datos sensibles.

## 22. Preguntas frecuentes

**¿gceTools puede cambiar algo en mi Google Cloud?**
No. Solo pide el scope de solo lectura y solo hace consultas.

**¿Cuesta dinero usarlo?**
La app es gratuita. Las APIs que usa son gratuitas salvo Cloud Monitoring, cuyo tramo gratuito es amplio, y BigQuery si activas el coste real. Ver [Coste de uso de las APIs](#18-coste-de-uso-de-las-apis).

**¿Dónde están mis datos?**
En tu equipo. Ver [Archivos y datos locales](#19-archivos-y-datos-locales).

**¿Por qué el coste estimado no coincide con mi factura?**
Porque es precio de lista, sin descuentos por uso sostenido, CUDs ni precios negociados, y sin red. Para el coste real, usa la exportación de facturación a BigQuery.

**¿Puedo escanear varias organizaciones?**
Sí, si tu identidad tiene acceso: la lista de proyectos incluye todo lo que ve. Para escanear una organización concreta, usa el filtro o el ámbito de detección.

**¿Cada cuánto conviene escanear?**
Para seguimiento, una vez por semana o por mes, con el historial activado. Para detectar cambios, a diario con el CLI y `diff --fail-on-relevant`.

**¿El modo rápido da los mismos datos?**
Sí, para los tipos de recurso que cubre, con la salvedad de que Cloud Asset Inventory puede tardar unos minutos en reflejar los cambios más recientes. Para comprobarlo, escanea en los dos modos y compara los resultados.

**¿Puedo añadir mis propias reglas o pestañas?**
El código es abierto: el README explica cómo añadir una pestaña. Las propuestas de reglas son bienvenidas en GitHub.

## 23. Equivalencias con RVTools

| RVTools | gceTools |
|---|---|
| vInfo | gInfo |
| vCPU / vMemory | gCPU / gMemory |
| vDisk | gDisk |
| vNetwork | gNetwork |
| vDatastore | gDisks |
| vSnapshot | gSnapshot |
| vLicense | gLicense |
| vCluster | gMIG / gTemplate |
| vSwitch / dvPort | gVPC / gSubnet (más gFirewall y gAddress) |
| vCD | gImage |
| vHost | gSoleTenant |
| vTools | gTools |
| vSource | gProject |
| vHealth | gHealth |
| — | gSecurity (postura de seguridad por VM) |
| vMetaData | gMetaData |
| — | gReservation, gCommitment, gQuota, gPerf, gRecommend, gCost, gCostReal, gErrors |
