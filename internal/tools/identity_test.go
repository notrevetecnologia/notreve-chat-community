package tools

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestNormalizeIdentityPhoneKeepsOnlyDigits(t *testing.T) {
	cases := map[string]string{
		"5511999999999":     "5511999999999",
		"+55 11 99999-9999": "5511999999999",
		"(11) 99999-9999":   "11999999999",
		"web:abc":           "",
		"telegram:123456":   "123456",
		"":                  "",
		"  *+#":             "",
		"abc":               "",
	}
	for input, expected := range cases {
		if got := normalizeIdentityPhone(input); got != expected {
			t.Fatalf("normalizeIdentityPhone(%q) = %q, esperado %q", input, got, expected)
		}
	}
}

func TestIdentityAllowedMatchesNormalizedAllowed(t *testing.T) {
	if !identityAllowed("5511999999999", []string{"55 11 99999-9999"}) {
		t.Fatal("telefone permitido deveria ser aceito")
	}
	if !identityAllowed("+55 11 99999-9999", []string{"5511999999999"}) {
		t.Fatal("identidade com formatação deveria casar com lista de dígitos")
	}
	if identityAllowed("5511888888888", []string{"5511999999999"}) {
		t.Fatal("telefone fora da lista deveria ser rejeitado")
	}
	if identityAllowed("", []string{"5511999999999"}) {
		t.Fatal("identidade vazia deveria ser rejeitada")
	}
	if identityAllowed("web:abc", []string{"5511999999999"}) {
		t.Fatal("canal não-numérico deveria ser rejeitado")
	}
	if identityAllowed("5511999999999", nil) {
		t.Fatal("lista vazia deveria negar qualquer identidade")
	}
}

func TestToolIdentityPhoneRoundTrip(t *testing.T) {
	ctx := context.Background()
	if got := ToolIdentityPhone(ctx); got != "" {
		t.Fatalf("identidade ausente deveria ser vazia, veio %q", got)
	}
	ctx = WithToolIdentityPhone(ctx, "+55 11 99999-9999")
	if got := ToolIdentityPhone(ctx); got != "+55 11 99999-9999" {
		t.Fatalf("identidade não preservada: %q", got)
	}
}

func TestMaskedMatch(t *testing.T) {
	cases := []struct {
		got, expected string
		want          bool
	}{
		{"29/07/1990", "29/07/19**", true},
		{"29/07/2020", "29/07/19**", false},
		{"123", "123", true},
		{"123", "1*3", true},
		{"129", "1*3", false},
		{"123", "123***", true},
		{"124", "123***", false},
		{"", "123", false},
		{"A.", "*", false},
	}
	for _, c := range cases {
		if got := maskedMatch(c.got, c.expected); got != c.want {
			t.Fatalf("maskedMatch(%q, %q) = %v, esperado %v", c.got, c.expected, got, c.want)
		}
	}
}

func TestResolveJSONField(t *testing.T) {
	body := []byte(`{"data":{"id":42,"cpf":"123.456.789-00"},"items":[{"key":"a"},{"key":"b"}]}`)
	if got := resolveJSONField(body, "data.id"); got != "42" {
		t.Fatalf("resolveJSONField data.id = %q, esperado \"42\"", got)
	}
	if got := resolveJSONField(body, "data.cpf"); got != "123.456.789-00" {
		t.Fatalf("resolveJSONField data.cpf = %q", got)
	}
	if got := resolveJSONField(body, "items.1.key"); got != "b" {
		t.Fatalf("resolveJSONField items.1.key = %q, esperado \"b\"", got)
	}
	if got := resolveJSONField(body, "data.missing"); got != "" {
		t.Fatalf("resolveJSONField data.missing = %q, esperado vazio", got)
	}
	if got := resolveJSONField([]byte(`not json`), "data"); got != "" {
		t.Fatalf("resolveJSONField JSON inválido deveria retornar vazio, veio %q", got)
	}
	if got := resolveJSONField([]byte(`{"flag":true}`), "flag"); got != "" {
		t.Fatalf("resolveJSONField bool deveria retornar vazio, veio %q", got)
	}
}

