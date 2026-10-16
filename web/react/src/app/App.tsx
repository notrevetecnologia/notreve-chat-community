import { useEffect, useMemo, useRef, useState } from 'react'
import { ChannelsPage } from '../features/channels/ChannelsPage'
import { ContactsPage } from '../features/contacts/ContactsPage'
import { AgentsPage } from '../features/agents/AgentsPage'
import { AgentEditorPage } from '../features/agents/AgentEditorPage'
import { ChatflowPage } from '../features/chatflow/ChatflowPage'
import { LoginPage, SetupPage } from '../features/auth'
import { CampaignsPage } from '../features/campaigns/CampaignsPage'
import { InboxPage, type TransferTarget } from '../features/inbox'
import { IntegrationsPage } from '../features/integrations'
import LearningPage from '../features/learning/LearningPage'
import KnowledgePage from '../features/knowledge/KnowledgePage'
import SkillsPage from '../features/skills/SkillsPage'
import SchedulesPage from '../features/schedules/SchedulesPage'
import LogsPage from '../features/logs/LogsPage'
import MessageLibraryPage from '../features/message-library/MessageLibraryPage'
import PlatformPage from '../features/platform/PlatformPage'
import { PlatformSetup } from '../features/platform/PlatformSetup'
import PlansPage from '../features/plans/PlansPage'
import { ProvidersPage } from '../features/providers'
import { ReportsPage } from '../features/reports/ReportsPage'
import SettingsPage from '../features/settings/SettingsPage'
import { TeamPage } from '../features/team/TeamPage'
import { LgpdPage } from '../features/lgpd/LgpdPage'
import { WizardPage } from '../features/wizard'
import { AppShell } from './app-shell'
import { ErrorBoundary } from '../shared/ErrorBoundary'
import { applyBrandingTheme, brandingDocumentTitle, type Branding } from '../branding'
import { playNewMessageSound, showDesktopNotification, startTitleAlert, stopTitleAlert } from '../shared/notifications'

type SessionIdentity = {
  id?: number
  name: string
  role: string
}

type BrandPresentation = {
  name: string
  product: string
  eyebrow: string
}

type OperationalSummary = {
  active: number | null
  waiting: number | null
  ai: number | null
  channels: number | null
}

type ConversationSummaryItem = {
  status?: string
  channel?: string
  origin?: string
}

type ConversationSummaryResponse = {
  data?: ConversationSummaryItem[] | { summary?: Partial<OperationalSummary> }
}

type Surface = 'overview' | 'inbox' | 'channels' | 'contacts' | 'agents' | 'agent' | 'providers' | 'integrations' | 'team' | 'campaigns' | 'reports' | 'learning' | 'knowledge' | 'library' | 'empresas' | 'servidores' | 'marca' | 'plans' | 'settings' | 'setup' | 'login' | 'chatflows' | 'lgpd' | 'wizard' | 'skills' | 'schedules' | 'logs' | 'platformSetup'

const surfaceNavigation: Record<Surface, string> = {
  overview: 'conversas', inbox: 'inbox', channels: 'canais', contacts: 'contatos', agents: 'agentes', agent: 'agentes',
  providers: 'ias', integrations: 'integracoes', team: 'equipe', campaigns: 'campanhas', reports: 'relatorios', learning: 'aprendizado', library: 'biblioteca', empresas: 'empresas', servidores: 'servidores', marca: 'marca', plans: 'planos',
  settings: 'configuracoes', knowledge: 'conhecimento', setup: 'configuracoes', login: 'login', chatflows: 'chatflows', lgpd: 'lgpd', wizard: 'wizard', skills: 'skills', schedules: 'agendamentos', logs: 'logs', platformSetup: 'primeiros-passos',
}

function readSurface(): Surface {
  const value = new URLSearchParams(window.location.search).get('view') as Surface | null
  return value && Object.prototype.hasOwnProperty.call(surfaceNavigation, value) ? value : 'overview'
}

function readAgentId(): number | null {
  const raw = new URLSearchParams(window.location.search).get('id')
  const value = raw ? Number(raw) : NaN
  return Number.isFinite(value) && value > 0 ? value : null
}

function readKnowledgeAgentId(): number | null {
  const raw = new URLSearchParams(window.location.search).get('agent_id')
  const value = raw ? Number(raw) : NaN
  return Number.isFinite(value) && value > 0 ? value : null
}

function readChatflowId(): number | null {
  const raw = new URLSearchParams(window.location.search).get('chatflow')
  const value = raw ? Number(raw) : NaN
  return Number.isFinite(value) && value > 0 ? value : null
}

