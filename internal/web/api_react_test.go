package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestReactCursorRoundTrip(t *testing.T) {
	for _, offset := range []int{0, 1, 25, 100} {
		cursor := encodeReactCursor(offset)
		got, err := decodeReactCursor(cursor)
		if err != nil || got != offset {
			t.Fatalf("cursor %q: obtido=%d erro=%v esperado=%d", cursor, got, err, offset)
		}
	}
	if _, err := decodeReactCursor("not-a-cursor"); err == nil {
		t.Fatal("cursor inválido deveria retornar erro")
	}
}

func TestReactConversationFilters(t *testing.T) {
	conversation := store.Conversation{
		CustomerName: "Cliente exemplo", CustomerPhone: "0000000000000",
		Channel: "whatsapp", Origem: "checkout.exemplo.com", Status: "ia",
	}
	for _, query := range []string{"cliente", "000000", "whatsapp", "checkout", "ia", ""} {
		if !reactConversationMatches(conversation, query) {
			t.Fatalf("consulta %q deveria encontrar a conversa", query)
		}
	}
	if reactConversationMatches(conversation, "joão") {
		t.Fatal("consulta sem correspondência não deveria encontrar a conversa")
	}
}

func TestReactStatusAndLimitValidation(t *testing.T) {
	for _, status := range []string{"", "all", "ia", "humano_aguardando", "humano_atendendo", "closed"} {
		if !validReactConversationStatus(status) {
			t.Fatalf("status %q deveria ser aceito", status)
		}
	}
	if validReactConversationStatus("human") {
		t.Fatal("status legado inválido não deveria ser aceito")
	}
	if got, err := parseReactLimit(""); err != nil || got != 25 {
		t.Fatalf("limite padrão inválido: %d %v", got, err)
	}
	if _, err := parseReactLimit("101"); err == nil {
		t.Fatal("limite acima do máximo deveria retornar erro")
	}
}

func TestReactEffectivePermissions(t *testing.T) {
	user := &store.Usuario{Role: "atendente", Permissions: "conversas_ver, conversas_responder"}
	permissions := effectiveReactPermissions(user)
	if strings.Join(permissions, ",") != "conversas_responder,conversas_ver" {
		t.Fatalf("permissões efetivas inesperadas: %v", permissions)
	}
	superAdmin := effectiveReactPermissions(&store.Usuario{Role: "super_admin"})
	if len(superAdmin) != len(allPermsList()) {
		t.Fatalf("super_admin deveria receber todo o catálogo: %d/%d", len(superAdmin), len(allPermsList()))
	}
	if !userHasPerm(&store.Usuario{Role: "super_admin"}, permPlataformaGerenciar) {
		t.Fatal("super_admin deveria ter permissão de plataforma")
	}
	if userHasPerm(&store.Usuario{Role: "admin"}, permPlataformaGerenciar) {
		t.Fatal("admin não deveria ter permissão de plataforma")
	}
}

func TestReactActionPermissions(t *testing.T) {
	checks := map[string]string{
		"accept":       permConversasResponder,
		"aceitar":      permConversasResponder,
		"close":        permConversasEncerrar,
		"encerrar":     permConversasEncerrar,
		"reopen":       permConversasEncerrar,
		"reabrir":      permConversasEncerrar,
		"return_to_ai": permConversasEncerrar,
		"return-to-ai": permConversasEncerrar,
		"transfer":     permConversasTransferir,
		"transferir":   permConversasTransferir,
	}
	for action, expected := range checks {
		if permission, ok := reactActionPermission(action); !ok || permission != expected {
			t.Fatalf("ação %q: permissão=%q válida=%v esperada=%q", action, permission, ok, expected)
		}
	}
	if _, ok := reactActionPermission("send"); ok {
		t.Fatal("ação não suportada não deveria receber permissão")
	}
}

func TestReactTransferTargetsAreSafe(t *testing.T) {
	sector := reactTransferTargetFromSector(store.Setor{ID: 2, Name: "Comercial", Active: true})
	user := reactTransferTargetFromUser(store.Usuario{ID: 7, Name: "Usuário exemplo", Role: "admin", PasswordHash: "hash-secreto"})
	b, err := json.Marshal([]reactTransferTarget{sector, user})
	if err != nil {
		t.Fatal(err)
	}
	payload := string(b)
	if strings.Contains(payload, "hash-secreto") || strings.Contains(payload, "password") || strings.Contains(payload, "username") {
		t.Fatalf("destinos de transferência expuseram dados sensíveis: %s", payload)
	}
	if !strings.Contains(payload, `"kind":"sector"`) || !strings.Contains(payload, `"kind":"agent"`) {
		t.Fatalf("destinos de transferência incompletos: %s", payload)
	}
	if reactTransferTargetUserAllowed(store.Usuario{ID: 8, Name: "Inativo", Role: "admin", Active: false}) {
		t.Fatal("usuário inativo não deveria ser destino de transferência")
	}
	if reactTransferTargetUserAllowed(store.Usuario{ID: 9, Name: "Plataforma", Role: "super_admin", Active: true}) {
		t.Fatal("super_admin não deveria ser destino operacional")
	}
}

