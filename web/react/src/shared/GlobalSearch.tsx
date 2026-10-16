import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from 'react'
import { normalizeConversation } from '../features/inbox'
import './GlobalSearch.css'

type SearchKind = 'conversation' | 'contact' | 'agent'

type SearchEntry = {
  key: string
  kind: SearchKind
  id: number
  title: string
  subtitle: string
}

export type GlobalSearchProps = {
  open: boolean
  onClose: () => void
  permissions?: string[] | null
  modules?: string[] | null
  onOpenConversation: (id: number) => void
  onOpenContact: (query: string) => void
  onOpenAgent: (id: number) => void
}

type Envelope = {
  data?: unknown
  error?: { message?: string }
}

const groupLabels: Record<SearchKind, string> = {
  conversation: 'Conversas',
  contact: 'Contatos',
  agent: 'Agentes',
}

const statusLabels: Record<string, string> = {
  ia: 'IA atendendo',
  humano_aguardando: 'Aguardando',
  humano_atendendo: 'Atendimento humano',
  closed: 'Encerrada',
}

const hasAccess = (
  permissions: string[] | null,
  modules: string[] | null,
  module: string,
  permission: string,
): boolean =>
  (modules === null || modules.includes(module)) &&
  (permissions === null || permissions.includes(permission))

const fetchEnvelope = async (url: string, signal: AbortSignal): Promise<Envelope | null> => {
  try {
    const response = await fetch(url, { credentials: 'same-origin', signal })
    if (!response.ok) return null
    return await response.json() as Envelope
  } catch {
    return null
  }
}

const searchConversations = async (term: string, signal: AbortSignal): Promise<SearchEntry[]> => {
  const payload = await fetchEnvelope(`/api/react/conversations?limit=6&q=${encodeURIComponent(term)}`, signal)
  const items = Array.isArray(payload?.data) ? payload.data : []
  return items.map((raw) => {
    const conversation = normalizeConversation(raw)
    return {
      key: `conversation-${conversation.id}`,
      kind: 'conversation' as const,
      id: conversation.id,
      title: conversation.customerName.trim() || 'Visitante',
      subtitle: [conversation.customerPhone, conversation.channel, statusLabels[conversation.status] ?? conversation.status]
        .filter(Boolean)
        .join(' · '),
    }
  }).filter((entry) => entry.id > 0)
}

const searchContacts = async (term: string, signal: AbortSignal): Promise<SearchEntry[]> => {
  const payload = await fetchEnvelope(`/api/react/contacts?limit=6&q=${encodeURIComponent(term)}`, signal)
  const data = payload?.data as { items?: Array<{ id?: number; name?: string; phone?: string; email?: string }> } | undefined
  return (data?.items ?? []).map((raw) => {
    const id = typeof raw.id === 'number' ? raw.id : 0
    const name = (raw.name ?? '').trim()
    const phone = (raw.phone ?? '').trim()
    return {
      key: `contact-${id}`,
      kind: 'contact' as const,
      id,
      title: name || phone || 'Contato',
      subtitle: [phone, raw.email?.trim()].filter(Boolean).join(' · '),
    }
  }).filter((entry) => entry.id > 0)
}

const searchAgents = async (term: string, signal: AbortSignal): Promise<SearchEntry[]> => {
  const payload = await fetchEnvelope(`/api/react/agents?limit=6&q=${encodeURIComponent(term)}`, signal)
  const data = payload?.data as { items?: Array<{ id?: number; name?: string; provider_name?: string; active?: boolean }> } | undefined
  return (data?.items ?? []).map((raw) => {
    const id = typeof raw.id === 'number' ? raw.id : 0
    return {
      key: `agent-${id}`,
      kind: 'agent' as const,
      id,
      title: (raw.name ?? '').trim() || 'Agente',
      subtitle: [raw.provider_name?.trim(), raw.active ? 'Ativo' : 'Inativo'].filter(Boolean).join(' · '),
    }
  }).filter((entry) => entry.id > 0)
}

