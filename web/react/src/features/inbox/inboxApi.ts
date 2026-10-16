import {
  ActionResult,
  Contact,
  ContactInput,
  Conversation,
  ConversationDetail,
  ConversationListResult,
  ConversationStatus,
  InboxApi,
  InboxDetailParams,
  InboxListParams,
  MediaPayload,
  MediaUpload,
  Message,
  TransferTarget,
} from './types'

export interface InboxApiOptions {
  basePath?: string
  fetcher?: typeof fetch
}

export class InboxApiError extends Error {
  readonly status: number
  readonly payload: unknown

  constructor(status: number, message: string, payload: unknown) {
    super(message)
    this.name = 'InboxApiError'
    this.status = status
    this.payload = payload
  }
}

type JsonRecord = Record<string, unknown>

const asRecord = (value: unknown): JsonRecord =>
  value !== null && typeof value === 'object' ? (value as JsonRecord) : {}

const asString = (value: unknown): string =>
  typeof value === 'string' ? value : value === null || value === undefined ? '' : String(value)

const asNumber = (value: unknown): number | null => {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : null
  }
  return null
}

const asBoolean = (value: unknown): boolean => value === true || value === 'true' || value === 1

const firstArray = (...values: unknown[]): unknown[] => {
  for (const value of values) {
    if (Array.isArray(value)) return value
  }
  return []
}

const normalizeMessageReference = (value: unknown): Message['replyTo'] => {
  if (!value) return null
  const raw = asRecord(value)
  const id = asNumber(raw.id)
  if (!id) return null
  return {
    id,
    content: asString(raw.content ?? raw.text),
    mediaName: asString(raw.media_name ?? raw.mediaName) || undefined,
    source: asString(raw.source) || undefined,
  }
}

const normalizeMediaPayload = (payload: unknown): MediaPayload => {
  const root = asRecord(payload)
  const data = asRecord(root.data)
  const value = Object.keys(data).length ? data : root
  return {
    id: asNumber(value.id) ?? 0,
    mediaType: asString(value.media_type ?? value.mediaType),
    mediaB64: asString(value.media_b64 ?? value.mediaB64),
    mediaMime: asString(value.media_mime ?? value.mediaMime),
    mediaName: asString(value.media_name ?? value.mediaName),
  }
}

const normalizeActionResult = (payload: unknown): ActionResult => {
  const root = asRecord(payload)
  const data = asRecord(root.data)
  const value = Object.keys(data).length ? data : root
  return value as ActionResult
}

const normalizeConversationStatus = (value: string): string => {
  switch (value.trim().toLowerCase()) {
    case 'ai':
      return 'ia'
    case 'human':
    case 'aguardando':
      return 'humano_aguardando'
    case 'em_atendimento':
    case 'atendendo':
      return 'humano_atendendo'
    case 'encerrada':
    case 'encerrado':
      return 'closed'
    default:
      return value
  }
}

export const normalizeConversation = (value: unknown): Conversation => {
  const raw = asRecord(value)
  const status = asString(raw.status)
  const normalizedStatus = normalizeConversationStatus(status)
  return {
    id: asNumber(raw.id) ?? 0,
    hash: asString(raw.hash),
    customerName: asString(raw.customer_name ?? raw.customerName),
    customerPhone: asString(raw.customer_phone ?? raw.customerPhone),
    avatarUrl: asString(raw.avatar_url ?? raw.avatarUrl),
    channel: asString(raw.channel),
    origin: asString(raw.origem ?? raw.origin),
    status: normalizedStatus,
    typing: asBoolean(raw.typing),
    botBlocked: asBoolean(raw.bot_blocked ?? raw.botBlocked),
    agentId: asNumber(raw.atendente_id ?? raw.agent_id ?? raw.agentId),
    agentName: asString(raw.atendente_name ?? raw.agent_name ?? raw.agentName),
    sectorId: asNumber(raw.setor_id ?? raw.sector_id ?? raw.sectorId),
    sectorName: asString(raw.setor_name ?? raw.sector_name ?? raw.sectorName),
    sectorColor: asString(raw.setor_cor ?? raw.sector_color ?? raw.sectorColor),
    subject: asString(raw.subject ?? raw.motivo),
    aiSummary: asString(raw.ai_summary ?? raw.aiSummary),
    aiSummaryAt: asString(raw.ai_summary_at ?? raw.aiSummaryAt),
    updatedAt: asString(raw.updated_at ?? raw.updatedAt),
    createdAt: asString(raw.created_at ?? raw.createdAt),
  }
}

