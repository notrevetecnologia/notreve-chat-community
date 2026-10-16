import { useEffect, useState } from 'react'
import './ApiDocs.css'

type DocEnvelope<T> = { data?: T; error?: { message?: string } }
type Scope = { scope: string; what: string }
type DocBody = { content_type?: string; example?: string }
type Endpoint = { method: string; path: string; scope: string; summary: string; description: string; request?: DocBody; response?: DocBody; errors?: string[] }
type WebhookDef = { key: string; name: string; description: string; version: number }
type WebhookHeader = { name: string; what: string }
type ErrorRow = { http: string; code: string; action: string }
type ApiDocs = {
  base_path: string
  auth: { bearer: string; header_hint: string; key_prefix: string }
  scopes: Scope[]
  endpoints: Endpoint[]
  webhooks: { catalog: WebhookDef[]; envelope: string; signature: string; headers: WebhookHeader[]; retry: string; dedupe: string; active: boolean; provisional?: boolean }
  errors: ErrorRow[]
  limits: { max_json_bytes: string; max_text_runes: string; max_name_runes: string; max_reason_runes: string; idempotency_ttl: string; rate_limit: string }
}

async function request<T>(url: string): Promise<T> {
  const response = await fetch(url, { credentials: 'same-origin' })
  const payload = await response.json() as DocEnvelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível carregar a documentação.')
  return payload.data as T
}

const methodClass = (method: string) => ({ GET: 'is-get', POST: 'is-post', PATCH: 'is-patch', DELETE: 'is-delete' }[method] || 'is-post')

export function ApiDocsPanel({ onBack }: { onBack: () => void }) {
  const [docs, setDocs] = useState<ApiDocs | null>(null)
  const [tab, setTab] = useState<'intro' | 'auth' | 'endpoints' | 'webhooks' | 'errors'>('intro')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    void (async () => {
      setLoading(true)
      try { setDocs(await request<ApiDocs>('/api/react/api-docs')); setError(null) }
      catch (loadError) { setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar a documentação.') }
      finally { setLoading(false) }
    })()
  }, [])

  return <section className="api-docs">
    <div className="integrations-section-heading">
      <div><span className="eyebrow">Para desenvolvedores</span><h3>Documentação da API</h3><p>Guia de integração para o seu ERP, site ou automação. Tudo roda dentro da sua empresa (tenant) — a chave nunca acessa dados de outra.</p></div>
      <button className="secondary-button" type="button" onClick={onBack}>← Voltar</button>
    </div>
    <div className="api-docs-tabs">
      {([['intro', 'Introdução'], ['auth', 'Autenticação'], ['endpoints', 'Endpoints'], ['webhooks', 'Webhooks'], ['errors', 'Erros e limites']] as const).map(([key, label]) => <button key={key} className={`api-docs-tab ${tab === key ? 'is-active' : ''}`} type="button" onClick={() => setTab(key)}>{label}</button>)}
    </div>
    {loading && <div className="integrations-state"><span className="integrations-spinner" /> Carregando…</div>}
    {error && <div className="integrations-alert" role="alert"><span>!</span>{error}</div>}
    {docs && tab === 'intro' && <Intro docs={docs} />}
    {docs && tab === 'auth' && <Auth docs={docs} />}
    {docs && tab === 'endpoints' && <Endpoints docs={docs} />}
    {docs && tab === 'webhooks' && <Webhooks docs={docs} />}
    {docs && tab === 'errors' && <Errors docs={docs} />}
  </section>
}

function Code({ children }: { children: string }) {
  return <pre className="api-docs-code"><code>{children}</code></pre>
}

function Intro({ docs }: { docs: ApiDocs }) {
  return <div className="api-docs-section">
    <h4>Comece em 4 passos</h4>
    <ol className="api-docs-steps">
      <li>O administrador da plataforma habilita o módulo <strong>API e webhooks</strong>.</li>
      <li>Em <strong>Chaves</strong>, crie uma chave com os escopos <code>conversations:read</code> e <code>conversations:write</code>.</li>
      <li>Copie o segredo exibido na criação (ele não aparece de novo) e use como <code>Authorization: Bearer &lt;chave&gt;</code>.</li>
      <li>Use a URL base <code>{docs.base_path}</code> nos exemplos.</li>
    </ol>
    <p className="api-docs-paragraph">Toda requisição de escrita exige <code>Idempotency-Key</code>. Respostas são <code>{'{ data: ... }'}</code> em sucesso e <code>{'{ error: { code, message } }'}</code> em erro.</p>
    <div className="api-docs-note"><strong>O mínimo para enviar uma mensagem:</strong><Code>{`curl -X POST "${docs.base_path}/api/v1/conversations" \\
  -H "Authorization: Bearer nrv_..." \\
  -H "Idempotency-Key: pedido-8472" \\
  -H "Content-Type: application/json" \\
  -d '{"channel_id":12,"customer_id":"5511999999999","customer_name":"Ana Silva"}'`}</Code></div>
  </div>
}

