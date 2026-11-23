// Mock backend for UI work in a plain browser (no Wails, no GCP).
import type { Api, Column, Footprint, HealthRule, HistoryEntry, OAuthStatus, Inventory, InventorySummary, Meta, Profile, Progress, Project, Row, Sheet, SnapshotInfo } from './api'

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

// Same rules as the Go view package: tab order, per-VM sheets, figures.
const sheetOrder = ['gInfo', 'gHealth', 'gSecurity', 'gCost', 'gCostReal', 'gRecommend', 'gPerf', 'gTools', 'gDisks', 'gDisk', 'gNetwork', 'gSnapshot', 'gImage', 'gCPU', 'gMemory', 'gLicense', 'gMIG', 'gTemplate', 'gReservation', 'gCommitment', 'gSoleTenant', 'gQuota', 'gVPC', 'gSubnet', 'gFirewall', 'gAddress', 'gProject', 'gErrors', 'gMetaData']
const vmColumn: Record<string, string> = { gInfo: 'name', gCPU: 'name', gMemory: 'name', gDisk: 'vm', gNetwork: 'vm', gPerf: 'vm', gTools: 'vm', gLicense: 'vm', gHealth: 'resource', gSecurity: 'vm' }
const managedKeys = (inv: Inventory) => new Set((inv.sheets.find(s => s.name === 'gInfo')?.rows ?? [])
  .filter(r => ['GKE', 'Dataproc', 'Dataflow', 'Batch'].includes(String(r.managedBy ?? ''))).map(r => `${r.project}/${r.name}`))
