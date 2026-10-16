package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type Policy struct {
	Connector            string    `json:"connector,omitempty"`
	Risk                 RiskLevel `json:"risk"`
	RequiresConfirmation bool      `json:"requires_confirmation"`
	RequiredScopes       []string  `json:"required_scopes,omitempty"`
	AllowedHosts         []string  `json:"allowed_hosts,omitempty"`
	TimeoutSeconds       int       `json:"timeout_seconds"`
	MaxCallsPerMinute    int       `json:"max_calls_per_minute"`
	MaxCallsPerHour      int       `json:"max_calls_per_hour"`
	MaxResponseBytes     int       `json:"max_response_bytes"`
	MaxArgumentBytes     int       `json:"max_argument_bytes"`
	RequiresIdentity     string    `json:"requires_identity,omitempty"`
	AllowedIdentities    []string  `json:"allowed_identities,omitempty"`
}

type CatalogEntry struct {
	Key                  string    `json:"key"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	Method               string    `json:"method"`
	Risk                 RiskLevel `json:"risk"`
	RequiresConfirmation bool      `json:"requires_confirmation"`
	RequiredScopes       []string  `json:"required_scopes"`
	TimeoutSeconds       int       `json:"timeout_seconds"`
	MaxCallsPerMinute    int       `json:"max_calls_per_minute"`
	MaxCallsPerHour      int       `json:"max_calls_per_hour"`
	Parameters           string    `json:"parameters,omitempty"`          // JSON schema dos parâmetros que a IA preenche
	RequiresIdentity     string    `json:"requires_identity,omitempty"`   // "" | phone | challenge | resolve
	IdentityHint         string    `json:"identity_hint,omitempty"`       // o que o admin deve preencher no guard
}

type AuditEvent struct {
	At                   time.Time `json:"at"`
	Tool                 string    `json:"tool"`
	Connector            string    `json:"connector,omitempty"`
	Risk                 RiskLevel `json:"risk"`
	Status               string    `json:"status"`
	HTTPStatus           int       `json:"http_status,omitempty"`
	DurationMilliseconds int64     `json:"duration_ms"`
	RequiredScopes       []string  `json:"required_scopes,omitempty"`
	MissingScopes        []string  `json:"missing_scopes,omitempty"`
	ConfirmationRequired bool      `json:"confirmation_required,omitempty"`
	ArgumentNames        []string  `json:"argument_names,omitempty"`
}

type AuditSink interface {
	RecordToolAudit(context.Context, AuditEvent) error
}

type AuditFunc func(context.Context, AuditEvent) error

func (f AuditFunc) RecordToolAudit(ctx context.Context, event AuditEvent) error {
	return f(ctx, event)
}

type LogAuditSink struct{}

func (LogAuditSink) RecordToolAudit(_ context.Context, event AuditEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	log.Printf("tool_audit %s", payload)
	return nil
}

type MemoryAuditSink struct {
	mu     sync.Mutex
	Events []AuditEvent
}

func (s *MemoryAuditSink) RecordToolAudit(_ context.Context, event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Events = append(s.Events, event)
	return nil
}

type contextKey string

const (
	toolScopesKey       contextKey = "tool_scopes"
	toolConfirmationKey contextKey = "tool_confirmation"
)

func WithToolScopes(ctx context.Context, scopes ...string) context.Context {
	return context.WithValue(ctx, toolScopesKey, normalizeScopes(scopes))
}

func WithToolConfirmation(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, toolConfirmationKey, strings.TrimSpace(token))
}

func ToolScopes(ctx context.Context) []string {
	values, _ := ctx.Value(toolScopesKey).([]string)
	return append([]string{}, values...)
}

func DefaultPolicy(method string) Policy {
	method = strings.ToUpper(strings.TrimSpace(method))
	policy := Policy{Risk: RiskMedium, TimeoutSeconds: 20, MaxCallsPerMinute: 30, MaxCallsPerHour: 300, MaxResponseBytes: 1 << 20, MaxArgumentBytes: 32 << 10}
	if method == "GET" {
		policy.Risk = RiskLow
		policy.TimeoutSeconds = 10
		policy.MaxCallsPerMinute = 60
		policy.MaxCallsPerHour = 1000
	}
	if method == "DELETE" {
		policy.Risk = RiskHigh
		policy.RequiresConfirmation = true
		policy.MaxCallsPerMinute = 10
		policy.MaxCallsPerHour = 100
	}
	return policy
}

func Catalog() []CatalogEntry {
	return []CatalogEntry{
		{Key: "rest_read", Name: "Consulta REST", Description: "Consulta dados de um endpoint externo sem alterar registros.", Method: "GET", Risk: RiskLow, RequiredScopes: []string{"data.read"}, TimeoutSeconds: 10, MaxCallsPerMinute: 60, MaxCallsPerHour: 1000},
		{Key: "rest_action", Name: "Ação REST", Description: "Executa uma operação em um sistema externo com escopo de escrita.", Method: "POST", Risk: RiskHigh, RequiresConfirmation: true, RequiredScopes: []string{"data.write"}, TimeoutSeconds: 20, MaxCallsPerMinute: 20, MaxCallsPerHour: 200},
		{Key: "webhook", Name: "Webhook seguro", Description: "Envia um evento para um endpoint autorizado.", Method: "POST", Risk: RiskHigh, RequiresConfirmation: true, RequiredScopes: []string{"webhook.send"}, TimeoutSeconds: 15, MaxCallsPerMinute: 20, MaxCallsPerHour: 200},
		{Key: "payment_action", Name: "Ação financeira", Description: "Executa uma ação financeira somente após confirmação explícita.", Method: "POST", Risk: RiskCritical, RequiresConfirmation: true, RequiredScopes: []string{"payments.write"}, TimeoutSeconds: 20, MaxCallsPerMinute: 5, MaxCallsPerHour: 30},
		{
			Key: "mkauth_customer", Name: "MKAUTH · Consultar cliente/fatura", Method: "GET", Risk: RiskLow,
			Description: "Localiza o cliente no MKAUTH por CPF/CNPJ ou telefone e devolve a fatura (2ª via). Use a identificação por resolução no sistema ou por telefone do dono.",
			RequiredScopes: []string{"data.read"}, TimeoutSeconds: 15, MaxCallsPerMinute: 30, MaxCallsPerHour: 300,
			Parameters:      `{"type":"object","properties":{"cpf_cnpj":{"type":"string","description":"CPF ou CNPJ do cliente"},"phone":{"type":"string","description":"Telefone do cliente"}},"required":[]}`,
			RequiresIdentity: "resolve", IdentityHint: "identity_verify_url = endpoint do MKAUTH que retorna o cliente; identity_match_field = telefone/e-mail do registro; client_id_field = id do cliente",
		},
		{
			Key: "mkauth_ticket", Name: "MKAUTH · Abrir chamado", Method: "POST", Risk: RiskHigh, RequiresConfirmation: true,
			Description: "Abre um chamado no MKAUTH. Exija identificação por desafio (perguntas) para não deixar qualquer número abrir chamado.",
			RequiredScopes: []string{"data.write"}, TimeoutSeconds: 25, MaxCallsPerMinute: 10, MaxCallsPerHour: 100,
			Parameters:      `{"type":"object","properties":{"cpf_cnpj":{"type":"string","description":"CPF ou CNPJ do cliente"},"phone":{"type":"string","description":"Telefone do cliente"},"assunto":{"type":"string"},"mensagem":{"type":"string"}},"required":["assunto","mensagem"]}`,
			RequiresIdentity: "challenge", IdentityHint: "identity_verify_url = endpoint MKAUTH que devolve respostas mascaradas (nascimento, nome da mãe...); lista perguntas em identity_config",
		},
		{
			Key: "whmcs_customer", Name: "WHMCS · Consultar cliente", Method: "GET", Risk: RiskLow,
			Description: "Localiza o cliente no WHMCS (GetClients/GetClientsDetails) por e-mail, CPF ou ID.",
			RequiredScopes: []string{"data.read"}, TimeoutSeconds: 15, MaxCallsPerMinute: 30, MaxCallsPerHour: 300,
			Parameters:      `{"type":"object","properties":{"email":{"type":"string"},"cpf_cnpj":{"type":"string"},"clientid":{"type":"string"}},"required":[]}`,
			RequiresIdentity: "resolve", IdentityHint: "identity_verify_url = endpoint que valida/retorna o cliente no WHMCS; match_field = email/telefone; client_id_field = clientid",
		},
		{
			Key: "whmcs_invoice", Name: "WHMCS · 2ª via de fatura", Method: "GET", Risk: RiskLow,
			Description: "Consulta a fatura (GetInvoice) do cliente no WHMCS e devolve o valor/link de pagamento.",
			RequiredScopes: []string{"data.read"}, TimeoutSeconds: 15, MaxCallsPerMinute: 30, MaxCallsPerHour: 300,
			Parameters:      `{"type":"object","properties":{"clientid":{"type":"string"},"invoiceid":{"type":"string"}},"required":["invoiceid"]}`,
			RequiresIdentity: "phone", IdentityHint: "allowed_identities = números dos clientes que podem ver a própria fatura",
		},
		{
			Key: "whmcs_ticket", Name: "WHMCS · Abrir ticket", Method: "POST", Risk: RiskHigh, RequiresConfirmation: true,
			Description: "Abre um ticket no WHMCS (OpenTicket). Exija identificação por desafio/OTP para proteção.",
			RequiredScopes: []string{"data.write"}, TimeoutSeconds: 25, MaxCallsPerMinute: 10, MaxCallsPerHour: 100,
			Parameters:      `{"type":"object","properties":{"clientid":{"type":"string"},"subject":{"type":"string"},"message":{"type":"string"}},"required":["subject","message"]}`,
			RequiresIdentity: "challenge", IdentityHint: "identity_verify_url = endpoint WHMCS que devolve respostas mascaradas; perguntas em identity_config",
		},
		{
			Key: "asaas_customer", Name: "ASAAS · Consultar cliente", Method: "GET", Risk: RiskLow,
			Description: "Localiza o cliente na ASAAS (GET /customers?cpfCnpj=) e devolve o id do cliente.",
			RequiredScopes: []string{"data.read"}, TimeoutSeconds: 15, MaxCallsPerMinute: 30, MaxCallsPerHour: 300,
			Parameters:      `{"type":"object","properties":{"cpf_cnpj":{"type":"string"}},"required":["cpf_cnpj"]}`,
			RequiresIdentity: "resolve", IdentityHint: "identity_verify_url = endpoint ASAAS de busca por cpfCnpj; match_field = phone/email; client_id_field = data.id",
		},
		{
			Key: "asaas_charge", Name: "ASAAS · Criar cobrança (boleto/Pix)", Method: "POST", Risk: RiskCritical, RequiresConfirmation: true,
			Description: "Cria uma cobrança na ASAAS (POST /payments). Ação financeira crítica: exija confirmação e identificação.",
			RequiredScopes: []string{"payments.write"}, TimeoutSeconds: 25, MaxCallsPerMinute: 5, MaxCallsPerHour: 30,
			Parameters:      `{"type":"object","properties":{"customer":{"type":"string","description":"id do cliente ASAAS"},"value":{"type":"number"},"dueDate":{"type":"string","description":"AAAA-MM-DD"}},"required":["customer","value"]}`,
			RequiresIdentity: "challenge", IdentityHint: "identity_verify_url = endpoint ASAAS que valida o cliente; perguntas de segurança em identity_config",
		},
	}
}

const (
	policySchemaKey       = "x-tool-policy"
	legacyPolicySchemaKey = "x_notreve"
	legacyPolicyDashedKey = "x-notreve"
)

var policySchemaKeys = []string{policySchemaKey, legacyPolicySchemaKey, legacyPolicyDashedKey}

func isPolicySchemaKey(key string) bool {
	for _, candidate := range policySchemaKeys {
		if key == candidate {
			return true
		}
	}
	return false
}

func PolicyFromParameters(parameters, method string) (Policy, map[string]interface{}, error) {
	schema, err := parseJSONSchema(parameters)
	if err != nil {
		return Policy{}, nil, err
	}
	policy := DefaultPolicy(method)
	for _, key := range policySchemaKeys {
		raw, ok := schema[key]
		if !ok {
			continue
		}
		encoded, err := json.Marshal(raw)
		if err != nil {
			return Policy{}, nil, errors.New("política da tool inválida")
		}
		if err := json.Unmarshal(encoded, &policy); err != nil {
			return Policy{}, nil, errors.New("política da tool inválida")
		}
		delete(schema, key)
	}
	policy, err = NormalizePolicy(policy, method)
	if err != nil {
		return Policy{}, nil, err
	}
	return policy, schema, nil
}

func EncodeParameters(schema map[string]interface{}, policy Policy) (string, error) {
	if schema == nil {
		schema = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	}
	copySchema := map[string]interface{}{}
	for key, value := range schema {
		if !isPolicySchemaKey(key) {
			copySchema[key] = value
		}
	}
	normalized, err := NormalizePolicy(policy, "POST")
	if err != nil {
		return "", err
	}
	encodedPolicy, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	var policyJSON map[string]interface{}
	if err := json.Unmarshal(encodedPolicy, &policyJSON); err != nil {
		return "", err
	}
	copySchema[policySchemaKey] = policyJSON
	encoded, err := json.Marshal(copySchema)
	return string(encoded), err
}

func ParameterSchema(parameters string) (map[string]interface{}, error) {
	_, schema, err := PolicyFromParameters(parameters, "POST")
	return schema, err
}

func PolicyFromTool(def store.Tool) (Policy, error) {
	if def.Risk != "" || len(def.RequiredScopes) > 0 || def.TimeoutSeconds > 0 || def.MaxCallsPerMinute > 0 || def.MaxCallsPerHour > 0 || def.MaxResponseBytes > 0 || def.MaxArgumentBytes > 0 || def.Connector != "" || def.RequiresConfirmation || def.RequiresIdentity != "" || len(def.AllowedIdentities) > 0 {
		return NormalizePolicy(Policy{Connector: def.Connector, Risk: RiskLevel(def.Risk), RequiresConfirmation: def.RequiresConfirmation, RequiredScopes: def.RequiredScopes, AllowedHosts: def.AllowedHosts, TimeoutSeconds: def.TimeoutSeconds, MaxCallsPerMinute: def.MaxCallsPerMinute, MaxCallsPerHour: def.MaxCallsPerHour, MaxResponseBytes: def.MaxResponseBytes, MaxArgumentBytes: def.MaxArgumentBytes, RequiresIdentity: def.RequiresIdentity, AllowedIdentities: def.AllowedIdentities}, def.Method)
	}
	policy, _, err := PolicyFromParameters("", def.Method)
	return policy, err
}

func NormalizePolicy(policy Policy, method string) (Policy, error) {
	defaults := DefaultPolicy(method)
	if policy.Risk == "" {
		policy.Risk = defaults.Risk
	}
	if policy.TimeoutSeconds == 0 {
		policy.TimeoutSeconds = defaults.TimeoutSeconds
	}
	if policy.MaxCallsPerMinute == 0 {
		policy.MaxCallsPerMinute = defaults.MaxCallsPerMinute
	}
	if policy.MaxCallsPerHour == 0 {
		policy.MaxCallsPerHour = defaults.MaxCallsPerHour
	}
	if policy.MaxResponseBytes == 0 {
		policy.MaxResponseBytes = defaults.MaxResponseBytes
	}
	if policy.MaxArgumentBytes == 0 {
		policy.MaxArgumentBytes = defaults.MaxArgumentBytes
	}
	policy.Connector = strings.ToLower(strings.TrimSpace(policy.Connector))
	policy.RequiredScopes = normalizeScopes(policy.RequiredScopes)
	policy.AllowedHosts = normalizeHosts(policy.AllowedHosts)
	policy.RequiresIdentity = strings.ToLower(strings.TrimSpace(policy.RequiresIdentity))
	policy.AllowedIdentities = normalizeIdentities(policy.AllowedIdentities)
	if policy.Risk == RiskCritical {
		policy.RequiresConfirmation = true
	}
	if err := ValidatePolicy(policy); err != nil {
		return Policy{}, err
	}
	return policy, nil
}

func ValidatePolicy(policy Policy) error {
	if policy.Risk != RiskLow && policy.Risk != RiskMedium && policy.Risk != RiskHigh && policy.Risk != RiskCritical {
		return errors.New("nível de risco inválido")
	}
	if policy.Connector != "" && !regexp.MustCompile(`^[a-z][a-z0-9._-]{1,63}$`).MatchString(policy.Connector) {
		return errors.New("conector inválido")
	}
	if len(policy.RequiredScopes) > 16 {
		return errors.New("a tool pode exigir no máximo 16 escopos")
	}
	for _, scope := range policy.RequiredScopes {
		if !regexp.MustCompile(`^[a-z][a-z0-9_.:-]{1,63}$`).MatchString(scope) {
			return errors.New("escopo inválido")
		}
	}
	if len(policy.AllowedHosts) > 16 {
		return errors.New("a tool pode autorizar no máximo 16 hosts")
	}
	for _, host := range policy.AllowedHosts {
		if strings.ContainsAny(host, "/\\?#\r\n") || host == "*" || strings.HasPrefix(host, "*.") && len(host) <= 2 {
			return errors.New("host autorizado inválido")
		}
	}
	if policy.TimeoutSeconds < 1 || policy.TimeoutSeconds > 60 {
		return errors.New("timeout deve estar entre 1 e 60 segundos")
	}
	if policy.MaxCallsPerMinute < 1 || policy.MaxCallsPerMinute > 1000 || policy.MaxCallsPerHour < 1 || policy.MaxCallsPerHour > 10000 {
		return errors.New("limite de chamadas inválido")
	}
	if policy.MaxResponseBytes < 1024 || policy.MaxResponseBytes > 4<<20 {
		return errors.New("limite de resposta inválido")
	}
	if policy.MaxArgumentBytes < 128 || policy.MaxArgumentBytes > 128<<10 {
		return errors.New("limite de argumentos inválido")
	}
	if policy.RequiresIdentity != "" && policy.RequiresIdentity != "phone" && policy.RequiresIdentity != "challenge" && policy.RequiresIdentity != "resolve" && policy.RequiresIdentity != "otp" {
		return errors.New("identificação exigida inválida")
	}
	if len(policy.AllowedIdentities) > 200 {
		return errors.New("a tool pode autorizar no máximo 200 identidades")
	}
	return nil
}

func ValidateEndpoint(rawURL, method string, policy Policy) error {
	return validateEndpoint(rawURL, method, policy, false)
}

func ValidateArguments(schema map[string]interface{}, arguments map[string]interface{}) error {
	if schema == nil {
		return nil
	}
	if schemaType, ok := schema["type"].(string); ok && schemaType != "object" {
		return errors.New("parameters deve declarar um objeto")
	}
	properties, _ := schema["properties"].(map[string]interface{})
	if required, ok := schema["required"].([]interface{}); ok {
		for _, value := range required {
			name, ok := value.(string)
			if ok {
				if _, exists := arguments[name]; !exists {
					return fmt.Errorf("argumento obrigatório ausente: %s", name)
				}
			}
		}
	}
	additional := true
	if value, ok := schema["additionalProperties"].(bool); ok {
		additional = value
	}
	for name, value := range arguments {
		definition, exists := properties[name]
		if !exists && !additional {
			return fmt.Errorf("argumento não permitido: %s", name)
		}
		if !exists {
			continue
		}
		definitionMap, _ := definition.(map[string]interface{})
		if err := validateArgumentValue(name, value, definitionMap); err != nil {
			return err
		}
	}
	return nil
}

func ConfirmationToken(def store.Tool, arguments map[string]interface{}) string {
	payload, _ := json.Marshal(struct {
		Tool string                 `json:"tool"`
		Args map[string]interface{} `json:"args"`
	}{Tool: def.Name, Args: arguments})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])[:24]
}

func MissingScopes(policy Policy, granted []string) []string {
	set := map[string]bool{}
	for _, scope := range normalizeScopes(granted) {
		set[scope] = true
	}
	var missing []string
	for _, scope := range policy.RequiredScopes {
		if !set[scope] {
			missing = append(missing, scope)
		}
	}
	return missing
}

func parseJSONSchema(parameters string) (map[string]interface{}, error) {
	if strings.TrimSpace(parameters) == "" {
		return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}, nil
	}
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(parameters), &schema); err != nil || schema == nil {
		return nil, errors.New("parameters deve ser um objeto JSON válido")
	}
	return schema, nil
}

func normalizeScopes(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
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

func normalizeHosts(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if parsed, err := url.Parse(value); err == nil && parsed.Hostname() != "" {
			value = parsed.Hostname()
		}
		set[value] = true
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func normalizeIdentities(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		set[value] = true
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func validateEndpoint(rawURL, method string, policy Policy, allowPrivate bool) error {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method != "GET" && method != "POST" && method != "PUT" && method != "PATCH" && method != "DELETE" {
		return errors.New("método HTTP inválido")
	}
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("url deve ser uma URL HTTP ou HTTPS válida")
	}
	if parsed.User != nil {
		return errors.New("url não pode conter credenciais")
	}
	host := strings.ToLower(parsed.Hostname())
	if !allowPrivate && isPrivateHost(host) {
		return errors.New("host privado não permitido")
	}
	if len(policy.AllowedHosts) > 0 && !hostAllowed(host, policy.AllowedHosts) {
		return errors.New("host não está no escopo autorizado")
	}
	return nil
}

func isPrivateHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast())
}

func hostAllowed(host string, allowed []string) bool {
	for _, rule := range allowed {
		rule = strings.ToLower(strings.TrimSpace(rule))
		if rule == host || strings.HasPrefix(rule, "*.") && strings.HasSuffix(host, strings.TrimPrefix(rule, "*")) {
			return true
		}
	}
	return false
}

func validateArgumentValue(name string, value interface{}, definition map[string]interface{}) error {
	typ, _ := definition["type"].(string)
	valid := true
	switch typ {
	case "string":
		_, valid = value.(string)
	case "number", "integer":
		switch value.(type) {
		case float64, float32, int, int64, int32, json.Number:
		default:
			valid = false
		}
	case "boolean":
		_, valid = value.(bool)
	case "object":
		_, valid = value.(map[string]interface{})
	case "array":
		_, valid = value.([]interface{})
	}
	if !valid {
		return fmt.Errorf("tipo inválido para o argumento: %s", name)
	}
	if enum, ok := definition["enum"].([]interface{}); ok {
		encoded, _ := json.Marshal(value)
		matched := false
		for _, item := range enum {
			candidate, _ := json.Marshal(item)
			if string(candidate) == string(encoded) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("valor não permitido para o argumento: %s", name)
		}
	}
	if maxLength, ok := definition["maxLength"].(float64); ok {
		if text, ok := value.(string); ok && len([]rune(text)) > int(maxLength) {
			return fmt.Errorf("argumento excede o tamanho permitido: %s", name)
		}
	}
	return nil
}
