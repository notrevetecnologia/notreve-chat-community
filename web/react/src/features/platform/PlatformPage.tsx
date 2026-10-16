import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import './Platform.css'
import './PlatformOnboarding.css'

type Branding = { name: string; logo_url: string; favicon_url: string; primary_color: string; secondary_color: string; accent_color: string; background_color: string; loading_color: string }
type Limits = {
  max_canais: number
  max_atendentes: number
  max_campanhas_ativas: number
  campanha_limite_diario: number
  max_agentes: number
  max_usuarios: number
  max_conexoes_whatsapp: number
  max_tools: number
  max_api_keys: number
  max_webhooks: number
  creditos_ia_mensais: number
}
type Tenant = { id: number; name: string; slug: string; plan: string; status: string; timezone: string; dominio: string; creditos: number; limits?: Partial<Limits>; modules?: string[] }
type ContextSwitchAudit = { id: number; actor_id: number; actor_name: string; from_tenant_id: number; from_slug: string; to_tenant_id: number; to_slug: string; reason: string; created_at: string }
type WhatsAppServer = { id: number; name: string; base_url: string; capacity: number; status: string; usage?: number; connections?: number; active_connections?: number }
type TenantDraft = Omit<Tenant, 'id'> & { limits: Limits; modules: string[]; admin_name: string; admin_username: string; admin_password: string }
type ServerDraft = { name: string; base_url: string; capacity: number; status: string; token: string }
type TenantAdmin = { id: number; tenant_id: number; name: string; username: string; role: string; active: boolean; created_at: string }
type TenantAdminDraft = { name: string; username: string; temporary_password: string }
type PlatformData = { branding: Branding; can_edit_branding: boolean }
type Envelope<T> = { data?: T; error?: { message?: string } }
type Tab = 'tenants' | 'whatsapp' | 'appearance' | 'limits'

const modules = [
  ['conversas', 'Atendimento', true], ['canais', 'Canais', true], ['equipe', 'Equipe e setores', true],
  ['ia', 'Agentes de IA', false], ['tools', 'Tools para agentes', false], ['api', 'API e webhooks', false],
  ['campanhas', 'Campanhas', false], ['contatos', 'Contatos', false], ['relatorios', 'Relatórios', false], ['mensagens', 'Biblioteca de mensagens', false],
] as const
const fixedModules = modules.filter(([, , fixed]) => fixed).map(([key]) => key)
const defaultBranding: Branding = { name: 'Notreve Chat', logo_url: '', favicon_url: '', primary_color: '#2dd27f', secondary_color: '#101a2a', accent_color: '#7c6cff', background_color: '#f3f6fa', loading_color: '#2dd27f' }
const defaultLimits: Limits = {
  max_canais: 0,
  max_atendentes: 0,
  max_campanhas_ativas: 0,
  campanha_limite_diario: 0,
  max_agentes: 0,
  max_usuarios: 0,
  max_conexoes_whatsapp: 0,
  max_tools: 0,
  max_api_keys: 0,
  max_webhooks: 0,
  creditos_ia_mensais: 0,
}
const newTenant = (): TenantDraft => ({ name: '', slug: '', plan: 'Essencial', status: 'active', timezone: 'America/Sao_Paulo', dominio: '', creditos: 0, limits: { ...defaultLimits }, modules: [...fixedModules], admin_name: '', admin_username: '', admin_password: '' })
const newServer = (): ServerDraft => ({ name: '', base_url: '', capacity: 100, status: 'active', token: '' })

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, { ...options, credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) } })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return payload.data as T
}