function visibleRows(inv: Inventory, s: Sheet, hide: boolean): Row[] {
  const col = vmColumn[s.name], managed = managedKeys(inv)
  return hide && col && managed.size ? s.rows.filter(r => !managed.has(`${r.project}/${r[col]}`)) : s.rows
}
function summarize(inv: Inventory): InventorySummary {
  const rows = (n: string) => inv.sheets.find(s => s.name === n)?.rows ?? []
  const rank = (n: string) => { const i = sheetOrder.indexOf(n); return i < 0 ? 99 : i }
  const vms = rows('gInfo')
  const sum = (rs: Row[], k: string) => rs.reduce((a, r) => a + (Number(r[k]) || 0), 0)
  const real = rows('gCostReal'), recs = rows('gRecommend')
  return {
    meta: inv.meta, managed: managedKeys(inv).size,
    sheets: [...inv.sheets].sort((a, b) => rank(a.name) - rank(b.name)).map(s => ({
      name: s.name, title: s.title, description: s.description, columns: s.columns,
      rows: s.rows.length, rowsUnmanaged: visibleRows(inv, s, true).length,
    })),
    stats: {
      vms: vms.length, running: vms.filter(r => r.status === 'RUNNING').length, vcpu: sum(vms, 'vcpu'), memoryGB: sum(vms, 'memoryGB'),
      diskGB: sum(rows('gDisks'), 'sizeGB'), orphans: rows('gDisks').filter(r => r.orphan === true).length,
      high: rows('gHealth').filter(r => r.severity === 'Alta' || r.severity === 'High').length,
      hasSecurity: inv.sheets.some(s => s.name === 'gSecurity'), exposed: rows('gSecurity').filter(r => r.internetExposed === true).length,
      costList: sum(rows('gCost').filter(r => r.project !== 'TOTAL'), 'total'),
      hasCostReal: real.length > 0, costReal: Number(real.find(r => r.project === 'TOTAL')?.net) || 0,
      costRealMonth: String(real[0]?.month ?? ''), costRealCurrency: String(real[0]?.currency ?? ''),
      hasSavings: recs.length > 0, savings: recs.reduce((a, r) => a + Math.max(0, Number(r.savingsMonthly) || 0), 0),
      savingsCurrency: String(recs.find(r => r.currency)?.currency ?? 'USD'),
    },
  }
}

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
      const gke = p === 'acme-data-lake' && i >= 2
      const name = gke ? `gke-lake-default-pool-8f3a${i}` : `${p.split('-').slice(1).join('-')}-vm-${String(i + 1).padStart(2, '0')}`
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
        ...(gke ? { managedBy: 'GKE', gkeCluster: 'lake', gkeNodePool: 'default-pool', mig: 'gke-lake-default-pool-grp', labels: 'goog-gke-node=; goog-k8s-cluster-name=lake' } : {}),
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
  const security: Row[] = info.map((v, i) => {
    const gke = v.managedBy === 'GKE', win = String(v.os).startsWith('windows')
    const exposed = Boolean(v.externalIP) && i % 2 === 0
    const r: Row = {
      project: v.project, vm: v.name, zone: v.zone, status: v.status, managedBy: v.managedBy ?? '', externalIP: v.externalIP,
      internetExposed: exposed, exposedPorts: exposed ? (win ? '3389 (RDP)' : '22 (SSH)') : '', exposingRules: exposed ? (win ? 'allow-rdp-temp' : 'allow-ssh') : '',
      serviceAccount: v.serviceAccount, defaultSA: String(v.serviceAccount).includes('-compute@'), fullScope: String(v.serviceAccount).includes('-compute@'),
      osLogin: !gke && i % 3 !== 0, projectSSHKeys: !gke && i % 3 === 0, instanceSSHKeys: false, serialPort: i % 13 === 4, ipForward: gke,
      secureBoot: v.secureBoot, vtpm: true, integrityMonitoring: i % 5 !== 2, confidential: false, diskEncryption: i % 7 === 0 ? 'CMEK' : 'Google', notes: '',
    }
    r.issues = [r.internetExposed, r.defaultSA, !gke && !r.osLogin, r.projectSSHKeys, r.serialPort, !gke && r.ipForward, !r.secureBoot, !r.integrityMonitoring].filter(Boolean).length
    if (exposed) health.push({ severity: 'Alta', category: 'Seguridad', rule: 'VM expuesta a Internet en puertos sensibles', ruleId: 'internet-exposed', project: v.project, resource: v.name, location: v.zone, detail: `${r.exposedPorts} abiertos en ${v.externalIP} por ${r.exposingRules}` })
    if (r.projectSSHKeys) health.push({ severity: 'Media', category: 'Seguridad', rule: 'VM que acepta claves SSH de proyecto', ruleId: 'project-ssh-keys', project: v.project, resource: v.name, location: v.zone, detail: '' })
    if (r.serialPort) health.push({ severity: 'Media', category: 'Seguridad', rule: 'Acceso por puerto serie habilitado', ruleId: 'serial-port', project: v.project, resource: v.name, location: v.zone, detail: 'serial-port-enable=TRUE en la VM o en el proyecto' })
    return r
  })
  for (const p of proj) {
    p.exposedVMs = security.filter(r => r.project === p.projectId && r.internetExposed).length
    p.osLogin = p.projectId !== 'acme-legacy'
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
      col('labels', 'Labels'), col('deletionProtection', 'Deletion protection', 'bool'), col('secureBoot', 'Secure Boot', 'bool'),
      col('mig', 'MIG'), col('managedBy', 'Gestionada por'), col('gkeCluster', 'Clúster (GKE/Dataproc)'), col('gkeNodePool', 'Node pool GKE')] },
    { name: 'gDisks', title: 'Todos los discos', description: 'Inventario de discos, incluidos los huérfanos', rows: disks, columns: [
      col('project', 'Proyecto'), col('name', 'Disco'), col('location', 'Ubicación'), col('type', 'Tipo'), col('sizeGB', 'Tamaño (GB)', 'gb'),
      col('status', 'Estado'), col('users', 'VMs'), col('orphan', 'Huérfano', 'bool'), col('created', 'Creado', 'datetime'), col('lastDetach', 'Último detach', 'datetime')] },
    { name: 'gProject', title: 'Proyectos', description: 'Proyectos escaneados', rows: proj, columns: [
      col('projectId', 'Proyecto'), col('name', 'Nombre'), col('scanStatus', 'Resultado escaneo'), col('vms', 'VMs', 'int'), col('vcpu', 'vCPU', 'int'), col('memoryGB', 'RAM (GB)', 'gb'),
      col('osLogin', 'OS Login (proyecto)', 'bool'), col('exposedVMs', 'VMs expuestas a Internet', 'int')] },
    { name: 'gHealth', title: 'Salud', description: 'Hallazgos de buenas prácticas', rows: health, columns: [
      col('severity', 'Severidad'), col('category', 'Categoría'), col('rule', 'Regla'), col('project', 'Proyecto'), col('resource', 'Recurso'), col('location', 'Ubicación'), col('detail', 'Detalle')] },
    { name: 'gSecurity', title: 'Seguridad', description: 'Postura de seguridad por VM: exposición a Internet, acceso e identidad', rows: security, columns: [
      col('project', 'Proyecto'), col('vm', 'VM'), col('zone', 'Zona'), col('status', 'Estado'), col('managedBy', 'Gestionada por'), col('issues', 'Problemas', 'int'),
      col('externalIP', 'IP externa'), col('internetExposed', 'Expuesta a Internet', 'bool'), col('exposedPorts', 'Puertos sensibles abiertos'),
      col('exposingRules', 'Reglas que los abren'), col('serviceAccount', 'Service account'), col('defaultSA', 'SA por defecto', 'bool'),
      col('fullScope', 'Scope cloud-platform', 'bool'), col('osLogin', 'OS Login', 'bool'), col('projectSSHKeys', 'Acepta claves SSH de proyecto', 'bool'),
      col('instanceSSHKeys', 'Claves SSH en la VM', 'bool'), col('serialPort', 'Puerto serie', 'bool'), col('ipForward', 'IP forwarding', 'bool'),
      col('secureBoot', 'Secure Boot', 'bool'), col('vtpm', 'vTPM', 'bool'), col('integrityMonitoring', 'Integrity monitoring', 'bool'),
      col('confidential', 'Confidential', 'bool'), col('diskEncryption', 'Cifrado de discos'), col('notes', 'Notas')] },
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
// Mirrors health.Rules (internal/health/health.go).
const healthRules: HealthRule[] = [{"id": "orphan-disk", "title": "Disco huérfano", "category": "Coste", "severity": "Alta", "labels": "disco o proyecto"}, {"id": "stopped-vm", "title": "VM parada más de N días", "category": "Coste", "severity": "Media", "param": "stoppedDays", "labels": "VM o proyecto"}, {"id": "unused-ip", "title": "IP externa reservada sin uso", "category": "Coste", "severity": "Media", "labels": "proyecto"}, {"id": "unused-reservation", "title": "Reserva con capacidad sin usar", "category": "Coste", "severity": "Media", "labels": "proyecto"}, {"id": "cud-expiring", "title": "Commitment (CUD) que vence pronto sin renovación", "category": "Coste", "severity": "Media", "param": "cudDays", "labels": "proyecto"}, {"id": "cpu-low", "title": "VM infrautilizada (CPU p95 baja)", "category": "Coste", "severity": "Media", "param": "cpuLowPct", "labels": "VM o proyecto"}, {"id": "old-family", "title": "Familia de máquina antigua (N1)", "category": "Coste", "severity": "Baja", "labels": "VM o proyecto"}, {"id": "old-snapshots", "title": "Snapshots manuales antiguos", "category": "Coste", "severity": "Baja", "param": "snapshotDays", "labels": "proyecto"}, {"id": "firewall-open", "title": "Regla de firewall abierta a Internet", "category": "Seguridad", "severity": "Alta", "labels": "proyecto"}, {"id": "default-sa", "title": "Service account por defecto de Compute", "category": "Seguridad", "severity": "Media", "labels": "VM o proyecto"}, {"id": "public-ip", "title": "VM con IP pública", "category": "Seguridad", "severity": "Media", "labels": "VM o proyecto"}, {"id": "no-secure-boot", "title": "Secure Boot deshabilitado", "category": "Seguridad", "severity": "Baja", "labels": "VM o proyecto"}, {"id": "internet-exposed", "title": "VM expuesta a Internet en puertos sensibles", "category": "Seguridad", "severity": "Alta", "labels": "VM o proyecto"}, {"id": "project-ssh-keys", "title": "VM que acepta claves SSH de proyecto", "category": "Seguridad", "severity": "Media", "labels": "VM o proyecto"}, {"id": "serial-port", "title": "Acceso por puerto serie habilitado", "category": "Seguridad", "severity": "Media", "labels": "VM o proyecto"}, {"id": "no-os-login", "title": "OS Login deshabilitado", "category": "Seguridad", "severity": "Baja", "labels": "VM o proyecto"}, {"id": "ip-forward", "title": "IP forwarding habilitado", "category": "Seguridad", "severity": "Baja", "labels": "VM o proyecto"}, {"id": "no-shielded-vm", "title": "vTPM o integrity monitoring deshabilitado", "category": "Seguridad", "severity": "Baja", "labels": "VM o proyecto"}, {"id": "template-image", "title": "Template con imagen obsoleta, deprecada o antigua", "category": "Seguridad", "severity": "Media", "param": "imageDays", "labels": "template o proyecto"}, {"id": "snapshot-schedule", "title": "VM con discos sin snapshot schedule", "category": "Protección de datos", "severity": "Baja", "labels": "VM o proyecto"}, {"id": "cpu-high", "title": "CPU saturada", "category": "Rendimiento", "severity": "Media", "param": "cpuHighPct", "labels": "VM o proyecto"}, {"id": "mem-high", "title": "Memoria saturada", "category": "Rendimiento", "severity": "Media", "param": "memHighPct", "labels": "VM o proyecto"}, {"id": "boot-pd-standard", "title": "Disco de arranque pd-standard", "category": "Rendimiento", "severity": "Baja", "labels": "VM o proyecto"}, {"id": "quota", "title": "Cuota regional casi agotada", "category": "Capacidad", "severity": "Media", "param": "quotaPct", "labels": "proyecto"}, {"id": "no-labels", "title": "VM sin labels", "category": "Gobierno", "severity": "Baja", "labels": "VM o proyecto"}, {"id": "stopped-no-disks", "title": "VM parada sin discos", "category": "Gobierno", "severity": "Baja", "labels": "VM o proyecto"}, {"id": "no-deletion-protection", "title": "Deletion protection deshabilitada", "category": "Operación", "severity": "Baja", "labels": "VM o proyecto"}, {"id": "no-ops-agent", "title": "Sin Ops Agent", "category": "Operación", "severity": "Baja", "labels": "VM o proyecto"}]

