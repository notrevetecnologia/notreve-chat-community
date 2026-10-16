import { useState, type ReactNode } from 'react'
import { EntityScreen, type EntityTab } from '../../shared/EntityScreen'
import type { Channel, Draft, GroupDraft, Options, SegmentDraft } from './campaignsShared'
import './Campaigns.css'

const campaignTabs: EntityTab[] = [
  { key: 'mensagem', label: 'Mensagem', hint: 'Texto e mídia' },
  { key: 'publico', label: 'Público', hint: 'Destino e segmento' },
  { key: 'agendamento', label: 'Agendamento', hint: 'Horário e limites' },
]

export function CampaignEditorPage({
  draft,
  options,
  saving,
  onBack,
  onChange,
  onSave,
}: {
  draft: Draft
  options: Options
  saving: boolean
  onBack: () => void
  onChange: <K extends keyof Draft>(key: K, value: Draft[K]) => void
  onSave: () => void
}) {
  const [tab, setTab] = useState('mensagem')
  const targetOptions = draft.target_kind === 'channel' ? options.channels : options.groups
  const title = draft.name.trim() || 'Nova campanha'

  return (
    <EntityScreen
      eyebrow="Nova campanha"
      title={title}
      onBack={onBack}
      backLabel="Voltar para as campanhas"
      actions={
        <>
          <button className="secondary-button" type="button" disabled={saving} onClick={onBack}>Voltar</button>
          <button className="primary-button" type="button" disabled={saving} onClick={onSave}>{saving ? 'Salvando…' : 'Salvar preparação'}</button>
        </>
      }
      tabs={campaignTabs}
      activeTab={tab}
      onTabChange={setTab}
    >
      {tab === 'mensagem' && (
        <div className="entity-tab-body">
          <p className="entity-tab-lead">O nome identifica a campanha no painel; a mensagem é o que será enviado ao público selecionado.</p>
          <div className="entity-field-grid">
            <Field label="Nome da campanha *" wide help="Uso interno, não é enviado ao cliente.">
              <input autoFocus value={draft.name} onChange={(event) => onChange('name', event.target.value)} placeholder="Ex.: Convite para lançamento" />
            </Field>
            <Field label="Mensagem *" wide help="Escreva como o cliente deve receber. Evite links suspeitos para reduzir risco de bloqueio.">
              <textarea rows={6} value={draft.message} onChange={(event) => onChange('message', event.target.value)} placeholder="Escreva a mensagem que será preparada para o público." />
            </Field>
            <Field label="Mídia opcional (até 16 MB)" wide help="Imagem, áudio, vídeo ou documento enviado junto da mensagem.">
              <input type="file" accept="image/*,audio/*,video/*,.pdf,.doc,.docx" onChange={(event) => onChange('media_file', event.target.files?.[0] || null)} />
              <small className="entity-help">{draft.media_file ? `${draft.media_file.name} · ${(draft.media_file.size / 1024 / 1024).toFixed(2)} MB` : 'Nenhum arquivo selecionado.'}</small>
            </Field>
          </div>
        </div>
      )}

      {tab === 'publico' && (
        <div className="entity-tab-body">
          <p className="entity-tab-lead">Escolha se a campanha sai por um número ou por um grupo de canais e restrinja o público com um segmento.</p>
          <div className="entity-field-grid">
            <Field label="Destino" help="Canal único ou grupo que distribui entre vários números WhatsApp.">
              <select
                value={draft.target_kind}
                onChange={(event) => {
                  const kind = event.target.value as Draft['target_kind']
                  onChange('target_kind', kind)
                  onChange('target_id', kind === 'channel' ? options.channels[0]?.id ?? null : options.groups[0]?.id ?? null)
                }}
              >
                <option value="channel">Canal WhatsApp</option>
                <option value="group">Grupo de canais</option>
              </select>
            </Field>
            <Field label={draft.target_kind === 'channel' ? 'Canal' : 'Grupo'} help="Somente canais liberados para campanha aparecem aqui.">
              <select value={draft.target_id ?? ''} onChange={(event) => onChange('target_id', event.target.value ? Number(event.target.value) : null)}>
                <option value="">Selecione…</option>
                {targetOptions.map((target) => <option key={target.id} value={target.id}>{target.name}{'active' in target && !target.active ? ' · pausado' : ''}</option>)}
              </select>
            </Field>
            <Field label="Público" wide help="Segmento reutilizável de contatos. Vazio envia para todos os contatos válidos.">
              <select value={draft.segmento_id ?? ''} onChange={(event) => onChange('segmento_id', event.target.value ? Number(event.target.value) : null)}>
                <option value="">Todos os contatos</option>
                {options.segments.map((segment) => <option key={segment.id} value={segment.id}>{segment.name} · {segment.count ?? 0} contatos</option>)}
              </select>
            </Field>
          </div>
        </div>
      )}

      {tab === 'agendamento' && (
        <div className="entity-tab-body">
          <p className="entity-tab-lead">Defina quando começar e como espaçar os envios para proteger o número contra bloqueios.</p>
          <div className="entity-field-grid">
            <Field label="Agendar para depois" help="Vazio inicia manualmente depois de salvar.">
              <input type="datetime-local" value={draft.scheduled_at} onChange={(event) => onChange('scheduled_at', event.target.value)} />
            </Field>
            <Field label="Intervalo entre envios (seg.)" help="Tempo de espera entre uma mensagem e outra. Mínimo de 5 segundos.">
              <input type="number" min={5} value={draft.intervalo_segundos} onChange={(event) => onChange('intervalo_segundos', Number(event.target.value))} />
            </Field>
            <Field label="Limite diário" help="Máximo de envios por dia. Zero usa o limite do plano.">
              <input type="number" min={0} value={draft.limite_diario} onChange={(event) => onChange('limite_diario', Number(event.target.value))} />
            </Field>
          </div>
          <section className="entity-section">
            <Toggle
              label="Modo teste"
              help="Marca os envios como enviados sem chamar a API. Ideal para validar a campanha."
              checked={draft.modo_teste}
              onChange={(value) => onChange('modo_teste', value)}
            />
            <Toggle
              label="Variar mensagem"
              help="Pequenas variações no texto reduzem o risco de bloqueio por repetição."
              checked={draft.variar_mensagem}
              onChange={(value) => onChange('variar_mensagem', value)}
            />
          </section>
        </div>
      )}
    </EntityScreen>
  )
}

