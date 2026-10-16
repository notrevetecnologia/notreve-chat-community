package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func reactResourceID(path, resource string) (int, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] != resource {
			continue
		}
		id, err := strconv.Atoi(parts[i+1])
		return id, err == nil && id > 0
	}
	return 0, false
}

func readJSONProbe(r *http.Request, target interface{}) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 256*1024+1))
	if err != nil || len(body) > 256*1024 {
		return errors.New("payload inválido")
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	return json.Unmarshal(body, target)
}

func (s *Server) writeSaaSLimitError(w http.ResponseWriter, r *http.Request, err error) {
	if writeReactPlanLimitError(w, err) {
		return
	}
	if errors.Is(err, store.ErrTenantAttendantLimitReached) {
		writeReactError(w, http.StatusConflict, "attendant_limit_reached", "limite de atendentes ativos atingido")
		return
	}
	var channelErr *store.TenantChannelTypeLimitError
	if errors.As(err, &channelErr) {
		writeReactError(w, http.StatusConflict, "channel_limit_reached", "limite de canais atingido")
		return
	}
	writeReactError(w, http.StatusInternalServerError, "plan_check_failed", "não foi possível validar o limite do plano")
}

func attendantWillConsumeSlot(s *Server, r *http.Request, user store.Usuario, targetActive bool, targetRole string) (bool, error) {
	if !targetActive || targetRole == "super_admin" {
		return false, nil
	}
	if user.AtendenteID == nil {
		return true, nil
	}
	attendant, err := s.st.GetAtendente(r.Context(), *user.AtendenteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil
		}
		return false, err
	}
	return !attendant.Active, nil
}

func validBcryptPasswordSize(password *string) bool {
	return password == nil || strings.TrimSpace(*password) == "" || len([]byte(strings.TrimSpace(*password))) <= 72
}

func (s *Server) preflightReactAgentLimit(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodPost && r.URL.Path == "/api/react/agents" {
		return true
	}
	id, ok := reactResourceID(r.URL.Path, "agents")
	if !ok {
		return true
	}
	agent, err := s.st.GetAgent(r.Context(), id)
	if err != nil {
		return true
	}
	activating := false
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/toggle") {
		activating = !agent.Active
	} else if r.Method == http.MethodPatch {
		var probe struct{ Active *bool `json:"active"` }
		if err := readJSONProbe(r, &probe); err != nil {
			return true
		}
		activating = !agent.Active && probe.Active != nil && *probe.Active
	}
	if activating {
		if err := s.st.EnsureCanCreateActiveAgent(r.Context()); err != nil {
			s.writeSaaSLimitError(w, r, err)
			return false
		}
	}
	return true
}

func (s *Server) preflightReactUserLimit(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/api/react/users" && r.Method == http.MethodPost {
		var probe struct {
			Active   *bool   `json:"active"`
			Role     *string `json:"role"`
			Password *string `json:"password"`
		}
		if err := readJSONProbe(r, &probe); err != nil {
			return true
		}
		if !validBcryptPasswordSize(probe.Password) {
			writeReactError(w, http.StatusBadRequest, "invalid_password", "senha deve ter no máximo 72 bytes")
			return false
		}
		active := probe.Active == nil || *probe.Active
		role := "atendente"
		if probe.Role != nil {
			role = strings.ToLower(strings.TrimSpace(*probe.Role))
		}
		if active && role != "super_admin" {
			if err := s.st.EnsureCanCreateActiveAttendant(r.Context()); err != nil {
				s.writeSaaSLimitError(w, r, err)
				return false
			}
		}
		return true
	}
	id, ok := reactResourceID(r.URL.Path, "users")
	if !ok {
		return true
	}
	user, err := s.st.GetUsuarioByID(r.Context(), id)
	if err != nil {
		return true
	}
	activatingUser := false
	targetActive := user.Active
	targetRole := user.Role
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/toggle") {
		targetActive = !user.Active
		activatingUser = targetActive
	} else if r.Method == http.MethodPatch {
		var probe struct {
			Active   *bool   `json:"active"`
			Role     *string `json:"role"`
			Password *string `json:"password"`
		}
		if err := readJSONProbe(r, &probe); err != nil {
			return true
		}
		if !validBcryptPasswordSize(probe.Password) {
			writeReactError(w, http.StatusBadRequest, "invalid_password", "senha deve ter no máximo 72 bytes")
			return false
		}
		if probe.Active != nil {
			targetActive = *probe.Active
		}
		if probe.Role != nil {
			targetRole = strings.ToLower(strings.TrimSpace(*probe.Role))
		}
		activatingUser = !user.Active && targetActive
	}
	if activatingUser {
		if err := s.st.EnsureCanCreateActiveUser(r.Context()); err != nil {
			s.writeSaaSLimitError(w, r, err)
			return false
		}
	}
	consumeAttendant, err := attendantWillConsumeSlot(s, r, user, targetActive, targetRole)
	if err != nil {
		s.writeSaaSLimitError(w, r, err)
		return false
	}
	if consumeAttendant {
		if err := s.st.EnsureCanCreateActiveAttendant(r.Context()); err != nil {
			s.writeSaaSLimitError(w, r, err)
			return false
		}
	}
	return true
}

func (s *Server) preflightSaaSResourceLimits(w http.ResponseWriter, r *http.Request) bool {
	// O contrato canônico de conversas precisa ser resolvido antes de cair no
	// roteador legado. O dispatcher aplica authJSON/permissões por conta própria.
	if s.dispatchSaaSCanonicalConversationRoute(w, r) {
		return false
	}
	if _, err := store.TenantIDRequired(r.Context()); err != nil {
		return true
	}
	if strings.HasPrefix(r.URL.Path, "/api/react/agents") {
		return s.preflightReactAgentLimit(w, r)
	}
	if strings.HasPrefix(r.URL.Path, "/api/react/users") {
		return s.preflightReactUserLimit(w, r)
	}
	return true
}
