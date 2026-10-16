package web

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestApplyReactWebhookSubscriptionInputValidatesCatalogHTTPSAndSecret(t *testing.T) {
	name := "Eventos do ERP"
	endpoint := "https://hooks.example.com/notreve"
	events := []string{"message.received", "conversation.closed", "message.received"}
	secret := strings.Repeat("s", 16)
	active := true
	input, err := applyReactWebhookSubscriptionInput(&store.WebhookSubscription{}, reactWebhookSubscriptionInput{
		Name: &name, EndpointURL: &endpoint, Events: &events, SigningSecret: &secret, Active: &active,
	}, true)
	if err != nil {
		t.Fatalf("webhook válido rejeitado: %v", err)
	}
	if input.EndpointURL != endpoint || len(input.Events) != 2 || input.SigningSecret != secret || !input.Active {
		t.Fatalf("entrada normalizada inesperada: %#v", input)
	}

	httpEndpoint := "http://hooks.example.com/notreve"
	if _, err := applyReactWebhookSubscriptionInput(&store.WebhookSubscription{}, reactWebhookSubscriptionInput{Name: &name, EndpointURL: &httpEndpoint, Events: &events}, true); err == nil {
		t.Fatal("endpoint sem HTTPS deveria ser rejeitado")
	}
	unknownEvents := []string{"message.received", "billing.paid"}
	if _, err := applyReactWebhookSubscriptionInput(&store.WebhookSubscription{}, reactWebhookSubscriptionInput{Name: &name, EndpointURL: &endpoint, Events: &unknownEvents}, true); err == nil {
		t.Fatal("evento fora do catálogo deveria ser rejeitado")
	}
	shortSecret := "curto"
	if _, err := applyReactWebhookSubscriptionInput(&store.WebhookSubscription{}, reactWebhookSubscriptionInput{Name: &name, EndpointURL: &endpoint, Events: &events, SigningSecret: &shortSecret}, true); err == nil {
		t.Fatal("segredo curto deveria ser rejeitado")
	}
}

func TestReactWebhookSubscriptionViewNeverExposesSecretOrTenant(t *testing.T) {
	now := time.Date(2026, time.August, 22, 14, 0, 0, 0, time.UTC)
	view := reactWebhookSubscriptionFromStore(store.WebhookSubscription{
		ID: 10, TenantID: 999, Name: "ERP", EndpointURL: "https://hooks.example.com/notreve",
		Events: []string{"message.received"}, HasSigningSecret: true, Active: true,
		CreatedAt: now, UpdatedAt: now,
	})
	payload, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	value := string(payload)
	for _, forbidden := range []string{"tenant_id", "signing_secret_hash", "segredo-real"} {
		if strings.Contains(value, forbidden) {
			t.Fatalf("resposta expôs campo sensível %q: %s", forbidden, value)
		}
	}
	if !strings.Contains(value, `"has_signing_secret":true`) {
		t.Fatalf("resposta deveria informar apenas a existência do segredo: %s", value)
	}
}

func TestReactWebhookSubscriptionNeedsPlanCheckOnlyOnActivation(t *testing.T) {
	cases := []struct {
		wasActive, willBeActive, want bool
	}{
		{false, false, false},
		{false, true, true},
		{true, false, false},
		{true, true, false},
	}
	for _, testCase := range cases {
		if got := reactWebhookSubscriptionNeedsPlanCheck(testCase.wasActive, testCase.willBeActive); got != testCase.want {
			t.Fatalf("was=%t will=%t: recebeu %t, esperado %t", testCase.wasActive, testCase.willBeActive, got, testCase.want)
		}
	}
}
