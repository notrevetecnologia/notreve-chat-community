package store

import (
	"strings"
	"testing"
	"time"
)

func TestLogFilterClauseSemFiltros(t *testing.T) {
	args := []interface{}{1}
	clause, out := logFilterClause(args, LogFilter{})
	if clause != "" {
		t.Fatalf("sem filtros não deveria haver cláusula: %q", clause)
	}
	if len(out) != 1 {
		t.Fatalf("args inesperados: %v", out)
	}
}

func TestLogFilterClauseComFiltros(t *testing.T) {
	from := time.Now().Add(-time.Hour)
	to := time.Now()
	args := []interface{}{1}
	clause, out := logFilterClause(args, LogFilter{Level: "error", Module: "midia", Search: "falha", From: &from, To: &to})
	if !strings.Contains(clause, "level=$2") {
		t.Fatalf("filtro de nível ausente: %q", clause)
	}
	if !strings.Contains(clause, "module=$3") {
		t.Fatalf("filtro de módulo ausente: %q", clause)
	}
	if !strings.Contains(clause, "ts >= $5") || !strings.Contains(clause, "ts <= $6") {
		t.Fatalf("filtro de período ausente: %q", clause)
	}
	if len(out) != 6 {
		t.Fatalf("esperava 6 argumentos, obtive %d: %v", len(out), out)
	}
	if out[1] != "ERROR" {
		t.Fatalf("nível deveria ser normalizado para maiúsculas: %v", out[1])
	}
}