function readInboxConversationId(): number | null {
  const raw = new URLSearchParams(window.location.search).get('conversation')
  const value = raw ? Number(raw) : NaN
  return Number.isFinite(value) && value > 0 ? value : null
}

function readCampaignTab(): 'campaigns' | 'segments' | 'groups' {
  const tab = new URLSearchParams(window.location.search).get('tab')
  return tab === 'segments' || tab === 'groups' ? tab : 'campaigns'
}

function readLibraryTab(): 'quick-replies' | 'templates' {
  return new URLSearchParams(window.location.search).get('tab') === 'templates' ? 'templates' : 'quick-replies'
}

const emptySummary: OperationalSummary = {
  active: null,
  waiting: null,
  ai: null,
  channels: null,
}

export default function App() {
  if (window.location.pathname === '/react/auth' || window.location.pathname === '/auth' || window.location.pathname === '/auth/') {
    return <LoginPage />
  }
  return <AppContent />
}

function AppContent() {
  const [identity, setIdentity] = useState<SessionIdentity | null>(null)
  const [branding, setBranding] = useState<Branding | null>(null)
  const [permissions, setPermissions] = useState<string[] | null>(null)
  const [modules, setModules] = useState<string[] | null>(null)
  const [transferTargets, setTransferTargets] = useState<TransferTarget[]>([])
  const [surface, setSurface] = useState<Surface>(readSurface)
  const [agentId, setAgentId] = useState<number | null>(readAgentId)
  const [knowledgeAgentId, setKnowledgeAgentId] = useState<number | null>(readKnowledgeAgentId)
  const [chatflowId, setChatflowId] = useState<number | null>(readChatflowId)
  const [inboxConversationId, setInboxConversationId] = useState<number | null>(readInboxConversationId)
  const [inboxNonce, setInboxNonce] = useState(0)
  const [contactsQuery, setContactsQuery] = useState('')
  const [contactsNonce, setContactsNonce] = useState(0)
  const [searchOpen, setSearchOpen] = useState(false)
  const [campaignTab, setCampaignTab] = useState<'campaigns' | 'segments' | 'groups'>(readCampaignTab)
  const [libraryTab, setLibraryTab] = useState<'quick-replies' | 'templates'>(readLibraryTab)
  const [summary, setSummary] = useState<OperationalSummary>(emptySummary)
  const [summaryError, setSummaryError] = useState<string | null>(null)
  const [inboxUnread, setInboxUnread] = useState<number | null>(null)
  const previousWaitingRef = useRef<number | null>(null)
  const firstRunCheckedRef = useRef(false)

  useEffect(() => {
    let active = true
    void fetch('/api/react/bootstrap', { credentials: 'same-origin' })
      .then(async (response) => {
        if (!response.ok) return null
        return await response.json() as { data?: { user?: { id?: number; name?: string; role?: string }; branding?: Branding; transfer_targets?: TransferTarget[]; permissions?: string[]; modules?: Array<{ key?: string; enabled?: boolean }> } }
      })
      .then((payload) => {
        if (!active || !payload?.data) return
        const user = payload.data.user
        if (user) {
          const role = user.role?.trim() || 'operador'
          setIdentity({ id: typeof user.id === 'number' ? user.id : undefined, name: user.name?.trim() || 'Usuário', role })
          if (role === 'super_admin') {
            setSurface('empresas')
            const params = new URLSearchParams(window.location.search)
            params.set('view', 'empresas')
            params.delete('tab')
            window.history.replaceState(null, '', `${window.location.pathname}?${params.toString()}`)
          }
        }
        if (payload.data.branding) {
          setBranding(payload.data.branding)
          applyBrandingTheme(payload.data.branding)
          brandingDocumentTitle(payload.data.branding.name)
        }
        if (payload.data.transfer_targets) setTransferTargets(payload.data.transfer_targets)
        if (payload.data.permissions) setPermissions(payload.data.permissions)
        if (payload.data.modules) setModules(payload.data.modules.filter((module) => module.enabled !== false).map((module) => module.key || '').filter(Boolean))
      })
      .catch(() => undefined)
    return () => {
      active = false
    }
  }, [])

  useEffect(() => {
    if (!identity || firstRunCheckedRef.current) return
    const isSuper = identity.role === 'super_admin'
    const canSetupTenant = permissions?.includes('config_gerenciar') ?? false
    if (!isSuper && !canSetupTenant) return
    firstRunCheckedRef.current = true
    let active = true
    const asArray = (value: unknown): unknown[] => {
      const data = (value as { data?: unknown })?.data
      if (Array.isArray(data)) return data
      return Array.isArray(value) ? value : []
    }
    const asCount = (value: unknown): number => {
      const data = (value as { data?: unknown })?.data
      if (Array.isArray(data)) return data.length
      const items = (data as { items?: unknown[] })?.items
      return Array.isArray(items) ? items.length : 0
    }
    void (async () => {
      try {
        if (isSuper) {
          const [tenants, servers] = await Promise.all([
            fetch('/api/react/platform/tenants', { credentials: 'same-origin' }).then((response) => response.json()),
            fetch('/api/react/platform/whatsapp-servers', { credentials: 'same-origin' }).then((response) => response.json()),
          ])
          if (!active) return
          if (asArray(tenants).length === 0 && asArray(servers).length === 0) setSurface('platformSetup')
        } else {
          const [providers, channels, agents] = await Promise.all([
            fetch('/api/react/providers', { credentials: 'same-origin' }).then((response) => response.json()),
            fetch('/api/react/channels', { credentials: 'same-origin' }).then((response) => response.json()),
            fetch('/api/react/agents', { credentials: 'same-origin' }).then((response) => response.json()),
          ])
          if (!active) return
          if (asCount(providers) === 0 && asCount(channels) === 0 && asCount(agents) === 0) setSurface('wizard')
        }
      } catch {
        return
      }
    })()
    return () => {
      active = false
    }
  }, [identity, permissions])

  const brandPresentation = useMemo(() => ({
    name: branding?.name?.trim() || 'Notreve',
    product: 'Agente IA',
    eyebrow: 'Centro de comando',
  }), [branding?.name])

  useEffect(() => {
    let active = true
    if (!identity || identity.role === 'super_admin') {
      setSummary(emptySummary)
      setSummaryError(null)
      return () => { active = false }
    }
    const loadSummary = () => {
      void fetch('/api/react/overview', { credentials: 'same-origin' })
        .then(async (response) => {
          if (!response.ok) throw new Error('overview_unavailable')
          return await response.json() as ConversationSummaryResponse
        })
        .then((payload) => {
          if (!active || !payload?.data) return
          if (Array.isArray(payload.data)) return
          const next = payload.data.summary
          if (!next) return
          setSummary({
            active: typeof next.active === 'number' ? next.active : null,
            waiting: typeof next.waiting === 'number' ? next.waiting : null,
            ai: typeof next.ai === 'number' ? next.ai : null,
            channels: typeof next.channels === 'number' ? next.channels : null,
          })
          setSummaryError(null)
        })
        .catch(() => { if (active) setSummaryError('Não foi possível atualizar o resumo da operação.') })
    }
    loadSummary()
    const interval = window.setInterval(loadSummary, 20000)
    return () => {
      active = false
      window.clearInterval(interval)
    }
  }, [identity])

  useEffect(() => {
    const previous = previousWaitingRef.current
    const waiting = summary.waiting
    previousWaitingRef.current = waiting
    if (previous === null || waiting === null || waiting <= previous) return
    if (surface !== 'inbox') playNewMessageSound()
    if (typeof document !== 'undefined' && document.hidden) {
      showDesktopNotification('Nova mensagem', 'Novos atendimentos aguardando na caixa de entrada.')
      startTitleAlert(branding?.name?.trim() || 'Agente IA')
    }
  }, [branding?.name, summary.waiting, surface])

  useEffect(() => {
    const handleVisibility = () => {
      if (!document.hidden) stopTitleAlert()
    }
    document.addEventListener('visibilitychange', handleVisibility)
    return () => document.removeEventListener('visibilitychange', handleVisibility)
  }, [])

  const firstName = useMemo(() => {
    const name = identity?.name?.trim() || 'equipe'
    return name.split(/\s+/)[0]
  }, [identity?.name])

  const navigateTo = (next: Surface) => {
    setSurface(next)
    setKnowledgeAgentId(null)
    setChatflowId(null)
    const params = new URLSearchParams(window.location.search)
    if (next === 'overview') params.delete('view')
    else params.set('view', next)
    if (!['campaigns', 'library'].includes(next)) params.delete('tab')
    params.delete('id')
    params.delete('agent_id')
    params.delete('conversation')
    params.delete('chatflow')
    const query = params.toString()
    window.history.replaceState(null, '', `${window.location.pathname}${query ? `?${query}` : ''}`)
  }

  const openInbox = (conversationId?: number) => {
    setSurface('inbox')
    setInboxConversationId(conversationId ?? null)
    if (conversationId) setInboxNonce((value) => value + 1)
    const params = new URLSearchParams(window.location.search)
    params.set('view', 'inbox')
    params.delete('tab')
    params.delete('id')
    params.delete('agent_id')
    if (conversationId) params.set('conversation', String(conversationId))
    else params.delete('conversation')
    params.delete('chatflow')
    const query = params.toString()
    window.history.replaceState(null, '', `${window.location.pathname}${query ? `?${query}` : ''}`)
  }

  const openContact = (search: string) => {
    setContactsQuery(search)
    setContactsNonce((value) => value + 1)
    navigateTo('contacts')
  }

  const openAgent = (id: number | null) => {
    setAgentId(id)
    setSurface('agent')
    const params = new URLSearchParams(window.location.search)
    params.set('view', 'agent')
    if (id === null) params.delete('id')
    else params.set('id', String(id))
    params.delete('tab')
    params.delete('agent_id')
    params.delete('conversation')
    params.delete('chatflow')
    const query = params.toString()
    window.history.replaceState(null, '', `${window.location.pathname}${query ? `?${query}` : ''}`)
  }

  const openKnowledge = (id: number | null) => {
    setKnowledgeAgentId(id)
    setSurface('knowledge')
    const params = new URLSearchParams(window.location.search)
    params.set('view', 'knowledge')
    if (id === null) params.delete('agent_id')
    else params.set('agent_id', String(id))
    params.delete('id')
    params.delete('tab')
    params.delete('conversation')
    params.delete('chatflow')
    const query = params.toString()
    window.history.replaceState(null, '', `${window.location.pathname}${query ? `?${query}` : ''}`)
  }

  const activeNavigationId = surfaceNavigation[surface]
  const unreadCount = inboxUnread ?? summary.waiting ?? 0
  const canOpen = (module: string, permission: string) =>
    (modules === null || modules.includes(module)) && (permissions === null || permissions.includes(permission))
  const canOpenInbox = canOpen('conversas', 'conversas_ver')
  const canSearch = canOpenInbox || canOpen('contatos', 'contatos_gerenciar') || canOpen('ia', 'agentes_gerenciar')

  if (surface === 'login') return <LoginPage />
  if (surface === 'setup') {
    return <SetupPage brandName={branding?.name || 'Notreve'} onNavigate={(next) => navigateToSetupSurface(next, setSurface)} />
  }

  return (
    <AppShell
      initialActiveId={activeNavigationId}
      userName={identity?.name}
      userRole={identity?.role}
      branding={{
        name: branding?.name,
        logo: branding?.logo_url,
        primary: branding?.primary_color,
        secondary: branding?.secondary_color,
        accent: branding?.accent_color,
        background: branding?.background_color,
        loading: branding?.loading_color,
      }}
      permissions={permissions}
      modules={modules}
      unreadCount={unreadCount}
      searchOpen={searchOpen}
      onSearchOpen={() => setSearchOpen(true)}
      onSearchClose={() => setSearchOpen(false)}
      onOpenConversation={(id) => openInbox(id)}
      onOpenContact={(query) => openContact(query)}
      onOpenAgent={(id) => openAgent(id)}
      onOpenInbox={(conversationId) => openInbox(conversationId)}
      onOpenLearning={() => navigateTo('learning')}
      onNavigate={(id) => {
        if (id === 'canais') navigateTo('channels')
        else if (id === 'contatos') navigateTo('contacts')
        else if (id === 'agentes') navigateTo('agents')
        else if (id === 'ias') navigateTo('providers')
        else if (id === 'integracoes') navigateTo('integrations')
        else if (id === 'equipe') navigateTo('team')
        else if (id === 'campanhas') { setCampaignTab('campaigns'); navigateTo('campaigns') }
        else if (id === 'relatorios') navigateTo('reports')
        else if (id === 'aprendizado') navigateTo('learning')
        else if (id === 'skills') navigateTo('skills')
        else if (id === 'agendamentos') navigateTo('schedules')
        else if (id === 'conhecimento') navigateTo('knowledge')
        else if (id === 'chatflows') navigateTo('chatflows')
        else if (id === 'biblioteca') { setLibraryTab('quick-replies'); navigateTo('library') }
        else if (id === 'empresas') navigateTo('empresas')
        else if (id === 'primeiros-passos') navigateTo('platformSetup')
        else if (id === 'planos') navigateTo('plans')
        else if (id === 'servidores') navigateTo('servidores')
        else if (id === 'marca') navigateTo('marca')
        else if (id === 'configuracoes') navigateTo('settings')
        else if (id === 'logs') navigateTo('logs')
        else if (id === 'lgpd') navigateTo('lgpd')
        else if (id === 'wizard') navigateTo('wizard')
        else if (id === 'inbox') openInbox()
        else if (id === 'conversas') navigateTo('overview')
      }}
    >
      <ErrorBoundary resetKey={surface} onGoHome={() => navigateTo('overview')}>
      {surface === 'overview' ? (
          <OverviewPage
            brand={brandPresentation}
            firstName={firstName}
            canOpenInbox={canOpenInbox}
            canSearch={canSearch}
            onOpenSearch={() => setSearchOpen(true)}
            onOpenInbox={() => openInbox()}
            onOpenChannels={canOpen('canais', 'canais_gerenciar') ? () => navigateTo('channels') : undefined}
            onOpenAgents={canOpen('ia', 'agentes_gerenciar') ? () => navigateTo('agents') : undefined}
            onOpenLearning={canOpen('ia', 'aprendizado_ver') ? () => navigateTo('learning') : undefined}
          />
      ) : surface === 'inbox' ? (
        <div className="inbox-full">
          <InboxPage key={`inbox-${inboxNonce}`} transferTargets={transferTargets} permissions={permissions} initialConversationId={inboxConversationId ?? undefined} onUnreadChange={setInboxUnread} />
        </div>
      ) : surface === 'channels' ? (
        <ChannelsPage permissions={permissions} />
      ) : surface === 'contacts' ? (
        <ContactsPage key={`contacts-${contactsNonce}`} initialQuery={contactsQuery} onBack={() => navigateTo('overview')} />
      ) : surface === 'agent' ? (
        <AgentEditorPage
          agentId={agentId}
          onBack={() => navigateTo('agents')}
          onSaved={openAgent}
          onOpenKnowledge={(id) => openKnowledge(id)}
          onOpenProviders={() => navigateTo('providers')}
          onOpenSkills={() => navigateTo('skills')}
          onOpenChannels={() => navigateTo('channels')}
        />
      ) : surface === 'agents' ? (
        <AgentsPage onBack={() => navigateTo('overview')} onOpenAgent={openAgent} />
      ) : surface === 'providers' ? (
        <ProvidersPage onBack={() => navigateTo('overview')} permissions={permissions} />
      ) : surface === 'integrations' ? (
        <IntegrationsPage modules={modules} permissions={permissions} onBack={() => navigateTo('overview')} />
      ) : surface === 'team' ? (
        <TeamPage onBack={() => navigateTo('overview')} permissions={permissions} currentUserId={identity?.id ?? null} />
      ) : surface === 'campaigns' ? (
        <CampaignsPage initialTab={campaignTab} onBack={() => navigateTo('overview')} permissions={permissions} />
      ) : surface === 'learning' ? (
        <LearningPage onBack={() => navigateTo('overview')} permissions={permissions} />
      ) : surface === 'knowledge' ? (
        <KnowledgePage
          onBack={() => navigateTo('overview')}
          initialAgentId={knowledgeAgentId}
          permissions={permissions}
          onOpenAgent={(id) => openAgent(id)}
        />
      ) : surface === 'skills' ? (
        <SkillsPage onBack={() => navigateTo('overview')} permissions={permissions} onOpenAgent={(id) => openAgent(id)} />
      ) : surface === 'schedules' ? (
        <SchedulesPage onBack={() => navigateTo('overview')} permissions={permissions} />
      ) : surface === 'logs' ? (
        <LogsPage onBack={() => navigateTo('overview')} />
      ) : surface === 'chatflows' ? (
        <ChatflowPage initialFlowId={chatflowId} onBack={() => navigateTo('overview')} onOpenChannels={() => navigateTo('channels')} />
      ) : surface === 'library' ? (
        <MessageLibraryPage initialTab={libraryTab} onBack={() => navigateTo('overview')} />
      ) : surface === 'platformSetup' ? (
        <PlatformSetup onDone={(next) => navigateTo((next as Surface) ?? 'empresas')} />
      ) : surface === 'empresas' ? (
        <PlatformPage initialTab="tenants" onBack={() => navigateTo('overview')} />
      ) : surface === 'servidores' ? (
        <PlatformPage initialTab="whatsapp" onBack={() => navigateTo('overview')} />
      ) : surface === 'marca' ? (
        <PlatformPage initialTab="appearance" onBack={() => navigateTo('overview')} />
      ) : surface === 'plans' ? (
        <PlansPage onBack={() => navigateTo('empresas')} />
      ) : surface === 'settings' ? (
        <SettingsPage onBack={() => navigateTo('overview')} />
      ) : surface === 'lgpd' ? (
        <LgpdPage permissions={permissions} onBack={() => navigateTo('overview')} />
      ) : surface === 'wizard' ? (
        <WizardPage onDone={() => navigateTo('overview')} onNavigate={(view) => navigateTo(view as Surface)} />
      ) : surface === 'reports' ? (
        <ReportsPage onBack={() => navigateTo('overview')} permissions={permissions} />
      ) : (
        <OverviewPage
          brand={brandPresentation}
          firstName={firstName}
          canOpenInbox={canOpenInbox}
          canSearch={canSearch}
          onOpenSearch={() => setSearchOpen(true)}
          onOpenInbox={() => openInbox()}
          onOpenChannels={canOpen('canais', 'canais_gerenciar') ? () => navigateTo('channels') : undefined}
          onOpenAgents={canOpen('ia', 'agentes_gerenciar') ? () => navigateTo('agents') : undefined}
          onOpenLearning={canOpen('ia', 'aprendizado_ver') ? () => navigateTo('learning') : undefined}
        />
      )}
      </ErrorBoundary>
    </AppShell>
  )
}

