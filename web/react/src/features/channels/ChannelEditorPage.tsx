import { useEffect, useState, type ReactNode } from 'react'
import { EntityScreen, type EntityTab } from '../../shared/EntityScreen'
import type { ChannelConnectionState } from './channelActions'
import {
  channelActionAvailable,
  channelConnectLabel,
  channelHealthClass,
  channelHealthText,
  channelHealthTime,
  channelLabel,
  statusText,
  telegramStatusHint,
  telegramStatusText,
  widgetScriptURL,
  type AgentSummary,
  type Channel,
  type ChannelDraft,
  type ChatflowSummary,
  type WidgetConfig,
} from './channelsShared'
import './Channels.css'

type ChannelEditorPageProps = {
  draft: ChannelDraft
  editing: boolean
  saving: boolean
  agents: AgentSummary[]
  chatflows: ChatflowSummary[]
  channel: Channel | null
  agentChannels: Record<number, { id: number; name: string }[]>
  connection?: ChannelConnectionState
  actionId: number | null
  canManage: boolean
  onBack: () => void
  onSave: () => void
  onChange: <K extends keyof ChannelDraft>(key: K, value: ChannelDraft[K]) => void
  onWidgetChange: <K extends keyof WidgetConfig>(key: K, value: WidgetConfig[K]) => void
  onConnect: () => void
  onDisconnect: (action: 'desconectar' | 'logout') => void
  onTest: () => void
}

type TabKey = 'conexao' | 'widget' | 'config'

const allTabs: EntityTab[] = [
  { key: 'conexao', label: 'Conexão', hint: 'Tipo, nome e modo de atendimento' },
  { key: 'widget', label: 'Widget', hint: 'Aparência no site' },
  { key: 'config', label: 'Configurações', hint: 'Grupos, chamadas e campanhas' },
]

