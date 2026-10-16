package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	agentpkg "github.com/notrevetecnologia/notreve-chat-community/internal/agent"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type reactAgent struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	SystemPrompt     string   `json:"system_prompt"`
	ProviderID       *int     `json:"provider_id"`
	ProviderName     string   `json:"provider_name,omitempty"`
	TemperatureLevel string   `json:"temperature_level"`
	Knowledge        string   `json:"knowledge"`
	KnowledgeLinks   []string `json:"knowledge_links"`
	KnowledgeChars   int      `json:"knowledge_chars"`
	SetorIDs         []int    `json:"setor_ids"`
	ToolIDs          []int    `json:"tool_ids"`
	MenuOpcoes       string   `json:"menu_opcoes"`
	Active           bool     `json:"active"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}

type reactProviderOption struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	ProviderType string `json:"provider_type"`
	Model        string `json:"model"`
	Active       bool   `json:"active"`
	Capacidade   string `json:"capacidade"`
}

type reactToolOption struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type reactAgentOptions struct {
	Providers []reactProviderOption `json:"providers"`
	Sectors   []reactSectorOption   `json:"sectors"`
	Tools     []reactToolOption     `json:"tools"`
	Channels  []reactChannelOption  `json:"channels"`
}

type reactSectorOption struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type reactChannelOption struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Active  bool   `json:"active"`
	AgentID *int   `json:"agent_id,omitempty"`
}

type reactAgentInput struct {
	Name             *string   `json:"name"`
	SystemPrompt     *string   `json:"system_prompt"`
	ProviderID       *int      `json:"provider_id"`
	TemperatureLevel *string   `json:"temperature_level"`
	Knowledge        *string   `json:"knowledge"`
	KnowledgeLinks   *[]string `json:"knowledge_links"`
	SetorIDs         *[]int    `json:"setor_ids"`
	ToolIDs          *[]int    `json:"tool_ids"`
	MenuOpcoes       *string   `json:"menu_opcoes"`
	Active           *bool     `json:"active"`
}

func reactAgentFromStore(agent store.Agent) reactAgent {
	return reactAgent{
		ID: agent.ID, Name: agent.Name, SystemPrompt: agent.SystemPrompt, ProviderID: agent.ProviderID,
		ProviderName: agent.ProviderName, TemperatureLevel: agent.TemperatureLevel, Knowledge: agent.Knowledge,
		KnowledgeLinks: append([]string{}, agent.KnowledgeLinks...), KnowledgeChars: len([]rune(agent.Knowledge)) + len([]rune(agent.KnowledgeContent)),
		SetorIDs: append([]int{}, agent.SetorIDs...), ToolIDs: append([]int{}, agent.ToolIDs...),
		MenuOpcoes: agent.MenuOpcoes, Active: agent.Active,
		CreatedAt: agent.CreatedAt.Format(time.RFC3339), UpdatedAt: agent.UpdatedAt.Format(time.RFC3339),
	}
}

func (s *Server) reactAgentOptions(ctx context.Context) (reactAgentOptions, error) {
	providers, err := s.st.ListProviders(ctx)
	if err != nil {
		return reactAgentOptions{}, err
	}
	sectors, err := s.st.ListSetores(ctx)
	if err != nil {
		return reactAgentOptions{}, err
	}
	tools, err := s.st.ListIntegrationsForTenant(ctx)
	if err != nil {
		return reactAgentOptions{}, err
	}
	options := reactAgentOptions{
		Providers: make([]reactProviderOption, 0, len(providers)),
		Sectors:   make([]reactSectorOption, 0, len(sectors)),
		Tools:     make([]reactToolOption, 0, len(tools)),
		Channels:  make([]reactChannelOption, 0),
	}
	for _, provider := range providers {
		options.Providers = append(options.Providers, reactProviderOption{ID: provider.ID, Name: provider.Name, ProviderType: provider.ProviderType, Model: provider.Model, Active: provider.Active, Capacidade: provider.Capacidade})
	}
	for _, sector := range sectors {
		if sector.Active {
			options.Sectors = append(options.Sectors, reactSectorOption{ID: sector.ID, Name: sector.Name})
		}
	}
	for _, tool := range tools {
		options.Tools = append(options.Tools, reactToolOption{ID: tool.ID, Name: tool.Name, Description: tool.Description})
	}
	channels, err := s.st.ListChannels(ctx)
	if err != nil {
		return reactAgentOptions{}, err
	}
	for _, item := range channels {
		if item.Type == "whatsapp" || item.Type == "telegram" || item.Type == "webchat" {
			options.Channels = append(options.Channels, reactChannelOption{ID: item.ID, Name: item.Name, Type: item.Type, Active: item.Active, AgentID: item.AgentID})
		}
	}
	return options, nil
}

func decodeReactAgentInput(r *http.Request) (reactAgentInput, error) {
	var input reactAgentInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 256*1024+1)).Decode(&input); err != nil {
		return input, errors.New("JSON inválido")
	}
	return input, nil
}

func validateAgentName(value string) error {
	if strings.TrimSpace(value) == "" || len([]rune(value)) > 120 {
		return errors.New("nome deve ter entre 1 e 120 caracteres")
	}
	return nil
}

func normalizeAgentTemperature(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "equilibrado", nil
	}
	switch value {
	case "preciso", "equilibrado", "criativo":
		return value, nil
	default:
		return "", errors.New("temperatura deve ser preciso, equilibrado ou criativo")
	}
}

func normalizeAgentLinks(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len(value) > 2048 || (!strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://")) {
			return nil, errors.New("cada fonte deve ser uma URL http ou https válida")
		}
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
		if len(out) > 32 {
			return nil, errors.New("um agente pode ter no máximo 32 fontes")
		}
	}
	return out, nil
}

func normalizeAgentIDs(values []int) ([]int, error) {
	seen := map[int]bool{}
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value < 1 {
			return nil, errors.New("identificador inválido")
		}
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out, nil
}

func validateAgentReferences(ctx context.Context, s *Server, providerID *int, sectorIDs, toolIDs []int) error {
	if providerID != nil {
		if _, err := s.st.GetProvider(ctx, *providerID); err != nil {
			return errors.New("provedor de IA não pertence a esta empresa")
		}
	}
	sectors, err := s.st.ListSetores(ctx)
	if err != nil {
		return err
	}
	sectorMap := map[int]bool{}
	for _, sector := range sectors {
		sectorMap[sector.ID] = true
	}
	for _, id := range sectorIDs {
		if !sectorMap[id] {
			return errors.New("setor não pertence a esta empresa")
		}
	}
	tools, err := s.st.ListIntegrationsForTenant(ctx)
	if err != nil {
		return err
	}
	toolMap := map[int]bool{}
	for _, tool := range tools {
		toolMap[tool.ID] = true
	}
	for _, id := range toolIDs {
		if !toolMap[id] {
			return errors.New("tool não pertence a esta empresa")
		}
	}
	return nil
}

func applyReactAgentInput(ctx context.Context, s *Server, existing *store.Agent, input reactAgentInput, creating bool) error {
	if input.Name != nil {
		existing.Name = strings.TrimSpace(*input.Name)
	}
	if err := validateAgentName(existing.Name); err != nil {
		return err
	}
	if input.SystemPrompt != nil {
		existing.SystemPrompt = *input.SystemPrompt
	}
	if input.ProviderID != nil {
		existing.ProviderID = input.ProviderID
	}
	if input.TemperatureLevel != nil {
		normalized, err := normalizeAgentTemperature(*input.TemperatureLevel)
		if err != nil {
			return err
		}
		existing.TemperatureLevel = normalized
	} else if creating {
		existing.TemperatureLevel = "equilibrado"
	}
	if input.Knowledge != nil {
		existing.Knowledge = *input.Knowledge
	}
	if input.KnowledgeLinks != nil {
		links, err := normalizeAgentLinks(*input.KnowledgeLinks)
		if err != nil {
			return err
		}
		existing.KnowledgeLinks = links
		existing.KnowledgeContent = agentpkg.FetchLinks(ctx, links)
	}
	if input.SetorIDs != nil {
		ids, err := normalizeAgentIDs(*input.SetorIDs)
		if err != nil {
			return err
		}
		existing.SetorIDs = ids
	}
	if input.ToolIDs != nil {
		ids, err := normalizeAgentIDs(*input.ToolIDs)
		if err != nil {
			return err
		}
		existing.ToolIDs = ids
	}
	if input.MenuOpcoes != nil {
		existing.MenuOpcoes = *input.MenuOpcoes
		if len([]rune(existing.MenuOpcoes)) > 10000 {
			return errors.New("menu_opcoes excede o limite permitido")
		}
	}
	if input.Active != nil {
		existing.Active = *input.Active
	} else if creating {
		existing.Active = true
	}
	return validateAgentReferences(ctx, s, existing.ProviderID, existing.SetorIDs, existing.ToolIDs)
}

func (s *Server) handleReactAgents(w http.ResponseWriter, r *http.Request) {
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	agents, err := s.st.ListAgents(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os agentes")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	filtered := make([]reactAgent, 0, len(agents))
	for _, item := range agents {
		if status == "ativos" && !item.Active {
			continue
		}
		if status == "inativos" && item.Active {
			continue
		}
		if !reactContainsFold([]string{item.Name, item.SystemPrompt, item.Knowledge, item.ProviderName}, query) {
			continue
		}
		filtered = append(filtered, reactAgentFromStore(item))
	}
	pageItems, err := reactPageSlice(filtered, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	options, err := s.reactAgentOptions(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as opções dos agentes")
		return
	}
	meta := reactPageMeta(len(filtered), page.Limit, page.Offset, reactQueryFilter(map[string]string{"q": query, "status": status}))
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": pageItems, "options": options}, meta)
}

func (s *Server) handleReactAgentCreate(w http.ResponseWriter, r *http.Request) {
	input, err := decodeReactAgentInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	agent := store.Agent{}
	if err := applyReactAgentInput(r.Context(), s, &agent, input, true); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_agent", err.Error())
		return
	}
	if agent.Active {
		if err := s.st.EnsureCanCreateActiveAgent(r.Context()); err != nil {
			if writeReactPlanLimitError(w, err) {
				return
			}
			writeReactError(w, http.StatusInternalServerError, "plan_check_failed", "não foi possível validar o limite do plano")
			return
		}
	}
	id, err := s.st.CreateAgent(r.Context(), agent)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível criar o agente")
		return
	}
	created, err := s.st.GetAgent(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "agente criado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusCreated, reactAgentFromStore(created), nil)
}

func (s *Server) handleReactAgentUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	agent, err := s.st.GetAgent(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "agente não encontrado")
		return
	}
	wasActive := agent.Active
	input, err := decodeReactAgentInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := applyReactAgentInput(r.Context(), s, &agent, input, false); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_agent", err.Error())
		return
	}
	if !wasActive && agent.Active {
		if err := s.st.EnsureCanCreateActiveAgent(r.Context()); err != nil {
			if writeReactPlanLimitError(w, err) {
				return
			}
			writeReactError(w, http.StatusInternalServerError, "plan_check_failed", "não foi possível validar o limite do plano")
			return
		}
	}
	if err := s.st.UpdateAgent(r.Context(), agent); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível atualizar o agente")
		return
	}
	updated, err := s.st.GetAgent(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "agente atualizado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusOK, reactAgentFromStore(updated), nil)
}

func (s *Server) handleReactAgentToggle(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	agent, err := s.st.GetAgent(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "agente não encontrado")
		return
	}
	if !agent.Active {
		if err := s.st.EnsureCanCreateActiveAgent(r.Context()); err != nil {
			if writeReactPlanLimitError(w, err) {
				return
			}
			writeReactError(w, http.StatusInternalServerError, "plan_check_failed", "não foi possível validar o limite do plano")
			return
		}
	}
	agent.Active = !agent.Active
	if err := s.st.UpdateAgent(r.Context(), agent); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível alterar o estado do agente")
		return
	}
	writeReactResponse(w, http.StatusOK, reactAgentFromStore(agent), nil)
}

func (s *Server) handleReactAgentRefetch(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	agent, err := s.st.GetAgent(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "agente não encontrado")
		return
	}
	agent.KnowledgeContent = agentpkg.FetchLinks(r.Context(), agent.KnowledgeLinks)
	if err := s.st.UpdateAgent(r.Context(), agent); err != nil {
		writeReactError(w, http.StatusInternalServerError, "refetch_failed", "não foi possível atualizar as fontes")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"agent": reactAgentFromStore(agent), "sources": len(agent.KnowledgeLinks)}, nil)
}

type reactAgentTestInput struct {
	Message    string `json:"message"`
	ProviderID *int   `json:"provider_id"`
}

// reactAgentTestChannel usa o tipo do primeiro canal vinculado ao agente
// (channels.agent_id) para o prompt, caindo em webchat quando não há vínculo.
func (s *Server) reactAgentTestChannel(ctx context.Context, agentID int) string {
	channels, err := s.st.ListChannels(ctx)
	if err != nil {
		return "webchat"
	}
	for _, channel := range channels {
		if channel.AgentID != nil && *channel.AgentID == agentID {
			return channel.Type
		}
	}
	return "webchat"
}

// handleReactAgentTest roda o agente com uma mensagem de teste, sem persistir
// conversa nem mensagem real. O dono vê a resposta da IA direto no wizard.
func (s *Server) handleReactAgentTest(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	var input reactAgentTestInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024+1)).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	message := strings.TrimSpace(input.Message)
	if message == "" {
		writeReactError(w, http.StatusBadRequest, "empty_message", "escreva uma mensagem para testar o agente")
		return
	}
	if len([]rune(message)) > 4000 {
		writeReactError(w, http.StatusBadRequest, "message_too_long", "a mensagem de teste deve ter no máximo 4000 caracteres")
		return
	}

	agent, err := s.st.GetAgent(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "agente não encontrado")
		return
	}
	if input.ProviderID != nil {
		if _, err := s.st.GetProvider(r.Context(), *input.ProviderID); err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_provider", "provedor de IA não pertence a esta empresa")
			return
		}
		agent.ProviderID = input.ProviderID
	}

	result, err := s.eng.TestMessage(r.Context(), agent, s.reactAgentTestChannel(r.Context(), agent.ID), message)
	if err != nil {
		if errors.Is(err, agentpkg.ErrNoProvider) {
			writeReactError(w, http.StatusBadRequest, "no_provider", "este agente ainda não tem uma IA configurada")
			return
		}
		writeReactError(w, http.StatusBadGateway, "agent_test_failed", err.Error())
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{
		"reply":      result.Reply.Text,
		"used_tools": result.Reply.UsedTools,
		"provider":   result.Provider,
		"model":      result.Model,
	}, nil)
}

func (s *Server) handleReactAgentDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if _, err := s.st.GetAgent(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "agente não encontrado")
		return
	}
	if err := s.st.DeleteAgent(r.Context(), id); err != nil {
		writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir o agente")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"deleted": id}, nil)
}
