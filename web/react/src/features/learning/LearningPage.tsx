import { useCallback, useEffect, useState } from 'react'
import { Modal } from '../../shared/Modal'
import { FilterBar, FilterSearch, FilterSelect, Pagination, DEFAULT_PAGE_SIZE, encodeCursor, type PaginationMeta } from '../../shared/Pagination'
import './Learning.css'

type LearningItem = {
  id: number
  question: string
  channel: string
  status: 'novo' | 'resolvido' | 'ignorado' | string
  created_at: string
}

type LearningAgent = {
  id: number
  name: string
  active: boolean
}

type AgentFailure = {
  id: number
  conversation_id?: number
  channel: string
  phone?: string
  origem: string
  tipo: string
  exception_class: string
  mensagem_erro: string
  tentativas: number
  created_at: string
  resolvido: boolean
  resolvido_em?: string
  resolvido_por?: string
  resolucao_nota?: string
}

const failureTypeLabel: Record<string, string> = {
  llm: 'Provedor de IA',
  tool: 'Ferramenta',
  channel: 'Canal',
  pipeline: 'Atendimento',
  outros: 'Outros',
}

type Envelope<T> = { data?: T; meta?: PaginationMeta; error?: { message?: string } }

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

function dateLabel(value: string) {
  if (!value) return 'Sem data'
  return new Intl.DateTimeFormat('pt-BR', { dateStyle: 'short', timeStyle: 'short' }).format(new Date(value))
}

const statusLabel: Record<string, string> = { novo: 'Pendente', resolvido: 'Resolvido', ignorado: 'Ignorado' }

