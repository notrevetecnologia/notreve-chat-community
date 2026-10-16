package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newConversationHash gera um identificador curto e difícil de adivinhar.
func newConversationHash() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type Store struct {
	pool                          *pgxpool.Pool
	webhookSigningSecretKeyBase64 string
}

type Option func(*Store)

// WithWebhookSigningSecretKeyBase64 configura a chave usada apenas para
// cifrar/decifrar segredos de assinatura. A validação ocorre no uso para que
// uma configuração ausente ou inválida nunca impeça a aplicação de iniciar.
func WithWebhookSigningSecretKeyBase64(value string) Option {
	return func(store *Store) {
		store.webhookSigningSecretKeyBase64 = value
	}
}

func New(pool *pgxpool.Pool, options ...Option) *Store {
	store := &Store{pool: pool}
	for _, option := range options {
		if option != nil {
			option(store)
		}
	}
	return store
}

// ---------- Settings ----------

func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	var st Settings
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, default_base_url, default_model, default_api_key, temperature, whatsapp_base_url, whatsapp_token, modo_aberto, ia_monitora, numeros_permitidos, distribuicao_automatica, followup_ativo, followup_horas, ia_na_fila, whatsapp_admin_token, webhook_base_url, stt_base_url, stt_api_key, stt_model, brand_name, logo_url, favicon_url, primary_color, secondary_color, accent_color, background_color, loading_color, detectar_bots, bot_patterns, fallback_registrar_aprendizado, fallback_transferir_humano, knowledge_top_k, watchdog_ativo, watchdog_minutos, media_ttl_dias, media_purga_ativo, image_model, image_aspect_ratio, logs_retencao_dias, updated_at
			 FROM settings WHERE tenant_id = $1 ORDER BY id ASC LIMIT 1`, TenantID(ctx)).
		Scan(&st.ID, &st.TenantID, &st.DefaultBaseURL, &st.DefaultModel, &st.DefaultAPIKey, &st.Temperature,
			&st.WhatsAppBaseURL, &st.WhatsAppToken, &st.ModoAberto, &st.IAMonitora,
			&st.NumerosPermitidos, &st.DistribuicaoAuto, &st.FollowupAtivo, &st.FollowupHoras, &st.IaNaFila,
			&st.WhatsAppAdminToken, &st.WebhookBaseURL, &st.STTBaseURL, &st.STTAPIKey, &st.STTModel,
			&st.BrandName, &st.LogoURL, &st.FaviconURL, &st.PrimaryColor, &st.SecondaryColor, &st.AccentColor,
			&st.BackgroundColor, &st.LoadingColor, &st.DetectarBots, &st.BotPatterns,
			&st.RegistrarFalhasAprendizado, &st.TransferirFalhasHumano, &st.KnowledgeTopK,
			&st.WatchdogAtivo, &st.WatchdogMinutos, &st.MediaTTLDias, &st.MediaPurgaAtivo,
			&st.ImageModel, &st.ImageAspectRatio, &st.LogsRetencaoDias, &st.UpdatedAt)
	return st, err
}

func (s *Store) UpdateSettings(ctx context.Context, st Settings) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE settings SET default_base_url=$1, default_model=$2, default_api_key=$3, temperature=$4,
				 whatsapp_base_url=$5, whatsapp_token=$6, modo_aberto=$7, ia_monitora=$8, numeros_permitidos=$9, distribuicao_automatica=$10, followup_ativo=$11, followup_horas=$12, ia_na_fila=$13, whatsapp_admin_token=$14, webhook_base_url=$15, stt_base_url=$16, stt_api_key=$17, stt_model=$18, brand_name=$19, logo_url=$20, favicon_url=$21, primary_color=$22, secondary_color=$23, accent_color=$24, background_color=$25, loading_color=$26, detectar_bots=$27, bot_patterns=$28, fallback_registrar_aprendizado=$29, fallback_transferir_humano=$30, knowledge_top_k=$31, watchdog_ativo=$32, watchdog_minutos=$33, media_ttl_dias=$34, media_purga_ativo=$35, image_model=$36, image_aspect_ratio=$37, logs_retencao_dias=$38, updated_at=now()
				 WHERE tenant_id = $39`,
		st.DefaultBaseURL, st.DefaultModel, st.DefaultAPIKey, st.Temperature,
		st.WhatsAppBaseURL, st.WhatsAppToken, st.ModoAberto, st.IAMonitora, st.NumerosPermitidos, st.DistribuicaoAuto,
		st.FollowupAtivo, st.FollowupHoras, st.IaNaFila, st.WhatsAppAdminToken, st.WebhookBaseURL,
		st.STTBaseURL, st.STTAPIKey, st.STTModel, st.BrandName, st.LogoURL, st.FaviconURL,
		st.PrimaryColor, st.SecondaryColor, st.AccentColor, st.BackgroundColor, st.LoadingColor,
		st.DetectarBots, st.BotPatterns, st.RegistrarFalhasAprendizado, st.TransferirFalhasHumano, st.KnowledgeTopK,
		st.WatchdogAtivo, st.WatchdogMinutos, st.MediaTTLDias, st.MediaPurgaAtivo,
		st.ImageModel, st.ImageAspectRatio, st.LogsRetencaoDias, TenantID(ctx))
	return err
}

// ---------- Agents ----------

func scanAgent(row pgx.Row) (Agent, error) {
	var a Agent
	var links, setorIDs, toolIDs []byte
	err := row.Scan(&a.ID, &a.TenantID, &a.Name, &a.SystemPrompt, &a.ProviderID, &a.ProviderName, &a.TemperatureLevel,
		&a.Knowledge, &links, &a.KnowledgeContent, &setorIDs, &toolIDs, &a.MenuOpcoes, &a.Active, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return a, err
	}
	if len(links) > 0 {
		_ = json.Unmarshal(links, &a.KnowledgeLinks)
	}
	if len(setorIDs) > 0 {
		_ = json.Unmarshal(setorIDs, &a.SetorIDs)
	}
	if len(toolIDs) > 0 {
		_ = json.Unmarshal(toolIDs, &a.ToolIDs)
	}
	return a, nil
}

const agentCols = `a.id, a.tenant_id, a.name, a.system_prompt, a.provider_id, COALESCE(p.name,''), a.temperature_level,
	a.knowledge, a.knowledge_links, a.knowledge_content, a.setor_ids, a.tool_ids, a.menu_opcoes, a.active, a.created_at, a.updated_at`

func (s *Store) ListAgents(ctx context.Context) ([]Agent, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+agentCols+` FROM agents a LEFT JOIN providers p ON p.id=a.provider_id AND p.tenant_id=a.tenant_id WHERE a.tenant_id=$1 ORDER BY a.id ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Agent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetAgent(ctx context.Context, id int) (Agent, error) {
	return scanAgent(s.pool.QueryRow(ctx, `SELECT `+agentCols+` FROM agents a LEFT JOIN providers p ON p.id=a.provider_id AND p.tenant_id=a.tenant_id WHERE a.tenant_id=$1 AND a.id=$2`, TenantID(ctx), id))
}

func (s *Store) CreateAgent(ctx context.Context, a Agent) (int, error) {
	tenantID := TenantID(ctx)
	if err := s.validateProviderReference(ctx, tenantID, a.ProviderID); err != nil {
		return 0, err
	}
	if err := s.validateSetorReferences(ctx, tenantID, a.SetorIDs); err != nil {
		return 0, err
	}
	if err := s.validateIntegrationReferences(ctx, tenantID, a.ToolIDs); err != nil {
		return 0, err
	}
	links, _ := json.Marshal(a.KnowledgeLinks)
	setorIDs, _ := json.Marshal(a.SetorIDs)
	toolIDs, _ := json.Marshal(a.ToolIDs)
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO agents (tenant_id, name, system_prompt, provider_id, temperature_level, knowledge, knowledge_links, knowledge_content, setor_ids, tool_ids, menu_opcoes, active)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`,
		tenantID, a.Name, a.SystemPrompt, a.ProviderID, a.TemperatureLevel, a.Knowledge,
		links, a.KnowledgeContent, setorIDs, toolIDs, a.MenuOpcoes, a.Active).
		Scan(&id)
	return id, err
}

func (s *Store) UpdateAgent(ctx context.Context, a Agent) error {
	tenantID := TenantID(ctx)
	if err := s.validateProviderReference(ctx, tenantID, a.ProviderID); err != nil {
		return err
	}
	if err := s.validateSetorReferences(ctx, tenantID, a.SetorIDs); err != nil {
		return err
	}
	if err := s.validateIntegrationReferences(ctx, tenantID, a.ToolIDs); err != nil {
		return err
	}
	links, _ := json.Marshal(a.KnowledgeLinks)
	setorIDs, _ := json.Marshal(a.SetorIDs)
	toolIDs, _ := json.Marshal(a.ToolIDs)
	_, err := s.pool.Exec(ctx,
		`UPDATE agents SET name=$1, system_prompt=$2, provider_id=$3, temperature_level=$4,
		 knowledge=$5, knowledge_links=$6, knowledge_content=$7, setor_ids=$8, tool_ids=$9, menu_opcoes=$10, active=$11, updated_at=now() WHERE tenant_id=$13 AND id=$12`,
		a.Name, a.SystemPrompt, a.ProviderID, a.TemperatureLevel,
		a.Knowledge, links, a.KnowledgeContent, setorIDs, toolIDs, a.MenuOpcoes, a.Active, a.ID, tenantID)
	return err
}