export function PlatformPage({ initialTab = 'tenants', onBack }: { initialTab?: Tab; onBack?: () => void }) {
  const tab: Exclude<Tab, 'limits'> = initialTab === 'limits' ? 'tenants' : initialTab
  const [branding, setBranding] = useState(defaultBranding)
  const [canEditBranding, setCanEditBranding] = useState(false)
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [servers, setServers] = useState<WhatsAppServer[]>([])
  const [auditTrail, setAuditTrail] = useState<ContextSwitchAudit[]>([])
  const [switchReason, setSwitchReason] = useState('')
  const [switching, setSwitching] = useState(false)
  const [tenant, setTenant] = useState<TenantDraft>(newTenant)
  const [server, setServer] = useState<ServerDraft>(newServer)
  const [tenantAdmins, setTenantAdmins] = useState<TenantAdmin[]>([])
  const [adminDraft, setAdminDraft] = useState<TenantAdminDraft>({ name: '', username: '', temporary_password: '' })
  const [resetPassword, setResetPassword] = useState('')
  const [tenantID, setTenantID] = useState<number | null>(null)
  const [serverID, setServerID] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [savingAdmin, setSavingAdmin] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [platform, tenantData, serverData, switchData] = await Promise.all([
        request<PlatformData>('/api/react/platform'),
        request<Tenant[] | { tenants?: Tenant[] }>('/api/react/platform/tenants'),
        request<WhatsAppServer[] | { servers?: WhatsAppServer[] }>('/api/react/platform/whatsapp-servers'),
        request<ContextSwitchAudit[]>('/api/react/platform/context-switches?limit=20'),
      ])
      setBranding(platform.branding || defaultBranding); setCanEditBranding(Boolean(platform.can_edit_branding))
      setTenants(Array.isArray(tenantData) ? tenantData : tenantData.tenants || [])
      setServers(Array.isArray(serverData) ? serverData : serverData.servers || [])
      setAuditTrail(Array.isArray(switchData) ? switchData : [])
      setError(null)
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Não foi possível carregar o control plane.') } finally { setLoading(false) }
  }, [])
  useEffect(() => { void load() }, [load])

  const activeCompanies = useMemo(() => tenants.filter((item) => item.status !== 'closed').length, [tenants])
  const activeServers = useMemo(() => servers.filter((item) => item.status === 'active').length, [servers])
  const connections = useMemo(() => servers.reduce((total, item) => total + (item.usage ?? item.active_connections ?? item.connections ?? 0), 0), [servers])
  const changeTenant = <K extends keyof TenantDraft>(key: K, value: TenantDraft[K]) => setTenant((current) => ({ ...current, [key]: value }))
  const changeServer = <K extends keyof ServerDraft>(key: K, value: ServerDraft[K]) => setServer((current) => ({ ...current, [key]: value }))
  const changeBrand = (key: keyof Branding, value: string) => setBranding((current) => ({ ...current, [key]: value }))
  const toggleModule = (key: string) => setTenant((current) => ({ ...current, modules: current.modules.includes(key) ? current.modules.filter((item) => item !== key) : [...current.modules, key] }))
  const loadTenantAdmins = async (id: number) => {
    try { setTenantAdmins(await request<TenantAdmin[]>(`/api/react/platform/tenants/${id}/admins`)) } catch (cause) { setTenantAdmins([]); setError(cause instanceof Error ? cause.message : 'Não foi possível carregar o administrador da empresa.') }
  }
  const resetTenant = () => { setTenantID(null); setTenant(newTenant()); setTenantAdmins([]); setAdminDraft({ name: '', username: '', temporary_password: '' }); setResetPassword('') }
  const resetServer = () => { setServerID(null); setServer(newServer()) }
  const selectTenant = (item: Tenant) => { setTenantID(item.id); setTenant({ ...item, limits: { ...defaultLimits, ...(item.limits || {}) }, modules: Array.from(new Set([...fixedModules, ...(item.modules || [])])), admin_name: '', admin_username: '', admin_password: '' }); setAdminDraft({ name: '', username: '', temporary_password: '' }); setResetPassword(''); void loadTenantAdmins(item.id) }
  const selectServer = (item: WhatsAppServer) => { setServerID(item.id); setServer({ name: item.name, base_url: item.base_url, capacity: item.capacity || 0, status: item.status || 'active', token: '' }) }
  const saveTenant = async () => {
    const name = tenant.name.trim(); const slug = tenant.slug.trim().toLowerCase()
    if (!name || !slug) { setError('Informe o nome e o slug da empresa.'); return }
    if (!tenantID && (!tenant.admin_name.trim() || !tenant.admin_username.trim() || tenant.admin_password.length < 6)) {
      setError('Informe o nome, o usuário e uma senha temporária (mínimo 6 caracteres) do primeiro administrador.')
      return
    }
    setSaving(true)
    try {
      const payload: Record<string, unknown> = { ...tenant, name, slug, modules: Array.from(new Set([...fixedModules, ...tenant.modules])) }
      delete payload.admin_name; delete payload.admin_username; delete payload.admin_password; delete payload.limits
      if (!tenantID) payload.admin = { name: tenant.admin_name.trim(), username: tenant.admin_username.trim(), temporary_password: tenant.admin_password }
      await request(tenantID ? `/api/react/platform/tenants/${tenantID}` : '/api/react/platform/tenants', { method: tenantID ? 'PATCH' : 'POST', body: JSON.stringify(payload) })
      setNotice(tenantID ? 'Empresa atualizada.' : 'Empresa e primeiro administrador criados. Entregue a senha temporária por um canal seguro.')
      resetTenant(); await load()
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Não foi possível salvar a empresa.') } finally { setSaving(false) }
  }
  const assumeTenant = async () => {
    if (tenantID === null) return
    if (!switchReason.trim()) { setError('Informe o motivo da assunção de contexto.'); return }
    setSwitching(true)
    try { await request(`/api/react/platform/tenants/${tenantID}/switch`, { method: 'POST', body: JSON.stringify({ reason: switchReason.trim() }) }); setNotice('Contexto assumido com sucesso. A troca foi registrada na auditoria.'); setSwitchReason(''); await load() } catch (cause) { setError(cause instanceof Error ? cause.message : 'Não foi possível assumir o contexto da empresa.') } finally { setSwitching(false) }
  }
  const saveServer = async () => {
    if (!server.name.trim() || !server.base_url.trim()) { setError('Informe o nome e a URL do servidor WhatsApp.'); return }
    setSaving(true)
    try { await request(serverID ? `/api/react/platform/whatsapp-servers/${serverID}` : '/api/react/platform/whatsapp-servers', { method: serverID ? 'PATCH' : 'POST', body: JSON.stringify({ ...server, name: server.name.trim(), base_url: server.base_url.trim(), capacity: Math.max(0, Number(server.capacity) || 0) }) }); setNotice(serverID ? 'Servidor WhatsApp atualizado.' : 'Servidor WhatsApp cadastrado para balanceamento.'); resetServer(); await load() } catch (cause) { setError(cause instanceof Error ? cause.message : 'Não foi possível salvar o servidor WhatsApp.') } finally { setSaving(false) }
  }
  const createTenantAdmin = async () => {
    if (tenantID === null) return
    if (!adminDraft.name.trim() || !adminDraft.username.trim() || adminDraft.temporary_password.length < 6) { setError('Informe nome, usuário/e-mail e uma senha temporária de pelo menos 6 caracteres.'); return }
    setSavingAdmin(true)
    try { await request(`/api/react/platform/tenants/${tenantID}/admins`, { method: 'POST', body: JSON.stringify({ ...adminDraft, name: adminDraft.name.trim(), username: adminDraft.username.trim() }) }); setNotice('Administrador da empresa criado. Entregue a senha temporária por um canal seguro.'); setAdminDraft({ name: '', username: '', temporary_password: '' }); await loadTenantAdmins(tenantID) } catch (cause) { setError(cause instanceof Error ? cause.message : 'Não foi possível criar o administrador.') } finally { setSavingAdmin(false) }
  }
  const resetTenantAdminPassword = async () => {
    if (tenantID === null || resetPassword.length < 6) { setError('Informe uma senha temporária de pelo menos 6 caracteres.'); return }
    setSavingAdmin(true)
    try { await request(`/api/react/platform/tenants/${tenantID}/admins/reset-password`, { method: 'POST', body: JSON.stringify({ temporary_password: resetPassword }) }); setNotice('Senha temporária redefinida. Entregue-a ao administrador por um canal seguro.'); setResetPassword('') } catch (cause) { setError(cause instanceof Error ? cause.message : 'Não foi possível redefinir a senha.') } finally { setSavingAdmin(false) }
  }
  const saveBranding = async () => {
    setSaving(true)
    try { const data = await request<{ branding: Branding }>('/api/react/platform/branding', { method: 'PUT', body: JSON.stringify(branding) }); setBranding(data.branding); setNotice('Marca da plataforma atualizada.'); window.dispatchEvent(new Event('notreve-branding-updated')) } catch (cause) { setError(cause instanceof Error ? cause.message : 'Não foi possível salvar a marca.') } finally { setSaving(false) }
  }

  return <div className="platform-page">
    <section className="page-intro platform-intro"><div><span className="eyebrow">Notreve · Control plane</span><h2>{tab === 'tenants' ? 'Empresas' : tab === 'whatsapp' ? 'Infraestrutura WhatsApp' : 'Marca da plataforma'}</h2><p>{tab === 'tenants' ? 'Cadastre empresas, defina limites e módulos e acompanhe o consumo de cada workspace.' : tab === 'whatsapp' ? 'Servidores WhatsApp centralizados, capacidade e credenciais protegidas no control plane.' : 'A marca global que todas as empresas enxergam. Exclusiva do super_admin.'}</p></div><div className="platform-intro-actions">{onBack && <button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button>}<button className="secondary-button" type="button" onClick={() => void load()}>↻ Atualizar</button></div></section>
    {error && <Alert kind="alert" text={error} onClose={() => setError(null)} />}{notice && <Alert kind="notice" text={notice} onClose={() => setNotice(null)} />}
    {loading ? <div className="workspace-state">Carregando control plane…</div> : <><section className="platform-usage-grid"><Stat label="Empresas ativas" value={activeCompanies} helper="workspaces cadastrados" icon="◈" /><Stat label="Servidores WhatsApp" value={activeServers} helper={`${servers.length} cadastrados`} icon="◌" /><Stat label="Conexões em uso" value={connections} helper="capacidade distribuída" icon="↗" /></section><section className="platform-panel">{tab === 'tenants' && <><TenantPanel tenants={tenants} draft={tenant} editing={tenantID !== null} saving={saving} select={selectTenant} change={changeTenant} toggleModule={toggleModule} save={saveTenant} reset={resetTenant} assume={assumeTenant} switchReason={switchReason} changeSwitchReason={setSwitchReason} switching={switching} />{tenantID !== null && <TenantAdminPanel admins={tenantAdmins} draft={adminDraft} saving={savingAdmin} password={resetPassword} changePassword={setResetPassword} change={(key, value) => setAdminDraft((current) => ({ ...current, [key]: value }))} create={createTenantAdmin} resetPassword={resetTenantAdminPassword} />}{tenantID !== null && <TenantUsagePanel id={tenantID} />}<ContextSwitchPanel audit={auditTrail} /></>}{tab === 'whatsapp' && <ServerPanel servers={servers} draft={server} editing={serverID !== null} saving={saving} select={selectServer} change={changeServer} save={saveServer} reset={resetServer} />}{tab === 'appearance' && <BrandPanel branding={branding} canEdit={canEditBranding} saving={saving} change={changeBrand} save={saveBranding} />}</section></>}
  </div>
}

