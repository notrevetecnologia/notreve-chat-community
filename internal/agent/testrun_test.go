package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/llm"
)

// fakeChatServer simula um provider OpenAI-compat sem rede real, devolvendo
// respostas canned em sequência. Valida o header de autorização.
func fakeChatServer(t *testing.T, replies []string) *httptest.Server {
	t.Helper()
	var calls int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer fake-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		index := int(atomic.AddInt32(&calls, 1)) - 1
		if index >= len(replies) {
			index = len(replies) - 1
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(replies[index]))
	}))
}

func TestRunToolLoopReturnsAssistantReply(t *testing.T) {
	server := fakeChatServer(t, []string{
		`{"choices":[{"index":0,"message":{"role":"assistant","content":"Olá! Tudo bem?"}}],"usage":{"prompt_tokens":12,"completion_tokens":4}}`,
	})
	defer server.Close()

	engine := NewEngine(nil)
	pc := providerConfig{BaseURL: server.URL, APIKey: "fake-key", Model: "fake-model", Temp: 0.3}
	out := engine.runToolLoop(context.Background(), pc,
		[]llm.Message{{Role: "user", Content: "oi"}}, nil, nil, 1, 0, "")
	if out.failed {
		t.Fatal("o ciclo não deveria falhar com provider fake")
	}
	if strings.TrimSpace(out.reply) != "Olá! Tudo bem?" {
		t.Fatalf("resposta = %q", out.reply)
	}
	if len(out.used) != 0 {
		t.Fatalf("nenhuma tool deveria ter sido usada, obtido %v", out.used)
	}
}

func TestRunToolLoopResolvesToolCalls(t *testing.T) {
	server := fakeChatServer(t, []string{
		`{"choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"consultar_status","arguments":"{}"}}]}}]}`,
		`{"choices":[{"index":0,"message":{"role":"assistant","content":"Seu pedido está a caminho."}}]}`,
	})
	defer server.Close()

	engine := NewEngine(nil)
	pc := providerConfig{BaseURL: server.URL, APIKey: "fake-key", Model: "fake-model"}
	out := engine.runToolLoop(context.Background(), pc,
		[]llm.Message{{Role: "user", Content: "status do pedido?"}}, nil, nil, 1, 0, "")
	if out.failed {
		t.Fatal("o ciclo com tool não deveria falhar")
	}
	if len(out.used) != 1 || out.used[0] != "consultar_status" {
		t.Fatalf("tools usadas = %v, esperado [consultar_status]", out.used)
	}
	if out.reply != "Seu pedido está a caminho." {
		t.Fatalf("resposta = %q", out.reply)
	}
}

func TestRunToolLoopFailsOnProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	engine := NewEngine(nil)
	pc := providerConfig{BaseURL: server.URL, APIKey: "fake-key", Model: "fake-model"}
	out := engine.runToolLoop(context.Background(), pc,
		[]llm.Message{{Role: "user", Content: "oi"}}, nil, nil, 1, 0, "")
	if !out.failed {
		t.Fatal("erro do provider deveria marcar falha para o chamador cair no fallback")
	}
}
