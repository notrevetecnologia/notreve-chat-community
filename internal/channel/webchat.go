package channel

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

// WebChat Ã© o canal de chat no site (widget embutÃ­vel).
type WebChat struct {
	proc *Processor
	st   *store.Store
}

func NewWebChat(proc *Processor, st *store.Store) *WebChat {
	return &WebChat{proc: proc, st: st}
}

// WidgetConfig Ã© a configuraÃ§Ã£o exibÃ­vel/editÃ¡vel do widget.
type WidgetConfig struct {
	Titulo        string   `json:"titulo"`
	Subtitulo     string   `json:"subtitulo"`
	BoasVindas    string   `json:"boas_vindas"`
	Posicao       string   `json:"posicao"`      // left | right
	Apresentacao  string   `json:"apresentacao"` // barra | botao
	TituloBotao   string   `json:"titulo_botao"`
	LogoURL       string   `json:"logo_url"`
	CorPrimaria   string   `json:"cor_primaria"`
	CorBotao      string   `json:"cor_botao"`
	CorFundo      string   `json:"cor_fundo"`
	CorTexto      string   `json:"cor_texto"`
	CorTextoBotao string   `json:"cor_texto_botao"`
	IconeBotao    string   `json:"icone_botao"`
	TextoInput    string   `json:"texto_input"`
	TextoRodape   string   `json:"texto_rodape"`
	Largura       int      `json:"largura"`
	Dominios      []string `json:"dominios"` // origens autorizadas
}

func defaultWidgetConfig() WidgetConfig {
	return WidgetConfig{
		Titulo:        "Atendimento",
		Subtitulo:     "Estamos aqui para ajudar. Envie sua mensagem.",
		BoasVindas:    "OlÃ¡! ðŸ‘‹ Sou o assistente virtual. Como posso ajudar?",
		Posicao:       "right",
		Apresentacao:  "botao",
		TituloBotao:   "Atendimento",
		CorPrimaria:   "#22c55e",
		CorBotao:      "#22c55e",
		CorFundo:      "#f4f6f8",
		CorTexto:      "#ffffff",
		CorTextoBotao: "#ffffff",
		IconeBotao:    "💬",
		TextoInput:    "Digite sua mensagem...",
		TextoRodape:   "Atendimento",
		Largura:       360,
	}
}

func limitWidgetText(value, fallback string, max int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	runes := []rune(value)
	if len(runes) > max {
		return string(runes[:max])
	}
	return value
}

func validWidgetColor(value, fallback string) string {
	value = strings.TrimSpace(value)
	if len(value) != 7 || value[0] != '#' {
		return fallback
	}
	for _, r := range value[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return fallback
		}
	}
	return value
}

func validWidgetAssetURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	u, err := url.ParseRequestURI(value)
	if err != nil || u.Host == "" || u.User != nil {
		return ""
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(u.Path), "."))
	switch ext {
	case "png", "jpg", "jpeg", "webp", "gif", "svg", "ico", "avif", "bmp":
		return value
	default:
		return ""
	}
}

func normalizeWidgetConfig(cfg WidgetConfig) WidgetConfig {
	defaults := defaultWidgetConfig()
	cfg.Titulo = limitWidgetText(cfg.Titulo, defaults.Titulo, 80)
	cfg.Subtitulo = limitWidgetText(cfg.Subtitulo, defaults.Subtitulo, 140)
	cfg.BoasVindas = limitWidgetText(cfg.BoasVindas, defaults.BoasVindas, 500)
	cfg.TituloBotao = limitWidgetText(cfg.TituloBotao, defaults.TituloBotao, 40)
	cfg.LogoURL = validWidgetAssetURL(cfg.LogoURL)
	cfg.IconeBotao = limitWidgetText(cfg.IconeBotao, defaults.IconeBotao, 8)
	cfg.TextoInput = limitWidgetText(cfg.TextoInput, defaults.TextoInput, 80)
	cfg.TextoRodape = limitWidgetText(cfg.TextoRodape, defaults.TextoRodape, 80)
	cfg.CorPrimaria = validWidgetColor(cfg.CorPrimaria, defaults.CorPrimaria)
	cfg.CorBotao = validWidgetColor(cfg.CorBotao, defaults.CorBotao)
	cfg.CorFundo = validWidgetColor(cfg.CorFundo, defaults.CorFundo)
	cfg.CorTexto = validWidgetColor(cfg.CorTexto, defaults.CorTexto)
	cfg.CorTextoBotao = validWidgetColor(cfg.CorTextoBotao, defaults.CorTextoBotao)
	if cfg.Largura < 320 || cfg.Largura > 520 {
		cfg.Largura = defaults.Largura
	}
	if cfg.Posicao != "left" {
		cfg.Posicao = "right"
	}
	if cfg.Apresentacao != "barra" {
		cfg.Apresentacao = "botao"
	}
	return cfg
}

func (wc *WebChat) parseConfig(raw string) WidgetConfig {
	cfg := defaultWidgetConfig()
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	return normalizeWidgetConfig(cfg)
}

func (wc *WebChat) tenantBranding(ctx context.Context, tenantID int) (name, logo, primary, button, text string) {
	defaults := defaultWidgetConfig()
	settings, err := wc.st.GetSettings(store.WithTenantID(ctx, tenantID))
	if err != nil {
		return defaults.Titulo, "", defaults.CorPrimaria, defaults.CorBotao, defaults.CorTextoBotao
	}
	name = limitWidgetText(settings.BrandName, defaults.Titulo, 80)
	logo = validWidgetAssetURL(settings.LogoURL)
	primary = validWidgetColor(settings.PrimaryColor, defaults.CorPrimaria)
	button = validWidgetColor(settings.PrimaryColor, defaults.CorBotao)
	text = defaults.CorTextoBotao
	return name, logo, primary, button, text
}

