import * as React from 'react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { createInboxApi, InboxApiError } from './inboxApi'
import {
  Contact,
  ContactInput,
  Conversation,
  ConversationDetail,
  ConversationStatus,
  InboxApi,
  InboxFilter,
  Message,
  TransferTarget,
} from './types'
import {
  notificationPermissionState,
  playNewMessageSound,
  requestNotificationPermission,
  showDesktopNotification,
} from '../../shared/notifications'
import './Inbox.css'

export interface InboxPageProps {
  api?: InboxApi
  className?: string
  initialConversationId?: number
  initialFilter?: InboxFilter
  pollEnabled?: boolean
  pollIntervalMs?: number
  transferTargets?: TransferTarget[]
  permissions?: string[] | null
  onUnreadChange?: (count: number | null) => void
}

const defaultInboxApi = createInboxApi()
const maxUploadBytes = 50 * 1024 * 1024

const filters: Array<{ value: InboxFilter; label: string }> = [
  { value: 'all', label: 'Todas' },
  { value: 'ia', label: 'IA' },
  { value: 'waiting', label: 'Aguardando' },
  { value: 'human', label: 'Atendendo' },
  { value: 'closed', label: 'Encerradas' },
]

const filterStatus: Record<InboxFilter, string | undefined> = {
  all: undefined,
  ia: 'ia',
  waiting: 'humano_aguardando',
  human: 'humano_atendendo',
  closed: 'closed',
}

const statusLabels: Record<string, string> = {
  ia: 'IA atendendo',
  humano_aguardando: 'Aguardando',
  humano_atendendo: 'Atendimento humano',
  closed: 'Encerrada',
}

const statusLabel = (status: ConversationStatus): string => (statusLabels[status] ?? status) || 'Sem status'

const transferOptionLabel = (target: TransferTarget, genericDescription: string): string => {
  const description = target.description?.trim()
  if (!description || description === genericDescription) return target.label
  return `${target.label} · ${description}`
}

const formatDateTime = (value: string): string => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const time = new Intl.DateTimeFormat('pt-BR', { hour: '2-digit', minute: '2-digit' }).format(date)
  const now = new Date()
  if (date.toDateString() === now.toDateString()) return time
  const day = new Intl.DateTimeFormat('pt-BR', { day: '2-digit', month: '2-digit' }).format(date)
  const year = date.getFullYear() !== now.getFullYear() ? `/${date.getFullYear()}` : ''
  return `${day}${year} ${time}`
}

const messageDayKey = (value: string): string => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

const daySeparatorLabel = (value: string): string => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const now = new Date()
  const key = messageDayKey(value)
  if (key === messageDayKey(now.toISOString())) return 'Hoje'
  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  if (key === messageDayKey(yesterday.toISOString())) return 'Ontem'
  const day = `${date.getDate()}`.padStart(2, '0')
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const year = date.getFullYear() !== now.getFullYear() ? `/${date.getFullYear()}` : ''
  return `${day}/${month}${year}`
}

const getConversationName = (conversation: Conversation): string => conversation.customerName.trim() || 'Visitante'

const getConversationPreview = (conversation: Conversation): string => {
  if (conversation.typing) return 'Digitando…'
  if (conversation.subject.trim()) return conversation.subject
  if (conversation.customerPhone.trim()) return conversation.customerPhone
  return 'Sem mensagens recentes'
}

type ContactExtra = { key: string; value: string }

type ContactDraft = {
  name: string
  email: string
  notes: string
  tagsText: string
  extras: ContactExtra[]
}

const contactToDraft = (contact: Contact): ContactDraft => ({
  name: contact.name,
  email: contact.email,
  notes: contact.notes,
  tagsText: contact.tags.join(', '),
  extras: Object.entries(contact.campos_extras).map(([key, value]) => ({ key, value })),
})

const draftToContactInput = (draft: ContactDraft): ContactInput => ({
  name: draft.name.trim(),
  email: draft.email.trim(),
  notes: draft.notes,
  tags: draft.tagsText.split(',').map((tag) => tag.trim()).filter((tag) => tag !== ''),
  campos_extras: draft.extras.reduce<Record<string, string>>((acc, item) => {
    const key = item.key.trim()
    if (key !== '') acc[key] = item.value.trim()
    return acc
  }, {}),
})

const getErrorMessage = (error: unknown): string => {
  if (error instanceof InboxApiError) return error.message
  if (error instanceof Error && error.message) return error.message
  return 'Não foi possível concluir esta ação.'
}

const isAbortError = (error: unknown): boolean =>
  typeof error === 'object' && error !== null && 'name' in error && error.name === 'AbortError'

const conversationSort = (left: Conversation, right: Conversation): number => {
  const leftTime = Date.parse(left.updatedAt) || 0
  const rightTime = Date.parse(right.updatedAt) || 0
  return rightTime - leftTime || right.id - left.id
}

const sameConversationList = (left: Conversation[], right: Conversation[]): boolean => {
  if (left.length !== right.length) return false
  return left.every((item, index) => {
    const other = right[index]
    return (
      item.id === other.id &&
      item.status === other.status &&
      item.updatedAt === other.updatedAt &&
      item.customerName === other.customerName &&
      item.typing === other.typing &&
      item.agentId === other.agentId &&
      item.sectorId === other.sectorId
    )
  })
}

const sameMessages = (left: Message[], right: Message[]): boolean => {
  if (left.length !== right.length) return false
  return left.every((message, index) => {
    const other = right[index]
    return message.id === other.id && message.content === other.content && message.createdAt === other.createdAt
  })
}

const latestUpdatedAt = (conversations: Conversation[]): string => {
  return conversations.reduce((latest, conversation) => {
    if (!latest) return conversation.updatedAt
    return Date.parse(conversation.updatedAt) > Date.parse(latest) ? conversation.updatedAt : latest
  }, '')
}

const mergeConversation = (base: Conversation | undefined, update: Conversation | null): Conversation | null => {
  if (!update) return base ?? null
  if (!base) return update
  return {
    ...base,
    ...update,
    customerName: update.customerName || base.customerName,
    customerPhone: update.customerPhone || base.customerPhone,
    avatarUrl: update.avatarUrl || base.avatarUrl,
    channel: update.channel || base.channel,
    origin: update.origin || base.origin,
    agentName: update.agentName || base.agentName,
    sectorName: update.sectorName || base.sectorName,
    sectorColor: update.sectorColor || base.sectorColor,
    subject: update.subject || base.subject,
    updatedAt: update.updatedAt || base.updatedAt,
    createdAt: update.createdAt || base.createdAt,
  }
}

const messageBubbleClass = (message: Message): string => {
  if (message.source === 'customer' || message.role === 'user') return 'inbox-message inbox-message--customer'
  if (message.source === 'system' || message.role === 'system') return 'inbox-message inbox-message--system'
  return 'inbox-message inbox-message--agent'
}

const messageLabel = (message: Message): string => {
  if (message.source === 'customer' || message.role === 'user') return 'Cliente'
  if (message.source === 'system' || message.role === 'system') return 'Sistema'
  if (message.source === 'ai') return 'IA'
  return 'Atendimento'
}

