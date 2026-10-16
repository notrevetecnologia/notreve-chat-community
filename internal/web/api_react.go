package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type reactError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type reactEnvelope struct {
	Data  interface{} `json:"data,omitempty"`
	Meta  interface{} `json:"meta,omitempty"`
	Error *reactError `json:"error,omitempty"`
}

type reactUser struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	AtendenteID *int   `json:"atendente_id,omitempty"`
	Active      bool   `json:"active"`
}

type reactTransferTarget struct {
	ID          int    `json:"id"`
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type reactMessage struct {
	ID             int       `json:"id"`
	ConversationID int       `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	Source         string    `json:"source"`
	WAMsgID        string    `json:"wa_msg_id,omitempty"`
	MediaType      string    `json:"media_type,omitempty"`
	MediaMime      string    `json:"media_mime,omitempty"`
	MediaName      string    `json:"media_name,omitempty"`
	MediaURL       string    `json:"media_url,omitempty"`
	HasMedia       bool      `json:"has_media"`
	Lat            float64   `json:"lat,omitempty"`
	Lng            float64   `json:"lng,omitempty"`
	ContactName    string    `json:"contact_name,omitempty"`
	ContactTel     string    `json:"contact_tel,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type reactActionInput struct {
	Action      string `json:"action"`
	Motivo      string `json:"motivo"`
	Silencioso  bool   `json:"silencioso"`
	SetorID     int    `json:"setor_id"`
	AtendenteID int    `json:"atendente_id"`
	Typing      bool   `json:"typing"`
	MsgID       int    `json:"msg_id"`
	Emoji       string `json:"emoji"`
}

type reactBranding struct {
	Name            string `json:"name"`
	LogoURL         string `json:"logo_url"`
	FaviconURL      string `json:"favicon_url"`
	PrimaryColor    string `json:"primary_color"`
	SecondaryColor  string `json:"secondary_color"`
	AccentColor     string `json:"accent_color"`
	BackgroundColor string `json:"background_color"`
	LoadingColor    string `json:"loading_color"`
}

type reactBrandingInput struct {
	Name            *string `json:"name"`
	LogoURL         *string `json:"logo_url"`
	FaviconURL      *string `json:"favicon_url"`
	PrimaryColor    *string `json:"primary_color"`
	SecondaryColor  *string `json:"secondary_color"`
	AccentColor     *string `json:"accent_color"`
	BackgroundColor *string `json:"background_color"`
	LoadingColor    *string `json:"loading_color"`
}

var reactHexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

var reactBrandingDefaults = reactBranding{
	Name: "Notreve Chat Community", LogoURL: "", FaviconURL: "",
	PrimaryColor: "#22c55e", SecondaryColor: "#16a34a", AccentColor: "#4f46e5",
	BackgroundColor: "#f4f6f8", LoadingColor: "#22c55e",
}

func reactBrandingFromSettings(st store.Settings) reactBranding {
	return reactBranding{
		Name: st.BrandName, LogoURL: st.LogoURL, FaviconURL: st.FaviconURL,
		PrimaryColor: st.PrimaryColor, SecondaryColor: st.SecondaryColor, AccentColor: st.AccentColor,
		BackgroundColor: st.BackgroundColor, LoadingColor: st.LoadingColor,
	}
}

func safeBrandName(value string) string {
	value = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	if value == "" {
		return reactBrandingDefaults.Name
	}
	runes := []rune(value)
	if len(runes) > 120 {
		return string(runes[:120])
	}
	return value
}

func safeBrandURL(value string) string {
	value = strings.TrimSpace(value)
	if !validReactBrandingURL(value) {
		return ""
	}
	return value
}

func safeBrandColor(value, fallback string) string {
	value = strings.TrimSpace(value)
	if !reactHexColorPattern.MatchString(value) {
		return fallback
	}
	return value
}

func reactBrandingWithFallback(st store.Settings) reactBranding {
	return reactBranding{
		Name:            safeBrandName(st.BrandName),
		LogoURL:         safeBrandURL(st.LogoURL),
		FaviconURL:      safeBrandURL(st.FaviconURL),
		PrimaryColor:    safeBrandColor(st.PrimaryColor, reactBrandingDefaults.PrimaryColor),
		SecondaryColor:  safeBrandColor(st.SecondaryColor, reactBrandingDefaults.SecondaryColor),
		AccentColor:     safeBrandColor(st.AccentColor, reactBrandingDefaults.AccentColor),
		BackgroundColor: safeBrandColor(st.BackgroundColor, reactBrandingDefaults.BackgroundColor),
		LoadingColor:    safeBrandColor(st.LoadingColor, reactBrandingDefaults.LoadingColor),
	}
}

// reactBrandingFromPlatform monta a identidade visual GLOBAL da plataforma
// (controlada pelo super_admin), para o painel/login. Fallback seguro para a
// marca padrão do produto.
func reactBrandingFromPlatform(b store.PlatformBranding) reactBranding {
	return reactBranding{
		Name:            safeBrandName(b.Name),
		LogoURL:         safeBrandURL(b.LogoURL),
		FaviconURL:      safeBrandURL(b.FaviconURL),
		PrimaryColor:    safeBrandColor(b.PrimaryColor, reactBrandingDefaults.PrimaryColor),
		SecondaryColor:  safeBrandColor(b.SecondaryColor, reactBrandingDefaults.SecondaryColor),
		AccentColor:     safeBrandColor(b.AccentColor, reactBrandingDefaults.AccentColor),
		BackgroundColor: safeBrandColor(b.BackgroundColor, reactBrandingDefaults.BackgroundColor),
		LoadingColor:    safeBrandColor(b.LoadingColor, reactBrandingDefaults.LoadingColor),
	}
}

func (s *Server) platformBranding(ctx context.Context) reactBranding {
	if b, err := s.st.GetPlatformBranding(ctx); err == nil {
		return reactBrandingFromPlatform(b)
	}
	return reactBrandingDefaults
}

var reactBrandAssetExtensions = map[string]bool{
	"png": true, "jpg": true, "jpeg": true, "webp": true, "gif": true,
	"svg": true, "ico": true, "avif": true, "bmp": true,
}

func validReactBrandingURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	u, err := url.ParseRequestURI(value)
	if err != nil || u.Host == "" || u.User != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(u.Path), "."))
	return reactBrandAssetExtensions[ext]
}

