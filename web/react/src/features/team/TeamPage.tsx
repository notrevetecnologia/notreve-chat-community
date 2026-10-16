import { useCallback, useEffect, useMemo, useState } from 'react'
import { createSector, createUser, deleteSector, deleteUser, loadTeam, loadUsers, toggleSector, toggleUser, updateSector, updateUser } from './teamApi'
import type { Sector, SectorDraft, TeamMember, TeamUsersData, UserDraft } from './types'
import { Modal } from '../../shared/Modal'
import { FilterBar, FilterSearch, FilterSelect, Pagination, DEFAULT_PAGE_SIZE } from '../../shared/Pagination'
import './Team.css'

type TeamPageProps = {
  onBack?: () => void
  permissions?: string[] | null
  currentUserId?: number | null
}

const dayOptions = [
  { value: '0', label: 'Dom' },
  { value: '1', label: 'Seg' },
  { value: '2', label: 'Ter' },
  { value: '3', label: 'Qua' },
  { value: '4', label: 'Qui' },
  { value: '5', label: 'Sex' },
  { value: '6', label: 'Sáb' },
]

const emptyDraft = (): SectorDraft => ({
  name: '',
  description: '',
  horario_abertura: '00:00',
  horario_fechamento: '23:59',
  dias_semana: '0,1,2,3,4,5,6',
  cor: '#0ea5e9',
  active: true,
})

const emptyUserDraft = (): UserDraft => ({
  name: '', username: '', password: '', role: 'atendente', permissions: [], setor_ids: [], habilidades: [], capacidade: 0, active: true,
})

const roleLabel = (role?: string) => ({
  admin: 'Administrador',
  supervisor: 'Supervisor',
  atendente: 'Atendente',
  super_admin: 'Super admin',
}[role || ''] || 'Operador')

const initials = (name: string) => name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]?.toUpperCase() || '').join('') || 'E'

const selectedDays = (value: string) => value.split(',').map((item) => item.trim()).filter(Boolean)

