import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { loadSettings, saveSettings } from './settingsApi'
import type { OperationalSettings } from './types'
import './Settings.css'
import { Icon } from '../../shared/icons'

type SettingsPageProps = {
  onBack?: () => void
  onDirtyChange?: (dirty: boolean) => void
}

const defaultSettings: OperationalSettings = {
  modo_aberto: false,
  ia_monitora: false,
  distribuicao_automatica: false,
  followup_ativo: false,
  followup_horas: 24,
  ia_na_fila: false,
  detectar_bots: false, bot_patterns: '', registrar_falhas_aprendizado: true, transferir_falhas_humano: true,
  knowledge_top_k: 6,
  image_model: '', image_aspect_ratio: '1:1', logs_retencao_dias: 30,
}

export function SettingsPage({ onBack, onDirtyChange }: SettingsPageProps) {
  const [settings, setSettings] = useState<OperationalSettings>(defaultSettings)
  const [savedSettings, setSavedSettings] = useState<OperationalSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await loadSettings()
      const loadedSettings = data.operational || defaultSettings
      setSettings(loadedSettings)
      setSavedSettings(loadedSettings)
      setError(null)
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar as configurações.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const isDirty = savedSettings !== null && !settingsEqual(settings, savedSettings)

  useEffect(() => {
    onDirtyChange?.(isDirty)
    return () => onDirtyChange?.(false)
  }, [isDirty, onDirtyChange])

  useEffect(() => {
    if (!isDirty) return
    const warnBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault()
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', warnBeforeUnload)
    return () => window.removeEventListener('beforeunload', warnBeforeUnload)
  }, [isDirty])

  const update = <K extends keyof OperationalSettings>(key: K, value: OperationalSettings[K]) => {
    setSettings((current) => ({ ...current, [key]: value }))
    setNotice(null)
  }

  const save = async () => {
    if (settings.followup_horas < 1 || settings.followup_horas > 720) {
      setError('O intervalo do follow-up deve ficar entre 1 e 720 horas.')
      return
    }
    if (settings.knowledge_top_k < 1 || settings.knowledge_top_k > 20) {
      setError('O número de trechos de conhecimento deve ficar entre 1 e 20.')
      return
    }
    setSaving(true)
    setError(null)
    try {
      const data = await saveSettings(settings)
      const saved = data.operational || settings
      setSettings(saved)
      setSavedSettings(saved)
      setNotice('Configurações operacionais atualizadas.')
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : 'Não foi possível salvar as configurações.')
    } finally {
      setSaving(false)
    }
  }

  const confirmDiscard = () => !isDirty || window.confirm('Há alterações não salvas. Deseja descartá-las?')
  const reload = () => {
    if (confirmDiscard()) void load()
  }
  const back = () => onBack?.()

  return <div className="settings-page">
    <section className="settings-intro">
      <div>
        <span className="eyebrow">Configuração · Empresa</span>
        <h2>O atendimento do seu jeito.</h2>
        <p>Defina como a IA, a fila e a distribuição trabalham juntas dentro deste workspace.</p>
      </div>
      <div className="settings-intro-actions">
        {onBack && <button className="secondary-button" type="button" onClick={back}><Icon name="arrow-left" /> Visão geral</button>}
        <button className="secondary-button" type="button" disabled={loading || saving} onClick={reload}><Icon name="refresh" /> Atualizar</button>
      </div>
    </section>

    {error && <div className="settings-alert" role="alert"><span>!</span><p>{error}</p><button type="button" onClick={() => setError(null)} aria-label="Fechar aviso"><Icon name="close" /></button></div>}
    {notice && <div className="settings-notice" role="status"><span><Icon name="check" /></span><p>{notice}</p><button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso"><Icon name="close" /></button></div>}

    {loading ? <div className="settings-state"><span className="settings-spinner" /> Carregando configurações…</div> : <section className="settings-panel">
      <header className="settings-panel-header"><div><span className="eyebrow">Operação inteligente</span><h3>Regras do atendimento</h3><p>Essas preferências valem somente para a empresa atual.</p></div><div className="settings-save-actions"><span className={`settings-save-status${isDirty ? ' is-dirty' : ''}`} role="status"><Icon name={isDirty ? 'alert-circle' : 'check'} /> {isDirty ? 'Alterações não salvas' : 'Tudo salvo'}</span><button className="primary-button" type="button" disabled={saving || !isDirty} onClick={() => void save()}>{saving ? 'Salvando…' : 'Salvar configurações'} <Icon name="send" /></button></div></header>
      <div className="settings-sections">
        <SettingsSection eyebrow="Atendimento" title="Acesso e distribuição" description="Defina como a equipe enxerga a operação e como novos atendimentos entram na fila.">
          <ToggleCard title="Modo aberto" description="Permite que a equipe visualize a operação completa, respeitando suas permissões." checked={settings.modo_aberto} onChange={(value) => update('modo_aberto', value)} />
          <ToggleCard title="Distribuição automática" description="Distribui novos atendimentos entre os operadores disponíveis dos setores." checked={settings.distribuicao_automatica} onChange={(value) => update('distribuicao_automatica', value)} />
          <ToggleCard title="IA monitora a operação" description="Mantém a assistente acompanhando conversas e pronta para assumir quando configurada." checked={settings.ia_monitora} onChange={(value) => update('ia_monitora', value)} />
          <ToggleCard title="IA participa da fila" description="Coloca a IA no mesmo fluxo de distribuição quando houver espaço para atendimento." checked={settings.ia_na_fila} onChange={(value) => update('ia_na_fila', value)} />
        </SettingsSection>
        <SettingsSection eyebrow="Automações" title="Retomada e proteção" description="Controle ações automáticas e reduza interações indevidas com outros bots.">
          <ToggleCard title="Follow-up automático" description="Permite retomar conversas que ficaram sem resposta depois do período definido." checked={settings.followup_ativo} onChange={(value) => update('followup_ativo', value)} />
          <label className="settings-number-field"><span>Intervalo do follow-up</span><div><input type="number" min="1" max="720" value={settings.followup_horas} disabled={!settings.followup_ativo} onChange={(event) => update('followup_horas', Math.max(1, Math.min(720, Number(event.target.value) || 1)))} /><strong>horas</strong></div><small>Disponível quando o follow-up está ativo. Use entre 1 e 720 horas.</small></label>
          <ToggleCard title="Detectar outros bots" description="Ao detectar um provável outro bot, transfere para atendimento humano em vez de a IA responder." checked={settings.detectar_bots} onChange={(value) => update('detectar_bots', value)} />
          <label className="settings-number-field"><span>Padrões de bot (um por linha)</span><div className="settings-bot-patterns-input"><textarea rows={3} value={settings.bot_patterns} disabled={!settings.detectar_bots} onChange={(event) => update('bot_patterns', event.target.value)} placeholder="test&#10;teste&#10;bot&#10;robô" /></div><small>Mensagens com algum padrão ou rajadas de 6+ mensagens em 60s são tratadas como possível bot.</small></label>
        </SettingsSection>
        <SettingsSection eyebrow="Inteligência" title="Respostas e aprendizado" description="Ajuste a recuperação de conhecimento e o tratamento de falhas da IA.">
          <ToggleCard title="Registrar falhas no Aprendizado" description="Salva dúvidas sem resposta confiável para que a equipe possa corrigir e ensinar a IA depois." checked={settings.registrar_falhas_aprendizado} onChange={(value) => update('registrar_falhas_aprendizado', value)} />
          <ToggleCard title="Encaminhar falhas para humanos" description="Depois de responder com transparência, coloca falhas técnicas e dúvidas sem agente na fila humana." checked={settings.transferir_falhas_humano} onChange={(value) => update('transferir_falhas_humano', value)} />
          <label className="settings-number-field"><span>Trechos de conhecimento por resposta</span><div><input type="number" min="1" max="20" value={settings.knowledge_top_k} onChange={(event) => update('knowledge_top_k', Math.max(1, Math.min(20, Number(event.target.value) || 6)))} /><strong>trechos</strong></div><small>A IA busca os trechos mais relevantes da base. Use entre 1 e 20.</small></label>
        </SettingsSection>
        <SettingsSection eyebrow="Mídia e dados" title="Geração de imagens e retenção" description="Configure preferências técnicas usadas no dia a dia da empresa.">
          <label className="settings-number-field"><span>Modelo de geração de imagem</span><div><input value={settings.image_model} onChange={(event) => update('image_model', event.target.value)} placeholder="Vazio = modelo do provedor" /></div><small>Modelo compatível com OpenRouter. Vazio usa o modelo principal do provedor.</small></label>
          <label className="settings-number-field"><span>Proporção padrão da imagem</span><div><select value={settings.image_aspect_ratio} onChange={(event) => update('image_aspect_ratio', event.target.value)}><option value="1:1">1:1 (quadrada)</option><option value="16:9">16:9 (paisagem)</option><option value="9:16">9:16 (retrato)</option><option value="4:3">4:3</option><option value="3:4">3:4</option></select></div><small>Usada quando o cliente não pede uma proporção específica.</small></label>
          <label className="settings-number-field"><span>Retenção dos logs</span><div><input type="number" min="1" max="3650" value={settings.logs_retencao_dias} onChange={(event) => update('logs_retencao_dias', Math.max(1, Math.min(3650, Number(event.target.value) || 30)))} /><strong>dias</strong></div><small>Logs mais antigos que isso são removidos automaticamente.</small></label>
        </SettingsSection>
      </div>
      <div className="settings-helper"><span aria-hidden="true"><Icon name="info" /></span><p><strong>Configure a inteligência nos lugares certos.</strong> Escolha a IA em <em>Provedores</em>, ensine o agente pela <em>Base de conhecimento</em> e conecte ações externas em <em>Integrações</em>. Os detalhes técnicos da plataforma ficam protegidos no control plane.</p></div>
    </section>}
  </div>
}

function settingsEqual(left: OperationalSettings, right: OperationalSettings) {
  return (Object.keys(defaultSettings) as Array<keyof OperationalSettings>).every((key) => left[key] === right[key])
}

function SettingsSection({ eyebrow, title, description, children }: { eyebrow: string; title: string; description: string; children: ReactNode }) {
  return <section className="settings-section"><header><span className="eyebrow">{eyebrow}</span><h4>{title}</h4><p>{description}</p></header><div className="settings-grid">{children}</div></section>
}

function ToggleCard({ title, description, checked, onChange }: { title: string; description: string; checked: boolean; onChange: (value: boolean) => void }) {
  return <label className={`settings-toggle-card${checked ? ' is-enabled' : ''}`}><span className="settings-toggle-copy"><strong>{title}</strong><small>{description}</small></span><input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} /><i aria-hidden="true" /></label>
}

export default SettingsPage
