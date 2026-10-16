package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/llm"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"github.com/notrevetecnologia/notreve-chat-community/internal/tools"
)

type reactProviderView struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	ProviderType string   `json:"provider_type"`
	BaseURL      string   `json:"base_url"`
	Model        string   `json:"model"`
	Active       bool     `json:"active"`
	SafeMode     bool     `json:"safe_mode"`
	Capacidade   string   `json:"capacidade"`
	Capabilities []string `json:"capabilities"`
	HasAPIKey    bool     `json:"has_api_key"`
	CreatedAt    string   `json:"created_at"`
}

type reactProviderInput struct {
	Name         *string   `json:"name"`
	ProviderType *string   `json:"provider_type"`
	BaseURL      *string   `json:"base_url"`
	Model        *string   `json:"model"`
	APIKey       *string   `json:"api_key"`
	Active       *bool     `json:"active"`
	SafeMode     *bool     `json:"safe_mode"`
	Capacidade   *string   `json:"capacidade"`
	Capabilities *[]string `json:"capabilities"`
}

type reactIntegrationSummary struct {
	ID             int                    `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Method         string                 `json:"method"`
	Policy         reactIntegrationPolicy `json:"policy"`
	HasCredentials bool                   `json:"has_credentials"`
	CreatedAt      string                 `json:"created_at"`
}

type reactIntegrationPolicy struct {
	Connector             string   `json:"connector"`
	Risk                  string   `json:"risk"`
	RequiresConfirmation  bool     `json:"requires_confirmation"`
	RequiredScopes        []string `json:"required_scopes"`
	AllowedHosts          []string `json:"allowed_hosts"`
	TimeoutSeconds        int      `json:"timeout_seconds"`
	MaxCallsPerMinute     int      `json:"max_calls_per_minute"`
	MaxCallsPerHour       int      `json:"max_calls_per_hour"`
	MaxResponseBytes      int      `json:"max_response_bytes"`
	MaxArgumentBytes      int      `json:"max_argument_bytes"`
	RequiresIdentity      string   `json:"requires_identity"`
	AllowedIdentities     []string `json:"allowed_identities"`
	IdentityConfig        string   `json:"identity_config"`
	IdentityVerifyURL     string   `json:"identity_verify_url"`
	IdentityVerifyMethod  string   `json:"identity_verify_method"`
	IdentityVerifyParam   string   `json:"identity_verify_param"`
	IdentityMatchField    string   `json:"identity_match_field"`
	IdentityClientIDField string   `json:"identity_client_id_field"`
}

type reactIntegrationDetail struct {
	ID             int                    `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Method         string                 `json:"method"`
	URL            string                 `json:"url"`
	Headers        map[string]string      `json:"headers"`
	Parameters     string                 `json:"parameters"`
	Policy         reactIntegrationPolicy `json:"policy"`
	HasCredentials bool                   `json:"has_credentials"`
	CreatedAt      string                 `json:"created_at"`
}

type reactIntegrationInput struct {
	Name        *string                 `json:"name"`
	Description *string                 `json:"description"`
	Method      *string                 `json:"method"`
	URL         *string                 `json:"url"`
	Headers     *map[string]string      `json:"headers"`
	BearerToken *string                 `json:"bearer_token"`
	Parameters  *string                 `json:"parameters"`
	Policy      *reactIntegrationPolicy `json:"policy"`
}

var reactToolNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,63}$`)

func reactProviderFromStore(provider store.Provider) reactProviderView {
	baseURL := provider.BaseURL
	if provider.ProviderType == "notreve" {
		// A integração direta não expõe o endpoint interno do gateway para o
		// admin da empresa. O modelo escolhido é exibido para poder ser mantido.
		baseURL = ""
	}
	return reactProviderView{
		ID: provider.ID, Name: provider.Name, ProviderType: provider.ProviderType,
		BaseURL: baseURL, Model: provider.Model, Active: provider.Active,
		SafeMode: provider.SafeMode, Capacidade: provider.Capacidade,
		Capabilities: append([]string{}, provider.Capabilities...),
		HasAPIKey:    strings.TrimSpace(provider.APIKey) != "",
		CreatedAt:    provider.CreatedAt.Format(time.RFC3339),
	}
}

func reactIntegrationHasCredentials(headers string) bool {
	var values map[string]string
	if json.Unmarshal([]byte(headers), &values) != nil {
		return false
	}
	for key, value := range values {
		if sensitiveHeaderName(key) && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func reactIntegrationFromStore(integration store.Integration) reactIntegrationSummary {
	return reactIntegrationSummary{
		ID: integration.ID, Name: integration.Name, Description: integration.Description,
		Method: integration.Method, Policy: reactIntegrationPolicyFromStore(integration), HasCredentials: reactIntegrationHasCredentials(integration.Headers),
		CreatedAt: integration.CreatedAt.Format(time.RFC3339),
	}
}

func reactIntegrationPolicyFromStore(integration store.Integration) reactIntegrationPolicy {
	policy, _, err := tools.PolicyFromParameters(integration.Parameters, integration.Method)
	if err != nil {
		policy = tools.DefaultPolicy(integration.Method)
	}
	return reactIntegrationPolicyFromTools(policy, integration)
}

func reactIntegrationPolicyFromTools(policy tools.Policy, integration store.Integration) reactIntegrationPolicy {
	return reactIntegrationPolicy{
		Connector: policy.Connector, Risk: string(policy.Risk), RequiresConfirmation: policy.RequiresConfirmation,
		RequiredScopes: append([]string{}, policy.RequiredScopes...), AllowedHosts: append([]string{}, policy.AllowedHosts...),
		TimeoutSeconds: policy.TimeoutSeconds, MaxCallsPerMinute: policy.MaxCallsPerMinute, MaxCallsPerHour: policy.MaxCallsPerHour,
		MaxResponseBytes: policy.MaxResponseBytes, MaxArgumentBytes: policy.MaxArgumentBytes,
		RequiresIdentity: integration.RequiresIdentity, AllowedIdentities: append([]string{}, integration.AllowedIdentities...),
		IdentityConfig: integration.IdentityConfig, IdentityVerifyURL: integration.IdentityVerifyURL, IdentityVerifyMethod: integration.IdentityVerifyMethod,
		IdentityVerifyParam: integration.IdentityVerifyParam, IdentityMatchField: integration.IdentityMatchField, IdentityClientIDField: integration.IdentityClientIDField,
	}
}

func (policy reactIntegrationPolicy) toTools() tools.Policy {
	return tools.Policy{
		Connector: policy.Connector, Risk: tools.RiskLevel(policy.Risk), RequiresConfirmation: policy.RequiresConfirmation,
		RequiredScopes: append([]string{}, policy.RequiredScopes...), AllowedHosts: append([]string{}, policy.AllowedHosts...),
		TimeoutSeconds: policy.TimeoutSeconds, MaxCallsPerMinute: policy.MaxCallsPerMinute, MaxCallsPerHour: policy.MaxCallsPerHour,
		MaxResponseBytes: policy.MaxResponseBytes, MaxArgumentBytes: policy.MaxArgumentBytes,
	}
}

func reactIntegrationDetailFromStore(integration store.Integration) reactIntegrationDetail {
	publicHeaders := map[string]string{}
	var headers map[string]string
	if json.Unmarshal([]byte(integration.Headers), &headers) == nil {
		for key, value := range headers {
			if !sensitiveHeaderName(key) {
				publicHeaders[key] = value
			}
		}
	}
	parameters := integration.Parameters
	if _, schema, err := tools.PolicyFromParameters(integration.Parameters, integration.Method); err == nil {
		if encoded, marshalErr := json.Marshal(schema); marshalErr == nil {
			parameters = string(encoded)
		}
	}
	return reactIntegrationDetail{
		ID: integration.ID, Name: integration.Name, Description: integration.Description,
		Method: integration.Method, URL: integration.URL, Headers: publicHeaders,
		Parameters: parameters, Policy: reactIntegrationPolicyFromStore(integration), HasCredentials: reactIntegrationHasCredentials(integration.Headers),
		CreatedAt: integration.CreatedAt.Format(time.RFC3339),
	}
}

func decodeReactAdminJSON(r *http.Request, target interface{}, limit int64) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, limit+1))
	if err := decoder.Decode(target); err != nil {
		return errors.New("JSON inválido")
	}
	return nil
}

func validateReactProvider(provider store.Provider) error {
	if strings.TrimSpace(provider.Name) == "" || len([]rune(provider.Name)) > 120 {
		return errors.New("nome deve ter entre 1 e 120 caracteres")
	}
	switch provider.ProviderType {
	case "notreve", "openai_compat":
	default:
		return errors.New("tipo de provedor inválido")
	}
	if provider.ProviderType == "notreve" && strings.TrimSpace(provider.BaseURL) == "" {
		provider.BaseURL = llm.NotreveBaseURL
	}
	if strings.TrimSpace(provider.BaseURL) != "" {
		parsed, err := url.ParseRequestURI(provider.BaseURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return errors.New("base_url deve ser uma URL HTTP ou HTTPS válida")
		}
	}
	if len([]rune(provider.Model)) > 160 {
		return errors.New("modelo deve ter no máximo 160 caracteres")
	}
	if provider.Capacidade != "texto" && provider.Capacidade != "multimodal" {
		return errors.New("capacidade deve ser texto ou multimodal")
	}
	return nil
}

var providerCapabilityWhitelist = map[string]bool{"imagem": true, "image": true, "image_generation": true}

func normalizeProviderCapabilities(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if !providerCapabilityWhitelist[normalized] || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	return out
}

func applyReactProviderInput(existing *store.Provider, input reactProviderInput, creating bool) error {
	if input.Name != nil {
		existing.Name = strings.TrimSpace(*input.Name)
	}
	if input.ProviderType != nil {
		existing.ProviderType = strings.ToLower(strings.TrimSpace(*input.ProviderType))
	}
	if input.BaseURL != nil {
		existing.BaseURL = strings.TrimSpace(*input.BaseURL)
	}
	if input.Model != nil {
		existing.Model = strings.TrimSpace(*input.Model)
	}
	if input.APIKey != nil && strings.TrimSpace(*input.APIKey) != "" {
		existing.APIKey = strings.TrimSpace(*input.APIKey)
	}
	if input.Active != nil {
		existing.Active = *input.Active
	} else if creating {
		existing.Active = true
	}
	if input.SafeMode != nil {
		existing.SafeMode = *input.SafeMode
	} else if creating {
		existing.SafeMode = true
	}
	if input.Capacidade != nil {
		existing.Capacidade = strings.ToLower(strings.TrimSpace(*input.Capacidade))
	} else if creating {
		existing.Capacidade = "texto"
	}
	if input.Capabilities != nil {
		existing.Capabilities = normalizeProviderCapabilities(*input.Capabilities)
	} else if creating {
		existing.Capabilities = []string{}
	}
	if existing.ProviderType == "notreve" {
		// Notreve IA é uma integração gerenciada: o endpoint é sempre o gateway
		// oficial. O modelo pode ser escolhido na lista retornada pelo gateway.
		existing.BaseURL = llm.NotreveBaseURL
		if strings.TrimSpace(existing.Model) == "" {
			existing.Model = llm.NotreveChatModel
			if existing.Capacidade == "multimodal" {
				existing.Model = llm.NotreveVisionModel
			}
		}
	}
	return validateReactProvider(*existing)
}

func (s *Server) handleReactProviders(w http.ResponseWriter, r *http.Request) {
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	providers, err := s.st.ListProvidersSecure(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os provedores")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	filtered := make([]reactProviderView, 0, len(providers))
	for _, provider := range providers {
		view := reactProviderFromStore(provider)
		if status == "ativos" && !provider.Active {
			continue
		}
		if status == "inativos" && provider.Active {
			continue
		}
		if !reactContainsFold([]string{provider.Name, provider.ProviderType, provider.Model, provider.BaseURL}, query) {
			continue
		}
		filtered = append(filtered, view)
	}
	pageItems, err := reactPageSlice(filtered, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	meta := reactPageMeta(len(filtered), page.Limit, page.Offset, reactQueryFilter(map[string]string{"q": query, "status": status}))
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": pageItems}, meta)
}

func (s *Server) handleReactProvider(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	provider, err := s.st.GetProviderSecure(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "provedor não encontrado")
		return
	}
	writeReactResponse(w, http.StatusOK, reactProviderFromStore(provider), nil)
}

func (s *Server) handleReactProviderCreate(w http.ResponseWriter, r *http.Request) {
	var input reactProviderInput
	if err := decodeReactAdminJSON(r, &input, 64*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	provider := store.Provider{}
	if err := applyReactProviderInput(&provider, input, true); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_provider", err.Error())
		return
	}
	id, err := s.st.CreateProviderSecure(r.Context(), provider)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível criar o provedor")
		return
	}
	created, err := s.st.GetProviderSecure(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "provedor criado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusCreated, reactProviderFromStore(created), nil)
}

func (s *Server) handleReactProviderUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	provider, err := s.st.GetProviderSecure(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "provedor não encontrado")
		return
	}
	var input reactProviderInput
	if err := decodeReactAdminJSON(r, &input, 64*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := applyReactProviderInput(&provider, input, false); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_provider", err.Error())
		return
	}
	if err := s.st.UpdateProviderSecure(r.Context(), provider); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível atualizar o provedor")
		return
	}
	updated, err := s.st.GetProviderSecure(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "provedor atualizado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusOK, reactProviderFromStore(updated), nil)
}

func (s *Server) handleReactProviderDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if _, err := s.st.GetProviderSecure(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "provedor não encontrado")
		return
	}
	if err := s.st.DeleteProvider(r.Context(), id); err != nil {
		writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir o provedor")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"deleted": id}, nil)
}

func (s *Server) handleReactProviderTest(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	provider, err := s.st.GetProviderSecure(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "provedor não encontrado")
		return
	}
	ok, message := testProvider(provider)
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"ok": ok == "true", "message": message}, nil)
}

type reactProviderProbeInput struct {
	ProviderID   *int    `json:"provider_id"`
	ProviderType *string `json:"provider_type"`
	BaseURL      *string `json:"base_url"`
	APIKey       *string `json:"api_key"`
	Capacidade   *string `json:"capacidade"`
}

func (s *Server) resolveReactProbeProvider(r *http.Request) (store.Provider, error) {
	var input reactProviderProbeInput
	if err := decodeReactAdminJSON(r, &input, 64*1024); err != nil {
		return store.Provider{}, err
	}
	provider := store.Provider{}
	if input.ProviderType != nil {
		provider.ProviderType = strings.ToLower(strings.TrimSpace(*input.ProviderType))
	}
	if input.BaseURL != nil {
		provider.BaseURL = strings.TrimSpace(*input.BaseURL)
	}
	if input.APIKey != nil {
		provider.APIKey = strings.TrimSpace(*input.APIKey)
	}
	if input.Capacidade != nil {
		provider.Capacidade = strings.ToLower(strings.TrimSpace(*input.Capacidade))
	}
	if input.ProviderID != nil && *input.ProviderID > 0 {
		stored, err := s.st.GetProviderSecure(r.Context(), *input.ProviderID)
		if err != nil {
			return store.Provider{}, errors.New("provedor não encontrado")
		}
		if provider.ProviderType == "" {
			provider.ProviderType = stored.ProviderType
		}
		if provider.APIKey == "" {
			provider.APIKey = stored.APIKey
		}
		if provider.BaseURL == "" {
			provider.BaseURL = stored.BaseURL
		}
		if provider.Capacidade == "" {
			provider.Capacidade = stored.Capacidade
		}
	}
	if provider.ProviderType == "" {
		provider.ProviderType = "notreve"
	}
	if provider.ProviderType == "notreve" {
		provider.BaseURL = llm.NotreveBaseURL
	}
	if strings.TrimSpace(provider.APIKey) == "" {
		return store.Provider{}, errors.New("informe o token do provedor para continuar")
	}
	return provider, nil
}

func (s *Server) handleReactProviderModels(w http.ResponseWriter, r *http.Request) {
	provider, err := s.resolveReactProbeProvider(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_provider", err.Error())
		return
	}
	models, err := listProviderModels(provider)
	if err != nil {
		writeReactError(w, http.StatusBadGateway, "provider_models_failed", err.Error())
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": models}, nil)
}

func (s *Server) handleReactProviderProbe(w http.ResponseWriter, r *http.Request) {
	provider, err := s.resolveReactProbeProvider(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_provider", err.Error())
		return
	}
	ok, message := testProvider(provider)
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"ok": ok == "true", "message": message}, nil)
}

func validateReactIntegration(integration store.Integration) error {
	if !reactToolNamePattern.MatchString(integration.Name) {
		return errors.New("o nome da tool deve começar com letra ou _ e usar até 64 caracteres")
	}
	if len([]rune(integration.Description)) > 500 {
		return errors.New("a descrição deve ter no máximo 500 caracteres")
	}
	switch strings.ToUpper(integration.Method) {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
	default:
		return errors.New("método HTTP inválido")
	}
	parsed, err := url.ParseRequestURI(integration.URL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("url deve ser uma URL HTTP ou HTTPS válida")
	}
	policy, _, err := tools.PolicyFromParameters(integration.Parameters, integration.Method)
	if err != nil {
		return err
	}
	if err := tools.ValidateEndpoint(integration.URL, integration.Method, policy); err != nil {
		return err
	}
	if integration.RequiresIdentity != "" && integration.RequiresIdentity != "phone" && integration.RequiresIdentity != "challenge" && integration.RequiresIdentity != "resolve" && integration.RequiresIdentity != "otp" {
		return errors.New("identificação exigida inválida")
	}
	if len(integration.AllowedIdentities) > 200 {
		return errors.New("a tool pode autorizar no máximo 200 telefones")
	}
	if integration.IdentityConfig != "" && !json.Valid([]byte(integration.IdentityConfig)) {
		return errors.New("identity_config deve ser um JSON válido")
	}
	if integration.RequiresIdentity == "challenge" {
		var cfg struct {
			Challenges []interface{} `json:"challenges"`
		}
		if err := json.Unmarshal([]byte(integration.IdentityConfig), &cfg); err != nil || len(cfg.Challenges) == 0 {
			return errors.New("o modo desafio exige ao menos uma pergunta em identity_config")
		}
	}
	if strings.TrimSpace(integration.IdentityVerifyURL) != "" {
		parsed, err := url.ParseRequestURI(integration.IdentityVerifyURL)
		if err != nil || parsed.Host == "" || parsed.Scheme != "https" {
			return errors.New("identity_verify_url deve ser uma URL HTTPS válida")
		}
	}
	if integration.IdentityVerifyMethod != "" {
		switch strings.ToUpper(integration.IdentityVerifyMethod) {
		case "GET", "POST", "PUT", "PATCH", "DELETE":
		default:
			return errors.New("identity_verify_method HTTP inválido")
		}
	}
	return nil
}

func normalizeAllowedIdentities(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = true
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func normalizeReactIntegrationHeaders(values map[string]string) (string, error) {
	if values == nil {
		values = map[string]string{}
	}
	clean := map[string]string{}
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if strings.ContainsAny(key, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return "", errors.New("headers não podem conter quebras de linha")
		}
		if len([]rune(key)) > 120 || len([]rune(value)) > 4096 {
			return "", errors.New("header excede o limite permitido")
		}
		clean[key] = value
	}
	b, err := json.Marshal(clean)
	return string(b), err
}

func applyReactIntegrationInput(existing *store.Integration, input reactIntegrationInput, creating bool) error {
	if input.Name != nil {
		existing.Name = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		existing.Description = strings.TrimSpace(*input.Description)
	}
	if input.Method != nil {
		existing.Method = strings.ToUpper(strings.TrimSpace(*input.Method))
	}
	if input.URL != nil {
		existing.URL = strings.TrimSpace(*input.URL)
	}
	if creating && existing.Method == "" {
		existing.Method = "POST"
	}
	parameterSource := existing.Parameters
	if input.Parameters != nil {
		parameterSource = strings.TrimSpace(*input.Parameters)
	}
	policy, schema, err := tools.PolicyFromParameters(parameterSource, existing.Method)
	if err != nil {
		return err
	}
	if input.Policy != nil {
		policy = input.Policy.toTools()
		existing.RequiresIdentity = input.Policy.RequiresIdentity
		existing.AllowedIdentities = normalizeAllowedIdentities(input.Policy.AllowedIdentities)
		existing.IdentityConfig = strings.TrimSpace(input.Policy.IdentityConfig)
		existing.IdentityVerifyURL = strings.TrimSpace(input.Policy.IdentityVerifyURL)
		existing.IdentityVerifyMethod = strings.ToUpper(strings.TrimSpace(input.Policy.IdentityVerifyMethod))
		existing.IdentityVerifyParam = strings.TrimSpace(input.Policy.IdentityVerifyParam)
		existing.IdentityMatchField = strings.TrimSpace(input.Policy.IdentityMatchField)
		existing.IdentityClientIDField = strings.TrimSpace(input.Policy.IdentityClientIDField)
	}
	policy, err = tools.NormalizePolicy(policy, existing.Method)
	if err != nil {
		return err
	}
	existing.Parameters, err = tools.EncodeParameters(schema, policy)
	if err != nil {
		return err
	}
	if input.Headers != nil || input.BearerToken != nil {
		headers := map[string]string{}
		if input.Headers != nil {
			headers = *input.Headers
		}
		if input.BearerToken != nil && strings.TrimSpace(*input.BearerToken) != "" {
			headers["Authorization"] = "Bearer " + strings.TrimSpace(*input.BearerToken)
		}
		headerJSON, err := normalizeReactIntegrationHeaders(headers)
		if err != nil {
			return err
		}
		if !creating {
			headerJSON = preserveSensitiveHeaders(existing.Headers, headerJSON)
		}
		existing.Headers = headerJSON
	}
	return validateReactIntegration(*existing)
}

func (s *Server) handleReactIntegrations(w http.ResponseWriter, r *http.Request) {
	integrations, err := s.st.ListIntegrationsSecure(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar as tools")
		return
	}
	items := make([]reactIntegrationSummary, 0, len(integrations))
	for _, integration := range integrations {
		items = append(items, reactIntegrationFromStore(integration))
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": items, "catalog": tools.Catalog()}, nil)
}

func (s *Server) handleReactIntegration(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	integration, err := s.st.GetIntegrationSecure(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "tool não encontrada")
		return
	}
	writeReactResponse(w, http.StatusOK, reactIntegrationDetailFromStore(integration), nil)
}

func (s *Server) handleReactIntegrationCreate(w http.ResponseWriter, r *http.Request) {
	var input reactIntegrationInput
	if err := decodeReactAdminJSON(r, &input, 128*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	integration := store.Integration{}
	if err := applyReactIntegrationInput(&integration, input, true); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_integration", err.Error())
		return
	}
	if err := s.st.EnsureCanCreateTool(r.Context()); err != nil {
		if writeReactPlanLimitError(w, err) {
			return
		}
		writeReactError(w, http.StatusInternalServerError, "plan_check_failed", "não foi possível validar o limite do plano")
		return
	}
	id, err := s.st.CreateIntegrationSecure(r.Context(), integration)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível criar a tool")
		return
	}
	created, err := s.st.GetIntegrationSecure(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "tool criada, mas não foi possível carregá-la")
		return
	}
	writeReactResponse(w, http.StatusCreated, reactIntegrationDetailFromStore(created), nil)
}

func (s *Server) handleReactIntegrationUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	integration, err := s.st.GetIntegrationSecure(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "tool não encontrada")
		return
	}
	var input reactIntegrationInput
	if err := decodeReactAdminJSON(r, &input, 128*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := applyReactIntegrationInput(&integration, input, false); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_integration", err.Error())
		return
	}
	if err := s.st.UpdateIntegrationSecure(r.Context(), integration); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível atualizar a tool")
		return
	}
	updated, err := s.st.GetIntegrationSecure(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "tool atualizada, mas não foi possível carregá-la")
		return
	}
	writeReactResponse(w, http.StatusOK, reactIntegrationDetailFromStore(updated), nil)
}

func (s *Server) handleReactIntegrationDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if _, err := s.st.GetIntegrationSecure(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "tool não encontrada")
		return
	}
	if err := s.st.DeleteIntegration(r.Context(), id); err != nil {
		writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir a tool")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"deleted": id}, nil)
}
