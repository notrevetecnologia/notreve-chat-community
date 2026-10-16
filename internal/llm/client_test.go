package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestCompleteRetriesTransientFailure(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL+"/", "key", "model", 0.7)
	resp, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "oi"}}, nil)
	if err != nil {
		t.Fatalf("Complete retornou erro: %v", err)
	}
	if got := contentFromTest(resp); got != "ok" {
		t.Fatalf("resposta inesperada: %q", got)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("esperava 2 chamadas, obtive %d", calls)
	}
}

func TestCompleteRetriesWithoutTemperatureWhenProviderRejectsIt(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("JSON inválido: %v", err)
		}
		if _, hasTemp := body["temperature"]; hasTemp {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"temperature is not supported for this model"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"sem temperature"}}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key", "reasoning-model", 0.7)
	resp, err := c.Complete(context.Background(), []Message{{Role: "user", Content: "oi"}}, nil)
	if err != nil {
		t.Fatalf("Complete retornou erro: %v", err)
	}
	if got := contentFromTest(resp); got != "sem temperature" {
		t.Fatalf("resposta inesperada: %q", got)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("esperava 2 chamadas, obtive %d", calls)
	}
}

func contentFromTest(resp *ChatResponse) string {
	if resp == nil || len(resp.Choices) == 0 {
		return ""
	}
	s, _ := resp.Choices[0].Message.Content.(string)
	return s
}
