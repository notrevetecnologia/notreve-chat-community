package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/agendamento"
	"github.com/notrevetecnologia/notreve-chat-community/internal/applog"
	"github.com/notrevetecnologia/notreve-chat-community/internal/llm"
	"github.com/notrevetecnologia/notreve-chat-community/internal/mcp"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

// runtimeTool é uma tool resolvida em runtime (skill ou MCP) que não passa pelo
// executor HTTP de integrations, mas participa do mesmo ciclo LLM+tools.
type runtimeTool struct {
	name   string
	def    llm.Tool
	invoke func(ctx context.Context, args map[string]interface{}) string
}

func (e *Engine) runtimeTools(ctx context.Context, agentID, convID int, customerPhone string, pc providerConfig) ([]runtimeTool, []store.Skill) {
	var out []runtimeTool
	skills, err := e.store.ListAgentSkills(ctx, agentID)
	if err != nil {
		Logger.Printf("não foi possível carregar as skills do agente %d: %v", agentID, err)
		skills = nil
	}
	if len(skills) > 0 {
		out = append(out, skillInvokerTool(skills))
	}
	mcpTools, err := e.store.ListAgentMCPTools(ctx, agentID)
	if err != nil {
		Logger.Printf("não foi possível carregar as tools MCP do agente %d: %v", agentID, err)
	} else {
		for _, tool := range mcpTools {
			out = append(out, mcpInvokerTool(tool))
		}
	}
	if convID > 0 {
		out = append(out, e.scheduleTool(convID, customerPhone))
	}
	if tool, ok := e.imageTool(pc, convID); ok {
		out = append(out, tool)
	}
	return out, skills
}

func (e *Engine) imageTool(pc providerConfig, convID int) (runtimeTool, bool) {
	if convID <= 0 || e.mediaSender == nil || strings.TrimSpace(pc.APIKey) == "" || !pc.SupportsImage {
		return runtimeTool{}, false
	}
	def := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name: "gerar_imagem",
			Description: "Gera uma imagem a partir de uma descrição e a envia ao cliente pelo canal. " +
				"Use quando o cliente pedir para criar/gerar uma imagem, ilustração, logotipo ou foto.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt":       map[string]interface{}{"type": "string", "description": "Descrição detalhada da imagem a gerar"},
					"aspect_ratio": map[string]interface{}{"type": "string", "description": "Proporção da imagem, ex.: 1:1, 16:9, 9:16. Opcional"},
				},
				"required": []string{"prompt"},
			},
		},
	}
	invoke := func(ctx context.Context, args map[string]interface{}) string {
		return e.generateAndSendImage(ctx, pc, convID, args)
	}
	return runtimeTool{name: "gerar_imagem", def: def, invoke: invoke}, true
}

func (e *Engine) generateAndSendImage(ctx context.Context, pc providerConfig, convID int, args map[string]interface{}) string {
	prompt := stringArg(args, "prompt")
	if prompt == "" {
		return "Informe a descrição da imagem que deve ser gerada."
	}
	model := pc.Model
	aspectRatio := defaultImageAspectRatio
	if settings, err := e.store.GetSettings(ctx); err == nil {
		if strings.TrimSpace(settings.ImageModel) != "" {
			model = strings.TrimSpace(settings.ImageModel)
		}
		if strings.TrimSpace(settings.ImageAspectRatio) != "" {
			aspectRatio = strings.TrimSpace(settings.ImageAspectRatio)
		}
	}
	if requested := stringArg(args, "aspect_ratio"); requested != "" {
		aspectRatio = requested
	}
	result, err := generateImage(ctx, pc, model, prompt, aspectRatio)
	if err != nil {
		applog.Error(ctx, "midia", "falha ao gerar imagem: "+err.Error(), map[string]interface{}{"conversation_id": convID, "model": model})
		if errors.Is(err, errProviderNoImage) {
			return "O provedor de IA configurado não suporta geração de imagem."
		}
		return "Não foi possível gerar a imagem agora. Tente novamente em instantes."
	}
	mime := result.MimeType
	if strings.TrimSpace(mime) == "" {
		mime = "image/png"
	}
	media := GeneratedMedia{MediaType: "image", DataB64: result.B64, Mime: mime, FileName: "imagem.png", Caption: prompt}
	if err := e.mediaSender.SendGeneratedMedia(ctx, convID, media); err != nil {
		applog.Error(ctx, "midia", "falha ao enviar imagem gerada: "+err.Error(), map[string]interface{}{"conversation_id": convID})
		return "A imagem foi gerada, mas não foi possível enviá-la pelo canal."
	}
	return "Imagem gerada e enviada ao cliente. Continue o atendimento normalmente, sem descrever novamente a imagem."
}

