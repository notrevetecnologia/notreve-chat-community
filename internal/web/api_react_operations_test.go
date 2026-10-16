package web

import "testing"

func TestReactActionPermissionIncludesInboxOperations(t *testing.T) {
	tests := map[string]string{
		"typing":    permConversasResponder,
		"digitando": permConversasResponder,
		"reaction":  permConversasResponder,
		"reagir":    permConversasResponder,
		"transfer":  permConversasTransferir,
		"close":     permConversasEncerrar,
	}
	for action, expected := range tests {
		permission, ok := reactActionPermission(action)
		if !ok || permission != expected {
			t.Fatalf("ação %q retornou %q/%v; esperado %q/true", action, permission, ok, expected)
		}
	}
}

func TestReactActionPermissionRejectsUnknownAction(t *testing.T) {
	if permission, ok := reactActionPermission("send-media"); ok || permission != "" {
		t.Fatalf("ação desconhecida foi aceita: %q/%v", permission, ok)
	}
}

func TestReactContactInputValidation(t *testing.T) {
	if err := validateContactPhone("0000000000000"); err != nil {
		t.Fatalf("telefone válido rejeitado: %v", err)
	}
	if err := validateContactPhone(""); err == nil {
		t.Fatal("telefone vazio foi aceito")
	}
	if err := validateContactName(string(make([]rune, 161))); err == nil {
		t.Fatal("nome acima do limite foi aceito")
	}
}
