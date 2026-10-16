import type { OperationalSettings, SettingsData } from './types'

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

export function loadSettings() {
  return request<SettingsData>('/api/react/settings')
}

export function saveSettings(settings: OperationalSettings) {
  return request<SettingsData>('/api/react/settings', { method: 'PATCH', body: JSON.stringify(settings) })
}
