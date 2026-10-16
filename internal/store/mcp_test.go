package store

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/securesecret"
)

func TestDefaultSkillsAreFunctional(t *testing.T) {
	skills := defaultSkills()
	if len(skills) < 3 {
		t.Fatalf("esperado ao menos 3 skills padrão, obtido %d", len(skills))
	}
	seen := map[string]bool{}
	for _, skill := range skills {
		if strings.TrimSpace(skill.Nome) == "" || strings.TrimSpace(skill.Descricao) == "" || strings.TrimSpace(skill.Instrucao) == "" {
			t.Fatalf("skill padrão incompleta: %+v", skill)
		}
		if seen[skill.Nome] {
			t.Fatalf("skill padrão duplicada: %q", skill.Nome)
		}
		seen[skill.Nome] = true
	}
	for _, expected := range []string{"atendimento", "vendas", "suporte"} {
		if !seen[expected] {
			t.Fatalf("skill padrão %q ausente", expected)
		}
	}
}

func TestMCPSecretMapRoundTrip(t *testing.T) {
	t.Setenv("AGENTE_IA_SECRETS_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("k"), 32)))
	input := map[string]string{"Authorization": "Bearer segredo", "X-Trace": "abc"}
	encrypted, err := transformMCPSecretMap(input, "mcp.headers.", false)
	if err != nil {
		t.Fatalf("cifrar falhou: %v", err)
	}
	if !securesecret.IsEncrypted(encrypted["Authorization"]) {
		t.Fatalf("header sensível deveria ser cifrado: %q", encrypted["Authorization"])
	}
	if encrypted["X-Trace"] != "abc" {
		t.Fatalf("header não sensível não deveria ser alterado: %q", encrypted["X-Trace"])
	}
	decrypted, err := decryptMCPSecretMap(encrypted, "mcp.headers.")
	if err != nil {
		t.Fatalf("decifrar falhou: %v", err)
	}
	if decrypted["Authorization"] != "Bearer segredo" {
		t.Fatalf("valor decifrado = %q", decrypted["Authorization"])
	}
}
