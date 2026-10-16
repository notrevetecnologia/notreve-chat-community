package store

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/webhooks"
)

func TestValidateWebhookOutboxEventInput(t *testing.T) {
	occurredAt := time.Date(2026, 8, 22, 12, 0, 0, 0, time.FixedZone("BRT", -3*60*60))
	input, err := validateWebhookOutboxEventInput(WebhookOutboxEventInput{
		EventKey:   "  message.sent:42  ",
		EventType:  webhooks.EventMessageSent,
		Payload:    json.RawMessage(`{"message_id":42}`),
		OccurredAt: occurredAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if input.EventKey != "message.sent:42" {
		t.Fatalf("event key = %q", input.EventKey)
	}
	if !input.OccurredAt.Equal(occurredAt.UTC()) {
		t.Fatalf("occurred at = %s, want UTC %s", input.OccurredAt, occurredAt.UTC())
	}

	for index, invalid := range []WebhookOutboxEventInput{
		{EventKey: "", EventType: webhooks.EventMessageSent, Payload: json.RawMessage(`{}`)},
		{EventKey: "event:1", EventType: webhooks.Event("not.catalogued"), Payload: json.RawMessage(`{}`)},
		{EventKey: "event:1", EventType: webhooks.EventMessageSent, Payload: json.RawMessage(`not-json`)},
	} {
		if _, err := validateWebhookOutboxEventInput(invalid); err == nil {
			t.Fatalf("case %d: expected validation error", index)
		}
	}
}

func TestNormalizeWebhookOutboxAttemptSanitizesPersistedError(t *testing.T) {
	next := time.Date(2026, 8, 22, 16, 10, 0, 0, time.UTC)
	normalized, err := normalizeWebhookOutboxAttempt(WebhookOutboxAttemptResult{
		ResponseStatus: 503,
		ErrorCode:      "HTTP_5XX",
		ErrorMessage:   " falhou\ncom\tcaracteres de controle ",
		Retryable:      true,
		NextAttemptAt:  &next,
	})
	if err != nil {
		t.Fatal(err)
	}
	if normalized.errorCode != "http_5xx" {
		t.Fatalf("error code = %q", normalized.errorCode)
	}
	if normalized.errorMessage != "falhoucomcaracteres de controle" {
		t.Fatalf("error message = %q", normalized.errorMessage)
	}
	if normalized.nextAttemptAt == nil || !normalized.nextAttemptAt.Equal(next) {
		t.Fatalf("next attempt = %v", normalized.nextAttemptAt)
	}

	if _, err := normalizeWebhookOutboxAttempt(WebhookOutboxAttemptResult{ResponseStatus: 99}); err == nil {
		t.Fatal("invalid HTTP response status should fail")
	}
	if _, err := normalizeWebhookOutboxAttempt(WebhookOutboxAttemptResult{ErrorCode: "unsafe value"}); err == nil {
		t.Fatal("unsafe error code should fail")
	}
}

func TestSanitizeWebhookOutboxErrorBoundsAndRemovesControls(t *testing.T) {
	value := "ok\x00" + strings.Repeat("x", maxWebhookOutboxErrorMessage+50)
	got := sanitizeWebhookOutboxError(value, maxWebhookOutboxErrorMessage)
	if len(got) != maxWebhookOutboxErrorMessage {
		t.Fatalf("length = %d, want %d", len(got), maxWebhookOutboxErrorMessage)
	}
	if strings.ContainsRune(got, '\x00') {
		t.Fatal("NUL should not be persisted")
	}
}
