import { useCallback, useEffect, useState } from 'react'
import { ProviderEditorPage } from './ProviderEditorPage'
import {
  emptyDraft,
  request,
  requestEnvelope,
  type Provider,
  type ProviderDraft,
  type ProviderResponse,
  type ProviderTestResult,
} from './providersShared'
import { FilterBar, FilterSearch, FilterSelect, Pagination, DEFAULT_PAGE_SIZE, encodeCursor } from '../../shared/Pagination'
import { LINKS } from '../../shared/links'
import './Providers.css'

export function ProvidersPage({ onBack, permissions = null }: { onBack?: () => void; permissions?: string[] | null }) {
  const canManage = permissions == null || permissions.includes('ias_gerenciar')
  const [providers, setProviders] = useState<Provider[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testingID, setTestingID] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [draft, setDraft] = useState<ProviderDraft | null>(null)
  const [editingID, setEditingID] = useState<number | null>(null)
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
      const payload = await requestEnvelope<ProviderResponse>(`/api/react/providers?${params.toString()}`)
      setProviders(payload.data.items || [])
      setTotal(payload.meta.total ?? payload.data.items?.length ?? 0)
      setError(null)
    }
    catch (loadError) { setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar os provedores.') }
    finally { setLoading(false) }
  }, [query, status, offset])

  useEffect(() => { void load() }, [load])
  useEffect(() => { setOffset(0) }, [query, status])

  const openCreate = () => { setEditingID(null); setDraft(emptyDraft()); setError(null); setNotice(null) }
  const openEdit = (provider: Provider) => {
    setEditingID(provider.id)
    setDraft({ name: provider.name, provider_type: provider.provider_type, base_url: provider.base_url, model: provider.model, api_key: '', active: provider.active, safe_mode: provider.safe_mode, capacidade: provider.capacidade, capabilities: provider.capabilities || [] })
    setError(null); setNotice(null)
  }
  const closeEditor = () => { if (!saving) { setDraft(null); setEditingID(null) } }
  const updateDraft = <K extends keyof ProviderDraft>(key: K, value: ProviderDraft[K]) => setDraft((current) => current ? { ...current, [key]: value } : current)
  const changeProviderType = (providerType: ProviderDraft['provider_type']) => setDraft((current) => {
    if (!current) return current
    if (providerType === 'notreve') return { ...current, provider_type: providerType, name: 'Notreve IA', base_url: '', model: '', capacidade: 'texto', capabilities: [] }
    return { ...current, provider_type: providerType, name: current.name === 'Notreve IA' ? '' : current.name }
  })

  const save = async () => {
    if (!draft) return
    const direct = draft.provider_type === 'notreve'
    if (!direct && !draft.name.trim()) { setError('Informe o nome do provedor.'); return }
    if (!editingID && direct && !draft.api_key.trim()) { setError('Cole o token da Notreve IA para conectar.'); return }
    setSaving(true)
    try {
      const url = editingID === null ? '/api/react/providers' : `/api/react/providers/${editingID}`
      await request<Provider>(url, { method: editingID === null ? 'POST' : 'PATCH', body: JSON.stringify({
        name: direct ? 'Notreve IA' : draft.name.trim(),
        provider_type: draft.provider_type,
        base_url: direct ? '' : draft.base_url.trim(),
        model: draft.model.trim(),
        api_key: draft.api_key.trim(),
        active: draft.active,
        safe_mode: direct ? true : draft.safe_mode,
        capacidade: direct ? (draft.model.includes('vision') ? 'multimodal' : 'texto') : draft.capacidade,
        capabilities: direct ? [] : draft.capabilities,
      }) })
      closeEditor(); setNotice('Conexão salva com sucesso.'); await load()
    } catch (saveError) { setError(saveError instanceof Error ? saveError.message : 'Não foi possível salvar o provedor.') }
    finally { setSaving(false) }
  }

  const remove = async (provider: Provider) => {
    if (!window.confirm(`Excluir “${provider.name}”? Agentes que a utilizam podem ficar sem resposta.`)) return
    try { await request(`/api/react/providers/${provider.id}`, { method: 'DELETE' }); setNotice('Provedor excluído.'); await load() }
    catch (removeError) { setError(removeError instanceof Error ? removeError.message : 'Não foi possível excluir o provedor.') }
  }

  const test = async (provider: Provider) => {
    setTestingID(provider.id); setError(null)
    try { const result = await request<ProviderTestResult>(`/api/react/providers/${provider.id}/test`, { method: 'POST' }); result.ok ? setNotice(`${provider.name}: ${result.message}`) : setError(`${provider.name}: ${result.message}`) }
    catch (testError) { setError(testError instanceof Error ? testError.message : 'Não foi possível testar o provedor.') }
    finally { setTestingID(null) }
  }

  if (draft) {
    return (
      <div className="providers-page">
        {error && <div className="providers-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
        {notice && <div className="providers-notice" role="status"><span>✓</span>{notice}<button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso">×</button></div>}
        <ProviderEditorPage
          draft={draft}
          editingID={editingID}
          saving={saving}
          onBack={closeEditor}
          onChange={updateDraft}
          onTypeChange={changeProviderType}
          onSave={() => void save()}
        />
      </div>
    )
  }

  return <div className="providers-page">
    <section className="providers-intro"><div><span className="eyebrow">Inteligência</span><h2>Conecte a IA e depois ensine seu agente.</h2><p>Use qualquer IA compatível com OpenAI ou, recomendado, a <a href={LINKS.ia} target="_blank" rel="noreferrer">Notreve IA</a> (só o token). Depois, crie um agente para definir como atender e uma base de conhecimento para definir o que ele sabe. Tools são opcionais: elas permitem que o agente execute ações em sistemas externos.</p></div><div className="providers-intro-actions">{onBack && <button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button>}{canManage && <button className="primary-button" type="button" onClick={openCreate}>Conectar Notreve IA</button>}</div></section>
    <section className="providers-guide" aria-label="Como preparar seu agente"><article><span>1</span><div><strong>Conecte a IA</strong><p>Token da Notreve ou credencial da sua IA própria.</p></div></article><article><span>2</span><div><strong>Configure o agente</strong><p>Tom de voz, objetivo e regras de atendimento.</p></div></article><article><span>3</span><div><strong>Adicione conhecimento</strong><p>Textos, sites, PDFs e áudios que a IA pode consultar.</p></div></article><article><span>4</span><div><strong>Conecte tools se precisar</strong><p>Para consultar ou executar ações em outros sistemas.</p></div></article></section>
    {error && <div className="providers-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
    {notice && <div className="providers-notice" role="status"><span>✓</span>{notice}<button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso">×</button></div>}
    <FilterBar resultLabel={`${total} provedor${total === 1 ? '' : 'es'}`} onReset={() => { setQuery(''); setStatus('') }}>
      <FilterSearch value={query} onChange={setQuery} placeholder="Buscar provedor, modelo ou endpoint" />
      <FilterSelect value={status} onChange={setStatus} label="Filtrar status" options={[{ value: '', label: 'Todos os status' }, { value: 'ativos', label: 'Ativos' }, { value: 'inativos', label: 'Pausados' }]} />
    </FilterBar>
    {loading ? <div className="providers-state"><span className="providers-spinner" /> Carregando provedores…</div> : providers.length === 0 ? <div className="providers-empty"><div className="providers-empty-icon">✦</div><strong>Nenhuma IA conectada</strong><p>Conecte a Notreve IA com seu token para começar a criar agentes.</p>{canManage && <button className="primary-button" type="button" onClick={openCreate}>Conectar Notreve IA</button>}</div> : <div className="providers-grid">{providers.map((provider) => <article className="provider-card" key={provider.id}><header><div className={`provider-avatar ${provider.active ? 'is-active' : ''}`}>✦</div><div><span className="provider-kicker">{provider.provider_type === 'notreve' ? 'Integração direta' : 'IA própria'}</span><h3>{provider.name}</h3></div><span className={`provider-status ${provider.active ? 'is-active' : ''}`}><i />{provider.active ? 'Ativo' : 'Pausado'}</span></header><div className="provider-card-body"><div className="provider-detail"><span>Conexão</span><strong>{provider.provider_type === 'notreve' ? 'Gerenciada pela Notreve' : 'Endpoint próprio'}</strong></div><div className="provider-detail"><span>Modelo</span><strong>{provider.model || (provider.provider_type === 'notreve' ? 'Automático' : 'Não informado')}</strong></div><div className="provider-detail"><span>Capacidade</span><strong>{provider.capacidade === 'multimodal' ? 'Multimodal' : 'Texto'}</strong></div><div className="provider-detail"><span>Credencial</span><strong>{provider.has_api_key ? 'Configurada' : 'Pendente'}</strong></div></div>{canManage && <footer><button type="button" onClick={() => void test(provider)} disabled={testingID === provider.id}>{testingID === provider.id ? 'Testando…' : 'Testar conexão'}</button><button type="button" onClick={() => openEdit(provider)}>Editar</button><button className="is-danger" type="button" onClick={() => void remove(provider)}>Excluir</button></footer>}</article>)}</div>}
    {!loading && providers.length > 0 && <Pagination total={total} limit={DEFAULT_PAGE_SIZE} offset={offset} onOffsetChange={setOffset} loading={loading} />}
  </div>
}

export default ProvidersPage
