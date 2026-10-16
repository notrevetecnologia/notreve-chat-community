package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	channelapi "github.com/notrevetecnologia/notreve-chat-community/internal/channel"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type reactChannel struct {
	ID                int                    `json:"id"`
	Type              string                 `json:"type"`
	Name              string                 `json:"name"`
	Active            bool                   `json:"active"`
	RespondGrupos     bool                   `json:"respond_grupos"`
	ResponderChamadas bool                   `json:"responder_chamadas"`
	CanCampaign       bool                   `json:"can_campaign"`
	Widget            map[string]interface{} `json:"widget,omitempty"`
	PublicID          string                 `json:"public_id,omitempty"`
	TokenSet          bool                   `json:"token_set,omitempty"`
	HealthStatus      string                 `json:"health_status,omitempty"`
	LastCheckedAt     *string                `json:"last_checked_at,omitempty"`
	LastError         string                 `json:"last_error,omitempty"`
	AgentID           *int                   `json:"agent_id,omitempty"`
	AgentName         string                 `json:"agent_name,omitempty"`
	Mode              string                 `json:"mode"`
	ChatflowID        *int                   `json:"chatflow_id,omitempty"`
	ChatflowName      string                 `json:"chatflow_name,omitempty"`
}

type reactChannelInput struct {
	Type              *string          `json:"type"`
	Name              *string          `json:"name"`
	Active            *bool            `json:"active"`
	RespondGrupos     *bool            `json:"respond_grupos"`
	ResponderChamadas *bool            `json:"responder_chamadas"`
	CanCampaign       *bool            `json:"can_campaign"`
	Widget            *json.RawMessage `json:"widget"`
	Token             *string          `json:"token"`
	Action            *string          `json:"action"`
	ChannelID         *int             `json:"channel_id"`
	AgentID           *int             `json:"agent_id"`
	Mode              *string          `json:"mode"`
	ChatflowID        *int             `json:"chatflow_id"`
}

func reactChannelFromStore(channel store.Channel) reactChannel {
	widget := map[string]interface{}{}
	if strings.TrimSpace(channel.WidgetConfig) != "" {
		_ = json.Unmarshal([]byte(channel.WidgetConfig), &widget)
	}
	if channel.Type != "webchat" {
		widget = nil
	}
	var lastCheckedAt *string
	if channel.LastCheckedAt != nil {
		formatted := channel.LastCheckedAt.Format(time.RFC3339)
		lastCheckedAt = &formatted
	}
	mode := channel.Mode
	if mode != "chatflow" {
		mode = "ai"
	}
	return reactChannel{
		ID: channel.ID, Type: channel.Type, Name: channel.Name, Active: channel.Active,
		RespondGrupos: channel.RespondGrupos, ResponderChamadas: channel.ResponderChamadas,
		CanCampaign: channel.CanCampaign, Widget: widget, PublicID: channel.PublicID,
		TokenSet: channel.Token != "", HealthStatus: channel.HealthStatus,
		LastCheckedAt: lastCheckedAt, LastError: channel.LastError, AgentID: channel.AgentID,
		Mode: mode, ChatflowID: channel.ChatflowID,
	}
}

func validateReactChannelType(value string) bool {
	return value == "whatsapp" || value == "webchat" || value == "telegram"
}

func validateReactChannelName(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || len([]rune(value)) > 120 {
		return errors.New("nome deve ter entre 1 e 120 caracteres")
	}
	return nil
}

func parseReactChannelWidget(raw *json.RawMessage) (string, error) {
	if raw == nil || len(*raw) == 0 || string(*raw) == "null" {
		return "{}", nil
	}
	if len(*raw) > 64*1024 {
		return "", errors.New("personalização do webchat excede o limite permitido")
	}
	var widget map[string]interface{}
	if err := json.Unmarshal(*raw, &widget); err != nil || widget == nil {
		return "", errors.New("personalização do webchat inválida")
	}
	encoded, err := json.Marshal(widget)
	if err != nil {
		return "", errors.New("não foi possível salvar a personalização do webchat")
	}
	return string(encoded), nil
}

