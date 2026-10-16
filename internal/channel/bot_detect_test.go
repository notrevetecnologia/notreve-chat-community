package channel

import "testing"

func TestBotPatternsNormalizes(t *testing.T) {
	got := botPatterns("Test, teste\nBot\nrobô,teste")
	want := []string{"test", "teste", "bot", "robô"}
	if len(got) != len(want) {
		t.Fatalf("esperava %d padrões, obtive %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("padrão %d = %q, esperava %q", i, got[i], want[i])
		}
	}
}

func TestBotPatternMatch(t *testing.T) {
	if !botPatternMatch("olá, isso é um TESTE automatizado", []string{"teste"}) {
		t.Fatal("deveria casar com 'teste'")
	}
	if botPatternMatch("olá, posso ajudar?", []string{"teste", "bot"}) {
		t.Fatal("não deveria casar")
	}
	if botPatternMatch("", []string{"teste"}) {
		t.Fatal("mensagem vazia não casa")
	}
	if botPatternMatch("qualquer coisa", []string{}) {
		t.Fatal("sem padrões não casa")
	}
}

func TestBotPatternsEmpty(t *testing.T) {
	if botPatterns("") != nil || botPatterns("   \n  ") != nil {
		t.Fatal("lista vazia deve devolver nil")
	}
}