function TenantPanel({ tenants, draft, editing, saving, select, change, toggleModule, save, reset, assume, switchReason, changeSwitchReason, switching }: { tenants: Tenant[]; draft: TenantDraft; editing: boolean; saving: boolean; select: (tenant: Tenant) => void; change: <K extends keyof TenantDraft>(key: K, value: TenantDraft[K]) => void; toggleModule: (key: string) => void; save: () => void; reset: () => void; assume: () => void; switchReason: string; changeSwitchReason: (value: string) => void; switching: boolean }) {
  return <div className="platform-content">
    <Heading eyebrow="Workspaces dos clientes" title="Cadastre empresas e libere somente o que cada uma contratou." text="O admin da empresa entra no próprio workspace; limites e módulos nunca se misturam com os de outra empresa." />
    <div className="control-plane-split">
      <List title="Empresas cadastradas" count={tenants.length}>
        {tenants.length
          ? <div className="tenant-list">{tenants.map((item) => <button className="tenant-row" type="button" key={item.id} onClick={() => select(item)}><span className="tenant-initial">{item.name.slice(0, 1).toUpperCase()}</span><span className="tenant-row-copy"><strong>{item.name}</strong><small>/{item.slug} · {item.plan || 'Sem plano'}</small></span><Status value={item.status} /><span className="row-action">Editar →</span></button>)}</div>
          : <Empty icon="◈" title="Nenhuma empresa cadastrada" text="Crie a primeira empresa para liberar o setup do admin." />}
      </List>
      <FormCard title={editing ? 'Editar empresa' : 'Nova empresa'} text={editing ? 'Atualize a oferta, os limites e os módulos desta empresa.' : 'Defina a oferta e o primeiro acesso em uma única etapa.'} cancel={editing ? reset : undefined}>
        <div className="platform-form-grid">
          <Field label="Nome da empresa"><input value={draft.name} placeholder="Ex.: Acme Serviços" onChange={(event) => change('name', event.target.value)} /></Field>
          <Field label="Slug do workspace"><input value={draft.slug} placeholder="acme-servicos" onChange={(event) => change('slug', event.target.value.replace(/[^a-z0-9-]/gi, '-').toLowerCase())} /><small>Usado em domínio, URL e integrações.</small></Field>
          <Field label="Plano"><input value={draft.plan} placeholder="Essencial" onChange={(event) => change('plan', event.target.value)} /></Field>
          <Field label="Domínio"><input value={draft.dominio} placeholder="acme.com.br" onChange={(event) => change('dominio', event.target.value)} /><small>Host principal usado para resolver o workspace.</small></Field>
          <Select label="Status" value={draft.status} change={(value) => change('status', value)} options={[['active', 'Ativa'], ['trial', 'Em teste'], ['suspended', 'Suspensa'], ['closed', 'Encerrada']]} />
          <Select label="Fuso horário" value={draft.timezone} change={(value) => change('timezone', value)} options={[['America/Sao_Paulo', 'Brasília (UTC−3)'], ['America/Fortaleza', 'Fortaleza (UTC−3)'], ['America/Manaus', 'Manaus (UTC−4)'], ['UTC', 'UTC']]} />
        </div>
        {!editing && <Subsection title="Primeiro acesso do cliente" text="Crie o administrador da empresa junto com o workspace. Ele é o dono da conta e não acessa o control plane.">
          <div className="platform-form-grid">
            <Field label="Nome do administrador"><input value={draft.admin_name} placeholder="Ex.: Maria Silva" onChange={(event) => change('admin_name', event.target.value)} /></Field>
            <Field label="Usuário ou e-mail"><input value={draft.admin_username} autoComplete="off" placeholder="maria@acme.com.br" onChange={(event) => change('admin_username', event.target.value)} /></Field>
            <Field label="Senha temporária"><input type="password" value={draft.admin_password} autoComplete="new-password" placeholder="mínimo 6 caracteres" onChange={(event) => change('admin_password', event.target.value)} /><small>Entregue por um canal seguro; não será exibida novamente.</small></Field>
          </div>
        </Subsection>}
        <Subsection title="Créditos pré-pagos" text="Saldo de créditos da Notreve IA desta empresa. Os limites de uso (canais, atendentes, agentes, campanhas) vêm do plano contratado — ajuste em Planos e limites.">
          <div className="limits-form">
            <Limit label="Créditos pré-pagos da empresa" value={draft.creditos} change={(value) => change('creditos', Math.max(0, Number(value) || 0))} />
          </div>
        </Subsection>
        <Subsection title="Módulos liberados" text="Os módulos nativos são necessários para atender e ficam sempre ativos.">
          <div className="module-grid">{modules.map(([key, name, fixed]) => <label className={`module-option${fixed ? ' fixed' : ''}`} key={key}><input type="checkbox" checked={draft.modules.includes(key)} disabled={fixed} onChange={() => toggleModule(key)} /><span><strong>{name}</strong><small>{fixed ? 'Nativo da plataforma' : 'Módulo contratável'}</small></span></label>)}</div>
        </Subsection>
        {editing && <Subsection title="Assumir contexto (auditado)" text="O super_admin passa a operar dentro desta empresa por tempo determinado; cada assunção fica registrada na auditoria.">
          <div className="switch-context-form">
            <Field label="Motivo da assunção"><input value={switchReason} placeholder="Ex.: investigar relatório, auxiliar o time da empresa" onChange={(event) => changeSwitchReason(event.target.value)} /><small>Obrigatório; este motivo fica gravado no histórico.</small></Field>
            <div className="form-actions"><button className="secondary-button" type="button" disabled={switching} onClick={assume}>{switching ? 'Assumindo…' : 'Assumir contexto desta empresa'} →</button></div>
          </div>
        </Subsection>}
        <Actions saving={saving} save={save} label={editing ? 'Salvar empresa' : 'Criar empresa'} cancel={editing ? reset : undefined} />
      </FormCard>
    </div>
  </div>
}