func validateReactBranding(branding reactBranding) error {
	if strings.TrimSpace(branding.Name) == "" || len([]rune(branding.Name)) > 120 {
		return errors.New("nome deve ter entre 1 e 120 caracteres")
	}
	if !validReactBrandingURL(branding.LogoURL) || !validReactBrandingURL(branding.FaviconURL) {
		return errors.New("logo_url e favicon_url devem ser URLs HTTP ou HTTPS válidas")
	}
	for _, color := range []string{branding.PrimaryColor, branding.SecondaryColor, branding.AccentColor, branding.BackgroundColor, branding.LoadingColor} {
		if !reactHexColorPattern.MatchString(color) {
			return errors.New("as cores devem usar o formato hexadecimal #RRGGBB")
		}
	}
	return nil
}

func applyReactBrandingInput(branding reactBranding, input reactBrandingInput) reactBranding {
	if input.Name != nil {
		branding.Name = strings.TrimSpace(*input.Name)
	}
	if input.LogoURL != nil {
		branding.LogoURL = strings.TrimSpace(*input.LogoURL)
	}
	if input.FaviconURL != nil {
		branding.FaviconURL = strings.TrimSpace(*input.FaviconURL)
	}
	if input.PrimaryColor != nil {
		branding.PrimaryColor = strings.TrimSpace(*input.PrimaryColor)
	}
	if input.SecondaryColor != nil {
		branding.SecondaryColor = strings.TrimSpace(*input.SecondaryColor)
	}
	if input.AccentColor != nil {
		branding.AccentColor = strings.TrimSpace(*input.AccentColor)
	}
	if input.BackgroundColor != nil {
		branding.BackgroundColor = strings.TrimSpace(*input.BackgroundColor)
	}
	if input.LoadingColor != nil {
		branding.LoadingColor = strings.TrimSpace(*input.LoadingColor)
	}
	return branding
}

