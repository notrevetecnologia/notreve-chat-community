import { useCallback, useEffect, useState } from 'react'
import { FilterBar, FilterSearch, Pagination, DEFAULT_PAGE_SIZE, encodeCursor, type PaginationMeta } from '../../shared/Pagination'
import './MessageLibrary.css'

type QuickReply = { id: number; shortcut: string; text: string; created_at: string }
type MessageTemplate = { id: number; key: string; title: string; text: string; created_at: string }
type Tab = 'quick-replies' | 'templates'
type Envelope<T> = { data?: T; meta?: PaginationMeta; error?: { message?: string } }

class RequestError extends Error {
  readonly status: number
  constructor(message: string, status: number) {
    super(message)
    this.name = 'RequestError'
    this.status = status
  }
}

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, { ...options, credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) } })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new RequestError(payload.error?.message || 'Não foi possível concluir a operação.', response.status)
  return payload.data as T
}

async function loadSection<T>(url: string): Promise<{ items: T[]; allowed: boolean; meta: PaginationMeta }> {
  try {
    const response = await fetch(url, { credentials: 'same-origin', headers: { 'Content-Type': 'application/json' } })
    const payload = await response.json() as Envelope<{ items: T[] }>
    if (!response.ok) throw new RequestError(payload.error?.message || 'Não foi possível concluir a operação.', response.status)
    return { items: payload.data?.items || [], allowed: true, meta: payload.meta || {} }
  } catch (error) {
    if (error instanceof RequestError && error.status === 403) return { items: [], allowed: false, meta: {} }
    throw error
  }
}

const emptyDraft = () => ({ shortcut: '', text: '' })

