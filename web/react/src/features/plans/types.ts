export type PlanoRecurso = {
  id?: number
  plano_id?: number
  recurso: string
  limite: number
}

export type Plano = {
  id: number
  tenant_owner: number | null
  nome: string
  preco: number
  intervalo: string
  ativo: boolean
  recursos?: PlanoRecurso[]
}

export type PlanDraft = {
  nome: string
  preco: number
  intervalo: string
  ativo: boolean
  recursos: PlanoRecurso[]
}

export type EffectiveLimits = {
  tenant_id: number
  plano_id: number | null
  plano_nome: string
  status: string
  trial_until: string | null
  recursos: Record<string, number>
  uso: Record<string, number>
}

export type TenantPlanAssignment = {
  plano_id: number | null
  trial_until: string | null
  status: string
  limits_override: Record<string, number>
}
