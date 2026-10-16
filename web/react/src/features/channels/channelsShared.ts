import type { ChannelConnectionState } from './channelActions'

export type WidgetConfig = {
  titulo: string
  subtitulo: string
  logo_url: string
  boas_vindas: string
  posicao: 'right' | 'left'
  apresentacao: 'botao' | 'barra'
  titulo_botao: string
  cor_primaria: string
  cor_botao: string
  cor_fundo: string
  cor_texto: string
  cor_texto_botao: string
  icone_botao: string
  largura: number
  texto_input: string
  texto_rodape: string
  dominios: string[]
}

export type Channel = {
  id: number
  type: 'whatsapp' | 'webchat' | 'telegram'
  name: string
  active: boolean
  respond_grupos: boolean
  responder_chamadas: boolean
  can_campaign: boolean
  widget?: Record<string, unknown>
  public_id?: string
  token_set?: boolean
  health_status?: string
  last_checked_at?: string
  last_error?: string
  agent_id?: number | null
  agent_name?: string
  mode: 'ai' | 'chatflow'
  chatflow_id?: number | null
  chatflow_name?: string
}

export type AgentSummary = {
  id: number
  name: string
}

export type ChatflowSummary = {
  id: number
  nome: string
  ativo: boolean
}

export type ChannelDraft = {
  type: Channel['type']
  name: string
  token: string
  active: boolean
  respond_grupos: boolean
  responder_chamadas: boolean
  can_campaign: boolean
  agent_id: number | null
  mode: 'ai' | 'chatflow'
  chatflow_id: number | null
  widget: WidgetConfig
}

export const defaultWidget: WidgetConfig = {
  titulo: 'Estamos aqui para ajudar',
  subtitulo: 'Envie sua mensagem',
  logo_url: '',
  boas_vindas: 'Olá! Como posso ajudar?',
  posicao: 'right',
  apresentacao: 'botao',
  titulo_botao: 'Atendimento',
  cor_primaria: '#22c55e',
  cor_botao: '#22c55e',
  cor_fundo: '#f4f6f8',
  cor_texto: '#ffffff',
  cor_texto_botao: '#ffffff',
  icone_botao: '💬',
  largura: 360,
  texto_input: 'Digite sua mensagem...',
  texto_rodape: 'Atendimento',
  dominios: [],
}

export const emptyDraft = (): ChannelDraft => ({
  type: 'whatsapp',
  name: '',
  token: '',
  active: true,
  respond_grupos: false,
  responder_chamadas: true,
  can_campaign: false,
  agent_id: null,
  mode: 'ai',
  chatflow_id: null,
  widget: { ...defaultWidget, dominios: [] },
})

const asText = (value: unknown, fallback: string): string => typeof value === 'string' ? value : fallback
const asColor = (value: unknown, fallback: string): string => /^#[0-9a-f]{6}$/i.test(asText(value, '')) ? asText(value, fallback) : fallback

export function widgetFromChannel(channel?: Channel): WidgetConfig {
  const source = channel?.widget ?? {}
  const width = typeof source.largura === 'number' ? source.largura : Number(source.largura)
  return {
    titulo: asText(source.titulo, defaultWidget.titulo),
    subtitulo: asText(source.subtitulo, defaultWidget.subtitulo),
    logo_url: asText(source.logo_url, defaultWidget.logo_url),
    boas_vindas: asText(source.boas_vindas, defaultWidget.boas_vindas),
    posicao: source.posicao === 'left' ? 'left' : 'right',
    apresentacao: source.apresentacao === 'barra' ? 'barra' : 'botao',
    titulo_botao: asText(source.titulo_botao, defaultWidget.titulo_botao),
    cor_primaria: asColor(source.cor_primaria, defaultWidget.cor_primaria),
    cor_botao: asColor(source.cor_botao, defaultWidget.cor_botao),
    cor_fundo: asColor(source.cor_fundo, defaultWidget.cor_fundo),
    cor_texto: asColor(source.cor_texto, defaultWidget.cor_texto),
    cor_texto_botao: asColor(source.cor_texto_botao, defaultWidget.cor_texto_botao),
    icone_botao: asText(source.icone_botao, defaultWidget.icone_botao),
    largura: Number.isFinite(width) && width >= 320 && width <= 520 ? width : defaultWidget.largura,
    texto_input: asText(source.texto_input, defaultWidget.texto_input),
    texto_rodape: asText(source.texto_rodape, defaultWidget.texto_rodape),
    dominios: Array.isArray(source.dominios) ? source.dominios.filter((domain): domain is string => typeof domain === 'string') : [],
  }
}