func (s *Store) DeleteAgent(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM agents WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

// ---------- Conversations ----------

func (s *Store) CreateConversation(ctx context.Context, c Conversation) (int, error) {
	var id int
	tenantID := TenantID(ctx)
	if err := s.validateConversationReferences(ctx, tenantID, c); err != nil {
		return 0, err
	}
	hash := c.Hash
	if hash == "" {
		hash = newConversationHash()
	}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO conversations (tenant_id, channel_id, agent_id, channel, customer_name, customer_phone, status, setor_id, origem, hash)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		tenantID, c.ChannelID, c.AgentID, c.Channel, c.CustomerName, c.CustomerPhone, c.Status, c.SetorID, c.Origem, hash).Scan(&id)
	return id, err
}

func (s *Store) GetConversationByHash(ctx context.Context, hash string) (Conversation, error) {
	return scanConv(s.pool.QueryRow(ctx, convSelect+` WHERE c.tenant_id=$1 AND c.hash=$2`, TenantID(ctx), hash))
}

// ListWaitingConversations retorna conversas aguardando humano (sem atendente
// atribuído) cuja última mensagem é do cliente (sem resposta humana) há mais de
// minAge minutos. Conversas já em atendimento humano (humano_atendendo) ficam de
// fora: a IA não deve mandar mensagem enquanto um humano está na conversa.
// cooldownMinutes evita repetir a mensagem de paciência em sequência.
func (s *Store) ListWaitingConversations(ctx context.Context, minAgeMinutes, cooldownMinutes int) ([]Conversation, error) {
	tenantID, err := tenantJobID(ctx)
	if err != nil {
		return nil, err
	}
	query := convSelect + ` WHERE c.tenant_id=$1 AND c.status='humano_aguardando' AND c.atendente_id IS NULL AND (
		(SELECT m.role FROM messages m WHERE m.tenant_id=c.tenant_id AND m.conversation_id=c.id ORDER BY m.id DESC LIMIT 1) = 'user'
		AND (SELECT m.created_at FROM messages m WHERE m.tenant_id=c.tenant_id AND m.conversation_id=c.id ORDER BY m.id DESC LIMIT 1) < now() - make_interval(mins => $2)
	) AND NOT EXISTS (
		SELECT 1 FROM messages pm
		WHERE pm.tenant_id=c.tenant_id AND pm.conversation_id=c.id AND pm.source='system'
		  AND pm.content LIKE '🤖 %' AND pm.created_at > now() - make_interval(mins => $3)
	) ORDER BY c.updated_at ASC`
	rows, err := s.pool.Query(ctx, query, tenantID, minAgeMinutes, cooldownMinutes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		c, err := scanConv(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) SetConversationAvatar(ctx context.Context, id int, url string) error {
	_, err := s.pool.Exec(ctx, `UPDATE conversations SET avatar_url=$1, avatar_tentado=true WHERE tenant_id=$2 AND id=$3 AND avatar_url=''`, url, TenantID(ctx), id)
	return err
}

func (s *Store) SetConversationAvatarTried(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `UPDATE conversations SET avatar_tentado=true WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

// ListConversationsSemAvatar retorna conversas (whatsapp) sem foto que ainda
// não foram tentadas.
func (s *Store) ListConversationsSemAvatar(ctx context.Context, limit int) ([]Conversation, error) {
	tenantID, err := tenantJobID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, convSelect+` WHERE c.tenant_id=$1 AND c.channel='whatsapp' AND c.avatar_url='' AND c.avatar_tentado=false AND c.customer_phone <> '' ORDER BY c.updated_at DESC LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		c, err := scanConv(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

const convSelect = `SELECT c.id, c.tenant_id, c.channel_id, c.agent_id, COALESCE(a.name,''), c.channel, c.customer_name, c.customer_phone,
	c.avatar_url, c.status, c.typing, c.atendente_id, COALESCE(u.name,''), c.origem, c.setor_id, COALESCE(s.name,''),
	COALESCE(s.cor,'#0ea5e9'),
	c.motivo_encerramento, c.satisfacao, c.hash, c.bot_blocked, c.ai_summary, c.ai_summary_at, c.created_at, c.updated_at
	FROM conversations c
	LEFT JOIN agents a ON a.id = c.agent_id AND a.tenant_id = c.tenant_id
	LEFT JOIN setores s ON s.id = c.setor_id AND s.tenant_id = c.tenant_id
	LEFT JOIN usuarios u ON u.id = c.atendente_id AND u.tenant_id = c.tenant_id`

func scanConv(row pgx.Row) (Conversation, error) {
	var c Conversation
	err := row.Scan(&c.ID, &c.TenantID, &c.ChannelID, &c.AgentID, &c.AgentName, &c.Channel, &c.CustomerName, &c.CustomerPhone,
		&c.AvatarURL, &c.Status, &c.Typing, &c.AtendenteID, &c.AtendenteName, &c.Origem, &c.SetorID, &c.SetorName,
		&c.SetorCor, &c.Motivo, &c.Satisfacao, &c.Hash, &c.BotBlocked, &c.AISummary, &c.AISummaryAt, &c.CreatedAt, &c.UpdatedAt)
	if err == nil {
		c.Status = NormalizeConversationStatus(c.Status)
	}
	return c, err
}

func (s *Store) SetConversationBotBlocked(ctx context.Context, id int, blocked bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE conversations SET bot_blocked=$1, updated_at=now() WHERE tenant_id=$2 AND id=$3`, blocked, TenantID(ctx), id)
	return err
}

func (s *Store) SetConversationTyping(ctx context.Context, id int, typing bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE conversations SET typing=$1, updated_at=now() WHERE tenant_id=$2 AND id=$3`, typing, TenantID(ctx), id)
	return err
}

// GetOpenConversationByPhone acha a conversa aberta de um telefone.
func (s *Store) GetOpenConversationByPhone(ctx context.Context, phone string) (Conversation, error) {
	return scanConv(s.pool.QueryRow(ctx, convSelect+` WHERE c.tenant_id=$1 AND c.customer_phone=$2 AND c.status <> 'closed' ORDER BY c.updated_at DESC LIMIT 1`, TenantID(ctx), phone))
}

func (s *Store) GetLatestConversationByPhoneChannel(ctx context.Context, phone, channel string) (Conversation, error) {
	return scanConv(s.pool.QueryRow(ctx, convSelect+` WHERE c.tenant_id=$1 AND c.customer_phone=$2 AND c.channel=$3 ORDER BY c.updated_at DESC LIMIT 1`, TenantID(ctx), phone, channel))
}

func (s *Store) GetConversation(ctx context.Context, id int) (Conversation, error) {
	return scanConv(s.pool.QueryRow(ctx, convSelect+` WHERE c.tenant_id=$1 AND c.id=$2`, TenantID(ctx), id))
}

// ListUnassignedWaitingConversations retorna conversas aguardando humano sem atendente atribuído.
func (s *Store) ListUnassignedWaitingConversations(ctx context.Context) ([]Conversation, error) {
	tenantID, err := tenantJobID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, convSelect+` WHERE c.tenant_id=$1 AND c.status='humano_aguardando' AND c.atendente_id IS NULL ORDER BY c.updated_at ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		c, err := scanConv(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListAvailableAtendentes retorna atendentes online/ativos com setores e carga,
// ordenados do menos para o mais carregado (round-robin de atribuição).
func (s *Store) ListAvailableAtendentes(ctx context.Context) ([]AtendenteDisponivel, error) {
	return s.listAtendentesParaDistribuicao(ctx, false)
}

func (s *Store) ListAtendentesFallback(ctx context.Context) ([]AtendenteDisponivel, error) {
	return s.listAtendentesParaDistribuicao(ctx, true)
}

func (s *Store) listAtendentesParaDistribuicao(ctx context.Context, incluirOffline bool) ([]AtendenteDisponivel, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.name, a.id, a.horario_abertura, a.horario_fechamento, a.dias_semana,
			COALESCE(ARRAY(SELECT ast.setor_id FROM atendente_setores ast WHERE ast.tenant_id=$1 AND ast.atendente_id=a.id), '{}') AS setores,
			(SELECT COUNT(*) FROM conversations c2 WHERE c2.tenant_id=$1 AND c2.atendente_id=u.id AND c2.status IN ('humano_aguardando','humano_atendendo')) AS carga,
			a.capacidade
		FROM usuarios u
		JOIN atendentes a ON a.id = u.atendente_id
		WHERE u.tenant_id=$1 AND a.tenant_id=$1 AND u.active AND a.active AND ($2::boolean OR a.online) AND u.role IN ('supervisor','admin','atendente')
		ORDER BY carga ASC, u.id ASC`, TenantID(ctx), incluirOffline)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AtendenteDisponivel
	for rows.Next() {
		var c AtendenteDisponivel
		if err := rows.Scan(&c.UsuarioID, &c.Nome, &c.AtendenteID, &c.HorarioAbertura, &c.HorarioFechamento, &c.DiasSemana, &c.Setores, &c.Carga, &c.Capacidade); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) ListConversations(ctx context.Context) ([]Conversation, error) {
	return s.ListConversationsForUser(ctx, "", nil, nil, false, true)
}

func conversationScopeFilter(query string, args []interface{}, user *Usuario, setorIDs []int, modoAberto bool, verIA bool) (string, []interface{}) {
	if user != nil && (user.Role == "atendente" || user.Role == "supervisor") {
		args = append(args, user.ID)
		if user.Role == "atendente" {
			// atendente não vê conversas encerradas nem, sem a permissão
			// conversas_ver_ia, conversas em atendimento pela IA.
			query += " AND c.status <> 'closed'"
			if !verIA {
				query += " AND c.status <> 'ia'"
			}
			query += " AND (c.atendente_id = $" + strconv.Itoa(len(args))
			if modoAberto {
				// modo aberto: qualquer conversa não atribuída
				query += " OR (c.atendente_id IS NULL)"
			} else if len(setorIDs) > 0 {
				args = append(args, setorIDs)
				query += " OR (c.atendente_id IS NULL AND (c.setor_id IS NULL OR c.setor_id = ANY($" + strconv.Itoa(len(args)) + ")))"
			} else {
				// sem setores: fallback — conversas sem setor
				query += " OR (c.atendente_id IS NULL AND c.setor_id IS NULL)"
			}
			query += ")"
		} else {
			query += " AND (c.atendente_id = $" + strconv.Itoa(len(args))
			if len(setorIDs) > 0 {
				args = append(args, setorIDs)
				query += " OR c.setor_id = ANY($" + strconv.Itoa(len(args)) + ")"
			}
			// supervisor sem setores vinculados só vê conversas atribuídas a ele.
			query += ")"
		}
	}
	return query, args
}

// ListConversationsForUser lista conversas com escopo por papel.
//   - admin/super_admin: todas as conversas do tenant atual
//   - supervisor: conversas atribuídas a ele ou pertencentes aos seus setores
//   - atendente: vê só as atribuídas a ele ou as não-atribuídas
//     (aguardando/em atendimento) do setor dele; conversas em modo IA só
//     aparecem quando verIA=true (permissão conversas_ver_ia).
func (s *Store) ListConversationsForUser(ctx context.Context, status string, user *Usuario, setorIDs []int, modoAberto bool, verIA bool) ([]Conversation, error) {
	query := convSelect + " WHERE c.tenant_id=$1"
	args := []interface{}{TenantID(ctx)}
	if status != "" {
		args = append(args, status)
		query += " AND c.status=$" + strconv.Itoa(len(args))
	}
	query, args = conversationScopeFilter(query, args, user, setorIDs, modoAberto, verIA)
	query += " ORDER BY c.updated_at DESC"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		c, err := scanConv(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) ListConversationsFiltered(ctx context.Context, status string) ([]Conversation, error) {
	return s.ListConversationsForUser(ctx, status, nil, nil, false, true)
}

func (s *Store) SetConversationAtendente(ctx context.Context, id int, atendenteID *int) error {
	if err := s.validateAtendenteReference(ctx, TenantID(ctx), atendenteID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE conversations SET atendente_id=$1, updated_at=now() WHERE tenant_id=$3 AND id=$2`, atendenteID, id, TenantID(ctx))
	return err
}

// ClaimConversation assume a conversa de forma atômica: só tem sucesso se ela
// ainda não está com outro atendente. Evita dois atendentes pegarem a mesma.
func (s *Store) ClaimConversation(ctx context.Context, id, atendenteID int) (bool, error) {
	if err := s.validateAtendenteReference(ctx, TenantID(ctx), &atendenteID); err != nil {
		return false, err
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE conversations SET atendente_id=$1, status='humano_atendendo', updated_at=now()
		 WHERE tenant_id=$3 AND id=$2 AND (atendente_id IS NULL OR atendente_id=$1)`, atendenteID, id, TenantID(ctx))
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s *Store) SetConversationStatus(ctx context.Context, id int, status string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE conversations SET status=$1, updated_at=now() WHERE tenant_id=$3 AND id=$2`, status, id, TenantID(ctx))
	return err
}

func (s *Store) SetConversationAISummary(ctx context.Context, id int, summary string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE conversations SET ai_summary=$1, ai_summary_at=now() WHERE tenant_id=$3 AND id=$2`, summary, id, TenantID(ctx))
	return err
}

// SetConversationToAI devolve o atendimento para a automação, removendo a
// atribuição humana e liberando o processador para a próxima mensagem.
func (s *Store) SetConversationToAI(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE conversations SET atendente_id=NULL, status='ia', bot_blocked=false, updated_at=now() WHERE tenant_id=$2 AND id=$1`,
		id, TenantID(ctx))
	return err
}

func (s *Store) SetConversationSetor(ctx context.Context, id int, setorID *int) error {
	if err := s.validateSetorReference(ctx, TenantID(ctx), setorID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE conversations SET setor_id=$1, updated_at=now() WHERE tenant_id=$3 AND id=$2`, setorID, id, TenantID(ctx))
	return err
}

// ListFollowupCandidates retorna conversas do WhatsApp onde o cliente ficou
// sem responder por mais de `hours` horas (última mensagem é da IA/humano),
// sem follow-up enviado desde a última resposta do cliente.
func (s *Store) ListFollowupCandidates(ctx context.Context, hours int) ([]Conversation, error) {
	tenantID, err := tenantJobID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, convSelect+` WHERE c.tenant_id=$1 AND c.channel='whatsapp' AND c.customer_phone<>''
		AND c.status IN ('ia','humano_atendendo','humano_aguardando')
		AND c.followup_enviado IS NULL
		AND (SELECT m.role FROM messages m WHERE m.tenant_id=c.tenant_id AND m.conversation_id=c.id ORDER BY m.id DESC LIMIT 1) IN ('assistant','system')
		AND (SELECT m.created_at FROM messages m WHERE m.tenant_id=c.tenant_id AND m.conversation_id=c.id ORDER BY m.id DESC LIMIT 1) < now() - make_interval(hours => $2)
		ORDER BY c.updated_at ASC`, tenantID, hours)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		c, err := scanConv(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) SetConversationFollowupSent(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `UPDATE conversations SET followup_enviado=now() WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

func (s *Store) ResetConversationFollowup(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `UPDATE conversations SET followup_enviado=NULL WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

// EncerrarConversation encerra com motivo e inicia a satisfação.
func (s *Store) EncerrarConversation(ctx context.Context, id int, motivo string, encerradoPor *int) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE conversations SET status='closed', motivo_encerramento=$1, encerrado_por=$2, satisfacao='aguardando', updated_at=now() WHERE tenant_id=$4 AND id=$3`,
		motivo, encerradoPor, id, TenantID(ctx))
	return err
}

func (s *Store) EncerrarConversationIfOpen(ctx context.Context, id int, motivo string, encerradoPor *int) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE conversations SET status='closed', motivo_encerramento=$1, encerrado_por=$2, satisfacao='aguardando', updated_at=now() WHERE tenant_id=$4 AND id=$3 AND status<>'closed'`,
		motivo, encerradoPor, id, TenantID(ctx))
	return tag.RowsAffected() > 0, err
}

// EncerrarConversationSilencioso encerra sem pesquisa de satisfação (cliente não recebe aviso).
func (s *Store) EncerrarConversationSilencioso(ctx context.Context, id int, motivo string, encerradoPor *int) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE conversations SET status='closed', motivo_encerramento=$1, encerrado_por=$2, satisfacao='sem_resposta', updated_at=now() WHERE tenant_id=$4 AND id=$3`,
		motivo, encerradoPor, id, TenantID(ctx))
	return err
}

func (s *Store) SetConversationSatisfacao(ctx context.Context, id int, satisfacao string) error {
	_, err := s.pool.Exec(ctx, `UPDATE conversations SET satisfacao=$1, updated_at=now() WHERE tenant_id=$3 AND id=$2`, satisfacao, id, TenantID(ctx))
	return err
}

func (s *Store) UpdateConversationName(ctx context.Context, id int, name string) error {
	_, err := s.pool.Exec(ctx, `UPDATE conversations SET customer_name=$1, updated_at=now() WHERE tenant_id=$3 AND id=$2`, name, id, TenantID(ctx))
	return err
}

// UpdateConversationIdentity mantém nome e site de origem atualizados quando
// um visitante retoma uma conversa do Web Chat.
func (s *Store) UpdateConversationIdentity(ctx context.Context, id int, name, origem string) error {
	_, err := s.pool.Exec(ctx, `UPDATE conversations SET
		customer_name=CASE WHEN trim($1)<>'' THEN trim($1) ELSE customer_name END,
		origem=CASE WHEN trim($2)<>'' THEN trim($2) ELSE origem END,
		updated_at=now() WHERE tenant_id=$4 AND id=$3`, name, origem, id, TenantID(ctx))
	return err
}

// ---------- Aprendizados (perguntas que a IA não soube responder) ----------

func (s *Store) AddAprendizado(ctx context.Context, pergunta, canal string) error {
	var n int
	_ = s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM aprendizados WHERE tenant_id=$2 AND lower(pergunta)=lower($1) AND status='novo'`, pergunta, TenantID(ctx)).Scan(&n)
	if n > 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO aprendizados (tenant_id, pergunta, canal) VALUES ($1,$2,$3)`, TenantID(ctx), pergunta, canal)
	return err
}

func (s *Store) ListAprendizados(ctx context.Context) ([]Aprendizado, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, pergunta, canal, status, created_at FROM aprendizados WHERE tenant_id=$1 ORDER BY created_at DESC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Aprendizado
	for rows.Next() {
		var a Aprendizado
		if err := rows.Scan(&a.ID, &a.Pergunta, &a.Canal, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAprendizado carrega uma dúvida da empresa atual. O filtro por tenant é
// intencional: o conteúdo pode virar conhecimento e nunca deve atravessar
// empresas em um painel multi-tenant.
func (s *Store) GetAprendizado(ctx context.Context, id int) (Aprendizado, error) {
	var item Aprendizado
	err := s.pool.QueryRow(ctx,
		`SELECT id, pergunta, canal, status, created_at FROM aprendizados WHERE tenant_id=$1 AND id=$2`,
		TenantID(ctx), id).Scan(&item.ID, &item.Pergunta, &item.Canal, &item.Status, &item.CreatedAt)
	return item, err
}

func (s *Store) MarkAprendizado(ctx context.Context, id int, status string) error {
	_, err := s.pool.Exec(ctx, `UPDATE aprendizados SET status=$1 WHERE tenant_id=$2 AND id=$3`, status, TenantID(ctx), id)
	return err
}

// ListSatisfacaoPendentes retorna conversas encerradas que aguardam avaliação
// há mais de timeoutMinutes (o cliente não respondeu).
func (s *Store) ListSatisfacaoPendentes(ctx context.Context, timeoutMinutes int) ([]Conversation, error) {
	tenantID, err := tenantJobID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, convSelect+` WHERE c.tenant_id=$1 AND c.status='closed' AND c.satisfacao IN ('aguardando','aguardando_operador') AND c.updated_at < now() - make_interval(mins => $2) ORDER BY c.updated_at ASC`, tenantID, timeoutMinutes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		c, err := scanConv(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) SaveAvaliacao(ctx context.Context, a Avaliacao) error {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO avaliacoes (tenant_id, conversation_id, nota_ia, nota_operador, comentario)
		 SELECT $1,$2,$3,$4,$5
		 WHERE EXISTS (SELECT 1 FROM conversations WHERE tenant_id=$1 AND id=$2)
		 ON CONFLICT (conversation_id) DO UPDATE SET
		   nota_ia = COALESCE(EXCLUDED.nota_ia, avaliacoes.nota_ia),
		   nota_operador = COALESCE(EXCLUDED.nota_operador, avaliacoes.nota_operador),
		   comentario = COALESCE(EXCLUDED.comentario, avaliacoes.comentario)`,
		TenantID(ctx), a.ConversationID, a.NotaIA, a.NotaOperador, a.Comentario)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func (s *Store) SaveAvaliacaoAndSetSatisfacao(ctx context.Context, a Avaliacao, satisfacao string) error {
	return s.WithTenantTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`INSERT INTO avaliacoes (tenant_id, conversation_id, nota_ia, nota_operador, comentario)
			 SELECT $1,$2,$3,$4,$5
			 WHERE EXISTS (SELECT 1 FROM conversations WHERE tenant_id=$1 AND id=$2)
			 ON CONFLICT (conversation_id) DO UPDATE SET
			   nota_ia = COALESCE(EXCLUDED.nota_ia, avaliacoes.nota_ia),
			   nota_operador = COALESCE(EXCLUDED.nota_operador, avaliacoes.nota_operador),
			   comentario = COALESCE(EXCLUDED.comentario, avaliacoes.comentario)`,
			TenantID(ctx), a.ConversationID, a.NotaIA, a.NotaOperador, a.Comentario)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		_, err = tx.Exec(ctx, `UPDATE conversations SET satisfacao=$1, updated_at=now() WHERE tenant_id=$3 AND id=$2`, satisfacao, a.ConversationID, TenantID(ctx))
		return err
	})
}

