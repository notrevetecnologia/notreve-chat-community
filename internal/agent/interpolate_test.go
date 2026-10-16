package agent

import "testing"

func TestInterpolateVars(t *testing.T) {
	vars := map[string]string{"cpf": "12345678900", "nome": "Maria"}
	if got := interpolateVars("Ola {{nome}}, seu CPF {{cpf}}.", vars); got != "Ola Maria, seu CPF 12345678900." {
		t.Fatalf("interpolacao simples falhou: %q", got)
	}
	if got := interpolateVars("sem tokens", vars); got != "sem tokens" {
		t.Fatalf("texto sem token nao deve mudar: %q", got)
	}
	if got := interpolateVars("{{desconhecida}} x", vars); got != " x" {
		t.Fatalf("variavel desconhecida vira vazio: %q", got)
	}
	if got := interpolateVars("{{nome}}", nil); got != "" {
		t.Fatalf("vars nil nao deve panicar: %q", got)
	}
	if got := interpolateVars("{{nome}} e {{nome}}", vars); got != "Maria e Maria" {
		t.Fatalf("substituicao multipla falhou: %q", got)
	}
}
