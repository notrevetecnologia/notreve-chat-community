package web

import (
	"net/http/httptest"
	"testing"
)

func TestPublicScopeAllowed(t *testing.T) {
	cases := []struct {
		name   string
		scopes []string
		want   bool
	}{
		{name: "exact", scopes: []string{"conversations:read"}, want: true},
		{name: "wildcard", scopes: []string{"conversations:*"}, want: true},
		{name: "all", scopes: []string{"*"}, want: true},
		{name: "denied", scopes: []string{"messages:write"}, want: false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := publicScopeAllowed(test.scopes, "conversations:read"); got != test.want {
				t.Fatalf("publicScopeAllowed() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPublicIdempotencyReplaysAndRejectsDifferentPayload(t *testing.T) {
	state := newPublicAPIState()
	first, err := state.begin("key:1:conversation.open", "request-1", "fingerprint-a")
	if err != nil || first.replay {
		t.Fatalf("first begin = %#v, %v", first, err)
	}
	state.finish("key:1:conversation.open", "request-1", "fingerprint-a", 200, []byte(`{"data":{"ok":true}}`))
	replay, err := state.begin("key:1:conversation.open", "request-1", "fingerprint-a")
	if err != nil || !replay.replay || replay.status != 200 || string(replay.body) != `{"data":{"ok":true}}` {
		t.Fatalf("replay = %#v, %v", replay, err)
	}
	if _, err := state.begin("key:1:conversation.open", "request-1", "fingerprint-b"); err != errPublicIdempotencyConflict {
		t.Fatalf("different fingerprint error = %v, want conflict", err)
	}
}

func TestPublicIdempotencyReplaysServerFailure(t *testing.T) {
	state := newPublicAPIState()
	if _, err := state.begin("key:1:conversation.message", "request-500", "fingerprint-a"); err != nil {
		t.Fatalf("first begin: %v", err)
	}
	state.finish("key:1:conversation.message", "request-500", "fingerprint-a", 502, []byte(`{"error":{"code":"delivery_failed"}}`))

	replay, err := state.begin("key:1:conversation.message", "request-500", "fingerprint-a")
	if err != nil || !replay.replay || replay.status != 502 {
		t.Fatalf("server-failure replay = %#v, %v", replay, err)
	}
}

func TestRunPublicMutationReplaysServerFailure(t *testing.T) {
	api := &PublicAPI{state: newPublicAPIState()}
	request := httptest.NewRequest("POST", "/api/public/v1/conversations/1/messages", nil)
	request.Header.Set("Authorization", "Bearer secret-key")
	request.Header.Set("Idempotency-Key", "request-500")

	first := httptest.NewRecorder()
	api.runPublicMutation(first, request, "1:conversation.message:1", "fingerprint-a", nil, func() publicOperationResult {
		return publicOperationResult{status: 502, body: publicErrorEnvelope{Error: publicAPIError{Code: "delivery_failed", Message: "falhou"}}}
	})
	if first.Code != 502 {
		t.Fatalf("first status = %d, want 502", first.Code)
	}

	replayRequest := httptest.NewRequest("POST", "/api/public/v1/conversations/1/messages", nil)
	replayRequest.Header.Set("Authorization", "Bearer secret-key")
	replayRequest.Header.Set("Idempotency-Key", "request-500")
	replay := httptest.NewRecorder()
	api.runPublicMutation(replay, replayRequest, "1:conversation.message:1", "fingerprint-a", nil, func() publicOperationResult {
		t.Fatal("the original operation must not be run again")
		return publicOperationResult{}
	})
	if replay.Code != 502 || replay.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay status/header = %d/%q, want 502/true", replay.Code, replay.Header().Get("Idempotent-Replayed"))
	}
}

func TestPublicIdempotencyKeyValidation(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/public/v1/conversations", nil)
	if _, err := publicIdempotencyKey(request); err == nil {
		t.Fatal("missing Idempotency-Key was accepted")
	}
	request.Header.Set("Idempotency-Key", "request-1")
	if got, err := publicIdempotencyKey(request); err != nil || got != "request-1" {
		t.Fatalf("publicIdempotencyKey() = %q, %v", got, err)
	}
}

func TestNormalizePublicOrigin(t *testing.T) {
	got, err := normalizePublicOrigin("HTTPS://Example.COM/some-page")
	if err != nil || got != "https://example.com" {
		t.Fatalf("normalizePublicOrigin() = %q, %v", got, err)
	}
	if _, err := normalizePublicOrigin("javascript:alert(1)"); err == nil {
		t.Fatal("unsafe origin was accepted")
	}
}