func (s *Store) GetAvaliacaoByConversation(ctx context.Context, convID int) (Avaliacao, error) {
	var a Avaliacao
	err := s.pool.QueryRow(ctx,
		`SELECT id, conversation_id, nota_ia, nota_operador, comentario, created_at FROM avaliacoes WHERE tenant_id=$1 AND conversation_id=$2`, TenantID(ctx), convID).
		Scan(&a.ID, &a.ConversationID, &a.NotaIA, &a.NotaOperador, &a.Comentario, &a.CreatedAt)
	return a, err
}

func (s *Store) SetConversationAgent(ctx context.Context, id int, agentID *int) error {
	if agentID != nil {
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agents WHERE tenant_id=$1 AND id=$2)`, TenantID(ctx), *agentID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("agent does not belong to tenant")
		}
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE conversations SET agent_id=$1, updated_at=now() WHERE tenant_id=$3 AND id=$2`, agentID, id, TenantID(ctx))
	return err
}

func (s *Store) TouchConversation(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `UPDATE conversations SET updated_at=now() WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

// ---------- Messages ----------

func (s *Store) AddMessage(ctx context.Context, m Message) error {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO messages (tenant_id, conversation_id, role, content, source, wa_msg_id, media_type, media_b64, media_mime, media_name, lat, lng, contact_name, contact_tel)
		 SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14
		 WHERE EXISTS (SELECT 1 FROM conversations WHERE tenant_id=$1 AND id=$2)`,
		TenantID(ctx), m.ConversationID, m.Role, m.Content, m.Source, m.WAMsgID, m.MediaType, m.MediaB64, m.MediaMime, m.MediaName,
		m.Lat, m.Lng, m.ContactName, m.ContactTel)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

