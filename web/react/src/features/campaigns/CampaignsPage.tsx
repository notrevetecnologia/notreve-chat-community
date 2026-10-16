import { useCallback, useEffect, useMemo, useState } from 'react'
import { Modal } from '../../shared/Modal'
import { FilterBar, FilterSearch, FilterSelect, Pagination, DEFAULT_PAGE_SIZE, encodeCursor } from '../../shared/Pagination'
import { CampaignEditorPage, GroupEditorPage, SegmentEditorPage } from './CampaignEditorPage'
import {
  campaignActionLabel,
  campaignActions,
  campaignCanDelete,
  campaignStatus,
  emptyDraft,
  emptySegmentDraft,
  fileToBase64,
  formatDate,
  formatDateTime,
  mediaTypeFromMime,
  request,
  requestEnvelope,
  type Campaign,
  type CampaignAction,
  type CampaignResponse,
  type Delivery,
  type DeliveryResponse,
  type Draft,
  type Group,
  type GroupDraft,
  type Options,
  type Segment,
  type SegmentDraft,
} from './campaignsShared'
import './Campaigns.css'

export function CampaignsPage({ onBack, initialTab = 'campaigns', permissions = null }: { onBack: () => void; initialTab?: 'campaigns' | 'segments' | 'groups'; permissions?: string[] | null }) {
  const canManage = permissions == null || permissions.includes('campanhas_gerenciar')
  const canManageGroups = permissions == null || permissions.includes('grupos_canais_gerenciar')
  const [tab, setTab] = useState<'campaigns' | 'segments' | 'groups'>(initialTab)
  const [campaigns, setCampaigns] = useState<Campaign[]>([])
  const [segments, setSegments] = useState<Segment[]>([])
  const [options, setOptions] = useState<Options>({ segments: [], groups: [], channels: [] })
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('')
  const [period, setPeriod] = useState('')
  const [offset, setOffset] = useState(0)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [actionID, setActionID] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [draft, setDraft] = useState<Draft | null>(null)
  const [segmentDraft, setSegmentDraft] = useState<SegmentDraft | null>(null)
  const [groupDraft, setGroupDraft] = useState<GroupDraft | null>(null)
  const [editingGroupID, setEditingGroupID] = useState<number | null>(null)
  const [details, setDetails] = useState<Campaign | null>(null)
  const [deliveries, setDeliveries] = useState<Delivery[]>([])
  const [detailsLoading, setDetailsLoading] = useState(false)
  const [deliveryOffset, setDeliveryOffset] = useState(0)
  const [deliveryTotal, setDeliveryTotal] = useState(0)
  const [deliveryStatus, setDeliveryStatus] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const params = new URLSearchParams({ limit: String(DEFAULT_PAGE_SIZE) })
      if (query.trim()) params.set('q', query.trim())
      if (status) params.set('status', status)
      if (period) params.set('period', period)
      if (offset > 0) params.set('cursor', encodeCursor(offset))
      const payload = await requestEnvelope<CampaignResponse>(`/api/react/campaigns?${params.toString()}`)
      const nextOptions = payload.data.options || { segments: [], groups: [], channels: [] }
      const safeOptions: Options = { segments: nextOptions.segments || [], groups: nextOptions.groups || [], channels: nextOptions.channels || [] }
      setCampaigns(payload.data.items || [])
      setOptions(safeOptions)
      setSegments(safeOptions.segments)
      setTotal(payload.meta.total ?? payload.data.items?.length ?? 0)
      setError(null)
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar as campanhas.')
    } finally { setLoading(false) }
  }, [query, status, period, offset])

  useEffect(() => { void load() }, [load])
  useEffect(() => { setOffset(0) }, [query, status, period])

  const openCreate = () => { setTab('campaigns'); setDraft(emptyDraft(options)); setError(null) }
  const openGroupCreate = () => { setTab('groups'); setEditingGroupID(null); setGroupDraft({ name: '', channel_ids: [] }); setError(null) }
  const openGroupEdit = (group: Group) => { setTab('groups'); setEditingGroupID(group.id); setGroupDraft({ name: group.name, channel_ids: group.channel_ids || [] }); setError(null) }
  const closeDraft = () => { if (!saving) setDraft(null) }
  const updateDraft = <K extends keyof Draft>(key: K, value: Draft[K]) => setDraft((current) => current ? { ...current, [key]: value } : current)

  const saveCampaign = async () => {
    if (!draft || !draft.name.trim() || !draft.message.trim()) { setError('Informe o nome e a mensagem da campanha.'); return }
    if (!draft.target_id) { setError('Selecione um canal ou grupo de canais.'); return }
    setSaving(true)
    try {
      const media = draft.media_file && draft.media_file.size <= 16 * 1024 * 1024 ? await fileToBase64(draft.media_file) : ''
      if (draft.media_file && !media) { setError('A mídia deve ter no máximo 16 MB.'); setSaving(false); return }
      const body = { name: draft.name.trim(), message: draft.message.trim(), channel_id: draft.target_kind === 'channel' ? draft.target_id : null, grupo_id: draft.target_kind === 'group' ? draft.target_id : null, segmento_id: draft.segmento_id, scheduled_at: draft.scheduled_at, intervalo_segundos: Number(draft.intervalo_segundos), limite_diario: Number(draft.limite_diario), variar_mensagem: draft.variar_mensagem, modo_teste: draft.modo_teste, media_b64: media, media_name: draft.media_file?.name || '', media_mime: draft.media_file?.type || 'application/octet-stream', media_type: draft.media_file ? mediaTypeFromMime(draft.media_file.type, draft.media_file.name) : '' }
      await request<Campaign>('/api/react/campaigns', { method: 'POST', body: JSON.stringify(body) })
      closeDraft(); await load()
    } catch (saveError) { setError(saveError instanceof Error ? saveError.message : 'Não foi possível criar a campanha.') }
    finally { setSaving(false) }
  }

  const runAction = async (id: number, action: string, confirm = false) => {
    setActionID(`${action}:${id}`)
    try { await request<Campaign>(`/api/react/campaigns/${id}/status`, { method: 'POST', body: JSON.stringify({ action, confirm }) }); await load() }
    catch (actionError) { setError(actionError instanceof Error ? actionError.message : 'Não foi possível atualizar a campanha.') }
    finally { setActionID(null) }
  }

  const runCampaignAction = (campaign: Campaign, action: CampaignAction) => {
    if (action === 'pausar') {
      void runAction(campaign.id, 'pausar')
      return
    }
    const messages: Record<CampaignAction, string> = {
      iniciar: campaign.status === 'agendada'
        ? `Iniciar a campanha “${campaign.name}” agora? O envio começará pelos contatos válidos do público selecionado.`
        : `Iniciar a campanha “${campaign.name}”? O envio começará pelos contatos válidos do público selecionado.`,
      retomar: `Retomar a campanha “${campaign.name}”? O envio continuará de onde parou.`,
      pausar: '',
      cancelar: `Cancelar a campanha “${campaign.name}”? Os envios pendentes não serão mais processados.`,
    }
    if (!window.confirm(messages[action])) return
    void runAction(campaign.id, action, true)
  }
  const removeCampaign = async (campaign: Campaign) => {
    if (!window.confirm(`Excluir “${campaign.name}”? Os envios associados também serão removidos.`)) return
    setActionID(`delete:${campaign.id}`)
    try { await request(`/api/react/campaigns/${campaign.id}`, { method: 'DELETE', body: JSON.stringify({ confirm: true }) }); await load() }
    catch (removeError) { setError(removeError instanceof Error ? removeError.message : 'Não foi possível excluir a campanha.') }
    finally { setActionID(null) }
  }

  const loadDeliveries = async (campaign: Campaign, nextOffset: number, nextStatus: string) => {
    setDetailsLoading(true)
    try {
      const params = new URLSearchParams({ limit: String(DEFAULT_PAGE_SIZE) })
      if (nextStatus) params.set('status', nextStatus)
      if (nextOffset > 0) params.set('cursor', encodeCursor(nextOffset))
      const payload = await requestEnvelope<DeliveryResponse>(`/api/react/campaigns/${campaign.id}/envios?${params.toString()}`)
      setDeliveries(payload.data.items || [])
      setDeliveryTotal(payload.meta.total ?? payload.data.items?.length ?? 0)
      setDeliveryOffset(nextOffset)
      setDeliveryStatus(nextStatus)
    } catch (detailsError) { setError(detailsError instanceof Error ? detailsError.message : 'Não foi possível carregar os envios.') }
    finally { setDetailsLoading(false) }
  }

  const openDetails = async (campaign: Campaign) => {
    setDetails(campaign); setDeliveries([]); setDeliveryOffset(0); setDeliveryTotal(0); setDeliveryStatus('')
    await loadDeliveries(campaign, 0, '')
  }

  const saveSegment = async () => {
    if (!segmentDraft || !segmentDraft.name.trim()) { setError('Informe o nome do segmento.'); return }
    setSaving(true)
    try { await request<Segment>('/api/react/segments', { method: 'POST', body: JSON.stringify({ name: segmentDraft.name.trim(), description: segmentDraft.description.trim(), filtro_tem_whatsapp: segmentDraft.filtro_tem_whatsapp, filtro_canal: segmentDraft.filtro_canal.trim() }) }); setSegmentDraft(null); await load() }
    catch (segmentError) { setError(segmentError instanceof Error ? segmentError.message : 'Não foi possível criar o segmento.') }
    finally { setSaving(false) }
  }

  const removeSegment = async (segment: Segment) => {
    if (!window.confirm(`Excluir o segmento “${segment.name}”?`)) return
    setActionID(`segment:${segment.id}`)
    try { await request(`/api/react/segments/${segment.id}`, { method: 'DELETE', body: JSON.stringify({ confirm: true }) }); await load() }
    catch (segmentError) { setError(segmentError instanceof Error ? segmentError.message : 'Não foi possível excluir o segmento.') }
    finally { setActionID(null) }
  }

  const saveGroup = async () => {
    if (!groupDraft || !groupDraft.name.trim() || groupDraft.channel_ids.length === 0) { setError('Informe o nome e selecione pelo menos um canal WhatsApp.'); return }
    setSaving(true)
    try {
      const url = editingGroupID === null ? '/api/react/campaign-groups' : `/api/react/campaign-groups/${editingGroupID}`
      await request<Group>(url, { method: editingGroupID === null ? 'POST' : 'PATCH', body: JSON.stringify({ name: groupDraft.name.trim(), channel_ids: groupDraft.channel_ids }) })
      setGroupDraft(null); setEditingGroupID(null); await load()
    } catch (groupError) { setError(groupError instanceof Error ? groupError.message : 'Não foi possível salvar o grupo.') }
    finally { setSaving(false) }
  }

  const removeGroup = async (group: Group) => {
    if (!window.confirm(`Excluir o grupo “${group.name}”? Campanhas existentes não serão apagadas.`)) return
    setActionID(`group:${group.id}`)
    try { await request(`/api/react/campaign-groups/${group.id}`, { method: 'DELETE', body: JSON.stringify({ confirm: true }) }); await load() }
    catch (groupError) { setError(groupError instanceof Error ? groupError.message : 'Não foi possível excluir o grupo.') }
    finally { setActionID(null) }
  }

  const stats = useMemo(() => ({ total: campaigns.length, active: campaigns.filter((item) => item.status === 'rodando' || item.status === 'agendada').length, sent: campaigns.reduce((sum, item) => sum + item.enviadas, 0), failures: campaigns.reduce((sum, item) => sum + item.falhas, 0) }), [campaigns])

  if (draft) {
    return (
      <div className="campaigns-page">
        {error && <div className="campaigns-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
        <CampaignEditorPage draft={draft} options={options} saving={saving} onBack={closeDraft} onChange={updateDraft} onSave={() => void saveCampaign()} />
      </div>
    )
  }

  if (segmentDraft) {
    return (
      <div className="campaigns-page">
        {error && <div className="campaigns-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
        <SegmentEditorPage
          draft={segmentDraft}
          saving={saving}
          onBack={() => { if (!saving) setSegmentDraft(null) }}
          onChange={(key, value) => setSegmentDraft((current) => current ? { ...current, [key]: value } : current)}
          onSave={() => void saveSegment()}
        />
      </div>
    )
  }

  if (groupDraft) {
    return (
      <div className="campaigns-page">
        {error && <div className="campaigns-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
        <GroupEditorPage
          draft={groupDraft}
          channels={options.channels}
          editing={editingGroupID !== null}
          saving={saving}
          onBack={() => { if (!saving) { setGroupDraft(null); setEditingGroupID(null) } }}
          onChange={(key, value) => setGroupDraft((current) => current ? { ...current, [key]: value } : current)}
          onSave={() => void saveGroup()}
        />
      </div>
    )
  }

  return <div className="campaigns-page">
    <section className="campaigns-intro"><div><span className="eyebrow">Comunicação segmentada</span><h2>Campanhas com controle antes do envio.</h2><p>Organize públicos, prepare mensagens e acompanhe cada operação sem transformar um rascunho em disparo por acidente.</p></div><div className="campaigns-intro-actions"><button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button>{tab === 'campaigns' && canManage && <button className="primary-button" type="button" onClick={openCreate}>+ Nova campanha</button>}{tab === 'segments' && canManage && <button className="primary-button" type="button" onClick={() => setSegmentDraft(emptySegmentDraft())}>+ Novo segmento</button>}{tab === 'groups' && canManageGroups && <button className="primary-button" type="button" onClick={openGroupCreate}>+ Novo grupo</button>}</div></section>
    {error && <div className="campaigns-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
    <nav className="campaigns-tabs" aria-label="Comunicação"><button type="button" className={tab === 'campaigns' ? 'is-active' : ''} onClick={() => setTab('campaigns')}>Campanhas <b>{campaigns.length}</b></button><button type="button" className={tab === 'segments' ? 'is-active' : ''} onClick={() => setTab('segments')}>Segmentos <b>{segments.length}</b></button><button type="button" className={tab === 'groups' ? 'is-active' : ''} onClick={() => setTab('groups')}>Grupos de canais <b>{options.groups.length}</b></button></nav>
    {tab === 'campaigns' ? <>
      <section className="campaigns-metrics"><div><span>Campanhas visíveis</span><strong>{stats.total}</strong></div><div><span>Em operação</span><strong>{stats.active}</strong></div><div><span>Mensagens enviadas</span><strong>{stats.sent}</strong></div><div><span>Falhas registradas</span><strong>{stats.failures}</strong></div></section>
      <FilterBar resultLabel={`${total} campanha${total === 1 ? '' : 's'}`} onReset={() => { setQuery(''); setStatus(''); setPeriod('') }}>
        <FilterSearch value={query} onChange={setQuery} placeholder="Buscar campanha, mensagem ou segmento" />
        <FilterSelect value={status} onChange={setStatus} label="Filtrar status" options={[{ value: '', label: 'Todos os status' }, { value: 'rascunho', label: 'Rascunhos' }, { value: 'agendada', label: 'Agendadas' }, { value: 'rodando', label: 'Rodando' }, { value: 'pausada', label: 'Pausadas' }, { value: 'concluida', label: 'Concluídas' }, { value: 'cancelada', label: 'Canceladas' }]} />
        <FilterSelect value={period} onChange={setPeriod} label="Filtrar período" options={[{ value: '', label: 'Todo o período' }, { value: '7', label: 'Últimos 7 dias' }, { value: '30', label: 'Últimos 30 dias' }, { value: '90', label: 'Últimos 90 dias' }]} />
      </FilterBar>
      {loading ? <div className="campaigns-state"><span className="campaigns-spinner" /> Carregando campanhas…</div> : campaigns.length === 0 ? <div className="campaigns-empty"><div className="campaigns-empty-icon">◉</div><strong>Nenhuma campanha encontrada</strong><p>Crie um rascunho para preparar a próxima comunicação com segurança.</p>{canManage && <button className="primary-button" type="button" onClick={openCreate}>Criar campanha</button>}</div> : <div className="campaigns-grid">{campaigns.map((campaign) => <CampaignCard key={campaign.id} campaign={campaign} busy={actionID?.endsWith(`:${campaign.id}`) || false} canManage={canManage} onOpen={() => void openDetails(campaign)} onAction={(action) => runCampaignAction(campaign, action)} onRemove={() => void removeCampaign(campaign)} />)}</div>}
      {!loading && campaigns.length > 0 && <Pagination total={total} limit={DEFAULT_PAGE_SIZE} offset={offset} onOffsetChange={setOffset} loading={loading} />}
    </> : tab === 'segments' ? <SegmentsPanel segments={segments} busyID={actionID} canManage={canManage} onCreate={() => setSegmentDraft(emptySegmentDraft())} onRemove={(segment) => void removeSegment(segment)} /> : <GroupsPanel groups={options.groups} busyID={actionID} canManage={canManageGroups} onCreate={openGroupCreate} onEdit={openGroupEdit} onRemove={(group) => void removeGroup(group)} />}
    {details && <CampaignDetails campaign={details} deliveries={deliveries} loading={detailsLoading} total={deliveryTotal} offset={deliveryOffset} status={deliveryStatus} onStatusChange={(value) => { if (details) void loadDeliveries(details, 0, value) }} onOffsetChange={(value) => { if (details) void loadDeliveries(details, value, deliveryStatus) }} onClose={() => setDetails(null)} />}
  </div>
}

function CampaignCard({ campaign, busy, canManage, onOpen, onAction, onRemove }: { campaign: Campaign; busy: boolean; canManage: boolean; onOpen: () => void; onAction: (action: CampaignAction) => void; onRemove: () => void }) {
  const progress = campaign.total > 0 ? Math.min(100, Math.round(((campaign.enviadas + campaign.falhas) / campaign.total) * 100)) : 0
  const active = campaign.status === 'rodando'
  const actions = campaignActions(campaign.status)
  return <article className="campaign-card"><header><div className={`campaign-icon campaign-icon--${active ? 'active' : 'idle'}`}>◉</div><div className="campaign-card-heading"><span className="campaign-kicker">Campanha #{String(campaign.id).padStart(4, '0')}</span><h3>{campaign.name}</h3></div><span className={`campaign-status campaign-status--${campaign.status}`}><i />{campaignStatus(campaign.status)}</span></header><p className="campaign-message">{campaign.message}</p><div className="campaign-destination"><span>{campaign.channel_name || campaign.grupo_name || 'Canal não informado'}</span><span>{campaign.segmento_name || 'Todos os contatos'}</span></div><div className="campaign-progress"><div><span>{campaign.enviadas + campaign.falhas} de {campaign.total} processados</span><b>{progress}%</b></div><div className="campaign-progress-track"><i style={{ width: `${progress}%` }} /></div></div><div className="campaign-meta"><span>⏱ {campaign.intervalo_segundos || 15}s entre envios</span><span>♧ {campaign.limite_diario || 'Plano'}/dia</span>{campaign.scheduled_at && <span>◷ {formatDateTime(campaign.scheduled_at)}</span>}{campaign.modo_teste && <span className="campaign-test-flag">◇ Modo teste · sem envio real</span>}</div>{canManage ? <footer><button type="button" onClick={onOpen}>Ver envios</button>{actions.map((action) => <button key={action} type="button" disabled={busy} onClick={() => onAction(action)}>{campaignActionLabel(action, campaign.status)}</button>)}{campaignCanDelete(campaign.status) && <button className="is-danger" type="button" disabled={busy} onClick={onRemove}>Excluir</button>}</footer> : <footer><button type="button" onClick={onOpen}>Ver envios</button></footer>}</article>
}

function SegmentsPanel({ segments, busyID, canManage, onCreate, onRemove }: { segments: Segment[]; busyID: string | null; canManage: boolean; onCreate: () => void; onRemove: (segment: Segment) => void }) {
  const [query, setQuery] = useState('')
  const [offset, setOffset] = useState(0)
  const filtered = useMemo(() => {
    const term = query.trim().toLowerCase()
    if (!term) return segments
    return segments.filter((segment) => [segment.name, segment.description, segment.filter_canal || ''].some((value) => value.toLowerCase().includes(term)))
  }, [segments, query])
  useEffect(() => { setOffset(0) }, [query])
  const visible = useMemo(() => filtered.slice(offset, offset + DEFAULT_PAGE_SIZE), [filtered, offset])
  return <section className="segments-panel"><div className="segments-panel-head"><div><span className="eyebrow">Públicos reutilizáveis</span><h3>Segmentos que dão contexto às campanhas.</h3><p>Use grupos de contatos para preparar comunicações mais relevantes. A edição do contato continua vinculada à área de Relacionamento.</p></div>{canManage && <button className="primary-button" type="button" onClick={onCreate}>+ Novo segmento</button>}</div><FilterBar resultLabel={`${filtered.length} segmento${filtered.length === 1 ? '' : 's'}`} onReset={() => setQuery('')}><FilterSearch value={query} onChange={setQuery} placeholder="Buscar segmento" /></FilterBar>{filtered.length === 0 ? <div className="campaigns-empty segments-empty"><div className="campaigns-empty-icon">⌁</div><strong>Nenhum segmento encontrado</strong><p>Crie o primeiro público para deixar suas campanhas prontas para reutilização.</p>{canManage && <button className="primary-button" type="button" onClick={onCreate}>Criar segmento</button>}</div> : <div className="segments-grid">{visible.map((segment) => <article className="segment-card" key={segment.id}><div className="segment-card-top"><div className="segment-avatar">⌁</div><span className="segment-count">{segment.count ?? 0} contatos</span></div><h4>{segment.name}</h4><p>{segment.description || 'Sem descrição adicional.'}</p>{(segment.filter_tem_whatsapp !== undefined && segment.filter_tem_whatsapp !== null || segment.filter_canal) && <div className="segment-filters">{segment.filter_tem_whatsapp !== undefined && segment.filter_tem_whatsapp !== null && <span>{segment.filter_tem_whatsapp ? 'Com WhatsApp' : 'Sem WhatsApp'}</span>}{segment.filter_canal && <span>Canal: {segment.filter_canal}</span>}</div>}<footer><span>Criado {formatDate(segment.created_at)}</span>{canManage && <button className="is-danger" type="button" disabled={busyID === `segment:${segment.id}`} onClick={() => onRemove(segment)}>Excluir</button>}</footer></article>)}</div>}<Pagination total={filtered.length} limit={DEFAULT_PAGE_SIZE} offset={offset} onOffsetChange={setOffset} /></section>
}

function GroupsPanel({ groups, busyID, canManage, onCreate, onEdit, onRemove }: { groups: Group[]; busyID: string | null; canManage: boolean; onCreate: () => void; onEdit: (group: Group) => void; onRemove: (group: Group) => void }) {
  const [query, setQuery] = useState('')
  const [offset, setOffset] = useState(0)
  const filtered = useMemo(() => {
    const term = query.trim().toLowerCase()
    if (!term) return groups
    return groups.filter((group) => [group.name, ...(group.channel_names || [])].some((value) => value.toLowerCase().includes(term)))
  }, [groups, query])
  useEffect(() => { setOffset(0) }, [query])
  const visible = useMemo(() => filtered.slice(offset, offset + DEFAULT_PAGE_SIZE), [filtered, offset])
  return <section className="segments-panel"><div className="segments-panel-head"><div><span className="eyebrow">Distribuição multicanal</span><h3>Grupos de canais prontos para campanhas.</h3><p>Combine números WhatsApp ativos em um único destino e reutilize o grupo em novas operações.</p></div>{canManage && <button className="primary-button" type="button" onClick={onCreate}>+ Novo grupo</button>}</div><FilterBar resultLabel={`${filtered.length} grupo${filtered.length === 1 ? '' : 's'}`} onReset={() => setQuery('')}><FilterSearch value={query} onChange={setQuery} placeholder="Buscar grupo ou canal" /></FilterBar>{filtered.length === 0 ? <div className="campaigns-empty segments-empty"><div className="campaigns-empty-icon">◈</div><strong>Nenhum grupo de canais encontrado</strong><p>Crie um grupo para distribuir uma campanha entre mais de um número WhatsApp.</p>{canManage && <button className="primary-button" type="button" onClick={onCreate}>Criar grupo</button>}</div> : <div className="segments-grid">{visible.map((group) => <article className="segment-card" key={group.id}><div className="segment-card-top"><div className="segment-avatar">◈</div><span className="segment-count">{(group.channel_ids || []).length} canais</span></div><h4>{group.name}</h4><p>{(group.channel_names || []).join(' · ') || 'Sem canais vinculados.'}</p>{canManage && <footer><button type="button" onClick={() => onEdit(group)}>Editar</button><button className="is-danger" type="button" disabled={busyID === `group:${group.id}`} onClick={() => onRemove(group)}>Excluir</button></footer>}</article>)}</div>}<Pagination total={filtered.length} limit={DEFAULT_PAGE_SIZE} offset={offset} onOffsetChange={setOffset} /></section>
}

function CampaignDetails({ campaign, deliveries, loading, total, offset, status, onStatusChange, onOffsetChange, onClose }: { campaign: Campaign; deliveries: Delivery[]; loading: boolean; total: number; offset: number; status: string; onStatusChange: (value: string) => void; onOffsetChange: (value: number) => void; onClose: () => void }) {
  return <Modal open title={campaign.name} onClose={onClose} size="lg" footer={<button className="secondary-button" type="button" onClick={onClose}>Fechar</button>}><div className="campaign-detail-summary"><span><b>{campaign.total}</b> destinatários</span><span><b>{campaign.enviadas}</b> enviados</span><span><b>{campaign.falhas}</b> falhas</span><span><b>{campaign.lidas}</b> lidos</span></div><FilterBar resultLabel={`${total} envio${total === 1 ? '' : 's'}`} onReset={() => onStatusChange('')}><FilterSelect value={status} onChange={onStatusChange} label="Filtrar envios" options={[{ value: '', label: 'Todos os status' }, { value: 'enviado', label: 'Enviados' }, { value: 'falha', label: 'Falhas' }, { value: 'pendente', label: 'Pendentes' }, { value: 'lido', label: 'Lidos' }]} /></FilterBar><div className="campaign-deliveries">{loading ? <div className="campaigns-state"><span className="campaigns-spinner" /> Carregando envios…</div> : deliveries.length === 0 ? <div className="campaigns-empty campaigns-empty--compact"><strong>Nenhum envio processado ainda</strong><p>Os destinatários aparecerão aqui quando a campanha começar.</p></div> : <div className="campaign-deliveries-table"><div className="campaign-delivery-row campaign-delivery-row--head"><span>Telefone</span><span>Status</span><span>Canal</span><span>Data</span></div>{deliveries.map((delivery) => <div className="campaign-delivery-row" key={delivery.id}><span>{delivery.phone}</span><span className={`delivery-status delivery-status--${delivery.status}`}>{delivery.status}</span><span>{delivery.channel_name || '—'}</span><span>{formatDateTime(delivery.sent_at || delivery.created_at)}</span></div>)}</div>}</div><Pagination total={total} limit={DEFAULT_PAGE_SIZE} offset={offset} onOffsetChange={onOffsetChange} loading={loading} /></Modal>
}

export default CampaignsPage
