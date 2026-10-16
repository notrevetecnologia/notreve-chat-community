package web

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestParseReactLogTime(t *testing.T) {
	if value, err := parseReactLogTime("", false); err != nil || value != nil {
		t.Fatalf("vazio deveria devolver nil sem erro: %v %v", value, err)
	}
	value, err := parseReactLogTime("2026-01-02", false)
	if err != nil || value == nil || value.Hour() != 0 || value.Minute() != 0 {
		t.Fatalf("data inválida: %v %v", value, err)
	}
	end, err := parseReactLogTime("2026-01-02", true)
	if err != nil || end == nil || end.Hour() != 23 || end.Minute() != 59 {
		t.Fatalf("fim do dia inválido: %v %v", end, err)
	}
	if _, err := parseReactLogTime("ontem", false); err == nil {
		t.Fatal("data inválida deveria gerar erro")
	}
}

func TestReactLogFilter(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/react/logs?level=error&module=midia&q=falha&from=2026-01-01&to=2026-01-02", nil)
	filter, err := reactLogFilter(req)
	if err != nil {
		t.Fatalf("filtro retornou erro: %v", err)
	}
	if filter.Level != "ERROR" || filter.Module != "midia" || filter.Search != "falha" {
		t.Fatalf("filtro inesperado: %+v", filter)
	}
	if filter.From == nil || filter.To == nil || !filter.To.After(*filter.From) {
		t.Fatalf("período inesperado: %+v", filter)
	}
}

func TestReactLogFilterBuscaLonga(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/react/logs?q="+strings.Repeat("a", maxLogSearchChars+1), nil)
	if _, err := reactLogFilter(req); err == nil {
		t.Fatal("busca acima do limite deveria gerar erro")
	}
}

func TestReactLogsPaginacao(t *testing.T) {
	entries := []store.LogEntry{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}}
	page, err := reactPageSlice(entries, 2, 2)
	if err != nil || len(page) != 2 || page[0].ID != 3 || page[1].ID != 4 {
		t.Fatalf("página inesperada: %+v erro=%v", page, err)
	}
	meta := reactPageMeta(len(entries), 2, 2, map[string]string{"level": "ERROR"})
	if meta["has_more"] != false || meta["next_cursor"] != "" {
		t.Fatalf("meta inesperada: %+v", meta)
	}
	first := reactPageMeta(len(entries), 2, 0, map[string]string{"level": "ERROR"})
	if first["has_more"] != true || first["next_cursor"] == "" {
		t.Fatalf("primeira página deveria ter próximo cursor: %+v", first)
	}
}
