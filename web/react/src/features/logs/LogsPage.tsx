import { useCallback, useEffect, useMemo, useState } from 'react'
import { FilterBar, FilterSearch, FilterSelect, Pagination, DEFAULT_PAGE_SIZE, encodeCursor, type PaginationMeta } from '../../shared/Pagination'
import './Logs.css'

type LogEntry = {
  id: number
  ts: string
  level: string
  module: string
  message: string
  extra: Record<string, unknown> | null
  traceback: string
  request_id: string
}

type LogStats = {
  total: number
  by_level: Record<string, number>
  by_module: Record<string, number>
  errors_last_hour: number
}

type Envelope<T> = { data?: T; meta?: PaginationMeta; error?: { message?: string } }
type LogsView = { items: LogEntry[] }

const levelOptions = [
  { value: '', label: 'Todos os níveis' },
  { value: 'DEBUG', label: 'Debug' },
  { value: 'INFO', label: 'Info' },
  { value: 'WARNING', label: 'Aviso' },
  { value: 'ERROR', label: 'Erro' },
  { value: 'CRITICAL', label: 'Crítico' },
]

const levelLabels: Record<string, string> = {
  DEBUG: 'Debug',
  INFO: 'Info',
  WARNING: 'Aviso',
  ERROR: 'Erro',
  CRITICAL: 'Crítico',
}

async function request<T>(url: string): Promise<T> {
  const response = await fetch(url, { credentials: 'same-origin', headers: { 'Content-Type': 'application/json' } })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return payload.data as T
}

async function requestEnvelope<T>(url: string): Promise<{ data: T; meta: PaginationMeta }> {
  const response = await fetch(url, { credentials: 'same-origin', headers: { 'Content-Type': 'application/json' } })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return { data: payload.data as T, meta: payload.meta || {} }
}

function formatDate(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return date.toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'medium' })
}

function extraText(entry: LogEntry): string {
  if (!entry.extra || Object.keys(entry.extra).length === 0) return ''
  try {
    return JSON.stringify(entry.extra, null, 2)
  } catch {
    return String(entry.extra)
  }
}

