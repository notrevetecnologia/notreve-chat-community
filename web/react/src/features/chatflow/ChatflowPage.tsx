import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent, type MouseEvent as ReactMouseEvent } from 'react'
import { ReactFlow, ReactFlowProvider, Background, Controls, MiniMap, applyNodeChanges, applyEdgeChanges, useReactFlow, MarkerType, type Connection, type Edge, type EdgeChange, type NodeChange, type XYPosition, type NodeTypes } from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { attachChannel, createChatflow, deleteChatflow, loadChatflows, toggleChatflow, updateChatflow } from './chatflowApi'
import { chatflowStepLabels, chatflowStepDescriptions, type Chatflow, type ChatflowChannel, type ChatflowData, type ChatflowDraft } from './types'
import { applyConnectionRef, buildGraphFromSteps, buildStepsFromGraph, chatflowTodos, defaultConfigFor, handleLabel, nextOrdFor, type ChatflowNode, type ChatflowNodeData } from './canvas'
import { ChatflowNodeComponent } from './ChatflowNode'
import './Chatflow.css'

type ChatflowPageProps = {
  onBack?: () => void
  onOpenChannels?: () => void
  initialFlowId?: number | null
}

type FlowMeta = {
  nome: string
  version: number
  ativo: boolean
  config: Record<string, any>
}

const emptyMeta = (): FlowMeta => ({ nome: '', version: 1, ativo: false, config: {} })

const nodeTypes: NodeTypes = { chatflow: ChatflowNodeComponent }

export default function ChatflowPage({ onBack, onOpenChannels, initialFlowId }: ChatflowPageProps) {
  return (
    <ReactFlowProvider>
      <ChatflowEditor onBack={onBack} onOpenChannels={onOpenChannels} initialFlowId={initialFlowId} />
    </ReactFlowProvider>
  )
}

export { ChatflowPage }

