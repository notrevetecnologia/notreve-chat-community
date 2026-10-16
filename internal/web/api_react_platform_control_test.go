package web

import (
	"net/http"
	"strings"
	"testing"
)

func TestPlatformTenantInputFromReactMapsCreditsAndDomain(t *testing.T) {
	input := platformTenantInputFromReact(reactPlatformTenantInput{
		Name: "  Empresa X  ", Slug: "Empresa-X", Status: "archived", Plan: "  pro ",
		Timezone: "America/Sao_Paulo", Locale: "", Dominio: " https://empresa.com.br ", Creditos: 120,
	})
	if input.Slug != "empresa-x" || input.Name != "Empresa X" {
		t.Fatalf("nome/slug não normalizados: %#v", input)
	}
	if input.Status != "closed" {
		t.Fatalf("status archived deveria virar closed, got %q", input.Status)
	}
	if input.Locale != "pt-BR" {
		t.Fatalf("locale sem valor deveria ser pt-BR, got %q", input.Locale)
	}
	if input.Dominio != "empresa.com.br" {
		t.Fatalf("domínio não normalizado: %q", input.Dominio)
	}
	if input.Creditos != 120 {
		t.Fatalf("créditos não preservados: %d", input.Creditos)
	}
}

func TestDecodeReactContextSwitchTrimsReason(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "/api/react/platform/tenants/1/switch", strings.NewReader(`{"reason":"  investigar relatório  "}`))
	decoded, err := decodeReactContextSwitch(req)
	if err != nil {
		t.Fatalf("decode falhou: %v", err)
	}
	if decoded.Reason != "investigar relatório" {
		t.Fatalf("motivo não normalizado: %q", decoded.Reason)
	}
}
