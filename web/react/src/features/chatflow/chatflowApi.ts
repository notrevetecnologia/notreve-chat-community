import type { Chatflow, ChatflowData, ChatflowDraft } from './types'

type Envelope<T> = {
  data?: T
  error?: { message?: string }
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

export function loadChatflows() {
  return request<ChatflowData>('/api/react/chatflows')
}

export function getChatflow(id: number) {
  return request<Chatflow>(`/api/react/chatflows/${id}`)
}

export function createChatflow(draft: ChatflowDraft) {
  return request<Chatflow>('/api/react/chatflows', { method: 'POST', body: JSON.stringify(draft) })
}

export function updateChatflow(id: number, draft: ChatflowDraft) {
  return request<Chatflow>(`/api/react/chatflows/${id}`, { method: 'PATCH', body: JSON.stringify(draft) })
}

export function deleteChatflow(id: number) {
  return request<{ deleted: number }>(`/api/react/chatflows/${id}`, { method: 'DELETE' })
}

export function toggleChatflow(id: number) {
  return request<boolean>(`/api/react/chatflows/${id}/activate`, { method: 'POST' })
}

export function attachChannel(chatflowID: number, channelID: number, attached: boolean) {
  return request<{ channels: ChatflowData['channels'] }>(`/api/react/chatflows/${chatflowID}/channels`, {
    method: 'POST',
    body: JSON.stringify({ channel_id: channelID, attached }),
  })
}