func (s *Server) handleReactApp(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/react" || r.URL.Path == "/react/" {
		target := "/"
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
		return
	}
	index := filepath.Join(s.webroot, "react", "dist", "index.html")
	if _, err := os.Stat(index); err != nil {
		http.Error(w, "frontend React indisponível", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, index)
}

func writeReactResponse(w http.ResponseWriter, status int, data, meta interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(reactEnvelope{Data: data, Meta: meta})
}

func writeReactError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(reactEnvelope{Error: &reactError{Code: code, Message: message}})
}

func (s *Server) authJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete {
			if !sameOriginRequest(r) {
				writeReactError(w, http.StatusForbidden, "origin_forbidden", "origem não permitida")
				return
			}
		}
		if s.sessions == nil {
			writeReactError(w, http.StatusUnauthorized, "unauthorized", "não autenticado")
			return
		}
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			writeReactError(w, http.StatusUnauthorized, "unauthorized", "não autenticado")
			return
		}
		uid, ok := s.sessions.Get(r.Context(), c.Value)
		if !ok {
			writeReactError(w, http.StatusUnauthorized, "unauthorized", "sessão expirada")
			return
		}
		_ = s.sessions.Touch(r.Context(), c.Value)
		tenantID, tenantStatus, tenantErr := s.st.ResolveTenantForHost(r.Context(), r.Host)
		if tenantErr != nil || (tenantStatus != "active" && tenantStatus != "trial") {
			writeReactError(w, http.StatusUnauthorized, "tenant_unavailable", "empresa não encontrada ou indisponível")
			return
		}
		tenantCtx := store.WithTenantID(r.Context(), tenantID)
		u, err := s.st.GetUsuarioByID(tenantCtx, uid)
		if err != nil || !u.Active {
			writeReactError(w, http.StatusUnauthorized, "unauthorized", "usuário não autenticado")
			return
		}
		// O super_admin é o dono da plataforma, não um operador de uma
		// empresa. Mesmo tendo todas as permissões, ele não deve consultar
		// conversas, contatos ou configurações operacionais de um tenant por
		// acidente. O control plane tem uma superfície própria e explícita.
		if u.Role == "super_admin" && !superAdminReactPathAllowed(r.URL.Path) {
			writeReactError(w, http.StatusForbidden, "platform_only", "o super_admin usa somente o control plane da plataforma")
			return
		}
		if _, err := r.Cookie(csrfCookie); err != nil {
			setCSRFCookie(w, r, newSessionToken())
		}
		ctx := contextWithUser(r.WithContext(tenantCtx), &u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func superAdminReactPathAllowed(path string) bool {
	if strings.HasPrefix(path, "/api/react/platform/") {
		return true
	}
	switch path {
	case "/api/react/session", "/api/react/bootstrap", "/api/react/platform", "/api/react/branding":
		return true
	default:
		return false
	}
}

func contextWithUser(r *http.Request, u *store.Usuario) context.Context {
	ctx := context.WithValue(r.Context(), ctxUser, u)
	return store.WithTenantID(ctx, u.TenantID)
}

func (s *Server) requireReactModule(module string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.reactModuleEnabled(r.Context(), module) {
			writeReactError(w, http.StatusForbidden, "module_disabled", "módulo desativado")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) reactModuleEnabled(ctx context.Context, module string) bool {
	enabled, err := s.st.ModuleEnabled(ctx, module)
	return err == nil && enabled
}

func (s *Server) requireReactPerm(permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !userHasPerm(currentUser(r), permission) {
			writeReactError(w, http.StatusForbidden, "permission_denied", "sem permissão para acessar")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func effectiveReactPermissions(u *store.Usuario) []string {
	if u == nil {
		return []string{}
	}
	set := userPermSet(u)
	if u.Role == "super_admin" {
		set = map[string]bool{}
		for _, permission := range allPermsList() {
			set[permission] = true
		}
	}
	permissions := make([]string, 0, len(set))
	for permission := range set {
		permissions = append(permissions, permission)
	}
	sort.Strings(permissions)
	return permissions
}

func reactUserFromStore(u *store.Usuario) reactUser {
	if u == nil {
		return reactUser{}
	}
	return reactUser{
		ID: u.ID, Name: u.Name, Username: u.Username, Role: u.Role,
		AtendenteID: u.AtendenteID, Active: u.Active,
	}
}

func reactTransferTargetFromSector(sector store.Setor) reactTransferTarget {
	return reactTransferTarget{
		ID: sector.ID, Kind: "sector", Label: sector.Name, Description: "Setor de atendimento",
	}
}

func reactTransferTargetFromUser(user store.Usuario) reactTransferTarget {
	role := map[string]string{
		"super_admin": "Administrador da plataforma",
		"admin":       "Administrador",
		"supervisor":  "Supervisor",
		"atendente":   "Atendente",
	}[user.Role]
	if role == "" {
		role = "Usuário"
	}
	return reactTransferTarget{ID: user.ID, Kind: "agent", Label: user.Name, Description: role}
}

func reactTransferTargetUserAllowed(user store.Usuario) bool {
	return user.Active && strings.TrimSpace(user.Name) != "" && (user.Role == "atendente" || user.Role == "supervisor" || user.Role == "admin")
}

func (s *Server) reactTransferTargets(ctx context.Context, user *store.Usuario) ([]reactTransferTarget, error) {
	sectors, err := s.st.ListSetores(ctx)
	if err != nil {
		return nil, err
	}
	users, err := s.st.ListUsuarios(ctx)
	if err != nil {
		return nil, err
	}
	targets := make([]reactTransferTarget, 0, len(sectors)+len(users))
	allowedSectors := map[int]bool{}
	if user != nil && user.Role == "supervisor" {
		for _, id := range s.userSetores(ctx, user) {
			allowedSectors[id] = true
		}
	}
	for _, sector := range sectors {
		if sector.Active && strings.TrimSpace(sector.Name) != "" && (user == nil || user.Role != "supervisor" || allowedSectors[sector.ID]) {
			targets = append(targets, reactTransferTargetFromSector(sector))
		}
	}
	for _, target := range users {
		if reactTransferTargetUserAllowed(target) && s.canSeeTransferTarget(ctx, user, target) {
			targets = append(targets, reactTransferTargetFromUser(target))
		}
	}
	return targets, nil
}

func (s *Server) reactSessionData(r *http.Request) (map[string]interface{}, error) {
	u := currentUser(r)
	modules, err := s.st.ListModules(r.Context())
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"user":        reactUserFromStore(u),
		"permissions": effectiveReactPermissions(u),
		"modules":     modules,
		"branding":    s.platformBranding(r.Context()),
		"api_version": "1",
	}, nil
}

func (s *Server) handleReactSession(w http.ResponseWriter, r *http.Request) {
	data, err := s.reactSessionData(r)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar a sessão")
		return
	}
	writeReactResponse(w, http.StatusOK, data, nil)
}

// handleReactPublicBranding expõe a identidade visual GLOBAL da plataforma
// (controlada pelo super_admin) para a tela de login. A marca da empresa
// aparece apenas nas superfícies do cliente.
func (s *Server) handleReactPublicBranding(w http.ResponseWriter, r *http.Request) {
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"branding": s.platformBranding(r.Context())}, nil)
}

