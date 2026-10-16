package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrTenantAttendantLimitReached = errors.New("tenant active attendant limit reached")

// GetLimites mantém compatibilidade com as telas legadas, mas devolve os
// limites efetivos do tenant: base da empresa + plano + override específico.
func (s *Store) GetLimites(ctx context.Context) (Limites, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return Limites{}, err
	}
	resources, err := s.resolveEffectiveResourceMap(ctx, tenantID)
	if err != nil {
		return Limites{}, err
	}
	lim := Limites{
		ID:                   tenantID,
		MaxCanais:            int(resources[ResourceMaxCanais]),
		MaxAtendentes:        int(resources[ResourceMaxAtendentes]),
		MaxCampanhasAtivas:   int(resources[ResourceMaxCampanhasAtivas]),
		CampanhaLimiteDiario: int(resources[ResourceCampanhaLimiteDiario]),
	}
	if err := s.pool.QueryRow(ctx, `SELECT updated_at FROM tenant_limits WHERE tenant_id=$1`, tenantID).Scan(&lim.UpdatedAt); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return Limites{}, err
		}
		lim.UpdatedAt = time.Now()
	}
	return lim, nil
}

// UpdateLimites altera somente os limites-base da empresa. Planos e overrides
// continuam sendo aplicados por resolveEffectiveResourceMap ao consultar o
// limite efetivo.
func (s *Store) UpdateLimites(ctx context.Context, lim Limites) error {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return err
	}
	if lim.MaxCanais < 0 || lim.MaxAtendentes < 0 || lim.MaxCampanhasAtivas < 0 || lim.CampanhaLimiteDiario < 0 {
		return ErrInvalidTenantPlanLimit
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO tenant_limits
		(tenant_id, max_canais, max_atendentes, max_campanhas_ativas, campanha_limite_diario)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (tenant_id) DO UPDATE SET
			max_canais=EXCLUDED.max_canais,
			max_atendentes=EXCLUDED.max_atendentes,
			max_campanhas_ativas=EXCLUDED.max_campanhas_ativas,
			campanha_limite_diario=EXCLUDED.campanha_limite_diario,
			updated_at=now()`, tenantID, lim.MaxCanais, lim.MaxAtendentes, lim.MaxCampanhasAtivas, lim.CampanhaLimiteDiario)
	return err
}

func (s *Store) CountCanais(ctx context.Context) (int, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return 0, err
	}
	var count int
	err = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM channels WHERE tenant_id=$1`, tenantID).Scan(&count)
	return count, err
}

func (s *Store) CountAtendentes(ctx context.Context) (int, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return 0, err
	}
	var count int
	err = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM atendentes WHERE tenant_id=$1 AND active`, tenantID).Scan(&count)
	return count, err
}

func (s *Store) CountCampanhasAtivas(ctx context.Context) (int, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return 0, err
	}
	var count int
	err = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM campanhas WHERE tenant_id=$1 AND status IN ('agendada','rodando')`, tenantID).Scan(&count)
	return count, err
}

// EnsureCanCreateActiveAttendant aplica max_atendentes usando o mesmo mapa de
// recursos efetivos dos demais limites SaaS. Zero significa ilimitado.
func (s *Store) EnsureCanCreateActiveAttendant(ctx context.Context) error {
	tenantID, err := tenantIDForPlanEnforcement(ctx)
	if err != nil {
		return err
	}
	resources, err := s.resolveEffectiveResourceMap(ctx, tenantID)
	if err != nil {
		return err
	}
	var active int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM atendentes WHERE tenant_id=$1 AND active`, tenantID).Scan(&active); err != nil {
		return err
	}
	return enforceTenantResourceLimit(active, int(resources[ResourceMaxAtendentes]), ErrTenantAttendantLimitReached)
}
