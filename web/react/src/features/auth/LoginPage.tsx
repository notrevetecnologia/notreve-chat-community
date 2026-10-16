import { FormEvent, useEffect, useState } from 'react'
import { submitLogin } from './authApi'
import { applyBrandingTheme, brandingDocumentTitle } from '../../branding'
import './Auth.css'

type LoginPageProps = {
  brandName?: string
  logoURL?: string
  onAuthenticated?: () => void
}

export function LoginPage({ brandName = 'Notreve Chat Community', logoURL, onAuthenticated }: LoginPageProps) {
  const [resolvedBrandName, setResolvedBrandName] = useState(brandName)
  const [resolvedLogoURL, setResolvedLogoURL] = useState(logoURL)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [showRecovery, setShowRecovery] = useState(false)

  useEffect(() => {
    let active = true
    void fetch('/api/public/branding', { credentials: 'same-origin' })
      .then((response) => response.ok ? response.json() as Promise<{ data?: { branding?: { name?: string; logo_url?: string } } }> : null)
      .then((payload) => {
        if (!active || !payload?.data?.branding) return
        const branding = payload.data.branding
        applyBrandingTheme(branding)
        brandingDocumentTitle(branding.name)
        if (branding.name?.trim()) setResolvedBrandName(branding.name.trim())
        if (branding.logo_url?.trim()) setResolvedLogoURL(branding.logo_url.trim())
      })
      .catch(() => undefined)
    return () => { active = false }
  }, [brandName, logoURL])

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!username.trim() || !password) {
      setError('Informe usuário e senha para continuar.')
      return
    }
    setError('')
    setLoading(true)
    try {
      const result = await submitLogin(username, password)
      if (!result.authenticated) {
        setError(result.error || 'Não foi possível entrar.')
        return
      }
      if (onAuthenticated) {
        onAuthenticated()
      } else {
        window.location.assign('/')
      }
    } catch {
      setError('Não foi possível conectar ao painel. Tente novamente.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="auth-page">
      <section className="auth-card">
        <div className="auth-brand">
          {resolvedLogoURL ? <img src={resolvedLogoURL} alt="" /> : <span className="auth-brand-mark">{resolvedBrandName.trim().charAt(0).toUpperCase() || 'N'}</span>}
          <strong>{resolvedBrandName}</strong>
        </div>
        <div className="auth-heading">
          <h1>Entrar</h1>
          <p>Acesse o painel para cuidar do seu atendimento.</p>
        </div>
        {showRecovery ? <div className="auth-recovery" role="region" aria-labelledby="auth-recovery-title">
          <h2 id="auth-recovery-title">Recuperar acesso</h2>
          <p>Por segurança, a redefinição é feita pelo administrador da sua empresa. Peça para ele acessar <strong>Equipe → Usuários</strong> e criar uma nova senha para seu usuário.</p>
          <button className="auth-link" type="button" onClick={() => setShowRecovery(false)}>Voltar para o login</button>
        </div> : <form className="auth-form" onSubmit={submit} noValidate>
          <label htmlFor="auth-username">Usuário</label>
          <input
            id="auth-username"
            name="username"
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            autoComplete="username"
            autoFocus
          />
          <label htmlFor="auth-password">Senha</label>
          <input
            id="auth-password"
            name="password"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            autoComplete="current-password"
          />
          {error && <p className="auth-error" role="alert">{error}</p>}
          <button className="auth-submit" type="submit" disabled={loading}>
            {loading ? 'Entrando…' : 'Entrar'}
          </button>
        </form>}
        {!showRecovery && <button className="auth-recovery-link" type="button" onClick={() => { setError(''); setShowRecovery(true) }}>Esqueci minha senha</button>}
        <p className="auth-footnote">Acesso protegido para sua equipe.</p>
      </section>
    </main>
  )
}
