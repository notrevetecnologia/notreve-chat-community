export type KnownConversationStatus =
  | 'ia'
  | 'humano_aguardando'
  | 'humano_atendendo'
  | 'closed'

export type ConversationStatus = KnownConversationStatus | (string & {})

export type InboxFilter = 'all' | 'ia' | 'waiting' | 'human' | 'closed'

export interface Conversation {
  id: number
  hash: string
  customerName: string
  customerPhone: string
  avatarUrl: string
  channel: string
  origin: string
  status: ConversationStatus
  typing: boolean
  botBlocked: boolean
  agentId: number | null
  agentName: string
  sectorId: number | null
  sectorName: string
  sectorColor: string
  subject: string
  aiSummary: string
  aiSummaryAt: string
  updatedAt: string
  createdAt: string
}

export interface Contact {
  id: number
  name: string
  phone: string
  email: string
  notes: string
  tem_whatsapp: boolean
  tags: string[]
  campos_extras: Record<string, string>
  segment_ids: number[]
  segments: string[]
  created_at: string
  updated_at: string
}

export interface ContactInput {
  name?: string
  phone?: string
  email?: string
  notes?: string
  tags?: string[]
  campos_extras?: Record<string, string>
}

export interface Message {
  id: number
  conversationId: number
  role: string
  content: string
  source: string
  waMessageId: string
  mediaType: string
  mediaMime: string
  mediaName: string
  hasMedia: boolean
  latitude: number | null
  longitude: number | null
  contactName: string
  contactPhone: string
  createdAt: string
  mediaB64?: string
  edited?: boolean
  replyToId?: number | null
  replyTo?: MessageReference | null
  reactions?: MessageReaction[]
}

export interface MessageReference {
  id: number
  content: string
  mediaName?: string
  source?: string
}

export interface MessageReaction {
  emoji: string
  count?: number
  reacted?: boolean
}

export interface MediaUpload {
  file: File
  caption?: string
  replyTo?: number | null
}

export interface MediaPayload {
  id: number
  mediaType: string
  mediaB64: string
  mediaMime: string
  mediaName: string
}

export interface ConversationDetail {
  conversation: Conversation | null
  messages: Message[]
  contact: Contact | null
}

export interface ConversationListResult {
  conversations: Conversation[]
  incremental: boolean
  cursor?: string
}

export interface InboxListParams {
  status?: string
  updatedAt?: string
  signal?: AbortSignal
}

export interface InboxDetailParams {
  updatedAt?: string
  signal?: AbortSignal
}

export interface TransferTarget {
  id: number
  kind: 'sector' | 'agent'
  label: string
  description?: string
}

export interface ActionResult {
  ok: boolean | string
  delivered?: boolean
  [key: string]: unknown
}

export interface InboxApi {
  listConversations(params?: InboxListParams): Promise<ConversationListResult>
  getConversation(id: number, params?: InboxDetailParams): Promise<ConversationDetail>
  sendMessage(id: number, content: string, replyTo?: number | null): Promise<ActionResult>
  sendMedia?(id: number, upload: MediaUpload): Promise<ActionResult>
  editMessage?(id: number, messageId: number, content: string): Promise<ActionResult>
  setTyping?(id: number, typing: boolean): Promise<ActionResult>
  reactToMessage?(id: number, messageId: number, emoji: string): Promise<ActionResult>
  getMessageMedia?(messageId: number): Promise<MediaPayload>
  setStatus(id: number, status: ConversationStatus): Promise<ActionResult>
  setBotBlocked?(id: number, blocked: boolean): Promise<ActionResult>
  closeConversation(id: number, reason?: string): Promise<ActionResult>
  reopenConversation(id: number): Promise<ActionResult>
  acceptConversation(id: number): Promise<ActionResult>
  transferConversation(id: number, target: TransferTarget): Promise<ActionResult>
  returnConversationToAI(id: number): Promise<ActionResult>
  updateContact?(contactId: number, input: ContactInput): Promise<Contact>
}
