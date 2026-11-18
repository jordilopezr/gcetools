// Mock backend for UI work in a plain browser (no Wails, no GCP).
import type { Api, Column, Inventory, Meta, Progress, Project, Row, Sheet } from './api'

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
}
