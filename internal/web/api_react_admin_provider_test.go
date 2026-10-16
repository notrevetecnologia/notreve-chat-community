package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/llm"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestProviderTestBaseURLUsesManagedNotreveGateway(t *testing.T) {
	notreve := store.Provider{ProviderType: "notreve", BaseURL: "https://ia.notreve.com.br/v1"}
	if got := providerTestBaseURL(notreve); got != llm.NotreveBaseURL {
		t.Fatalf("base URL do teste notreve = %q, esperado %q", got, llm.NotreveBaseURL)
	}
	custom := store.Provider{ProviderType: "openai_compat", BaseURL: "https://api.openai.com/v1/"}
	if got := providerTestBaseURL(custom); got != "https://api.openai.com/v1" {
		t.Fatalf("base URL do teste custom = %q", got)
	}
}

func TestApplyReactProviderInputNotreveUsesManagedGateway(t *testing.T) {
	provider := store.Provider{}
	input := reactProviderInput{
		Name:         stringPointer("Notreve IA"),
		ProviderType: stringPointer("notreve"),
		APIKey:       stringPointer("token-notreve"),
	}
	if err := applyReactProviderInput(&provider, input, true); err != nil {
		t.Fatal(err)
	}
	if provider.BaseURL != llm.NotreveBaseURL {
		t.Fatalf("base_url do preset = %q, esperado %q", provider.BaseURL, llm.NotreveBaseURL)
	}
	if provider.Model != llm.NotreveChatModel {
		t.Fatalf("modelo padrão do preset = %q, esperado %q", provider.Model, llm.NotreveChatModel)
	}
}

func TestApplyReactProviderInputNotreveKeepsChosenModel(t *testing.T) {
	provider := store.Provider{}
	input := reactProviderInput{
		Name:         stringPointer("Notreve IA"),
		ProviderType: stringPointer("notreve"),
		Model:        stringPointer("notreve-vision"),
		APIKey:       stringPointer("token-notreve"),
	}
	if err := applyReactProviderInput(&provider, input, true); err != nil {
		t.Fatal(err)
	}
	if provider.Model != "notreve-vision" {
		t.Fatalf("modelo escolhido = %q, esperado %q", provider.Model, "notreve-vision")
	}
}

func TestListProviderModelsOpenAICompat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer token-ok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"id":"gpt-4o","name":"GPT-4o"},{"id":"gpt-4o-mini"}]}`)
	}))
	defer server.Close()

	models, err := listProviderModels(store.Provider{ProviderType: "openai_compat", BaseURL: server.URL, APIKey: "token-ok"})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("modelos = %d, esperado 2", len(models))
	}
	if models[0].ID != "gpt-4o" || models[0].Name != "GPT-4o" {
		t.Fatalf("primeiro modelo = %+v", models[0])
	}
	if models[1].ID != "gpt-4o-mini" || models[1].Name != "gpt-4o-mini" {
		t.Fatalf("segundo modelo = %+v", models[1])
	}
}

func TestListProviderModelsTokenInvalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := listProviderModels(store.Provider{ProviderType: "openai_compat", BaseURL: server.URL, APIKey: "token-bad"})
	if err == nil || !strings.Contains(err.Error(), "token inválido") {
		t.Fatalf("esperado erro de token inválido, veio %v", err)
	}
}
