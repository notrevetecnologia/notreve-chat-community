package web

import (
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"github.com/notrevetecnologia/notreve-chat-community/internal/tools"
)

func TestApplyReactIntegrationInputPersistsToolPolicySeparatelyFromSchema(t *testing.T) {
	integration := store.Integration{
		Method:     "POST",
		URL:        "https://api.example.com/payments",
		Parameters: `{"type":"object","properties":{"amount":{"type":"number"}}}`,
	}
	input := reactIntegrationInput{
		Name:        stringPointer("emitir_pagamento"),
		Description: stringPointer("Emite um pagamento autorizado"),
		Policy: &reactIntegrationPolicy{
			Connector:            "payment_action",
			Risk:                 string(tools.RiskCritical),
			RequiresConfirmation: true,
			RequiredScopes:       []string{"payments.write"},
			AllowedHosts:         []string{"api.example.com"},
			TimeoutSeconds:       15,
			MaxCallsPerMinute:    5,
			MaxCallsPerHour:      30,
			MaxResponseBytes:     4096,
			MaxArgumentBytes:     2048,
		},
	}
	if err := applyReactIntegrationInput(&integration, input, false); err != nil {
		t.Fatal(err)
	}
	policy, schema, err := tools.PolicyFromParameters(integration.Parameters, integration.Method)
	if err != nil {
		t.Fatal(err)
	}
	if policy.Risk != tools.RiskCritical || !policy.RequiresConfirmation || policy.Connector != "payment_action" {
		t.Fatalf("política não persistida: %#v", policy)
	}
	if len(policy.RequiredScopes) != 1 || policy.RequiredScopes[0] != "payments.write" {
		t.Fatalf("escopo não persistido: %#v", policy.RequiredScopes)
	}
	if _, ok := schema["x-tool-policy"]; ok {
		t.Fatal("extensão interna vazou para o schema retornado")
	}
	detail := reactIntegrationDetailFromStore(integration)
	if strings.Contains(detail.Parameters, "x-tool-policy") || detail.Policy.Risk != string(tools.RiskCritical) {
		t.Fatalf("detalhe React expôs ou perdeu a política: %#v", detail)
	}
}

func TestApplyReactIntegrationInputRejectsInvalidScope(t *testing.T) {
	integration := store.Integration{Method: "GET", URL: "https://api.example.com/data"}
	input := reactIntegrationInput{Policy: &reactIntegrationPolicy{Risk: string(tools.RiskLow), RequiredScopes: []string{"scope with spaces"}}}
	if err := applyReactIntegrationInput(&integration, input, true); err == nil {
		t.Fatal("escopo inválido deveria ser rejeitado")
	}
}

func stringPointer(value string) *string {
	return &value
}
