import type { ReactNode } from 'react'
import './ListControls.css'

export type PaginationMeta = {
  limit?: number
  total?: number
  has_more?: boolean
  next_cursor?: string
  offset?: number
}

export const DEFAULT_PAGE_SIZE = 25

export function encodeCursor(offset: number): string {
  return btoa(String(offset)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

export function Pagination({
  total,
  limit,
  offset,
  onOffsetChange,
  loading,
}: {
  total: number
  limit: number
  offset: number
  onOffsetChange: (offset: number) => void
  loading?: boolean
}) {
  const safeLimit = limit > 0 ? limit : DEFAULT_PAGE_SIZE
  const totalPages = Math.max(1, Math.ceil(total / safeLimit))
  const currentPage = Math.min(totalPages, Math.floor(offset / safeLimit) + 1)
  const from = total === 0 ? 0 : offset + 1
  const to = Math.min(offset + safeLimit, total)
  return (
    <nav className="list-pagination" aria-label="Paginação">
      <span className="list-pagination-summary">
        {total === 0 ? 'Nenhum registro' : `Mostrando ${from}–${to} de ${total}`}
      </span>
      <div className="list-pagination-controls">
        <button type="button" onClick={() => onOffsetChange(Math.max(0, offset - safeLimit))} disabled={loading || offset <= 0}>
          ← Anterior
        </button>
        <span className="list-pagination-page">Página {currentPage} de {totalPages}</span>
        <button type="button" onClick={() => onOffsetChange(offset + safeLimit)} disabled={loading || to >= total}>
          Próxima →
        </button>
      </div>
    </nav>
  )
}

export function FilterBar({
  children,
  resultLabel,
  onReset,
}: {
  children: ReactNode
  resultLabel?: string
  onReset?: () => void
}) {
  return (
    <section className="list-filter-bar" aria-label="Filtros">
      {children}
      {resultLabel && <span className="list-filter-result">{resultLabel}</span>}
      {onReset && (
        <button className="list-filter-reset" type="button" onClick={onReset}>
          Limpar filtros
        </button>
      )}
    </section>
  )
}

export function FilterSearch({
  value,
  onChange,
  placeholder,
}: {
  value: string
  onChange: (value: string) => void
  placeholder?: string
}) {
  return (
    <label className="list-filter-search">
      <span aria-hidden="true">⌕</span>
      <input value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder || 'Buscar…'} />
    </label>
  )
}

export function FilterSelect({
  value,
  onChange,
  options,
  label,
}: {
  value: string
  onChange: (value: string) => void
  options: Array<{ value: string; label: string }>
  label: string
}) {
  return (
    <select className="list-filter-select" value={value} onChange={(event) => onChange(event.target.value)} aria-label={label}>
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  )
}

export default Pagination
