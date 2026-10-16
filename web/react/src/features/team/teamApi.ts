import { encodeCursor, type PaginationMeta } from '../../shared/Pagination'
import type { Sector, SectorDraft, TeamData, TeamMember, TeamUsersData, UserDraft } from './types'

type Envelope<T> = {
  data?: T
  meta?: PaginationMeta
  error?: { message?: string }
}

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    ...options,
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) },
  })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) {
    throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  }
  return payload.data as T
}

async function requestEnvelope<T>(url: string, options?: RequestInit): Promise<{ data: T; meta: PaginationMeta }> {
  const response = await fetch(url, {
    ...options,
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) },
  })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) {
    throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  }
  return { data: payload.data as T, meta: payload.meta || {} }
}

export function loadTeam() {
  return request<TeamData>('/api/react/team')
}

export function createSector(draft: SectorDraft) {
  return request<Sector>('/api/react/sectors', { method: 'POST', body: JSON.stringify(draft) })
}

export function updateSector(id: number, draft: SectorDraft) {
  return request<Sector>(`/api/react/sectors/${id}`, { method: 'PATCH', body: JSON.stringify(draft) })
}

export function toggleSector(id: number) {
  return request<Sector>(`/api/react/sectors/${id}/toggle`, { method: 'POST', body: '{}' })
}

export function deleteSector(id: number) {
  return request<{ deleted: number }>(`/api/react/sectors/${id}`, { method: 'DELETE' })
}

export async function loadUsers(params?: { limit?: number; offset?: number; q?: string; role?: string; status?: string }) {
  const search = new URLSearchParams()
  if (params?.limit) search.set('limit', String(params.limit))
  if (params?.offset) search.set('cursor', encodeCursor(params.offset))
  if (params?.q) search.set('q', params.q)
  if (params?.role) search.set('role', params.role)
  if (params?.status) search.set('status', params.status)
  const query = search.toString()
  const { data, meta } = await requestEnvelope<TeamUsersData>(`/api/react/users${query ? `?${query}` : ''}`)
  return { ...data, meta }
}

export function createUser(draft: UserDraft) {
  return request<TeamMember>('/api/react/users', { method: 'POST', body: JSON.stringify(draft) })
}

export function updateUser(id: number, draft: UserDraft) {
  return request<TeamMember>(`/api/react/users/${id}`, { method: 'PATCH', body: JSON.stringify(draft) })
}

export function toggleUser(id: number) {
  return request<TeamMember>(`/api/react/users/${id}/toggle`, { method: 'POST', body: '{}' })
}

export function deleteUser(id: number) {
  return request<{ deleted: number }>(`/api/react/users/${id}`, { method: 'DELETE' })
}
