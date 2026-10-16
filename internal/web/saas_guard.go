package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type saasTenantLimits struct {
	MaxCanais            int   `json:"max_canais"`
	MaxAtendentes        int   `json:"max_atendentes"`
	MaxCampanhasAtivas   int   `json:"max_campanhas_ativas"`
	CampanhaLimiteDiario int   `json:"campanha_limite_diario"`
	MaxAgentes           int   `json:"max_agentes"`
	MaxUsuarios          int   `json:"max_usuarios"`
	MaxTools             int   `json:"max_tools"`
	MaxAPIKeys           int   `json:"max_api_keys"`
	MaxWebhooks          int   `json:"max_webhooks"`
	CreditosIAMensais    int64 `json:"creditos_ia_mensais"`
}

type saasTenantCreateInput struct {
	Name          string                           `json:"name"`
	Slug          string                           `json:"slug"`
	Plan          string                           `json:"plan"`
	Status        string                           `json:"status"`
	Timezone      string                           `json:"timezone"`
	Locale        string                           `json:"locale"`
	Dominio       string                           `json:"dominio"`
	Creditos      int                              `json:"creditos"`
	Limits        *saasTenantLimits                `json:"limits"`
	ChannelLimits *store.PlatformChannelTypeLimits `json:"channel_limits,omitempty"`
	Modules       []string                         `json:"modules"`
	Admin         *reactPlatformAdminInput         `json:"admin,omitempty"`
}

type saasTenantResponse struct {
	ID            int                             `json:"id"`
	Slug          string                          `json:"slug"`
	Name          string                          `json:"name"`
	Status        string                          `json:"status"`
	Plan          string                          `json:"plan"`
	Timezone      string                          `json:"timezone"`
	Locale        string                          `json:"locale"`
	Dominio       string                          `json:"dominio"`
	Creditos      int                             `json:"creditos"`
	Limits        saasTenantLimits                `json:"limits"`
	ChannelLimits store.PlatformChannelTypeLimits `json:"channel_limits"`
	Usage         store.TenantUsage               `json:"usage"`
	Modules       []string                        `json:"modules"`
	CreatedAt     time.Time                       `json:"created_at"`
	UpdatedAt     time.Time                       `json:"updated_at"`
}

type saasChannelCreateProbe struct {
	Type   *string `json:"type"`
	Action *string `json:"action"`
}

func canonicalSaaSPath(path string) bool {
	if path == "/" || path == "/auth" || path == "/login" || path == "/logout" || path == "/react" || path == "/mfa/setup" || path == "/mfa/challenge" || path == "/mfa/confirm" || path == "/api/public/branding" {
		return true
	}
	return strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/react/") || strings.HasPrefix(path, "/api/react/") || strings.HasPrefix(path, "/api/v1/")
}