function navigateToSetupSurface(next: string, setSurface: (surface: Surface) => void) {
  const allowed = new Set<Surface>(['providers', 'channels', 'agents', 'team', 'integrations', 'library', 'knowledge'])
  if (allowed.has(next as Surface)) {
    setSurface(next as Surface)
    const params = new URLSearchParams(window.location.search)
    params.set('view', next)
    window.history.replaceState(null, '', `${window.location.pathname}?${params.toString()}`)
  }
}

type OverviewPageProps = {
  brand: BrandPresentation
  firstName: string
  canOpenInbox?: boolean
  canSearch?: boolean
  onOpenSearch: () => void
  onOpenInbox: () => void
  onOpenChannels?: () => void
  onOpenAgents?: () => void
  onOpenLearning?: () => void
}

type DashboardPeriod = '1' | '7' | '30'

type DashboardPermissions = { reports: boolean; failures: boolean }
type DashboardConversations = { ia: number; aguardando: number; atendendo: number; encerradas: number; novas: number; total: number }
type DashboardQueue = { aguardando: number; espera_max_segundos: number; espera_media_segundos: number }
type DashboardResolution = { total: number; ia_resolvidas: number; transferidas: number; taxa_ia: number }
type DashboardSatisfaction = { avaliacoes: number; media: number }
type DashboardVolume = { date: string; label: string; recebidas: number; enviadas: number }
type DashboardSector = { id?: number; name: string; conversations: number; waiting: number }
type DashboardAttendant = { id: number; name: string; active: number; waiting: number }
type DashboardFailures = { open: number; by_type: Record<string, number> }
type DashboardData = {
  period: string
  generated_at: string
  permissions: DashboardPermissions
  conversations: DashboardConversations
  queue: DashboardQueue
  resolution: DashboardResolution
  satisfaction?: DashboardSatisfaction
  volume: DashboardVolume[]
  sectors?: DashboardSector[]
  attendants: DashboardAttendant[]
  failures?: DashboardFailures
}
type DashboardEnvelope = { data?: DashboardData; error?: { message?: string } }

