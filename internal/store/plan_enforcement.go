package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Erros sentinela para que handlers e jobs possam devolver uma resposta
// consistente sem expor contagens, configurações ou dados de outro tenant.
var (
	ErrTenantPlanLimitsUnavailable = errors.New("tenant plan limits unavailable")
	ErrInvalidTenantPlanLimit      = errors.New("invalid tenant plan limit")
	ErrInvalidAICreditAmount       = errors.New("invalid ai credit amount")
	ErrTenantAgentLimitReached     = errors.New("tenant active agent limit reached")
	ErrTenantUserLimitReached      = errors.New("tenant active user limit reached")
	ErrTenantToolLimitReached      = errors.New("tenant tool limit reached")
	ErrTenantAPIKeyLimitReached    = errors.New("tenant active api key limit reached")
	ErrTenantWebhookLimitReached   = errors.New("tenant active webhook limit reached")
	ErrTenantAICreditLimitReached  = errors.New("tenant ai credit limit reached")
	ErrPlanNotFound                = errors.New("plan not found")
	ErrPlanInUse                   = errors.New("plan is assigned to a tenant")
	ErrInvalidPlan                 = errors.New("invalid plan")
)

// tenantPlanLimitUsage é deliberadamente privado: ele contém apenas o que é
// necessário para decidir uma criação. Nenhuma entidade ou identificador de
// outro tenant é retornado ao chamador.
type tenantPlanLimitUsage struct {
	MaxAgents   int
	MaxUsers    int
	MaxTools    int
	MaxAPIKeys  int
	MaxWebhooks int
	AICredits   int64

	ActiveAgents   int
	ActiveUsers    int
	Tools          int
	ActiveAPIKeys  int
	ActiveWebhooks int
	AICreditsUsed  int64
}

const tenantPlanLimitUsageQuery = `
	SELECT l.max_agentes, l.max_usuarios, l.max_tools, l.max_api_keys, l.max_webhooks,
		l.creditos_ia_mensais,
		(SELECT COUNT(*) FROM agents a WHERE a.tenant_id=l.tenant_id AND a.active),
		(SELECT COUNT(*) FROM usuarios u WHERE u.tenant_id=l.tenant_id AND u.active),
		(SELECT COUNT(*) FROM integrations i WHERE i.tenant_id=l.tenant_id),
		(SELECT COUNT(*) FROM public_api_keys k
			WHERE k.tenant_id=l.tenant_id AND k.active AND k.revoked_at IS NULL),
		(SELECT COUNT(*) FROM webhook_subscriptions w WHERE w.tenant_id=l.tenant_id AND w.active),
		COALESCE(usage.ai_credits_used, 0)
	FROM tenant_limits l
	LEFT JOIN tenant_usage_counters usage ON usage.tenant_id=l.tenant_id
	WHERE l.tenant_id=$1`

func (s *Store) tenantPlanLimitUsage(ctx context.Context) (tenantPlanLimitUsage, error) {
	tenantID, err := tenantIDForPlanEnforcement(ctx)
	if err != nil {
		return tenantPlanLimitUsage{}, err
	}

	var usage tenantPlanLimitUsage
	err = s.pool.QueryRow(ctx, tenantPlanLimitUsageQuery, tenantID).Scan(
		&usage.MaxAgents,
		&usage.MaxUsers,
		&usage.MaxTools,
		&usage.MaxAPIKeys,
		&usage.MaxWebhooks,
		&usage.AICredits,
		&usage.ActiveAgents,
		&usage.ActiveUsers,
		&usage.Tools,
		&usage.ActiveAPIKeys,
		&usage.ActiveWebhooks,
		&usage.AICreditsUsed,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tenantPlanLimitUsage{}, ErrTenantPlanLimitsUnavailable
		}
		return tenantPlanLimitUsage{}, err
	}
	effective, err := s.resolveEffectiveResourceMap(ctx, tenantID)
	if err != nil {
		return tenantPlanLimitUsage{}, err
	}
	usage.MaxAgents = int(effective[ResourceMaxAgentes])
	usage.MaxUsers = int(effective[ResourceMaxUsuarios])
	usage.MaxTools = int(effective[ResourceMaxTools])
	usage.MaxAPIKeys = int(effective[ResourceMaxAPIKeys])
	usage.MaxWebhooks = int(effective[ResourceMaxWebhooks])
	usage.AICredits = int64(effective[ResourceCreditosIAMensais])
	return usage, nil
}

