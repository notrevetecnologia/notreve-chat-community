package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/channel"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"github.com/notrevetecnologia/notreve-chat-community/internal/webhooks"
)

const (
	publicAPIMaxJSONBytes       = 64 << 10
	publicAPIMaxTextRunes       = 16000
	publicAPIMaxCustomerIDRunes = 160
	publicAPIMaxNameRunes       = 80
	publicAPIMaxOriginBytes     = 2048
	publicAPIMaxReasonRunes     = 240
)

type PublicAPI struct {
	st     *store.Store
	sender channel.Sender
	state  publicAPIState
}

type publicOpenConversationInput struct {
	ChannelID    int    `json:"channel_id"`
	Channel      string `json:"channel"`
	CustomerID   string `json:"customer_id"`
	Identifier   string `json:"identifier"`
	Phone        string `json:"phone"`
	CustomerName string `json:"customer_name"`
	Name         string `json:"name"`
	Origin       string `json:"origin"`
}

type publicTextInput struct {
	Content string `json:"content"`
}

type publicCloseInput struct {
	Reason string `json:"reason"`
}

type publicConversation struct {
	ID           int       `json:"id"`
	Hash         string    `json:"hash"`
	Channel      string    `json:"channel"`
	ChannelName  string    `json:"channel_name,omitempty"`
	CustomerID   string    `json:"customer_id"`
	CustomerName string    `json:"customer_name,omitempty"`
	Origin       string    `json:"origin,omitempty"`
	Status       string    `json:"status"`
	Satisfaction string    `json:"satisfaction,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type publicOpenConversationData struct {
	Conversation publicConversation `json:"conversation"`
	Reused       bool               `json:"reused"`
}

type publicMessageData struct {
	Conversation publicConversation `json:"conversation"`
	Delivered    bool               `json:"delivered"`
}

type publicCloseData struct {
	Conversation          publicConversation `json:"conversation"`
	Closed                bool               `json:"closed"`
	SatisfactionStatus    string             `json:"satisfaction_status,omitempty"`
	SatisfactionDelivered bool               `json:"satisfaction_delivered"`
}

type publicJSONEnvelope struct {
	Data interface{} `json:"data,omitempty"`
	Meta interface{} `json:"meta,omitempty"`
}

type publicErrorEnvelope struct {
	Error publicAPIError `json:"error"`
}

type publicAPIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type publicOperationResult struct {
	status int
	body   interface{}
}

func NewPublicAPI(st *store.Store, sender channel.Sender) *PublicAPI {
	return &PublicAPI{st: st, sender: sender, state: newPublicAPIState()}
}

// enqueueWebhook desacopla a resposta da API do transporte externo. Depois de
// inserido, o evento é entregue pelo worker com tentativas e lease; um erro na
// outbox é registrado para investigação, mas não desfaz uma operação já
// concluída com sucesso para o cliente da API.
func (api *PublicAPI) enqueueWebhook(ctx context.Context, event webhooks.Event, eventKey string, payload interface{}) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		Logger.Printf("webhook outbox: payload %s inválido: %v", event, err)
		return
	}
	if _, err := api.st.EnqueueWebhookOutboxEvent(ctx, store.WebhookOutboxEventInput{
		EventKey: eventKey, EventType: event, Payload: encoded, OccurredAt: time.Now().UTC(),
	}); err != nil {
		Logger.Printf("webhook outbox: evento %s não enfileirado: %v", event, err)
	}
}

func (api *PublicAPI) HandlePublicConversation(w http.ResponseWriter, r *http.Request) {
	ctx, _, ok := api.authorize(w, r, "conversations:read")
	if !ok {
		return
	}
	id, err := parsePublicConversationID(r)
	if err != nil {
		writePublicError(w, http.StatusBadRequest, "invalid_conversation", err.Error())
		return
	}
	conversation, channelInfo, result := api.ownedConversation(ctx, id)
	if result != nil {
		writePublicResult(w, *result)
		return
	}
	writePublicJSON(w, http.StatusOK, publicJSONEnvelope{Data: map[string]interface{}{
		"conversation": publicConversationFromStore(channelInfo, conversation),
	}})
}

func (api *PublicAPI) HandlePublicConversationOpen(w http.ResponseWriter, r *http.Request) {
	ctx, apiKey, ok := api.authorize(w, r, "conversations:write")
	if !ok {
		return
	}
	raw, input, result := decodePublicJSON[publicOpenConversationInput](w, r)
	if result != nil {
		writePublicResult(w, *result)
		return
	}
	normalized, result := normalizePublicOpenInput(input)
	if result != nil {
		writePublicResult(w, *result)
		return
	}
	channelInfo, result := api.resolvePublicChannel(ctx, normalized)
	if result != nil {
		writePublicResult(w, *result)
		return
	}
	normalized.Channel = channelInfo.Type
	if channelInfo.Type == "webchat" && !strings.HasPrefix(normalized.CustomerID, "web:") {
		normalized.CustomerID = "web:" + normalized.CustomerID
	}
	api.runPublicMutation(w, r, strconv.FormatInt(apiKey.ID, 10)+":conversation.open:"+strconv.Itoa(channelInfo.ID), publicFingerprint("conversation.open", normalized), raw, func() publicOperationResult {
		channelID := channelInfo.ID
		conversationID, reused, err := api.st.FindOrCreateConversationForChannel(ctx, store.Conversation{
			ChannelID:     &channelID,
			Channel:       channelInfo.Type,
			CustomerName:  normalized.CustomerName,
			CustomerPhone: normalized.CustomerID,
			Status:        "ia",
			Origem:        normalized.Origin,
		})
		if err != nil {
			return publicOperationResult{status: http.StatusInternalServerError, body: publicErrorEnvelope{Error: publicAPIError{Code: "conversation_open_failed", Message: "não foi possível abrir a conversa"}}}
		}
		conversation, err := api.st.GetConversation(ctx, conversationID)
		if err != nil {
			return publicOperationResult{status: http.StatusInternalServerError, body: publicErrorEnvelope{Error: publicAPIError{Code: "conversation_load_failed", Message: "conversa aberta, mas não foi possível carregá-la"}}}
		}
		publicConversation := publicConversationFromStore(channelInfo, conversation)
		if !reused {
			api.enqueueWebhook(ctx, webhooks.EventConversationCreated, "conversation.created:"+strconv.Itoa(conversation.ID), map[string]interface{}{
				"conversation": publicConversation,
			})
		}
		return publicOperationResult{status: http.StatusOK, body: publicJSONEnvelope{Data: publicOpenConversationData{
			Conversation: publicConversation,
			Reused:       reused,
		}}}
	})
}

func (api *PublicAPI) HandlePublicMessage(w http.ResponseWriter, r *http.Request) {
	ctx, apiKey, ok := api.authorize(w, r, "conversations:write")
	if !ok {
		return
	}
	id, err := parsePublicConversationID(r)
	if err != nil {
		writePublicError(w, http.StatusBadRequest, "invalid_conversation", err.Error())
		return
	}
	raw, input, result := decodePublicJSON[publicTextInput](w, r)
	if result != nil {
		writePublicResult(w, *result)
		return
	}
	input.Content = strings.TrimSpace(input.Content)
	if input.Content == "" {
		writePublicError(w, http.StatusBadRequest, "empty_content", "conteúdo é obrigatório")
		return
	}
	if len([]rune(input.Content)) > publicAPIMaxTextRunes {
		writePublicError(w, http.StatusRequestEntityTooLarge, "content_too_large", "conteúdo excede o limite de 16.000 caracteres")
		return
	}
	api.runPublicMutation(w, r, strconv.FormatInt(apiKey.ID, 10)+":conversation.message:"+strconv.Itoa(id), publicFingerprint("conversation.message", input), raw, func() publicOperationResult {
		conversation, channelInfo, ownedResult := api.ownedConversation(ctx, id)
		if ownedResult != nil {
			return *ownedResult
		}
		if conversation.Status == "closed" {
			return publicOperationResult{status: http.StatusConflict, body: publicErrorEnvelope{Error: publicAPIError{Code: "conversation_closed", Message: "a conversa está encerrada; abra uma nova conversa para enviar mensagens"}}}
		}
		if !channelInfo.Active {
			return publicOperationResult{status: http.StatusForbidden, body: publicErrorEnvelope{Error: publicAPIError{Code: "channel_inactive", Message: "canal inativo"}}}
		}
		if channelInfo.Type != "whatsapp" && channelInfo.Type != "webchat" {
			return publicOperationResult{status: http.StatusUnprocessableEntity, body: publicErrorEnvelope{Error: publicAPIError{Code: "unsupported_channel", Message: "este canal ainda não aceita mensagens pela API pública"}}}
		}
		delivered := true
		if channelInfo.Type == "whatsapp" {
			if api.sender == nil {
				return publicOperationResult{status: http.StatusServiceUnavailable, body: publicErrorEnvelope{Error: publicAPIError{Code: "channel_unavailable", Message: "o envio do canal não está disponível agora"}}}
			}
			if _, err := api.sender.SendTextChannel(ctx, channelInfo, conversation.CustomerPhone, input.Content); err != nil {
				return publicOperationResult{status: http.StatusBadGateway, body: publicErrorEnvelope{Error: publicAPIError{Code: "delivery_failed", Message: "não foi possível entregar a mensagem ao canal"}}}
			}
		}
		if err := api.st.AddMessage(ctx, store.Message{ConversationID: conversation.ID, Role: "assistant", Content: input.Content, Source: "human"}); err != nil {
			return publicOperationResult{status: http.StatusInternalServerError, body: publicErrorEnvelope{Error: publicAPIError{Code: "message_record_failed", Message: "mensagem entregue, mas não foi possível registrá-la"}}}
		}
		_ = api.st.TouchConversation(ctx, conversation.ID)
		updated, err := api.st.GetConversation(ctx, conversation.ID)
		if err != nil {
			updated = conversation
		}
		api.enqueueWebhook(ctx, webhooks.EventMessageSent, "message.sent:public:"+strconv.Itoa(conversation.ID)+":"+publicFingerprint("webhook.message.sent", input), map[string]interface{}{
			"conversation": publicConversationFromStore(channelInfo, updated),
			"message":      map[string]string{"role": "assistant", "source": "api", "content": input.Content},
		})
		return publicOperationResult{status: http.StatusOK, body: publicJSONEnvelope{Data: publicMessageData{
			Conversation: publicConversationFromStore(channelInfo, updated),
			Delivered:    delivered,
		}}}
	})
}

func (api *PublicAPI) HandlePublicClose(w http.ResponseWriter, r *http.Request) {
	ctx, apiKey, ok := api.authorize(w, r, "conversations:write")
	if !ok {
		return
	}
	id, err := parsePublicConversationID(r)
	if err != nil {
		writePublicError(w, http.StatusBadRequest, "invalid_conversation", err.Error())
		return
	}
	raw, input, result := decodePublicJSON[publicCloseInput](w, r)
	if result != nil {
		writePublicResult(w, *result)
		return
	}
	reason, result := normalizePublicReason(input.Reason)
	if result != nil {
		writePublicResult(w, *result)
		return
	}
	input.Reason = reason
	api.runPublicMutation(w, r, strconv.FormatInt(apiKey.ID, 10)+":conversation.close:"+strconv.Itoa(id), publicFingerprint("conversation.close", input), raw, func() publicOperationResult {
		conversation, channelInfo, ownedResult := api.ownedConversation(ctx, id)
		if ownedResult != nil {
			return *ownedResult
		}
		closed, err := api.st.EncerrarConversationIfOpen(ctx, conversation.ID, reason, nil)
		if err != nil {
			return publicOperationResult{status: http.StatusInternalServerError, body: publicErrorEnvelope{Error: publicAPIError{Code: "conversation_close_failed", Message: "não foi possível encerrar a conversa"}}}
		}
		satisfactionDelivered := false
		if closed {
			text, satisfactionErr := channel.BeginSatisfaction(ctx, api.st, conversation.ID)
			if satisfactionErr == nil && strings.TrimSpace(text) != "" {
				if channelInfo.Type == "whatsapp" && api.sender != nil {
					_, sendErr := api.sender.SendTextChannel(ctx, channelInfo, conversation.CustomerPhone, text)
					satisfactionDelivered = sendErr == nil
				} else if channelInfo.Type == "webchat" {
					satisfactionDelivered = true
				}
			}
		}
		updated, err := api.st.GetConversation(ctx, conversation.ID)
		if err != nil {
			updated = conversation
		}
		if closed {
			api.enqueueWebhook(ctx, webhooks.EventConversationClosed, "conversation.closed:"+strconv.Itoa(conversation.ID), map[string]interface{}{
				"conversation": publicConversationFromStore(channelInfo, updated), "reason": reason,
			})
			if satisfactionDelivered {
				api.enqueueWebhook(ctx, webhooks.EventSatisfactionRequested, "satisfaction.requested:"+strconv.Itoa(conversation.ID), map[string]interface{}{
					"conversation": publicConversationFromStore(channelInfo, updated),
				})
			}
		}
		return publicOperationResult{status: http.StatusOK, body: publicJSONEnvelope{Data: publicCloseData{
			Conversation:          publicConversationFromStore(channelInfo, updated),
			Closed:                closed || updated.Status == "closed",
			SatisfactionStatus:    updated.Satisfacao,
			SatisfactionDelivered: satisfactionDelivered,
		}}}
	})
}

func (api *PublicAPI) authorize(w http.ResponseWriter, r *http.Request, requiredScope string) (context.Context, store.APIKey, bool) {
	credential := publicCredential(r)
	if credential == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="public-api"`)
		writePublicError(w, http.StatusUnauthorized, "missing_credentials", "chave da API é obrigatória")
		return nil, store.APIKey{}, false
	}
	apiKey, err := api.st.AuthenticateAPIKey(r.Context(), credential)
	if err != nil {
		writePublicError(w, http.StatusUnauthorized, "invalid_credentials", "credencial inválida")
		return nil, store.APIKey{}, false
	}
	if !publicScopeAllowed(apiKey.Scopes, requiredScope) {
		writePublicError(w, http.StatusForbidden, "scope_forbidden", "a chave não possui o escopo necessário")
		return nil, store.APIKey{}, false
	}
	tenantCtx := store.WithTenantID(r.Context(), apiKey.TenantID)
	if enabled, moduleErr := api.st.ModuleEnabled(tenantCtx, "api"); moduleErr != nil || !enabled {
		writePublicError(w, http.StatusForbidden, "module_disabled", "o módulo de API não está habilitado para esta empresa")
		return nil, store.APIKey{}, false
	}
	if !api.state.allow(apiKey.Prefix) {
		w.Header().Set("Retry-After", "60")
		writePublicError(w, http.StatusTooManyRequests, "rate_limited", "limite temporário de requisições atingido")
		return nil, store.APIKey{}, false
	}
	return tenantCtx, apiKey, true
}

