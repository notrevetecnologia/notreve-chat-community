package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/agendamento"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

const (
	maxScheduleNameChars = 200
	maxScheduleDescChars = 500
)

type reactScheduleInput struct {
	Nome         string                 `json:"nome"`
	Descricao    string                 `json:"descricao"`
	Tipo         string                 `json:"tipo"`
	Quando       string                 `json:"quando"`
	Acao         string                 `json:"acao"`
	Payload      map[string]interface{} `json:"payload"`
	MaxExecucoes *int                   `json:"max_execucoes"`
	Status       string                 `json:"status"`
	CanalID      *int                   `json:"canal_id"`
	ContatoID    *int                   `json:"contato_id"`
	Telefone     string                 `json:"telefone"`
	AgenteID     *int                   `json:"agente_id"`
}

type reactScheduleChannel struct {
	ID   int    `json:"id"`
	Nome string `json:"nome"`
	Tipo string `json:"tipo"`
}

type reactScheduleAgent struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type reactSchedulesView struct {
	Items    []store.Agendamento    `json:"items"`
	Channels []reactScheduleChannel `json:"channels"`
	Agents   []reactScheduleAgent   `json:"agents"`
}

func requireReactSchedulesAccess(s *Server, w http.ResponseWriter, r *http.Request) bool {
	if _, err := store.TenantIDRequired(r.Context()); err != nil {
		writeReactError(w, http.StatusUnauthorized, "tenant_required", "contexto da empresa não identificado")
		return false
	}
	if !s.reactModuleEnabled(r.Context(), "agendamentos") {
		writeReactError(w, http.StatusForbidden, "module_disabled", "módulo desativado")
		return false
	}
	if !userHasPerm(currentUser(r), permAgendamentosGerenciar) {
		writeReactError(w, http.StatusForbidden, "permission_denied", "sem permissão para gerenciar agendamentos")
		return false
	}
	return true
}

func parseReactScheduleID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(strings.TrimSpace(r.PathValue("id")))
	if err != nil || id < 1 {
		return 0, errors.New("id inválido")
	}
	return id, nil
}

func decodeReactScheduleInput(r *http.Request) (reactScheduleInput, error) {
	var input reactScheduleInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 256<<10)).Decode(&input); err != nil {
		return input, errors.New("JSON inválido")
	}
	return input, nil
}

func agendamentoFirstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func reactSchedulesChannels(channels []store.Channel) []reactScheduleChannel {
	out := make([]reactScheduleChannel, 0, len(channels))
	for _, ch := range channels {
		out = append(out, reactScheduleChannel{ID: ch.ID, Nome: ch.Name, Tipo: ch.Type})
	}
	return out
}

func reactSchedulesAgents(agents []store.Agent) []reactScheduleAgent {
	out := make([]reactScheduleAgent, 0, len(agents))
	for _, a := range agents {
		out = append(out, reactScheduleAgent{ID: a.ID, Name: a.Name, Active: a.Active})
	}
	return out
}

func (s *Server) handleReactSchedules(w http.ResponseWriter, r *http.Request) {
	if !requireReactSchedulesAccess(s, w, r) {
		return
	}
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	tipo := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("tipo")))
	acao := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("acao")))
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	items, err := s.st.ListAgendamentosFilteredForTenant(r.Context(), status, tipo, acao, query, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível listar os agendamentos")
		return
	}
	total, err := s.st.CountAgendamentosForTenant(r.Context(), status, tipo, acao, query)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível contar os agendamentos")
		return
	}
	channels, err := s.st.ListChannels(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os canais")
		return
	}
	agents, err := s.st.ListAgents(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os agentes")
		return
	}
	if items == nil {
		items = []store.Agendamento{}
	}
	view := reactSchedulesView{
		Items:    items,
		Channels: reactSchedulesChannels(channels),
		Agents:   reactSchedulesAgents(agents),
	}
	meta := reactPageMeta(total, page.Limit, page.Offset, reactQueryFilter(map[string]string{"status": status, "tipo": tipo, "acao": acao, "q": query}))
	writeReactResponse(w, http.StatusOK, view, meta)
}