// tenantIDForPlanEnforcement não aceita o fallback do tenant legado. Uma
// operação de cobrança/criação sem contexto é um erro de isolamento, não uma
// ação válida para a empresa 1.
func tenantIDForPlanEnforcement(ctx context.Context) (int, error) {
	return TenantIDRequired(ctx)
}

// EnsureCanCreateActiveAgent verifica o limite antes de ativar um agente.
func (s *Store) EnsureCanCreateActiveAgent(ctx context.Context) error {
	usage, err := s.tenantPlanLimitUsage(ctx)
	if err != nil {
		return err
	}
	return enforceTenantResourceLimit(usage.ActiveAgents, usage.MaxAgents, ErrTenantAgentLimitReached)
}

// EnsureCanCreateActiveUser verifica o limite antes de ativar um usuário.
func (s *Store) EnsureCanCreateActiveUser(ctx context.Context) error {
	usage, err := s.tenantPlanLimitUsage(ctx)
	if err != nil {
		return err
	}
	return enforceTenantResourceLimit(usage.ActiveUsers, usage.MaxUsers, ErrTenantUserLimitReached)
}

// EnsureCanCreateIntegration verifica o limite de tools/integrations. A tabela
// ainda não possui estado ativo, então toda integração criada consome uma vaga.
func (s *Store) EnsureCanCreateIntegration(ctx context.Context) error {
	usage, err := s.tenantPlanLimitUsage(ctx)
	if err != nil {
		return err
	}
	return enforceTenantResourceLimit(usage.Tools, usage.MaxTools, ErrTenantToolLimitReached)
}

// EnsureCanCreateTool é o nome orientado ao produto para a mesma verificação
// de integrations usada internamente pelo banco.
func (s *Store) EnsureCanCreateTool(ctx context.Context) error {
	return s.EnsureCanCreateIntegration(ctx)
}

// EnsureCanCreateAPIKey verifica apenas chaves ainda utilizáveis pelo tenant.
func (s *Store) EnsureCanCreateAPIKey(ctx context.Context) error {
	usage, err := s.tenantPlanLimitUsage(ctx)
	if err != nil {
		return err
	}
	return enforceTenantResourceLimit(usage.ActiveAPIKeys, usage.MaxAPIKeys, ErrTenantAPIKeyLimitReached)
}

// EnsureCanCreateWebhookSubscription verifica inscrições de webhook ativas.
func (s *Store) EnsureCanCreateWebhookSubscription(ctx context.Context) error {
	usage, err := s.tenantPlanLimitUsage(ctx)
	if err != nil {
		return err
	}
	return enforceTenantResourceLimit(usage.ActiveWebhooks, usage.MaxWebhooks, ErrTenantWebhookLimitReached)
}

// EnsureCanConsumeAICredits verifica se a franquia mensal suporta o consumo.
// Zero de créditos significa que o plano não possui franquia Notreve IA; uso
// de provedor próprio não deve chamar este método.
func (s *Store) EnsureCanConsumeAICredits(ctx context.Context, credits int64) error {
	if credits < 0 {
		return ErrInvalidAICreditAmount
	}
	if credits == 0 {
		return nil
	}

	usage, err := s.tenantPlanLimitUsage(ctx)
	if err != nil {
		return err
	}
	return enforceTenantAICreditLimit(usage.AICreditsUsed, usage.AICredits, credits)
}

