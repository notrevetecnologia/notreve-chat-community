import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Modal } from '../../shared/Modal'
import { LINKS } from '../../shared/links'
import { ChannelEditorPage } from './ChannelEditorPage'
import {
  connectChannel,
  deleteChannel as deleteChannelRequest,
  disconnectChannel,
  fetchChannelStatus,
  request,
  requestEnvelope,
  testChannelConnection,
  type ChannelConnectionState as ConnectionState,
  type ChannelOperationResult,
} from './channelActions'
import { FilterBar, FilterSearch, FilterSelect, Pagination, DEFAULT_PAGE_SIZE, encodeCursor } from '../../shared/Pagination'
import {
  channelActionAvailable,
  channelConnectLabel,
  channelHealthClass,
  channelHealthText,
  channelHealthTime,
  channelLabel,
  channelTestFeedback,
  draftFromChannel,
  emptyDraft,
  statusText,
  telegramStatusHint,
  telegramStatusText,
  widgetFromChannel,
  widgetScriptURL,
  type AgentSummary,
  type Channel,
  type ChannelDraft,
  type ChatflowSummary,
  type WidgetConfig,
} from './channelsShared'
import './Channels.css'

function ChannelHealthLine({ channel }: { channel: Channel }) {
  if (channel.type !== 'whatsapp' && channel.type !== 'telegram') return null
  const status = channelHealthClass(channel.health_status)
  const text = channelHealthText(channel.health_status)
  const checkedAt = channelHealthTime(channel.last_checked_at)
  return (
    <div className="channel-health-line">
      <span className={`channel-health-dot ${status}`} aria-hidden="true" />
      <strong>{text}</strong>
      {checkedAt && <span className="channel-health-checked">Verificado em {checkedAt}</span>}
      {channel.last_error && <span className="channel-health-error" aria-label="Último erro de monitoramento">{channel.last_error}</span>}
    </div>
  )
}

