package web

import "testing"

func TestScheduleStatusTransitionAllowed(t *testing.T) {
	cases := []struct {
		from    string
		to      string
		allowed bool
	}{
		{"pendente", "pendente", true},
		{"pendente", "cancelada", true},
		{"executando", "cancelada", true},
		{"falhou", "cancelada", true},
		{"cancelada", "pendente", true},
		{"falhou", "pendente", true},
		{"concluida", "cancelada", false},
		{"concluida", "pendente", false},
		{"cancelada", "cancelada", true},
		{"concluida", "concluida", true},
		{"concluida", "falhou", false},
		{"cancelada", "falhou", false},
	}
	for _, tc := range cases {
		err := scheduleStatusTransitionAllowed(tc.from, tc.to)
		if (err == nil) != tc.allowed {
			t.Fatalf("%s -> %s: err=%v, esperado allowed=%v", tc.from, tc.to, err, tc.allowed)
		}
		if !tc.allowed && err.Error() == "" {
			t.Fatalf("%s -> %s: transição inválida precisa de motivo", tc.from, tc.to)
		}
	}
	if err := scheduleStatusTransitionAllowed("pendente", "voar"); err == nil {
		t.Fatal("status desconhecido deveria ser rejeitado")
	}
}
