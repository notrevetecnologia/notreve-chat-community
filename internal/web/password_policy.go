package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type saasPasswordChangeInput struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (s *Server) handleSaaSSession(w http.ResponseWriter, r *http.Request) {
	data, err := s.reactSessionData(r)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar a sessão")
		return
	}
	mustChange, err := s.currentUserMustChangePassword(r)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível validar a política de senha")
		return
	}
	data["must_change_password"] = mustChange
	writeReactResponse(w, http.StatusOK, data, nil)
}

func (s *Server) handleSaaSBootstrap(w http.ResponseWriter, r *http.Request) {
	data, err := s.reactSessionData(r)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar o bootstrap")
		return
	}
	mustChange, err := s.currentUserMustChangePassword(r)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível validar a política de senha")
		return
	}
	data["must_change_password"] = mustChange
	data["conversation_statuses"] = []string{"ia", "humano_aguardando", "humano_atendendo", "closed"}
	data["pagination"] = map[string]interface{}{"default_limit": 25, "max_limit": 100, "cursor": "offset"}
	data["conversation_capabilities"] = map[string]interface{}{
		"reply":     true,
		"media":     map[string]interface{}{"upload": "/api/react/conversations/{id}/messages", "download": "/api/react/messages/{id}/media", "max_bytes": 50 << 20},
		"audio":     true,
		"typing":    "/api/react/conversations/{id}/actions",
		"reactions": "/api/react/conversations/{id}/actions",
		"edit":      "/api/react/conversations/{id}/messages/{message_id}",
	}
	if current := currentUser(r); current != nil && current.Role == "super_admin" {
		data["platform_mode"] = true
		data["transfer_targets"] = []reactTransferTarget{}
		writeReactResponse(w, http.StatusOK, data, nil)
		return
	}
	if userHasPerm(currentUser(r), permConversasTransferir) {
		targets, err := s.reactTransferTargets(r.Context(), currentUser(r))
		if err != nil {
			writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os destinos de transferência")
			return
		}
		data["transfer_targets"] = targets
	} else {
		data["transfer_targets"] = []reactTransferTarget{}
	}
	writeReactResponse(w, http.StatusOK, data, nil)
}

func (s *Server) currentUserMustChangePassword(r *http.Request) (bool, error) {
	user := currentUser(r)
	if user == nil || user.Role == "super_admin" {
		return false, nil
	}
	return s.st.UserMustChangePassword(r.Context(), user.ID)
}

func decodeSaaSPasswordChange(r *http.Request) (saasPasswordChangeInput, error) {
	var input saasPasswordChangeInput
	err := json.NewDecoder(io.LimitReader(r.Body, 16*1024+1)).Decode(&input)
	input.CurrentPassword = strings.TrimSpace(input.CurrentPassword)
	input.NewPassword = strings.TrimSpace(input.NewPassword)
	return input, err
}

func validateNewUserPassword(password string) string {
	if len([]rune(password)) < 8 {
		return "a nova senha deve ter pelo menos 8 caracteres"
	}
	if len([]byte(password)) > 72 {
		return "a nova senha deve ter no máximo 72 bytes"
	}
	return ""
}

func (s *Server) handleSaaSPasswordChange(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		writeReactError(w, http.StatusUnauthorized, "unauthorized", "não autenticado")
		return
	}
	input, err := decodeSaaSPasswordChange(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.CurrentPassword)) != nil {
		writeReactError(w, http.StatusUnauthorized, "invalid_current_password", "senha atual incorreta")
		return
	}
	if message := validateNewUserPassword(input.NewPassword); message != "" {
		writeReactError(w, http.StatusBadRequest, "invalid_password", message)
		return
	}
	if input.CurrentPassword == input.NewPassword {
		writeReactError(w, http.StatusBadRequest, "password_unchanged", "a nova senha deve ser diferente da senha atual")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "password_failed", "não foi possível proteger a nova senha")
		return
	}
	if err := s.st.ChangeOwnPassword(r.Context(), user.ID, string(hash)); err != nil {
		writeReactError(w, http.StatusInternalServerError, "password_failed", "não foi possível alterar a senha")
		return
	}
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		s.invalidateSession(w, r, cookie.Value)
	}
	w.WriteHeader(http.StatusNoContent)
}