func (wc *WebChat) parseConfigFor(ctx context.Context, ch store.Channel) WidgetConfig {
	cfg := wc.parseConfig(ch.WidgetConfig)
	brandName, brandLogo, brandPrimary, brandButton, brandText := wc.tenantBranding(ctx, ch.TenantID)
	return overlayWidgetBrand(cfg, brandName, brandLogo, brandPrimary, brandButton, brandText)
}

func overlayWidgetBrand(cfg WidgetConfig, brandName, brandLogo, brandPrimary, brandButton, brandText string) WidgetConfig {
	defaults := defaultWidgetConfig()
	if strings.TrimSpace(cfg.Titulo) == "" || strings.TrimSpace(cfg.Titulo) == defaults.Titulo {
		cfg.Titulo = strings.TrimSpace(brandName)
	}
	if strings.TrimSpace(cfg.TituloBotao) == "" || strings.TrimSpace(cfg.TituloBotao) == defaults.TituloBotao {
		cfg.TituloBotao = strings.TrimSpace(brandName)
	}
	if strings.TrimSpace(cfg.TextoRodape) == "" || strings.TrimSpace(cfg.TextoRodape) == defaults.TextoRodape {
		cfg.TextoRodape = strings.TrimSpace(brandName)
	}
	if strings.TrimSpace(cfg.LogoURL) == "" {
		cfg.LogoURL = brandLogo
	}
	if strings.TrimSpace(cfg.CorPrimaria) == "" || strings.EqualFold(cfg.CorPrimaria, defaults.CorPrimaria) {
		cfg.CorPrimaria = brandPrimary
	}
	if strings.TrimSpace(cfg.CorBotao) == "" || strings.EqualFold(cfg.CorBotao, defaults.CorBotao) {
		cfg.CorBotao = brandButton
	}
	if strings.TrimSpace(cfg.CorTextoBotao) == "" || strings.EqualFold(cfg.CorTextoBotao, defaults.CorTextoBotao) {
		cfg.CorTextoBotao = brandText
	}
	return cfg
}

// channelInfo valida o canal por token pÃºblico (public_id) com fallback por id.
func (wc *WebChat) channelInfo(ctx context.Context, key string) (store.Channel, bool) {
	if key != "" {
		if c, err := wc.st.ChannelByPublicIDRouted(ctx, key); err == nil {
			return c, true
		}
	}
	if id, err := strconv.Atoi(key); err == nil {
		chs, err2 := wc.st.ListChannels(ctx)
		if err2 != nil {
			return store.Channel{}, false
		}
		for _, c := range chs {
			if c.ID == id && c.Type == "webchat" && c.Active {
				return c, true
			}
		}
	}
	return store.Channel{}, false
}

func webchatTenantContext(ctx context.Context, ch store.Channel) context.Context {
	return store.WithTenantID(ctx, ch.TenantID)
}

type webchatMsg struct {
	VisitorID    string `json:"visitor_id"`
	SessionToken string `json:"session_token"`
	Name         string `json:"name"`
	Message      string `json:"message"`
	SiteOrigin   string `json:"site_origin"`
}

func normalizeVisitorName(value string) (string, bool) {
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	runes := []rune(value)
	if len(runes) < 2 || len(runes) > 80 {
		return "", false
	}
	for _, r := range runes {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsSpace(r) || r == '\'' || r == '-' || r == '.' {
			continue
		}
		return "", false
	}
	generic := strings.ToLower(value)
	if generic == "visitante" || generic == "anônimo" || generic == "anonimo" {
		return "", false
	}
	return value, true
}

func validVisitorID(value string) bool {
	if len(value) < 6 || len(value) > 80 {
		return false
	}
	for _, r := range value {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func webchatSessionHash(value string) (string, bool) {
	if len(value) != 43 {
		return "", false
	}
	for _, r := range value {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
			return "", false
		}
	}
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum), true
}

func normalizeSiteOrigin(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	u, err := url.Parse(value)
	if err != nil || u == nil {
		return ""
	}
	scheme := strings.ToLower(u.Scheme)
	if (scheme != "http" && scheme != "https") || u.Host == "" || u.User != nil {
		return ""
	}
	return scheme + "://" + strings.ToLower(u.Host)
}