func decodeReactChannelInput(r *http.Request) (reactChannelInput, error) {
	var input reactChannelInput
	decoder := json.NewDecoder(io.LimitReader(r.Body, 96*1024+1))
	if err := decoder.Decode(&input); err != nil {
		return input, errors.New("JSON inválido")
	}
	return input, nil
}

func (s *Server) handleReactChannels(w http.ResponseWriter, r *http.Request) {
	page, err := parseReactPage(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	channels, err := s.st.ListChannelsSecure(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os canais")
		return
	}
	agentNames, err := s.reactAgentNamesByID(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os canais")
		return
	}
	chatflowNames, err := s.reactChatflowNamesByID(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "não foi possível carregar os canais")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	channelType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	filtered := make([]reactChannel, 0, len(channels))
	for _, channel := range channels {
		view := reactChannelFromStore(channel)
		if channel.AgentID != nil {
			view.AgentName = agentNames[*channel.AgentID]
		}
		if channel.ChatflowID != nil {
			view.ChatflowName = chatflowNames[*channel.ChatflowID]
		}
		if channelType != "" && strings.ToLower(channel.Type) != channelType {
			continue
		}
		if status == "ativos" && !channel.Active {
			continue
		}
		if status == "inativos" && channel.Active {
			continue
		}
		if !reactContainsFold([]string{channel.Name, channel.Type, view.AgentName}, query) {
			continue
		}
		filtered = append(filtered, view)
	}
	pageItems, err := reactPageSlice(filtered, page.Limit, page.Offset)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_cursor", err.Error())
		return
	}
	meta := reactPageMeta(len(filtered), page.Limit, page.Offset, reactQueryFilter(map[string]string{"q": query, "type": channelType, "status": status}))
	writeReactResponse(w, http.StatusOK, pageItems, meta)
}

func (s *Server) reactAgentNamesByID(ctx context.Context) (map[int]string, error) {
	agents, err := s.st.ListAgents(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[int]string, len(agents))
	for _, agent := range agents {
		names[agent.ID] = agent.Name
	}
	return names, nil
}

// validateReactChannelAgent confere que o agente escolhido pertence à empresa
// antes de gravar o vínculo canal -> agente.
func (s *Server) validateReactChannelAgent(ctx context.Context, agentID *int) error {
	if agentID == nil || *agentID <= 0 {
		return nil
	}
	if _, err := s.st.GetAgentForTenant(ctx, store.TenantID(ctx), *agentID); err != nil {
		return errors.New("agente não pertence a esta empresa")
	}
	return nil
}

func validateReactChannelMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "ai":
		return "ai", nil
	case "chatflow":
		return "chatflow", nil
	default:
		return "", errors.New("modo de atendimento inválido")
	}
}

// validateReactChannelChatflow confere que o chat flow escolhido pertence à empresa.
func (s *Server) validateReactChannelChatflow(ctx context.Context, chatflowID *int) error {
	if chatflowID == nil || *chatflowID <= 0 {
		return nil
	}
	if _, err := s.st.GetChatflow(ctx, *chatflowID); err != nil {
		return errors.New("chat flow não pertence a esta empresa")
	}
	return nil
}

// applyReactChannelMode resolve o modo de atendimento do canal (IA livre ou
// chat flow) a partir da entrada, validando o vínculo com o tenant.
func (s *Server) applyReactChannelMode(ctx context.Context, channel *store.Channel, input reactChannelInput) error {
	if input.Mode != nil {
		mode, err := validateReactChannelMode(*input.Mode)
		if err != nil {
			return err
		}
		channel.Mode = mode
	}
	if input.ChatflowID != nil {
		if *input.ChatflowID > 0 {
			if err := s.validateReactChannelChatflow(ctx, input.ChatflowID); err != nil {
				return err
			}
			channel.ChatflowID = input.ChatflowID
		} else {
			channel.ChatflowID = nil
		}
	}
	if channel.Mode == "chatflow" {
		if channel.ChatflowID == nil {
			return errors.New("selecione o chat flow deste canal")
		}
	} else {
		channel.ChatflowID = nil
	}
	return nil
}

