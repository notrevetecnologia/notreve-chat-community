package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/llm"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

var errProviderNoImage = errors.New("provedor não suporta geração de imagem")

const defaultImageAspectRatio = "1:1"

func providerSupportsImage(p store.Provider) bool {
	for _, capability := range p.Capabilities {
		switch strings.ToLower(strings.TrimSpace(capability)) {
		case "imagem", "image", "image_generation":
			return true
		}
	}
	return strings.Contains(strings.ToLower(p.BaseURL), "openrouter")
}

func normalizeAspectRatio(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultImageAspectRatio
	}
	return value
}

func generateImage(ctx context.Context, pc providerConfig, model, prompt, aspectRatio string) (llm.ImageResult, error) {
	if !pc.SupportsImage {
		return llm.ImageResult{}, errProviderNoImage
	}
	client := llm.NewClient(pc.BaseURL, pc.APIKey, model, pc.Temp)
	return client.GenerateImage(ctx, prompt, normalizeAspectRatio(aspectRatio))
}
