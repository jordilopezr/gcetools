# Code signing policy

Free code signing provided by [SignPath.io](https://about.signpath.io), certificate by [SignPath Foundation](https://signpath.org).

Política de firma de código de gceTools. Se aplica a los ejecutables de Windows publicados en las [releases de GitHub](https://github.com/jordilopezr/gcetools/releases).

## Qué se firma

- `gcetools.exe` (aplicación de escritorio) y `gcetools-cli.exe` (versión de consola), dentro del instalador y del `.zip` portable.
- El instalador `gcetools-<versión>-windows-amd64-setup.exe`.

Solo se firman binarios compilados por el workflow público [.github/workflows/build.yml](.github/workflows/build.yml) en GitHub Actions a partir de un tag `v*` de este repositorio. SignPath comprueba el origen de cada artefacto (repositorio, workflow y ejecución) antes de firmarlo. Nunca se firman binarios compilados en un equipo local.

Las dependencias de terceros se compilan desde su código fuente durante el build; no se firman binarios de terceros con el certificado de este proyecto.

## Equipo y funciones

| Función | Personas |
|---|---|
| Autores (commits) | [Jordi Lopez Reyes](https://github.com/jordilopezr) |
| Revisores | [Jordi Lopez Reyes](https://github.com/jordilopezr) |
| Aprobadores de firma | [Jordi Lopez Reyes](https://github.com/jordilopezr) |

- Cada solicitud de firma se aprueba manualmente en SignPath.
- Todos los miembros usan autenticación multifactor en GitHub y en SignPath.
- Las contribuciones externas entran por pull request y se revisan antes de fusionarse.

## Privacidad

This program will not transfer any information to other networked systems unless specifically requested by the user or the person installing or operating it.

En concreto, gceTools solo se conecta a:

- **Las APIs de Google Cloud** que elige el usuario (Compute Engine, Resource Manager, Cloud Billing Catalog y, si se activan, Cloud Monitoring, OS Config, Recommender, Cloud Asset Inventory y BigQuery), con las credenciales y los permisos del propio usuario, para leer su inventario.
- **La API pública de GitHub**, para comprobar si hay una versión nueva. Es una petición anónima que no envía datos del usuario ni del inventario, y se puede desactivar en "Acerca de".

El inventario se guarda únicamente en los archivos que el usuario exporta. gceTools no tiene servidores propios ni envía telemetría.

## Contacto

Para informar de un binario firmado sospechoso o de un problema de seguridad, abre un [issue](https://github.com/jordilopezr/gcetools/issues) o escribe a través del [sitio web](https://gcetools.jordilopezr.com).
