// Mock backend for UI work in a plain browser (no Wails, no GCP).
import type { Api, Column, Inventory, Meta, Progress, Project, Row, Sheet, SnapshotInfo } from './api'

const projects: Project[] = [
  ['acme-prod-erp', 'ERP producción', 'folders/1201'], ['acme-prod-web', 'Web producción', 'folders/1201'],
  ['acme-dev-sandbox', 'Sandbox desarrollo', 'folders/1302'], ['acme-shared-vpc', 'Shared VPC host', 'folders/1100'],
  ['acme-data-lake', 'Data lake', 'folders/1201'], ['acme-qa', 'QA', 'folders/1302'],
].map(([id, name, parent], i) => ({
  id, name, parent, number: String(482917300000 + i * 7919), state: 'ACTIVE', created: '2023-03-14T10:00:00Z',
  labels: { env: id.includes('prod') ? 'prod' : 'nonprod' },
}))

const col = (key: string, title: string, type: Column['type'] = 'string'): Column => ({ key, title, type })
const pick = <T,>(a: T[], i: number) => a[i % a.length]

function buildInventory(ids: string[]): Inventory {
  const zones = ['europe-west1-b', 'europe-southwest1-a', 'us-central1-a', 'europe-west4-c']
  const types: [string, number, number][] = [['e2-standard-4', 4, 16], ['n2-standard-8', 8, 32], ['n1-standard-2', 2, 7.5], ['c3-highmem-22', 22, 176], ['n2d-standard-4', 4, 16]]
  const info: Row[] = [], disks: Row[] = [], health: Row[] = [], proj: Row[] = []
  let n = 0
  for (const p of ids) {
    const count = 6 + (p.length % 9)
    let vcpu = 0, mem = 0
    for (let i = 0; i < count; i++, n++) {
      const [mt, cpu, ram] = pick(types, n), zone = pick(zones, n + i)
      const status = n % 7 === 3 ? 'TERMINATED' : 'RUNNING'
      const name = `${p.split('-').slice(1).join('-')}-vm-${String(i + 1).padStart(2, '0')}`
      vcpu += cpu; mem += ram
      info.push({
        project: p, name, id: String(5810000000000 + n * 3571), zone, region: zone.replace(/-[a-z]$/, ''), status,
        machineType: mt, family: mt.split('-')[0], vcpu: cpu, memoryGB: ram, cpuPlatform: 'Intel Cascade Lake',
        provisioning: n % 11 === 0 ? 'SPOT' : 'STANDARD', created: '2024-0' + ((n % 9) + 1) + '-12T08:30:00Z',
        lastStart: '2026-11-0' + ((n % 9) + 1) + 'T06:00:00Z', lastStop: status === 'TERMINATED' ? '2026-08-02T18:00:00Z' : null,
        internalIP: `10.${10 + (n % 4)}.${i}.${10 + n}`, externalIP: n % 5 === 0 ? `34.76.${n}.${20 + i}` : '',
        network: 'shared-vpc', subnet: 'sn-' + zone.replace(/-[a-z]$/, ''),
        os: n % 4 === 0 ? 'windows-server-2022-dc' : 'debian-12-bookworm', bootDiskGB: 50, totalDiskGB: 50 + (n % 3) * 200,
        serviceAccount: n % 6 === 0 ? `${482917300000}-compute@developer.gserviceaccount.com` : `app@${p}.iam.gserviceaccount.com`,
        labels: n % 8 === 0 ? '' : `app=erp; env=${p.includes('prod') ? 'prod' : 'dev'}`, deletionProtection: n % 3 === 0,
        secureBoot: n % 4 !== 1,
      })
      disks.push({ project: p, name: `${name}-boot`, location: zone, type: n % 9 === 0 ? 'pd-standard' : 'pd-balanced', sizeGB: 50, status: 'READY', users: name, orphan: false, created: '2024-05-01T09:00:00Z' })
    }
    disks.push({ project: p, name: `${p}-old-data`, location: zones[0], type: 'pd-ssd', sizeGB: 500, status: 'READY', users: '', orphan: true, created: '2023-01-10T09:00:00Z', lastDetach: '2025-02-18T12:00:00Z' })
    proj.push({ projectId: p, name: projects.find(x => x.id === p)?.name ?? p, scanStatus: p === 'acme-qa' ? 'Compute API deshabilitada' : 'OK', vms: count, vcpu, memoryGB: mem })
  }
  for (const d of disks.filter(d => d.orphan)) health.push({ severity: 'Alta', category: 'Coste', rule: 'Disco huérfano', project: d.project, resource: d.name, location: d.location, detail: `${d.sizeGB} GB pd-ssd sin VM` })
  for (const v of info) {
    if (String(v.serviceAccount).includes('-compute@')) health.push({ severity: 'Media', category: 'Seguridad', rule: 'Usa la service account por defecto de Compute', project: v.project, resource: v.name, location: v.zone, detail: String(v.serviceAccount) })
    if (v.externalIP) health.push({ severity: 'Media', category: 'Seguridad', rule: 'VM con IP pública', project: v.project, resource: v.name, location: v.zone, detail: String(v.externalIP) })
    if (v.status === 'TERMINATED') health.push({ severity: 'Media', category: 'Coste', rule: 'VM parada más de 30 días', project: v.project, resource: v.name, location: v.zone, detail: 'Parada desde 2026-08-02' })
    if (!v.labels) health.push({ severity: 'Baja', category: 'Gobierno', rule: 'VM sin labels', project: v.project, resource: v.name, location: v.zone, detail: '' })
  }
  const order = { Alta: 0, Media: 1, Baja: 2 } as Record<string, number>
  health.sort((a, b) => order[a.severity as string] - order[b.severity as string])

  const now = new Date().toISOString()
  const meta: Meta = { tool: 'gceTools', version: '0.1.0-mock', identity: 'demo@example.com', authMethod: 'adc', startedAt: now, finishedAt: now, projects: ids.length, projectsOk: ids.length - 1, errors: 1, cancelled: false }
  const sheets: Sheet[] = [
    { name: 'gInfo', title: 'Instancias', description: 'Una fila por VM (equivale a vInfo)', rows: info, columns: [
      col('project', 'Proyecto'), col('name', 'VM'), col('id', 'ID'), col('zone', 'Zona'), col('region', 'Región'), col('status', 'Estado'),
      col('machineType', 'Machine type'), col('family', 'Familia'), col('vcpu', 'vCPU', 'int'), col('memoryGB', 'RAM (GB)', 'gb'),
      col('cpuPlatform', 'CPU platform'), col('provisioning', 'Aprovisionamiento'), col('created', 'Creada', 'datetime'),
      col('lastStart', 'Último arranque', 'datetime'), col('lastStop', 'Última parada', 'datetime'), col('internalIP', 'IP interna'),
      col('externalIP', 'IP externa'), col('network', 'VPC'), col('subnet', 'Subred'), col('os', 'SO (licencia)'),
      col('bootDiskGB', 'Disco boot (GB)', 'gb'), col('totalDiskGB', 'Discos total (GB)', 'gb'), col('serviceAccount', 'Service account'),
      col('labels', 'Labels'), col('deletionProtection', 'Deletion protection', 'bool'), col('secureBoot', 'Secure Boot', 'bool')] },
    { name: 'gDisks', title: 'Todos los discos', description: 'Inventario de discos, incluidos los huérfanos', rows: disks, columns: [
      col('project', 'Proyecto'), col('name', 'Disco'), col('location', 'Ubicación'), col('type', 'Tipo'), col('sizeGB', 'Tamaño (GB)', 'gb'),
      col('status', 'Estado'), col('users', 'VMs'), col('orphan', 'Huérfano', 'bool'), col('created', 'Creado', 'datetime'), col('lastDetach', 'Último detach', 'datetime')] },
    { name: 'gProject', title: 'Proyectos', description: 'Proyectos escaneados', rows: proj, columns: [
      col('projectId', 'Proyecto'), col('name', 'Nombre'), col('scanStatus', 'Resultado escaneo'), col('vms', 'VMs', 'int'), col('vcpu', 'vCPU', 'int'), col('memoryGB', 'RAM (GB)', 'gb')] },
    { name: 'gHealth', title: 'Salud', description: 'Hallazgos de buenas prácticas', rows: health, columns: [
      col('severity', 'Severidad'), col('category', 'Categoría'), col('rule', 'Regla'), col('project', 'Proyecto'), col('resource', 'Recurso'), col('location', 'Ubicación'), col('detail', 'Detalle')] },
    { name: 'gErrors', title: 'Errores', description: 'Proyectos o APIs que no se pudieron leer', columns: [col('project', 'Proyecto'), col('collector', 'Colector'), col('kind', 'Tipo'), col('message', 'Mensaje')],
      rows: [{ project: 'acme-qa', collector: 'disks', kind: 'api_disabled', message: 'Compute Engine API has not been used in project acme-qa before or it is disabled.' }] },
    { name: 'gMetaData', title: 'Metadatos', description: 'Información del escaneo', columns: [col('property', 'Propiedad'), col('value', 'Valor')],
      rows: [{ property: 'Herramienta', value: 'gceTools 0.1.0 (mock)' }, { property: 'Identidad', value: meta.identity }] },
  ]
  return { meta, sheets, errors: [] }
}

