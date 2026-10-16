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
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type reactChatflowStep struct {
	ID         int                    `json:"id"`
	ChatflowID int                    `json:"chatflow_id"`
	Ord        int                    `json:"ord"`
	Tipo       string                 `json:"tipo"`
	Config     map[string]interface{} `json:"config"`
	CreatedAt  string                 `json:"created_at"`
}

type reactChatflow struct {
	ID        int                    `json:"id"`
	Nome      string                 `json:"nome"`
	Version   int                    `json:"version"`
	Ativo     bool                   `json:"ativo"`
	Config    map[string]interface{} `json:"config"`
	UpdatedAt string                 `json:"updated_at"`
	Steps     []reactChatflowStep    `json:"steps"`
}

type reactChatflowChannelOption struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	ChatflowID *int   `json:"chatflow_id"`
	Active     bool   `json:"active"`
}

type reactChatflowInput struct {
	Nome    *string                   `json:"nome"`
	Version *int                      `json:"version"`
	Ativo   *bool                     `json:"ativo"`
	Config  *map[string]interface{}   `json:"config"`
	Steps   *[]reactChatflowStepInput `json:"steps"`
}

type reactChatflowStepInput struct {
	Ord    int                    `json:"ord"`
	Tipo   string                 `json:"tipo"`
	Config map[string]interface{} `json:"config"`
}

func reactChatflowFromStore(flow store.Chatflow, steps []store.ChatflowStep) reactChatflow {
	config := map[string]interface{}{}
	if flow.Config != "" {
		_ = json.Unmarshal([]byte(flow.Config), &config)
	}
	out := reactChatflow{
		ID: flow.ID, Nome: flow.Nome, Version: flow.Version, Ativo: flow.Ativo,
		Config: config, UpdatedAt: flow.UpdatedAt.Format(time.RFC3339), Steps: []reactChatflowStep{},
	}
	for _, step := range steps {
		stepConfig := map[string]interface{}{}
		if step.Config != "" {
			_ = json.Unmarshal([]byte(step.Config), &stepConfig)
		}
		out.Steps = append(out.Steps, reactChatflowStep{
			ID: step.ID, ChatflowID: step.ChatflowID, Ord: step.Ord, Tipo: step.Tipo,
			Config: stepConfig, CreatedAt: step.CreatedAt.Format(time.RFC3339),
		})
	}
	return out
}

func (s *Server) loadReactChatflow(ctx context.Context, id int) (reactChatflow, error) {
	flow, err := s.st.GetChatflow(ctx, id)
	if err != nil {
		return reactChatflow{}, err
	}
	steps, err := s.st.ListChatflowSteps(ctx, flow.ID)
	if err != nil {
		return reactChatflow{}, err
	}
	return reactChatflowFromStore(flow, steps), nil
}

func (s *Server) reactChatflowChannels(ctx context.Context) ([]reactChatflowChannelOption, error) {
	channels, err := s.st.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]reactChatflowChannelOption, 0, len(channels))
	for _, channel := range channels {
		out = append(out, reactChatflowChannelOption{ID: channel.ID, Name: channel.Name, Type: channel.Type, ChatflowID: channel.ChatflowID, Active: channel.Active})
	}
	return out, nil
}

func decodeReactChatflowInput(r *http.Request) (reactChatflowInput, error) {
	var input reactChatflowInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 512*1024+1)).Decode(&input); err != nil {
		return input, errors.New("JSON inválido")
	}
	return input, nil
}

func validateChatflowName(value string) error {
	if strings.TrimSpace(value) == "" || len([]rune(value)) > 120 {
		return errors.New("nome deve ter entre 1 e 120 caracteres")
	}
	return nil
}

var chatflowStepTipos = map[string]bool{
	"start": true, "message": true, "collect": true, "condition": true, "intent": true,
	"tool": true, "queue": true, "end": true,
	"menu": true, "delay": true, "set_variable": true, "goto": true,
	"business_hours": true, "tag": true, "satisfaction": true, "note": true, "http": true,
}

