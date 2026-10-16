package web

import "testing"

func TestCampaignTransitionAllowed(t *testing.T) {
	cases := []struct {
		from    string
		action  string
		allowed bool
	}{
		{"rascunho", "iniciar", true},
		{"rascunho", "retomar", true},
		{"rascunho", "pausar", false},
		{"rascunho", "cancelar", true},
		{"agendada", "iniciar", true},
		{"agendada", "pausar", false},
		{"agendada", "cancelar", true},
		{"rodando", "iniciar", false},
		{"rodando", "retomar", false},
		{"rodando", "pausar", true},
		{"rodando", "cancelar", true},
		{"pausada", "iniciar", true},
		{"pausada", "retomar", true},
		{"pausada", "pausar", false},
		{"pausada", "cancelar", true},
		{"concluida", "iniciar", false},
		{"concluida", "pausar", false},
		{"concluida", "cancelar", false},
		{"cancelada", "iniciar", false},
		{"cancelada", "retomar", false},
		{"cancelada", "pausar", false},
		{"cancelada", "cancelar", false},
	}
	for _, tc := range cases {
		err := campaignTransitionAllowed(tc.from, tc.action)
		if (err == nil) != tc.allowed {
			t.Fatalf("%s + %s: err=%v, esperado allowed=%v", tc.from, tc.action, err, tc.allowed)
		}
		if !tc.allowed && err.Error() == "" {
			t.Fatalf("%s + %s: transição inválida precisa de motivo", tc.from, tc.action)
		}
	}
	if err := campaignTransitionAllowed("rascunho", "voar"); err == nil {
		t.Fatal("ação desconhecida deveria ser rejeitada")
	}
}

func TestCampaignDeleteAllowed(t *testing.T) {
	for _, status := range []string{"rascunho", "agendada", "pausada", "concluida", "cancelada"} {
		if err := campaignDeleteAllowed(status); err != nil {
			t.Fatalf("status %q deveria permitir exclusão: %v", status, err)
		}
	}
	if err := campaignDeleteAllowed("rodando"); err == nil {
		t.Fatal("campanha em andamento não pode ser excluída sem cancelar")
	}
}
