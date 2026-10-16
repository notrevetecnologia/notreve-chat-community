package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestEnforceTenantResourceLimit(t *testing.T) {
	tests := []struct {
		name    string
		active  int
		maximum int
		want    error
	}{
		{name: "capacity remains", active: 2, maximum: 3},
		{name: "at configured limit", active: 3, maximum: 3, want: ErrTenantAgentLimitReached},
		{name: "above configured limit", active: 4, maximum: 3, want: ErrTenantAgentLimitReached},
		{name: "zero means unlimited", active: 0, maximum: 0},
		{name: "invalid count", active: -1, maximum: 3, want: ErrInvalidTenantPlanLimit},
		{name: "invalid maximum", active: 1, maximum: -1, want: ErrInvalidTenantPlanLimit},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := enforceTenantResourceLimit(test.active, test.maximum, ErrTenantAgentLimitReached)
			if !errors.Is(err, test.want) {
				t.Fatalf("enforceTenantResourceLimit(%d, %d) error = %v, want %v", test.active, test.maximum, err, test.want)
			}
		})
	}
}

func TestEnforceTenantAICreditLimit(t *testing.T) {
	tests := []struct {
		name      string
		used      int64
		maximum   int64
		requested int64
		want      error
	}{
		{name: "remaining credit", used: 6, maximum: 10, requested: 4},
		{name: "request above remaining", used: 6, maximum: 10, requested: 5, want: ErrTenantAICreditLimitReached},
		{name: "no prepaid credit", used: 0, maximum: 0, requested: 1, want: ErrTenantAICreditLimitReached},
		{name: "invalid negative usage", used: -1, maximum: 10, requested: 1, want: ErrInvalidTenantPlanLimit},
		{name: "invalid negative request", used: 0, maximum: 10, requested: -1, want: ErrInvalidAICreditAmount},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := enforceTenantAICreditLimit(test.used, test.maximum, test.requested)
			if !errors.Is(err, test.want) {
				t.Fatalf("enforceTenantAICreditLimit(%d, %d, %d) error = %v, want %v", test.used, test.maximum, test.requested, err, test.want)
			}
		})
	}
}

func TestPlanEnforcementRequiresExplicitTenantContext(t *testing.T) {
	if _, err := tenantIDForPlanEnforcement(context.Background()); !errors.Is(err, ErrTenantContextRequired) {
		t.Fatalf("missing tenant context error = %v, want %v", err, ErrTenantContextRequired)
	}
	if got, err := tenantIDForPlanEnforcement(WithTenantID(context.Background(), 42)); err != nil || got != 42 {
		t.Fatalf("tenant id = (%d, %v), want (42, nil)", got, err)
	}
}

func TestTenantPlanLimitUsageQueryIsTenantScoped(t *testing.T) {
	for _, table := range []string{"agents", "usuarios", "integrations", "public_api_keys", "webhook_subscriptions"} {
		if !strings.Contains(tenantPlanLimitUsageQuery, "FROM "+table) {
			t.Fatalf("query does not count %s", table)
		}
	}
	if !strings.Contains(tenantPlanLimitUsageQuery, "usage.tenant_id=l.tenant_id") || !strings.Contains(tenantPlanLimitUsageQuery, "WHERE l.tenant_id=$1") {
		t.Fatal("plan enforcement query must remain scoped to one tenant")
	}
}

func TestDefaultResourceMap(t *testing.T) {
	m := defaultResourceMap()
	if m[ResourceMaxAgentes] != 3 {
		t.Fatalf("max_agentes default = %v, want 3", m[ResourceMaxAgentes])
	}
	if m[ResourceMaxCanais] != 3 {
		t.Fatalf("max_canais default = %v, want 3", m[ResourceMaxCanais])
	}
	if m[ResourceMaxUsuarios] != 10 {
		t.Fatalf("max_usuarios default = %v, want 10", m[ResourceMaxUsuarios])
	}
	if m[ResourceMaxMensagensDia] != 0 {
		t.Fatalf("max_mensagens_dia default = %v, want 0 (unlimited)", m[ResourceMaxMensagensDia])
	}
}

func TestValidatePlan(t *testing.T) {
	valid := Plano{Nome: "Essencial", Preco: 0, Intervalo: "mensal"}
	if err := validatePlan(valid); err != nil {
		t.Fatalf("valid plan error = %v, want nil", err)
	}
	tests := []struct {
		name string
		plano Plano
	}{
		{name: "nome obrigatório", plano: Plano{Nome: "", Intervalo: "mensal"}},
		{name: "preço negativo", plano: Plano{Nome: "X", Preco: -1, Intervalo: "mensal"}},
		{name: "intervalo inválido", plano: Plano{Nome: "X", Intervalo: "semanalmente"}},
		{name: "recurso sem nome", plano: Plano{Nome: "X", Intervalo: "mensal", Recursos: []PlanoRecurso{{Recurso: "", Limite: 1}}}},
		{name: "recurso negativo", plano: Plano{Nome: "X", Intervalo: "mensal", Recursos: []PlanoRecurso{{Recurso: "max_canais", Limite: -1}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validatePlan(test.plano); !errors.Is(err, ErrInvalidPlan) {
				t.Fatalf("validatePlan error = %v, want %v", err, ErrInvalidPlan)
			}
		})
	}
}
