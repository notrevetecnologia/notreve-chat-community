package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	agentpkg "github.com/notrevetecnologia/notreve-chat-community/internal/agent"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

const (
	maxKnowledgeSourcesPerAgent = 50
	maxKnowledgeRequestBytes    = 40 << 20
)

type reactKnowledgeSource struct {
	ID             int64                  `json:"id"`
	AgentID        int                    `json:"agent_id"`
	Kind           string                 `json:"kind"`
	Name           string                 `json:"name"`
	SourceURL      string                 `json:"source_url,omitempty"`
	MIMEType       string                 `json:"mime_type,omitempty"`
	ContentChars   int                    `json:"content_chars"`
	ContentPreview string                 `json:"content_preview,omitempty"`
	Status         string                 `json:"status"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
	Metadata       map[string]interface{} `json:"metadata"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
}

type reactKnowledgeAgent struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type reactKnowledgeInput struct {
	AgentID    int    `json:"agent_id"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	SourceURL  string `json:"source_url"`
	Text       string `json:"text"`
	FileName   string `json:"file_name"`
	MIMEType   string `json:"mime_type"`
	DataBase64 string `json:"data_base64"`
}

type reactKnowledgeView struct {
	Agents  []reactKnowledgeAgent  `json:"agents"`
	AgentID int                    `json:"agent_id"`
	Sources []reactKnowledgeSource `json:"sources"`
	Usage   store.KnowledgeUsage   `json:"usage"`
	Limits  map[string]int         `json:"limits"`
	Scope   string                 `json:"scope"`
}

func requireReactKnowledgeAccess(s *Server, w http.ResponseWriter, r *http.Request) bool {
	if _, err := store.TenantIDRequired(r.Context()); err != nil {
		writeReactError(w, http.StatusUnauthorized, "tenant_required", "contexto da empresa não identificado")
		return false
	}
	if !s.reactModuleEnabled(r.Context(), "ia") {
		writeReactError(w, http.StatusForbidden, "module_disabled", "módulo desativado")
		return false
	}
	if !userHasPerm(currentUser(r), permAgentesGerenciar) {
		writeReactError(w, http.StatusForbidden, "permission_denied", "sem permissão para gerenciar a base de conhecimento")
		return false
	}
	return true
}

func reactKnowledgeSourceFromStore(source store.KnowledgeSource) reactKnowledgeSource {
	preview := agentpkg.NormalizeKnowledgeText(source.Content, 800)
	return reactKnowledgeSource{
		ID: source.ID, AgentID: source.AgentID, Kind: source.Kind, Name: source.Name,
		SourceURL: source.SourceURL, MIMEType: source.MIMEType, ContentChars: source.ContentChars,
		ContentPreview: preview, Status: source.Status, ErrorMessage: source.ErrorMessage,
		Metadata: source.Metadata, CreatedAt: source.CreatedAt.Format(time.RFC3339), UpdatedAt: source.UpdatedAt.Format(time.RFC3339),
	}
}

func reactKnowledgeAgentsFromStore(agents []store.Agent) []reactKnowledgeAgent {
	items := make([]reactKnowledgeAgent, 0, len(agents))
	for _, item := range agents {
		items = append(items, reactKnowledgeAgent{ID: item.ID, Name: item.Name, Active: item.Active})
	}
	return items
}

func decodeReactKnowledgeInput(r *http.Request) (reactKnowledgeInput, error) {
	var input reactKnowledgeInput
	body, err := io.ReadAll(io.LimitReader(r.Body, maxKnowledgeRequestBytes+1))
	if err != nil {
		return input, errors.New("não foi possível ler a requisição")
	}
	if int64(len(body)) > maxKnowledgeRequestBytes {
		return input, errors.New("requisição excede o limite de 40 MB")
	}
	if err := json.Unmarshal(body, &input); err != nil {
		return input, errors.New("JSON inválido ou arquivo excede o limite da requisição")
	}
	return input, nil
}

func normalizeKnowledgeKind(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "text", "texto":
		return "text", nil
	case "pdf":
		return "pdf", nil
	case "audio", "áudio":
		return "audio", nil
	case "site", "url", "web":
		return "site", nil
	default:
		return "", errors.New("tipo de fonte deve ser text, pdf, audio ou site")
	}
}

func validateKnowledgeName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("nome da fonte é obrigatório")
	}
	if len([]rune(value)) > 160 {
		return "", errors.New("nome da fonte deve ter no máximo 160 caracteres")
	}
	return value, nil
}

func validateKnowledgeURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) > 2048 {
		return "", errors.New("URL deve ter no máximo 2048 caracteres")
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("site deve ser uma URL HTTP ou HTTPS válida")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return "", errors.New("site interno não pode ser usado como fonte")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
		return "", errors.New("endereço privado não pode ser usado como fonte")
	}
	return parsed.String(), nil
}

func validateKnowledgeFile(kind, fileName, mimeType string, raw []byte) error {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if kind == "pdf" {
		if len(raw) == 0 || len(raw) > agentpkg.MaxKnowledgePDFBytes {
			return errors.New("PDF deve ter entre 1 byte e 10 MB")
		}
		if mimeType != "" && mimeType != "application/pdf" && ext != ".pdf" {
			return errors.New("a fonte PDF precisa ser um arquivo PDF")
		}
		return nil
	}
	if kind == "audio" {
		if len(raw) == 0 || len(raw) > agentpkg.MaxKnowledgeAudioBytes {
			return errors.New("áudio deve ter entre 1 byte e 25 MB")
		}
		validExt := map[string]bool{".ogg": true, ".oga": true, ".webm": true, ".mp3": true, ".wav": true, ".m4a": true, ".mp4": true}
		if mimeType != "" && !strings.HasPrefix(mimeType, "audio/") && !validExt[ext] {
			return errors.New("a fonte de áudio precisa ser um arquivo de áudio")
		}
		return nil
	}
	return errors.New("tipo de arquivo não suportado")
}

func decodeKnowledgeFile(input reactKnowledgeInput, kind string) ([]byte, error) {
	encoded := strings.TrimSpace(input.DataBase64)
	if encoded == "" {
		return nil, errors.New("selecione um arquivo para treinar a IA")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	if err != nil {
		return nil, errors.New("arquivo codificado inválido")
	}
	if err := validateKnowledgeFile(kind, input.FileName, input.MIMEType, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func sourceContent(ctx context.Context, s *Server, input reactKnowledgeInput, kind string, agent store.Agent) (string, map[string]interface{}, error) {
	metadata := map[string]interface{}{"kind": kind}
	switch kind {
	case "text":
		content := agentpkg.NormalizeKnowledgeText(input.Text, agentpkg.MaxKnowledgeTextChars)
		if content == "" {
			return "", nil, errors.New("texto de conhecimento é obrigatório")
		}
		return content, metadata, nil
	case "site":
		address, err := validateKnowledgeURL(input.SourceURL)
		if err != nil {
			return "", nil, err
		}
		content := agentpkg.NormalizeKnowledgeText(agentpkg.FetchLinks(ctx, []string{address}), agentpkg.MaxKnowledgeTextChars)
		if content == "" {
			return "", nil, errors.New("não foi possível extrair texto deste site")
		}
		metadata["url"] = address
		return content, metadata, nil
	case "pdf":
		raw, err := decodeKnowledgeFile(input, kind)
		if err != nil {
			return "", nil, err
		}
		content, err := agentpkg.ExtractPDFText(raw)
		if err != nil {
			return "", nil, err
		}
		metadata["file_name"] = strings.TrimSpace(input.FileName)
		metadata["bytes"] = len(raw)
		return content, metadata, nil
	case "audio":
		raw, err := decodeKnowledgeFile(input, kind)
		if err != nil {
			return "", nil, err
		}
		settings, err := s.st.GetSettings(ctx)
		if err != nil {
			return "", nil, errors.New("não foi possível carregar a configuração de transcrição")
		}
		baseURL, apiKey, model := settings.STTBaseURL, settings.STTAPIKey, settings.STTModel
		if (strings.TrimSpace(baseURL) == "" || strings.TrimSpace(apiKey) == "") && agent.ProviderID != nil {
			provider, providerErr := s.st.GetProvider(ctx, *agent.ProviderID)
			if providerErr == nil {
				baseURL, apiKey, model = provider.BaseURL, provider.APIKey, provider.Model
			}
		}
		if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(apiKey) == "" {
			baseURL, apiKey, model = settings.DefaultBaseURL, settings.DefaultAPIKey, settings.DefaultModel
		}
		content, err := agentpkg.TranscribeKnowledgeAudio(ctx, baseURL, apiKey, model, raw, input.MIMEType, input.FileName)
		if err != nil {
			return "", nil, err
		}
		metadata["file_name"] = strings.TrimSpace(input.FileName)
		metadata["bytes"] = len(raw)
		metadata["transcribed"] = true
		return content, metadata, nil
	default:
		return "", nil, errors.New("tipo de fonte inválido")
	}
}

func parseKnowledgeAgentID(r *http.Request) (int, error) {
	value := r.URL.Query().Get("agent_id")
	if value == "" {
		value = r.PathValue("agent_id")
	}
	id, err := strconv.Atoi(value)
	if err != nil || id < 1 {
		return 0, errors.New("agente inválido")
	}
	return id, nil
}

func parseKnowledgeSourceID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("fonte inválida")
	}
	return id, nil
}

func (s *Server) handleReactKnowledge(w http.ResponseWriter, r *http.Request) {
	if !requireReactKnowledgeAccess(s, w, r) {
		return
	}
	agents, err := s.st.ListAgents(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os agentes")
		return
	}
	view := reactKnowledgeView{Agents: reactKnowledgeAgentsFromStore(agents), Sources: []reactKnowledgeSource{}, Limits: map[string]int{
		"max_sources_per_agent": maxKnowledgeSourcesPerAgent,
		"max_source_chars":      agentpkg.MaxKnowledgeTextChars,
		"max_total_chars":       agentpkg.MaxKnowledgeSourceChars,
		"max_pdf_bytes":         agentpkg.MaxKnowledgePDFBytes,
		"max_audio_bytes":       agentpkg.MaxKnowledgeAudioBytes,
	}, Scope: "tenant"}
	if len(agents) == 0 {
		writeReactResponse(w, http.StatusOK, view, nil)
		return
	}
	agentValue := strings.TrimSpace(r.URL.Query().Get("agent_id"))
	if agentValue == "" {
		agentValue = strings.TrimSpace(r.PathValue("agent_id"))
	}
	agentID := agents[0].ID
	if agentValue != "" {
		parsedID, agentErr := parseKnowledgeAgentID(r)
		if agentErr != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_agent", agentErr.Error())
			return
		}
		agentID = parsedID
	}
	if agentID < 1 {
		agentID = agents[0].ID
	}
	if _, err := s.st.GetAgent(r.Context(), agentID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "agent_not_found", "agente não encontrado")
		return
	}
	sources, err := s.st.ListKnowledgeSources(r.Context(), agentID)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as fontes de conhecimento")
		return
	}
	usage, err := s.st.KnowledgeUsage(r.Context(), agentID)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível calcular o uso da base")
		return
	}
	view.AgentID = agentID
	view.Usage = usage
	view.Sources = make([]reactKnowledgeSource, 0, len(sources))
	for _, source := range sources {
		view.Sources = append(view.Sources, reactKnowledgeSourceFromStore(source))
	}
	writeReactResponse(w, http.StatusOK, view, nil)
}

func (s *Server) handleReactKnowledgeCreate(w http.ResponseWriter, r *http.Request) {
	if !requireReactKnowledgeAccess(s, w, r) {
		return
	}
	input, err := decodeReactKnowledgeInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if input.AgentID < 1 {
		writeReactError(w, http.StatusBadRequest, "invalid_agent", "agente é obrigatório")
		return
	}
	agent, err := s.st.GetAgent(r.Context(), input.AgentID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "agent_not_found", "agente não encontrado")
		return
	}
	kind, err := normalizeKnowledgeKind(input.Kind)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_kind", err.Error())
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = map[string]string{"text": "Texto manual", "site": input.SourceURL, "pdf": input.FileName, "audio": input.FileName}[kind]
	}
	name, err = validateKnowledgeName(name)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_name", err.Error())
		return
	}
	existing, err := s.st.ListKnowledgeSources(r.Context(), input.AgentID)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível verificar o limite de fontes")
		return
	}
	if len(existing) >= maxKnowledgeSourcesPerAgent {
		writeReactError(w, http.StatusConflict, "source_limit", "este agente atingiu o limite de 50 fontes")
		return
	}
	content, metadata, processErr := sourceContent(r.Context(), s, input, kind, agent)
	if processErr != nil {
		writeReactError(w, http.StatusUnprocessableEntity, "processing_failed", processErr.Error())
		return
	}
	contentChars := len([]rune(content))
	usage, err := s.st.KnowledgeUsage(r.Context(), input.AgentID)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível calcular o uso da base")
		return
	}
	if usage.Chars+contentChars > agentpkg.MaxKnowledgeSourceChars {
		writeReactError(w, http.StatusConflict, "knowledge_limit", "a base deste agente atingiria o limite de 500.000 caracteres")
		return
	}
	source := store.KnowledgeSource{AgentID: input.AgentID, Kind: kind, Name: name, SourceURL: strings.TrimSpace(input.SourceURL), MIMEType: strings.TrimSpace(input.MIMEType), Content: content, ContentChars: contentChars, Status: "ready", Metadata: metadata}
	id, err := s.st.CreateKnowledgeSource(r.Context(), source)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível salvar a fonte de conhecimento")
		return
	}
	created, err := s.st.GetKnowledgeSource(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "fonte salva, mas não foi possível carregá-la")
		return
	}
	writeReactResponse(w, http.StatusCreated, reactKnowledgeSourceFromStore(created), nil)
}

func (s *Server) handleReactKnowledgeDelete(w http.ResponseWriter, r *http.Request) {
	if !requireReactKnowledgeAccess(s, w, r) {
		return
	}
	id, err := parseKnowledgeSourceID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if _, err := s.st.GetKnowledgeSource(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "fonte de conhecimento não encontrada")
		return
	}
	if err := s.st.DeleteKnowledgeSource(r.Context(), id); err != nil {
		writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir a fonte")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]int64{"deleted": id}, nil)
}