export function draftFromChannel(channel: Channel): ChannelDraft {
  return {
    type: channel.type,
    name: channel.name,
    token: '',
    active: channel.active,
    respond_grupos: channel.respond_grupos,
    responder_chamadas: channel.responder_chamadas,
    can_campaign: channel.can_campaign,
    agent_id: channel.agent_id ?? null,
    mode: channel.mode === 'chatflow' ? 'chatflow' : 'ai',
    chatflow_id: channel.chatflow_id ?? null,
    widget: widgetFromChannel(channel),
  }
}

export function channelLabel(type: string): string {
  return type === 'whatsapp' ? 'WhatsApp' : type === 'telegram' ? 'Telegram' : 'Webchat'
}

export function widgetScriptURL(publicID: string): string {
  return `${window.location.origin}/webchat/${encodeURIComponent(publicID)}/embed.js`
}

export function statusText(state?: ChannelConnectionState): string {
  if (!state) return 'Status não consultado'
  if (state.connected && state.loggedIn) return 'Conectado'
  if (state.connected) return 'Conectando'
  if (state.error) return 'Erro de conexão'
  return 'Desconectado'
}

export function telegramStatusText(channel: Channel, state?: ChannelConnectionState): string {
  if (!channel.token_set) return 'Configurar token'
  if (state?.ready || state?.webhook_set) return 'Webhook configurado'
  return 'Webhook pendente'
}

export function telegramStatusHint(channel: Channel, state?: ChannelConnectionState): string {
  if (!channel.token_set) return 'Informe o token do bot'
  if (state?.ready || state?.webhook_set) return 'Bot pronto para receber'
  return 'Configure na API do Telegram'
}

export type ChannelActionKey = 'connect' | 'disconnect' | 'logout' | 'test'

export function channelActionAvailable(channel: Channel, state: ChannelConnectionState | undefined, action: ChannelActionKey): boolean {
  if (channel.type === 'whatsapp') {
    const connected = Boolean(state?.connected)
    const loggedIn = Boolean(state?.connected && state?.loggedIn)
    if (action === 'connect') return !loggedIn
    if (action === 'disconnect' || action === 'logout') return connected
    if (action === 'test') return loggedIn
    return false
  }
  if (channel.type === 'telegram') {
    if (!channel.token_set) return false
    const ready = Boolean(state?.ready || state?.webhook_set)
    if (action === 'connect') return !ready
    if (action === 'disconnect' || action === 'logout') return ready
    if (action === 'test') return true
    return false
  }
  if (channel.type === 'webchat') return action === 'test'
  return false
}

export function channelConnectLabel(channel: Channel, state?: ChannelConnectionState): string {
  if (channel.type === 'telegram') return 'Conectar webhook'
  if (channel.type === 'whatsapp' && state?.connected && !state?.loggedIn) return 'Gerar novo QR'
  return 'Conectar'
}

export function channelTestFeedback(channel: Channel, state?: ChannelConnectionState): string {
  if (channel.type === 'whatsapp') {
    if (!state?.connected) return 'Canal desconectado: conecte o número antes de testar o envio.'
    if (!state?.loggedIn) return 'Conexão em andamento: conclua a leitura do QR Code para testar o envio.'
    return 'Canal conectado e pronto para enviar.'
  }
  if (channel.type === 'telegram') {
    return state?.ready || state?.webhook_set
      ? 'Webhook do bot configurado. O canal está pronto para receber mensagens.'
      : 'Webhook ainda não configurado. Use “Conectar webhook” para ativá-lo.'
  }
  return channel.active
    ? 'Widget validado. O canal está pronto para receber o script.'
    : 'Widget pausado: ative o canal para receber mensagens no site.'
}

export function channelHealthClass(status?: string): string {
  if (status === 'connected') return 'is-online'
  if (status === 'disconnected') return 'is-offline'
  return 'is-unknown'
}

export function channelHealthText(status?: string): string {
  if (status === 'connected') return 'Monitorado conectado'
  if (status === 'disconnected') return 'Desconectado'
  return 'Sem verificação'
}

export function channelHealthTime(value?: string): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString('pt-BR')
}