function TenantAdminPanel({ admins, draft, saving, password, changePassword, change, create, resetPassword }: { admins: TenantAdmin[]; draft: TenantAdminDraft; saving: boolean; password: string; changePassword: (value: string) => void; change: <K extends keyof TenantAdminDraft>(key: K, value: TenantAdminDraft[K]) => void; create: () => void; resetPassword: () => void }) {
  const admin = admins[0]
  return <div className="platform-content tenant-admin-panel"><Heading eyebrow="Acesso da empresa" title="Defina quem recebe o primeiro acesso administrativo." text="A senha temporária é exibida somente neste formulário: entregue-a ao cliente por um canal seguro." /><div className="control-plane-split"><List title="Administrador da empresa" count={admins.length}>{admin ? <div className="tenant-admin-summary"><span className="tenant-initial">{admin.name.slice(0, 1).toUpperCase()}</span><span className="tenant-row-copy"><strong>{admin.name}</strong><small>{admin.username}</small></span><Status value={admin.active ? 'active' : 'disabled'} /></div> : <Empty icon="♙" title="Administrador pendente" text="Crie o primeiro acesso antes de entregar o workspace ao cliente." />}</List><FormCard title={admin ? 'Redefinir senha temporária' : 'Criar primeiro administrador'} text={admin ? 'Esta ação não altera permissões, papel nem acesso ao control plane.' : 'Este usuário será o dono da empresa, sem acesso ao control plane.'}>{admin ? <><Field label="Nova senha temporária"><input type="password" value={password} autoComplete="new-password" onChange={(event) => changePassword(event.target.value)} /><small>Mínimo de 6 caracteres; ela não será mostrada novamente.</small></Field><Actions saving={saving} save={resetPassword} label="Redefinir senha" /></> : <><div className="platform-form-grid"><Field label="Nome completo"><input value={draft.name} onChange={(event) => change('name', event.target.value)} /></Field><Field label="Usuário ou e-mail"><input value={draft.username} autoComplete="username" onChange={(event) => change('username', event.target.value)} /></Field><Field label="Senha temporária"><input type="password" value={draft.temporary_password} autoComplete="new-password" onChange={(event) => change('temporary_password', event.target.value)} /><small>Mínimo de 6 caracteres; ela não será mostrada novamente.</small></Field></div><Actions saving={saving} save={create} label="Criar administrador" /></>}</FormCard></div></div>
}

