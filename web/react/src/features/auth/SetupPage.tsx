import { useEffect, useMemo, useState } from 'react'
import { fetchReactData } from './authApi'
import './Auth.css'

type SetupPageProps = {
  onNavigate: (surface: string) => void
  brandName?: string
}

type SetupStep = {
  id: string
  title: string
  description: string
  surface: string
  required: boolean
  done: boolean | null
}

type Collection<T> = { items?: T[] }
type TeamData = { sectors?: unknown[]; members?: unknown[] }

async function readCollection(path: string): Promise<number | null> {
  try {
    const data = await fetchReactData<Collection<unknown> | unknown[]>(path)
    return Array.isArray(data) ? data.length : data.items?.length ?? 0
  } catch {
    return null
  }
}

export function SetupPage({ onNavigate, brandName = 'Notreve' }: SetupPageProps) {
  const [steps, setSteps] = useState<SetupStep[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let active = true
    void Promise.all([
      readCollection('/api/react/providers'),
      readCollection('/api/react/channels'),
      readCollection('/api/react/agents'),
      fetchReactData<TeamData>('/api/react/team').then((data) => (data.sectors?.length ?? 0) + (data.members?.length ?? 0)).catch(() => null),
      readCollection('/api/react/integrations'),
      readCollection('/api/react/message-templates'),
    ]).then(([providers, channels, agents, team, integrations, templates]) => {
      if (!active) return
      setSteps([
        { id: 'provider', title: 'Conectar a IA', description: 'Escolha o provedor que responderá seus clientes.', surface: 'providers', required: true, done: providers === null ? null : providers > 0 },
        { id: 'channel', title: 'Criar um canal', description: 'Conecte WhatsApp, webchat ou outro canal de entrada.', surface: 'channels', required: true, done: channels === null ? null : channels > 0 },
        { id: 'agent', title: 'Criar o agente', description: 'Defina o jeito de falar, o objetivo e o canal da IA.', surface: 'agents', required: true, done: agents === null ? null : agents > 0 },
        { id: 'knowledge', title: 'Ensinar o agente', description: 'Adicione textos, site, PDF ou áudio para ele responder com contexto.', surface: 'knowledge', required: false, done: null },
        { id: 'team', title: 'Definir equipe e setores', description: 'Organize filas, setores e pessoas responsáveis pelo atendimento humano.', surface: 'team', required: true, done: team === null ? null : team > 0 },
        { id: 'integrations', title: 'Conectar tools', description: 'Dê ao agente acesso seguro a APIs e automações externas quando precisar.', surface: 'integrations', required: false, done: integrations === null ? null : integrations > 0 },
        { id: 'library', title: 'Personalizar mensagens', description: 'Revise respostas rápidas e templates da operação.', surface: 'library', required: false, done: templates === null ? null : templates > 0 },
      ])
      setLoading(false)
    })
    return () => {
      active = false
    }
  }, [])

  const requiredSteps = steps.filter((step) => step.required)
  const completed = requiredSteps.filter((step) => step.done === true).length
  const progress = requiredSteps.length === 0 ? 0 : Math.round((completed / requiredSteps.length) * 100)
  const unavailable = useMemo(() => steps.filter((step) => step.done === null).length, [steps])

  return (
    <main className="setup-page">
      <section className="setup-heading">
        <span className="eyebrow">Primeiros passos · {brandName}</span>
        <h1>Vamos preparar sua operação</h1>
        <p>Conclua o essencial para começar a atender. Você pode voltar a este guia quando quiser.</p>
      </section>
      <section className="setup-progress-card" aria-label="Progresso da configuração">
        <div className="setup-progress-copy">
          <strong>{completed} de {requiredSteps.length || 4} etapas essenciais</strong>
          <span>{progress}% preparado</span>
        </div>
        <div className="setup-progress-track"><span style={{ width: `${progress}%` }} /></div>
        {unavailable > 0 && <small>Algumas etapas não puderam ser verificadas com suas permissões atuais.</small>}
      </section>
      <section className="setup-list" aria-live="polite">
        {loading ? <div className="setup-loading">Carregando o estado da sua operação…</div> : steps.map((step, index) => (
          <article className={`setup-step-card${step.done === true ? ' done' : ''}`} key={step.id}>
            <div className="setup-step-number">{step.done === true ? '✓' : String(index + 1).padStart(2, '0')}</div>
            <div className="setup-step-copy">
              <div className="setup-step-title-row"><h2>{step.title}</h2>{step.required ? <span className="setup-required">Essencial</span> : <span className="setup-optional">Opcional</span>}</div>
              <p>{step.description}</p>
            </div>
            <button className={step.done === true ? 'setup-step-button secondary' : 'setup-step-button'} type="button" onClick={() => onNavigate(step.surface)}>
              {step.done === true ? 'Revisar' : 'Configurar'} <span aria-hidden="true">→</span>
            </button>
          </article>
        ))}
      </section>
    </main>
  )
}
