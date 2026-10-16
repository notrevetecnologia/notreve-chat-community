package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type toolIdentityPhoneKeyType string

const toolIdentityPhoneKey toolIdentityPhoneKeyType = "notreve_tool_identity_phone"

func WithToolIdentityPhone(ctx context.Context, phone string) context.Context {
	return context.WithValue(ctx, toolIdentityPhoneKey, phone)
}

func ToolIdentityPhone(ctx context.Context) string {
	phone, _ := ctx.Value(toolIdentityPhoneKey).(string)
	return phone
}

func normalizeIdentityPhone(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func identityAllowed(identity string, allowed []string) bool {
	identity = normalizeIdentityPhone(identity)
	if identity == "" {
		return false
	}
	for _, candidate := range allowed {
		if normalizeIdentityPhone(candidate) == identity {
			return true
		}
	}
	return false
}

const (
	argIdentityKey         = "_identity"
	argChallengeAnswersKey = "_challenge_answers"
	argOTPKey              = "_otp_code"
)

type Challenge struct {
	Field string `json:"field"`
	Label string `json:"label"`
}

type IdentityInput struct {
	Identifier    string
	ClientAnswers map[string]string
	OTPCode       string
}

type IdentityResult struct {
	Authorized bool
	ClientID   string
	Reason     string
}

type IdentitySettings struct {
	RequiresIdentity  string
	VerifyURL         string
	VerifyMethod      string
	VerifyParam       string
	MatchField        string
	ClientIDField     string
	Challenges        []Challenge
	AllowedIdentities []string
}

type identityConfigJSON struct {
	Challenges []Challenge `json:"challenges"`
}

func IdentitySettingsFromTool(def store.Tool) IdentitySettings {
	method := strings.ToUpper(strings.TrimSpace(def.IdentityVerifyMethod))
	if method == "" {
		method = http.MethodPost
	}
	var challenges []Challenge
	if strings.TrimSpace(def.IdentityConfig) != "" {
		var cfg identityConfigJSON
		if json.Unmarshal([]byte(def.IdentityConfig), &cfg) == nil && len(cfg.Challenges) > 0 {
			challenges = cfg.Challenges
		}
	}
	return IdentitySettings{
		RequiresIdentity:  strings.ToLower(strings.TrimSpace(def.RequiresIdentity)),
		VerifyURL:         strings.TrimSpace(def.IdentityVerifyURL),
		VerifyMethod:      method,
		VerifyParam:       strings.TrimSpace(def.IdentityVerifyParam),
		MatchField:        strings.TrimSpace(def.IdentityMatchField),
		ClientIDField:     strings.TrimSpace(def.IdentityClientIDField),
		Challenges:        challenges,
		AllowedIdentities: def.AllowedIdentities,
	}
}

func (e *Executor) VerifyIdentity(ctx context.Context, settings IdentitySettings, input IdentityInput) (IdentityResult, error) {
	switch settings.RequiresIdentity {
	case "phone":
		return e.verifyByPhone(ctx, settings, input.Identifier)
	case "resolve":
		return e.verifyByResolve(ctx, settings, input.Identifier)
	case "challenge":
		return e.verifyByChallenge(ctx, settings, input)
	case "otp":
		return e.verifyByOTP(ctx, settings, input)
	default:
		return IdentityResult{Authorized: false, Reason: "modo de identificação não configurado"}, nil
	}
}

// verifyByOTP envia o código informado pelo cliente ao verifier externo (que
// envia e valida o código no telefone/e-mail cadastrado) e autoriza se o sistema
// confirmar, retornando o client_id para a ação.
func (e *Executor) verifyByOTP(ctx context.Context, settings IdentitySettings, input IdentityInput) (IdentityResult, error) {
	if strings.TrimSpace(settings.VerifyURL) == "" {
		return IdentityResult{Authorized: false, Reason: "configuração de verificação incompleta (identity_verify_url vazio)"}, nil
	}
	if strings.TrimSpace(input.OTPCode) == "" {
		return IdentityResult{Authorized: false, Reason: "código de verificação não informado"}, nil
	}
	body, err := e.verifyHTTPWithCode(ctx, settings, input.Identifier, input.OTPCode)
	if err != nil {
		return IdentityResult{Authorized: false, Reason: "falha ao validar o código de verificação"}, err
	}
	clientID := resolveJSONField(body, settings.ClientIDField)
	if strings.TrimSpace(clientID) == "" {
		return IdentityResult{Authorized: false, Reason: "código de verificação inválido ou expirado"}, nil
	}
	return IdentityResult{Authorized: true, ClientID: clientID}, nil
}

func (e *Executor) verifyByPhone(ctx context.Context, settings IdentitySettings, identifier string) (IdentityResult, error) {
	if strings.TrimSpace(settings.VerifyURL) != "" {
		return e.verifyByResolve(ctx, settings, identifier)
	}
	if identityAllowed(identifier, settings.AllowedIdentities) {
		return IdentityResult{Authorized: true}, nil
	}
	return IdentityResult{Authorized: false, Reason: "acesso não autorizado para este cliente"}, nil
}

func (e *Executor) verifyByResolve(ctx context.Context, settings IdentitySettings, identifier string) (IdentityResult, error) {
	if strings.TrimSpace(settings.VerifyURL) == "" {
		return IdentityResult{Authorized: false, Reason: "configuração de verificação incompleta (identity_verify_url vazio)"}, nil
	}
	body, err := e.verifyHTTP(ctx, settings, identifier)
	if err != nil {
		return IdentityResult{Authorized: false, Reason: "falha ao consultar o sistema externo de identificação"}, err
	}
	clientID := resolveJSONField(body, settings.ClientIDField)
	if strings.TrimSpace(clientID) == "" {
		return IdentityResult{Authorized: false, Reason: "identidade não encontrada no sistema externo"}, nil
	}
	if strings.TrimSpace(settings.MatchField) != "" {
		matchValue := resolveJSONField(body, settings.MatchField)
		if strings.TrimSpace(matchValue) == "" || !identityValueEqual(matchValue, identifier) {
			return IdentityResult{Authorized: false, Reason: "identidade não confere com o cliente"}, nil
		}
	}
	return IdentityResult{Authorized: true, ClientID: clientID}, nil
}

func (e *Executor) verifyByChallenge(ctx context.Context, settings IdentitySettings, input IdentityInput) (IdentityResult, error) {
	if strings.TrimSpace(settings.VerifyURL) == "" {
		return IdentityResult{Authorized: false, Reason: "configuração de verificação incompleta (identity_verify_url vazio)"}, nil
	}
	if len(settings.Challenges) == 0 {
		return IdentityResult{Authorized: false, Reason: "configuração de verificação incompleta (nenhuma pergunta configurada)"}, nil
	}
	body, err := e.verifyHTTP(ctx, settings, input.Identifier)
	if err != nil {
		return IdentityResult{Authorized: false, Reason: "falha ao consultar o sistema externo de identificação"}, err
	}
	answers, err := challengeAnswers(body)
	if err != nil {
		return IdentityResult{Authorized: false, Reason: "resposta do sistema externo inválida"}, err
	}
	provided := 0
	for _, challenge := range settings.Challenges {
		clientAnswer, ok := input.ClientAnswers[challenge.Field]
		if !ok || strings.TrimSpace(clientAnswer) == "" {
			continue
		}
		provided++
		expected, ok := answers[challenge.Field]
		if !ok || !maskedMatch(clientAnswer, expected) {
			return IdentityResult{Authorized: false, Reason: "as respostas de segurança não conferem"}, nil
		}
	}
	if provided == 0 {
		return IdentityResult{Authorized: false, Reason: "nenhuma resposta de segurança informada"}, nil
	}
	return IdentityResult{Authorized: true, ClientID: resolveJSONField(body, settings.ClientIDField)}, nil
}

func (e *Executor) verifyHTTP(ctx context.Context, settings IdentitySettings, identifier string) ([]byte, error) {
	method := strings.ToUpper(strings.TrimSpace(settings.VerifyMethod))
	if method == "" {
		method = http.MethodPost
	}
	if strings.TrimSpace(settings.VerifyURL) == "" {
		return nil, errors.New("identity_verify_url vazio")
	}
	policy := DefaultPolicy(method)
	if err := validateEndpoint(settings.VerifyURL, method, policy, e.AllowPrivateNetwork); err != nil {
		return nil, err
	}
	param := strings.TrimSpace(settings.VerifyParam)
	if param == "" {
		param = "identifier"
	}
	requestContext, cancel := context.WithTimeout(ctx, time.Duration(policy.TimeoutSeconds)*time.Second)
	defer cancel()
	var body io.Reader
	requestURL := settings.VerifyURL
	payload := map[string]interface{}{param: identifier}
	payloadJSON, _ := json.Marshal(payload)
	if method == http.MethodGet || method == http.MethodDelete {
		requestURL = appendQuery(requestURL, payload)
	} else {
		body = strings.NewReader(string(payloadJSON))
	}
	req, err := http.NewRequestWithContext(requestContext, method, requestURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.clientFor(policy).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(policy.MaxResponseBytes)+1))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

// verifyHTTPWithCode envia identifier + code (OTP) ao verifier externo, que
// confirma se o código bate com o número/e-mail cadastrado.
func (e *Executor) verifyHTTPWithCode(ctx context.Context, settings IdentitySettings, identifier, code string) ([]byte, error) {
	method := strings.ToUpper(strings.TrimSpace(settings.VerifyMethod))
	if method == "" {
		method = http.MethodPost
	}
	if strings.TrimSpace(settings.VerifyURL) == "" {
		return nil, errors.New("identity_verify_url vazio")
	}
	policy := DefaultPolicy(method)
	if err := validateEndpoint(settings.VerifyURL, method, policy, e.AllowPrivateNetwork); err != nil {
		return nil, err
	}
	param := strings.TrimSpace(settings.VerifyParam)
	if param == "" {
		param = "identifier"
	}
	requestContext, cancel := context.WithTimeout(ctx, time.Duration(policy.TimeoutSeconds)*time.Second)
	defer cancel()
	var body io.Reader
	requestURL := settings.VerifyURL
	payload := map[string]interface{}{param: identifier, "code": code}
	payloadJSON, _ := json.Marshal(payload)
	if method == http.MethodGet || method == http.MethodDelete {
		requestURL = appendQuery(requestURL, payload)
	} else {
		body = strings.NewReader(string(payloadJSON))
	}
	req, err := http.NewRequestWithContext(requestContext, method, requestURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.clientFor(policy).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(policy.MaxResponseBytes)+1))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func identityOTPCode(args map[string]interface{}) string {
	if value, ok := args[argOTPKey]; ok {
		if text, ok := value.(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func identityIdentifier(ctx context.Context, args map[string]interface{}, settings IdentitySettings) string {
	if value, ok := args[argIdentityKey]; ok && value != nil {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return text
		}
	}
	if settings.VerifyParam != "" {
		if value, ok := args[settings.VerifyParam]; ok && value != nil {
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				return text
			}
		}
	}
	return ToolIdentityPhone(ctx)
}

func identityClientAnswers(args map[string]interface{}) map[string]string {
	value, ok := args[argChallengeAnswersKey]
	if !ok || value == nil {
		return nil
	}
	if direct, ok := value.(map[string]string); ok {
		return direct
	}
	raw, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	out := make(map[string]string, len(raw))
	for key, item := range raw {
		if text, ok := item.(string); ok {
			out[key] = text
		}
	}
	return out
}

func withoutReservedArgs(args map[string]interface{}) map[string]interface{} {
	if args == nil {
		return nil
	}
	out := make(map[string]interface{}, len(args))
	for key, value := range args {
		if key == argIdentityKey || key == argChallengeAnswersKey || key == argOTPKey {
			continue
		}
		out[key] = value
	}
	return out
}

func identityValueEqual(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if strings.EqualFold(a, b) {
		return true
	}
	return normalizeIdentityMatch(a) == normalizeIdentityMatch(b) && normalizeIdentityMatch(a) != ""
}

func normalizeIdentityMatch(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func maskedMatch(got, expected string) bool {
	got = strings.TrimSpace(got)
	expected = strings.TrimSpace(expected)
	if got == "" || expected == "" {
		return false
	}
	gotRunes := []rune(got)
	expRunes := []rune(expected)
	if len(gotRunes) == len(expRunes) {
		for i, expectedChar := range expRunes {
			if expectedChar == '*' || expectedChar == '.' {
				continue
			}
			if unicode.ToLower(gotRunes[i]) != unicode.ToLower(expectedChar) {
				return false
			}
		}
		return true
	}
	return normalizeIdentityMatch(got) == normalizeIdentityMatch(expected) && normalizeIdentityMatch(expected) != ""
}

func resolveJSONField(body []byte, path string) string {
	path = strings.TrimSpace(path)
	if path == "" || len(body) == 0 {
		return ""
	}
	var root interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		return ""
	}
	value, ok := walkJSON(root, strings.Split(path, "."))
	if !ok {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	if number, ok := value.(float64); ok {
		return strconv.FormatFloat(number, 'f', -1, 64)
	}
	if number, ok := value.(json.Number); ok {
		return number.String()
	}
	if number, ok := value.(int); ok {
		return strconv.Itoa(number)
	}
	return ""
}

func walkJSON(node interface{}, parts []string) (interface{}, bool) {
	for _, part := range parts {
		switch current := node.(type) {
		case map[string]interface{}:
			next, ok := current[part]
			if !ok {
				return nil, false
			}
			node = next
		case []interface{}:
			index := -1
			if parsed, err := strconv.Atoi(part); err == nil && parsed >= 0 && parsed < len(current) {
				index = parsed
			}
			if index < 0 {
				return nil, false
			}
			node = current[index]
		default:
			return nil, false
		}
	}
	return node, true
}

func challengeAnswers(body []byte) (map[string]string, error) {
	var root map[string]interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	container := root
	for _, key := range []string{"challenges", "data"} {
		if nested, ok := root[key].(map[string]interface{}); ok {
			container = nested
			break
		}
	}
	out := make(map[string]string, len(container))
	for key, value := range container {
		if text, ok := value.(string); ok {
			out[key] = text
		}
	}
	return out, nil
}
