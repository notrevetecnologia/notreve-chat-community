package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Os métodos deste arquivo são deliberadamente independentes de TenantID(ctx):
// eles compõem o control plane e devem receber a empresa alvo de forma explícita.
var (
	ErrInvalidPlatformTenant      = errors.New("invalid platform tenant")
	ErrInvalidWhatsAppServer      = errors.New("invalid whatsapp server")
	ErrWhatsAppChannelRequired    = errors.New("whatsapp channel required")
	ErrWhatsAppServerUnavailable  = errors.New("no whatsapp server with available capacity")
	ErrTenantWhatsAppLimitReached = errors.New("tenant whatsapp connection limit reached")
	ErrPlatformModuleNotFound     = errors.New("platform module not found")
	ErrPlatformTenantNotFound     = errors.New("platform tenant not found")
	ErrCoreModuleCannotBeDisabled = errors.New("core module cannot be disabled")
	ErrInvalidContextSwitch       = errors.New("invalid context switch")
)

// PlatformTenant representa a empresa vista pelo super_admin. Não contém
// usuários, conversas, tokens ou dados operacionais da empresa.
type PlatformTenant struct {
	ID        int                  `json:"id"`
	Slug      string               `json:"slug"`
	Name      string               `json:"name"`
	Status    string               `json:"status"`
	Plan      string               `json:"plan"`
	Timezone  string               `json:"timezone"`
	Locale    string               `json:"locale"`
	Dominio   string               `json:"dominio"`
	Creditos  int                  `json:"creditos"`
	Limits    PlatformTenantLimits `json:"limits"`
	Usage     TenantUsage          `json:"usage"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

type PlatformTenantInput struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Plan     string `json:"plan"`
	Timezone string `json:"timezone"`
	Locale   string `json:"locale"`
	Dominio  string `json:"dominio"`
	Creditos int    `json:"creditos"`
}

// PlatformTenantLimits reúne os limites do plano. Zero em créditos significa
// que aquele plano ainda não possui franquia pré-paga configurada.
type PlatformTenantLimits struct {
	MaxCanais            int       `json:"max_canais"`
	MaxAtendentes        int       `json:"max_atendentes"`
	MaxCampanhasAtivas   int       `json:"max_campanhas_ativas"`
	CampanhaLimiteDiario int       `json:"campanha_limite_diario"`
	MaxAgentes           int       `json:"max_agentes"`
	MaxUsuarios          int       `json:"max_usuarios"`
	MaxConexoesWhatsApp  int       `json:"max_conexoes_whatsapp"`
	MaxTools             int       `json:"max_tools"`
	MaxAPIKeys           int       `json:"max_api_keys"`
	MaxWebhooks          int       `json:"max_webhooks"`
	CreditosIAMensais    int64     `json:"creditos_ia_mensais"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// TenantUsage combina contagens de dados reais com contadores de consumo que
// não podem ser inferidos das tabelas (API, créditos e armazenamento).
type TenantUsage struct {
	CanaisAtivos           int       `json:"canais_ativos"`
	ConexoesWhatsAppAtivas int       `json:"conexoes_whatsapp_ativas"`
	AtendentesAtivos       int       `json:"atendentes_ativos"`
	UsuariosAtivos         int       `json:"usuarios_ativos"`
	AgentesAtivos          int       `json:"agentes_ativos"`
	ConversasAbertas       int       `json:"conversas_abertas"`
	APIRequests            int64     `json:"api_requests"`
	AICreditsUsed          int64     `json:"ai_credits_used"`
	StorageBytes           int64     `json:"storage_bytes"`
	PeriodStart            time.Time `json:"period_start"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type TenantUsageDelta struct {
	APIRequests   int64
	AICreditsUsed int64
	StorageBytes  int64
}

// WhatsAppServer é seguro para qualquer listagem do control plane: o token
// nunca faz parte desse tipo. TokenConfigured indica somente se há credencial.
type WhatsAppServer struct {
	ID                   int64     `json:"id"`
	Name                 string    `json:"name"`
	BaseURL              string    `json:"base_url"`
	Status               string    `json:"status"`
	Active               bool      `json:"active"`
	MaxConnections       int       `json:"max_connections"`
	ActiveConnections    int       `json:"active_connections"`
	AvailableConnections int       `json:"available_connections"`
	TokenConfigured      bool      `json:"token_configured"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// WhatsAppServerInput só é aceito pelo control plane. Em Update, Token vazio
// preserva o segredo armazenado; não há método de leitura do token para UI.
type WhatsAppServerInput struct {
	Name           string `json:"name"`
	BaseURL        string `json:"base_url"`
	Token          string `json:"-"`
	Status         string `json:"status"`
	MaxConnections int    `json:"max_connections"`
}

// WhatsAppServerCredentials é exclusivo de código de infraestrutura que vai
// falar com o servidor WhatsApp. O token é explicitamente omitido de JSON.
type WhatsAppServerCredentials struct {
	ServerID int64  `json:"server_id"`
	BaseURL  string `json:"base_url"`
	Token    string `json:"-"`
}

const platformTenantSelect = `
	SELECT t.id, t.slug, t.name, t.status, t.plan, t.timezone, t.locale, t.dominio, t.creditos,
		COALESCE(l.max_canais, 3), COALESCE(l.max_atendentes, 10),
		COALESCE(l.max_campanhas_ativas, 2), COALESCE(l.campanha_limite_diario, 100),
		COALESCE(l.max_agentes, 3), COALESCE(l.max_usuarios, 10),
		COALESCE(l.max_conexoes_whatsapp, 3), COALESCE(l.max_tools, 10),
		COALESCE(l.max_api_keys, 3), COALESCE(l.max_webhooks, 10),
		COALESCE(l.creditos_ia_mensais, 0), COALESCE(l.updated_at, t.updated_at),
		(SELECT COUNT(*) FROM channels c WHERE c.tenant_id=t.id AND c.active),
		(SELECT COUNT(*) FROM channels c WHERE c.tenant_id=t.id AND c.active AND c.type='whatsapp'),
		(SELECT COUNT(*) FROM atendentes a WHERE a.tenant_id=t.id AND a.active),
		(SELECT COUNT(*) FROM usuarios u WHERE u.tenant_id=t.id AND u.active),
		(SELECT COUNT(*) FROM agents a WHERE a.tenant_id=t.id AND a.active),
		(SELECT COUNT(*) FROM conversations c WHERE c.tenant_id=t.id AND c.status <> 'closed'),
		COALESCE(u.api_requests, 0), COALESCE(u.ai_credits_used, 0), COALESCE(u.storage_bytes, 0),
		COALESCE(u.period_start::timestamptz, t.created_at), COALESCE(u.updated_at, t.updated_at),
		t.created_at, t.updated_at
	FROM tenants t
	LEFT JOIN tenant_limits l ON l.tenant_id=t.id
	LEFT JOIN tenant_usage_counters u ON u.tenant_id=t.id`

func scanPlatformTenant(row pgx.Row) (PlatformTenant, error) {
	var tenant PlatformTenant
	err := row.Scan(
		&tenant.ID, &tenant.Slug, &tenant.Name, &tenant.Status, &tenant.Plan, &tenant.Timezone, &tenant.Locale,
		&tenant.Dominio, &tenant.Creditos,
		&tenant.Limits.MaxCanais, &tenant.Limits.MaxAtendentes, &tenant.Limits.MaxCampanhasAtivas, &tenant.Limits.CampanhaLimiteDiario,
		&tenant.Limits.MaxAgentes, &tenant.Limits.MaxUsuarios, &tenant.Limits.MaxConexoesWhatsApp, &tenant.Limits.MaxTools,
		&tenant.Limits.MaxAPIKeys, &tenant.Limits.MaxWebhooks, &tenant.Limits.CreditosIAMensais, &tenant.Limits.UpdatedAt,
		&tenant.Usage.CanaisAtivos, &tenant.Usage.ConexoesWhatsAppAtivas, &tenant.Usage.AtendentesAtivos,
		&tenant.Usage.UsuariosAtivos, &tenant.Usage.AgentesAtivos, &tenant.Usage.ConversasAbertas,
		&tenant.Usage.APIRequests, &tenant.Usage.AICreditsUsed, &tenant.Usage.StorageBytes,
		&tenant.Usage.PeriodStart, &tenant.Usage.UpdatedAt, &tenant.CreatedAt, &tenant.UpdatedAt,
	)
	return tenant, err
}

func (s *Store) ListPlatformTenants(ctx context.Context) ([]PlatformTenant, error) {
	rows, err := s.pool.Query(ctx, platformTenantSelect+` WHERE t.id <> $1 ORDER BY t.name ASC, t.id ASC`, SystemTenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]PlatformTenant, 0)
	for rows.Next() {
		tenant, err := scanPlatformTenant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, tenant)
	}
	return out, rows.Err()
}