func enforceTenantResourceLimit(active, maximum int, limitErr error) error {
	if active < 0 || maximum < 0 {
		return ErrInvalidTenantPlanLimit
	}
	// Em todos os limites de recursos do painel, zero significa ilimitado.
	// Créditos de IA são tratados separadamente porque zero representa a
	// ausência de franquia Notreve IA.
	if maximum == 0 {
		return nil
	}
	if active >= maximum {
		return limitErr
	}
	return nil
}

func enforceTenantAICreditLimit(used, maximum, requested int64) error {
	if used < 0 || maximum < 0 {
		return ErrInvalidTenantPlanLimit
	}
	if requested < 0 {
		return ErrInvalidAICreditAmount
	}
	if used > maximum || requested > maximum-used {
		return ErrTenantAICreditLimitReached
	}
	return nil
}

const (
	ResourceMaxMensagensDia      = "max_mensagens_dia"
	ResourceMaxCanais            = "max_canais"
	ResourceMaxAtendentes        = "max_atendentes"
	ResourceMaxCampanhasAtivas   = "max_campanhas_ativas"
	ResourceCampanhaLimiteDiario = "campanha_limite_diario"
	ResourceMaxAgentes           = "max_agentes"
	ResourceMaxUsuarios          = "max_usuarios"
	ResourceMaxConexoesWhatsApp  = "max_conexoes_whatsapp"
	ResourceMaxTools             = "max_tools"
	ResourceMaxAPIKeys           = "max_api_keys"
	ResourceMaxWebhooks          = "max_webhooks"
	ResourceCreditosIAMensais    = "creditos_ia_mensais"
)

func defaultResourceMap() map[string]float64 {
	return map[string]float64{
		ResourceMaxMensagensDia:      0,
		ResourceMaxCanais:            3,
		ResourceMaxAtendentes:        10,
		ResourceMaxCampanhasAtivas:   2,
		ResourceCampanhaLimiteDiario: 100,
		ResourceMaxAgentes:           3,
		ResourceMaxUsuarios:          10,
		ResourceMaxConexoesWhatsApp:  3,
		ResourceMaxTools:             10,
		ResourceMaxAPIKeys:           3,
		ResourceMaxWebhooks:          10,
		ResourceCreditosIAMensais:    0,
	}
}

func (s *Store) tenantDefaultsResourceMap(ctx context.Context, tenantID int) (map[string]float64, error) {
	var maxCanais, maxAtendentes, maxCampanhas, campanhaDiario, maxAgentes, maxUsuarios, maxConexoes, maxTools, maxAPIKeys, maxWebhooks float64
	var creditos float64
	err := s.pool.QueryRow(ctx, `SELECT max_canais, max_atendentes, max_campanhas_ativas, campanha_limite_diario,
		max_agentes, max_usuarios, max_conexoes_whatsapp, max_tools, max_api_keys, max_webhooks,
		creditos_ia_mensais FROM tenant_limits WHERE tenant_id=$1`, tenantID).
		Scan(&maxCanais, &maxAtendentes, &maxCampanhas, &campanhaDiario, &maxAgentes, &maxUsuarios,
			&maxConexoes, &maxTools, &maxAPIKeys, &maxWebhooks, &creditos)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultResourceMap(), nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]float64{
		ResourceMaxMensagensDia:      0,
		ResourceMaxCanais:            maxCanais,
		ResourceMaxAtendentes:        maxAtendentes,
		ResourceMaxCampanhasAtivas:   maxCampanhas,
		ResourceCampanhaLimiteDiario: campanhaDiario,
		ResourceMaxAgentes:           maxAgentes,
		ResourceMaxUsuarios:          maxUsuarios,
		ResourceMaxConexoesWhatsApp:  maxConexoes,
		ResourceMaxTools:             maxTools,
		ResourceMaxAPIKeys:           maxAPIKeys,
		ResourceMaxWebhooks:          maxWebhooks,
		ResourceCreditosIAMensais:    creditos,
	}, nil
}