export function ChannelEditorPage({
  draft,
  editing,
  saving,
  agents,
  chatflows,
  channel,
  agentChannels,
  connection,
  actionId,
  canManage,
  onBack,
  onSave,
  onChange,
  onWidgetChange,
  onConnect,
  onDisconnect,
  onTest,
}: ChannelEditorPageProps) {
  const [tab, setTab] = useState<TabKey>('conexao')
  const tabs = allTabs.filter((item) => item.key !== 'widget' || draft.type === 'webchat')

  useEffect(() => {
    if (tab === 'widget' && draft.type !== 'webchat') setTab('conexao')
  }, [draft.type, tab])

  const title = draft.name.trim() || (editing ? channelLabel(draft.type) : 'Novo canal')
  const busy = actionId !== null
  const otherChannelsForAgent =
    draft.agent_id !== null
      ? (agentChannels[draft.agent_id] ?? []).filter((item) => item.id !== channel?.id)
      : []
  const selectedFlow =
    draft.chatflow_id !== null
      ? chatflows.find((flow) => flow.id === draft.chatflow_id) ?? null
      : null

  return (
    <EntityScreen
      eyebrow={editing ? 'Editar canal' : 'Novo canal'}
      title={title}
      onBack={onBack}
      backLabel="Voltar para os canais"
      actions={
        <>
          <button className="secondary-button" type="button" onClick={onBack}>
            Voltar
          </button>
          <button className="primary-button" type="button" disabled={saving} onClick={onSave}>
            {saving ? 'Salvando…' : 'Salvar canal'}
          </button>
        </>
      }
      tabs={tabs}
      activeTab={tab}
      onTabChange={(key) => setTab(key as TabKey)}
    >
      {tab === 'conexao' && (
        <div className="entity-tab-body">
          <p className="entity-tab-lead">
            Defina o tipo do canal, o nome interno e o modo de atendimento (IA livre ou chat
            flow). A conexão com o número acontece depois de salvar.
          </p>
          <div className="entity-field-grid">
            <Field label="Tipo de canal" help="WhatsApp, Webchat para site ou Telegram. Não muda depois de criar.">
              <select
                value={draft.type}
                disabled={editing}
                onChange={(event) => onChange('type', event.target.value as Channel['type'])}
              >
                <option value="whatsapp">◉ WhatsApp</option>
                <option value="webchat">◈ Webchat para site</option>
                <option value="telegram">✈ Telegram</option>
              </select>
            </Field>
            <Field label="Nome interno" help="Como este canal aparece para a equipe no painel.">
              <input
                autoFocus
                value={draft.name}
                onChange={(event) => onChange('name', event.target.value)}
                placeholder="Ex.: WhatsApp Comercial"
              />
            </Field>
            <Field
              label="Modo de atendimento"
              help="Quem conduz o atendimento neste canal: a IA livre (persona do agente) ou um chat flow guiado."
            >
              <select
                value={draft.mode}
                onChange={(event) => onChange('mode', event.target.value as ChannelDraft['mode'])}
              >
                <option value="ai">IA (conversa livre)</option>
                <option value="chatflow">Chat Flow (fluxo guiado)</option>
              </select>
            </Field>
            {draft.mode === 'chatflow' ? (
              <>
                <Field
                  label="Chat Flow do canal"
                  help="Fluxo que conduz o atendimento. Se estiver inativo, o canal volta a responder com a IA livre."
                >
                  <select
                    value={draft.chatflow_id ?? ''}
                    onChange={(event) => onChange('chatflow_id', event.target.value ? Number(event.target.value) : null)}
                  >
                    <option value="">Selecione um chat flow</option>
                    {chatflows.map((flow) => (
                      <option key={flow.id} value={flow.id}>
                        {flow.nome}
                        {flow.ativo ? '' : ' · inativo'}
                      </option>
                    ))}
                  </select>
                  {selectedFlow && !selectedFlow.ativo && (
                    <small className="entity-help">Este fluxo está inativo: o atendimento cai na IA livre até você ativá-lo.</small>
                  )}
                  {!chatflows.length && (
                    <small className="entity-help">Nenhum chat flow cadastrado. Crie um na tela Chat Flow.</small>
                  )}
                </Field>
                <Field
                  label="Agente do bloco de IA (opcional)"
                  help="Persona usada pelos blocos de IA do fluxo e para a IA livre caso o fluxo esteja inativo."
                >
                  <select
                    value={draft.agent_id ?? ''}
                    onChange={(event) => onChange('agent_id', event.target.value ? Number(event.target.value) : null)}
                  >
                    <option value="">Sem agente definido</option>
                    {agents.map((agent) => (
                      <option key={agent.id} value={agent.id}>
                        {agent.name}
                      </option>
                    ))}
                  </select>
                </Field>
              </>
            ) : (
              <Field
                label="Agente que atende"
                help="Persona que responde neste canal com a IA livre."
              >
                <select
                  value={draft.agent_id ?? ''}
                  onChange={(event) => onChange('agent_id', event.target.value ? Number(event.target.value) : null)}
                >
                  <option value="">Sem agente definido</option>
                  {agents.map((agent) => (
                    <option key={agent.id} value={agent.id}>
                      {agent.name}
                    </option>
                  ))}
                </select>
              </Field>
            )}
          </div>

          {otherChannelsForAgent.length > 0 && (
            <div className="entity-cross-links" role="status">
              <span>
                Este agente também atende {otherChannelsForAgent.map((item) => item.name).join(', ')}.
                Tudo bem — um agente pode atender vários canais.
              </span>
            </div>
          )}

          {draft.type === 'whatsapp' && (
            <WhatsAppConnection
              channel={channel}
              connection={connection}
              busy={busy}
              canManage={canManage}
              onConnect={onConnect}
              onDisconnect={onDisconnect}
              onTest={onTest}
            />
          )}

          {draft.type === 'telegram' && (
            <section className="entity-section">
              <header>
                <div>
                  <strong>Bot do Telegram</strong>
                  <Help>Cole o token do bot criado no @BotFather. O webhook é configurado no teste.</Help>
                </div>
              </header>
              <Field label="Token do bot" wide help={editing ? 'Deixe vazio para manter o token atual.' : 'Obrigatório para conectar o bot.'}>
                <input
                  type="password"
                  value={draft.token}
                  onChange={(event) => onChange('token', event.target.value)}
                  placeholder="123456789:AAFlorenBW..."
                  autoComplete="new-password"
                />
              </Field>
              <TelegramStatus channel={channel} connection={connection} busy={busy} canManage={canManage} onConnect={onConnect} onDisconnect={onDisconnect} onTest={onTest} />
            </section>
          )}

          {draft.type === 'webchat' && (
            <section className="entity-section">
              <header>
                <div>
                  <strong>Widget do site</strong>
                  <Help>Copie o script e cole antes do fechamento da tag body do seu site.</Help>
                </div>
              </header>
              <div className="channel-status-line">
                <span className="channel-status-dot is-online" />
                <strong>Widget pronto</strong>
                <span>Personalização ativa</span>
              </div>
              {channel?.public_id ? (
                <>
                  <label className="channel-install-label">Código do site</label>
                  <code className="channel-install-code">&lt;script src="{widgetScriptURL(channel.public_id)}" async&gt;&lt;/script&gt;</code>
                </>
              ) : (
                <p className="entity-tab-empty">Salve o canal para gerar o código do widget.</p>
              )}
            </section>
          )}
        </div>
      )}

      {tab === 'widget' && draft.type === 'webchat' && (
        <div className="entity-tab-body">
          <p className="entity-tab-lead">
            A identidade visual e os textos do widget que aparece no seu site. As mudanças valem
            após salvar.
          </p>
          <div className="entity-field-grid">
            <Field label="Logo do widget (URL)" help="Imagem exibida no cabeçalho. Deixe vazio para usar só o título.">
              <input value={draft.widget.logo_url} onChange={(event) => onWidgetChange('logo_url', event.target.value)} placeholder="https://cdn.exemplo.com/logo.svg" />
            </Field>
            <Field label="Título do widget" help="Texto principal exibido no topo da conversa.">
              <input value={draft.widget.titulo} onChange={(event) => onWidgetChange('titulo', event.target.value)} placeholder="Estamos aqui para ajudar" />
            </Field>
            <Field label="Subtítulo" help="Linha de apoio abaixo do título.">
              <input value={draft.widget.subtitulo} onChange={(event) => onWidgetChange('subtitulo', event.target.value)} placeholder="Envie sua mensagem" />
            </Field>
            <Field label="Mensagem de boas-vindas" help="Primeira mensagem mostrada ao abrir o chat.">
              <input value={draft.widget.boas_vindas} onChange={(event) => onWidgetChange('boas_vindas', event.target.value)} placeholder="Olá! Como posso ajudar?" />
            </Field>
            <Field label="Posição" help="Lado da tela onde o botão do widget aparece.">
              <select value={draft.widget.posicao} onChange={(event) => onWidgetChange('posicao', event.target.value as WidgetConfig['posicao'])}>
                <option value="right">Direita</option>
                <option value="left">Esquerda</option>
              </select>
            </Field>
            <Field label="Apresentação" help="Botão redondo ou barra com texto.">
              <select value={draft.widget.apresentacao} onChange={(event) => onWidgetChange('apresentacao', event.target.value as WidgetConfig['apresentacao'])}>
                <option value="botao">Botão redondo</option>
                <option value="barra">Barra com texto</option>
              </select>
            </Field>
            <Field label="Título do botão" help="Texto exibido quando a apresentação é barra.">
              <input value={draft.widget.titulo_botao} onChange={(event) => onWidgetChange('titulo_botao', event.target.value)} placeholder="Atendimento" />
            </Field>
            <Field label="Ícone do botão" help="Emoji mostrado no botão do widget.">
              <input value={draft.widget.icone_botao} maxLength={8} onChange={(event) => onWidgetChange('icone_botao', event.target.value)} placeholder="💬" />
            </Field>
            <ColorField label="Cor principal" help="Cor de destaque do cabeçalho e botões." value={draft.widget.cor_primaria} onChange={(value) => onWidgetChange('cor_primaria', value)} />
            <ColorField label="Cor do botão" help="Cor do botão flutuante quando fechado." value={draft.widget.cor_botao} onChange={(value) => onWidgetChange('cor_botao', value)} />
            <ColorField label="Fundo das mensagens" help="Cor de fundo da área de conversa." value={draft.widget.cor_fundo} onChange={(value) => onWidgetChange('cor_fundo', value)} />
            <ColorField label="Texto do cabeçalho" help="Cor do texto no topo do widget." value={draft.widget.cor_texto} onChange={(value) => onWidgetChange('cor_texto', value)} />
            <ColorField label="Texto do botão" help="Cor do texto dentro dos botões." value={draft.widget.cor_texto_botao} onChange={(value) => onWidgetChange('cor_texto_botao', value)} />
            <Field label="Largura do chat (px)" help="Entre 320 e 520 pixels.">
              <input type="number" min={320} max={520} step={10} value={draft.widget.largura} onChange={(event) => onWidgetChange('largura', Math.min(520, Math.max(320, Number(event.target.value) || 360)))} />
            </Field>
            <Field label="Placeholder da mensagem" help="Texto de exemplo no campo de digitação.">
              <input value={draft.widget.texto_input} onChange={(event) => onWidgetChange('texto_input', event.target.value)} placeholder="Digite sua mensagem..." />
            </Field>
            <Field label="Texto do rodapé" help="Assinatura exibida na base do widget.">
              <input value={draft.widget.texto_rodape} onChange={(event) => onWidgetChange('texto_rodape', event.target.value)} placeholder="Atendimento" />
            </Field>
            <Field label="Domínios autorizados" wide help="Um domínio por linha. Vazio libera qualquer site.">
              <textarea
                rows={3}
                value={draft.widget.dominios.join('\n')}
                onChange={(event) => onWidgetChange('dominios', event.target.value.split('\n').map((item) => item.trim()).filter(Boolean))}
                placeholder="um domínio por linha · vazio = qualquer"
              />
            </Field>
          </div>
        </div>
      )}

      {tab === 'config' && (
        <div className="entity-tab-body">
          <p className="entity-tab-lead">Ajustes de operação do canal. Deixe pausado para parar de responder sem apagar a configuração.</p>
          <section className="entity-section">
            {draft.type !== 'webchat' && (
              <Toggle
                label="Responder mensagens em grupos"
                help="Quando ligado, a IA também responde em grupos do WhatsApp ou Telegram."
                checked={draft.respond_grupos}
                onChange={(value) => onChange('respond_grupos', value)}
              />
            )}
            {draft.type === 'whatsapp' && (
              <>
                <Toggle
                  label="Responder chamadas com mensagem"
                  help="Envia uma mensagem automática quando alguém liga para o número."
                  checked={draft.responder_chamadas}
                  onChange={(value) => onChange('responder_chamadas', value)}
                />
                <Toggle
                  label="Liberar este número para campanhas"
                  help="Permite usar este número como destino de campanhas em massa."
                  checked={draft.can_campaign}
                  onChange={(value) => onChange('can_campaign', value)}
                />
              </>
            )}
            <Toggle
              label="Canal ativo"
              help="Pausado deixa de responder sem excluir o canal."
              checked={draft.active}
              onChange={(value) => onChange('active', value)}
            />
          </section>
        </div>
      )}
    </EntityScreen>
  )
}