func TestIdentitySettingsFromTool(t *testing.T) {
	def := store.Tool{
		RequiresIdentity:      "challenge",
		IdentityVerifyURL:     "https://example.com/verify",
		IdentityVerifyMethod:  "get",
		IdentityVerifyParam:   "cpf",
		IdentityMatchField:    "data.cpf",
		IdentityClientIDField: "data.id",
		IdentityConfig:        `{"challenges":[{"field":"birthdate","label":"Data de nascimento"}]}`,
		AllowedIdentities:     []string{"555"},
	}
	settings := IdentitySettingsFromTool(def)
	if settings.RequiresIdentity != "challenge" || settings.VerifyURL != "https://example.com/verify" ||
		settings.VerifyMethod != "GET" || settings.VerifyParam != "cpf" ||
		settings.MatchField != "data.cpf" || settings.ClientIDField != "data.id" {
		t.Fatalf("settings incorretas: %#v", settings)
	}
	if len(settings.Challenges) != 1 || settings.Challenges[0].Field != "birthdate" || settings.Challenges[0].Label != "Data de nascimento" {
		t.Fatalf("challenges não carregados: %#v", settings.Challenges)
	}
	if len(settings.AllowedIdentities) != 1 || settings.AllowedIdentities[0] != "555" {
		t.Fatalf("identidades permitidas não preservadas: %#v", settings.AllowedIdentities)
	}
}

func TestIdentitySettingsFromToolDefaultsMethod(t *testing.T) {
	def := store.Tool{RequiresIdentity: "resolve", IdentityVerifyURL: "https://example.com"}
	if got := IdentitySettingsFromTool(def).VerifyMethod; got != http.MethodPost {
		t.Fatalf("VerifyMethod default deveria ser POST, veio %q", got)
	}
}

func TestWithoutReservedArgsStripsIdentityKeys(t *testing.T) {
	args := map[string]interface{}{"cpf": "123", "_identity": "123", "_challenge_answers": map[string]interface{}{"birthdate": "10/10/1990"}}
	cleaned := withoutReservedArgs(args)
	if _, ok := cleaned["_identity"]; ok {
		t.Fatal("_identity não deveria ir para a requisição")
	}
	if _, ok := cleaned["_challenge_answers"]; ok {
		t.Fatal("_challenge_answers não deveria ir para a requisição")
	}
	if cleaned["cpf"] != "123" {
		t.Fatalf("chave normal perdida: %#v", cleaned)
	}
}

func TestExecutorIdentityPhoneAllowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	executor := NewExecutor()
	executor.AllowPrivateNetwork = true
	definition := store.Tool{Name: "consulta", Method: "GET", URL: server.URL, RequiresIdentity: "phone", AllowedIdentities: []string{"5511999999999"}}
	ctx := WithToolIdentityPhone(context.Background(), "+55 11 99999-9999")
	if result := executor.ExecuteDetailed(ctx, definition, nil); result.Status != "success" {
		t.Fatalf("telefone permitido deveria executar: %#v", result)
	}
	ctxBlocked := WithToolIdentityPhone(context.Background(), "5511888888888")
	if result := executor.ExecuteDetailed(ctxBlocked, definition, nil); result.Status != "blocked_identity" {
		t.Fatalf("telefone fora da lista deveria bloquear: %#v", result)
	}
}

