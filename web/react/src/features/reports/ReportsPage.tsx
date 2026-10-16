import { Children, useCallback, useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react'
import { Pagination, DEFAULT_PAGE_SIZE, encodeCursor } from '../../shared/Pagination'
import './Reports.css'

type Period = '1' | '7' | '30' | 'custom'

type ReportOption = { id: number; name: string }

type ReportsFilters = {
  period: string
  from: string
  to: string
  channel?: string
  sector_id?: number
  attendant_id?: number
}

type ReportsSummary = {
  conversations: number
  active: number
  waiting: number
  closed: number
  messages: number
  customer_messages: number
  ai_messages: number
  human_messages: number
  evaluated_conversations: number
  rating_count: number
  overall_rating: number
  overall_rating_count: number
  ai_rating: number
  ai_rating_count: number
  human_rating: number
  human_rating_count: number
  avg_response_time: number
  avg_response_time_count: number
  transfer_rate: number
  transferred_count: number
  nps: number
  nps_promoters: number
  nps_detractors: number
  nps_count: number
  queue_abandonment_rate: number
  queue_abandonment_count: number
  avg_wait_time: number
  avg_wait_time_count: number
  avg_handling_time: number
  avg_handling_time_count: number
  csat: number
  csat_satisfied: number
  csat_count: number
  resolution_rate: number
  resolved_count: number
  ai_resolved_count: number
  human_resolved_count: number
  ai_resolution_rate: number
  human_resolution_rate: number
}

type DailyReport = { date: string; label: string; conversations: number; messages: number; evaluations: number }
type AttendantReport = { id?: number; name: string; conversations: number; human_messages: number; evaluations: number; rating: number; rating_count: number; ai_rating: number; ai_rating_count: number }
type SectorReport = { id?: number; name: string; conversations: number; evaluations: number; rating: number; rating_count: number; ai_rating: number; ai_rating_count: number; human_rating: number; human_rating_count: number }
type RatingReport = { rating: number; ai: number; human: number; total: number }
type EvaluationReport = { conversation_id: number; protocol: string; channel: string; sector: string; attendant: string; overall_rating?: number; ai_rating?: number; human_rating?: number; evaluated_at: string }
type ReportsData = { filters: ReportsFilters; options: { sectors: ReportOption[]; attendants: ReportOption[] }; summary: ReportsSummary; daily: DailyReport[]; by_attendant: AttendantReport[]; by_sector: SectorReport[]; ratings: RatingReport[]; evaluations: EvaluationReport[] }
type ReportsMeta = { generated_at: string; evaluation_total: number; evaluation_limit: number; evaluation_offset?: number; evaluation_has_more?: boolean; evaluation_next_cursor?: string; evaluation_truncated: boolean }
type Envelope<T> = { data?: T; meta?: ReportsMeta; error?: { message?: string } }
type DraftFilters = { period: Period; from: string; to: string; channel: string; sectorID: string; attendantID: string }

const emptySummary: ReportsSummary = { conversations: 0, active: 0, waiting: 0, closed: 0, messages: 0, customer_messages: 0, ai_messages: 0, human_messages: 0, evaluated_conversations: 0, rating_count: 0, overall_rating: 0, overall_rating_count: 0, ai_rating: 0, ai_rating_count: 0, human_rating: 0, human_rating_count: 0, avg_response_time: 0, avg_response_time_count: 0, transfer_rate: 0, transferred_count: 0, nps: 0, nps_promoters: 0, nps_detractors: 0, nps_count: 0, queue_abandonment_rate: 0, queue_abandonment_count: 0, avg_wait_time: 0, avg_wait_time_count: 0, avg_handling_time: 0, avg_handling_time_count: 0, csat: 0, csat_satisfied: 0, csat_count: 0, resolution_rate: 0, resolved_count: 0, ai_resolved_count: 0, human_resolved_count: 0, ai_resolution_rate: 0, human_resolution_rate: 0 }
const emptyDraft: DraftFilters = { period: '7', from: '', to: '', channel: '', sectorID: '', attendantID: '' }

async function requestReports(query: string): Promise<{ data: ReportsData; meta: ReportsMeta }> {
  const response = await fetch(`/api/react/reports?${query}`, { credentials: 'same-origin' })
  const payload = await response.json() as Envelope<ReportsData>
  if (!response.ok || !payload.data) throw new Error(payload.error?.message || 'Não foi possível carregar os relatórios.')
  return { data: payload.data, meta: payload.meta || { generated_at: '', evaluation_total: 0, evaluation_limit: 200, evaluation_truncated: false } }
}

function buildReportsQuery(applied: DraftFilters, limit?: number, offset?: number): string {
  const query = new URLSearchParams({ period: applied.period })
  if (limit) query.set('limit', String(limit))
  if (applied.period === 'custom') {
    query.set('from', applied.from)
    query.set('to', applied.to)
  }
  if (applied.channel) query.set('channel', applied.channel)
  if (applied.sectorID) query.set('sector_id', applied.sectorID)
  if (applied.attendantID) query.set('attendant_id', applied.attendantID)
  if (offset && offset > 0) query.set('cursor', encodeCursor(offset))
  return query.toString()
}

export function ReportsPage({ onBack, permissions = null }: { onBack?: () => void; permissions?: string[] | null }) {
  const [draft, setDraft] = useState<DraftFilters>(emptyDraft)
  const [applied, setApplied] = useState<DraftFilters>(emptyDraft)
  const [reports, setReports] = useState<ReportsData | null>(null)
  const [meta, setMeta] = useState<ReportsMeta | null>(null)
  const [evaluationOffset, setEvaluationOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const canExport = permissions == null || permissions.includes('relatorios_ver')

  const load = useCallback(async () => {
    setLoading(true)
    if (applied.period === 'custom' && (!applied.from || !applied.to)) {
      setError('Informe as datas inicial e final para usar um período personalizado.')
      setLoading(false)
      return
    }
    const query = buildReportsQuery(applied, DEFAULT_PAGE_SIZE, evaluationOffset)
    try {
      const result = await requestReports(query)
      setReports({
        ...result.data,
        options: result.data.options || { sectors: [], attendants: [] },
        summary: result.data.summary || emptySummary,
        daily: result.data.daily || [],
        by_attendant: result.data.by_attendant || [],
        by_sector: result.data.by_sector || [],
        ratings: result.data.ratings || [],
        evaluations: result.data.evaluations || [],
      })
      setMeta(result.meta)
      setError(null)
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar os relatórios.')
    } finally {
      setLoading(false)
    }
  }, [applied, evaluationOffset])

  useEffect(() => { void load() }, [load])

  const updateDraft = <K extends keyof DraftFilters>(key: K, value: DraftFilters[K]) => setDraft((current) => ({ ...current, [key]: value }))
  const applyFilters = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setEvaluationOffset(0)
    setApplied({ ...draft })
  }

  const exportHref = useMemo(() => `/api/react/reports/export?${buildReportsQuery(applied)}`, [applied])
  const exportReady = canExport && !(applied.period === 'custom' && (!applied.from || !applied.to))

  const data = reports ?? { filters: { period: '7', from: '', to: '' }, options: { sectors: [], attendants: [] }, summary: emptySummary, daily: [], by_attendant: [], by_sector: [], ratings: [], evaluations: [] }
  const maxDailyValue = useMemo(() => Math.max(1, ...data.daily.flatMap((day) => [day.conversations, day.messages])), [data.daily])
  const maxRatingValue = useMemo(() => Math.max(1, ...data.ratings.map((rating) => rating.total)), [data.ratings])
  const hasRatings = data.ratings.some((rating) => rating.total > 0)

  return <div className="reports-page">
    <section className="reports-intro">
      <div><span className="eyebrow">Inteligência da operação</span><h2>Decisões com contexto.</h2><p>Acompanhe o volume do atendimento, a experiência do cliente e o desempenho de cada equipe em um só lugar.</p></div>
      <div className="reports-intro-actions">{onBack && <button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button>}<button className="secondary-button" type="button" onClick={() => void load()} disabled={loading}>↻ Atualizar</button>{canExport && <a className={`primary-button reports-export${exportReady ? '' : ' is-disabled'}`} href={exportReady ? exportHref : undefined} aria-disabled={!exportReady} download onClick={(event) => { if (!exportReady) event.preventDefault() }}>⇩ Exportar CSV</a>}</div>
    </section>

    <form className="reports-filters" onSubmit={applyFilters}>
      <label><span>Período</span><select value={draft.period} onChange={(event) => updateDraft('period', event.target.value as Period)}><option value="1">Hoje</option><option value="7">Últimos 7 dias</option><option value="30">Últimos 30 dias</option><option value="custom">Personalizado</option></select></label>
      {draft.period === 'custom' && <><label><span>De</span><input type="date" value={draft.from} onChange={(event) => updateDraft('from', event.target.value)} /></label><label><span>Até</span><input type="date" value={draft.to} onChange={(event) => updateDraft('to', event.target.value)} /></label></>}
      <label><span>Canal</span><select value={draft.channel} onChange={(event) => updateDraft('channel', event.target.value)}><option value="">Todos os canais</option><option value="whatsapp">WhatsApp</option><option value="webchat">Webchat</option><option value="telegram">Telegram</option></select></label>
      <label><span>Setor</span><select value={draft.sectorID} onChange={(event) => updateDraft('sectorID', event.target.value)}><option value="">Todos os setores</option>{data.options.sectors.map((sector) => <option key={sector.id} value={sector.id}>{sector.name}</option>)}</select></label>
      <label><span>Atendente</span><select value={draft.attendantID} onChange={(event) => updateDraft('attendantID', event.target.value)}><option value="">Toda a equipe</option>{data.options.attendants.map((attendant) => <option key={attendant.id} value={attendant.id}>{attendant.name}</option>)}</select></label>
      <button className="primary-button" type="submit">Aplicar filtros <span>↗</span></button>
    </form>

    {error && <div className="reports-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
    {loading && !reports ? <div className="reports-state"><span className="reports-spinner" /> Carregando visão analítica…</div> : <>
      <section className="reports-kpis">
        <MetricCard icon="◌" tone="blue" label="Conversas" value={formatNumber(data.summary.conversations)} detail={`${formatNumber(data.summary.active)} ativas · ${formatNumber(data.summary.closed)} encerradas`} />
        <MetricCard icon="✉" tone="violet" label="Mensagens" value={formatNumber(data.summary.messages)} detail={`${formatNumber(data.summary.customer_messages)} cliente · ${formatNumber(data.summary.human_messages)} equipe`} />
        <MetricCard icon="★" tone="amber" label="Nota geral" value={formatRating(data.summary.overall_rating, data.summary.overall_rating_count)} detail={`${formatNumber(data.summary.evaluated_conversations)} atendimentos avaliados`} featured />
        <MetricCard icon="✦" tone="green" label="Assistente IA" value={formatRating(data.summary.ai_rating, data.summary.ai_rating_count)} detail={`${formatNumber(data.summary.ai_rating_count)} notas da IA`} />
        <MetricCard icon="♧" tone="teal" label="Equipe humana" value={formatRating(data.summary.human_rating, data.summary.human_rating_count)} detail={`${formatNumber(data.summary.human_rating_count)} notas do operador`} />
      </section>

      <section className="reports-kpis">
        <MetricCard icon="⌛" tone="blue" label="Tempo médio de resposta" value={formatDuration(data.summary.avg_response_time)} detail={data.summary.avg_response_time_count > 0 ? `Primeira resposta em ${formatNumber(data.summary.avg_response_time_count)} atendimentos` : 'Sem respostas medidas no período'} />
        <MetricCard icon="⇄" tone="violet" label="Transferência p/ humano" value={formatPercent(data.summary.transfer_rate)} detail={`${formatNumber(data.summary.transferred_count)} conversas transferidas`} />
        <MetricCard icon="◉" tone="green" label="NPS" value={formatNPS(data.summary.nps)} detail={`${formatNumber(data.summary.nps_promoters)} promotores · ${formatNumber(data.summary.nps_detractors)} detratores`} />
        <MetricCard icon="⏳" tone="amber" label="Abandono de fila" value={formatPercent(data.summary.queue_abandonment_rate)} detail={`${formatNumber(data.summary.queue_abandonment_count)} conversas abandonadas`} />
      </section>

      <section className="reports-kpis">
        <MetricCard icon="◔" tone="teal" label="Tempo médio de espera" value={formatDuration(data.summary.avg_wait_time)} detail={data.summary.avg_wait_time_count > 0 ? `Espera por humano em ${formatNumber(data.summary.avg_wait_time_count)} atendimentos` : 'Sem espera humana medida'} />
        <MetricCard icon="◕" tone="teal" label="Tempo médio de atendimento" value={formatDuration(data.summary.avg_handling_time)} detail={data.summary.avg_handling_time_count > 0 ? `${formatNumber(data.summary.avg_handling_time_count)} atendimentos humanos` : 'Sem atendimento humano medido'} />
        <MetricCard icon="☺" tone="green" label="CSAT" value={formatPercent(data.summary.csat)} detail={`${formatNumber(data.summary.csat_satisfied)} de ${formatNumber(data.summary.csat_count)} notas ≥ 4`} />
        <MetricCard icon="✓" tone="blue" label="Taxa de encerramento" value={formatPercent(data.summary.resolution_rate)} detail={`${formatNumber(data.summary.resolved_count)} de ${formatNumber(data.summary.conversations)} conversas encerradas`} />
        <MetricCard icon="⚙" tone="violet" label="Resolução pela IA" value={formatPercent(data.summary.ai_resolution_rate)} detail={`${formatNumber(data.summary.ai_resolved_count)} IA · ${formatNumber(data.summary.human_resolved_count)} humano`} />
      </section>

      <section className="reports-main-grid">
        <article className="reports-panel reports-panel--chart"><PanelHeading eyebrow="Ritmo da operação" title="Volume por dia" description="Conversas abertas, mensagens trocadas e avaliações recebidas." /><div className="reports-chart-legend"><span><i className="is-conversations" />Conversas</span><span><i className="is-messages" />Mensagens</span><span><i className="is-evaluations" />Avaliações</span></div><div className="reports-chart" aria-label="Gráfico de volume por dia">{data.daily.map((day) => <div className="reports-chart-column" key={day.date} title={`${day.label}: ${day.conversations} conversas, ${day.messages} mensagens`}><div className="reports-chart-values"><b>{day.conversations}</b><b>{day.messages}</b></div><div className="reports-chart-bars"><i className="is-conversations" style={{ height: `${Math.max(day.conversations / maxDailyValue * 100, day.conversations ? 4 : 0)}%` }} /><i className="is-messages" style={{ height: `${Math.max(day.messages / maxDailyValue * 100, day.messages ? 4 : 0)}%` }} /><i className="is-evaluations" style={{ height: `${Math.max(day.evaluations / maxDailyValue * 100, day.evaluations ? 4 : 0)}%` }} /></div><span>{day.label}</span></div>)}</div>{data.daily.length === 0 && <EmptyState label="Ainda não há dados diários para este período." />}</article>
        <article className="reports-panel reports-panel--ratings"><PanelHeading eyebrow="Qualidade percebida" title="Distribuição das notas" description="Veja onde a experiência está ganhando força — e onde precisa de atenção." />{!hasRatings ? <EmptyState label="Nenhuma avaliação recebida no período." /> : <div className="reports-rating-list">{[...data.ratings].sort((left, right) => right.rating - left.rating).map((rating) => <div className="reports-rating-row" key={rating.rating}><span className="reports-rating-score">★ {rating.rating}</span><div className="reports-rating-track"><i style={{ width: `${rating.total / maxRatingValue * 100}%` }} /></div><strong>{rating.total}</strong><small>{rating.ai} IA · {rating.human} humano</small></div>)}</div>}</article>
      </section>

      <section className="reports-two-columns"><ReportTable title="Desempenho por atendente" description="A nota humana e o volume de cada pessoa da equipe." columns={['Atendente', 'Conversas', 'Nota', 'Avaliações', 'Mensagens']}><tbody>{data.by_attendant.map((attendant) => <tr key={`${attendant.id ?? 'none'}-${attendant.name}`}><td><div className="reports-person"><span>{attendant.name.slice(0, 1).toUpperCase()}</span><strong>{attendant.name}</strong></div></td><td>{formatNumber(attendant.conversations)}</td><td><Score value={attendant.rating} count={attendant.rating_count} /></td><td>{formatNumber(attendant.evaluations)}</td><td>{formatNumber(attendant.human_messages)}</td></tr>)}{data.by_attendant.length === 0 && <tr><td colSpan={5}><EmptyState label="Nenhum atendente encontrado para os filtros atuais." /></td></tr>}</tbody></ReportTable><ReportTable title="Desempenho por setor" description="Compare a experiência completa em cada equipe." columns={['Setor', 'Conversas', 'Geral', 'IA', 'Humano']}><tbody>{data.by_sector.map((sector) => <tr key={`${sector.id ?? 'none'}-${sector.name}`}><td><strong>{sector.name}</strong></td><td>{formatNumber(sector.conversations)}</td><td><Score value={sector.rating} count={sector.rating_count} /></td><td><Score value={sector.ai_rating} count={sector.ai_rating_count} subtle /></td><td><Score value={sector.human_rating} count={sector.human_rating_count} subtle /></td></tr>)}{data.by_sector.length === 0 && <tr><td colSpan={5}><EmptyState label="Nenhum setor encontrado para os filtros atuais." /></td></tr>}</tbody></ReportTable></section>

      <section className="reports-panel reports-panel--evaluations"><PanelHeading eyebrow="Auditoria individual" title="Avaliação por atendimento" description="Cada protocolo avaliado, sem exibir dados pessoais do cliente." /><div className="reports-table-wrap"><table className="reports-table"><thead><tr><th>Protocolo</th><th>Canal</th><th>Setor</th><th>Atendente</th><th>Geral</th><th>IA</th><th>Humano</th><th>Avaliado em</th></tr></thead><tbody>{data.evaluations.map((evaluation) => <tr key={`${evaluation.conversation_id}-${evaluation.evaluated_at}`}><td><strong>{evaluation.protocol}</strong></td><td><span className={`reports-channel reports-channel--${evaluation.channel}`}>{channelLabel(evaluation.channel)}</span></td><td>{evaluation.sector}</td><td>{evaluation.attendant}</td><td><Score value={evaluation.overall_rating ?? 0} count={evaluation.overall_rating ? 1 : 0} /></td><td>{evaluation.ai_rating ? `${evaluation.ai_rating}/5` : '—'}</td><td>{evaluation.human_rating ? `${evaluation.human_rating}/5` : '—'}</td><td>{formatDateTime(evaluation.evaluated_at)}</td></tr>)}{data.evaluations.length === 0 && <tr><td colSpan={8}><EmptyState label="Nenhum atendimento avaliado no período." /></td></tr>}</tbody></table></div>{meta?.evaluation_truncated && <p className="reports-footnote">Mostrando os {meta.evaluation_limit} atendimentos mais recentes. Ajuste o período ou os filtros para aprofundar a análise.</p>}{meta && meta.evaluation_total > 0 && <Pagination total={meta.evaluation_total} limit={meta.evaluation_limit || DEFAULT_PAGE_SIZE} offset={evaluationOffset} onOffsetChange={setEvaluationOffset} loading={loading} />}</section>
    </>}
  </div>
}

function MetricCard({ icon, tone, label, value, detail, featured = false }: { icon: string; tone: string; label: string; value: string; detail: string; featured?: boolean }) {
  return <article className={`reports-kpi reports-kpi--${tone}${featured ? ' is-featured' : ''}`}><span className="reports-kpi-icon">{icon}</span><div><span>{label}</span><strong>{value}</strong><small>{detail}</small></div></article>
}

function PanelHeading({ eyebrow, title, description }: { eyebrow: string; title: string; description: string }) {
  return <header className="reports-panel-heading"><div><span className="eyebrow">{eyebrow}</span><h3>{title}</h3><p>{description}</p></div></header>
}

function ReportTable({ title, description, columns, children }: { title: string; description: string; columns: string[]; children: ReactNode }) {
  return <article className="reports-panel reports-table-panel"><PanelHeading eyebrow="Desempenho" title={title} description={description} /><div className="reports-table-wrap"><table className="reports-table"><thead><tr>{columns.map((column) => <th key={column}>{column}</th>)}</tr></thead>{children}</table></div>{Children.count(children) === 0 && <EmptyState label="Nenhum dado encontrado para os filtros atuais." />}</article>
}

function Score({ value, count, subtle = false }: { value: number; count: number; subtle?: boolean }) {
  return count > 0 ? <span className={`reports-score${subtle ? ' is-subtle' : ''}`}>★ {value.toFixed(1)}</span> : <span className="reports-muted">—</span>
}

function EmptyState({ label }: { label: string }) {
  return <div className="reports-empty"><span>◌</span><p>{label}</p></div>
}

function formatNumber(value: number) { return value.toLocaleString('pt-BR') }
function formatDuration(value: number) { const total = Math.round(Number(value) || 0); if (total <= 0) return '—'; if (total >= 60) return `${Math.floor(total / 60)}m ${total % 60}s`; return `${total}s` }
function formatPercent(value: number) { const v = Number(value) || 0; return `${v.toFixed(1).replace('.', ',')}%` }
function formatNPS(value: number) { const v = Math.round(Number(value) || 0); return v > 0 ? `+${v}` : `${v}` }
function formatRating(value: number, count: number) { return count > 0 ? `${value.toFixed(1)}/5` : '—' }
function formatDateTime(value: string) { const date = new Date(value); return Number.isNaN(date.valueOf()) ? '—' : date.toLocaleString('pt-BR', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' }) }
function channelLabel(value: string) { return value === 'whatsapp' ? 'WhatsApp' : value === 'webchat' ? 'Webchat' : value || 'Outro' }

export default ReportsPage