func TestReactMessageDoesNotExposeMediaPayload(t *testing.T) {
	message := reactMessageFromStore(store.Message{ID: 1, ConversationID: 2, Content: "arquivo", MediaB64: "segredo", MediaType: "document"})
	b, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "segredo") || strings.Contains(string(b), "media_b64") {
		t.Fatalf("payload de mídia não deveria sair na resposta: %s", b)
	}
	if !strings.Contains(string(b), `"media_url":"/api/mensagens/1/media"`) {
		t.Fatalf("payload de mídia deveria expor somente a URL segura: %s", b)
	}
}

func TestReactSendRequiresResponderPermission(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/react/conversations/1/messages", strings.NewReader(`{"content":"oi"}`))
	r.SetPathValue("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), ctxUser, &store.Usuario{Role: "atendente", Permissions: "conversas_ver"}))
	rr := httptest.NewRecorder()
	(&Server{}).handleReactSend(rr, r)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status obtido=%d esperado=%d", rr.Code, http.StatusForbidden)
	}
	var envelope reactEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error == nil || envelope.Error.Code != "permission_denied" {
		t.Fatalf("erro JSON inesperado: %+v", envelope.Error)
	}
}

func TestReactActionsRequireMappedPermissions(t *testing.T) {
	cases := []struct {
		action      string
		permissions string
	}{
		{action: "transfer", permissions: "conversas_responder"},
		{action: "close", permissions: "conversas_responder"},
		{action: "return_to_ai", permissions: "conversas_responder"},
	}
	for _, testCase := range cases {
		r := httptest.NewRequest(http.MethodPost, "/api/react/conversations/1/actions", strings.NewReader(`{"action":"`+testCase.action+`","setor_id":1}`))
		r.SetPathValue("id", "1")
		r = r.WithContext(context.WithValue(r.Context(), ctxUser, &store.Usuario{Role: "atendente", Permissions: testCase.permissions}))
		rr := httptest.NewRecorder()
		(&Server{}).handleReactAction(rr, r)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("ação %q: status obtido=%d esperado=%d", testCase.action, rr.Code, http.StatusForbidden)
		}
		var envelope reactEnvelope
		if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Error == nil || envelope.Error.Code != "permission_denied" {
			t.Fatalf("ação %q: erro JSON inesperado: %+v", testCase.action, envelope.Error)
		}
	}
}

func TestReactAuthJSONReturnsJSONUnauthorized(t *testing.T) {
	s := &Server{sessions: newSessionStore(nil, sessionTTL)}
	r := httptest.NewRequest(http.MethodGet, "/api/react/session", nil)
	rr := httptest.NewRecorder()
	s.authJSON(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler protegido não deveria ser chamado")
	})).ServeHTTP(rr, r)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status obtido=%d esperado=%d", rr.Code, http.StatusUnauthorized)
	}
	var envelope reactEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error == nil || envelope.Error.Code != "unauthorized" {
		t.Fatalf("erro JSON inesperado: %+v", envelope.Error)
	}
}

func TestReactPermissionMiddlewareReturnsJSON(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/react/conversations", nil)
	r = r.WithContext(context.WithValue(r.Context(), ctxUser, &store.Usuario{Role: "atendente", Permissions: ""}))
	rr := httptest.NewRecorder()
	(&Server{}).requireReactPerm(permEquipeGerenciar, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler sem permissão não deveria ser chamado")
	})).ServeHTTP(rr, r)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status obtido=%d esperado=%d", rr.Code, http.StatusForbidden)
	}
	var envelope reactEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error == nil || envelope.Error.Code != "permission_denied" {
		t.Fatalf("erro JSON inesperado: %+v", envelope.Error)
	}
}

func TestReactBrandingValidation(t *testing.T) {
	valid := reactBranding{
		Name: "Cliente Notreve", LogoURL: "https://cdn.example.com/logo.svg",
		FaviconURL: "", PrimaryColor: "#22c55e", SecondaryColor: "#16a34a",
		AccentColor: "#4f46e5", BackgroundColor: "#f4f6f8", LoadingColor: "#22c55e",
	}
	if err := validateReactBranding(valid); err != nil {
		t.Fatalf("branding válido rejeitado: %v", err)
	}
	invalidColor := valid
	invalidColor.PrimaryColor = "red"
	if err := validateReactBranding(invalidColor); err == nil {
		t.Fatal("cor fora do formato hexadecimal deveria ser rejeitada")
	}
	invalidURL := valid
	invalidURL.LogoURL = "javascript:alert(1)"
	if err := validateReactBranding(invalidURL); err == nil {
		t.Fatal("URL insegura deveria ser rejeitada")
	}
	dataURL := valid
	dataURL.FaviconURL = "data:image/svg+xml;base64,PHN2Zz48L3N2Zz4="
	if err := validateReactBranding(dataURL); err == nil {
		t.Fatal("data: URI deveria ser rejeitada")
	}
	noExtension := valid
	noExtension.LogoURL = "https://cdn.example.com/logo"
	if err := validateReactBranding(noExtension); err == nil {
		t.Fatal("URL sem extensão de imagem deveria ser rejeitada")
	}
	userInfo := valid
	userInfo.LogoURL = "https://user:pass@cdn.example.com/logo.svg"
	if err := validateReactBranding(userInfo); err == nil {
		t.Fatal("URL com credenciais deveria ser rejeitada")
	}
}

