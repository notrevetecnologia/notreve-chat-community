package store

import (
	"context"
	"testing"
	"time"
)

func TestAgentFailuresRecordListResolveAndIsolation(t *testing.T) {
	pool := newSaaSIsolationPool(t)
	defer pool.Close()

	tenantA := createIsolationTenant(t, pool, "failures-a")
	tenantB := createIsolationTenant(t, pool, "failures-b")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM agent_failures WHERE tenant_id=$1 OR tenant_id=$2`, tenantA, tenantB)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1 OR id=$2`, tenantA, tenantB)
	})

	ctxA := WithTenantID(context.Background(), tenantA)
	ctxB := WithTenantID(context.Background(), tenantB)
	s := New(pool)

	conversationID := 4242
	if err := s.RecordAgentFailure(ctxA, AgentFailure{
		ConversationID: &conversationID, Channel: "whatsapp", Phone: "5511999999999",
		Origem: "engine", Tipo: FailureKindLLM, ExceptionClass: "errorString", MensagemErro: "LLM HTTP 500",
	}); err != nil {
		t.Fatalf("registrando falha: %v", err)
	}
	if err := s.RecordAgentFailure(ctxA, AgentFailure{
		Channel: "telegram", Origem: "watchdog", Tipo: FailureKindPipeline,
		ExceptionClass: "OrphanMessage", MensagemErro: "mensagem órfã",
	}); err != nil {
		t.Fatalf("registrando falha de watchdog: %v", err)
	}

	items, err := s.ListAgentFailures(ctxA, "", true, 50)
	if err != nil {
		t.Fatalf("listando falhas: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("esperava 2 falhas, obtido %d", len(items))
	}

	filtered, err := s.ListAgentFailures(ctxA, FailureKindPipeline, true, 50)
	if err != nil {
		t.Fatalf("listando falhas por tipo: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Origem != "watchdog" {
		t.Fatalf("filtro por tipo inesperado: %#v", filtered)
	}

	open, err := s.CountOpenAgentFailures(ctxA)
	if err != nil || open != 2 {
		t.Fatalf("falhas abertas: %d erro=%v", open, err)
	}
	stats, err := s.CountAgentFailuresByType(ctxA, time.Now().Add(-time.Hour))
	if err != nil || stats[FailureKindLLM] != 1 || stats[FailureKindPipeline] != 1 {
		t.Fatalf("estatísticas inesperadas: %#v erro=%v", stats, err)
	}

	isolated, err := s.ListAgentFailures(ctxB, "", true, 50)
	if err != nil {
		t.Fatalf("listando falhas do tenant B: %v", err)
	}
	if len(isolated) != 0 {
		t.Fatalf("tenant B não deveria ver falhas do tenant A: %#v", isolated)
	}

	resolved, err := s.MarkAgentFailureResolved(ctxA, items[0].ID, "admin", "verificado")
	if err != nil || !resolved {
		t.Fatalf("resolvendo falha: resolved=%v erro=%v", resolved, err)
	}
	again, err := s.MarkAgentFailureResolved(ctxA, items[0].ID, "admin", "verificado")
	if err != nil || again {
		t.Fatalf("falha já resolvida não deveria resolver de novo: resolved=%v erro=%v", again, err)
	}
	open, err = s.CountOpenAgentFailures(ctxA)
	if err != nil || open != 1 {
		t.Fatalf("após resolver esperava 1 aberta, obtido %d erro=%v", open, err)
	}
}
