import { useCallback, useEffect, useMemo, useState } from 'react'
import { Modal } from '../../shared/Modal'
import { FilterBar, FilterSearch, FilterSelect, Pagination, DEFAULT_PAGE_SIZE, encodeCursor, type PaginationMeta } from '../../shared/Pagination'
import './Contacts.css'

type Contact = {
  id: number
  name: string
  phone: string
  email: string
  notes: string
  tem_whatsapp: boolean
  tags: string[]
  campos_extras: Record<string, string>
  segment_ids: number[]
  segments: string[]
  created_at: string
  updated_at: string
}

type Segment = { id: number; name: string; count?: number }
type ContactResponse = { items: Contact[]; segments: Segment[] }
type Envelope<T> = { data?: T; meta?: PaginationMeta; error?: { message?: string } }
type ContactExtra = { key: string; value: string }
type Draft = { name: string; phone: string; email: string; notes: string; tem_whatsapp: boolean; segment_ids: number[]; tagsText: string; extras: ContactExtra[] }
type ContactActionResponse = { ok?: boolean; tem_whatsapp?: boolean; imported?: number; total?: number; contact?: Contact; conversation?: { id?: number; hash?: string } }
type ImportResult = { imported: number; duplicated: number; errors: number; total: number }

const emptyDraft = (): Draft => ({ name: '', phone: '', email: '', notes: '', tem_whatsapp: false, segment_ids: [], tagsText: '', extras: [] })

const contactTags = (contact: Contact): string[] => Array.isArray(contact.tags) ? contact.tags : []

const contactSegments = (contact: Contact): string[] => Array.isArray(contact.segments) ? contact.segments : []

const contactExtras = (contact: Contact): ContactExtra[] => Object.entries(contact.campos_extras ?? {}).map(([key, value]) => ({ key, value }))

const draftPayload = (draft: Draft) => ({
  name: draft.name.trim(),
  phone: draft.phone.trim(),
  email: draft.email.trim(),
  notes: draft.notes,
  tem_whatsapp: draft.tem_whatsapp,
  segment_ids: draft.segment_ids,
  tags: draft.tagsText.split(',').map((tag) => tag.trim()).filter((tag) => tag !== ''),
  campos_extras: draft.extras.reduce<Record<string, string>>((acc, item) => {
    const key = item.key.trim()
    if (key !== '') acc[key] = item.value.trim()
    return acc
  }, {}),
})

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, { ...options, credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) } })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return payload.data as T
}

async function requestEnvelope<T>(url: string, options?: RequestInit): Promise<{ data: T; meta: PaginationMeta }> {
  const response = await fetch(url, { ...options, credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) } })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return { data: payload.data as T, meta: payload.meta || {} }
}

async function requestUpload<T>(url: string, form: FormData): Promise<T> {  const response = await fetch(url, { method: 'POST', body: form, credentials: 'same-origin' })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return payload.data as T
}

