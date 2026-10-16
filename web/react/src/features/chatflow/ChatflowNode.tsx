import { Fragment, type CSSProperties, type ReactNode } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import { chatflowStepDescriptions, chatflowStepLabels } from './types'
import { handleLabel, sourceHandleIds, type ChatflowNode } from './canvas'

const richLabels: Record<string, string> = {
  buttons: 'Botões',
  list: 'Lista',
  poll: 'Enquete',
}

const nodeIcons: Record<string, ReactNode> = {
  start: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M10 8.4l6 3.6-6 3.6z" />
    </>
  ),
  message: <path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />,
  collect: (
    <>
      <path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2" />
      <rect x="8" y="2" width="8" height="4" rx="1" />
      <path d="M9 12h6M9 16h4" />
    </>
  ),
  condition: (
    <>
      <path d="M6 3v12" />
      <circle cx="18" cy="6" r="3" />
      <circle cx="6" cy="18" r="3" />
      <path d="M18 9a9 9 0 0 1-9 9" />
    </>
  ),
  intent: (
    <>
      <path d="M12 3l1.9 5.1L19 10l-5.1 1.9L12 17l-1.9-5.1L5 10l5.1-1.9z" />
      <path d="M19 3v4M21 5h-4" />
    </>
  ),
  tool: <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />,
  queue: (
    <>
      <path d="M4 4h16v16H4z" />
      <path d="M4 13h4l2 3h4l2-3h4" />
    </>
  ),
  end: (
    <>
      <path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z" />
      <path d="M4 22v-7" />
    </>
  ),
  menu: (
    <>
      <path d="M8 6h13M8 12h13M8 18h13" />
      <path d="M3 6h.01M3 12h.01M3 18h.01" />
    </>
  ),
  delay: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v5l3 2" />
    </>
  ),
  set_variable: (
    <>
      <path d="M8 4c-2 0-2 3-2 4s0 4-2 4c2 0 2 3 2 4s0 4 2 4" />
      <path d="M16 4c2 0 2 3 2 4s0 4 2 4c-2 0-2 3-2 4s0 4-2 4" />
    </>
  ),
  goto: (
    <>
      <path d="M4 12h12" />
      <path d="M12 6l6 6-6 6" />
      <path d="M20 4v16" />
    </>
  ),
  business_hours: (
    <>
      <rect x="3" y="5" width="18" height="16" rx="2" />
      <path d="M3 9h18M8 3v4M16 3v4M12 13v3l2 1" />
    </>
  ),
  tag: (
    <>
      <path d="M20.6 13.4l-7.2 7.2a2 2 0 0 1-2.8 0l-7-7A2 2 0 0 1 3 12.2V5a2 2 0 0 1 2-2h7.2a2 2 0 0 1 1.4.6l7 7a2 2 0 0 1 0 2.8z" />
      <path d="M7.5 7.5h.01" />
    </>
  ),
  satisfaction: (
    <>
      <path d="M12 3l2.6 5.3 5.9.9-4.3 4.1 1 5.8L12 16.8 6.8 19.1l1-5.8L3.5 9.2l5.9-.9z" />
    </>
  ),
  note: (
    <>
      <path d="M4 4h16v11l-5 5H4z" />
      <path d="M20 15h-5v5" />
      <path d="M8 9h8M8 12h5" />
    </>
  ),
  http: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18M12 3a14 14 0 0 1 0 18 14 14 0 0 1 0-18z" />
    </>
  ),
}

function NodeIcon({ tipo }: { tipo: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {nodeIcons[tipo] ?? nodeIcons.message}
    </svg>
  )
}

function truncate(value: string, max = 60): string {
  return value.length > max ? `${value.slice(0, max).trimEnd()}…` : value
}

