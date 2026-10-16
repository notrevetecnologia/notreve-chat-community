import { useState, type ReactNode } from 'react'
import { EntityScreen, type EntityTab } from '../../shared/EntityScreen'
import {
  mergeModels,
  request,
  type ProviderDraft,
  type ProviderModel,
  type ProviderModelResponse,
  type ProviderTestResult,
} from './providersShared'
import { LINKS } from '../../shared/links'
import './Providers.css'

type TabKey = 'conexao' | 'modelos' | 'avancado'

const tabs: EntityTab[] = [
  { key: 'conexao', label: 'Conexão', hint: 'Tipo, token e endpoint' },
  { key: 'modelos', label: 'Modelos', hint: 'Escolher e testar' },
  { key: 'avancado', label: 'Avançado', hint: 'Capacidade e modo' },
]

export function ProviderEditorPage({
  draft,
  editingID,
  saving,
  onBack,
  onChange,
  onTypeChange,
  onSave,
}: {
  draft: ProviderDraft
  editingID: number | null
  saving: boolean
  onBack: () => void
  onChange: <K extends keyof ProviderDraft>(key: K, value: ProviderDraft[K]) => void
  onTypeChange: (providerType: ProviderDraft['provider_type']) => void
  onSave: () => void
}) {
  const direct = draft.provider_type === 'notreve'
  const editing = editingID !== null
  const [tab, setTab] = useState<TabKey>('conexao')
  const [models, setModels] = useState<ProviderModel[]>([])
  const [loadingModels, setLoadingModels] = useState(false)
  const [testing, setTesting] = useState(false)
  const [probeError, setProbeError] = useState<string | null>(null)
  const [probeNotice, setProbeNotice] = useState<string | null>(null)

  const probePayload = () => ({
    provider_id: editingID ?? undefined,
    provider_type: draft.provider_type,
    base_url: direct ? '' : draft.base_url.trim(),
    api_key: draft.api_key.trim(),
    capacidade: draft.capacidade,
  })

  const loadModels = async () => {
    setProbeError(null)
    setProbeNotice(null)
    if (direct && !draft.api_key.trim() && !editing) { setProbeError('Cole o token da Notreve IA para carregar os modelos.'); return }
    if (!direct && !draft.base_url.trim()) { setProbeError('Informe o endpoint para carregar os modelos.'); return }
    setLoadingModels(true)
    try {
      const payload = await request<ProviderModelResponse>('/api/react/providers/models', { method: 'POST', body: JSON.stringify(probePayload()) })
      setModels(payload.items || [])
      setProbeNotice(payload.items?.length ? `${payload.items.length} modelo(s) carregado(s).` : 'O provedor não retornou modelos.')
    } catch (loadError) {
      setProbeError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar os modelos.')
    } finally {
      setLoadingModels(false)
    }
  }

  const testConnection = async () => {
    setProbeError(null)
    setProbeNotice(null)
    if (direct && !draft.api_key.trim() && !editing) { setProbeError('Cole o token da Notreve IA para testar.'); return }
    if (!direct && !draft.base_url.trim()) { setProbeError('Informe o endpoint para testar.'); return }
    setTesting(true)
    try {
      const result = await request<ProviderTestResult>('/api/react/providers/test', { method: 'POST', body: JSON.stringify(probePayload()) })
      if (result.ok) setProbeNotice(`Conexão validada: ${result.message}`)
      else setProbeError(result.message || 'Não foi possível conectar à IA.')
    } catch (testError) {
      setProbeError(testError instanceof Error ? testError.message : 'Não foi possível testar a conexão.')
    } finally {
      setTesting(false)
    }
  }

  const modelOptions = mergeModels(models, draft.model)
  const title = direct ? 'Notreve IA' : (draft.name.trim() || (editing ? 'IA própria' : 'Nova IA própria'))

  return (
    <EntityScreen
      eyebrow={direct ? 'Provedor de IA' : editing ? 'Editar IA própria' : 'Nova IA própria'}
      title={title}
      onBack={onBack}
      backLabel="Voltar para os provedores"
      actions={
        <>
          <button className="secondary-button" type="button" onClick={onBack}>
            Voltar
          </button>
          <button className="primary-button" type="button" disabled={saving} onClick={onSave}>
            {saving ? 'Salvando…' : 'Salvar conexão'}
          </button>
        </>
      }
      tabs={tabs}
      activeTab={tab}
      onTabChange={(key) => setTab(key as TabKey)}
      alert={probeError ? <div className="entity-alert" role="alert"><span>!</span><p>{probeError}</p><button type="button" onClick={() => setProbeError(null)} aria-label="Fechar aviso">×</button></div> : undefined}
      notice={probeNotice ? <div className="entity-notice" role="status"><span>✓</span><p>{probeNotice}</p><button type="button" onClick={() => setProbeNotice(null)} aria-label="Fechar aviso">×</button></div> : undefined}
    >
      {tab === 'conexao' && (
        <div className="entity-tab-body">
          <p className="entity-tab-lead">
            {direct
              ? 'A Notreve gerencia endpoint e transcrição. Informe o token e escolha o modelo na aba Modelos.'
              : `Aponte para qualquer serviço compatível com a API da OpenAI. Recomendamos a Notreve IA (${LINKS.ia}). Informe endpoint, modelo e token.`}
          </p>
          <div className="entity-field-grid">
            <Field label="Tipo de provedor" help="Notreve IA é o preset recomendado; IA própria usa um endpoint compatível com OpenAI.">
              <select value={draft.provider_type} onChange={(event) => onTypeChange(event.target.value as ProviderDraft['provider_type'])}>
                <option value="notreve">Notreve IA (recomendado)</option>
                <option value="openai_compat">IA própria (OpenAI-compatível)</option>
              </select>
            </Field>
            {!direct && (
              <Field label="Nome do provedor *" help="Nome exibido para a equipe.">
                <input value={draft.name} onChange={(event) => onChange('name', event.target.value)} placeholder="Ex.: IA da empresa" />
              </Field>
            )}
            {!direct && (
              <Field label="Endpoint" wide help="URL base da API, terminando em /v1 na maioria dos provedores.">
                <input value={draft.base_url} onChange={(event) => onChange('base_url', event.target.value)} placeholder="https://api.openai.com/v1" />
              </Field>
            )}
            <Field
              label={direct ? 'Token da Notreve IA' : 'Token'}
              wide
              help={editing ? 'Deixe vazio para manter o token atual.' : 'Obrigatório para conectar o provedor.'}
            >
              <input
                type="password"
                value={draft.api_key}
                onChange={(event) => onChange('api_key', event.target.value)}
                placeholder={editing ? 'Token protegido' : 'Cole o token aqui'}
                autoComplete="new-password"
              />
            </Field>
          </div>
          <div className="entity-cross-links">
            <span>Escolha e teste o modelo na aba <b>Modelos</b>. Os ajustes de capacidade ficam em <b>Avançado</b>.</span>
          </div>
        </div>
      )}

      {tab === 'modelos' && (
        <div className="entity-tab-body">
          <p className="entity-tab-lead">Carregue a lista do provedor, escolha o modelo e valide a conexão. O teste não salva nada.</p>
          <div className="entity-field-grid">
            <Field label="Modelo" wide help="Deixe automático para o provedor escolher. Notreve usa notreve-v1-pro por padrão.">
              <div className="providers-model-row">
                <select value={draft.model} onChange={(event) => onChange('model', event.target.value)}>
                  <option value="">Automático (recomendado)</option>
                  {modelOptions.map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}
                </select>
                <button className="secondary-button" type="button" onClick={() => void loadModels()} disabled={loadingModels}>
                  {loadingModels ? 'Carregando…' : 'Carregar modelos'}
                </button>
              </div>
            </Field>
          </div>
          <div className="entity-section">
            <header>
              <div>
                <strong>Teste de conexão</strong>
                <Help>Consulta o provedor com as credenciais informadas, sem gravar alterações.</Help>
              </div>
            </header>
            <button className="secondary-button" type="button" onClick={() => void testConnection()} disabled={testing}>
              {testing ? 'Testando…' : 'Testar conexão'}
            </button>
          </div>
        </div>
      )}

      {tab === 'avancado' && (
        <div className="entity-tab-body">
          <p className="entity-tab-lead">Recursos do modelo e comportamento da conexão. A IA própria pode variar entre texto e multimodal.</p>
          <div className="entity-field-grid">
            <Field label="Capacidade" help={direct ? 'Definida pelo modelo da Notreve (vision = multimodal).' : 'Texto responde só a texto; multimodal entende imagem e arquivos.'}>
              <select
                value={direct ? (draft.model.includes('vision') ? 'multimodal' : 'texto') : draft.capacidade}
                disabled={direct}
                onChange={(event) => onChange('capacidade', event.target.value as ProviderDraft['capacidade'])}
              >
                <option value="texto">Texto</option>
                <option value="multimodal">Texto, imagem e arquivos</option>
              </select>
            </Field>
          </div>
          <section className="entity-section">
            <Toggle
              label="Modo seguro"
              help="Restringe o comportamento do modelo a respostas mais previsíveis. Recomendado para atendimento."
              checked={direct ? true : draft.safe_mode}
              disabled={direct}
              onChange={(value) => onChange('safe_mode', value)}
            />
            <Toggle
              label="Conexão ativa"
              help="Pausado deixa os agentes sem resposta sem apagar a configuração."
              checked={draft.active}
              onChange={(value) => onChange('active', value)}
            />
            <Toggle
              label="Geração de imagem"
              help="Habilita a ferramenta gerar_imagem para provedores compatíveis com OpenRouter (modalities de imagem)."
              checked={direct ? false : draft.capabilities.includes('imagem')}
              disabled={direct}
              onChange={(value) => onChange('capabilities', value ? ['imagem'] : [])}
            />
          </section>
        </div>
      )}
    </EntityScreen>
  )
}

function Help({ children }: { children: ReactNode }) {
  return <small className="entity-help">{children}</small>
}

function Field({ label, help, wide, children }: { label: string; help?: string; wide?: boolean; children: ReactNode }) {
  return (
    <label className={`entity-field${wide ? ' entity-field-wide' : ''}`}>
      <span>{label}</span>
      {children}
      {help && <Help>{help}</Help>}
    </label>
  )
}

function Toggle({ label, help, checked, disabled, onChange }: { label: string; help?: string; checked: boolean; disabled?: boolean; onChange: (value: boolean) => void }) {
  return (
    <label className="providers-toggle-row" style={{ display: 'flex', alignItems: 'flex-start', gap: 10, opacity: disabled ? 0.7 : 1 }}>
      <input type="checkbox" checked={checked} disabled={disabled} onChange={(event) => onChange(event.target.checked)} style={{ marginTop: 3 }} />
      <span style={{ display: 'grid', gap: 3 }}>
        <strong style={{ color: 'var(--ink)', fontSize: 13 }}>{label}</strong>
        {help && <Help>{help}</Help>}
      </span>
    </label>
  )
}

export default ProviderEditorPage
