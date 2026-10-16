package channel

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestFindOrCreateConversationSkipsInvalidWhatsAppPhone(t *testing.T) {
	proc := NewProcessor(nil, nil, nil)
	for _, phone := range []string{"", "123456789@lid", "123456789@newsletter", "status@broadcast", "0000000000000"} {
		if _, _, err := proc.findOrCreateConversation(context.Background(), Inbound{Channel: "whatsapp", Phone: phone}, nil); !errors.Is(err, errConversationPhoneInvalid) {
			t.Fatalf("telefone %q deveria ser ignorado, err=%v", phone, err)
		}
	}
}

func TestInboundMatchesConversationUsesExplicitChannel(t *testing.T) {
	channelID := 12
	otherChannelID := 13
	base := store.Conversation{Channel: "whatsapp", ChannelID: &channelID}

	if !inboundMatchesConversation(base, Inbound{Channel: "whatsapp", ChannelID: &channelID}) {
		t.Fatal("o mesmo canal deveria reutilizar a conversa")
	}
	if inboundMatchesConversation(base, Inbound{Channel: "whatsapp", ChannelID: &otherChannelID}) {
		t.Fatal("canais diferentes não podem reutilizar a conversa")
	}
	if inboundMatchesConversation(store.Conversation{Channel: "whatsapp"}, Inbound{Channel: "whatsapp", ChannelID: &channelID}) {
		t.Fatal("canal explícito não deve reutilizar conversa legada sem vínculo")
	}
	if !inboundMatchesConversation(store.Conversation{Channel: "whatsapp"}, Inbound{Channel: "whatsapp"}) {
		t.Fatal("fluxo legado sem canal explícito deve continuar compatível")
	}
}

func TestSatisfactionTargetFollowsActualParticipants(t *testing.T) {
	tests := []struct {
		hasAI    bool
		hasHuman bool
		want     string
	}{
		{hasAI: true, hasHuman: false, want: "ia"},
		{hasAI: false, hasHuman: true, want: "operador"},
		{hasAI: true, hasHuman: true, want: "ia"},
		{hasAI: false, hasHuman: false, want: "ia"},
	}
	for _, test := range tests {
		if got := satisfactionTarget(test.hasAI, test.hasHuman); got != test.want {
			t.Fatalf("IA=%v humano=%v: esperado %q, obtido %q", test.hasAI, test.hasHuman, test.want, got)
		}
	}
}

func TestEndConversationIntent(t *testing.T) {
	for _, input := range []string{
		"quero encerrar",
		"Quero encerrar o atendimento!",
		"pode finalizar a conversa?",
		"  encerrar   atendimento  ",
	} {
		if !isEndConversationRequest(input) {
			t.Fatalf("pedido de encerramento não reconhecido: %q", input)
		}
	}
	for _, input := range []string{
		"como encerrar minha conta?",
		"não quero encerrar",
		"quero encerrar o plano",
		"até logo",
	} {
		if isEndConversationRequest(input) {
			t.Fatalf("falso positivo de encerramento: %q", input)
		}
	}
}

func TestProcessorFallbackDoesNotQueueHuman(t *testing.T) {
	reply := (&Processor{}).fallbackReply(context.Background())
	if reply.NeedsHuman {
		t.Fatal("erro interno do processador não pode colocar a conversa na fila humana")
	}
	lower := strings.ToLower(reply.Text)
	if strings.Contains(lower, "transfer") || strings.Contains(lower, "encaminh") {
		t.Fatalf("fallback técnico não deve prometer transferência: %q", reply.Text)
	}
	if strings.TrimSpace(reply.Text) == "" {
		t.Fatal("fallback técnico precisa responder ao cliente")
	}
}

func TestNoAgentReplyDoesNotLeakInternalText(t *testing.T) {
	reply := (&Processor{}).noAgentReply()
	if strings.TrimSpace(reply.Text) != "" {
		t.Fatalf("sem agente não pode enviar texto interno ao cliente, obtido: %q", reply.Text)
	}
	if !reply.RecordLearning {
		t.Fatal("ausência de agente deve ser registrada para aprendizado")
	}
}
