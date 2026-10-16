package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	agentpkg "github.com/notrevetecnologia/notreve-chat-community/internal/agent"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type reactLearningItem struct {
	ID        int    `json:"id"`
	Question  string `json:"question"`
	Channel   string `json:"channel"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type reactLearningAgent struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type reactLearningTeachInput struct {
	AgentID   int    `json:"agent_id"`
	Response  string `json:"resposta"`
	Procedure string `json:"procedimento"`
}

type reactQuickReply struct {
	ID        int    `json:"id"`
	Shortcut  string `json:"shortcut"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

type reactQuickReplyInput struct {
	Shortcut string `json:"shortcut"`
	Text     string `json:"text"`
}

type reactMessageTemplate struct {
	ID        int    `json:"id"`
	Key       string `json:"key"`
	Title     string `json:"title"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

type reactMessageTemplateInput struct {
	Text string `json:"text"`
}

type reactPlatformView struct {
	Branding        reactBranding  `json:"branding"`
	Limits          store.Limites  `json:"limits"`
	Usage           map[string]int `json:"usage"`
	CanEditBranding bool           `json:"can_edit_branding"`
	CanEditLimits   bool           `json:"can_edit_limits"`
	Scope           string         `json:"scope"`
}

type reactLimitInput struct {
	MaxChannels        int `json:"max_channels"`
	MaxAgents          int `json:"max_agents"`
	MaxActiveCampaigns int `json:"max_active_campaigns"`
	DailyCampaignLimit int `json:"daily_campaign_limit"`
}

func reactLearningFromStore(item store.Aprendizado) reactLearningItem {
	return reactLearningItem{
		ID: item.ID, Question: item.Pergunta, Channel: item.Canal, Status: item.Status,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
	}
}

func reactQuickReplyFromStore(item store.RespostaRapida) reactQuickReply {
	return reactQuickReply{
		ID: item.ID, Shortcut: item.Atalho, Text: item.Texto,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
	}
}

func reactMessageTemplateFromStore(item store.Mensagem) reactMessageTemplate {
	return reactMessageTemplate{
		ID: item.ID, Key: item.Chave, Title: item.Titulo, Text: item.Texto,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
	}
}

func (s *Server) handleReactLearning(w http.ResponseWriter, r *http.Request) {
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	items, err := s.st.ListAprendizados(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar o aprendizado")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	counts := map[string]int{}
	filtered := make([]reactLearningItem, 0, len(items))
	for _, item := range items {
		counts[strings.ToLower(item.Status)]++
		if status != "" && status != "todos" && strings.ToLower(item.Status) != status {
			continue
		}
		if !reactContainsFold([]string{item.Pergunta, item.Canal}, query) {
			continue
		}
		filtered = append(filtered, reactLearningFromStore(item))
	}
	pageItems, err := reactPageSlice(filtered, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	agents := make([]reactLearningAgent, 0)
	// Ver o aprendizado continua separado de gerenciar agentes. Somente quem
	// pode alterar agentes recebe a lista usada pelo botão "Ensinar IA".
	if userHasPerm(currentUser(r), permAgentesGerenciar) {
		if values, err := s.st.ListAgents(r.Context()); err == nil {
			agents = make([]reactLearningAgent, 0, len(values))
			for _, value := range values {
				agents = append(agents, reactLearningAgent{ID: value.ID, Name: value.Name, Active: value.Active})
			}
		}
	}
	meta := reactPageMeta(len(filtered), page.Limit, page.Offset, reactQueryFilter(map[string]string{"q": query, "status": status}))
	meta["scope"] = "tenant"
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": pageItems, "agents": agents, "counts": counts}, meta)
}

func (s *Server) handleReactLearningMark(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		writeReactError(w, http.StatusBadRequest, "invalid_id", "aprendizado inválido")
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if err := decodeReactAdminJSON(r, &input, 8*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if input.Status != "novo" && input.Status != "resolvido" && input.Status != "ignorado" {
		writeReactError(w, http.StatusBadRequest, "invalid_status", "status de aprendizado inválido")
		return
	}
	if err := s.st.MarkAprendizado(r.Context(), id, input.Status); err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível atualizar o aprendizado")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"id": id, "status": input.Status}, nil)
}

func learningSourceName(item store.Aprendizado) string {
	question := agentpkg.NormalizeKnowledgeText(item.Pergunta, 112)
	if question == "" {
		question = "dúvida sem texto"
	}
	return "Aprendizado #" + strconv.Itoa(item.ID) + " · " + question
}

func learningSourceContent(item store.Aprendizado, response, procedure string) string {
	return "Dúvida original do cliente: " + agentpkg.NormalizeKnowledgeText(item.Pergunta, agentpkg.MaxKnowledgeTextChars) +
		"\n\nResposta correta que a IA deve oferecer:\n" + agentpkg.NormalizeKnowledgeText(response, agentpkg.MaxKnowledgeTextChars) +
		"\n\nComo a IA deve agir neste caso:\n" + agentpkg.NormalizeKnowledgeText(procedure, agentpkg.MaxKnowledgeTextChars)
}

func (s *Server) handleReactLearningTeach(w http.ResponseWriter, r *http.Request) {
	if !requireReactKnowledgeAccess(s, w, r) {
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		writeReactError(w, http.StatusBadRequest, "invalid_id", "aprendizado inválido")
		return
	}
	var input reactLearningTeachInput
	if err := decodeReactAdminJSON(r, &input, 220*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if input.AgentID < 1 {
		writeReactError(w, http.StatusBadRequest, "invalid_agent", "selecione o agente que deve aprender")
		return
	}
	response := agentpkg.NormalizeKnowledgeText(input.Response, agentpkg.MaxKnowledgeTextChars)
	procedure := agentpkg.NormalizeKnowledgeText(input.Procedure, agentpkg.MaxKnowledgeTextChars)
	if response == "" || procedure == "" {
		writeReactError(w, http.StatusBadRequest, "incomplete_training", "informe a resposta correta e como a IA deve agir")
		return
	}
	item, err := s.st.GetAprendizado(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "learning_not_found", "aprendizado não encontrado")
		return
	}
	if _, err := s.st.GetAgent(r.Context(), input.AgentID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "agent_not_found", "agente não encontrado")
		return
	}
	existing, err := s.st.ListKnowledgeSources(r.Context(), input.AgentID)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível verificar a base do agente")
		return
	}
	if len(existing) >= maxKnowledgeSourcesPerAgent {
		writeReactError(w, http.StatusConflict, "source_limit", "este agente atingiu o limite de 50 fontes")
		return
	}
	for _, source := range existing {
		if source.Metadata != nil && fmt.Sprint(source.Metadata["learning_id"]) == strconv.Itoa(id) && source.Status == "ready" {
			// Idempotência: um duplo clique ou uma falha depois da criação não
			// deve gerar fontes repetidas nem bloquear a conclusão do item.
			if item.Status != "resolvido" {
				if err := s.st.MarkAprendizado(r.Context(), id, "resolvido"); err != nil {
					writeReactError(w, http.StatusInternalServerError, "status_failed", "a fonte já existe, mas não foi possível marcar o aprendizado como resolvido")
					return
				}
			}
			writeReactResponse(w, http.StatusOK, map[string]interface{}{"id": id, "source_id": source.ID, "status": "resolvido"}, nil)
			return
		}
	}
	itemContent := learningSourceContent(item, response, procedure)
	contentChars := len([]rune(itemContent))
	usage, err := s.st.KnowledgeUsage(r.Context(), input.AgentID)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível calcular o uso da base")
		return
	}
	if usage.Chars+contentChars > agentpkg.MaxKnowledgeSourceChars {
		writeReactError(w, http.StatusConflict, "knowledge_limit", "a base deste agente atingiria o limite de 500.000 caracteres")
		return
	}
	source := store.KnowledgeSource{
		AgentID: input.AgentID, Kind: "text", Name: learningSourceName(item), Content: itemContent,
		ContentChars: contentChars, Status: "ready",
		Metadata: map[string]interface{}{"kind": "aprendizado", "learning_id": id, "canal": item.Canal},
	}
	sourceID, err := s.st.CreateKnowledgeSource(r.Context(), source)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível cadastrar o aprendizado na base")
		return
	}
	if err := s.st.MarkAprendizado(r.Context(), id, "resolvido"); err != nil {
		writeReactError(w, http.StatusInternalServerError, "status_failed", "a fonte foi criada, mas não foi possível marcar o aprendizado como resolvido")
		return
	}
	writeReactResponse(w, http.StatusCreated, map[string]interface{}{"id": id, "source_id": sourceID, "status": "resolvido"}, nil)
}

func (s *Server) handleReactQuickReplies(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		writeReactError(w, http.StatusUnauthorized, "unauthorized", "usuário não autenticado")
		return
	}
	if r.Method == http.MethodGet {
		page, err := parseReactPage(r)
		if err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		items, err := s.st.ListRespostasRapidas(r.Context(), user.ID)
		if err != nil {
			writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as respostas rápidas")
			return
		}
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		filtered := make([]reactQuickReply, 0, len(items))
		for _, item := range items {
			if !reactContainsFold([]string{item.Atalho, item.Texto}, query) {
				continue
			}
			filtered = append(filtered, reactQuickReplyFromStore(item))
		}
		pageItems, err := reactPageSlice(filtered, page.Limit, page.Offset)
		if err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
			return
		}
		meta := reactPageMeta(len(filtered), page.Limit, page.Offset, reactQueryFilter(map[string]string{"q": query}))
		meta["scope"] = "user"
		writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": pageItems}, meta)
		return
	}
	var input reactQuickReplyInput
	if err := decodeReactAdminJSON(r, &input, 32*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	input.Shortcut = strings.TrimPrefix(strings.TrimSpace(input.Shortcut), "/")
	input.Text = strings.TrimSpace(input.Text)
	if input.Shortcut == "" || len([]rune(input.Shortcut)) > 80 || input.Text == "" || len([]rune(input.Text)) > 10000 {
		writeReactError(w, http.StatusBadRequest, "invalid_input", "atalho e texto são obrigatórios e possuem limite de tamanho")
		return
	}
	id, err := s.st.CreateRespostaRapida(r.Context(), store.RespostaRapida{Atalho: input.Shortcut, Texto: input.Text, UserID: user.ID})
	if err != nil {
		writeReactError(w, http.StatusConflict, "duplicate_or_invalid", "não foi possível criar a resposta rápida; verifique se o atalho já existe")
		return
	}
	writeReactResponse(w, http.StatusCreated, map[string]interface{}{"id": id}, nil)
}

func (s *Server) handleReactQuickReplyUpdate(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	id, err := strconv.Atoi(r.PathValue("id"))
	if user == nil || err != nil || id < 1 {
		writeReactError(w, http.StatusBadRequest, "invalid_request", "resposta rápida inválida")
		return
	}
	var input reactQuickReplyInput
	if err := decodeReactAdminJSON(r, &input, 32*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	input.Shortcut = strings.TrimPrefix(strings.TrimSpace(input.Shortcut), "/")
	input.Text = strings.TrimSpace(input.Text)
	if input.Shortcut == "" || len([]rune(input.Shortcut)) > 80 || input.Text == "" || len([]rune(input.Text)) > 10000 {
		writeReactError(w, http.StatusBadRequest, "invalid_input", "atalho e texto são obrigatórios e possuem limite de tamanho")
		return
	}
	if err := s.st.UpdateRespostaRapidaOwned(r.Context(), store.RespostaRapida{ID: id, Atalho: input.Shortcut, Texto: input.Text, UserID: user.ID}); err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível atualizar a resposta rápida")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"id": id}, nil)
}

func (s *Server) handleReactQuickReplyDelete(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	id, err := strconv.Atoi(r.PathValue("id"))
	if user == nil || err != nil || id < 1 {
		writeReactError(w, http.StatusBadRequest, "invalid_id", "resposta rápida inválida")
		return
	}
	if err := s.st.DeleteRespostaRapidaOwned(r.Context(), id, user.ID); err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível excluir a resposta rápida")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"deleted": id}, nil)
}

func (s *Server) handleReactMessageTemplates(w http.ResponseWriter, r *http.Request) {
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	items, err := s.st.ListMensagens(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as mensagens padrão")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	filtered := make([]reactMessageTemplate, 0, len(items))
	for _, item := range items {
		if !reactContainsFold([]string{item.Chave, item.Titulo, item.Texto}, query) {
			continue
		}
		filtered = append(filtered, reactMessageTemplateFromStore(item))
	}
	pageItems, err := reactPageSlice(filtered, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	meta := reactPageMeta(len(filtered), page.Limit, page.Offset, reactQueryFilter(map[string]string{"q": query}))
	meta["scope"] = "tenant"
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": pageItems}, meta)
}

func (s *Server) handleReactMessageTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		writeReactError(w, http.StatusBadRequest, "invalid_id", "mensagem padrão inválida")
		return
	}
	var input reactMessageTemplateInput
	if err := decodeReactAdminJSON(r, &input, 64*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	input.Text = strings.TrimSpace(input.Text)
	if input.Text == "" || len([]rune(input.Text)) > 20000 {
		writeReactError(w, http.StatusBadRequest, "invalid_text", "o texto é obrigatório e deve ter até 20.000 caracteres")
		return
	}
	if err := s.st.UpdateMensagem(r.Context(), id, input.Text); err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível atualizar a mensagem padrão")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"id": id}, nil)
}

func (s *Server) handleReactPlatform(w http.ResponseWriter, r *http.Request) {
	limits, err := s.st.GetLimites(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os limites")
		return
	}
	usage := map[string]int{}
	if value, countErr := s.st.CountCanais(r.Context()); countErr == nil {
		usage["channels"] = value
	}
	if value, countErr := s.st.CountAtendentes(r.Context()); countErr == nil {
		usage["agents"] = value
	}
	if value, countErr := s.st.CountCampanhasAtivas(r.Context()); countErr == nil {
		usage["active_campaigns"] = value
	}
	user := currentUser(r)
	view := reactPlatformView{
		Branding: s.platformBranding(r.Context()), Limits: limits, Usage: usage,
		CanEditBranding: user != nil && user.Role == "super_admin",
		CanEditLimits:   user != nil && user.Role == "super_admin",
		Scope:           "platform",
	}
	writeReactResponse(w, http.StatusOK, view, nil)
}

func validateReactLimit(value int) bool { return value >= 0 && value <= 1000000 }

func (s *Server) handleReactPlatformLimitsUpdate(w http.ResponseWriter, r *http.Request) {
	var input reactLimitInput
	if err := decodeReactAdminJSON(r, &input, 16*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if !validateReactLimit(input.MaxChannels) || !validateReactLimit(input.MaxAgents) || !validateReactLimit(input.MaxActiveCampaigns) || !validateReactLimit(input.DailyCampaignLimit) {
		writeReactError(w, http.StatusBadRequest, "invalid_limits", "os limites devem estar entre 0 e 1.000.000")
		return
	}
	if err := s.st.UpdateLimites(r.Context(), store.Limites{MaxCanais: input.MaxChannels, MaxAtendentes: input.MaxAgents, MaxCampanhasAtivas: input.MaxActiveCampaigns, CampanhaLimiteDiario: input.DailyCampaignLimit}); err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível salvar os limites")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"limits": input}, nil)
}
