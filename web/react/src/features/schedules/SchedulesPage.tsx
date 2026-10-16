import { useCallback, useEffect, useState } from 'react'
import { FilterBar, FilterSearch, FilterSelect, Pagination, DEFAULT_PAGE_SIZE, encodeCursor, type PaginationMeta } from '../../shared/Pagination'
import './Schedules.css'

type ScheduleTipo = 'once' | 'interval' | 'cron'
type ScheduleAcao = 'enviar_mensagem' | 'rodar_ferramenta' | 'callback_agente'

type Schedule = {
  id: number
  nome: string
  descricao: string
  tipo: ScheduleTipo
  quando: string
  acao: ScheduleAcao
  payload: Record<string, unknown>
  status: string
  max_execucoes: number | null
  execucoes: number
  proxima_execucao: string | null
  ultima_execucao: string | null
  resultado: string
  erro: string
  canal_id: number | null
  contato_id: number | null
  telefone: string
  agente_id: number | null
}

type Channel = { id: number; nome: string; tipo: string }
type Agent = { id: number; name: string; active: boolean }
type SchedulesView = { items: Schedule[]; channels: Channel[]; agents: Agent[] }
type Envelope<T> = { data?: T; meta?: PaginationMeta; error?: { message?: string } }

type ScheduleDraft = {
  nome: string
  descricao: string
  tipo: ScheduleTipo
  quando: string
  acao: ScheduleAcao
  texto: string
  prompt: string
  ferramenta: string
  argumentos: string
  max_execucoes: string
  canal_id: string
  telefone: string
  agente_id: string
}

const emptyDraft = (): ScheduleDraft => ({
  nome: '', descricao: '', tipo: 'once', quando: '', acao: 'enviar_mensagem',
  texto: '', prompt: '', ferramenta: '', argumentos: '{}', max_execucoes: '',
  canal_id: '', telefone: '', agente_id: '',
})

const statusLabels: Record<string, string> = {
  pendente: 'Pendente',
  executando: 'Executando',
  concluida: 'Concluída',
  falhou: 'Falhou',
  cancelada: 'Cancelada',
}

const tipoLabels: Record<string, string> = { once: 'Uma vez', interval: 'Intervalo', cron: 'Cron' }
const acaoLabels: Record<string, string> = {
  enviar_mensagem: 'Enviar mensagem',
  rodar_ferramenta: 'Rodar ferramenta',
  callback_agente: 'Reativar a IA',
}

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

function formatDate(value: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return date.toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' })
}

function payloadPreview(schedule: Schedule): string {
  const payload = schedule.payload || {}
  if (schedule.acao === 'enviar_mensagem') return String(payload.texto ?? '')
  if (schedule.acao === 'callback_agente') return String(payload.prompt ?? '')
  if (schedule.acao === 'rodar_ferramenta') return String(payload.nome_ferramenta ?? '')
  return ''
}

function scheduleCanCancel(status: string): boolean {
  return status === 'pendente' || status === 'executando' || status === 'falhou'
}

