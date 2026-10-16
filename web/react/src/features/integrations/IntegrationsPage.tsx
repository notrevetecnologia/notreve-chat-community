import { useCallback, useEffect, useState } from 'react'
import { Modal } from '../../shared/Modal'
import './Integrations.css'
import { ApiDocsPanel } from './ApiDocs'
import { McpPanel } from './McpPanel'

type IntegrationPolicy = { connector: string; risk: string; requires_confirmation: boolean; required_scopes: string[]; allowed_hosts: string[]; timeout_seconds: number; max_calls_per_minute: number; max_calls_per_hour: number; max_response_bytes: number; max_argument_bytes: number; requires_identity: string; allowed_identities: string[]; identity_config: string; identity_verify_url: string; identity_verify_method: string; identity_verify_param: string; identity_match_field: string; identity_client_id_field: string }
type CatalogEntry = { key: string; name: string; description: string; method: string; risk: string; requires_confirmation: boolean; required_scopes: string[]; timeout_seconds: number; max_calls_per_minute: number; max_calls_per_hour: number; parameters?: string; requires_identity?: string; identity_hint?: string }
type Integration = { id: number; name: string; description: string; method: string; policy: IntegrationPolicy; has_credentials: boolean }
type IntegrationDetail = Integration & { url: string; headers: Record<string, string>; parameters: string }
type IntegrationDraft = { name: string; description: string; method: string; url: string; bearer_token: string; headers: string; parameters: string; connector: string; risk: string; requires_confirmation: boolean; required_scopes: string; allowed_hosts: string; timeout_seconds: string; max_calls_per_minute: string; max_calls_per_hour: string; max_response_bytes: string; max_argument_bytes: string; requires_identity: string; allowed_identities: string; identity_config: string; identity_verify_url: string; identity_verify_method: string; identity_verify_param: string; identity_match_field: string; identity_client_id_field: string }
type APIKey = { id: number; name: string; prefix: string; scopes: string[]; active: boolean }
type WebhookEvent = { key: string; name: string; description: string; version: number }
type WebhookSubscription = { id: number; name: string; endpoint_url: string; events: string[]; has_signing_secret: boolean; active: boolean; last_delivery_at?: string; created_at: string; updated_at: string }
type WebhookDraft = { name: string; endpoint_url: string; events: string[]; signing_secret: string; active: boolean }
type Envelope<T> = { data?: T; error?: { message?: string } }
type IntegrationResponse = { items: Integration[]; catalog?: CatalogEntry[] }
type APIKeyResponse = { items: APIKey[] }
type WebhookResponse = { items: WebhookSubscription[]; catalog?: WebhookEvent[] }
type WebhookDelivery = { id: number; subscription_id: number; subscription_name: string; endpoint_url: string; event_key: string; event_type: string; status: string; attempts: number; max_attempts: number; next_attempt_at?: string; last_attempt_at?: string; response_status: number; last_error_code?: string; last_error_message?: string; delivered_at?: string; event_occurred_at?: string; created_at: string; updated_at: string }
type WebhookDeliveryResponse = { items: WebhookDelivery[] }
type IdentityChallengeItem = { field: string; label: string }

const emptyPolicy = (): IntegrationPolicy => ({ connector: '', risk: 'medium', requires_confirmation: false, required_scopes: [], allowed_hosts: [], timeout_seconds: 20, max_calls_per_minute: 30, max_calls_per_hour: 300, max_response_bytes: 1048576, max_argument_bytes: 32768, requires_identity: '', allowed_identities: [], identity_config: '', identity_verify_url: '', identity_verify_method: 'POST', identity_verify_param: '', identity_match_field: '', identity_client_id_field: '' })
const emptyDraft = (): IntegrationDraft => ({ name: '', description: '', method: 'POST', url: '', bearer_token: '', headers: '{}', parameters: '{\n  "type": "object",\n  "properties": {}\n}', connector: '', risk: 'medium', requires_confirmation: false, required_scopes: '', allowed_hosts: '', timeout_seconds: '20', max_calls_per_minute: '30', max_calls_per_hour: '300', max_response_bytes: '1048576', max_argument_bytes: '32768', requires_identity: '', allowed_identities: '', identity_config: '', identity_verify_url: '', identity_verify_method: 'POST', identity_verify_param: '', identity_match_field: '', identity_client_id_field: '' })
const emptyWebhookDraft = (): WebhookDraft => ({ name: '', endpoint_url: '', events: [], signing_secret: '', active: true })
const splitValues = (value: string) => value.split(/[\n,]/).map((item) => item.trim()).filter(Boolean)
const policyValue = (policy: IntegrationPolicy | undefined) => policy || emptyPolicy()
const riskLabel = (risk: string) => ({ low: 'Baixo', medium: 'Médio', high: 'Alto', critical: 'Crítico' }[risk] || risk)
const identityLabel = (value?: string) => ({ phone: 'Por telefone', challenge: 'Desafio (perguntas)', resolve: 'Resolver no sistema', otp: 'Código de verificação (OTP)' }[value || ''] || '')
const deliveryStatusLabel = (status: string) => ({ delivered: 'Entregue', retry_scheduled: 'Retentativa', pending: 'Pendente', processing: 'Processando', dead_letter: 'Falhou' }[status] || status)
const formatDeliveryTime = (value?: string) => { if (!value) return '—'; const date = new Date(value); return Number.isNaN(date.getTime()) ? value : date.toLocaleString('pt-BR') }

type IntegrationTab = 'tools' | 'mcp' | 'api' | 'webhooks'

const INTEGRATION_TABS: { key: IntegrationTab; label: string; short: string; title: string; intro: string }[] = [
  { key: 'tools', label: 'Tools para agentes', short: 'O que a IA pode chamar', title: 'Tools para agentes', intro: 'As ferramentas que a IA pode chamar para consultar dados e executar ações dentro das regras do agente.' },
  { key: 'mcp', label: 'MCP', short: 'Servidores de tools externas', title: 'Servidores MCP', intro: 'Conecte servidores MCP e exponha as tools deles para os seus agentes de IA.' },
  { key: 'api', label: 'REST API', short: 'Seu sistema envia e recebe', title: 'REST API', intro: 'Chaves e documentação para o seu sistema enviar e receber mensagens.' },
  { key: 'webhooks', label: 'Webhooks', short: 'Avisamos o seu sistema', title: 'Webhooks', intro: 'Assinaturas e log de entregas para avisarmos o seu sistema quando algo acontecer.' },
]

