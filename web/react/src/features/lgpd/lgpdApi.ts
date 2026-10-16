import type { PaginationMeta } from '../../shared/Pagination'
import type { AuditLog, ConsentInput, LGPDConsent, LGPDPolitica, LGPDSummary, PoliticaInput } from './types'

type Envelope<T> = {
  data?: T
  meta?: PaginationMeta
  error?: { message?: string }
}

type Page<T> = {
  items: T[]
  total: number
  next_cursor: string
  has_more: boolean
}

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    ...options,
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) },
  })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return payload.data as T
}

async function requestPage<T>(url: string): Promise<Page<T>> {
  const response = await fetch(url, { credentials: 'same-origin', headers: { 'Content-Type': 'application/json' } })
  const payload = await response.json() as Envelope<T[]>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  const meta = payload.meta || {}
  return { items: payload.data || [], total: meta.total ?? 0, next_cursor: meta.next_cursor ?? '', has_more: Boolean(meta.has_more) }
}

export function loadLGPDSummary() {
  return request<LGPDSummary>('/api/react/lgpd/summary')
}

export function listConsents(cursor = '', scope = '') {
  const params = new URLSearchParams()
  if (cursor) params.set('cursor', cursor)
  if (scope) params.set('scope', scope)
  const query = params.toString()
  return requestPage<LGPDConsent>(`/api/react/lgpd/consents${query ? `?${query}` : ''}`)
}

export function createConsent(input: ConsentInput) {
  return request<{ id: number }>('/api/react/lgpd/consents', { method: 'POST', body: JSON.stringify(input) })
}

export function listPoliticas() {
  return request<LGPDPolitica[]>('/api/react/lgpd/politicas')
}

export function createPolitica(input: PoliticaInput) {
  return request<LGPDPolitica>('/api/react/lgpd/politica', { method: 'POST', body: JSON.stringify(input) })
}

export function listAudit(cursor = '', q = '', acao = '') {
  const params = new URLSearchParams()
  if (cursor) params.set('cursor', cursor)
  if (q) params.set('q', q)
  if (acao) params.set('acao', acao)
  const query = params.toString()
  return requestPage<AuditLog>(`/api/react/lgpd/audit${query ? `?${query}` : ''}`)
}
