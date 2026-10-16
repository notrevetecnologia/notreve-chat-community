import { useCallback, useEffect, useRef, useState } from 'react'
import { normalizeConversation } from '../features/inbox'
import './NotificationCenter.css'

type WaitingItem = {
  id: number
  title: string
  subtitle: string
}

type FailureItem = {
  id: number
  title: string
  subtitle: string
}

export type NotificationCenterProps = {
  waitingCount: number
  permissions?: string[] | null
  modules?: string[] | null
  onOpenInbox: (conversationId?: number) => void
  onOpenLearning: () => void
}

type Envelope = {
  data?: unknown
}

const failureKindLabels: Record<string, string> = {
  llm: 'Falha de IA',
  tool: 'Falha de ferramenta',
  channel: 'Falha de canal',
  pipeline: 'Falha de pipeline',
  outros: 'Falha',
}

const hasAccess = (
  permissions: string[] | null,
  modules: string[] | null,
  module: string,
  permission: string,
): boolean =>
  (modules === null || modules.includes(module)) &&
  (permissions === null || permissions.includes(permission))

const truncate = (value: string, size: number): string => {
  const text = value.trim()
  return text.length > size ? `${text.slice(0, size - 1)}…` : text
}

const loadEnvelope = async (url: string): Promise<Envelope | null> => {
  try {
    const response = await fetch(url, { credentials: 'same-origin' })
    if (!response.ok) return null
    return await response.json() as Envelope
  } catch {
    return null
  }
}

function BellIcon() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9" />
      <path d="M13.73 21a2 2 0 0 1-3.46 0" />
    </svg>
  )
}