const emojiList = [
  '😀', '😃', '😄', '😁', '😆', '😅', '😂', '🤣', '🥲', '☺️', '😊', '😇', '🙂', '🙃', '😉', '😌',
  '😍', '🥰', '😘', '😗', '😙', '😚', '😋', '😛', '😝', '😜', '🤪', '🤨', '🧐', '🤓', '😎', '🤩',
  '🥳', '😏', '😒', '😞', '😔', '😟', '😕', '🙁', '☹️', '😣', '😖', '😫', '😩', '🥺', '😢', '😭',
  '😤', '😠', '😡', '🤬', '🤯', '😳', '🥵', '🥶', '😱', '😨', '😰', '😥', '😓', '🤗', '🤔', '🫡',
  '🤭', '🤫', '🤥', '😶', '🫢', '😐', '😑', '😬', '🙄', '😯', '😦', '😧', '😮', '😲', '🥱', '😴',
  '🤤', '😪', '😵', '🤐', '🤢', '🤮', '🤧', '😷', '🤒', '🤕', '🤑', '🤠', '😈', '👿', '👹', '👺',
  '🤡', '💩', '👻', '💀', '☠️', '👽', '👾', '🤖', '🎃', '😺', '😸', '😹', '😻', '😼', '😽', '🙀',
  '😿', '😾', '🙈', '🙉', '🙊', '💋', '💌', '💘', '💝', '💖', '💗', '💓', '💞', '💕', '💟', '❣️',
  '💔', '❤️', '🧡', '💛', '💚', '💙', '💜', '🤎', '🖤', '🤍', '💯', '💢', '💥', '💫', '💦', '💨',
  '🕳️', '💣', '💬', '👋', '🤚', '🖐️', '✋', '🖖', '👌', '🤏', '✌️', '🤞', '🤟', '🤘', '🤙', '👈',
  '👉', '👆', '👇', '☝️', '✊', '👊', '🤛', '🤜', '👏', '🙌', '👐', '🤲', '🙏', '✍️', '💅', '🤝',
  '💪', '🫶', '❤️‍🔥', '🔥', '✨', '⭐', '🌟', '💫', '🎉', '🎊', '🎈', '🎁', '🏆', '🚀', '✅', '❌',
  '⚠️', '❗', '❓', '‼️', '⁉️', '⭕', '🚫', '💡', '🔔', '📌', '📍', '📎', '🔗', '🔒', '🔑', '💰',
  '💳', '📦', '🛒', '📅', '⏰', '📞', '📱', '💻', '📧', '📝', '📄', '📷', '🎥', '🎵', '🎶', '☀️',
  '🌙', '☁️', '🌈', '🌎', '🍎', '🍕', '☕', '🍺', '⚽', '🏠', '🚗', '✈️', '🐶', '🐱', '🌸', '🌱',
]

const shortcutList = [
  { trigger: '/oi', label: 'Saudação', text: 'Olá! Como posso ajudar você hoje?' },
  { trigger: '/aguarde', label: 'Aguardar', text: 'Só um momento, por favor. Já vou verificar isso para você.' },
  { trigger: '/obrigado', label: 'Agradecimento', text: 'Obrigado pelo contato! Se precisar de algo mais, estou à disposição.' },
  { trigger: '/encerrar', label: 'Encerrar', text: 'Posso ajudar em mais alguma coisa antes de encerrarmos o atendimento?' },
]

const formatDuration = (seconds: number): string => `${Math.floor(seconds / 60).toString().padStart(2, '0')}:${(seconds % 60).toString().padStart(2, '0')}`

const messageMediaURL = (message: Message): string => `/api/react/messages/${message.id}/media`

const renderMedia = (message: Message): React.ReactNode => {
  const source = messageMediaURL(message)
  const type = message.mediaType || ''
  const name = message.mediaName || 'arquivo'
  if (type === 'image') {
    return <a className="inbox-media-link" href={source} target="_blank" rel="noreferrer"><img className="inbox-media-image" src={source} alt={name} loading="lazy" /></a>
  }
  if (type === 'video') {
    return <video className="inbox-media-video" controls preload="metadata" src={source} />
  }
  if (type === 'audio') {
    return <audio className="inbox-media-audio" controls preload="metadata" src={source} />
  }
  return <a className="inbox-media-document" href={source} download={name} target="_blank" rel="noreferrer"><span className="inbox-media-document-icon">▣</span><span><strong>{name}</strong><small>{message.mediaMime || 'arquivo'}</small></span><span aria-hidden="true">↓</span></a>
}

function LazyMedia({ message }: { message: Message }) {
  if (!message.hasMedia && !message.mediaB64) return null
  return <div className="inbox-message-media">{renderMedia(message)}</div>
}