func (s *Server) reactScheduleFromInput(ctx context.Context, input reactScheduleInput, current *store.Agendamento) (store.Agendamento, error) {
	task := store.Agendamento{}
	if current != nil {
		task = *current
	}
	task.Nome = agendamentoFirstNonEmpty(input.Nome, task.Nome)
	task.Descricao = agendamentoFirstNonEmpty(input.Descricao, task.Descricao)
	task.Tipo = agendamento.NormalizeTipo(agendamentoFirstNonEmpty(input.Tipo, task.Tipo))
	task.Quando = agendamentoFirstNonEmpty(input.Quando, task.Quando)
	task.Acao = agendamento.NormalizeAcao(agendamentoFirstNonEmpty(input.Acao, task.Acao))
	if task.Nome == "" {
		return store.Agendamento{}, errors.New("informe o nome do agendamento")
	}
	if len([]rune(task.Nome)) > maxScheduleNameChars {
		return store.Agendamento{}, errors.New("o nome deve ter no máximo 200 caracteres")
	}
	if len([]rune(task.Descricao)) > maxScheduleDescChars {
		return store.Agendamento{}, errors.New("a descrição deve ter no máximo 500 caracteres")
	}
	if !agendamento.ValidTipo(task.Tipo) {
		return store.Agendamento{}, errors.New("tipo de agendamento inválido")
	}
	if !agendamento.ValidAcao(task.Acao) {
		return store.Agendamento{}, errors.New("ação do agendamento inválida")
	}
	if task.Quando == "" {
		return store.Agendamento{}, errors.New("informe quando o agendamento deve rodar")
	}

	if input.CanalID != nil {
		if *input.CanalID > 0 {
			task.CanalID = input.CanalID
		} else {
			task.CanalID = nil
		}
	}
	if input.ContatoID != nil {
		if *input.ContatoID > 0 {
			task.ContatoID = input.ContatoID
		} else {
			task.ContatoID = nil
		}
	}
	if input.AgenteID != nil {
		if *input.AgenteID > 0 {
			task.AgenteID = input.AgenteID
		} else {
			task.AgenteID = nil
		}
	}
	if input.Telefone != "" {
		task.Telefone = strings.TrimSpace(input.Telefone)
	}
	if input.MaxExecucoes != nil {
		if *input.MaxExecucoes > 0 {
			task.MaxExecucoes = input.MaxExecucoes
		} else {
			task.MaxExecucoes = nil
		}
	}

	payload := input.Payload
	if payload == nil && current != nil {
		payload = agendamento.DecodePayload(current.Payload)
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}
	switch task.Acao {
	case agendamento.AcaoEnviarMensagem:
		if strings.TrimSpace(fmt.Sprintf("%v", payload["texto"])) == "" {
			return store.Agendamento{}, errors.New("informe o texto da mensagem")
		}
	case agendamento.AcaoRodarFerramenta:
		if strings.TrimSpace(fmt.Sprintf("%v", payload["nome_ferramenta"])) == "" {
			return store.Agendamento{}, errors.New("informe a ferramenta a executar")
		}
	case agendamento.AcaoCallbackAgente:
		if strings.TrimSpace(fmt.Sprintf("%v", payload["prompt"])) == "" {
			return store.Agendamento{}, errors.New("informe o prompt do agente")
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return store.Agendamento{}, errors.New("payload inválido")
	}
	task.Payload = encoded

	if task.Telefone == "" && task.ContatoID != nil {
		if contato, err := s.st.GetContatoForTenant(ctx, *task.ContatoID); err == nil {
			task.Telefone = strings.TrimSpace(contato.Phone)
		}
	}
	if (task.Acao == agendamento.AcaoEnviarMensagem || task.Acao == agendamento.AcaoCallbackAgente) && task.Telefone == "" {
		return store.Agendamento{}, errors.New("informe o telefone do destinatário")
	}
	if task.Acao == agendamento.AcaoEnviarMensagem && task.CanalID == nil {
		return store.Agendamento{}, errors.New("selecione o canal de envio")
	}

	status := strings.ToLower(strings.TrimSpace(input.Status))
	if current == nil {
		if status == "" {
			status = agendamento.StatusPendente
		}
	} else if status == "" {
		status = current.Status
	}
	switch status {
	case agendamento.StatusPendente, agendamento.StatusCancelada, agendamento.StatusFalhou, agendamento.StatusConcluida:
	default:
		return store.Agendamento{}, errors.New("status inválido")
	}
	if current != nil {
		if err := scheduleStatusTransitionAllowed(current.Status, status); err != nil {
			return store.Agendamento{}, err
		}
	}
	task.Status = status
	if task.Status == agendamento.StatusPendente {
		next, err := agendamento.NextRun(task.Tipo, task.Quando, time.Now())
		if err != nil {
			return store.Agendamento{}, err
		}
		task.ProximaExecucao = &next
	} else {
		task.ProximaExecucao = nil
	}
	return task, nil
}

func (s *Server) handleReactScheduleCreate(w http.ResponseWriter, r *http.Request) {
	if !requireReactSchedulesAccess(s, w, r) {
		return
	}
	input, err := decodeReactScheduleInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	task, err := s.reactScheduleFromInput(r.Context(), input, nil)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_schedule", err.Error())
		return
	}
	if user := currentUser(r); user != nil {
		task.CriadoPor = &user.ID
	}
	id, err := s.st.CreateAgendamentoForTenant(r.Context(), task)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "create_failed", err.Error())
		return
	}
	created, err := s.st.GetAgendamentoForTenant(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "agendamento criado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusCreated, created, nil)
}

