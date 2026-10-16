package agent

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/llm"
)

// applyMedia enriquece o último turno do usuário com a mídia enviada:
// áudio -> transcrição (STT); imagem/PDF -> conteúdo multimodal (se o provedor suportar).
func (e *Engine) applyMedia(ctx context.Context, messages *[]llm.Message, pc providerConfig, media Media) {
	if media.B64 == "" || media.Type == "" {
		return
	}
	msgs := *messages
	var lastUser *llm.Message
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			lastUser = &msgs[i]
			break
		}
	}
	if lastUser == nil {
		return
	}
	text, _ := lastUser.Content.(string)

	if media.Type == "audio" {
		transc, err := e.transcribe(ctx, pc, media)
		if err != nil {
			Logger.Printf("stt: erro ao transcrever: %v", err)
		} else {
			Logger.Printf("stt: transcricao ok (len=%d)", len(transc))
		}
		if err == nil && strings.TrimSpace(transc) != "" {
			lastUser.Content = text + "\n\n[Transcrição do áudio do cliente]: " + strings.TrimSpace(transc)
		}
		return
	}

	if pc.Capacidade != "multimodal" {
		return // provedor só texto: mantém o texto-resumo da mídia
	}
	if media.Type == "image" || media.Type == "document" {
		lastUser.Content = []interface{}{
			map[string]interface{}{"type": "text", "text": text},
			map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]interface{}{
					"url": "data:" + media.Mime + ";base64," + media.B64,
				},
			},
		}
	}
}

// transcribe converte o áudio do cliente em texto (endpoint OpenAI-compatível).
func (e *Engine) transcribe(ctx context.Context, pc providerConfig, media Media) (string, error) {
	st, err := e.store.GetSettingsSecure(ctx)
	if err != nil {
		return "", err
	}
	base := strings.TrimRight(st.STTBaseURL, "/")
	if base == "" {
		base = pc.BaseURL
	}
	key := st.STTAPIKey
	if key == "" {
		key = pc.APIKey
	}
	model := st.STTModel
	if model == "" {
		model = "whisper-1"
	}
	raw, err := base64.StdEncoding.DecodeString(media.B64)
	if err != nil {
		return "", err
	}
	name := media.Name
	if name == "" {
		name = "audio.ogg"
	}
	client := llm.NewClient(pc.BaseURL, pc.APIKey, pc.Model, pc.Temp)
	return client.Transcribe(ctx, base, key, model, raw, media.Mime, name)
}
