import { MarkerType, type Edge, type Node, type XYPosition } from '@xyflow/react'
import type { ChatflowStepInput } from './types'

export type ChatflowNodeData = {
  tipo: string
  config: Record<string, any>
  ord: number
}

export type ChatflowNode = Node<ChatflowNodeData>

export const chatflowTodos = [
  'start',
  'message',
  'collect',
  'condition',
  'intent',
  'menu',
  'tool',
  'http',
  'set_variable',
  'delay',
  'business_hours',
  'tag',
  'queue',
  'satisfaction',
  'goto',
  'end',
  'note',
] as const

const defaultConfig = (tipo: string): Record<string, any> => {
  switch (tipo) {
    case 'start':
      return { next: 0 }
    case 'message':
      return { text: '', type: '', title: '', footer: '', options: [], sections: [], rows: [], next: 0 }
    case 'collect':
      return { prompt: '', variable: '', next: 0 }
    case 'condition':
      return { field: 'text', operator: 'contains', value: '', then: 0, else: 0 }
    case 'intent':
      return { prompt: '', branches: [{ label: '', next: 0 }], default: 0 }
    case 'menu':
      return { text: '', type: '', title: '', footer: '', options: [{ label: '', value: '', next: 0 }], default: 0 }
    case 'tool':
      return { name: '', inputs: {}, to_var: '', next: 0 }
    case 'http':
      return { method: 'GET', url: '', headers: {}, inputs: {}, to_var: '', next: 0 }
    case 'set_variable':
      return { variable: '', value: '', operation: 'set', next: 0 }
    case 'delay':
      return { seconds: 5, minutes: 0, next: 0 }
    case 'business_hours':
      return { setor: '', abertura: '', fechamento: '', dias: '', open: 0, closed: 0 }
    case 'tag':
      return { tags: '', next: 0 }
    case 'queue':
      return { setor: '', text: '' }
    case 'satisfaction':
      return { text: '', next: 0, skip: 0 }
    case 'goto':
      return { target: 0 }
    case 'end':
      return { text: '' }
    case 'note':
      return { text: '', next: 0 }
    default:
      return {}
  }
}

export const defaultConfigFor = defaultConfig

export const nextOrdFor = (nodes: ChatflowNode[]): number =>
  nodes.reduce((max, node) => Math.max(max, node.data.ord), 0) + 1

export function sourceHandleIds(tipo: string, config: Record<string, any>): string[] {
  if (tipo === 'start' || tipo === 'message' || tipo === 'collect' || tipo === 'tool' || tipo === 'http' || tipo === 'set_variable' || tipo === 'delay' || tipo === 'tag' || tipo === 'note') return ['next']
  if (tipo === 'condition') return ['then', 'else']
  if (tipo === 'satisfaction') return ['next', 'skip']
  if (tipo === 'business_hours') return ['open', 'closed']
  if (tipo === 'menu') {
    const count = Array.isArray(config?.options) ? config.options.length : 0
    return Array.from({ length: count }, (_, index) => `option-${index}`).concat(['default'])
  }
  if (tipo === 'intent') {
    const count = Array.isArray(config?.branches) ? config.branches.length : 0
    return Array.from({ length: count }, (_, index) => `branch-${index}`).concat(['default'])
  }
  return []
}

export function handleLabel(tipo: string, handleId: string, config: Record<string, any>): string {
  if (handleId === 'next') return 'Próximo'
  if (handleId === 'then') return 'Então'
  if (handleId === 'else') return 'Senão'
  if (handleId === 'default') return 'Padrão'
  if (handleId === 'open') return 'Aberto'
  if (handleId === 'closed') return 'Fechado'
  if (handleId === 'skip') return 'Sem participantes'
  if (handleId.startsWith('option-')) {
    const index = Number(handleId.slice('option-'.length))
    const options = Array.isArray(config?.options) ? config.options : []
    return options[index]?.label?.trim() || `Opção ${index + 1}`
  }
  if (handleId.startsWith('branch-')) {
    const index = Number(handleId.slice('branch-'.length))
    const branches = Array.isArray(config?.branches) ? config.branches : []
    return branches[index]?.label?.trim() || `Ramo ${index + 1}`
  }
  return ''
}

export function connectionTargetOrd(tipo: string, handleId: string, config: Record<string, any>): number | undefined {
  const read = (value: any): number | undefined => {
    const parsed = Number(value)
    return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined
  }
  if (handleId === 'next') return read(config?.next)
  if (handleId === 'then') return read(config?.then)
  if (handleId === 'else') return read(config?.else)
  if (handleId === 'default') return read(config?.default)
  if (handleId === 'open') return read(config?.open)
  if (handleId === 'closed') return read(config?.closed)
  if (handleId === 'skip') return read(config?.skip)
  if (handleId.startsWith('option-')) {
    const index = Number(handleId.slice('option-'.length))
    const option = Array.isArray(config?.options) ? config.options[index] : undefined
    return read(option?.next)
  }
  if (handleId.startsWith('branch-')) {
    const index = Number(handleId.slice('branch-'.length))
    const branch = Array.isArray(config?.branches) ? config.branches[index] : undefined
    return read(branch?.next)
  }
  return undefined
}