func (s *Server) handleReactBootstrap(w http.ResponseWriter, r *http.Request) {
	data, err := s.reactSessionData(r)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar o bootstrap")
		return
	}
	data["conversation_statuses"] = []string{"ia", "humano_aguardando", "humano_atendendo", "closed"}
	data["pagination"] = map[string]interface{}{"default_limit": 25, "max_limit": 100, "cursor": "offset"}
	data["conversation_capabilities"] = map[string]interface{}{
		"reply":     true,
		"media":     map[string]interface{}{"upload": "/api/react/conversations/{id}/messages", "download": "/api/mensagens/{id}/media", "max_bytes": 50 << 20},
		"audio":     true,
		"typing":    "/api/react/conversations/{id}/actions",
		"reactions": "/api/react/conversations/{id}/actions",
		"edit":      "/api/conversas/{id}/editar",
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

func canEditReactBranding(u *store.Usuario) bool {
	return u != nil && u.Role == "super_admin"
}

// handleReactBrandingGet devolve a identidade visual do tenant atual. A marca
// da empresa é configurada no canal Webchat; este endpoint permanece restrito
// ao super_admin para o fallback legado do widget.
func (s *Server) handleReactBrandingGet(w http.ResponseWriter, r *http.Request) {
	if !canEditReactBranding(currentUser(r)) {
		writeReactError(w, http.StatusForbidden, "permission_denied", "somente o super_admin pode configurar a marca")
		return
	}
	settings, err := s.st.GetSettings(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar o branding")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{
		"branding": reactBrandingWithFallback(settings),
		"can_edit": true,
	}, nil)
}

