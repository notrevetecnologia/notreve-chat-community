import { Component, type ErrorInfo, type ReactNode } from 'react'
import './ErrorBoundary.css'

type ErrorBoundaryProps = {
  children: ReactNode
  title?: string
  description?: string
  onReset?: () => void
  onGoHome?: () => void
  goHomeLabel?: string
  resetKey?: string | number
  variant?: 'surface' | 'app'
}

type ErrorBoundaryState = {
  error: Error | null
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('[ErrorBoundary] erro de renderização capturado', error, info.componentStack)
  }

  componentDidUpdate(previous: ErrorBoundaryProps) {
    if (this.state.error && previous.resetKey !== this.props.resetKey) {
      this.setState({ error: null })
    }
  }

  private reset = () => {
    this.setState({ error: null })
    this.props.onReset?.()
  }

  render() {
    const { error } = this.state
    if (!error) return this.props.children

    const {
      title = 'Não foi possível exibir esta tela',
      description = 'Algo inesperado aconteceu ao montar o conteúdo. O restante do painel continua funcionando normalmente.',
      onGoHome,
      goHomeLabel = 'Voltar à visão geral',
      variant = 'surface',
    } = this.props

    const detail = error.message?.trim() || String(error)

    return (
      <div className={`error-boundary error-boundary--${variant}`} role="alert">
        <div className="error-boundary-card">
          <span className="error-boundary-icon" aria-hidden="true">!</span>
          <span className="eyebrow">Algo saiu do previsto</span>
          <h2>{title}</h2>
          <p>{description}</p>
          <details className="error-boundary-details">
            <summary>Detalhes técnicos</summary>
            <code>{detail}</code>
          </details>
          <div className="error-boundary-actions">
            <button className="primary-button" type="button" onClick={this.reset}>
              Tentar novamente
            </button>
            {onGoHome && (
              <button className="secondary-button" type="button" onClick={onGoHome}>
                {goHomeLabel}
              </button>
            )}
          </div>
        </div>
      </div>
    )
  }
}

export default ErrorBoundary