const segmentTabs: EntityTab[] = [
  { key: 'identificacao', label: 'Identificação', hint: 'Nome e descrição' },
  { key: 'filtros', label: 'Filtros', hint: 'WhatsApp e canal' },
]

export function SegmentEditorPage({
  draft,
  saving,
  onBack,
  onChange,
  onSave,
}: {
  draft: SegmentDraft
  saving: boolean
  onBack: () => void
  onChange: (key: keyof SegmentDraft, value: string | boolean | null) => void
  onSave: () => void
}) {
  const [tab, setTab] = useState('identificacao')
  return (
    <EntityScreen
      eyebrow="Novo segmento"
      title={draft.name.trim() || 'Novo segmento'}
      onBack={onBack}
      backLabel="Voltar para os segmentos"
      actions={
        <>
          <button className="secondary-button" type="button" disabled={saving} onClick={onBack}>Voltar</button>
          <button className="primary-button" type="button" disabled={saving} onClick={onSave}>{saving ? 'Salvando…' : 'Criar segmento'}</button>
        </>
      }
      tabs={segmentTabs}
      activeTab={tab}
      onTabChange={setTab}
    >
      {tab === 'identificacao' && (
        <div className="entity-tab-body">
          <p className="entity-tab-lead">Segmentos são públicos reutilizáveis que dão contexto às campanhas.</p>
          <div className="entity-field-grid">
            <Field label="Nome *" wide help="Como o segmento aparece ao escolher o público de uma campanha.">
              <input autoFocus value={draft.name} onChange={(event) => onChange('name', event.target.value)} placeholder="Ex.: Clientes premium" />
            </Field>
            <Field label="Descrição" wide help="Explique para que este público será usado.">
              <textarea rows={3} value={draft.description} onChange={(event) => onChange('description', event.target.value)} placeholder="Para que este público será usado?" />
            </Field>
          </div>
        </div>
      )}

      {tab === 'filtros' && (
        <div className="entity-tab-body">
          <p className="entity-tab-lead">Filtros opcionais que reúnem contatos automaticamente, além dos vinculados manualmente.</p>
          <section className="entity-section">
            <label className="campaigns-check">
              <input type="checkbox" checked={draft.filtro_tem_whatsapp !== null} onChange={(event) => onChange('filtro_tem_whatsapp', event.target.checked ? true : null)} />
              <span>Filtrar por WhatsApp verificado</span>
            </label>
            <Help>Ligado, você escolhe entre contatos com ou sem WhatsApp.</Help>
            {draft.filtro_tem_whatsapp !== null && (
              <div className="campaigns-filter-options">
                <label><input type="radio" name="filter-whatsapp" checked={draft.filtro_tem_whatsapp === true} onChange={() => onChange('filtro_tem_whatsapp', true)} /><span>Com WhatsApp</span></label>
                <label><input type="radio" name="filter-whatsapp" checked={draft.filtro_tem_whatsapp === false} onChange={() => onChange('filtro_tem_whatsapp', false)} /><span>Sem WhatsApp</span></label>
              </div>
            )}
            <label className="segment-filter-canal">
              <span>Canal de conversa</span>
              <select value={draft.filtro_canal} onChange={(event) => onChange('filtro_canal', event.target.value)}>
                <option value="">Qualquer canal</option>
                <option value="whatsapp">WhatsApp</option>
                <option value="webchat">Web Chat</option>
                <option value="instagram">Instagram</option>
                <option value="telegram">Telegram</option>
              </select>
            </label>
            <Help>O segmento reúne automaticamente os contatos que casam com o filtro.</Help>
          </section>
        </div>
      )}
    </EntityScreen>
  )
}

