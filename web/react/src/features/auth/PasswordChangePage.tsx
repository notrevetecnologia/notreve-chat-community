import { FormEvent, useState } from 'react'
import './Auth.css'

type PasswordChangePageProps = {
  brandName?: string
  logoURL?: string
}

type ErrorEnvelope = {
  error?: { message?: string }
}

export function PasswordChangePage({ brandName = 'Notreve Chat Community', logoURL }: PasswordChangePageProps) {
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!currentPassword || !newPassword) {
      setError('Informe a senha temporária e a nova senha.')
      return
    }
    if (newPassword.length < 8) {
      setError('A nova senha deve ter pelo menos 8 caracteres.')
      return
    }
    if (newPassword !== confirmPassword) {
      setError('A confirmação da nova senha não confere.')
      return
    }
    setLoading(true)
    setError('')
    try {
      const response = await fetch('/api/react/password/change', {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
      })
      if (!response.ok) {
        let message = 'Não foi possível alterar a senha.'
        try {
          const payload = await response.json() as ErrorEnvelope
          if (payload.error?.message) message = payload.error.message
        } catch {
          // resposta sem JSON
        }
        setError(message)
        return
      }
      window.location.assign('/auth?view=login')
    } catch {
      setError('Não foi possível conectar ao painel. Tente novamente.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="auth-page">
      <section className="auth-art" aria-label={brandName}>
        <div className="auth-orbit auth-orbit-one" />
        <div className="auth-orbit auth-orbit-two" />
        <div className="auth-art-core"><span>✦</span><strong>IA</strong></div>
        <p>Primeiro acesso protegido.</p>
      </section>
      <section className="auth-panel">
        <div className="auth-brand">
          {logoURL ? <img src={logoURL} alt="" /> : <span className="auth-brand-mark">◆</span>}
          <strong>{brandName}</strong>
        </div>
        <div className="auth-heading">
          <span className="eyebrow">Segurança da conta</span>
          <h1>Crie sua senha</h1>
          <p>A senha recebida é temporária. Defina uma senha pessoal antes de acessar o painel.</p>
        </div>
        <form className="auth-form" onSubmit={submit} noValidate>
          <label htmlFor="current-password">Senha temporária</label>
          <input id="current-password" type="password" autoComplete="current-password" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} autoFocus />
          <label htmlFor="new-password">Nova senha</label>
          <input id="new-password" type="password" autoComplete="new-password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} />
          <label htmlFor="confirm-password">Confirmar nova senha</label>
          <input id="confirm-password" type="password" autoComplete="new-password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} />
          {error && <p className="auth-error" role="alert">{error}</p>}
          <button className="auth-submit" type="submit" disabled={loading}>
            {loading ? 'Salvando…' : 'Salvar nova senha'}
            <span aria-hidden="true">→</span>
          </button>
        </form>
        <p className="auth-footnote">Após a troca, entre novamente com a nova senha.</p>
      </section>
    </main>
  )
}
