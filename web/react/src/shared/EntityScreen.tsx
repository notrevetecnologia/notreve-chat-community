import type { ReactNode } from 'react'
import './EntityScreen.css'

export type EntityTab = {
  key: string
  label: string
  hint: string
}

export function EntityScreen({
  eyebrow,
  title,
  onBack,
  backLabel = 'Voltar',
  actions,
  tabs,
  activeTab,
  onTabChange,
  alert,
  notice,
  children,
}: {
  eyebrow: string
  title: string
  onBack: () => void
  backLabel?: string
  actions?: ReactNode
  tabs: EntityTab[]
  activeTab: string
  onTabChange: (key: string) => void
  alert?: ReactNode
  notice?: ReactNode
  children: ReactNode
}) {
  return (
    <div className="entity-screen">
      <header className="entity-screen-header">
        <button className="entity-back" type="button" onClick={onBack} aria-label={backLabel}>
          ←
        </button>
        <div className="entity-screen-title">
          <span className="eyebrow">{eyebrow}</span>
          <h1>{title}</h1>
        </div>
        <div className="entity-screen-actions">{actions}</div>
      </header>

      {alert}
      {notice}

      <div className="entity-tabs-card">
        <nav className="entity-tabs" role="tablist" aria-label="Seções">
          {tabs.map((item) => (
            <button
              key={item.key}
              type="button"
              role="tab"
              aria-selected={activeTab === item.key}
              className={activeTab === item.key ? 'is-active' : ''}
              onClick={() => onTabChange(item.key)}
            >
              <strong>{item.label}</strong>
              <small>{item.hint}</small>
            </button>
          ))}
        </nav>
        <section className="entity-tab-panel" role="tabpanel">
          {children}
        </section>
      </div>
    </div>
  )
}

export function EntityAlert({ children, onClose }: { children: ReactNode; onClose: () => void }) {
  return (
    <div className="entity-alert" role="alert">
      <span>!</span>
      <p>{children}</p>
      <button type="button" onClick={onClose} aria-label="Fechar aviso">
        ×
      </button>
    </div>
  )
}

export function EntityNotice({ children, onClose }: { children: ReactNode; onClose: () => void }) {
  return (
    <div className="entity-notice" role="status">
      <span>✓</span>
      <p>{children}</p>
      <button type="button" onClick={onClose} aria-label="Fechar aviso">
        ×
      </button>
    </div>
  )
}

export default EntityScreen
