package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestSummaryTranscriptKeepsRecentAndLabels(t *testing.T) {
	history := []store.Message{
		{Role: "user", Content: "primeira mensagem"},
		{Role: "assistant", Content: "resposta antiga"},
		{Role: "system", Content: "protocolo aberto"},
		{Role: "user", Content: "quero falar com um humano"},
		{Role: "assistant", Content: "vou transferir você"},
	}
	got := summaryTranscript(history)
	if strings.Contains(got, "protocolo aberto") {
		t.Fatalf("mensagens de sistema não devem entrar no resumo: %q", got)
	}
	if !strings.Contains(got, "Cliente: quero falar com um humano") {
		t.Fatalf("mensagem recente do cliente ausente: %q", got)
	}
	if !strings.Contains(got, "Atendimento: vou transferir você") {
		t.Fatalf("resposta da IA ausente: %q", got)
	}
	if strings.Index(got, "primeira mensagem") > strings.Index(got, "quero falar com um humano") {
		t.Fatalf("ordem cronológica quebrada: %q", got)
	}
}

func TestCleanSummaryLimitsLinesAndBullets(t *testing.T) {
	raw := "- Cliente quer segunda via\n* Já foi resolvido o cadastro\n• Falta enviar o boleto\n4\n5\n6\n7\n8"
	got := cleanSummary(raw)
	lines := strings.Split(got, "\n")
	if len(lines) != summaryMaxLines {
		t.Fatalf("esperava %d linhas, obteve %d: %q", summaryMaxLines, len(lines), got)
	}
	if strings.HasPrefix(lines[0], "-") || strings.HasPrefix(lines[0], "*") {
		t.Fatalf("marcadores de lista deveriam ser removidos: %q", got)
	}
}

func TestCleanSummaryEmpty(t *testing.T) {
	if got := cleanSummary("   \n  "); got != "" {
		t.Fatalf("resumo vazio deveria continuar vazio, obteve %q", got)
	}
}

func TestSummarizeConversationWithoutEngineIsSafe(t *testing.T) {
	if _, err := (&Engine{}).SummarizeConversation(context.Background(), 1, store.Agent{}); err == nil {
		t.Fatal("motor sem store deveria retornar erro em vez de entrar em pânico")
	}
}