export function TeamPage({ onBack, permissions = null, currentUserId = null }: TeamPageProps) {
  const [sectors, setSectors] = useState<Sector[]>([])
  const [members, setMembers] = useState<TeamMember[]>([])
  const [stats, setStats] = useState({ sectors: 0, active_sectors: 0, members: 0, online_members: 0 })
  const [userOptions, setUserOptions] = useState<TeamUsersData | null>(null)
  const [view, setView] = useState<'sectors' | 'members'>('sectors')
  const [draft, setDraft] = useState<SectorDraft | null>(null)
  const [userDraft, setUserDraft] = useState<UserDraft | null>(null)
  const [editingID, setEditingID] = useState<number | null>(null)
  const [editingUserID, setEditingUserID] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [userSaving, setUserSaving] = useState(false)
  const [actionID, setActionID] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [editorError, setEditorError] = useState<string | null>(null)
  const [userEditorError, setUserEditorError] = useState<string | null>(null)
  const [memberQuery, setMemberQuery] = useState('')
  const [memberRole, setMemberRole] = useState('')
  const [memberStatus, setMemberStatus] = useState('')
  const [memberOffset, setMemberOffset] = useState(0)
  const [memberTotal, setMemberTotal] = useState(0)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await loadTeam()
      setSectors(data.sectors || [])
      setMembers(data.members || [])
      setStats(data.stats || { sectors: 0, active_sectors: 0, members: 0, online_members: 0 })
      try {
        const users = await loadUsers({ limit: DEFAULT_PAGE_SIZE, offset: memberOffset, q: memberQuery.trim() || undefined, role: memberRole || undefined, status: memberStatus || undefined })
        setMembers(users.items || [])
        setUserOptions(users)
        setMemberTotal(users.meta?.total ?? users.items?.length ?? 0)
        setStats({ ...(data.stats || { sectors: 0, active_sectors: 0, members: 0, online_members: 0 }), members: users.meta?.total ?? users.items?.length ?? 0, online_members: users.items?.filter((member) => member.online).length || 0 })
      } catch {
        // A supervisor may manage sectors without having the granular team
        // permission. Keep the sector screen usable instead of blanking it.
        setUserOptions(null)
      }
      setError(null)
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar a equipe.')
    } finally {
      setLoading(false)
    }
  }, [memberQuery, memberRole, memberStatus, memberOffset])

  useEffect(() => { void load() }, [load])
  useEffect(() => { setMemberOffset(0) }, [memberQuery, memberRole, memberStatus])

  useEffect(() => {
    if (!draft && !userDraft) return undefined
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || saving || userSaving) return
      if (userDraft) setUserDraft(null)
      else setDraft(null)
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [draft, userDraft, saving, userSaving])

  const updateDraft = <K extends keyof SectorDraft>(key: K, value: SectorDraft[K]) => {
    setDraft((current) => current ? { ...current, [key]: value } : current)
  }

  const openCreate = () => {
    setEditingID(null)
    setDraft(emptyDraft())
    setEditorError(null)
    setError(null)
  }

  const openUserCreate = () => {
    setEditingUserID(null)
    setUserDraft(emptyUserDraft())
    setUserEditorError(null)
    setError(null)
  }

  const openUserEdit = (member: TeamMember) => {
    setEditingUserID(member.id)
    setUserDraft({ name: member.name, username: member.username || '', password: '', role: member.role || 'atendente', permissions: member.permissions || [], setor_ids: member.setor_ids || [], habilidades: member.habilidades || [], capacidade: member.capacidade || 0, active: member.active })
    setUserEditorError(null)
    setError(null)
  }

  const openEdit = (sector: Sector) => {
    setEditingID(sector.id)
    setDraft({
      name: sector.name,
      description: sector.description,
      horario_abertura: sector.horario_abertura || '00:00',
      horario_fechamento: sector.horario_fechamento || '23:59',
      dias_semana: sector.dias_semana || '0,1,2,3,4,5,6',
      cor: sector.cor || '#0ea5e9',
      active: sector.active,
    })
    setEditorError(null)
    setError(null)
  }

  const save = async () => {
    if (!draft || !draft.name.trim()) {
      setEditorError('Informe um nome para o setor.')
      return
    }
    setEditorError(null)
    setSaving(true)
    try {
      const payload = { ...draft, name: draft.name.trim(), description: draft.description.trim() }
      if (editingID === null) await createSector(payload)
      else await updateSector(editingID, payload)
      setDraft(null)
      setEditingID(null)
      await load()
    } catch (saveError) {
      const message = saveError instanceof Error ? saveError.message : 'Não foi possível salvar o setor.'
      setEditorError(message)
    } finally {
      setSaving(false)
    }
  }

  const updateUserDraft = <K extends keyof UserDraft>(key: K, value: UserDraft[K]) => {
    setUserDraft((current) => current ? { ...current, [key]: value } : current)
  }

  const saveUser = async () => {
    if (!userDraft || !userDraft.name.trim() || !userDraft.username.trim()) {
      setUserEditorError('Informe nome e usuário de acesso.')
      return
    }
    if (editingUserID === null && userDraft.password.trim().length < 6) {
      setUserEditorError('A senha precisa ter pelo menos 6 caracteres.')
      return
    }
    setUserEditorError(null)
    setUserSaving(true)
    try {
      const payload = { ...userDraft, name: userDraft.name.trim(), username: userDraft.username.trim(), password: userDraft.password.trim() }
      if (editingUserID === null) await createUser(payload)
      else await updateUser(editingUserID, payload)
      setUserDraft(null)
      setEditingUserID(null)
      await load()
    } catch (saveError) {
      setUserEditorError(saveError instanceof Error ? saveError.message : 'Não foi possível salvar o usuário.')
    } finally {
      setUserSaving(false)
    }
  }

  const runAction = async (id: number, action: () => Promise<unknown>) => {
    setActionID(id)
    try {
      await action()
      await load()
      setError(null)
    } catch (actionError) {
      setError(actionError instanceof Error ? actionError.message : 'Não foi possível concluir a ação.')
    } finally {
      setActionID(null)
    }
  }

  const remove = (sector: Sector) => {
    if (!window.confirm(`Excluir o setor “${sector.name}”? Conversas existentes não serão apagadas.`)) return
    void runAction(sector.id, () => deleteSector(sector.id))
  }

  const removeUser = (member: TeamMember) => {
    if (!window.confirm(`Excluir o acesso de “${member.name}”? Essa ação não pode ser desfeita.`)) return
    void runAction(member.id, () => deleteUser(member.id))
  }

  const sectorNames = useMemo(() => new Map(sectors.map((sector) => [sector.id, sector.name])), [sectors])
  const canManageSectors = permissions == null || permissions.includes('setores_gerenciar')
  const canManageUsers = userOptions !== null && (permissions == null || permissions.includes('equipe_gerenciar'))

  return <div className="team-page">
    <section className="team-intro">
      <div>
        <span className="eyebrow">Operação e pessoas</span>
        <h2>Equipe no ritmo certo.</h2>
        <p>Organize os setores, acompanhe quem está online e mantenha a distribuição do atendimento clara para toda a empresa.</p>
      </div>
      <div className="team-intro-actions">
        {onBack && <button className="secondary-button" type="button" onClick={onBack}>← Visão geral</button>}
        {view === 'sectors'
          ? canManageSectors && <button className="primary-button" type="button" onClick={openCreate}>+ Novo setor</button>
          : canManageUsers && <button className="primary-button" type="button" onClick={openUserCreate}>+ Novo usuário</button>}
      </div>
    </section>

    {error && <div className="team-alert" role="alert"><span>!</span><p>{error}</p><button type="button" onClick={() => setError(null)} aria-label="Fechar aviso">×</button></div>}

    <section className="team-stats" aria-label="Resumo da operação">
      <article><span className="team-stat-icon is-blue">♧</span><div><small>Setores ativos</small><strong>{stats.active_sectors}<em> / {stats.sectors}</em></strong></div></article>
      <article><span className="team-stat-icon is-purple">◉</span><div><small>Pessoas na equipe</small><strong>{stats.members}</strong></div></article>
      <article><span className="team-stat-icon is-green">●</span><div><small>Online agora</small><strong>{stats.online_members}</strong></div></article>
    </section>

    <section className="team-workspace">
      <header className="team-workspace-header">
        <div><span className="eyebrow">Estrutura da operação</span><h3>Equipe e setores</h3></div>
        <div className="team-tabs" role="tablist" aria-label="Equipe e setores">
          <button className={view === 'sectors' ? 'is-active' : ''} type="button" role="tab" aria-selected={view === 'sectors'} onClick={() => setView('sectors')}>Setores <b>{stats.sectors}</b></button>
          <button className={view === 'members' ? 'is-active' : ''} type="button" role="tab" aria-selected={view === 'members'} onClick={() => setView('members')}>Equipe <b>{stats.members}</b></button>
        </div>
      </header>

      {loading ? <div className="team-state"><span className="team-spinner" /> Carregando equipe…</div> : view === 'sectors' ? <SectorList sectors={sectors} actionID={actionID} canManage={canManageSectors} onCreate={openCreate} onEdit={openEdit} onToggle={(sector) => void runAction(sector.id, () => toggleSector(sector.id))} onDelete={remove} /> : <>
        <FilterBar resultLabel={`${memberTotal} pessoa${memberTotal === 1 ? '' : 's'}`} onReset={() => { setMemberQuery(''); setMemberRole(''); setMemberStatus('') }}>
          <FilterSearch value={memberQuery} onChange={setMemberQuery} placeholder="Buscar nome, usuário ou perfil" />
          <FilterSelect value={memberRole} onChange={setMemberRole} label="Filtrar perfil" options={[{ value: '', label: 'Todos os perfis' }, { value: 'admin', label: 'Administrador' }, { value: 'supervisor', label: 'Supervisor' }, { value: 'atendente', label: 'Atendente' }]} />
          <FilterSelect value={memberStatus} onChange={setMemberStatus} label="Filtrar status" options={[{ value: '', label: 'Todos os status' }, { value: 'online', label: 'Online' }, { value: 'ativos', label: 'Ativos' }, { value: 'inativos', label: 'Inativos' }]} />
        </FilterBar>
        <MemberList members={members} sectorNames={sectorNames} actionID={actionID} canManageUsers={canManageUsers} currentUserId={currentUserId} onCreate={openUserCreate} onEdit={openUserEdit} onToggle={(member) => void runAction(member.id, () => toggleUser(member.id))} onDelete={removeUser} />
        {members.length > 0 && <Pagination total={memberTotal} limit={DEFAULT_PAGE_SIZE} offset={memberOffset} onOffsetChange={setMemberOffset} loading={loading} />}
      </>}
    </section>

    {draft && <SectorEditor draft={draft} editing={editingID !== null} saving={saving} error={editorError} onChange={updateDraft} onSave={() => void save()} onClose={() => { if (!saving) setDraft(null) }} />}
    {userDraft && userOptions && <UserEditor draft={userDraft} editing={editingUserID !== null} isSelf={editingUserID !== null && editingUserID === currentUserId} saving={userSaving} error={userEditorError} options={userOptions} onChange={updateUserDraft} onSave={() => void saveUser()} onClose={() => { if (!userSaving) setUserDraft(null) }} />}
  </div>
}