func (s *Store) GetPlatformTenant(ctx context.Context, tenantID int) (PlatformTenant, error) {
	tenant, err := scanPlatformTenant(s.pool.QueryRow(ctx, platformTenantSelect+` WHERE t.id=$1`, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return PlatformTenant{}, ErrPlatformTenantNotFound
	}
	return tenant, err
}

func (s *Store) CreatePlatformTenant(ctx context.Context, input PlatformTenantInput) (PlatformTenant, error) {
	input = normalizedPlatformTenantInput(input)
	if err := validatePlatformTenantInput(input); err != nil {
		return PlatformTenant{}, err
	}

	var tenantID int
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Quando uma empresa assume o domínio que a instalação usou para o
		// tenant de sistema, transfere o mapeamento para a empresa (login por
		// host passa a resolver a empresa, não a plataforma).
		if input.Dominio != "" {
			if _, err := tx.Exec(ctx, `UPDATE tenants SET dominio = '' WHERE id = $1 AND lower(btrim(dominio)) = lower($2)`, SystemTenantID, input.Dominio); err != nil {
				return err
			}
		}
		if err := tx.QueryRow(ctx, `INSERT INTO tenants (slug, name, status, plan, timezone, locale, dominio, creditos)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, input.Slug, input.Name, input.Status, input.Plan, input.Timezone, input.Locale, input.Dominio, input.Creditos).Scan(&tenantID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO tenant_limits (tenant_id) VALUES ($1) ON CONFLICT (tenant_id) DO NOTHING`, tenantID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO tenant_usage_counters (tenant_id) VALUES ($1) ON CONFLICT (tenant_id) DO NOTHING`, tenantID); err != nil {
			return err
		}
		// As funções básicas do painel não são vendidas como módulos opcionais.
		if _, err := tx.Exec(ctx, `INSERT INTO tenant_modules (tenant_id, module_key, enabled)
			SELECT $1, key, key = ANY($2::text[]) FROM modulos
			ON CONFLICT (tenant_id, module_key) DO NOTHING`, tenantID, coreModuleKeys()); err != nil {
			return err
		}
		// Sem settings a empresa não consegue concluir o setup, mesmo antes de
		// configurar um agente ou canal.
		_, err := tx.Exec(ctx, `INSERT INTO settings (tenant_id)
			SELECT $1 WHERE NOT EXISTS (SELECT 1 FROM settings WHERE tenant_id=$1)`, tenantID)
		return err
	})
	if err != nil {
		return PlatformTenant{}, err
	}
	return s.GetPlatformTenant(ctx, tenantID)
}

func (s *Store) UpdatePlatformTenant(ctx context.Context, tenantID int, input PlatformTenantInput) error {
	input = normalizedPlatformTenantInput(input)
	if err := validatePlatformTenantInput(input); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE tenants SET slug=$1, name=$2, status=$3, plan=$4, timezone=$5, locale=$6, dominio=$7, creditos=$8, updated_at=now()
		WHERE id=$9`, input.Slug, input.Name, input.Status, input.Plan, input.Timezone, input.Locale, input.Dominio, input.Creditos, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPlatformTenantNotFound
	}
	return nil
}

func validatePlatformTenantInput(input PlatformTenantInput) error {
	if input.Slug == "" || input.Name == "" || !validSlug(input.Slug) {
		return fmt.Errorf("%w: slug and name are required", ErrInvalidPlatformTenant)
	}
	if input.Status != "trial" && input.Status != "active" && input.Status != "suspended" && input.Status != "closed" {
		return fmt.Errorf("%w: invalid status", ErrInvalidPlatformTenant)
	}
	if strings.TrimSpace(input.Plan) == "" || strings.TrimSpace(input.Locale) == "" {
		return fmt.Errorf("%w: plan and locale are required", ErrInvalidPlatformTenant)
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return fmt.Errorf("%w: invalid timezone", ErrInvalidPlatformTenant)
	}
	if input.Creditos < 0 {
		return fmt.Errorf("%w: creditos cannot be negative", ErrInvalidPlatformTenant)
	}
	if dominio := strings.TrimSpace(input.Dominio); dominio != "" && !validDominio(dominio) {
		return fmt.Errorf("%w: invalid dominio", ErrInvalidPlatformTenant)
	}
	return nil
}

func normalizedPlatformTenantInput(input PlatformTenantInput) PlatformTenantInput {
	input.Slug = strings.TrimSpace(input.Slug)
	input.Name = strings.TrimSpace(input.Name)
	input.Status = strings.TrimSpace(input.Status)
	input.Plan = strings.TrimSpace(input.Plan)
	input.Timezone = strings.TrimSpace(input.Timezone)
	input.Locale = strings.TrimSpace(input.Locale)
	input.Dominio = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(input.Dominio), "https://"), "http://"))
	return input
}

