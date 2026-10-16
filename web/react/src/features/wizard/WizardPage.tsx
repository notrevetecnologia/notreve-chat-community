import { useCallback, useEffect, useRef, useState } from 'react'
import {
  agentPresets,
  bindAgentToChannel,
  connectChannel,
  createAgent,
  createChannel,
  createProvider,
  fetchChannelStatus,
  listProviderModels,
  listProviders,
  testAgent,
  testProvider,
  type AgentPreset,
  type Capacidade,
  type ChannelOperationResponse,
  type ChannelType,
  type Provider,
  type ProviderModel,
  type ProviderType,
} from './wizardApi'
import { LINKS } from '../../shared/links'
import './Wizard.css'

export type WizardPageProps = {
  onDone?: () => void
  onNavigate?: (view: string) => void
}

type ProviderDraft = {
  name: string
  base_url: string
  model: string
  api_key: string
  safe_mode: boolean
  capacidade: Capacidade
}

type TestTurn = {
  role: 'user' | 'assistant'
  text: string
  meta?: string
}

const steps = [
  { id: 'cerebro', label: 'Cérebro', hint: 'Escolha a IA' },
  { id: 'canal', label: 'Canal', hint: 'Conecte a entrada' },
  { id: 'agente', label: 'Agente', hint: 'Defina a persona' },
  { id: 'testar', label: 'Testar', hint: 'Confira o resultado' },
]

const emptyProviderDraft = (): ProviderDraft => ({
  name: '',
  base_url: '',
  model: '',
  api_key: '',
  safe_mode: true,
  capacidade: 'texto',
})

const defaultNotreveModels: ProviderModel[] = [
  { id: 'notreve-v1-pro', name: 'notreve-v1-pro · texto (recomendado)' },
  { id: 'notreve-vision', name: 'notreve-vision · multimodal' },
]

function mergeProviderModels(models: ProviderModel[], current: string): ProviderModel[] {
  const merged = [...defaultNotreveModels, ...models.filter((model) => !defaultNotreveModels.some((item) => item.id === model.id))]
  if (current && !merged.some((model) => model.id === current)) return [{ id: current, name: current }, ...merged]
  return merged
}

const channelOptions: { type: ChannelType; label: string; icon: string; description: string }[] = [
  { type: 'whatsapp', label: 'WhatsApp', icon: '◉', description: 'Número via Notreve WhatsApp API.' },
  { type: 'telegram', label: 'Telegram', icon: '✈', description: 'Bot criado no @BotFather.' },
  { type: 'webchat', label: 'Webchat', icon: '◈', description: 'Widget para o seu site.' },
]

function channelTypeLabel(type: ChannelType): string {
  return type === 'whatsapp' ? 'WhatsApp' : type === 'telegram' ? 'Telegram' : 'Webchat'
}

function errorMessage(value: unknown, fallback: string): string {
  return value instanceof Error ? value.message : fallback
}

