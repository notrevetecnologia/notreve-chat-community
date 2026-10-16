export type OperationalSettings = {
  modo_aberto: boolean
  ia_monitora: boolean
  distribuicao_automatica: boolean
  followup_ativo: boolean
  followup_horas: number
  ia_na_fila: boolean
  detectar_bots: boolean
  bot_patterns: string
  registrar_falhas_aprendizado: boolean
  transferir_falhas_humano: boolean
  knowledge_top_k: number
  image_model: string
  image_aspect_ratio: string
  logs_retencao_dias: number
}

export type SettingsData = {
  operational: OperationalSettings
  scope: string
  can_edit: boolean
}
