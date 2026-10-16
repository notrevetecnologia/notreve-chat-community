package store

import (
	"context"
	"strings"
	"testing"
)

func TestPlatformControlInputValidation(t *testing.T) {
	validTenant := PlatformTenantInput{
		Slug: "empresa-teste", Name: "Empresa teste", Status: "active", Plan: "pro", Timezone: "America/Sao_Paulo", Locale: "pt-BR",
	}
	if err := validatePlatformTenantInput(validTenant); err != nil {
		t.Fatalf("tenant válido rejeitado: %v", err)
	}
	if got := normalizedPlatformTenantInput(PlatformTenantInput{Slug: " empresa-teste ", Name: " Empresa teste "}); got.Slug != "empresa-teste" || got.Name != "Empresa teste" {
		t.Fatalf("normalização inesperada: %#v", got)
	}

	invalidSlug := validTenant
	invalidSlug.Slug = "Empresa Teste"
	if err := validatePlatformTenantInput(invalidSlug); err == nil {
		t.Fatal("slug inválido foi aceito")
	}

	if err := validateWhatsAppServerInput(WhatsAppServerInput{Name: "pool-a", BaseURL: "https://wa.example", Token: "secret", Status: "active", MaxConnections: 10}, true); err != nil {
		t.Fatalf("servidor válido rejeitado: %v", err)
	}
	if err := validateWhatsAppServerInput(WhatsAppServerInput{Name: "pool-a", BaseURL: "https://wa.example", Status: "active", MaxConnections: 10}, true); err == nil {
		t.Fatal("servidor sem token foi aceito na criação")
	}
	if err := validateWhatsAppServerInput(WhatsAppServerInput{Name: "pool-sem-teto", BaseURL: "https://wa.example", Token: "secret", Status: "maintenance", MaxConnections: 0}, true); err != nil {
		t.Fatalf("capacidade ilimitada válida rejeitada: %v", err)
	}
	if err := validateWhatsAppServerInput(WhatsAppServerInput{Name: "pool-a", BaseURL: "https://wa.example", Token: "secret", Status: "invalid", MaxConnections: 1}, true); err == nil {
		t.Fatal("status inválido foi aceito")
	}
}

func TestPlatformTenantCreditsAndDomainValidation(t *testing.T) {
	valid := PlatformTenantInput{
		Slug: "empresa-teste", Name: "Empresa teste", Status: "active", Plan: "pro",
		Timezone: "America/Sao_Paulo", Locale: "pt-BR", Dominio: "empresa.com.br", Creditos: 250,
	}
	if err := validatePlatformTenantInput(valid); err != nil {
		t.Fatalf("empresa com domínio e créditos válidos rejeitada: %v", err)
	}
	if got := normalizedPlatformTenantInput(PlatformTenantInput{Dominio: " https://empresa.com.br "}).Dominio; got != "empresa.com.br" {
		t.Fatalf("normalização de domínio inesperada: %q", got)
	}

	negativeCredits := valid
	negativeCredits.Creditos = -1
	if err := validatePlatformTenantInput(negativeCredits); err == nil {
		t.Fatal("créditos negativos foram aceitos")
	}

	invalidDomain := valid
	invalidDomain.Dominio = "empresa"
	if err := validatePlatformTenantInput(invalidDomain); err == nil {
		t.Fatal("domínio sem ponto foi aceito")
	}

	invalidDomainChars := valid
	invalidDomainChars.Dominio = "empresa_domain.com.br"
	if err := validatePlatformTenantInput(invalidDomainChars); err == nil {
		t.Fatal("domínio com caractere inválido foi aceito")
	}
}

func TestContextSwitchValidation(t *testing.T) {
	var st Store
	if _, err := st.RecordContextSwitch(context.Background(), Usuario{ID: 1}, 1, 0, "motivo"); err == nil {
		t.Fatal("troca sem tenant de destino foi aceita")
	}
	if _, err := st.RecordContextSwitch(context.Background(), Usuario{}, 1, 2, "motivo"); err == nil {
		t.Fatal("troca sem ator foi aceita")
	}
	reason := strings.Repeat("x", 501)
	if _, err := st.RecordContextSwitch(context.Background(), Usuario{ID: 1}, 1, 2, reason); err == nil {
		t.Fatal("motivo acima do limite foi aceito")
	}
}

func TestCoreModulesRemainMandatory(t *testing.T) {
	if !isCoreModule("canais") || !isCoreModule("equipe") {
		t.Fatal("módulos essenciais devem ser reconhecidos")
	}
	if isCoreModule("campanhas") || isCoreModule("tools") {
		t.Fatal("módulos contratáveis não podem ser marcados como essenciais")
	}
}