func validSlug(slug string) bool {
	if len(slug) > 80 {
		return false
	}
	for _, r := range slug {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

func validDominio(dominio string) bool {
	if len(dominio) > 255 {
		return false
	}
	d := strings.TrimSuffix(strings.TrimSpace(dominio), ".")
	if d == "" || !strings.Contains(d, ".") {
		return false
	}
	for _, r := range d {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '.') {
			return false
		}
	}
	return !strings.HasPrefix(d, ".") && !strings.HasSuffix(d, ".")
}

func coreModuleKeys() []string {
	return []string{"conversas", "canais", "equipe", "config"}
}

func isCoreModule(key string) bool {
	for _, core := range coreModuleKeys() {
		if key == core {
			return true
		}
	}
	return false
}

func (s *Store) GetPlatformTenantLimits(ctx context.Context, tenantID int) (PlatformTenantLimits, error) {
	var limits PlatformTenantLimits
	err := s.pool.QueryRow(ctx, `SELECT max_canais, max_atendentes, max_campanhas_ativas, campanha_limite_diario,
		max_agentes, max_usuarios, max_conexoes_whatsapp, max_tools, max_api_keys, max_webhooks,
		creditos_ia_mensais, updated_at FROM tenant_limits WHERE tenant_id=$1`, tenantID).Scan(
		&limits.MaxCanais, &limits.MaxAtendentes, &limits.MaxCampanhasAtivas, &limits.CampanhaLimiteDiario,
		&limits.MaxAgentes, &limits.MaxUsuarios, &limits.MaxConexoesWhatsApp, &limits.MaxTools,
		&limits.MaxAPIKeys, &limits.MaxWebhooks, &limits.CreditosIAMensais, &limits.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlatformTenantLimits{}, ErrPlatformTenantNotFound
	}
	return limits, err
}

func (s *Store) UpdatePlatformTenantLimits(ctx context.Context, tenantID int, limits PlatformTenantLimits) error {
	if limits.MaxCanais < 0 || limits.MaxAtendentes < 0 || limits.MaxCampanhasAtivas < 0 || limits.CampanhaLimiteDiario < 0 ||
		limits.MaxAgentes < 0 || limits.MaxUsuarios < 0 || limits.MaxConexoesWhatsApp < 0 || limits.MaxTools < 0 ||
		limits.MaxAPIKeys < 0 || limits.MaxWebhooks < 0 || limits.CreditosIAMensais < 0 {
		return fmt.Errorf("%w: limits cannot be negative", ErrInvalidPlatformTenant)
	}
	tag, err := s.pool.Exec(ctx, `UPDATE tenant_limits SET
		max_canais=$1, max_atendentes=$2, max_campanhas_ativas=$3, campanha_limite_diario=$4,
		max_agentes=$5, max_usuarios=$6, max_conexoes_whatsapp=$7, max_tools=$8, max_api_keys=$9,
		max_webhooks=$10, creditos_ia_mensais=$11, updated_at=now()
		WHERE tenant_id=$12`,
		limits.MaxCanais, limits.MaxAtendentes, limits.MaxCampanhasAtivas, limits.CampanhaLimiteDiario,
		limits.MaxAgentes, limits.MaxUsuarios, limits.MaxConexoesWhatsApp, limits.MaxTools, limits.MaxAPIKeys,
		limits.MaxWebhooks, limits.CreditosIAMensais, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPlatformTenantNotFound
	}
	return nil
}

func (s *Store) ListPlatformTenantModules(ctx context.Context, tenantID int) ([]Module, error) {
	if err := s.ensurePlatformTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT m.key, m.name, m.description, COALESCE(tm.enabled, false), m.sort
		FROM modulos m
		LEFT JOIN tenant_modules tm ON tm.module_key=m.key AND tm.tenant_id=$1
		ORDER BY m.sort ASC, m.name ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	modules := make([]Module, 0)
	for rows.Next() {
		var module Module
		if err := rows.Scan(&module.Key, &module.Name, &module.Description, &module.Enabled, &module.Sort); err != nil {
			return nil, err
		}
		modules = append(modules, module)
	}
	return modules, rows.Err()
}

func (s *Store) ensurePlatformTenant(ctx context.Context, tenantID int) error {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenants WHERE id=$1)`, tenantID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrPlatformTenantNotFound
	}
	return nil
}

func (s *Store) SetPlatformTenantModule(ctx context.Context, tenantID int, key string, enabled bool) error {
	key = strings.TrimSpace(key)
	if isCoreModule(key) && !enabled {
		return ErrCoreModuleCannotBeDisabled
	}
	tag, err := s.pool.Exec(ctx, `INSERT INTO tenant_modules (tenant_id, module_key, enabled)
		SELECT $1, $2, $3
		WHERE EXISTS (SELECT 1 FROM tenants WHERE id=$1) AND EXISTS (SELECT 1 FROM modulos WHERE key=$2)
		ON CONFLICT (tenant_id, module_key) DO UPDATE SET enabled=EXCLUDED.enabled, updated_at=now()`, tenantID, key, enabled)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenants WHERE id=$1)`, tenantID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrPlatformTenantNotFound
		}
		return ErrPlatformModuleNotFound
	}
	return nil
}