func (s *Server) handleReactBranding(w http.ResponseWriter, r *http.Request) {
	if !canEditReactBranding(currentUser(r)) {
		writeReactError(w, http.StatusForbidden, "permission_denied", "somente o super_admin pode alterar a marca")
		return
	}
	settings, err := s.st.GetSettings(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar o branding")
		return
	}
	var input reactBrandingInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	branding := applyReactBrandingInput(reactBrandingFromSettings(settings), input)
	if err := validateReactBranding(branding); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_branding", err.Error())
		return
	}
	settings.BrandName = branding.Name
	settings.LogoURL = branding.LogoURL
	settings.FaviconURL = branding.FaviconURL
	settings.PrimaryColor = branding.PrimaryColor
	settings.SecondaryColor = branding.SecondaryColor
	settings.AccentColor = branding.AccentColor
	settings.BackgroundColor = branding.BackgroundColor
	settings.LoadingColor = branding.LoadingColor
	if err := s.st.UpdateSettings(r.Context(), settings); err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível salvar o branding")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"branding": branding}, nil)
}

// reactRequireSuperAdmin valida que o operador é o dono da plataforma.
func reactRequireSuperAdmin(s *Server, w http.ResponseWriter, r *http.Request) bool {
	user := currentUser(r)
	if user == nil || user.Role != "super_admin" {
		writeReactError(w, http.StatusForbidden, "super_admin_required", "ação restrita ao super_admin")
		return false
	}
	return true
}

// handleReactPlatformBrandingGet devolve a marca GLOBAL da plataforma.
// Somente o super_admin controla; aparece no painel/login.
func (s *Server) handleReactPlatformBrandingGet(w http.ResponseWriter, r *http.Request) {
	if !reactRequireSuperAdmin(s, w, r) {
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"branding": s.platformBranding(r.Context()), "can_edit": true}, nil)
}

func (s *Server) handleReactPlatformBranding(w http.ResponseWriter, r *http.Request) {
	if !reactRequireSuperAdmin(s, w, r) {
		return
	}
	var input reactBrandingInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	current, err := s.st.GetPlatformBranding(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar a marca da plataforma")
		return
	}
	apply := reactBranding{Name: current.Name, LogoURL: current.LogoURL, FaviconURL: current.FaviconURL, PrimaryColor: current.PrimaryColor, SecondaryColor: current.SecondaryColor, AccentColor: current.AccentColor, BackgroundColor: current.BackgroundColor, LoadingColor: current.LoadingColor}
	branding := applyReactBrandingInput(apply, input)
	if err := validateReactBranding(branding); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_branding", err.Error())
		return
	}
	next := store.PlatformBranding{Name: branding.Name, LogoURL: branding.LogoURL, FaviconURL: branding.FaviconURL, PrimaryColor: branding.PrimaryColor, SecondaryColor: branding.SecondaryColor, AccentColor: branding.AccentColor, BackgroundColor: branding.BackgroundColor, LoadingColor: branding.LoadingColor}
	if err := s.st.SetPlatformBranding(r.Context(), next); err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível salvar a marca da plataforma")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"branding": reactBrandingFromPlatform(next)}, nil)
}

func parseReactLimit(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 25, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > 100 {
		return 0, errors.New("limit deve estar entre 1 e 100")
	}
	return limit, nil
}

func encodeReactCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func decodeReactCursor(cursor string) (int, error) {
	if strings.TrimSpace(cursor) == "" {
		return 0, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, errors.New("cursor inválido")
	}
	offset, err := strconv.Atoi(string(b))
	if err != nil || offset < 0 {
		return 0, errors.New("cursor inválido")
	}
	return offset, nil
}

