package store

import "testing"

func TestNormalizeConversationStatus(t *testing.T) {
	cases := map[string]string{
		"ai":                ConversationStatusIA,
		"":                  ConversationStatusIA,
		"ia":                ConversationStatusIA,
		"human":             ConversationStatusWaiting,
		"aguardando":        ConversationStatusWaiting,
		"humano_aguardando": ConversationStatusWaiting,
		"em_atendimento":    ConversationStatusAttending,
		"atendendo":         ConversationStatusAttending,
		"humano_atendendo":  ConversationStatusAttending,
		"encerrada":         ConversationStatusClosed,
		"encerrado":         ConversationStatusClosed,
		"closed":            ConversationStatusClosed,
		"lixo":              ConversationStatusWaiting,
		"  HUMAN  ":         ConversationStatusWaiting,
	}
	for input, expected := range cases {
		if got := NormalizeConversationStatus(input); got != expected {
			t.Fatalf("normalizando %q: obtido=%q esperado=%q", input, got, expected)
		}
	}
}