export function WizardPage({ onDone, onNavigate }: WizardPageProps) {
  const [step, setStep] = useState(0)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const [providers, setProviders] = useState<Provider[]>([])
  const [loadingProviders, setLoadingProviders] = useState(true)
  const [providerChoice, setProviderChoice] = useState<'existing' | 'new'>('new')
  const [selectedProviderId, setSelectedProviderId] = useState<number | null>(null)
  const [providerType, setProviderType] = useState<ProviderType>('notreve')
  const [providerDraft, setProviderDraft] = useState<ProviderDraft>(emptyProviderDraft)
  const [providerId, setProviderId] = useState<number | null>(null)
  const [createdProviderId, setCreatedProviderId] = useState<number | null>(null)
  const [testingProvider, setTestingProvider] = useState(false)
  const [providerTestOk, setProviderTestOk] = useState<boolean | null>(null)
  const [providerTestManaged, setProviderTestManaged] = useState(false)
  const [providerModels, setProviderModels] = useState<ProviderModel[]>([])
  const [loadingProviderModels, setLoadingProviderModels] = useState(false)

  const [channelType, setChannelType] = useState<ChannelType>('whatsapp')
  const [channelName, setChannelName] = useState('')
  const [channelToken, setChannelToken] = useState('')
  const [channelId, setChannelId] = useState<number | null>(null)
  const [createdChannelId, setCreatedChannelId] = useState<number | null>(null)
  const [channelConnected, setChannelConnected] = useState(false)
  const [channelConnecting, setChannelConnecting] = useState(false)
  const [whatsappQr, setWhatsappQr] = useState<string | null>(null)
  const pollRef = useRef<number | null>(null)

  const [agentMode, setAgentMode] = useState('atendimento')
  const [agentName, setAgentName] = useState('')
  const [agentPrompt, setAgentPrompt] = useState('')
  const [agentTemperature, setAgentTemperature] = useState('equilibrado')
  const [createdAgentId, setCreatedAgentId] = useState<number | null>(null)

  const [testMessage, setTestMessage] = useState('')
  const [testTurns, setTestTurns] = useState<TestTurn[]>([])
  const [testLoading, setTestLoading] = useState(false)
  const [testError, setTestError] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    listProviders()
      .then((items) => {
        if (!active) return
        setProviders(items)
        if (items.length > 0) {
          setProviderChoice('existing')
          setSelectedProviderId(items[0].id)
        } else {
          setProviderChoice('new')
        }
      })
      .catch((loadError) => {
        if (active) setError(errorMessage(loadError, 'Não foi possível carregar os provedores.'))
      })
      .finally(() => {
        if (active) setLoadingProviders(false)
      })
    return () => {
      active = false
    }
  }, [])

  useEffect(() => {
    const preset = agentPresets.find((item) => item.id === agentMode) ?? agentPresets[0]
    setAgentName(preset.name)
    setAgentPrompt(preset.system_prompt)
    setAgentTemperature(preset.temperature_level)
    setCreatedAgentId(null)
  }, [agentMode])

  const stopChannelPolling = useCallback(() => {
    if (pollRef.current !== null) {
      window.clearInterval(pollRef.current)
      pollRef.current = null
    }
  }, [])

  useEffect(() => {
    return () => stopChannelPolling()
  }, [stopChannelPolling])

  const resetProviderTest = () => {
    setProviderTestOk(null)
    setProviderTestManaged(false)
  }

  const resetChannelConnection = () => {
    stopChannelPolling()
    setChannelConnected(false)
    setChannelConnecting(false)
    setWhatsappQr(null)
  }

  const updateProviderDraft = <K extends keyof ProviderDraft>(key: K, value: ProviderDraft[K]) => {
    setProviderDraft((current) => ({ ...current, [key]: value }))
    setCreatedProviderId(null)
    resetProviderTest()
  }

  const chooseProviderType = (value: ProviderType) => {
    setProviderType(value)
    setProviderModels([])
    setCreatedProviderId(null)
    resetProviderTest()
  }

  const loadProviderModels = useCallback(async () => {
    setError(null)
    setNotice(null)
    const isNotreve = providerType === 'notreve'
    if (isNotreve && !providerDraft.api_key.trim()) {
      setError('Cole o token da Notreve IA para carregar os modelos.')
      return
    }
    if (!isNotreve && !providerDraft.base_url.trim()) {
      setError('Informe o endpoint da sua IA para carregar os modelos.')
      return
    }
    setLoadingProviderModels(true)
    try {
      const items = await listProviderModels({
        provider_type: providerType,
        base_url: isNotreve ? '' : providerDraft.base_url.trim(),
        api_key: providerDraft.api_key.trim(),
      })
      setProviderModels(items)
      setNotice(items.length ? `${items.length} modelo(s) carregado(s).` : 'O provedor não retornou modelos.')
    } catch (loadError) {
      setError(errorMessage(loadError, 'Não foi possível carregar os modelos.'))
    } finally {
      setLoadingProviderModels(false)
    }
  }, [providerType, providerDraft.api_key, providerDraft.base_url])

  const chooseProviderChoice = (value: 'existing' | 'new') => {
    setProviderChoice(value)
    resetProviderTest()
  }

  const chooseExistingProvider = (id: number) => {
    setSelectedProviderId(id)
    resetProviderTest()
  }

  const submitProvider = useCallback(async (): Promise<number | null> => {
    if (providerChoice === 'existing') {
      if (selectedProviderId === null) {
        setError('Selecione um provedor existente ou crie um novo.')
        return null
      }
      setProviderId(selectedProviderId)
      return selectedProviderId
    }
    if (createdProviderId !== null) {
      setProviderId(createdProviderId)
      return createdProviderId
    }
    const isNotreve = providerType === 'notreve'
    if (!providerDraft.api_key.trim()) {
      setError(isNotreve ? 'Cole o token da Notreve IA para conectar.' : 'Informe o token de acesso da sua IA.')
      return null
    }
    if (!isNotreve && !providerDraft.name.trim()) {
      setError('Informe o nome do provedor.')
      return null
    }
    if (!isNotreve && !providerDraft.base_url.trim()) {
      setError('Informe o endpoint (base_url) da sua IA.')
      return null
    }
    setSaving(true)
    try {
      const created = await createProvider({
        name: isNotreve ? 'Notreve IA' : providerDraft.name.trim(),
        provider_type: providerType,
        base_url: isNotreve ? '' : providerDraft.base_url.trim(),
        model: providerDraft.model.trim(),
        api_key: providerDraft.api_key.trim(),
        active: true,
        safe_mode: isNotreve ? true : providerDraft.safe_mode,
        capacidade: isNotreve ? (providerDraft.model.includes('vision') ? 'multimodal' : 'texto') : providerDraft.capacidade,
      })
      setCreatedProviderId(created.id)
      setProviderId(created.id)
      return created.id
    } catch (saveError) {
      setError(errorMessage(saveError, 'Não foi possível salvar o provedor.'))
      return null
    } finally {
      setSaving(false)
    }
  }, [providerChoice, selectedProviderId, createdProviderId, providerType, providerDraft])

  const runProviderTest = useCallback(async () => {
    setError(null)
    setNotice(null)
    const isNotreve = providerChoice === 'existing'
      ? providers.find((item) => item.id === selectedProviderId)?.provider_type === 'notreve'
      : providerType === 'notreve'
    const id = await submitProvider()
    if (id === null) return
    setTestingProvider(true)
    setProviderTestOk(null)
    setProviderTestManaged(false)
    try {
      const result = await testProvider(id)
      if (result.ok) {
        setProviderTestOk(true)
        setNotice(`Conexão validada: ${result.message}`)
        return
      }
      if (isNotreve && (/HTTP\s*404/i.test(result.message) || /sem base url/i.test(result.message))) {
        setProviderTestOk(true)
        setProviderTestManaged(true)
        setNotice('Preset gerenciado pela Notreve — o teste automático não está disponível. O token será validado no primeiro atendimento.')
        return
      }
      setProviderTestOk(false)
      setError(result.message || 'Não foi possível conectar à IA.')
    } catch (testError) {
      setProviderTestOk(false)
      setError(errorMessage(testError, 'Não foi possível testar a conexão da IA.'))
    } finally {
      setTestingProvider(false)
    }
  }, [providerChoice, providers, selectedProviderId, providerType, submitProvider])

  const refreshChannelStatus = useCallback(async (id: number): Promise<boolean> => {
    try {
      const response: ChannelOperationResponse = await fetchChannelStatus(id)
      const state = response.result
      if (state?.connected && state.loggedIn) {
        setChannelConnected(true)
        setWhatsappQr(null)
        stopChannelPolling()
        return true
      }
      return false
    } catch {
      return false
    }
  }, [stopChannelPolling])

  const connectWhatsApp = useCallback(async (id: number) => {
    setError(null)
    setChannelConnecting(true)
    try {
      const response = await connectChannel(id)
      const result = response.result
      if (result?.connected || result?.loggedIn || result?.already) {
        setChannelConnected(true)
        setWhatsappQr(null)
        setNotice(result?.already ? 'Este número já está conectado.' : 'WhatsApp conectado com sucesso.')
        stopChannelPolling()
        return
      }
      if (result?.qr) {
        setWhatsappQr(result.qr)
        setChannelConnected(false)
        setNotice(result.already ? 'Este número já está conectado.' : 'Escaneie o QR Code no WhatsApp do número da empresa.')
      } else {
        setError('A API não retornou um QR Code agora. Tente novamente.')
      }
      stopChannelPolling()
      pollRef.current = window.setInterval(() => { void refreshChannelStatus(id) }, 3000)
    } catch (connectError) {
      setError(errorMessage(connectError, 'Não foi possível conectar o WhatsApp.'))
    } finally {
      setChannelConnecting(false)
    }
  }, [refreshChannelStatus, stopChannelPolling])

  const submitChannel = useCallback(async (): Promise<number | null> => {
    if (createdChannelId !== null) {
      setChannelId(createdChannelId)
      return createdChannelId
    }
    if (!channelName.trim()) {
      setError('Informe um nome para o canal.')
      return null
    }
    if (channelType === 'telegram' && !channelToken.trim()) {
      setError('Informe o token do bot do Telegram.')
      return null
    }
    setSaving(true)
    try {
      const created = await createChannel({
        type: channelType,
        name: channelName.trim(),
        active: true,
        respond_grupos: false,
        responder_chamadas: true,
        can_campaign: false,
        ...(channelType === 'telegram' ? { token: channelToken.trim() } : {}),
      })
      setCreatedChannelId(created.id)
      setChannelId(created.id)
      return created.id
    } catch (saveError) {
      setError(errorMessage(saveError, 'Não foi possível criar o canal.'))
      return null
    } finally {
      setSaving(false)
    }
  }, [createdChannelId, channelName, channelType, channelToken])

  const submitAgent = useCallback(async (): Promise<number | null> => {
    if (createdAgentId !== null) return createdAgentId
    if (!agentName.trim()) {
      setError('Informe um nome para o agente.')
      return null
    }
    setSaving(true)
    try {
      const created = await createAgent({
        name: agentName.trim(),
        system_prompt: agentPrompt,
        provider_id: providerId,
        temperature_level: agentTemperature,
        knowledge: '',
        knowledge_links: [],
        setor_ids: [],
        tool_ids: [],
        menu_opcoes: '',
        active: true,
      })
      if (channelId !== null) {
        await bindAgentToChannel(channelId, created.id)
      }
      setCreatedAgentId(created.id)
      return created.id
    } catch (saveError) {
      setError(errorMessage(saveError, 'Não foi possível criar o agente.'))
      return null
    } finally {
      setSaving(false)
    }
  }, [createdAgentId, agentName, agentPrompt, providerId, agentTemperature, channelType, channelId])

  const goNext = async () => {
    setError(null)
    setNotice(null)
    if (step === 0) {
      if (providerTestOk !== true) {
        setError('Teste a conexão da IA antes de continuar.')
        return
      }
      const id = await submitProvider()
      if (id === null) return
      setStep(1)
      return
    }
    if (step === 1) {
      const id = await submitChannel()
      if (id === null) return
      if (channelType === 'whatsapp' && !channelConnected) {
        await connectWhatsApp(id)
        return
      }
      setStep(2)
      return
    }
    if (step === 2) {
      const id = await submitAgent()
      if (id === null) return
      setStep(3)
    }
  }

  const goBack = () => {
    if (saving) return
    setError(null)
    setNotice(null)
    setStep((current) => Math.max(0, current - 1))
  }

  const finish = () => {
    onNavigate?.('inbox')
    onDone?.()
  }

  const sendTest = async () => {
    setTestError(null)
    const message = testMessage.trim()
    if (!message) {
      setTestError('Escreva uma mensagem para testar o agente.')
      return
    }
    if (createdAgentId === null) {
      setTestError('Crie o agente antes de testar.')
      return
    }
    setTestTurns((current) => [...current, { role: 'user', text: message }])
    setTestMessage('')
    setTestLoading(true)
    try {
      const result = await testAgent(createdAgentId, { message, provider_id: providerId })
      const details = [result.provider, result.model].filter(Boolean).join(' · ')
      const tools = result.used_tools?.length ? `tools: ${result.used_tools.join(', ')}` : ''
      const meta = [details, tools].filter(Boolean).join(' — ')
      setTestTurns((current) => [
        ...current,
        { role: 'assistant', text: result.reply || 'A IA não retornou resposta.', meta: meta || undefined },
      ])
    } catch (testErr) {
      setTestError(errorMessage(testErr, 'Não foi possível testar o agente.'))
    } finally {
      setTestLoading(false)
    }
  }

  const selectPreset = (preset: AgentPreset) => setAgentMode(preset.id)

  const renderCerebro = () => (
    <div className="wizard-step">
      <div className="wizard-step-heading">
        <h2>Qual cérebro vai responder?</h2>
        <p>Use qualquer IA compatível com OpenAI ou, recomendado, a <a href={LINKS.ia} target="_blank" rel="noreferrer">Notreve IA</a> — só com o token.</p>
      </div>
      {loadingProviders ? (
        <div className="wizard-loading"><span className="wizard-spinner" /> Carregando provedores…</div>
      ) : (
        <>
          {providers.length > 0 && (
            <div className="wizard-choice-group">
              <button type="button" className={`wizard-choice${providerChoice === 'existing' ? ' is-selected' : ''}`} onClick={() => chooseProviderChoice('existing')}>
                <strong>Usar provedor existente</strong>
                <span>Escolha uma IA já conectada nesta empresa.</span>
              </button>
              {providerChoice === 'existing' && (
                <div className="wizard-provider-list">
                  {providers.map((provider) => (
                    <label key={provider.id} className={`wizard-provider-item${selectedProviderId === provider.id ? ' is-selected' : ''}`}>
                      <input type="radio" name="provider" checked={selectedProviderId === provider.id} onChange={() => chooseExistingProvider(provider.id)} />
                      <span className="wizard-provider-copy">
                        <strong>{provider.name}</strong>
                        <small>{provider.provider_type === 'notreve' ? 'Notreve IA' : provider.model || 'IA própria'} · {provider.capacidade === 'multimodal' ? 'Multimodal' : 'Texto'}{provider.active ? '' : ' · pausado'}</small>
                      </span>
                    </label>
                  ))}
                </div>
              )}
            </div>
          )}
          <div className="wizard-choice-group">
            <button type="button" className={`wizard-choice${providerChoice === 'new' ? ' is-selected' : ''}`} onClick={() => chooseProviderChoice('new')}>
              <strong>Conectar uma nova IA</strong>
              <span>Preset Notreve IA (recomendado) ou a sua própria IA.</span>
            </button>
            {providerChoice === 'new' && (
              <div className="wizard-new-provider">
                <div className="wizard-toggle-row">
                  <button type="button" className={`wizard-toggle${providerType === 'notreve' ? ' is-selected' : ''}`} onClick={() => chooseProviderType('notreve')}>Notreve IA · recomendado</button>
                  <button type="button" className={`wizard-toggle${providerType === 'openai_compat' ? ' is-selected' : ''}`} onClick={() => chooseProviderType('openai_compat')}>IA própria</button>
                </div>
                {providerType === 'notreve' ? (
                  <div className="wizard-field-grid">
                    <label className="wizard-field">
                      <span>Token da Notreve IA</span>
                      <input type="password" value={providerDraft.api_key} onChange={(event) => updateProviderDraft('api_key', event.target.value)} placeholder="Cole o token aqui" autoComplete="new-password" />
                    </label>
                    <label className="wizard-field">
                      <span>Modelo</span>
                      <div className="wizard-model-row">
                        <select value={providerDraft.model} onChange={(event) => updateProviderDraft('model', event.target.value)}>
                          <option value="">Automático (recomendado)</option>
                          {mergeProviderModels(providerModels, providerDraft.model).map((model) => <option key={model.id} value={model.id}>{model.name}</option>)}
                        </select>
                        <button className="secondary-button" type="button" onClick={() => void loadProviderModels()} disabled={loadingProviderModels}>{loadingProviderModels ? 'Carregando…' : 'Carregar modelos'}</button>
                      </div>
                    </label>
                  </div>
                ) : (
                  <div className="wizard-field-grid">
                    <label className="wizard-field">
                      <span>Nome do provedor</span>
                      <input value={providerDraft.name} onChange={(event) => updateProviderDraft('name', event.target.value)} placeholder="Ex.: IA da empresa" />
                    </label>
                    <label className="wizard-field">
                      <span>Endpoint (base_url)</span>
                      <input value={providerDraft.base_url} onChange={(event) => updateProviderDraft('base_url', event.target.value)} placeholder="https://api.openai.com/v1" />
                    </label>
                    <label className="wizard-field">
                      <span>Modelo</span>
                      <input value={providerDraft.model} onChange={(event) => updateProviderDraft('model', event.target.value)} placeholder="gpt-4o-mini" />
                    </label>
                    <label className="wizard-field">
                      <span>Token de acesso</span>
                      <input type="password" value={providerDraft.api_key} onChange={(event) => updateProviderDraft('api_key', event.target.value)} placeholder="Cole o token aqui" autoComplete="new-password" />
                    </label>
                    <label className="wizard-field">
                      <span>Capacidade</span>
                      <select value={providerDraft.capacidade} onChange={(event) => updateProviderDraft('capacidade', event.target.value as Capacidade)}>
                        <option value="texto">Texto</option>
                        <option value="multimodal">Multimodal (com visão)</option>
                      </select>
                    </label>
                    <label className="wizard-check">
                      <input type="checkbox" checked={providerDraft.safe_mode} onChange={(event) => updateProviderDraft('safe_mode', event.target.checked)} />
                      <span>Modo seguro</span>
                    </label>
                  </div>
                )}
              </div>
            )}
          </div>
          <div className="wizard-provider-test">
            <button className="secondary-button" type="button" onClick={() => void runProviderTest()} disabled={testingProvider || saving}>
              {testingProvider ? 'Testando conexão…' : 'Testar conexão'}
            </button>
            {providerTestOk === true && (
              <span className="wizard-test-status is-ok">{providerTestManaged ? 'Preset gerenciado — teste indisponível' : 'Conexão validada'}</span>
            )}
            {providerTestOk === false && <span className="wizard-test-status is-fail">Falha no teste — veja o aviso acima</span>}
            {providerTestOk === null && <span className="wizard-test-status">Teste a conexão para liberar o próximo passo.</span>}
          </div>
        </>
      )}
    </div>
  )

  const renderCanal = () => (
    <div className="wizard-step">
      <div className="wizard-step-heading">
        <h2>Por onde os clientes chegam?</h2>
        <p>Escolha o canal de entrada e dê um nome para identificá-lo no painel.</p>
      </div>
      <div className="wizard-channel-grid">
        {channelOptions.map((option) => (
          <button key={option.type} type="button" className={`wizard-channel${channelType === option.type ? ' is-selected' : ''}`} onClick={() => { setChannelType(option.type); setCreatedChannelId(null); resetChannelConnection() }}>
            <span className="wizard-channel-icon">{option.icon}</span>
            <strong>{option.label}</strong>
            <small>{option.description}</small>
          </button>
        ))}
      </div>
      <label className="wizard-field">
        <span>Nome do canal</span>
        <input value={channelName} onChange={(event) => { setChannelName(event.target.value); setCreatedChannelId(null); resetChannelConnection() }} placeholder={`Ex.: ${channelTypeLabel(channelType)} principal`} />
      </label>
      {channelType === 'telegram' && (
        <label className="wizard-field">
          <span>Token do bot</span>
          <input type="password" value={channelToken} onChange={(event) => { setChannelToken(event.target.value); setCreatedChannelId(null) }} placeholder="123456789:AAFlorenBW..." autoComplete="new-password" />
        </label>
      )}
      {channelType === 'whatsapp' && createdChannelId !== null && (
        <div className="wizard-qr">
          {channelConnected ? (
            <div className="wizard-connect-ok"><span>✓</span> WhatsApp conectado. Você já pode continuar.</div>
          ) : (
            <>
              {whatsappQr ? (
                <img className="wizard-qr-image" src={whatsappQr} alt="QR Code para conectar o WhatsApp" />
              ) : channelConnecting ? (
                <div className="wizard-qr-placeholder"><span className="wizard-spinner" /> Gerando QR Code…</div>
              ) : (
                <div className="wizard-qr-placeholder">QR Code indisponível no momento.</div>
              )}
              {whatsappQr && (
                <>
                  <p className="wizard-qr-status"><span className="wizard-pulse" aria-hidden="true" /> Aguardando leitura…</p>
                  <span className="wizard-qr-hint">No celular: WhatsApp → Aparelhos conectados → Conectar um aparelho.</span>
                </>
              )}
              <button className="secondary-button" type="button" onClick={() => void connectWhatsApp(createdChannelId)} disabled={channelConnecting}>
                {channelConnecting ? 'Gerando…' : 'Gerar novo QR Code'}
              </button>
            </>
          )}
        </div>
      )}
      <div className="wizard-hint">
        {channelType === 'whatsapp' && <><strong>Conexão por QR Code</strong><span>O WhatsApp usa a <a href={LINKS.whatsappApi} target="_blank" rel="noreferrer">Notreve WhatsApp API</a> — contrate o serviço lá antes de conectar. Ao continuar, criamos o canal e mostramos o QR Code.</span></>}
        {channelType === 'telegram' && <><strong>Webhook do Telegram</strong><span>Use o token do @BotFather; depois configure o webhook na página de Canais.</span></>}
        {channelType === 'webchat' && <><strong>Widget do site</strong><span>Um widget será criado com um código pronto para colar no seu site.</span></>}
      </div>
    </div>
  )

  const renderAgente = () => (
    <div className="wizard-step">
      <div className="wizard-step-heading">
        <h2>Como o agente deve atender?</h2>
        <p>Comece com um perfil pronto e ajuste o texto se quiser.</p>
      </div>
      <div className="wizard-preset-grid">
        {agentPresets.map((preset) => (
          <button key={preset.id} type="button" className={`wizard-preset${agentMode === preset.id ? ' is-selected' : ''}`} onClick={() => selectPreset(preset)}>
            <strong>{preset.label}</strong>
            <small>{preset.description}</small>
          </button>
        ))}
      </div>
      <label className="wizard-field">
        <span>Nome do agente</span>
        <input value={agentName} onChange={(event) => { setAgentName(event.target.value); setCreatedAgentId(null) }} placeholder="Ex.: Concierge financeiro" />
      </label>
      <label className="wizard-field">
        <span>Instrução principal</span>
        <textarea rows={6} value={agentPrompt} onChange={(event) => { setAgentPrompt(event.target.value); setCreatedAgentId(null) }} placeholder="Explique o papel, tom e limites deste agente." />
      </label>
      <label className="wizard-field">
        <span>Estilo de resposta</span>
        <select value={agentTemperature} onChange={(event) => { setAgentTemperature(event.target.value); setCreatedAgentId(null) }}>
          <option value="preciso">Preciso</option>
          <option value="equilibrado">Equilibrado</option>
          <option value="criativo">Criativo</option>
        </select>
      </label>
      <div className="wizard-hint">
        <strong>Vínculos deste agente</strong>
        <span>
          Provedor: {providerId !== null ? (providers.find((item) => item.id === providerId)?.name || 'Novo provedor') : 'padrão'} · Canal: {channelTypeLabel(channelType)}{channelName.trim() ? ` (${channelName.trim()})` : ''}
        </span>
      </div>
    </div>
  )

  const renderTestar = () => (
    <div className="wizard-step">
      <div className="wizard-step-heading">
        <h2>Tudo pronto para testar</h2>
        <p>Escreva como se fosse um cliente e veja a resposta da IA aqui mesmo, sem abrir a Caixa de entrada.</p>
      </div>
      <div className="wizard-summary">
        <div><span>Provedor</span><strong>{providerId !== null ? (providers.find((item) => item.id === providerId)?.name || 'Novo provedor') : 'Padrão'}</strong></div>
        <div><span>Canal</span><strong>{channelTypeLabel(channelType)}</strong></div>
        <div><span>Agente</span><strong>{agentName || 'Sem nome'}</strong></div>
      </div>
      {testTurns.length > 0 && (
        <div className="wizard-chat" role="log" aria-live="polite">
          {testTurns.map((turn, index) => (
            <div key={index} className={`wizard-bubble is-${turn.role}`}>
              <p>{turn.text}</p>
              {turn.meta && <small>{turn.meta}</small>}
            </div>
          ))}
          {testLoading && (
            <div className="wizard-bubble is-assistant is-typing">
              <span className="wizard-spinner" /> Pensando…
            </div>
          )}
        </div>
      )}
      {testError && <div className="wizard-alert" role="alert"><span>!</span>{testError}<button type="button" onClick={() => setTestError(null)} aria-label="Fechar aviso">×</button></div>}
      <label className="wizard-field">
        <span>Mensagem de teste</span>
        <textarea rows={4} value={testMessage} onChange={(event) => setTestMessage(event.target.value)} placeholder="Olá! Quanto custa o serviço de vocês?" />
      </label>
      <button className="secondary-button wizard-test-button" type="button" onClick={() => void sendTest()} disabled={testLoading}>
        {testLoading ? 'Enviando…' : 'Enviar teste'}
      </button>
    </div>
  )

  const progress = ((step + 1) / steps.length) * 100
  const providerBlocked = step === 0 && providerTestOk !== true
  const channelBlocked = step === 1 && channelType === 'whatsapp' && createdChannelId !== null && !channelConnected
  const nextDisabled = saving || testingProvider || providerBlocked || channelBlocked
  const nextLabel = saving
    ? 'Salvando…'
    : testingProvider
      ? 'Testando…'
      : channelBlocked
        ? 'Aguardando conexão…'
        : 'Continuar'

  return (
    <div className="wizard-page">
      <section className="wizard-card">
        <header className="wizard-head">
          <span className="eyebrow">Configuração guiada</span>
          <h1>Monte seu atendimento em 4 passos</h1>
          <p>Conecte a IA, crie um canal, defina o agente e teste. Você pode ajustar tudo depois nas telas específicas.</p>
        </header>
        <ol className="wizard-stepper">
          {steps.map((item, index) => (
            <li key={item.id} className={index === step ? 'is-active' : index < step ? 'is-done' : ''}>
              <span className="wizard-step-dot">{index < step ? '✓' : index + 1}</span>
              <span className="wizard-step-text">
                <strong>{item.label}</strong>
                <small>{item.hint}</small>
              </span>
            </li>
          ))}
        </ol>
        <div className="wizard-progress" role="progressbar" aria-valuenow={Math.round(progress)} aria-valuemin={0} aria-valuemax={100}>
          <span style={{ width: `${progress}%` }} />
        </div>
        {error && <div className="wizard-alert" role="alert"><span>!</span>{error}<button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
        {notice && <div className="wizard-notice" role="status"><span>✓</span>{notice}<button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso">×</button></div>}
        <div className="wizard-body">
          {step === 0 && renderCerebro()}
          {step === 1 && renderCanal()}
          {step === 2 && renderAgente()}
          {step === 3 && renderTestar()}
        </div>
        <footer className="wizard-footer">
          {step > 0 && <button className="secondary-button" type="button" onClick={goBack} disabled={saving}>Voltar</button>}
          {step < 3 ? (
            <button className="primary-button" type="button" onClick={() => void goNext()} disabled={nextDisabled}>{nextLabel} <span aria-hidden="true">→</span></button>
          ) : (
            <button className="primary-button" type="button" onClick={finish}>Concluir e abrir Inbox <span aria-hidden="true">→</span></button>
          )}
        </footer>
      </section>
    </div>
  )
}

export default WizardPage