func validReactConversationStatus(status string) bool {
	switch status {
	case "", "all", "ia", "humano_aguardando", "humano_atendendo", "closed":
		return true
	default:
		return false
	}
}

func reactConversationMatches(c store.Conversation, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	values := []string{c.CustomerName, c.CustomerPhone, c.Channel, c.Origem, c.Hash, c.Status, c.AtendenteName, c.SetorName}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func (s *Server) handleReactConversations(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	status := strings.TrimSpace(query.Get("status"))
	if !validReactConversationStatus(status) {
		writeReactError(w, http.StatusBadRequest, "invalid_status", "status inválido")
		return
	}
	if status == "all" {
		status = ""
	}
	limit, err := parseReactLimit(query.Get("limit"))
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_limit", err.Error())
		return
	}
	offset, err := decodeReactCursor(query.Get("cursor"))
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	user := currentUser(r)
	conversations, err := s.st.ListConversationsForUser(r.Context(), status, user, s.userSetores(r.Context(), user), s.modoAberto(r.Context()), userHasPerm(user, permConversasVerIA))
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível listar as conversas")
		return
	}
	search := strings.TrimSpace(query.Get("q"))
	if search == "" {
		search = strings.TrimSpace(query.Get("search"))
	}
	filtered := make([]store.Conversation, 0, len(conversations))
	for _, conversation := range conversations {
		if reactConversationMatches(conversation, search) {
			filtered = append(filtered, conversation)
		}
	}
	if offset > len(filtered) {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", "cursor fora do resultado atual")
		return
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	items := make([]store.Conversation, end-offset)
	copy(items, filtered[offset:end])
	hasMore := end < len(filtered)
	nextCursor := ""
	if hasMore {
		nextCursor = encodeReactCursor(end)
	}
	meta := map[string]interface{}{
		"limit": limit, "total": len(filtered), "has_more": hasMore,
		"next_cursor": nextCursor, "cursor": query.Get("cursor"),
		"filters": map[string]string{"status": status, "search": search},
	}
	writeReactResponse(w, http.StatusOK, items, meta)
}

func parseReactConversationID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(strings.TrimSpace(r.PathValue("id")))
	if err != nil || id < 1 {
		return 0, errors.New("id inválido")
	}
	return id, nil
}