export default function SchedulesPage({ onBack, permissions = null }: { onBack?: () => void; permissions?: string[] | null }) {
  const canManage = permissions == null || permissions.includes('agendamentos_gerenciar')
  const [items, setItems] = useState<Schedule[]>([])
  const [channels, setChannels] = useState<Channel[]>([])
  const [agents, setAgents] = useState<Agent[]>([])
  const [draft, setDraft] = useState<ScheduleDraft | null>(null)
  const [editingID, setEditingID] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [busyID, setBusyID] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('')
  const [tipo, setTipo] = useState('')
  const [acao, setAcao] = useState('')
  const [offset, setOffset] = useState(0)
  const [total, setTotal] = useState(0)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const params = new URLSearchParams({ limit: String(DEFAULT_PAGE_SIZE) })
      if (query.trim()) params.set('q', query.trim())
      if (status) params.set('status', status)
      if (tipo) params.set('tipo', tipo)
      if (acao) params.set('acao', acao)
      if (offset > 0) params.set('cursor', encodeCursor(offset))
      const payload = await requestEnvelope<SchedulesView>(`/api/react/schedules?${params.toString()}`)
      setItems(payload.data.items || [])
      setChannels(payload.data.channels || [])
      setAgents(payload.data.agents || [])
      setTotal(payload.meta.total ?? payload.data.items?.length ?? 0)
      setError(null)
    } catch (loadError) {
      setItems([])
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar os agendamentos.')
    } finally {
      setLoading(false)
    }
  }, [query, status, tipo, acao, offset])

  useEffect(() => { void load() }, [load])
  useEffect(() => { setOffset(0) }, [query, status, tipo, acao])

  const openCreate = () => { setEditingID(null); setDraft(emptyDraft()); setError(null); setNotice(null) }
  const openEdit = (schedule: Schedule) => {
    const payload = schedule.payload || {}
    setEditingID(schedule.id)
    setDraft({
      nome: schedule.nome,
      descricao: schedule.descricao,
      tipo: schedule.tipo,
      quando: schedule.quando,
      acao: schedule.acao,
      texto: String(payload.texto ?? ''),
      prompt: String(payload.prompt ?? ''),
      ferramenta: String(payload.nome_ferramenta ?? ''),
      argumentos: JSON.stringify(payload.argumentos ?? {}, null, 2),
      max_execucoes: schedule.max_execucoes ? String(schedule.max_execucoes) : '',
      canal_id: schedule.canal_id ? String(schedule.canal_id) : '',
      telefone: schedule.telefone || '',
      agente_id: schedule.agente_id ? String(schedule.agente_id) : '',
    })
    setError(null); setNotice(null)
  }
  const closeEditor = () => { if (!saving) { setDraft(null); setEditingID(null) } }
  const update = <K extends keyof ScheduleDraft>(key: K, value: ScheduleDraft[K]) => setDraft((current) => current ? { ...current, [key]: value } : current)

  const buildPayload = (current: ScheduleDraft): Record<string, unknown> => {
    if (current.acao === 'enviar_mensagem') return { texto: current.texto.trim() }
    if (current.acao === 'callback_agente') return { prompt: current.prompt.trim() }
    let argumentos: unknown = {}
    try { argumentos = current.argumentos.trim() ? JSON.parse(current.argumentos) : {} } catch { argumentos = null }
    return { nome_ferramenta: current.ferramenta.trim(), argumentos }
  }

  const save = async () => {
    if (!draft) return
    if (!draft.nome.trim() || !draft.quando.trim()) { setError('Informe o nome e quando o agendamento deve rodar.'); return }
    const payload = buildPayload(draft)
    if (payload.argumentos === null) { setError('Os argumentos da ferramenta precisam ser um JSON válido.'); return }
    setSaving(true)
    try {
      const body: Record<string, unknown> = {
        nome: draft.nome.trim(), descricao: draft.descricao.trim(), tipo: draft.tipo, quando: draft.quando.trim(),
        acao: draft.acao, payload, telefone: draft.telefone.trim(),
        canal_id: draft.canal_id ? Number(draft.canal_id) : 0,
        agente_id: draft.agente_id ? Number(draft.agente_id) : 0,
        max_execucoes: draft.max_execucoes ? Number(draft.max_execucoes) : 0,
      }
      const url = editingID === null ? '/api/react/schedules' : `/api/react/schedules/${editingID}`
      await request<Schedule>(url, { method: editingID === null ? 'POST' : 'PATCH', body: JSON.stringify(body) })
      closeEditor(); setNotice(editingID === null ? 'Agendamento criado.' : 'Agendamento atualizado.')
      await load()
    } catch (saveError) { setError(saveError instanceof Error ? saveError.message : 'Não foi possível salvar o agendamento.') }
    finally { setSaving(false) }
  }

  const trigger = async (schedule: Schedule) => {
    setBusyID(schedule.id)
    try { await request(`/api/react/schedules/${schedule.id}/trigger`, { method: 'POST', body: '{}' }); setNotice('Execução disparada.'); await load() }
    catch (triggerError) { setError(triggerError instanceof Error ? triggerError.message : 'Não foi possível disparar agora.') }
    finally { setBusyID(null) }
  }

  const cancel = async (schedule: Schedule) => {
    if (!window.confirm(`Cancelar o agendamento “${schedule.nome}”?`)) return
    setBusyID(schedule.id)
    try { await request(`/api/react/schedules/${schedule.id}`, { method: 'PATCH', body: JSON.stringify({ status: 'cancelada' }) }); setNotice('Agendamento cancelado.'); await load() }
    catch (cancelError) { setError(cancelError instanceof Error ? cancelError.message : 'Não foi possível cancelar.') }
    finally { setBusyID(null) }
  }

  const reactivate = async (schedule: Schedule) => {
    setBusyID(schedule.id)
    try { await request(`/api/react/schedules/${schedule.id}`, { method: 'PATCH', body: JSON.stringify({ status: 'pendente' }) }); setNotice('Agendamento reativado.'); await load() }
    catch (reactError) { setError(reactError instanceof Error ? reactError.message : 'Não foi possível reativar.') }
    finally { setBusyID(null) }
  }

  const remove = async (schedule: Schedule) => {
    if (!window.confirm(`Excluir o agendamento “${schedule.nome}”?`)) return
    setBusyID(schedule.id)
    try { await request(`/api/react/schedules/${schedule.id}`, { method: 'DELETE' }); setNotice('Agendamento excluído.'); await load() }
    catch (removeError) { setError(removeError instanceof Error ? removeError.message : 'Não foi possível excluir.') }
    finally { setBusyID(null) }
  }

  const channelName = (id: number | null) => channels.find((channel) => channel.id === id)?.nome || (id ? `#${id}` : '—')
  const agentName = (id: number | null) => agents.find((agent) => agent.id === id)?.name || (id ? `#${id}` : '—')

  return <div className="schedules-page">
    <section className="schedules-intro">
      <div>
        <span className="eyebrow">Operação</span>
        <h2>Agendamentos</h2>
        <p>Tarefas que rodam sozinhas: enviar uma mensagem, executar uma ferramenta do sistema ou reativar a IA com um prompt (follow-up). Sobrevive a reinícios e pode ser disparada na hora.</p>
      </div>
      <div className="schedules-intro-actions">
        {onBack && <button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button>}
        {canManage && <button className="primary-button" type="button" onClick={openCreate}>+ Novo agendamento</button>}
      </div>
    </section>

    {error && <div className="schedules-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
    {notice && <div className="schedules-notice" role="status"><span>✓</span>{notice}<button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso">×</button></div>}

    <FilterBar resultLabel={`${total} agendamento${total === 1 ? '' : 's'}`} onReset={() => { setQuery(''); setStatus(''); setTipo(''); setAcao('') }}>
      <FilterSearch value={query} onChange={setQuery} placeholder="Buscar por nome ou telefone" />
      <FilterSelect value={status} onChange={setStatus} label="Filtrar status" options={[{ value: '', label: 'Todos os status' }, { value: 'pendente', label: 'Pendentes' }, { value: 'concluida', label: 'Concluídos' }, { value: 'falhou', label: 'Falharam' }, { value: 'cancelada', label: 'Cancelados' }]} />
      <FilterSelect value={tipo} onChange={setTipo} label="Filtrar tipo" options={[{ value: '', label: 'Todos os tipos' }, { value: 'once', label: 'Uma vez' }, { value: 'interval', label: 'Intervalo' }, { value: 'cron', label: 'Cron' }]} />
      <FilterSelect value={acao} onChange={setAcao} label="Filtrar ação" options={[{ value: '', label: 'Todas as ações' }, { value: 'enviar_mensagem', label: 'Enviar mensagem' }, { value: 'rodar_ferramenta', label: 'Rodar ferramenta' }, { value: 'callback_agente', label: 'Reativar a IA' }]} />
    </FilterBar>

    {loading ? <div className="schedules-state">Carregando agendamentos…</div>
      : items.length === 0 ? <div className="schedules-empty"><strong>Nenhum agendamento cadastrado</strong><p>Crie um agendamento para automatizar mensagens, ferramentas ou follow-ups da IA.</p>{canManage && <button className="primary-button" type="button" onClick={openCreate}>Criar agendamento</button>}</div>
      : <div className="schedules-list">{items.map((schedule) => <article className="schedule-card" key={schedule.id}>
          <header>
            <div>
              <div className="schedule-tags">
                <span className="schedule-tag">{tipoLabels[schedule.tipo] || schedule.tipo}</span>
                <span className="schedule-tag is-action">{acaoLabels[schedule.acao] || schedule.acao}</span>
              </div>
              <h3>{schedule.nome}</h3>
            </div>
            <span className={`schedule-state is-${schedule.status}`}>{statusLabels[schedule.status] || schedule.status}</span>
          </header>
          {schedule.descricao && <p>{schedule.descricao}</p>}
          <dl className="schedule-meta">
            <div><dt>Quando</dt><dd>{schedule.quando}</dd></div>
            <div><dt>Próxima</dt><dd>{formatDate(schedule.proxima_execucao)}</dd></div>
            <div><dt>Execuções</dt><dd>{schedule.execucoes}{schedule.max_execucoes ? ` / ${schedule.max_execucoes}` : ''}</dd></div>
            <div><dt>Destino</dt><dd>{schedule.telefone || '—'}</dd></div>
            <div><dt>Canal</dt><dd>{channelName(schedule.canal_id)}</dd></div>
            <div><dt>Agente</dt><dd>{agentName(schedule.agente_id)}</dd></div>
          </dl>
          {payloadPreview(schedule) && <p className="schedule-payload">{payloadPreview(schedule)}</p>}
          {schedule.erro && <p className="schedule-error">Erro: {schedule.erro}</p>}
          {canManage && <footer>
            <button type="button" disabled={busyID === schedule.id || schedule.status === 'concluida' || schedule.status === 'cancelada'} onClick={() => void trigger(schedule)}>Disparar agora</button>
            <button type="button" onClick={() => openEdit(schedule)}>Editar</button>
            {schedule.status === 'cancelada' && <button type="button" disabled={busyID === schedule.id} onClick={() => void reactivate(schedule)}>Reativar</button>}
            {scheduleCanCancel(schedule.status) && <button type="button" disabled={busyID === schedule.id} onClick={() => void cancel(schedule)}>Cancelar</button>}
            <button type="button" className="is-danger" disabled={busyID === schedule.id} onClick={() => void remove(schedule)}>Excluir</button>
          </footer>}
        </article>)}</div>}

    {!loading && items.length > 0 && <Pagination total={total} limit={DEFAULT_PAGE_SIZE} offset={offset} onOffsetChange={setOffset} loading={loading} />}

    {draft && <div className="schedules-modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) closeEditor() }}>
      <section className="schedules-modal" role="dialog" aria-modal="true" aria-labelledby="schedule-editor-title">
        <header><div><span className="eyebrow">{editingID === null ? 'Novo agendamento' : 'Editar agendamento'}</span><h2 id="schedule-editor-title">{editingID === null ? 'Criar agendamento' : 'Ajustar agendamento'}</h2></div><button type="button" className="schedules-modal-close" onClick={closeEditor} aria-label="Fechar">×</button></header>
        <div className="schedules-form">
          <label className="schedules-form-wide"><span>Nome *</span><input autoFocus value={draft.nome} onChange={(event) => update('nome', event.target.value)} placeholder="Ex.: follow-up de proposta" /></label>
          <label className="schedules-form-wide"><span>Descrição</span><input value={draft.descricao} onChange={(event) => update('descricao', event.target.value)} placeholder="Para que serve este agendamento" /></label>
          <label><span>Tipo *</span>
            <select value={draft.tipo} onChange={(event) => update('tipo', event.target.value as ScheduleTipo)}>
              <option value="once">Uma vez (data/hora)</option>
              <option value="interval">Intervalo (segundos)</option>
              <option value="cron">Cron (5 campos)</option>
            </select>
          </label>
          <label><span>Quando *</span>
            {draft.tipo === 'once'
              ? <input type="datetime-local" value={draft.quando} onChange={(event) => update('quando', event.target.value)} />
              : <input value={draft.quando} onChange={(event) => update('quando', event.target.value)} placeholder={draft.tipo === 'interval' ? '300' : '0 9 * * *'} />}
          </label>
          <label><span>Ação *</span>
            <select value={draft.acao} onChange={(event) => update('acao', event.target.value as ScheduleAcao)}>
              <option value="enviar_mensagem">Enviar mensagem</option>
              <option value="rodar_ferramenta">Rodar ferramenta</option>
              <option value="callback_agente">Reativar a IA (follow-up)</option>
            </select>
          </label>
          <label><span>Telefone do destinatário *</span><input value={draft.telefone} onChange={(event) => update('telefone', event.target.value)} placeholder="5511999999999" /></label>
          <label><span>Canal</span>
            <select value={draft.canal_id} onChange={(event) => update('canal_id', event.target.value)}>
              <option value="">Sem canal específico</option>
              {channels.map((channel) => <option key={channel.id} value={channel.id}>{channel.nome} ({channel.tipo})</option>)}
            </select>
          </label>
          <label><span>Agente</span>
            <select value={draft.agente_id} onChange={(event) => update('agente_id', event.target.value)}>
              <option value="">Agente do canal</option>
              {agents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name}{agent.active ? '' : ' (inativo)'}</option>)}
            </select>
          </label>
          <label><span>Máximo de execuções</span><input type="number" min={0} value={draft.max_execucoes} onChange={(event) => update('max_execucoes', event.target.value)} placeholder="Vazio = ilimitado" /></label>

          {draft.acao === 'enviar_mensagem' && <label className="schedules-form-wide"><span>Mensagem *</span><textarea rows={4} value={draft.texto} onChange={(event) => update('texto', event.target.value)} placeholder="Texto que será enviado ao contato" /></label>}
          {draft.acao === 'callback_agente' && <label className="schedules-form-wide"><span>Prompt da IA *</span><textarea rows={4} value={draft.prompt} onChange={(event) => update('prompt', event.target.value)} placeholder="Ex.: Retome a conversa e pergunte se o cliente decidiu pela proposta." /></label>}
          {draft.acao === 'rodar_ferramenta' && <>
            <label className="schedules-form-wide"><span>Ferramenta *</span><input value={draft.ferramenta} onChange={(event) => update('ferramenta', event.target.value)} placeholder="Nome exato da ferramenta (integração)" /></label>
            <label className="schedules-form-wide"><span>Argumentos (JSON)</span><textarea rows={4} value={draft.argumentos} onChange={(event) => update('argumentos', event.target.value)} placeholder='{"chave": "valor"}' /></label>
          </>}
        </div>
        <footer><button className="secondary-button" type="button" onClick={closeEditor} disabled={saving}>Cancelar</button><button className="primary-button" type="button" onClick={() => void save()} disabled={saving}>{saving ? 'Salvando…' : 'Salvar agendamento'}</button></footer>
      </section>
    </div>}
  </div>
}