function ChatflowEditor({ onBack, onOpenChannels, initialFlowId }: ChatflowPageProps) {
  const [data, setData] = useState<ChatflowData | null>(null)
  const [selectedID, setSelectedID] = useState<number | null>(null)
  const [meta, setMeta] = useState<FlowMeta | null>(null)
  const [nodes, setNodes] = useState<ChatflowNode[]>([])
  const [edges, setEdges] = useState<Edge[]>([])
  const [selectedNodeID, setSelectedNodeID] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [actionID, setActionID] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [fitKey, setFitKey] = useState(0)
  const [agentsOpen, setAgentsOpen] = useState(false)
  const [paletteOpen, setPaletteOpen] = useState(true)

  const nodesRef = useRef<ChatflowNode[]>(nodes)
  const edgesRef = useRef<Edge[]>(edges)
  nodesRef.current = nodes
  edgesRef.current = edges

  const rf = useReactFlow()

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const payload = await loadChatflows()
      setData(payload)
      setError(null)
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar os chat flows.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const selected = useMemo(() => data?.items.find((flow) => flow.id === selectedID) ?? null, [data?.items, selectedID])

  const openFlow = useCallback((flow: Chatflow | null) => {
    if (flow) {
      const { nodes: nextNodes, edges: nextEdges } = buildGraphFromSteps(flow.steps)
      setSelectedID(flow.id)
      setMeta({ nome: flow.nome, version: flow.version, ativo: flow.ativo, config: flow.config || {} })
      setNodes(nextNodes)
      setEdges(nextEdges)
      setSelectedNodeID(nextNodes.length ? nextNodes[0].id : null)
      setFitKey((key) => key + 1)
    } else {
      setSelectedID(null)
      setMeta({ ...emptyMeta(), config: {} })
      setNodes([])
      setEdges([])
      setSelectedNodeID(null)
      setFitKey((key) => key + 1)
    }
    setError(null)
    setNotice(null)
  }, [])

  const selectFlow = (flow: Chatflow) => openFlow(flow)
  const startNew = () => openFlow(null)

  const initialFlowRef = useRef<number | null>(initialFlowId ?? null)

  useEffect(() => {
    const pending = initialFlowRef.current
    if (!pending || !data) return
    const flow = data.items.find((item) => item.id === pending)
    if (flow) openFlow(flow)
    initialFlowRef.current = null
  }, [data, openFlow])

  useEffect(() => {
    if (fitKey === 0) return
    const timer = window.setTimeout(() => { void rf.fitView({ padding: 0.3 }) }, 60)
    return () => window.clearTimeout(timer)
  }, [fitKey, rf])

  const rename = (value: string) => setMeta((current) => current ? { ...current, nome: value } : current)

  const addNodeAt = useCallback((tipo: string, position?: XYPosition) => {
    if (tipo === 'start' && nodesRef.current.some((node) => node.data.tipo === 'start')) {
      setError('O fluxo já tem um bloco de início.')
      return
    }
    const ord = nextOrdFor(nodesRef.current)
    const node: ChatflowNode = {
      id: String(ord),
      type: 'chatflow',
      position: position ?? autoDropPosition(nodesRef.current),
      data: { tipo, config: defaultConfigFor(tipo), ord },
    }
    setNodes((current) => [...current, node])
    setSelectedNodeID(String(ord))
  }, [])

  const updateNodeConfig = useCallback((nodeId: string, patch: Record<string, any>) => {
    setNodes((current) => current.map((node) => node.id === nodeId
      ? { ...node, data: { ...node.data, config: { ...node.data.config, ...patch } } }
      : node))
  }, [])

  const addIntentBranch = useCallback((nodeId: string) => {
    setNodes((current) => current.map((node) => node.id === nodeId
      ? { ...node, data: { ...node.data, config: { ...node.data.config, branches: [...(node.data.config.branches || []), { label: '', next: 0 }] } } }
      : node))
  }, [])

  const removeIntentBranch = useCallback((nodeId: string, index: number) => {
    setNodes((current) => current.map((node) => {
      if (node.id !== nodeId) return node
      const branches = (node.data.config.branches || []).slice()
      branches.splice(index, 1)
      return { ...node, data: { ...node.data, config: { ...node.data.config, branches } } }
    }))
    setEdges((current) => {
      const removedHandle = `branch-${index}`
      const next = current.filter((edge) => !(edge.source === nodeId && edge.sourceHandle === removedHandle))
      return next.map((edge) => {
        if (edge.source === nodeId && edge.sourceHandle && edge.sourceHandle.startsWith('branch-')) {
          const branchIndex = Number(edge.sourceHandle.slice('branch-'.length))
          if (branchIndex > index) {
            return { ...edge, sourceHandle: `branch-${branchIndex - 1}`, id: `${nodeId}-branch-${branchIndex - 1}` }
          }
        }
        return edge
      })
    })
  }, [])

  const addMenuOption = useCallback((nodeId: string) => {
    setNodes((current) => current.map((node) => node.id === nodeId
      ? { ...node, data: { ...node.data, config: { ...node.data.config, options: [...(node.data.config.options || []), { label: '', value: '', next: 0 }] } } }
      : node))
  }, [])

  const removeMenuOption = useCallback((nodeId: string, index: number) => {
    setNodes((current) => current.map((node) => {
      if (node.id !== nodeId) return node
      const options = (node.data.config.options || []).slice()
      options.splice(index, 1)
      return { ...node, data: { ...node.data, config: { ...node.data.config, options } } }
    }))
    setEdges((current) => {
      const removedHandle = `option-${index}`
      const next = current.filter((edge) => !(edge.source === nodeId && edge.sourceHandle === removedHandle))
      return next.map((edge) => {
        if (edge.source === nodeId && edge.sourceHandle && edge.sourceHandle.startsWith('option-')) {
          const optionIndex = Number(edge.sourceHandle.slice('option-'.length))
          if (optionIndex > index) {
            return { ...edge, sourceHandle: `option-${optionIndex - 1}`, id: `${nodeId}-option-${optionIndex - 1}` }
          }
        }
        return edge
      })
    })
  }, [])

  const removeNode = useCallback((nodeId: string) => {
    const node = nodesRef.current.find((item) => item.id === nodeId)
    if (node?.data.tipo === 'start') {
      setError('O bloco de início não pode ser removido.')
      return
    }
    setNodes((current) => current.filter((node) => node.id !== nodeId))
    setEdges((current) => current.filter((edge) => edge.source !== nodeId && edge.target !== nodeId))
    setSelectedNodeID((current) => current === nodeId ? null : current)
  }, [])

  const handleNodesChange = useCallback((changes: NodeChange<ChatflowNode>[]) => {
    const guarded = changes.filter((change) => {
      if (change.type !== 'remove') return true
      const node = nodesRef.current.find((item) => item.id === change.id)
      return !(node && node.data.tipo === 'start')
    })
    const removed = new Set(guarded.filter((change) => change.type === 'remove').map((change) => change.id))
    setNodes((current) => applyNodeChanges(guarded, current))
    if (removed.size) {
      setEdges((current) => current.filter((edge) => !removed.has(edge.source) && !removed.has(edge.target)))
      setSelectedNodeID((current) => current && removed.has(current) ? null : current)
    }
  }, [])

  const handleEdgesChange = useCallback((changes: EdgeChange[]) => {
    const removes = changes.filter((change) => change.type === 'remove')
    const removedIds = new Set(removes.map((change) => change.id))
    const removedEdges = edgesRef.current.filter((edge) => removedIds.has(edge.id))
    setEdges((current) => applyEdgeChanges(changes, current))
    if (removedEdges.length) {
      setNodes((current) => current.map((node) => {
        let config = node.data.config
        for (const edge of removedEdges) {
          if (edge.source === node.id && edge.sourceHandle) config = applyConnectionRef(config, edge.sourceHandle, 0)
        }
        return config === node.data.config ? node : { ...node, data: { ...node.data, config } }
      }))
    }
  }, [])

  const handleConnect = useCallback((connection: Connection) => {
    const { source, sourceHandle, target } = connection
    if (!sourceHandle || !target || source === target) return
    const sourceNode = nodesRef.current.find((node) => node.id === source)
    if (!sourceNode) return
    const targetNode = nodesRef.current.find((node) => node.id === target)
    if (!targetNode || targetNode.data.tipo === 'start') return
    const targetOrd = Number(target)
    if (!Number.isFinite(targetOrd) || targetOrd <= 0) return
    const edge: Edge = {
      id: `${source}-${sourceHandle}`,
      source,
      target,
      sourceHandle,
      targetHandle: 'in',
      type: 'smoothstep',
      label: handleLabel(sourceNode.data.tipo, sourceHandle, sourceNode.data.config),
      markerEnd: { type: MarkerType.ArrowClosed, width: 18, height: 18 },
    }
    setEdges((current) => [...current.filter((item) => !(item.source === source && item.sourceHandle === sourceHandle)), edge])
    setNodes((current) => current.map((node) => node.id === source
      ? { ...node, data: { ...node.data, config: applyConnectionRef(node.data.config, sourceHandle, targetOrd) } }
      : node))
  }, [])

  const onNodeClick = useCallback((_event: ReactMouseEvent, node: ChatflowNode) => setSelectedNodeID(node.id), [])
  const onPaneClick = useCallback(() => setSelectedNodeID(null), [])

  const onDragOver = useCallback((event: DragEvent) => {
    event.preventDefault()
    event.dataTransfer.dropEffect = 'move'
  }, [])

  const onDrop = useCallback((event: DragEvent) => {
    event.preventDefault()
    const tipo = event.dataTransfer.getData('application/cf-node')
    if (!tipo) return
    const position = rf.screenToFlowPosition({ x: event.clientX, y: event.clientY })
    addNodeAt(tipo, position)
  }, [rf, addNodeAt])

  const save = async () => {
    if (!meta) return
    if (!meta.nome.trim()) {
      setError('Dê um nome ao chat flow.')
      return
    }
    const startCount = nodesRef.current.filter((node) => node.data.tipo === 'start').length
    if (startCount !== 1) {
      setError('O fluxo precisa de exatamente um bloco de início.')
      return
    }
    const validationError = validateFlow(nodesRef.current)
    if (validationError) {
      setError(validationError)
      return
    }
    setSaving(true)
    setError(null)
    setNotice(null)
    try {
      const steps = buildStepsFromGraph(nodesRef.current, edgesRef.current)
      const normalized: ChatflowDraft = {
        nome: meta.nome.trim(),
        version: meta.version,
        ativo: meta.ativo,
        config: meta.config,
        steps,
      }
      if (selectedID) {
        await updateChatflow(selectedID, normalized)
        setNotice('Chat flow atualizado.')
      } else {
        const created = await createChatflow(normalized)
        setSelectedID(created.id)
        setNotice('Chat flow criado.')
      }
      await load()
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : 'Não foi possível salvar o chat flow.')
    } finally {
      setSaving(false)
    }
  }

  const toggle = async (flow: Chatflow) => {
    setActionID(flow.id)
    try {
      await toggleChatflow(flow.id)
      setMeta((current) => current ? { ...current, ativo: !flow.ativo } : current)
      setNotice(flow.ativo ? 'Chat flow desativado.' : 'Chat flow ativado.')
      await load()
    } catch (toggleError) {
      setError(toggleError instanceof Error ? toggleError.message : 'Não foi possível alterar o estado.')
    } finally {
      setActionID(null)
    }
  }

  const remove = async (flow: Chatflow) => {
    if (!window.confirm(`Excluir o chat flow “${flow.nome}”?`)) return
    setActionID(flow.id)
    try {
      await deleteChatflow(flow.id)
      if (selectedID === flow.id) startNew()
      setNotice('Chat flow excluído.')
      await load()
    } catch (removeError) {
      setError(removeError instanceof Error ? removeError.message : 'Não foi possível excluir o chat flow.')
    } finally {
      setActionID(null)
    }
  }

  const attach = async (channel: ChatflowChannel, chatflowID: number | null) => {
    setActionID(-channel.id)
    try {
      const flowID = chatflowID ?? channel.chatflow_id
      if (flowID == null) return
      const payload = await attachChannel(flowID, channel.id, chatflowID !== null)
      if (payload?.channels) setData((current) => current ? { ...current, channels: payload.channels } : current)
      setNotice('Vínculo do canal atualizado.')
    } catch (attachError) {
      setError(attachError instanceof Error ? attachError.message : 'Não foi possível vincular o canal.')
    } finally {
      setActionID(null)
    }
  }

  const selectedNode = nodes.find((node) => node.id === selectedNodeID) ?? null
  const activeFlow = selectedID ? data?.items.find((flow) => flow.id === selectedID) ?? null : null
  const startExists = nodes.some((node) => node.data.tipo === 'start')

  return (
    <div className={`cf-full${selectedNode ? ' cf-full--inspector' : ''}`}>
      {error && <div className="cf-toast cf-toast--error" role="alert"><span>!</span><p>{error}</p><button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}
      {notice && <div className="cf-toast cf-toast--ok" role="status"><span>✓</span><p>{notice}</p><button type="button" onClick={() => setNotice(null)} aria-label="Fechar aviso">×</button></div>}

      <div className="cf-stage">
        {nodes.length === 0 && (
          <div className="cf-stage-empty">
            <strong>Comece a montar seu fluxo</strong>
            <p>Adicione blocos pela barra de baixo e ligue os conectores para definir o percurso.</p>
          </div>
        )}
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          onNodesChange={handleNodesChange}
          onEdgesChange={handleEdgesChange}
          onConnect={handleConnect}
          onNodeClick={onNodeClick}
          onPaneClick={onPaneClick}
          onDragOver={onDragOver}
          onDrop={onDrop}
          connectOnClick
          className="cf-reactflow"
          defaultEdgeOptions={{ type: 'smoothstep' }}
        >
          <Background gap={22} size={1.5} />
          <MiniMap pannable zoomable nodeColor="#cfe4e1" />
          <Controls />
        </ReactFlow>
      </div>

      <div className="cf-bar">
        <button className="cf-bar-btn" type="button" onClick={startNew} title="Novo fluxo">+</button>
        <select
          className="cf-bar-select"
          value={selectedID ?? ''}
          onChange={(event) => {
            const value = event.target.value
            if (!value) { startNew(); return }
            const flow = data?.items.find((item) => item.id === Number(value))
            if (flow) selectFlow(flow)
          }}
        >
          <option value="">Novo fluxo…</option>
          {(data?.items ?? []).map((flow) => (
            <option key={flow.id} value={flow.id}>{flow.nome}{flow.ativo ? '' : ' (inativo)'}</option>
          ))}
        </select>
        {meta && <input className="cf-bar-input" value={meta.nome} onChange={(event) => rename(event.target.value)} placeholder="Nome do fluxo" />}
        {selectedID && activeFlow && (
          <button className={`cf-bar-btn${meta?.ativo ? ' is-on' : ''}`} type="button" disabled={actionID === selectedID} onClick={() => void toggle(activeFlow)}>
            {meta?.ativo ? 'Ativo' : 'Inativo'}
          </button>
        )}
        {meta && <button className="cf-bar-btn cf-bar-primary" type="button" disabled={saving} onClick={() => void save()}>{saving ? 'Salvando…' : 'Salvar'}</button>}
        {selectedID && <button className="cf-bar-btn" type="button" onClick={() => setAgentsOpen(true)}>Agentes</button>}
        {selectedID && activeFlow && <button className="cf-bar-btn cf-bar-danger" type="button" disabled={actionID === selectedID} onClick={() => void remove(activeFlow)}>Excluir</button>}
        <button className="cf-bar-btn" type="button" disabled={loading} onClick={() => void load()} title="Atualizar">↻</button>
      </div>

      <div className={`cf-palette${paletteOpen ? '' : ' is-closed'}`}>
        <button className="cf-palette-toggle" type="button" onClick={() => setPaletteOpen((value) => !value)}>
          {paletteOpen ? '⌄ Blocos' : '⌃ Blocos'}
        </button>
        {paletteOpen && (
          <div className="cf-palette-items">
            {chatflowTodos.map((tipo) => {
              const blocked = tipo === 'start' && startExists
              return (
                <button
                  key={tipo}
                  type="button"
                  className={`cf-palette-item cf-node--${tipo}${blocked ? ' is-disabled' : ''}`}
                  draggable={!blocked}
                  disabled={blocked}
                  title={blocked ? 'O fluxo já tem um bloco de início.' : chatflowStepDescriptions[tipo]}
                  onDragStart={(event) => {
                    if (blocked) {
                      event.preventDefault()
                      return
                    }
                    event.dataTransfer.setData('application/cf-node', tipo)
                    event.dataTransfer.effectAllowed = 'move'
                  }}
                  onClick={() => addNodeAt(tipo)}
                >
                  {chatflowStepLabels[tipo]}
                </button>
              )
            })}
          </div>
        )}
      </div>

      {selectedNode && (
        <aside className="cf-inspector">
          <NodeConfigPanel
            node={selectedNode}
            onPatch={(patch) => updateNodeConfig(selectedNode.id, patch)}
            onAddBranch={() => addIntentBranch(selectedNode.id)}
            onRemoveBranch={(index) => removeIntentBranch(selectedNode.id, index)}
            onAddOption={() => addMenuOption(selectedNode.id)}
            onRemoveOption={(index) => removeMenuOption(selectedNode.id, index)}
            onRemove={() => removeNode(selectedNode.id)}
            onClose={() => setSelectedNodeID(null)}
          />
        </aside>
      )}

      {agentsOpen && (
        <div className="cf-overlay" onClick={() => setAgentsOpen(false)}>
          <div className="cf-overlay-panel" onClick={(event) => event.stopPropagation()}>
            <header>
              <div><span className="eyebrow">Vínculo por canal</span><strong>Onde este fluxo rodará</strong></div>
              <button type="button" onClick={() => setAgentsOpen(false)} aria-label="Fechar">×</button>
            </header>
            <p className="cf-overlay-hint">O fluxo só substitui o atendimento livre quando estiver ativo e vinculado a um canal.</p>
            <p className="cf-overlay-hint">
              Rodando nos canais:{' '}
              {selectedID
                ? (data?.channels ?? []).filter((channel) => channel.chatflow_id === selectedID).map((channel) => channel.name).join(', ') || 'nenhum ainda'
                : 'salve o fluxo para vincular'}
            </p>
            <div className="cf-agents-list">
              {(data?.channels ?? []).map((channel) => (
                <div className="cf-agent-row" key={channel.id}>
                  <div className="cf-agent-info"><strong>{channel.name}</strong><span>{channel.active ? 'ativo' : 'pausado'} · {channel.chatflow_id ? `fluxo #${channel.chatflow_id}` : 'sem fluxo'}</span></div>
                  <select
                    value={channel.chatflow_id ?? ''}
                    disabled={actionID === -channel.id}
                    onChange={(event) => {
                      const value = event.target.value
                      void attach(channel, value ? Number(value) : null)
                    }}
                  >
                    <option value="">— Atendimento livre —</option>
                    {(data?.items ?? []).map((flow) => <option key={flow.id} value={flow.id}>{flow.nome}{flow.ativo ? '' : ' · inativo'}</option>)}
                  </select>
                  {onOpenChannels && <button type="button" className="entity-chip-link" onClick={onOpenChannels}>Gerenciar canais</button>}
                </div>
              ))}
              {!data?.channels.length && <p className="cf-agents-empty">Crie um canal para vincular fluxos.</p>}
            </div>
          </div>
        </div>
      )}

      {loading && <div className="cf-loading"><span className="cf-spinner" /> Carregando…</div>}
    </div>
  )
}

