import { type CSSProperties, type ReactNode, useEffect, useMemo, useRef, useState } from 'react'
import {
  getNavigationItem,
  navigationGroups,
  navigationItems,
  type NavigationItem,
} from './navigation'
import { LogoutButton } from '../features/auth'
import { GlobalSearch } from '../shared/GlobalSearch'
import { NotificationCenter } from '../shared/NotificationCenter'
import { SecurityModal } from '../features/security/SecurityModal'
import { applyTheme, currentTheme, persistTheme, toggleTheme, type Theme } from '../shared/theme'

type AppShellProps = {
  initialActiveId?: string
  children?: ReactNode
  userName?: string
  userRole?: string
  branding?: ShellBranding
  permissions?: string[] | null
  modules?: string[] | null
  unreadCount?: number
  searchOpen?: boolean
  onSearchOpen?: () => void
  onSearchClose?: () => void
  onOpenConversation?: (id: number) => void
  onOpenContact?: (query: string) => void
  onOpenAgent?: (id: number) => void
  onOpenInbox?: (conversationId?: number) => void
  onOpenLearning?: () => void
  onNavigate?: (id: string) => void
}

export type ShellBranding = {
  name?: string
  logo?: string
  primary?: string
  secondary?: string
  accent?: string
  background?: string
  loading?: string
}

const defaultBranding: Required<ShellBranding> = {
  name: 'Notreve Chat',
  logo: '',
  primary: '#12948e',
  secondary: '#071c1c',
  accent: '#d5f37a',
  background: '#f5faf8',
  loading: '#12948e',
}

const SIDEBAR_STORAGE_KEY = 'ncc-sidebar'
const SIDEBAR_WIDTH_EXPANDED = '248px'
const SIDEBAR_WIDTH_COLLAPSED = '68px'

const readStoredCollapsed = (): boolean => {
  try {
    return window.localStorage.getItem(SIDEBAR_STORAGE_KEY) === 'collapsed'
  } catch {
    return false
  }
}

const roleLabels: Record<string, string> = {
  super_admin: 'Dono da plataforma',
  admin: 'Dono da empresa',
  supervisor: 'Supervisor',
  atendente: 'Atendente',
}

const groupOrder: NavigationItem['group'][] = [
  'atendimento',
  'relacionamento',
  'inteligencia',
  'canais',
  'automacao',
  'operacao',
  'administracao',
  'plataforma',
]

const groupIcons: Record<NavigationItem['group'], string> = {
  atendimento: '◉',
  relacionamento: '◎',
  inteligencia: '✦',
  canais: '◈',
  automacao: '⛓',
  operacao: '♧',
  administracao: '☷',
  plataforma: '⚙',
}

const createExpandedGroups = (activeId: string): Record<NavigationItem['group'], boolean> => {
  const activeGroup = getNavigationItem(activeId).group
  return groupOrder.reduce((groups, group) => {
    groups[group] = group === activeGroup
    return groups
  }, {} as Record<NavigationItem['group'], boolean>)
}

const hasAccess = (
  permissions: string[] | null,
  modules: string[] | null,
  module: string,
  permission: string,
): boolean =>
  (modules === null || modules.includes(module)) &&
  (permissions === null || permissions.includes(permission))

const searchShortcutLabel = (): string => {
  if (typeof navigator === 'undefined') return 'Ctrl K'
  return /Mac|iPhone|iPad|iPod/.test(navigator.userAgent) ? '⌘ K' : 'Ctrl K'
}

