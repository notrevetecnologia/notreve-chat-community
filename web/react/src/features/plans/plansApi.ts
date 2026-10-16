import type { EffectiveLimits, PlanDraft, Plano, TenantPlanAssignment } from './types'

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

export function loadPlans() {
  return request<Plano[]>('/api/react/platform/plans')
}

export function createPlan(plan: PlanDraft) {
  return request<Plano>('/api/react/platform/plans', { method: 'POST', body: JSON.stringify(plan) })
}

export function updatePlan(id: number, plan: PlanDraft) {
  return request<Plano>(`/api/react/platform/plans/${id}`, { method: 'PATCH', body: JSON.stringify(plan) })
}

export function deletePlan(id: number) {
  return request<{ deleted: boolean }>(`/api/react/platform/plans/${id}`, { method: 'DELETE' })
}

export function loadTenantPlan(tenantId: number) {
  return request<EffectiveLimits>(`/api/react/platform/tenants/${tenantId}/plan`)
}

export function saveTenantPlan(tenantId: number, assignment: TenantPlanAssignment) {
  return request<EffectiveLimits>(`/api/react/platform/tenants/${tenantId}/plan`, { method: 'PATCH', body: JSON.stringify(assignment) })
}
