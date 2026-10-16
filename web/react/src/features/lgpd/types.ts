export type LGPDConsent = {
  id: number
  contato_id: number | null
  conversation_id: number | null
  scope: string
  base_legal: string
  aceito: boolean
  canal: string
  token: string
  ip: string
  data: string
}

export type LGPDPolitica = {
  id: number
  version: number
  tipo: string
  texto: string
  url: string
  vigencia_inicio: string
  vigencia_fim: string | null
  active: boolean
  created_at: string
}

export type AuditLog = {
  id: number
  usuario_id: number | null
  usuario: string
  acao: string
  recurso: string
  entidade_id: number | null
  ip: string
  data: string
  metadados?: Record<string, unknown>
}

export type LGPDSummary = {
  consents: number
  audit: number
  has_politica: boolean
  encrypted: boolean
  politica: LGPDPolitica | null
}

export type ConsentInput = {
  contato_id?: number | null
  conversation_id?: number | null
  scope?: string
  base_legal?: string
  aceito?: boolean
  canal?: string
  token?: string
}

export type PoliticaInput = {
  tipo: string
  texto: string
  url: string
}
