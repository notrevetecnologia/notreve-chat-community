package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/llm"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

// ErrNoProvider indica que o agente não tem nenhuma IA (provider) configurada.
var ErrNoProvider = errors.New("agente sem provedor de IA configurado")

// TestResult é a resposta do modo de teste do painel: o texto que a IA
// devolveria ao cliente, as tools usadas e qual provedor/modelo respondeu.
type TestResult struct {
	Reply    Reply
	Provider string
	Model    string
}

// TestMessage roda o motor de IA para uma única mensagem, com histórico
// efêmero em memória, sem persistir conversa, mensagem ou consumo. É o caminho
// usado pelo passo "Testar" do wizard para o dono conversar com o agente antes
// de publicá-lo. Reaproveita o mesmo prompt, tools e ciclo do atendimento real,
// inclusive a regra de que a IA nunca transfere por conta própria: marcadores
// de transferência apenas sinalizam NeedsHuman/SetorID no resultado.
func (e *Engine) TestMessage(ctx context.Context, a store.Agent, channel, message string) (TestResult, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return TestResult{}, errors.New("mensagem vazia")
	}

	pc, err := e.resolveProviderConfig(ctx, a)
	if err != nil {
		return TestResult{}, ErrNoProvider
	}
	if strings.TrimSpace(pc.APIKey) == "" {
		return TestResult{}, ErrNoProvider
	}

	setores, err := e.setoresOf(ctx, a.SetorIDs)
	if err != nil {
		return TestResult{}, err
	}
	knowledge := e.knowledgeSourcesForPrompt(ctx, a.ID, message)
	extraTools, skills := e.runtimeTools(ctx, a.ID, 0, "", pc)
	system := e.buildSystemPrompt(ctx, a, setores, knowledge, skills, pc.SafeMode, pc.Capacidade, channel, "", false, "")
	messages := []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: message},
	}

	integrations, err := e.store.GetIntegrationsByIDs(ctx, a.ToolIDs)
	if err != nil {
		return TestResult{}, err
	}
	toolDefs := mergeToolDefs(buildLLMTools(integrations), extraTools)

	outcome := e.runToolLoop(ctx, pc, messages, toolDefs, integrations, a.ID, 0, "", extraTools...)
	if outcome.failed {
		return TestResult{}, errors.New("não foi possível obter resposta da IA")
	}

	reply := strings.TrimSpace(outcome.reply)
	needsHuman := strings.Contains(reply, humanMarker)
	reply = strings.ReplaceAll(reply, humanMarker, "")
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return TestResult{}, errors.New("a IA não retornou resposta")
	}

	var setorID *int
	if strings.Contains(strings.ToLower(reply), "[setor:") {
		idx := strings.Index(reply, "[SETOR:")
		if idx < 0 {
			idx = strings.Index(strings.ToLower(reply), "[setor:")
		}
		end := strings.Index(reply[idx:], "]")
		if end > 0 {
			name := strings.TrimSpace(reply[idx+7 : idx+end])
			reply = strings.TrimSpace(reply[:idx] + reply[idx+end+1:])
			if setor, err := e.store.FindSetorByName(ctx, name); err == nil && setorPermitido(setor.ID, a.SetorIDs) {
				sid := setor.ID
				setorID = &sid
				needsHuman = true
			}
		}
	}

	rich, clean := e.richReply(reply, "", false)
	out := Reply{Text: clean, NeedsHuman: needsHuman, SetorID: setorID, UsedTools: outcome.used, Rich: rich}
	if rich != nil {
		out.Fallback = rich.Fallback
	}
	return TestResult{Reply: out, Provider: pc.Name, Model: pc.Model}, nil
}
