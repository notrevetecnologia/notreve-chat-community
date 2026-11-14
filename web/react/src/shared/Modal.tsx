import { useEffect, useId, useRef, type JSX, type ReactNode } from 'react'
import { Icon } from './icons'
import './Modal.css'

const focusableSelector = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[contenteditable="true"]',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

const focusableElements = (container: HTMLElement): HTMLElement[] =>
  Array.from(container.querySelectorAll<HTMLElement>(focusableSelector)).filter((element) =>
    !element.closest('[hidden], [aria-hidden="true"]'),
  )

export type ModalProps = {
  open: boolean
  title?: string
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
  size?: 'sm' | 'md' | 'lg'
  closeOnBackdrop?: boolean
}

export function Modal({
  open,
  title,
  onClose,
  children,
  footer,
  size = 'md',
  closeOnBackdrop = true,
}: ModalProps): JSX.Element | null {
  const dialogRef = useRef<HTMLElement>(null)
  const restoreFocusRef = useRef<HTMLElement | null>(null)
  const wasOpenRef = useRef(false)
  const onCloseRef = useRef(onClose)
  const titleId = `nv-modal-title-${useId().replace(/:/g, '')}`

  onCloseRef.current = onClose

  if (open && !wasOpenRef.current && typeof document !== 'undefined') {
    restoreFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
  }
  wasOpenRef.current = open

  useEffect(() => {
    if (!open) return

    const dialog = dialogRef.current
    if (!dialog) return

    if (!dialog.contains(document.activeElement)) {
      const elements = focusableElements(dialog)
      const target = elements.find((element) => element.hasAttribute('autofocus')) ?? elements[0] ?? dialog
      target.focus()
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        onCloseRef.current()
        return
      }
      if (event.key !== 'Tab') return

      const elements = focusableElements(dialog)
      if (elements.length === 0) {
        event.preventDefault()
        dialog.focus()
        return
      }

      const first = elements[0]
      const last = elements[elements.length - 1]
      const activeElement = document.activeElement
      if (event.shiftKey && (activeElement === first || activeElement === dialog || !dialog.contains(activeElement))) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && (activeElement === last || activeElement === dialog || !dialog.contains(activeElement))) {
        event.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      const element = restoreFocusRef.current
      restoreFocusRef.current = null
      if (element?.isConnected) element.focus()
    }
  }, [open])

  useEffect(() => {
    if (!open) return
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = previousOverflow
    }
  }, [open])

  if (!open) return null

  return (
    <div
      className="nv-modal-backdrop"
      onMouseDown={(event) => {
        if (closeOnBackdrop && event.target === event.currentTarget) onClose()
      }}
    >
      <section
        ref={dialogRef}
        className={`nv-modal nv-modal-${size}`}
        role="dialog"
        aria-modal="true"
        aria-labelledby={title ? titleId : undefined}
        aria-label={title ? undefined : 'Janela de diálogo'}
        tabIndex={-1}
      >
        <header className="nv-modal-header">
          {title ? <h2 className="nv-modal-title" id={titleId}>{title}</h2> : <span aria-hidden="true" />}
          <button className="nv-modal-close" type="button" onClick={onClose} aria-label="Fechar">
            <Icon name="close" />
          </button>
        </header>
        <div className="nv-modal-body">{children}</div>
        {footer ? <footer className="nv-modal-footer">{footer}</footer> : null}
      </section>
    </div>
  )
}
