package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReactPageSlice(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7}
	cases := []struct {
		limit  int
		offset int
		want   []int
	}{
		{limit: 3, offset: 0, want: []int{1, 2, 3}},
		{limit: 3, offset: 3, want: []int{4, 5, 6}},
		{limit: 3, offset: 6, want: []int{7}},
		{limit: 3, offset: 7, want: []int{}},
		{limit: 5, offset: 20, want: []int{}},
	}
	for _, tc := range cases {
		got, err := reactPageSlice(items, tc.limit, tc.offset)
		if err != nil {
			t.Fatalf("limit=%d offset=%d: erro inesperado: %v", tc.limit, tc.offset, err)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("limit=%d offset=%d: tamanho=%d esperado=%d", tc.limit, tc.offset, len(got), len(tc.want))
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Fatalf("limit=%d offset=%d: item %d=%d esperado=%d", tc.limit, tc.offset, i, got[i], tc.want[i])
			}
		}
	}
	if _, err := reactPageSlice(items, 3, -1); err == nil {
		t.Fatal("offset negativo deveria retornar erro")
	}
}

func TestReactPageMeta(t *testing.T) {
	meta := reactPageMeta(100, 25, 0, map[string]string{"status": "novo"})
	if meta["total"] != 100 || meta["limit"] != 25 || meta["has_more"] != true {
		t.Fatalf("meta inesperada: %+v", meta)
	}
	next, _ := meta["next_cursor"].(string)
	if offset, err := decodeReactCursor(next); err != nil || offset != 25 {
		t.Fatalf("next_cursor=%q offset=%d erro=%v", next, offset, err)
	}
	filters, ok := meta["filters"].(map[string]string)
	if !ok || filters["status"] != "novo" {
		t.Fatalf("filtros não propagados: %+v", meta["filters"])
	}
	last := reactPageMeta(100, 25, 75, nil)
	if last["has_more"] != false || last["next_cursor"] != "" {
		t.Fatalf("última página inesperada: %+v", last)
	}
	if _, ok := last["filters"]; ok {
		t.Fatalf("meta sem filtros não deveria conter a chave filters: %+v", last)
	}
}

func TestParseReactPage(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/react/contacts", nil)
	page, err := parseReactPage(r)
	if err != nil || page.Limit != 25 || page.Offset != 0 {
		t.Fatalf("default inesperado: %+v erro=%v", page, err)
	}
	r = httptest.NewRequest(http.MethodGet, "/api/react/contacts?limit=10&cursor="+encodeReactCursor(30), nil)
	page, err = parseReactPage(r)
	if err != nil || page.Limit != 10 || page.Offset != 30 {
		t.Fatalf("paginação inesperada: %+v erro=%v", page, err)
	}
	r = httptest.NewRequest(http.MethodGet, "/api/react/contacts?limit=0", nil)
	if _, err := parseReactPage(r); err == nil {
		t.Fatal("limit=0 deveria retornar erro")
	}
	r = httptest.NewRequest(http.MethodGet, "/api/react/contacts?cursor=invalido", nil)
	if _, err := parseReactPage(r); err == nil {
		t.Fatal("cursor inválido deveria retornar erro")
	}
}

func TestReactContainsFoldAppliesFilter(t *testing.T) {
	values := []string{"Campanha de Verão", "whatsapp"}
	if !reactContainsFold(values, "verão") {
		t.Fatal("busca deveria encontrar ignorando maiúsculas/acentuação de caixa")
	}
	if !reactContainsFold(values, "WHATS") {
		t.Fatal("busca deveria ser case-insensitive")
	}
	if reactContainsFold(values, "inexistente") {
		t.Fatal("busca sem correspondência deveria ser filtrada")
	}
	if !reactContainsFold(values, "  ") {
		t.Fatal("busca vazia deveria aceitar todos os itens")
	}
}

func TestCampaignPeriodStart(t *testing.T) {
	if _, ok := campaignPeriodStart(""); ok {
		t.Fatal("período vazio não deveria filtrar")
	}
	for _, period := range []string{"7", "30", "90"} {
		start, ok := campaignPeriodStart(period)
		if !ok || !start.Before(time.Now()) {
			t.Fatalf("período %q deveria gerar data no passado: %v ok=%v", period, start, ok)
		}
	}
}

func TestReactQueryFilterDropsEmptyValues(t *testing.T) {
	filter := reactQueryFilter(map[string]string{"q": "  contato ", "status": "", "period": "7"})
	if len(filter) != 2 || filter["q"] != "contato" || filter["period"] != "7" {
		t.Fatalf("filtros inesperados: %+v", filter)
	}
	if _, ok := filter["status"]; ok {
		t.Fatalf("valor vazio não deveria entrar nos filtros: %+v", filter)
	}
}