func (s *Server) reactChatflowNamesByID(ctx context.Context) (map[int]string, error) {
	flows, err := s.st.ListChatflows(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[int]string, len(flows))
	for _, flow := range flows {
		names[flow.ID] = flow.Nome
	}
	return names, nil
}

func (s *Server) handleReactChannelCreate(w http.ResponseWriter, r *http.Request) {
	input, err := decodeReactChannelInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if input.Action != nil && strings.TrimSpace(*input.Action) != "" {
		s.handleReactChannelAction(w, r, input)
		return
	}
	typeValue := ""
	if input.Type != nil {
		typeValue = strings.ToLower(strings.TrimSpace(*input.Type))
	}
	if !validateReactChannelType(typeValue) {
		writeReactError(w, http.StatusBadRequest, "invalid_type", "tipo de canal inválido")
		return
	}
	name := ""
	if input.Name != nil {
		name = strings.TrimSpace(*input.Name)
	}
	if err := validateReactChannelName(name); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_name", err.Error())
		return
	}
	if typeValue == "telegram" && (input.Token == nil || strings.TrimSpace(*input.Token) == "") {
		writeReactError(w, http.StatusBadRequest, "missing_token", "informe o token do bot do Telegram")
		return
	}
	ctx := r.Context()
	if limits, err := s.st.GetLimites(ctx); err == nil {
		if usage, _ := s.st.CountCanais(ctx); !s.limiteCheck(ctx, currentUser(r), usage, limits.MaxCanais) {
			writeReactError(w, http.StatusConflict, "channel_limit_reached", "limite de canais do plano atingido")
			return
		}
	}
	widgetConfig, err := parseReactChannelWidget(input.Widget)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_widget", err.Error())
		return
	}
	active := true
	if input.Active != nil {
		active = *input.Active
	}
	responderChamadas := true
	if input.ResponderChamadas != nil {
		responderChamadas = *input.ResponderChamadas
	}
	channel := store.Channel{
		Type: typeValue, Name: name, Active: active, ResponderChamadas: responderChamadas,
		WidgetConfig: widgetConfig,
	}
	if typeValue == "webchat" || typeValue == "telegram" {
		channel.PublicID = newPublicID()
	}
	if typeValue == "telegram" {
		channel.Token = strings.TrimSpace(*input.Token)
	}
	if input.RespondGrupos != nil {
		channel.RespondGrupos = *input.RespondGrupos
	}
	if input.CanCampaign != nil {
		channel.CanCampaign = *input.CanCampaign
	}
	if typeValue == "whatsapp" {
		channel.Active = true
	}
	if input.AgentID != nil && *input.AgentID > 0 {
		if err := s.validateReactChannelAgent(ctx, input.AgentID); err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_agent", err.Error())
			return
		}
		channel.AgentID = input.AgentID
	}
	if err := s.applyReactChannelMode(ctx, &channel, input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_mode", err.Error())
		return
	}
	id, err := s.st.CreateChannelSecure(ctx, channel)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível criar o canal")
		return
	}
	if typeValue == "whatsapp" {
		if err := s.provisionReactWhatsAppChannel(ctx, id); err != nil {
			_ = s.st.DeleteChannel(ctx, id)
			if errors.Is(err, store.ErrWhatsAppServerUnavailable) {
				writeReactError(w, http.StatusConflict, "whatsapp_infrastructure_unavailable", "nenhum servidor WhatsApp com capacidade está disponível; fale com o administrador da plataforma")
				return
			}
			if errors.Is(err, store.ErrTenantWhatsAppLimitReached) {
				writeReactError(w, http.StatusConflict, "whatsapp_connection_limit_reached", "o limite de conexões WhatsApp do plano foi atingido")
				return
			}
			writeReactError(w, http.StatusInternalServerError, "whatsapp_provision_failed", "não foi possível preparar a infraestrutura do WhatsApp")
			return
		}
	}
	created, err := s.st.GetChannelByIDSecure(ctx, id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "canal criado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusCreated, reactChannelFromStore(created), nil)
}

