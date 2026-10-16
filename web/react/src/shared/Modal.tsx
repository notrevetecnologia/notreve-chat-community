import { useEffect, type ReactNode } from 'react'
import './Modal.css'

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
  useEffect(() => {
    if (!open) return
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [open, onClose])

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
        className={`nv-modal nv-modal-${size}`}
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <header className="nv-modal-header">
          {title ? <h2 className="nv-modal-title">{title}</h2> : <span aria-hidden="true" />}
          <button className="nv-modal-close" type="button" onClick={onClose} aria-label="Fechar">
            ×
          </button>
        </header>
        <div className="nv-modal-body">{children}</div>
        {footer ? <footer className="nv-modal-footer">{footer}</footer> : null}
      </section>
    </div>
  )
}
