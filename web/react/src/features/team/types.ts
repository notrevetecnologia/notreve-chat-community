import type { PaginationMeta } from '../../shared/Pagination'

export type Sector = {
  id: number
  name: string
  description: string
  horario_abertura: string
  horario_fechamento: string
  dias_semana: string
  cor: string
  active: boolean
  member_count: number
  online_count: number
  created_at: string
}

export type TeamMember = {
  id: number
  name: string
  username?: string
  role?: string
  active: boolean
  online: boolean
  atendente_id?: number
  setor_ids: number[]
  habilidades?: string[]
  capacidade?: number
  permissions: string[]
  permissions_customized?: boolean
  created_at: string
}

export type TeamSectorOption = {
  id: number
  name: string
  active: boolean
}

export type TeamRoleOption = {
  key: string
  label: string
}

export type TeamPermission = {
  key: string
  label: string
}

export type TeamPermissionGroup = {
  module: string
  label: string
  permissions: TeamPermission[]
}

export type TeamUsersData = {
  items: TeamMember[]
  sectors: TeamSectorOption[]
  roles: TeamRoleOption[]
  permission_groups: TeamPermissionGroup[]
  meta?: PaginationMeta
}

export type UserDraft = {
  name: string
  username: string
  password: string
  role: string
  permissions: string[]
  setor_ids: number[]
  habilidades: string[]
  capacidade: number
  active: boolean
}

export type TeamStats = {
  sectors: number
  active_sectors: number
  members: number
  online_members: number
}

export type TeamData = {
  sectors: Sector[]
  members: TeamMember[]
  stats: TeamStats
}

export type SectorDraft = {
  name: string
  description: string
  horario_abertura: string
  horario_fechamento: string
  dias_semana: string
  cor: string
  active: boolean
}
