import { useId, useRef, type KeyboardEvent, type ReactNode } from 'react'
import { Icon } from './icons'
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
  const tabsId = useId().replace(/:/g, '')
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([])
  const activeIndex = tabs.findIndex((item) => item.key === activeTab)
  const selectedIndex = activeIndex >= 0 ? activeIndex : 0
  const tabIds = tabs.map((_, index) => `entity-tab-${tabsId}-${index}`)
  const panelIds = tabs.map((_, index) => `entity-tab-panel-${tabsId}-${index}`)

  const handleTabKeyDown = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
    let nextIndex = index
    if (event.key === 'ArrowRight') nextIndex = (index + 1) % tabs.length
    else if (event.key === 'ArrowLeft') nextIndex = (index - 1 + tabs.length) % tabs.length
    else if (event.key === 'Home') nextIndex = 0
    else if (event.key === 'End') nextIndex = tabs.length - 1
    else return

    event.preventDefault()
    const nextTab = tabs[nextIndex]
    if (!nextTab) return
    onTabChange(nextTab.key)
    tabRefs.current[nextIndex]?.focus()
  }

  return (
    <div className="entity-screen">
      <header className="entity-screen-header">
        <button className="entity-back" type="button" onClick={onBack} aria-label={backLabel}>
          <Icon name="arrow-left" />
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
          {tabs.map((item, index) => (
            <button
              key={item.key}
              ref={(element) => { tabRefs.current[index] = element }}
              id={tabIds[index]}
              type="button"
              role="tab"
              aria-selected={selectedIndex === index}
              aria-controls={panelIds[index]}
              tabIndex={selectedIndex === index ? 0 : -1}
              className={selectedIndex === index ? 'is-active' : ''}
              onClick={() => onTabChange(item.key)}
              onKeyDown={(event) => handleTabKeyDown(event, index)}
            >
              <strong>{item.label}</strong>
              <small>{item.hint}</small>
            </button>
          ))}
        </nav>
        {tabs.map((_, index) => (
          <section
            className="entity-tab-panel"
            id={panelIds[index]}
            role="tabpanel"
            aria-labelledby={tabIds[index]}
            hidden={selectedIndex !== index}
            key={panelIds[index]}
          >
            {selectedIndex === index ? children : null}
          </section>
        ))}
      </div>
    </div>
  )
}

export function EntityAlert({ children, onClose }: { children: ReactNode; onClose: () => void }) {
  return (
    <div className="entity-alert" role="alert">
      <span><Icon name="alert-circle" /></span>
      <p>{children}</p>
      <button type="button" onClick={onClose} aria-label="Fechar aviso">
        <Icon name="close" />
      </button>
    </div>
  )
}

export function EntityNotice({ children, onClose }: { children: ReactNode; onClose: () => void }) {
  return (
    <div className="entity-notice" role="status">
      <span><Icon name="check" /></span>
      <p>{children}</p>
      <button type="button" onClick={onClose} aria-label="Fechar aviso">
        <Icon name="close" />
      </button>
    </div>
  )
}

export default EntityScreen
