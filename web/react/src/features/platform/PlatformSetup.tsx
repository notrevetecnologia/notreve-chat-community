import { useState, type ReactNode } from 'react'
import './Platform.css'

type Envelope<T> = { data?: T; error?: { message?: string } }

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, { ...options, credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) } })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return payload.data as T
}

type ServerDraft = { name: string; base_url: string; token: string; capacity: number }
type CompanyDraft = { name: string; slug: string; domain: string; admin_name: string; admin_username: string; admin_password: string }

const newServer = (): ServerDraft => ({ name: 'WhatsApp principal', base_url: '', token: '', capacity: 100 })
const newCompany = (): CompanyDraft => ({ name: '', slug: '', domain: '', admin_name: '', admin_username: '', admin_password: '' })

const slugify = (value: string) => value.trim().toLowerCase().replace(/[^a-z0-9-]/g, '-').replace(/-+/g, '-').replace(/^-|-$/g, '')
const messageOf = (cause: unknown, fallback: string) => (cause instanceof Error ? cause.message : fallback)

const steps = [
  { id: 0, label: 'API WhatsApp', hint: 'Conecte a plataforma' },
  { id: 1, label: 'Primeira empresa', hint: 'Crie o workspace' },
]

export function PlatformSetup({ onDone }: { onDone: (surface?: string) => void }) {
  const [step, setStep] = useState(0)
  const [server, setServer] = useState<ServerDraft>(newServer)
  const [company, setCompany] = useState<CompanyDraft>(newCompany)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [serverReady, setServerReady] = useState(false)

  const changeServer = <K extends keyof ServerDraft>(key: K, value: ServerDraft[K]) => setServer((current) => ({ ...current, [key]: value }))
  const changeCompany = <K extends keyof CompanyDraft>(key: K, value: CompanyDraft[K]) => setCompany((current) => ({ ...current, [key]: value }))

  const submitServer = async () => {
    if (!server.base_url.trim() || !server.token.trim()) { setError('Informe a base URL e o token admin da API WhatsApp contratada.'); return }
    setBusy(true); setError(null)
    try {
      await request('/api/react/platform/whatsapp-servers', {
        method: 'POST',
        body: JSON.stringify({ name: server.name.trim() || 'WhatsApp principal', base_url: server.base_url.trim(), token: server.token.trim(), capacity: Math.max(0, Number(server.capacity) || 0), status: 'active' }),
      })
      setServerReady(true)
      setStep(1)
    } catch (cause) {
      setError(messageOf(cause, 'Não foi possível cadastrar a API WhatsApp.'))
    } finally {
      setBusy(false)
    }
  }

  const submitCompany = async () => {
    const name = company.name.trim()
    const slug = slugify(company.slug || company.name)
    if (!name || !slug) { setError('Informe o nome da empresa.'); return }
    if (!company.admin_name.trim() || !company.admin_username.trim() || company.admin_password.length < 6) {
      setError('Informe o nome, o usuário e uma senha temporária (mínimo 6 caracteres) do primeiro administrador.')
      return
    }
    setBusy(true); setError(null)
    try {
      await request('/api/react/platform/tenants', {
        method: 'POST',
        body: JSON.stringify({
          name,
          slug,
          plan: 'Essencial',
          status: 'active',
          timezone: 'America/Sao_Paulo',
          locale: 'pt-BR',
          dominio: company.domain.trim(),
          admin: { name: company.admin_name.trim(), username: company.admin_username.trim(), temporary_password: company.admin_password },
        }),
      })
      onDone('empresas')
    } catch (cause) {
      setError(messageOf(cause, 'Não foi possível criar a empresa.'))
    } finally {
      setBusy(false)
    }
  }

  const progress = ((step + 1) / steps.length) * 100

  return (
    <div className="platform-content">
      <div className="platform-content-heading">
        <div>
          <span className="eyebrow">Primeiros passos · plataforma</span>
          <h3>Deixe a plataforma pronta em 2 passos</h3>
          <p>Conecte a API WhatsApp da Notreve e crie a primeira empresa com o administrador dela. Nada vem preenchido por padrão.</p>
        </div>
      </div>

      <div className="platform-setup-steps">
        {steps.map((item) => (
          <span key={item.id} className={item.id === step ? 'is-active' : item.id < step ? 'is-done' : ''}>
            <b>{item.id < step ? '✓' : item.id + 1}</b> {item.label}
          </span>
        ))}
        <div className="platform-setup-progress"><span style={{ width: `${progress}%` }} /></div>
      </div>

      {error && <div className="workspace-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}

      {step === 0 ? (
        <FormCard title="Notreve WhatsApp API" text="Informe a base URL e o token admin da API que você contratou com a Notreve. É o pool que atende todas as empresas.">
          <div className="platform-form-grid">
            <Field label="Nome do servidor"><input value={server.name} placeholder="WhatsApp produção" onChange={(event) => changeServer('name', event.target.value)} /></Field>
            <Field label="Base URL *"><input value={server.base_url} placeholder="https://sua-api.notreve.com.br" onChange={(event) => changeServer('base_url', event.target.value)} /></Field>
            <Field label="Token admin *"><input type="password" autoComplete="new-password" value={server.token} placeholder="Cole o token admin" onChange={(event) => changeServer('token', event.target.value)} /></Field>
            <Field label="Capacidade de conexões"><input type="number" min="0" step="1" value={server.capacity} onChange={(event) => changeServer('capacity', Math.max(0, Number(event.target.value) || 0))} /></Field>
          </div>
          <div className="form-actions">
            <button className="primary-button" type="button" disabled={busy} onClick={() => void submitServer()}>{busy ? 'Salvando…' : 'Salvar e continuar'} →</button>
          </div>
        </FormCard>
      ) : (
        <FormCard title="Primeira empresa" text="Crie a empresa e o administrador dela. O admin entra no próprio workspace e conclui a configuração (provedor de IA → agente → canal).">
          {serverReady && <p className="platform-setup-hint">API WhatsApp cadastrada. Agora a empresa.</p>}
          <div className="platform-form-grid">
            <Field label="Nome da empresa *"><input value={company.name} placeholder="Ex.: Acme Serviços" onChange={(event) => changeCompany('name', event.target.value)} /></Field>
            <Field label="Slug do workspace"><input value={company.slug} placeholder="acme-servicos" onChange={(event) => changeCompany('slug', slugify(event.target.value))} /></Field>
            <Field label="Domínio do painel" help="Onde o admin e a equipe entram (ex.: app.acme.com.br)."><input value={company.domain} placeholder="app.acme.com.br" onChange={(event) => changeCompany('domain', event.target.value.trim())} /></Field>
          </div>
          <div className="platform-form-grid" style={{ marginTop: 12 }}>
            <Field label="Nome do administrador *"><input value={company.admin_name} placeholder="Ex.: Maria Silva" onChange={(event) => changeCompany('admin_name', event.target.value)} /></Field>
            <Field label="Usuário ou e-mail *"><input autoComplete="off" value={company.admin_username} placeholder="maria@acme.com.br" onChange={(event) => changeCompany('admin_username', event.target.value)} /></Field>
            <Field label="Senha temporária *"><input type="password" autoComplete="new-password" value={company.admin_password} placeholder="mínimo 6 caracteres" onChange={(event) => changeCompany('admin_password', event.target.value)} /></Field>
          </div>
          <div className="form-actions">
            <button className="secondary-button" type="button" disabled={busy} onClick={() => setStep(0)}>← Voltar</button>
            <button className="primary-button" type="button" disabled={busy} onClick={() => void submitCompany()}>{busy ? 'Criando…' : 'Criar empresa e finalizar'} →</button>
          </div>
        </FormCard>
      )}
    </div>
  )
}

function Field({ label, help, children }: { label: string; help?: string; children: ReactNode }) {
  return <label className="field"><span>{label}</span>{children}{help && <small>{help}</small>}</label>
}

function FormCard({ title, text, children }: { title: string; text: string; children: ReactNode }) {
  return <div className="platform-form-card"><div className="form-heading"><div><strong>{title}</strong><p>{text}</p></div></div>{children}</div>
}

export default PlatformSetup