func (s *Server) SaaSGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSaaSSecurityHeaders(w, r)
		if !canonicalSaaSPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}

		var auditWriter *auditResponseWriter
		if shouldAuditPlatformMutation(r) {
			auditWriter = &auditResponseWriter{ResponseWriter: w}
			w = auditWriter
			defer func() {
				status := auditWriter.status
				if status == 0 {
					status = http.StatusOK
				}
				s.auditPlatformMutation(r, status)
			}()
		}

		if unsafeHTTPMethod(r.Method) && hasAuthenticatedPanelCookie(r) && !strictSameOriginRequest(r) {
			if strings.HasPrefix(r.URL.Path, "/api/react/") {
				writeReactError(w, http.StatusForbidden, "origin_forbidden", "origem não permitida")
			} else {
				http.Error(w, "origem não permitida", http.StatusForbidden)
			}
			return
		}

		var mustChangePassword, mfaPending bool
		r, mustChangePassword, mfaPending = s.bindAuthenticatedSession(w, r)

		if r.Method == http.MethodGet && r.URL.Path == "/mfa/setup" {
			s.handleMFASetupPage(w, r)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/mfa/challenge" {
			s.handleMFAChallengePage(w, r)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/mfa/confirm" {
			s.handleMFAConfirm(w, r)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/api/react/mfa" {
			s.handleMFAStatus(w, r)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/api/react/mfa/setup" {
			s.handleMFASetupAPI(w, r)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/react/mfa/confirm" {
			s.handleMFAConfirm(w, r)
			return
		}
		if mfaPending {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeReactError(w, http.StatusPreconditionRequired, "mfa_required", "conclua a autenticação em duas etapas")
			} else {
				http.Redirect(w, r, "/mfa/challenge", http.StatusFound)
			}
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/react/mfa/disable" {
			s.handleMFADisable(w, r)
			return
		}

		if r.Method == http.MethodGet && r.URL.Path == "/api/react/session" {
			s.authJSON(http.HandlerFunc(s.handleSaaSSession)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/api/react/bootstrap" {
			s.authJSON(http.HandlerFunc(s.handleSaaSBootstrap)).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/react/password/change" {
			s.authJSON(http.HandlerFunc(s.handleSaaSPasswordChange)).ServeHTTP(w, r)
			return
		}
		if mustChangePassword && strings.HasPrefix(r.URL.Path, "/api/react/") {
			writeReactError(w, http.StatusPreconditionRequired, "password_change_required", "troque a senha temporária antes de continuar")
			return
		}
		if !s.preflightSaaSResourceLimits(w, r) {
			return
		}

		if r.URL.Path == "/api/react/platform/tenants" && r.Method == http.MethodGet {
			s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleSaaSPlatformTenants))).ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/react/platform/tenants" && r.Method == http.MethodPost {
			s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleSaaSTenantCreate))).ServeHTTP(w, r)
			return
		}
		if tenantID, ok := saasPlatformTenantID(r.URL.Path); ok {
			switch r.Method {
			case http.MethodGet:
				r.SetPathValue("id", strconv.Itoa(tenantID))
				s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleSaaSTenantGet))).ServeHTTP(w, r)
				return
			case http.MethodPatch:
				r.SetPathValue("id", strconv.Itoa(tenantID))
				s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleSaaSTenantUpdate))).ServeHTTP(w, r)
				return
			}
		}
		if tenantID, ok := saasPlatformTenantUsageID(r.URL.Path); ok && r.Method == http.MethodGet {
			r.SetPathValue("id", strconv.Itoa(tenantID))
			s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleSaaSTenantUsage))).ServeHTTP(w, r)
			return
		}
		if _, ok := saasPlatformTenantChannelLimitsID(r.URL.Path); ok && (r.Method == http.MethodGet || r.Method == http.MethodPut) {
			s.authJSON(s.requireSuperAdmin(http.HandlerFunc(s.handleSaaSPlatformChannelLimits))).ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/react/channels" {
			s.authJSON(s.requireReactPerm(permCanaisGerenciar, s.requireReactModule("canais", http.HandlerFunc(s.handleSaaSChannelCreate)))).ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) bindAuthenticatedSession(w http.ResponseWriter, r *http.Request) (*http.Request, bool, bool) {
	if s.sessions == nil {
		return r, false, false
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return r, false, false
	}
	identity, ok := s.sessions.Identity(r.Context(), cookie.Value)
	if !ok {
		return r, false, false
	}
	state, err := s.st.GetSessionUserStateByID(r.Context(), identity.UserID)
	if err != nil || !state.Active || state.TenantID != identity.TenantID || state.Role != identity.Role {
		s.invalidateSession(w, r, cookie.Value)
		return r, false, false
	}
	if state.Role != "super_admin" && state.TenantStatus != "active" && state.TenantStatus != "trial" {
		s.invalidateSession(w, r, cookie.Value)
		return r, false, false
	}
	startedAt, ok := s.sessions.StartedAt(r.Context(), cookie.Value)
	if !ok || startedAt.IsZero() || state.CredentialsChangedAt.After(startedAt) {
		s.invalidateSession(w, r, cookie.Value)
		return r, false, false
	}
	status := state.TenantStatus
	if state.Role == "super_admin" {
		status = "active"
	}
	ctx := store.WithResolvedTenant(r.Context(), state.TenantID, status)
	mfaPending := !identity.MFAVerified && (state.MFAEnabled || state.Role == "super_admin")
	return r.WithContext(ctx), state.MustChangePassword && state.Role != "super_admin", mfaPending
}

func (s *Server) invalidateSession(w http.ResponseWriter, r *http.Request, token string) {
	if s.sessions != nil {
		s.sessions.Delete(r.Context(), token)
	}
	setSessionCookie(w, r, "", -1)
}

