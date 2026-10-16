package store

import (
	"context"
	"errors"
	"testing"
)

func TestClassifyAgentFailure(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{err: nil, want: FailureKindOther},
		{err: errors.New("LLM HTTP 500: provedor indisponível"), want: FailureKindLLM},
		{err: errors.New("modelo da IA está vazio"), want: FailureKindLLM},
		{err: errors.New("timeout ao consultar o provedor"), want: FailureKindLLM},
		{err: errors.New("falha ao executar ferramenta de consulta"), want: FailureKindTool},
		{err: errors.New("envio pelo canal whatsapp falhou"), want: FailureKindChannel},
		{err: errors.New("conversa sem mensagens"), want: FailureKindPipeline},
	}
	for _, testCase := range cases {
		if got := ClassifyAgentFailure(testCase.err); got != testCase.want {
			t.Fatalf("erro %v: classificação %q, esperado %q", testCase.err, got, testCase.want)
		}
	}
}

func TestRecordAgentFailureRequiresTenant(t *testing.T) {
	s := &Store{}
	if err := s.RecordAgentFailure(context.Background(), AgentFailure{}); !errors.Is(err, ErrTenantContextRequired) {
		t.Fatalf("esperava erro de tenant obrigatório, obtido %v", err)
	}
}
