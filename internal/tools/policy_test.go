package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestPolicyRoundTripRemovesRuntimeExtensionFromSchema(t *testing.T) {
	schema := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"account_id": map[string]interface{}{"type": "string", "maxLength": 40},
		},
	}
	policy := Policy{
		Connector:            "payment_action",
		Risk:                 RiskCritical,
		RequiresConfirmation: true,
		RequiredScopes:       []string{"payments.write"},
		AllowedHosts:         []string{"api.example.com"},
		TimeoutSeconds:       15,
		MaxCallsPerMinute:    5,
		MaxCallsPerHour:      30,
		MaxResponseBytes:     4096,
		MaxArgumentBytes:     2048,
	}
	encoded, err := EncodeParameters(schema, policy)
	if err != nil {
		t.Fatal(err)
	}
	decoded, cleanSchema, err := PolicyFromParameters(encoded, "POST")
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Connector != policy.Connector || decoded.Risk != policy.Risk || !decoded.RequiresConfirmation {
		t.Fatalf("política não preservada: %#v", decoded)
	}
	if len(decoded.RequiredScopes) != 1 || decoded.RequiredScopes[0] != "payments.write" {
		t.Fatalf("escopos não preservados: %#v", decoded.RequiredScopes)
	}
	if _, ok := cleanSchema["x-tool-policy"]; ok {
		t.Fatal("extensão interna exposta ao schema da IA")
	}
}

func TestValidateArgumentsHonorsRequiredTypesAndAdditionalProperties(t *testing.T) {
	schema := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []interface{}{"name"},
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string", "maxLength": float64(10)},
		},
	}
	if err := ValidateArguments(schema, map[string]interface{}{"name": "Exemplo"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateArguments(schema, map[string]interface{}{}); err == nil {
		t.Fatal("argumento obrigatório deveria ser rejeitado")
	}
	if err := ValidateArguments(schema, map[string]interface{}{"name": "Exemplo", "unknown": true}); err == nil {
		t.Fatal("propriedade adicional deveria ser rejeitada")
	}
	if err := ValidateArguments(schema, map[string]interface{}{"name": 10}); err == nil {
		t.Fatal("tipo inválido deveria ser rejeitado")
	}
}

func TestExecutorRequiresScopesAndConfirmationAndAuditsWithoutValues(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	audit := &MemoryAuditSink{}
	executor := NewExecutor()
	executor.AllowPrivateNetwork = true
	executor.Audit = audit
	definition := store.Tool{
		Name:                 "emitir_pagamento",
		Method:               "POST",
		URL:                  server.URL,
		Parameters:           map[string]interface{}{"type": "object"},
		Risk:                 string(RiskCritical),
		RequiresConfirmation: true,
		RequiredScopes:       []string{"payments.write"},
	}
	arguments := map[string]interface{}{"amount": 100, "secret": "não deve ir para a auditoria"}
	ctx := WithToolScopes(context.Background(), "payments.write")
	result := executor.ExecuteDetailed(ctx, definition, arguments)
	if result.Status != "confirmation_required" || result.ConfirmationToken == "" {
		t.Fatalf("confirmação não exigida: %#v", result)
	}
	if calls.Load() != 0 {
		t.Fatal("ação crítica foi executada antes da confirmação")
	}
	confirmed := WithToolConfirmation(ctx, result.ConfirmationToken)
	result = executor.ExecuteDetailed(confirmed, definition, arguments)
	if result.Status != "success" || calls.Load() != 1 {
		t.Fatalf("execução confirmada falhou: %#v", result)
	}
	if len(audit.Events) != 2 || audit.Events[0].Status != "confirmation_required" || audit.Events[1].Status != "success" {
		t.Fatalf("auditoria incompleta: %#v", audit.Events)
	}
	encodedAudit, err := json.Marshal(audit.Events)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encodedAudit, []byte("não deve ir para a auditoria")) {
		t.Fatal("auditoria registrou valor de argumento")
	}
}

func TestExecutorEnforcesCallLimits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	executor := NewExecutor()
	executor.AllowPrivateNetwork = true
	definition := store.Tool{Name: "consulta_limitada", Method: "GET", URL: server.URL, MaxCallsPerMinute: 1, MaxCallsPerHour: 1}
	if result := executor.ExecuteDetailed(context.Background(), definition, nil); result.Status != "success" {
		t.Fatalf("primeira chamada falhou: %#v", result)
	}
	if result := executor.ExecuteDetailed(context.Background(), definition, nil); result.Status != "blocked_limit" {
		t.Fatalf("segunda chamada não foi limitada: %#v", result)
	}
}

func TestValidateEndpointRejectsPrivateHostsAndHonorsAllowedHosts(t *testing.T) {
	policy := DefaultPolicy("GET")
	if err := ValidateEndpoint("http://127.0.0.1:8080", "GET", policy); err == nil {
		t.Fatal("host privado deveria ser rejeitado")
	}
	policy.AllowedHosts = []string{"api.example.com"}
	if err := ValidateEndpoint("https://other.example.com/data", "GET", policy); err == nil {
		t.Fatal("host fora do escopo deveria ser rejeitado")
	}
}