func validateChatflowStart(steps []reactChatflowStepInput) error {
	count := 0
	for _, step := range steps {
		if step.Tipo == "start" {
			count++
		}
	}
	if count != 1 {
		return errors.New("o chat flow precisa de exatamente um bloco de início")
	}
	return nil
}

func validateChatflowSteps(steps []reactChatflowStepInput) error {
	if err := validateChatflowStart(steps); err != nil {
		return err
	}
	ords := map[int]bool{}
	for _, step := range steps {
		if !chatflowStepTipos[strings.TrimSpace(step.Tipo)] {
			return errors.New("tipo de passo inválido: " + step.Tipo)
		}
		ords[step.Ord] = true
	}
	for _, step := range steps {
		switch step.Tipo {
		case "goto":
			target := reactChatflowConfigInt(step.Config, "target")
			if target <= 0 || !ords[target] {
				return errors.New("o bloco de salto (goto) aponta para um bloco inexistente")
			}
		case "condition":
			if reactChatflowConfigInt(step.Config, "then") <= 0 && reactChatflowConfigInt(step.Config, "else") <= 0 {
				return errors.New("o bloco de condição precisa de ao menos um ramo (então/senão)")
			}
		case "menu":
			if !reactChatflowOptionsHaveNext(step.Config, "options") {
				return errors.New("o bloco de menu precisa de ao menos uma opção com destino")
			}
		case "intent":
			if !reactChatflowOptionsHaveNext(step.Config, "branches") {
				return errors.New("o bloco de intenção precisa de ao menos um ramo com destino")
			}
		}
	}
	return nil
}

func reactChatflowConfigInt(config map[string]interface{}, key string) int {
	if config == nil {
		return 0
	}
	switch value := config[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(value))
		return parsed
	default:
		return 0
	}
}

func reactChatflowOptionsHaveNext(config map[string]interface{}, key string) bool {
	items, ok := config[key].([]interface{})
	if !ok {
		return false
	}
	for _, raw := range items {
		item, ok := raw.(map[string]interface{})
		if ok && reactChatflowConfigInt(item, "next") > 0 {
			return true
		}
	}
	return false
}

func chatflowConfigBytes(config map[string]interface{}) (string, error) {
	if config == nil {
		return "{}", nil
	}
	bytes, err := json.Marshal(config)
	if err != nil {
		return "{}", err
	}
	return string(bytes), nil
}

func (s *Server) applyReactChatflowSteps(ctx context.Context, chatflowID int, steps []reactChatflowStepInput) error {
	for _, step := range steps {
		if strings.TrimSpace(step.Tipo) == "" || !chatflowStepTipos[step.Tipo] {
			return errors.New("tipo de passo inválido")
		}
		if step.Ord < 1 {
			step.Ord = 1
		}
		config, err := chatflowConfigBytes(step.Config)
		if err != nil {
			return errors.New("config do passo inválida")
		}
		if _, err := s.st.CreateChatflowStep(ctx, store.ChatflowStep{ChatflowID: chatflowID, Ord: step.Ord, Tipo: step.Tipo, Config: config}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) replaceReactChatflowSteps(ctx context.Context, chatflowID int, steps []reactChatflowStepInput) error {
	existing, err := s.st.ListChatflowSteps(ctx, chatflowID)
	if err != nil {
		return err
	}
	for _, step := range existing {
		if err := s.st.DeleteChatflowStep(ctx, step.ID); err != nil {
			return err
		}
	}
	return s.applyReactChatflowSteps(ctx, chatflowID, steps)
}

func (s *Server) handleReactChatflows(w http.ResponseWriter, r *http.Request) {
	flows, err := s.st.ListChatflows(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os chat flows")
		return
	}
	items := make([]reactChatflow, 0, len(flows))
	for _, flow := range flows {
		steps, err := s.st.ListChatflowSteps(r.Context(), flow.ID)
		if err != nil {
			writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os passos do chat flow")
			return
		}
		items = append(items, reactChatflowFromStore(flow, steps))
	}
	channels, err := s.reactChatflowChannels(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os canais")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": items, "channels": channels}, nil)
}

func (s *Server) handleReactChatflowGet(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	flow, err := s.loadReactChatflow(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "chat flow não encontrado")
		return
	}
	writeReactResponse(w, http.StatusOK, flow, nil)
}

