import { useCallback, useEffect, useState } from 'react'
import { createConsent, createPolitica, listAudit, listConsents, listPoliticas, loadLGPDSummary } from './lgpdApi'
import type { AuditLog, LGPDConsent, LGPDPolitica, LGPDSummary } from './types'
import { FilterBar, FilterSearch, FilterSelect, Pagination, DEFAULT_PAGE_SIZE, encodeCursor } from '../../shared/Pagination'
import './Lgpd.css'

type LgpdPageProps = {
  onBack?: () => void
  permissions?: string[] | null
}

type Tab = 'consents' | 'politica' | 'audit'

export function LgpdPage({ onBack, permissions = null }: LgpdPageProps) {
  const canManage = permissions == null || permissions.includes('config_gerenciar')
  const [tab, setTab] = useState<Tab>('consents')
  const [summary, setSummary] = useState<LGPDSummary | null>(null)
  const [consents, setConsents] = useState<LGPDConsent[]>([])
  const [consentsTotal, setConsentsTotal] = useState(0)
  const [politicas, setPoliticas] = useState<LGPDPolitica[]>([])
  const [audit, setAudit] = useState<AuditLog[]>([])
  const [auditTotal, setAuditTotal] = useState(0)
  const [consentsOffset, setConsentsOffset] = useState(0)
  const [consentScope, setConsentScope] = useState('')
  const [auditOffset, setAuditOffset] = useState(0)
  const [auditQuery, setAuditQuery] = useState('')
  const [auditAcao, setAuditAcao] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const loadBase = useCallback(async () => {
    try {
      const data = await loadLGPDSummary()
      setSummary(data)
      setError(null)
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar a LGPD.')
    }
  }, [])

  const loadConsents = useCallback(async () => {
    try {
      const data = await listConsents(consentsOffset > 0 ? encodeCursor(consentsOffset) : '', consentScope)
      setConsents(data.items || [])
      setConsentsTotal(data.total)
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar os consentimentos.')
    }
  }, [consentsOffset, consentScope])

  const loadPoliticas = useCallback(async () => {
    try {
      const data = await listPoliticas()
      setPoliticas(data || [])
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar a política.')
    }
  }, [])

  const loadAudit = useCallback(async () => {
    try {
      const data = await listAudit(auditOffset > 0 ? encodeCursor(auditOffset) : '', auditQuery.trim(), auditAcao)
      setAudit(data.items || [])
      setAuditTotal(data.total)
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar a auditoria.')
    }
  }, [auditOffset, auditQuery, auditAcao])

  useEffect(() => {
    let active = true
    setLoading(true)
    void loadBase()
    void loadConsents()
    void loadPoliticas()
    void loadAudit().finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [loadBase, loadConsents, loadPoliticas, loadAudit])

  useEffect(() => { setConsentsOffset(0) }, [consentScope])
  useEffect(() => { setAuditOffset(0) }, [auditQuery, auditAcao])

  return (
    <div className="lgpd-page">
      <section className="settings-intro">
        <div>
          <span className="eyebrow">Privacidade · Empresa</span>
          <h2>LGPD e consentimento.</h2>
          <p>Publique a política de privacidade, registre consentimentos e acompanhe os acessos aos dados.</p>
        </div>
        <div className="settings-intro-actions">
          {onBack && <button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button>}
          <button className="secondary-button" type="button" disabled={loading} onClick={() => { setLoading(true); void Promise.all([loadBase(), loadConsents(), loadPoliticas(), loadAudit()]).finally(() => setLoading(false)) }}>↻ Atualizar</button>
        </div>
      </section>

      {error && <div className="settings-alert" role="alert"><span>!</span><p>{error}</p><button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
      {notice && <div className="settings-notice" role="status"><span>✓</span><p>{notice}</p><button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso">×</button></div>}

      {loading && !summary ? <div className="settings-state"><span className="settings-spinner" /> Carregando LGPD…</div> : <div className="lgpd-content">
        {summary && (
          <div className={`lgpd-status ${summary.encrypted ? 'enabled' : 'disabled'}`}>
            <span className="lgpd-status-dot" aria-hidden="true" />
            <div>
              <strong>{summary.encrypted ? 'Criptografia em repouso ativa' : 'Criptografia em repouso ausente'}</strong>
              <p>{summary.encrypted ? 'Campos sensíveis são cifrados com AES-GCM usando a chave da variável de ambiente AGENTE_IA_LGPD_KEY.' : 'Defina AGENTE_IA_LGPD_KEY para cifrar documentos sensíveis. Sem a chave, os campos são armazenados em texto plano.'}</p>
            </div>
          </div>
        )}

        <nav className="lgpd-tabs" aria-label="Seções da LGPD">
          <button type="button" className={tab === 'consents' ? 'active' : ''} onClick={() => setTab('consents')}>Consentimentos{consentsTotal > 0 ? ` (${consentsTotal})` : ''}</button>
          <button type="button" className={tab === 'politica' ? 'active' : ''} onClick={() => setTab('politica')}>Política e termos</button>
          <button type="button" className={tab === 'audit' ? 'active' : ''} onClick={() => setTab('audit')}>Auditoria{auditTotal > 0 ? ` (${auditTotal})` : ''}</button>
        </nav>

        {tab === 'consents' && <ConsentsTab summary={summary} items={consents} scopeFilter={consentScope} onScopeFilterChange={setConsentScope} total={consentsTotal} offset={consentsOffset} onOffsetChange={setConsentsOffset} canManage={canManage} onCreated={() => { void loadConsents(); void loadBase() }} setNotice={setNotice} />}
        {tab === 'politica' && <PoliticaTab politica={summary?.politica ?? null} historico={politicas} canManage={canManage} onPublished={() => { void loadBase(); void loadPoliticas() }} setNotice={setNotice} />}
        {tab === 'audit' && <AuditTab items={audit} total={auditTotal} offset={auditOffset} onOffsetChange={setAuditOffset} query={auditQuery} onQueryChange={setAuditQuery} acao={auditAcao} onAcaoChange={setAuditAcao} />}
      </div>}
    </div>
  )
}

function ConsentsTab({ summary, items, scopeFilter, onScopeFilterChange, total, offset, onOffsetChange, canManage, onCreated, setNotice }: {
  summary: LGPDSummary | null
  items: LGPDConsent[]
  scopeFilter: string
  onScopeFilterChange: (value: string) => void
  total: number
  offset: number
  onOffsetChange: (value: number) => void
  canManage: boolean
  onCreated: () => void
  setNotice: (value: string) => void
}) {
  const [contatoId, setContatoId] = useState('')
  const [scope, setScope] = useState('contato')
  const [baseLegal, setBaseLegal] = useState('consentimento')
  const [canal, setCanal] = useState('whatsapp')
  const [aceito, setAceito] = useState(true)
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const register = async () => {
    const contato = parseInt(contatoId, 10)
    if (!Number.isFinite(contato) || contato < 1) {
      setFormError('Informe um contato válido.')
      return
    }
    setSaving(true)
    setFormError(null)
    try {
      const { id } = await createConsent({ contato_id: contato, scope, base_legal: baseLegal, canal, aceito })
      setNotice(`Consentimento #${id} registrado.`)
      onCreated()
      setContatoId('')
    } catch (saveError) {
      setFormError(saveError instanceof Error ? saveError.message : 'Não foi possível registrar o consentimento.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="lgpd-tab-stack">
      {canManage && <section className="lgpd-card">
        <header className="lgpd-card-header">
          <div><span className="eyebrow">Base legal</span><h3>Registrar consentimento</h3><p>Associe um contato a uma base legal para registrar quando e por qual canal o dado foi autorizado.</p></div>
        </header>
        <div className="lgpd-form">
          <label><span>Contato (ID)</span><input type="number" min="1" value={contatoId} onChange={(event) => setContatoId(event.target.value)} placeholder="123" /></label>
          <label><span>Escopo</span><input value={scope} onChange={(event) => setScope(event.target.value)} /></label>
          <label><span>Base legal</span><input value={baseLegal} onChange={(event) => setBaseLegal(event.target.value)} /></label>
          <label><span>Canal</span><input value={canal} onChange={(event) => setCanal(event.target.value)} /></label>
          <label className="lgpd-check"><span>Registrar aceite</span><input type="checkbox" checked={aceito} onChange={(event) => setAceito(event.target.checked)} /></label>
          <button className="primary-button" type="button" disabled={saving} onClick={() => void register()}>{saving ? 'Registrando…' : 'Registrar consentimento'} <span aria-hidden="true">↗</span></button>
        </div>
        {formError && <p className="lgpd-form-error">{formError}</p>}
      </section>}

      <section className="lgpd-card">
        <header className="lgpd-card-header"><div><span className="eyebrow">Registros</span><h3>Consentimentos registrados</h3><p>Histórico de autorizações coletadas, com escopo e base legal.</p></div></header>
        <FilterBar resultLabel={`${total} consentimento${total === 1 ? '' : 's'}`} onReset={() => onScopeFilterChange('')}>
          <FilterSearch value={scopeFilter} onChange={onScopeFilterChange} placeholder="Filtrar por escopo" />
        </FilterBar>
        {items.length === 0 ? <div className="workspace-empty"><span>◎</span><strong>Nenhum consentimento registrado</strong><p>Os aceites aparecerão aqui conforme forem coletados.</p></div> : <table className="lgpd-table">
          <thead><tr><th>Quando</th><th>Contato</th><th>Escopo</th><th>Base legal</th><th>Canal</th><th>Aceite</th></tr></thead>
          <tbody>{items.map((entry) => <tr key={entry.id}><td>{new Date(entry.data).toLocaleString('pt-BR')}</td><td>{entry.contato_id ?? '—'}</td><td>{entry.scope}</td><td>{entry.base_legal}</td><td>{entry.canal || '—'}</td><td>{entry.aceito ? 'Sim' : 'Não'}</td></tr>)}</tbody>
        </table>}
        {items.length > 0 && <Pagination total={total} limit={DEFAULT_PAGE_SIZE} offset={offset} onOffsetChange={onOffsetChange} />}
      </section>
    </div>
  )
}

function PoliticaTab({ politica, historico, canManage, onPublished, setNotice }: {
  politica: LGPDPolitica | null
  historico: LGPDPolitica[]
  canManage: boolean
  onPublished: () => void
  setNotice: (value: string) => void
}) {
  const [tipo, setTipo] = useState('privacidade')
  const [texto, setTexto] = useState('')
  const [url, setUrl] = useState('')
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const publish = async () => {
    if (!texto.trim() && !url.trim()) {
      setFormError('Informe o texto ou a URL da política.')
      return
    }
    setSaving(true)
    setFormError(null)
    try {
      const created = await createPolitica({ tipo, texto: texto.trim(), url: url.trim() })
      setNotice(`Política v${created.version} publicada.`)
      onPublished()
      setTexto('')
      setUrl('')
    } catch (saveError) {
      setFormError(saveError instanceof Error ? saveError.message : 'Não foi possível publicar a política.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="lgpd-tab-stack">
      <section className="lgpd-card">
        <header className="lgpd-card-header">
          <div><span className="eyebrow">{politica ? `Versão vigente v${politica.version}` : 'Sem versão vigente'}</span><h3>{politica ? politica.tipo === 'termos' ? 'Termos de uso' : 'Política de privacidade' : 'Publicar política ou termos'}</h3><p>{politica ? (politica.url || politica.texto) : 'Publique uma nova versão para exibir aos clientes e registrar a circulação de dados.'}</p></div>
        </header>
        {canManage && <><div className="lgpd-form">
          <label><span>Tipo</span><select value={tipo} onChange={(event) => setTipo(event.target.value)}><option value="privacidade">Privacidade</option><option value="termos">Termos de uso</option></select></label>
          <label className="lgpd-wide"><span>Texto</span><textarea rows={5} value={texto} onChange={(event) => setTexto(event.target.value)} placeholder="Conteúdo integral da política…" /></label>
          <label className="lgpd-wide"><span>URL (opcional)</span><input value={url} onChange={(event) => setUrl(event.target.value)} placeholder="https://…" /></label>
          <button className="primary-button" type="button" disabled={saving} onClick={() => void publish()}>{saving ? 'Publicando…' : 'Publicar nova versão'} <span aria-hidden="true">↗</span></button>
        </div>
        {formError && <p className="lgpd-form-error">{formError}</p>}</>}
      </section>

      {historico.length > 0 && (
        <section className="lgpd-card">
          <header className="lgpd-card-header"><div><span className="eyebrow">Histórico</span><h3>Versões publicadas</h3><p>Cada aceite registrado fica vinculado à versão vigente no momento da coleta.</p></div></header>
          <ul className="lgpd-list">
            {historico.map((version) => (
              <li key={version.id} className={version.active ? 'active' : ''}>
                <div className="lgpd-list-main"><strong>v{version.version} · {version.tipo}</strong><span>{version.url || (version.texto ? version.texto.slice(0, 120) : '')}</span></div>
                <span className="lgpd-list-meta">{new Date(version.created_at).toLocaleString('pt-BR')}</span>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  )
}

function AuditTab({ items, total, offset, onOffsetChange, query, onQueryChange, acao, onAcaoChange }: { items: AuditLog[]; total: number; offset: number; onOffsetChange: (value: number) => void; query: string; onQueryChange: (value: string) => void; acao: string; onAcaoChange: (value: string) => void }) {
  if (items.length === 0) {
    return <section className="lgpd-card"><header className="lgpd-card-header"><div><span className="eyebrow">Trilha de auditoria</span><h3>Nenhum evento registrado</h3><p>Os acessos a contatos, mensagens e login aparecerão aqui.</p></div></header><FilterBar resultLabel="0 eventos" onReset={() => { onQueryChange(''); onAcaoChange('') }}><FilterSearch value={query} onChange={onQueryChange} placeholder="Buscar ação ou recurso" /><FilterSelect value={acao} onChange={onAcaoChange} label="Filtrar ação" options={[{ value: '', label: 'Todas as ações' }, { value: 'login', label: 'Login' }, { value: 'contato_visualizado', label: 'Contato visualizado' }, { value: 'politica_publicada', label: 'Política publicada' }]} /></FilterBar></section>
  }
  return (
    <section className="lgpd-card">
      <header className="lgpd-card-header"><div><span className="eyebrow">Trilha de auditoria</span><h3>Eventos registrados</h3><p>Login, acessos a contatos/mensagens e ações de dados são registrados automaticamente.</p></div></header>
      <FilterBar resultLabel={`${total} evento${total === 1 ? '' : 's'}`} onReset={() => { onQueryChange(''); onAcaoChange('') }}>
        <FilterSearch value={query} onChange={onQueryChange} placeholder="Buscar ação ou recurso" />
        <FilterSelect value={acao} onChange={onAcaoChange} label="Filtrar ação" options={[{ value: '', label: 'Todas as ações' }, { value: 'login', label: 'Login' }, { value: 'contato_visualizado', label: 'Contato visualizado' }, { value: 'politica_publicada', label: 'Política publicada' }]} />
      </FilterBar>
      <table className="lgpd-table">
        <thead><tr><th>Quando</th><th>Usuário</th><th>Ação</th><th>Recurso</th><th>Entidade</th><th>IP</th></tr></thead>
        <tbody>
          {items.map((entry) => (
            <tr key={entry.id}>
              <td>{new Date(entry.data).toLocaleString('pt-BR')}</td>
              <td>{entry.usuario || `#${entry.usuario_id ?? ''}`}</td>
              <td>{entry.acao}</td>
              <td>{entry.recurso}</td>
              <td>{entry.entidade_id ?? '—'}</td>
              <td>{entry.ip || '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <Pagination total={total} limit={DEFAULT_PAGE_SIZE} offset={offset} onOffsetChange={onOffsetChange} />
    </section>
  )
}
