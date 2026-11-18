// Thin typed facade over the Wails bindings. When the page runs outside
// Wails (plain `npm run dev` in a browser) it falls back to a mock backend,
// so the UI can be designed and reviewed without GCP credentials.
import * as App from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
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
export interface Project {
  id: string; number: string; name: string; parent: string; state: string; created: string
  labels: Record<string, string> | null
}
export interface Progress { total: number; done: number; project: string; step: string; vms: number; errors: number; message: string }
export interface KeyInfo { clientEmail: string; projectId: string; keyId: string }
export interface SessionInfo { identity: string; method: string; projectHint: string }
export interface ADCStatus { found: boolean; identity: string }
export interface Modules { osconfig: boolean; monitoring: boolean; recommender: boolean; metricsDays: number; pricing: boolean; currency: string }
export interface ScanOptions { concurrency: number; health: { stoppedDays: number; snapshotDays: number }; modules: Modules }
export type AuthMethod = 'adc' | 'sa' | 'impersonate'
export type ExportFormat = 'xlsx' | 'csv' | 'json'

export interface Api {
  mock: boolean
  version(): Promise<string>
  detectADC(): Promise<ADCStatus>
  pickKey(): Promise<KeyInfo | null>
  connect(method: AuthMethod, impersonate: string): Promise<SessionInfo>
  disconnect(): Promise<void>
  listProjects(query: string): Promise<Project[]>
  startScan(ids: string[], opt: ScanOptions): Promise<void>
  cancelScan(): Promise<void>
  getInventory(): Promise<Inventory | null>
  exportInventory(format: ExportFormat, csvSep: string): Promise<string>
  onProgress(cb: (p: Progress) => void): () => void
  onDone(cb: (m: Meta) => void): () => void
  onError(cb: (msg: string) => void): () => void
}

const wailsApi: Api = {
  mock: false,
  version: () => App.Version(),
  detectADC: () => App.DetectADC(),
  pickKey: async () => (await App.PickServiceAccountKey()) ?? null,
  connect: (m, sa) => App.Connect(m, sa),
  disconnect: () => App.Disconnect(),
  listProjects: async (q) => (await App.ListProjects(q)) ?? [],
  startScan: (ids, opt) => App.StartScan(ids, opt as never),
  cancelScan: () => App.CancelScan(),
  getInventory: async () => ((await App.GetInventory()) as unknown as Inventory) ?? null,
  exportInventory: (f, sep) => App.Export(f, sep),
  onProgress: (cb) => EventsOn('scan:progress', cb),
  onDone: (cb) => EventsOn('scan:done', cb),
  onError: (cb) => EventsOn('scan:error', cb),
}

const inWails = typeof window !== 'undefined' && (window as unknown as { go?: unknown }).go !== undefined
export const api: Api = inWails ? wailsApi : mockApi

export function errorText(e: unknown): string {
  if (typeof e === 'string') return e
  if (e instanceof Error) return e.message
  return JSON.stringify(e)
}
