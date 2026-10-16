package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/mcp"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

const (
	maxMCPNameChars  = 120
	maxMCPURLChars   = 1000
	maxMCPCommandLen = 500
	maxMCPArgs       = 32
	mcpProbeTimeout  = 25 * time.Second
)

type reactMCPTool struct {
	ID          int                    `json:"id"`
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
	Ativa       bool                   `json:"ativa"`
}

type reactMCPServer struct {
	ID                  int                    `json:"id"`
	Nome                string                 `json:"nome"`
	Descricao           string                 `json:"descricao"`
	Transport           string                 `json:"transport"`
	Command             string                 `json:"command"`
	Args                []string               `json:"args"`
	Env                 map[string]string      `json:"env"`
	URL                 string                 `json:"url"`
	Headers             map[string]string      `json:"headers"`
	Ativo               bool                   `json:"ativo"`
	Conectado           bool                   `json:"conectado"`
	UltimoErro          string                 `json:"ultimo_erro"`
	ServerName          string                 `json:"server_name"`
	ServerVersion       string                 `json:"server_version"`
	Capabilities        map[string]interface{} `json:"capabilities"`
	AgentIDs            []int                  `json:"agent_ids"`
	ToolCount           int                    `json:"tool_count"`
	Tools               []reactMCPTool         `json:"tools"`
	UltimaSincronizacao *time.Time             `json:"ultima_sincronizacao,omitempty"`
}