func decodeSAASTenant(r *http.Request) (saasTenantCreateInput, error) {
	var input saasTenantCreateInput
	decoder := json.NewDecoder(io.LimitReader(r.Body, 96*1024+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return input, err
	}
	return input, nil
}

func (input saasTenantCreateInput) platformInput() store.PlatformTenantInput {
	return platformTenantInputFromReact(reactPlatformTenantInput{
		Name: input.Name, Slug: input.Slug, Plan: input.Plan, Status: input.Status,
		Timezone: input.Timezone, Locale: input.Locale, Dominio: input.Dominio, Creditos: input.Creditos,
	})
}

func (limits *saasTenantLimits) storeLimits() *store.PlatformTenantLimits {
	if limits == nil {
		return nil
	}
	return &store.PlatformTenantLimits{
		MaxCanais: limits.MaxCanais, MaxAtendentes: limits.MaxAtendentes,
		MaxCampanhasAtivas: limits.MaxCampanhasAtivas, CampanhaLimiteDiario: limits.CampanhaLimiteDiario,
		MaxAgentes: limits.MaxAgentes, MaxUsuarios: limits.MaxUsuarios, MaxTools: limits.MaxTools,
		MaxAPIKeys: limits.MaxAPIKeys, MaxWebhooks: limits.MaxWebhooks, CreditosIAMensais: limits.CreditosIAMensais,
	}
}

func saasLimitsFromStore(limits store.PlatformTenantLimits) saasTenantLimits {
	return saasTenantLimits{
		MaxCanais: limits.MaxCanais, MaxAtendentes: limits.MaxAtendentes,
		MaxCampanhasAtivas: limits.MaxCampanhasAtivas, CampanhaLimiteDiario: limits.CampanhaLimiteDiario,
		MaxAgentes: limits.MaxAgentes, MaxUsuarios: limits.MaxUsuarios, MaxTools: limits.MaxTools,
		MaxAPIKeys: limits.MaxAPIKeys, MaxWebhooks: limits.MaxWebhooks, CreditosIAMensais: limits.CreditosIAMensais,
	}
}

func (s *Server) saasTenantResponse(r *http.Request, tenant store.PlatformTenant) (saasTenantResponse, error) {
	modules, err := platformTenantModules(r, s.st, tenant.ID)
	if err != nil {
		return saasTenantResponse{}, err
	}
	channelLimits, err := s.st.GetPlatformChannelTypeLimits(r.Context(), tenant.ID)
	if err != nil {
		return saasTenantResponse{}, err
	}
	return saasTenantResponse{
		ID: tenant.ID, Slug: tenant.Slug, Name: tenant.Name, Status: tenant.Status, Plan: tenant.Plan,
		Timezone: tenant.Timezone, Locale: tenant.Locale, Dominio: tenant.Dominio, Creditos: tenant.Creditos,
		Limits: saasLimitsFromStore(tenant.Limits), ChannelLimits: channelLimits, Usage: tenant.Usage,
		Modules: modules, CreatedAt: tenant.CreatedAt, UpdatedAt: tenant.UpdatedAt,
	}, nil
}

func (s *Server) handleSaaSPlatformTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := s.st.ListPlatformTenants(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as empresas")
		return
	}
	out := make([]saasTenantResponse, 0, len(tenants))
	for _, tenant := range tenants {
		item, err := s.saasTenantResponse(r, tenant)
		if err != nil {
			writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar a empresa")
			return
		}
		out = append(out, item)
	}
	writeReactResponse(w, http.StatusOK, out, map[string]interface{}{"scope": "platform"})
}

func (s *Server) handleSaaSTenantCreate(w http.ResponseWriter, r *http.Request) {
	input, err := decodeSAASTenant(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido ou contém campo não suportado")
		return
	}
	var initialAdmin *store.PlatformInitialAdminInput
	if input.Admin != nil {
		input.Admin.Name = strings.TrimSpace(input.Admin.Name)
		input.Admin.Username = strings.TrimSpace(input.Admin.Username)
		input.Admin.TemporaryPassword = strings.TrimSpace(input.Admin.TemporaryPassword)
		if err := validateReactPlatformAdmin(*input.Admin); err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_admin", err.Error())
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Admin.TemporaryPassword), bcrypt.DefaultCost)
		if err != nil {
			writeReactError(w, http.StatusInternalServerError, "password_failed", "não foi possível proteger a senha temporária")
			return
		}
		initialAdmin = &store.PlatformInitialAdminInput{
			Name: input.Admin.Name, Username: input.Admin.Username,
			PasswordHash: string(hash), Permissions: platformAdminPermissions(),
		}
	}

	tenant, err := s.st.CreatePlatformTenantProvisioned(r.Context(), input.platformInput(), input.Limits.storeLimits(), input.ChannelLimits, input.Modules, initialAdmin)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_tenant", "não foi possível criar a empresa; nenhuma alteração parcial foi mantida")
		return
	}
	out, err := s.saasTenantResponse(r, tenant)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "empresa criada, mas não foi possível carregá-la")
		return
	}
	writeReactResponse(w, http.StatusCreated, out, nil)
}

func saasPlatformTenantID(path string) (int, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[0] != "api" || parts[1] != "react" || parts[2] != "platform" || parts[3] != "tenants" {
		return 0, false
	}
	id, err := strconv.Atoi(parts[4])
	return id, err == nil && id > 0
}

