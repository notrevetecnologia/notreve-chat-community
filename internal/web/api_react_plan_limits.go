package web

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

// writeReactPlanLimitError converte apenas erros esperados de franquia em uma
// resposta de produto. Contagens e dados de outras empresas nunca entram no
// corpo da resposta.
func writeReactPlanLimitError(w http.ResponseWriter, err error) bool {
	var code, message string
	switch {
	case errors.Is(err, store.ErrTenantAgentLimitReached):
		code, message = "agent_limit_reached", "o limite de agentes ativos do plano foi atingido"
	case errors.Is(err, store.ErrTenantUserLimitReached):
		code, message = "user_limit_reached", "o limite de usuários ativos do plano foi atingido"
	case errors.Is(err, store.ErrTenantToolLimitReached):
		code, message = "tool_limit_reached", "o limite de tools do plano foi atingido"
	case errors.Is(err, store.ErrTenantAPIKeyLimitReached):
		code, message = "api_key_limit_reached", "o limite de chaves de API do plano foi atingido"
	case errors.Is(err, store.ErrTenantWebhookLimitReached):
		code, message = "webhook_limit_reached", "o limite de webhooks do plano foi atingido"
	case errors.Is(err, store.ErrTenantAICreditLimitReached):
		code, message = "ai_credit_limit_reached", "os créditos de IA deste plano foram consumidos"
	case errors.Is(err, store.ErrTenantPlanLimitsUnavailable):
		writeReactError(w, http.StatusServiceUnavailable, "plan_unavailable", "os limites do plano ainda não estão disponíveis")
		return true
	default:
		return false
	}
	writeReactError(w, http.StatusConflict, code, message)
	return true
}

type reactPlanInput struct {
	TenantOwner *int                `json:"tenant_owner"`
	Nome        string              `json:"nome"`
	Preco       float64             `json:"preco"`
	Intervalo   string              `json:"intervalo"`
	Ativo       bool                `json:"ativo"`
	Recursos    []store.PlanoRecurso `json:"recursos"`
}

type reactTenantPlanInput struct {
	PlanoID        *int               `json:"plano_id"`
	TrialUntil     *time.Time         `json:"trial_until"`
	Status         string             `json:"status"`
	LimitsOverride map[string]float64 `json:"limits_override"`
}

func reactPlanToStore(input reactPlanInput) store.Plano {
	return store.Plano{
		TenantOwner: input.TenantOwner,
		Nome:        input.Nome,
		Preco:       input.Preco,
		Intervalo:   input.Intervalo,
		Ativo:       input.Ativo,
		Recursos:    input.Recursos,
	}
}

func reactTenantPlanToStore(input reactTenantPlanInput) store.TenantPlanAssignment {
	return store.TenantPlanAssignment{
		PlanoID:        input.PlanoID,
		TrialUntil:     input.TrialUntil,
		Status:         input.Status,
		LimitsOverride: input.LimitsOverride,
	}
}

func reactPlanID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		return 0, errors.New("id inválido")
	}
	return id, nil
}

func (s *Server) handleReactPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := s.st.ListPlans(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os planos")
		return
	}
	if plans == nil {
		plans = []store.Plano{}
	}
	writeReactResponse(w, http.StatusOK, plans, nil)
}

func (s *Server) handleReactPlanCreate(w http.ResponseWriter, r *http.Request) {
	var input reactPlanInput
	if err := decodeReactAdminJSON(r, &input, 64*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	created, err := s.st.CreatePlan(r.Context(), reactPlanToStore(input))
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_plan", "não foi possível criar o plano; revise nome, preço, intervalo e recursos")
		return
	}
	writeReactResponse(w, http.StatusCreated, created, nil)
}

func (s *Server) handleReactPlanUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := reactPlanID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	var input reactPlanInput
	if err := decodeReactAdminJSON(r, &input, 64*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	updated, err := s.st.UpdatePlan(r.Context(), id, reactPlanToStore(input))
	if err != nil {
		if errors.Is(err, store.ErrPlanNotFound) {
			writeReactError(w, http.StatusNotFound, "not_found", "plano não encontrado")
			return
		}
		writeReactError(w, http.StatusBadRequest, "invalid_plan", "não foi possível atualizar o plano")
		return
	}
	writeReactResponse(w, http.StatusOK, updated, nil)
}

func (s *Server) handleReactPlanDelete(w http.ResponseWriter, r *http.Request) {
	id, err := reactPlanID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if err := s.st.DeletePlan(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, store.ErrPlanNotFound):
			writeReactError(w, http.StatusNotFound, "not_found", "plano não encontrado")
		case errors.Is(err, store.ErrPlanInUse):
			writeReactError(w, http.StatusConflict, "plan_in_use", "este plano está atribuído a uma empresa e não pode ser excluído")
		default:
			writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir o plano")
		}
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]bool{"deleted": true}, nil)
}

func (s *Server) handleReactTenantPlan(w http.ResponseWriter, r *http.Request) {
	id, err := reactPlanID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	limits, err := s.st.GetTenantPlan(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrTenantNotFound) {
			writeReactError(w, http.StatusNotFound, "not_found", "empresa não encontrada")
			return
		}
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os limites efetivos")
		return
	}
	writeReactResponse(w, http.StatusOK, limits, nil)
}

func (s *Server) handleReactTenantPlanUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := reactPlanID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	var input reactTenantPlanInput
	if err := decodeReactAdminJSON(r, &input, 64*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	limits, err := s.st.SetTenantPlan(r.Context(), id, reactTenantPlanToStore(input))
	if err != nil {
		switch {
		case errors.Is(err, store.ErrTenantNotFound):
			writeReactError(w, http.StatusNotFound, "not_found", "empresa não encontrada")
		case errors.Is(err, store.ErrPlanNotFound):
			writeReactError(w, http.StatusBadRequest, "invalid_plan", "plano não encontrado")
		default:
			writeReactError(w, http.StatusBadRequest, "invalid_assignment", "não foi possível atribuir o plano à empresa")
		}
		return
	}
	writeReactResponse(w, http.StatusOK, limits, nil)
}