export function ChannelsPage({ permissions = null }: { permissions?: string[] | null }) {
  const canManage = permissions == null || permissions.includes('canais_gerenciar')
  const [channels, setChannels] = useState<Channel[]>([])
  const [agents, setAgents] = useState<AgentSummary[]>([])
  const [chatflows, setChatflows] = useState<ChatflowSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [draft, setDraft] = useState<ChannelDraft | null>(null)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [saving, setSaving] = useState(false)
  const [connection, setConnection] = useState<Record<number, ConnectionState>>({})
  const [qr, setQr] = useState<{ id: number; image?: string; message: string } | null>(null)
  const [actionId, setActionId] = useState<number | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [offset, setOffset] = useState(0)
  const [total, setTotal] = useState(0)
  const pollRef = useRef<number | null>(null)

  const loadChannels = useCallback(async () => {
    setLoading(true)
    try {
      const params = new URLSearchParams({ limit: String(DEFAULT_PAGE_SIZE) })
      if (query.trim()) params.set('q', query.trim())
      if (typeFilter) params.set('type', typeFilter)
      if (statusFilter) params.set('status', statusFilter)
      if (offset > 0) params.set('cursor', encodeCursor(offset))
      const payload = await requestEnvelope<Channel[]>(`/api/react/channels?${params.toString()}`)
      setChannels(payload.data || [])
      setTotal(payload.meta.total ?? payload.data?.length ?? 0)
      setError(null)
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar os canais.')
    } finally {
      setLoading(false)
    }
  }, [query, typeFilter, statusFilter, offset])

  const loadAgents = useCallback(async () => {
    try {
      const payload = await request<{ items?: AgentSummary[] }>('/api/react/agents?limit=100')
      setAgents(payload.items ?? [])
    } catch {
      setAgents([])
    }
  }, [])

  const loadChatflows = useCallback(async () => {
    try {
      const payload = await request<{ items?: ChatflowSummary[] }>('/api/react/chatflows')
      setChatflows(payload.items ?? [])
    } catch {
      setChatflows([])
    }
  }, [])

  useEffect(() => {
    void loadChannels()
    void loadAgents()
    void loadChatflows()
    return () => {
      if (pollRef.current !== null) window.clearInterval(pollRef.current)
    }
  }, [loadChannels, loadAgents, loadChatflows])
  useEffect(() => { setOffset(0) }, [query, typeFilter, statusFilter])

  const markChannelHealth = (id: number, status: string) => {
    setChannels((current) => current.map((channel) => channel.id === id ? { ...channel, health_status: status, last_checked_at: new Date().toISOString() } : channel))
  }

  const refreshStatus = useCallback(async (id: number) => {
    try {
      const response = await fetchChannelStatus(id)
      const state = response.result
      if (!state) return undefined
      setConnection((current) => ({ ...current, [id]: state }))
      return state
    } catch {
      return undefined
    }
  }, [])

  useEffect(() => {
    channels.filter((channel) => channel.type === 'whatsapp' || channel.type === 'telegram').forEach((channel) => { void refreshStatus(channel.id) })
  }, [channels, refreshStatus])

  const openCreate = () => {
    setEditingId(null)
    setDraft(emptyDraft())
    setError(null)
  }

  const openEdit = (channel: Channel) => {
    setEditingId(channel.id)
    setDraft(draftFromChannel(channel))
    setError(null)
  }

  const closeEditor = () => {
    if (saving) return
    setDraft(null)
    setEditingId(null)
  }

  const saveChannel = async () => {
    if (!draft || !draft.name.trim()) {
      setError('Informe um nome para o canal.')
      return
    }
    if (draft.type === 'telegram' && editingId === null && !draft.token.trim()) {
      setError('Informe o token do bot do Telegram.')
      return
    }
    setSaving(true)
    try {
      const payload: Record<string, unknown> = {
        type: draft.type,
        name: draft.name.trim(),
        active: draft.active,
        respond_grupos: draft.respond_grupos,
        responder_chamadas: draft.responder_chamadas,
        can_campaign: draft.can_campaign,
        agent_id: draft.agent_id ?? 0,
        mode: draft.mode,
        chatflow_id: draft.mode === 'chatflow' ? draft.chatflow_id ?? 0 : 0,
        ...(draft.type === 'webchat' ? { widget: draft.widget } : {}),
        ...(draft.type === 'telegram' && draft.token.trim() ? { token: draft.token.trim() } : {}),
      }
      if (editingId === null) {
        await request<Channel>('/api/react/channels', { method: 'POST', body: JSON.stringify(payload) })
      } else {
        await request<Channel>(`/api/react/channels/${editingId}`, { method: 'PATCH', body: JSON.stringify(payload) })
      }
      closeEditor()
      await loadChannels()
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : 'Não foi possível salvar o canal.')
    } finally {
      setSaving(false)
    }
  }

  const deleteChannel = async (channel: Channel) => {
    if (!window.confirm(`Excluir o canal “${channel.name}”? Essa ação não pode ser desfeita.`)) return
    setActionId(channel.id)
    try {
      await deleteChannelRequest(channel.id)
      await loadChannels()
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : 'Não foi possível excluir o canal.')
    } finally {
      setActionId(null)
    }
  }

  const connectChannelAction = async (channel: Channel) => {
    setActionId(channel.id)
    setNotice(null)
    if (channel.type === 'whatsapp') setQr({ id: channel.id, message: 'Gerando QR Code…' })
    try {
      const response = await connectChannel(channel.id)
      const result = response.result
      if (!result) throw new Error('A conexão não retornou um estado válido.')
      await refreshStatus(channel.id)
      if (channel.type === 'telegram') {
        setQr(null)
        markChannelHealth(channel.id, 'connected')
        setNotice(result.already ? 'O webhook do Telegram já estava configurado.' : 'Webhook do Telegram configurado. O canal está pronto para receber mensagens.')
        return
      }
      if (result.already) {
        setQr(null)
        markChannelHealth(channel.id, 'connected')
        setNotice('Este número já está conectado.')
        return
      }
      setQr({ id: channel.id, image: result.qr, message: result.qr ? 'Escaneie o QR Code no WhatsApp do número da empresa.' : 'A API não retornou um QR Code agora. Tente “Gerar novo QR”.' })
      if (pollRef.current !== null) window.clearInterval(pollRef.current)
      pollRef.current = window.setInterval(() => { void refreshStatus(channel.id) }, 3000)
    } catch (connectError) {
      const message = connectError instanceof Error ? connectError.message : 'Não foi possível conectar.'
      if (channel.type === 'whatsapp') setQr({ id: channel.id, message })
      else setError(message)
    } finally {
      setActionId(null)
    }
  }

  const disconnectChannelAction = async (channel: Channel, action: 'desconectar' | 'logout') => {
    if (action === 'logout' && channel.type === 'whatsapp' && !window.confirm('Fazer logout deste WhatsApp? Será necessário escanear o QR novamente.')) return
    setActionId(channel.id)
    setNotice(null)
    try {
      const response = await disconnectChannel(channel.id, action === 'logout' ? 'logout' : 'disconnect')
      const already = Boolean((response.result as { already?: boolean } | undefined)?.already)
      await refreshStatus(channel.id)
      markChannelHealth(channel.id, 'disconnected')
      setNotice(already ? 'O canal já estava desconectado.' : action === 'logout' ? 'Sessão encerrada. Reconecte pelo QR Code.' : 'Canal desconectado. A sessão foi preservada para reconectar sem QR.')
    } catch (disconnectError) {
      setError(disconnectError instanceof Error ? disconnectError.message : 'Não foi possível desconectar.')
    } finally {
      setActionId(null)
    }
  }

  const testChannel = async (channel: Channel) => {
    setActionId(channel.id)
    setNotice(null)
    try {
      const response = await testChannelConnection(channel.id)
      if (response.result) setConnection((current) => ({ ...current, [channel.id]: response.result as ConnectionState }))
      const state = response.result as ChannelOperationResult
      const feedback = channelTestFeedback(channel, state)
      if (channel.type === 'whatsapp' && !(state?.connected && state?.loggedIn)) setError(feedback)
      else setNotice(feedback)
    } catch (testError) {
      setError(testError instanceof Error ? testError.message : 'Não foi possível testar o canal.')
    } finally {
      setActionId(null)
    }
  }

  const updateDraft = <K extends keyof ChannelDraft>(key: K, value: ChannelDraft[K]) => {
    setDraft((current) => current ? { ...current, [key]: value } : current)
  }

  const updateWidget = <K extends keyof WidgetConfig>(key: K, value: WidgetConfig[K]) => {
    setDraft((current) => current ? { ...current, widget: { ...current.widget, [key]: value } } : current)
  }

  const sortedChannels = useMemo(() => [...channels].sort((left, right) => left.name.localeCompare(right.name, 'pt-BR')), [channels])
  const editingChannel = editingId !== null ? channels.find((channel) => channel.id === editingId) ?? null : null
  const agentChannels = useMemo(() => {
    const map: Record<number, { id: number; name: string }[]> = {}
    for (const channel of channels) {
      if (channel.agent_id) {
        ;(map[channel.agent_id] ??= []).push({ id: channel.id, name: channel.name })
      }
    }
    return map
  }, [channels])

  if (draft) {
    return (
      <div className="channels-page">
        {error && <div className="channels-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
        {notice && <div className="channels-notice" role="status"><span>✓</span>{notice}<button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso">×</button></div>}
        <ChannelEditorPage
          draft={draft}
          editing={editingId !== null}
          saving={saving}
          agents={agents}
          chatflows={chatflows}
          channel={editingChannel}
          agentChannels={agentChannels}
          connection={editingId !== null ? connection[editingId] : undefined}
          actionId={actionId}
          onBack={closeEditor}
          onSave={() => void saveChannel()}
          onChange={updateDraft}
          onWidgetChange={updateWidget}
          canManage={canManage}
          onConnect={() => { if (editingChannel) void connectChannelAction(editingChannel) }}
          onDisconnect={(action) => { if (editingChannel) void disconnectChannelAction(editingChannel, action) }}
          onTest={() => { if (editingChannel) void testChannel(editingChannel) }}
        />
        {qr && <Modal open title="Conectar WhatsApp" onClose={() => setQr(null)} size="sm"><div className="qr-content">{qr.image ? <img src={qr.image} alt="QR Code para conectar o WhatsApp" /> : <div className="qr-placeholder">◉</div>}<p>{qr.message}</p><span className="modal-hint">No celular: WhatsApp → Aparelhos conectados → Conectar um aparelho.</span></div></Modal>}
      </div>
    )
  }

  return (
    <div className="channels-page">
      <section className="channels-intro">
        <div>
          <span className="eyebrow">Conexões da operação</span>
          <h2>Seus canais, sob controle.</h2>
          <p>Configure WhatsApp e Webchat em um só lugar. O WhatsApp usa a <a href={LINKS.whatsappApi} target="_blank" rel="noreferrer">Notreve WhatsApp API</a> (contrate o serviço); sua equipe só conecta o número e acompanha o status.</p>
          <ol className="channels-steps">
            <li>1. Crie um provedor de IA</li>
            <li>2. Crie o agente (persona)</li>
            <li>3. Conecte o canal e escolha qual agente atende</li>
          </ol>
        </div>
        {canManage && <button className="primary-button" type="button" onClick={openCreate}>+ Criar canal</button>}
      </section>

      {error && <div className="channels-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
      {notice && <div className="channels-notice" role="status"><span>✓</span>{notice}<button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso">×</button></div>}

      <FilterBar resultLabel={`${total} canal${total === 1 ? '' : 'is'}`} onReset={() => { setQuery(''); setTypeFilter(''); setStatusFilter('') }}>
        <FilterSearch value={query} onChange={setQuery} placeholder="Buscar canal ou agente" />
        <FilterSelect value={typeFilter} onChange={setTypeFilter} label="Filtrar tipo" options={[{ value: '', label: 'Todos os tipos' }, { value: 'whatsapp', label: 'WhatsApp' }, { value: 'telegram', label: 'Telegram' }, { value: 'webchat', label: 'Webchat' }]} />
        <FilterSelect value={statusFilter} onChange={setStatusFilter} label="Filtrar status" options={[{ value: '', label: 'Todos os status' }, { value: 'ativos', label: 'Ativos' }, { value: 'inativos', label: 'Inativos' }]} />
      </FilterBar>
      {loading ? (
        <div className="channels-state"><span className="channels-spinner" /> Carregando canais…</div>
      ) : sortedChannels.length === 0 ? (
        <div className="channels-empty"><span className="channels-empty-icon">◈</span><strong>Nenhum canal conectado</strong><p>Comece pelo WhatsApp ou coloque o Webchat no seu site.</p>{canManage && <button className="primary-button" type="button" onClick={openCreate}>Criar primeiro canal</button>}</div>
      ) : (
        <div className="channels-grid">
          {sortedChannels.map((channel) => {
            const state = connection[channel.id]
            const widget = widgetFromChannel(channel)
            return (
              <article className="channel-card" key={channel.id}>
                <header className="channel-card-header">
                  <div className={`channel-icon channel-icon--${channel.type}`}>{channel.type === 'whatsapp' ? '◉' : channel.type === 'telegram' ? '✈' : '◈'}</div>
                  <div className="channel-card-title"><span>{channelLabel(channel.type)}</span><h3>{channel.name}</h3></div>
                  <span className={`channel-active-badge ${channel.active ? 'is-active' : ''}`}><i />{channel.active ? 'Ativo' : 'Inativo'}</span>
                </header>
                <div className="channel-card-body">
                  <div className="channel-agent-line">
                    <span>{channel.mode === 'chatflow' ? 'Chat Flow' : 'Agente que atende'}</span>
                    <strong>
                      {channel.mode === 'chatflow'
                        ? channel.chatflow_name || 'Fluxo não definido'
                        : channel.agent_name || 'Sem agente definido'}
                    </strong>
                  </div>
                  {channel.type === 'whatsapp' ? (
                    <>
                      <div className="channel-status-line"><span className={`channel-status-dot ${state?.connected && state.loggedIn ? 'is-online' : ''}`} /><strong>{statusText(state)}</strong><span>{state?.error || state?.jid || 'QR Code para conectar'}</span></div>
                      <ChannelHealthLine channel={channel} />
                      <div className="channel-feature-list"><span>{channel.respond_grupos ? 'Grupos liberados' : 'Grupos bloqueados'}</span><span>{channel.can_campaign ? 'Campanhas liberadas' : 'Campanhas bloqueadas'}</span><span>Infraestrutura gerenciada</span></div>
                    </>
                  ) : channel.type === 'telegram' ? (
                    <>
                      <div className="channel-status-line"><span className={`channel-status-dot ${state?.ready || state?.webhook_set ? 'is-online' : ''}`} /><strong>{telegramStatusText(channel, state)}</strong><span>{telegramStatusHint(channel, state)}</span></div>
                      <ChannelHealthLine channel={channel} />
                      <div className="channel-feature-list"><span>{channel.respond_grupos ? 'Grupos liberados' : 'Grupos bloqueados'}</span><span>Bot via API</span><span>Resposta por texto</span></div>
                      <label className="channel-install-label">URL do webhook (cole no BotFather)</label>
                      <code className="channel-install-code">{`${window.location.origin}/telegram/webhook/${channel.token_set ? 'SEU_TOKEN' : '…'}`}</code>
                    </>
                  ) : (
                    <>
                      <div className="channel-status-line"><span className="channel-status-dot is-online" /><strong>Widget pronto</strong><span>Personalização ativa</span></div>
                      <div className="channel-feature-list"><span>{widget.posicao === 'left' ? 'Botão à esquerda' : 'Botão à direita'}</span><span>{widget.largura}px de largura</span></div>
                      <label className="channel-install-label">Código do site</label>
                      <code className="channel-install-code">&lt;script src="{widgetScriptURL(channel.public_id || '')}" async&gt;&lt;/script&gt;</code>
                    </>
                  )}
                </div>
                {canManage && (
                  <footer className="channel-card-actions">
                    {channelActionAvailable(channel, state, 'connect') && <button className="channel-action channel-action--primary" type="button" onClick={() => void connectChannelAction(channel)} disabled={actionId === channel.id}>{actionId === channel.id ? 'Aguarde…' : channelConnectLabel(channel, state)}</button>}
                    {channelActionAvailable(channel, state, 'disconnect') && <button className="channel-action" type="button" onClick={() => void disconnectChannelAction(channel, 'desconectar')} disabled={actionId === channel.id}>Desconectar</button>}
                    {channel.type === 'whatsapp' && channelActionAvailable(channel, state, 'logout') && <button className="channel-action" type="button" onClick={() => void disconnectChannelAction(channel, 'logout')} disabled={actionId === channel.id}>Sair</button>}
                    {channelActionAvailable(channel, state, 'test') && <button className="channel-action" type="button" onClick={() => void testChannel(channel)} disabled={actionId === channel.id}>Testar</button>}
                    <button className="channel-action" type="button" onClick={() => openEdit(channel)}>Editar</button>
                    <button className="channel-action channel-action--danger" type="button" onClick={() => void deleteChannel(channel)} disabled={actionId === channel.id}>Excluir</button>
                  </footer>
                )}
              </article>
            )
          })}
        </div>
      )}

      {!loading && sortedChannels.length > 0 && <Pagination total={total} limit={DEFAULT_PAGE_SIZE} offset={offset} onOffsetChange={setOffset} loading={loading} />}

      {qr && <Modal open title="Conectar WhatsApp" onClose={() => setQr(null)} size="sm"><div className="qr-content">{qr.image ? <img src={qr.image} alt="QR Code para conectar o WhatsApp" /> : <div className="qr-placeholder">◉</div>}<p>{qr.message}</p><span className="modal-hint">No celular: WhatsApp → Aparelhos conectados → Conectar um aparelho.</span></div></Modal>}
    </div>
  )
}

export default ChannelsPage
