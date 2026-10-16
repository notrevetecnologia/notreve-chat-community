package channel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestGenericWebhookRequiresExplicitTenant(t *testing.T) {
	handler := NewWebhook(nil)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"message":"oi","phone":"NUMERO_COM_DDI"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "tenant context required" {
		t.Fatalf("error = %q, want tenant context required", body["error"])
	}
}

func TestGenericWebhookValidatesPayloadAfterTenantResolution(t *testing.T) {
	handler := NewWebhook(nil)
	ctx := store.WithTenantID(context.Background(), 7)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader("{"))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGenericWebhookRejectsEmptyMessageAfterTenantResolution(t *testing.T) {
	handler := NewWebhook(nil)
	ctx := store.WithTenantID(context.Background(), 7)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"message":"   "}`)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestProcessorRequiresExplicitTenantBeforeStoreAccess(t *testing.T) {
	processor := NewProcessor(nil, nil, nil)

	_, err := processor.Process(context.Background(), Inbound{Message: "oi"})

	if !errors.Is(err, store.ErrTenantContextRequired) {
		t.Fatalf("error = %v, want %v", err, store.ErrTenantContextRequired)
	}
}

func TestProcessorRejectsInboundTenantMismatchBeforeStoreAccess(t *testing.T) {
	processor := NewProcessor(nil, nil, nil)
	ctx := store.WithTenantID(context.Background(), 7)

	_, err := processor.Process(ctx, Inbound{Message: "oi", TenantID: 8})

	if err == nil || !strings.Contains(err.Error(), "inbound tenant does not match context") {
		t.Fatalf("error = %v, want tenant mismatch error", err)
	}
}
