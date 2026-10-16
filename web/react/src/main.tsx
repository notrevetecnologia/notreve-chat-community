import { StrictMode, useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import App from './app/App'
import { PasswordChangePage } from './features/auth'
import type { Branding } from './branding'
import { applyTheme, resolveInitialTheme } from './shared/theme'
import { ErrorBoundary } from './shared/ErrorBoundary'
import './styles/global.css'
import './styles/app-shell.css'

applyTheme(resolveInitialTheme())

type SessionPolicyResponse = {
  data?: {
    must_change_password?: boolean
    branding?: Branding
  }
}

function RootApp() {
  const [checking, setChecking] = useState(true)
  const [mustChangePassword, setMustChangePassword] = useState(false)
  const [branding, setBranding] = useState<Branding | null>(null)

  useEffect(() => {
    let active = true
    void fetch('/api/react/session', { credentials: 'same-origin' })
      .then(async (response) => response.ok ? await response.json() as SessionPolicyResponse : null)
      .then((payload) => {
        if (!active) return
        setMustChangePassword(payload?.data?.must_change_password === true)
        if (payload?.data?.branding) setBranding(payload.data.branding)
      })
      .catch(() => undefined)
      .finally(() => {
        if (active) setChecking(false)
      })
    return () => { active = false }
  }, [])

  if (checking && window.location.pathname !== '/auth' && window.location.pathname !== '/auth/') {
    return <div className="react-page-loading" role="status">Carregando…</div>
  }
  if (mustChangePassword) {
    return <PasswordChangePage brandName={branding?.name || 'Notreve Chat Community'} logoURL={branding?.logo_url} />
  }
  return <App />
}

const rootElement = document.getElementById('root')

if (!rootElement) {
  throw new Error('Elemento raiz do React não encontrado')
}

createRoot(rootElement).render(
  <StrictMode>
    <ErrorBoundary
      variant="app"
      title="O painel encontrou um erro inesperado"
      description="Não foi possível montar a interface. Você pode tentar novamente ou voltar para a visão geral."
      onGoHome={() => window.location.assign('/')}
    >
      <RootApp />
    </ErrorBoundary>
  </StrictMode>,
)