export function AppShell({ initialActiveId = 'conversas', children, userName, userRole, branding, permissions = null, modules = null, unreadCount = 0, searchOpen = false, onSearchOpen, onSearchClose, onOpenConversation, onOpenContact, onOpenAgent, onOpenInbox, onOpenLearning, onNavigate }: AppShellProps) {
  const [activeId, setActiveId] = useState(initialActiveId)
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [profileOpen, setProfileOpen] = useState(false)
  const [securityOpen, setSecurityOpen] = useState(false)
  const [theme, setTheme] = useState<Theme>(() => currentTheme())
  const [expandedGroups, setExpandedGroups] = useState(() => createExpandedGroups(initialActiveId))
  const [collapsed, setCollapsed] = useState(readStoredCollapsed)
  const [isDesktop, setIsDesktop] = useState(() =>
    typeof window === 'undefined' || typeof window.matchMedia !== 'function'
      ? true
      : window.matchMedia('(min-width: 981px)').matches,
  )
  const profileRef = useRef<HTMLDivElement>(null)
  const activeItem = useMemo(() => getNavigationItem(activeId), [activeId])
  const visualBranding = { ...defaultBranding, ...branding }
  const brandName = visualBranding.name || defaultBranding.name
  const displayName = userName?.trim() || 'Usuário'
  const displayRole = roleLabels[userRole?.trim() || ''] || 'Operador'
  const displayInitials = displayName
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? '')
    .join('') || 'U'
  const activeGroup = activeItem.group
  const visibleNavigationItems = useMemo(() => navigationItems.filter((item) => {
    if (userRole === 'super_admin') return item.group === 'plataforma'
    const moduleEnabled = modules === null || (item.requiredAnyModule ? item.requiredAnyModule.some((module) => modules.includes(module)) : (!item.module || modules.includes(item.module)))
    const permissionGranted = permissions === null || (!item.requiredPermission && !item.requiredAnyPermission) ||
      (item.requiredPermission ? permissions.includes(item.requiredPermission) : item.requiredAnyPermission?.some((permission) => permissions.includes(permission)))
    return moduleEnabled && permissionGranted
  }), [modules, permissions, userRole])
  const brandingStyle = {
    '--brand-primary': visualBranding.primary,
    '--brand-secondary': visualBranding.secondary,
    '--brand-accent': visualBranding.accent,
    '--shell-background': visualBranding.background,
    '--brand-loading': visualBranding.loading,
  } as CSSProperties
  const shellStyle = {
    ...brandingStyle,
    '--sidebar-width': collapsed ? SIDEBAR_WIDTH_COLLAPSED : SIDEBAR_WIDTH_EXPANDED,
  } as CSSProperties
  const canSearchConversations = hasAccess(permissions, modules, 'conversas', 'conversas_ver')
  const canSearchContacts = hasAccess(permissions, modules, 'contatos', 'contatos_gerenciar')
  const canSearchAgents = hasAccess(permissions, modules, 'ia', 'agentes_gerenciar')
  const canSearch = canSearchConversations || canSearchContacts || canSearchAgents
  const shortcutLabel = useMemo(searchShortcutLabel, [])
  const railMode = collapsed && isDesktop

  useEffect(() => {
    setActiveId(initialActiveId)
    setExpandedGroups(createExpandedGroups(initialActiveId))
  }, [initialActiveId])

  useEffect(() => {
    applyTheme(theme)
  }, [theme])

  useEffect(() => {
    try {
      window.localStorage.setItem(SIDEBAR_STORAGE_KEY, collapsed ? 'collapsed' : 'expanded')
    } catch {
      return
    }
  }, [collapsed])

  const toggleSidebar = () => setCollapsed((value) => !value)

  const switchTheme = () => {
    const next = toggleTheme(theme)
    setTheme(next)
    persistTheme(next)
  }

  useEffect(() => {
    const query = window.matchMedia('(min-width: 981px)')
    const handleChange = (event: MediaQueryListEvent) => setIsDesktop(event.matches)
    setIsDesktop(query.matches)
    query.addEventListener('change', handleChange)
    return () => query.removeEventListener('change', handleChange)
  }, [])

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setMobileMenuOpen(false)
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        if (canSearch) onSearchOpen?.()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [canSearch, onSearchOpen])

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (profileRef.current && !profileRef.current.contains(event.target as Node)) {
        setProfileOpen(false)
      }
    }
    window.addEventListener('mousedown', handleClickOutside)
    return () => window.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const selectNavigation = (id: string) => {
    const nextItem = getNavigationItem(id)
    setActiveId(id)
    setExpandedGroups((current) => ({ ...current, [nextItem.group]: true }))
    setMobileMenuOpen(false)
    onNavigate?.(id)
  }

  const toggleGroup = (group: NavigationItem['group']) => {
    setExpandedGroups((current) => ({ ...current, [group]: !current[group] }))
  }

  return (
    <div className={`app-shell${mobileMenuOpen ? ' menu-open' : ''}${collapsed ? ' sidebar-collapsed' : ''}`} style={shellStyle}>
      <button
        className="mobile-backdrop"
        type="button"
        aria-label="Fechar menu"
        onClick={() => setMobileMenuOpen(false)}
      />

      <aside className="app-sidebar" id="app-sidebar" aria-label="Navegação principal">
        <div className="sidebar-head">
          <div className="sidebar-brand">
            {visualBranding.logo ? (
              <img className="brand-logo" src={visualBranding.logo} alt={brandName} />
            ) : (
              <>
                <span className="brand-mark" aria-hidden="true">{brandName.trim().charAt(0).toUpperCase() || 'N'}</span>
                <div className="brand-copy">
                  <strong>{brandName}</strong>
                </div>
              </>
            )}
          </div>
          <button
            className="sidebar-toggle"
            type="button"
            aria-label={collapsed ? 'Expandir menu lateral' : 'Recolher menu lateral'}
            aria-expanded={!collapsed}
            aria-controls="app-sidebar"
            title={collapsed ? 'Expandir menu' : 'Recolher menu'}
            onClick={toggleSidebar}
          >
            <span className="sidebar-toggle-glyph" aria-hidden="true">«</span>
          </button>
          <button
            className="sidebar-close"
            type="button"
            aria-label="Fechar menu"
            onClick={() => setMobileMenuOpen(false)}
          >
            ×
          </button>
        </div>

        <nav className="sidebar-navigation">
          {railMode ? (
            visibleNavigationItems.map((item) => (
              <button
                className={`rail-item${activeId === item.id ? ' active' : ''}`}
                type="button"
                key={item.id}
                title={item.label}
                aria-label={item.label}
                aria-current={activeId === item.id ? 'page' : undefined}
                onClick={() => {
                  if (item.href) {
                    window.location.assign(item.href)
                    return
                  }
                  selectNavigation(item.id)
                }}
              >
                <span className="navigation-icon" aria-hidden="true">{item.icon}</span>
                {item.id === 'inbox' && unreadCount > 0 && (
                  <span className="rail-badge" aria-hidden="true">{unreadCount > 99 ? '99+' : unreadCount}</span>
                )}
              </button>
            ))
          ) : (
            groupOrder.map((group) => {
              const groupItems = visibleNavigationItems.filter((item) => item.group === group)
              if (groupItems.length === 0) return null
              const expanded = expandedGroups[group]
              return (
                <div className={`navigation-group${expanded ? ' expanded' : ''}`} key={group}>
                  <button
                    className={`navigation-label${activeGroup === group ? ' current' : ''}`}
                    type="button"
                    aria-expanded={expanded}
                    aria-controls={`navigation-group-${group}`}
                    onClick={() => toggleGroup(group)}
                  >
                    <span className="navigation-label-icon" aria-hidden="true">{groupIcons[group]}</span>
                    <span>{navigationGroups[group]}</span>
                    <span className="navigation-label-chevron" aria-hidden="true">{expanded ? '⌃' : '⌄'}</span>
                  </button>
                  {expanded && (
                    <div className="navigation-items" id={`navigation-group-${group}`}>
                      {groupItems.map((item) => (
                        <button
                          className={`navigation-item${activeId === item.id ? ' active' : ''}`}
                          type="button"
                          aria-current={activeId === item.id ? 'page' : undefined}
                          key={item.id}
                          onClick={() => {
                            if (item.href) {
                              window.location.assign(item.href)
                              return
                            }
                            selectNavigation(item.id)
                          }}
                        >
                          <span className="navigation-icon" aria-hidden="true">{item.icon}</span>
                          <span>{item.label}</span>
                          {item.id === 'inbox' && unreadCount > 0 && (
                            <span
                              className="navigation-badge"
                              aria-label={`${unreadCount} conversas aguardando`}
                              style={{
                                display: 'inline-flex',
                                minWidth: 20,
                                height: 20,
                                alignItems: 'center',
                                justifyContent: 'center',
                                marginLeft: 'auto',
                                padding: '0 6px',
                                borderRadius: 999,
                                background: 'var(--coral)',
                                color: 'var(--ink)',
                                fontSize: 10,
                                fontWeight: 800,
                                lineHeight: 1,
                              }}
                            >
                              {unreadCount > 99 ? '99+' : unreadCount}
                            </span>
                          )}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              )
            })
          )}
        </nav>

        <div className="sidebar-footer">
          <div className="profile-card" title={railMode ? displayName : undefined}>
            <span className="profile-avatar" aria-hidden="true">{displayInitials}</span>
            <span className="profile-copy">
              <strong>{displayName}</strong>
              <span>{displayRole}</span>
            </span>
          </div>
        </div>
      </aside>

      <div className="app-content">
        <header className="app-header">
          <div className="header-leading">
            <button
              className="mobile-menu-toggle"
              type="button"
              aria-label="Abrir menu"
              aria-expanded={mobileMenuOpen}
              onClick={() => setMobileMenuOpen(true)}
            >
              <span />
              <span />
              <span />
            </button>
            <div className="header-context">
              <h1>{activeItem.label}</h1>
              <span className="header-breadcrumb">{navigationGroups[activeItem.group]}</span>
            </div>
          </div>
          <div className="header-actions">
            {canSearch && (
              <button className="header-search" type="button" aria-label="Abrir busca global" onClick={() => onSearchOpen?.()}>
                <span className="header-search-icon" aria-hidden="true">⌕</span>
                <span className="header-search-label">Buscar</span>
                <kbd>{shortcutLabel}</kbd>
              </button>
            )}
            <button
              className="header-icon-button theme-toggle"
              type="button"
              aria-label={theme === 'dark' ? 'Ativar tema claro' : 'Ativar tema escuro'}
              aria-pressed={theme === 'dark'}
              title={theme === 'dark' ? 'Tema claro' : 'Tema escuro'}
              onClick={switchTheme}
            >
              <span aria-hidden="true">{theme === 'dark' ? '☀' : '☾'}</span>
            </button>
            <NotificationCenter
              waitingCount={unreadCount}
              permissions={permissions}
              modules={modules}
              onOpenInbox={(conversationId) => onOpenInbox?.(conversationId)}
              onOpenLearning={() => onOpenLearning?.()}
            />
            <div className="header-profile" ref={profileRef}>
              <button
                className="header-profile-trigger"
                type="button"
                aria-haspopup="menu"
                aria-expanded={profileOpen}
                onClick={() => setProfileOpen((value) => !value)}
              >
                <span className="header-user-avatar" aria-hidden="true">{displayInitials}</span>
                <span className="header-user-copy">
                  <strong>{displayName}</strong>
                  <span>{displayRole}</span>
                </span>
                <span className="header-profile-chevron" aria-hidden="true">⌄</span>
              </button>
              {profileOpen && (
                <div className="profile-menu" role="menu">
                  <div className="profile-menu-head">
                    <strong>{displayName}</strong>
                    <span className="profile-menu-role">{displayRole}</span>
                  </div>
                  <button className="profile-menu-item" type="button" role="menuitem" onClick={() => { setProfileOpen(false); setSecurityOpen(true) }}>
                    Segurança (2FA)
                  </button>
                  <LogoutButton className="profile-menu-item profile-menu-logout" onLoggedOut={() => window.location.assign('/auth?view=login')}>
                    Sair
                  </LogoutButton>
                </div>
              )}
            </div>
          </div>
        </header>

        <main className="main-content">
          {children}
        </main>
      </div>

      {canSearch && (
        <GlobalSearch
          open={searchOpen}
          onClose={() => onSearchClose?.()}
          permissions={permissions}
          modules={modules}
          onOpenConversation={(id) => onOpenConversation?.(id)}
          onOpenContact={(query) => onOpenContact?.(query)}
          onOpenAgent={(id) => onOpenAgent?.(id)}
        />
      )}
      <SecurityModal open={securityOpen} onClose={() => setSecurityOpen(false)} />
    </div>
  )
}