export const normalizeContact = (value: unknown): Contact => {
  const raw = asRecord(value)
  const extrasRaw = asRecord(raw.campos_extras ?? raw.camposExtras)
  const campos_extras: Record<string, string> = {}
  Object.keys(extrasRaw).forEach((key) => {
    campos_extras[key] = asString(extrasRaw[key])
  })
  return {
    id: asNumber(raw.id) ?? 0,
    name: asString(raw.name),
    phone: asString(raw.phone),
    email: asString(raw.email),
    notes: asString(raw.notes),
    tem_whatsapp: asBoolean(raw.tem_whatsapp ?? raw.temWhatsapp),
    tags: firstArray(raw.tags).map((tag) => asString(tag)).filter((tag) => tag.trim() !== ''),
    campos_extras,
    segment_ids: firstArray(raw.segment_ids ?? raw.segmentIds).map((id) => asNumber(id) ?? 0).filter((id) => id > 0),
    segments: firstArray(raw.segments).map((segment) => asString(segment)).filter((segment) => segment.trim() !== ''),
    created_at: asString(raw.created_at ?? raw.createdAt),
    updated_at: asString(raw.updated_at ?? raw.updatedAt),
  }
}

export const normalizeMessage = (value: unknown): Message => {
  const raw = asRecord(value)
  return {
    id: asNumber(raw.id) ?? 0,
    conversationId: asNumber(raw.conversation_id ?? raw.conversationId) ?? 0,
    role: asString(raw.role),
    content: asString(raw.content),
    source: asString(raw.source),
    waMessageId: asString(raw.wa_msg_id ?? raw.waMessageId),
    mediaType: asString(raw.media_type ?? raw.mediaType),
    mediaMime: asString(raw.media_mime ?? raw.mediaMime),
    mediaName: asString(raw.media_name ?? raw.mediaName),
    hasMedia: asBoolean(raw.has_media ?? raw.hasMedia),
    latitude: asNumber(raw.lat ?? raw.latitude),
    longitude: asNumber(raw.lng ?? raw.longitude),
    contactName: asString(raw.contact_name ?? raw.contactName),
    contactPhone: asString(raw.contact_tel ?? raw.contactPhone),
    createdAt: asString(raw.created_at ?? raw.createdAt),
    mediaB64: asString(raw.media_b64 ?? raw.mediaB64) || undefined,
    edited: asBoolean(raw.edited),
    replyToId: asNumber(raw.reply_to_id ?? raw.replyToId ?? raw.reply_to),
    replyTo: normalizeMessageReference(raw.reply ?? raw.reply_to_message ?? raw.replyTo),
    reactions: firstArray(raw.reactions).map((reaction) => {
      const item = asRecord(reaction)
      return {
        emoji: asString(item.emoji),
        count: asNumber(item.count) ?? undefined,
        reacted: asBoolean(item.reacted),
      }
    }).filter((reaction) => reaction.emoji),
  }
}

const normalizeListPayload = (payload: unknown): ConversationListResult => {
  const root = asRecord(payload)
  const rootData = root.data
  const data = asRecord(rootData)
  const source = Array.isArray(payload)
    ? payload
    : Array.isArray(rootData)
      ? rootData
      : firstArray(root.conversations, root.items, data.conversations, data.items)
  const meta = asRecord(root.meta)
  return {
    conversations: source.map(normalizeConversation).filter((conversation) => conversation.id > 0),
    incremental: root.incremental === true || data.incremental === true,
    cursor: asString(root.cursor ?? data.cursor ?? meta.next_cursor) || undefined,
  }
}

const normalizeDetailPayload = (payload: unknown): ConversationDetail => {
  const root = asRecord(payload)
  const data = asRecord(root.data)
  const rawConversation = root.conversation ?? data.conversation
  const rawMessages = firstArray(root.messages, data.messages)
  const rawContact = root.contact ?? data.contact
  return {
    conversation: rawConversation ? normalizeConversation(rawConversation) : null,
    messages: rawMessages.map(normalizeMessage).filter((message) => message.id > 0),
    contact: rawContact ? normalizeContact(rawContact) : null,
  }
}

const trimPath = (value: string): string => `/${value.replace(/^\/+|\/+$/g, '')}`