// AddMessageReturningID records a message and returns its database ID for
// callers that need to publish an event referring to the persisted message.
func (s *Store) AddMessageReturningID(ctx context.Context, m Message) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO messages (tenant_id, conversation_id, role, content, source, wa_msg_id, media_type, media_b64, media_mime, media_name, lat, lng, contact_name, contact_tel)
		 SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14
		 WHERE EXISTS (SELECT 1 FROM conversations WHERE tenant_id=$1 AND id=$2)
		 RETURNING id`,
		TenantID(ctx), m.ConversationID, m.Role, m.Content, m.Source, m.WAMsgID, m.MediaType, m.MediaB64, m.MediaMime, m.MediaName,
		m.Lat, m.Lng, m.ContactName, m.ContactTel).Scan(&id)
	return id, err
}

const msgSelect = `SELECT id, tenant_id, conversation_id, role, content, source, wa_msg_id, media_type, media_b64, media_mime, media_name, COALESCE(lat,0), COALESCE(lng,0), contact_name, contact_tel, created_at FROM messages`

func scanMsg(row interface{ Scan(...interface{}) error }) (Message, error) {
	var m Message
	err := row.Scan(&m.ID, &m.TenantID, &m.ConversationID, &m.Role, &m.Content, &m.Source, &m.WAMsgID,
		&m.MediaType, &m.MediaB64, &m.MediaMime, &m.MediaName, &m.Lat, &m.Lng, &m.ContactName, &m.ContactTel, &m.CreatedAt)
	return m, err
}

func (s *Store) ListMessages(ctx context.Context, conversationID int) ([]Message, error) {
	rows, err := s.pool.Query(ctx, msgSelect+` WHERE tenant_id=$1 AND conversation_id=$2 ORDER BY id ASC`, TenantID(ctx), conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		m, err := scanMsg(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) ListAssistantMessagesAfter(ctx context.Context, conversationID, afterID int) ([]Message, error) {
	rows, err := s.pool.Query(ctx, msgSelect+` WHERE tenant_id=$1 AND conversation_id=$2 AND id>$3 AND role='assistant' ORDER BY id ASC LIMIT 100`, TenantID(ctx), conversationID, afterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		message, err := scanMsg(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, message)
	}
	return out, rows.Err()
}

func (s *Store) ListWebchatHistory(ctx context.Context, conversationID, limit int) ([]Message, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `SELECT id, tenant_id, conversation_id, role, content, source, wa_msg_id, media_type, media_b64, media_mime, media_name, lat, lng, contact_name, contact_tel, created_at FROM (
		SELECT id, tenant_id, conversation_id, role, content, source, wa_msg_id, media_type, media_b64, media_mime, media_name, COALESCE(lat,0) AS lat, COALESCE(lng,0) AS lng, contact_name, contact_tel, created_at
		FROM messages WHERE tenant_id=$1 AND conversation_id=$2 AND role IN ('user','assistant') ORDER BY id DESC LIMIT $3
	) recent ORDER BY id ASC`, TenantID(ctx), conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		message, err := scanMsg(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, message)
	}
	return out, rows.Err()
}

func (s *Store) LatestAssistantMessageID(ctx context.Context, conversationID int) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(MAX(id),0) FROM messages WHERE tenant_id=$1 AND conversation_id=$2 AND role='assistant'`, TenantID(ctx), conversationID).Scan(&id)
	return id, err
}

