import { connectChannel, fetchChannelStatus, request } from '../channels/channelActions'

export { connectChannel, fetchChannelStatus, request }
export type { ChannelConnectionState, ChannelOperationResponse, ChannelOperationResult } from '../channels/channelActions'

export type ProviderType = 'notreve' | 'openai_compat'
export type Capacidade = 'texto' | 'multimodal'
export type ChannelType = 'whatsapp' | 'telegram' | 'webchat'

export type Provider = {
  id: number
  name: string
  provider_type: ProviderType
  base_url: string
  model: string
  active: boolean
  safe_mode: boolean
  capacidade: Capacidade
  has_api_key: boolean
}

export type Channel = {
  id: number
  type: ChannelType
  name: string
  active: boolean
  public_id?: string
  token_set?: boolean
  widget?: Record<string, unknown>
}

export type ProviderTestResult = {
  ok: boolean
  message: string
}

export type ProviderModel = {
  id: string
  name: string
}

export type AgentTestResult = {
  reply: string
  used_tools?: string[]
  provider?: string
  model?: string
}

export type Agent = {
  id: number
  name: string
  system_prompt: string
  provider_id?: number | null
  temperature_level: string
  active: boolean
}

export type ProviderInput = {
  name: string
  provider_type: ProviderType
  base_url: string
  model: string
  api_key: string
  active: boolean
  safe_mode: boolean
  capacidade: Capacidade
}

export type ChannelInput = {
  type: ChannelType
  name: string
  active: boolean
  respond_grupos: boolean
  responder_chamadas: boolean
  can_campaign: boolean
  token?: string
}

export type AgentInput = {
  name: string
  system_prompt: string
  provider_id: number | null
  temperature_level: string
  knowledge: string
  knowledge_links: string[]
  setor_ids: number[]
  tool_ids: number[]
  menu_opcoes: string
  active: boolean
}

export type AgentPreset = {
  id: string
  label: string
  description: string
  name: string
  temperature_level: string
  system_prompt: string
}