export function NotificationCenter({
  waitingCount,
  permissions = null,
  modules = null,
  onOpenInbox,
  onOpenLearning,
}: NotificationCenterProps) {
  const [open, setOpen] = useState(false)
  const [waiting, setWaiting] = useState<WaitingItem[]>([])
  const [failures, setFailures] = useState<FailureItem[]>([])
  const [openFailures, setOpenFailures] = useState(0)
  const [loading, setLoading] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)

  const canSeeWaiting = hasAccess(permissions, modules, 'conversas', 'conversas_ver')
  const canSeeFailures = hasAccess(permissions, modules, 'ia', 'aprendizado_ver')
  const enabled = canSeeWaiting || canSeeFailures
  const badgeCount = (canSeeWaiting ? waitingCount : 0) + (canSeeFailures ? openFailures : 0)

  useEffect(() => {
    if (!canSeeFailures) return
    let active = true
    const load = () => {
      void loadEnvelope('/api/react/agent-failures?limit=1').then((payload) => {
        if (!active) return
        const data = payload?.data as { open?: number } | undefined
        setOpenFailures(typeof data?.open === 'number' ? data.open : 0)
      })
    }
    load()
    const interval = window.setInterval(load, 30000)
    return () => {
      active = false
      window.clearInterval(interval)
    }
  }, [canSeeFailures])

  useEffect(() => {
    if (!open) return
    let active = true
    setLoading(true)
    const tasks: Array<Promise<void>> = []
    if (canSeeWaiting) {
      tasks.push(loadEnvelope('/api/react/conversations?status=humano_aguardando&limit=6').then((payload) => {
        if (!active || !Array.isArray(payload?.data)) return
        setWaiting(payload.data.map((raw) => {
          const conversation = normalizeConversation(raw)
          return {
            id: conversation.id,
            title: conversation.customerName.trim() || conversation.customerPhone || 'Visitante',
            subtitle: [conversation.customerPhone, conversation.channel].filter(Boolean).join(' · '),
          }
        }).filter((item) => item.id > 0))
      }))
    }
    if (canSeeFailures) {
      tasks.push(loadEnvelope('/api/react/agent-failures?limit=6').then((payload) => {
        if (!active) return
        const data = payload?.data as { items?: Array<{ id?: number; tipo?: string; phone?: string; mensagem_erro?: string; channel?: string }>; open?: number } | undefined
        if (!data) return
        setOpenFailures(typeof data.open === 'number' ? data.open : 0)
        setFailures((data.items ?? []).map((raw) => {
          const id = typeof raw.id === 'number' ? raw.id : 0
          return {
            id,
            title: failureKindLabels[raw.tipo ?? ''] ?? 'Falha do agente',
            subtitle: truncate([raw.phone, raw.channel, raw.mensagem_erro].filter(Boolean).join(' · '), 90),
          }
        }).filter((item) => item.id > 0))
      }))
    }
    void Promise.all(tasks).finally(() => {
      if (active) setLoading(false)
    })
    return () => { active = false }
  }, [canSeeFailures, canSeeWaiting, open])

  useEffect(() => {
    if (!open) return
    const handleClickOutside = (event: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(event.target as Node)) setOpen(false)
    }
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false)
    }
    window.addEventListener('mousedown', handleClickOutside)
    window.addEventListener('keydown', handleEscape)
    return () => {
      window.removeEventListener('mousedown', handleClickOutside)
      window.removeEventListener('keydown', handleEscape)
    }
  }, [open])

  const selectWaiting = useCallback((id: number) => {
    setOpen(false)
    onOpenInbox(id)
  }, [onOpenInbox])

  const selectFailure = useCallback(() => {
    setOpen(false)
    onOpenLearning()
  }, [onOpenLearning])

  if (!enabled) return null

  const hasItems = waiting.length > 0 || failures.length > 0

  return (
    <div className="notifications" ref={rootRef}>
      <button
        className="header-icon-button notification-button"
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={badgeCount > 0 ? `${badgeCount} notificações` : 'Notificações'}
        title="Notificações"
        onClick={() => setOpen((current) => !current)}
      >
        <BellIcon />
        {badgeCount > 0 && (
          <span className="notification-count" aria-hidden="true">
            {badgeCount > 99 ? '99+' : badgeCount}
          </span>
        )}
      </button>

      {open && (
        <div className="notifications-menu" role="menu" aria-label="Notificações recentes">
          <div className="notifications-head">
            <strong>Notificações</strong>
            <span>{badgeCount > 0 ? `${badgeCount} em aberto` : 'Tudo em dia'}</span>
          </div>

          {loading && !hasItems && (
            <p className="notifications-hint" role="status"><span className="notifications-spinner" /> Carregando…</p>
          )}

          {!loading && !hasItems && (
            <div className="notifications-empty">
              <span aria-hidden="true">✓</span>
              <strong>Nada pendente</strong>
              <small>Sem conversas aguardando ou falhas recentes.</small>
            </div>
          )}

          {canSeeWaiting && waiting.length > 0 && (
            <div className="notifications-group">
              <span className="notifications-group-title">Conversas aguardando</span>
              {waiting.map((item) => (
                <button key={`waiting-${item.id}`} type="button" className="notifications-item" role="menuitem" onClick={() => selectWaiting(item.id)}>
                  <span className="notifications-item-icon notifications-item-icon--waiting" aria-hidden="true">▣</span>
                  <span className="notifications-item-copy">
                    <strong>{item.title}</strong>
                    {item.subtitle && <small>{item.subtitle}</small>}
                  </span>
                </button>
              ))}
            </div>
          )}

          {canSeeFailures && failures.length > 0 && (
            <div className="notifications-group">
              <span className="notifications-group-title">Falhas do agente</span>
              {failures.map((item) => (
                <button key={`failure-${item.id}`} type="button" className="notifications-item" role="menuitem" onClick={selectFailure}>
                  <span className="notifications-item-icon notifications-item-icon--failure" aria-hidden="true">!</span>
                  <span className="notifications-item-copy">
                    <strong>{item.title}</strong>
                    {item.subtitle && <small>{item.subtitle}</small>}
                  </span>
                </button>
              ))}
            </div>
          )}

          <div className="notifications-footer">
            {canSeeWaiting && (
              <button type="button" onClick={() => { setOpen(false); onOpenInbox() }}>Abrir caixa de entrada</button>
            )}
            {canSeeFailures && (
              <button type="button" onClick={() => { setOpen(false); onOpenLearning() }}>Ver aprendizado</button>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

export default NotificationCenter
