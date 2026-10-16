import { useState, type ReactNode } from 'react'
import { endSession } from './authApi'

type LogoutButtonProps = {
  className?: string
  onLoggedOut?: () => void
  children?: ReactNode
}

export function LogoutButton({ className = '', onLoggedOut, children = 'Sair' }: LogoutButtonProps) {
  const [loading, setLoading] = useState(false)

  const logout = async () => {
    if (loading) return
    setLoading(true)
    try {
      await endSession()
      onLoggedOut?.()
    } finally {
      setLoading(false)
    }
  }

  return <button className={className} type="button" onClick={logout} disabled={loading}>{loading ? 'Saindo…' : children}</button>
}