const tabMeta = (key: IntegrationTab) => INTEGRATION_TABS.find((item) => item.key === key) || INTEGRATION_TABS[0]

function availableIntegrationTabs(toolsEnabled: boolean, apiEnabled: boolean): IntegrationTab[] {
  const tabs: IntegrationTab[] = []
  if (toolsEnabled) tabs.push('tools', 'mcp')
  if (apiEnabled) tabs.push('api', 'webhooks')
  return tabs
}

function resolveIntegrationTab(value: string | null, toolsEnabled: boolean, apiEnabled: boolean): IntegrationTab {
  const tabs = availableIntegrationTabs(toolsEnabled, apiEnabled)
  if (value && tabs.includes(value as IntegrationTab)) return value as IntegrationTab
  return tabs[0] || 'tools'
}

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, { ...options, credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) } })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return payload.data as T
}

export function IntegrationsPage({ onBack, modules, permissions = null }: { onBack?: () => void; modules?: string[] | null; permissions?: string[] | null }) {
  const toolsEnabled = modules == null || modules.includes('tools') || modules.includes('integracoes')
  const apiEnabled = modules == null || modules.includes('api')
  const canManage = permissions == null || permissions.includes('integracoes_gerenciar')
  const hasEnabledArea = toolsEnabled || apiEnabled
  const [tab, setTab] = useState<IntegrationTab>(() => resolveIntegrationTab(new URLSearchParams(window.location.search).get('tab'), toolsEnabled, apiEnabled))
  const [integrations, setIntegrations] = useState<Integration[]>([])
  const [catalog, setCatalog] = useState<CatalogEntry[]>([])
  const [apiKeys, setAPIKeys] = useState<APIKey[]>([])
  const [webhooks, setWebhooks] = useState<WebhookSubscription[]>([])
  const [webhookEvents, setWebhookEvents] = useState<WebhookEvent[]>([])
  const [apiKeyName, setAPIKeyName] = useState('')
  const [newAPIKeySecret, setNewAPIKeySecret] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [apiKeyCreating, setAPIKeyCreating] = useState(false)
  const [webhookSaving, setWebhookSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [draft, setDraft] = useState<IntegrationDraft | null>(null)
  const [editingID, setEditingID] = useState<number | null>(null)
  const [webhookDraft, setWebhookDraft] = useState<WebhookDraft | null>(null)
  const [editingWebhookID, setEditingWebhookID] = useState<number | null>(null)
  const [showAPIDocs, setShowAPIDocs] = useState(false)

  const load = useCallback(async () => {
    if (!hasEnabledArea) { setIntegrations([]); setCatalog([]); setAPIKeys([]); setWebhooks([]); setWebhookEvents([]); setLoading(false); return }
    setLoading(true)
    const errors: string[] = []
    try {
      if (toolsEnabled) {
        try { const payload = await request<IntegrationResponse>('/api/react/integrations'); setIntegrations(payload.items || []); setCatalog(payload.catalog || []) }
        catch (loadError) { setIntegrations([]); setCatalog([]); errors.push(loadError instanceof Error ? loadError.message : 'Não foi possível carregar as tools.') }
      } else { setIntegrations([]); setCatalog([]) }
      if (apiEnabled) {
        try { const payload = await request<APIKeyResponse>('/api/react/api-keys'); setAPIKeys(payload.items || []) }
        catch (loadError) { setAPIKeys([]); errors.push(loadError instanceof Error ? loadError.message : 'Não foi possível carregar as chaves da API.') }
        try { const payload = await request<WebhookResponse>('/api/react/webhooks'); setWebhooks(payload.items || []); setWebhookEvents(payload.catalog || []) }
        catch (loadError) { setWebhooks([]); setWebhookEvents([]); errors.push(loadError instanceof Error ? loadError.message : 'Não foi possível carregar os webhooks.') }
      } else { setAPIKeys([]); setNewAPIKeySecret(null); setWebhooks([]); setWebhookEvents([]) }
      setError(errors[0] || null)
    } finally { setLoading(false) }
  }, [apiEnabled, hasEnabledArea, toolsEnabled])

  useEffect(() => { void load() }, [load])
  useEffect(() => { if (!toolsEnabled) { setDraft(null); setEditingID(null) }; if (!apiEnabled) { setNewAPIKeySecret(null); setWebhookDraft(null); setEditingWebhookID(null) } }, [apiEnabled, toolsEnabled])
  useEffect(() => { setTab((current) => resolveIntegrationTab(current, toolsEnabled, apiEnabled)) }, [toolsEnabled, apiEnabled])
  useEffect(() => { const params = new URLSearchParams(window.location.search); if (tab === 'tools') params.delete('tab'); else params.set('tab', tab); const query = params.toString(); window.history.replaceState(null, '', `${window.location.pathname}${query ? `?${query}` : ''}`) }, [tab])

  const openCreate = (template?: CatalogEntry) => {
    if (!toolsEnabled) return
    const next = emptyDraft()
    if (template) Object.assign(next, { description: template.description, method: template.method, connector: template.key, risk: template.risk, requires_confirmation: template.requires_confirmation, required_scopes: template.required_scopes.join('\n'), timeout_seconds: String(template.timeout_seconds), max_calls_per_minute: String(template.max_calls_per_minute), max_calls_per_hour: String(template.max_calls_per_hour), parameters: template.parameters || next.parameters, requires_identity: template.requires_identity || '' })
    setEditingID(null); setDraft(next); setError(null); setNotice(null)
  }
  const openEdit = async (integration: Integration) => {
    if (!toolsEnabled) return
    setEditingID(integration.id); setDraft(emptyDraft()); setError(null)
    try {
      const detail = await request<IntegrationDetail>(`/api/react/integrations/${integration.id}`)
      const policy = policyValue(detail.policy)
      setDraft({ name: detail.name, description: detail.description, method: detail.method, url: detail.url, bearer_token: '', headers: JSON.stringify(detail.headers || {}, null, 2), parameters: detail.parameters || emptyDraft().parameters, connector: policy.connector, risk: policy.risk, requires_confirmation: policy.requires_confirmation, required_scopes: policy.required_scopes.join('\n'), allowed_hosts: policy.allowed_hosts.join('\n'), timeout_seconds: String(policy.timeout_seconds), max_calls_per_minute: String(policy.max_calls_per_minute), max_calls_per_hour: String(policy.max_calls_per_hour), max_response_bytes: String(policy.max_response_bytes), max_argument_bytes: String(policy.max_argument_bytes), requires_identity: policy.requires_identity, allowed_identities: policy.allowed_identities.join('\n'), identity_config: policy.identity_config || '', identity_verify_url: policy.identity_verify_url || '', identity_verify_method: policy.identity_verify_method || 'POST', identity_verify_param: policy.identity_verify_param || '', identity_match_field: policy.identity_match_field || '', identity_client_id_field: policy.identity_client_id_field || '' })
    } catch (loadError) { setDraft(null); setEditingID(null); setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar a tool.') }
  }
  const closeEditor = () => { if (!saving) { setDraft(null); setEditingID(null) } }
  const updateDraft = <K extends keyof IntegrationDraft>(key: K, value: IntegrationDraft[K]) => setDraft((current) => current ? { ...current, [key]: value } : current)
  const save = async () => {
    if (!toolsEnabled || !draft || !draft.name.trim() || !draft.url.trim()) { setError('Informe o nome e a URL da tool.'); return }
    let headers: Record<string, string> = {}
    try {
      const parsed = JSON.parse(draft.headers || '{}') as unknown
      if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error()
      for (const [key, value] of Object.entries(parsed as Record<string, unknown>)) { if (typeof value !== 'string') throw new Error(); headers[key] = value }
      const parameters = JSON.parse(draft.parameters || '{}') as unknown
      if (!parameters || Array.isArray(parameters) || typeof parameters !== 'object') throw new Error()
    } catch { setError('Headers e parâmetros precisam ser objetos JSON válidos.'); return }
    if (draft.requires_identity === 'challenge') {
      try {
        const config = JSON.parse(draft.identity_config || '{}') as { challenges?: unknown[] }
        if (!config || Array.isArray(config) || !Array.isArray(config.challenges) || config.challenges.length === 0) throw new Error()
      } catch { setError('O modo desafio exige ao menos uma pergunta em identity_config (JSON com challenges).'); return }
    } else if (draft.identity_config.trim()) {
      try { const config = JSON.parse(draft.identity_config); if (!config || Array.isArray(config) || typeof config !== 'object') throw new Error() } catch { setError('identity_config precisa ser um JSON objeto válido.'); return }
    }
    if (draft.identity_verify_url.trim()) {
      try { const parsed = new URL(draft.identity_verify_url); if (parsed.protocol !== 'https:') throw new Error() } catch { setError('identity_verify_url deve ser uma URL HTTPS válida.'); return }
    }
    const numberValue = (value: string) => Number.parseInt(value, 10)
    const policy = { connector: draft.connector.trim(), risk: draft.risk, requires_confirmation: draft.requires_confirmation, required_scopes: splitValues(draft.required_scopes), allowed_hosts: splitValues(draft.allowed_hosts), timeout_seconds: numberValue(draft.timeout_seconds), max_calls_per_minute: numberValue(draft.max_calls_per_minute), max_calls_per_hour: numberValue(draft.max_calls_per_hour), max_response_bytes: numberValue(draft.max_response_bytes), max_argument_bytes: numberValue(draft.max_argument_bytes), requires_identity: draft.requires_identity, allowed_identities: splitValues(draft.allowed_identities), identity_config: draft.identity_config.trim(), identity_verify_url: draft.identity_verify_url.trim(), identity_verify_method: draft.identity_verify_method, identity_verify_param: draft.identity_verify_param.trim(), identity_match_field: draft.identity_match_field.trim(), identity_client_id_field: draft.identity_client_id_field.trim() }
    setSaving(true)
    try {
      const url = editingID === null ? '/api/react/integrations' : `/api/react/integrations/${editingID}`
      await request<IntegrationDetail>(url, { method: editingID === null ? 'POST' : 'PATCH', body: JSON.stringify({ name: draft.name.trim(), description: draft.description.trim(), method: draft.method, url: draft.url.trim(), bearer_token: draft.bearer_token, headers, parameters: draft.parameters.trim(), policy }) })
      closeEditor(); setNotice('Tool salva com política aplicada.'); await load()
    } catch (saveError) { setError(saveError instanceof Error ? saveError.message : 'Não foi possível salvar a tool.') }
    finally { setSaving(false) }
  }
  const remove = async (integration: Integration) => {
    if (!toolsEnabled || !window.confirm(`Excluir a tool “${integration.name}”? Agentes que a utilizam deixarão de chamá-la.`)) return
    try { await request(`/api/react/integrations/${integration.id}`, { method: 'DELETE' }); setNotice('Tool excluída.'); await load() }
    catch (removeError) { setError(removeError instanceof Error ? removeError.message : 'Não foi possível excluir a tool.') }
  }
  const createAPIKey = async () => {
    if (!apiEnabled || !apiKeyName.trim()) { setError('Informe um nome para a chave da API.'); return }
    setAPIKeyCreating(true)
    try {
      const result = await request<{ api_key: APIKey; secret: string }>('/api/react/api-keys', { method: 'POST', body: JSON.stringify({ name: apiKeyName.trim(), scopes: ['conversations:read', 'conversations:write'] }) })
      setAPIKeys((current) => [result.api_key, ...current]); setNewAPIKeySecret(result.secret); setAPIKeyName(''); setNotice('Chave criada. Copie o segredo agora; ele não será exibido novamente.')
    } catch (createError) { setError(createError instanceof Error ? createError.message : 'Não foi possível criar a chave.') }
    finally { setAPIKeyCreating(false) }
  }
  const toggleAPIKey = async (key: APIKey) => {
    if (!apiEnabled) return
    try { await request(`/api/react/api-keys/${key.id}/toggle`, { method: 'POST', body: JSON.stringify({ active: !key.active }) }); setAPIKeys((current) => current.map((item) => item.id === key.id ? { ...item, active: !item.active } : item)) }
    catch (toggleError) { setError(toggleError instanceof Error ? toggleError.message : 'Não foi possível atualizar a chave.') }
  }
  const deleteAPIKey = async (key: APIKey) => {
    if (!apiEnabled || !window.confirm(`Revogar a chave “${key.name}”?`)) return
    try { await request(`/api/react/api-keys/${key.id}`, { method: 'DELETE' }); setAPIKeys((current) => current.filter((item) => item.id !== key.id)); setNotice('Chave revogada.') }
    catch (deleteError) { setError(deleteError instanceof Error ? deleteError.message : 'Não foi possível revogar a chave.') }
  }
  const openWebhookCreate = () => { if (apiEnabled) { setEditingWebhookID(null); setWebhookDraft(emptyWebhookDraft()); setError(null); setNotice(null) } }
  const openWebhookEdit = (subscription: WebhookSubscription) => {
    if (!apiEnabled) return
    setEditingWebhookID(subscription.id)
    setWebhookDraft({ name: subscription.name, endpoint_url: subscription.endpoint_url, events: [...subscription.events], signing_secret: '', active: subscription.active })
    setError(null); setNotice(null)
  }
  const saveWebhook = async () => {
    if (!apiEnabled || !webhookDraft) return
    if (!webhookDraft.name.trim() || !webhookDraft.endpoint_url.trim() || webhookDraft.events.length === 0) { setError('Informe nome, endpoint HTTPS e ao menos um evento.'); return }
    setWebhookSaving(true)
    try {
      const url = editingWebhookID === null ? '/api/react/webhooks' : `/api/react/webhooks/${editingWebhookID}`
      const saved = await request<WebhookSubscription>(url, { method: editingWebhookID === null ? 'POST' : 'PATCH', body: JSON.stringify({ name: webhookDraft.name.trim(), endpoint_url: webhookDraft.endpoint_url.trim(), events: webhookDraft.events, signing_secret: webhookDraft.signing_secret, active: webhookDraft.active }) })
      setWebhooks((current) => editingWebhookID === null ? [saved, ...current] : current.map((item) => item.id === saved.id ? saved : item))
      setWebhookDraft(null); setEditingWebhookID(null); setNotice(editingWebhookID === null ? 'Webhook criado.' : 'Webhook atualizado.')
    } catch (saveError) { setError(saveError instanceof Error ? saveError.message : 'Não foi possível salvar o webhook.') }
    finally { setWebhookSaving(false) }
  }
  const toggleWebhook = async (subscription: WebhookSubscription) => {
    if (!apiEnabled) return
    try { await request(`/api/react/webhooks/${subscription.id}/toggle`, { method: 'POST', body: JSON.stringify({ active: !subscription.active }) }); setWebhooks((current) => current.map((item) => item.id === subscription.id ? { ...item, active: !item.active } : item)) }
    catch (toggleError) { setError(toggleError instanceof Error ? toggleError.message : 'Não foi possível atualizar o webhook.') }
  }
  const deleteWebhook = async (subscription: WebhookSubscription) => {
    if (!apiEnabled || !window.confirm(`Excluir o webhook “${subscription.name}”?`)) return
    try { await request(`/api/react/webhooks/${subscription.id}`, { method: 'DELETE' }); setWebhooks((current) => current.filter((item) => item.id !== subscription.id)); setNotice('Webhook excluído.') }
    catch (deleteError) { setError(deleteError instanceof Error ? deleteError.message : 'Não foi possível excluir o webhook.') }
  }

  if (!hasEnabledArea) return <div className="integrations-page"><section className="integrations-intro"><div><span className="eyebrow">Automação</span><h2>Recursos sob demanda</h2><p>API, webhooks e tools são liberados pela plataforma conforme o plano contratado pela empresa.</p></div>{onBack && <div className="integrations-intro-actions"><button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button></div>}</section><div className="integrations-module-unavailable"><span>◌</span><div><strong>Nenhum recurso de automação está habilitado</strong><p>Fale com o administrador da plataforma para liberar o módulo necessário para sua operação.</p></div></div></div>

  const activeTab = tabMeta(tab)
  const tabs = availableIntegrationTabs(toolsEnabled, apiEnabled)

  return <div className="integrations-page">
    <section className="integrations-intro">
      <div>
        <span className="eyebrow">Automação</span>
        <h2>{activeTab.title}</h2>
        <p>{activeTab.intro}</p>
      </div>
      <div className="integrations-intro-actions">
        {onBack && <button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button>}
        {tab === 'tools' && toolsEnabled && canManage && <button className="primary-button" type="button" onClick={() => openCreate()}>+ Nova tool</button>}
        {tab === 'api' && apiEnabled && <button className="secondary-button" type="button" onClick={() => setShowAPIDocs((value) => !value)}>{showAPIDocs ? '← Voltar às chaves' : 'Documentação da API'}</button>}
      </div>
    </section>

    {!canManage && <div className="integrations-tip"><span className="integrations-tip-icon">◌</span><div><strong>Acesso somente para consulta</strong><p>Você não tem a permissão de integrações; as ações de criar, editar, ativar ou excluir ficam indisponíveis.</p></div></div>}

    <nav className="integrations-tabs" aria-label="Seções de integração">
      {tabs.map((key) => { const item = tabMeta(key); const active = key === tab; return <button key={key} className={`integrations-tab ${active ? 'is-active' : ''}`} type="button" aria-current={active ? 'page' : undefined} onClick={() => setTab(key)}><strong>{item.label}</strong><span>{item.short}</span></button> })}
    </nav>

    {error && <div className="integrations-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
    {notice && <div className="integrations-notice" role="status"><span>✓</span>{notice}<button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso">×</button></div>}

    {tab === 'tools' && toolsEnabled && <ToolsSection loading={loading} integrations={integrations} catalog={catalog} canManage={canManage} onCreate={openCreate} onEdit={openEdit} onRemove={remove} />}
    {tab === 'mcp' && toolsEnabled && <McpPanel canManage={canManage} />}
    {tab === 'api' && apiEnabled && (showAPIDocs ? <ApiDocsPanel onBack={() => setShowAPIDocs(false)} /> : <APISection apiKeys={apiKeys} apiKeyName={apiKeyName} apiKeyCreating={apiKeyCreating} newAPIKeySecret={newAPIKeySecret} canManage={canManage} onNameChange={setAPIKeyName} onCreate={createAPIKey} onCopy={() => { void navigator.clipboard?.writeText(newAPIKeySecret || ''); setNotice('Segredo copiado.') }} onCloseSecret={() => setNewAPIKeySecret(null)} onToggle={toggleAPIKey} onDelete={deleteAPIKey} />)}
    {tab === 'webhooks' && apiEnabled && <WebhookSection subscriptions={webhooks} catalog={webhookEvents} draft={webhookDraft} editing={editingWebhookID !== null} saving={webhookSaving} canManage={canManage} onCreate={openWebhookCreate} onEdit={openWebhookEdit} onChange={setWebhookDraft} onSave={() => void saveWebhook()} onCancel={() => { if (!webhookSaving) { setWebhookDraft(null); setEditingWebhookID(null) } }} onToggle={toggleWebhook} onDelete={deleteWebhook} />}

    {draft && toolsEnabled && <IntegrationEditor draft={draft} catalog={catalog} editing={editingID !== null} saving={saving} onChange={updateDraft} onSave={() => void save()} onClose={closeEditor} />}
  </div>
}

function ToolsSection({ loading, integrations, catalog, canManage, onCreate, onEdit, onRemove }: { loading: boolean; integrations: Integration[]; catalog: CatalogEntry[]; canManage: boolean; onCreate: (template?: CatalogEntry) => void; onEdit: (integration: Integration) => void; onRemove: (integration: Integration) => void }) {
  return <><div className="integrations-tip"><span className="integrations-tip-icon">✦</span><div><strong>Explique para a IA quando cada tool deve ser usada.</strong><p>Escopos limitam o que o agente pode fazer. Ações críticas pedem confirmação e cada tentativa fica registrada sem expor credenciais.</p></div></div>{catalog.length > 0 && <section className="integrations-catalog"><div className="integrations-section-heading"><div><span className="eyebrow">Catálogo de conectores</span><h3>Comece por uma política pronta</h3></div><span>{catalog.length} modelos seguros</span></div><div className="integrations-catalog-grid">{catalog.map((entry) => <article className="integration-template" key={entry.key}><div className="integration-template-top"><span className={`risk-badge risk-${entry.risk}`}>{riskLabel(entry.risk)}</span><span>{entry.method}</span></div><h4>{entry.name}</h4><p>{entry.description}</p><div className="integration-template-meta"><span>{entry.required_scopes.join(' · ') || 'Sem escopo'}</span><span>{entry.max_calls_per_minute}/min</span></div>{entry.requires_identity && <div className="integration-template-hint">🔒 Identificação: {identityLabel(entry.requires_identity)}</div>}{canManage && <button type="button" onClick={() => onCreate(entry)}>Usar modelo ↗</button>}</article>)}</div></section>}{loading ? <div className="integrations-state"><span className="integrations-spinner" /> Carregando tools…</div> : integrations.length === 0 ? <div className="integrations-empty"><div className="integrations-empty-icon">⛓</div><strong>Nenhuma tool configurada</strong><p>Comece conectando um endpoint seguro da sua operação.</p>{canManage && <button className="primary-button" type="button" onClick={() => onCreate()}>Adicionar tool</button>}</div> : <div className="integrations-grid">{integrations.map((integration) => { const policy = policyValue(integration.policy); return <article className="integration-card" key={integration.id}><header><div className="integration-avatar">⛓</div><div><span className="integration-kicker">Tool da operação</span><h3>{integration.name}</h3></div><span className={`risk-badge risk-${policy.risk}`}>{riskLabel(policy.risk)}</span><span className="integration-method">{integration.method}</span></header><p className="integration-description">{integration.description || 'Sem descrição. Edite para explicar quando a IA deve usar esta tool.'}</p><div className="integration-meta"><span><i />{integration.has_credentials ? 'Credencial protegida' : 'Sem credencial'}</span><span>{policy.requires_confirmation ? 'Confirmação exigida' : 'Execução automática'}</span><span>{policy.required_scopes.length ? `${policy.required_scopes.length} escopo(s)` : 'Sem escopos'}</span><span>{policy.max_calls_per_minute}/min · {policy.max_calls_per_hour}/hora</span></div>{canManage && <footer><button type="button" onClick={() => void onEdit(integration)}>Editar política</button><button className="is-danger" type="button" onClick={() => void onRemove(integration)}>Excluir</button></footer>}</article> })}</div>}</>
}

function APISection({ apiKeys, apiKeyName, apiKeyCreating, newAPIKeySecret, canManage, onNameChange, onCreate, onCopy, onCloseSecret, onToggle, onDelete }: { apiKeys: APIKey[]; apiKeyName: string; apiKeyCreating: boolean; newAPIKeySecret: string | null; canManage: boolean; onNameChange: (value: string) => void; onCreate: () => void; onCopy: () => void; onCloseSecret: () => void; onToggle: (key: APIKey) => void; onDelete: (key: APIKey) => void }) {
  return <section className="integrations-access"><div className="integrations-section-heading"><div><span className="eyebrow">Integrações externas</span><h3>API pública</h3></div><span>Bearer · escopos · idempotência</span></div><p className="integrations-access-copy">Crie uma chave para seu ERP, site ou automação iniciar conversas e enviar mensagens. O segredo aparece apenas uma vez.</p>{canManage && <div className="integrations-access-create"><input value={apiKeyName} onChange={(event) => onNameChange(event.target.value)} placeholder="Ex.: ERP produção" aria-label="Nome da nova chave" /><button className="primary-button" type="button" disabled={apiKeyCreating} onClick={onCreate}>{apiKeyCreating ? 'Criando…' : 'Criar chave'}</button></div>}{newAPIKeySecret && <div className="integrations-secret" role="status"><strong>Copie este segredo agora</strong><code>{newAPIKeySecret}</code><button type="button" onClick={onCopy}>Copiar segredo</button><button type="button" aria-label="Fechar segredo" onClick={onCloseSecret}>×</button></div>}{apiKeys.length > 0 && <div className="integrations-key-list">{apiKeys.map((key) => <div className="integrations-key-row" key={key.id}><span className={`integrations-key-status ${key.active ? 'active' : ''}`} /><div><strong>{key.name}</strong><small>{key.prefix}… · {key.scopes.join(', ')}</small></div><span className={`integrations-key-state ${key.active ? 'is-on' : 'is-off'}`}>{key.active ? 'Ativa' : 'Revogada'}</span>{canManage && <><button type="button" onClick={() => onToggle(key)}>{key.active ? 'Desativar' : 'Ativar'}</button><button type="button" className="is-danger" onClick={() => onDelete(key)}>Revogar</button></>}</div>)}</div>}</section>
}

function WebhookSection({ subscriptions, catalog, draft, editing, saving, canManage, onCreate, onEdit, onChange, onSave, onCancel, onToggle, onDelete }: { subscriptions: WebhookSubscription[]; catalog: WebhookEvent[]; draft: WebhookDraft | null; editing: boolean; saving: boolean; canManage: boolean; onCreate: () => void; onEdit: (subscription: WebhookSubscription) => void; onChange: (draft: WebhookDraft | null) => void; onSave: () => void; onCancel: () => void; onToggle: (subscription: WebhookSubscription) => void; onDelete: (subscription: WebhookSubscription) => void }) {
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([])
  const [deliveriesOpen, setDeliveriesOpen] = useState(false)
  const [deliveriesSubscription, setDeliveriesSubscription] = useState<number | null>(null)
  const [deliveriesStatus, setDeliveriesStatus] = useState('')
  const [deliveriesLoading, setDeliveriesLoading] = useState(false)
  const [deliveriesError, setDeliveriesError] = useState<string | null>(null)
  const loadDeliveries = useCallback(async (subscription: number | null, status: string) => {
    setDeliveriesLoading(true); setDeliveriesError(null)
    try {
      const params = new URLSearchParams({ limit: '100' })
      if (subscription != null) params.set('subscription_id', String(subscription))
      if (status) params.set('status', status)
      const payload = await request<WebhookDeliveryResponse>(`/api/react/webhooks/deliveries?${params.toString()}`)
      setDeliveries(payload.items || [])
    } catch (loadError) {
      setDeliveries([]); setDeliveriesError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar as entregas.')
    } finally { setDeliveriesLoading(false) }
  }, [])
  const openDeliveries = (subscription: number | null) => {
    setDeliveriesSubscription(subscription); setDeliveriesStatus(''); setDeliveriesOpen(true); void loadDeliveries(subscription, '')
  }
  const closeDeliveries = () => { setDeliveriesOpen(false); setDeliveries([]); setDeliveriesError(null) }
  const changeDeliveriesStatus = (status: string) => { setDeliveriesStatus(status); void loadDeliveries(deliveriesSubscription, status) }
  const toggleEvent = (event: string) => {
    if (!draft) return
    const events = draft.events.includes(event) ? draft.events.filter((item) => item !== event) : [...draft.events, event]
    onChange({ ...draft, events })
  }
  return <section className="integrations-access integrations-webhooks"><div className="integrations-section-heading"><div><span className="eyebrow">Eventos de saída</span><h3>Webhooks</h3></div><div className="integrations-webhook-head-actions"><button className="secondary-button" type="button" onClick={() => openDeliveries(null)}>Ver entregas</button>{canManage && <button className="secondary-button" type="button" onClick={onCreate}>+ Novo webhook</button>}</div></div><p className="integrations-access-copy">Uma assinatura define quais eventos futuros podem ser entregues ao seu endpoint. Salvar não envia teste nem reprocessa eventos antigos; a entrega acontece de forma assíncrona quando o processamento de eventos está ativo.</p>{subscriptions.length === 0 && !draft && <div className="integrations-webhook-empty">Nenhuma assinatura criada. Use um endpoint HTTPS público para receber eventos.</div>}{subscriptions.length > 0 && <div className="integrations-key-list">{subscriptions.map((subscription) => <div className="integrations-key-row integrations-webhook-row" key={subscription.id}><span className={`integrations-key-status ${subscription.active ? 'active' : ''}`} /><div><strong>{subscription.name}</strong><small>{subscription.endpoint_url}</small><small>{subscription.events.join(' · ')}</small></div><span className={`integrations-key-state ${subscription.active ? 'is-on' : 'is-off'}`}>{subscription.active ? 'Ativo' : 'Pausado'}</span>{subscription.has_signing_secret && <span className="integrations-key-state is-info">Assinado</span>}<button type="button" onClick={() => openDeliveries(subscription.id)}>Ver entregas</button>{canManage && <><button type="button" onClick={() => onEdit(subscription)}>Editar</button><button type="button" onClick={() => onToggle(subscription)}>{subscription.active ? 'Pausar' : 'Ativar'}</button><button type="button" className="is-danger" onClick={() => onDelete(subscription)}>Excluir</button></>}</div>)}</div>}{draft && <form className="integrations-webhook-form" onSubmit={(event) => { event.preventDefault(); onSave() }}><div className="integrations-webhook-fields"><label><span>Nome *</span><input autoFocus value={draft.name} onChange={(event) => onChange({ ...draft, name: event.target.value })} placeholder="Ex.: ERP produção" /></label><label><span>Endpoint HTTPS *</span><input type="url" inputMode="url" value={draft.endpoint_url} onChange={(event) => onChange({ ...draft, endpoint_url: event.target.value })} placeholder="https://erp.exemplo.com/webhooks/notreve" /></label></div><fieldset><legend>Eventos *</legend><div className="integrations-event-grid">{catalog.map((event) => <label key={event.key} title={event.description}><input type="checkbox" checked={draft.events.includes(event.key)} onChange={() => toggleEvent(event.key)} /><span><strong>{event.name}</strong><small>{event.key}</small></span></label>)}</div></fieldset><label className="integrations-webhook-secret"><span>Segredo de assinatura <small>· opcional</small></span><input type="password" autoComplete="new-password" value={draft.signing_secret} onChange={(event) => onChange({ ...draft, signing_secret: event.target.value })} placeholder={editing ? 'Vazio mantém o segredo atual' : 'Mínimo de 16 caracteres'} /><small>{editing ? 'O segredo atual é protegido e nunca pode ser recuperado; informe outro valor para rotacioná-lo.' : 'Se definido, será protegido no servidor e nunca poderá ser consultado depois.'}</small></label><label className="integrations-webhook-active"><input type="checkbox" checked={draft.active} onChange={(event) => onChange({ ...draft, active: event.target.checked })} /><span>Assinatura ativa após salvar</span></label><div className="integrations-webhook-actions"><button className="secondary-button" type="button" disabled={saving} onClick={onCancel}>Cancelar</button><button className="primary-button" type="submit" disabled={saving}>{saving ? 'Salvando…' : editing ? 'Salvar alterações' : 'Criar webhook'}</button></div></form>}{deliveriesOpen && <div className="integrations-modal-backdrop deliveries-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) closeDeliveries() }}><section className="integrations-modal deliveries-modal" role="dialog" aria-modal="true" aria-labelledby="deliveries-title"><header><div><span className="eyebrow">Eventos de saída</span><h2 id="deliveries-title">Entregas de webhook</h2></div><div className="deliveries-head-actions"><select value={deliveriesStatus} onChange={(event) => changeDeliveriesStatus(event.target.value)} aria-label="Filtrar por status"><option value="">Todos os status</option><option value="delivered">Entregue</option><option value="retry_scheduled">Retentativa</option><option value="pending">Pendente</option><option value="processing">Processando</option><option value="dead_letter">Falhou</option></select><button type="button" className="integrations-modal-close" onClick={closeDeliveries} aria-label="Fechar">×</button></div></header><div className="deliveries-body">{deliveriesLoading ? <div className="deliveries-state"><span className="integrations-spinner" /> Carregando entregas…</div> : deliveriesError ? <div className="delivery-error" role="alert">{deliveriesError}</div> : deliveries.length === 0 ? <div className="deliveries-empty">Nenhuma entrega registrada ainda.</div> : <div className="deliveries-table-wrap"><table className="deliveries-table"><thead><tr><th>Evento</th><th>Status</th><th>Tentativas</th><th>Próxima / Entregue</th><th>Código do erro</th><th>Mensagem de erro</th><th>Endpoint</th><th>Data</th></tr></thead><tbody>{deliveries.map((delivery) => <tr key={delivery.id}><td className="deliveries-cell-event"><strong>{delivery.event_type}</strong><small>{delivery.subscription_name}</small></td><td><span className={`delivery-badge delivery-badge--${delivery.status}`}>{deliveryStatusLabel(delivery.status)}</span></td><td>{delivery.attempts}/{delivery.max_attempts}</td><td>{formatDeliveryTime(delivery.delivered_at || delivery.next_attempt_at)}</td><td>{delivery.last_error_code || '—'}</td><td>{delivery.last_error_message ? <span className="deliveries-error-msg" title={delivery.last_error_message} aria-label={delivery.last_error_message}>{delivery.last_error_message}</span> : '—'}</td><td className="deliveries-cell-endpoint">{delivery.endpoint_url}</td><td>{formatDeliveryTime(delivery.created_at)}</td></tr>)}</tbody></table></div>}</div></section></div>}</section>
}

function IntegrationEditor({ draft, catalog, editing, saving, onChange, onSave, onClose }: { draft: IntegrationDraft; catalog: CatalogEntry[]; editing: boolean; saving: boolean; onChange: <K extends keyof IntegrationDraft>(key: K, value: IntegrationDraft[K]) => void; onSave: () => void; onClose: () => void }) {
  const parseChallenges = (json: string): IdentityChallengeItem[] => {
    try {
      const config = JSON.parse(json || '{}') as { challenges?: IdentityChallengeItem[] }
      return Array.isArray(config.challenges) ? config.challenges.filter((item) => item && typeof item.field === 'string') : []
    } catch { return [] }
  }
  const challenges = parseChallenges(draft.identity_config)
  const challengesToJSON = (items: IdentityChallengeItem[]) => JSON.stringify({ challenges: items })
  const updateChallenge = (index: number, field: string, label: string) => onChange('identity_config', challengesToJSON(challenges.map((item, i) => (i === index ? { ...item, field, label } : item))))
  const addChallenge = () => onChange('identity_config', challengesToJSON([...challenges, { field: '', label: '' }]))
  const removeChallenge = (index: number) => onChange('identity_config', challengesToJSON(challenges.filter((_, i) => i !== index)))
  const identityLocked = draft.requires_identity !== ''
  return <Modal open title={editing ? 'Ajustar tool' : 'Conectar uma tool'} onClose={onClose} size="lg" footer={<><button className="secondary-button" type="button" disabled={saving} onClick={onClose}>Cancelar</button><button className="primary-button" type="button" disabled={saving} onClick={onSave}>{saving ? 'Salvando…' : 'Salvar tool'} ↗</button></>}><div className="integrations-form"><label><span>Nome da tool *</span><input autoFocus value={draft.name} onChange={(event) => onChange('name', event.target.value)} placeholder="consultar_saldo" /></label><label><span>Modelo de conector</span><select value={draft.connector} onChange={(event) => onChange('connector', event.target.value)}><option value="">Personalizado</option>{catalog.map((entry) => <option value={entry.key} key={entry.key}>{entry.name}</option>)}</select></label><label><span>Método</span><select value={draft.method} onChange={(event) => onChange('method', event.target.value)}><option>GET</option><option>POST</option><option>PUT</option><option>PATCH</option><option>DELETE</option></select></label><label><span>Nível de risco</span><select value={draft.risk} onChange={(event) => onChange('risk', event.target.value)}><option value="low">Baixo · consulta</option><option value="medium">Médio · operação controlada</option><option value="high">Alto · alteração externa</option><option value="critical">Crítico · financeiro ou irreversível</option></select></label><label className="integrations-form-wide"><span>Quando a IA deve usar?</span><input value={draft.description} onChange={(event) => onChange('description', event.target.value)} placeholder="Consulta o saldo atualizado do cliente" /></label><label className="integrations-form-wide"><span>URL do endpoint *</span><input value={draft.url} onChange={(event) => onChange('url', event.target.value)} placeholder="https://api.suaempresa.com/saldo" /></label><label className="integrations-form-wide"><span>Token Bearer {editing && <small>· vazio mantém o token atual</small>}</span><input type="password" value={draft.bearer_token} onChange={(event) => onChange('bearer_token', event.target.value)} placeholder={editing ? 'Credencial protegida' : 'Opcional'} autoComplete="new-password" /></label><label><span>Escopos exigidos <small>· um por linha</small></span><textarea rows={3} value={draft.required_scopes} onChange={(event) => onChange('required_scopes', event.target.value)} placeholder="data.read" /></label><label><span>Hosts autorizados <small>· vazio usa a validação padrão</small></span><textarea rows={3} value={draft.allowed_hosts} onChange={(event) => onChange('allowed_hosts', event.target.value)} placeholder="api.suaempresa.com" /></label><label><span>Timeout (segundos)</span><input type="number" min="1" max="60" value={draft.timeout_seconds} onChange={(event) => onChange('timeout_seconds', event.target.value)} /></label><label><span>Limite por minuto</span><input type="number" min="1" max="1000" value={draft.max_calls_per_minute} onChange={(event) => onChange('max_calls_per_minute', event.target.value)} /></label><label><span>Limite por hora</span><input type="number" min="1" max="10000" value={draft.max_calls_per_hour} onChange={(event) => onChange('max_calls_per_hour', event.target.value)} /></label><label><span>Resposta máxima (bytes)</span><input type="number" min="1024" max="4194304" value={draft.max_response_bytes} onChange={(event) => onChange('max_response_bytes', event.target.value)} /></label><label><span>Argumentos máximos (bytes)</span><input type="number" min="128" max="131072" value={draft.max_argument_bytes} onChange={(event) => onChange('max_argument_bytes', event.target.value)} /></label><label className="integrations-confirmation"><input type="checkbox" checked={draft.requires_confirmation} onChange={(event) => onChange('requires_confirmation', event.target.checked)} /><span><strong>Exigir confirmação explícita</strong><small>A IA não executará a ação até receber a confirmação correspondente.</small></span></label><label><span>Exigir identificação do cliente</span><select value={draft.requires_identity} onChange={(event) => onChange('requires_identity', event.target.value)}><option value="">Não exigir</option><option value="phone">Por telefone (só o número permitido)</option><option value="challenge">Desafio (perguntas de segurança)</option><option value="resolve">Resolver no sistema externo</option><option value="otp">Código de verificação (OTP/2FA)</option></select></label>{draft.requires_identity === 'phone' && <label><span>Telefones permitidos <small>· um por linha</small></span><textarea rows={3} value={draft.allowed_identities} onChange={(event) => onChange('allowed_identities', event.target.value)} placeholder="5511999999999" /></label>}{draft.requires_identity === 'challenge' && <div className="integrations-challenges"><span>Perguntas do desafio <small>· a IA pede essas respostas ao cliente</small></span>{challenges.map((item, index) => <div className="integrations-challenge-row" key={index}><input value={item.field} onChange={(event) => updateChallenge(index, event.target.value, item.label)} placeholder="campo (ex.: birthdate)" /><input value={item.label} onChange={(event) => updateChallenge(index, item.field, event.target.value)} placeholder="pergunta ao cliente (ex.: Data de nascimento)" /><button type="button" onClick={() => removeChallenge(index)} aria-label="Remover pergunta">×</button></div>)}<button type="button" onClick={addChallenge}>+ Adicionar pergunta</button></div>}{identityLocked && <><label className="integrations-form-wide"><span>URL do verificador de identidade (HTTPS)</span><input value={draft.identity_verify_url} onChange={(event) => onChange('identity_verify_url', event.target.value)} placeholder="https://api.suaempresa.com/identidade/verificar" /></label><label><span>Método do verificador</span><select value={draft.identity_verify_method} onChange={(event) => onChange('identity_verify_method', event.target.value)}><option>POST</option><option>GET</option><option>PUT</option><option>PATCH</option><option>DELETE</option></select></label><label><span>Campo do identificador</span><input value={draft.identity_verify_param} onChange={(event) => onChange('identity_verify_param', event.target.value)} placeholder="ex.: cpf" /></label><label><span>Campo de confirmação (match)</span><input value={draft.identity_match_field} onChange={(event) => onChange('identity_match_field', event.target.value)} placeholder="ex.: data.client.cpf" /></label><label><span>Campo do ID do cliente</span><input value={draft.identity_client_id_field} onChange={(event) => onChange('identity_client_id_field', event.target.value)} placeholder="ex.: data.client.id" /></label></>}<label><span>Headers não sensíveis (JSON)</span><textarea rows={5} value={draft.headers} onChange={(event) => onChange('headers', event.target.value)} spellCheck={false} /></label><label><span>Parâmetros da função (JSON)</span><textarea rows={5} value={draft.parameters} onChange={(event) => onChange('parameters', event.target.value)} spellCheck={false} /></label></div></Modal>
}

export default IntegrationsPage