func (api *PublicAPI) ownedConversation(ctx context.Context, id int) (store.Conversation, store.Channel, *publicOperationResult) {
	conversation, err := api.st.GetConversation(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Conversation{}, store.Channel{}, publicErrorResult(http.StatusNotFound, "conversation_not_found", "conversa não encontrada")
		}
		return store.Conversation{}, store.Channel{}, publicErrorResult(http.StatusInternalServerError, "conversation_load_failed", "não foi possível carregar a conversa")
	}
	if conversation.ChannelID == nil {
		return store.Conversation{}, store.Channel{}, publicErrorResult(http.StatusNotFound, "conversation_not_found", "conversa não encontrada")
	}
	channelInfo, err := api.st.GetChannelByID(ctx, *conversation.ChannelID)
	if err != nil || !channelInfo.Active {
		return store.Conversation{}, store.Channel{}, publicErrorResult(http.StatusNotFound, "conversation_not_found", "conversa não encontrada")
	}
	return conversation, channelInfo, nil
}

func normalizePublicOpenInput(input publicOpenConversationInput) (publicOpenConversationInput, *publicOperationResult) {
	identifier := strings.TrimSpace(input.CustomerID)
	for _, alias := range []string{input.Identifier, input.Phone} {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		if identifier != "" && identifier != alias {
			return input, publicErrorResult(http.StatusBadRequest, "ambiguous_customer", "informe apenas um identificador do cliente")
		}
		identifier = alias
	}
	if identifier == "" {
		return input, publicErrorResult(http.StatusBadRequest, "missing_customer_id", "customer_id é obrigatório")
	}
	if len([]rune(identifier)) > publicAPIMaxCustomerIDRunes || containsControl(identifier) {
		return input, publicErrorResult(http.StatusBadRequest, "invalid_customer_id", "customer_id deve ter no máximo 160 caracteres")
	}
	name := strings.TrimSpace(input.CustomerName)
	if input.Name != "" {
		if name != "" && name != strings.TrimSpace(input.Name) {
			return input, publicErrorResult(http.StatusBadRequest, "ambiguous_customer_name", "informe apenas um nome do cliente")
		}
		name = strings.TrimSpace(input.Name)
	}
	if name == "" {
		name = "Cliente"
	}
	if len([]rune(name)) > publicAPIMaxNameRunes || containsControl(name) {
		return input, publicErrorResult(http.StatusBadRequest, "invalid_customer_name", "nome do cliente deve ter no máximo 80 caracteres")
	}
	origin, err := normalizePublicOrigin(input.Origin)
	if err != nil {
		return input, publicErrorResult(http.StatusBadRequest, "invalid_origin", err.Error())
	}
	return publicOpenConversationInput{ChannelID: input.ChannelID, Channel: strings.ToLower(strings.TrimSpace(input.Channel)), CustomerID: identifier, CustomerName: name, Origin: origin}, nil
}