func (s *Store) GetPlatformTenantUsage(ctx context.Context, tenantID int) (TenantUsage, error) {
	tenant, err := s.GetPlatformTenant(ctx, tenantID)
	if err != nil {
		return TenantUsage{}, err
	}
	return tenant.Usage, nil
}

// AddPlatformTenantUsage atualiza somente os contadores que não podem ser
// calculados por uma consulta simples. Contadores negativos são rejeitados.
func (s *Store) AddPlatformTenantUsage(ctx context.Context, tenantID int, delta TenantUsageDelta) error {
	if delta.APIRequests < 0 || delta.AICreditsUsed < 0 || delta.StorageBytes < 0 {
		return fmt.Errorf("%w: usage delta cannot be negative", ErrInvalidPlatformTenant)
	}
	tag, err := s.pool.Exec(ctx, `INSERT INTO tenant_usage_counters
		(tenant_id, api_requests, ai_credits_used, storage_bytes)
		SELECT $1, $2, $3, $4 WHERE EXISTS (SELECT 1 FROM tenants WHERE id=$1)
		ON CONFLICT (tenant_id) DO UPDATE SET
			api_requests=tenant_usage_counters.api_requests + EXCLUDED.api_requests,
			ai_credits_used=tenant_usage_counters.ai_credits_used + EXCLUDED.ai_credits_used,
			storage_bytes=tenant_usage_counters.storage_bytes + EXCLUDED.storage_bytes,
			updated_at=now()`, tenantID, delta.APIRequests, delta.AICreditsUsed, delta.StorageBytes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPlatformTenantNotFound
	}
	return nil
}