function SectorList({ sectors, actionID, canManage, onCreate, onEdit, onToggle, onDelete }: { sectors: Sector[]; actionID: number | null; canManage: boolean; onCreate: () => void; onEdit: (sector: Sector) => void; onToggle: (sector: Sector) => void; onDelete: (sector: Sector) => void }) {
  if (!sectors.length) return <div className="team-empty"><div className="team-empty-icon">♧</div><strong>Nenhum setor cadastrado</strong><p>{canManage ? 'Crie setores como Comercial, Suporte ou Financeiro para organizar a distribuição.' : 'Não há setores visíveis para o seu perfil.'}</p>{canManage && <button className="primary-button" type="button" onClick={onCreate}>Criar primeiro setor</button>}</div>
  return <div className="sector-grid">{sectors.map((sector) => <article className={`sector-card${sector.active ? '' : ' is-paused'}`} key={sector.id}>
    <header><span className="sector-color" style={{ backgroundColor: sector.cor || '#0ea5e9' }} /><div><span className="sector-kicker">Setor de atendimento</span><h4>{sector.name}</h4></div><span className={`sector-status${sector.active ? ' is-active' : ''}`}><i />{sector.active ? 'Ativo' : 'Pausado'}</span></header>
    <p className="sector-description">{sector.description || 'Sem descrição cadastrada.'}</p>
    <div className="sector-meta"><span>♧ {sector.member_count} pessoa{sector.member_count === 1 ? '' : 's'}</span><span>● {sector.online_count} online</span><span>◷ {sector.horario_abertura}–{sector.horario_fechamento}</span></div>
    {canManage && <footer><button type="button" onClick={() => onEdit(sector)}>Editar</button><button type="button" disabled={actionID === sector.id} onClick={() => onToggle(sector)}>{sector.active ? 'Pausar' : 'Ativar'}</button><button className="is-danger" type="button" disabled={actionID === sector.id} onClick={() => onDelete(sector)}>Excluir</button></footer>}
  </article>)}</div>
}