func (e *Engine) scheduleTool(convID int, customerPhone string) runtimeTool {
	def := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name: "agendar_tarefa",
			Description: "Agenda uma tarefa futura para este contato: enviar uma mensagem, rodar uma ferramenta do sistema " +
				"ou reativar a IA com um prompt (follow-up). Use quando o cliente pedir um lembrete ou retorno no futuro.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"nome":          map[string]interface{}{"type": "string", "description": "Título curto da tarefa"},
					"tipo":          map[string]interface{}{"type": "string", "enum": []string{"once", "interval", "cron"}, "description": "once=data/hora; interval=a cada N segundos; cron=expressão de 5 campos"},
					"quando":        map[string]interface{}{"type": "string", "description": "once: data/hora (ex.: 2026-05-27T09:00); interval: segundos (mínimo 10); cron: ex.: 0 9 * * *"},
					"acao":          map[string]interface{}{"type": "string", "enum": []string{"enviar_mensagem", "rodar_ferramenta", "callback_agente"}, "description": "callback_agente reativa a IA com um prompt (follow-up)"},
					"texto":         map[string]interface{}{"type": "string", "description": "Mensagem a enviar (ação enviar_mensagem)"},
					"prompt":        map[string]interface{}{"type": "string", "description": "Prompt para reativar a IA (ação callback_agente)"},
					"ferramenta":    map[string]interface{}{"type": "string", "description": "Nome da ferramenta (ação rodar_ferramenta)"},
					"argumentos":    map[string]interface{}{"type": "object", "description": "Argumentos da ferramenta"},
					"max_execucoes": map[string]interface{}{"type": "integer", "description": "Limite de execuções em interval/cron. Vazio = ilimitado"},
				},
				"required": []string{"nome", "tipo", "quando", "acao"},
			},
		},
	}
	invoke := func(ctx context.Context, args map[string]interface{}) string {
		return e.createScheduledTask(ctx, convID, customerPhone, args)
	}
	return runtimeTool{name: "agendar_tarefa", def: def, invoke: invoke}
}

func stringArg(args map[string]interface{}, key string) string {
	value, ok := args[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func intArg(args map[string]interface{}, key string) *int {
	value, ok := args[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case float64:
		parsed := int(typed)
		return &parsed
	case int:
		parsed := typed
		return &parsed
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			result := int(parsed)
			return &result
		}
	}
	return nil
}

func (e *Engine) createScheduledTask(ctx context.Context, convID int, customerPhone string, args map[string]interface{}) string {
	nome := stringArg(args, "nome")
	quando := stringArg(args, "quando")
	tipo := agendamento.NormalizeTipo(stringArg(args, "tipo"))
	acao := agendamento.NormalizeAcao(stringArg(args, "acao"))
	if nome == "" || quando == "" || !agendamento.ValidTipo(tipo) || !agendamento.ValidAcao(acao) {
		return "Para agendar, informe nome, tipo (once/interval/cron), quando e acao (enviar_mensagem/rodar_ferramenta/callback_agente)."
	}
	payload := map[string]interface{}{}
	switch acao {
	case agendamento.AcaoEnviarMensagem:
		texto := stringArg(args, "texto")
		if texto == "" {
			return "A ação enviar_mensagem exige o campo texto."
		}
		payload["texto"] = texto
	case agendamento.AcaoCallbackAgente:
		prompt := stringArg(args, "prompt")
		if prompt == "" {
			return "A ação callback_agente exige o campo prompt."
		}
		payload["prompt"] = prompt
	case agendamento.AcaoRodarFerramenta:
		ferramenta := stringArg(args, "ferramenta")
		if ferramenta == "" {
			return "A ação rodar_ferramenta exige o campo ferramenta."
		}
		payload["nome_ferramenta"] = ferramenta
		if argumentos, ok := args["argumentos"].(map[string]interface{}); ok {
			payload["argumentos"] = argumentos
		}
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "Não foi possível montar o payload do agendamento."
	}
	next, err := agendamento.NextRun(tipo, quando, time.Now())
	if err != nil {
		return "Não foi possível calcular o horário do agendamento: " + err.Error()
	}
	task := store.Agendamento{
		Nome: nome, Tipo: tipo, Quando: quando, Acao: acao, Payload: payloadJSON,
		Status: agendamento.StatusPendente, ProximaExecucao: &next,
		Telefone: customerPhone, MaxExecucoes: intArg(args, "max_execucoes"),
	}
	if convID > 0 {
		if conv, convErr := e.store.GetConversation(ctx, convID); convErr == nil {
			task.CanalID = conv.ChannelID
			task.AgenteID = conv.AgentID
		}
	}
	id, err := e.store.CreateAgendamentoForTenant(ctx, task)
	if err != nil {
		Logger.Printf("falha ao criar agendamento pelo agente: %v", err)
		return "Não foi possível criar o agendamento agora."
	}
	return fmt.Sprintf("Agendamento #%d criado (%s, ação %s) para %s.", id, tipo, acao, next.UTC().Format(time.RFC3339))
}

func skillInvokerTool(skills []store.Skill) runtimeTool {
	byName := make(map[string]store.Skill, len(skills))
	for _, skill := range skills {
		byName[skill.Nome] = skill
	}
	def := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name: "invocar_skill",
			Description: "Acessa as instruções completas de uma skill disponível para este agente. " +
				"Use quando a solicitação se encaixar em uma das skills listadas. Depois de receber as instruções, siga-as à risca.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"skill_nome": map[string]interface{}{
						"type":        "string",
						"description": "Nome exato da skill a invocar (conforme listado no system prompt)",
					},
				},
				"required": []string{"skill_nome"},
			},
		},
	}
	invoke := func(_ context.Context, args map[string]interface{}) string {
		name := strings.TrimSpace(fmt.Sprintf("%v", args["skill_nome"]))
		skill, ok := byName[name]
		if !ok {
			return "Skill não encontrada: " + name + ". Use o nome exato listado no system prompt."
		}
		return "INSTRUÇÕES DA SKILL " + skill.Nome + ":\n" + skill.InstrucaoCompleta +
			"\n\nSiga rigorosamente estas instruções antes de responder ao cliente."
	}
	return runtimeTool{name: "invocar_skill", def: def, invoke: invoke}
}

