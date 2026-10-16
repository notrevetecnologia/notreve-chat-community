export type AuthResult = {
  authenticated: boolean
  error?: string
}

export async function submitLogin(username: string, password: string): Promise<AuthResult> {
  const body = new URLSearchParams({ username: username.trim(), password })
  const response = await fetch('/login', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body,
    redirect: 'follow',
  })

  if (!response.ok || new URL(response.url, window.location.origin).pathname === '/login') {
    return { authenticated: false, error: 'Usuário ou senha incorretos.' }
  }
  return { authenticated: true }
}

export async function endSession(): Promise<void> {
  await fetch('/logout', { method: 'GET', credentials: 'same-origin', redirect: 'follow' })
}

type ReactEnvelope<T> = {
  data?: T
}

export async function fetchReactData<T>(path: string): Promise<T> {
  const response = await fetch(path, { credentials: 'same-origin' })
  if (!response.ok) {
    throw new Error(`Não foi possível carregar ${path}`)
  }
  const payload = await response.json() as ReactEnvelope<T> | T
  if (payload && typeof payload === 'object' && 'data' in payload && payload.data !== undefined) {
    return payload.data
  }
  return payload as T
}
