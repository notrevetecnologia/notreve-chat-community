package store

import (
	"context"
	"testing"
)

func TestResolvedTenantOverridesHostLookup(t *testing.T) {
	s := New(nil)
	ctx := WithResolvedTenant(context.Background(), 42, "trial")
	id, status, err := s.ResolveTenantForHost(ctx, "cliente.exemplo.com")
	if err != nil {
		t.Fatalf("ResolveTenantForHost: %v", err)
	}
	if id != 42 || status != "trial" {
		t.Fatalf("tenant resolvido = (%d,%q), esperado (42,%q)", id, status, "trial")
	}
}

func TestChannelTypeResourceKeys(t *testing.T) {
	cases := map[string]string{
		"whatsapp": ResourceMaxCanaisWhatsApp,
		"telegram": ResourceMaxCanaisTelegram,
		"webchat":  ResourceMaxCanaisWebChat,
	}
	for channelType, expected := range cases {
		if got := channelTypeResourceKey(channelType); got != expected {
			t.Fatalf("resource de %s = %q, esperado %q", channelType, got, expected)
		}
	}
	if got := channelTypeResourceKey("email"); got != "" {
		t.Fatalf("canal não suportado retornou resource %q", got)
	}
}