const groupTabs: EntityTab[] = [
  { key: 'identificacao', label: 'Identificação', hint: 'Nome do grupo' },
  { key: 'canais', label: 'Canais', hint: 'Números participantes' },
]

export function GroupEditorPage({
  draft,
  channels,
  editing,
  saving,
  onBack,
  onChange,
  onSave,
}: {
  draft: GroupDraft
  channels: Channel[]
  editing: boolean
  saving: boolean
  onBack: () => void
  onChange: <K extends keyof GroupDraft>(key: K, value: GroupDraft[K]) => void
  onSave: () => void
}) {
  const [tab, setTab] = useState('identificacao')
  const toggleChannel = (id: number, checked: boolean) => onChange('channel_ids', checked ? [...draft.channel_ids, id] : draft.channel_ids.filter((item) => item !== id))
  return (
    <EntityScreen
      eyebrow={editing ? 'Editar grupo de canais' : 'Novo grupo de canais'}
      title={draft.name.trim() || 'Novo grupo de canais'}
      onBack={onBack}
      backLabel="Voltar para os grupos"
      actions={
        <>
          <button className="secondary-button" type="button" disabled={saving} onClick={onBack}>Voltar</button>
          <button className="primary-button" type="button" disabled={saving} onClick={onSave}>{saving ? 'Salvando…' : 'Salvar grupo'}</button>
        </>
      }
      tabs={groupTabs}
      activeTab={tab}
      onTabChange={setTab}
    >
      {tab === 'identificacao' && (
        <div className="entity-tab-body">
          <p className="entity-tab-lead">Grupos combinam vários números WhatsApp em um único destino reutilizável nas campanhas.</p>
          <div className="entity-field-grid">
            <Field label="Nome do grupo *" wide help="Nome exibido ao escolher o destino de uma campanha.">
              <input autoFocus value={draft.name} onChange={(event) => onChange('name', event.target.value)} placeholder="Ex.: Números comerciais" />
            </Field>
          </div>
        </div>
      )}

      {tab === 'canais' && (
        <div className="entity-tab-body">
          <p className="entity-tab-lead">Marque pelo menos um canal WhatsApp. A campanha distribui os envios entre os selecionados.</p>
          <div className="campaigns-channel-picker">
            {channels.length ? channels.map((channel) => (
              <label key={channel.id}>
                <input type="checkbox" checked={draft.channel_ids.includes(channel.id)} onChange={(event) => toggleChannel(channel.id, event.target.checked)} />
                <span>{channel.name}{!channel.active && ' · pausado'}</span>
              </label>
            )) : <small>Nenhum canal WhatsApp disponível.</small>}
          </div>
          <Help>O vínculo não é validado na criação; ajuste depois se um número sair de operação.</Help>
        </div>
      )}
    </EntityScreen>
  )
}

function Help({ children }: { children: ReactNode }) {
  return <small className="entity-help">{children}</small>
}

function Field({ label, help, wide, children }: { label: string; help?: string; wide?: boolean; children: ReactNode }) {
  return (
    <label className={`entity-field${wide ? ' entity-field-wide' : ''}`}>
      <span>{label}</span>
      {children}
      {help && <Help>{help}</Help>}
    </label>
  )
}

function Toggle({ label, help, checked, onChange }: { label: string; help?: string; checked: boolean; onChange: (value: boolean) => void }) {
  return (
    <label className="campaigns-check" style={{ alignItems: 'flex-start' }}>
      <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} style={{ marginTop: 3 }} />
      <span style={{ display: 'grid', gap: 3 }}>
        <strong style={{ color: 'var(--ink)', fontSize: 13 }}>{label}</strong>
        {help && <Help>{help}</Help>}
      </span>
    </label>
  )
}

export default CampaignEditorPage
