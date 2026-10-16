import type { KnowledgeData, KnowledgeDraft, KnowledgeSource } from './types'

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

export function loadKnowledge(agentID?: number) {
  const query = agentID ? `?agent_id=${encodeURIComponent(agentID)}` : ''
  return request<KnowledgeData>(`/api/react/knowledge${query}`)
}

export function createKnowledgeSource(draft: KnowledgeDraft, agentID: number) {
  return request<KnowledgeSource>('/api/react/knowledge', { method: 'POST', body: JSON.stringify({ ...draft, agent_id: agentID }) })
}

export function deleteKnowledgeSource(id: number) {
  return request<{ deleted: number }>(`/api/react/knowledge/${id}`, { method: 'DELETE' })
}