func (s *Server) provisionReactWhatsAppChannel(ctx context.Context, channelID int) error {
	tenantID, err := store.TenantIDRequired(ctx)
	if err != nil {
		return err
	}
	if _, err := s.st.AllocateWhatsAppServer(ctx, tenantID, channelID); err != nil {
		return err
	}
	credentials, err := s.st.WhatsAppServerCredentialsForChannelSecure(ctx, tenantID, channelID)
	if err != nil {
		return err
	}
	channel, err := s.st.GetChannelByIDSecure(ctx, channelID)
	if err != nil {
		return err
	}
	channel.BaseURL = credentials.BaseURL
	channel.Token = credentials.Token
	return s.st.UpdateChannelSecure(ctx, channel)
}

func (s *Server) handleReactChannelUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	ctx := r.Context()
	channel, err := s.st.GetChannelByIDSecure(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "canal não encontrado")
		return
	}
	input, err := decodeReactChannelInput(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if input.Type != nil && strings.TrimSpace(*input.Type) != "" && strings.ToLower(strings.TrimSpace(*input.Type)) != channel.Type {
		writeReactError(w, http.StatusBadRequest, "type_immutable", "o tipo do canal não pode ser alterado")
		return
	}
	if input.Name != nil {
		channel.Name = strings.TrimSpace(*input.Name)
	}
	if err := validateReactChannelName(channel.Name); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_name", err.Error())
		return
	}
	if input.Active != nil {
		channel.Active = *input.Active
	}
	if input.RespondGrupos != nil {
		channel.RespondGrupos = *input.RespondGrupos
	}
	if input.ResponderChamadas != nil {
		channel.ResponderChamadas = *input.ResponderChamadas
	}
	if input.CanCampaign != nil {
		channel.CanCampaign = *input.CanCampaign
	}
	if input.AgentID != nil {
		if err := s.validateReactChannelAgent(ctx, input.AgentID); err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_agent", err.Error())
			return
		}
		if *input.AgentID > 0 {
			channel.AgentID = input.AgentID
		} else {
			channel.AgentID = nil
		}
	}
	if input.Widget != nil && channel.Type == "webchat" {
		channel.WidgetConfig, err = parseReactChannelWidget(input.Widget)
		if err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_widget", err.Error())
			return
		}
	}
	if input.Token != nil && channel.Type == "telegram" {
		token := strings.TrimSpace(*input.Token)
		if token == "" {
			writeReactError(w, http.StatusBadRequest, "missing_token", "informe o token do bot do Telegram")
			return
		}
		channel.Token = token
	}
	if channel.Type == "telegram" && strings.TrimSpace(channel.PublicID) == "" {
		channel.PublicID = newPublicID()
	}
	if err := s.applyReactChannelMode(ctx, &channel, input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_mode", err.Error())
		return
	}
	if err := s.st.UpdateChannelSecure(ctx, channel); err != nil {
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível atualizar o canal")
		return
	}
	updated, err := s.st.GetChannelByIDSecure(ctx, id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "internal_error", "canal atualizado, mas não foi possível carregá-lo")
		return
	}
	view := reactChannelFromStore(updated)
	if updated.AgentID != nil {
		names, nameErr := s.reactAgentNamesByID(ctx)
		if nameErr == nil {
			view.AgentName = names[*updated.AgentID]
		}
	}
	if updated.ChatflowID != nil {
		names, nameErr := s.reactChatflowNamesByID(ctx)
		if nameErr == nil {
			view.ChatflowName = names[*updated.ChatflowID]
		}
	}
	writeReactResponse(w, http.StatusOK, view, nil)
}

func disconnectReactChannel(ctx context.Context, channel store.Channel) error {
	switch channel.Type {
	case "whatsapp":
		base := strings.TrimRight(strings.TrimSpace(channel.BaseURL), "/")
		if base == "" || strings.TrimSpace(channel.Token) == "" {
			return nil
		}
		return channelapi.InstanciaLogout(ctx, base, channel.Token)
	case "telegram":
		if strings.TrimSpace(channel.Token) == "" {
			return nil
		}
		return channelapi.TelegramDeleteWebhook(ctx, channel.Token)
	default:
		return nil
	}
}