func mcpInvokerTool(tool store.MCPTool) runtimeTool {
	cfg := mcp.ServerConfig{
		Transport: tool.Transport,
		Command:   tool.Command,
		Args:      tool.Args,
		Env:       tool.Env,
		URL:       tool.URL,
		Headers:   tool.Headers,
	}
	toolName := tool.Name
	description := strings.TrimSpace(tool.Description)
	if description == "" {
		description = "Ferramenta externa conectada via MCP (" + tool.ServerNome + ")."
	}
	def := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name:        toolName,
			Description: description,
			Parameters:  normalizeMCPInputSchema(tool.InputSchema),
		},
	}
	invoke := func(ctx context.Context, args map[string]interface{}) string {
		result, err := mcp.CallTool(ctx, cfg, toolName, args)
		if err != nil {
			Logger.Printf("tool MCP %q falhou: %v", toolName, err)
			return "Erro ao executar a ferramenta MCP " + toolName + ": " + err.Error()
		}
		if strings.TrimSpace(result) == "" {
			return "A ferramenta MCP " + toolName + " não retornou conteúdo."
		}
		return result
	}
	return runtimeTool{name: toolName, def: def, invoke: invoke}
}

func normalizeMCPInputSchema(schema map[string]interface{}) map[string]interface{} {
	if schema == nil {
		return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	}
	if _, ok := schema["type"]; !ok {
		schema["type"] = "object"
	}
	if _, ok := schema["properties"]; !ok {
		schema["properties"] = map[string]interface{}{}
	}
	return schema
}

func mergeToolDefs(base []llm.Tool, extras []runtimeTool) []llm.Tool {
	existing := make(map[string]bool, len(base))
	for _, tool := range base {
		existing[tool.Function.Name] = true
	}
	for _, extra := range extras {
		if existing[extra.name] {
			continue
		}
		base = append(base, extra.def)
		existing[extra.name] = true
	}
	return base
}

func skillsPromptSection(skills []store.Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nSKILLS DISPONÍVEIS (use a ferramenta invocar_skill com o nome exato para carregar as instruções completas antes de agir):\n")
	for _, skill := range skills {
		b.WriteString(fmt.Sprintf("- [%s] %s\n", skill.Nome, skill.Descricao))
	}
	b.WriteString("REGRA: quando o pedido do cliente se encaixar em uma skill, invoque-a ANTES de responder e siga rigorosamente as instruções recebidas.\n")
	return b.String()
}