export function ContactsPage({ onBack, initialQuery = '' }: { onBack: () => void; initialQuery?: string }) {
  const [contacts, setContacts] = useState<Contact[]>([])
  const [segments, setSegments] = useState<Segment[]>([])
  const [query, setQuery] = useState(initialQuery)
  const [segmentFilter, setSegmentFilter] = useState('')
  const [whatsappFilter, setWhatsappFilter] = useState('')
  const [offset, setOffset] = useState(0)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [draft, setDraft] = useState<Draft | null>(null)
  const [editingID, setEditingID] = useState<number | null>(null)
  const [actionID, setActionID] = useState<number | null>(null)
  const [actionKey, setActionKey] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [importOpen, setImportOpen] = useState(false)
  const [importing, setImporting] = useState(false)
  const [importFile, setImportFile] = useState<File | null>(null)
  const [importSegment, setImportSegment] = useState('')
  const [importResult, setImportResult] = useState<ImportResult | null>(null)
  const [importError, setImportError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const params = new URLSearchParams({ limit: String(DEFAULT_PAGE_SIZE), q: query.trim() })
      if (segmentFilter) params.set('segment_id', segmentFilter)
      if (whatsappFilter) params.set('tem_whatsapp', whatsappFilter)
      if (offset > 0) params.set('cursor', encodeCursor(offset))
      const payload = await requestEnvelope<ContactResponse>(`/api/react/contacts?${params.toString()}`)
      setContacts(payload.data.items || [])
      setSegments(payload.data.segments || [])
      setTotal(payload.meta.total ?? payload.data.items?.length ?? 0)
      setError(null)
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar os contatos.')
    } finally { setLoading(false) }
  }, [query, segmentFilter, whatsappFilter, offset])

  useEffect(() => { void load() }, [load])
  useEffect(() => { setOffset(0) }, [query, segmentFilter, whatsappFilter])

  const openCreate = () => { setEditingID(null); setDraft(emptyDraft()); setError(null) }
  const openEdit = (contact: Contact) => {
    setEditingID(contact.id)
    setDraft({ name: contact.name, phone: contact.phone, email: contact.email, notes: contact.notes, tem_whatsapp: contact.tem_whatsapp, segment_ids: contact.segment_ids || [], tagsText: contactTags(contact).join(', '), extras: contactExtras(contact) })
    setError(null)
  }
  const closeEditor = () => { if (!saving) { setDraft(null); setEditingID(null) } }
  const updateDraft = <K extends keyof Draft>(key: K, value: Draft[K]) => setDraft((current) => current ? { ...current, [key]: value } : current)

  const save = async () => {
    if (!draft || !draft.phone.trim()) { setError('Informe o telefone do contato.'); return }
    setSaving(true)
    try {
      const method = editingID === null ? 'POST' : 'PATCH'
      const url = editingID === null ? '/api/react/contacts' : `/api/react/contacts/${editingID}`
      await request<Contact>(url, { method, body: JSON.stringify(draftPayload(draft)) })
      closeEditor(); await load()
    } catch (saveError) { setError(saveError instanceof Error ? saveError.message : 'Não foi possível salvar o contato.') }
    finally { setSaving(false) }
  }

  const remove = async (contact: Contact) => {
    if (!window.confirm(`Excluir “${contact.name || contact.phone}”? Essa ação não pode ser desfeita.`)) return
    setActionID(contact.id)
    try { await request(`/api/react/contacts/${contact.id}`, { method: 'DELETE' }); await load() }
    catch (removeError) { setError(removeError instanceof Error ? removeError.message : 'Não foi possível excluir o contato.') }
    finally { setActionID(null) }
  }

  const runContactAction = async (contact: Contact, action: 'verify' | 'conversation') => {
    const key = `${action}:${contact.id}`
    setActionKey(key)
    setNotice(null)
    try {
      const result = await request<ContactActionResponse>('/api/react/contacts', { method: 'POST', body: JSON.stringify({ action, contact_id: contact.id }) })
      if (action === 'verify') {
        setNotice(result.tem_whatsapp ? `${contact.phone} está disponível no WhatsApp.` : `${contact.phone} não foi encontrado no WhatsApp.`)
      } else {
        setNotice(result.conversation?.hash ? `Conversa iniciada no protocolo ${result.conversation.hash}.` : 'Conversa iniciada com sucesso.')
      }
      await load()
    } catch (actionError) {
      setError(actionError instanceof Error ? actionError.message : 'Não foi possível concluir a ação.')
    } finally { setActionKey(null) }
  }

  const syncContacts = async () => {
    setActionKey('sync')
    setNotice(null)
    try {
      const result = await request<ContactActionResponse>('/api/react/contacts', { method: 'POST', body: JSON.stringify({ action: 'sync' }) })
      setNotice(`${result.imported ?? 0} contato${result.imported === 1 ? '' : 's'} sincronizado${result.imported === 1 ? '' : 's'} do WhatsApp.`)
      await load()
    } catch (syncError) {
      setError(syncError instanceof Error ? syncError.message : 'Não foi possível sincronizar os contatos.')
    } finally { setActionKey(null) }
  }

  const visibleSegments = useMemo(() => segments.slice(0, 12), [segments])

  const runImport = async () => {
    if (!importFile) { setImportError('Selecione um arquivo CSV para importar.'); return }
    setImportError(null)
    setImporting(true)
    setImportResult(null)
    try {
      const form = new FormData()
      form.append('file', importFile)
      if (importSegment) form.append('segment_id', importSegment)
      const result = await requestUpload<ImportResult>('/api/react/contacts/import', form)
      setImportResult(result)
      await load()
    } catch (uploadError) {
      setImportError(uploadError instanceof Error ? uploadError.message : 'Não foi possível importar o arquivo.')
    } finally { setImporting(false) }
  }

  return <div className="contacts-page">
    <section className="contacts-intro">
      <div><span className="eyebrow">Relacionamento</span><h2>Contatos que dão contexto.</h2><p>Organize pessoas, segmentos e histórico para sua equipe nunca começar uma conversa do zero.</p></div>
      <div className="contacts-intro-actions"><button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button><button className="primary-button" type="button" onClick={openCreate}>+ Novo contato</button></div>
    </section>
    {error && <div className="contacts-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
    {notice && <div className="contacts-notice" role="status"><span>✓</span>{notice}<button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso">×</button></div>}
    <FilterBar resultLabel={`${total} contato${total === 1 ? '' : 's'}`} onReset={() => { setQuery(''); setSegmentFilter(''); setWhatsappFilter('') }}>
      <FilterSearch value={query} onChange={setQuery} placeholder="Buscar por nome ou telefone" />
      <FilterSelect value={segmentFilter} onChange={setSegmentFilter} label="Filtrar por segmento" options={[{ value: '', label: 'Todos os segmentos' }, ...segments.map((segment) => ({ value: String(segment.id), label: segment.name }))]} />
      <FilterSelect value={whatsappFilter} onChange={setWhatsappFilter} label="Filtrar por WhatsApp" options={[{ value: '', label: 'WhatsApp: todos' }, { value: 'true', label: 'Com WhatsApp' }, { value: 'false', label: 'Sem WhatsApp' }]} />
      <button className="secondary-button" type="button" onClick={() => void syncContacts()} disabled={actionKey === 'sync'}>{actionKey === 'sync' ? 'Sincronizando…' : 'Sincronizar WhatsApp'}</button>
      <button className="secondary-button" type="button" onClick={() => { setImportOpen(true); setImportResult(null); setImportFile(null); setImportSegment(''); setImportError(null) }}>Importar CSV</button>
    </FilterBar>
    {loading ? <div className="contacts-state"><span className="contacts-spinner" /> Carregando contatos…</div> : contacts.length === 0 ? <div className="contacts-empty"><div className="contacts-empty-icon">◎</div><strong>Nenhum contato encontrado</strong><p>Cadastre o primeiro contato ou ajuste sua busca.</p><button className="primary-button" type="button" onClick={openCreate}>Adicionar contato</button></div> : <div className="contacts-table-wrap"><table className="contacts-table"><thead><tr><th>Contato</th><th>Canal</th><th>Tags e observações</th><th>Segmentos</th><th>Atualizado</th><th aria-label="Ações" /></tr></thead><tbody>{contacts.map((contact) => <tr key={contact.id}><td><div className="contact-person"><div className="contact-avatar">{(contact.name || contact.phone).slice(0, 1).toUpperCase()}</div><div><strong>{contact.name || 'Sem nome'}</strong><span>{contact.phone}{contact.email ? ` · ${contact.email}` : ''}</span></div></div></td><td><span className={`contact-channel ${contact.tem_whatsapp ? 'is-whatsapp' : ''}`}><i />{contact.tem_whatsapp ? 'WhatsApp verificado' : 'Telefone'}</span></td><td><div className="contact-tags">{contactTags(contact).length ? contactTags(contact).slice(0, 3).map((tag) => <span key={tag}>{tag}</span>) : <small>Sem tags</small>}{contactTags(contact).length > 3 && <small>+{contactTags(contact).length - 3}</small>}{contact.notes ? <small className="contact-note" title={contact.notes}>📝 {contact.notes.length > 40 ? `${contact.notes.slice(0, 40)}…` : contact.notes}</small> : null}</div></td><td><div className="contact-tags">{contactSegments(contact).length ? contactSegments(contact).slice(0, 3).map((segment) => <span key={segment}>{segment}</span>) : <small>Sem segmento</small>}{contactSegments(contact).length > 3 && <small>+{contactSegments(contact).length - 3}</small>}</div></td><td><span className="contact-date">{formatDate(contact.updated_at)}</span></td><td><div className="contact-actions"><button type="button" onClick={() => void runContactAction(contact, 'verify')} disabled={actionKey === `verify:${contact.id}`}>{actionKey === `verify:${contact.id}` ? 'Verificando…' : 'Verificar'}</button><button type="button" onClick={() => void runContactAction(contact, 'conversation')} disabled={actionKey === `conversation:${contact.id}`}>{actionKey === `conversation:${contact.id}` ? 'Abrindo…' : 'Conversar'}</button><button type="button" onClick={() => openEdit(contact)}>Editar</button><button className="is-danger" type="button" disabled={actionID === contact.id} onClick={() => void remove(contact)}>Excluir</button></div></td></tr>)}</tbody></table></div>}
    {!loading && contacts.length > 0 && <Pagination total={total} limit={DEFAULT_PAGE_SIZE} offset={offset} onOffsetChange={setOffset} loading={loading} />}
    {visibleSegments.length > 0 && <section className="segments-strip"><div><span className="eyebrow">Segmentação</span><strong>Visão rápida dos grupos</strong></div><div className="segments-strip-list">{visibleSegments.map((segment) => <span key={segment.id}><b>{segment.count ?? 0}</b>{segment.name}</span>)}</div></section>}
    {draft && <ContactEditor draft={draft} editing={editingID !== null} saving={saving} segments={segments} onChange={updateDraft} onSave={() => void save()} onClose={closeEditor} />}
    {importOpen && <ImportModal file={importFile} segment={importSegment} importing={importing} result={importResult} error={importError} segments={segments} onFile={setImportFile} onSegment={setImportSegment} onImport={() => void runImport()} onClose={() => { if (!importing) { setImportOpen(false) } }} />}
  </div>
}

function ContactEditor({ draft, editing, saving, segments, onChange, onSave, onClose }: { draft: Draft; editing: boolean; saving: boolean; segments: Segment[]; onChange: <K extends keyof Draft>(key: K, value: Draft[K]) => void; onSave: () => void; onClose: () => void }) {
  return <Modal open title={editing ? 'Ajustar contato' : 'Adicionar contato'} onClose={onClose} size="lg" footer={<><button className="secondary-button" type="button" disabled={saving} onClick={onClose}>Cancelar</button><button className="primary-button" type="button" disabled={saving} onClick={onSave}>{saving ? 'Salvando…' : 'Salvar contato'} ↗</button></>}><div className="contacts-form"><label><span>Nome</span><input autoFocus value={draft.name} onChange={(event) => onChange('name', event.target.value)} placeholder="Nome do cliente" /></label><label><span>Telefone *</span><input value={draft.phone} onChange={(event) => onChange('phone', event.target.value)} placeholder="NUMERO_COM_DDI" /></label><label><span>E-mail</span><input type="email" value={draft.email} onChange={(event) => onChange('email', event.target.value)} placeholder="cliente@empresa.com" /></label><label className="contacts-form-wide"><span>Tags (separadas por vírgula)</span><input value={draft.tagsText} onChange={(event) => onChange('tagsText', event.target.value)} placeholder="vip, atacado, recompra" /></label><label className="contacts-form-wide"><span>Observações</span><textarea rows={3} value={draft.notes} onChange={(event) => onChange('notes', event.target.value)} placeholder="Contexto importante para a equipe" /></label><div className="contacts-form-wide"><span className="contacts-field-label">Campos extras</span><div className="contact-extras">{draft.extras.map((extra, index) => <div key={index} className="contact-extra-row"><input value={extra.key} onChange={(event) => onChange('extras', draft.extras.map((item, i) => i === index ? { ...item, key: event.target.value } : item))} placeholder="chave" /><input value={extra.value} onChange={(event) => onChange('extras', draft.extras.map((item, i) => i === index ? { ...item, value: event.target.value } : item))} placeholder="valor" /><button type="button" onClick={() => onChange('extras', draft.extras.filter((_, i) => i !== index))} aria-label="Remover campo">×</button></div>)}<button type="button" className="contact-extra-add" onClick={() => onChange('extras', [...draft.extras, { key: '', value: '' }])}>+ Adicionar campo</button></div></div><label className="contacts-check"><input type="checkbox" checked={draft.tem_whatsapp} onChange={(event) => onChange('tem_whatsapp', event.target.checked)} /><span>Este número tem WhatsApp</span></label><div className="contacts-form-wide"><span className="contacts-field-label">Segmentos</span><div className="segment-picker">{segments.length ? segments.map((segment) => <label key={segment.id}><input type="checkbox" checked={draft.segment_ids.includes(segment.id)} onChange={(event) => onChange('segment_ids', event.target.checked ? [...draft.segment_ids, segment.id] : draft.segment_ids.filter((id) => id !== segment.id))} /><span>{segment.name}</span></label>) : <small>Nenhum segmento criado ainda.</small>}</div></div></div></Modal>
}

function formatDate(value: string) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.valueOf()) ? '—' : date.toLocaleDateString('pt-BR', { day: '2-digit', month: 'short' }).replace('.', '')
}