function ServerPanel({ servers, draft, editing, saving, select, change, save, reset }: { servers: WhatsAppServer[]; draft: ServerDraft; editing: boolean; saving: boolean; select: (server: WhatsAppServer) => void; change: <K extends keyof ServerDraft>(key: K, value: ServerDraft[K]) => void; save: () => void; reset: () => void }) { return <div className="platform-content"><Heading eyebrow="Pool de conexão" title="Servidores WhatsApp centralizados e prontos para balancear." text="O admin da empresa só conecta um canal. Servidor, capacidade e credenciais ficam protegidos aqui." /><div className="control-plane-split"><List title="Servidores cadastrados" count={servers.length}>{servers.length ? <div className="server-list">{servers.map((item) => { const usage = item.usage ?? item.active_connections ?? item.connections ?? 0; const percent = item.capacity ? Math.min(100, Math.round(usage / item.capacity * 100)) : 0; return <button className="server-row" type="button" key={item.id} onClick={() => select(item)}><div className="server-row-top"><span className="server-icon">◌</span><span className="tenant-row-copy"><strong>{item.name}</strong><small>{item.base_url}</small></span><Status value={item.status} /></div><div className="server-capacity"><span>{usage} em uso{item.capacity ? ` de ${item.capacity}` : ''}</span>{item.capacity > 0 && <div className="usage-progress"><i style={{ width: `${percent}%` }} /></div>}</div><span className="row-action">Editar servidor →</span></button> })}</div> : <Empty icon="◌" title="Nenhum servidor WhatsApp" text="Cadastre o primeiro servidor antes de conectar canais de clientes." />}</List><FormCard title={editing ? 'Editar servidor' : 'Novo servidor WhatsApp'} text="A capacidade evita concentrar novas conexões em um único servidor." cancel={editing ? reset : undefined}><div className="platform-form-grid"><Field label="Nome do servidor"><input value={draft.name} placeholder="WA Produção 01" onChange={(event) => change('name', event.target.value)} /></Field><Field label="URL base"><input type="url" value={draft.base_url} placeholder="https://whatsapp-01.exemplo.com" onChange={(event) => change('base_url', event.target.value)} /><small>Endpoint privado usado pelo sistema.</small></Field><Field label="Capacidade de conexões"><input type="number" min="0" value={draft.capacity} onChange={(event) => change('capacity', Math.max(0, Number(event.target.value) || 0))} /><small>0 = sem limite definido.</small></Field><Select label="Status" value={draft.status} change={(value) => change('status', value)} options={[['active', 'Ativo para novas conexões'], ['maintenance', 'Em manutenção'], ['disabled', 'Desativado']]} /><Field label={editing ? 'Novo token (opcional)' : 'Token do servidor'}><input type="password" autoComplete="new-password" value={draft.token} placeholder={editing ? 'Deixe em branco para manter o atual' : 'Token de acesso'} onChange={(event) => change('token', event.target.value)} /><small>O token salvo nunca é exibido novamente.</small></Field></div><div className="infrastructure-note"><span>↹</span><p>Quando um servidor alcançar sua capacidade, novas conexões poderão ser direcionadas a outro servidor ativo. Empresas nunca veem essa camada.</p></div><Actions saving={saving} save={save} label={editing ? 'Salvar servidor' : 'Cadastrar servidor'} cancel={editing ? reset : undefined} /></FormCard></div></div> }