function summarize(tipo: string, config: Record<string, any>): string {
  const text = (value: any) => String(value ?? '').trim()
  switch (tipo) {
    case 'start':
      return 'Ponto de entrada do atendimento'
    case 'message': {
      const rich = config?.type ? `[${richLabels[config.type] || config.type}] ` : ''
      return rich + (truncate(text(config?.text)) || 'Mensagem sem texto')
    }
    case 'collect':
      return truncate(text(config?.prompt)) || 'Pergunta ao cliente'
    case 'condition': {
      const field = text(config?.field) || 'text'
      const operator = config?.operator === 'eq' ? '=' : 'contém'
      return `${field} ${operator} ${truncate(text(config?.value), 30) || '…'}`
    }
    case 'intent': {
      const branches = Array.isArray(config?.branches) ? config.branches.length : 0
      const prompt = text(config?.prompt)
      return `${branches} ${branches === 1 ? 'ramo' : 'ramos'}${prompt ? ` · ${truncate(prompt, 36)}` : ''}`
    }
    case 'tool':
      return truncate(text(config?.name)) || 'Tool do agente'
    case 'http': {
      const method = text(config?.method) || 'GET'
      const url = truncate(text(config?.url), 44)
      return url ? `${method} ${url}` : `${method} · sem URL`
    }
    case 'set_variable': {
      const name = text(config?.variable)
      const operation = text(config?.operation) || 'set'
      const value = truncate(text(config?.value), 30)
      return name ? `${name} ${operation} ${value || '…'}` : 'Definir variável'
    }
    case 'delay': {
      const seconds = Number(config?.seconds) || 0
      const minutes = Number(config?.minutes) || 0
      const total = seconds + minutes * 60
      return total > 0 ? `Aguardar ${total}s` : 'Sem espera configurada'
    }
    case 'business_hours':
      return text(config?.setor) ? `Horário do setor ${truncate(text(config.setor), 30)}` : 'Ramifica aberto / fechado'
    case 'tag': {
      const tags = text(config?.tags)
      return tags ? `Tags: ${truncate(tags, 40)}` : 'Aplicar tags ao contato'
    }
    case 'satisfaction':
      return truncate(text(config?.text), 44) || 'Envia a pesquisa de satisfação'
    case 'goto': {
      const target = Number(config?.target)
      return target > 0 ? `Saltar para o bloco #${target}` : 'Salto sem destino'
    }
    case 'note':
      return truncate(text(config?.text), 50) || 'Anotação (não executa)'
    case 'menu': {
      const options = Array.isArray(config?.options) ? config.options.filter((option: any) => text(option?.label)).length : 0
      const body = truncate(text(config?.text), 36)
      const rich = config?.type ? `[${richLabels[config.type] || config.type}] ` : ''
      return `${rich}${body || 'Menu'}${options ? ` · ${options} opç${options === 1 ? 'ão' : 'ões'}` : ''}`
    }
    case 'queue':
      return text(config?.setor) ? `Enfileirar no setor ${truncate(text(config.setor), 34)}` : 'Enfileirar para atendimento humano'
    case 'end':
      return truncate(text(config?.text)) || 'Encerra o atendimento'
    default:
      return chatflowStepDescriptions[tipo] ?? tipo
  }
}

export function ChatflowNodeComponent(props: NodeProps<ChatflowNode>) {
  const { data, selected } = props
  const { tipo, config, ord } = data
  const outputs = sourceHandleIds(tipo, config)
  const count = Math.max(outputs.length, 1)
  const label = chatflowStepLabels[tipo] ?? tipo

  return (
    <div className={`cf-node cf-node--${tipo}${selected ? ' is-selected' : ''}`}>
      {tipo === 'start' && <span className="cf-node-entry-flag">Entrada</span>}
      {tipo !== 'start' && <Handle type="target" position={Position.Left} id="in" className="cf-handle cf-handle--in" />}
      <div className="cf-node-header">
        <span className="cf-node-icon"><NodeIcon tipo={tipo} /></span>
        <span className="cf-node-type">{label}</span>
        <span className="cf-node-badge">#{ord}</span>
      </div>
      <div className="cf-node-body">{summarize(tipo, config)}</div>
      {outputs.length > 0 && (
        <div className="cf-node-outputs">
          {outputs.map((handleId, index) => {
            const top = `${((index + 1) / (count + 1)) * 100}%`
            const style: CSSProperties = { top }
            return (
              <Fragment key={handleId}>
                <span className="cf-handle-label" style={style}>{handleLabel(tipo, handleId, config)}</span>
                <Handle type="source" position={Position.Right} id={handleId} className="cf-handle cf-handle--out" style={style} />
              </Fragment>
            )
          })}
        </div>
      )}
    </div>
  )
}