// CountRecentMessagesByRole conta as mensagens de um papel na conversa dentro
// de uma janela de segundos. Usado para detectar rajada de mensagens (bot).
func (s *Store) CountRecentMessagesByRole(ctx context.Context, conversationID int, role string, withinSeconds int) (int, error) {
	if withinSeconds < 1 {
		withinSeconds = 60
	}
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM messages
		WHERE tenant_id=$1 AND conversation_id=$2 AND role=$3
		  AND created_at >= now() - make_interval(secs => $4)`,
		TenantID(ctx), conversationID, role, withinSeconds).Scan(&n)
	return n, err
}

func (s *Store) EnsureWebchatSession(ctx context.Context, channelID int, visitorID, tokenHash string) (bool, error) {
	if _, err := s.pool.Exec(ctx, `INSERT INTO webchat_sessions (tenant_id, channel_id, visitor_id, token_hash)
		SELECT $1,$2,$3,$4 WHERE EXISTS (SELECT 1 FROM channels WHERE tenant_id=$1 AND id=$2)
		ON CONFLICT (channel_id, visitor_id) DO NOTHING`, TenantID(ctx), channelID, visitorID, tokenHash); err != nil {
		return false, err
	}
	return s.ValidateWebchatSession(ctx, channelID, visitorID, tokenHash)
}

func (s *Store) ValidateWebchatSession(ctx context.Context, channelID int, visitorID, tokenHash string) (bool, error) {
	var valid bool
	err := s.pool.QueryRow(ctx, `UPDATE webchat_sessions SET last_seen=now()
		WHERE tenant_id=$1 AND channel_id=$2 AND visitor_id=$3 AND token_hash=$4 RETURNING true`, TenantID(ctx), channelID, visitorID, tokenHash).Scan(&valid)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	return valid, err
}

func (s *Store) GetMessage(ctx context.Context, id int) (Message, error) {
	return scanMsg(s.pool.QueryRow(ctx, msgSelect+` WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id))
}

// HasHumanMessages diz se houve resposta de atendente humano na conversa.
func (s *Store) HasHumanMessages(ctx context.Context, conversationID int) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE tenant_id=$1 AND conversation_id=$2 AND source='human'`, TenantID(ctx), conversationID).Scan(&n)
	return n > 0, err
}

func (s *Store) SatisfactionParticipants(ctx context.Context, conversationID int) (bool, bool, error) {
	var hasAI, hasHuman bool
	err := s.pool.QueryRow(ctx, `SELECT
		EXISTS(SELECT 1 FROM messages WHERE tenant_id=$1 AND conversation_id=$2 AND source='ai'),
		EXISTS(SELECT 1 FROM messages WHERE tenant_id=$1 AND conversation_id=$2 AND source='human')`, TenantID(ctx), conversationID).Scan(&hasAI, &hasHuman)
	return hasAI, hasHuman, err
}

// MessageExistsWAMsgID evita processar duas vezes um webhook reenviado.
func (s *Store) MessageExistsWAMsgID(ctx context.Context, waMsgID string) (bool, error) {
	if waMsgID == "" {
		return false, nil
	}
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages WHERE tenant_id=$1 AND wa_msg_id=$2 AND role='user'`, TenantID(ctx), waMsgID).Scan(&n)
	return n > 0, err
}

func (s *Store) UpdateMessageContent(ctx context.Context, id int, content string) error {
	_, err := s.pool.Exec(ctx, `UPDATE messages SET content=$1 WHERE tenant_id=$3 AND id=$2`, content, id, TenantID(ctx))
	return err
}

// ---------- Setores ----------

func (s *Store) ListSetores(ctx context.Context) ([]Setor, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, description, horario_abertura, horario_fechamento, dias_semana, cor, active, created_at FROM setores WHERE tenant_id=$1 ORDER BY name ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Setor
	for rows.Next() {
		var st Setor
		if err := rows.Scan(&st.ID, &st.Name, &st.Description, &st.HorarioAbertura, &st.HorarioFechamento, &st.DiasSemana, &st.Cor, &st.Active, &st.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) GetSetor(ctx context.Context, id int) (Setor, error) {
	var st Setor
	err := s.pool.QueryRow(ctx, `SELECT id, name, description, horario_abertura, horario_fechamento, dias_semana, cor, active, created_at FROM setores WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id).
		Scan(&st.ID, &st.Name, &st.Description, &st.HorarioAbertura, &st.HorarioFechamento, &st.DiasSemana, &st.Cor, &st.Active, &st.CreatedAt)
	return st, err
}

func (s *Store) FindSetorByName(ctx context.Context, name string) (Setor, error) {
	var st Setor
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, description, horario_abertura, horario_fechamento, dias_semana, cor, active, created_at FROM setores WHERE tenant_id=$1 AND lower(name)=lower($2) AND active=true LIMIT 1`, TenantID(ctx), name).
		Scan(&st.ID, &st.Name, &st.Description, &st.HorarioAbertura, &st.HorarioFechamento, &st.DiasSemana, &st.Cor, &st.Active, &st.CreatedAt)
	return st, err
}

func (s *Store) CreateSetor(ctx context.Context, st Setor) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO setores (tenant_id, name, description, horario_abertura, horario_fechamento, dias_semana, cor, active) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		TenantID(ctx), st.Name, st.Description, st.HorarioAbertura, st.HorarioFechamento, st.DiasSemana, st.Cor, st.Active).Scan(&id)
	return id, err
}

func (s *Store) UpdateSetor(ctx context.Context, st Setor) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE setores SET name=$1, description=$2, horario_abertura=$3, horario_fechamento=$4, dias_semana=$5, cor=$6, active=$7 WHERE tenant_id=$9 AND id=$8`,
		st.Name, st.Description, st.HorarioAbertura, st.HorarioFechamento, st.DiasSemana, st.Cor, st.Active, st.ID, TenantID(ctx))
	return err
}

func (s *Store) DeleteSetor(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM setores WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

// ---------- Atendentes ----------

func (s *Store) ListAtendentes(ctx context.Context) ([]Atendente, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, active, online, horario_abertura, horario_fechamento, dias_semana, capacidade, created_at FROM atendentes WHERE tenant_id=$1 ORDER BY name ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Atendente
	for rows.Next() {
		var a Atendente
		if err := rows.Scan(&a.ID, &a.Name, &a.Active, &a.Online, &a.HorarioAbertura, &a.HorarioFechamento, &a.DiasSemana, &a.Capacidade, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.SetorIDs, _ = s.setoresOfAtendente(ctx, a.ID)
		_ = s.pool.QueryRow(ctx,
			`SELECT COALESCE(username,''), COALESCE(role,'atendente'), COALESCE(permissions,'') FROM usuarios WHERE tenant_id=$1 AND atendente_id=$2 LIMIT 1`, TenantID(ctx), a.ID).
			Scan(&a.Username, &a.Role, &a.Permissions)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetAtendente(ctx context.Context, id int) (Atendente, error) {
	var a Atendente
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, active, online, horario_abertura, horario_fechamento, dias_semana, capacidade, created_at FROM atendentes WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id).
		Scan(&a.ID, &a.Name, &a.Active, &a.Online, &a.HorarioAbertura, &a.HorarioFechamento, &a.DiasSemana, &a.Capacidade, &a.CreatedAt)
	if err != nil {
		return a, err
	}
	a.SetorIDs, _ = s.setoresOfAtendente(ctx, a.ID)
	_ = s.pool.QueryRow(ctx,
		`SELECT COALESCE(username,''), COALESCE(role,'atendente'), COALESCE(permissions,'') FROM usuarios WHERE tenant_id=$1 AND atendente_id=$2 LIMIT 1`, TenantID(ctx), a.ID).
		Scan(&a.Username, &a.Role, &a.Permissions)
	return a, nil
}

func (s *Store) setoresOfAtendente(ctx context.Context, atendenteID int) ([]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT setor_id FROM atendente_setores WHERE tenant_id=$1 AND atendente_id=$2`, TenantID(ctx), atendenteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) CreateAtendente(ctx context.Context, a Atendente) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO atendentes (tenant_id, name, active, online, horario_abertura, horario_fechamento, dias_semana, capacidade) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		TenantID(ctx), a.Name, a.Active, a.Online, a.HorarioAbertura, a.HorarioFechamento, a.DiasSemana, a.Capacidade).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, s.setAtendenteSetores(ctx, id, a.SetorIDs)
}