function BrandPanel({ branding, canEdit, saving, change, save }: { branding: Branding; canEdit: boolean; saving: boolean; change: (key: keyof Branding, value: string) => void; save: () => void }) { return <div className="platform-content"><Heading eyebrow="White-label global" title="A marca que toda a plataforma reconhece." text="Esta é uma configuração global da Notreve, não uma configuração de cliente." /><div className="branding-preview"><div className="branding-preview-top"><div className="preview-mark" style={{ background: branding.primary_color }} /><div><strong>{branding.name || 'Nome da plataforma'}</strong><span>Prévia do workspace</span></div><span className="preview-live"><i /> online</span></div><div className="branding-preview-body"><div className="preview-sidebar" style={{ background: branding.secondary_color }}><span style={{ background: branding.primary_color }} /><i style={{ background: branding.accent_color }} /></div><div className="preview-main" style={{ background: branding.background_color }}><span style={{ background: branding.loading_color }} /><div /></div></div></div><div className="branding-form"><Field label="Nome exibido"><input value={branding.name} disabled={!canEdit} onChange={(event) => change('name', event.target.value)} /></Field><Field label="Logo URL"><input value={branding.logo_url} disabled={!canEdit} placeholder="https://.../logo.svg" onChange={(event) => change('logo_url', event.target.value)} /></Field><Field label="Favicon URL"><input value={branding.favicon_url} disabled={!canEdit} placeholder="https://.../favicon.png" onChange={(event) => change('favicon_url', event.target.value)} /></Field>{(['primary_color', 'secondary_color', 'accent_color', 'background_color', 'loading_color'] as const).map((key) => <Color key={key} label={{ primary_color: 'Cor primária', secondary_color: 'Cor secundária', accent_color: 'Cor de destaque', background_color: 'Fundo', loading_color: 'Carregamento' }[key]} value={branding[key]} disabled={!canEdit} change={(value) => change(key, value)} />)}</div>{canEdit && <div className="form-actions"><button className="primary-button" type="button" disabled={saving} onClick={save}>{saving ? 'Salvando…' : 'Salvar marca'} ↗</button></div>}</div> }

