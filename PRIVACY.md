# Política de privacidad de gceTools

*Última actualización: 20 de noviembre de 2026* · [English](PRIVACY.en.md)

gceTools es una aplicación de escritorio de código abierto (licencia GPL-3.0) que genera un inventario de los recursos de Google Compute Engine a los que tiene acceso la cuenta del usuario. Esta política explica qué datos maneja y qué hace con ellos.

## Resumen

- gceTools **no tiene servidores propios** y **no envía datos a su autor ni a terceros**. Se comunica únicamente con las APIs de Google y, para avisar de nuevas versiones, con GitHub.
- Todo el inventario se procesa y se guarda **en el equipo del usuario**.
- El acceso a Google Cloud es **de solo lectura**.

## Datos de Google a los que accede

Al iniciar sesión con Google, gceTools solicita estos permisos:

| Permiso | Para qué |
|---|---|
| `https://www.googleapis.com/auth/cloud-platform.read-only` | Leer, sin modificar, los recursos de Google Cloud a los que ya tiene acceso la cuenta: proyectos, máquinas virtuales, discos, redes, snapshots, métricas, recomendaciones, precios y, si el usuario lo configura, la exportación de facturación |
| `openid`, `email` | Mostrar en la aplicación con qué cuenta se ha iniciado sesión |

gceTools nunca crea, modifica ni borra recursos de Google Cloud.

## Qué guarda y dónde

Todo queda en el equipo del usuario:

- **Token de inicio de sesión:** el token de refresco de Google se guarda en el llavero del sistema operativo (Credential Manager en Windows, Keychain en macOS, Secret Service en Linux). Se borra con "Cerrar sesión".
- **Inventarios:** los archivos que el usuario exporta (XLSX, CSV, JSON, HTML) y, si está activado, el historial de escaneos en la carpeta de datos del usuario.
- **Preferencias y perfiles:** en la carpeta de configuración del usuario. No contienen secretos.

## Comunicaciones de red

- **APIs de Google** (`*.googleapis.com`, `accounts.google.com`, `oauth2.googleapis.com`): para iniciar sesión y leer el inventario.
- **GitHub** (`api.github.com`): una consulta anónima para saber si hay una versión nueva, que se puede desactivar en "Acerca de". No incluye datos del usuario ni del inventario.

No se usan analíticas, telemetría, cookies ni publicidad.

## Uso de los datos de Google

El uso que gceTools hace de la información recibida de las APIs de Google cumple la [Política de datos de usuario de los servicios de API de Google](https://developers.google.com/terms/api-services-user-data-policy), incluidos los requisitos de uso limitado. Los datos se usan solo para mostrar y exportar el inventario al propio usuario. No se transfieren a terceros, no se usan para publicidad y no se emplean para entrenar modelos de inteligencia artificial.

## Revocar el acceso

En cualquier momento, desde gceTools ("Cerrar sesión") o desde [la página de permisos de tu cuenta de Google](https://myaccount.google.com/permissions).

## Contacto

Para dudas sobre privacidad, abre una incidencia en https://github.com/jordilopezr/gcetools/issues o escribe a través de https://gcetools.jordilopezr.com.