func TestExecutorIdentityResolveWithFakeVerifier(t *testing.T) {
	var lastBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			data, _ := io.ReadAll(r.Body)
			lastBody = string(data)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"cpf":"123.456.789-00","id":"42"}}`))
	}))
	defer server.Close()
	executor := NewExecutor()
	executor.AllowPrivateNetwork = true
	definition := store.Tool{
		Name: "consulta", Method: "GET", URL: server.URL,
		RequiresIdentity:      "resolve",
		IdentityVerifyURL:     server.URL,
		IdentityVerifyParam:   "cpf",
		IdentityMatchField:    "data.cpf",
		IdentityClientIDField: "data.id",
	}
	args := map[string]interface{}{"cpf": "123.456.789-00", "_challenge_answers": map[string]interface{}{"birthdate": "10/10/1990"}, "_identity": "123.456.789-00"}
	result := executor.ExecuteDetailed(context.Background(), definition, args)
	if result.Status != "success" {
		t.Fatalf("resolução com verifier deveria autorizar: %#v", result)
	}
	if args["client_id"] != "42" {
		t.Fatalf("client_id não injetado: %#v", args)
	}
	if strings.Contains(lastBody, "birthdate") || strings.Contains(lastBody, "_challenge_answers") {
		t.Fatalf("campos reservados vazaram para o corpo: %q", lastBody)
	}
	if !strings.Contains(lastBody, "123.456.789-00") {
		t.Fatalf("identificador não enviado ao verifier: %q", lastBody)
	}
}

func TestExecutorIdentityResolveBlocksWhenConfigMissing(t *testing.T) {
	executor := NewExecutor()
	definition := store.Tool{Name: "consulta", Method: "GET", URL: "https://example.com", RequiresIdentity: "resolve"}
	result := executor.ExecuteDetailed(context.Background(), definition, map[string]interface{}{"cpf": "123"})
	if result.Status != "blocked_identity" {
		t.Fatalf("resolve sem verify_url deveria bloquear: %#v", result)
	}
}

func TestExecutorIdentityChallengeWithFakeVerifier(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"birthdate":"29/07/19**","cpf_first3":"123*","id":"77"}`))
	}))
	defer server.Close()
	executor := NewExecutor()
	executor.AllowPrivateNetwork = true
	definition := store.Tool{
		Name: "consulta", Method: "GET", URL: server.URL,
		RequiresIdentity:      "challenge",
		IdentityVerifyURL:     server.URL,
		IdentityVerifyMethod:  "POST",
		IdentityClientIDField: "id",
		IdentityConfig:        `{"challenges":[{"field":"birthdate","label":"Data de nascimento"},{"field":"cpf_first3","label":"3 primeiros do CPF"},{"field":"mother","label":"Nome da mãe"}]}`,
	}
	args := map[string]interface{}{"_challenge_answers": map[string]interface{}{"birthdate": "29/07/1990", "cpf_first3": "123"}}
	ctx := WithToolIdentityPhone(context.Background(), "5511999999999")
	result := executor.ExecuteDetailed(ctx, definition, args)
	if result.Status != "success" {
		t.Fatalf("desafio com respostas corretas deveria autorizar: %#v", result)
	}
	if args["client_id"] != "77" {
		t.Fatalf("client_id do desafio não injetado: %#v", args)
	}
}

func TestExecutorIdentityChallengeRejectsWrongAnswers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"birthdate":"29/07/19**","id":"77"}`))
	}))
	defer server.Close()
	executor := NewExecutor()
	executor.AllowPrivateNetwork = true
	definition := store.Tool{
		Name: "consulta", Method: "GET", URL: server.URL,
		RequiresIdentity:      "challenge",
		IdentityVerifyURL:     server.URL,
		IdentityClientIDField: "id",
		IdentityConfig:        `{"challenges":[{"field":"birthdate","label":"Data de nascimento"}]}`,
	}
	args := map[string]interface{}{"_challenge_answers": map[string]interface{}{"birthdate": "15/03/1998"}}
	result := executor.ExecuteDetailed(context.Background(), definition, args)
	if result.Status != "blocked_identity" {
		t.Fatalf("desafio com resposta incorreta deveria bloquear: %#v", result)
	}
}
