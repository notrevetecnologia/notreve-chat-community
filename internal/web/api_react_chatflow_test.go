package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateChatflowStepsAcceptsExactlyOneStart(t *testing.T) {
	steps := []reactChatflowStepInput{
		{Ord: 1, Tipo: "start", Config: map[string]interface{}{"next": 2}},
		{Ord: 2, Tipo: "message", Config: map[string]interface{}{"text": "oi"}},
	}
	if err := validateChatflowSteps(steps); err != nil {
		t.Fatalf("fluxo com um start deveria ser aceito, obteve %v", err)
	}
}

func TestValidateChatflowStepsRejectsZeroStart(t *testing.T) {
	steps := []reactChatflowStepInput{
		{Ord: 1, Tipo: "message", Config: map[string]interface{}{"text": "oi"}},
	}
	if err := validateChatflowSteps(steps); err == nil {
		t.Fatal("fluxo sem start deveria ser rejeitado")
	}
}

func TestValidateChatflowStepsRejectsTwoStarts(t *testing.T) {
	steps := []reactChatflowStepInput{
		{Ord: 1, Tipo: "start", Config: map[string]interface{}{"next": 2}},
		{Ord: 2, Tipo: "start", Config: map[string]interface{}{"next": 3}},
	}
	if err := validateChatflowSteps(steps); err == nil {
		t.Fatal("fluxo com dois start deveria ser rejeitado")
	}
}

func TestDecodeReactChatflowInputKeepsStartStep(t *testing.T) {
	body := `{"nome":"Fluxo","steps":[{"ord":1,"tipo":"start","config":{"next":2}},{"ord":2,"tipo":"end","config":{}}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/react/chatflows", strings.NewReader(body))
	input, err := decodeReactChatflowInput(req)
	if err != nil {
		t.Fatal(err)
	}
	if input.Steps == nil || len(*input.Steps) != 2 {
		t.Fatalf("esperava 2 passos, obteve %+v", input.Steps)
	}
	if (*input.Steps)[0].Tipo != "start" {
		t.Fatalf("tipo do primeiro passo = %q, esperado start", (*input.Steps)[0].Tipo)
	}
}

func TestChatflowInputJSONFieldNames(t *testing.T) {
	raw, err := json.Marshal(reactChatflowStepInput{Ord: 3, Tipo: "start", Config: map[string]interface{}{"next": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"tipo":"start"`) {
		t.Fatalf("campo tipo deveria serializar como start, obteve %s", raw)
	}
}
