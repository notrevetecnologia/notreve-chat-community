package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/llm"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type providerConfig struct {
	Name          string
	BaseURL       string
	Model         string
	APIKey        string
	Temp          float64
	SafeMode      bool
	Capacidade    string
	SupportsImage bool
}

// resolveProviderConfig define qual IA o agente usa:
// 1. o provider configurado no agente; 2. senão, o primeiro provider ativo.
// A chave permanece cifrada no banco e só é aberta neste limite de runtime.
func (e *Engine) resolveProviderConfig(ctx context.Context, a store.Agent) (providerConfig, error) {
	var p store.Provider
	var err error
	if a.ProviderID != nil {
		p, err = e.store.GetProviderSecure(ctx, *a.ProviderID)
		if err != nil {
			Logger.Printf("agente %d: provider %d indisponível (%v); usando o primeiro provider ativo", a.ID, *a.ProviderID, err)
			p, err = e.store.FirstActiveProviderSecure(ctx)
		}
	} else {
		p, err = e.store.FirstActiveProviderSecure(ctx)
	}
	if err != nil {
		return providerConfig{}, errors.New("nenhuma IA (provider) configurada")
	}
	baseURL := p.BaseURL
	model := p.Model
	if p.ProviderType == "notreve" {
		baseURL = llm.NotreveBaseURL
		model = notreveModel(model, p.Capacidade)
	}
	return providerConfig{
		Name:          p.Name,
		BaseURL:       baseURL,
		Model:         model,
		APIKey:        p.APIKey,
		Temp:          tempForLevel(a.TemperatureLevel),
		SafeMode:      p.SafeMode,
		Capacidade:    p.Capacidade,
		SupportsImage: providerSupportsImage(p),
	}, nil
}

func notreveModel(model, capacidade string) string {
	if chosen := strings.TrimSpace(model); chosen != "" {
		return chosen
	}
	if capacidade == "multimodal" {
		return llm.NotreveVisionModel
	}
	return llm.NotreveChatModel
}

// tempForLevel converte o nível amigável em temperatura.
// Atendimento é factual: mesmo o nível "equilibrado" usa temperatura baixa
// para reduzir invenção e variação entre respostas.
func tempForLevel(level string) float64 {
	switch level {
	case "preciso":
		return 0.1
	case "criativo":
		return 0.8
	default:
		return 0.3 // equilibrado
	}
}
