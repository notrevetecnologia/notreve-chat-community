package agent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type fakeMediaSender struct {
	sent []GeneratedMedia
	err  error
}

func (f *fakeMediaSender) SendGeneratedMedia(_ context.Context, _ int, media GeneratedMedia) error {
	f.sent = append(f.sent, media)
	return f.err
}

func TestProviderSupportsImage(t *testing.T) {
	cases := []struct {
		name     string
		provider store.Provider
		want     bool
	}{
		{"capability", store.Provider{Capabilities: []string{"imagem"}}, true},
		{"openrouter-host", store.Provider{BaseURL: "https://openrouter.ai/api/v1"}, true},
		{"text-only", store.Provider{BaseURL: "https://api.openai.com/v1"}, false},
		{"notreve", store.Provider{ProviderType: "notreve"}, false},
	}
	for _, tc := range cases {
		if got := providerSupportsImage(tc.provider); got != tc.want {
			t.Fatalf("%s: providerSupportsImage = %v, esperado %v", tc.name, got, tc.want)
		}
	}
}

func TestNormalizeAspectRatio(t *testing.T) {
	if got := normalizeAspectRatio("  "); got != defaultImageAspectRatio {
		t.Fatalf("aspect ratio padrão = %q", got)
	}
	if got := normalizeAspectRatio(" 16:9 "); got != "16:9" {
		t.Fatalf("aspect ratio = %q", got)
	}
}

func TestGenerateImageWithFakeProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"images":[{"url":"data:image/png;base64,QUJD"}]}}]}`))
	}))
	defer srv.Close()

	pc := providerConfig{BaseURL: srv.URL, APIKey: "key", Model: "model", SupportsImage: true}
	result, err := generateImage(context.Background(), pc, "model", "um gato", "1:1")
	if err != nil {
		t.Fatalf("generateImage retornou erro: %v", err)
	}
	if result.B64 != "QUJD" || result.MimeType != "image/png" {
		t.Fatalf("resultado inesperado: %+v", result)
	}
}

func TestGenerateImageRejectsUnsupportedProvider(t *testing.T) {
	_, err := generateImage(context.Background(), providerConfig{SupportsImage: false}, "m", "p", "1:1")
	if !errors.Is(err, errProviderNoImage) {
		t.Fatalf("esperava errProviderNoImage, obtive: %v", err)
	}
}

func TestImageToolAvailability(t *testing.T) {
	pc := providerConfig{APIKey: "k", SupportsImage: true}
	engine := &Engine{}
	if _, ok := engine.imageTool(pc, 1); ok {
		t.Fatal("sem media sender a tool não deveria existir")
	}
	engine.mediaSender = &fakeMediaSender{}
	if _, ok := engine.imageTool(pc, 0); ok {
		t.Fatal("modo de teste (convID 0) não deveria expor a tool")
	}
	if _, ok := engine.imageTool(providerConfig{APIKey: "k", SupportsImage: false}, 1); ok {
		t.Fatal("provider sem suporte não deveria expor a tool")
	}
	if _, ok := engine.imageTool(providerConfig{SupportsImage: true}, 1); ok {
		t.Fatal("sem chave a tool não deveria existir")
	}
	if _, ok := engine.imageTool(pc, 1); !ok {
		t.Fatal("tool deveria existir com media sender, chave e suporte")
	}
}
