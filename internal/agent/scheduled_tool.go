package agent

import (
	"context"
	"errors"

	"github.com/notrevetecnologia/notreve-chat-community/internal/tools"
)

// RunScheduledTool executa uma integração do tenant pelo nome, com o mesmo
// executor/política do ciclo LLM, para a ação rodar_ferramenta dos agendamentos.
func (e *Engine) RunScheduledTool(ctx context.Context, name string, args map[string]interface{}, customerPhone string) (string, error) {
	integrations, err := e.store.ListIntegrations(ctx)
	if err != nil {
		return "", err
	}
	def := findTool(integrations, name)
	if def.Name == "" {
		return "", errors.New("ferramenta não encontrada: " + name)
	}
	toolCtx := ctx
	if customerPhone != "" {
		toolCtx = tools.WithToolIdentityPhone(ctx, customerPhone)
	}
	return e.exec.Execute(toolCtx, def, args), nil
}