// handleReactBotBlock marca/desmarca uma conversa como "possível outro bot".
// Quando marcada, o processador ignora novas mensagens do cliente (sem gastar a
// fila humana nem a IA) até um operador desbloquear. Respeita o acesso do
// operador à conversa.
func (s *Server) handleReactBotBlock(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if _, status, message := s.reactConversationForUser(r, id); status != 0 {
		code := "not_found"
		if status >= 500 {
			code = "internal_error"
		}
		writeReactError(w, status, code, message)
		return
	}
	var input struct {
		Blocked bool `json:"blocked"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	if err := s.st.SetConversationBotBlocked(r.Context(), id, input.Blocked); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível alterar o bloqueio")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"blocked": input.Blocked}, nil)
}

func (s *Server) reactConversationForUser(r *http.Request, id int) (store.Conversation, int, string) {
	conversation, err := s.st.GetConversation(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Conversation{}, http.StatusNotFound, "conversa não encontrada"
		}
		return store.Conversation{}, http.StatusInternalServerError, "não foi possível carregar a conversa"
	}
	user := currentUser(r)
	if !s.canSee(user, conversation, s.userSetores(r.Context(), user), s.modoAberto(r.Context())) {
		return store.Conversation{}, http.StatusNotFound, "conversa não encontrada"
	}
	return conversation, 0, ""
}

func reactMessageFromStore(message store.Message) reactMessage {
	mediaURL := ""
	if message.MediaB64 != "" {
		mediaURL = "/api/mensagens/" + strconv.Itoa(message.ID) + "/media"
	}
	return reactMessage{
		ID: message.ID, ConversationID: message.ConversationID, Role: message.Role,
		Content: message.Content, Source: message.Source, WAMsgID: message.WAMsgID,
		MediaType: message.MediaType, MediaMime: message.MediaMime, MediaName: message.MediaName, MediaURL: mediaURL,
		HasMedia: message.MediaB64 != "", Lat: message.Lat, Lng: message.Lng,
		ContactName: message.ContactName, ContactTel: message.ContactTel, CreatedAt: message.CreatedAt,
	}
}

func (s *Server) handleReactConversation(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	conversation, status, message := s.reactConversationForUser(r, id)
	if status != 0 {
		code := "not_found"
		if status >= 500 {
			code = "internal_error"
		}
		writeReactError(w, status, code, message)
		return
	}
	messages, err := s.st.ListMessages(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as mensagens")
		return
	}
	items := make([]reactMessage, 0, len(messages))
	for _, item := range messages {
		items = append(items, reactMessageFromStore(item))
	}
	payload := map[string]interface{}{"conversation": conversation, "messages": items}
	if contact, contactErr := s.st.GetContatoByPhone(r.Context(), conversation.CustomerPhone); contactErr == nil {
		payload["contact"] = s.reactContactFromStore(r.Context(), contact)
	}
	if usage, err := s.st.ConversationLLMUsage(r.Context(), id); err == nil {
		payload["usage"] = usage
	}
	writeReactResponse(w, http.StatusOK, payload, nil)
}

func (s *Server) callReactLegacyHandler(r *http.Request, body []byte, handler http.HandlerFunc) (interface{}, int, string) {
	previousBody := r.Body
	r.Body = io.NopCloser(bytes.NewReader(body))
	defer func() { r.Body = previousBody }()
	return s.callReactLegacyRequest(r, handler)
}

func (s *Server) callReactLegacyRequest(r *http.Request, handler http.HandlerFunc) (interface{}, int, string) {
	recorder := httptest.NewRecorder()
	handler(recorder, r)
	status := recorder.Code
	if status == 0 {
		status = http.StatusOK
	}
	raw := strings.TrimSpace(recorder.Body.String())
	var payload interface{}
	if raw != "" && json.Unmarshal([]byte(raw), &payload) != nil {
		payload = map[string]interface{}{}
	}
	if status < 200 || status >= 300 {
		message := "requisição não processada"
		if object, ok := payload.(map[string]interface{}); ok {
			if value, ok := object["error"].(string); ok && strings.TrimSpace(value) != "" {
				message = value
			}
		}
		if message == "requisição não processada" && raw != "" && payload == nil {
			message = raw
		}
		return nil, status, message
	}
	return payload, status, ""
}

func (s *Server) handleReactSend(w http.ResponseWriter, r *http.Request) {
	if !userHasPerm(currentUser(r), permConversasResponder) {
		writeReactError(w, http.StatusForbidden, "permission_denied", "sem permissão para responder mensagens")
		return
	}
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	conversation, status, message := s.reactConversationForUser(r, id)
	if status != 0 {
		code := "not_found"
		if status >= 500 {
			code = "internal_error"
		}
		writeReactError(w, status, code, message)
		return
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type"))), "multipart/form-data") {
		result, status, message := s.callReactLegacyRequest(r, s.handleMedia)
		if status < 200 || status >= 300 {
			writeReactError(w, status, "media_send_failed", message)
			return
		}
		if updated, err := s.st.GetConversation(r.Context(), id); err == nil {
			conversation = updated
		}
		data := map[string]interface{}{"conversation": conversation}
		if object, ok := result.(map[string]interface{}); ok {
			for key, value := range object {
				data[key] = value
			}
		}
		writeReactResponse(w, http.StatusOK, data, nil)
		return
	}
	var input struct {
		Content string `json:"content"`
		ReplyTo *int   `json:"reply_to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	if strings.TrimSpace(input.Content) == "" {
		writeReactError(w, http.StatusBadRequest, "empty_content", "conteúdo vazio")
		return
	}
	body, _ := json.Marshal(input)
	result, status, message := s.callReactLegacyHandler(r, body, s.handleSend)
	if status < 200 || status >= 300 {
		writeReactError(w, status, "message_send_failed", message)
		return
	}
	updated, err := s.st.GetConversation(r.Context(), id)
	if err == nil {
		conversation = updated
	}
	data := map[string]interface{}{"conversation": conversation}
	if object, ok := result.(map[string]interface{}); ok {
		for key, value := range object {
			data[key] = value
		}
	}
	writeReactResponse(w, http.StatusOK, data, nil)
}

