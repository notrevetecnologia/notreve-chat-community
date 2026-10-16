package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/securesecret"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type Executor struct {
	HTTP                *http.Client
	Audit               AuditSink
	AllowPrivateNetwork bool
	now                 func() time.Time
	limiter             *rateLimiter
}

type ExecutionResult struct {
	Text              string
	Status            string
	HTTPStatus        int
	ConfirmationToken string
	MissingScopes     []string
}

func NewExecutor() *Executor {
	return &Executor{HTTP: &http.Client{Timeout: 30 * time.Second}, Audit: LogAuditSink{}, now: time.Now, limiter: &rateLimiter{calls: map[string][]time.Time{}}}
}

func (e *Executor) Execute(ctx context.Context, def store.Tool, arguments map[string]interface{}) string {
	return e.ExecuteDetailed(ctx, def, arguments).Text
}

func decryptRuntimeHeaders(def store.Tool) (store.Tool, error) {
	if len(def.Headers) == 0 {
		return def, nil
	}
	out := make(map[string]string, len(def.Headers))
	for key, value := range def.Headers {
		if securesecret.IsEncrypted(value) {
			plain, err := securesecret.Decrypt("integrations.headers."+strings.ToLower(strings.TrimSpace(key)), value)
			if err != nil {
				return store.Tool{}, fmt.Errorf("credencial da integração indisponível: %w", err)
			}
			value = plain
		}
		out[key] = value
	}
	def.Headers = out
	return def, nil
}

func (e *Executor) ExecuteDetailed(ctx context.Context, def store.Tool, arguments map[string]interface{}) ExecutionResult {
	started := e.currentTime()
	var err error
	def, err = decryptRuntimeHeaders(def)
	if err != nil {
		return ExecutionResult{Text: "Ferramenta bloqueada: credencial protegida indisponível.", Status: "blocked_credentials"}
	}
	policy, err := PolicyFromTool(def)
	if err != nil {
		return e.finish(ctx, def, policy, arguments, started, ExecutionResult{Text: "Ferramenta bloqueada: política inválida.", Status: "blocked_policy"})
	}
	if err := validateEndpoint(def.URL, def.Method, policy, e.AllowPrivateNetwork); err != nil {
		return e.finish(ctx, def, policy, arguments, started, ExecutionResult{Text: "Ferramenta bloqueada: " + err.Error(), Status: "blocked_validation"})
	}
	if policy.RequiresIdentity == "phone" || policy.RequiresIdentity == "challenge" || policy.RequiresIdentity == "resolve" {
		settings := IdentitySettingsFromTool(def)
		res, err := e.VerifyIdentity(ctx, settings, IdentityInput{
			Identifier:    identityIdentifier(ctx, arguments, settings),
			ClientAnswers: identityClientAnswers(arguments),
			OTPCode:       identityOTPCode(arguments),
		})
		if err != nil {
			return e.finish(ctx, def, policy, arguments, started, ExecutionResult{Text: "Identidade não confirmada: " + err.Error(), Status: "blocked_identity"})
		}
		if !res.Authorized {
			return e.finish(ctx, def, policy, arguments, started, ExecutionResult{Text: "Identidade não confirmada. " + res.Reason, Status: "blocked_identity"})
		}
		if res.ClientID != "" {
			arguments["client_id"] = res.ClientID
		}
	}
	requestArgs := withoutReservedArgs(arguments)
	if err := ValidateArguments(def.Parameters, requestArgs); err != nil {
		return e.finish(ctx, def, policy, arguments, started, ExecutionResult{Text: "Argumentos inválidos: " + err.Error(), Status: "blocked_arguments"})
	}
	encodedArguments, err := json.Marshal(requestArgs)
	if err != nil || len(encodedArguments) > policy.MaxArgumentBytes {
		return e.finish(ctx, def, policy, arguments, started, ExecutionResult{Text: "Ferramenta bloqueada: argumentos excedem o limite permitido.", Status: "blocked_arguments"})
	}
	missing := MissingScopes(policy, ToolScopes(ctx))
	if len(missing) > 0 {
		return e.finish(ctx, def, policy, arguments, started, ExecutionResult{Text: "Ferramenta bloqueada: escopos ausentes (" + strings.Join(missing, ", ") + ").", Status: "blocked_scope", MissingScopes: missing})
	}
	token := ConfirmationToken(def, arguments)
	provided, _ := ctx.Value(toolConfirmationKey).(string)
	if policy.RequiresConfirmation && provided != token {
		return e.finish(ctx, def, policy, arguments, started, ExecutionResult{Text: "Confirmação necessária para executar esta ação. Token: " + token, Status: "confirmation_required", ConfirmationToken: token})
	}
	if !e.limiterFor().Allow(def.Name, policy, e.currentTime()) {
		return e.finish(ctx, def, policy, arguments, started, ExecutionResult{Text: "Ferramenta bloqueada: limite de chamadas atingido.", Status: "blocked_limit"})
	}

	requestContext, cancel := context.WithTimeout(ctx, time.Duration(policy.TimeoutSeconds)*time.Second)
	defer cancel()
	method := strings.ToUpper(strings.TrimSpace(def.Method))
	var body io.Reader
	requestURL := def.URL
	if method == "GET" || method == "DELETE" {
		requestURL = appendQuery(requestURL, requestArgs)
	} else {
		body = strings.NewReader(string(encodedArguments))
	}
	req, err := http.NewRequestWithContext(requestContext, method, requestURL, body)
	if err != nil {
		return e.finish(ctx, def, policy, arguments, started, ExecutionResult{Text: fmt.Sprintf("Erro ao montar requisição: %v", err), Status: "request_error"})
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range def.Headers {
		req.Header.Set(key, value)
	}
	resp, err := e.clientFor(policy).Do(req)
	if err != nil {
		return e.finish(ctx, def, policy, arguments, started, ExecutionResult{Text: fmt.Sprintf("Erro na chamada externa: %v", err), Status: "request_error"})
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(policy.MaxResponseBytes)+1))
	if err != nil {
		return e.finish(ctx, def, policy, arguments, started, ExecutionResult{Text: fmt.Sprintf("Erro lendo resposta: %v", err), Status: "response_error", HTTPStatus: resp.StatusCode})
	}
	if len(data) > policy.MaxResponseBytes {
		return e.finish(ctx, def, policy, arguments, started, ExecutionResult{Text: fmt.Sprintf("HTTP %d: resposta externa excede o limite de %d bytes", resp.StatusCode, policy.MaxResponseBytes), Status: "response_too_large", HTTPStatus: resp.StatusCode})
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		text = fmt.Sprintf("HTTP %d (sem corpo)", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return e.finish(ctx, def, policy, arguments, started, ExecutionResult{Text: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, text), Status: "http_error", HTTPStatus: resp.StatusCode})
	}
	return e.finish(ctx, def, policy, arguments, started, ExecutionResult{Text: text, Status: "success", HTTPStatus: resp.StatusCode})
}

