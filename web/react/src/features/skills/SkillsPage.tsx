import { useCallback, useEffect, useState } from 'react'
import { FilterBar, FilterSearch, FilterSelect, Pagination, DEFAULT_PAGE_SIZE, encodeCursor, type PaginationMeta } from '../../shared/Pagination'
import './Skills.css'

type Skill = {
  id: number
  nome: string
  descricao: string
  instrucao_completa: string
  categoria: string
  ativa: boolean
  agent_ids: number[]
}
type Agent = { id: number; name: string; active: boolean }
type SkillsView = { items: Skill[]; agents: Agent[]; categories: string[] }
type Envelope<T> = { data?: T; meta?: PaginationMeta; error?: { message?: string } }

type SkillDraft = {
  nome: string
  descricao: string
  categoria: string
  instrucao_completa: string
  ativa: boolean
  agent_ids: number[]
}

const emptyDraft = (): SkillDraft => ({ nome: '', descricao: '', categoria: 'geral', instrucao_completa: '', ativa: true, agent_ids: [] })

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

export default function SkillsPage({ onBack, permissions = null, onOpenAgent }: { onBack?: () => void; permissions?: string[] | null; onOpenAgent?: (id: number) => void }) {
  const canManage = permissions == null || permissions.includes('agentes_gerenciar')
  const [skills, setSkills] = useState<Skill[]>([])
  const [agents, setAgents] = useState<Agent[]>([])
  const [draft, setDraft] = useState<SkillDraft | null>(null)
  const [editingID, setEditingID] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState('')
  const [status, setStatus] = useState('')
  const [offset, setOffset] = useState(0)
  const [total, setTotal] = useState(0)
  const [categories, setCategories] = useState<string[]>([])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const params = new URLSearchParams({ limit: String(DEFAULT_PAGE_SIZE) })
      if (query.trim()) params.set('q', query.trim())
      if (category) params.set('category', category)
      if (status) params.set('status', status)
      if (offset > 0) params.set('cursor', encodeCursor(offset))
      const payload = await requestEnvelope<SkillsView>(`/api/react/skills?${params.toString()}`)
      setSkills(payload.data.items || [])
      setAgents(payload.data.agents || [])
      setCategories(payload.data.categories || [])
      setTotal(payload.meta.total ?? payload.data.items?.length ?? 0)
      setError(null)
    } catch (loadError) {
      setSkills([])
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar as skills.')
    } finally {
      setLoading(false)
    }
  }, [query, category, status, offset])

  useEffect(() => { void load() }, [load])
  useEffect(() => { setOffset(0) }, [query, category, status])

  const openCreate = () => { setEditingID(null); setDraft(emptyDraft()); setError(null); setNotice(null) }
  const openEdit = (skill: Skill) => {
    setEditingID(skill.id)
    setDraft({ nome: skill.nome, descricao: skill.descricao, categoria: skill.categoria, instrucao_completa: skill.instrucao_completa, ativa: skill.ativa, agent_ids: skill.agent_ids || [] })
    setError(null); setNotice(null)
  }
  const closeEditor = () => { if (!saving) { setDraft(null); setEditingID(null) } }
  const update = <K extends keyof SkillDraft>(key: K, value: SkillDraft[K]) => setDraft((current) => current ? { ...current, [key]: value } : current)
  const toggleAgent = (id: number) => {
    if (!draft) return
    const ids = draft.agent_ids.includes(id) ? draft.agent_ids.filter((item) => item !== id) : [...draft.agent_ids, id]
    update('agent_ids', ids)
  }

  const save = async () => {
    if (!draft) return
    if (!draft.nome.trim() || !draft.instrucao_completa.trim()) { setError('Informe o nome e a instrução completa da skill.'); return }
    setSaving(true)
    try {
      const body = {
        nome: draft.nome.trim(), descricao: draft.descricao.trim(), categoria: draft.categoria.trim() || 'geral',
        instrucao_completa: draft.instrucao_completa.trim(), ativa: draft.ativa, agent_ids: draft.agent_ids,
      }
      const url = editingID === null ? '/api/react/skills' : `/api/react/skills/${editingID}`
      await request<Skill>(url, { method: editingID === null ? 'POST' : 'PATCH', body: JSON.stringify(body) })
      closeEditor(); setNotice(editingID === null ? 'Skill criada.' : 'Skill atualizada.')
      await load()
    } catch (saveError) { setError(saveError instanceof Error ? saveError.message : 'Não foi possível salvar a skill.') }
    finally { setSaving(false) }
  }

  const remove = async (skill: Skill) => {
    if (!window.confirm(`Excluir a skill “${skill.nome}”? Os agentes deixarão de usá-la.`)) return
    try { await request(`/api/react/skills/${skill.id}`, { method: 'DELETE' }); setNotice('Skill excluída.'); await load() }
    catch (removeError) { setError(removeError instanceof Error ? removeError.message : 'Não foi possível excluir a skill.') }
  }

  const restoreDefaults = async () => {
    if (!window.confirm('Criar as skills padrão (atendimento, vendas, suporte) caso ainda não existam?')) return
    try { await request('/api/react/skills/seed', { method: 'POST', body: '{}' }); setNotice('Skills padrão disponíveis.'); await load() }
    catch (seedError) { setError(seedError instanceof Error ? seedError.message : 'Não foi possível restaurar as skills padrão.') }
  }

  const agentName = (id: number) => agents.find((agent) => agent.id === id)?.name || `#${id}`

  return <div className="skills-page">
    <section className="skills-intro">
      <div>
        <span className="eyebrow">Inteligência</span>
        <h2>Skills dos agentes</h2>
        <p>Pacotes de instruções especializadas que a IA carrega sob demanda. O agente vê apenas o nome e a descrição; a instrução completa é carregada quando ele invoca a skill.</p>
      </div>
      <div className="skills-intro-actions">
        {onBack && <button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button>}
        {canManage && <><button className="secondary-button" type="button" onClick={() => void restoreDefaults()}>Restaurar padrão</button>
        <button className="primary-button" type="button" onClick={openCreate}>+ Nova skill</button></>}
      </div>
    </section>

    {error && <div className="skills-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
    {notice && <div className="skills-notice" role="status"><span>✓</span>{notice}<button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso">×</button></div>}

    <FilterBar resultLabel={`${total} skill${total === 1 ? '' : 's'}`} onReset={() => { setQuery(''); setCategory(''); setStatus('') }}>
      <FilterSearch value={query} onChange={setQuery} placeholder="Buscar skill ou instrução" />
      <FilterSelect value={category} onChange={setCategory} label="Filtrar categoria" options={[{ value: '', label: 'Todas as categorias' }, ...categories.map((item) => ({ value: item, label: item }))]} />
      <FilterSelect value={status} onChange={setStatus} label="Filtrar status" options={[{ value: '', label: 'Todos os status' }, { value: 'ativas', label: 'Ativas' }, { value: 'inativas', label: 'Inativas' }]} />
    </FilterBar>

    {loading ? <div className="skills-state">Carregando skills…</div>
      : skills.length === 0 ? <div className="skills-empty"><strong>Nenhuma skill cadastrada</strong><p>Crie uma skill ou restaure as padrão para começar.</p>{canManage && <button className="primary-button" type="button" onClick={openCreate}>Criar skill</button>}</div>
      : <div className="skills-grid">{skills.map((skill) => <article className="skill-card" key={skill.id}>
          <header>
            <div><span className="skill-category">{skill.categoria}</span><h3>{skill.nome}</h3></div>
            <span className={`skill-state ${skill.ativa ? 'is-on' : 'is-off'}`}>{skill.ativa ? 'Ativa' : 'Inativa'}</span>
          </header>
          <p>{skill.descricao}</p>
          {skill.agent_ids.length === 0 ? (
            <small>Sem agentes vinculados</small>
          ) : (
            <div className="entity-cross-links" style={{ padding: '8px 10px' }}>
              <span>Usada por {skill.agent_ids.length} agente{skill.agent_ids.length === 1 ? '' : 's'}:</span>
              {skill.agent_ids.map((id) => (
                <button key={id} type="button" disabled={!onOpenAgent} onClick={() => onOpenAgent?.(id)}>
                  {agentName(id)}
                </button>
              ))}
            </div>
          )}
          {canManage && <footer><button type="button" onClick={() => openEdit(skill)}>Editar</button><button type="button" className="is-danger" onClick={() => void remove(skill)}>Excluir</button></footer>}
        </article>)}</div>}

    {!loading && skills.length > 0 && <Pagination total={total} limit={DEFAULT_PAGE_SIZE} offset={offset} onOffsetChange={setOffset} loading={loading} />}

    {draft && <div className="skills-modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) closeEditor() }}>
      <section className="skills-modal" role="dialog" aria-modal="true" aria-labelledby="skill-editor-title">
        <header><div><span className="eyebrow">{editingID === null ? 'Nova skill' : 'Editar skill'}</span><h2 id="skill-editor-title">{editingID === null ? 'Criar skill' : 'Ajustar skill'}</h2></div><button type="button" className="skills-modal-close" onClick={closeEditor} aria-label="Fechar">×</button></header>
        <div className="skills-form">
          <label><span>Nome *</span><input autoFocus value={draft.nome} onChange={(event) => update('nome', event.target.value)} placeholder="Ex.: vendas-abertura" /></label>
          <label><span>Categoria</span><input value={draft.categoria} onChange={(event) => update('categoria', event.target.value)} placeholder="comercial" /></label>
          <label className="skills-form-wide"><span>Descrição (aparece no prompt) *</span><input value={draft.descricao} onChange={(event) => update('descricao', event.target.value)} placeholder="Quando a IA deve usar esta skill" /></label>
          <label className="skills-form-wide"><span>Instrução completa *</span><textarea rows={10} value={draft.instrucao_completa} onChange={(event) => update('instrucao_completa', event.target.value)} placeholder="Passo a passo que a IA deve seguir quando invocar a skill" /></label>
          <label className="skills-form-wide"><span>Agentes vinculados</span>
            <div className="skills-agents">
              {agents.length === 0 && <small>Nenhum agente cadastrado.</small>}
              {agents.map((agent) => <label key={agent.id} className="skills-agent-option"><input type="checkbox" checked={draft.agent_ids.includes(agent.id)} onChange={() => toggleAgent(agent.id)} />{agent.name}</label>)}
            </div>
          </label>
          <label className="skills-agent-option"><input type="checkbox" checked={draft.ativa} onChange={(event) => update('ativa', event.target.checked)} /> Skill ativa</label>
        </div>
        <footer><button className="secondary-button" type="button" onClick={closeEditor} disabled={saving}>Cancelar</button><button className="primary-button" type="button" onClick={() => void save()} disabled={saving}>{saving ? 'Salvando…' : 'Salvar skill'}</button></footer>
      </section>
    </div>}
  </div>
}