export function MessageLibraryPage({ initialTab = 'quick-replies', onBack }: { initialTab?: Tab; onBack?: () => void }) {
  const [tab, setTab] = useState<Tab>(initialTab)
  const [quickReplies, setQuickReplies] = useState<QuickReply[]>([])
  const [templates, setTemplates] = useState<MessageTemplate[]>([])
  const [access, setAccess] = useState<{ quick: boolean; templates: boolean } | null>(null)
  const [draft, setDraft] = useState(emptyDraft())
  const [editingID, setEditingID] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [quickQuery, setQuickQuery] = useState('')
  const [quickOffset, setQuickOffset] = useState(0)
  const [quickTotal, setQuickTotal] = useState(0)
  const [templateQuery, setTemplateQuery] = useState('')
  const [templateOffset, setTemplateOffset] = useState(0)
  const [templateTotal, setTemplateTotal] = useState(0)

  useEffect(() => setTab(initialTab), [initialTab])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const quickParams = new URLSearchParams({ limit: String(DEFAULT_PAGE_SIZE) })
      if (quickQuery.trim()) quickParams.set('q', quickQuery.trim())
      if (quickOffset > 0) quickParams.set('cursor', encodeCursor(quickOffset))
      const templateParams = new URLSearchParams({ limit: String(DEFAULT_PAGE_SIZE) })
      if (templateQuery.trim()) templateParams.set('q', templateQuery.trim())
      if (templateOffset > 0) templateParams.set('cursor', encodeCursor(templateOffset))
      const [quick, standard] = await Promise.all([
        loadSection<QuickReply>(`/api/react/quick-replies?${quickParams.toString()}`),
        loadSection<MessageTemplate>(`/api/react/message-templates?${templateParams.toString()}`),
      ])
      setQuickReplies(quick.items)
      setTemplates(standard.items)
      setQuickTotal(quick.meta.total ?? quick.items.length)
      setTemplateTotal(standard.meta.total ?? standard.items.length)
      setAccess({ quick: quick.allowed, templates: standard.allowed })
      setError(quick.allowed || standard.allowed ? null : 'Seu perfil não tem permissão para acessar a biblioteca de mensagens.')
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar a biblioteca.')
    } finally {
      setLoading(false)
    }
  }, [quickQuery, quickOffset, templateQuery, templateOffset])

  useEffect(() => { void load() }, [load])
  useEffect(() => { setQuickOffset(0) }, [quickQuery])
  useEffect(() => { setTemplateOffset(0) }, [templateQuery])

  useEffect(() => {
    if (!access) return
    if (tab === 'quick-replies' && !access.quick && access.templates) setTab('templates')
    else if (tab === 'templates' && !access.templates && access.quick) setTab('quick-replies')
  }, [access, tab])

  const openCreate = () => { setEditingID(null); setDraft(emptyDraft()); setError(null); setNotice(null) }
  const openEdit = (item: QuickReply) => { setEditingID(item.id); setDraft({ shortcut: item.shortcut, text: item.text }); setError(null); setNotice(null) }
  const saveQuickReply = async () => {
    if (!draft.shortcut.trim() || !draft.text.trim()) { setError('Informe o atalho e o texto da resposta.'); return }
    setSaving(true)
    try {
      const url = editingID === null ? '/api/react/quick-replies' : `/api/react/quick-replies/${editingID}`
      await request(url, { method: editingID === null ? 'POST' : 'PATCH', body: JSON.stringify(draft) })
      setNotice(editingID === null ? 'Resposta rápida criada.' : 'Resposta rápida atualizada.')
      setEditingID(null)
      setDraft(emptyDraft())
      await load()
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : 'Não foi possível salvar a resposta.')
    } finally {
      setSaving(false)
    }
  }
  const removeQuickReply = async (item: QuickReply) => {
    if (!window.confirm(`Excluir a resposta “/${item.shortcut}”?`)) return
    try { await request(`/api/react/quick-replies/${item.id}`, { method: 'DELETE' }); setNotice('Resposta rápida excluída.'); await load() } catch (removeError) { setError(removeError instanceof Error ? removeError.message : 'Não foi possível excluir a resposta.') }
  }
  const saveTemplate = async (item: MessageTemplate, text: string) => {
    try { await request(`/api/react/message-templates/${item.id}`, { method: 'PATCH', body: JSON.stringify({ text }) }); setTemplates((current) => current.map((entry) => entry.id === item.id ? { ...entry, text } : entry)); setNotice(`Mensagem “${item.title}” salva.`) } catch (saveError) { setError(saveError instanceof Error ? saveError.message : 'Não foi possível salvar a mensagem.') }
  }

  const quickCount = quickTotal
  return <div className="message-library-page">
    <section className="page-intro library-intro"><div><span className="eyebrow">Operação · Biblioteca</span><h2>Mensagens prontas no ritmo da equipe.</h2><p>Respostas rápidas agilizam o atendimento; mensagens padrão mantêm a voz da operação consistente em todos os canais.</p></div><div className="library-intro-actions">{onBack && <button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button>}<button className="secondary-button" type="button" onClick={() => void load()}>↻ Atualizar</button></div></section>
    {error && <div className="workspace-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
    {notice && <div className="workspace-notice" role="status"><span>✓</span>{notice}<button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso">×</button></div>}
    <section className="library-panel">
      <header className="library-tabs" role="tablist" aria-label="Biblioteca de mensagens">{access?.quick !== false && <button className={tab === 'quick-replies' ? 'active' : ''} type="button" role="tab" aria-selected={tab === 'quick-replies'} onClick={() => { setTab('quick-replies'); setNotice(null) }}>Respostas rápidas <span>{quickCount}</span></button>}{access?.templates !== false && <button className={tab === 'templates' ? 'active' : ''} type="button" role="tab" aria-selected={tab === 'templates'} onClick={() => { setTab('templates'); setNotice(null) }}>Mensagens padrão <span>{templateTotal}</span></button>}</header>
      {tab === 'quick-replies' && access?.quick !== false && (
        <div className="library-content" style={{ paddingBottom: 0 }}>
          <FilterBar resultLabel={`${quickTotal} resposta${quickTotal === 1 ? '' : 's'}`} onReset={() => setQuickQuery('')}>
            <FilterSearch value={quickQuery} onChange={setQuickQuery} placeholder="Buscar atalho ou texto" />
          </FilterBar>
        </div>
      )}
      {tab === 'templates' && access?.templates !== false && (
        <div className="library-content" style={{ paddingBottom: 0 }}>
          <FilterBar resultLabel={`${templateTotal} mensagem${templateTotal === 1 ? '' : 's'}`} onReset={() => setTemplateQuery('')}>
            <FilterSearch value={templateQuery} onChange={setTemplateQuery} placeholder="Buscar chave, título ou texto" />
          </FilterBar>
        </div>
      )}
      {!loading && access && !access.quick && !access.templates ? <div className="workspace-state">Seu perfil não tem permissão para acessar a biblioteca de mensagens.</div> : loading ? <div className="workspace-state">Carregando biblioteca…</div> : (tab === 'quick-replies' && access?.quick !== false) || access?.templates === false ? <div className="library-content"><div className="library-content-heading"><div><span className="eyebrow">Atalhos pessoais</span><h3>Respostas rápidas</h3><p>Use no Inbox digitando <b>/atalho</b>. Cada atendente pode manter sua própria biblioteca.</p></div><button className="primary-button" type="button" onClick={openCreate}>+ Nova resposta</button></div><div className="quick-editor"><div><span className="eyebrow">{editingID === null ? 'Criar atalho' : 'Editar atalho'}</span><h4>{editingID === null ? 'Uma resposta que economiza tempo' : 'Ajuste sua resposta'}</h4></div><div className="quick-form"><label><span>Atalho</span><input value={draft.shortcut} onChange={(event) => setDraft((current) => ({ ...current, shortcut: event.target.value }))} placeholder="saudacao" /></label><label className="quick-form-wide"><span>Texto enviado ao cliente</span><textarea rows={3} value={draft.text} onChange={(event) => setDraft((current) => ({ ...current, text: event.target.value }))} placeholder="Olá! Como posso ajudar?" /></label></div><div className="quick-editor-actions"><button className="secondary-button" type="button" disabled={saving} onClick={openCreate}>Limpar</button><button className="primary-button" type="button" disabled={saving} onClick={() => void saveQuickReply()}>{saving ? 'Salvando…' : editingID === null ? 'Criar resposta' : 'Salvar alterações'} ↗</button></div></div>{quickReplies.length === 0 ? <div className="workspace-empty"><span>⚡</span><strong>Nenhuma resposta rápida ainda</strong><p>Crie atalhos para o time responder com mais agilidade.</p></div> : <div className="quick-list">{quickReplies.map((item) => <article className="quick-row" key={item.id}><div className="quick-shortcut">/{item.shortcut}</div><p>{item.text}</p><div className="quick-row-actions"><button type="button" onClick={() => openEdit(item)}>Editar</button><button className="danger-link" type="button" onClick={() => void removeQuickReply(item)}>Excluir</button></div></article>)}</div>}</div> : <div className="library-content"><div className="library-content-heading"><div><span className="eyebrow">Voz da operação</span><h3>Mensagens padrão</h3><p>Textos usados pela IA e pelos fluxos automáticos. As variáveis entre chaves são preenchidas pelo sistema.</p></div></div>{templates.length === 0 ? <div className="workspace-empty"><span>▤</span><strong>As mensagens padrão ainda não foram provisionadas</strong><p>Quando os modelos da empresa estiverem disponíveis, você poderá ajustá-los aqui.</p></div> : <div className="template-list">{templates.map((item) => <TemplateRow item={item} key={item.id} onSave={saveTemplate} />)}</div>}</div>}
      {((tab === 'quick-replies' && access?.quick !== false && quickReplies.length > 0) || (tab === 'templates' && access?.templates !== false && templates.length > 0)) && (
        <div style={{ padding: '0 25px 25px' }}>
          {tab === 'quick-replies'
            ? <Pagination total={quickTotal} limit={DEFAULT_PAGE_SIZE} offset={quickOffset} onOffsetChange={setQuickOffset} loading={loading} />
            : <Pagination total={templateTotal} limit={DEFAULT_PAGE_SIZE} offset={templateOffset} onOffsetChange={setTemplateOffset} loading={loading} />}
        </div>
      )}
    </section>
  </div>
}

function TemplateRow({ item, onSave }: { item: MessageTemplate; onSave: (item: MessageTemplate, text: string) => Promise<void> }) {
  const [text, setText] = useState(item.text)
  const [changed, setChanged] = useState(false)
  useEffect(() => { setText(item.text); setChanged(false) }, [item.text])
  return <article className="template-row"><div className="template-heading"><div><span className="template-key">{item.key}</span><h4>{item.title}</h4></div><span className="template-badge">Automação</span></div><textarea value={text} onChange={(event) => { setText(event.target.value); setChanged(true) }} rows={4} /><div className="template-footer"><span>Você pode usar variáveis como <b>{'{setor}'}</b> e <b>{'{horario}'}</b>.</span>{changed && <button className="primary-button" type="button" onClick={() => void onSave(item, text)}>Salvar mensagem ↗</button>}</div></article>
}

export default MessageLibraryPage