func (s *Store) resolveEffectiveResourceMap(ctx context.Context, tenantID int) (map[string]float64, error) {
	resources, err := s.tenantDefaultsResourceMap(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	var planoID int
	var overrideJSON []byte
	err = s.pool.QueryRow(ctx, `SELECT COALESCE(plano_id, 0), COALESCE(limits_override, '{}'::jsonb) FROM tenants WHERE id=$1`, tenantID).
		Scan(&planoID, &overrideJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTenantPlanLimitsUnavailable
		}
		return nil, err
	}
	if planoID > 0 {
		rows, qerr := s.pool.Query(ctx, `SELECT recurso, limite FROM plano_recursos WHERE plano_id=$1`, planoID)
		if qerr != nil {
			return nil, qerr
		}
		for rows.Next() {
			var recurso string
			var limite float64
			if err := rows.Scan(&recurso, &limite); err != nil {
				rows.Close()
				return nil, err
			}
			resources[recurso] = limite
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	if len(overrideJSON) > 0 {
		var overrides map[string]float64
		if err := json.Unmarshal(overrideJSON, &overrides); err != nil {
			return nil, fmt.Errorf("%w: override inválido", ErrInvalidTenantPlanLimit)
		}
		for key, value := range overrides {
			if value >= 0 {
				resources[key] = value
			}
		}
	}
	return resources, nil
}

func (s *Store) EffectiveTenantLimits(ctx context.Context) (EffectiveLimits, error) {
	tenantID, err := tenantIDForPlanEnforcement(ctx)
	if err != nil {
		return EffectiveLimits{}, err
	}
	resources, err := s.resolveEffectiveResourceMap(ctx, tenantID)
	if err != nil {
		return EffectiveLimits{}, err
	}
	var out EffectiveLimits
	out.TenantID = tenantID
	out.Recursos = resources
	err = s.pool.QueryRow(ctx, `SELECT p.id, p.nome, t.status, t.trial_until
		FROM tenants t LEFT JOIN planos p ON p.id=t.plano_id WHERE t.id=$1`, tenantID).
		Scan(&out.PlanoID, &out.PlanoNome, &out.Status, &out.TrialUntil)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return EffectiveLimits{}, err
	}
	out.Uso = s.computeTenantUsageMap(ctx, tenantID)
	return out, nil
}

func (s *Store) computeTenantUsageMap(ctx context.Context, tenantID int) map[string]float64 {
	usage := map[string]float64{
		"agentes": 0, "usuarios": 0, "tools": 0, "api_keys": 0, "webhooks": 0,
		"ai_credits_used": 0, "api_requests": 0, "storage_bytes": 0,
	}
	var agentes, usuarios, tools, apiKeys, webhooks float64
	var aiCredits, apiRequests, storage float64
	err := s.pool.QueryRow(ctx, `SELECT
		(SELECT COUNT(*) FROM agents a WHERE a.tenant_id=$1 AND a.active),
		(SELECT COUNT(*) FROM usuarios u WHERE u.tenant_id=$1 AND u.active),
		(SELECT COUNT(*) FROM integrations i WHERE i.tenant_id=$1),
		(SELECT COUNT(*) FROM public_api_keys k WHERE k.tenant_id=$1 AND k.active AND k.revoked_at IS NULL),
		(SELECT COUNT(*) FROM webhook_subscriptions w WHERE w.tenant_id=$1 AND w.active),
		COALESCE((SELECT c.ai_credits_used FROM tenant_usage_counters c WHERE c.tenant_id=$1), 0),
		COALESCE((SELECT c.api_requests FROM tenant_usage_counters c WHERE c.tenant_id=$1), 0),
		COALESCE((SELECT c.storage_bytes FROM tenant_usage_counters c WHERE c.tenant_id=$1), 0)`, tenantID).
		Scan(&agentes, &usuarios, &tools, &apiKeys, &webhooks, &aiCredits, &apiRequests, &storage)
	if err != nil {
		return usage
	}
	usage["agentes"] = agentes
	usage["usuarios"] = usuarios
	usage["tools"] = tools
	usage["api_keys"] = apiKeys
	usage["webhooks"] = webhooks
	usage["ai_credits_used"] = aiCredits
	usage["api_requests"] = apiRequests
	usage["storage_bytes"] = storage
	return usage
}

const planSelect = `SELECT p.id, p.tenant_owner, p.nome, p.preco, p.intervalo, p.ativo, p.created_at, p.updated_at
	FROM planos p`

func scanPlano(row pgx.Row) (Plano, error) {
	var plano Plano
	err := row.Scan(&plano.ID, &plano.TenantOwner, &plano.Nome, &plano.Preco, &plano.Intervalo, &plano.Ativo, &plano.CreatedAt, &plano.UpdatedAt)
	return plano, err
}

func (s *Store) planRecursos(ctx context.Context, planoID int) ([]PlanoRecurso, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, plano_id, recurso, limite FROM plano_recursos WHERE plano_id=$1 ORDER BY recurso ASC`, planoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PlanoRecurso, 0)
	for rows.Next() {
		var r PlanoRecurso
		if err := rows.Scan(&r.ID, &r.PlanoID, &r.Recurso, &r.Limite); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ListPlans(ctx context.Context) ([]Plano, error) {
	rows, err := s.pool.Query(ctx, planSelect+` ORDER BY p.ativo DESC, p.nome ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Plano
	ids := make([]int, 0)
	for rows.Next() {
		plano, err := scanPlano(rows)
		if err != nil {
			return nil, err
		}
		ids = append(ids, plano.ID)
		out = append(out, plano)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		recursos, rerr := s.planRecursos(ctx, out[i].ID)
		if rerr != nil {
			return nil, rerr
		}
		out[i].Recursos = recursos
	}
	return out, nil
}

func (s *Store) GetPlan(ctx context.Context, id int) (Plano, error) {
	plano, err := scanPlano(s.pool.QueryRow(ctx, planSelect+` WHERE p.id=$1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Plano{}, ErrPlanNotFound
		}
		return Plano{}, err
	}
	if plano.Recursos, err = s.planRecursos(ctx, id); err != nil {
		return Plano{}, err
	}
	return plano, nil
}

func (s *Store) CreatePlan(ctx context.Context, plano Plano) (Plano, error) {
	if err := validatePlan(plano); err != nil {
		return Plano{}, err
	}
	var id int
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		owner := plano.TenantOwner
		if err := tx.QueryRow(ctx, `INSERT INTO planos (tenant_owner, nome, preco, intervalo, ativo)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`, owner, plano.Nome, plano.Preco, plano.Intervalo, plano.Ativo).Scan(&id); err != nil {
			return err
		}
		for _, recurso := range plano.Recursos {
			if strings.TrimSpace(recurso.Recurso) == "" || recurso.Limite < 0 {
				return ErrInvalidPlan
			}
			if _, err := tx.Exec(ctx, `INSERT INTO plano_recursos (plano_id, recurso, limite) VALUES ($1, $2, $3)`, id, recurso.Recurso, recurso.Limite); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Plano{}, err
	}
	return s.GetPlan(ctx, id)
}

func (s *Store) UpdatePlan(ctx context.Context, id int, plano Plano) (Plano, error) {
	plano.ID = id
	if err := validatePlan(plano); err != nil {
		return Plano{}, err
	}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE planos SET tenant_owner=$1, nome=$2, preco=$3, intervalo=$4, ativo=$5, updated_at=now()
			WHERE id=$6`, plano.TenantOwner, plano.Nome, plano.Preco, plano.Intervalo, plano.Ativo, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrPlanNotFound
		}
		if _, err := tx.Exec(ctx, `DELETE FROM plano_recursos WHERE plano_id=$1`, id); err != nil {
			return err
		}
		for _, recurso := range plano.Recursos {
			if strings.TrimSpace(recurso.Recurso) == "" || recurso.Limite < 0 {
				return ErrInvalidPlan
			}
			if _, err := tx.Exec(ctx, `INSERT INTO plano_recursos (plano_id, recurso, limite) VALUES ($1, $2, $3)`, id, recurso.Recurso, recurso.Limite); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Plano{}, err
	}
	return s.GetPlan(ctx, id)
}

func (s *Store) DeletePlan(ctx context.Context, id int) error {
	var inUse bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tenants WHERE plano_id=$1)`, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return ErrPlanInUse
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM planos WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPlanNotFound
	}
	return nil
}

func validatePlan(plano Plano) error {
	if strings.TrimSpace(plano.Nome) == "" {
		return fmt.Errorf("%w: nome é obrigatório", ErrInvalidPlan)
	}
	if plano.Preco < 0 {
		return fmt.Errorf("%w: preço não pode ser negativo", ErrInvalidPlan)
	}
	switch plano.Intervalo {
	case "semanal", "mensal", "anual":
	default:
		return fmt.Errorf("%w: intervalo inválido", ErrInvalidPlan)
	}
	for _, recurso := range plano.Recursos {
		if strings.TrimSpace(recurso.Recurso) == "" || recurso.Limite < 0 {
			return fmt.Errorf("%w: recurso inválido", ErrInvalidPlan)
		}
	}
	return nil
}

func (s *Store) GetTenantPlan(ctx context.Context, tenantID int) (EffectiveLimits, error) {
	resources, err := s.resolveEffectiveResourceMap(ctx, tenantID)
	if err != nil {
		return EffectiveLimits{}, err
	}
	var out EffectiveLimits
	out.TenantID = tenantID
	out.Recursos = resources
	err = s.pool.QueryRow(ctx, `SELECT p.id, p.nome, t.status, t.trial_until
		FROM tenants t LEFT JOIN planos p ON p.id=t.plano_id WHERE t.id=$1`, tenantID).
		Scan(&out.PlanoID, &out.PlanoNome, &out.Status, &out.TrialUntil)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EffectiveLimits{}, ErrTenantNotFound
		}
		return EffectiveLimits{}, err
	}
	out.Uso = s.computeTenantUsageMap(ctx, tenantID)
	return out, nil
}

func (s *Store) SetTenantPlan(ctx context.Context, tenantID int, assignment TenantPlanAssignment) (EffectiveLimits, error) {
	var overrideJSON []byte
	if assignment.LimitsOverride != nil {
		cleaned := make(map[string]float64, len(assignment.LimitsOverride))
		for key, value := range assignment.LimitsOverride {
			if key != "" && value >= 0 {
				cleaned[key] = value
			}
		}
		data, err := json.Marshal(cleaned)
		if err != nil {
			return EffectiveLimits{}, err
		}
		overrideJSON = data
	} else {
		overrideJSON = []byte("{}")
	}
	if assignment.PlanoID != nil && *assignment.PlanoID > 0 {
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM planos WHERE id=$1)`, *assignment.PlanoID).Scan(&exists); err != nil {
			return EffectiveLimits{}, err
		}
		if !exists {
			return EffectiveLimits{}, ErrPlanNotFound
		}
	} else if assignment.PlanoID != nil {
		return EffectiveLimits{}, ErrInvalidPlan
	}
	status := strings.TrimSpace(assignment.Status)
	if status != "" {
		switch status {
		case "trial", "active", "suspended", "closed":
		default:
			return EffectiveLimits{}, ErrInvalidPlan
		}
	}
	tag, err := s.pool.Exec(ctx, `UPDATE tenants SET
		plano_id=$1, trial_until=$2, limits_override=$3::jsonb,
		status=CASE WHEN $4='' THEN status ELSE $4 END, updated_at=now()
		WHERE id=$5`, assignment.PlanoID, assignment.TrialUntil, string(overrideJSON), status, tenantID)
	if err != nil {
		return EffectiveLimits{}, err
	}
	if tag.RowsAffected() == 0 {
		return EffectiveLimits{}, ErrTenantNotFound
	}
	return s.GetTenantPlan(ctx, tenantID)
}
