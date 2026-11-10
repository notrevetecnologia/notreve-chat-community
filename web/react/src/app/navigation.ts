export type NavigationItem = {
  id: string
  label: string
  group: 'atendimento' | 'relacionamento' | 'inteligencia' | 'canais' | 'automacao' | 'operacao' | 'administracao' | 'plataforma'
  href?: string
  module?: string
  requiredAnyModule?: string[]
  requiredPermission?: string
  requiredAnyPermission?: string[]
}

export const navigationItems: NavigationItem[] = [
  { id: 'conversas', label: 'Visão geral', group: 'atendimento', module: 'conversas', requiredPermission: 'conversas_ver' },
  { id: 'inbox', label: 'Caixa de entrada', group: 'atendimento', module: 'conversas', requiredPermission: 'conversas_ver' },
  { id: 'relatorios', label: 'Relatórios', group: 'atendimento', module: 'relatorios', requiredPermission: 'relatorios_ver' },
  { id: 'wizard', label: 'Assistente de configuração', group: 'atendimento', module: 'config', requiredPermission: 'config_gerenciar' },
  { id: 'contatos', label: 'Contatos', group: 'relacionamento', module: 'contatos', requiredPermission: 'contatos_gerenciar' },
  { id: 'campanhas', label: 'Campanhas e segmentos', group: 'relacionamento', module: 'campanhas', requiredPermission: 'campanhas_ver' },
  { id: 'agentes', label: 'Agentes', group: 'inteligencia', module: 'ia', requiredPermission: 'agentes_gerenciar' },
  { id: 'conhecimento', label: 'Base de conhecimento', group: 'inteligencia', module: 'ia', requiredPermission: 'agentes_gerenciar' },
  { id: 'ias', label: 'Provedores de IA', group: 'inteligencia', module: 'ia', requiredPermission: 'ias_gerenciar' },
  { id: 'aprendizado', label: 'Aprendizado', group: 'inteligencia', module: 'ia', requiredPermission: 'aprendizado_ver' },
  { id: 'skills', label: 'Skills', group: 'inteligencia', module: 'ia', requiredPermission: 'agentes_gerenciar' },
  { id: 'chatflows', label: 'Chat Flow', group: 'inteligencia', module: 'chatflows', requiredPermission: 'chatflows_gerenciar' },
  { id: 'canais', label: 'Canais', group: 'canais', module: 'canais', requiredPermission: 'canais_gerenciar' },
  { id: 'integracoes', label: 'Integrações e API', group: 'automacao', module: 'tools', requiredAnyModule: ['tools', 'api'], requiredPermission: 'integracoes_gerenciar' },
  { id: 'equipe', label: 'Equipe e setores', group: 'operacao', module: 'equipe', requiredAnyPermission: ['setores_gerenciar', 'equipe_gerenciar'] },
  { id: 'agendamentos', label: 'Agendamentos', group: 'operacao', module: 'agendamentos', requiredPermission: 'agendamentos_gerenciar' },
  { id: 'biblioteca', label: 'Biblioteca de mensagens', group: 'operacao', module: 'mensagens', requiredAnyPermission: ['respostas_gerenciar', 'mensagens_gerenciar'] },
  { id: 'empresas', label: 'Empresas', group: 'plataforma', module: 'config', requiredPermission: 'plataforma_gerenciar' },
  { id: 'primeiros-passos', label: 'Primeiros passos', group: 'plataforma', module: 'config', requiredPermission: 'plataforma_gerenciar' },
  { id: 'planos', label: 'Planos e limites', group: 'plataforma', module: 'config', requiredPermission: 'plataforma_gerenciar' },
  { id: 'servidores', label: 'Infraestrutura WhatsApp', group: 'plataforma', module: 'config', requiredPermission: 'plataforma_gerenciar' },
  { id: 'marca', label: 'Marca da plataforma', group: 'plataforma', module: 'config', requiredPermission: 'plataforma_gerenciar' },
  { id: 'configuracoes', label: 'Configurações do atendimento', group: 'administracao', module: 'config', requiredPermission: 'config_gerenciar' },
  { id: 'logs', label: 'Logs', group: 'administracao', module: 'config', requiredPermission: 'logs_ver' },
  { id: 'lgpd', label: 'LGPD e privacidade', group: 'administracao', module: 'lgpd', requiredPermission: 'config_gerenciar' },
]

export const navigationGroups: Record<NavigationItem['group'], string> = {
  atendimento: 'Atendimento',
  relacionamento: 'Relacionamento',
  inteligencia: 'Inteligência',
  canais: 'Canais',
  automacao: 'Automação',
  operacao: 'Operação',
  administracao: 'Administração',
  plataforma: 'Plataforma',
}

export const getNavigationItem = (id: string) =>
  navigationItems.find((item) => item.id === id) ?? navigationItems[0]