function MemberList({ members, sectorNames, actionID, canManageUsers, currentUserId, onCreate, onEdit, onToggle, onDelete }: { members: TeamMember[]; sectorNames: Map<number, string>; actionID: number | null; canManageUsers: boolean; currentUserId: number | null; onCreate: () => void; onEdit: (member: TeamMember) => void; onToggle: (member: TeamMember) => void; onDelete: (member: TeamMember) => void }) {
  if (!members.length) return <div className="team-empty"><div className="team-empty-icon">◉</div><strong>Nenhuma pessoa cadastrada</strong><p>{canManageUsers ? 'Cadastre usuários com perfil, setores e permissões granulares.' : 'Não há pessoas visíveis para os setores deste perfil.'}</p>{canManageUsers && <button className="primary-button" type="button" onClick={onCreate}>Adicionar primeiro usuário</button>}</div>
  return <div className="member-list">{members.map((member) => <article className="member-row" key={member.id}><div className={`member-avatar${member.online ? ' is-online' : ''}`}>{initials(member.name)}</div><div className="member-identity"><strong>{member.name}</strong><span>{roleLabel(member.role)}{member.username ? ` · @${member.username}` : ''}</span></div><div className="member-sectors">{member.setor_ids.length ? member.setor_ids.map((id) => <span key={id}>{sectorNames.get(id) || 'Setor'}</span>) : <span className="is-muted">Sem setor vinculado</span>}</div>{member.habilidades && member.habilidades.length > 0 && <div className="member-habilidades">{member.habilidades.map((skill) => <span key={skill}>{skill}</span>)}</div>}<span className="member-permissions">{member.permissions.length} permissões</span><span className={`member-status${member.online ? ' is-online' : ''}`}><i />{member.online ? 'Online' : member.active ? 'Offline' : 'Desativado'}</span>{canManageUsers && <footer className="member-actions"><button type="button" onClick={() => onEdit(member)}>Editar</button>{member.id !== currentUserId && <button type="button" disabled={actionID === member.id} onClick={() => onToggle(member)}>{member.active ? 'Desativar' : 'Ativar'}</button>}{member.id !== currentUserId && <button className="is-danger" type="button" disabled={actionID === member.id} onClick={() => onDelete(member)}>Excluir</button>}</footer>}</article>)}</div>
}