const whatsAppServerSelect = `
	SELECT ws.id, ws.name, ws.base_url, ws.status, ws.active, ws.max_connections,
		COALESCE(load.active_connections, 0),
		CASE WHEN ws.max_connections = 0 THEN 0
			ELSE GREATEST(ws.max_connections - COALESCE(load.active_connections, 0), 0) END,
		(ws.token <> ''), ws.created_at, ws.updated_at
	FROM whatsapp_servers ws
	LEFT JOIN LATERAL (
		SELECT COUNT(*)::int AS active_connections
		FROM channel_whatsapp_server_allocations a
		JOIN channels c ON c.id=a.channel_id AND c.tenant_id=a.tenant_id
		WHERE a.server_id=ws.id AND c.type='whatsapp' AND c.active
	) load ON TRUE`

func scanWhatsAppServer(row pgx.Row) (WhatsAppServer, error) {
	var server WhatsAppServer
	err := row.Scan(&server.ID, &server.Name, &server.BaseURL, &server.Status, &server.Active, &server.MaxConnections,
		&server.ActiveConnections, &server.AvailableConnections, &server.TokenConfigured, &server.CreatedAt, &server.UpdatedAt)
	return server, err
}

func (s *Store) ListWhatsAppServers(ctx context.Context) ([]WhatsAppServer, error) {
	rows, err := s.pool.Query(ctx, whatsAppServerSelect+` ORDER BY CASE ws.status WHEN 'active' THEN 0 WHEN 'maintenance' THEN 1 ELSE 2 END, ws.name ASC, ws.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	servers := make([]WhatsAppServer, 0)
	for rows.Next() {
		server, err := scanWhatsAppServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, rows.Err()
}

func (s *Store) CreateWhatsAppServer(ctx context.Context, input WhatsAppServerInput) (WhatsAppServer, error) {
	if err := validateWhatsAppServerInput(input, true); err != nil {
		return WhatsAppServer{}, err
	}
	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO whatsapp_servers (name, base_url, token, status, active, max_connections)
		VALUES ($1,$2,$3,$4,$4='active',$5) RETURNING id`, strings.TrimSpace(input.Name), strings.TrimSpace(input.BaseURL), input.Token, input.Status, input.MaxConnections).Scan(&id)
	if err != nil {
		return WhatsAppServer{}, err
	}
	return s.getWhatsAppServer(ctx, id)
}

