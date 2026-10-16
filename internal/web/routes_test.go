package web

import "testing"

// Garante que o registro das rotas não entra em conflito (o ServeMux do Go
// entra em pânico ao registrar padrões ambíguos).
func TestRoutesRegistersWithoutPanic(t *testing.T) {
	s := &Server{}
	if handler := s.Routes(); handler == nil {
		t.Fatal("Routes devolveu handler nulo")
	}
}