function SectorEditor({ draft, editing, saving, error, onChange, onSave, onClose }: { draft: SectorDraft; editing: boolean; saving: boolean; error: string | null; onChange: <K extends keyof SectorDraft>(key: K, value: SectorDraft[K]) => void; onSave: () => void; onClose: () => void }) {
  const days = selectedDays(draft.dias_semana)
  const toggleDay = (value: string) => onChange('dias_semana', days.includes(value) ? days.filter((day) => day !== value).join(',') : [...days, value].sort().join(','))
  return <Modal open title={editing ? 'Editar setor' : 'Criar setor'} onClose={onClose} size="md" footer={<><button className="secondary-button" type="button" disabled={saving} onClick={onClose}>Cancelar</button><button className="primary-button" type="button" disabled={saving} onClick={onSave}>{saving ? 'Salvando…' : 'Salvar setor'} <span>↗</span></button></>}>{error && <div className="team-modal-error" role="alert">{error}</div>}<div className="team-form"><label><span>Nome do setor *</span><input autoFocus value={draft.name} onChange={(event) => onChange('name', event.target.value)} placeholder="Ex.: Suporte" /></label><label><span>Cor de identificação</span><span className="team-color-input"><input type="color" value={draft.cor} onChange={(event) => onChange('cor', event.target.value)} /><code>{draft.cor}</code></span></label><label className="team-form-wide"><span>Descrição</span><textarea rows={3} value={draft.description} onChange={(event) => onChange('description', event.target.value)} placeholder="Que tipo de atendimento acontece neste setor?" /></label><label><span>Início do atendimento</span><input type="time" value={draft.horario_abertura} onChange={(event) => onChange('horario_abertura', event.target.value)} /></label><label><span>Fim do atendimento</span><input type="time" value={draft.horario_fechamento} onChange={(event) => onChange('horario_fechamento', event.target.value)} /></label><div className="team-form-wide"><span className="team-field-label">Dias de atendimento</span><div className="team-day-options">{dayOptions.map((day) => <label key={day.value} className={days.includes(day.value) ? 'is-selected' : ''}><input type="checkbox" checked={days.includes(day.value)} onChange={() => toggleDay(day.value)} /><span>{day.label}</span></label>)}</div></div><label className="team-check team-form-wide"><input type="checkbox" checked={draft.active} onChange={(event) => onChange('active', event.target.checked)} /><span>Setor disponível para novos atendimentos</span></label></div></Modal>
}

