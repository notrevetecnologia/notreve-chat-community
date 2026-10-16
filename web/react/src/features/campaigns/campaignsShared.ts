import type { PaginationMeta } from '../../shared/Pagination'

export type Campaign = {
  id: number
  name: string
  message: string
  grupo_id?: number | null
  grupo_name: string
  channel_id?: number | null
  channel_name: string
  segmento_id?: number | null
  segmento_name: string
  status: string
  scheduled_at?: string | null
  intervalo_segundos: number
  limite_diario: number
  variar_mensagem: boolean
  modo_teste: boolean
  media_type: string
  media_name: string
  has_media: boolean
  total: number
  enviadas: number
  falhas: number
  lidas: number
  created_at: string
  updated_at: string
  started_at?: string | null
  completed_at?: string | null
}

export type Segment = { id: number; name: string; description: string; count?: number; filter_tem_whatsapp?: boolean | null; filter_canal?: string; created_at: string }
export type Channel = { id: number; name: string; type: string; active: boolean }
export type Group = { id: number; name: string; channel_ids: number[]; channel_names: string[] }
export type Options = { segments: Segment[]; groups: Group[]; channels: Channel[] }
export type Delivery = { id: number; phone: string; status: string; erro: string; channel_name: string; sent_at?: string | null; created_at: string }
export type Envelope<T> = { data?: T; meta?: PaginationMeta; error?: { message?: string } }
export type CampaignResponse = { items: Campaign[]; options: Options }
export type DeliveryResponse = { items: Delivery[] }

export type Draft = {
  name: string
  message: string
  target_kind: 'channel' | 'group'
  target_id: number | null
  segmento_id: number | null
  scheduled_at: string
  intervalo_segundos: number
  limite_diario: number
  variar_mensagem: boolean
  modo_teste: boolean
  media_file: File | null
}

export type SegmentDraft = { name: string; description: string; filtro_tem_whatsapp: boolean | null; filtro_canal: string }
export type GroupDraft = { name: string; channel_ids: number[] }

export const emptyDraft = (options: Options): Draft => ({ name: '', message: '', target_kind: 'channel', target_id: options.channels[0]?.id ?? null, segmento_id: null, scheduled_at: '', intervalo_segundos: 15, limite_diario: 0, variar_mensagem: false, modo_teste: false, media_file: null })

export const emptySegmentDraft = (): SegmentDraft => ({ name: '', description: '', filtro_tem_whatsapp: null, filtro_canal: '' })

export async function fileToBase64(file: File): Promise<string> {
  const buffer = await file.arrayBuffer()
  let binary = ''
  const bytes = new Uint8Array(buffer)
  const chunkSize = 0x8000
  for (let index = 0; index < bytes.length; index += chunkSize) binary += String.fromCharCode(...bytes.subarray(index, index + chunkSize))
  return btoa(binary)
}

export async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, { ...options, credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) } })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return payload.data as T
}

export async function requestEnvelope<T>(url: string, options?: RequestInit): Promise<{ data: T; meta: PaginationMeta }> {
  const response = await fetch(url, { ...options, credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(options?.headers ?? {}) } })
  const payload = await response.json() as Envelope<T>
  if (!response.ok) throw new Error(payload.error?.message || 'Não foi possível concluir a operação.')
  return { data: payload.data as T, meta: payload.meta || {} }
}

export function campaignStatus(status: string) {
  const labels: Record<string, string> = { rascunho: 'Rascunho', agendada: 'Agendada', rodando: 'Rodando', pausada: 'Pausada', concluida: 'Concluída', cancelada: 'Cancelada' }
  return labels[status] || status
}

export type CampaignAction = 'iniciar' | 'retomar' | 'pausar' | 'cancelar'

export function campaignActions(status: string): CampaignAction[] {
  switch (status) {
    case 'rascunho': return ['iniciar', 'cancelar']
    case 'agendada': return ['iniciar', 'cancelar']
    case 'rodando': return ['pausar', 'cancelar']
    case 'pausada': return ['retomar', 'cancelar']
    default: return []
  }
}

export function campaignActionLabel(action: CampaignAction, status: string): string {
  if (action === 'iniciar') return status === 'agendada' ? 'Iniciar agora' : 'Iniciar'
  if (action === 'retomar') return 'Retomar'
  if (action === 'pausar') return 'Pausar'
  return 'Cancelar'
}

export function campaignCanDelete(status: string): boolean {
  return status !== 'rodando'
}

export function formatDate(value: string) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.valueOf()) ? '—' : date.toLocaleDateString('pt-BR', { day: '2-digit', month: 'short' }).replace('.', '')
}

export function formatDateTime(value: string) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.valueOf()) ? '—' : date.toLocaleString('pt-BR', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })
}

export function mediaTypeFromMime(mime: string, name: string) {
  const normalized = mime.toLowerCase()
  if (normalized.startsWith('image/')) return 'image'
  if (normalized.startsWith('audio/')) return 'audio'
  if (normalized.startsWith('video/')) return 'video'
  if (normalized === 'application/pdf' || /\.(pdf|docx?|xlsx?)$/i.test(name)) return 'document'
  return 'document'
}