func (s *Store) UpdateAtendente(ctx context.Context, a Atendente) error {
	if _, err := s.pool.Exec(ctx,
		`UPDATE atendentes SET name=$1, active=$2, online=$3, horario_abertura=$4, horario_fechamento=$5, dias_semana=$6, capacidade=$9 WHERE tenant_id=$8 AND id=$7`,
		a.Name, a.Active, a.Online, a.HorarioAbertura, a.HorarioFechamento, a.DiasSemana, a.ID, TenantID(ctx), a.Capacidade); err != nil {
		return err
	}
	return s.setAtendenteSetores(ctx, a.ID, a.SetorIDs)
}

// UsuarioByAtendente retorna o usuário vinculado a um atendente (se houver).
func (s *Store) UsuarioByAtendente(ctx context.Context, atendenteID int) (Usuario, error) {
	return scanUsuario(s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, username, password_hash, role, permissions, atendente_id, active, created_at FROM usuarios WHERE tenant_id=$1 AND atendente_id=$2 LIMIT 1`, TenantID(ctx), atendenteID))
}

// SetoresOnline retorna quantos atendentes online cada setor tem.
func (s *Store) SetoresOnline(ctx context.Context) (map[int]int, error) {
	out := map[int]int{}
	rows, err := s.pool.Query(ctx,
		`SELECT ast.setor_id, COUNT(a.id) FROM atendente_setores ast
			 JOIN atendentes a ON a.id = ast.atendente_id
			 WHERE ast.tenant_id=$1 AND a.tenant_id=$1 AND a.active AND a.online
			 GROUP BY ast.setor_id`, TenantID(ctx))
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var sid, n int
		if err := rows.Scan(&sid, &n); err != nil {
			return out, err
		}
		out[sid] = n
	}
	return out, rows.Err()
}

func (s *Store) setAtendenteSetores(ctx context.Context, id int, setorIDs []int) error {
	if err := s.validateSetorReferences(ctx, TenantID(ctx), setorIDs); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM atendente_setores WHERE tenant_id=$1 AND atendente_id=$2`, TenantID(ctx), id); err != nil {
		return err
	}
	for _, sid := range setorIDs {
		if _, err := s.pool.Exec(ctx,
			`INSERT INTO atendente_setores (tenant_id, atendente_id, setor_id) VALUES ($1,$2,$3)`, TenantID(ctx), id, sid); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteAtendente(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM atendentes WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

// ---------- Providers (IAs) ----------

func decodeProviderCapabilities(raw []byte, p *Provider) {
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &p.Capabilities)
	}
	if p.Capabilities == nil {
		p.Capabilities = []string{}
	}
}

func providerCapabilitiesJSON(values []string) []byte {
	if len(values) == 0 {
		values = []string{"text"}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return []byte(`["text"]`)
	}
	return raw
}

func (s *Store) ListProviders(ctx context.Context) ([]Provider, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, provider_type, base_url, model, api_key, active, safe_mode, capacidade, capabilities, created_at FROM providers WHERE tenant_id=$1 ORDER BY id ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Provider
	for rows.Next() {
		var p Provider
		var capabilities []byte
		if err := rows.Scan(&p.ID, &p.Name, &p.ProviderType, &p.BaseURL, &p.Model, &p.APIKey, &p.Active, &p.SafeMode, &p.Capacidade, &capabilities, &p.CreatedAt); err != nil {
			return nil, err
		}
		decodeProviderCapabilities(capabilities, &p)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetProvider(ctx context.Context, id int) (Provider, error) {
	var p Provider
	var capabilities []byte
	err := s.pool.QueryRow(ctx, `SELECT id, name, provider_type, base_url, model, api_key, active, safe_mode, capacidade, capabilities, created_at FROM providers WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id).
		Scan(&p.ID, &p.Name, &p.ProviderType, &p.BaseURL, &p.Model, &p.APIKey, &p.Active, &p.SafeMode, &p.Capacidade, &capabilities, &p.CreatedAt)
	decodeProviderCapabilities(capabilities, &p)
	return p, err
}

func (s *Store) FirstActiveProvider(ctx context.Context) (Provider, error) {
	var p Provider
	var capabilities []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, provider_type, base_url, model, api_key, active, safe_mode, capacidade, capabilities, created_at FROM providers WHERE tenant_id=$1 AND active=true ORDER BY id ASC LIMIT 1`, TenantID(ctx)).
		Scan(&p.ID, &p.Name, &p.ProviderType, &p.BaseURL, &p.Model, &p.APIKey, &p.Active, &p.SafeMode, &p.Capacidade, &capabilities, &p.CreatedAt)
	decodeProviderCapabilities(capabilities, &p)
	return p, err
}

func (s *Store) CreateProvider(ctx context.Context, p Provider) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO providers (tenant_id, name, provider_type, base_url, model, api_key, active, safe_mode, capacidade, capabilities) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		TenantID(ctx), p.Name, p.ProviderType, p.BaseURL, p.Model, p.APIKey, p.Active, p.SafeMode, p.Capacidade, providerCapabilitiesJSON(p.Capabilities)).Scan(&id)
	return id, err
}

func (s *Store) UpdateProvider(ctx context.Context, p Provider) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE providers SET name=$1, provider_type=$2, base_url=$3, model=$4, api_key=$5, active=$6, safe_mode=$7, capacidade=$8, capabilities=$9 WHERE tenant_id=$11 AND id=$10`,
		p.Name, p.ProviderType, p.BaseURL, p.Model, p.APIKey, p.Active, p.SafeMode, p.Capacidade, providerCapabilitiesJSON(p.Capabilities), p.ID, TenantID(ctx))
	return err
}

func (s *Store) DeleteProvider(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM providers WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

// ---------- Canais ----------

const channelCols = `id, tenant_id, type, name, base_url, token, instance_id, active, respond_grupos, responder_chamadas, can_campaign, widget_config, public_id, created_at, health_status, last_checked_at, COALESCE(last_error, ''), agent_id, COALESCE(mode, 'ai'), chatflow_id`

func scanChannel(row interface{ Scan(...interface{}) error }) (Channel, error) {
	var c Channel
	err := row.Scan(&c.ID, &c.TenantID, &c.Type, &c.Name, &c.BaseURL, &c.Token, &c.InstanceID, &c.Active, &c.RespondGrupos, &c.ResponderChamadas, &c.CanCampaign, &c.WidgetConfig, &c.PublicID, &c.CreatedAt, &c.HealthStatus, &c.LastCheckedAt, &c.LastError, &c.AgentID, &c.Mode, &c.ChatflowID)
	return c, err
}

func (s *Store) ListChannels(ctx context.Context) ([]Channel, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+channelCols+` FROM channels WHERE tenant_id=$1 ORDER BY id ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetChannelByID(ctx context.Context, id int) (Channel, error) {
	return scanChannel(s.pool.QueryRow(ctx, `SELECT `+channelCols+` FROM channels WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id))
}

// ChannelByToken acha o canal WhatsApp pelo identificador enviado no webhook.
// A API envia o ID interno da instância, que é diferente do token de autenticação.
// O token continua aceito como compatibilidade com payloads antigos.
func (s *Store) ChannelByToken(ctx context.Context, identifier string) (Channel, error) {
	return s.ChannelByTokenRouted(ctx, identifier)
}

// ChannelByPublicID acha um canal pelo token público (webchat).
func (s *Store) ChannelByPublicID(ctx context.Context, publicID string) (Channel, error) {
	return s.ChannelByPublicIDRouted(ctx, publicID)
}

// ActiveChannel retorna o canal ativo de um tipo (ex.: whatsapp).
func (s *Store) ActiveChannel(ctx context.Context, channelType string) (Channel, error) {
	return scanChannel(s.pool.QueryRow(ctx,
		`SELECT `+channelCols+` FROM channels WHERE tenant_id=$1 AND type=$2 AND active=true ORDER BY id ASC LIMIT 1`, TenantID(ctx), channelType))
}

func channelModeOrDefault(mode string) string {
	if strings.TrimSpace(mode) == "chatflow" {
		return "chatflow"
	}
	return "ai"
}

func (s *Store) CreateChannel(ctx context.Context, c Channel) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO channels (tenant_id, type, name, base_url, token, instance_id, active, respond_grupos, responder_chamadas, can_campaign, widget_config, public_id, agent_id, mode, chatflow_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING id`,
		TenantID(ctx), c.Type, c.Name, c.BaseURL, c.Token, c.InstanceID, c.Active, c.RespondGrupos, c.ResponderChamadas, c.CanCampaign, c.WidgetConfig, c.PublicID, c.AgentID, channelModeOrDefault(c.Mode), c.ChatflowID).Scan(&id)
	return id, err
}

func (s *Store) UpdateChannel(ctx context.Context, c Channel) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE channels SET type=$1, name=$2, base_url=$3, token=$4, instance_id=$5, active=$6, respond_grupos=$7, responder_chamadas=$8, can_campaign=$9, widget_config=$10, public_id=$11, agent_id=$12, mode=$13, chatflow_id=$14 WHERE tenant_id=$15 AND id=$16`,
		c.Type, c.Name, c.BaseURL, c.Token, c.InstanceID, c.Active, c.RespondGrupos, c.ResponderChamadas, c.CanCampaign, c.WidgetConfig, c.PublicID, c.AgentID, channelModeOrDefault(c.Mode), c.ChatflowID, TenantID(ctx), c.ID)
	return err
}

// SetChannelInstanceID sincroniza o ID interno retornado pela API WhatsApp.
func (s *Store) SetChannelInstanceID(ctx context.Context, id int, instanceID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE channels SET instance_id=$1 WHERE tenant_id=$3 AND id=$2`, strings.TrimSpace(instanceID), id, TenantID(ctx))
	return err
}

