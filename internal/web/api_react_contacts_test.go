package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestNormalizeContactTagsTrimsAndDedupes(t *testing.T) {
	tags, err := normalizeContactTags([]string{" vip ", "VIP", "", "atacado"})
	if err != nil {
		t.Fatalf("tags válidas não deveriam falhar: %v", err)
	}
	if len(tags) != 2 || tags[0] != "vip" || tags[1] != "atacado" {
		t.Fatalf("tags normalizadas incorretas: %v", tags)
	}
}

func TestNormalizeContactTagsRejectsLong(t *testing.T) {
	long := strings.Repeat("a", maxContactTagLength+1)
	if _, err := normalizeContactTags([]string{long}); err == nil {
		t.Fatal("tag acima do limite deveria ser rejeitada")
	}
	many := make([]string, maxContactTags+1)
	for i := range many {
		many[i] = "tag" + string(rune('a'+i%26)) + strings.Repeat("x", i)
	}
	if _, err := normalizeContactTags(many); err == nil {
		t.Fatal("quantidade acima do limite deveria ser rejeitada")
	}
}

func TestNormalizeContactCamposExtrasTrimsAndFilters(t *testing.T) {
	campos, err := normalizeContactCamposExtras(map[string]string{"  plano ": "  premium ", "": "ignorar", "cidade": "São Paulo"})
	if err != nil {
		t.Fatalf("campos válidos não deveriam falhar: %v", err)
	}
	if len(campos) != 2 {
		t.Fatalf("esperava 2 campos, obteve %d: %v", len(campos), campos)
	}
	if campos["plano"] != "premium" || campos["cidade"] != "São Paulo" {
		t.Fatalf("campos normalizados incorretos: %v", campos)
	}
}

func TestNormalizeContactCamposExtrasRejectsLongValue(t *testing.T) {
	value := strings.Repeat("v", maxContactExtraValue+1)
	if _, err := normalizeContactCamposExtras(map[string]string{"campo": value}); err == nil {
		t.Fatal("valor acima do limite deveria ser rejeitado")
	}
}

func TestDecodeReactContactInputReadsTagsAndExtras(t *testing.T) {
	body := `{"name":"Ana","email":"ana@exemplo.com","notes":"cliente fiel","tags":["vip","recompra"],"campos_extras":{"plano":"ouro"}}`
	request := httptest.NewRequest(http.MethodPatch, "/api/react/contacts/1", strings.NewReader(body))
	input, err := decodeReactContactInput(request)
	if err != nil {
		t.Fatalf("decode falhou: %v", err)
	}
	if input.Notes == nil || *input.Notes != "cliente fiel" {
		t.Fatalf("observações não decodificadas: %+v", input.Notes)
	}
	if input.Tags == nil || len(*input.Tags) != 2 {
		t.Fatalf("tags não decodificadas: %+v", input.Tags)
	}
	if input.CamposExtras == nil || (*input.CamposExtras)["plano"] != "ouro" {
		t.Fatalf("campos extras não decodificados: %+v", input.CamposExtras)
	}
}

func TestParseContactTagsAndCamposExtrasView(t *testing.T) {
	if got := parseContactTags("vip, vip ,, atacado"); len(got) != 2 || got[0] != "vip" || got[1] != "atacado" {
		t.Fatalf("parseContactTags incorreto: %v", got)
	}
	if got := contactCamposExtras(store.Contato{}); len(got) != 0 {
		t.Fatalf("campos extras nulos deveriam virar mapa vazio: %v", got)
	}
}
