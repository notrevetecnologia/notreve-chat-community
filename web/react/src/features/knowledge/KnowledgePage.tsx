import { useCallback, useEffect, useMemo, useState } from 'react'
import { createKnowledgeSource, deleteKnowledgeSource, loadKnowledge } from './knowledgeApi'
import type { KnowledgeData, KnowledgeDraft, KnowledgeSource } from './types'
import './Knowledge.css'

type KnowledgePageProps = {
  onBack?: () => void
  initialAgentId?: number | null
  permissions?: string[] | null
  onOpenAgent?: (id: number) => void
}

const defaultDraft = (): KnowledgeDraft => ({ kind: 'text', name: '', source_url: '', text: '', file_name: '', mime_type: '', data_base64: '' })
const kindLabels: Record<KnowledgeDraft['kind'], { label: string; icon: string; description: string }> = {
  text: { label: 'Texto', icon: 'Aa', description: 'Cole políticas, produtos, processos e respostas.' },
  site: { label: 'Site', icon: '◌', description: 'Busque texto de uma página pública.' },
  pdf: { label: 'PDF', icon: '▤', description: 'Extraia texto de um documento PDF.' },
  audio: { label: 'Áudio', icon: '◉', description: 'Transcreva uma gravação de forma protegida.' },
}

const formatChars = (value: number) => new Intl.NumberFormat('pt-BR').format(value)
const formatBytes = (value: number) => value >= 1 << 20 ? `${Math.round(value / (1 << 20))} MB` : `${Math.round(value / (1 << 10))} KB`
const sourceKindLabel = (kind: string) => kindLabels[kind as KnowledgeDraft['kind']]?.label || kind

