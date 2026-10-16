package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/llm"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

const (
	summaryMaxLines      = 6
	summaryMaxChars      = 1200
	summaryTranscriptMax = 8000
)

func (e *Engine) SummarizeConversation(ctx context.Context, convID int, a store.Agent) (string, error) {
	if e == nil || e.store == nil {
		return "", fmt.Errorf("motor indisponível para resumir")
	}
	history, err := e.store.ListMessages(ctx, convID)
	if err != nil {
		return "", err
	}
	transcript := summaryTranscript(history)
	if transcript == "" {
		return "", nil
	}
	pc, err := e.resolveProviderConfig(ctx, a)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(pc.APIKey) == "" {
		return "", fmt.Errorf("IA sem chave configurada")
	}
	client := llm.NewClient(pc.BaseURL, pc.APIKey, pc.Model, 0.2)
	response, err := client.Complete(ctx, summaryMessages(transcript), nil)
	if err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("IA não retornou resumo")
	}
	return cleanSummary(contentString(response.Choices[0].Message.Content)), nil
}

func summaryTranscript(history []store.Message) string {
	lines := make([]string, 0, len(history))
	for _, message := range history {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		role := ""
		switch message.Role {
		case "user":
			role = "Cliente"
		case "assistant":
			role = "Atendimento"
		default:
			continue
		}
		lines = append(lines, role+": "+content)
	}
	kept := make([]string, 0, len(lines))
	total := 0
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if total+len(line) > summaryTranscriptMax && len(kept) > 0 {
			break
		}
		kept = append(kept, line)
		total += len(line) + 1
	}
	for left, right := 0, len(kept)-1; left < right; left, right = left+1, right-1 {
		kept[left], kept[right] = kept[right], kept[left]
	}
	return strings.Join(kept, "\n")
}

func summaryMessages(transcript string) []llm.Message {
	system := "Você resume conversas de atendimento ao cliente. Produza um resumo curto em português do Brasil, " +
		"com no máximo " + strconv.Itoa(summaryMaxLines) + " linhas, cobrindo o que o cliente quer, o que já foi resolvido " +
		"e o que ainda falta. Responda apenas com o resumo, sem saudações, títulos ou introduções."
	return []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: "Resuma a conversa abaixo:\n\n" + transcript},
	}
}

func cleanSummary(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lines := make([]string, 0, summaryMaxLines)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "-*• "))
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) == summaryMaxLines {
			break
		}
	}
	out := strings.TrimSpace(strings.Join(lines, "\n"))
	if runes := []rune(out); len(runes) > summaryMaxChars {
		out = string(runes[:summaryMaxChars])
	}
	return out
}
