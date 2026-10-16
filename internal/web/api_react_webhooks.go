package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"github.com/notrevetecnologia/notreve-chat-community/internal/webhooks"
)

// reactWebhookSubscriptionView mantém a superfície React deliberadamente
// pública: o hash e o valor do segredo de assinatura nunca saem do servidor.
type reactWebhookSubscriptionView struct {
	ID               int64    `json:"id"`
	Name             string   `json:"name"`
	EndpointURL      string   `json:"endpoint_url"`
	Events           []string `json:"events"`
	HasSigningSecret bool     `json:"has_signing_secret"`
	Active           bool     `json:"active"`
	LastDeliveryAt   *string  `json:"last_delivery_at,omitempty"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}

type reactWebhookSubscriptionInput struct {
	Name          *string   `json:"name"`
	EndpointURL   *string   `json:"endpoint_url"`
	Events        *[]string `json:"events"`
	SigningSecret *string   `json:"signing_secret"`
	Active        *bool     `json:"active"`
}

type reactWebhookSubscriptionToggleInput struct {
	Active bool `json:"active"`
}

// reactWebhookDeliveryView é a superfície pública do log de entregas: nunca
// inclui payload do evento nem tenant_id.
type reactWebhookDeliveryView struct {
	ID               int64   `json:"id"`
	SubscriptionID   int64   `json:"subscription_id"`
	SubscriptionName string  `json:"subscription_name"`
	EndpointURL      string  `json:"endpoint_url"`
	EventKey         string  `json:"event_key"`
	EventType        string  `json:"event_type"`
	Status           string  `json:"status"`
	Attempts         int     `json:"attempts"`
	MaxAttempts      int     `json:"max_attempts"`
	NextAttemptAt    string  `json:"next_attempt_at"`
	LastAttemptAt    *string `json:"last_attempt_at,omitempty"`
	ResponseStatus   int     `json:"response_status,omitempty"`
	LastErrorCode    string  `json:"last_error_code,omitempty"`
	LastErrorMessage string  `json:"last_error_message,omitempty"`
	DeliveredAt      *string `json:"delivered_at,omitempty"`
	EventOccurredAt  string  `json:"event_occurred_at"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

func reactWebhookSubscriptionFromStore(subscription store.WebhookSubscription) reactWebhookSubscriptionView {
	view := reactWebhookSubscriptionView{
		ID:               subscription.ID,
		Name:             subscription.Name,
		EndpointURL:      subscription.EndpointURL,
		Events:           append([]string{}, subscription.Events...),
		HasSigningSecret: subscription.HasSigningSecret,
		Active:           subscription.Active,
		CreatedAt:        subscription.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        subscription.UpdatedAt.Format(time.RFC3339),
	}
	if subscription.LastDeliveryAt != nil {
		formatted := subscription.LastDeliveryAt.Format(time.RFC3339)
		view.LastDeliveryAt = &formatted
	}
	return view
}

func reactWebhookDeliveryFromStore(delivery store.WebhookOutboxDeliveryView) reactWebhookDeliveryView {
	view := reactWebhookDeliveryView{
		ID: delivery.ID, SubscriptionID: delivery.SubscriptionID, SubscriptionName: delivery.SubscriptionName,
		EndpointURL: delivery.EndpointURL, EventKey: delivery.EventKey, EventType: delivery.EventType,
		Status: delivery.Status, Attempts: delivery.Attempts, MaxAttempts: delivery.MaxAttempts,
		NextAttemptAt: delivery.NextAttemptAt.Format(time.RFC3339), ResponseStatus: delivery.ResponseStatus,
		LastErrorCode: delivery.LastErrorCode, LastErrorMessage: delivery.LastErrorMessage,
		EventOccurredAt: delivery.EventOccurredAt.Format(time.RFC3339),
		CreatedAt: delivery.CreatedAt.Format(time.RFC3339), UpdatedAt: delivery.UpdatedAt.Format(time.RFC3339),
	}
	if delivery.LastAttemptAt != nil {
		formatted := delivery.LastAttemptAt.Format(time.RFC3339)
		view.LastAttemptAt = &formatted
	}
	if delivery.DeliveredAt != nil {
		formatted := delivery.DeliveredAt.Format(time.RFC3339)
		view.DeliveredAt = &formatted
	}
	return view
}

func parseReactWebhookSubscriptionID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("webhook inválido")
	}
	return id, nil
}

