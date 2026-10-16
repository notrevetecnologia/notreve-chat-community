export type MFAStatus = {
  enabled: boolean
  required: boolean
}

type Envelope<T> = {
  data?: T
  error?: { code?: string; message?: string }
}

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    ...options,
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) },
  })
  const payload = await response.json().catch(() => null) as Envelope<T> | null
  if (!response.ok) {
    throw new Error(payload?.error?.message || 'Não foi possível concluir a operação.')
  }
  return payload?.data as T
}

export function loadMFAStatus() {
  return request<MFAStatus>('/api/react/mfa')
}

export function startMFASetup() {
  return request<{ enabled: boolean; secret?: string; uri?: string; qr?: string }>('/api/react/mfa/setup')
}

export function confirmMFA(code: string) {
  return request<{ verified: boolean }>('/api/react/mfa/confirm', { method: 'POST', body: JSON.stringify({ code }) })
}

export function disableMFA(code: string) {
  return request<{ enabled: boolean }>('/api/react/mfa/disable', { method: 'POST', body: JSON.stringify({ code }) })
}