func (s *Server) handleReactAction(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	var input reactActionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	input.Action = strings.ToLower(strings.TrimSpace(input.Action))
	permission, valid := reactActionPermission(input.Action)
	if !valid {
		writeReactError(w, http.StatusBadRequest, "invalid_action", "ação inválida")
		return
	}
	var handler http.HandlerFunc
	var body []byte
	switch input.Action {
	case "reopen", "reabrir":
		handler = s.handleStatus
		body = []byte(`{"status":"humano_atendendo"}`)
	case "return_to_ai", "return-to-ai", "devolver_ia":
		handler = s.handleReturnToAI
		body = []byte(`{}`)
	case "close", "encerrar":
		handler = s.handleEncerrar
		body, _ = json.Marshal(map[string]interface{}{"motivo": input.Motivo, "silencioso": input.Silencioso})
	case "transfer", "transferir":
		if input.SetorID > 0 && input.AtendenteID > 0 {
			writeReactError(w, http.StatusBadRequest, "ambiguous_transfer", "informe apenas setor ou atendente")
			return
		}
		if input.SetorID < 1 && input.AtendenteID < 1 {
			writeReactError(w, http.StatusBadRequest, "missing_destination", "informe setor ou atendente")
			return
		}
		handler = s.handleConversationSetor
		body, _ = json.Marshal(map[string]int{"setor_id": input.SetorID, "atendente_id": input.AtendenteID})
	case "accept", "aceitar":
		handler = s.handleAccept
		body = []byte(`{}`)
	case "typing", "digitando":
		handler = s.handleTyping
		body, _ = json.Marshal(map[string]bool{"typing": input.Typing})
	case "reaction", "reagir":
		if input.MsgID < 1 {
			writeReactError(w, http.StatusBadRequest, "missing_message", "informe a mensagem da reação")
			return
		}
		input.Emoji = strings.TrimSpace(input.Emoji)
		if input.Emoji == "" || len([]rune(input.Emoji)) > 16 {
			writeReactError(w, http.StatusBadRequest, "invalid_emoji", "emoji inválido")
			return
		}
		handler = s.handleReaction
		body, _ = json.Marshal(map[string]interface{}{"msg_id": input.MsgID, "emoji": input.Emoji})
	}
	if !userHasPerm(currentUser(r), permission) {
		writeReactError(w, http.StatusForbidden, "permission_denied", "sem permissão para esta ação")
		return
	}
	conversation, status, message := s.reactConversationForUser(r, id)
	if status != 0 {
		code := "not_found"
		if status >= 500 {
			code = "internal_error"
		}
		writeReactError(w, status, code, message)
		return
	}
	if canonical := canonicalConversationAction(input.Action); canonical != "" {
		if !conversationActionAllowed(conversation.Status, canonical) {
			writeReactError(w, http.StatusConflict, "invalid_transition", conversationTransitionMessage(conversation.Status, canonical))
			return
		}
		if !s.canManageConversation(r.Context(), currentUser(r), conversation) {
			writeReactError(w, http.StatusForbidden, "conversation_not_assigned", "aceite o atendimento antes de agir")
			return
		}
	}
	result, status, message := s.callReactLegacyHandler(r, body, handler)
	if status < 200 || status >= 300 {
		writeReactError(w, status, "action_failed", message)
		return
	}
	updated, err := s.st.GetConversation(r.Context(), id)
	if err != nil {
		updated = conversation
	}
	data := map[string]interface{}{"action": input.Action, "conversation": updated}
	if result != nil {
		data["result"] = result
	}
	writeReactResponse(w, http.StatusOK, data, nil)
}

func reactActionPermission(action string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "accept", "aceitar":
		return permConversasResponder, true
	case "close", "encerrar", "reopen", "reabrir", "return_to_ai", "return-to-ai", "devolver_ia":
		return permConversasEncerrar, true
	case "transfer", "transferir":
		return permConversasTransferir, true
	case "typing", "digitando", "reaction", "reagir":
		return permConversasResponder, true
	default:
		return "", false
	}
}