func (api *PublicAPI) resolvePublicChannel(ctx context.Context, input publicOpenConversationInput) (store.Channel, *publicOperationResult) {
	if input.ChannelID < 1 {
		return store.Channel{}, publicErrorResult(http.StatusBadRequest, "missing_channel_id", "channel_id é obrigatório")
	}
	channelInfo, err := api.st.GetChannelByID(ctx, input.ChannelID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.Channel{}, publicErrorResult(http.StatusNotFound, "channel_not_found", "canal não encontrado")
		}
		return store.Channel{}, publicErrorResult(http.StatusInternalServerError, "channel_load_failed", "não foi possível carregar o canal")
	}
	if !channelInfo.Active {
		return store.Channel{}, publicErrorResult(http.StatusForbidden, "channel_inactive", "canal inativo")
	}
	if channelInfo.Type != "whatsapp" && channelInfo.Type != "webchat" {
		return store.Channel{}, publicErrorResult(http.StatusUnprocessableEntity, "unsupported_channel", "este canal ainda não aceita conversas pela API pública")
	}
	if input.Channel != "" && !strings.EqualFold(input.Channel, channelInfo.Type) {
		return store.Channel{}, publicErrorResult(http.StatusBadRequest, "channel_mismatch", "o canal informado não corresponde ao channel_id")
	}
	return channelInfo, nil
}

