import type { PaginationMeta } from '../../shared/Pagination'

export type Provider = {
  id: number
  name: string
  provider_type: 'notreve' | 'openai_compat'
  base_url: string
  model: string
  active: boolean
  safe_mode: boolean
  capacidade: 'texto' | 'multimodal'
  capabilities: string[]
  has_api_key: boolean
}

export type ProviderDraft = {
  name: string
  provider_type: Provider['provider_type']
  base_url: string
  model: string
  api_key: string
  active: boolean
  safe_mode: boolean
  capacidade: Provider['capacidade']
  capabilities: string[]
}

export type ProviderModel = { id: string; name: string }
export type Envelope<T> = { data?: T; meta?: PaginationMeta; error?: { message?: string } }
export type ProviderResponse = { items: Provider[] }
export type ProviderModelResponse = { items: ProviderModel[] }
export type ProviderTestResult = { ok: boolean; message: string }

export const emptyDraft = (): ProviderDraft => ({ name: 'Notreve IA', provider_type: 'notreve', base_url: '', model: '', api_key: '', active: true, safe_mode: true, capacidade: 'texto', capabilities: [] })

export const defaultNotreveModels: ProviderModel[] = [
  { id: 'notreve-v1-pro', name: 'notreve-v1-pro · texto (recomendado)' },
  { id: 'notreve-vision', name: 'notreve-vision · multimodal' },
]

export function mergeModels(models: ProviderModel[], current: string): ProviderModel[] {
  const merged = [...defaultNotreveModels, ...models.filter((model) => !defaultNotreveModels.some((item) => item.id === model.id))]
  if (current && !merged.some((model) => model.id === current)) return [{ id: current, name: current }, ...merged]
  return merged
}

export async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, { ...options, credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) } })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return payload.data as T
}

export async function requestEnvelope<T>(url: string, options?: RequestInit): Promise<{ data: T; meta: PaginationMeta }> {
  const response = await fetch(url, { ...options, credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) } })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return { data: payload.data as T, meta: payload.meta || {} }
}
