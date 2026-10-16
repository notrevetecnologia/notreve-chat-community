package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

// Estes handlers são a superfície administrativa global. Eles nunca obtêm a
// empresa a partir da sessão: o super_admin sempre informa a empresa-alvo e
// os métodos da Store recebem esse ID explicitamente.
type reactPlatformTenantInput struct {
	Name     string                      `json:"name"`
	Slug     string                      `json:"slug"`
	Plan     string                      `json:"plan"`
	Status   string                      `json:"status"`
	Timezone string                      `json:"timezone"`
	Locale   string                      `json:"locale"`
	Dominio  string                      `json:"dominio"`
	Creditos int                         `json:"creditos"`
	Limits   *store.PlatformTenantLimits `json:"limits"`
	Modules  []string                    `json:"modules"`
}

type reactPlatformTenant struct {
	store.PlatformTenant
	Modules []string `json:"modules"`
}

type reactPlatformWhatsAppServerInput struct {
	Name     string `json:"name"`
	BaseURL  string `json:"base_url"`
	Token    string `json:"token"`
	Capacity int    `json:"capacity"`
	Status   string `json:"status"`
}

type reactPlatformWhatsAppServer struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	BaseURL           string `json:"base_url"`
	Capacity          int    `json:"capacity"`
	Status            string `json:"status"`
	Usage             int    `json:"usage"`
	ActiveConnections int    `json:"active_connections"`
	TokenConfigured   bool   `json:"token_configured"`
}

func platformTenantInputFromReact(input reactPlatformTenantInput) store.PlatformTenantInput {
	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status == "archived" {
		status = "closed"
	}
	locale := strings.TrimSpace(input.Locale)
	if locale == "" {
		locale = "pt-BR"
	}
	dominio := strings.TrimSpace(input.Dominio)
	dominio = strings.TrimPrefix(strings.TrimPrefix(dominio, "https://"), "http://")
	dominio = strings.TrimSpace(strings.TrimSuffix(dominio, "/"))
	return store.PlatformTenantInput{
		Name: strings.TrimSpace(input.Name), Slug: strings.ToLower(strings.TrimSpace(input.Slug)),
		Plan: strings.TrimSpace(input.Plan), Status: status, Timezone: strings.TrimSpace(input.Timezone), Locale: locale,
		Dominio: dominio, Creditos: input.Creditos,
	}
}

func platformTenantModules(ctx *http.Request, st *store.Store, tenantID int) ([]string, error) {
	items, err := st.ListPlatformTenantModules(ctx.Context(), tenantID)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(items))
	for _, item := range items {
		if item.Enabled {
			keys = append(keys, item.Key)
		}
	}
	return keys, nil
}

func (s *Server) platformTenantResponse(r *http.Request, tenant store.PlatformTenant) (reactPlatformTenant, error) {
	modules, err := platformTenantModules(r, s.st, tenant.ID)
	if err != nil {
		return reactPlatformTenant{}, err
	}
	return reactPlatformTenant{PlatformTenant: tenant, Modules: modules}, nil
}

func (s *Server) handleReactPlatformTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := s.st.ListPlatformTenants(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as empresas")
		return
	}
	out := make([]reactPlatformTenant, 0, len(tenants))
	for _, tenant := range tenants {
		item, err := s.platformTenantResponse(r, tenant)
		if err != nil {
			writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os módulos da empresa")
			return
		}
		out = append(out, item)
	}
	writeReactResponse(w, http.StatusOK, out, map[string]interface{}{"scope": "platform"})
}

func (s *Server) handleReactPlatformTenantUsage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id < 1 {
		writeReactError(w, http.StatusBadRequest, "invalid_id", "id inválido")
		return
	}
	usage, err := s.st.GetPlatformTenantUsage(r.Context(), int(id))
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar o consumo")
		return
	}
	limits, err := s.st.GetPlatformTenantLimits(r.Context(), int(id))
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os limites")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"usage": usage, "limits": limits}, nil)
}

func decodeReactPlatformTenant(r *http.Request) (reactPlatformTenantInput, error) {
	var input reactPlatformTenantInput
	if err := decodeReactAdminJSON(r, &input, 64*1024); err != nil {
		return input, err
	}
	return input, nil
}

