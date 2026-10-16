package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type reactAPIKeyInput struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type reactAPIKeyToggleInput struct {
	Active bool `json:"active"`
}

func (s *Server) handleReactAPIKeys(w http.ResponseWriter, r *http.Request) {
	items, err := s.st.ListAPIKeys(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "api_keys_load_failed", "não foi possível carregar as chaves")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": items}, nil)
}

func (s *Server) handleReactAPIKeyCreate(w http.ResponseWriter, r *http.Request) {
	var input reactAPIKeyInput
	decoder := json.NewDecoder(io.LimitReader(r.Body, 32<<10))
	if err := decoder.Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len([]rune(input.Name)) > 120 {
		writeReactError(w, http.StatusBadRequest, "invalid_name", "informe um nome entre 1 e 120 caracteres")
		return
	}
	if len(input.Scopes) == 0 {
		input.Scopes = []string{"conversations:read", "conversations:write"}
	}
	if err := s.st.EnsureCanCreateAPIKey(r.Context()); err != nil {
		if writeReactPlanLimitError(w, err) {
			return
		}
		writeReactError(w, http.StatusInternalServerError, "plan_check_failed", "não foi possível validar o limite do plano")
		return
	}
	credential, err := s.st.CreateAPIKey(r.Context(), input.Name, input.Scopes)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "api_key_create_failed", "não foi possível criar a chave")
		return
	}
	writeReactResponse(w, http.StatusCreated, map[string]interface{}{
		"api_key": credential.APIKey,
		"secret":  credential.Secret,
	}, nil)
}

func (s *Server) handleReactAPIKeyToggle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id < 1 {
		writeReactError(w, http.StatusBadRequest, "invalid_id", "chave inválida")
		return
	}
	var input reactAPIKeyToggleInput
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	if err := s.st.SetAPIKeyActive(r.Context(), id, input.Active); err != nil {
		writeReactError(w, http.StatusNotFound, "api_key_not_found", "chave não encontrada")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]bool{"active": input.Active}, nil)
}

func (s *Server) handleReactAPIKeyDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id < 1 {
		writeReactError(w, http.StatusBadRequest, "invalid_id", "chave inválida")
		return
	}
	if err := s.st.DeleteAPIKey(r.Context(), id); err != nil {
		writeReactError(w, http.StatusNotFound, "api_key_not_found", "chave não encontrada")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]bool{"deleted": true}, nil)
}