func (s *Store) UpdateWhatsAppServer(ctx context.Context, serverID int64, input WhatsAppServerInput) error {
	if err := validateWhatsAppServerInput(input, false); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE whatsapp_servers SET name=$1, base_url=$2,
		token=CASE WHEN $3 <> '' THEN $3 ELSE token END,
		status=$4, active=($4='active'), max_connections=$5, updated_at=now() WHERE id=$6`,
		strings.TrimSpace(input.Name), strings.TrimSpace(input.BaseURL), input.Token, input.Status, input.MaxConnections, serverID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidWhatsAppServer
	}
	return nil
}

func validateWhatsAppServerInput(input WhatsAppServerInput, requireToken bool) error {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.BaseURL) == "" || input.MaxConnections < 0 {
		return fmt.Errorf("%w: name, base url and non-negative capacity are required", ErrInvalidWhatsAppServer)
	}
	if input.Status != "active" && input.Status != "maintenance" && input.Status != "disabled" {
		return fmt.Errorf("%w: invalid status", ErrInvalidWhatsAppServer)
	}
	if requireToken && strings.TrimSpace(input.Token) == "" {
		return fmt.Errorf("%w: token is required when creating a server", ErrInvalidWhatsAppServer)
	}
	return nil
}

func (s *Store) getWhatsAppServer(ctx context.Context, serverID int64) (WhatsAppServer, error) {
	server, err := scanWhatsAppServer(s.pool.QueryRow(ctx, whatsAppServerSelect+` WHERE ws.id=$1`, serverID))
	if errors.Is(err, pgx.ErrNoRows) {
		return WhatsAppServer{}, ErrInvalidWhatsAppServer
	}
	return server, err
}

// AllocateWhatsAppServer escolhe o servidor ativo de menor ocupação. Uma trava
// transacional global do pool elimina a corrida quando vários canais são criados
// ao mesmo tempo e garante que a capacidade não seja ultrapassada.
func (s *Store) AllocateWhatsAppServer(ctx context.Context, tenantID, channelID int) (WhatsAppServer, error) {
	var server WhatsAppServer
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := lockWhatsAppPool(ctx, tx); err != nil {
			return err
		}
		if err := validatePooledWhatsAppChannel(ctx, tx, tenantID, channelID); err != nil {
			return err
		}

		// Uma alocação válida e ativa é estável; não troca servidor sem motivo.
		allocated, err := scanWhatsAppServer(tx.QueryRow(ctx, whatsAppServerSelect+`
			JOIN channel_whatsapp_server_allocations a ON a.server_id=ws.id
			WHERE a.tenant_id=$1 AND a.channel_id=$2
				AND ws.status IN ('active', 'maintenance') AND ws.token <> ''`, tenantID, channelID))
		if err == nil {
			server = allocated
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := ensureTenantWhatsAppCapacity(ctx, tx, tenantID); err != nil {
			return err
		}

		selected, err := leastLoadedWhatsAppServer(ctx, tx)
		if err != nil {
			return err
		}
		if err := upsertWhatsAppServerAllocation(ctx, tx, tenantID, channelID, selected.ID); err != nil {
			return err
		}
		selected.ActiveConnections++
		selected.AvailableConnections = max(selected.MaxConnections-selected.ActiveConnections, 0)
		server = selected
		return nil
	})
	return server, err
}

// AssignWhatsAppServer permite ao super_admin fixar uma conexão em um servidor
// específico, respeitando a mesma capacidade e a mesma trava do alocador.
func (s *Store) AssignWhatsAppServer(ctx context.Context, tenantID, channelID int, serverID int64) (WhatsAppServer, error) {
	var server WhatsAppServer
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := lockWhatsAppPool(ctx, tx); err != nil {
			return err
		}
		if err := validatePooledWhatsAppChannel(ctx, tx, tenantID, channelID); err != nil {
			return err
		}
		if err := ensureTenantWhatsAppCapacity(ctx, tx, tenantID); err != nil {
			return err
		}
		selected, err := specificWhatsAppServerWithCapacity(ctx, tx, channelID, serverID)
		if err != nil {
			return err
		}
		if err := upsertWhatsAppServerAllocation(ctx, tx, tenantID, channelID, selected.ID); err != nil {
			return err
		}
		server = selected
		return nil
	})
	return server, err
}

func (s *Store) WhatsAppServerForChannel(ctx context.Context, tenantID, channelID int) (WhatsAppServer, error) {
	server, err := scanWhatsAppServer(s.pool.QueryRow(ctx, whatsAppServerSelect+`
		JOIN channel_whatsapp_server_allocations a ON a.server_id=ws.id
		WHERE a.tenant_id=$1 AND a.channel_id=$2`, tenantID, channelID))
	if errors.Is(err, pgx.ErrNoRows) {
		return WhatsAppServer{}, ErrWhatsAppServerUnavailable
	}
	return server, err
}

// WhatsAppServerCredentialsForChannel é para o worker/cliente HTTP interno.
// O token jamais atravessa tipos usados pelas listagens do control plane.
func (s *Store) WhatsAppServerCredentialsForChannel(ctx context.Context, tenantID, channelID int) (WhatsAppServerCredentials, error) {
	var credentials WhatsAppServerCredentials
	err := s.pool.QueryRow(ctx, `SELECT ws.id, ws.base_url, ws.token
		FROM channel_whatsapp_server_allocations a
		JOIN whatsapp_servers ws ON ws.id=a.server_id
		WHERE a.tenant_id=$1 AND a.channel_id=$2
			AND ws.status IN ('active', 'maintenance') AND ws.token <> ''`, tenantID, channelID).
		Scan(&credentials.ServerID, &credentials.BaseURL, &credentials.Token)
	if errors.Is(err, pgx.ErrNoRows) {
		return WhatsAppServerCredentials{}, ErrWhatsAppServerUnavailable
	}
	return credentials, err
}

func lockWhatsAppPool(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('notreve:whatsapp-server-pool', 0))`)
	return err
}