func platformKnownModules(ctx *http.Request, st *store.Store, tenantID int) ([]store.Module, error) {
	return st.ListPlatformTenantModules(ctx.Context(), tenantID)
}

func (s *Server) savePlatformTenantModules(r *http.Request, tenantID int, requested []string) error {
	wanted := make(map[string]bool, len(requested))
	for _, key := range requested {
		key = strings.TrimSpace(key)
		if key != "" {
			wanted[key] = true
		}
	}
	modules, err := platformKnownModules(r, s.st, tenantID)
	if err != nil {
		return err
	}
	for _, module := range modules {
		if err := s.st.SetPlatformTenantModule(r.Context(), tenantID, module.Key, wanted[module.Key]); err != nil {
			if errors.Is(err, store.ErrCoreModuleCannotBeDisabled) {
				continue
			}
			return err
		}
	}
	return nil
}

func (s *Server) handleReactPlatformTenantCreate(w http.ResponseWriter, r *http.Request) {
	input, err := decodeReactPlatformTenant(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	tenant, err := s.st.CreatePlatformTenant(r.Context(), platformTenantInputFromReact(input))
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_tenant", "não foi possível criar a empresa; revise nome, slug, plano, status e fuso")
		return
	}
	if input.Limits != nil {
		if err := s.st.UpdatePlatformTenantLimits(r.Context(), tenant.ID, *input.Limits); err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_limits", "os limites da empresa são inválidos")
			return
		}
	}
	if err := s.savePlatformTenantModules(r, tenant.ID, input.Modules); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_modules", "não foi possível definir os módulos da empresa")
		return
	}
	tenant, err = s.st.GetPlatformTenant(r.Context(), tenant.ID)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "empresa criada, mas não foi possível carregá-la")
		return
	}
	out, err := s.platformTenantResponse(r, tenant)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar a empresa")
		return
	}
	writeReactResponse(w, http.StatusCreated, out, nil)
}

func platformTenantID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		return 0, errors.New("empresa inválida")
	}
	return id, nil
}

func (s *Server) handleReactPlatformTenantUpdate(w http.ResponseWriter, r *http.Request) {
	tenantID, err := platformTenantID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	input, err := decodeReactPlatformTenant(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := s.st.UpdatePlatformTenant(r.Context(), tenantID, platformTenantInputFromReact(input)); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrPlatformTenantNotFound) || errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "invalid_tenant", "não foi possível atualizar a empresa")
		return
	}
	if input.Limits != nil {
		if err := s.st.UpdatePlatformTenantLimits(r.Context(), tenantID, *input.Limits); err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_limits", "os limites da empresa são inválidos")
			return
		}
	}
	if err := s.savePlatformTenantModules(r, tenantID, input.Modules); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_modules", "não foi possível definir os módulos da empresa")
		return
	}
	tenant, err := s.st.GetPlatformTenant(r.Context(), tenantID)
	if err != nil {
		writeReactError(w, http.StatusNotFound, "not_found", "empresa não encontrada")
		return
	}
	out, err := s.platformTenantResponse(r, tenant)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar a empresa")
		return
	}
	writeReactResponse(w, http.StatusOK, out, nil)
}

func reactWhatsAppServerFromStore(server store.WhatsAppServer) reactPlatformWhatsAppServer {
	return reactPlatformWhatsAppServer{ID: server.ID, Name: server.Name, BaseURL: server.BaseURL,
		Capacity: server.MaxConnections, Status: server.Status, Usage: server.ActiveConnections,
		ActiveConnections: server.ActiveConnections, TokenConfigured: server.TokenConfigured}
}

func (s *Server) handleReactPlatformWhatsAppServers(w http.ResponseWriter, r *http.Request) {
	servers, err := s.st.ListWhatsAppServers(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os servidores WhatsApp")
		return
	}
	out := make([]reactPlatformWhatsAppServer, 0, len(servers))
	for _, server := range servers {
		out = append(out, reactWhatsAppServerFromStore(server))
	}
	writeReactResponse(w, http.StatusOK, out, map[string]interface{}{"scope": "platform"})
}