function ContextSwitchPanel({ audit }: { audit: ContextSwitchAudit[] }) { return <div className="platform-content"> <Heading eyebrow="Auditoria" title="Trocas de contexto do super_admin" text="Cada assunção de contexto registra quem trocou, de qual empresa, para qual e o motivo." /> <div className="platform-list-card"><div className="list-card-heading"><strong>Histórico recente</strong><span>{audit.length}</span></div>{audit.length ? <div className="context-switch-list">{audit.map((item) => <div className="context-switch-row" key={item.id}><span className="tenant-initial">{item.actor_name.slice(0, 1).toUpperCase()}</span><span className="tenant-row-copy"><strong>{item.actor_name} → {item.to_slug || `#${item.to_tenant_id}`}</strong><small>de {item.from_slug || `#${item.from_tenant_id}`} · {item.reason || 'sem motivo'}</small></span><small className="context-switch-time">{new Date(item.created_at).toLocaleString()}</small></div>)}</div> : <Empty icon="🗲" title="Nenhuma troca registrada" text="Assuma o contexto de uma empresa para começar o histórico de auditoria." />}</div> </div> }
function Heading({ eyebrow, title, text }: { eyebrow: string; title: string; text: string }) { return <div className="platform-content-heading"><div><span className="eyebrow">{eyebrow}</span><h3>{title}</h3><p>{text}</p></div></div> }
function List({ title, count, children }: { title: string; count: number; children: ReactNode }) { return <div className="platform-list-card"><div className="list-card-heading"><strong>{title}</strong><span>{count}</span></div>{children}</div> }
function FormCard({ title, text, cancel, children }: { title: string; text: string; cancel?: () => void; children: ReactNode }) { return <div className="platform-form-card"><div className="form-heading"><div><strong>{title}</strong><p>{text}</p></div>{cancel && <button className="inline-action" type="button" onClick={cancel}>Nova ficha</button>}</div>{children}</div> }
function Field({ label, children }: { label: string; children: ReactNode }) { return <label className="field"><span>{label}</span>{children}</label> }
function Select({ label, value, change, options }: { label: string; value: string; change: (value: string) => void; options: Array<[string, string]> }) { return <Field label={label}><select value={value} onChange={(event) => change(event.target.value)}>{options.map(([key, name]) => <option key={key} value={key}>{name}</option>)}</select></Field> }
function Limit({ label, value, change }: { label: string; value: number; change: (value: string) => void }) { return <label className="limit-field"><span>{label}</span><input type="number" min="0" step="1" value={value} onChange={(event) => change(event.target.value)} /></label> }
function Subsection({ title, text, children }: { title: string; text: string; children: ReactNode }) { return <section className="form-subsection"><div><strong>{title}</strong><p>{text}</p></div>{children}</section> }
function Actions({ saving, save, label, cancel }: { saving: boolean; save: () => void; label: string; cancel?: () => void }) { return <div className="form-actions"><button className="primary-button" type="button" disabled={saving} onClick={save}>{saving ? 'Salvando…' : label} ↗</button>{cancel && <button className="secondary-button" type="button" onClick={cancel}>Cancelar edição</button>}</div> }
function Status({ value }: { value: string }) { const labels: Record<string, string> = { active: 'Ativa', trial: 'Em teste', suspended: 'Suspensa', closed: 'Encerrada', maintenance: 'Manutenção', disabled: 'Desativado' }; return <span className={`status-badge status-${value}`}>{labels[value] || value}</span> }
function Stat({ label, value, helper, icon }: { label: string; value: number; helper: string; icon: string }) { return <article className="platform-usage-card"><div className="platform-usage-icon">{icon}</div><div className="platform-usage-copy"><span>{label}</span><strong>{value}</strong><em>{helper}</em></div></article> }
function Empty({ icon, title, text }: { icon: string; title: string; text: string }) { return <div className="workspace-empty"><span>{icon}</span><strong>{title}</strong><p>{text}</p></div> }
function Alert({ kind, text, onClose }: { kind: 'alert' | 'notice'; text: string; onClose: () => void }) { return <div className={`workspace-${kind}`} role={kind === 'alert' ? 'alert' : 'status'}><span>{kind === 'alert' ? '!' : '✓'}</span>{text}<button type="button" onClick={onClose} aria-label="Fechar aviso">×</button></div> }
function Color({ label, value, disabled, change }: { label: string; value: string; disabled: boolean; change: (value: string) => void }) { const safe = /^#[0-9a-f]{6}$/i.test(value) ? value : '#2dd27f'; return <label className="color-field"><span>{label}</span><div><input type="color" value={safe} disabled={disabled} onChange={(event) => change(event.target.value)} /><input value={value} disabled={disabled} onChange={(event) => change(event.target.value)} /></div></label> }

type TenantUsageRow = { label: string; used: number; max: number | null; unit?: string }
function TenantUsagePanel({ id }: { id: number }) {
  const [usage, setUsage] = useState<TenantUsage | null>(null)
  const [limits, setLimits] = useState<Partial<Limits> | null>(null)
  const [error, setError] = useState<string | null>(null)
  useEffect(() => {
    let active = true
    void (async () => {
      try {
        const payload = await request<{ usage: TenantUsage; limits: Partial<Limits> }>(`/api/react/platform/tenants/${id}/usage`)
        if (!active) return
        setUsage(payload.usage); setLimits(payload.limits); setError(null)
      } catch (cause) {
        if (active) setError(cause instanceof Error ? cause.message : 'Não foi possível carregar o consumo.')
      }
    })()
    return () => { active = false }
  }, [id])
  if (error) return <div className="platform-content"><Alert kind="alert" text={error} onClose={() => setError(null)} /></div>
  if (!usage) return null
  const rows: TenantUsageRow[] = [
    { label: 'Canais ativos', used: usage.canais_ativos, max: limits?.max_canais ?? null, unit: 'canais' },
    { label: 'Conexões WhatsApp', used: usage.conexoes_whatsapp_ativas, max: limits?.max_conexoes_whatsapp ?? null, unit: 'conexões' },
    { label: 'Atendentes', used: usage.atendentes_ativos, max: limits?.max_atendentes ?? null },
    { label: 'Agentes de IA', used: usage.agentes_ativos, max: limits?.max_agentes ?? null },
    { label: 'Usuários', used: usage.usuarios_ativos, max: limits?.max_usuarios ?? null },
    { label: 'Conversas abertas', used: usage.conversas_abertas, max: null },
  ]
  return <div className="platform-content"><section className="form-subsection"><div><strong>Consumo da empresa</strong><p>Uso real vs. limites do plano. Créditos de IA são consumidos pelas respostas do agente.</p></div><div className="platform-usage-grid">{rows.map((row) => { const hasMax = row.max != null && row.max > 0; const percent = hasMax ? Math.min(100, Math.round(row.used / (row.max as number) * 100)) : 0; return <div className="tenant-usage-item" key={row.label}><span className="tenant-usage-label">{row.label}</span><strong>{row.used}{hasMax ? ` de ${row.max}` : ''}</strong>{hasMax && <div className="usage-progress"><i style={{ width: `${percent}%` }} /></div>}</div> })}</div><div className="tenant-usage-foot"><span>Requisições de API</span><strong>{usage.api_requests}</strong><span>Créditos IA usados</span><strong>{usage.ai_credits_used}</strong></div></section></div>
}

type TenantUsage = { canais_ativos: number; conexoes_whatsapp_ativas: number; atendentes_ativos: number; usuarios_ativos: number; agentes_ativos: number; conversas_abertas: number; api_requests: number; ai_credits_used: number; storage_bytes: number }

export default PlatformPage