// HandleConfig devolve a config do widget (pÃºblico, pro widget renderizar).
func (wc *WebChat) HandleConfig(w http.ResponseWriter, r *http.Request, key string) {
	w.Header().Set("Content-Type", "application/json")
	setWebChatNoCache(w)
	ch, ok := wc.channelInfo(r.Context(), key)
	if !ok {
		http.Error(w, `{"error":"canal nÃ£o encontrado"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(wc.parseConfigFor(r.Context(), ch))
}

func setWebChatNoCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

// HandleMessage processa uma mensagem do widget e devolve a resposta da IA.
func (wc *WebChat) HandleMessage(w http.ResponseWriter, r *http.Request, key string) {
	w.Header().Set("Content-Type", "application/json")
	ch, ok := wc.channelInfo(r.Context(), key)
	if !ok {
		http.Error(w, `{"error":"canal nÃ£o encontrado"}`, http.StatusNotFound)
		return
	}
	ctx := webchatTenantContext(r.Context(), ch)

	cfg := wc.parseConfigFor(ctx, ch)
	var in webchatMsg
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Message) == "" {
		http.Error(w, `{"error":"mensagem vazia"}`, http.StatusBadRequest)
		return
	}
	name, validName := normalizeVisitorName(in.Name)
	if !validName {
		http.Error(w, `{"error":"informe seu nome para iniciar o atendimento"}`, http.StatusBadRequest)
		return
	}
	visitorID := strings.TrimSpace(in.VisitorID)
	if !validVisitorID(visitorID) {
		http.Error(w, `{"error":"identificador do visitante inválido"}`, http.StatusBadRequest)
		return
	}
	sessionHash, validSession := webchatSessionHash(strings.TrimSpace(in.SessionToken))
	if !validSession {
		http.Error(w, `{"error":"sessão do webchat inválida"}`, http.StatusUnauthorized)
		return
	}
	siteOrigin := normalizeSiteOrigin(in.SiteOrigin)
	if len(cfg.Dominios) > 0 && (siteOrigin == "" || !originAllowed(siteOrigin, cfg.Dominios)) {
		http.Error(w, `{"error":"origem não autorizada"}`, http.StatusForbidden)
		return
	}
	allowedSession, err := wc.st.EnsureWebchatSessionForTenant(ctx, ch.TenantID, ch.ID, visitorID, sessionHash)
	if err != nil {
		http.Error(w, `{"error":"erro interno"}`, http.StatusInternalServerError)
		return
	}
	if !allowedSession {
		http.Error(w, `{"error":"sessão do webchat não autorizada"}`, http.StatusForbidden)
		return
	}
	phone := "web:" + visitorID
	notes := "Contato originado pelo Web Chat"
	if siteOrigin != "" {
		notes += " em " + siteOrigin
	}
	_ = wc.st.UpsertContato(ctx, store.Contato{Name: name, Phone: phone, Notes: notes})

	channelID := ch.ID
	res, err := wc.proc.Process(ctx, Inbound{
		Message:   strings.TrimSpace(in.Message),
		Phone:     phone,
		Name:      name,
		TenantID:  ch.TenantID,
		ChannelID: &channelID,
		Channel:   "webchat",
		Origem:    siteOrigin,
	})
	if err != nil {
		http.Error(w, `{"error":"erro interno"}`, http.StatusInternalServerError)
		return
	}
	if err := wc.st.BindWebchatSession(ctx, ch.ID, visitorID, sessionHash, res.ConversationID); err != nil {
		http.Error(w, `{"error":"não foi possível vincular a sessão ao atendimento"}`, http.StatusInternalServerError)
		return
	}
	lastMessageID, _ := wc.st.LatestAssistantMessageID(ctx, res.ConversationID)
	satisfaction := ""
	if conv, convErr := wc.st.GetConversation(ctx, res.ConversationID); convErr == nil {
		res.Status = conv.Status
		satisfaction = conv.Satisfacao
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"conversation_id": res.ConversationID,
		"status":          res.Status,
		"satisfaction":    satisfaction,
		"reply":           res.Reply,
		"last_message_id": lastMessageID,
	})
}

func (wc *WebChat) HandleEnd(w http.ResponseWriter, r *http.Request, key string) {
	w.Header().Set("Content-Type", "application/json")
	setWebChatNoCache(w)
	ch, ok := wc.channelInfo(r.Context(), key)
	if !ok {
		http.Error(w, `{"error":"canal não encontrado"}`, http.StatusNotFound)
		return
	}
	ctx := webchatTenantContext(r.Context(), ch)
	var in webchatMsg
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, `{"error":"json inválido"}`, http.StatusBadRequest)
		return
	}
	visitorID := strings.TrimSpace(in.VisitorID)
	if !validVisitorID(visitorID) {
		http.Error(w, `{"error":"identificador do visitante inválido"}`, http.StatusBadRequest)
		return
	}
	sessionHash, validSession := webchatSessionHash(strings.TrimSpace(in.SessionToken))
	if !validSession {
		http.Error(w, `{"error":"sessão do webchat inválida"}`, http.StatusUnauthorized)
		return
	}
	allowedSession, err := wc.st.ValidateWebchatSessionForTenant(ctx, ch.TenantID, ch.ID, visitorID, sessionHash)
	if err != nil {
		http.Error(w, `{"error":"erro interno"}`, http.StatusInternalServerError)
		return
	}
	if !allowedSession {
		http.Error(w, `{"error":"sessão do webchat não autorizada"}`, http.StatusForbidden)
		return
	}
	siteOrigin := normalizeSiteOrigin(in.SiteOrigin)
	cfg := wc.parseConfigFor(ctx, ch)
	if len(cfg.Dominios) > 0 && (siteOrigin == "" || !originAllowed(siteOrigin, cfg.Dominios)) {
		http.Error(w, `{"error":"origem não autorizada"}`, http.StatusForbidden)
		return
	}
	conv, err := wc.webchatConversation(ctx, ch.ID, visitorID, sessionHash)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, `{"error":"nenhum atendimento ativo"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"erro interno"}`, http.StatusInternalServerError)
		return
	}
	reply := ""
	if conv.Status != "closed" {
		closed, closeErr := wc.st.EncerrarConversationIfOpen(ctx, conv.ID, "Encerrado pelo cliente no Web Chat", nil)
		if closeErr != nil {
			http.Error(w, `{"error":"não foi possível encerrar o atendimento"}`, http.StatusInternalServerError)
			return
		}
		if closed {
			reply, err = BeginSatisfaction(ctx, wc.st, conv.ID)
			if err != nil {
				http.Error(w, `{"error":"não foi possível iniciar a avaliação"}`, http.StatusInternalServerError)
				return
			}
		}
	}
	conv, err = wc.st.GetConversation(ctx, conv.ID)
	if err != nil {
		http.Error(w, `{"error":"erro interno"}`, http.StatusInternalServerError)
		return
	}
	lastMessageID, _ := wc.st.LatestAssistantMessageID(ctx, conv.ID)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"conversation_id": conv.ID,
		"status":          conv.Status,
		"satisfaction":    conv.Satisfacao,
		"reply":           reply,
		"last_message_id": lastMessageID,
	})
}

func (wc *WebChat) HandleMessages(w http.ResponseWriter, r *http.Request, key string) {
	w.Header().Set("Content-Type", "application/json")
	setWebChatNoCache(w)
	ch, ok := wc.channelInfo(r.Context(), key)
	if !ok {
		http.Error(w, `{"error":"canal não encontrado"}`, http.StatusNotFound)
		return
	}
	ctx := webchatTenantContext(r.Context(), ch)
	visitorID := strings.TrimSpace(r.URL.Query().Get("visitor_id"))
	if !validVisitorID(visitorID) {
		http.Error(w, `{"error":"identificador do visitante inválido"}`, http.StatusBadRequest)
		return
	}
	sessionHash, validSession := webchatSessionHash(strings.TrimSpace(r.URL.Query().Get("session_token")))
	if !validSession {
		http.Error(w, `{"error":"sessão do webchat inválida"}`, http.StatusUnauthorized)
		return
	}
	allowedSession, err := wc.st.ValidateWebchatSessionForTenant(ctx, ch.TenantID, ch.ID, visitorID, sessionHash)
	if err != nil {
		http.Error(w, `{"error":"erro interno"}`, http.StatusInternalServerError)
		return
	}
	if !allowedSession {
		http.Error(w, `{"error":"sessão do webchat não autorizada"}`, http.StatusForbidden)
		return
	}
	siteOrigin := normalizeSiteOrigin(r.URL.Query().Get("site_origin"))
	cfg := wc.parseConfigFor(ctx, ch)
	if len(cfg.Dominios) > 0 && (siteOrigin == "" || !originAllowed(siteOrigin, cfg.Dominios)) {
		http.Error(w, `{"error":"origem não autorizada"}`, http.StatusForbidden)
		return
	}
	afterID, _ := strconv.Atoi(r.URL.Query().Get("after"))
	if afterID < 0 {
		afterID = 0
	}
	conv, err := wc.webchatConversation(ctx, ch.ID, visitorID, sessionHash)
	if errors.Is(err, pgx.ErrNoRows) {
		json.NewEncoder(w).Encode(map[string]interface{}{"messages": []interface{}{}, "status": "", "satisfaction": ""})
		return
	}
	if err != nil {
		http.Error(w, `{"error":"erro interno"}`, http.StatusInternalServerError)
		return
	}
	var messages []store.Message
	if r.URL.Query().Get("history") == "1" {
		messages, err = wc.st.ListWebchatHistory(ctx, conv.ID, 100)
	} else {
		messages, err = wc.st.ListAssistantMessagesAfter(ctx, conv.ID, afterID)
	}
	if err != nil {
		http.Error(w, `{"error":"erro interno"}`, http.StatusInternalServerError)
		return
	}
	type outbound struct {
		ID      int    `json:"id"`
		Content string `json:"content"`
		Role    string `json:"role"`
	}
	out := make([]outbound, 0, len(messages))
	for _, message := range messages {
		out = append(out, outbound{ID: message.ID, Content: message.Content, Role: message.Role})
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"messages": out, "status": conv.Status, "satisfaction": conv.Satisfacao})
}

func (wc *WebChat) webchatConversation(ctx context.Context, channelID int, visitorID, sessionHash string) (store.Conversation, error) {
	conversationID, err := wc.st.WebchatSessionConversation(ctx, channelID, visitorID, sessionHash)
	if err == nil {
		return wc.st.GetConversation(ctx, conversationID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return store.Conversation{}, err
	}
	// Compatibilidade com sessões criadas antes da migration 025 ou antes do
	// primeiro bind. Assim a atualização não perde atendimentos existentes.
	return wc.st.GetLatestConversationByPhoneChannel(ctx, "web:"+visitorID, "webchat")
}

func originOf(r *http.Request) string {
	if o := r.Header.Get("Origin"); o != "" {
		return o
	}
	if ref := r.Header.Get("Referer"); ref != "" {
		return ref
	}
	return ""
}

func originAllowed(origin string, dominios []string) bool {
	origin = normalizeSiteOrigin(origin)
	for _, d := range dominios {
		d = normalizeSiteOrigin(d)
		if d == "" {
			continue
		}
		if strings.EqualFold(origin, d) {
			return true
		}
		// A lista pode conter http ou https; o host precisa ser exatamente igual.
		ou, _ := url.Parse(origin)
		du, _ := url.Parse(d)
		if ou != nil && du != nil && strings.EqualFold(ou.Host, du.Host) {
			return true
		}
	}
	return false
}

// ServeWidget entrega o widget de chat (HTML autossuficiente).
func (wc *WebChat) ServeWidget(w http.ResponseWriter, r *http.Request, key string) {
	if _, ok := wc.channelInfo(r.Context(), key); !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	setWebChatNoCache(w)
	fmt.Fprint(w, strings.ReplaceAll(widgetHTML, "%s", key))
}

// ServeEmbed entrega o JS de embed: injeta o iframe do widget no site do cliente.
// Funciona com <script src=".../webchat/{key}/embed.js"></script>.
func (wc *WebChat) ServeEmbed(w http.ResponseWriter, r *http.Request, key string) {
	ch, ok := wc.channelInfo(r.Context(), key)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	setWebChatNoCache(w)
	ctx := webchatTenantContext(r.Context(), ch)
	cfg := wc.parseConfigFor(ctx, ch)
	position := "right:0;left:auto;"
	if cfg.Posicao == "left" {
		position = "left:0;right:auto;"
	}
	fmt.Fprintf(w, `(function(){var k=%q;var s=(document.currentScript&&document.currentScript.src)||'';var o=s.indexOf('://');var p=s.indexOf('/',o+3);var base=(o>0&&p>0)?s.slice(0,p):location.origin;var old=document.getElementById('notreve-widget');if(old)old.remove();var f=document.createElement('iframe');f.src=base+'/webchat/'+k+'?embed=v5&site='+encodeURIComponent(location.origin);f.setAttribute('title','Widget de atendimento');f.setAttribute('style','position:fixed;bottom:0;%s;width:min(420px,100vw);height:min(640px,100vh);border:0;z-index:99999;pointer-events:auto;background:transparent');f.setAttribute('allow','microphone');f.id='notreve-widget';document.body.appendChild(f);})();`, key, position)
}

const widgetHTML = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Atendimento</title>
<style>
* { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
html, body { width: 100%; height: 100%; overflow: hidden; background: transparent; }
button { touch-action: manipulation; }
:root { --wc-primary: #22c55e; --wc-button: #22c55e; --wc-button-text: #fff; --wc-bg: #f4f6f8; --wc-text: #fff; --wc-width: 360px; }
.wc-btn { position: fixed; bottom: 20px; right: 20px; width: 60px; height: 60px; border-radius: 50%; border: none; background: var(--wc-button); color: var(--wc-button-text); font-size: 24px; cursor: pointer; box-shadow: 0 6px 20px rgba(0,0,0,.25); z-index: 9999; pointer-events: auto; transition: transform .18s ease, box-shadow .18s ease; }
.wc-btn:hover, .wc-bar:hover { transform: translateY(-2px); box-shadow: 0 9px 24px rgba(0,0,0,.28); }
.wc-btn.left { right: auto; left: 20px; }
.wc-bar { position: fixed; bottom: 20px; right: 20px; border: none; background: var(--wc-button); color: var(--wc-button-text); padding: 14px 20px; border-radius: 999px; font-size: 15px; font-weight: 600; cursor: pointer; box-shadow: 0 6px 20px rgba(0,0,0,.25); z-index: 9999; pointer-events: auto; transition: transform .18s ease, box-shadow .18s ease; }
.wc-bar.left { right: auto; left: 20px; }
.wc-box { position: fixed; bottom: 92px; right: 20px; width: var(--wc-width); max-width: calc(100vw - 40px); height: 480px; max-height: calc(100vh - 120px); background: #fff; border-radius: 16px; box-shadow: 0 12px 40px rgba(0,0,0,.25); display: none; flex-direction: column; overflow: hidden; z-index: 9999; font-size: 14px; pointer-events: auto; }
.wc-box.left { right: auto; left: 20px; }
.wc-head { background: var(--wc-primary); color: var(--wc-text); padding: 14px 16px; display: flex; flex-direction: column; gap: 2px; }
.wc-head-top { display: flex; justify-content: space-between; align-items: center; gap: 10px; }
.wc-head-brand { display: flex; align-items: center; gap: 10px; min-width: 0; }
.wc-head-logo { display: none; height: 30px; max-width: 120px; object-fit: contain; }
.wc-head-title { min-width: 0; }
.wc-head-actions { display: flex; align-items: center; gap: 8px; }
.wc-head h3 { font-size: 15px; }
.wc-head p { font-size: 12px; opacity: .9; }
.wc-head button { background: none; border: none; color: var(--wc-text); font-size: 18px; cursor: pointer; }
.wc-head .wc-end { display: none; border: 1px solid color-mix(in srgb, var(--wc-text) 45%, transparent); border-radius: 999px; padding: 5px 9px; font-size: 11px; font-weight: 700; }
.wc-identify { flex: 1; padding: 28px 22px; background: var(--wc-bg); display: none; flex-direction: column; justify-content: center; gap: 12px; }
.wc-identify h4 { color: #172033; font-size: 18px; }
.wc-identify p { color: #667085; line-height: 1.5; }
.wc-identify input { width: 100%; padding: 11px 12px; border: 1px solid #d5dae1; border-radius: 10px; font-size: 15px; outline: none; }
.wc-identify input:focus { border-color: var(--wc-primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--wc-primary) 18%, transparent); }
.wc-identify button { padding: 11px 14px; border: none; border-radius: 10px; background: var(--wc-primary); color: var(--wc-text); font-weight: 700; cursor: pointer; }
.wc-identify-error { min-height: 18px; color: #b42318; font-size: 12px; }
.wc-msgs { flex: 1; overflow-y: auto; padding: 14px; display: flex; flex-direction: column; gap: 8px; background: var(--wc-bg); }
.wc-msg { max-width: 78%; padding: 9px 12px; border-radius: 12px; white-space: pre-wrap; word-break: break-word; }
.wc-in { align-self: flex-start; background: #fff; border: 1px solid #e5e8ec; border-bottom-left-radius: 4px; }
.wc-out { align-self: flex-end; background: var(--wc-primary); color: var(--wc-text); border-bottom-right-radius: 4px; }
.wc-input { display: flex; gap: 8px; padding: 10px; border-top: 1px solid #e5e8ec; }
.wc-input input { flex: 1; padding: 9px 12px; border: 1px solid #e5e8ec; border-radius: 10px; font-size: 14px; }
.wc-input button { padding: 9px 14px; border: none; border-radius: 10px; background: var(--wc-primary); color: var(--wc-text); font-weight: 600; cursor: pointer; }
.wc-new { display: none; padding: 12px; border-top: 1px solid #e5e8ec; background: #fff; }
.wc-new button { width: 100%; padding: 10px 14px; border: none; border-radius: 10px; background: var(--wc-primary); color: var(--wc-text); font-weight: 700; cursor: pointer; }
.wc-confirm { position: absolute; inset: 0; z-index: 5; display: none; align-items: center; justify-content: center; padding: 22px; background: rgba(17,24,39,.48); }
.wc-confirm-card { width: 100%; padding: 20px; border-radius: 14px; background: #fff; box-shadow: 0 18px 40px rgba(0,0,0,.2); color: #172033; }
.wc-confirm-card h4 { margin-bottom: 8px; font-size: 17px; }
.wc-confirm-card p { color: #667085; line-height: 1.45; }
.wc-confirm-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 18px; }
.wc-confirm-actions button { padding: 9px 12px; border: 1px solid #d5dae1; border-radius: 9px; background: #fff; color: #344054; font-weight: 700; cursor: pointer; }
.wc-confirm-actions .danger { border-color: #dc2626; background: #dc2626; color: #fff; }
.wc-foot { text-align: center; padding: 6px; font-size: 11px; color: #9aa4b0; background: #fff; border-top: 1px solid #eef0f3; }
@media (max-width: 520px) { .wc-box { right: 10px; left: 10px; width: auto; max-width: none; bottom: 84px; max-height: calc(100vh - 104px); } .wc-box.left { left: 10px; } .wc-btn, .wc-bar { right: 14px; bottom: 14px; } .wc-btn.left, .wc-bar.left { left: 14px; right: auto; } }
</style>
</head>
<body>
<button type="button" class="wc-btn" id="wc-btn" style="display:none" aria-label="Abrir atendimento">💬</button>
<button type="button" class="wc-bar" id="wc-bar" style="display:none"><span id="wc-bar-icon">💬</span> <span id="wc-bar-txt"></span></button>
<div class="wc-box" id="wc-box">
  <div class="wc-head">
    <div class="wc-head-top"><div class="wc-head-brand"><img class="wc-head-logo" id="wc-logo" alt="" /><div class="wc-head-title"><h3 id="wc-title"></h3><p id="wc-subtitle"></p></div></div><div class="wc-head-actions"><button type="button" class="wc-end" id="wc-end">Encerrar</button><button type="button" id="wc-close" aria-label="Fechar janela">×</button></div></div>
  </div>
  <div class="wc-identify" id="wc-identify">
    <h4>Antes de começarmos</h4>
    <p>Com quem estou falando?</p>
    <input id="wc-name" type="text" maxlength="80" placeholder="Digite seu nome" autocomplete="name">
    <div class="wc-identify-error" id="wc-name-error" role="alert"></div>
    <button type="button" id="wc-identify-btn">Continuar atendimento</button>
  </div>
  <div class="wc-msgs" id="wc-msgs"></div>
  <div class="wc-input">
    <input id="wc-input" placeholder="Digite sua mensagem..." autocomplete="off">
    <button type="button" id="wc-send">Enviar</button>
  </div>
  <div class="wc-new" id="wc-new"><button type="button" id="wc-new-btn">Iniciar novo atendimento</button></div>
  <div class="wc-foot" id="wc-foot"></div>
  <div class="wc-confirm" id="wc-end-confirm">
    <div class="wc-confirm-card"><h4>Encerrar atendimento?</h4><p>A conversa será finalizada e, se houve atendimento, você poderá fazer a avaliação.</p><div class="wc-confirm-actions"><button type="button" id="wc-end-cancel">Continuar conversa</button><button type="button" class="danger" id="wc-end-confirm-btn">Encerrar e avaliar</button></div></div>
  </div>
</div>
<script>
(function () {
  function cleanOrigin(value) { try { return value ? new URL(value).origin : ''; } catch (_) { return ''; } }
  var requestedSite = new URLSearchParams(location.search).get('site');
  var ancestorSite = (location.ancestorOrigins && location.ancestorOrigins.length) ? location.ancestorOrigins[0] : '';
  var siteOrigin = cleanOrigin(requestedSite || ancestorSite || document.referrer);
  var storageKey = '%s_' + (siteOrigin || 'direct');
  var vid = localStorage.getItem('wc_visitor_' + storageKey);
  if (!vid) { vid = Math.random().toString(36).slice(2, 12); localStorage.setItem('wc_visitor_' + storageKey, vid); }
  function createSessionToken() {
    var bytes = new Uint8Array(32);
    crypto.getRandomValues(bytes);
    var binary = '';
    bytes.forEach(function (value) { binary += String.fromCharCode(value); });
    return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  }
  var sessionToken = localStorage.getItem('wc_session_' + storageKey);
  if (!sessionToken || sessionToken.length !== 43) { sessionToken = createSessionToken(); localStorage.setItem('wc_session_' + storageKey, sessionToken); }
  var visitorName = localStorage.getItem('wc_name_' + storageKey) || '';
  var cfg = { titulo: 'Atendimento', subtitulo: 'Estamos aqui para ajudar. Envie sua mensagem.', boas_vindas: 'Olá! 👋 Sou o assistente virtual. Como posso ajudar?', apresentacao: 'botao', titulo_botao: 'Atendimento', logo_url: '', cor_primaria: '#22c55e', cor_botao: '#22c55e', cor_fundo: '#f4f6f8', cor_texto: '#ffffff', cor_texto_botao: '#ffffff', icone_botao: '💬', texto_input: 'Digite sua mensagem...', texto_rodape: 'Atendimento', largura: 360 };
  var box = document.getElementById('wc-box');
  var btn = document.getElementById('wc-btn');
  var bar = document.getElementById('wc-bar');
  var msgs = document.getElementById('wc-msgs');
  var input = document.getElementById('wc-input');
  var identify = document.getElementById('wc-identify');
  var nameInput = document.getElementById('wc-name');
  var opened = localStorage.getItem('wc_open_' + storageKey) === '1';
  var messageCursor = Number(localStorage.getItem('wc_cursor_' + storageKey) || 0);
  var pollTimer = null;
  var pollBusy = false;
  var conversationStatus = '';
  var satisfactionState = '';
  var startingNew = false;
  var ending = false;

  render();
  fetch('/webchat/%s/config').then(function (r) { if (!r.ok) throw new Error('config'); return r.json(); }).then(function (c) { cfg = c; render(); }).catch(function () { render(); });

  function render() {
    document.title = cfg.titulo || 'Atendimento';
    document.getElementById('wc-title').textContent = cfg.titulo || 'Atendimento';
    document.getElementById('wc-subtitle').textContent = cfg.subtitulo || '';
    document.getElementById('wc-bar-txt').textContent = cfg.titulo_botao || 'Atendimento';
    document.getElementById('wc-btn').textContent = cfg.icone_botao || '💬';
    document.getElementById('wc-bar-icon').textContent = cfg.icone_botao || '💬';
    document.getElementById('wc-foot').textContent = cfg.texto_rodape || 'Atendimento';
    var brandLogo = document.getElementById('wc-logo');
    if (cfg.logo_url) { brandLogo.src = cfg.logo_url; brandLogo.style.display = 'inline-block'; } else { brandLogo.removeAttribute('src'); brandLogo.style.display = 'none'; }
    input.placeholder = cfg.texto_input || 'Digite sua mensagem...';
    document.documentElement.style.setProperty('--wc-primary', cfg.cor_primaria || '#22c55e');
    document.documentElement.style.setProperty('--wc-button', cfg.cor_botao || '#22c55e');
    document.documentElement.style.setProperty('--wc-bg', cfg.cor_fundo || '#f4f6f8');
    document.documentElement.style.setProperty('--wc-text', cfg.cor_texto || '#ffffff');
    document.documentElement.style.setProperty('--wc-button-text', cfg.cor_texto_botao || '#ffffff');
    document.documentElement.style.setProperty('--wc-width', (Number(cfg.largura) || 360) + 'px');
    // The config is rendered once with defaults and again after the fetch. Hide
    // both presentation controls before selecting exactly one, otherwise the
    // fallback control can remain visible and create two overlapping buttons.
    btn.style.display = 'none';
    bar.style.display = 'none';
    if (cfg.posicao === 'left') { box.classList.add('left'); btn.classList.add('left'); bar.classList.add('left'); }
    if (cfg.apresentacao === 'barra') { bar.style.display = 'flex'; } else { btn.style.display = 'flex'; }
    applyIdentityState();
    if (opened) box.style.display = 'flex';
  }
  function applyIdentityState() {
    var identified = !!visitorName;
    identify.style.display = identified ? 'none' : 'flex';
    msgs.style.display = identified ? 'flex' : 'none';
    document.getElementById('wc-foot').style.display = identified ? 'block' : 'none';
    applyConversationState();
    if (identified) startPolling();
  }
  function applyConversationState(status, satisfaction) {
    if (typeof status === 'string') conversationStatus = status;
    if (typeof satisfaction === 'string') satisfactionState = satisfaction;
    var identified = !!visitorName;
    var closed = conversationStatus === 'closed';
    var awaitingRating = closed && (satisfactionState === 'aguardando' || satisfactionState === 'aguardando_operador');
    document.getElementById('wc-end').style.display = identified && conversationStatus && !closed ? 'inline-block' : 'none';
    document.querySelector('.wc-input').style.display = identified && (!closed || awaitingRating) ? 'flex' : 'none';
    document.getElementById('wc-new').style.display = identified && closed && !awaitingRating ? 'block' : 'none';
  }
  function add(text, isOut) {
    var d = document.createElement('div');
    d.className = 'wc-msg ' + (isOut ? 'wc-out' : 'wc-in');
    d.textContent = text;
    msgs.appendChild(d);
    msgs.scrollTop = msgs.scrollHeight;
  }
  function toggle() {
    opened = box.style.display !== 'flex';
    box.style.display = opened ? 'flex' : 'none';
    localStorage.setItem('wc_open_' + storageKey, opened ? '1' : '0');
    if (opened) setTimeout(function () { (visitorName ? input : nameInput).focus(); }, 50);
  }
  function setMessageCursor(id) {
    id = Number(id) || 0;
    if (id > messageCursor) {
      messageCursor = id;
      localStorage.setItem('wc_cursor_' + storageKey, String(messageCursor));
    }
  }
  function showWelcome() {
    if (cfg.boas_vindas && !msgs.children.length) add(cfg.boas_vindas, false);
  }
  function restoreHistory() {
    return fetch('/webchat/%s/messages?visitor_id=' + encodeURIComponent(vid) + '&session_token=' + encodeURIComponent(sessionToken) + '&history=1&site_origin=' + encodeURIComponent(siteOrigin))
      .then(function (r) { if (!r.ok) throw new Error('history'); return r.json(); })
      .then(function (d) {
        applyConversationState(d.status || '', d.satisfaction || '');
        var history = d.messages || [];
        msgs.innerHTML = '';
        history.forEach(function (message) {
          add(message.content, message.role === 'user');
          setMessageCursor(message.id);
        });
        if (!history.length) showWelcome();
      })
      .catch(function () { showWelcome(); });
  }
  function pollMessages() {
    if (!visitorName || pollBusy || startingNew) return;
    pollBusy = true;
    fetch('/webchat/%s/messages?visitor_id=' + encodeURIComponent(vid) + '&session_token=' + encodeURIComponent(sessionToken) + '&after=' + messageCursor + '&site_origin=' + encodeURIComponent(siteOrigin))
      .then(function (r) { if (!r.ok) throw new Error('poll'); return r.json(); })
      .then(function (d) {
        applyConversationState(d.status || '', d.satisfaction || '');
        (d.messages || []).forEach(function (message) {
          if (Number(message.id) > messageCursor) add(message.content, false);
          setMessageCursor(message.id);
        });
      })
      .catch(function () {})
      .finally(function () { pollBusy = false; });
  }
  function startPolling() {
    if (pollTimer) return;
    restoreHistory().finally(function () {
      pollMessages();
      pollTimer = setInterval(pollMessages, 3000);
    });
  }
  btn.addEventListener('click', toggle);
  bar.addEventListener('click', toggle);
  document.getElementById('wc-close').addEventListener('click', toggle);
  document.getElementById('wc-end').addEventListener('click', function () { document.getElementById('wc-end-confirm').style.display = 'flex'; });
  document.getElementById('wc-end-cancel').addEventListener('click', function () { document.getElementById('wc-end-confirm').style.display = 'none'; });
  document.getElementById('wc-end-confirm-btn').addEventListener('click', function () {
    if (ending) return;
    ending = true;
    var confirmButton = document.getElementById('wc-end-confirm-btn');
    confirmButton.disabled = true;
    confirmButton.textContent = 'Encerrando...';
    fetch('/webchat/%s/end', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ visitor_id: vid, session_token: sessionToken, site_origin: siteOrigin })
    }).then(function (r) { return r.json().then(function (d) { if (!r.ok) throw new Error(d.error || 'erro'); return d; }); })
      .then(function (d) {
        document.getElementById('wc-end-confirm').style.display = 'none';
        var replyID = Number(d.last_message_id) || 0;
        if (d.reply && (!replyID || replyID > messageCursor)) add(d.reply, false);
        if (d.reply) setMessageCursor(replyID);
        applyConversationState(d.status || 'closed', d.satisfaction || '');
      })
      .catch(function (e) { add(e.message || 'Não foi possível encerrar agora. Tente novamente.', false); })
      .finally(function () { ending = false; confirmButton.disabled = false; confirmButton.textContent = 'Encerrar e avaliar'; });
  });
  document.getElementById('wc-new-btn').addEventListener('click', function () {
    startingNew = true;
    conversationStatus = '';
    satisfactionState = '';
    msgs.innerHTML = '';
    showWelcome();
    applyConversationState('', '');
    input.focus();
  });
  function identifyVisitor() {
    var value = nameInput.value.trim().replace(/\s+/g, ' ');
    var error = document.getElementById('wc-name-error');
    if (value.length < 2) { error.textContent = 'Digite seu nome para continuar.'; nameInput.focus(); return; }
    visitorName = value;
    localStorage.setItem('wc_name_' + storageKey, visitorName);
    error.textContent = '';
    applyIdentityState();
    if (cfg.boas_vindas && !msgs.children.length) add(cfg.boas_vindas, false);
    input.focus();
  }
  document.getElementById('wc-identify-btn').addEventListener('click', identifyVisitor);
  nameInput.addEventListener('keydown', function (e) { if (e.key === 'Enter') identifyVisitor(); });
  function send() {
    var msg = input.value.trim();
    if (!msg || !visitorName) return;
    input.value = '';
    add(msg, true);
    fetch('/webchat/%s/message', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ visitor_id: vid, session_token: sessionToken, name: visitorName, message: msg, site_origin: siteOrigin })
    }).then(function (r) { return r.json().then(function (d) { if (!r.ok) throw new Error(d.error || 'erro'); return d; }); })
      .then(function (d) {
        startingNew = false;
        var replyID = Number(d.last_message_id) || 0;
        if (d.reply && (!replyID || replyID > messageCursor)) add(d.reply, false);
        if (d.reply) setMessageCursor(replyID);
        applyConversationState(d.status || '', d.satisfaction || '');
      })
      .catch(function (e) { add(e.message || 'Não foi possível responder agora. Tente novamente em instantes.', false); });
  }
  document.getElementById('wc-send').addEventListener('click', send);
  input.addEventListener('keydown', function (e) { if (e.key === 'Enter') send(); });
})();
</script>
</body>
</html>`