// Twelve weekly scans with a plausible drift, newest first.
let historyEntries: HistoryEntry[] = Array.from({ length: 12 }, (_, i) => {
  const w = 11 - i // 0 = oldest
  const d = new Date(Date.UTC(2026, 8, 1 + w * 7, 8, 0, 0))
  const vms = 52 + Math.round(w * 0.8 + (w % 3 === 0 ? 1 : 0))
  return {
    id: `${d.toISOString().slice(0, 10).replace(/-/g, '')}-080000`, startedAt: d.toISOString(), identity: 'demo@example.com',
    version: w < 6 ? '0.5.0' : '0.6.0', profile: 'Producción', projects: 6, projectsOk: w === 4 ? 5 : 6, cancelled: false,
    vms, running: vms - 6 + (w % 2), vcpu: vms * 4 + w * 2, memoryGB: vms * 15, disks: vms + 9, diskGB: vms * 180, orphans: Math.max(1, 6 - Math.floor(w / 2)),
    currency: 'USD', costList: Math.round(17200 + w * 210 + (w === 7 ? 900 : 0)), costReal: 0, savings: Math.round(2100 - w * 60),
    high: Math.max(2, 14 - w), medium: 30 - Math.floor(w / 2), low: 70 + w, sizeBytes: 1_400_000 + w * 20_000,
  }
})