func (s *Store) DeleteChannel(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM channels WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

func (s *Store) SetChannelHealth(ctx context.Context, id int, status, lastError string) error {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return err
	}
	switch status {
	case "connected", "disconnected", "unknown":
	default:
		return fmt.Errorf("status de canal inválido: %q", status)
	}
	lastError = strings.TrimSpace(lastError)
	if runes := []rune(lastError); len(runes) > 1024 {
		lastError = string(runes[:1024])
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE channels SET health_status=$1, last_checked_at=now(), last_error=NULLIF(btrim($2),'') WHERE tenant_id=$3 AND id=$4`,
		status, lastError, tenantID, id)
	return err
}

func (s *Store) ListMonitorableChannels(ctx context.Context) ([]Channel, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT `+channelCols+` FROM channels WHERE tenant_id=$1 AND active=true AND type IN ('whatsapp','telegram') ORDER BY id ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---------- Integrações (tools por empresa) ----------

func (s *Store) ListIntegrations(ctx context.Context) ([]Integration, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, description, method, url, headers, parameters, requires_identity, allowed_identities, identity_config, identity_verify_url, identity_verify_method, identity_verify_param, identity_match_field, identity_client_id_field, created_at FROM integrations WHERE tenant_id=$1 ORDER BY name ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Integration
	for rows.Next() {
		i, err := scanIntegration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (s *Store) GetIntegrationsByIDs(ctx context.Context, ids []int) ([]Integration, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []Integration
	for _, id := range ids {
		i, err := scanIntegration(s.pool.QueryRow(ctx,
			`SELECT id, name, description, method, url, headers, parameters, requires_identity, allowed_identities, identity_config, identity_verify_url, identity_verify_method, identity_verify_param, identity_match_field, identity_client_id_field, created_at FROM integrations WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id))
		if err == nil {
			out = append(out, i)
		}
	}
	return out, nil
}

func (s *Store) GetIntegration(ctx context.Context, id int) (Integration, error) {
	return scanIntegration(s.pool.QueryRow(ctx,
		`SELECT id, name, description, method, url, headers, parameters, requires_identity, allowed_identities, identity_config, identity_verify_url, identity_verify_method, identity_verify_param, identity_match_field, identity_client_id_field, created_at FROM integrations WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id))
}

func (s *Store) CreateIntegration(ctx context.Context, i Integration) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO integrations (tenant_id, name, description, method, url, headers, parameters, requires_identity, allowed_identities, identity_config, identity_verify_url, identity_verify_method, identity_verify_param, identity_match_field, identity_client_id_field) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING id`,
		TenantID(ctx), i.Name, i.Description, i.Method, i.URL, i.Headers, i.Parameters, i.RequiresIdentity, joinIntegrationList(i.AllowedIdentities), i.IdentityConfig, i.IdentityVerifyURL, i.IdentityVerifyMethod, i.IdentityVerifyParam, i.IdentityMatchField, i.IdentityClientIDField).Scan(&id)
	return id, err
}

func (s *Store) UpdateIntegration(ctx context.Context, i Integration) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE integrations SET name=$1, description=$2, method=$3, url=$4, headers=$5, parameters=$6, requires_identity=$7, allowed_identities=$8, identity_config=$9, identity_verify_url=$10, identity_verify_method=$11, identity_verify_param=$12, identity_match_field=$13, identity_client_id_field=$14 WHERE id=$15 AND tenant_id=$16`,
		i.Name, i.Description, i.Method, i.URL, i.Headers, i.Parameters, i.RequiresIdentity, joinIntegrationList(i.AllowedIdentities), i.IdentityConfig, i.IdentityVerifyURL, i.IdentityVerifyMethod, i.IdentityVerifyParam, i.IdentityMatchField, i.IdentityClientIDField, i.ID, TenantID(ctx))
	return err
}

func (s *Store) DeleteIntegration(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM integrations WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

type integrationScanner interface {
	Scan(dest ...interface{}) error
}

func scanIntegration(scanner integrationScanner) (Integration, error) {
	var i Integration
	var allowed string
	if err := scanner.Scan(&i.ID, &i.Name, &i.Description, &i.Method, &i.URL, &i.Headers, &i.Parameters, &i.RequiresIdentity, &allowed, &i.IdentityConfig, &i.IdentityVerifyURL, &i.IdentityVerifyMethod, &i.IdentityVerifyParam, &i.IdentityMatchField, &i.IdentityClientIDField, &i.CreatedAt); err != nil {
		return Integration{}, err
	}
	i.AllowedIdentities = splitIntegrationList(allowed)
	return i, nil
}

func splitIntegrationList(value string) []string {
	set := map[string]bool{}
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
		part = strings.TrimSpace(part)
		if part != "" {
			set[part] = true
		}
	}
	result := make([]string, 0, len(set))
	for item := range set {
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func joinIntegrationList(values []string) string {
	return strings.Join(values, ",")
}

// ---------- Mensagens (templates) ----------

func (s *Store) ListMensagens(ctx context.Context) ([]Mensagem, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, chave, titulo, texto, created_at FROM mensagens WHERE tenant_id=$1 ORDER BY id ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Mensagem
	for rows.Next() {
		var m Mensagem
		if err := rows.Scan(&m.ID, &m.Chave, &m.Titulo, &m.Texto, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) UpdateMensagem(ctx context.Context, id int, texto string) error {
	_, err := s.pool.Exec(ctx, `UPDATE mensagens SET texto=$1 WHERE tenant_id=$2 AND id=$3`, texto, TenantID(ctx), id)
	return err
}

func (s *Store) GetMensagemByChave(ctx context.Context, chave string) (Mensagem, error) {
	var m Mensagem
	err := s.pool.QueryRow(ctx, `SELECT id, chave, titulo, texto, created_at FROM mensagens WHERE tenant_id=$1 AND chave=$2 LIMIT 1`, TenantID(ctx), chave).
		Scan(&m.ID, &m.Chave, &m.Titulo, &m.Texto, &m.CreatedAt)
	return m, err
}

// ---------- Usuários ----------

func (s *Store) CountUsuarios(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM usuarios WHERE tenant_id=$1`, TenantID(ctx)).Scan(&n)
	return n, err
}

func scanUsuario(row pgx.Row) (Usuario, error) {
	var u Usuario
	err := row.Scan(&u.ID, &u.TenantID, &u.Name, &u.Username, &u.PasswordHash, &u.Role, &u.Permissions, &u.AtendenteID, &u.Active, &u.CreatedAt)
	return u, err
}

func (s *Store) GetUsuarioByUsername(ctx context.Context, username string) (Usuario, error) {
	return scanUsuario(s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, username, password_hash, role, permissions, atendente_id, active, created_at FROM usuarios WHERE tenant_id=$1 AND lower(username)=lower($2) LIMIT 1`, TenantID(ctx), username))
}

func (s *Store) GetUsuarioByID(ctx context.Context, id int) (Usuario, error) {
	return scanUsuario(s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, username, password_hash, role, permissions, atendente_id, active, created_at FROM usuarios WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id))
}

func (s *Store) ListUsuarios(ctx context.Context) ([]Usuario, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, name, username, password_hash, role, permissions, atendente_id, active, created_at FROM usuarios WHERE tenant_id=$1 ORDER BY id ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Usuario
	for rows.Next() {
		u, err := scanUsuario(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) CreateUsuario(ctx context.Context, u Usuario) (int, error) {
	tenantID := TenantID(ctx)
	if err := s.validateAtendenteProfileReference(ctx, tenantID, u.AtendenteID); err != nil {
		return 0, err
	}
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO usuarios (tenant_id, name, username, password_hash, role, permissions, atendente_id, active) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		tenantID, u.Name, u.Username, u.PasswordHash, u.Role, u.Permissions, u.AtendenteID, u.Active).Scan(&id)
	return id, err
}

func (s *Store) UpdateUsuario(ctx context.Context, u Usuario) error {
	tenantID := TenantID(ctx)
	if err := s.validateAtendenteProfileReference(ctx, tenantID, u.AtendenteID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE usuarios SET name=$1, username=$2, password_hash=$3, role=$4, permissions=$5, atendente_id=$6, active=$7 WHERE tenant_id=$8 AND id=$9`,
		u.Name, u.Username, u.PasswordHash, u.Role, u.Permissions, u.AtendenteID, u.Active, tenantID, u.ID)
	return err
}

// ---------- Módulos ----------