function autoDropPosition(nodes: ChatflowNode[]): XYPosition {
  if (!nodes.length) return { x: 80, y: 80 }
  const maxY = Math.max(...nodes.map((node) => node.position.y))
  return { x: 80, y: maxY + 260 }
}

function validateFlow(nodes: ChatflowNode[]): string | null {
  const ords = new Set(nodes.map((node) => node.data.ord))
  for (const node of nodes) {
    const { tipo, config, ord } = node.data
    if (tipo === 'goto') {
      const target = Number(config.target)
      if (!Number.isFinite(target) || target <= 0 || !ords.has(target)) {
        return `O bloco #${ord} (Ir para bloco) aponta para um bloco inexistente.`
      }
    }
    if (tipo === 'condition') {
      const then = Number(config.then) || 0
      const otherwise = Number(config.else) || 0
      if (then <= 0 && otherwise <= 0) {
        return `O bloco #${ord} (Condição) precisa de ao menos um ramo ligado.`
      }
    }
    if (tipo === 'menu') {
      const options = Array.isArray(config.options) ? config.options : []
      if (!options.some((option: any) => Number(option?.next) > 0)) {
        return `O bloco #${ord} (Menu) precisa de ao menos uma opção com destino.`
      }
    }
    if (tipo === 'intent') {
      const branches = Array.isArray(config.branches) ? config.branches : []
      if (!branches.some((branch: any) => Number(branch?.next) > 0)) {
        return `O bloco #${ord} (Intenção) precisa de ao menos um ramo com destino.`
      }
    }
  }
  return null
}

