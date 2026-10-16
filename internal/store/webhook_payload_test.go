package store

import (
	"encoding/json"
	"testing"
	"time"
)

func TestConversationWebhookPayloadRedactsInternals(t *testing.T) {
	conv := Conversation{
		ID: 7, TenantID: 3, Channel: "whatsapp", Status: "ia", CustomerName: "Maria",
		CustomerPhone: "5511999999999", Origem: "web", CreatedAt: time.Now(),
		SetorName: "Vendas", AtendenteName: "João",
	}
	payload := ConversationWebhookPayload(conv)
	for _, key := range []string{"id", "status", "channel", "customer_id", "customer_name", "origem", "created_at", "setor", "atendente"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("payload deve conter %q", key)
		}
	}
	for _, hidden := range []string{"tenant_id", "atendente_id", "agent_id", "media_b64"} {
		if _, ok := payload[hidden]; ok {
			t.Fatalf("payload não deve expor %q", hidden)
		}
	}
}

func TestMessageWebhookPayloadRedactsSensitive(t *testing.T) {
	msg := Message{
		ID: 9, TenantID: 3, ConversationID: 7, Role: "user", Source: "customer",
		Content: "Olá", MediaType: "image", MediaName: "foto.jpg",
		MediaB64: "AAAA", ContactTel: "5511999999999", ContactName: "Maria",
	}
	payload := MessageWebhookPayload(msg)
	if payload["content"] != "Olá" || payload["source"] != "customer" || payload["role"] != "user" {
		t.Fatalf("campos esperados incorretos: %v", payload)
	}
	for _, hidden := range []string{"media_b64", "contact_tel", "contact_name", "tenant_id", "lat", "lng"} {
		if _, ok := payload[hidden]; ok {
			t.Fatalf("payload não deve expor %q", hidden)
		}
	}
}

func TestConversationWebhookPayloadSerializable(t *testing.T) {
	if _, err := json.Marshal(ConversationWebhookPayload(Conversation{ID: 1, Status: "ia"})); err != nil {
		t.Fatalf("payload deve ser serializável: %v", err)
	}
}
