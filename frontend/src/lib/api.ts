// Thin typed facade over the Wails bindings. When the page runs outside
// Wails (plain `npm run dev` in a browser) it falls back to a mock backend,
// so the UI can be designed and reviewed without GCP credentials.
import * as App from '../../wailsjs/go/main/App'
import { BrowserOpenURL, EventsOn } from '../../wailsjs/runtime/runtime'
import { mockApi } from './mock'

export type ColType = 'string' | 'int' | 'float' | 'bool' | 'datetime' | 'gb' | 'money'
export interface Column { key: string; title: string; type: ColType; width?: number }
export type Row = Record<string, unknown>
export interface Sheet { name: string; title: string; description: string; columns: Column[]; rows: Row[] }
export interface ScanError { project: string; collector: string; kind: string; message: string }
export interface Meta {
  tool: string; version: string; identity: string; authMethod: string
  startedAt: string; finishedAt: string; projects: number; projectsOk: number; errors: number; cancelled: boolean; currency?: string
}
export interface Inventory { meta: Meta; sheets: Sheet[]; errors: ScanError[] }
// The inventory page gets a summary first and then one sheet at a time: a
// large inventory sent whole is hundreds of MB through the Wails bridge.
export interface SheetInfo { name: string; title: string; description: string; columns: Column[]; rows: number; rowsUnmanaged: number }
export interface InventoryStats {
  vms: number; running: number; vcpu: number; memoryGB: number; diskGB: number; orphans: number; high: number
  hasSecurity: boolean; exposed: number; costList: number
  hasCostReal: boolean; costReal: number; costRealMonth: string; costRealCurrency: string
  hasSavings: boolean; savings: number; savingsCurrency: string
}
export interface InventorySummary { meta: Meta; sheets: SheetInfo[]; stats: InventoryStats; managed: number }
export interface Project {
  id: string; number: string; name: string; parent: string; state: string; created: string
  labels: Record<string, string> | null
}
export interface Progress { total: number; done: number; project: string; step: string; vms: number; errors: number; message: string }
export interface KeyInfo { clientEmail: string; projectId: string; keyId: string }
// fullScope false (Google sign-in, service account key, impersonation): Cloud
// Asset Inventory, Recommender and OS Config are unavailable.
export interface SessionInfo { identity: string; method: string; projectHint: string; fullScope: boolean }
export interface ADCStatus { found: boolean; identity: string }
export interface Modules {
  osconfig: boolean; monitoring: boolean; recommender: boolean; metricsDays: number; pricing: boolean; currency: string
  billingTable?: string; billingMonth?: string; billingMaxGB?: number
  fast?: boolean; fastScope?: string
}
export interface HealthOptions {
  stoppedDays: number; snapshotDays: number; imageDays: number
  cpuLowPct: number; cpuHighPct: number; memHighPct: number; quotaPct: number; cudDays: number
  disabled: string[]
}
export interface HealthRule { id: string; title: string; category: string; severity: string; param?: string; labels: string }
export interface ScanOptions { concurrency: number; rps?: number; health: HealthOptions; modules: Modules; saveHistory?: boolean; profile?: string }
export interface HistoryEntry {
  id: string; startedAt: string; identity: string; version: string; profile: string
  projects: number; projectsOk: number; cancelled: boolean
  vms: number; running: number; vcpu: number; memoryGB: number; disks: number; diskGB: number; orphans: number
  currency: string; costList: number; costReal: number; savings: number
  high: number; medium: number; low: number; sizeBytes: number
}
export interface Footprint { projectId: string; number: string; vms: number; running: number; disks: number; other: number }
export interface Profile {
  name: string; auth?: string; keyPath?: string; impersonate?: string
  query?: string; projects?: string[]; discover?: string
  concurrency?: number; rps?: number; modules: Modules; health: HealthOptions; updated?: string
}
export interface AboutInfo { version: string; website: string; donate: string; license: string; licenseUrl: string; releases: string }
export interface UpdateInfo { current: string; latest: string; available: boolean; url: string; notes: string; published: string }
export interface SnapshotInfo { path: string; name: string; startedAt: string; identity: string; version: string; projects: number; vms: number }
export type AuthMethod = 'adc' | 'sa' | 'impersonate' | 'oauth'
export interface OAuthStatus { available: boolean; builtin: boolean; custom: boolean; client: string; projectId: string; account: string; error?: string }
export type ExportFormat = 'xlsx' | 'csv' | 'json'