function OverviewPage({ brand, firstName, canOpenInbox = true, canSearch = true, onOpenSearch, onOpenInbox, onOpenChannels, onOpenAgents, onOpenLearning }: OverviewPageProps) {
  const greeting = getGreeting()
  const [period, setPeriod] = useState<DashboardPeriod>('7')
  const [data, setData] = useState<DashboardData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    setLoading(true)
    void fetch(`/api/react/dashboard?period=${period}`, { credentials: 'same-origin' })
      .then(async (response) => {
        const payload = await response.json() as DashboardEnvelope
        if (!response.ok || !payload.data) throw new Error(payload.error?.message || 'Não foi possível carregar a visão geral.')
        return payload.data
      })
      .then((next) => { if (active) { setData(next); setError(null) } })
      .catch((loadError) => { if (active) setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar a visão geral.') })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [period])

  const maxVolume = useMemo(() => Math.max(1, ...(data?.volume ?? []).flatMap((day) => [day.recebidas, day.enviadas])), [data])
  const hasVolume = useMemo(() => (data?.volume ?? []).some((day) => day.recebidas + day.enviadas > 0), [data])
  const reports = data?.permissions.reports ?? false
  const openConversations = data ? data.conversations.ia + data.conversations.aguardando + data.conversations.atendendo : 0

  return (
    <div className="home-view">
      <section className="page-intro">
        <div>
          <span className="eyebrow">{brand.eyebrow}</span>
          <h1>{greeting}, {firstName} <span aria-hidden="true">✦</span></h1>
          <p>Uma visão clara para sua equipe agir rápido, com contexto e inteligência em cada conversa.</p>
        </div>
        <div className="intro-actions">
          {canSearch && (
            <button className="secondary-button" type="button" aria-label="Buscar em toda a operação" onClick={onOpenSearch}>
              <span aria-hidden="true">⌕</span>
              <span>Buscar em tudo</span>
              <kbd>Ctrl K</kbd>
            </button>
          )}
          {canOpenInbox && (
            <button className="primary-button" type="button" onClick={onOpenInbox}>
              Abrir Inbox
              <span aria-hidden="true">→</span>
            </button>
          )}
        </div>
      </section>

      {error && (
        <div className="workspace-alert" role="alert">
          <span>!</span>
          {error}
          <button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button>
        </div>
      )}

      <section className="overview-section" aria-labelledby="overview-title">
        <div className="section-heading">
          <div>
            <span className="eyebrow">Agora na operação</span>
            <h2 id="overview-title">O que merece atenção</h2>
          </div>
          <div className="dashboard-period-wrap">
            {loading && data && <span className="date-label">Atualizando…</span>}
            <div className="dashboard-period" role="group" aria-label="Período">
              {(['1', '7', '30'] as DashboardPeriod[]).map((value) => (
                <button key={value} type="button" className={period === value ? 'is-active' : ''} onClick={() => setPeriod(value)}>
                  {value === '1' ? 'Hoje' : value === '7' ? '7 dias' : '30 dias'}
                </button>
              ))}
            </div>
          </div>
        </div>

        {loading && !data ? (
          <div className="workspace-state">Carregando a operação…</div>
        ) : !data ? (
          <div className="workspace-state">Sem dados para exibir.</div>
        ) : (
          <>
            {data.failures && data.failures.open > 0 && (
              <div className="dashboard-failure" role="alert">
                <div>
                  <strong>{data.failures.open} falha(s) do agente em aberto</strong>
                  <span>{failureTypeSummary(data.failures.by_type)}</span>
                </div>
                {onOpenLearning && <button className="secondary-button" type="button" onClick={onOpenLearning}>Ver Aprendizado →</button>}
              </div>
            )}

            <div className="dashboard-kpis">
              <DashboardKpi icon="✧" tone="blue" label="Novas no período" value={formatNumber(data.conversations.novas)} detail={`${formatNumber(data.conversations.total)} conversas no total`} />
              <DashboardKpi icon="◌" tone="green" label="Em aberto" value={formatNumber(openConversations)} detail={`IA ${formatNumber(data.conversations.ia)} · fila ${formatNumber(data.conversations.aguardando)} · atendendo ${formatNumber(data.conversations.atendendo)}`} />
              <DashboardKpi icon="✓" tone="teal" label="Encerradas" value={formatNumber(data.conversations.encerradas)} detail="No total do escopo" />
              <DashboardKpi icon="⌛" tone="amber" label="Aguardando humano" value={formatNumber(data.queue.aguardando)} detail={data.queue.aguardando > 0 ? `Espera média ${formatWait(data.queue.espera_media_segundos)} · maior ${formatWait(data.queue.espera_max_segundos)}` : 'Nenhuma conversa na fila'} />
              {reports && <DashboardKpi icon="★" tone="violet" label="Satisfação (CSAT)" value={data.satisfaction && data.satisfaction.avaliacoes > 0 ? `${data.satisfaction.media.toFixed(1)}/5` : '—'} detail={`${formatNumber(data.satisfaction?.avaliacoes ?? 0)} avaliações no período`} />}
              <DashboardKpi icon="⇄" tone="teal" label="Resolução pela IA" value={formatPercent(data.resolution.taxa_ia)} detail={`IA ${formatNumber(data.resolution.ia_resolvidas)} · transferidas ${formatNumber(data.resolution.transferidas)}`} />
            </div>

            <div className="dashboard-panels">
              <article className="dashboard-panel dashboard-panel--chart">
                <header className="dashboard-panel-heading">
                  <div>
                    <span className="eyebrow">Ritmo da operação</span>
                    <h3>Mensagens por dia</h3>
                  </div>
                  <div className="dashboard-chart-legend">
                    <span><i className="is-received" />Recebidas</span>
                    <span><i className="is-sent" />Enviadas</span>
                  </div>
                </header>
                {hasVolume ? (
                  <div className="dashboard-chart" aria-label="Volume de mensagens por dia">
                    {data.volume.map((day) => (
                      <div className="dashboard-chart-column" key={day.date} title={`${day.label}: ${day.recebidas} recebidas, ${day.enviadas} enviadas`}>
                        <div className="dashboard-chart-bars">
                          <i className="is-received" style={{ height: `${barHeight(day.recebidas, maxVolume)}%` }} />
                          <i className="is-sent" style={{ height: `${barHeight(day.enviadas, maxVolume)}%` }} />
                        </div>
                        <span>{day.label}</span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="dashboard-empty">Sem mensagens no período.</p>
                )}
              </article>

              {reports && (
                <article className="dashboard-panel">
                  <header className="dashboard-panel-heading">
                    <div>
                      <span className="eyebrow">Onde está o volume</span>
                      <h3>Top setores</h3>
                    </div>
                  </header>
                  {data.sectors && data.sectors.length > 0 ? (
                    <ul className="dashboard-list">
                      {data.sectors.map((sector) => (
                        <li className="dashboard-list-row" key={sector.id ?? sector.name}>
                          <span className="dashboard-list-name">{sector.name}</span>
                          {sector.waiting > 0 && <span className="dashboard-badge">{sector.waiting} na fila</span>}
                          <span className="dashboard-list-value">{formatNumber(sector.conversations)}<small>conversas</small></span>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p className="dashboard-empty">Sem conversas no período.</p>
                  )}
                </article>
              )}
            </div>

            <div className="dashboard-panels">
              <article className="dashboard-panel">
                <header className="dashboard-panel-heading">
                  <div>
                    <span className="eyebrow">Carga da equipe</span>
                    <h3>Conversas por atendente</h3>
                  </div>
                </header>
                {data.attendants.length > 0 ? (
                  <ul className="dashboard-list">
                    {data.attendants.map((attendant) => (
                      <li className="dashboard-list-row" key={attendant.id}>
                        <span className="dashboard-list-name">{attendant.name}</span>
                        {attendant.waiting > 0 && <span className="dashboard-badge">{attendant.waiting} aguardando</span>}
                        <span className="dashboard-list-value">{formatNumber(attendant.active)}<small>em atendimento</small></span>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="dashboard-empty">Nenhuma conversa atribuída a atendentes.</p>
                )}
              </article>

              <article className="dashboard-panel">
                <header className="dashboard-panel-heading">
                  <div>
                    <span className="eyebrow">Próximo movimento</span>
                    <h3>Comece por onde importa</h3>
                  </div>
                </header>
                <div className="dashboard-actions">
                  {canOpenInbox && <button className="overview-card overview-card-action" type="button" onClick={onOpenInbox}><div className="overview-icon blue" aria-hidden="true">→</div><div className="overview-card-copy"><span>Atalho da operação</span><strong>Assumir atendimentos</strong><small>Entre no Inbox e veja o que está acontecendo.</small></div></button>}
                  {onOpenAgents && <button className="overview-card overview-card-action" type="button" onClick={onOpenAgents}><div className="overview-icon green" aria-hidden="true">✦</div><div className="overview-card-copy"><span>Atalho da operação</span><strong>Desenhar sua IA</strong><small>Prepare agentes, conhecimento e automações.</small></div></button>}
                  {onOpenChannels && <button className="overview-card overview-card-action" type="button" onClick={onOpenChannels}><div className="overview-icon purple" aria-hidden="true">◈</div><div className="overview-card-copy"><span>Atalho da operação</span><strong>Conectar um canal</strong><small>Leve sua operação para onde seu cliente está.</small></div></button>}
                </div>
              </article>
            </div>
          </>
        )}
      </section>
    </div>
  )
}

function DashboardKpi({ icon, tone, label, value, detail }: { icon: string; tone: string; label: string; value: string; detail: string }) {
  return (
    <article className={`dashboard-kpi dashboard-kpi--${tone}`}>
      <span className="dashboard-kpi-icon" aria-hidden="true">{icon}</span>
      <div className="dashboard-kpi-copy">
        <span>{label}</span>
        <strong>{value}</strong>
        <small>{detail}</small>
      </div>
    </article>
  )
}

function failureTypeSummary(byType: Record<string, number> | null | undefined) {
  const entries = Object.entries(byType ?? {}).filter(([, count]) => count > 0)
  if (entries.length === 0) return 'Sem detalhamento por tipo'
  return entries.map(([kind, count]) => `${kind} ${count}`).join(' · ')
}

function barHeight(value: number, max: number) {
  if (value <= 0) return 0
  return Math.max((value / max) * 100, 4)
}

function formatNumber(value: number) {
  return (Number(value) || 0).toLocaleString('pt-BR')
}

function formatPercent(value: number) {
  return `${(Number(value) || 0).toFixed(1).replace('.', ',')}%`
}

function formatWait(seconds: number) {
  const total = Math.max(0, Math.round(Number(seconds) || 0))
  if (total < 60) return `${total}s`
  const minutes = Math.floor(total / 60)
  if (minutes < 60) return `${minutes}min`
  const hours = Math.floor(minutes / 60)
  const rest = minutes % 60
  return rest ? `${hours}h ${rest}min` : `${hours}h`
}

function getGreeting() {
  const hour = Number(new Intl.DateTimeFormat('pt-BR', { hour: 'numeric', hour12: false, timeZone: 'America/Sao_Paulo' }).format(new Date()))
  if (hour < 12) return 'Bom dia'
  if (hour < 18) return 'Boa tarde'
  return 'Boa noite'
}