func validatePooledWhatsAppChannel(ctx context.Context, tx pgx.Tx, tenantID, channelID int) error {
	var channelType string
	var active bool
	err := tx.QueryRow(ctx, `SELECT type, active FROM channels WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, channelID).Scan(&channelType, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrWhatsAppChannelRequired
	}
	if err != nil {
		return err
	}
	if channelType != "whatsapp" || !active {
		return ErrWhatsAppChannelRequired
	}
	return nil
}

// A criação do canal ocorre antes da alocação; por isso a contagem já inclui
// o canal que está sendo associado e o limite é ultrapassado apenas se count>
// max. Alocações existentes permanecem estáveis mesmo depois de uma redução
// de plano, para não derrubar conexões já ativas.
func ensureTenantWhatsAppCapacity(ctx context.Context, tx pgx.Tx, tenantID int) error {
	var maximum, active int
	err := tx.QueryRow(ctx, `SELECT l.max_conexoes_whatsapp,
		(SELECT COUNT(*) FROM channels c WHERE c.tenant_id=l.tenant_id AND c.type='whatsapp' AND c.active)
		FROM tenant_limits l WHERE l.tenant_id=$1 FOR UPDATE`, tenantID).Scan(&maximum, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPlatformTenantNotFound
	}
	if err != nil {
		return err
	}
	if maximum > 0 && active > maximum {
		return ErrTenantWhatsAppLimitReached
	}
	return nil
}

func leastLoadedWhatsAppServer(ctx context.Context, tx pgx.Tx) (WhatsAppServer, error) {
	server, err := scanWhatsAppServer(tx.QueryRow(ctx, whatsAppServerSelect+`
		WHERE ws.status = 'active' AND ws.token <> ''
		AND (ws.max_connections = 0 OR COALESCE(load.active_connections, 0) < ws.max_connections)
		ORDER BY CASE WHEN ws.max_connections = 0 THEN 1
			ELSE (COALESCE(load.active_connections, 0)::numeric / ws.max_connections) END ASC,
			COALESCE(load.active_connections, 0) ASC, ws.id ASC
		LIMIT 1 FOR UPDATE OF ws`))
	if errors.Is(err, pgx.ErrNoRows) {
		return WhatsAppServer{}, ErrWhatsAppServerUnavailable
	}
	return server, err
}

func specificWhatsAppServerWithCapacity(ctx context.Context, tx pgx.Tx, channelID int, serverID int64) (WhatsAppServer, error) {
	server, err := scanWhatsAppServer(tx.QueryRow(ctx, whatsAppServerSelect+`
		WHERE ws.id=$1 AND ws.status = 'active' AND ws.token <> ''
		AND (
			ws.max_connections = 0 OR COALESCE(load.active_connections, 0) < ws.max_connections
			OR EXISTS (SELECT 1 FROM channel_whatsapp_server_allocations a WHERE a.channel_id=$2 AND a.server_id=ws.id)
		)
		FOR UPDATE OF ws`, serverID, channelID))
	if errors.Is(err, pgx.ErrNoRows) {
		return WhatsAppServer{}, ErrWhatsAppServerUnavailable
	}
	return server, err
}

func upsertWhatsAppServerAllocation(ctx context.Context, tx pgx.Tx, tenantID, channelID int, serverID int64) error {
	_, err := tx.Exec(ctx, `INSERT INTO channel_whatsapp_server_allocations (channel_id, tenant_id, server_id)
		VALUES ($1,$2,$3)
		ON CONFLICT (channel_id) DO UPDATE SET tenant_id=EXCLUDED.tenant_id, server_id=EXCLUDED.server_id, updated_at=now()`, channelID, tenantID, serverID)
	return err
}

// ContextSwitchAudit registra a assunção auditada de contexto pelo super_admin.
type ContextSwitchAudit struct {
	ID           int64     `json:"id"`
	ActorID      int       `json:"actor_id"`
	ActorName    string    `json:"actor_name"`
	FromTenantID int       `json:"from_tenant_id"`
	FromSlug     string    `json:"from_slug"`
	ToTenantID   int       `json:"to_tenant_id"`
	ToSlug       string    `json:"to_slug"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
}

// RecordContextSwitch persiste quem, de qual empresa para qual, quando e por quê
// um super_admin assumiu o contexto de uma empresa. O tenant de destino precisa
// existir; a auditoria é imutável e serve para rastrear acessos ao control plane.
func (s *Store) RecordContextSwitch(ctx context.Context, actor Usuario, fromTenantID, toTenantID int, reason string) (ContextSwitchAudit, error) {
	reason = strings.TrimSpace(reason)
	if toTenantID < 1 || actor.ID < 1 {
		return ContextSwitchAudit{}, ErrInvalidContextSwitch
	}
	if len([]rune(reason)) > 500 {
		return ContextSwitchAudit{}, fmt.Errorf("%w: reason too long", ErrInvalidContextSwitch)
	}
	fromSlug, toSlug, err := s.tenantAndFromSlugs(ctx, fromTenantID, toTenantID)
	if err != nil {
		return ContextSwitchAudit{}, err
	}
	from := fromTenantID
	if fromTenantID < 1 {
		from = toTenantID
	}
	audit := ContextSwitchAudit{
		ActorID: actor.ID, ActorName: actor.Name,
		FromTenantID: from, FromSlug: fromSlug,
		ToTenantID: toTenantID, ToSlug: toSlug, Reason: reason,
	}
	err = s.pool.QueryRow(ctx, `INSERT INTO audit_context_switch
		(actor_id, actor_name, from_tenant_id, from_slug, to_tenant_id, to_slug, reason)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, created_at`,
		audit.ActorID, audit.ActorName, audit.FromTenantID, audit.FromSlug, audit.ToTenantID, audit.ToSlug, audit.Reason).
		Scan(&audit.ID, &audit.CreatedAt)
	return audit, err
}

// ListContextSwitchAudit devolve o histórico recente de trocas de contexto.
func (s *Store) ListContextSwitchAudit(ctx context.Context, limit int) ([]ContextSwitchAudit, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `SELECT id, actor_id, actor_name,
		COALESCE(from_tenant_id, 0), from_slug, COALESCE(to_tenant_id, 0), to_slug, reason, created_at
		FROM audit_context_switch ORDER BY created_at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ContextSwitchAudit, 0)
	for rows.Next() {
		var a ContextSwitchAudit
		if err := rows.Scan(&a.ID, &a.ActorID, &a.ActorName, &a.FromTenantID, &a.FromSlug,
			&a.ToTenantID, &a.ToSlug, &a.Reason, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (s *Store) tenantAndFromSlugs(ctx context.Context, fromTenantID, toTenantID int) (string, string, error) {
	var toSlug string
	if err := s.pool.QueryRow(ctx, `SELECT slug FROM tenants WHERE id=$1`, toTenantID).Scan(&toSlug); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrPlatformTenantNotFound
		}
		return "", "", err
	}
	if fromTenantID < 1 {
		return toSlug, toSlug, nil
	}
	var fromSlug string
	if err := s.pool.QueryRow(ctx, `SELECT slug FROM tenants WHERE id=$1`, fromTenantID).Scan(&fromSlug); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return toSlug, toSlug, nil
		}
		return "", "", err
	}
	return fromSlug, toSlug, nil
}
