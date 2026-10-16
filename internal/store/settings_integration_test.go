package store

import (
	"context"
	"testing"
)

func TestSettingsKnowledgeTopKRoundTrip(t *testing.T) {
	pool := newSaaSIsolationPool(t)
	defer pool.Close()

	tenantID := createIsolationTenant(t, pool, "settings-topk")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, tenantID)
	})

	ctx := WithTenantID(context.Background(), tenantID)
	if _, err := pool.Exec(ctx, `INSERT INTO settings (tenant_id) VALUES ($1)`, tenantID); err != nil {
		t.Fatalf("criando settings: %v", err)
	}

	s := New(pool)
	settings, err := s.GetSettings(ctx)
	if err != nil {
		t.Fatalf("lendo settings: %v", err)
	}
	if settings.KnowledgeTopK != 6 {
		t.Fatalf("knowledge_top_k padrão = %d, esperava 6", settings.KnowledgeTopK)
	}

	settings.KnowledgeTopK = 9
	if err := s.UpdateSettings(ctx, settings); err != nil {
		t.Fatalf("salvando settings: %v", err)
	}

	reloaded, err := s.GetSettings(ctx)
	if err != nil {
		t.Fatalf("relendo settings: %v", err)
	}
	if reloaded.KnowledgeTopK != 9 {
		t.Fatalf("knowledge_top_k = %d, esperava 9", reloaded.KnowledgeTopK)
	}
}