func decodeReactWhatsAppServer(r *http.Request) (reactPlatformWhatsAppServerInput, error) {
	var input reactPlatformWhatsAppServerInput
	if err := decodeReactAdminJSON(r, &input, 32*1024); err != nil {
		return input, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.BaseURL = strings.TrimSpace(input.BaseURL)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	return input, nil
}

func storeWhatsAppServerInput(input reactPlatformWhatsAppServerInput) (store.WhatsAppServerInput, error) {
	if input.Status != "active" && input.Status != "maintenance" && input.Status != "disabled" {
		return store.WhatsAppServerInput{}, errors.New("status inválido")
	}
	return store.WhatsAppServerInput{Name: input.Name, BaseURL: input.BaseURL, Token: input.Token,
		Status: input.Status, MaxConnections: input.Capacity}, nil
}

func (s *Server) handleReactPlatformWhatsAppServerCreate(w http.ResponseWriter, r *http.Request) {
	input, err := decodeReactWhatsAppServer(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	serverInput, err := storeWhatsAppServerInput(input)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_server", err.Error())
		return
	}
	server, err := s.st.CreateWhatsAppServerSecure(r.Context(), serverInput)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_server", "não foi possível cadastrar o servidor; informe URL, token e capacidade válidos")
		return
	}
	writeReactResponse(w, http.StatusCreated, reactWhatsAppServerFromStore(server), nil)
}

func (s *Server) handleReactPlatformWhatsAppServerUpdate(w http.ResponseWriter, r *http.Request) {
	serverID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || serverID < 1 {
		writeReactError(w, http.StatusBadRequest, "invalid_id", "servidor inválido")
		return
	}
	input, err := decodeReactWhatsAppServer(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	serverInput, err := storeWhatsAppServerInput(input)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_server", err.Error())
		return
	}
	if err := s.st.UpdateWhatsAppServerSecure(r.Context(), serverID, serverInput); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_server", "não foi possível atualizar o servidor")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"id": serverID}, nil)
}

// reactPlatformContextSwitchInput carrega o motivo que será gravado na
// auditoria da troca de contexto. O motivo é obrigatório para rastrear por que
// um super_admin assumiu o contexto de uma empresa.
type reactPlatformContextSwitchInput struct {
	Reason string `json:"reason"`
}

func decodeReactContextSwitch(r *http.Request) (reactPlatformContextSwitchInput, error) {
	var input reactPlatformContextSwitchInput
	if err := decodeReactAdminJSON(r, &input, 16*1024); err != nil {
		return input, err
	}
	input.Reason = strings.TrimSpace(input.Reason)
	return input, nil
}

func (s *Server) handleReactPlatformContextSwitch(w http.ResponseWriter, r *http.Request) {
	tenantID, err := platformTenantID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	input, err := decodeReactContextSwitch(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if input.Reason == "" || len([]rune(input.Reason)) > 500 {
		writeReactError(w, http.StatusBadRequest, "reason_required", "informe um motivo de até 500 caracteres")
		return
	}
	tenant, err := s.st.GetPlatformTenant(r.Context(), tenantID)
	if err != nil {
		writeReactError(w, http.StatusNotFound, "not_found", "empresa não encontrada")
		return
	}
	if tenant.Status != "active" && tenant.Status != "trial" {
		writeReactError(w, http.StatusConflict, "tenant_unavailable", "a empresa precisa estar ativa ou em trial")
		return
	}
	actor := currentUser(r)
	if actor == nil {
		writeReactError(w, http.StatusUnauthorized, "unauthorized", "sessão não identificada")
		return
	}
	audit, err := s.st.RecordContextSwitch(r.Context(), *actor, actor.TenantID, tenantID, input.Reason)
	if err != nil {
		status := http.StatusBadRequest
		code := "invalid_switch"
		if errors.Is(err, store.ErrPlatformTenantNotFound) {
			status, code = http.StatusNotFound, "not_found"
		}
		writeReactError(w, status, code, "não foi possível registrar a troca de contexto")
		return
	}
	out, err := s.platformTenantResponse(r, tenant)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar a empresa")
		return
	}
	writeReactResponse(w, http.StatusOK, out, map[string]interface{}{"audit": audit})
}

func (s *Server) handleReactPlatformContextSwitchAudit(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil {
			limit = parsed
		}
	}
	items, err := s.st.ListContextSwitchAudit(r.Context(), limit)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar o histórico de trocas de contexto")
		return
	}
	writeReactResponse(w, http.StatusOK, items, map[string]interface{}{"scope": "platform"})
}