function WhatsAppConnection({
  channel,
  connection,
  busy,
  canManage,
  onConnect,
  onDisconnect,
  onTest,
}: {
  channel: Channel | null
  connection?: ChannelConnectionState
  busy: boolean
  canManage: boolean
  onConnect: () => void
  onDisconnect: (action: 'desconectar' | 'logout') => void
  onTest: () => void
}) {
  const healthAt = channelHealthTime(channel?.last_checked_at)
  return (
    <section className="entity-section">
      <header>
        <div>
          <strong>WhatsApp sem complicação</strong>
          <Help>URL de API e token são configurados automaticamente. Conecte pelo QR Code depois de salvar.</Help>
        </div>
      </header>
      <div className="channel-status-line">
        <span className={`channel-status-dot ${connection?.connected && connection.loggedIn ? 'is-online' : ''}`} />
        <strong>{statusText(connection)}</strong>
        <span>{connection?.error || connection?.jid || 'QR Code para conectar'}</span>
      </div>
      {channel && (
        <div className="channel-health-line">
          <span className={`channel-health-dot ${channelHealthClass(channel.health_status)}`} aria-hidden="true" />
          <strong>{channelHealthText(channel.health_status)}</strong>
          {healthAt && <span className="channel-health-checked">Verificado em {healthAt}</span>}
          {channel.last_error && <span className="channel-health-error">{channel.last_error}</span>}
        </div>
      )}
      {channel && canManage ? (
        <div className="channel-card-actions" style={{ padding: 0, border: 0, background: 'transparent' }}>
          {channelActionAvailable(channel, connection, 'connect') && (
            <button className="channel-action channel-action--primary" type="button" onClick={onConnect} disabled={busy}>
              {channelConnectLabel(channel, connection)}
            </button>
          )}
          {channelActionAvailable(channel, connection, 'disconnect') && (
            <button className="channel-action" type="button" onClick={() => onDisconnect('desconectar')} disabled={busy}>
              Desconectar
            </button>
          )}
          {channelActionAvailable(channel, connection, 'logout') && (
            <button className="channel-action" type="button" onClick={() => onDisconnect('logout')} disabled={busy}>
              Sair
            </button>
          )}
          {channelActionAvailable(channel, connection, 'test') && (
            <button className="channel-action" type="button" onClick={onTest} disabled={busy}>
              Testar
            </button>
          )}
        </div>
      ) : (
        <p className="entity-tab-empty">{channel ? 'Seu perfil não pode operar a conexão deste canal.' : 'Salve o canal para gerar o QR Code de conexão.'}</p>
      )}
    </section>
  )
}