func TestReactBrandingInputPreservesOmittedFields(t *testing.T) {
	branding := reactBranding{
		Name: "Original", LogoURL: "https://example.com/logo.svg", FaviconURL: "",
		PrimaryColor: "#111111", SecondaryColor: "#222222", AccentColor: "#333333",
		BackgroundColor: "#444444", LoadingColor: "#555555",
	}
	name := "Atualizado"
	updated := applyReactBrandingInput(branding, reactBrandingInput{Name: &name})
	if updated.Name != name || updated.LogoURL != branding.LogoURL || updated.PrimaryColor != branding.PrimaryColor {
		t.Fatalf("campos omitidos não foram preservados: %+v", updated)
	}
}

func TestReactBrandingWithFallback(t *testing.T) {
	empty := reactBrandingWithFallback(store.Settings{})
	if empty.Name != reactBrandingDefaults.Name || empty.PrimaryColor != reactBrandingDefaults.PrimaryColor {
		t.Fatalf("fallback vazio não aplicou a marca Notreve: %+v", empty)
	}
	if empty.LogoURL != "" || empty.FaviconURL != "" {
		t.Fatalf("fallback não deveria inventar URLs: %+v", empty)
	}
	unsafe := reactBrandingWithFallback(store.Settings{
		BrandName: "<script>alert(1)</script>", LogoURL: "javascript:alert(1)",
		FaviconURL: "https://cdn.example.com/favicon.ico", PrimaryColor: "red;body{display:none}",
		SecondaryColor: "#16a34a", AccentColor: "#4f46e5", BackgroundColor: "#f4f6f8", LoadingColor: "#22c55e",
	})
	if unsafe.Name != "<script>alert(1)</script>" {
		t.Fatalf("nome deveria preservar o texto escapado pelo template: %q", unsafe.Name)
	}
	if unsafe.LogoURL != "" {
		t.Fatalf("URL insegura deveria ser descartada: %q", unsafe.LogoURL)
	}
	if unsafe.PrimaryColor != reactBrandingDefaults.PrimaryColor {
		t.Fatalf("cor insegura deveria cair no fallback: %q", unsafe.PrimaryColor)
	}
	if unsafe.FaviconURL != "https://cdn.example.com/favicon.ico" || unsafe.SecondaryColor != "#16a34a" {
		t.Fatalf("valores válidos deveriam ser preservados: %+v", unsafe)
	}
}

func TestReactBrandingSafeHelpers(t *testing.T) {
	if got := safeBrandName("   "); got != reactBrandingDefaults.Name {
		t.Fatalf("nome em branco deveria cair no fallback: %q", got)
	}
	if got := safeBrandColor("not-a-color", "#22c55e"); got != "#22c55e" {
		t.Fatalf("cor inválida deveria cair no fallback: %q", got)
	}
	if got := safeBrandURL("javascript:alert(1)"); got != "" {
		t.Fatalf("URL com javascript: deveria ser descartada: %q", got)
	}
	if got := safeBrandURL("https://cdn.example.com/logo.svg"); got != "https://cdn.example.com/logo.svg" {
		t.Fatalf("URL válida foi alterada: %q", got)
	}
}

func TestCanEditReactBranding(t *testing.T) {
	if !canEditReactBranding(&store.Usuario{Role: "super_admin"}) {
		t.Fatal("super_admin deveria poder configurar a marca")
	}
	for _, role := range []string{"admin", "supervisor", "atendente", ""} {
		if canEditReactBranding(&store.Usuario{Role: role}) {
			t.Fatalf("perfil %q não deveria poder configurar a marca", role)
		}
	}
}

func TestReactBrandingHandlerRequiresTenantAdmin(t *testing.T) {
	r := httptest.NewRequest(http.MethodPut, "/api/react/branding", strings.NewReader(`{"name":"Novo"}`))
	r = r.WithContext(context.WithValue(r.Context(), ctxUser, &store.Usuario{Role: "supervisor"}))
	r.Header.Set("Origin", "https://painel.example.com")
	r.Header.Set("Referer", "https://painel.example.com/config")
	r.Host = "painel.example.com"
	rr := httptest.NewRecorder()
	(&Server{}).handleReactBranding(rr, r)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status obtido=%d esperado=%d", rr.Code, http.StatusForbidden)
	}
	var envelope reactEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error == nil || envelope.Error.Code != "permission_denied" {
		t.Fatalf("erro JSON inesperado: %+v", envelope.Error)
	}
}
