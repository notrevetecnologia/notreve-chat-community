import { useCallback, useEffect, useState } from 'react'
import { Modal } from '../../shared/Modal'
import { FilterBar, FilterSearch, FilterSelect, Pagination, DEFAULT_PAGE_SIZE, encodeCursor, type PaginationMeta } from '../../shared/Pagination'

type McpTool = { id: number; name: string; display_name: string; description: string; ativa: boolean }
type McpServer = {
  id: number
  nome: string
  descricao: string
  transport: string
  command: string
  args: string[]
  env: Record<string, string>
  url: string
  headers: Record<string, string>
  ativo: boolean
  conectado: boolean
  ultimo_erro: string
  server_name: string
  server_version: string
  agent_ids: number[]
  tool_count: number
  tools: McpTool[]
}
type McpAgent = { id: number; name: string; active: boolean }
type McpView = { items: McpServer[]; agents: McpAgent[] }
type Envelope<T> = { data?: T; meta?: PaginationMeta; error?: { message?: string } }

type McpDraft = {
  nome: string
  descricao: string
  transport: string
  command: string
  args: string
  env: string
  url: string
  headers: string
  ativo: boolean
  agent_ids: number[]
}

const emptyDraft = (): McpDraft => ({ nome: '', descricao: '', transport: 'stdio', command: '', args: '', env: '{}', url: '', headers: '{}', ativo: true, agent_ids: [] })
const transportLabel = (value: string) => ({ stdio: 'stdio (processo local)', http: 'HTTP', sse: 'SSE' }[value] || value)

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, { ...options, credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) } })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return payload.data as T
}

async function requestEnvelope<T>(url: string, options?: RequestInit): Promise<{ data: T; meta: PaginationMeta }> {
  const response = await fetch(url, { ...options, credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) } })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return { data: payload.data as T, meta: payload.meta || {} }
}