func saasPlatformTenantUsageID(path string) (int, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 || parts[0] != "api" || parts[1] != "react" || parts[2] != "platform" || parts[3] != "tenants" || parts[5] != "usage" {
		return 0, false
	}
	id, err := strconv.Atoi(parts[4])
	return id, err == nil && id > 0
}

func (s *Server) handleSaaSTenantGet(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := saasPlatformTenantID(r.URL.Path)
	tenant, err := s.st.GetPlatformTenant(r.Context(), tenantID)
	if err != nil {
		writeReactError(w, http.StatusNotFound, "not_found", "empresa não encontrada")
		return
	}
	out, err := s.saasTenantResponse(r, tenant)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar a empresa")
		return
	}
	writeReactResponse(w, http.StatusOK, out, nil)
}

func (s *Server) handleSaaSTenantUpdate(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := saasPlatformTenantID(r.URL.Path)
	input, err := decodeSAASTenant(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido ou contém campo não suportado")
		return
	}
	if input.Admin != nil {
		writeReactError(w, http.StatusBadRequest, "admin_not_updatable_here", "admin inicial só pode ser informado na criação da empresa")
		return
	}
	tenant, err := s.st.UpdatePlatformTenantProvisioned(r.Context(), tenantID, input.platformInput(), input.Limits.storeLimits(), input.ChannelLimits, input.Modules)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrPlatformTenantNotFound) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "tenant_update_failed", "nenhuma alteração parcial foi mantida")
		return
	}
	out, err := s.saasTenantResponse(r, tenant)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "empresa atualizada, mas não foi possível carregá-la")
		return
	}
	writeReactResponse(w, http.StatusOK, out, nil)
}

func (s *Server) handleSaaSTenantUsage(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := saasPlatformTenantUsageID(r.URL.Path)
	tenant, err := s.st.GetPlatformTenant(r.Context(), tenantID)
	if err != nil {
		writeReactError(w, http.StatusNotFound, "not_found", "empresa não encontrada")
		return
	}
	channelLimits, err := s.st.GetPlatformChannelTypeLimits(r.Context(), tenantID)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as quotas de canais")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{
		"usage": tenant.Usage, "limits": saasLimitsFromStore(tenant.Limits), "channel_limits": channelLimits,
	}, nil)
}

func saasPlatformTenantChannelLimitsID(path string) (int, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 || parts[0] != "api" || parts[1] != "react" || parts[2] != "platform" || parts[3] != "tenants" || parts[5] != "channel-limits" {
		return 0, false
	}
	id, err := strconv.Atoi(parts[4])
	return id, err == nil && id > 0
}

func (s *Server) handleSaaSPlatformChannelLimits(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := saasPlatformTenantChannelLimitsID(r.URL.Path)
	if !ok {
		writeReactError(w, http.StatusBadRequest, "invalid_tenant", "empresa inválida")
		return
	}
	if r.Method == http.MethodGet {
		limits, err := s.st.GetPlatformChannelTypeLimits(r.Context(), tenantID)
		if err != nil {
			writeReactError(w, http.StatusNotFound, "not_found", "empresa não encontrada")
			return
		}
		writeReactResponse(w, http.StatusOK, limits, map[string]interface{}{"tenant_id": tenantID})
		return
	}
	var limits store.PlatformChannelTypeLimits
	decoder := json.NewDecoder(io.LimitReader(r.Body, 16*1024+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&limits); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	if err := s.st.UpdatePlatformChannelTypeLimits(r.Context(), tenantID, limits); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_limits", "limites de canais inválidos")
		return
	}
	writeReactResponse(w, http.StatusOK, limits, map[string]interface{}{"tenant_id": tenantID})
}

func (s *Server) handleSaaSChannelCreate(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 96*1024+1))
	if err != nil || len(body) > 96*1024 {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var probe saasChannelCreateProbe
	if err := json.Unmarshal(body, &probe); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	if probe.Action == nil || strings.TrimSpace(*probe.Action) == "" {
		channelType := ""
		if probe.Type != nil {
			channelType = strings.ToLower(strings.TrimSpace(*probe.Type))
		}
		if validateReactChannelType(channelType) {
			if err := s.st.EnsureCanCreateChannelType(r.Context(), channelType); err != nil {
				var limitErr *store.TenantChannelTypeLimitError
				if errors.As(err, &limitErr) {
					writeReactError(w, http.StatusConflict, "channel_type_limit_reached", fmt.Sprintf("limite de canais %s atingido", channelType))
					return
				}
				writeReactError(w, http.StatusInternalServerError, "plan_check_failed", "não foi possível validar o limite de canais do plano")
				return
			}
		}
	}
	s.handleReactChannelCreate(w, r)
}
