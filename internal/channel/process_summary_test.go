package channel

import (
	"context"
	"errors"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestGenerateHandoffSummaryIsBestEffort(t *testing.T) {
	proc := &Processor{}
	proc.generateHandoffSummary(context.Background(), 7, store.Agent{})

	proc.summarize = func(context.Context, int, store.Agent) (string, error) {
		return "", errors.New("IA indisponível")
	}
	proc.generateHandoffSummary(context.Background(), 7, store.Agent{})

	proc.summarize = func(context.Context, int, store.Agent) (string, error) {
		return "   ", nil
	}
	proc.generateHandoffSummary(context.Background(), 7, store.Agent{})

	if proc.summarize == nil {
		t.Fatal("o sumarizador deveria permanecer configurado")
	}
}