func normalizeReactWebhookEvents(events []string) ([]string, error) {
	seen := make(map[string]bool, len(events))
	clean := make([]string, 0, len(events))
	for _, event := range events {
		event = strings.ToLower(strings.TrimSpace(event))
		if event == "" || !webhooks.IsKnownEvent(webhooks.Event(event)) {
			return nil, errors.New("selecione somente eventos do catálogo")
		}
		if seen[event] {
			continue
		}
		seen[event] = true
		clean = append(clean, event)
	}
	if len(clean) == 0 {
		return nil, errors.New("selecione pelo menos um evento")
	}
	return clean, nil
}

func validateReactWebhookSigningSecret(secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil
	}
	if size := len([]byte(secret)); size < 16 || size > 256 {
		return errors.New("o segredo de assinatura deve ter entre 16 e 256 bytes")
	}
	return nil
}

func applyReactWebhookSubscriptionInput(existing *store.WebhookSubscription, input reactWebhookSubscriptionInput, creating bool) (store.WebhookSubscriptionInput, error) {
	if input.Name != nil {
		existing.Name = strings.TrimSpace(*input.Name)
	}
	if input.EndpointURL != nil {
		existing.EndpointURL = strings.TrimSpace(*input.EndpointURL)
	}
	if input.Events != nil {
		events, err := normalizeReactWebhookEvents(*input.Events)
		if err != nil {
			return store.WebhookSubscriptionInput{}, err
		}
		existing.Events = events
	}
	if input.Active != nil {
		existing.Active = *input.Active
	} else if creating {
		existing.Active = true
	}
	if strings.TrimSpace(existing.Name) == "" || len([]rune(existing.Name)) > 120 {
		return store.WebhookSubscriptionInput{}, errors.New("nome deve ter entre 1 e 120 caracteres")
	}
	if _, err := normalizeReactWebhookEvents(existing.Events); err != nil {
		return store.WebhookSubscriptionInput{}, err
	}
	if err := webhooks.ValidateURL(existing.EndpointURL, nil); err != nil {
		return store.WebhookSubscriptionInput{}, err
	}

	signingSecret := ""
	if input.SigningSecret != nil {
		signingSecret = strings.TrimSpace(*input.SigningSecret)
		if err := validateReactWebhookSigningSecret(signingSecret); err != nil {
			return store.WebhookSubscriptionInput{}, err
		}
	}
	return store.WebhookSubscriptionInput{
		ID: existing.ID, Name: existing.Name, EndpointURL: existing.EndpointURL,
		Events: append([]string{}, existing.Events...), SigningSecret: signingSecret, Active: existing.Active,
	}, nil
}

func reactWebhookSubscriptionNeedsPlanCheck(wasActive, willBeActive bool) bool {
	return willBeActive && !wasActive
}

func writeReactWebhookPlanCheck(w http.ResponseWriter, s *Server, r *http.Request) bool {
	if err := s.st.EnsureCanCreateWebhookSubscription(r.Context()); err != nil {
		if writeReactPlanLimitError(w, err) {
			return false
		}
		writeReactError(w, http.StatusInternalServerError, "plan_check_failed", "não foi possível validar o limite do plano")
		return false
	}
	return true
}

// writeReactWebhookSecretStorageError evita que uma configuração de plataforma
// ausente pareça falha do segredo informado pelo administrador da empresa.
// O detalhe da chave de cifragem nunca é revelado pela API.
func writeReactWebhookSecretStorageError(w http.ResponseWriter, err error) bool {
	if !errors.Is(err, store.ErrWebhookSecretStorageUnavailable) {
		return false
	}
	writeReactError(w, http.StatusServiceUnavailable, "signing_secret_unavailable", "a assinatura segura de webhooks ainda não está disponível na plataforma")
	return true
}

func writeReactWebhookNotFound(w http.ResponseWriter) {
	writeReactError(w, http.StatusNotFound, "webhook_not_found", "webhook não encontrado")
}

func isReactWebhookEndpointUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *Server) handleReactWebhookSubscriptions(w http.ResponseWriter, r *http.Request) {
	subscriptions, err := s.st.ListWebhookSubscriptions(r.Context())
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "webhooks_load_failed", "não foi possível carregar os webhooks")
		return
	}
	items := make([]reactWebhookSubscriptionView, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		items = append(items, reactWebhookSubscriptionFromStore(subscription))
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": items, "catalog": webhooks.Catalog()}, nil)
}

func (s *Server) handleReactWebhookSubscriptionCreate(w http.ResponseWriter, r *http.Request) {
	var input reactWebhookSubscriptionInput
	if err := decodeReactAdminJSON(r, &input, 32*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	subscription := store.WebhookSubscription{}
	storeInput, err := applyReactWebhookSubscriptionInput(&subscription, input, true)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_webhook", err.Error())
		return
	}
	if storeInput.Active && !writeReactWebhookPlanCheck(w, s, r) {
		return
	}
	created, err := s.st.CreateWebhookSubscription(r.Context(), storeInput)
	if err != nil {
		if writeReactWebhookSecretStorageError(w, err) {
			return
		}
		if isReactWebhookEndpointUniqueViolation(err) {
			writeReactError(w, http.StatusConflict, "endpoint_exists", "já existe um webhook para este endpoint")
			return
		}
		writeReactError(w, http.StatusInternalServerError, "create_failed", "não foi possível criar o webhook")
		return
	}
	writeReactResponse(w, http.StatusCreated, reactWebhookSubscriptionFromStore(created), nil)
}

