import type { PaginationMeta } from '../../shared/Pagination'

export type ChannelConnectionState = {
  connected: boolean
  loggedIn: boolean
  jid?: string
  avatar?: string
  ready?: boolean
  webhook_set?: boolean
  error?: string
}

export type ChannelOperationResult = ChannelConnectionState & {
  qr?: string
  already?: boolean
  ok?: boolean
}

export type ChannelOperationResponse<C = unknown> = {
  action: string
  channel: C
  result?: ChannelOperationResult
}

export type ChannelDisconnectAction = 'disconnect' | 'logout'

type ApiEnvelope<T> = {
  data?: T
  meta?: PaginationMeta
  error?: { message?: string }
}

export async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const headers = options?.body instanceof FormData ? { ...(options?.headers ?? {}) } : { 'Content-Type': 'application/json', ...(options?.headers ?? {}) }
  const response = await fetch(url, { ...options, credentials: 'same-origin', headers })
  const raw = await response.text()
  const contentType = response.headers.get('content-type')?.toLowerCase() ?? ''
  if (!contentType.includes('application/json')) {
    if (response.redirected || response.url.includes('/login') || /^\s*<!doctype html|^\s*<html[\s>]/i.test(raw)) {
      throw new Error('Sua sessão expirou. Faça login novamente.')
    }
    throw new Error(`O servidor respondeu com uma resposta inesperada (HTTP ${response.status}).`)
  }
  let payload: ApiEnvelope<T>
  try {
    payload = JSON.parse(raw) as ApiEnvelope<T>
  } catch {
    throw new Error('O servidor enviou um JSON inválido.')
  }
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return payload.data as T
}

export async function connectChannel<C = unknown>(channelId: number): Promise<ChannelOperationResponse<C>> {
  return request<ChannelOperationResponse<C>>('/api/react/channels', { method: 'POST', body: JSON.stringify({ action: 'connect', channel_id: channelId }) })
}

export async function fetchChannelStatus<C = unknown>(channelId: number): Promise<ChannelOperationResponse<C>> {
  return request<ChannelOperationResponse<C>>('/api/react/channels', { method: 'POST', body: JSON.stringify({ action: 'status', channel_id: channelId }) })
}

export async function disconnectChannel<C = unknown>(channelId: number, action: ChannelDisconnectAction = 'disconnect'): Promise<ChannelOperationResponse<C>> {
  return request<ChannelOperationResponse<C>>('/api/react/channels', { method: 'POST', body: JSON.stringify({ action, channel_id: channelId }) })
}

export async function testChannelConnection<C = unknown>(channelId: number): Promise<ChannelOperationResponse<C>> {
  return request<ChannelOperationResponse<C>>('/api/react/channels', { method: 'POST', body: JSON.stringify({ action: 'test', channel_id: channelId }) })
}

export async function deleteChannel(channelId: number): Promise<void> {
  await request<{ deleted: number }>(`/api/react/channels/${channelId}`, { method: 'DELETE' })
}

export async function requestEnvelope<T>(url: string, options?: RequestInit): Promise<{ data: T; meta: PaginationMeta }> {
  const response = await fetch(url, { ...options, credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) } })
  const raw = await response.text()
  const payload = JSON.parse(raw) as ApiEnvelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return { data: payload.data as T, meta: payload.meta || {} }
}
