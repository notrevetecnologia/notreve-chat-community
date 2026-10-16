package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestReactFailureKindValidation(t *testing.T) {
	for _, kind := range []string{"llm", "tool", "channel", "pipeline", "outros"} {
		if !validReactFailureKind(kind) {
			t.Fatalf("tipo %q deveria ser válido", kind)
		}
	}
	for _, kind := range []string{"", "desconhecido", "LLM"} {
		if validReactFailureKind(kind) {
			t.Fatalf("tipo %q não deveria ser válido", kind)
		}
	}
}

func TestReactAgentFailureFromStore(t *testing.T) {
	conversationID := 7
	resolvedAt := time.Now()
	item := reactAgentFailureFromStore(store.AgentFailure{
		ID: 3, ConversationID: &conversationID, Tipo: store.FailureKindLLM,
		CreatedAt: time.Now(), ResolvidoEm: &resolvedAt, ResolvidoPor: "admin",
	})
	if !item.Resolvido || item.ResolvidoEm == "" || item.CreatedAt == "" {
		t.Fatalf("conversão inesperada: %#v", item)
	}
	open := reactAgentFailureFromStore(store.AgentFailure{ID: 4, CreatedAt: time.Now()})
	if open.Resolvido || open.ResolvidoEm != "" {
		t.Fatalf("falha aberta não deveria ter resolução: %#v", open)
	}
}

func TestReactAgentFailureResolveRejectsInvalidID(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/react/agent-failures/abc/resolve", strings.NewReader(`{}`))
	r.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	(&Server{}).handleReactAgentFailureResolve(rr, r)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status obtido=%d esperado=%d", rr.Code, http.StatusBadRequest)
	}
}

func TestReactAgentFailuresRequirePermission(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/react/agent-failures", nil)
	r = r.WithContext(context.WithValue(r.Context(), ctxUser, &store.Usuario{Role: "atendente", Permissions: "conversas_ver"}))
	rr := httptest.NewRecorder()
	(&Server{}).requireReactPerm(permAprendizadoVer, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler não deveria ser chamado sem permissão")
	})).ServeHTTP(rr, r)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status obtido=%d esperado=%d", rr.Code, http.StatusForbidden)
	}
}