func publicScopeAllowed(scopes []string, required string) bool {
	for _, scope := range scopes {
		scope = strings.TrimSpace(strings.ToLower(scope))
		if scope == "*" || scope == strings.ToLower(required) {
			return true
		}
		if strings.HasSuffix(scope, ":*") && strings.HasPrefix(strings.ToLower(required), strings.TrimSuffix(scope, "*")) {
			return true
		}
	}
	return false
}

func normalizePublicReason(value string) (string, *publicOperationResult) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "Encerrado via API pública"
	}
	if len([]rune(value)) > publicAPIMaxReasonRunes || containsControl(value) {
		return "", publicErrorResult(http.StatusBadRequest, "invalid_reason", "motivo deve ter no máximo 240 caracteres")
	}
	return value, nil
}

func normalizePublicOrigin(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len([]byte(value)) > publicAPIMaxOriginBytes {
		return "", errors.New("origin excede o limite de 2.048 bytes")
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("origin deve ser uma URL HTTP ou HTTPS válida")
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), nil
}

func publicConversationFromStore(channelInfo store.Channel, conversation store.Conversation) publicConversation {
	customerID := conversation.CustomerPhone
	if channelInfo.Type == "webchat" {
		customerID = strings.TrimPrefix(customerID, "web:")
		customerID = strings.TrimPrefix(customerID, "api:")
	}
	return publicConversation{
		ID: conversation.ID, Hash: conversation.Hash, Channel: conversation.Channel,
		ChannelName: channelInfo.Name, CustomerID: customerID, CustomerName: conversation.CustomerName,
		Origin: conversation.Origem, Status: conversation.Status, Satisfaction: conversation.Satisfacao,
		CreatedAt: conversation.CreatedAt, UpdatedAt: conversation.UpdatedAt,
	}
}