const builtinOAuth: OAuthStatus = { available: true, builtin: true, custom: false, client: '1234-gcetools.apps.googleusercontent.com', projectId: 'gcetools-app', account: '' }
let oauth: OAuthStatus = { ...builtinOAuth }
let loginTimer: ReturnType<typeof setTimeout> | undefined
let oauthCancelReject: (() => void) | undefined

let picks = 0
let profiles: Profile[] = [{
  name: 'Producción', query: '', projects: ['acme-prod-erp', 'acme-prod-web'], concurrency: 8,
  modules: { osconfig: true, monitoring: true, recommender: true, metricsDays: 30, pricing: true, currency: 'EUR' },
  health: { stoppedDays: 30, snapshotDays: 90, imageDays: 180, cpuLowPct: 10, cpuHighPct: 90, memHighPct: 90, quotaPct: 80, cudDays: 60, disabled: ['snapshot-schedule'] }, updated: '2026-11-10T09:00:00Z',
}]
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
  setLanguage: async () => { /* mock: nothing to tell */ },
  detectADC: async () => ({ found: true, identity: 'demo@example.com' }),
  pickKey: async () => ({ clientEmail: 'inventory@acme-tools.iam.gserviceaccount.com', projectId: 'acme-tools', keyId: 'f3a1…' }),
  connect: async (method, sa) => {
    await sleep(500)
    if (method === 'oauth' && !oauth.account) throw new Error('no has iniciado sesión con Google en gceTools')
    const identity = method === 'sa' ? 'inventory@acme-tools.iam.gserviceaccount.com' : method === 'impersonate' ? sa : 'demo@example.com'
    return { identity, method, projectHint: 'acme-tools', fullScope: method === 'adc' }
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
  inventorySummary: async () => inventory && summarize(inventory),
  sheetRows: async (name, hide) => {
    const s = inventory?.sheets.find(x => x.name === name)
    if (!inventory || !s) throw new Error(`la pestaña ${name} no existe en este inventario`)
    await sleep(80)
    return visibleRows(inventory, s, hide)
  },
  exportInventory: async (f) => `/home/demo/gcetools-demo.${f === 'csv' ? 'zip' : f}`,
  onProgress: (cb) => on(listeners.progress, cb),
  onDone: (cb) => on(listeners.done, cb),
  onError: (cb) => on(listeners.error, cb),
  onWarning: () => () => {}, // the demo never fails to save history
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
  listProfiles: async () => [...profiles].sort((a, b) => a.name.localeCompare(b.name)),
  saveProfile: async (p) => {
    if (!p.name.trim()) throw new Error('el perfil necesita un nombre')
    profiles = [...profiles.filter(x => x.name.toLowerCase() !== p.name.toLowerCase()), { ...p, updated: new Date().toISOString() }]
  },
  deleteProfile: async (n) => { profiles = profiles.filter(x => x.name.toLowerCase() !== n.toLowerCase()) },
  discover: async (scope) => {
    await sleep(700)
    if (!/^(organizations|folders)\/\d+$|^projects\//.test(scope)) throw new Error(`ámbito no válido "${scope}": usa organizations/123, folders/456 o projects/mi-proyecto`)
    const fp: Footprint[] = [
      { projectId: 'acme-prod-erp', number: '482917300000', vms: 9, running: 8, disks: 14, other: 6 },
      { projectId: 'acme-prod-web', number: '482917307919', vms: 12, running: 12, disks: 12, other: 2 },
      { projectId: 'acme-data-lake', number: '482917331676', vms: 4, running: 3, disks: 7, other: 0 },
      { projectId: 'acme-legacy-batch', number: '482917399999', vms: 2, running: 0, disks: 3, other: 1 },
    ]
    return fp
  },
  healthRules: async () => healthRules,
  listHistory: async () => historyEntries,
  openHistory: async (id) => {
    await sleep(300)
    const e = historyEntries.find(x => x.id === id)
    if (!e) throw new Error('escaneo no encontrado')
    const inv = buildInventory(projects.map(p => p.id))
    inv.meta = { ...inv.meta, startedAt: e.startedAt, finishedAt: e.startedAt }
    inventory = inv
    return summarize(inv)
  },
  deleteHistory: async (id) => { historyEntries = historyEntries.filter(x => x.id !== id) },
  historySnapshot: async (id) => {
    const e = historyEntries.find(x => x.id === id)
    if (!e) throw new Error('escaneo no encontrado')
    const s: SnapshotInfo = { path: `history:${id}`, name: `Historial ${id}`, startedAt: e.startedAt, identity: e.identity, version: e.version, projects: e.projects, vms: e.vms }
    known.set(s.path, s)
    return s
  },
  historyDir: async () => '/home/demo/.local/share/gcetools/history',
  exportReport: async () => '/home/demo/gcetools-informe-demo.html',
  oauthStatus: async () => oauth,
  oauthPickClient: async () => (oauth = { ...oauth, custom: true, client: '482917300000-demo.apps.googleusercontent.com', projectId: 'acme-tools', account: '' }),
  oauthLogin: () => new Promise((resolve, reject) => {
    loginTimer = setTimeout(() => resolve(oauth = { ...oauth, account: 'demo@example.com' }), 1500)
    oauthCancelReject = () => reject(new Error('inicio de sesión cancelado'))
  }),
  oauthCancel: async () => { clearTimeout(loginTimer); oauthCancelReject?.() },
  oauthLogout: async (all) => (oauth = all ? { ...builtinOAuth } : { ...oauth, account: '' }),
  oauthUseBuiltin: async () => (oauth = { ...builtinOAuth }),
}
