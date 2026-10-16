package agent

import (
	"strings"
	"testing"
)

func TestRichButtonsFallbackNumbers(t *testing.T) {
	rich := buildRichButtons("Escolha uma opção:", []string{"Suporte", "Vendas"})
	if rich == nil {
		t.Fatal("menu deve gerar mensagem rica")
	}
	if rich.Kind != "buttons" || len(rich.Buttons) != 2 {
		t.Fatalf("opções incorretas: %+v", rich)
	}
	fallback := rich.FallbackText()
	if !strings.Contains(fallback, "1. Suporte") || !strings.Contains(fallback, "2. Vendas") {
		t.Fatalf("fallback deve numerar as opções: %q", fallback)
	}
}

func TestParseOpcoesMarker(t *testing.T) {
	rich, rest := parseRichMarkers("Olá! Escolha:\n[OPCOES:Menu|Planos|Suporte]")
	if rich == nil || rich.Kind != "buttons" {
		t.Fatalf("deveria parsear botões: %+v", rich)
	}
	if len(rich.Buttons) != 2 {
		t.Fatalf("esperava 2 botões, obtive %d", len(rich.Buttons))
	}
	if strings.Contains(rest, "[OPCOES") {
		t.Fatalf("marcador não removido: %q", rest)
	}
	if rich.FallbackText() == "" {
		t.Fatal("fallback não pode ser vazio")
	}
}

func TestParseListaMarker(t *testing.T) {
	rich, _ := parseRichMarkers("[LISTA:Menu|Suporte,Atendimento|Vendas,Comercial]")
	if rich == nil || rich.Kind != "list" {
		t.Fatalf("deveria parsear lista: %+v", rich)
	}
	if len(rich.Sections) != 1 || len(rich.Sections[0].Rows) != 2 {
		t.Fatalf("seções/linhas incorretas: %+v", rich.Sections)
	}
}

func TestParseEnqueteMarker(t *testing.T) {
	rich, _ := parseRichMarkers("[ENQUETE:qual canal?|WhatsApp,E-mail,Telefone|multiple]")
	if rich == nil || rich.Kind != "poll" || rich.Poll == nil {
		t.Fatalf("deveria parsear enquete: %+v", rich)
	}
	if !rich.Poll.Multiple || len(rich.Poll.Options) != 3 {
		t.Fatalf("enquete incorreta: %+v", rich.Poll)
	}
}

func TestParseContactMarker(t *testing.T) {
	rich, _ := parseRichMarkers("[CONTATO:João|5511999999999]")
	if rich == nil || rich.Kind != "contact" || rich.Contact == nil {
		t.Fatalf("deveria parsear contato: %+v", rich)
	}
	if rich.Contact.Name != "João" || rich.Contact.Phone != "5511999999999" {
		t.Fatalf("contato incorreto: %+v", rich.Contact)
	}
}

func TestParseLocationMarker(t *testing.T) {
	rich, _ := parseRichMarkers("[LOCALIZACAO:-23.5|-46.6|Loja Centro]")
	if rich == nil || rich.Kind != "location" || rich.Location == nil {
		t.Fatalf("deveria parsear localização: %+v", rich)
	}
	if rich.Location.Latitude != -23.5 || rich.Location.Longitude != -46.6 {
		t.Fatalf("coordenadas incorretas: %+v", rich.Location)
	}
}

func TestNoMarkerReturnsText(t *testing.T) {
	rich, rest := parseRichMarkers("Sou uma resposta simples.")
	if rich != nil {
		t.Fatalf("não deve haver conteúdo rico: %+v", rich)
	}
	if rest != "Sou uma resposta simples." {
		t.Fatalf("texto deve permanecer: %q", rest)
	}
}

func TestRichInvalidKinds(t *testing.T) {
	rich := &RichMessage{Kind: "unknown"}
	if rich.Valid() {
		t.Fatal("tipo desconhecido não é válido")
	}
}