func parsePublicConversationID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(strings.TrimSpace(r.PathValue("id")))
	if err != nil || id < 1 {
		return 0, errors.New("id da conversa inválido")
	}
	return id, nil
}

func publicCredential(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value) >= 7 && strings.EqualFold(value[:7], "bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return strings.TrimSpace(r.Header.Get("X-API-Key"))
}

func decodePublicJSON[T any](w http.ResponseWriter, r *http.Request) ([]byte, T, *publicOperationResult) {
	var input T
	r.Body = http.MaxBytesReader(w, r.Body, publicAPIMaxJSONBytes+1)
	raw, err := io.ReadAll(io.LimitReader(r.Body, publicAPIMaxJSONBytes+1))
	if err != nil {
		return raw, input, publicErrorResult(http.StatusBadRequest, "invalid_json", "não foi possível ler o JSON")
	}
	if len(raw) > publicAPIMaxJSONBytes {
		return raw, input, publicErrorResult(http.StatusRequestEntityTooLarge, "payload_too_large", "JSON excede o limite de 64 KB")
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return raw, input, publicErrorResult(http.StatusBadRequest, "invalid_json", "JSON obrigatório")
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return raw, input, publicErrorResult(http.StatusBadRequest, "invalid_json", "JSON inválido")
	}
	return raw, input, nil
}