// A plausible comparison result, shaped like internal/diff output.
function buildDiff(a: SnapshotInfo, b: SnapshotInfo): Inventory {
  const common = [col('relevant', 'Relevante', 'bool'), col('reason', 'Motivo'), col('sheet', 'Pestaña'), col('kind', 'Tipo de recurso'),
    col('project', 'Proyecto'), col('resource', 'Recurso'), col('location', 'Ubicación')]
  const r = (relevant: boolean, reason: string, sheet: string, kind: string, project: string, resource: string, location: string, extra: Row): Row =>
    ({ relevant, reason, sheet, kind, project, resource, location, ...extra })
  const added = [
    r(true, 'Regla nueva abierta a Internet', 'gFirewall', 'Firewall', 'acme-prod-web', 'allow-rdp-temp', 'shared-vpc', { detail: 'Dirección: INGRESS; Acción: allow; Protocolos/puertos: tcp:3389; Orígenes: 0.0.0.0/0' }),
    r(true, 'VM nueva con IP pública', 'gInfo', 'Instancias', 'acme-prod-web', 'web-vm-09', 'europe-west1-b', { detail: 'Machine type: e2-standard-4; Estado: RUNNING; IP externa: 34.76.12.40' }),
    r(false, '', 'gInfo', 'Instancias', 'acme-dev-sandbox', 'sandbox-vm-12', 'us-central1-a', { detail: 'Machine type: e2-medium; Estado: RUNNING' }),
    r(false, '', 'gDisks', 'Todos los discos', 'acme-prod-erp', 'erp-vm-03-data2', 'europe-southwest1-a', { detail: 'Tipo: pd-balanced; Tamaño (GB): 500; VMs: erp-vm-03' }),
  ]
  const removed = [
    r(false, '', 'gInfo', 'Instancias', 'acme-dev-sandbox', 'sandbox-vm-04', 'us-central1-a', { detail: 'Machine type: n1-standard-2; Estado: TERMINATED' }),
    r(false, '', 'gSnapshot', 'Snapshots', 'acme-prod-erp', 'erp-before-upgrade', '', { detail: 'Disco origen: erp-vm-01-boot; Almacenado (GB): 41.2' }),
  ]
  const changed = [
    r(true, 'Cambio de tamaño de la VM', 'gInfo', 'Instancias', 'acme-prod-erp', 'erp-vm-02', 'europe-southwest1-a', { field: 'Machine type', before: 'n2-standard-8', after: 'n2-standard-16' }),
    r(true, 'Cambio de identidad (service account)', 'gInfo', 'Instancias', 'acme-prod-web', 'web-vm-03', 'europe-west1-b', { field: 'Service account', before: 'web@acme-prod-web.iam.gserviceaccount.com', after: '482917300000-compute@developer.gserviceaccount.com' }),
    r(true, 'Disco que se quedó huérfano', 'gDisks', 'Todos los discos', 'acme-data-lake', 'lake-vm-02-data', 'europe-west4-c', { field: 'Huérfano', before: 'No', after: 'Sí' }),
    r(false, '', 'gInfo', 'Instancias', 'acme-prod-erp', 'erp-vm-02', 'europe-southwest1-a', { field: 'vCPU', before: '8', after: '16' }),
    r(false, '', 'gInfo', 'Instancias', 'acme-dev-sandbox', 'sandbox-vm-07', 'europe-west1-b', { field: 'Estado', before: 'RUNNING', after: 'TERMINATED' }),
    r(false, '', 'gMIG', 'MIGs', 'acme-prod-web', 'web-mig', 'europe-west1', { field: 'Target size', before: '3', after: '5' }),
  ]
  const summary: Row[] = [
    { sheet: 'gInfo', kind: 'Instancias', added: 2, removed: 1, modified: 3, fields: 4, relevant: 3 },
    { sheet: 'gDisks', kind: 'Todos los discos', added: 1, removed: 0, modified: 1, fields: 1, relevant: 1 },
    { sheet: 'gSnapshot', kind: 'Snapshots', added: 0, removed: 1, modified: 0, fields: 0, relevant: 0 },
    { sheet: 'gMIG', kind: 'MIGs', added: 0, removed: 0, modified: 1, fields: 1, relevant: 0 },
    { sheet: 'gFirewall', kind: 'Firewall', added: 1, removed: 0, modified: 0, fields: 0, relevant: 1 },
  ]
  const meta = [
    ['Anterior', a.name], ['Anterior: fecha', a.startedAt.slice(0, 16).replace('T', ' ')], ['Nuevo', b.name], ['Nuevo: fecha', b.startedAt.slice(0, 16).replace('T', ' ')],
    ['No comparado: gInfo', 'acme-qa (el proyecto no se leyó completo en alguno de los dos escaneos)'],
    ['Coste/mes estimado anterior', '18450.20 USD'], ['Coste/mes estimado nuevo', '19112.75 USD'], ['Diferencia de coste/mes', '+662.55 USD'],
  ].map(([property, value]) => ({ property, value }))
  const now = new Date().toISOString()
  return {
    meta: { tool: 'gceTools', version: '0.3.0', identity: 'demo@example.com', authMethod: '', startedAt: a.startedAt, finishedAt: now, projects: 6, projectsOk: 5, errors: 0, cancelled: false },
    errors: [],
    sheets: [
      { name: 'Resumen', title: 'Resumen', description: 'Altas, bajas y cambios por tipo de recurso', rows: summary, columns: [
        col('sheet', 'Pestaña'), col('kind', 'Tipo de recurso'), col('added', 'Altas', 'int'), col('removed', 'Bajas', 'int'),
        col('modified', 'Recursos modificados', 'int'), col('fields', 'Campos cambiados', 'int'), col('relevant', 'Relevantes', 'int')] },
      { name: 'Altas', title: 'Altas', description: 'Recursos que existen en el inventario nuevo y no en el anterior', rows: added, columns: [...common, col('detail', 'Detalle')] },
      { name: 'Bajas', title: 'Bajas', description: 'Recursos del inventario anterior que ya no existen', rows: removed, columns: [...common, col('detail', 'Detalle')] },
      { name: 'Cambios', title: 'Cambios', description: 'Un campo por fila: valor anterior y valor nuevo', rows: changed, columns: [...common, col('field', 'Campo'), col('before', 'Antes'), col('after', 'Después')] },
      { name: 'Metadatos', title: 'Metadatos', description: 'Inventarios comparados y alcance de la comparación', rows: meta, columns: [col('property', 'Propiedad'), col('value', 'Valor')] },
    ],
  }
}

