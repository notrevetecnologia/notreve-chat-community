package store

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

type PlatformInitialAdminInput struct {
	Name         string
	Username     string
	PasswordHash string
	Permissions  string
}

func validatePlatformTenantLimits(limits PlatformTenantLimits) error {
	if limits.MaxCanais < 0 || limits.MaxAtendentes < 0 || limits.MaxCampanhasAtivas < 0 || limits.CampanhaLimiteDiario < 0 ||
		limits.MaxAgentes < 0 || limits.MaxUsuarios < 0 || limits.MaxTools < 0 ||
		limits.MaxAPIKeys < 0 || limits.MaxWebhooks < 0 || limits.CreditosIAMensais < 0 {
		return fmt.Errorf("%w: limits cannot be negative", ErrInvalidPlatformTenant)
	}
	return nil
}

func normalizedPlatformModuleKeys(modules []string) []string {
	set := map[string]bool{}
	for _, key := range coreModuleKeys() {
		set[key] = true
	}
	for _, key := range modules {
		key = strings.TrimSpace(key)
		if key != "" {
			set[key] = true
		}
	}
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func validatePlatformInitialAdmin(input *PlatformInitialAdminInput) error {
	if input == nil {
		return nil
	}
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Username) == "" || strings.TrimSpace(input.PasswordHash) == "" {
		return fmt.Errorf("%w: initial admin is incomplete", ErrInvalidPlatformTenant)
	}
	return nil
}

func validatePlatformModulesTx(ctx context.Context, tx pgx.Tx, moduleKeys []string) error {
	rows, err := tx.Query(ctx, `SELECT key FROM modulos`)
	if err != nil {
		return err
	}
	defer rows.Close()
	known := map[string]bool{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return err
		}
		known[key] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, key := range moduleKeys {
		if !known[key] {
			return fmt.Errorf("%w: %s", ErrPlatformModuleNotFound, key)
		}
	}
	return nil
}

func applyTenantModulesTx(ctx context.Context, tx pgx.Tx, tenantID int, moduleKeys []string) error {
	if _, err := tx.Exec(ctx, `INSERT INTO tenant_modules (tenant_id, module_key, enabled)
		SELECT $1, key, key = ANY($2::text[]) FROM modulos
		ON CONFLICT (tenant_id, module_key) DO UPDATE SET enabled=EXCLUDED.enabled, updated_at=now()`, tenantID, moduleKeys); err != nil {
		return err
	}
	return nil
}

func applyTenantBaseLimitsTx(ctx context.Context, tx pgx.Tx, tenantID int, limits *PlatformTenantLimits) error {
	if limits == nil {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE tenant_limits SET
		max_canais=$1, max_atendentes=$2, max_campanhas_ativas=$3, campanha_limite_diario=$4,
		max_agentes=$5, max_usuarios=$6, max_tools=$7, max_api_keys=$8,
		max_webhooks=$9, creditos_ia_mensais=$10, updated_at=now()
		WHERE tenant_id=$11`,
		limits.MaxCanais, limits.MaxAtendentes, limits.MaxCampanhasAtivas, limits.CampanhaLimiteDiario,
		limits.MaxAgentes, limits.MaxUsuarios, limits.MaxTools, limits.MaxAPIKeys,
		limits.MaxWebhooks, limits.CreditosIAMensais, tenantID)
	return err
}

func applyTenantChannelLimitsTx(ctx context.Context, tx pgx.Tx, tenantID int, limits *PlatformChannelTypeLimits) error {
	if limits == nil {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE tenant_limits SET
		max_canais_whatsapp=$1, max_conexoes_whatsapp=$1,
		max_canais_telegram=$2, max_canais_webchat=$3, updated_at=now()
		WHERE tenant_id=$4`, limits.WhatsApp, limits.Telegram, limits.WebChat, tenantID)
	return err
}

