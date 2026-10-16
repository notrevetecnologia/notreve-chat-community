package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequireWebhookHMACAllowsUnsignedRequest(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"type":"Message"}` {
			t.Fatalf("corpo alterado: %s", body)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	record := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/notreve/webhook", strings.NewReader(`{"type":"Message"}`))
	requireWebhookHMAC("segredo", next).ServeHTTP(record, req)
	if record.Code != http.StatusNoContent {
		t.Fatalf("webhook sem assinatura retornou %d, esperado %d", record.Code, http.StatusNoContent)
	}
}

func TestRequireWebhookHMACStillRejectsInvalidProvidedSignature(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("não deveria encaminhar") })
	record := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/notreve/webhook", strings.NewReader(`{"type":"Message"}`))
	req.Header.Set("x-hmac-signature", "assinatura-incorreta")
	requireWebhookHMAC("segredo", next).ServeHTTP(record, req)
	if record.Code != http.StatusUnauthorized {
		t.Fatalf("assinatura inválida retornou %d, esperado %d", record.Code, http.StatusUnauthorized)
	}
}