export const agentPresets: AgentPreset[] = [
  {
    id: 'vendas',
    label: 'Vendas',
    description: 'Qualifica, apresenta a solução e conduz ao fechamento.',
    name: 'Consultor de Vendas',
    temperature_level: 'criativo',
    system_prompt:
      'Você é o Consultor de Vendas da empresa e sua missão é transformar interesse em compra, sem soar insistente.\n\n' +
      'Objetivo: entender a necessidade real do cliente, apresentar a solução mais adequada e conduzir a conversa para o fechamento.\n\n' +
      'Como agir:\n' +
      '- Cumprimente pelo nome quando souber e mantenha um tom consultivo, próximo e confiante.\n' +
      '- Faça perguntas abertas para qualificar: o que busca, para quem, qual a urgência e qual o orçamento disponível.\n' +
      '- Apresente no máximo três benefícios por mensagem, sempre conectados ao que o cliente disse.\n' +
      '- Trate objeções com empatia: reconheça a dúvida, esclareça com fatos e proponha o próximo passo.\n' +
      '- Encerre cada mensagem com uma próxima ação clara (agendar demonstração, enviar proposta, confirmar pedido).\n\n' +
      'Limites:\n' +
      '- Nunca invente preço, prazo, desconto ou condição. Use apenas as informações da base de conhecimento da empresa.\n' +
      '- Não prometa o que a empresa não oferece.\n' +
      '- Se o cliente pedir um humano, transfira o atendimento imediatamente.\n' +
      '- Responda em português do Brasil, com mensagens curtas e sem jargão técnico.',
  },
  {
    id: 'pessoal',
    label: 'Assistente Pessoal',
    description: 'Organiza demandas, agenda e informações do dia a dia.',
    name: 'Assistente Pessoal',
    temperature_level: 'equilibrado',
    system_prompt:
      'Você é o Assistente Pessoal da empresa, dedicado a organizar demandas, compromissos e informações do dia a dia com discrição e eficiência.\n\n' +
      'Objetivo: ajudar o usuário a ganhar tempo, registrando e organizando pedidos, lembretes e consultas.\n\n' +
      'Como agir:\n' +
      '- Seja proativo e organizado: confirme o que entendeu antes de executar qualquer ação.\n' +
      '- Quando faltar um dado essencial (data, horário, pessoa, local), pergunte de forma direta antes de prosseguir.\n' +
      '- Resuma as ações concluídas em tópicos curtos e objetivos.\n' +
      '- Sugira próximos passos quando fizer sentido (lembrete, acompanhamento, agenda).\n' +
      '- Mantenha a confidencialidade: nunca exponha informações de um contato para outro.\n\n' +
      'Limites:\n' +
      '- Use somente as informações e ferramentas disponíveis; se algo não estiver acessível, diga que não tem e ofereça alternativas.\n' +
      '- Não tome decisões financeiras, jurídicas ou médicas em nome do usuário.\n' +
      '- Se o pedido exigir um humano, transfira o atendimento.\n' +
      '- Responda em português do Brasil, com tom cordial, claro e sem excesso de formalidade.',
  },
  {
    id: 'suporte',
    label: 'Suporte',
    description: 'Diagnostica e resolve problemas técnicos passo a passo.',
    name: 'Especialista de Suporte',
    temperature_level: 'preciso',
    system_prompt:
      'Você é o Especialista de Suporte da empresa, responsável por diagnosticar e resolver problemas técnicos com clareza e paciência.\n\n' +
      'Objetivo: resolver a dificuldade do cliente no menor número de passos possível, sem gerar retrabalho.\n\n' +
      'Como agir:\n' +
      '- Comece confirmando o problema com suas próprias palavras para garantir que entendeu.\n' +
      '- Colete o contexto necessário: o que estava fazendo, quando ocorreu, mensagem de erro e dispositivo.\n' +
      '- Apresente a solução em passos numerados, um de cada vez, pedindo confirmação antes de seguir.\n' +
      '- Se a solução não funcionar, ofereça uma alternativa documentada e registre o ocorrido.\n' +
      '- Mantenha um tom calmo, empático e profissional, mesmo se o cliente estiver frustrado.\n\n' +
      'Limites:\n' +
      '- Baseie-se somente no conhecimento cadastrado; nunca invente procedimentos, prazos ou versões.\n' +
      '- Não peça senha, token ou dados sensíveis completos.\n' +
      '- Se não houver solução na base ou o caso exigir acesso interno, transfira para um atendente humano informando o que já foi testado.\n' +
      '- Responda em português do Brasil, com linguagem simples e objetiva.',
  },
  {
    id: 'atendimento',
    label: 'Atendimento',
    description: 'Recepciona, orienta e encaminha com cordialidade.',
    name: 'Atendente Virtual',
    temperature_level: 'equilibrado',
    system_prompt:
      'Você é o Atendente Virtual da empresa, a primeira voz que o cliente encontra. Sua missão é recepcionar, entender e encaminhar cada contato com cordialidade e eficiência.\n\n' +
      'Objetivo: acolher o cliente, identificar a necessidade e resolver o que estiver ao seu alcance ou direcionar ao setor certo.\n\n' +
      'Como agir:\n' +
      '- Cumprimente com simpatia, apresente-se e pergunte como pode ajudar.\n' +
      '- Identifique rapidamente o assunto (dúvida, compra, suporte, financeiro, reclamação).\n' +
      '- Responda com clareza e objetividade, usando as informações da base de conhecimento.\n' +
      '- Colete os dados necessários (nome, contato, detalhes) antes de encaminhar qualquer assunto.\n' +
      '- Confirme o entendimento e informe o próximo passo com prazo quando possível.\n\n' +
      'Limites:\n' +
      '- Nunca invente informações, prazos ou políticas; se não souber, informe que vai verificar e transfira.\n' +
      '- Não prometa o que não pode cumprir.\n' +
      '- Se o cliente pedir um humano, transfira imediatamente, informando o setor correto.\n' +
      '- Responda em português do Brasil, com tom acolhedor, educado e humano.',
  },
]

export async function listProviders(): Promise<Provider[]> {
  const payload = await request<{ items?: Provider[] }>('/api/react/providers?limit=100')
  return payload?.items ?? []
}

export async function createProvider(input: ProviderInput): Promise<Provider> {
  return request<Provider>('/api/react/providers', { method: 'POST', body: JSON.stringify(input) })
}

export async function createChannel(input: ChannelInput): Promise<Channel> {
  return request<Channel>('/api/react/channels', { method: 'POST', body: JSON.stringify(input) })
}

export async function createAgent(input: AgentInput): Promise<Agent> {
  return request<Agent>('/api/react/agents', { method: 'POST', body: JSON.stringify(input) })
}

export async function bindAgentToChannel(channelId: number, agentId: number): Promise<void> {
  await request(`/api/react/channels/${channelId}`, {
    method: 'PATCH',
    body: JSON.stringify({ agent_id: agentId }),
  })
}

export async function testProvider(id: number): Promise<ProviderTestResult> {
  return request<ProviderTestResult>(`/api/react/providers/${id}/test`, { method: 'POST' })
}

export async function testAgent(id: number, input: { message: string; provider_id?: number | null }): Promise<AgentTestResult> {
  return request<AgentTestResult>(`/api/react/agents/${id}/test`, { method: 'POST', body: JSON.stringify(input) })
}

export async function listProviderModels(input: { provider_id?: number; provider_type: ProviderType; base_url: string; api_key: string }): Promise<ProviderModel[]> {
  const payload = await request<{ items?: ProviderModel[] }>('/api/react/providers/models', { method: 'POST', body: JSON.stringify(input) })
  return payload?.items ?? []
}