func (s *Server) handleReactChatflowCreate(w http.ResponseWriter, r *http.Request) {
	input, err := decodeReactChatflowInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	nome := ""
	if input.Nome != nil {
		nome = strings.TrimSpace(*input.Nome)
	}
	if err := validateChatflowName(nome); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_flow", err.Error())
		return
	}
	ativo := false
	if input.Ativo != nil {
		ativo = *input.Ativo
	}
	if input.Steps != nil {
		if err := validateChatflowSteps(*input.Steps); err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_step", err.Error())
			return
		}
	}
	config, _ := chatflowConfigBytes(mapOrNil(input.Config))
	id, err := s.st.CreateChatflow(r.Context(), store.Chatflow{Nome: nome, Version: 0, Ativo: ativo, Config: config})
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível criar o chat flow")
		return
	}
	if input.Steps != nil {
		if err := s.applyReactChatflowSteps(r.Context(), id, *input.Steps); err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_step", err.Error())
			return
		}
	}
	flow, err := s.loadReactChatflow(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "chat flow criado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusCreated, flow, nil)
}

func (s *Server) handleReactChatflowUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	current, err := s.st.GetChatflow(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "chat flow não encontrado")
		return
	}
	input, err := decodeReactChatflowInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if input.Nome != nil {
		current.Nome = strings.TrimSpace(*input.Nome)
	}
	if err := validateChatflowName(current.Nome); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_flow", err.Error())
		return
	}
	if input.Version != nil {
		current.Version = *input.Version
	}
	if input.Config != nil {
		config, _ := chatflowConfigBytes(*input.Config)
		current.Config = config
	}
	if input.Ativo != nil && *input.Ativo != current.Ativo {
		if err := s.st.ActivateChatflow(r.Context(), id, *input.Ativo); err != nil {
			writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível alterar o estado do chat flow")
			return
		}
		current.Ativo = *input.Ativo
	}
	if err := s.st.UpdateChatflow(r.Context(), current); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível atualizar o chat flow")
		return
	}
	if input.Steps != nil {
		if err := validateChatflowSteps(*input.Steps); err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_step", err.Error())
			return
		}
		if err := s.replaceReactChatflowSteps(r.Context(), id, *input.Steps); err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_step", err.Error())
			return
		}
	}
	flow, err := s.loadReactChatflow(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "chat flow atualizado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusOK, flow, nil)
}

func (s *Server) handleReactChatflowActivate(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	flow, err := s.st.GetChatflow(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "chat flow não encontrado")
		return
	}
	flow.Ativo = !flow.Ativo
	if err := s.st.ActivateChatflow(r.Context(), id, flow.Ativo); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível alterar o estado do chat flow")
		return
	}
	writeReactResponse(w, http.StatusOK, flow.Ativo, nil)
}

func (s *Server) handleReactChatflowDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if _, err := s.st.GetChatflow(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "chat flow não encontrado")
		return
	}
	if err := s.st.DeleteChatflow(r.Context(), id); err != nil {
		writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir o chat flow")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"deleted": id}, nil)
}

func (s *Server) handleReactChatflowAttach(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if _, err := s.st.GetChatflow(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "chat flow não encontrado")
		return
	}
	var input struct {
		ChannelID *int  `json:"channel_id"`
		Attached  *bool `json:"attached"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024+1)).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	if input.ChannelID == nil {
		writeReactError(w, http.StatusBadRequest, "invalid_channel", "canal não informado")
		return
	}
	if _, err := s.st.GetChannelByID(r.Context(), *input.ChannelID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "invalid_channel", "canal não encontrado")
		return
	}
	var chatflow *int
	if input.Attached != nil && !*input.Attached {
		chatflow = nil
	} else {
		chatflow = &id
	}
	if err := s.st.SetChannelChatflow(r.Context(), *input.ChannelID, chatflow); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "attach_failed", "não foi possível vincular o canal ao chat flow")
		return
	}
	channels, err := s.reactChatflowChannels(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível recarregar os canais")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"channels": channels}, nil)
}

func mapOrNil(input *map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}
	return *input
}
