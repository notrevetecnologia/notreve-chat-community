import { useCallback, useEffect, useMemo, useState } from 'react'
import './Plans.css'
import { createPlan, deletePlan, loadPlans, loadTenantPlan, saveTenantPlan, updatePlan } from './plansApi'
import type { EffectiveLimits, PlanDraft, Plano, PlanoRecurso, TenantPlanAssignment } from './types'

type Tenant = { id: number; name: string; slug: string; status: string }

const resourceKeys = [
  ['max_canais', 'Canais'],
  ['max_atendentes', 'Atendentes'],
  ['max_campanhas_ativas', 'Campanhas simultâneas'],
  ['campanha_limite_diario', 'Campanha limite diário'],
  ['max_agentes', 'Agentes ativos'],
  ['max_usuarios', 'Usuários ativos'],
  ['max_conexoes_whatsapp', 'Conexões WhatsApp'],
  ['max_tools', 'Tools para agentes'],
  ['max_api_keys', 'Chaves de API'],
  ['max_webhooks', 'Webhooks ativos'],
  ['creditos_ia_mensais', 'Créditos IA mensais'],
] as const

const resourceLabel = (key: string) => resourceKeys.find((entry) => entry[0] === key)?.[1] ?? key

function emptyPlanDraft(): PlanDraft {
  return { nome: '', preco: 0, intervalo: 'mensal', ativo: true, recursos: [] }
}

function emptyAssignment(plans: Plano[]): TenantPlanAssignment {
  const fallback = plans.find((plan) => plan.nome === 'Padrão') ?? plans[0]
  return { plano_id: fallback?.id ?? null, trial_until: null, status: 'active', limits_override: {} }
}