func (s *Server) syncReactChannelHealth(ctx context.Context, channelID int, status, lastError string) {
	if err := s.st.SetChannelHealth(ctx, channelID, status, lastError); err != nil {
		log.Printf("canal %d: não foi possível atualizar a saúde para %q: %v", channelID, status, err)
	}
}

func removeReactChannel(ctx context.Context, channel store.Channel, disconnect func(context.Context, store.Channel) error, remove func(context.Context, int) error) error {
	if disconnect != nil {
		if err := disconnect(ctx, channel); err != nil {
			log.Printf("canal %d (%s): falha ao desconectar antes de excluir: %v", channel.ID, channel.Type, err)
		}
	}
	if remove == nil {
		return nil
	}
	return remove(ctx, channel.ID)
}

func (s *Server) handleReactChannelDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactConversationID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	ctx := r.Context()
	channel, err := s.st.GetChannelByIDSecure(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "canal não encontrado")
		return
	}
	if err := removeReactChannel(ctx, channel, disconnectReactChannel, s.st.DeleteChannel); err != nil {
		writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir o canal")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"deleted": id}, nil)
}

func telegramWebhookURLForRequest(r *http.Request, publicID string) (string, error) {
	host := strings.TrimSpace(r.Host)
	if host == "" || strings.ContainsAny(host, "/\\\r\n\t ") {
		return "", errors.New("host público inválido")
	}
	publicID = strings.TrimSpace(publicID)
	if publicID == "" || strings.ContainsAny(publicID, "/\\?#") {
		return "", errors.New("identificador público do canal inválido")
	}
	proto := "https"
	forwarded := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]))
	if forwarded == "http" || forwarded == "https" {
		proto = forwarded
	} else if r.TLS == nil && (strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") || strings.HasPrefix(host, "[::1]")) {
		proto = "http"
	}
	return fmt.Sprintf("%s://%s/telegram/webhook/%s", proto, host, publicID), nil
}