export function applyConnectionRef(config: Record<string, any>, handleId: string, targetOrd: number): Record<string, any> {
  if (handleId === 'next') return { ...config, next: targetOrd }
  if (handleId === 'then') return { ...config, then: targetOrd }
  if (handleId === 'else') return { ...config, else: targetOrd }
  if (handleId === 'default') return { ...config, default: targetOrd }
  if (handleId === 'open') return { ...config, open: targetOrd }
  if (handleId === 'closed') return { ...config, closed: targetOrd }
  if (handleId === 'skip') return { ...config, skip: targetOrd }
  if (handleId.startsWith('option-')) {
    const index = Number(handleId.slice('option-'.length))
    const options = Array.isArray(config.options) ? config.options.slice() : []
    if (options[index]) options[index] = { ...options[index], next: targetOrd }
    return { ...config, options }
  }
  if (handleId.startsWith('branch-')) {
    const index = Number(handleId.slice('branch-'.length))
    const branches = Array.isArray(config.branches) ? config.branches.slice() : []
    if (branches[index]) branches[index] = { ...branches[index], next: targetOrd }
    return { ...config, branches }
  }
  return config
}

function buildEdge(source: string, sourceHandle: string, target: string, label: string): Edge {
  return {
    id: `${source}-${sourceHandle}`,
    source,
    target,
    sourceHandle,
    targetHandle: 'in',
    type: 'smoothstep',
    label,
    markerEnd: { type: MarkerType.ArrowClosed, width: 18, height: 18 },
  }
}

export function buildGraphFromSteps(steps: ChatflowStepInput[]): { nodes: ChatflowNode[]; edges: Edge[] } {
  const sorted = [...steps].sort((a, b) => a.ord - b.ord)
  const working = ensureStartStep(sorted)
  const byOrd = new Map<number, ChatflowStepInput>()
  working.forEach((step) => byOrd.set(step.ord, step))

  const adjacency = new Map<string, string[]>()
  const ensure = (id: string) => {
    if (!adjacency.has(id)) adjacency.set(id, [])
  }
  const edges: Edge[] = []

  for (const step of working) {
    const id = String(step.ord)
    ensure(id)
    for (const handleId of sourceHandleIds(step.tipo, step.config)) {
      const targetOrd = connectionTargetOrd(step.tipo, handleId, step.config)
      if (targetOrd === undefined || !byOrd.has(targetOrd)) continue
      const target = String(targetOrd)
      const label = handleLabel(step.tipo, handleId, step.config)
      edges.push(buildEdge(id, handleId, target, label))
      adjacency.get(id)!.push(target)
    }
  }

  const startStep = working.find((step) => step.tipo === 'start')
  const startId = startStep ? String(startStep.ord) : String(working[0]?.ord ?? '')
  const positions = layoutPositions(working, adjacency, startId)
  const nodes: ChatflowNode[] = working.map((step) => ({
    id: String(step.ord),
    type: 'chatflow',
    position: positions.get(String(step.ord)) ?? { x: 60, y: 40 },
    data: { tipo: step.tipo, config: { ...step.config }, ord: step.ord },
  }))

  return { nodes, edges }
}

function ensureStartStep(sorted: ChatflowStepInput[]): ChatflowStepInput[] {
  if (sorted.some((step) => step.tipo === 'start')) return sorted
  const maxOrd = sorted.reduce((max, step) => Math.max(max, step.ord), 0)
  const next = sorted.length ? sorted[0].ord : 0
  return [...sorted, { ord: maxOrd + 1, tipo: 'start', config: { next } }]
}

function layoutPositions(sorted: ChatflowStepInput[], adjacency: Map<string, string[]>, startId: string): Map<string, XYPosition> {
  const depth = new Map<string, number>()
  if (startId) {
    const queue = [startId]
    depth.set(startId, 0)
    while (queue.length) {
      const current = queue.shift()
      const d = depth.get(current as string) ?? 0
      for (const next of adjacency.get(current as string) ?? []) {
        if (!depth.has(next)) {
          depth.set(next, d + 1)
          queue.push(next)
        }
      }
    }
  }

  const knownMax = [...depth.values()].reduce((max, d) => Math.max(max, d), 0)
  let extra = knownMax + 1
  for (const step of sorted) {
    const id = String(step.ord)
    if (!depth.has(id)) {
      depth.set(id, extra)
      extra += 1
    }
  }

  const byDepth = new Map<number, ChatflowStepInput[]>()
  for (const step of sorted) {
    const d = depth.get(String(step.ord)) ?? 0
    if (!byDepth.has(d)) byDepth.set(d, [])
    byDepth.get(d)!.push(step)
  }

  const positions = new Map<string, XYPosition>()
  for (const [d, list] of byDepth) {
    list.sort((a, b) => a.ord - b.ord)
    list.forEach((step, index) => {
      positions.set(String(step.ord), { x: d * 340 + 60, y: index * 200 + 40 })
    })
  }
  return positions
}

export function buildStepsFromGraph(nodes: ChatflowNode[], edges: Edge[]): ChatflowStepInput[] {
  const edgeBySourceHandle = new Map<string, Edge>()
  for (const edge of edges) {
    if (edge.sourceHandle) edgeBySourceHandle.set(`${edge.source}:${edge.sourceHandle}`, edge)
  }

  const seenOrd = new Set<number>()
  const steps: ChatflowStepInput[] = []
  for (const node of nodes) {
    if (seenOrd.has(node.data.ord)) continue
    seenOrd.add(node.data.ord)
    let config = { ...node.data.config }
    for (const handleId of sourceHandleIds(node.data.tipo, config)) {
      const edge = edgeBySourceHandle.get(`${node.id}:${handleId}`)
      const targetOrd = edge ? Number(edge.target) : 0
      const resolved = Number.isFinite(targetOrd) && targetOrd > 0 ? targetOrd : 0
      config = applyConnectionRef(config, handleId, resolved)
    }
    steps.push({ ord: node.data.ord, tipo: node.data.tipo, config })
  }

  return steps.sort((a, b) => a.ord - b.ord)
}
