export type KnowledgeAgent = {
  id: number
  name: string
  active: boolean
}

export type KnowledgeSource = {
  id: number
  agent_id: number
  kind: 'text' | 'pdf' | 'audio' | 'site' | string
  name: string
  source_url?: string
  mime_type?: string
  content_chars: number
  content_preview?: string
  status: 'processing' | 'ready' | 'error' | string
  error_message?: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type KnowledgeUsage = {
  sources: number
  chars: number
}

export type KnowledgeLimits = {
  max_sources_per_agent: number
  max_source_chars: number
  max_total_chars: number
  max_pdf_bytes: number
  max_audio_bytes: number
}

export type KnowledgeData = {
  agents: KnowledgeAgent[]
  agent_id: number
  sources: KnowledgeSource[]
  usage: KnowledgeUsage
  limits: KnowledgeLimits
  scope: string
}

export type KnowledgeDraft = {
  kind: 'text' | 'pdf' | 'audio' | 'site'
  name: string
  source_url: string
  text: string
  file_name: string
  mime_type: string
  data_base64: string
}
