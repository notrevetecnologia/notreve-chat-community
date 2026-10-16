package web

import (
	"strings"
	"testing"
)

func TestValidateNewUserPassword(t *testing.T) {
	if message := validateNewUserPassword("curta"); message == "" {
		t.Fatal("senha curta deveria ser rejeitada")
	}
	if message := validateNewUserPassword("12345678"); message != "" {
		t.Fatalf("senha válida rejeitada: %s", message)
	}
	if message := validateNewUserPassword(strings.Repeat("a", 73)); message == "" {
		t.Fatal("senha acima do limite do bcrypt deveria ser rejeitada")
	}
}

func TestSaaSPlatformTenantChannelLimitsID(t *testing.T) {
	id, ok := saasPlatformTenantChannelLimitsID("/api/react/platform/tenants/42/channel-limits")
	if !ok || id != 42 {
		t.Fatalf("rota válida não reconhecida: id=%d ok=%v", id, ok)
	}
	for _, path := range []string{
		"/api/react/platform/tenants/x/channel-limits",
		"/api/react/platform/tenants/0/channel-limits",
		"/api/react/platform/tenants/42/limits",
	} {
		if _, ok := saasPlatformTenantChannelLimitsID(path); ok {
			t.Fatalf("rota inválida aceita: %s", path)
		}
	}
}