func (e *Executor) finish(ctx context.Context, def store.Tool, policy Policy, arguments map[string]interface{}, started time.Time, result ExecutionResult) ExecutionResult {
	e.audit(ctx, AuditEvent{At: e.currentTime(), Tool: def.Name, Connector: policy.Connector, Risk: policy.Risk, Status: result.Status, HTTPStatus: result.HTTPStatus, DurationMilliseconds: e.currentTime().Sub(started).Milliseconds(), RequiredScopes: append([]string{}, policy.RequiredScopes...), MissingScopes: append([]string{}, result.MissingScopes...), ConfirmationRequired: policy.RequiresConfirmation, ArgumentNames: argumentNames(arguments)})
	return result
}

func (e *Executor) audit(ctx context.Context, event AuditEvent) {
	if e.Audit != nil {
		_ = e.Audit.RecordToolAudit(ctx, event)
	}
}

func (e *Executor) currentTime() time.Time {
	if e.now == nil {
		return time.Now()
	}
	return e.now()
}

func (e *Executor) limiterFor() *rateLimiter {
	if e.limiter == nil {
		e.limiter = &rateLimiter{calls: map[string][]time.Time{}}
	}
	return e.limiter
}

func (e *Executor) clientFor(policy Policy) *http.Client {
	base := e.HTTP
	if base == nil {
		base = http.DefaultClient
	}
	client := *base
	client.CheckRedirect = func(req *http.Request, _ []*http.Request) error {
		return validateEndpoint(req.URL.String(), req.Method, policy, e.AllowPrivateNetwork)
	}
	if !e.AllowPrivateNetwork {
		transport, ok := base.Transport.(*http.Transport)
		if !ok || transport == nil {
			transport = http.DefaultTransport.(*http.Transport)
		}
		clone := transport.Clone()
		clone.DialContext = safeDialContext
		client.Transport = clone
	}
	return &client
}

func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
			return nil, fmt.Errorf("endereço privado não permitido")
		}
	}
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	for _, ip := range ips {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		err = dialErr
	}
	return nil, err
}

func argumentNames(arguments map[string]interface{}) []string {
	result := make([]string, 0, len(arguments))
	for key := range arguments {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

type rateLimiter struct {
	mu    sync.Mutex
	calls map[string][]time.Time
}

func (r *rateLimiter) Allow(name string, policy Policy, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	values := r.calls[name]
	cutoff := now.Add(-time.Hour)
	kept := values[:0]
	for _, value := range values {
		if value.After(cutoff) {
			kept = append(kept, value)
		}
	}
	if len(kept) >= policy.MaxCallsPerHour {
		r.calls[name] = kept
		return false
	}
	minuteCutoff := now.Add(-time.Minute)
	minuteCalls := 0
	for _, value := range kept {
		if value.After(minuteCutoff) {
			minuteCalls++
		}
	}
	if minuteCalls >= policy.MaxCallsPerMinute {
		r.calls[name] = kept
		return false
	}
	r.calls[name] = append(kept, now)
	return true
}

func appendQuery(rawURL string, args map[string]interface{}) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	for key, value := range args {
		query.Set(key, fmt.Sprintf("%v", value))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