function ImportModal({ file, segment, importing, result, error, segments, onFile, onSegment, onImport, onClose }: { file: File | null; segment: string; importing: boolean; result: ImportResult | null; error: string | null; segments: Segment[]; onFile: (file: File | null) => void; onSegment: (value: string) => void; onImport: () => void; onClose: () => void }) {
  return <Modal open title="Importar contatos (CSV)" onClose={onClose} size="lg" footer={<><button className="secondary-button" type="button" onClick={onClose} disabled={importing}>Fechar</button><button className="primary-button" type="button" onClick={onImport} disabled={importing || !file}>{importing ? 'Importando…' : 'Importar contatos'}</button></>}><div className="contacts-form"><label className="contacts-form-wide"><span>Arquivo CSV *</span><input type="file" accept=".csv,text/csv" onChange={(event) => onFile(event.target.files?.[0] ?? null)} /><small className="contacts-field-hint">Cabeçalho com colunas <code>phone</code>/<code>telefone</code> (obrigatório) e opcionalmente <code>name</code>, <code>email</code>, <code>notes</code> e <code>tem_whatsapp</code>. Até 5.000 contatos por envio.</small></label>{segments.length > 0 && <label className="contacts-form-wide"><span>Adicionar ao segmento (opcional)</span><select value={segment} onChange={(event) => onSegment(event.target.value)}><option value="">Nenhum</option>{segments.map((option) => <option key={option.id} value={option.id}>{option.name}</option>)}</select></label>}{error && <div className="contacts-modal-error" role="alert"><span>!</span>{error}</div>}{result && <div className="contacts-import-result" role="status"><div><b>{result.imported}</b><span>importados</span></div><div><b>{result.duplicated}</b><span>duplicados</span></div><div><b>{result.errors}</b><span>com erro</span></div><div><b>{result.total}</b><span>linhas (total)</span></div></div>}</div></Modal>
}

export default ContactsPage