type NodeConfigPanelProps = {
  node: ChatflowNode
  onPatch: (patch: Record<string, any>) => void
  onAddBranch: () => void
  onRemoveBranch: (index: number) => void
  onAddOption: () => void
  onRemoveOption: (index: number) => void
  onRemove: () => void
  onClose: () => void
}

function NodeConfigPanel({ node, onPatch, onAddBranch, onRemoveBranch, onAddOption, onRemoveOption, onRemove, onClose }: NodeConfigPanelProps) {
  const { tipo, config } = node.data as ChatflowNodeData
  const set = (key: string, value: any) => onPatch({ [key]: value })

  return (
    <div className="cf-inspector-panel">
      <header className="cf-inspector-header">
        <span className="cf-inspector-type">{chatflowStepLabels[tipo] ?? tipo} <small>#{node.data.ord}</small></span>
        <div className="cf-inspector-header-actions">
          {tipo !== 'start' && <button type="button" className="cf-inspector-remove" onClick={onRemove}>Remover</button>}
          <button type="button" className="cf-icon-btn" onClick={onClose} aria-label="Fechar painel">×</button>
        </div>
      </header>
      <div className="cf-inspector-body">
        {tipo === 'start' && (
          <p className="cf-inspector-note">Ponto de entrada do fluxo. Ligue a saída “Próximo” ao primeiro bloco do atendimento. Este bloco não pode ser removido.</p>
        )}
        {tipo === 'message' && <MessageFields config={config} set={set} />}
        {tipo === 'collect' && (
          <>
            <label><span>Pergunta ao cliente</span><textarea value={config.prompt ?? ''} onChange={(event) => set('prompt', event.target.value)} rows={2} /></label>
            <label><span>Nome da variável</span><input value={config.variable ?? ''} onChange={(event) => set('variable', event.target.value)} placeholder="ex.: nome, documento" /></label>
          </>
        )}
        {tipo === 'condition' && (
          <>
            <label><span>Campo (text = mensagem, senão variável)</span><input value={config.field ?? 'text'} onChange={(event) => set('field', event.target.value)} /></label>
            <label><span>Operador</span><select value={config.operator ?? 'contains'} onChange={(event) => set('operator', event.target.value)}><option value="eq">igual</option><option value="contains">contém</option></select></label>
            <label><span>Valor de comparação</span><input value={config.value ?? ''} onChange={(event) => set('value', event.target.value)} /></label>
          </>
        )}
        {tipo === 'intent' && (
          <>
            <label><span>Instrução da roteadora (opcional)</span><textarea value={config.prompt ?? ''} onChange={(event) => set('prompt', event.target.value)} rows={2} /></label>
            <div className="cf-list-editor">
              <span className="cf-label">Ramos possíveis (rótulo)</span>
              {(config.branches ?? []).map((branch: any, index: number) => (
                <div className="cf-list-row" key={index}>
                  <input
                    value={branch.label ?? ''}
                    onChange={(event) => {
                      const next = (config.branches ?? []).slice()
                      next[index] = { ...next[index], label: event.target.value }
                      set('branches', next)
                    }}
                    placeholder="Ex.: Financeiro"
                  />
                  <button type="button" onClick={() => onRemoveBranch(index)} aria-label="Remover ramo">×</button>
                </div>
              ))}
              <button className="secondary-button" type="button" onClick={onAddBranch}>+ Ramo</button>
            </div>
          </>
        )}
        {tipo === 'tool' && (
          <>
            <label><span>Nome da tool (integração do agente)</span><input value={config.name ?? ''} onChange={(event) => set('name', event.target.value)} /></label>
            <label><span>Salvar resultado na variável (opcional)</span><input value={config.to_var ?? ''} onChange={(event) => set('to_var', event.target.value)} placeholder="ex.: resposta_api" /></label>
            <InputsEditor value={config.inputs ?? {}} onChange={(inputs) => set('inputs', inputs)} />
          </>
        )}
        {tipo === 'menu' && (
          <>
            <label><span>Tipo do menu</span>
              <select value={config.type ?? ''} onChange={(event) => set('type', event.target.value)}>
                <option value="">Texto numerado (qualquer canal)</option>
                <option value="buttons">Botões (se o canal suportar)</option>
                <option value="list">Lista (se o canal suportar)</option>
              </select>
            </label>
            <label><span>Texto/introdução do menu</span><textarea value={config.text ?? ''} onChange={(event) => set('text', event.target.value)} rows={2} /></label>
            {config.type === 'list' && (
              <>
                <label><span>Título do botão da lista (opcional)</span><input value={config.title ?? ''} onChange={(event) => set('title', event.target.value)} /></label>
                <label><span>Rodapé (opcional)</span><input value={config.footer ?? ''} onChange={(event) => set('footer', event.target.value)} /></label>
              </>
            )}
            <div className="cf-list-editor">
              <span className="cf-label">Opções (cada uma é uma saída do bloco)</span>
              {(config.options ?? []).map((option: any, index: number) => (
                <div className="cf-list-row" key={index}>
                  <input
                    value={option.label ?? ''}
                    onChange={(event) => {
                      const next = (config.options ?? []).slice()
                      next[index] = { ...next[index], label: event.target.value }
                      set('options', next)
                    }}
                    placeholder={`Opção ${index + 1}`}
                  />
                  <input
                    value={option.value ?? ''}
                    onChange={(event) => {
                      const next = (config.options ?? []).slice()
                      next[index] = { ...next[index], value: event.target.value }
                      set('options', next)
                    }}
                    placeholder="Valor aceito (opcional)"
                  />
                  <button type="button" onClick={() => onRemoveOption(index)} aria-label="Remover opção">×</button>
                </div>
              ))}
              <button className="secondary-button" type="button" onClick={onAddOption}>+ Opção</button>
            </div>
            <p className="cf-inspector-note">O cliente pode responder pelo número ou pelo texto da opção. A saída “Padrão” é usada quando nada casa.</p>
          </>
        )}
        {tipo === 'delay' && (
          <>
            <label><span>Segundos</span><input type="number" min={0} value={config.seconds ?? 0} onChange={(event) => set('seconds', Number(event.target.value))} /></label>
            <label><span>Minutos</span><input type="number" min={0} value={config.minutes ?? 0} onChange={(event) => set('minutes', Number(event.target.value))} /></label>
            <p className="cf-inspector-note">A espera é limitada (máx. 30s) para não travar o atendimento; acima disso o fluxo segue.</p>
          </>
        )}
        {tipo === 'set_variable' && (
          <>
            <label><span>Nome da variável</span><input value={config.variable ?? ''} onChange={(event) => set('variable', event.target.value)} placeholder="ex.: nome, pedido" /></label>
            <label><span>Operação</span>
              <select value={config.operation ?? 'set'} onChange={(event) => set('operation', event.target.value)}>
                <option value="set">Definir</option>
                <option value="append">Acrescentar ao final</option>
                <option value="prepend">Acrescentar no início</option>
                <option value="add">Somar número</option>
              </select>
            </label>
            <label><span>Valor (aceita {"{{variavel}}"})</span><textarea value={config.value ?? ''} onChange={(event) => set('value', event.target.value)} rows={2} /></label>
          </>
        )}
        {tipo === 'goto' && (
          <>
            <label><span>Número do bloco de destino (#)</span><input type="number" min={1} value={config.target ?? 0} onChange={(event) => set('target', Number(event.target.value))} /></label>
            <p className="cf-inspector-note">Salta direto para o bloco informado. Há limite de saltos para impedir loop infinito.</p>
          </>
        )}
        {tipo === 'business_hours' && (
          <>
            <label><span>Setor (usa o horário cadastrado do setor)</span><input value={config.setor ?? ''} onChange={(event) => set('setor', event.target.value)} placeholder="ex.: suporte" /></label>
            <p className="cf-inspector-note">Sem setor, usa os horários abaixo. Tudo no fuso de Brasília.</p>
            <label><span>Abertura (HH:MM)</span><input value={config.abertura ?? ''} onChange={(event) => set('abertura', event.target.value)} placeholder="09:00" /></label>
            <label><span>Fechamento (HH:MM)</span><input value={config.fechamento ?? ''} onChange={(event) => set('fechamento', event.target.value)} placeholder="18:00" /></label>
            <label><span>Dias da semana (0=Dom … 6=Sáb)</span><input value={config.dias ?? ''} onChange={(event) => set('dias', event.target.value)} placeholder="1,2,3,4,5" /></label>
          </>
        )}
        {tipo === 'tag' && (
          <>
            <label><span>Tags (separadas por vírgula, aceita {"{{variavel}}"})</span><input value={config.tags ?? ''} onChange={(event) => set('tags', event.target.value)} placeholder="ex.: vip, financeiro" /></label>
            <p className="cf-inspector-note">Aplica as tags ao contato identificado pelo telefone da conversa (sem duplicar).</p>
          </>
        )}
        {tipo === 'satisfaction' && (
          <>
            <label><span>Texto da pesquisa (opcional — usa o texto padrão do sistema)</span><textarea value={config.text ?? ''} onChange={(event) => set('text', event.target.value)} rows={2} /></label>
            <p className="cf-inspector-note">Dispara a pesquisa de satisfação (reusa o fluxo existente). Se não houver participantes, segue pela saída “Padrão”.</p>
          </>
        )}
        {tipo === 'note' && (
          <>
            <label><span>Anotação</span><textarea value={config.text ?? ''} onChange={(event) => set('text', event.target.value)} rows={3} /></label>
            <p className="cf-inspector-note">Bloco de comentário: não executa nada no motor, serve só para documentar o fluxo no canvas.</p>
          </>
        )}
        {tipo === 'http' && (
          <>
            <label><span>Método</span>
              <select value={config.method ?? 'GET'} onChange={(event) => set('method', event.target.value)}>
                <option value="GET">GET</option>
                <option value="POST">POST</option>
                <option value="PUT">PUT</option>
                <option value="PATCH">PATCH</option>
                <option value="DELETE">DELETE</option>
              </select>
            </label>
            <label><span>URL (aceita {"{{variavel}}"})</span><input value={config.url ?? ''} onChange={(event) => set('url', event.target.value)} placeholder="https://api.exemplo.com/recurso" /></label>
            <InputsEditor value={config.headers ?? {}} onChange={(headers) => set('headers', headers)} />
            <InputsEditor value={config.inputs ?? {}} onChange={(inputs) => set('inputs', inputs)} />
            <label><span>Salvar resultado na variável (opcional)</span><input value={config.to_var ?? ''} onChange={(event) => set('to_var', event.target.value)} placeholder="ex.: resposta_api" /></label>
            <p className="cf-inspector-note">Usa o mesmo executor/política das tools (timeout, limite de resposta e bloqueio de rede privada). GET/DELETE viram query; os demais viram corpo JSON.</p>
          </>
        )}
        {tipo === 'queue' && (
          <>
            <label><span>Setor de destino</span><input value={config.setor ?? ''} onChange={(event) => set('setor', event.target.value)} placeholder="ex.: suporte" /></label>
            <label><span>Mensagem ao cliente (opcional)</span><textarea value={config.text ?? ''} onChange={(event) => set('text', event.target.value)} rows={2} /></label>
            <p className="cf-inspector-note">A conversa entra na fila do setor e fica aguardando um atendente humano.</p>
          </>
        )}
        {tipo === 'end' && (
          <label><span>Mensagem de despedida (opcional)</span><textarea value={config.text ?? ''} onChange={(event) => set('text', event.target.value)} rows={2} /></label>
        )}
      </div>
      <footer className="cf-inspector-footer">
        <span>Ligue os conectores do bloco para definir o próximo passo (ou os ramos de condição/intenção).</span>
      </footer>
    </div>
  )
}

function MessageFields({ config, set }: { config: Record<string, any>; set: (key: string, value: any) => void }) {
  return (
    <>
      <label>
        <span>Tipo da mensagem rica</span>
        <select value={config.type ?? ''} onChange={(event) => set('type', event.target.value)}>
          <option value="">Texto simples</option>
          <option value="buttons">Botões</option>
          <option value="list">Lista</option>
          <option value="poll">Enquete</option>
        </select>
      </label>
      <label><span>Texto da mensagem</span><textarea value={config.text ?? ''} onChange={(event) => set('text', event.target.value)} rows={2} /></label>
      {config.type === 'buttons' && <OptionsEditor value={config.options ?? []} onChange={(options) => set('options', options)} placeholder="Opção" />}
      {config.type === 'poll' && <OptionsEditor value={config.options ?? []} onChange={(options) => set('options', options)} placeholder="Alternativa" />}
      {config.type === 'list' && (
        <>
          <label><span>Título do botão (opcional)</span><input value={config.title ?? ''} onChange={(event) => set('title', event.target.value)} /></label>
          <label><span>Rodapé (opcional)</span><input value={config.footer ?? ''} onChange={(event) => set('footer', event.target.value)} /></label>
          <RowsEditor value={config.rows ?? []} onChange={(rows) => set('rows', rows)} />
        </>
      )}
    </>
  )
}

function OptionsEditor({ value, onChange, placeholder }: { value: string[]; onChange: (value: string[]) => void; placeholder?: string }) {
  const remove = (index: number) => onChange(value.filter((_, i) => i !== index))
  return (
    <div className="cf-list-editor">
      <span className="cf-label">Opções</span>
      {value.map((option, index) => (
        <div className="cf-list-row" key={index}>
          <input value={option} onChange={(event) => {
            const next = value.slice()
            next[index] = event.target.value
            onChange(next)
          }} placeholder={placeholder ?? `Opção ${index + 1}`} />
          <button type="button" onClick={() => remove(index)} aria-label="Remover opção">×</button>
        </div>
      ))}
      <button className="secondary-button" type="button" onClick={() => onChange([...value, ''])}>+ Adicionar</button>
    </div>
  )
}

function RowsEditor({ value, onChange }: { value: Array<{ title?: string; desc?: string; id?: string }>; onChange: (value: Array<{ title?: string; desc?: string; id?: string }>) => void }) {
  const setRow = (index: number, key: string, fieldValue: string) => {
    const next = value.slice()
    next[index] = { ...next[index], [key]: fieldValue }
    onChange(next)
  }
  return (
    <div className="cf-list-editor">
      <span className="cf-label">Itens da lista</span>
      {value.map((row, index) => (
        <div className="cf-row-card" key={index}>
          <button type="button" className="cf-row-remove" onClick={() => onChange(value.filter((_, i) => i !== index))} aria-label="Remover item">×</button>
          <input value={row.title ?? ''} onChange={(event) => setRow(index, 'title', event.target.value)} placeholder="Título" />
          <input value={row.desc ?? ''} onChange={(event) => setRow(index, 'desc', event.target.value)} placeholder="Descrição (opcional)" />
        </div>
      ))}
      <button className="secondary-button" type="button" onClick={() => onChange([...value, { title: '', desc: '' }])}>+ Item</button>
    </div>
  )
}

function InputsEditor({ value, onChange }: { value: Record<string, string>; onChange: (value: Record<string, string>) => void }) {
  const entries = Object.entries(value)
  const setRow = (index: number, key: string, fieldValue: string) => {
    const next = entries.slice()
    next[index] = [key, fieldValue]
    onChange(Object.fromEntries(next))
  }
  return (
    <div className="cf-list-editor">
      <span className="cf-label">Entradas (chave → valor, usa variáveis com {"{{chave}}"})</span>
      {entries.map(([key, fieldValue], index) => (
        <div className="cf-list-row" key={index}>
          <input value={key} onChange={(event) => setRow(index, event.target.value, fieldValue)} placeholder="chave" />
          <input value={fieldValue} onChange={(event) => setRow(index, key, event.target.value)} placeholder="valor" />
          <button type="button" onClick={() => onChange(Object.fromEntries(entries.filter((_, i) => i !== index)))} aria-label="Remover entrada">×</button>
        </div>
      ))}
      <button className="secondary-button" type="button" onClick={() => onChange({ ...value, '': '' })}>+ Entrada</button>
    </div>
  )
}