function TelegramStatus({
  channel,
  connection,
  busy,
  canManage,
  onConnect,
  onDisconnect,
  onTest,
}: {
  channel: Channel | null
  connection?: ChannelConnectionState
  busy: boolean
  canManage: boolean
  onConnect: () => void
  onDisconnect: (action: 'desconectar' | 'logout') => void
  onTest: () => void
}) {
  return (
    <div className="entity-tab-body">
      <div className="channel-status-line">
        <span className={`channel-status-dot ${connection?.ready || connection?.webhook_set ? 'is-online' : ''}`} />
        <strong>{channel ? telegramStatusText(channel, connection) : 'Aguardando salvamento'}</strong>
        <span>{channel ? telegramStatusHint(channel, connection) : 'Salve para configurar o webhook'}</span>
      </div>
      {channel?.token_set && (
        <>
          <label className="channel-install-label">URL do webhook (cole no BotFather)</label>
          <code className="channel-install-code">{`${window.location.origin}/telegram/webhook/SEU_TOKEN`}</code>
        </>
      )}
      {channel && canManage ? (
        <div className="channel-card-actions" style={{ padding: 0, border: 0, background: 'transparent' }}>
          {channelActionAvailable(channel, connection, 'connect') && (
            <button className="channel-action channel-action--primary" type="button" onClick={onConnect} disabled={busy}>
              {channelConnectLabel(channel, connection)}
            </button>
          )}
          {channelActionAvailable(channel, connection, 'disconnect') && (
            <button className="channel-action" type="button" onClick={() => onDisconnect('desconectar')} disabled={busy}>
              Desconectar webhook
            </button>
          )}
          {channelActionAvailable(channel, connection, 'test') && (
            <button className="channel-action" type="button" onClick={onTest} disabled={busy}>
              Testar webhook
            </button>
          )}
        </div>
      ) : (
        <p className="entity-tab-empty">{channel ? 'Seu perfil não pode operar este canal.' : 'Salve o canal para configurar o webhook.'}</p>
      )}
    </div>
  )
}

function Help({ children }: { children: ReactNode }) {
  return <small className="entity-help">{children}</small>
}

function Field({ label, help, wide, children }: { label: string; help?: string; wide?: boolean; children: ReactNode }) {
  return (
    <label className={`entity-field${wide ? ' entity-field-wide' : ''}`}>
      <span>{label}</span>
      {children}
      {help && <Help>{help}</Help>}
    </label>
  )
}

function ColorField({ label, help, value, onChange }: { label: string; help?: string; value: string; onChange: (value: string) => void }) {
  return (
    <label className="channel-color-field">
      <span>{label}</span>
      <span className="color-input-wrap">
        <input type="color" value={value} onChange={(event) => onChange(event.target.value)} />
        <code>{value}</code>
      </span>
      {help && <Help>{help}</Help>}
    </label>
  )
}

function Toggle({ label, help, checked, onChange }: { label: string; help?: string; checked: boolean; onChange: (value: boolean) => void }) {
  return (
    <label className="channel-toggle">
      <span>
        <strong>{label}</strong>
        {help && <Help>{help}</Help>}
      </span>
      <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} />
      <i aria-hidden="true" />
    </label>
  )
}

export default ChannelEditorPage