export function KnowledgePage({ onBack, initialAgentId = null, permissions = null, onOpenAgent }: KnowledgePageProps) {
  const canManage = permissions == null || permissions.includes('agentes_gerenciar')
  const [data, setData] = useState<KnowledgeData | null>(null)
  const [agentID, setAgentID] = useState<number | undefined>(initialAgentId ?? undefined)
  const [draft, setDraft] = useState<KnowledgeDraft>(defaultDraft)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [actionID, setActionID] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const load = useCallback(async (requestedAgentID?: number) => {
    setLoading(true)
    try {
      const payload = await loadKnowledge(requestedAgentID)
      const safe: KnowledgeData = {
        ...payload,
        agents: payload.agents || [],
        sources: payload.sources || [],
        usage: payload.usage || { sources: 0, chars: 0 },
        limits: payload.limits || { max_sources_per_agent: 0, max_source_chars: 0, max_total_chars: 0, max_pdf_bytes: 0, max_audio_bytes: 0 },
      }
      setData(safe)
      setAgentID(safe.agent_id || safe.agents[0]?.id)
      setError(null)
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar a base de conhecimento.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load(agentID) }, [load, agentID])

  const selectedAgent = useMemo(() => data?.agents.find((agent) => agent.id === agentID), [agentID, data?.agents])
  const updateDraft = <K extends keyof KnowledgeDraft>(key: K, value: KnowledgeDraft[K]) => setDraft((current) => ({ ...current, [key]: value }))

  const chooseKind = (kind: KnowledgeDraft['kind']) => {
    setDraft({ ...defaultDraft(), kind })
    setError(null)
    setNotice(null)
  }

  const handleFile = async (file: File | undefined) => {
    if (!file) return
    const maxBytes = draft.kind === 'pdf' ? 10 << 20 : 25 << 20
    if (file.size > maxBytes) {
      setError(`Este arquivo excede o limite de ${formatBytes(maxBytes)}.`)
      return
    }
    const reader = new FileReader()
    reader.onload = () => {
      const value = String(reader.result || '')
      updateDraft('data_base64', value.includes(',') ? value.slice(value.indexOf(',') + 1) : value)
      updateDraft('file_name', file.name)
      updateDraft('mime_type', file.type)
      setError(null)
    }
    reader.onerror = () => setError('Não foi possível ler o arquivo selecionado.')
    reader.readAsDataURL(file)
  }

  const save = async () => {
    if (!agentID) {
      setError('Crie ou selecione um agente antes de treinar a IA.')
      return
    }
    if (draft.kind === 'site' && !draft.source_url.trim()) {
      setError('Informe a URL do site que será usado como fonte.')
      return
    }
    if (draft.kind === 'text' && !draft.text.trim()) {
      setError('Cole o texto que será usado no treinamento.')
      return
    }
    if ((draft.kind === 'pdf' || draft.kind === 'audio') && !draft.data_base64) {
      setError('Selecione um arquivo antes de processar a fonte.')
      return
    }
    setSaving(true)
    setError(null)
    setNotice(null)
    try {
      await createKnowledgeSource({ ...draft, name: draft.name.trim() }, agentID)
      setDraft(defaultDraft())
      setNotice('Fonte processada e adicionada à base do agente.')
      await load(agentID)
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : 'Não foi possível processar a fonte.')
    } finally {
      setSaving(false)
    }
  }

  const remove = async (source: KnowledgeSource) => {
    if (!window.confirm(`Remover “${source.name}” da base de conhecimento?`)) return
    setActionID(source.id)
    try {
      await deleteKnowledgeSource(source.id)
      setNotice('Fonte removida da base.')
      await load(agentID)
    } catch (removeError) {
      setError(removeError instanceof Error ? removeError.message : 'Não foi possível remover a fonte.')
    } finally {
      setActionID(null)
    }
  }

  const usagePercent = data?.limits.max_total_chars ? Math.min(100, Math.round(((data.usage.chars || 0) / data.limits.max_total_chars) * 100)) : 0
  const limitReached = Boolean(data && (data.usage.sources >= data.limits.max_sources_per_agent || data.usage.chars >= data.limits.max_total_chars))

  return <div className="knowledge-page">
    <section className="knowledge-intro"><div><span className="eyebrow">Inteligência · Treinamento</span><h2>Ensine a IA com o contexto real do negócio.</h2><p>O agente define como conversar; a base de conhecimento define o que ele pode consultar. Adicione texto, site público, PDF ou áudio — sempre isolado por empresa e por agente.</p></div><div className="knowledge-intro-actions">{onBack && <button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button>}<button className="secondary-button" type="button" disabled={loading || saving} onClick={() => void load(agentID)}>↻ Atualizar</button></div></section>
    {error && <div className="knowledge-alert" role="alert"><span>!</span><p>{error}</p><button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
    {notice && <div className="knowledge-notice" role="status"><span>✓</span><p>{notice}</p><button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso">×</button></div>}
    {loading ? <div className="knowledge-state"><span className="knowledge-spinner" /> Carregando base de conhecimento…</div> : !data?.agents.length ? <div className="knowledge-empty"><span>✦</span><strong>Nenhum agente disponível</strong><p>Crie um agente de IA antes de adicionar fontes de conhecimento.</p></div> : <><section className="knowledge-toolbar"><label><span>Treinando o agente</span><select value={agentID || ''} onChange={(event) => { const next = Number(event.target.value); setAgentID(next); setDraft(defaultDraft()); setNotice(null) }}>{data.agents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name}{agent.active ? '' : ' · pausado'}</option>)}</select></label><div className="knowledge-usage"><div><span>{formatChars(data.usage.chars)} / {formatChars(data.limits.max_total_chars)} caracteres</span><strong>{data.usage.sources} / {data.limits.max_sources_per_agent} fontes</strong></div><div className="knowledge-progress"><i style={{ width: `${usagePercent}%` }} /></div><small>{selectedAgent?.name || 'Agente selecionado'} · uso isolado por empresa</small></div></section><section className="entity-cross-links" style={{ marginTop: 4 }}><span>{data.sources.length ? 'Base usada por 1 agente' : 'Base ainda sem fontes'}</span>{onOpenAgent && selectedAgent && <button type="button" onClick={() => onOpenAgent(selectedAgent.id)}>Abrir agente: {selectedAgent.name} →</button>}</section><section className="knowledge-layout">{canManage && <div className="knowledge-form-panel"><header><div><span className="eyebrow">Nova fonte</span><h3>Escolha como ensinar</h3><p>O processamento acontece no servidor; chaves de IA nunca vão para o navegador.</p></div></header><div className="knowledge-kind-tabs" role="tablist" aria-label="Tipo de fonte">{(Object.keys(kindLabels) as KnowledgeDraft['kind'][]).map((kind) => <button key={kind} type="button" role="tab" aria-selected={draft.kind === kind} className={draft.kind === kind ? 'is-active' : ''} onClick={() => chooseKind(kind)}><strong>{kindLabels[kind].icon}</strong><span>{kindLabels[kind].label}</span></button>)}</div><div className="knowledge-form"><label><span>Nome da fonte</span><input value={draft.name} onChange={(event) => updateDraft('name', event.target.value)} placeholder={draft.kind === 'site' ? 'Ex.: Central de ajuda' : draft.kind === 'text' ? 'Ex.: Política de reembolso' : 'Nome opcional do arquivo'} /></label>{draft.kind === 'text' && <label><span>Conteúdo</span><textarea rows={11} value={draft.text} onChange={(event) => updateDraft('text', event.target.value)} placeholder="Cole aqui as informações que a IA deve conhecer…" /></label>}{draft.kind === 'site' && <label><span>URL pública</span><input type="url" value={draft.source_url} onChange={(event) => updateDraft('source_url', event.target.value)} placeholder="https://suaempresa.com.br/ajuda" /><small>Somente sites públicos HTTP/HTTPS. O servidor extrai o texto e ignora scripts.</small></label>}{(draft.kind === 'pdf' || draft.kind === 'audio') && <label className="knowledge-file-field"><span>Arquivo {draft.kind === 'pdf' ? 'PDF' : 'de áudio'}</span><input type="file" accept={draft.kind === 'pdf' ? 'application/pdf,.pdf' : 'audio/*,.ogg,.webm,.mp3,.wav,.m4a'} onChange={(event) => void handleFile(event.target.files?.[0])} /><small>{draft.file_name ? `${draft.file_name} · pronto para processar` : `${kindLabels[draft.kind].description} Limite: ${draft.kind === 'pdf' ? '10 MB' : '25 MB'}.`}</small></label>}{draft.kind === 'audio' && <div className="knowledge-info">A transcrição usa o STT configurado no servidor ou o provedor do agente. Se nenhum estiver configurado, o processamento será recusado.</div>}{draft.kind === 'pdf' && <div className="knowledge-info">PDFs com texto são extraídos automaticamente. Arquivos escaneados exigem OCR e serão recusados nesta primeira versão.</div>}{limitReached && <div className="knowledge-info">Limite da base atingido. Remova uma fonte antes de adicionar outra.</div>}<button className="primary-button knowledge-submit" type="button" disabled={saving || limitReached} onClick={() => void save()}>{saving ? 'Processando fonte…' : 'Adicionar à base'} <span>↗</span></button></div></div>}<section className="knowledge-sources-panel"><header><div><span className="eyebrow">Base atual</span><h3>Fontes do agente</h3></div><span className="knowledge-source-count">{data.sources.length}</span></header>{data.sources.length === 0 ? <div className="knowledge-empty-inline"><span>◌</span><strong>Comece adicionando uma fonte</strong><p>O conteúdo processado ficará disponível para a IA deste agente.</p></div> : <div className="knowledge-source-list">{data.sources.map((source) => <article className="knowledge-source-card" key={source.id}><div className={`knowledge-source-icon is-${source.kind}`}>{kindLabels[source.kind as KnowledgeDraft['kind']]?.icon || '•'}</div><div className="knowledge-source-copy"><div><strong>{source.name}</strong><span className={`knowledge-source-status is-${source.status}`}><i />{source.status === 'ready' ? 'Pronto' : source.status === 'error' ? 'Erro' : 'Processando'}</span></div><small>{sourceKindLabel(source.kind)} · {formatChars(source.content_chars)} caracteres{source.source_url ? ` · ${source.source_url}` : ''}</small>{source.error_message ? <p className="knowledge-source-error">{source.error_message}</p> : source.content_preview ? <p>{source.content_preview}</p> : null}</div>{canManage && <button className="knowledge-source-delete" type="button" disabled={actionID === source.id} onClick={() => void remove(source)} aria-label={`Remover ${source.name}`}>×</button>}</article>)}</div>}</section></section></>}
  </div>
}

export default KnowledgePage