func (s *Server) handleReactChannelAction(w http.ResponseWriter, r *http.Request, input reactChannelInput) {
	if input.ChannelID == nil || *input.ChannelID < 1 {
		writeReactError(w, http.StatusBadRequest, "missing_channel", "informe o canal")
		return
	}
	id := *input.ChannelID
	channel, err := s.st.GetChannelByIDSecure(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeReactError(w, status, "not_found", "canal não encontrado")
		return
	}
	action := strings.ToLower(strings.TrimSpace(*input.Action))
	if action == "test" && channel.Type == "webchat" {
		writeReactResponse(w, http.StatusOK, map[string]interface{}{
			"action":  action,
			"channel": reactChannelFromStore(channel),
			"result":  map[string]interface{}{"ok": channel.Active, "ready": channel.Active, "type": channel.Type},
		}, nil)
		return
	}
	if channel.Type == "telegram" {
		if strings.TrimSpace(channel.PublicID) == "" {
			channel.PublicID = newPublicID()
			if err := s.st.UpdateChannelSecure(r.Context(), channel); err != nil {
				writeReactError(w, http.StatusInternalServerError, "telegram_prepare_failed", "não foi possível preparar o identificador público do Telegram")
				return
			}
		}
		expectedURL, urlErr := telegramWebhookURLForRequest(r, channel.PublicID)
		switch action {
		case "connect", "conectar":
			if urlErr != nil {
				writeReactError(w, http.StatusBadRequest, "telegram_webhook_url_invalid", urlErr.Error())
				return
			}
			currentURL, infoErr := channelapi.TelegramWebhookInfo(r.Context(), channel.Token)
			if infoErr == nil && currentURL == expectedURL {
				s.syncReactChannelHealth(r.Context(), channel.ID, "connected", "")
				writeReactResponse(w, http.StatusOK, map[string]interface{}{
					"action": "connect", "channel": reactChannelFromStore(channel),
					"result": map[string]interface{}{"ok": true, "already": true, "ready": true, "connected": true, "webhook_set": true, "type": channel.Type},
				}, nil)
				return
			}
			if err := channelapi.TelegramSetWebhook(r.Context(), channel.Token, expectedURL); err != nil {
				writeReactError(w, http.StatusBadGateway, "telegram_connect_failed", "não foi possível configurar o webhook do Telegram")
				return
			}
			s.syncReactChannelHealth(r.Context(), channel.ID, "connected", "")
			writeReactResponse(w, http.StatusOK, map[string]interface{}{
				"action": "connect", "channel": reactChannelFromStore(channel),
				"result": map[string]interface{}{"ok": true, "ready": true, "connected": true, "webhook_set": true, "type": channel.Type},
			}, nil)
			return
		case "disconnect", "desconectar", "logout":
			currentURL, infoErr := channelapi.TelegramWebhookInfo(r.Context(), channel.Token)
			if infoErr == nil && strings.TrimSpace(currentURL) == "" {
				s.syncReactChannelHealth(r.Context(), channel.ID, "disconnected", "")
				writeReactResponse(w, http.StatusOK, map[string]interface{}{
					"action": "disconnect", "channel": reactChannelFromStore(channel),
					"result": map[string]interface{}{"ok": true, "already": true, "ready": false, "connected": false, "webhook_set": false, "type": channel.Type},
				}, nil)
				return
			}
			if err := channelapi.TelegramDeleteWebhook(r.Context(), channel.Token); err != nil {
				writeReactError(w, http.StatusBadGateway, "telegram_disconnect_failed", "não foi possível remover o webhook do Telegram")
				return
			}
			s.syncReactChannelHealth(r.Context(), channel.ID, "disconnected", "")
			writeReactResponse(w, http.StatusOK, map[string]interface{}{
				"action": "disconnect", "channel": reactChannelFromStore(channel),
				"result": map[string]interface{}{"ok": true, "ready": false, "connected": false, "webhook_set": false, "type": channel.Type},
			}, nil)
			return
		case "test", "status":
			webhookURL, infoErr := channelapi.TelegramWebhookInfo(r.Context(), channel.Token)
			configured := infoErr == nil && webhookURL != "" && urlErr == nil && webhookURL == expectedURL
			webhookError := ""
			if infoErr != nil {
				webhookError = infoErr.Error()
			} else if webhookURL != "" && urlErr == nil && webhookURL != expectedURL {
				webhookError = "webhook aponta para uma URL diferente da aplicação"
			}
			writeReactResponse(w, http.StatusOK, map[string]interface{}{
				"action": "status", "channel": reactChannelFromStore(channel),
				"result": map[string]interface{}{
					"ok": channel.Active && configured, "ready": configured, "connected": configured,
					"loggedIn": configured, "webhook_set": configured, "type": channel.Type, "error": webhookError,
				},
			}, nil)
			return
		default:
			writeReactError(w, http.StatusBadRequest, "invalid_action", "ação de Telegram inválida")
			return
		}
	}
	if action == "test" {
		action = "status"
	}
	if channel.Type != "whatsapp" {
		writeReactError(w, http.StatusBadRequest, "unsupported_action", "esta ação não está disponível para este canal")
		return
	}
	var handler http.HandlerFunc
	switch action {
	case "status":
		handler = s.handleCanalStatusAPI
	case "connect", "conectar":
		handler = s.handleCanalConectar
		action = "connect"
	case "disconnect", "desconectar", "logout":
		handler = s.handleCanalDesconectar
	default:
		writeReactError(w, http.StatusBadRequest, "invalid_action", "ação de canal inválida")
		return
	}
	operationRequest := r.Clone(r.Context())
	operationRequest.SetPathValue("id", strconv.Itoa(id))
	operationRequest.Method = http.MethodPost
	if action == "disconnect" || action == "logout" {
		operationRequest.Body = io.NopCloser(strings.NewReader("action=" + action))
		operationRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	result, status, message := s.callReactLegacyRequest(operationRequest, handler)
	if status < 200 || status >= 300 {
		writeReactError(w, status, "channel_action_failed", message)
		return
	}
	if object, ok := result.(map[string]interface{}); ok {
		if failure, ok := object["error"].(string); ok && strings.TrimSpace(failure) != "" {
			writeReactError(w, http.StatusBadGateway, "channel_action_failed", failure)
			return
		}
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{
		"action": action, "channel": reactChannelFromStore(channel), "result": result,
	}, nil)
}
