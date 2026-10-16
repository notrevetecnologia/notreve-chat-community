package agent

import (
	"context"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type fakeChatflowResolver struct {
	conv  store.Conversation
	flow  *store.Chatflow
	steps []store.ChatflowStep
}

func (f fakeChatflowResolver) GetConversation(context.Context, int) (store.Conversation, error) {
	return f.conv, nil
}

func (f fakeChatflowResolver) ResolveActiveChatflow(context.Context, *int) (*store.Chatflow, []store.ChatflowStep, error) {
	return f.flow, f.steps, nil
}

func TestResolveChatflowUsesActiveFlowForChannel(t *testing.T) {
	flowID := 7
	flow := &store.Chatflow{ID: flowID, Ativo: true}
	steps := []store.ChatflowStep{{ChatflowID: flowID, Ord: 1, Tipo: "start"}}
	resolver := fakeChatflowResolver{conv: store.Conversation{Status: "ia"}, flow: flow, steps: steps}

	got, gotSteps, err := resolveChatflowFor(context.Background(), resolver, 1, ChatflowTarget{Mode: "chatflow", ChatflowID: &flowID})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got == nil || got.ID != flowID || len(gotSteps) != 1 {
		t.Fatalf("esperava o flow ativo, obtive %+v com %d passos", got, len(gotSteps))
	}
}

func TestResolveChatflowFallsBackToFreeAIWhenChannelModeIsAI(t *testing.T) {
	flowID := 7
	flow := &store.Chatflow{ID: flowID, Ativo: true}
	resolver := fakeChatflowResolver{conv: store.Conversation{Status: "ia"}, flow: flow}

	got, _, err := resolveChatflowFor(context.Background(), resolver, 1, ChatflowTarget{Mode: "ai", ChatflowID: &flowID})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got != nil {
		t.Fatalf("modo ai deveria usar IA livre, obtive flow %+v", got)
	}
}

func TestResolveChatflowFallsBackToFreeAIWithoutLink(t *testing.T) {
	resolver := fakeChatflowResolver{conv: store.Conversation{Status: "ia"}}

	got, _, err := resolveChatflowFor(context.Background(), resolver, 1, ChatflowTarget{Mode: "chatflow"})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got != nil {
		t.Fatalf("sem vínculo deveria usar IA livre, obtive flow %+v", got)
	}
}

func TestResolveChatflowFallsBackWhenFlowInactive(t *testing.T) {
	flowID := 9
	resolver := fakeChatflowResolver{conv: store.Conversation{Status: "ia"}}

	got, _, err := resolveChatflowFor(context.Background(), resolver, 1, ChatflowTarget{Mode: "chatflow", ChatflowID: &flowID})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got != nil {
		t.Fatalf("flow inativo deveria usar IA livre, obtive flow %+v", got)
	}
}

func TestResolveChatflowIgnoresFlowOutsideAIState(t *testing.T) {
	flowID := 5
	flow := &store.Chatflow{ID: flowID, Ativo: true}
	resolver := fakeChatflowResolver{conv: store.Conversation{Status: "humano_aguardando"}, flow: flow}

	got, _, err := resolveChatflowFor(context.Background(), resolver, 1, ChatflowTarget{Mode: "chatflow", ChatflowID: &flowID})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got != nil {
		t.Fatalf("fora do estado ia não deveria rodar flow, obtive %+v", got)
	}
}