func (api *PublicAPI) runPublicMutation(w http.ResponseWriter, r *http.Request, scope, fingerprint string, _ []byte, operation func() publicOperationResult) {
	key, err := publicIdempotencyKey(r)
	if err != nil {
		writePublicError(w, http.StatusBadRequest, "invalid_idempotency_key", err.Error())
		return
	}
	decision, err := api.state.begin(publicCredentialFingerprint(publicCredential(r))+":"+scope, key, fingerprint)
	if err != nil {
		if errors.Is(err, errPublicIdempotencyConflict) {
			writePublicError(w, http.StatusConflict, "idempotency_conflict", "a mesma Idempotency-Key foi usada com outro conteúdo")
			return
		}
		if errors.Is(err, errPublicIdempotencyInProgress) {
			w.Header().Set("Retry-After", "1")
			writePublicError(w, http.StatusConflict, "idempotency_in_progress", "a requisição original ainda está sendo processada")
			return
		}
		writePublicError(w, http.StatusInternalServerError, "idempotency_failed", "não foi possível controlar a idempotência")
		return
	}
	if decision.replay {
		w.Header().Set("Idempotent-Replayed", "true")
		writePublicBytes(w, decision.status, decision.body)
		return
	}
	result := operation()
	body, marshalErr := json.Marshal(result.body)
	if marshalErr != nil {
		result = publicOperationResult{status: http.StatusInternalServerError, body: publicErrorEnvelope{Error: publicAPIError{Code: "response_failed", Message: "não foi possível montar a resposta"}}}
		body, _ = json.Marshal(result.body)
	}
	// A mutação pode ter produzido um efeito antes de retornar um 5xx (por
	// exemplo, o provedor aceitou a mensagem mas o registro local falhou).
	// Guardar também a resposta de erro evita que a repetição com a mesma
	// Idempotency-Key fique presa como "em processamento" ou duplique o efeito.
	api.state.finish(publicCredentialFingerprint(publicCredential(r))+":"+scope, key, fingerprint, result.status, body)
	writePublicBytes(w, result.status, body)
}

func publicErrorResult(status int, code, message string) *publicOperationResult {
	return &publicOperationResult{status: status, body: publicErrorEnvelope{Error: publicAPIError{Code: code, Message: message}}}
}

func publicFingerprint(scope string, value interface{}) string {
	return publicHash(scope + "\x00" + mustPublicJSON(value))
}

func mustPublicJSON(value interface{}) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func publicCredentialFingerprint(value string) string {
	return publicHash(value)
}

func publicHash(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func containsControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func writePublicError(w http.ResponseWriter, status int, code, message string) {
	writePublicJSON(w, status, publicErrorEnvelope{Error: publicAPIError{Code: code, Message: message}})
}

func writePublicResult(w http.ResponseWriter, result publicOperationResult) {
	writePublicJSON(w, result.status, result.body)
}

func writePublicJSON(w http.ResponseWriter, status int, value interface{}) {
	body, err := json.Marshal(value)
	if err != nil {
		status = http.StatusInternalServerError
		body, _ = json.Marshal(publicErrorEnvelope{Error: publicAPIError{Code: "response_failed", Message: "não foi possível montar a resposta"}})
	}
	writePublicBytes(w, status, body)
}

func writePublicBytes(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