func (s *Store) ListModules(ctx context.Context) ([]Module, error) {
	rows, err := s.pool.Query(ctx, `SELECT m.key, m.name, m.description, COALESCE(tm.enabled, m.enabled), m.sort
		FROM modulos m LEFT JOIN tenant_modules tm ON tm.module_key=m.key AND tm.tenant_id=$1
		ORDER BY m.sort ASC, m.name ASC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Module
	for rows.Next() {
		var m Module
		if err := rows.Scan(&m.Key, &m.Name, &m.Description, &m.Enabled, &m.Sort); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) SetModuleEnabled(ctx context.Context, key string, enabled bool) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO tenant_modules (tenant_id, module_key, enabled)
		SELECT $1,$2,$3 WHERE EXISTS (SELECT 1 FROM modulos WHERE key=$2)
		ON CONFLICT (tenant_id, module_key) DO UPDATE SET enabled=EXCLUDED.enabled`, TenantID(ctx), key, enabled)
	return err
}

func (s *Store) ModuleEnabled(ctx context.Context, key string) (bool, error) {
	var enabled bool
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(tm.enabled, m.enabled)
		FROM modulos m LEFT JOIN tenant_modules tm ON tm.module_key=m.key AND tm.tenant_id=$1
		WHERE m.key=$2`, TenantID(ctx), key).Scan(&enabled)
	return enabled, err
}

func (s *Store) DeleteUsuario(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM usuarios WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

// ---------- Respostas rápidas ----------

func (s *Store) ListRespostasRapidas(ctx context.Context, userID int) ([]RespostaRapida, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, atalho, texto, user_id, created_at FROM respostas_rapidas WHERE tenant_id=$1 AND user_id=$2 ORDER BY atalho ASC`, TenantID(ctx), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RespostaRapida
	for rows.Next() {
		var r RespostaRapida
		if err := rows.Scan(&r.ID, &r.Atalho, &r.Texto, &r.UserID, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CreateRespostaRapida(ctx context.Context, r RespostaRapida) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO respostas_rapidas (tenant_id, atalho, texto, user_id) VALUES ($1,$2,$3,$4) RETURNING id`, TenantID(ctx), r.Atalho, r.Texto, r.UserID).Scan(&id)
	return id, err
}

func (s *Store) UpdateRespostaRapida(ctx context.Context, r RespostaRapida) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE respostas_rapidas SET atalho=$1, texto=$2 WHERE tenant_id=$3 AND id=$4`, r.Atalho, r.Texto, TenantID(ctx), r.ID)
	return err
}

func (s *Store) DeleteRespostaRapida(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM respostas_rapidas WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

// UpdateRespostaRapidaOwned e DeleteRespostaRapidaOwned restringem a operação
// ao usuário que criou a resposta. A versão antiga continua disponível para
// compatibilidade com as rotas legadas, mas as APIs novas usam estas versões.
func (s *Store) UpdateRespostaRapidaOwned(ctx context.Context, r RespostaRapida) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE respostas_rapidas SET atalho=$1, texto=$2 WHERE tenant_id=$3 AND id=$4 AND user_id=$5`, r.Atalho, r.Texto, TenantID(ctx), r.ID, r.UserID)
	return err
}

func (s *Store) DeleteRespostaRapidaOwned(ctx context.Context, id, userID int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM respostas_rapidas WHERE tenant_id=$1 AND id=$2 AND user_id=$3`, TenantID(ctx), id, userID)
	return err
}

// ---------- Contatos ----------

func (s *Store) ListContatos(ctx context.Context) ([]Contato, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, phone, email, notes, tem_whatsapp, tags, campos_extras, created_at, updated_at FROM contatos WHERE tenant_id=$1 ORDER BY name ASC, id DESC`, TenantID(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Contato
	for rows.Next() {
		var c Contato
		if err := rows.Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.Notes, &c.TemWhatsapp, &c.Tags, &c.CamposExtras, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetContato(ctx context.Context, id int) (Contato, error) {
	var c Contato
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, phone, email, notes, tem_whatsapp, tags, campos_extras, created_at, updated_at FROM contatos WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id).
		Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.Notes, &c.TemWhatsapp, &c.Tags, &c.CamposExtras, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (s *Store) GetContatoByPhone(ctx context.Context, phone string) (Contato, error) {
	var c Contato
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, phone, email, notes, tem_whatsapp, tags, campos_extras, created_at, updated_at FROM contatos WHERE tenant_id=$1 AND phone=$2`, TenantID(ctx), phone).
		Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.Notes, &c.TemWhatsapp, &c.Tags, &c.CamposExtras, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

// ListContatosFiltered busca por nome/telefone com paginação.
func (s *Store) ListContatosFiltered(ctx context.Context, q string, limit, offset int) ([]Contato, error) {
	query := `SELECT id, name, phone, email, notes, tem_whatsapp, tags, campos_extras, created_at, updated_at FROM contatos WHERE tenant_id=$1`
	args := []interface{}{TenantID(ctx)}
	if q != "" {
		args = append(args, "%"+q+"%")
		query += ` AND (lower(name) LIKE lower($` + strconv.Itoa(len(args)) + `) OR phone LIKE $` + strconv.Itoa(len(args)) + `)`
	}
	args = append(args, limit, offset)
	query += ` ORDER BY name ASC, id DESC LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Contato
	for rows.Next() {
		var c Contato
		if err := rows.Scan(&c.ID, &c.Name, &c.Phone, &c.Email, &c.Notes, &c.TemWhatsapp, &c.Tags, &c.CamposExtras, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CountContatos(ctx context.Context, q string) (int, error) {
	query := `SELECT COUNT(*) FROM contatos WHERE tenant_id=$1`
	args := []interface{}{TenantID(ctx)}
	if q != "" {
		args = append(args, "%"+q+"%")
		query += ` AND (lower(name) LIKE lower($2) OR phone LIKE $2)`
	}
	var n int
	err := s.pool.QueryRow(ctx, query, args...).Scan(&n)
	return n, err
}

func (s *Store) CreateContato(ctx context.Context, c Contato) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO contatos (tenant_id, name, phone, email, notes, tem_whatsapp, tags, campos_extras) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		TenantID(ctx), c.Name, c.Phone, c.Email, c.Notes, c.TemWhatsapp, c.Tags, contatoCamposExtras(c)).Scan(&id)
	return id, err
}

func (s *Store) UpdateContato(ctx context.Context, c Contato) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE contatos SET name=$1, phone=$2, email=$3, notes=$4, tem_whatsapp=$5, tags=$7, campos_extras=$8, updated_at=now() WHERE tenant_id=$9 AND id=$6`,
		c.Name, c.Phone, c.Email, c.Notes, c.TemWhatsapp, c.ID, c.Tags, contatoCamposExtras(c), TenantID(ctx))
	return err
}

func (s *Store) DeleteContato(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM contatos WHERE tenant_id=$1 AND id=$2`, TenantID(ctx), id)
	return err
}

// UpsertContato insere ou atualiza por telefone (usado na sincronização).
func (s *Store) UpsertContato(ctx context.Context, c Contato) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO contatos (tenant_id, name, phone, email, notes, tem_whatsapp, tags, campos_extras) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT (tenant_id, phone) DO UPDATE SET name=EXCLUDED.name, notes=EXCLUDED.notes, tem_whatsapp=EXCLUDED.tem_whatsapp, updated_at=now()`,
		TenantID(ctx), c.Name, c.Phone, c.Email, c.Notes, c.TemWhatsapp, c.Tags, contatoCamposExtras(c))
	return err
}

// UpsertContatoGetID igual ao UpsertContato, mas devolve o id do contato.
func (s *Store) UpsertContatoGetID(ctx context.Context, c Contato) (int, error) {
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO contatos (tenant_id, name, phone, email, notes, tem_whatsapp, tags, campos_extras) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT (tenant_id, phone) DO UPDATE SET name=EXCLUDED.name, notes=EXCLUDED.notes, tem_whatsapp=EXCLUDED.tem_whatsapp, updated_at=now()
		 RETURNING id`,
		TenantID(ctx), c.Name, c.Phone, c.Email, c.Notes, c.TemWhatsapp, c.Tags, contatoCamposExtras(c)).Scan(&id)
	return id, err
}

func (s *Store) SetContatoWhatsapp(ctx context.Context, id int, ok bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE contatos SET tem_whatsapp=$1 WHERE tenant_id=$3 AND id=$2`, ok, id, TenantID(ctx))
	return err
}

func contatoCamposExtras(c Contato) map[string]string {
	if c.CamposExtras == nil {
		return map[string]string{}
	}
	return c.CamposExtras
}

// SetAtendenteOnline marca o atendente vinculado como online/offline.
func (s *Store) SetAtendenteOnline(ctx context.Context, atendenteID *int, online bool) error {
	if atendenteID == nil {
		return nil
	}
	_, err := s.pool.Exec(ctx, `UPDATE atendentes SET online=$1 WHERE tenant_id=$3 AND id=$2`, online, *atendenteID, TenantID(ctx))
	return err
}
