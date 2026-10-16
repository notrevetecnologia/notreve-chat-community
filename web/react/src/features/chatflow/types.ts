export type ChatflowConfig = Record<string, any>

export type ChatflowStep = {
  id: number
  chatflow_id: number
  ord: number
  tipo: string
  config: ChatflowConfig
  created_at: string
}

export type Chatflow = {
  id: number
  nome: string
  version: number
  ativo: boolean
  config: ChatflowConfig
  updated_at: string
  steps: ChatflowStep[]
}

export type ChatflowChannel = {
  id: number
  name: string
  type: string
  chatflow_id: number | null
  active: boolean
}

export type ChatflowData = {
  items: Chatflow[]
  channels: ChatflowChannel[]
}

export type ChatflowStepInput = {
  ord: number
  tipo: string
  config: ChatflowConfig
}

export type ChatflowDraft = {
  nome: string
  version: number
  ativo: boolean
  config: ChatflowConfig
  steps: ChatflowStepInput[]
}

export const chatflowStepLabels: Record<string, string> = {
  start: 'Início',
  message: 'Mensagem',
  collect: 'Coleta',
  condition: 'Condição',
  intent: 'Intenção (IA)',
  tool: 'Tool',
  queue: 'Setor / Fila',
  end: 'Encerramento',
  menu: 'Menu de opções',
  delay: 'Aguardar',
  set_variable: 'Definir variável',
  goto: 'Ir para bloco',
  business_hours: 'Horário de atendimento',
  tag: 'Aplicar tag',
  satisfaction: 'Pesquisa de satisfação',
  note: 'Anotação',
  http: 'Requisição HTTP',
}

export const chatflowStepDescriptions: Record<string, string> = {
  start: 'Ponto de entrada do fluxo. É por aqui que o atendimento começa.',
  message: 'Envia uma mensagem ao cliente.',
  collect: 'Pergunta algo e salva a resposta do cliente.',
  condition: 'Verifica a mensagem ou uma variável coletada.',
  intent: 'Delega a uma IA a escolha do próximo ramo.',
  tool: 'Chama uma integração (tool) do agente.',
  queue: 'Enfileira a conversa no setor escolhido e aguarda um atendente humano.',
  end: 'Encerra o atendimento.',
  menu: 'Apresenta opções e ramifica conforme a escolha do cliente (botões/lista ou texto numerado).',
  delay: 'Aguarda alguns segundos antes de continuar (limitado para não travar o atendimento).',
  set_variable: 'Define ou atualiza uma variável do fluxo (aceita {{variavel}} e operações).',
  goto: 'Salta para outro bloco do fluxo, com limite de saltos para evitar loop infinito.',
  business_hours: 'Ramifica entre aberto e fechado conforme o horário de Brasília.',
  tag: 'Aplica tag(s) ao contato da conversa.',
  satisfaction: 'Dispara a pesquisa de satisfação e segue (ou desvia se não houver participantes).',
  note: 'Bloco de anotação: não executa nada, só documenta o fluxo no canvas.',
  http: 'Faz uma requisição HTTP e salva o resultado numa variável.',
}