type reactMCPInput struct {
	Nome      string            `json:"nome"`
	Descricao string            `json:"descricao"`
	Transport string            `json:"transport"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Ativo     *bool             `json:"ativo"`
	AgentIDs  []int             `json:"agent_ids"`
}

type reactMCPView struct {
	Items  []reactMCPServer  `json:"items"`
	Agents []reactSkillAgent `json:"agents"`
}

func requireReactMCPAccess(s *Server, w http.ResponseWriter, r *http.Request) bool {
	if _, err := store.TenantIDRequired(r.Context()); err != nil {
		writeReactError(w, http.StatusUnauthorized, "tenant_required", "contexto da empresa não identificado")
		return false
	}
	if !s.reactModuleEnabled(r.Context(), "tools") {
		writeReactError(w, http.StatusForbidden, "module_disabled", "módulo desativado")
		return false
	}
	if !userHasPerm(currentUser(r), permIntegracoesGerenciar) {
		writeReactError(w, http.StatusForbidden, "permission_denied", "sem permissão para gerenciar servidores MCP")
		return false
	}
	return true
}

// sensitiveMCPName espelha a regra de cifra do store (integrationSecretHeader):
// valores sensíveis nunca são devolvidos ao painel e campos vazios preservam o
// segredo já salvo.
func sensitiveMCPName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch lower {
	case "authorization", "proxy-authorization", "x-api-key", "api-key", "apikey", "x-auth-token", "x-access-token", "cookie", "set-cookie":
		return true
	}
	return strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "key")
}

func blankMCPSecretValues(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		if sensitiveMCPName(key) {
			out[key] = ""
			continue
		}
		out[key] = value
	}
	return out
}

func mergeMCPSecrets(current, submitted map[string]string) map[string]string {
	out := make(map[string]string, len(submitted))
	for key, value := range submitted {
		if strings.TrimSpace(value) == "" && sensitiveMCPName(key) {
			if existing, ok := current[key]; ok {
				out[key] = existing
				continue
			}
		}
		out[key] = value
	}
	return out
}

func reactMCPToolFromStore(tool store.MCPTool) reactMCPTool {
	schema := tool.InputSchema
	if schema == nil {
		schema = map[string]interface{}{}
	}
	return reactMCPTool{ID: tool.ID, Name: tool.Name, DisplayName: tool.DisplayName, Description: tool.Description, InputSchema: schema, Ativa: tool.Ativa}
}

func reactMCPServerFromStore(srv store.MCPServer, tools []store.MCPTool) reactMCPServer {
	agentIDs := srv.AgentIDs
	if agentIDs == nil {
		agentIDs = []int{}
	}
	args := srv.Args
	if args == nil {
		args = []string{}
	}
	items := make([]reactMCPTool, 0, len(tools))
	for _, tool := range tools {
		items = append(items, reactMCPToolFromStore(tool))
	}
	return reactMCPServer{
		ID: srv.ID, Nome: srv.Nome, Descricao: srv.Descricao, Transport: srv.Transport,
		Command: srv.Command, Args: args, Env: blankMCPSecretValues(srv.Env), URL: srv.URL,
		Headers: blankMCPSecretValues(srv.Headers), Ativo: srv.Ativo, Conectado: srv.Conectado,
		UltimoErro: srv.UltimoErro, ServerName: srv.ServerName, ServerVersion: srv.ServerVersion,
		Capabilities: srv.Capabilities, AgentIDs: agentIDs, ToolCount: srv.ToolCount, Tools: items,
		UltimaSincronizacao: srv.UltimaSincronizacao,
	}
}

func mcpConfigFromServer(srv store.MCPServer) mcp.ServerConfig {
	return mcp.ServerConfig{
		Transport: srv.Transport, Command: srv.Command, Args: srv.Args,
		Env: srv.Env, URL: srv.URL, Headers: srv.Headers, Timeout: mcpProbeTimeout,
	}
}

func parseReactMCPID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(strings.TrimSpace(r.PathValue("id")))
	if err != nil || id < 1 {
		return 0, errors.New("id inválido")
	}
	return id, nil
}

func normalizeMCPTransport(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "stdio":
		return "stdio", nil
	case "http", "streamable-http":
		return "http", nil
	case "sse":
		return "sse", nil
	default:
		return "", errors.New("transporte deve ser stdio, http ou sse")
	}
}

func validateReactMCPInput(input reactMCPInput) (reactMCPInput, error) {
	input.Nome = strings.TrimSpace(input.Nome)
	input.Descricao = strings.TrimSpace(input.Descricao)
	input.Command = strings.TrimSpace(input.Command)
	input.URL = strings.TrimSpace(input.URL)
	transport, err := normalizeMCPTransport(input.Transport)
	if err != nil {
		return input, err
	}
	input.Transport = transport
	if input.Nome == "" {
		return input, errors.New("informe o nome do servidor MCP")
	}
	if len([]rune(input.Nome)) > maxMCPNameChars {
		return input, errors.New("o nome deve ter no máximo 120 caracteres")
	}
	if len([]rune(input.URL)) > maxMCPURLChars {
		return input, errors.New("a URL deve ter no máximo 1000 caracteres")
	}
	if len([]rune(input.Command)) > maxMCPCommandLen {
		return input, errors.New("o comando deve ter no máximo 500 caracteres")
	}
	if len(input.Args) > maxMCPArgs {
		return input, errors.New("o número de argumentos excede o limite")
	}
	if transport == "stdio" {
		if input.Command == "" {
			return input, errors.New("servidor stdio exige um comando")
		}
	} else if input.URL == "" {
		return input, errors.New("servidor HTTP/SSE exige uma URL")
	} else if !strings.HasPrefix(input.URL, "http://") && !strings.HasPrefix(input.URL, "https://") {
		return input, errors.New("a URL deve começar com http:// ou https://")
	}
	if input.Env == nil {
		input.Env = map[string]string{}
	}
	if input.Headers == nil {
		input.Headers = map[string]string{}
	}
	return input, nil
}

func (s *Server) handleReactMCPServers(w http.ResponseWriter, r *http.Request) {
	if !requireReactMCPAccess(s, w, r) {
		return
	}
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	servers, err := s.st.ListMCPServers(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível listar os servidores MCP")
		return
	}
	agents, err := s.st.ListAgents(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os agentes")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	filtered := make([]store.MCPServer, 0, len(servers))
	for _, srv := range servers {
		if status == "ativos" && !srv.Ativo {
			continue
		}
		if status == "inativos" && srv.Ativo {
			continue
		}
		if !reactContainsFold([]string{srv.Nome, srv.Descricao, srv.Transport, srv.URL, srv.Command}, query) {
			continue
		}
		filtered = append(filtered, srv)
	}
	pageServers, err := reactPageSlice(filtered, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	view := reactMCPView{Items: make([]reactMCPServer, 0, len(pageServers)), Agents: reactSkillAgentsFromStore(agents)}
	for _, srv := range pageServers {
		tools, err := s.st.ListMCPServerTools(r.Context(), srv.ID)
		if err != nil {
			writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as tools do servidor")
			return
		}
		view.Items = append(view.Items, reactMCPServerFromStore(srv, tools))
	}
	meta := reactPageMeta(len(filtered), page.Limit, page.Offset, reactQueryFilter(map[string]string{"q": query, "status": status}))
	writeReactResponse(w, http.StatusOK, view, meta)
}

func decodeReactMCPInput(r *http.Request) (reactMCPInput, error) {
	var input reactMCPInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 512<<10)).Decode(&input); err != nil {
		return input, errors.New("JSON inválido")
	}
	return input, nil
}

func (s *Server) handleReactMCPCreate(w http.ResponseWriter, r *http.Request) {
	if !requireReactMCPAccess(s, w, r) {
		return
	}
	input, err := decodeReactMCPInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	input, err = validateReactMCPInput(input)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_server", err.Error())
		return
	}
	ativo := true
	if input.Ativo != nil {
		ativo = *input.Ativo
	}
	srv := store.MCPServer{
		Nome: input.Nome, Descricao: input.Descricao, Transport: input.Transport, Command: input.Command,
		Args: input.Args, Env: input.Env, URL: input.URL, Headers: input.Headers, Ativo: ativo, AgentIDs: input.AgentIDs,
	}
	id, err := s.st.CreateMCPServerSecure(r.Context(), srv)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível criar o servidor MCP")
		return
	}
	// Sincroniza de forma best-effort: se o servidor estiver acessível, as tools
	// já ficam disponíveis; se não, o cadastro permanece e o erro é registrado.
	_, info, tools, probeErr := s.probeMCPServer(r.Context(), id)
	if probeErr == nil {
		stored := make([]store.MCPTool, 0, len(tools))
		for _, tool := range tools {
			stored = append(stored, store.MCPTool{ServerID: id, Name: tool.Name, DisplayName: tool.Title, Description: tool.Description, InputSchema: tool.InputSchema})
		}
		_ = s.st.ReplaceMCPServerTools(r.Context(), id, stored)
		_ = s.st.MarkMCPServerStatus(r.Context(), id, true, info.Name, info.Version, info.Capabilities, "")
	}
	created, err := s.st.GetMCPServer(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "servidor criado, mas não foi possível carregá-lo")
		return
	}
	savedTools, _ := s.st.ListMCPServerTools(r.Context(), id)
	writeReactResponse(w, http.StatusCreated, reactMCPServerFromStore(created, savedTools), nil)
}

func (s *Server) handleReactMCPUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireReactMCPAccess(s, w, r) {
		return
	}
	id, err := parseReactMCPID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	current, err := s.st.GetMCPServer(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "servidor MCP não encontrado")
		return
	}
	previous, _ := s.st.GetMCPServerSecure(r.Context(), id)
	input, err := decodeReactMCPInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	input, err = validateReactMCPInput(input)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_server", err.Error())
		return
	}
	ativo := current.Ativo
	if input.Ativo != nil {
		ativo = *input.Ativo
	}
	srv := store.MCPServer{
		ID: id, Nome: input.Nome, Descricao: input.Descricao, Transport: input.Transport, Command: input.Command,
		Args: input.Args, Env: mergeMCPSecrets(current.Env, input.Env), URL: input.URL,
		Headers: mergeMCPSecrets(current.Headers, input.Headers), Ativo: ativo, AgentIDs: input.AgentIDs,
	}
	if err := s.st.UpdateMCPServerSecure(r.Context(), srv); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível atualizar o servidor MCP")
		return
	}
	mcp.CloseServer(mcpConfigFromServer(previous))
	updated, err := s.st.GetMCPServer(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "servidor atualizado, mas não foi possível carregá-lo")
		return
	}
	tools, _ := s.st.ListMCPServerTools(r.Context(), id)
	writeReactResponse(w, http.StatusOK, reactMCPServerFromStore(updated, tools), nil)
}

func (s *Server) handleReactMCPDelete(w http.ResponseWriter, r *http.Request) {
	if !requireReactMCPAccess(s, w, r) {
		return
	}
	id, err := parseReactMCPID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if _, err := s.st.GetMCPServer(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "servidor MCP não encontrado")
		return
	}
	previous, _ := s.st.GetMCPServerSecure(r.Context(), id)
	if err := s.st.DeleteMCPServer(r.Context(), id); err != nil {
		writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir o servidor MCP")
		return
	}
	mcp.CloseServer(mcpConfigFromServer(previous))
	writeReactResponse(w, http.StatusOK, map[string]int{"deleted": id}, nil)
}

// probeMCPServer conecta, faz o handshake e lista as tools do servidor salvo.
func (s *Server) probeMCPServer(ctx context.Context, id int) (store.MCPServer, mcp.ServerInfo, []mcp.Tool, error) {
	srv, err := s.st.GetMCPServerSecure(ctx, id)
	if err != nil {
		return srv, mcp.ServerInfo{}, nil, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, mcpProbeTimeout)
	defer cancel()
	info, tools, err := mcp.Probe(probeCtx, mcpConfigFromServer(srv))
	if err != nil {
		_ = s.st.MarkMCPServerStatus(ctx, id, false, srv.ServerName, srv.ServerVersion, srv.Capabilities, err.Error())
		return srv, mcp.ServerInfo{}, nil, err
	}
	return srv, info, tools, nil
}

func (s *Server) handleReactMCPTest(w http.ResponseWriter, r *http.Request) {
	if !requireReactMCPAccess(s, w, r) {
		return
	}
	id, err := parseReactMCPID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if _, err := s.st.GetMCPServer(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "servidor MCP não encontrado")
		return
	}
	srv, info, tools, probeErr := s.probeMCPServer(r.Context(), id)
	if probeErr != nil {
		writeReactResponse(w, http.StatusOK, map[string]interface{}{
			"connected": false, "error": probeErr.Error(),
		}, nil)
		return
	}
	_ = s.st.MarkMCPServerStatus(r.Context(), id, true, info.Name, info.Version, info.Capabilities, "")
	writeReactResponse(w, http.StatusOK, map[string]interface{}{
		"connected":      true,
		"server_name":    info.Name,
		"server_version": info.Version,
		"protocol":       info.ProtocolVersion,
		"tool_count":     len(tools),
		"server_id":      srv.ID,
	}, nil)
}

func (s *Server) handleReactMCPSync(w http.ResponseWriter, r *http.Request) {
	if !requireReactMCPAccess(s, w, r) {
		return
	}
	id, err := parseReactMCPID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if _, err := s.st.GetMCPServer(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "servidor MCP não encontrado")
		return
	}
	_, info, tools, probeErr := s.probeMCPServer(r.Context(), id)
	if probeErr != nil {
		writeReactError(w, http.StatusBadGateway, "connection_failed", "não foi possível conectar ao servidor MCP: "+probeErr.Error())
		return
	}
	stored := make([]store.MCPTool, 0, len(tools))
	for _, tool := range tools {
		stored = append(stored, store.MCPTool{
			ServerID: id, Name: tool.Name, DisplayName: tool.Title, Description: tool.Description, InputSchema: tool.InputSchema,
		})
	}
	if err := s.st.ReplaceMCPServerTools(r.Context(), id, stored); err != nil {
		writeReactError(w, http.StatusInternalServerError, "sync_failed", "não foi possível salvar as tools sincronizadas")
		return
	}
	_ = s.st.MarkMCPServerStatus(r.Context(), id, true, info.Name, info.Version, info.Capabilities, "")
	updated, err := s.st.GetMCPServer(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "tools sincronizadas, mas não foi possível carregar o servidor")
		return
	}
	savedTools, _ := s.st.ListMCPServerTools(r.Context(), id)
	writeReactResponse(w, http.StatusOK, reactMCPServerFromStore(updated, savedTools), nil)
}
