package agent

import (
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/llm"
)

func TestNotreveModelDefaults(t *testing.T) {
	cases := []struct {
		name       string
		model      string
		capacidade string
		want       string
	}{
		{"texto sem modelo", "", "texto", llm.NotreveChatModel},
		{"multimodal sem modelo", "", "multimodal", llm.NotreveVisionModel},
		{"modelo escolhido", "notreve-vision", "texto", "notreve-vision"},
		{"modelo customizado", "meu-modelo", "texto", "meu-modelo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := notreveModel(tc.model, tc.capacidade); got != tc.want {
				t.Fatalf("notreveModel(%q, %q) = %q, esperado %q", tc.model, tc.capacidade, got, tc.want)
			}
		})
	}
}