export default function LogsPage({ onBack }: { onBack?: () => void }) {
  const [items, setItems] = useState<LogEntry[]>([])
  const [stats, setStats] = useState<LogStats | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [level, setLevel] = useState('')
  const [module, setModule] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [offset, setOffset] = useState(0)
  const [total, setTotal] = useState(0)
  const [expandedID, setExpandedID] = useState<number | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const params = new URLSearchParams({ limit: String(DEFAULT_PAGE_SIZE) })
      if (query.trim()) params.set('q', query.trim())
      if (level) params.set('level', level)
      if (module) params.set('module', module)
      if (from) params.set('from', from)
      if (to) params.set('to', to)
      if (offset > 0) params.set('cursor', encodeCursor(offset))
      const payload = await requestEnvelope<LogsView>(`/api/react/logs?${params.toString()}`)
      setItems(payload.data.items || [])
      setTotal(payload.meta.total ?? payload.data.items?.length ?? 0)
      setError(null)
    } catch (loadError) {
      setItems([])
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar os logs.')
    } finally {
      setLoading(false)
    }
  }, [query, level, module, from, to, offset])

  const loadStats = useCallback(async () => {
    try {
      setStats(await request<LogStats>('/api/react/logs/stats'))
    } catch {
      setStats(null)
    }
  }, [])

  useEffect(() => { void load() }, [load])
  useEffect(() => { void loadStats() }, [loadStats])
  useEffect(() => { setOffset(0) }, [query, level, module, from, to])

  const moduleOptions = useMemo(() => {
    const modules = Object.keys(stats?.by_module || {}).filter(Boolean).sort()
    return [{ value: '', label: 'Todos os módulos' }, ...modules.map((value) => ({ value, label: value }))]
  }, [stats])

  const errorsByLevel = useMemo(() => {
    const byLevel = stats?.by_level || {}
    return (byLevel.ERROR || 0) + (byLevel.CRITICAL || 0)
  }, [stats])

  const reset = () => { setQuery(''); setLevel(''); setModule(''); setFrom(''); setTo('') }

  return <div className="logs-page">
    <section className="logs-intro">
      <div>
        <span className="eyebrow">Administração</span>
        <h2>Logs do sistema</h2>
        <p>Eventos estruturados da operação: geração de mídia, integrações, canais e erros. Filtre por nível, módulo, período ou busque no conteúdo. A retenção é configurável em Configurações.</p>
      </div>
      <div className="logs-intro-actions">
        {onBack && <button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button>}
        <button className="secondary-button" type="button" disabled={loading} onClick={() => { void load(); void loadStats() }}>↻ Atualizar</button>
      </div>
    </section>

    {error && <div className="logs-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}

    <div className="logs-stats">
      <article><span>Total</span><strong>{stats ? stats.total : '—'}</strong><small>entradas retidas</small></article>
      <article className={errorsByLevel > 0 ? 'is-alert' : ''}><span>Erros e críticos</span><strong>{stats ? errorsByLevel : '—'}</strong><small>todos os módulos</small></article>
      <article className={stats && stats.errors_last_hour > 0 ? 'is-alert' : ''}><span>Última hora</span><strong>{stats ? stats.errors_last_hour : '—'}</strong><small>erros recentes</small></article>
    </div>

    <FilterBar resultLabel={`${total} log${total === 1 ? '' : 's'}`} onReset={reset}>
      <FilterSearch value={query} onChange={setQuery} placeholder="Buscar na mensagem ou traceback" />
      <FilterSelect value={level} onChange={setLevel} label="Filtrar nível" options={levelOptions} />
      <FilterSelect value={module} onChange={setModule} label="Filtrar módulo" options={moduleOptions} />
      <label className="logs-date-field"><span>De</span><input type="date" value={from} onChange={(event) => setFrom(event.target.value)} /></label>
      <label className="logs-date-field"><span>Até</span><input type="date" value={to} onChange={(event) => setTo(event.target.value)} /></label>
    </FilterBar>

    {loading ? <div className="logs-state">Carregando logs…</div>
      : items.length === 0 ? <div className="logs-empty"><strong>Nenhum log encontrado</strong><p>Ajuste os filtros ou aguarde novos eventos da operação.</p></div>
      : <div className="logs-list">{items.map((entry) => {
        const expanded = expandedID === entry.id
        const extra = extraText(entry)
        return <article className={`log-entry is-${entry.level.toLowerCase()}`} key={entry.id}>
          <button type="button" className="log-entry-head" onClick={() => setExpandedID(expanded ? null : entry.id)} aria-expanded={expanded}>
            <span className={`log-level is-${entry.level.toLowerCase()}`}>{levelLabels[entry.level] || entry.level}</span>
            <span className="log-module">{entry.module || '—'}</span>
            <span className="log-message">{entry.message}</span>
            <span className="log-ts">{formatDate(entry.ts)}</span>
            <span className="log-chevron" aria-hidden="true">{expanded ? '▾' : '▸'}</span>
          </button>
          {expanded && <div className="log-entry-detail">
            {entry.request_id && <p className="log-request"><span>request_id</span><code>{entry.request_id}</code></p>}
            {extra && <div className="log-block"><span>Detalhes</span><pre>{extra}</pre></div>}
            {entry.traceback && <div className="log-block"><span>Traceback</span><pre>{entry.traceback}</pre></div>}
            {!extra && !entry.traceback && !entry.request_id && <p className="log-muted">Sem detalhes adicionais.</p>}
          </div>}
        </article>
      })}</div>}

    {!loading && items.length > 0 && <Pagination total={total} limit={DEFAULT_PAGE_SIZE} offset={offset} onOffsetChange={setOffset} loading={loading} />}
  </div>
}