func (s *Server) handleReactScheduleUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireReactSchedulesAccess(s, w, r) {
		return
	}
	id, err := parseReactScheduleID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	current, err := s.st.GetAgendamentoForTenant(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "agendamento não encontrado")
		return
	}
	input, err := decodeReactScheduleInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	task, err := s.reactScheduleFromInput(r.Context(), input, &current)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_schedule", err.Error())
		return
	}
	task.ID = id
	if err := s.st.UpdateAgendamentoForTenant(r.Context(), task); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível atualizar o agendamento")
		return
	}
	updated, err := s.st.GetAgendamentoForTenant(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "agendamento atualizado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusOK, updated, nil)
}

func (s *Server) handleReactScheduleDelete(w http.ResponseWriter, r *http.Request) {
	if !requireReactSchedulesAccess(s, w, r) {
		return
	}
	id, err := parseReactScheduleID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if _, err := s.st.GetAgendamentoForTenant(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "agendamento não encontrado")
		return
	}
	if err := s.st.DeleteAgendamentoForTenant(r.Context(), id); err != nil {
		writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir o agendamento")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]int{"deleted": id}, nil)
}

func (s *Server) handleReactScheduleTrigger(w http.ResponseWriter, r *http.Request) {
	if !requireReactSchedulesAccess(s, w, r) {
		return
	}
	id, err := parseReactScheduleID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	task, err := s.st.GetAgendamentoForTenant(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "agendamento não encontrado")
		return
	}
	if task.Status == agendamento.StatusConcluida || task.Status == agendamento.StatusCancelada {
		writeReactError(w, http.StatusBadRequest, "not_runnable", "agendamento concluído ou cancelado não pode ser disparado")
		return
	}
	now := time.Now()
	outcome := agendamento.RunOne(r.Context(), task, s.agendamentoHandlers, now)
	if err := s.st.SetAgendamentoStatusForTenant(r.Context(), id, outcome.Status, outcome.Resultado, outcome.Erro, outcome.Execucoes, now, outcome.Proxima); err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível registrar a execução")
		return
	}
	updated, err := s.st.GetAgendamentoForTenant(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "execução concluída, mas não foi possível carregar o agendamento")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{
		"agendamento": updated,
		"status":      outcome.Status,
		"resultado":   outcome.Resultado,
		"erro":        outcome.Erro,
	}, nil)
}
