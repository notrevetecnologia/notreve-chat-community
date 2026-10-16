package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	agentpkg "github.com/notrevetecnologia/notreve-chat-community/internal/agent"
	"github.com/notrevetecnologia/notreve-chat-community/internal/db"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestReactAgentTestRejectsEmptyMessage(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/react/agents/1/test", strings.NewReader(`{"message":"   "}`))
	r.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	(&Server{}).handleReactAgentTest(rr, r)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, esperado %d", rr.Code, http.StatusBadRequest)
	}
	var envelope reactEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error == nil || envelope.Error.Code != "empty_message" {
		t.Fatalf("erro inesperado: %+v", envelope.Error)
	}
}

// TestHandleReactAgentTestWithFakeProvider exercita o endpoint completo com um
// provider fake (httptest, sem rede real). Requer TEST_DATABASE_URL.
func TestHandleReactAgentTestWithFakeProvider(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping agent test endpoint integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("criando pool de teste: %v", err)
	}
	defer pool.Close()
	if err := db.Migrate(context.Background(), pool, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("aplicando migrations: %v", err)
	}

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"Olá! Posso ajudar com seu pedido."}}],"usage":{"prompt_tokens":8,"completion_tokens":6}}`))
	}))
	defer provider.Close()

	slug := fmt.Sprintf("agent-test-%d", time.Now().UnixNano())
	var tenantID int
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO tenants (slug, name, status, plan, timezone, locale) VALUES ($1,$2,'active','test','UTC','pt-BR') RETURNING id`,
		slug, "Agent Test").Scan(&tenantID); err != nil {
		t.Fatalf("criando tenant: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, tenantID)
	})

	st := store.New(pool)
	ctx := store.WithTenantID(context.Background(), tenantID)
	providerID, err := st.CreateProvider(ctx, store.Provider{
		Name: "Fake IA", ProviderType: "openai_compat", BaseURL: provider.URL,
		Model: "fake-model", APIKey: "fake-key", Active: true, Capacidade: "texto",
	})
	if err != nil {
		t.Fatalf("criando provider: %v", err)
	}
	agentID, err := st.CreateAgent(ctx, store.Agent{
		Name: "Agente de teste", ProviderID: &providerID, TemperatureLevel: "equilibrado",
		Active: true,
	})
	if err != nil {
		t.Fatalf("criando agente: %v", err)
	}

	server := &Server{st: st, eng: agentpkg.NewEngine(st)}
	r := httptest.NewRequest(http.MethodPost, "/api/react/agents/"+strconv.Itoa(agentID)+"/test", strings.NewReader(`{"message":"oi"}`))
	r.SetPathValue("id", strconv.Itoa(agentID))
	rr := httptest.NewRecorder()
	server.handleReactAgentTest(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo = %s", rr.Code, rr.Body.String())
	}
	var envelope struct {
		Data struct {
			Reply    string   `json:"reply"`
			Provider string   `json:"provider"`
			Model    string   `json:"model"`
			Used     []string `json:"used_tools"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Reply != "Olá! Posso ajudar com seu pedido." {
		t.Fatalf("resposta = %q", envelope.Data.Reply)
	}
	if envelope.Data.Provider != "Fake IA" || envelope.Data.Model != "fake-model" {
		t.Fatalf("provider/model inesperados: %+v", envelope.Data)
	}
}