func (s *Store) CreatePlatformTenantProvisioned(ctx context.Context, input PlatformTenantInput, limits *PlatformTenantLimits, channelLimits *PlatformChannelTypeLimits, modules []string, admin *PlatformInitialAdminInput) (PlatformTenant, error) {
	ctx = WithGlobalDataAccess(ctx)
	input = normalizedPlatformTenantInput(input)
	if err := validatePlatformTenantInput(input); err != nil {
		return PlatformTenant{}, err
	}
	if limits != nil {
		if err := validatePlatformTenantLimits(*limits); err != nil {
			return PlatformTenant{}, err
		}
	}
	if channelLimits != nil {
		if err := validatePlatformChannelTypeLimits(*channelLimits); err != nil {
			return PlatformTenant{}, err
		}
	}
	if err := validatePlatformInitialAdmin(admin); err != nil {
		return PlatformTenant{}, err
	}
	moduleKeys := normalizedPlatformModuleKeys(modules)
	var tenantID int
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := validatePlatformModulesTx(ctx, tx, moduleKeys); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO tenants (slug, name, status, plan, timezone, locale, dominio, creditos, plano_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,(SELECT id FROM planos WHERE nome='Padrão' LIMIT 1)) RETURNING id`, input.Slug, input.Name, input.Status, input.Plan, input.Timezone, input.Locale, input.Dominio, input.Creditos).Scan(&tenantID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO tenant_limits (tenant_id) VALUES ($1)`, tenantID); err != nil {
			return err
		}
		if err := applyTenantBaseLimitsTx(ctx, tx, tenantID, limits); err != nil {
			return err
		}
		if err := applyTenantChannelLimitsTx(ctx, tx, tenantID, channelLimits); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO tenant_usage_counters (tenant_id) VALUES ($1)`, tenantID); err != nil {
			return err
		}
		if err := applyTenantModulesTx(ctx, tx, tenantID, moduleKeys); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO settings (tenant_id) VALUES ($1)`, tenantID); err != nil {
			return err
		}
		if admin != nil {
			var attendantID int
			if err := tx.QueryRow(ctx, `INSERT INTO atendentes
				(tenant_id, name, active, online, horario_abertura, horario_fechamento, dias_semana)
				VALUES ($1,$2,true,false,'00:00','23:59','0,1,2,3,4,5,6') RETURNING id`, tenantID, strings.TrimSpace(admin.Name)).Scan(&attendantID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO usuarios
				(tenant_id, name, username, password_hash, role, permissions, atendente_id, active, must_change_password)
				VALUES ($1,$2,$3,$4,'admin',$5,$6,true,true)`, tenantID, strings.TrimSpace(admin.Name), strings.TrimSpace(admin.Username), admin.PasswordHash, admin.Permissions, attendantID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return PlatformTenant{}, err
	}
	return s.GetPlatformTenant(ctx, tenantID)
}

func (s *Store) UpdatePlatformTenantProvisioned(ctx context.Context, tenantID int, input PlatformTenantInput, limits *PlatformTenantLimits, channelLimits *PlatformChannelTypeLimits, modules []string) (PlatformTenant, error) {
	ctx = WithGlobalDataAccess(ctx)
	if tenantID < 1 {
		return PlatformTenant{}, ErrInvalidPlatformTenant
	}
	input = normalizedPlatformTenantInput(input)
	if err := validatePlatformTenantInput(input); err != nil {
		return PlatformTenant{}, err
	}
	if limits != nil {
		if err := validatePlatformTenantLimits(*limits); err != nil {
			return PlatformTenant{}, err
		}
	}
	if channelLimits != nil {
		if err := validatePlatformChannelTypeLimits(*channelLimits); err != nil {
			return PlatformTenant{}, err
		}
	}
	moduleKeys := normalizedPlatformModuleKeys(modules)
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := validatePlatformModulesTx(ctx, tx, moduleKeys); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE tenants SET slug=$1, name=$2, status=$3, plan=$4, timezone=$5, locale=$6, dominio=$7, creditos=$8, updated_at=now()
			WHERE id=$9`, input.Slug, input.Name, input.Status, input.Plan, input.Timezone, input.Locale, input.Dominio, input.Creditos, tenantID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrPlatformTenantNotFound
		}
		if err := applyTenantBaseLimitsTx(ctx, tx, tenantID, limits); err != nil {
			return err
		}
		if err := applyTenantChannelLimitsTx(ctx, tx, tenantID, channelLimits); err != nil {
			return err
		}
		return applyTenantModulesTx(ctx, tx, tenantID, moduleKeys)
	})
	if err != nil {
		return PlatformTenant{}, err
	}
	return s.GetPlatformTenant(ctx, tenantID)
}