export function McpPanel({ canManage = true }: { canManage?: boolean }) {
  const [servers, setServers] = useState<McpServer[]>([])
  const [agents, setAgents] = useState<McpAgent[]>([])
  const [draft, setDraft] = useState<McpDraft | null>(null)
  const [editingID, setEditingID] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [busyID, setBusyID] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('')
  const [offset, setOffset] = useState(0)
  const [total, setTotal] = useState(0)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const params = new URLSearchParams({ limit: String(DEFAULT_PAGE_SIZE) })
      if (query.trim()) params.set('q', query.trim())
      if (status) params.set('status', status)
      if (offset > 0) params.set('cursor', encodeCursor(offset))
      const payload = await requestEnvelope<McpView>(`/api/react/mcp?${params.toString()}`)
      setServers(payload.data.items || [])
      setAgents(payload.data.agents || [])
      setTotal(payload.meta.total ?? payload.data.items?.length ?? 0)
      setError(null)
    } catch (loadError) {
      setServers([])
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar os servidores MCP.')
    } finally {
      setLoading(false)
    }
  }, [query, status, offset])

  useEffect(() => { void load() }, [load])
  useEffect(() => { setOffset(0) }, [query, status])

  const openCreate = () => { setEditingID(null); setDraft(emptyDraft()); setError(null); setNotice(null) }
  const openEdit = (server: McpServer) => {
    setEditingID(server.id)
    setDraft({
      nome: server.nome, descricao: server.descricao, transport: server.transport,
      command: server.command, args: (server.args || []).join('\n'),
      env: JSON.stringify(server.env || {}, null, 2), url: server.url,
      headers: JSON.stringify(server.headers || {}, null, 2), ativo: server.ativo, agent_ids: server.agent_ids || [],
    })
    setError(null); setNotice(null)
  }
  const closeEditor = () => { if (!saving) { setDraft(null); setEditingID(null) } }
  const update = <K extends keyof McpDraft>(key: K, value: McpDraft[K]) => setDraft((current) => current ? { ...current, [key]: value } : current)

  const parseObject = (raw: string): Record<string, string> => {
    const parsed = JSON.parse(raw || '{}') as unknown
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error('Os campos precisam ser um objeto JSON.')
    const out: Record<string, string> = {}
    for (const [key, value] of Object.entries(parsed as Record<string, unknown>)) {
      if (typeof value !== 'string') throw new Error('Todos os valores precisam ser texto.')
      out[key] = value
    }
    return out
  }

  const save = async () => {
    if (!draft) return
    if (!draft.nome.trim()) { setError('Informe o nome do servidor MCP.'); return }
    if (draft.transport === 'stdio' && !draft.command.trim()) { setError('Servidor stdio exige um comando.'); return }
    if (draft.transport !== 'stdio' && !draft.url.trim()) { setError('Servidor HTTP/SSE exige uma URL.'); return }
    let env: Record<string, string> = {}
    let headers: Record<string, string> = {}
    try { env = parseObject(draft.env); headers = parseObject(draft.headers) }
    catch (parseError) { setError(parseError instanceof Error ? parseError.message : 'JSON inválido em env/headers.'); return }
    const body = {
      nome: draft.nome.trim(), descricao: draft.descricao.trim(), transport: draft.transport,
      command: draft.command.trim(), args: draft.args.split('\n').map((item) => item.trim()).filter(Boolean),
      env, url: draft.url.trim(), headers, ativo: draft.ativo, agent_ids: draft.agent_ids,
    }
    setSaving(true)
    try {
      const url = editingID === null ? '/api/react/mcp' : `/api/react/mcp/${editingID}`
      await request<McpServer>(url, { method: editingID === null ? 'POST' : 'PATCH', body: JSON.stringify(body) })
      closeEditor(); setNotice(editingID === null ? 'Servidor MCP cadastrado. As tools foram sincronizadas automaticamente quando possível.' : 'Servidor MCP atualizado.')
      await load()
    } catch (saveError) { setError(saveError instanceof Error ? saveError.message : 'Não foi possível salvar o servidor MCP.') }
    finally { setSaving(false) }
  }

  const runAction = async (server: McpServer, action: 'test' | 'sync') => {
    setBusyID(server.id); setError(null); setNotice(null)
    try {
      if (action === 'test') {
        const result = await request<{ connected: boolean; error?: string; server_name?: string; server_version?: string; tool_count?: number }>(`/api/react/mcp/${server.id}/test`, { method: 'POST', body: '{}' })
        if (result.connected) setNotice(`Conectado a ${result.server_name || 'servidor'}${result.server_version ? ' v' + result.server_version : ''} · ${result.tool_count ?? 0} tools.`)
        else setError(result.error || 'Não foi possível conectar ao servidor MCP.')
      } else {
        const updated = await request<McpServer>(`/api/react/mcp/${server.id}/sync`, { method: 'POST', body: '{}' })
        setServers((current) => current.map((item) => item.id === updated.id ? updated : item))
        setNotice(`${(updated.tools || []).length} tools sincronizadas de ${updated.nome}.`)
      }
      await load()
    } catch (actionError) { setError(actionError instanceof Error ? actionError.message : 'A operação falhou.') }
    finally { setBusyID(null) }
  }

  const remove = async (server: McpServer) => {
    if (!window.confirm(`Excluir o servidor MCP “${server.nome}”? As tools dele deixarão de ser usadas pelos agentes.`)) return
    try { await request(`/api/react/mcp/${server.id}`, { method: 'DELETE' }); setNotice('Servidor MCP excluído.'); await load() }
    catch (removeError) { setError(removeError instanceof Error ? removeError.message : 'Não foi possível excluir o servidor MCP.') }
  }

  const toggleAgent = (id: number) => {
    if (!draft) return
    const ids = draft.agent_ids.includes(id) ? draft.agent_ids.filter((item) => item !== id) : [...draft.agent_ids, id]
    update('agent_ids', ids)
  }

  return <section className="integrations-access">
    <div className="integrations-section-heading">
      <div><span className="eyebrow">Model Context Protocol</span><h3>Servidores MCP</h3></div>
      <div className="integrations-webhook-head-actions"><button className="secondary-button" type="button" onClick={() => void load()}>Atualizar</button>{canManage && <button className="primary-button" type="button" onClick={openCreate}>+ Novo servidor</button>}</div>
    </div>
    <p className="integrations-access-copy">Conecte servidores MCP (stdio ou HTTP/SSE) para expor tools externas aos seus agentes. Depois de cadastrar, clique em <strong>Sincronizar</strong> para importar as tools.</p>

    {error && <div className="integrations-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
    {notice && <div className="integrations-notice" role="status"><span>✓</span>{notice}<button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso">×</button></div>}

    <FilterBar resultLabel={`${total} servidor${total === 1 ? '' : 'es'}`} onReset={() => { setQuery(''); setStatus('') }}>
      <FilterSearch value={query} onChange={setQuery} placeholder="Buscar servidor, comando ou URL" />
      <FilterSelect value={status} onChange={setStatus} label="Filtrar status" options={[{ value: '', label: 'Todos os status' }, { value: 'ativos', label: 'Ativos' }, { value: 'inativos', label: 'Pausados' }]} />
    </FilterBar>

    {loading ? <div className="integrations-state"><span className="integrations-spinner" /> Carregando servidores MCP…</div>
      : servers.length === 0 ? <div className="integrations-empty"><div className="integrations-empty-icon">⧉</div><strong>Nenhum servidor MCP conectado</strong><p>Cadastre um servidor para a IA usar tools externas.</p>{canManage && <button className="primary-button" type="button" onClick={openCreate}>Adicionar servidor</button>}</div>
      : <div className="integrations-key-list">{servers.map((server) => <div className="integrations-key-row integrations-webhook-row" key={server.id}>
          <span className={`integrations-key-status ${server.conectado ? 'active' : ''}`} />
          <div>
            <strong>{server.nome}</strong>
            <small>{transportLabel(server.transport)} · {server.transport === 'stdio' ? (server.command || 'sem comando') : server.url}</small>
            <small>{(server.tool_count || 0)} tools · {server.agent_ids.length} agente(s){server.ultimo_erro ? ` · erro: ${server.ultimo_erro}` : ''}</small>
            {(server.tools || []).length > 0 && <small>{(server.tools || []).map((tool) => tool.name).join(', ')}</small>}
          </div>
          <span className={`integrations-key-state ${server.ativo ? 'is-on' : 'is-off'}`}>{server.ativo ? 'Ativo' : 'Pausado'}</span>
          {canManage && <><button type="button" disabled={busyID === server.id} onClick={() => void runAction(server, 'test')}>Testar</button>
          <button type="button" disabled={busyID === server.id} onClick={() => void runAction(server, 'sync')}>Sincronizar</button>
          <button type="button" onClick={() => openEdit(server)}>Editar</button>
          <button type="button" className="is-danger" onClick={() => void remove(server)}>Excluir</button></>}
        </div>)}</div>}

    {!loading && servers.length > 0 && <Pagination total={total} limit={DEFAULT_PAGE_SIZE} offset={offset} onOffsetChange={setOffset} loading={loading} />}

    {draft && <Modal open title={editingID === null ? 'Conectar servidor MCP' : 'Ajustar servidor MCP'} onClose={closeEditor} size="lg" footer={<><button className="secondary-button" type="button" onClick={closeEditor} disabled={saving}>Cancelar</button><button className="primary-button" type="button" onClick={() => void save()} disabled={saving}>{saving ? 'Salvando…' : 'Salvar servidor'}</button></>}>
        <div className="integrations-form">
          <label><span>Nome *</span><input autoFocus value={draft.nome} onChange={(event) => update('nome', event.target.value)} placeholder="Ex.: GitHub Tools" /></label>
          <label><span>Transporte</span><select value={draft.transport} onChange={(event) => update('transport', event.target.value)}><option value="stdio">stdio (processo local)</option><option value="http">HTTP</option><option value="sse">SSE</option></select></label>
          <label className="integrations-form-wide"><span>Descrição</span><input value={draft.descricao} onChange={(event) => update('descricao', event.target.value)} placeholder="Para que servem estas tools" /></label>
          {draft.transport === 'stdio' ? <>
            <label><span>Comando *</span><input value={draft.command} onChange={(event) => update('command', event.target.value)} placeholder="npx" /></label>
            <label><span>Argumentos (um por linha)</span><textarea value={draft.args} onChange={(event) => update('args', event.target.value)} placeholder={"-y\n@modelcontextprotocol/server-github"} /></label>
            <label className="integrations-form-wide"><span>Variáveis de ambiente (JSON)</span><textarea value={draft.env} onChange={(event) => update('env', event.target.value)} placeholder='{"GITHUB_TOKEN": "..."}' /></label>
          </> : <>
            <label className="integrations-form-wide"><span>URL *</span><input value={draft.url} onChange={(event) => update('url', event.target.value)} placeholder="https://servidor/mcp" /></label>
            <label className="integrations-form-wide"><span>Headers (JSON)</span><textarea value={draft.headers} onChange={(event) => update('headers', event.target.value)} placeholder='{"Authorization": "Bearer ..."}' /></label>
          </>}
          <label className="integrations-form-wide"><span>Agentes que usam este servidor</span>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12, paddingTop: 4 }}>
              {agents.length === 0 && <small>Nenhum agente cadastrado.</small>}
              {agents.map((agent) => <label key={agent.id} style={{ display: 'flex', alignItems: 'center', gap: 6, fontWeight: 400 }}><input type="checkbox" checked={draft.agent_ids.includes(agent.id)} onChange={() => toggleAgent(agent.id)} />{agent.name}</label>)}
            </div>
          </label>
          <label style={{ display: 'flex', alignItems: 'center', gap: 8 }}><input type="checkbox" checked={draft.ativo} onChange={(event) => update('ativo', event.target.checked)} /> Servidor ativo</label>
        </div>
    </Modal>}
  </section>
}