export function GlobalSearch({
  open,
  onClose,
  permissions = null,
  modules = null,
  onOpenConversation,
  onOpenContact,
  onOpenAgent,
}: GlobalSearchProps) {
  const [query, setQuery] = useState('')
  const [entries, setEntries] = useState<SearchEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [activeIndex, setActiveIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const abortRef = useRef<AbortController | null>(null)

  const canSearchConversations = hasAccess(permissions, modules, 'conversas', 'conversas_ver')
  const canSearchContacts = hasAccess(permissions, modules, 'contatos', 'contatos_gerenciar')
  const canSearchAgents = hasAccess(permissions, modules, 'ia', 'agentes_gerenciar')

  useEffect(() => {
    if (!open) return
    setQuery('')
    setEntries([])
    setActiveIndex(0)
    const timer = window.setTimeout(() => inputRef.current?.focus(), 30)
    return () => window.clearTimeout(timer)
  }, [open])

  useEffect(() => {
    if (!open) return
    const term = query.trim()
    abortRef.current?.abort()
    if (term.length < 2) {
      setEntries([])
      setLoading(false)
      return
    }
    const controller = new AbortController()
    abortRef.current = controller
    setLoading(true)
    const timer = window.setTimeout(() => {
      const tasks: Array<Promise<SearchEntry[]>> = []
      if (canSearchConversations) tasks.push(searchConversations(term, controller.signal))
      if (canSearchContacts) tasks.push(searchContacts(term, controller.signal))
      if (canSearchAgents) tasks.push(searchAgents(term, controller.signal))
      void Promise.all(tasks)
        .then((groups) => {
          if (controller.signal.aborted) return
          setEntries(groups.flat())
          setActiveIndex(0)
        })
        .finally(() => {
          if (!controller.signal.aborted) setLoading(false)
        })
    }, 220)
    return () => {
      window.clearTimeout(timer)
      controller.abort()
    }
  }, [canSearchAgents, canSearchContacts, canSearchConversations, open, query])

  const groups = useMemo(
    () => (['conversation', 'contact', 'agent'] as SearchKind[])
      .map((kind) => ({ kind, items: entries.filter((entry) => entry.kind === kind) }))
      .filter((group) => group.items.length > 0),
    [entries],
  )

  const select = useCallback((entry: SearchEntry) => {
    if (entry.kind === 'conversation') onOpenConversation(entry.id)
    else if (entry.kind === 'agent') onOpenAgent(entry.id)
    else onOpenContact(entry.title)
    onClose()
  }, [onClose, onOpenAgent, onOpenContact, onOpenConversation])

  const handleKeyDown = (event: ReactKeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Escape') {
      onClose()
      return
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      setActiveIndex((current) => Math.min(current + 1, Math.max(entries.length - 1, 0)))
      return
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      setActiveIndex((current) => Math.max(current - 1, 0))
      return
    }
    if (event.key === 'Enter') {
      event.preventDefault()
      const entry = entries[activeIndex]
      if (entry) select(entry)
    }
  }

  if (!open) return null

  const term = query.trim()
  const showHint = term.length < 2
  const showEmpty = !showHint && !loading && entries.length === 0

  return (
    <div
      className="gs-backdrop"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose()
      }}
    >
      <section className="gs-panel" role="dialog" aria-modal="true" aria-label="Busca global">
        <div className="gs-input-row">
          <span className="gs-input-icon" aria-hidden="true">⌕</span>
          <input
            ref={inputRef}
            className="gs-input"
            type="text"
            value={query}
            placeholder="Buscar conversas, contatos e agentes…"
            autoComplete="off"
            spellCheck={false}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={handleKeyDown}
          />
          <button className="gs-close" type="button" onClick={onClose} aria-label="Fechar busca">×</button>
        </div>

        <div className="gs-body">
          {showHint && (
            <p className="gs-hint">Digite ao menos 2 caracteres para buscar.</p>
          )}
          {loading && (
            <p className="gs-hint" role="status"><span className="gs-spinner" /> Buscando…</p>
          )}
          {showEmpty && (
            <div className="gs-empty" role="status">
              <span aria-hidden="true">⌕</span>
              <strong>Nenhum resultado</strong>
              <small>Não encontramos nada para “{term}”.</small>
            </div>
          )}
          {!showHint && groups.map((group) => (
            <div className="gs-group" key={group.kind}>
              <span className="gs-group-title">{groupLabels[group.kind]}</span>
              {group.items.map((entry) => {
                const index = entries.indexOf(entry)
                return (
                  <button
                    key={entry.key}
                    type="button"
                    className={index === activeIndex ? 'gs-item gs-item--active' : 'gs-item'}
                    onMouseEnter={() => setActiveIndex(index)}
                    onClick={() => select(entry)}
                  >
                    <span className={`gs-item-icon gs-item-icon--${entry.kind}`} aria-hidden="true">
                      {entry.kind === 'conversation' ? '▣' : entry.kind === 'contact' ? '◎' : '✦'}
                    </span>
                    <span className="gs-item-copy">
                      <strong>{entry.title}</strong>
                      {entry.subtitle && <small>{entry.subtitle}</small>}
                    </span>
                    <span className="gs-item-go" aria-hidden="true">↵</span>
                  </button>
                )
              })}
            </div>
          ))}
        </div>
      </section>
    </div>
  )
}

export default GlobalSearch