function Auth({ docs }: { docs: ApiDocs }) {
  return <div className="api-docs-section">
    <h4>Autenticação</h4>
    <p className="api-docs-paragraph">{docs.auth.header_hint}</p>
    <div className="api-docs-note"><Code>{`Authorization: Bearer ${docs.auth.bearer}`}</Code></div>
    <h4>Escopos</h4>
    <table className="api-docs-table"><thead><tr><th>Escopo</th><th>Permite</th></tr></thead><tbody>{docs.scopes.map((s) => <tr key={s.scope}><td><code>{s.scope}</code></td><td>{s.what}</td></tr>)}</tbody></table>
  </div>
}

function Endpoints({ docs }: { docs: ApiDocs }) {
  return <div className="api-docs-section">
    <div className="api-docs-endpoints">{docs.endpoints.map((endpoint) => <article className="api-docs-endpoint" key={`${endpoint.method}-${endpoint.path}`}><header><span className={`api-docs-method ${methodClass(endpoint.method)}`}>{endpoint.method}</span><code className="api-docs-path">{endpoint.path}</code><span className="api-docs-scope">{endpoint.scope}</span></header><h4>{endpoint.summary}</h4><p>{endpoint.description}</p>{endpoint.request && <div className="api-docs-note"><strong>Requisição</strong><Code>{endpoint.request.example || ''}</Code></div>}{endpoint.response && <div className="api-docs-note"><strong>Resposta</strong><Code>{endpoint.response.example || ''}</Code></div>}{endpoint.errors && endpoint.errors.length > 0 && <div className="api-docs-hint"><strong>Erros possíveis:</strong> {endpoint.errors.join(', ')}</div>}</article>)}</div>
  </div>
}

function Webhooks({ docs }: { docs: ApiDocs }) {
  const wh = docs.webhooks
  return <div className="api-docs-section">
    <div className="api-docs-status-line"><span className={`integrations-key-status ${wh.active ? 'active' : ''}`} /><strong>{wh.active ? 'Webhooks ativos em produção' : 'Webhooks ainda não ativos'}</strong>{wh.provisional && <span className="api-docs-badge is-warn">contrato previsto</span>}</div>
    <p className="api-docs-paragraph">Configure uma assinatura em <strong>Webhooks</strong> com os eventos que quiser receber. Cada evento vira um POST HTTPS para o seu endpoint.</p>
    <h4>Envelope</h4>
    <div className="api-docs-note"><Code>{wh.envelope}</Code></div>
    <h4>Assinatura</h4>
    <p className="api-docs-paragraph">{wh.signature}</p>
    <h4>Cabeçalhos</h4>
    <table className="api-docs-table"><thead><tr><th>Cabeçalho</th><th>O que é</th></tr></thead><tbody>{wh.headers.map((h) => <tr key={h.name}><td><code>{h.name}</code></td><td>{h.what}</td></tr>)}</tbody></table>
    <p className="api-docs-paragraph">{wh.retry}</p>
    <h4>Eventos</h4>
    <div className="api-docs-events">{wh.catalog.map((event) => <div className="api-docs-event" key={event.key}><code>{event.key}</code><span>{event.name}</span></div>)}</div>
  </div>
}

function Errors({ docs }: { docs: ApiDocs }) {
  return <div className="api-docs-section">
    <h4>Erros para tratar</h4>
    <table className="api-docs-table"><thead><tr><th>HTTP</th><th>Código</th><th>Ação</th></tr></thead><tbody>{docs.errors.map((row) => <tr key={row.http + row.code}><td className="api-docs-http">{row.http}</td><td><code>{row.code}</code></td><td>{row.action}</td></tr>)}</tbody></table>
    <h4>Limites e idempotência</h4>
    <table className="api-docs-table"><thead><tr><th>Item</th><th>Valor</th></tr></thead><tbody>
      <tr><td>Corpo JSON</td><td>{docs.limits.max_json_bytes}</td></tr>
      <tr><td>Conteúdo de uma mensagem</td><td>{docs.limits.max_text_runes}</td></tr>
      <tr><td>Nome do cliente</td><td>{docs.limits.max_name_runes}</td></tr>
      <tr><td>Motivo de encerramento</td><td>{docs.limits.max_reason_runes}</td></tr>
      <tr><td>Retenção da Idempotency-Key</td><td>{docs.limits.idempotency_ttl}</td></tr>
      <tr><td>Rate limit</td><td>{docs.limits.rate_limit}</td></tr>
    </tbody></table>
  </div>
}