export interface Api {
  mock: boolean
  version(): Promise<string>
  setLanguage(lang: string): Promise<void>
  detectADC(): Promise<ADCStatus>
  pickKey(): Promise<KeyInfo | null>
  connect(method: AuthMethod, impersonate: string): Promise<SessionInfo>
  disconnect(): Promise<void>
  listProjects(query: string): Promise<Project[]>
  startScan(ids: string[], opt: ScanOptions): Promise<void>
  cancelScan(): Promise<void>
  inventorySummary(): Promise<InventorySummary | null>
  sheetRows(name: string, hideManaged: boolean): Promise<Row[]>
  exportInventory(format: ExportFormat, csvSep: string): Promise<string>
  onProgress(cb: (p: Progress) => void): () => void
  onDone(cb: (m: Meta) => void): () => void
  onError(cb: (msg: string) => void): () => void
  onWarning(cb: (msg: string) => void): () => void
  appInfo(): Promise<AboutInfo>
  checkUpdate(): Promise<UpdateInfo>
  openURL(url: string): void
  pickSnapshot(): Promise<SnapshotInfo | null>
  currentSnapshot(): Promise<SnapshotInfo | null>
  compare(before: string, after: string): Promise<Inventory>
  exportDiff(format: ExportFormat, csvSep: string): Promise<string>
  listProfiles(): Promise<Profile[]>
  saveProfile(p: Profile): Promise<void>
  deleteProfile(name: string): Promise<void>
  discover(scope: string): Promise<Footprint[]>
  healthRules(): Promise<HealthRule[]>
  listHistory(): Promise<HistoryEntry[]>
  openHistory(id: string): Promise<InventorySummary>
  deleteHistory(id: string): Promise<void>
  historySnapshot(id: string): Promise<SnapshotInfo>
  historyDir(): Promise<string>
  exportReport(): Promise<string>
  oauthStatus(): Promise<OAuthStatus>
  oauthPickClient(): Promise<OAuthStatus>
  oauthLogin(): Promise<OAuthStatus>
  oauthCancel(): Promise<void>
  oauthLogout(all: boolean): Promise<OAuthStatus>
  oauthUseBuiltin(): Promise<OAuthStatus>
}

/** Compare path prefix for history entries (see app.go HistoryPrefix). */
export const HISTORY = 'history:'

/** Whether scans are saved to the history (per-viewer preference, default on). */
export function loadSaveHistory(): boolean {
  try { return localStorage.getItem('saveHistory') !== 'false' } catch { return true }
}
export function storeSaveHistory(v: boolean) {
  try { localStorage.setItem('saveHistory', String(v)) } catch { /* storage unavailable */ }
}

/** Pseudo-path that selects the inventory in memory (see app.go). */
export const CURRENT = 'current'

const wailsApi: Api = {
  mock: false,
  version: () => App.Version(),
  setLanguage: (l) => App.SetLanguage(l),
  detectADC: () => App.DetectADC(),
  pickKey: async () => (await App.PickServiceAccountKey()) ?? null,
  connect: (m, sa) => App.Connect(m, sa),
  disconnect: () => App.Disconnect(),
  listProjects: async (q) => (await App.ListProjects(q)) ?? [],
  startScan: (ids, opt) => App.StartScan(ids, opt as never),
  cancelScan: () => App.CancelScan(),
  inventorySummary: async () => ((await App.InventorySummary()) as unknown as InventorySummary) ?? null,
  sheetRows: async (name, hide) => ((await App.SheetRows(name, hide)) as unknown as Row[]) ?? [],
  exportInventory: (f, sep) => App.Export(f, sep),
  onProgress: (cb) => EventsOn('scan:progress', cb),
  onDone: (cb) => EventsOn('scan:done', cb),
  onError: (cb) => EventsOn('scan:error', cb),
  onWarning: (cb) => EventsOn('scan:warning', cb),
  appInfo: () => App.AppInfo(),
  checkUpdate: () => App.CheckUpdate(),
  openURL: (url) => BrowserOpenURL(url),
  pickSnapshot: async () => ((await App.PickSnapshot()) as unknown as SnapshotInfo) ?? null,
  currentSnapshot: async () => ((await App.CurrentSnapshot()) as unknown as SnapshotInfo) ?? null,
  compare: async (a, b) => (await App.Compare(a, b)) as unknown as Inventory,
  exportDiff: (f, sep) => App.ExportDiff(f, sep),
  listProfiles: async () => ((await App.ListProfiles()) as unknown as Profile[]) ?? [],
  saveProfile: (p) => App.SaveProfile(p as never),
  deleteProfile: (n) => App.DeleteProfile(n),
  discover: async (scope) => ((await App.Discover(scope)) as unknown as Footprint[]) ?? [],
  healthRules: async () => ((await App.HealthRules()) as unknown as HealthRule[]) ?? [],
  listHistory: async () => ((await App.ListHistory()) as unknown as HistoryEntry[]) ?? [],
  openHistory: async (id) => (await App.OpenHistory(id)) as unknown as InventorySummary,
  deleteHistory: (id) => App.DeleteHistory(id),
  historySnapshot: async (id) => (await App.HistorySnapshot(id)) as unknown as SnapshotInfo,
  historyDir: () => App.HistoryDir(),
  exportReport: () => App.ExportReport(),
  oauthStatus: async () => (await App.OAuthStatus()) as unknown as OAuthStatus,
  oauthPickClient: async () => (await App.OAuthPickClient()) as unknown as OAuthStatus,
  oauthLogin: async () => (await App.OAuthLogin()) as unknown as OAuthStatus,
  oauthCancel: () => App.OAuthCancel(),
  oauthLogout: async (all) => (await App.OAuthLogout(all)) as unknown as OAuthStatus,
  oauthUseBuiltin: async () => (await App.OAuthUseBuiltin()) as unknown as OAuthStatus,
}

const inWails = typeof window !== 'undefined' && (window as unknown as { go?: unknown }).go !== undefined
export const api: Api = inWails ? wailsApi : mockApi

export function errorText(e: unknown): string {
  if (typeof e === 'string') return e
  if (e instanceof Error) return e.message
  return JSON.stringify(e)
}
