package channel

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestWebchatTenantContextUsesResolvedChannel(t *testing.T) {
	ctx := webchatTenantContext(context.Background(), store.Channel{TenantID: 9})
	if got := store.TenantID(ctx); got != 9 {
		t.Fatalf("tenant do webchat = %d, esperado 9", got)
	}
}

func TestNormalizeVisitorName(t *testing.T) {
	valid := map[string]string{
		"  João   Silva ": "João Silva",
		"Ana-Maria":       "Ana-Maria",
		"D'Ávila":         "D'Ávila",
	}
	for input, want := range valid {
		got, ok := normalizeVisitorName(input)
		if !ok || got != want {
			t.Fatalf("nome %q: obtido %q, válido=%v; esperado %q", input, got, ok, want)
		}
	}
	for _, input := range []string{"", "A", "Visitante", "João <script>", "João\nAdministrador", "12345"} {
		if got, ok := normalizeVisitorName(input); ok {
			t.Fatalf("nome inválido %q foi aceito como %q", input, got)
		}
	}
}

func TestNormalizeSiteOrigin(t *testing.T) {
	if got := normalizeSiteOrigin("HTTPS://Docs.NotrevePay.com.br/alguma/pagina?q=1"); got != "https://docs.notrevepay.com.br" {
		t.Fatalf("origem inesperada: %q", got)
	}
	for _, input := range []string{"javascript:alert(1)", "//sem-esquema.example", "https://user:pass@example.com"} {
		if got := normalizeSiteOrigin(input); got != "" {
			t.Fatalf("origem inválida %q foi aceita como %q", input, got)
		}
	}
}

func TestOriginAllowedMatchesHostButNotSubdomain(t *testing.T) {
	allowed := []string{"https://docs.notrevepay.com.br"}
	if !originAllowed("http://docs.notrevepay.com.br", allowed) {
		t.Fatal("mesmo host deveria ser permitido")
	}
	if originAllowed("https://evil.docs.notrevepay.com.br", allowed) {
		t.Fatal("subdomínio diferente não deveria ser permitido")
	}
}

func TestNormalizeWidgetConfigUsesSafeDefaults(t *testing.T) {
	cfg := normalizeWidgetConfig(WidgetConfig{
		CorPrimaria: "red; background:url(https://example.invalid)",
		CorBotao:    "#123456",
		Largura:     900,
		Posicao:     "unknown",
	})
	if cfg.CorPrimaria != "#22c55e" {
		t.Fatalf("cor primaria insegura não foi substituída: %q", cfg.CorPrimaria)
	}
	if cfg.CorBotao != "#123456" || cfg.Largura != 360 || cfg.Posicao != "right" {
		t.Fatalf("configuração normalizada inesperada: %+v", cfg)
	}
}

func TestNormalizeWidgetConfigPreservesValidCustomization(t *testing.T) {
	cfg := normalizeWidgetConfig(WidgetConfig{
		Titulo:        "  Suporte  ",
		CorPrimaria:   "#123ABC",
		CorBotao:      "#101010",
		CorFundo:      "#fefefe",
		CorTexto:      "#eeeeee",
		CorTextoBotao: "#000000",
		IconeBotao:    "🛟",
		TextoInput:    "Fale com a equipe",
		TextoRodape:   "Notreve",
		Largura:       420,
		Posicao:       "left",
		Apresentacao:  "barra",
	})
	if cfg.Titulo != "Suporte" || cfg.CorPrimaria != "#123ABC" || cfg.Largura != 420 || cfg.Posicao != "left" || cfg.Apresentacao != "barra" {
		t.Fatalf("configuração válida não foi preservada: %+v", cfg)
	}
}

func TestSetWebChatNoCache(t *testing.T) {
	recorder := httptest.NewRecorder()
	setWebChatNoCache(recorder)

	if got := recorder.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate, max-age=0" {
		t.Fatalf("Cache-Control inesperado: %q", got)
	}
	if got := recorder.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("Pragma inesperado: %q", got)
	}
}

func TestWebchatSessionHashRequiresStrongToken(t *testing.T) {
	token := strings.Repeat("a", 43)
	hash, ok := webchatSessionHash(token)
	if !ok || len(hash) != 64 {
		t.Fatalf("token forte rejeitado: hash=%q válido=%v", hash, ok)
	}
	for _, input := range []string{"", "curto", strings.Repeat("a", 42), strings.Repeat("a", 42) + "!"} {
		if _, ok := webchatSessionHash(input); ok {
			t.Fatalf("token inválido foi aceito: %q", input)
		}
	}
}

func TestOverlayWidgetBrandUsesFallbackAndPreservesCustom(t *testing.T) {
	cfg := normalizeWidgetConfig(WidgetConfig{
		Titulo:        "Suporte",
		CorPrimaria:   "#123ABC",
		CorBotao:      "#101010",
		CorTextoBotao: "#000000",
	})
	got := overlayWidgetBrand(cfg, "Cliente Acme", "", "#0eaa64", "#0eaa64", "#ffffff")
	if got.Titulo != "Suporte" || got.CorPrimaria != "#123ABC" || got.CorBotao != "#101010" || got.CorTextoBotao != "#000000" {
		t.Fatalf("configuração customizada não foi preservada: %+v", got)
	}
	if got.LogoURL != "" {
		t.Fatalf("logo não definido no cfg não deveria ser inventado: %q", got.LogoURL)
	}
	base := normalizeWidgetConfig(WidgetConfig{})
	branded := overlayWidgetBrand(base, "Cliente Acme", "https://cdn.example.com/logo.png", "#0eaa64", "#0eaa64", "#ffffff")
	if branded.Titulo != "Cliente Acme" || branded.CorPrimaria != "#0eaa64" || branded.CorBotao != "#0eaa64" || branded.LogoURL != "https://cdn.example.com/logo.png" {
		t.Fatalf("fallback da marca não foi aplicado: %+v", branded)
	}
	if branded.TextoRodape != "Cliente Acme" {
		t.Fatalf("rodapé sem marca customizada deveria usar o nome da empresa: %+v", branded)
	}
}

func TestWidgetRestoresPersistedHistory(t *testing.T) {
	for _, expected := range []string{"history=1", "wc_session_", "wc_cursor_", "setInterval(pollMessages, 3000)"} {
		if !strings.Contains(widgetHTML, expected) {
			t.Fatalf("widget sem persistência esperada: %q", expected)
		}
	}
}

func TestWidgetSupportsEndingAndStartingAgain(t *testing.T) {
	for _, expected := range []string{"wc-end-confirm", "/webchat/%s/end", "Encerrar e avaliar", "Iniciar novo atendimento", "applyConversationState"} {
		if !strings.Contains(widgetHTML, expected) {
			t.Fatalf("widget sem fluxo de encerramento esperado: %q", expected)
		}
	}
}

func TestWidgetRendersBrandMarkAndLogo(t *testing.T) {
	for _, expected := range []string{"wc-logo", "wc-head-brand", "logo_url", "wc-foot"} {
		if !strings.Contains(widgetHTML, expected) {
			t.Fatalf("widget sem elemento de marca esperado: %q", expected)
		}
	}
}