export function LearningPage({ onBack, permissions = null }: { onBack?: () => void; permissions?: string[] | null }) {
  const canTeach = permissions == null || permissions.includes('agentes_gerenciar')
  const [items, setItems] = useState<LearningItem[]>([])
  const [agents, setAgents] = useState<LearningAgent[]>([])
  const [counts, setCounts] = useState<Record<string, number>>({})
  const [failures, setFailures] = useState<AgentFailure[]>([])
  const [failuresOpen, setFailuresOpen] = useState(0)
  const [failuresLoading, setFailuresLoading] = useState(true)
  const [failureSavingID, setFailureSavingID] = useState<number | null>(null)
  const [filter, setFilter] = useState('novo')
  const [query, setQuery] = useState('')
  const [offset, setOffset] = useState(0)
  const [total, setTotal] = useState(0)
  const [failureQuery, setFailureQuery] = useState('')
  const [failureStatus, setFailureStatus] = useState('abertas')
  const [failureOffset, setFailureOffset] = useState(0)
  const [failureTotal, setFailureTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [savingID, setSavingID] = useState<number | null>(null)
  const [teaching, setTeaching] = useState<LearningItem | null>(null)
  const [teachingAgentID, setTeachingAgentID] = useState<number | ''>('')
  const [responseText, setResponseText] = useState('')
  const [procedureText, setProcedureText] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setFailuresLoading(true)
    try {
      const params = new URLSearchParams({ limit: String(DEFAULT_PAGE_SIZE) })
      if (filter && filter !== 'todos') params.set('status', filter)
      if (query.trim()) params.set('q', query.trim())
      if (offset > 0) params.set('cursor', encodeCursor(offset))
      const payload = await requestEnvelope<{ items: LearningItem[]; agents?: LearningAgent[]; counts?: Record<string, number> }>(`/api/react/learning?${params.toString()}`)
      setItems(payload.data.items || [])
      setAgents(payload.data.agents || [])
      setCounts(payload.data.counts || {})
      setTotal(payload.meta.total ?? payload.data.items?.length ?? 0)
      setError(null)
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar o aprendizado.')
    } finally {
      setLoading(false)
    }
    try {
      const params = new URLSearchParams({ limit: String(DEFAULT_PAGE_SIZE) })
      if (failureStatus === 'abertas') params.set('status', 'abertas')
      if (failureQuery.trim()) params.set('q', failureQuery.trim())
      if (failureOffset > 0) params.set('cursor', encodeCursor(failureOffset))
      const failuresPayload = await requestEnvelope<{ items: AgentFailure[]; open: number }>(`/api/react/agent-failures?${params.toString()}`)
      setFailures(failuresPayload.data.items || [])
      setFailuresOpen(failuresPayload.data.open || 0)
      setFailureTotal(failuresPayload.meta.total ?? failuresPayload.data.items?.length ?? 0)
    } catch (failuresError) {
      setFailures([])
      setFailuresOpen(0)
      setError(failuresError instanceof Error ? failuresError.message : 'Não foi possível carregar as falhas da IA.')
    } finally {
      setFailuresLoading(false)
    }
  }, [filter, query, offset, failureQuery, failureStatus, failureOffset])

  useEffect(() => { void load() }, [load])
  useEffect(() => { setOffset(0) }, [filter, query])
  useEffect(() => { setFailureOffset(0) }, [failureQuery, failureStatus])

  const summary = { pending: counts.novo || 0, resolved: counts.resolvido || 0, ignored: counts.ignorado || 0 }

  const mark = async (item: LearningItem, status: 'novo' | 'resolvido' | 'ignorado') => {
    setSavingID(item.id)
    try {
      await request(`/api/react/learning/${item.id}/status`, { method: 'POST', body: JSON.stringify({ status }) })
      setItems((current) => current.map((entry) => entry.id === item.id ? { ...entry, status } : entry))
      setError(null)
      setNotice(null)
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : 'Não foi possível atualizar o item.')
    } finally {
      setSavingID(null)
    }
  }

  const resolveFailure = async (failure: AgentFailure) => {
    setFailureSavingID(failure.id)
    try {
      await request(`/api/react/agent-failures/${failure.id}/resolve`, { method: 'POST', body: JSON.stringify({ nota: '' }) })
      setFailures((current) => current.filter((entry) => entry.id !== failure.id))
      setFailuresOpen((current) => Math.max(0, current - 1))
      setError(null)
      setNotice('Falha marcada como resolvida.')
    } catch (resolveError) {
      setError(resolveError instanceof Error ? resolveError.message : 'Não foi possível resolver a falha.')
    } finally {
      setFailureSavingID(null)
    }
  }

  const openTeaching = (item: LearningItem) => {
    const firstAgent = agents.find((agent) => agent.active) || agents[0]
    setTeaching(item)
    setTeachingAgentID(firstAgent?.id || '')
    setResponseText('')
    setProcedureText('')
    setError(null)
    setNotice(null)
  }

  const closeTeaching = () => {
    if (savingID !== null) return
    setTeaching(null)
  }

  const teach = async () => {
    if (!teaching || !teachingAgentID || !responseText.trim() || !procedureText.trim()) {
      setError('Selecione o agente e informe a resposta correta e como a IA deve agir.')
      return
    }
    setSavingID(teaching.id)
    try {
      await request(`/api/react/learning/${teaching.id}/teach`, {
        method: 'POST',
        body: JSON.stringify({ agent_id: teachingAgentID, resposta: responseText, procedimento: procedureText }),
      })
      setItems((current) => current.map((entry) => entry.id === teaching.id ? { ...entry, status: 'resolvido' } : entry))
      setTeaching(null)
      setNotice('Aprendizado cadastrado na base do agente. A IA poderá usar essa orientação nas próximas conversas.')
      setError(null)
    } catch (teachError) {
      setError(teachError instanceof Error ? teachError.message : 'Não foi possível ensinar a IA.')
    } finally {
      setSavingID(null)
    }
  }

  return <div className="learning-page">
    <section className="page-intro learning-intro">
      <div><span className="eyebrow">Inteligência · Aprendizado</span><h2>Transforme dúvidas em conhecimento.</h2><p>Veja o que a IA ainda não soube responder e organize os próximos conteúdos da sua base.</p></div>
      <div className="learning-intro-actions">{onBack && <button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button>}<button className="secondary-button" type="button" onClick={() => void load()}>↻ Atualizar</button></div>
    </section>
    {error && <div className="workspace-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
    {notice && <div className="workspace-notice" role="status"><span>✓</span>{notice}<button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso">×</button></div>}
    <section className="learning-summary-grid" aria-label="Resumo do aprendizado">
      <article><span className="learning-summary-icon pending">?</span><div><span>Perguntas pendentes</span><strong>{summary.pending}</strong></div></article>
      <article><span className="learning-summary-icon resolved">✓</span><div><span>Resolvidas</span><strong>{summary.resolved}</strong></div></article>
      <article><span className="learning-summary-icon ignored">−</span><div><span>Ignoradas</span><strong>{summary.ignored}</strong></div></article>
    </section>
    <section className="learning-panel">
      <header className="learning-panel-heading"><div><span className="eyebrow">Fila de evolução</span><h3>Perguntas que merecem atenção</h3></div><div className="learning-filters" role="tablist" aria-label="Filtrar aprendizado">{[['novo', 'Pendentes'], ['resolvido', 'Resolvidas'], ['todos', 'Todas']].map(([value, label]) => <button key={value} className={filter === value ? 'active' : ''} type="button" role="tab" aria-selected={filter === value} onClick={() => setFilter(value)}>{label}</button>)}</div></header>
      <FilterBar resultLabel={`${total} pergunta${total === 1 ? '' : 's'}`} onReset={() => { setQuery(''); setFilter('novo') }}>
        <FilterSearch value={query} onChange={setQuery} placeholder="Buscar pergunta ou canal" />
      </FilterBar>
      {loading ? <div className="workspace-state">Carregando aprendizado…</div> : items.length === 0 ? <div className="workspace-empty"><span>✦</span><strong>Nada pendente neste filtro</strong><p>Quando a IA encontrar uma pergunta fora da base, ela aparecerá aqui.</p></div> : <div className="learning-list">{items.map((item) => <article className="learning-row" key={item.id}><div className="learning-question"><span className={`learning-status-dot ${item.status}`} /><div><strong>{item.question}</strong><span>{item.channel || 'Canal não identificado'} · {dateLabel(item.created_at)}</span></div></div><span className={`learning-status ${item.status}`}>{statusLabel[item.status] || item.status}</span><div className="learning-actions">{canTeach && agents.length > 0 && <button type="button" className="teach" disabled={savingID === item.id} onClick={() => openTeaching(item)}>Ensinar IA</button>}{item.status !== 'resolvido' && <button type="button" disabled={savingID === item.id} onClick={() => void mark(item, 'resolvido')}>Marcar resolvida</button>}{item.status !== 'ignorado' && <button type="button" className="muted" disabled={savingID === item.id} onClick={() => void mark(item, 'ignorado')}>Ignorar</button>}{item.status !== 'novo' && <button type="button" className="muted" disabled={savingID === item.id} onClick={() => void mark(item, 'novo')}>Reabrir</button>}</div></article>)}</div>}
      {!loading && items.length > 0 && <Pagination total={total} limit={DEFAULT_PAGE_SIZE} offset={offset} onOffsetChange={setOffset} loading={loading} />}
    </section>
    <section className="learning-panel">
      <header className="learning-panel-heading"><div><span className="eyebrow">Confiabilidade</span><h3>Falhas técnicas da IA</h3></div><span className="learning-status">{failuresOpen} em aberto</span></header>
      <FilterBar resultLabel={`${failureTotal} falha${failureTotal === 1 ? '' : 's'}`} onReset={() => { setFailureQuery(''); setFailureStatus('abertas') }}>
        <FilterSearch value={failureQuery} onChange={setFailureQuery} placeholder="Buscar erro, telefone ou canal" />
        <FilterSelect value={failureStatus} onChange={setFailureStatus} label="Filtrar falhas" options={[{ value: 'abertas', label: 'Em aberto' }, { value: 'todos', label: 'Todas' }]} />
      </FilterBar>
      {failuresLoading ? <div className="workspace-state">Carregando falhas…</div> : failures.length === 0 ? <div className="workspace-empty"><span>✓</span><strong>Nenhuma falha em aberto</strong><p>Quando a IA falhar ao responder, o registro aparece aqui para acompanhamento.</p></div> : <div className="learning-list">{failures.map((failure) => <article className="learning-row" key={failure.id}><div className="learning-question"><span className={`learning-status-dot ${failure.resolvido ? 'resolvido' : 'novo'}`} /><div><strong>{failure.mensagem_erro || failure.exception_class}</strong><span>{failureTypeLabel[failure.tipo] || failure.tipo} · {failure.channel || 'canal não identificado'}{failure.phone ? ` · ${failure.phone}` : ''} · {dateLabel(failure.created_at)}</span></div></div><span className={`learning-status ${failure.resolvido ? 'resolvido' : 'novo'}`}>{failure.resolvido ? 'Resolvida' : (failure.origem || 'pipeline')}</span><div className="learning-actions">{!failure.resolvido && <button type="button" disabled={failureSavingID === failure.id} onClick={() => void resolveFailure(failure)}>{failureSavingID === failure.id ? 'Salvando…' : 'Marcar resolvida'}</button>}</div></article>)}</div>}
      {!failuresLoading && failures.length > 0 && <Pagination total={failureTotal} limit={DEFAULT_PAGE_SIZE} offset={failureOffset} onOffsetChange={setFailureOffset} loading={failuresLoading} />}
    </section>
    {teaching && <Modal open title="Ensinar como agir" onClose={closeTeaching} size="md" footer={<><button type="button" className="secondary-button" disabled={savingID === teaching.id} onClick={closeTeaching}>Cancelar</button><button type="button" className="primary-button" disabled={savingID === teaching.id} onClick={() => void teach()}>{savingID === teaching.id ? 'Salvando…' : 'Cadastrar na base'} <span>↗</span></button></>}><div className="learning-modal"><div className="learning-modal-question"><span>Dúvida original</span><strong>{teaching.question}</strong><small>{teaching.channel || 'Canal não identificado'}</small></div><label><span>Agente que deve aprender</span><select value={teachingAgentID} onChange={(event) => setTeachingAgentID(event.target.value ? Number(event.target.value) : '')}>{agents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name}{agent.active ? '' : ' · pausado'}</option>)}</select></label><label><span>Resposta correta</span><textarea rows={4} value={responseText} onChange={(event) => setResponseText(event.target.value)} placeholder="Escreva a resposta que a IA deveria dar ao cliente…" /></label><label><span>Como a IA deve agir</span><textarea rows={4} value={procedureText} onChange={(event) => setProcedureText(event.target.value)} placeholder="Descreva o procedimento: dados a coletar, ferramenta/setor, limites e próximo passo…" /></label></div></Modal>}
  </div>
}

export default LearningPage
