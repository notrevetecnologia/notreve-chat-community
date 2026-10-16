package store

import "testing"

func TestNormalizeWebhookEvents(t *testing.T) {
	got := normalizeWebhookEvents([]string{" Message.Created ", "message.created", "conversation.closed"})
	want := []string{"conversation.closed", "message.created"}
	if len(got) != len(want) {
		t.Fatalf("events length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeWebhookEndpoint(t *testing.T) {
	if got, err := normalizeWebhookEndpoint("https://hooks.example.test/events"); err != nil || got != "https://hooks.example.test/events" {
		t.Fatalf("valid endpoint = %q, err = %v", got, err)
	}
	if _, err := normalizeWebhookEndpoint("file:///tmp/hook"); err == nil {
		t.Fatal("file endpoint should be rejected")
	}
}

func TestValidWebhookDeliveryStatus(t *testing.T) {
	for _, status := range []string{"pending", "processing", "failed", "delivered", "dead_letter"} {
		if !validWebhookDeliveryStatus(status) {
			t.Fatalf("status %q should be valid", status)
		}
	}
	if validWebhookDeliveryStatus("queued") {
		t.Fatal("unknown status should be rejected")
	}
}