function UserEditor({ draft, editing, isSelf, saving, error, options, onChange, onSave, onClose }: { draft: UserDraft; editing: boolean; isSelf: boolean; saving: boolean; error: string | null; options: TeamUsersData; onChange: <K extends keyof UserDraft>(key: K, value: UserDraft[K]) => void; onSave: () => void; onClose: () => void }) {
  const toggleSector = (id: number, checked: boolean) => onChange('setor_ids', checked ? [...draft.setor_ids, id] : draft.setor_ids.filter((item) => item !== id))
  const togglePermission = (key: string, checked: boolean) => onChange('permissions', checked ? [...draft.permissions, key] : draft.permissions.filter((item) => item !== key))
  const roleOptions = isSelf && draft.role && !options.roles.some((role) => role.key === draft.role)
    ? [...options.roles, { key: draft.role, label: roleLabel(draft.role) }]
    : options.roles
  return <Modal open title={editing ? 'Editar usuário' : 'Adicionar usuário'} onClose={onClose} size="lg" footer={<><button className="secondary-button" type="button" disabled={saving} onClick={onClose}>Cancelar</button><button className="primary-button" type="button" disabled={saving} onClick={onSave}>{saving ? 'Salvando…' : 'Salvar usuário'} <span>↗</span></button></>}>{error && <div className="team-modal-error" role="alert">{error}</div>}<div className="team-form"><label><span>Nome completo *</span><input autoFocus value={draft.name} onChange={(event) => onChange('name', event.target.value)} placeholder="Nome da pessoa" /></label><label><span>Usuário de acesso *</span><input value={draft.username} onChange={(event) => onChange('username', event.target.value)} placeholder="nome.empresa" autoComplete="username" /></label><label><span>Perfil</span><select value={draft.role} disabled={isSelf} onChange={(event) => onChange('role', event.target.value)}>{roleOptions.map((role) => <option key={role.key} value={role.key}>{role.label}</option>)}</select>{isSelf && <small className="team-field-hint">Você não pode alterar o próprio perfil.</small>}</label><label><span>Senha {editing && <small>· vazia mantém a atual</small>}</span><input type="password" value={draft.password} onChange={(event) => onChange('password', event.target.value)} placeholder={editing ? 'Senha protegida' : 'Mínimo de 6 caracteres'} autoComplete={editing ? 'new-password' : 'new-password'} /></label><label><span>Capacidade simultânea</span><input type="number" min={0} max={999} value={draft.capacidade} onChange={(event) => onChange('capacidade', Math.max(0, Math.min(999, Number(event.target.value) || 0)))} placeholder="0 = sem limite" /><small className="team-field-hint">Máximo de conversas simultâneas. 0 = sem limite.</small></label><label className="team-check team-form-wide"><input type="checkbox" checked={draft.active} disabled={isSelf} onChange={(event) => onChange('active', event.target.checked)} /><span>Acesso ativo</span>{isSelf && <small className="team-field-hint">Você não pode desativar o próprio acesso.</small>}</label><label className="team-form-wide"><span>Habilidades / nichos</span><input value={draft.habilidades.join(', ')} onChange={(event) => onChange('habilidades', event.target.value.split(',').map((item) => item.trim()).filter(Boolean))} placeholder="Ex.: financeiro, suporte_técnico, cobrança" /><small className="team-field-hint">Separe por vírgula. A distribuição automática roteia a conversa ao atendente com o nicho correspondente.</small></label><div className="team-form-wide"><span className="team-field-label">Setores vinculados</span><div className="team-picker">{options.sectors.length ? options.sectors.map((sector) => <label key={sector.id} className={!sector.active ? 'is-disabled' : ''}><input type="checkbox" checked={draft.setor_ids.includes(sector.id)} onChange={(event) => toggleSector(sector.id, event.target.checked)} /><span>{sector.name}{!sector.active && ' · pausado'}</span></label>) : <small>Nenhum setor disponível para este perfil.</small>}</div></div><div className="team-form-wide"><span className="team-field-label">Permissões granulares</span><p className="team-permission-hint">Selecione apenas o que esta pessoa precisa. Se nada for selecionado, entram as permissões padrão do perfil.</p><div className="team-permission-groups">{options.permission_groups.map((group) => <fieldset key={group.module}><legend>{group.label}</legend>{group.permissions.map((permission) => <label key={permission.key}><input type="checkbox" checked={draft.permissions.includes(permission.key)} onChange={(event) => togglePermission(permission.key, event.target.checked)} /><span>{permission.label}</span></label>)}</fieldset>)}</div></div></div></Modal>
}

export default TeamPage
