package web

import (
	"context"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestValidateReactChannelMode(t *testing.T) {
	cases := map[string]string{
		"":           "ai",
		"ai":         "ai",
		"AI":         "ai",
		" chatflow ": "chatflow",
		"chatflow":   "chatflow",
	}
	for input, expected := range cases {
		got, err := validateReactChannelMode(input)
		if err != nil || got != expected {
			t.Fatalf("modo %q: esperado %q, obteve %q (%v)", input, expected, got, err)
		}
	}
	if _, err := validateReactChannelMode("humano"); err == nil {
		t.Fatal("modo desconhecido deveria ser rejeitado")
	}
}

func TestReactChannelFromStoreExposesModeAndChatflow(t *testing.T) {
	flowID := 12
	channel := reactChannelFromStore(store.Channel{ID: 1, Type: "whatsapp", Name: "Canal", Mode: "chatflow", ChatflowID: &flowID})
	if channel.Mode != "chatflow" || channel.ChatflowID == nil || *channel.ChatflowID != flowID {
		t.Fatalf("modo/flow não expostos: %+v", channel)
	}
	legacy := reactChannelFromStore(store.Channel{ID: 2, Type: "webchat", Name: "Canal 2"})
	if legacy.Mode != "ai" || legacy.ChatflowID != nil {
		t.Fatalf("default deveria ser ai sem flow: %+v", legacy)
	}
}

func TestApplyReactChannelModeClearsFlowOnAI(t *testing.T) {
	flowID := 3
	channel := store.Channel{Mode: "chatflow", ChatflowID: &flowID}
	mode := "ai"
	if err := (&Server{}).applyReactChannelMode(context.Background(), &channel, reactChannelInput{Mode: &mode}); err != nil {
		t.Fatalf("modo ai deveria limpar o flow sem erro: %v", err)
	}
	if channel.Mode != "ai" || channel.ChatflowID != nil {
		t.Fatalf("flow deveria ter sido limpo: %+v (%q)", channel.ChatflowID, channel.Mode)
	}
}

func TestApplyReactChannelModeRequiresFlowForChatflow(t *testing.T) {
	channel := store.Channel{Mode: "ai"}
	mode := "chatflow"
	if err := (&Server{}).applyReactChannelMode(context.Background(), &channel, reactChannelInput{Mode: &mode}); err == nil {
		t.Fatal("chatflow sem fluxo deveria ser rejeitado")
	}
}