func (s *Server) handleReactWebhookSubscriptionUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactWebhookSubscriptionID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	subscription, err := s.st.GetWebhookSubscription(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeReactWebhookNotFound(w)
			return
		}
		writeReactError(w, http.StatusInternalServerError, "webhook_load_failed", "não foi possível carregar o webhook")
		return
	}
	wasActive := subscription.Active
	var input reactWebhookSubscriptionInput
	if err := decodeReactAdminJSON(r, &input, 32*1024); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	storeInput, err := applyReactWebhookSubscriptionInput(&subscription, input, false)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_webhook", err.Error())
		return
	}
	if reactWebhookSubscriptionNeedsPlanCheck(wasActive, storeInput.Active) && !writeReactWebhookPlanCheck(w, s, r) {
		return
	}
	if err := s.st.UpdateWebhookSubscription(r.Context(), storeInput); err != nil {
		if writeReactWebhookSecretStorageError(w, err) {
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			writeReactWebhookNotFound(w)
			return
		}
		if isReactWebhookEndpointUniqueViolation(err) {
			writeReactError(w, http.StatusConflict, "endpoint_exists", "já existe um webhook para este endpoint")
			return
		}
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível atualizar o webhook")
		return
	}
	updated, err := s.st.GetWebhookSubscription(r.Context(), id)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "webhook_load_failed", "webhook atualizado, mas não foi possível carregá-lo")
		return
	}
	writeReactResponse(w, http.StatusOK, reactWebhookSubscriptionFromStore(updated), nil)
}

func (s *Server) handleReactWebhookSubscriptionToggle(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactWebhookSubscriptionID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	var input reactWebhookSubscriptionToggleInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&input); err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_json", "JSON inválido")
		return
	}
	subscription, err := s.st.GetWebhookSubscription(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeReactWebhookNotFound(w)
			return
		}
		writeReactError(w, http.StatusInternalServerError, "webhook_load_failed", "não foi possível carregar o webhook")
		return
	}
	if reactWebhookSubscriptionNeedsPlanCheck(subscription.Active, input.Active) && !writeReactWebhookPlanCheck(w, s, r) {
		return
	}
	if err := s.st.SetWebhookSubscriptionActive(r.Context(), id, input.Active); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeReactWebhookNotFound(w)
			return
		}
		writeReactError(w, http.StatusInternalServerError, "update_failed", "não foi possível alterar o estado do webhook")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]bool{"active": input.Active}, nil)
}

func (s *Server) handleReactWebhookSubscriptionDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseReactWebhookSubscriptionID(r)
	if err != nil {
		writeReactError(w, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if err := s.st.DeleteWebhookSubscription(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeReactWebhookNotFound(w)
			return
		}
		writeReactError(w, http.StatusInternalServerError, "delete_failed", "não foi possível excluir o webhook")
		return
	}
	writeReactResponse(w, http.StatusOK, map[string]bool{"deleted": true}, nil)
}

func (s *Server) handleReactWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	var subscriptionID *int64
	if raw := strings.TrimSpace(query.Get("subscription_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 {
			writeReactError(w, http.StatusBadRequest, "invalid_filter", "filtro de assinatura inválido")
			return
		}
		subscriptionID = &id
	}
	status := strings.TrimSpace(query.Get("status"))
	eventType := strings.TrimSpace(query.Get("event_type"))
	limit, offset := 50, 0
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_filter", "limite inválido")
			return
		}
		limit = parsed
	}
	if raw := strings.TrimSpace(query.Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeReactError(w, http.StatusBadRequest, "invalid_filter", "deslocamento inválido")
			return
		}
		offset = parsed
	}
	items, err := s.st.ListWebhookOutboxDeliveries(r.Context(), subscriptionID, status, eventType, limit, offset)
	if err != nil {
		writeReactError(w, http.StatusInternalServerError, "webhook_deliveries_load_failed", "não foi possível carregar as entregas")
		return
	}
	views := make([]reactWebhookDeliveryView, 0, len(items))
	for _, item := range items {
		views = append(views, reactWebhookDeliveryFromStore(item))
	}
	writeReactResponse(w, http.StatusOK, map[string]interface{}{"items": views}, nil)
}
