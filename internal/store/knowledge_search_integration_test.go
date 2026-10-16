package store

import (
	"context"
	"strings"
	"testing"
)

func TestSearchKnowledgeChunksRetrieval(t *testing.T) {
	pool := newSaaSIsolationPool(t)
	defer pool.Close()

	tenantID := createIsolationTenant(t, pool, "retrieval")
	otherTenantID := createIsolationTenant(t, pool, "retrieval-other")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1 OR id=$2`, tenantID, otherTenantID)
	})

	ctx := WithTenantID(context.Background(), tenantID)
	var agentID int
	if err := pool.QueryRow(ctx, `INSERT INTO agents (tenant_id, name) VALUES ($1,$2) RETURNING id`, tenantID, "Agente Retrieval").Scan(&agentID); err != nil {
		t.Fatalf("criando agente: %v", err)
	}

	s := New(pool)
	source := KnowledgeSource{
		AgentID: agentID,
		Kind:    "text",
		Name:    "Manual de planos",
		Status:  "ready",
		Content: strings.Repeat("Introdução geral sobre a empresa e seus valores. ", 400) +
			"O plano Pro custa noventa e nove reais por mês e inclui suporte prioritário. " +
			strings.Repeat("Encerramento com informações adicionais sobre a companhia. ", 400),
	}
	source.ContentChars = len([]rune(source.Content))
	sourceID, err := s.CreateKnowledgeSource(ctx, source)
	if err != nil {
		t.Fatalf("criando fonte de conhecimento: %v", err)
	}

	hasChunks, err := s.HasKnowledgeChunks(ctx, agentID)
	if err != nil {
		t.Fatalf("consultando chunks: %v", err)
	}
	if !hasChunks {
		t.Fatal("o trigger deveria ter indexado a fonte em trechos")
	}

	chunks, err := s.SearchKnowledgeChunks(ctx, agentID, "quanto custa o plano pro?", 5)
	if err != nil {
		t.Fatalf("buscando trechos: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("a busca não retornou nenhum trecho")
	}
	found := false
	for _, chunk := range chunks {
		if strings.Contains(chunk.Content, "noventa e nove") {
			found = true
		}
	}
	if !found {
		t.Fatalf("o trecho relevante não apareceu no top-K: %#v", chunks)
	}

	updated := "O plano Enterprise custa quatrocentos e noventa reais por mês. " +
		strings.Repeat("Detalhes complementares do plano corporativo. ", 600)
	if err := s.UpdateKnowledgeSourceResult(ctx, sourceID, "ready", updated, "", len([]rune(updated)), nil); err != nil {
		t.Fatalf("atualizando a fonte: %v", err)
	}
	afterUpdate, err := s.SearchKnowledgeChunks(ctx, agentID, "plano enterprise", 5)
	if err != nil {
		t.Fatalf("buscando após atualização: %v", err)
	}
	if len(afterUpdate) == 0 || !strings.Contains(afterUpdate[0].Content, "quatrocentos e noventa") {
		t.Fatalf("a atualização de conteúdo não reindexou os trechos: %#v", afterUpdate)
	}
	oldMatch, err := s.SearchKnowledgeChunks(ctx, agentID, "plano pro noventa e nove", 5)
	if err != nil {
		t.Fatalf("buscando trecho antigo: %v", err)
	}
	for _, chunk := range oldMatch {
		if strings.Contains(chunk.Content, "noventa e nove") {
			t.Fatalf("trecho antigo permaneceu após a atualização: %#v", chunk)
		}
	}

	otherCtx := WithTenantID(context.Background(), otherTenantID)
	foreign, err := s.SearchKnowledgeChunks(otherCtx, agentID, "plano pro", 5)
	if err != nil {
		t.Fatalf("buscando como outro tenant: %v", err)
	}
	if len(foreign) != 0 {
		t.Fatalf("a busca vazou trechos entre tenants: %#v", foreign)
	}
}