const snapshots: SnapshotInfo[] = [
  { path: '/home/demo/gcetools-20261018-0900.json', name: 'gcetools-20261018-0900.json', startedAt: '2026-10-18T09:00:00Z', identity: 'demo@example.com', version: '0.2.2', projects: 6, vms: 58 },
  { path: '/home/demo/gcetools-20261118-0900.json', name: 'gcetools-20261118-0900.json', startedAt: '2026-11-18T09:00:00Z', identity: 'demo@example.com', version: '0.3.0', projects: 6, vms: 59 },
]
let picks = 0
const known = new Map<string, SnapshotInfo>()

type Cb<T> = (v: T) => void
const listeners = { progress: new Set<Cb<Progress>>(), done: new Set<Cb<Meta>>(), error: new Set<Cb<string>>() }
let inventory: Inventory | null = null
let timer: ReturnType<typeof setInterval> | undefined
const sleep = (ms: number) => new Promise(r => setTimeout(r, ms))
const on = <T,>(set: Set<Cb<T>>, cb: Cb<T>) => { set.add(cb); return () => { set.delete(cb) } }

export const mockApi: Api = {
  mock: true,
  version: async () => '0.1.0-mock',
  detectADC: async () => ({ found: true, identity: 'demo@example.com' }),
  pickKey: async () => ({ clientEmail: 'inventory@acme-tools.iam.gserviceaccount.com', projectId: 'acme-tools', keyId: 'f3a1…' }),
  connect: async (method, sa) => {
    await sleep(500)
    const identity = method === 'sa' ? 'inventory@acme-tools.iam.gserviceaccount.com' : method === 'impersonate' ? sa : 'demo@example.com'
    return { identity, method, projectHint: 'acme-tools' }
  },
  disconnect: async () => { inventory = null },
  listProjects: async (q) => { await sleep(400); return projects.filter(p => !q || JSON.stringify(p).includes(q.replace(/^.*:/, ''))) },
  startScan: async (ids) => {
    let step = 0
    const steps = ['disks', 'instances', 'snapshots', 'migs', 'networks', 'firewalls']
    const total = ids.length * steps.length
    timer = setInterval(() => {
      step++
      const done = Math.floor(step / steps.length)
      const p: Progress = { total: ids.length, done, project: ids[Math.min(done, ids.length - 1)], step: steps[step % steps.length], vms: done * 9, errors: done > 3 ? 1 : 0, message: '' }
      if (step % steps.length === 0) { p.step = 'done'; p.message = p.project === 'acme-qa' ? 'Compute API deshabilitada' : 'OK' }
      listeners.progress.forEach(cb => cb(p))
      if (step >= total) {
        clearInterval(timer)
        inventory = buildInventory(ids)
        listeners.done.forEach(cb => cb(inventory!.meta))
      }
    }, 120)
  },
  cancelScan: async () => { clearInterval(timer) },
  getInventory: async () => inventory,
  exportInventory: async (f) => `/home/demo/gcetools-demo.${f === 'csv' ? 'zip' : f}`,
  onProgress: (cb) => on(listeners.progress, cb),
  onDone: (cb) => on(listeners.done, cb),
  onError: (cb) => on(listeners.error, cb),
  appInfo: async () => ({
    version: '0.2.1', website: 'https://gcetools.jordilopezr.com', donate: 'https://buymeacoffee.com/jordimlopezr',
    license: 'GPL-3.0', licenseUrl: 'https://www.gnu.org/licenses/gpl-3.0.html', releases: 'https://github.com/jordilopezr/gcetools/releases',
  }),
  checkUpdate: async () => {
    await sleep(600)
    return { current: '0.2.1', latest: '0.3.0', available: true, url: 'https://github.com/jordilopezr/gcetools/releases', notes: '- Ejemplo de notas de la versión', published: '2026-12-01T10:00:00Z' }
  },
  openURL: (url) => { window.open(url, '_blank', 'noopener') },
  pickSnapshot: async () => { const s = snapshots[picks++ % snapshots.length]; known.set(s.path, s); return s },
  currentSnapshot: async () => {
    if (!inventory) return null
    const s: SnapshotInfo = { path: 'current', name: 'Inventario actual', startedAt: inventory.meta.startedAt, identity: inventory.meta.identity,
      version: inventory.meta.version, projects: inventory.meta.projects, vms: inventory.sheets.find(x => x.name === 'gInfo')?.rows.length ?? 0 }
    known.set(s.path, s)
    return s
  },
  compare: async (a, b) => {
    await sleep(500)
    const sa = known.get(a), sb = known.get(b)
    if (!sa || !sb) throw new Error('snapshot no cargado: vuelve a elegirlo')
    return buildDiff(sa, sb)
  },
  exportDiff: async (f) => `/home/demo/gcetools-diff.${f === 'csv' ? 'zip' : f}`,
}