export function InboxPage({
  api = defaultInboxApi,
  className = '',
  initialConversationId,
  initialFilter = 'all',
  pollEnabled = true,
  pollIntervalMs = 5000,
  transferTargets = [],
  permissions,
  onUnreadChange,
}: InboxPageProps) {
  const [filter, setFilter] = useState<InboxFilter>(initialFilter)
  const [search, setSearch] = useState('')
  const [conversations, setConversations] = useState<Conversation[]>([])
  const [selectedId, setSelectedId] = useState<number | null>(initialConversationId ?? null)
  const [detail, setDetail] = useState<ConversationDetail | null>(null)
  const [listLoading, setListLoading] = useState(true)
  const [detailLoading, setDetailLoading] = useState(false)
  const [listLoaded, setListLoaded] = useState(false)
  const [listError, setListError] = useState<string | null>(null)
  const [detailError, setDetailError] = useState<string | null>(null)
  const [reconnecting, setReconnecting] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const [actionPending, setActionPending] = useState<string | null>(null)
  const [transferValue, setTransferValue] = useState('')
  const [mobileDetailOpen, setMobileDetailOpen] = useState(selectedId !== null)
  const [contextOpen, setContextOpen] = useState(false)
  const [replyTo, setReplyTo] = useState<Message | null>(null)
  const [editingMessageId, setEditingMessageId] = useState<number | null>(null)
  const [reactionPickerId, setReactionPickerId] = useState<number | null>(null)
  const [composerEmojiOpen, setComposerEmojiOpen] = useState(false)
  const [shortcutOpen, setShortcutOpen] = useState(false)
  const [pendingFile, setPendingFile] = useState<File | null>(null)
  const [pendingFileUrl, setPendingFileUrl] = useState('')
  const [recording, setRecording] = useState(false)
  const [recordingSeconds, setRecordingSeconds] = useState(0)
  const [activityCount, setActivityCount] = useState(0)
  const [notificationState, setNotificationState] = useState<NotificationPermission | 'unsupported'>(() => notificationPermissionState())
  const [contactDraft, setContactDraft] = useState<ContactDraft | null>(null)
  const [contactSaving, setContactSaving] = useState(false)
  const [contactError, setContactError] = useState<string | null>(null)
  const [contactNotice, setContactNotice] = useState<string | null>(null)

  const conversationsRef = useRef(conversations)
  const detailRef = useRef(detail)
  const selectedIdRef = useRef(selectedId)
  const initialConversationRef = useRef(initialConversationId ?? null)
  const lastUpdatedAtRef = useRef('')
  const detailUpdatedAtRef = useRef('')
  const listRequestRef = useRef(0)
  const detailRequestRef = useRef(0)
  const listAbortRef = useRef<AbortController | null>(null)
  const detailAbortRef = useRef<AbortController | null>(null)
  const listScrollRef = useRef<HTMLDivElement | null>(null)
  const messageScrollRef = useRef<HTMLDivElement | null>(null)
  const messageInputRef = useRef<HTMLTextAreaElement | null>(null)
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const messageViewportRef = useRef({ stickToBottom: true, scrollTop: 0, scrollHeight: 0 })
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const recordingStreamRef = useRef<MediaStream | null>(null)
  const recordingChunksRef = useRef<Blob[]>([])
  const recordingTimerRef = useRef<number | null>(null)
  const discardRecordingRef = useRef(false)
  const typingTimerRef = useRef<number | null>(null)
  const typingSentRef = useRef(false)
  const pendingScrollRef = useRef(false)

  conversationsRef.current = conversations
  detailRef.current = detail
  selectedIdRef.current = selectedId

  const selectedConversation = useMemo(
    () => detail?.conversation?.id === selectedId
      ? detail.conversation
      : conversations.find((conversation) => conversation.id === selectedId) ?? null,
    [conversations, detail, selectedId],
  )

  const canRespond = permissions == null || permissions.includes('conversas_responder')
  const canClose = permissions == null || permissions.includes('conversas_encerrar')
  const canTransfer = permissions == null || permissions.includes('conversas_transferir')
  const canViewIA = permissions == null || permissions.includes('conversas_ver_ia')
  const canManageContacts = permissions == null || permissions.includes('contatos_gerenciar')
  const canReturnToAI = canClose
  const composerDisabled = !canRespond || selectedConversation?.status === 'closed'
  const contact = detail?.contact ?? null
  const canEditContact = canManageContacts && contact !== null && Boolean(api.updateContact)

  const visibleFilters = useMemo(
    () => (canViewIA ? filters : filters.filter((item) => item.value !== 'ia')),
    [canViewIA],
  )

  const transferGroups = useMemo(() => ({
    sectors: transferTargets.filter((target) => target.kind === 'sector'),
    agents: transferTargets.filter((target) => target.kind === 'agent'),
  }), [transferTargets])

  useEffect(() => {
    if (!canViewIA && filter === 'ia') setFilter('all')
  }, [canViewIA, filter])

  const visibleConversations = useMemo(() => {
    const normalizedSearch = search.trim().toLocaleLowerCase('pt-BR')
    if (!normalizedSearch) return conversations
    return conversations.filter((conversation) => {
      const searchable = [
        getConversationName(conversation),
        conversation.customerPhone,
        conversation.channel,
        conversation.origin,
        conversation.sectorName,
        conversation.agentName,
      ]
        .join(' ')
        .toLocaleLowerCase('pt-BR')
      return searchable.includes(normalizedSearch)
    })
  }, [conversations, search])

  const focusComposer = useCallback(() => {
    window.requestAnimationFrame(() => messageInputRef.current?.focus())
  }, [])

  const clearPendingFile = useCallback(() => {
    if (pendingFileUrl) URL.revokeObjectURL(pendingFileUrl)
    setPendingFile(null)
    setPendingFileUrl('')
  }, [pendingFileUrl])

  const setFileForUpload = useCallback((file: File) => {
    if (file.size > maxUploadBytes) {
      setActionError('Esse arquivo excede o limite de 50 MB.')
      return
    }
    setActionError(null)
    if (pendingFileUrl) URL.revokeObjectURL(pendingFileUrl)
    setPendingFile(file)
    setPendingFileUrl(URL.createObjectURL(file))
    setComposerEmojiOpen(false)
    setShortcutOpen(false)
    focusComposer()
  }, [focusComposer, pendingFileUrl])

  const sendTyping = useCallback(async (active: boolean) => {
    const id = selectedIdRef.current
    if (!id || !api.setTyping || (selectedConversation?.status === 'closed' && active)) return
    if (typingSentRef.current === active) return
    typingSentRef.current = active
    try {
      await api.setTyping(id, active)
    } catch {
      if (active) typingSentRef.current = false
    }
  }, [api, selectedConversation?.status])

  const scheduleTyping = useCallback((value: string) => {
    if (typingTimerRef.current !== null) window.clearTimeout(typingTimerRef.current)
    if (!value.trim()) {
      void sendTyping(false)
      return
    }
    void sendTyping(true)
    typingTimerRef.current = window.setTimeout(() => {
      void sendTyping(false)
    }, 1600)
  }, [sendTyping])

  const captureMessageViewport = useCallback(() => {
    const element = messageScrollRef.current
    if (!element) {
      messageViewportRef.current = { stickToBottom: true, scrollTop: 0, scrollHeight: 0 }
      return
    }
    const distanceToBottom = element.scrollHeight - element.clientHeight - element.scrollTop
    messageViewportRef.current = {
      stickToBottom: distanceToBottom < 48,
      scrollTop: element.scrollTop,
      scrollHeight: element.scrollHeight,
    }
  }, [])

  const restoreMessageViewport = useCallback(() => {
    const viewport = messageViewportRef.current
    const forceBottom = pendingScrollRef.current
    pendingScrollRef.current = false
    window.requestAnimationFrame(() => {
      const element = messageScrollRef.current
      if (!element) return
      if (viewport.stickToBottom || forceBottom) {
        element.scrollTop = element.scrollHeight
        return
      }
      element.scrollTop = viewport.scrollTop + element.scrollHeight - viewport.scrollHeight
    })
  }, [])

  const announceIncoming = useCallback((items: Conversation[]) => {
    if (items.length === 0) return
    setActivityCount((current) => current + items.length)
    playNewMessageSound()
    if (typeof document !== 'undefined' && document.hidden) {
      const first = items[0]
      showDesktopNotification('Nova mensagem', `${getConversationName(first)}: ${getConversationPreview(first)}`)
    }
  }, [])

  const syncList = useCallback(
    async (mode: 'initial' | 'poll') => {
      const requestId = ++listRequestRef.current
      listAbortRef.current?.abort()
      const controller = new AbortController()
      listAbortRef.current = controller
      if (mode === 'initial') setListLoading(true)
      try {
        const result = await api.listConversations({
          status: filterStatus[filter],
          updatedAt: mode === 'poll' ? lastUpdatedAtRef.current || undefined : undefined,
          signal: controller.signal,
        })
        if (requestId !== listRequestRef.current) return
        const incoming = [...result.conversations].sort(conversationSort)
        const next = mode === 'poll' && result.incremental
          ? Array.from(
              incoming.reduce((map, conversation) => {
                map.set(conversation.id, conversation)
                return map
              }, new Map(conversationsRef.current.map((conversation) => [conversation.id, conversation]))),
            ).map(([, conversation]) => conversation).sort(conversationSort)
          : incoming
        if (mode === 'poll') {
          const known = new Map(conversationsRef.current.map((conversation) => [conversation.id, conversation]))
          const novelties = next.filter((conversation) => {
            const before = known.get(conversation.id)
            return !before || before.updatedAt !== conversation.updatedAt
          })
          announceIncoming(novelties.filter((conversation) => conversation.id !== selectedIdRef.current))
        }
        const previousScrollTop = listScrollRef.current?.scrollTop ?? 0
        if (!sameConversationList(conversationsRef.current, next)) setConversations(next)
        lastUpdatedAtRef.current = latestUpdatedAt(next)
        setListError(null)
        setReconnecting(false)
        setListLoaded(true)
        if (listScrollRef.current) {
          window.requestAnimationFrame(() => {
            if (listScrollRef.current) listScrollRef.current.scrollTop = previousScrollTop
          })
        }
      } catch (error) {
        if (isAbortError(error) || requestId !== listRequestRef.current) return
        if (mode === 'initial') setListError(getErrorMessage(error))
        setReconnecting(true)
      } finally {
        if (requestId === listRequestRef.current && mode === 'initial') setListLoading(false)
      }
    },
    [announceIncoming, api, filter],
  )

  const loadDetail = useCallback(
    async (id: number, mode: 'initial' | 'poll' = 'initial') => {
      const requestId = ++detailRequestRef.current
      detailAbortRef.current?.abort()
      const controller = new AbortController()
      detailAbortRef.current = controller
      const base = conversationsRef.current.find((conversation) => conversation.id === id)
      if (mode === 'poll') {
        captureMessageViewport()
      } else {
        messageViewportRef.current = { stickToBottom: true, scrollTop: 0, scrollHeight: 0 }
      }
      if (mode === 'initial') setDetailLoading(true)
      try {
        const result = await api.getConversation(id, {
          updatedAt: mode === 'poll' ? detailUpdatedAtRef.current || undefined : undefined,
          signal: controller.signal,
        })
        if (requestId !== detailRequestRef.current || selectedIdRef.current !== id) return
        const conversation = mergeConversation(base, result.conversation)
        const previous = detailRef.current
        if (
          mode === 'poll' &&
          previous &&
          sameMessages(previous.messages, result.messages) &&
          previous.conversation?.updatedAt === conversation?.updatedAt
        ) {
          setReconnecting(false)
          return
        }
        if (mode === 'poll' && previous) {
          const known = new Set(previous.messages.map((message) => message.id))
          const incoming = result.messages.filter((message) => !known.has(message.id) && (message.source === 'customer' || message.role === 'user'))
          if (incoming.length > 0) {
            pendingScrollRef.current = true
            playNewMessageSound()
            if (typeof document !== 'undefined' && document.hidden) {
              const label = conversation ? getConversationName(conversation) : 'Conversa'
              const last = incoming[incoming.length - 1]
              showDesktopNotification('Nova mensagem', `${label}: ${last.content || 'Nova mensagem recebida'}`)
            }
          }
        }
        const nextDetail = { conversation, messages: result.messages, contact: result.contact }
        setDetail(nextDetail)
        detailUpdatedAtRef.current = conversation?.updatedAt || detailUpdatedAtRef.current
        setDetailError(null)
        setReconnecting(false)
        if (conversation) {
          setConversations((current) => {
            const next = current.map((item) => (item.id === conversation.id ? conversation : item)).sort(conversationSort)
            return sameConversationList(current, next) ? current : next
          })
        }
        restoreMessageViewport()
      } catch (error) {
        if (isAbortError(error) || requestId !== detailRequestRef.current) return
        if (mode === 'initial') setDetailError(getErrorMessage(error))
        setReconnecting(true)
      } finally {
        if (requestId === detailRequestRef.current && mode === 'initial') setDetailLoading(false)
      }
    },
    [api, captureMessageViewport, restoreMessageViewport],
  )

  useEffect(() => {
    lastUpdatedAtRef.current = ''
    void syncList('initial')
    return () => listAbortRef.current?.abort()
  }, [syncList])

  useEffect(() => {
    if (!listLoaded || listLoading) return
    if (selectedId === null) {
      const preferred = initialConversationRef.current
      const next = preferred !== null && conversations.some((conversation) => conversation.id === preferred)
        ? preferred
        : conversations[0]?.id ?? null
      initialConversationRef.current = null
      if (next !== null) setSelectedId(next)
      return
    }
    if (!conversations.some((conversation) => conversation.id === selectedId)) {
      setSelectedId(null)
      setDetail(null)
      setMobileDetailOpen(false)
    }
  }, [conversations, listLoaded, listLoading, selectedId])

  useEffect(() => {
    detailAbortRef.current?.abort()
    detailUpdatedAtRef.current = ''
    setActionError(null)
    setTransferValue('')
    setDraft('')
    setDetail(null)
    if (selectedId === null) {
      setMobileDetailOpen(false)
      return
    }
    setMobileDetailOpen(true)
    void loadDetail(selectedId)
    return () => detailAbortRef.current?.abort()
  }, [loadDetail, selectedId])

  useEffect(() => {
    if (!pollEnabled) return
    const interval = window.setInterval(() => {
      void syncList('poll')
      if (selectedIdRef.current !== null) void loadDetail(selectedIdRef.current, 'poll')
    }, Math.max(3000, pollIntervalMs))
    return () => window.clearInterval(interval)
  }, [loadDetail, pollEnabled, pollIntervalMs, syncList])

  const waitingCount = useMemo(
    () => conversations.filter((conversation) => conversation.status === 'humano_aguardando').length,
    [conversations],
  )

  useEffect(() => {
    if (filter !== 'all') return
    onUnreadChange?.(waitingCount)
  }, [filter, onUnreadChange, waitingCount])

  useEffect(() => () => { onUnreadChange?.(null) }, [onUnreadChange])

  useEffect(() => {
    return () => {
      listAbortRef.current?.abort()
      detailAbortRef.current?.abort()
      if (typingTimerRef.current !== null) window.clearTimeout(typingTimerRef.current)
      if (recordingTimerRef.current !== null) window.clearInterval(recordingTimerRef.current)
      mediaRecorderRef.current?.stop()
      recordingStreamRef.current?.getTracks().forEach((track) => track.stop())
      if (typingSentRef.current && selectedIdRef.current && api.setTyping) void api.setTyping(selectedIdRef.current, false)
    }
  }, [api])

  useEffect(() => {
    setReplyTo(null)
    setEditingMessageId(null)
    setReactionPickerId(null)
    setComposerEmojiOpen(false)
    setShortcutOpen(false)
    if (typingSentRef.current && selectedIdRef.current && api.setTyping) {
      typingSentRef.current = false
      void api.setTyping(selectedIdRef.current, false)
    }
    if (pendingFileUrl) URL.revokeObjectURL(pendingFileUrl)
    setPendingFile(null)
    setPendingFileUrl('')
  }, [selectedId])

  useEffect(() => {
    if (reactionPickerId === null && !composerEmojiOpen && !shortcutOpen) return

    const closeOpenPopovers = (event: PointerEvent) => {
      const target = event.target
      if (target instanceof Element && target.closest('[data-inbox-popover-trigger], .inbox-reaction-picker, .inbox-composer-emoji-picker, .inbox-shortcut-menu')) return
      setReactionPickerId(null)
      setComposerEmojiOpen(false)
      setShortcutOpen(false)
    }

    const closeWithEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      setReactionPickerId(null)
      setComposerEmojiOpen(false)
      setShortcutOpen(false)
    }

    document.addEventListener('pointerdown', closeOpenPopovers)
    document.addEventListener('keydown', closeWithEscape)
    return () => {
      document.removeEventListener('pointerdown', closeOpenPopovers)
      document.removeEventListener('keydown', closeWithEscape)
    }
  }, [composerEmojiOpen, reactionPickerId, shortcutOpen])

  useEffect(() => () => {
    if (pendingFileUrl) URL.revokeObjectURL(pendingFileUrl)
  }, [pendingFileUrl])

  const handleSelect = (id: number) => {
    setActivityCount(0)
    setSelectedId(id)
    setMobileDetailOpen(true)
  }

  const handleEnableNotifications = async () => {
    const permission = await requestNotificationPermission()
    setNotificationState(permission)
    if (permission === 'granted') playNewMessageSound()
  }

  const handleSend = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!selectedConversation || sending || selectedConversation.status === 'closed' || !canRespond) return
    const content = draft.trim()
    if (!pendingFile && !content) return
    setSending(true)
    setActionError(null)
    try {
      if (pendingFile) {
        if (!api.sendMedia) throw new Error('O envio de arquivos não está disponível neste momento.')
        await api.sendMedia(selectedConversation.id, { file: pendingFile, caption: content, replyTo: replyTo?.id })
        clearPendingFile()
      } else if (editingMessageId !== null) {
        if (!api.editMessage) throw new Error('A edição de mensagens não está disponível neste momento.')
        await api.editMessage(selectedConversation.id, editingMessageId, content)
      } else {
        await api.sendMessage(selectedConversation.id, content, replyTo?.id)
      }
      void sendTyping(false)
      setDraft('')
      setReplyTo(null)
      setEditingMessageId(null)
      setShortcutOpen(false)
      setComposerEmojiOpen(false)
      await loadDetail(selectedConversation.id)
      await syncList('poll')
    } catch (error) {
      setActionError(getErrorMessage(error))
    } finally {
      setSending(false)
    }
  }

  const handleFileInput = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (file) setFileForUpload(file)
  }

  const handleFileDrop = (event: React.DragEvent<HTMLElement>) => {
    event.preventDefault()
    if (selectedConversation?.status === 'closed') return
    const file = event.dataTransfer.files?.[0]
    if (file) setFileForUpload(file)
  }

  const startRecording = async () => {
    if (recording || selectedConversation?.status === 'closed') return
    if (!navigator.mediaDevices?.getUserMedia || typeof MediaRecorder === 'undefined') {
      setActionError('Seu navegador não oferece suporte à gravação de áudio.')
      return
    }
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const mime = ['audio/ogg;codecs=opus', 'audio/ogg', 'audio/webm;codecs=opus', 'audio/webm', 'audio/mp4'].find((candidate) => MediaRecorder.isTypeSupported(candidate))
      const recorder = mime ? new MediaRecorder(stream, { mimeType: mime }) : new MediaRecorder(stream)
      recordingStreamRef.current = stream
      recordingChunksRef.current = []
      discardRecordingRef.current = false
      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) recordingChunksRef.current.push(event.data)
      }
      recorder.onerror = () => {
        setActionError('Não foi possível gravar o áudio.')
        setRecording(false)
      }
      recorder.onstop = () => {
        const chunks = recordingChunksRef.current
        const wasDiscarded = discardRecordingRef.current
        const type = recorder.mimeType || mime || 'audio/webm'
        stream.getTracks().forEach((track) => track.stop())
        recordingStreamRef.current = null
        mediaRecorderRef.current = null
        if (recordingTimerRef.current !== null) window.clearInterval(recordingTimerRef.current)
        recordingTimerRef.current = null
        setRecording(false)
        if (!wasDiscarded && chunks.length) {
          const extension = type.includes('ogg') ? 'ogg' : type.includes('mp4') ? 'm4a' : 'webm'
          setFileForUpload(new File(chunks, `audio-${Date.now()}.${extension}`, { type }))
        }
      }
      mediaRecorderRef.current = recorder
      recorder.start()
      setRecordingSeconds(0)
      setRecording(true)
      recordingTimerRef.current = window.setInterval(() => setRecordingSeconds((current) => current + 1), 1000)
    } catch {
      setActionError('Não foi possível acessar o microfone. Verifique a permissão do navegador.')
    }
  }

  const stopRecording = () => {
    if (mediaRecorderRef.current?.state === 'recording') mediaRecorderRef.current.stop()
  }

  const cancelRecording = () => {
    if (!mediaRecorderRef.current) return
    discardRecordingRef.current = true
    if (mediaRecorderRef.current.state === 'recording') mediaRecorderRef.current.stop()
  }

  const handleReply = (message: Message) => {
    setEditingMessageId(null)
    setReplyTo(message)
    setReactionPickerId(null)
    focusComposer()
  }

  const handleEdit = (message: Message) => {
    setReplyTo(null)
    setEditingMessageId(message.id)
    setDraft(message.content)
    setReactionPickerId(null)
    focusComposer()
  }

  const handleReaction = (message: Message, emoji: string) => {
    setReactionPickerId(null)
    if (!selectedConversation || !api.reactToMessage) {
      setActionError('As reações não estão disponíveis neste momento.')
      return
    }
    void runAction(`reaction-${message.id}`, () => api.reactToMessage!(selectedConversation.id, message.id, emoji))
  }

  const insertEmoji = (emoji: string) => {
    setDraft((current) => `${current}${emoji}`)
    setComposerEmojiOpen(false)
    focusComposer()
  }

  const applyShortcut = (text: string) => {
    setDraft(text)
    setShortcutOpen(false)
    focusComposer()
  }

  const runAction = async (key: string, action: () => Promise<unknown>) => {
    if (!selectedConversation || actionPending) return
    setActionPending(key)
    setActionError(null)
    try {
      await action()
      await loadDetail(selectedConversation.id)
      await syncList('poll')
    } catch (error) {
      setActionError(getErrorMessage(error))
    } finally {
      setActionPending(null)
    }
  }

  const handleClose = () => {
    if (!selectedConversation || !window.confirm('Encerrar este atendimento?')) return
    void runAction('close', () => api.closeConversation(selectedConversation.id))
  }

  const handleReopen = () => {
    if (!selectedConversation) return
    void runAction('reopen', () => api.reopenConversation(selectedConversation.id))
  }

  const handleAccept = () => {
    if (!selectedConversation) return
    void runAction('accept', () => api.acceptConversation(selectedConversation.id))
  }

  const handleReturnToAI = () => {
    if (!selectedConversation || selectedConversation.status !== 'humano_atendendo') return
    if (!window.confirm('Devolver este atendimento para a IA? O próximo contato será processado pelo assistente virtual.')) return
    void runAction('return-to-ai', () => api.returnConversationToAI(selectedConversation.id))
  }

  const handleTransfer = () => {
    if (!selectedConversation) return
    const target = transferTargets.find((item) => `${item.kind}:${item.id}` === transferValue)
    if (!target) return
    void runAction('transfer', () => api.transferConversation(selectedConversation.id, target))
  }

  const openContactEditor = () => {
    if (!contact) return
    setContactError(null)
    setContactNotice(null)
    setContactDraft(contactToDraft(contact))
  }

  const closeContactEditor = () => {
    if (!contactSaving) setContactDraft(null)
  }

  const updateContactDraft = (patch: Partial<ContactDraft>) => {
    setContactDraft((current) => (current ? { ...current, ...patch } : current))
  }

  const addContactExtra = () => {
    setContactDraft((current) => (current ? { ...current, extras: [...current.extras, { key: '', value: '' }] } : current))
  }

  const updateContactExtra = (index: number, patch: Partial<ContactExtra>) => {
    setContactDraft((current) => (current ? { ...current, extras: current.extras.map((item, i) => (i === index ? { ...item, ...patch } : item)) } : current))
  }

  const removeContactExtra = (index: number) => {
    setContactDraft((current) => (current ? { ...current, extras: current.extras.filter((_, i) => i !== index) } : current))
  }

  const saveContact = async () => {
    if (!contact || !contactDraft || !api.updateContact) return
    setContactSaving(true)
    setContactError(null)
    try {
      const updated = await api.updateContact(contact.id, draftToContactInput(contactDraft))
      setDetail((current) => (current ? { ...current, contact: updated } : current))
      setContactDraft(null)
      setContactNotice('Contato atualizado.')
    } catch (error) {
      setContactError(getErrorMessage(error))
    } finally {
      setContactSaving(false)
    }
  }

  const wrapperClassName = ['inbox-page', mobileDetailOpen ? 'inbox-page--detail-open' : '', contextOpen ? 'inbox-page--context-open' : 'inbox-page--context-closed', className]
    .filter(Boolean)
    .join(' ')

  return (
    <main className={wrapperClassName} aria-label="Caixa de entrada">
      <section className="inbox-list-panel" aria-label="Lista de conversas">
        <header className="inbox-list-header">
          <div>
            <p className="inbox-eyebrow">Central de atendimento</p>
            <h1>Conversas</h1>
          </div>
          <div className="inbox-list-summary">
            {activityCount > 0 && (
              <span className="inbox-activity-badge" role="status" aria-live="polite">
                {activityCount} nova{activityCount > 1 ? 's' : ''}
              </span>
            )}
            <span className="inbox-count" aria-label={`${visibleConversations.length} conversas visíveis`}>
              {visibleConversations.length}
            </span>
            <span>na fila</span>
          </div>
        </header>

        {notificationState === 'default' && (
          <div className="inbox-notify-bar" role="status">
            <span className="inbox-notify-icon" aria-hidden="true">♢</span>
            <span className="inbox-notify-text">Receba um aviso quando chegar uma nova mensagem.</span>
            <button type="button" onClick={() => void handleEnableNotifications()}>Ativar avisos</button>
          </div>
        )}

        <div className="inbox-search-wrap">
          <label htmlFor="inbox-search" className="sr-only">Buscar conversa</label>
          <span className="inbox-search-icon" aria-hidden="true">⌕</span>
          <input
            id="inbox-search"
            className="inbox-search"
            type="search"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Buscar por nome, telefone ou canal"
            autoComplete="off"
          />
        </div>

        <nav className="inbox-filters" aria-label="Filtrar conversas">
          {visibleFilters.map((item) => (
            <button
              key={item.value}
              type="button"
              className={filter === item.value ? 'inbox-filter inbox-filter--active' : 'inbox-filter'}
              aria-pressed={filter === item.value}
              onClick={() => {
                setFilter(item.value)
                lastUpdatedAtRef.current = ''
              }}
            >
              {item.label}
            </button>
          ))}
        </nav>

        {reconnecting && (
          <div className="inbox-reconnect" role="status" aria-live="polite">
            <span className="inbox-pulse" />
            <span>Atualizando atendimentos</span>
          </div>
        )}

        {listError && (
          <div className="inbox-inline-error" role="alert">
            <span>{listError}</span>
            <button type="button" onClick={() => void syncList('initial')}>Tentar novamente</button>
          </div>
        )}

        <div ref={listScrollRef} className="inbox-conversation-list" tabIndex={0} aria-label="Conversas disponíveis">
          {listLoading && conversations.length === 0 ? (
            <div className="inbox-state inbox-state--loading" role="status">
              <span className="inbox-spinner" /> Carregando conversas…
            </div>
          ) : visibleConversations.length === 0 ? (
            <div className="inbox-state" role="status">
              <span className="inbox-state-icon" aria-hidden="true">⌕</span>
              <strong>{search ? 'Nenhuma conversa encontrada' : 'Nenhuma conversa por aqui'}</strong>
              <span>{search ? 'Tente outro termo de busca.' : 'Novos atendimentos aparecerão nesta lista.'}</span>
            </div>
          ) : (
            visibleConversations.map((conversation) => {
              const isSelected = conversation.id === selectedId
              return (
                <button
                  key={conversation.id}
                  type="button"
                  className={isSelected ? 'inbox-conversation inbox-conversation--selected' : 'inbox-conversation'}
                  aria-current={isSelected ? 'true' : undefined}
                  onClick={() => handleSelect(conversation.id)}
                >
                  <span className="inbox-avatar" aria-hidden="true">
                    {conversation.avatarUrl ? <img src={conversation.avatarUrl} alt="" /> : getConversationName(conversation).slice(0, 1).toUpperCase()}
                  </span>
                  <span className="inbox-conversation-body">
                    <span className="inbox-conversation-topline">
                      <strong>{getConversationName(conversation)}</strong>
                      <time dateTime={conversation.updatedAt}>{formatDateTime(conversation.updatedAt)}</time>
                    </span>
                    <span className="inbox-conversation-preview">{getConversationPreview(conversation)}</span>
                    <span className="inbox-conversation-meta">
                      <span className="inbox-channel-badge">
                        <span className="inbox-channel-mark" aria-hidden="true">{(conversation.channel || 'C').slice(0, 1).toUpperCase()}</span>
                        {conversation.channel || 'Canal não informado'}
                      </span>
                      <span className={`inbox-status inbox-status--${conversation.status}`}>
                        <span className="inbox-status-dot" /> {statusLabel(conversation.status)}
                      </span>
                    </span>
                    {conversation.origin && <span className="inbox-conversation-origin">{conversation.origin}</span>}
                  </span>
                </button>
              )
            })
          )}
        </div>
      </section>

      <section className="inbox-detail-panel" aria-label="Conversa selecionada">
        {!selectedConversation ? (
          <div className="inbox-state inbox-state--detail">
            <span className="inbox-state-icon" aria-hidden="true">◌</span>
            <strong>Selecione uma conversa</strong>
            <span>Escolha um atendimento para visualizar as mensagens.</span>
          </div>
        ) : (
          <>
            <header className="inbox-detail-header">
              <button type="button" className="inbox-back-button" onClick={() => setMobileDetailOpen(false)} aria-label="Voltar para a lista de conversas">
                ← <span>Conversas</span>
              </button>
              <div className="inbox-detail-header-main">
                <div className="inbox-detail-kicker">
                  <span>Atendimento</span>
                  <span className="inbox-detail-separator">/</span>
                  <strong>{selectedConversation.hash || `#${selectedConversation.id}`}</strong>
                </div>
                <div className="inbox-detail-identity">
                  <span className="inbox-avatar inbox-avatar--detail" aria-hidden="true">
                    {selectedConversation.avatarUrl ? <img src={selectedConversation.avatarUrl} alt="" /> : getConversationName(selectedConversation).slice(0, 1).toUpperCase()}
                  </span>
                  <div>
                    <h2>{getConversationName(selectedConversation)}</h2>
                    <p>
                      {selectedConversation.customerPhone || 'Contato sem telefone'}
                      <span className="inbox-detail-separator">·</span>
                      {selectedConversation.channel || 'Canal não informado'}
                    </p>
                  </div>
                </div>
              </div>
              <div className="inbox-detail-header-side">
                <button
                  type="button"
                  className="inbox-context-toggle"
                  aria-expanded={contextOpen}
                  aria-controls="inbox-context-panel"
                  onClick={() => setContextOpen((current) => !current)}
                >
                  <span aria-hidden="true">◎</span>
                  {contextOpen ? 'Fechar contexto' : 'Abrir contexto'}
                </button>
                <span className={`inbox-status inbox-status--large inbox-status--${selectedConversation.status}`}>
                  <span className="inbox-status-dot" /> {statusLabel(selectedConversation.status)}
                </span>
                {selectedConversation.botBlocked && <span className="inbox-bot-badge" title="Possível outro bot — mensagens estão sendo ignoradas">🤖 Bot suspeito</span>}
                <span className="inbox-last-seen">Atualizado {formatDateTime(selectedConversation.updatedAt)}</span>
              </div>
            </header>

            <div className="inbox-action-bar">
              <div className="inbox-action-group">
                {selectedConversation.status === 'closed' ? (
                  canClose && (
                    <button type="button" className="inbox-action inbox-action--primary" onClick={handleReopen} disabled={Boolean(actionPending)}>
                      <span aria-hidden="true">↗</span>
                      {actionPending === 'reopen' ? 'Reabrindo…' : 'Reabrir atendimento'}
                    </button>
                  )
                ) : (
                  <>
                    {canClose && (
                      <button type="button" className="inbox-action inbox-action--danger" onClick={handleClose} disabled={Boolean(actionPending)}>
                        <span aria-hidden="true">✓</span>
                        {actionPending === 'close' ? 'Encerrando…' : 'Encerrar atendimento'}
                      </button>
                    )}
                    {canRespond && (selectedConversation.status === 'ia' || selectedConversation.status === 'humano_aguardando') && (
                      <button type="button" className="inbox-action inbox-action--primary" onClick={handleAccept} disabled={Boolean(actionPending)}>
                        <span aria-hidden="true">＋</span>
                        {actionPending === 'accept' ? 'Assumindo…' : 'Assumir conversa'}
                      </button>
                    )}
                    {canReturnToAI && (selectedConversation.status === 'humano_atendendo' || selectedConversation.status === 'humano_aguardando') && (
                      <button type="button" className="inbox-action inbox-action--ai" onClick={handleReturnToAI} disabled={Boolean(actionPending)}>
                        <span aria-hidden="true">✦</span>
                        {actionPending === 'return-to-ai' ? 'Devolvendo…' : 'Devolver para IA'}
                      </button>
                    )}
                    {!canClose && !canRespond && !canTransfer && (
                      <span className="inbox-action-hint">Seu perfil não pode alterar este atendimento.</span>
                    )}
                  </>
                )}
                {selectedConversation.botBlocked && api.setBotBlocked && (
                  <button type="button" className="inbox-action" onClick={() => void runAction('unblock', () => api.setBotBlocked!(selectedConversation.id, false))} disabled={Boolean(actionPending)}>
                    <span aria-hidden="true">✓</span>
                    {actionPending === 'unblock' ? 'Desbloqueando…' : 'Desbloquear bot'}
                  </button>
                )}
              </div>
              {canTransfer && selectedConversation.status !== 'closed' && (
                <div className="inbox-transfer-control">
                  <span className="inbox-transfer-label" aria-hidden="true">↗</span>
                  <label htmlFor="inbox-transfer">Transferir atendimento</label>
                  <select id="inbox-transfer" value={transferValue} onChange={(event) => setTransferValue(event.target.value)} disabled={Boolean(actionPending)}>
                    <option value="">Selecione um destino</option>
                    {transferGroups.sectors.length > 0 && (
                      <optgroup label="Setores">
                        {transferGroups.sectors.map((target) => (
                          <option key={`${target.kind}:${target.id}`} value={`${target.kind}:${target.id}`}>
                            {transferOptionLabel(target, 'Setor de atendimento')}
                          </option>
                        ))}
                      </optgroup>
                    )}
                    {transferGroups.agents.length > 0 && (
                      <optgroup label="Atendentes">
                        {transferGroups.agents.map((target) => (
                          <option key={`${target.kind}:${target.id}`} value={`${target.kind}:${target.id}`}>
                            {transferOptionLabel(target, 'Atendente')}
                          </option>
                        ))}
                      </optgroup>
                    )}
                  </select>
                  <button type="button" className="inbox-action" onClick={handleTransfer} disabled={!transferValue || Boolean(actionPending)}>
                    {actionPending === 'transfer' ? 'Transferindo…' : 'Transferir'}
                  </button>
                </div>
              )}
            </div>

            {actionError && <div className="inbox-inline-error inbox-inline-error--detail" role="alert">{actionError}</div>}
            {detailError && (
              <div className="inbox-inline-error inbox-inline-error--detail" role="alert">
                <span>{detailError}</span>
                <button type="button" onClick={() => void loadDetail(selectedConversation.id)}>Tentar novamente</button>
              </div>
            )}

            {selectedConversation.aiSummary.trim() && (
              <aside className="inbox-ai-summary" role="note" aria-label="Resumo da IA">
                <div className="inbox-ai-summary-head">
                  <span className="inbox-ai-summary-icon" aria-hidden="true">✦</span>
                  <strong>Resumo da IA</strong>
                  {selectedConversation.aiSummaryAt && <time dateTime={selectedConversation.aiSummaryAt}>{formatDateTime(selectedConversation.aiSummaryAt)}</time>}
                </div>
                <p>{selectedConversation.aiSummary}</p>
              </aside>
            )}

            <div ref={messageScrollRef} className="inbox-messages" onScroll={captureMessageViewport} aria-live="polite">
              {detailLoading && !detail ? (
                <div className="inbox-state" role="status"><span className="inbox-spinner" /> Carregando mensagens…</div>
              ) : detail?.messages.length ? (
                <>
                  <div className="inbox-timeline-heading"><span>Histórico do atendimento</span></div>
                  {selectedConversation.typing && <div className="inbox-typing-indicator" role="status"><span className="inbox-typing-dots"><i /><i /><i /></span> Cliente está digitando…</div>}
                  {detail.messages.map((message, index) => {
                    const dayKey = messageDayKey(message.createdAt)
                    const previousDayKey = index > 0 ? messageDayKey(detail.messages[index - 1].createdAt) : ''
                    const showDaySeparator = Boolean(dayKey) && dayKey !== previousDayKey
                    return (
                      <React.Fragment key={message.id}>
                        {showDaySeparator && <div className="inbox-day-separator"><span>{daySeparatorLabel(message.createdAt)}</span></div>}
                        <article className={messageBubbleClass(message)}>
                          <div className="inbox-message-content">
                            <div className="inbox-message-heading">
                              <span>{messageLabel(message)}</span>
                              <time dateTime={message.createdAt}>{formatDateTime(message.createdAt)}</time>
                            </div>
                            {message.replyTo && <div className="inbox-message-quote"><span>↩</span><span><strong>Respondendo</strong><small>{message.replyTo.content || message.replyTo.mediaName || 'Mensagem com mídia'}</small></span></div>}
                            <LazyMedia message={message} />
                            {message.content && (!message.hasMedia || !message.content.startsWith('📎')) && <p>{message.content}</p>}
                            {!message.content && !message.hasMedia && <p>Mensagem sem texto</p>}
                            <footer>
                              <span>{message.source === 'ai' ? 'Assistente virtual' : message.source === 'customer' || message.role === 'user' ? 'Mensagem recebida' : 'Mensagem enviada'}{message.edited ? ' · editada' : ''}</span>
                              <span className="inbox-message-actions">
                                {(message.source === 'customer' || message.role === 'user') && <button type="button" className="inbox-message-action" onClick={() => handleReply(message)} title="Responder esta mensagem" aria-label="Responder esta mensagem">↩</button>}
                                {message.source !== 'customer' && message.role !== 'user' && message.waMessageId && <button type="button" className="inbox-message-action" onClick={() => handleEdit(message)} title="Editar mensagem" aria-label="Editar mensagem">✎</button>}
                                {message.waMessageId && <button type="button" className="inbox-message-action" data-inbox-popover-trigger="reaction" onClick={() => setReactionPickerId((current) => current === message.id ? null : message.id)} title="Reagir" aria-label="Reagir">☺</button>}
                              </span>
                            </footer>
                            {message.reactions && message.reactions.length > 0 && <div className="inbox-message-reactions">{message.reactions.map((reaction) => <span key={reaction.emoji} title={`${reaction.count ?? 1} reação(ões)`}>{reaction.emoji}{(reaction.count ?? 0) > 1 ? <small>{reaction.count}</small> : null}</span>)}</div>}
                          </div>
                          {reactionPickerId === message.id && <div className="inbox-reaction-picker" role="dialog" aria-label="Escolher reação">{emojiList.slice(0, 96).map((emoji) => <button key={emoji} type="button" onClick={() => handleReaction(message, emoji)}>{emoji}</button>)}</div>}
                        </article>
                      </React.Fragment>
                    )
                  })}
                </>
              ) : (
                <div className="inbox-state" role="status">
                  <span className="inbox-state-icon" aria-hidden="true">✦</span>
                  <strong>Sem mensagens ainda</strong>
                  <span>Envie a primeira mensagem para iniciar o atendimento.</span>
                </div>
              )}
            </div>

            <form className="inbox-composer" onSubmit={handleSend} onDragOver={(event) => event.preventDefault()} onDrop={handleFileDrop}>
              <div className="inbox-composer-shell">
                <div className="inbox-composer-topline">
                  <span className="inbox-composer-mode"><span className="inbox-composer-mode-dot" /> {editingMessageId !== null ? 'Editando mensagem' : pendingFile ? 'Arquivo pronto para enviar' : 'Resposta manual'}</span>
                  <span className="inbox-composer-hint">Enter envia · Shift + Enter quebra linha</span>
                </div>
                {replyTo && <div className="inbox-reply-preview"><span><strong>Respondendo a {messageLabel(replyTo)}</strong><small>{replyTo.content || replyTo.mediaName || 'Mensagem com mídia'}</small></span><button type="button" onClick={() => setReplyTo(null)} aria-label="Cancelar resposta">×</button></div>}
                {editingMessageId !== null && <div className="inbox-edit-preview"><span>Você está editando uma mensagem</span><button type="button" onClick={() => { setEditingMessageId(null); setDraft('') }} aria-label="Cancelar edição">Cancelar</button></div>}
                {pendingFile && <div className="inbox-upload-preview">
                  <div className="inbox-upload-preview-media">
                    {pendingFile.type.startsWith('image/') && pendingFileUrl ? <img src={pendingFileUrl} alt="Prévia do arquivo" /> : pendingFile.type.startsWith('video/') && pendingFileUrl ? <video src={pendingFileUrl} muted controls /> : pendingFile.type.startsWith('audio/') && pendingFileUrl ? <audio src={pendingFileUrl} controls /> : <span className="inbox-upload-file-icon">▣</span>}
                  </div>
                  <div className="inbox-upload-preview-info"><strong>{pendingFile.name}</strong><small>{(pendingFile.size / 1024 / 1024).toFixed(2)} MB · legenda opcional</small></div>
                  <button type="button" onClick={clearPendingFile} aria-label="Remover arquivo">×</button>
                </div>}
                <label htmlFor="inbox-message" className="sr-only">Mensagem</label>
                <textarea
                  ref={messageInputRef}
                  id="inbox-message"
                  value={draft}
                  onChange={(event) => {
                    const value = event.target.value
                    setDraft(value)
                    setShortcutOpen(value.startsWith('/') && !value.includes(' '))
                    scheduleTyping(value)
                  }}
                  placeholder={selectedConversation.status === 'closed' ? 'Reabra o atendimento para responder' : !canRespond ? 'Seu perfil não pode responder esta conversa' : 'Escreva uma mensagem para o cliente…'}
                  rows={2}
                  disabled={sending || recording || composerDisabled}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' && !event.shiftKey) {
                      event.preventDefault()
                      event.currentTarget.form?.requestSubmit()
                    }
                  }}
                />
                {shortcutOpen && <div className="inbox-shortcut-menu" role="listbox" aria-label="Atalhos de resposta">{shortcutList.filter((item) => item.trigger.startsWith(draft.trim().toLowerCase())).map((item) => <button key={item.trigger} type="button" onClick={() => applyShortcut(item.text)}><strong>{item.trigger}</strong><span>{item.label}</span><small>{item.text}</small></button>)}</div>}
                {recording && <div className="inbox-recording-state"><span className="inbox-recording-dot" /> Gravando áudio <strong>{formatDuration(recordingSeconds)}</strong><button type="button" onClick={cancelRecording}>Descartar</button></div>}
                <div className="inbox-composer-bottomline">
                  <div className="inbox-composer-tools">
                    <input ref={fileInputRef} type="file" hidden accept="image/*,video/*,audio/*,.pdf,.doc,.docx,.xls,.xlsx,.txt,.csv,.zip" onChange={handleFileInput} />
                    <button type="button" className="inbox-tool-button" onClick={() => fileInputRef.current?.click()} disabled={sending || recording || composerDisabled} title="Anexar arquivo" aria-label="Anexar arquivo">＋</button>
                    <button type="button" className={`inbox-tool-button ${composerEmojiOpen ? 'inbox-tool-button--active' : ''}`} data-inbox-popover-trigger="composer-emoji" onClick={() => { setComposerEmojiOpen((current) => !current); setShortcutOpen(false) }} disabled={sending || recording || composerDisabled} title="Adicionar emoji" aria-label="Adicionar emoji">☺</button>
                    <button type="button" className={`inbox-tool-button inbox-record-button ${recording ? 'inbox-record-button--active' : ''}`} onClick={recording ? stopRecording : startRecording} disabled={sending || composerDisabled} title={recording ? 'Parar gravação' : 'Gravar áudio'} aria-label={recording ? 'Parar gravação' : 'Gravar áudio'}>{recording ? '■' : '●'}</button>
                    {composerEmojiOpen && <div className="inbox-composer-emoji-picker" role="dialog" aria-label="Escolher emoji">{emojiList.map((emoji) => <button key={emoji} type="button" onClick={() => insertEmoji(emoji)}>{emoji}</button>)}</div>}
                  </div>
                  <span className="inbox-composer-counter">{draft.length} caracteres</span>
                  <button type="submit" className="inbox-send-button" disabled={(!draft.trim() && !pendingFile) || sending || recording || composerDisabled}>
                    {sending ? 'Enviando…' : <><span>{editingMessageId !== null ? 'Salvar edição' : pendingFile ? 'Enviar arquivo' : 'Enviar mensagem'}</span><span aria-hidden="true">↗</span></>}
                  </button>
                </div>
              </div>
            </form>
          </>
        )}
      </section>

      <aside id="inbox-context-panel" className="inbox-context-panel" aria-label="Contexto do atendimento" aria-hidden={!contextOpen}>
        {!selectedConversation ? (
          <div className="inbox-context-empty">
            <span className="inbox-context-empty-icon" aria-hidden="true">◎</span>
            <strong>Contexto do cliente</strong>
            <span>Selecione uma conversa para ver os detalhes do atendimento.</span>
          </div>
        ) : (
          <>
            <header className="inbox-context-header">
              <div>
                <p className="inbox-eyebrow">Visão rápida</p>
                <h2>Contexto</h2>
              </div>
              <span className="inbox-context-live"><span className="inbox-pulse" /> Ao vivo</span>
            </header>

            <div className="inbox-context-scroll">
              <section className="inbox-profile-card">
                <span className="inbox-avatar inbox-avatar--profile" aria-hidden="true">
                  {selectedConversation.avatarUrl ? <img src={selectedConversation.avatarUrl} alt="" /> : getConversationName(selectedConversation).slice(0, 1).toUpperCase()}
                </span>
                <h3>{getConversationName(selectedConversation)}</h3>
                <p>{selectedConversation.customerPhone || 'Contato sem telefone'}</p>
                {contact?.email && <p className="inbox-contact-email">{contact.email}</p>}
                <div className="inbox-profile-tags">
                  <span className="inbox-channel-badge"><span className="inbox-channel-mark" aria-hidden="true">{(selectedConversation.channel || 'C').slice(0, 1).toUpperCase()}</span>{selectedConversation.channel || 'Canal não informado'}</span>
                  {selectedConversation.origin && <span className="inbox-origin-tag">{selectedConversation.origin}</span>}
                  {contact?.tags.map((tag) => <span key={tag} className="inbox-contact-tag">{tag}</span>)}
                </div>
                {contact && Object.keys(contact.campos_extras).length > 0 && (
                  <dl className="inbox-contact-extras-view">
                    {Object.entries(contact.campos_extras).map(([key, value]) => (
                      <div key={key}><dt>{key}</dt><dd>{value}</dd></div>
                    ))}
                  </dl>
                )}
                {contact?.notes && <p className="inbox-contact-notes">{contact.notes}</p>}
                {canEditContact && !contactDraft && (
                  <button type="button" className="inbox-contact-edit" onClick={openContactEditor}>Editar contato</button>
                )}
                {contactNotice && <div className="inbox-contact-notice" role="status">{contactNotice}</div>}
                {contactError && <div className="inbox-contact-error" role="alert">{contactError}</div>}
                {contactDraft && (
                  <div className="inbox-contact-form">
                    <label><span>Nome</span><input value={contactDraft.name} onChange={(event) => updateContactDraft({ name: event.target.value })} placeholder="Nome do cliente" /></label>
                    <label><span>E-mail</span><input type="email" value={contactDraft.email} onChange={(event) => updateContactDraft({ email: event.target.value })} placeholder="cliente@empresa.com" /></label>
                    <label><span>Tags (separadas por vírgula)</span><input value={contactDraft.tagsText} onChange={(event) => updateContactDraft({ tagsText: event.target.value })} placeholder="vip, atacado" /></label>
                    <label><span>Observações</span><textarea rows={3} value={contactDraft.notes} onChange={(event) => updateContactDraft({ notes: event.target.value })} placeholder="Contexto importante para a equipe" /></label>
                    <div className="inbox-contact-extras">
                      <span className="inbox-contact-extras-label">Campos extras</span>
                      {contactDraft.extras.map((extra, index) => (
                        <div key={index} className="inbox-contact-extra-row">
                          <input value={extra.key} onChange={(event) => updateContactExtra(index, { key: event.target.value })} placeholder="chave" />
                          <input value={extra.value} onChange={(event) => updateContactExtra(index, { value: event.target.value })} placeholder="valor" />
                          <button type="button" onClick={() => removeContactExtra(index)} aria-label="Remover campo">×</button>
                        </div>
                      ))}
                      <button type="button" className="inbox-contact-extra-add" onClick={addContactExtra}>+ Adicionar campo</button>
                    </div>
                    <div className="inbox-contact-form-actions">
                      <button type="button" className="inbox-contact-cancel" onClick={closeContactEditor} disabled={contactSaving}>Cancelar</button>
                      <button type="button" className="inbox-contact-save" onClick={() => void saveContact()} disabled={contactSaving}>{contactSaving ? 'Salvando…' : 'Salvar'}</button>
                    </div>
                  </div>
                )}
              </section>

              <section className="inbox-context-card">
                <div className="inbox-context-card-heading"><span>Detalhes do ticket</span><span className="inbox-context-card-code">{selectedConversation.hash || `#${selectedConversation.id}`}</span></div>
                <dl className="inbox-context-details">
                  <div><dt>Status</dt><dd><span className={`inbox-status inbox-status--${selectedConversation.status}`}><span className="inbox-status-dot" /> {statusLabel(selectedConversation.status)}</span></dd></div>
                  <div><dt>Responsável</dt><dd>{selectedConversation.agentName || (selectedConversation.status === 'ia' ? 'Assistente IA' : 'Ainda não atribuído')}</dd></div>
                  <div><dt>Setor</dt><dd>{selectedConversation.sectorName || 'Sem setor definido'}</dd></div>
                  <div><dt>Última atividade</dt><dd>{formatDateTime(selectedConversation.updatedAt)}</dd></div>
                </dl>
              </section>

              <section className="inbox-insight-card">
                <span className="inbox-insight-icon" aria-hidden="true">✦</span>
                <div>
                  <strong>{selectedConversation.status === 'ia' ? 'IA conduzindo o atendimento' : selectedConversation.status === 'closed' ? 'Atendimento encerrado' : 'Atendimento em andamento'}</strong>
                  <p>{selectedConversation.status === 'ia' ? 'O assistente virtual está acompanhando esta conversa.' : selectedConversation.status === 'closed' ? 'Este ticket não aceita novas mensagens até ser reaberto.' : 'A equipe pode acompanhar e responder este cliente em tempo real.'}</p>
                </div>
              </section>
            </div>
          </>
        )}
      </aside>
    </main>
  )
}

export default InboxPage