export const createInboxApi = (options: InboxApiOptions = {}): InboxApi => {
  const fetcher = options.fetcher ?? fetch
  const basePath = trimPath(options.basePath ?? '/api/react/conversations')

  const request = async <T>(path: string, init: RequestInit = {}): Promise<T> => {
    const headers = new Headers(init.headers)
    const isFormData = typeof FormData !== 'undefined' && init.body instanceof FormData
    if (init.body && !isFormData && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
    const response = await fetcher(path, {
      ...init,
      credentials: 'same-origin',
      headers,
    })
    const text = await response.text()
    let payload: unknown = null
    if (text.trim() !== '') {
      try {
        payload = JSON.parse(text)
      } catch {
        payload = text
      }
    }
    if (!response.ok) {
      const body = asRecord(payload)
      const nested = asRecord(body.error)
      const message = asString(nested.message ?? body.message ?? body.error) || `Erro na API (${response.status})`
      throw new InboxApiError(response.status, message, payload)
    }
    return payload as T
  }

  const action = async (path: string, body?: unknown): Promise<ActionResult> => {
    const payload = await request<unknown>(path, {
      method: 'POST',
      body: body === undefined ? undefined : JSON.stringify(body),
    })
    return normalizeActionResult(payload)
  }

  return {
    async listConversations(params: InboxListParams = {}) {
      const query = new URLSearchParams()
      if (params.status) query.set('status', params.status)
      query.set('limit', '100')
      const suffix = query.toString()
      const payload = await request<unknown>(`${basePath}${suffix ? `?${suffix}` : ''}`, {
        signal: params.signal,
      })
      return normalizeListPayload(payload)
    },

    async getConversation(id: number, params: InboxDetailParams = {}) {
      const query = new URLSearchParams()
      if (params.updatedAt) query.set('updated_at', params.updatedAt)
      const suffix = query.toString()
      const payload = await request<unknown>(`${basePath}/${encodeURIComponent(id)}${suffix ? `?${suffix}` : ''}`, {
        signal: params.signal,
      })
      return normalizeDetailPayload(payload)
    },

    sendMessage(id: number, content: string, replyTo?: number | null) {
      return action(`${basePath}/${encodeURIComponent(id)}/messages`, { content, ...(replyTo ? { reply_to: replyTo } : {}) })
    },

    async sendMedia(id: number, upload: MediaUpload) {
      const form = new FormData()
      form.append('file', upload.file)
      if (upload.caption?.trim()) form.append('caption', upload.caption.trim())
      if (upload.replyTo) form.append('reply_to', String(upload.replyTo))
      const payload = await request<unknown>(`${basePath}/${encodeURIComponent(id)}/messages`, {
        method: 'POST',
        body: form,
      })
      return normalizeActionResult(payload)
    },

    async editMessage(id: number, messageId: number, content: string) {
      const payload = await request<unknown>(`${basePath}/${encodeURIComponent(id)}/messages/${encodeURIComponent(messageId)}`, {
        method: 'PATCH',
        body: JSON.stringify({ content }),
      })
      return normalizeActionResult(payload)
    },

    setTyping(id: number, typing: boolean) {
      return action(`${basePath}/${encodeURIComponent(id)}/actions`, { action: 'typing', typing })
    },

    reactToMessage(id: number, messageId: number, emoji: string) {
      return action(`${basePath}/${encodeURIComponent(id)}/actions`, { action: 'reaction', msg_id: messageId, emoji })
    },

    async getMessageMedia(messageId: number) {
      const payload = await request<unknown>(`/api/react/messages/${encodeURIComponent(messageId)}/media`)
      return normalizeMediaPayload(payload)
    },

    async setStatus(id: number, status: ConversationStatus) {
      const payload = await request<unknown>(`${basePath}/${encodeURIComponent(id)}/status`, {
        method: 'PATCH',
        body: JSON.stringify({ status }),
      })
      return normalizeActionResult(payload)
    },

    setBotBlocked(id: number, blocked: boolean) {
      return action(`${basePath}/${encodeURIComponent(id)}/bot-block`, { blocked })
    },

    closeConversation(id: number, reason = 'Atendimento encerrado pelo operador') {
      return action(`${basePath}/${encodeURIComponent(id)}/actions`, { action: 'close', motivo: reason })
    },

    reopenConversation(id: number) {
      return action(`${basePath}/${encodeURIComponent(id)}/actions`, { action: 'reopen' })
    },

    acceptConversation(id: number) {
      return action(`${basePath}/${encodeURIComponent(id)}/actions`, { action: 'accept' })
    },

    transferConversation(id: number, target: TransferTarget) {
      const body = target.kind === 'sector' ? { action: 'transfer', setor_id: target.id } : { action: 'transfer', atendente_id: target.id }
      return action(`${basePath}/${encodeURIComponent(id)}/actions`, body)
    },

    returnConversationToAI(id: number) {
      return action(`${basePath}/${encodeURIComponent(id)}/actions`, { action: 'return_to_ai' })
    },

    async updateContact(contactId: number, input: ContactInput) {
      const payload = await request<unknown>(`/api/react/contacts/${encodeURIComponent(contactId)}`, {
        method: 'PATCH',
        body: JSON.stringify(input),
      })
      const root = asRecord(payload)
      const data = root.data ?? root
      return normalizeContact(data)
    },
  }
}
