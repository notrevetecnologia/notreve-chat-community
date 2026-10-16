package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type reactAgentFailure struct {
	ID             int    `json:"id"`
	ConversationID *int   `json:"conversation_id,omitempty"`
	Channel        string `json:"channel"`
	Phone          string `json:"phone,omitempty"`
	Origem         string `json:"origem"`
	Tipo           string `json:"tipo"`
	ExceptionClass string `json:"exception_class"`
	MensagemErro   string `json:"mensagem_erro"`
	Tentativas     int    `json:"tentativas"`
	CreatedAt      string `json:"created_at"`
	Resolvido      bool   `json:"resolvido"`
	ResolvidoEm    string `json:"resolvido_em,omitempty"`
	ResolvidoPor   string `json:"resolvido_por,omitempty"`
	ResolucaoNota  string `json:"resolucao_nota,omitempty"`
}

func reactAgentFailureFromStore(item store.AgentFailure) reactAgentFailure {
	out := reactAgentFailure{
		ID: item.ID, ConversationID: item.ConversationID, Channel: item.Channel, Phone: item.Phone,
		Origem: item.Origem, Tipo: item.Tipo, ExceptionClass: item.ExceptionClass,
		MensagemErro: item.MensagemErro, Tentativas: item.Tentativas,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
		Resolvido: item.ResolvidoEm != nil, ResolvidoPor: item.ResolvidoPor, ResolucaoNota: item.ResolucaoNota,
	}
	if item.ResolvidoEm != nil {
		out.ResolvidoEm = item.ResolvidoEm.Format(time.RFC3339)
	}
	return out
}

func validReactFailureKind(kind string) bool {
	switch kind {
	case store.FailureKindLLM, store.FailureKindTool, store.FailureKindChannel, store.FailureKindPipeline, store.FailureKindOther:
		return true
	default:
		return false
	}
}

func (s *Server) handleReactAgentFailures(w http.ResponseWriter, r *http.Request) {
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	tipo := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("tipo")))
	if !validReactFailureKind(tipo) {
		tipo = ""
	}
	onlyOpen := r.URL.Query().Get("status") != "todos"
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	items, total, err := s.st.ListAgentFailuresPage(r.Context(), tipo, onlyOpen, query, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as falhas da IA")
		return
	}
	out := make([]reactAgentFailure, 0, len(items))
	for _, item := range items {
		out = append(out, reactAgentFailureFromStore(item))
	}
	open, err := s.st.CountOpenAgentFailures(r.Context())
	if err != nil {
		open = 0
	}
	stats, err := s.st.CountAgentFailuresByType(r.Context(), time.Now().Add(-7*24*time.Hour))
	if err != nil {
		stats = map[string]int{}
	}
	meta := reactPageMeta(total, page.Limit, page.Offset, reactQueryFilter(map[string]string{"tipo": tipo, "q": query}))
	meta["scope"] = "tenant"
	meta["window_days"] = 7
	writeReactResponse(w, http.StatusOK, map[string]interface{}{
		"items": out, "open": open, "stats": stats,
	}, meta)
}

func (s *Server) handleReactAgentFailureResolve(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		writeReactError(w, http.StatusBadRequest, "invalid_id", "falha inválida")
		return
	}
	var input struct {
		Nota string `json:"nota"`
	}
	if err := decodeReactAdminJSON(r, &input, 8*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	by := ""
	if user := currentUser(r); user != nil {
		by = user.Name
	}
	resolved, err := s.st.MarkAgentFailureResolved(r.Context(), id, by, strings.TrimSpace(input.Nota))
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível resolver a falha")
		return
	}
	if !resolved {
		writeReactError(w, http.StatusNotFound, "failure_not_found", "falha não encontrada ou já resolvida")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"id": id, "resolvido": true}, nil)
}