export default function PlansPage({ onBack }: { onBack?: () => void }) {
  const [plans, setPlans] = useState<Plano[]>([])
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [planDraft, setPlanDraft] = useState<PlanDraft>(emptyPlanDraft)
  const [editingPlanID, setEditingPlanID] = useState<number | null>(null)
  const [selectedTenant, setSelectedTenant] = useState<number | null>(null)
  const [tenantDraft, setTenantDraft] = useState<TenantPlanAssignment>(emptyAssignment([]))
  const [effective, setEffective] = useState<EffectiveLimits | null>(null)
  const [dirty, setDirty] = useState(false)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [planData, tenantData] = await Promise.all([
        loadPlans(),
        fetch('/api/react/platform/tenants', { credentials: 'same-origin' }).then(async (response) => {
          if (!response.ok) return []
          const payload = await response.json() as { data?: Tenant[] | { tenants?: Tenant[] } }
          const data = payload.data
          return Array.isArray(data) ? data : data?.tenants ?? []
        }),
      ])
      setPlans(planData)
      setTenants(tenantData)
      setError(null)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Não foi possível carregar os planos.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  useEffect(() => {
    if (!selectedTenant) { setEffective(null); return }
    let active = true
    void loadTenantPlan(selectedTenant)
      .then((limits) => {
        if (!active) return
        setEffective(limits)
        setTenantDraft((current) => ({
          ...current,
          plano_id: limits.plano_id,
          trial_until: limits.trial_until,
          status: limits.status,
        }))
        setDirty(false)
      })
      .catch((cause) => { if (active) setError(cause instanceof Error ? cause.message : 'Não foi possível carregar os limites.') })
    return () => { active = false }
  }, [selectedTenant])

  const changePlan = <K extends keyof PlanDraft>(key: K, value: PlanDraft[K]) => setPlanDraft((current) => ({ ...current, [key]: value }))
  const changeResource = (index: number, field: keyof PlanoRecurso, value: string | number) => {
    setPlanDraft((current) => {
      const recursos = current.recursos.map((recurso, i) => (i === index ? { ...recurso, [field]: value } : recurso))
      return { ...current, recursos }
    })
  }
  const addResource = () => setPlanDraft((current) => {
    const used = new Set(current.recursos.map((recurso) => recurso.recurso))
    const next = resourceKeys.find(([key]) => !used.has(key))?.[0]
    if (!next) return current
    return { ...current, recursos: [...current.recursos, { recurso: next, limite: 0 }] }
  })
  const removeResource = (index: number) => setPlanDraft((current) => ({
    ...current,
    recursos: current.recursos.filter((_, i) => i !== index),
  }))

  const savePlan = async () => {
    const nome = planDraft.nome.trim()
    if (!nome) { setError('Informe o nome do plano.'); return }
    const seen = new Set<string>()
    for (const recurso of planDraft.recursos) {
      if (seen.has(recurso.recurso)) { setError(`O recurso “${resourceLabel(recurso.recurso)}” está duplicado.`); return }
      seen.add(recurso.recurso)
    }
    setSaving(true)
    try {
      const payload = { ...planDraft, nome, preco: Math.max(0, Number(planDraft.preco) || 0) }
      if (editingPlanID === null) {
        await createPlan(payload)
        setNotice('Plano criado.')
      } else {
        await updatePlan(editingPlanID, payload)
        setNotice('Plano atualizado.')
      }
      setPlanDraft(emptyPlanDraft())
      setEditingPlanID(null)
      await load()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Não foi possível salvar o plano.')
    } finally {
      setSaving(false)
    }
  }

  const editPlan = (plan: Plano) => {
    setEditingPlanID(plan.id)
    setPlanDraft({
      nome: plan.nome,
      preco: plan.preco,
      intervalo: plan.intervalo,
      ativo: plan.ativo,
      recursos: (plan.recursos ?? []).map((recurso) => ({ recurso: recurso.recurso, limite: recurso.limite })),
    })
  }

  const erasePlan = async (plan: Plano) => {
    if (!window.confirm(`Excluir o plano “${plan.nome}”?`)) return
    try {
      await deletePlan(plan.id)
      setNotice('Plano excluído.')
      if (editingPlanID === plan.id) { setEditingPlanID(null); setPlanDraft(emptyPlanDraft()) }
      await load()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Não foi possível excluir o plano.')
    }
  }

  const changeOverride = (key: string, value: string) => {
    setTenantDraft((current) => {
      const numeric = Math.max(0, Number(value) || 0)
      if (value === '' || value === '0') {
        const next = { ...current.limits_override }
        delete next[key]
        return { ...current, limits_override: next }
      }
      return { ...current, limits_override: { ...current.limits_override, [key]: numeric } }
    })
    setDirty(true)
  }

  const saveAssignment = async () => {
    if (selectedTenant === null) return
    setSaving(true)
    try {
      await saveTenantPlan(selectedTenant, tenantDraft)
      setNotice('Plano e limites da empresa atualizados.')
      const limits = await loadTenantPlan(selectedTenant)
      setEffective(limits)
      setTenantDraft((current) => ({
        ...current,
        plano_id: limits.plano_id,
        trial_until: limits.trial_until,
        status: limits.status,
      }))
      setDirty(false)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Não foi possível salvar os limites.')
    } finally {
      setSaving(false)
    }
  }

  const selectedTenantName = useMemo(() => tenants.find((tenant) => tenant.id === selectedTenant), [tenants, selectedTenant])

  return (
    <div className="plans-page">
      <section className="page-intro plans-intro">
        <div>
          <span className="eyebrow">Notreve · Planos e limites</span>
          <h2>Catálogo de planos, recursos e limites efetivos por empresa.</h2>
          <p>Defina quais recursos cada plano franqueia e, quando necessário, sobrescreva um limite para uma empresa específica.</p>
        </div>
        <div className="intro-actions">
          {onBack && <button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button>}
          <button className="secondary-button" type="button" onClick={() => void load()}>↻ Atualizar</button>
        </div>
      </section>

      {error && <Alert kind="alert" text={error} onClose={() => setError(null)} />}
      {notice && <Alert kind="notice" text={notice} onClose={() => setNotice(null)} />}

      {loading ? <div className="workspace-state">Carregando planos…</div> : (
        <div className="plans-layout">
          <section className="plans-panel">
            <div className="plans-panel-heading">
              <div><h3>Catálogo de planos</h3><p>Crie planos e defina os recursos francesa em cada um. 0 significa ilimitado.</p></div>
            </div>
            <div className="plans-split">
              <div className="plans-list">
                {plans.length === 0 ? <div className="workspace-empty"><span>▦</span><strong>Nenhum plano</strong><p>Crie o primeiro plano para começar a franquear recursos.</p></div> : plans.map((plan) => (
                  <button className={`plan-row${editingPlanID === plan.id ? ' active' : ''}`} type="button" key={plan.id} onClick={() => editPlan(plan)}>
                    <span className="plan-row-copy"><strong>{plan.nome}</strong><small>{plan.intervalo} · {formatMoney(plan.preco)} · {(plan.recursos ?? []).length} recursos</small></span>
                    <span className={`status-badge ${plan.ativo ? 'status-active' : 'status-disabled'}`}>{plan.ativo ? 'Ativo' : 'Inativo'}</span>
                    <span className="row-action">{editingPlanID === plan.id ? 'Editando' : 'Editar'} →</span>
                  </button>
                ))}
              </div>
              <div className="plans-form">
                <div className="form-heading"><div><strong>{editingPlanID === null ? 'Novo plano' : 'Editar plano'}</strong><p>Nome e característica da oferta.</p></div>{editingPlanID !== null && <button className="inline-action" type="button" onClick={() => setEditingPlanID(null)}>Nova ficha</button>}</div>
                <div className="plans-form-grid">
                  <label className="field"><span>Nome do plano</span><input value={planDraft.nome} placeholder="Ex.: Essencial" onChange={(event) => changePlan('nome', event.target.value)} /></label>
                  <label className="field"><span>Preço</span><input type="number" min="0" step="0.01" value={planDraft.preco} onChange={(event) => changePlan('preco', Number(event.target.value) || 0)} /></label>
                  <label className="field"><span>Intervalo</span><select value={planDraft.intervalo} onChange={(event) => changePlan('intervalo', event.target.value)}><option value="semanal">Semanal</option><option value="mensal">Mensal</option><option value="anual">Anual</option></select></label>
                  <label className="module-option"><input type="checkbox" checked={planDraft.ativo} onChange={(event) => changePlan('ativo', event.target.checked)} /><span><strong>Plano ativo</strong><small>Permite atribuir a empresas</small></span></label>
                </div>
                <div className="form-subsection"><div><strong>Recursos do plano</strong><p>0 significa ilimitado; para créditos IA, 0 significa sem franquia pré-paga.</p></div>
                  <div className="plans-recursos">{planDraft.recursos.length === 0 ? <span className="plans-hint">Nenhum recurso adicionado. Utilize os limites padrão da empresa.</span> : planDraft.recursos.map((recurso, index) => (
                    <div className="recurso-row" key={index}>
                      <span className="recurso-label">{resourceLabel(recurso.recurso)}</span>
                      <input type="number" min="0" step="1" value={recurso.limite} onChange={(event) => changeResource(index, 'limite', Number(event.target.value) || 0)} />
                      <button className="recurso-remove" type="button" onClick={() => removeResource(index)} aria-label="Remover recurso">×</button>
                    </div>
                  ))}</div>
                  <button className="secondary-button" type="button" onClick={addResource}>+ Adicionar recurso</button>
                </div>
                <div className="form-actions" style={{ marginTop: 16 }}>
                  <button className="primary-button" type="button" disabled={saving} onClick={() => void savePlan()}>{saving ? 'Salvando…' : editingPlanID === null ? 'Criar plano ↗' : 'Salvar plano ↗'}</button>
                  {editingPlanID !== null && <button className="secondary-button" type="button" onClick={() => { setEditingPlanID(null); setPlanDraft(emptyPlanDraft()) }}>Cancelar edição</button>}
                </div>
              </div>
            </div>
          </section>

          <section className="plans-panel">
            <div className="plans-panel-heading"><div><h3>Empresas · plano e limites efetivos</h3><p>Atribua um plano, marque teste/suspensão e sobrescreva um limite específico.</p></div></div>
            <div className="plans-split">
              <div className="plans-list">
                <label className="field"><span>Selecione a empresa</span><select value={selectedTenant ?? ''} onChange={(event) => setSelectedTenant(event.target.value ? Number(event.target.value) : null)}><option value="">Escolher…</option>{tenants.map((tenant) => <option key={tenant.id} value={tenant.id}>{tenant.name} · /{tenant.slug}</option>)}</select></label>
                {effective && selectedTenantName && <div className="tenant-plan-summary"><span className="tenant-initial">{(selectedTenantName.name || 'N').slice(0, 1).toUpperCase()}</span><span className="plan-row-copy"><strong>{effective.plano_nome || 'Sem plano contratado'}</strong><small>{effective.status === 'trial' ? 'Em teste' : effective.status === 'suspended' ? 'Suspensa' : 'Ativa'}{effective.trial_until ? ` · teste até ${formatDate(effective.trial_until)}` : ''}</small></span></div>}
              </div>
              <div className="plans-form">
                {selectedTenant === null ? <div className="workspace-empty"><span>◈</span><strong>Escolha uma empresa</strong><p>Selecione uma empresa para consultar e editar os limites efetivos.</p></div> : <div className="plans-assignment">
                  <div className="plans-form-grid">
                    <label className="field"><span>Plano contratado</span><select value={tenantDraft.plano_id ?? ''} onChange={(event) => { setTenantDraft((current) => ({ ...current, plano_id: event.target.value ? Number(event.target.value) : null })); setDirty(true) }}><option value="">Sem plano (padrão)</option>{plans.filter((plan) => plan.ativo).map((plan) => <option key={plan.id} value={plan.id}>{plan.nome}</option>)}</select></label>
                    <label className="field"><span>Status da empresa</span><select value={tenantDraft.status} onChange={(event) => { setTenantDraft((current) => ({ ...current, status: event.target.value })); setDirty(true) }}><option value="active">Ativa</option><option value="trial">Em teste</option><option value="suspended">Suspensa</option><option value="closed">Encerrada</option></select></label>
                    <label className="field"><span>Fim do teste</span><input type="date" value={tenantDraft.trial_until ? tenantDraft.trial_until.slice(0, 10) : ''} onChange={(event) => { setTenantDraft((current) => ({ ...current, trial_until: event.target.value ? `${event.target.value}T00:00:00Z` : null })); setDirty(true) }} disabled={tenantDraft.status !== 'trial'} /></label>
                  </div>
                  <div className="form-subsection"><div><strong>Sobrescrever limite</strong><p>Valores maiores que 0 sobrescrevem a franquia do plano para esta empresa. Remova para voltar ao plano.</p></div>
                    <div className="plans-recursos">{Object.keys(tenantDraft.limits_override).length === 0 ? <span className="plans-hint">Sem override. A empresa segue o plano contratado.</span> : Object.entries(tenantDraft.limits_override).map(([key, value]) => (
                      <div className="recurso-row" key={key}><span className="recurso-label">{resourceLabel(key)}</span><input type="number" min="0" step="1" value={value} onChange={(event) => changeOverride(key, event.target.value)} /><button className="recurso-remove" type="button" onClick={() => changeOverride(key, '')} aria-label="Remover override">×</button></div>
                    ))}</div>
                    <div className="plans-override-add"><select value="" onChange={(event) => { if (event.target.value) changeOverride(event.target.value, '1') }}><option value="">Adicionar override…</option>{resourceKeys.filter(([key]) => !(key in tenantDraft.limits_override)).map(([key, label]) => <option key={key} value={key}>{label}</option>)}</select></div>
                  </div>
                  {effective && <div className="form-subsection"><div><strong>Limites efetivos</strong><p>Resultado do plano + override. Mostra o limite e o consumo atual.</p></div><div className="plans-recursos">{Object.entries(effective.recursos).map(([key, max]) => { const uso = effective.uso[key] ?? 0; return <div className="plans-recursos-efetivos" key={key}><span>{resourceLabel(key)}</span><span>{uso} / {max === 0 ? 'ilimitado' : max}</span></div> })}</div></div>}
                  <div className="form-actions" style={{ marginTop: 16 }}><button className="primary-button" type="button" disabled={saving || !dirty} onClick={() => void saveAssignment()}>{saving ? 'Salvando…' : 'Salvar plano e limites ↗'}</button></div>
                </div>}
              </div>
            </div>
          </section>
        </div>
      )}
    </div>
  )
}

function Alert({ kind, text, onClose }: { kind: 'alert' | 'notice'; text: string; onClose: () => void }) {
  return <div className={`workspace-${kind}`} role={kind === 'alert' ? 'alert' : 'status'}><span>{kind === 'alert' ? '!' : '✓'}</span>{text}<button type="button" onClick={onClose} aria-label="Fechar aviso">×</button></div>
}

function formatMoney(value: number) {
  return `R$ ${value.toFixed(2).replace('.', ',')}`
}

function formatDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleDateString('pt-BR')
}
