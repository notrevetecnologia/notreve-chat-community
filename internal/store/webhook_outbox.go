package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/webhooks"
)

const (
	defaultWebhookOutboxMaxAttempts = 5
	maxWebhookOutboxErrorCode       = 64
	maxWebhookOutboxErrorMessage    = 1024
)

var webhookOutboxErrorCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_:.\-]{0,63}$`)

// WebhookOutboxEventInput representa um fato de domínio pronto para ser
// publicado. EventKey deve ser determinístico no escopo do tenant, por
// exemplo "message.sent:12345"; repetições da mesma chave não criam uma nova
// entrega.
type WebhookOutboxEventInput struct {
	EventKey   string
	EventType  webhooks.Event
	Payload    json.RawMessage
	OccurredAt time.Time
}

type WebhookOutboxEvent struct {
	ID         int64           `json:"id"`
	TenantID   int             `json:"tenant_id"`
	EventKey   string          `json:"event_key"`
	EventType  webhooks.Event  `json:"event_type"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt time.Time       `json:"occurred_at"`
	CreatedAt  time.Time       `json:"created_at"`
}

type EnqueueWebhookOutboxResult struct {
	Event              WebhookOutboxEvent `json:"event"`
	Created            bool               `json:"created"`
	DeliveriesEnqueued int64              `json:"deliveries_enqueued"`
}

type WebhookOutboxDelivery struct {
	ID               int64           `json:"id"`
	TenantID         int             `json:"tenant_id"`
	EventID          int64           `json:"event_id"`
	SubscriptionID   int64           `json:"subscription_id"`
	EndpointURL      string          `json:"endpoint_url"`
	HasSigningSecret bool            `json:"has_signing_secret"`
	EventKey         string          `json:"event_key"`
	EventType        webhooks.Event  `json:"event_type"`
	Payload          json.RawMessage `json:"payload"`
	Status           string          `json:"status"`
	Attempts         int             `json:"attempts"`
	MaxAttempts      int             `json:"max_attempts"`
	NextAttemptAt    time.Time       `json:"next_attempt_at"`
	LastAttemptAt    *time.Time      `json:"last_attempt_at,omitempty"`
	LeaseUntil       *time.Time      `json:"lease_until,omitempty"`
	ResponseStatus   int             `json:"response_status,omitempty"`
	LastErrorCode    string          `json:"last_error_code,omitempty"`
	LastErrorMessage string          `json:"last_error_message,omitempty"`
	DeliveredAt      *time.Time      `json:"delivered_at,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type WebhookOutboxDeliveryView struct {
	ID               int64      `json:"id"`
	SubscriptionID   int64      `json:"subscription_id"`
	SubscriptionName string     `json:"subscription_name"`
	EndpointURL      string     `json:"endpoint_url"`
	EventKey         string     `json:"event_key"`
	EventType        string     `json:"event_type"`
	Status           string     `json:"status"`
	Attempts         int        `json:"attempts"`
	MaxAttempts      int        `json:"max_attempts"`
	NextAttemptAt    time.Time  `json:"next_attempt_at"`
	LastAttemptAt    *time.Time `json:"last_attempt_at,omitempty"`
	ResponseStatus   int        `json:"response_status,omitempty"`
	LastErrorCode    string     `json:"last_error_code,omitempty"`
	LastErrorMessage string     `json:"last_error_message,omitempty"`
	DeliveredAt      *time.Time `json:"delivered_at,omitempty"`
	EventOccurredAt  time.Time  `json:"event_occurred_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// WebhookOutboxAttemptResult é o resultado já higienizado de uma tentativa de
// transporte. A outbox não persiste corpo de resposta para evitar reter dados
// potencialmente sensíveis de sistemas de terceiros.
type WebhookOutboxAttemptResult struct {
	ResponseStatus int
	ErrorCode      string
	ErrorMessage   string
	Retryable      bool
	NextAttemptAt  *time.Time
}

type normalizedWebhookOutboxAttempt struct {
	responseStatus int
	errorCode      string
	errorMessage   string
	retryable      bool
	nextAttemptAt  *time.Time
}

const webhookOutboxEventColumns = `id, tenant_id, event_key, event_type, payload, occurred_at, created_at`

const webhookOutboxDeliveryColumns = `d.id, d.tenant_id, d.event_id, d.subscription_id, d.endpoint_url,
	(s.signing_secret_hash IS NOT NULL),
	e.event_key, e.event_type, e.payload, d.status, d.attempts, d.max_attempts, d.next_attempt_at,
	d.last_attempt_at, d.lease_until, COALESCE(d.response_status, 0), d.last_error_code,
	d.last_error_message, d.delivered_at, d.created_at, d.updated_at`

func validateWebhookOutboxEventInput(input WebhookOutboxEventInput) (WebhookOutboxEventInput, error) {
	input.EventKey = strings.TrimSpace(input.EventKey)
	if input.EventKey == "" || len(input.EventKey) > 200 {
		return WebhookOutboxEventInput{}, errors.New("webhook outbox event key is required and must have at most 200 characters")
	}
	if !webhooks.IsKnownEvent(input.EventType) {
		return WebhookOutboxEventInput{}, errors.New("webhook outbox event type is not in the catalog")
	}
	if !json.Valid(input.Payload) {
		return WebhookOutboxEventInput{}, errors.New("webhook outbox payload must be valid json")
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now().UTC()
	} else {
		input.OccurredAt = input.OccurredAt.UTC()
	}
	return input, nil
}

func scanWebhookOutboxEvent(row interface{ Scan(...interface{}) error }) (WebhookOutboxEvent, error) {
	var event WebhookOutboxEvent
	err := row.Scan(&event.ID, &event.TenantID, &event.EventKey, &event.EventType, &event.Payload, &event.OccurredAt, &event.CreatedAt)
	return event, err
}

func scanWebhookOutboxDelivery(row interface{ Scan(...interface{}) error }) (WebhookOutboxDelivery, error) {
	var delivery WebhookOutboxDelivery
	err := row.Scan(&delivery.ID, &delivery.TenantID, &delivery.EventID, &delivery.SubscriptionID, &delivery.EndpointURL, &delivery.HasSigningSecret,
		&delivery.EventKey, &delivery.EventType, &delivery.Payload, &delivery.Status, &delivery.Attempts,
		&delivery.MaxAttempts, &delivery.NextAttemptAt, &delivery.LastAttemptAt, &delivery.LeaseUntil,
		&delivery.ResponseStatus, &delivery.LastErrorCode, &delivery.LastErrorMessage, &delivery.DeliveredAt,
		&delivery.CreatedAt, &delivery.UpdatedAt)
	return delivery, err
}

// EnqueueWebhookOutboxEvent grava o evento e cria uma entrega para cada
// assinatura ativa que recebeu o tipo (ou o curinga "*") no mesmo commit.
// Um EventKey repetido devolve o evento original sem criar novas entregas.
func (s *Store) EnqueueWebhookOutboxEvent(ctx context.Context, input WebhookOutboxEventInput) (EnqueueWebhookOutboxResult, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return EnqueueWebhookOutboxResult{}, err
	}
	input, err = validateWebhookOutboxEventInput(input)
	if err != nil {
		return EnqueueWebhookOutboxResult{}, err
	}

	var result EnqueueWebhookOutboxResult
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		event, created, err := insertOrLoadWebhookOutboxEvent(ctx, tx, tenantID, input)
		if err != nil {
			return err
		}
		result.Event = event
		result.Created = created
		if !created {
			return nil
		}

		tag, err := tx.Exec(ctx, `INSERT INTO webhook_outbox_deliveries
			(tenant_id, event_id, subscription_id, endpoint_url, max_attempts, next_attempt_at)
			SELECT $1, $2, ws.id, ws.endpoint_url, $3, $4
			FROM webhook_subscriptions ws
			WHERE ws.tenant_id=$1 AND ws.active
			  AND ($5 = ANY(ws.events) OR '*' = ANY(ws.events))
			ON CONFLICT (tenant_id, subscription_id, event_id) DO NOTHING`,
			tenantID, event.ID, defaultWebhookOutboxMaxAttempts, input.OccurredAt, string(input.EventType))
		if err != nil {
			return err
		}
		result.DeliveriesEnqueued = tag.RowsAffected()
		return nil
	})
	if err != nil {
		return EnqueueWebhookOutboxResult{}, err
	}
	return result, nil
}

// PublishOutboxEvent enfileira um evento de domínio na outbox de forma simples.
// O tenant é resolvido do contexto (obrigatório) e o payload deve ser um valor
// serializável. O retorno permite ao chamador decidir como registrar falha; em
// geral a publicação é best-effort e não deve interromper o atendimento.
func (s *Store) PublishOutboxEvent(ctx context.Context, event webhooks.Event, eventKey string, payload interface{}) error {
	if strings.TrimSpace(eventKey) == "" {
		return errors.New("webhook outbox event key is required")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.EnqueueWebhookOutboxEvent(ctx, WebhookOutboxEventInput{
		EventKey: eventKey, EventType: event, Payload: encoded, OccurredAt: time.Now().UTC(),
	})
	return err
}

func insertOrLoadWebhookOutboxEvent(ctx context.Context, tx pgx.Tx, tenantID int, input WebhookOutboxEventInput) (WebhookOutboxEvent, bool, error) {
	event, err := scanWebhookOutboxEvent(tx.QueryRow(ctx, `INSERT INTO webhook_outbox_events
		(tenant_id, event_key, event_type, payload, occurred_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (tenant_id, event_key) DO NOTHING
		RETURNING `+webhookOutboxEventColumns, tenantID, input.EventKey, string(input.EventType), input.Payload, input.OccurredAt))
	if err == nil {
		return event, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return WebhookOutboxEvent{}, false, err
	}
	event, err = scanWebhookOutboxEvent(tx.QueryRow(ctx, `SELECT `+webhookOutboxEventColumns+`
		FROM webhook_outbox_events WHERE tenant_id=$1 AND event_key=$2`, tenantID, input.EventKey))
	return event, false, err
}

// ClaimDueWebhookOutboxDeliveries reserva entregas para um worker. Leases
// vencidos voltam a ser tentados; após esgotar max_attempts tornam-se
// dead_letter para não permanecerem presas em processing.
func (s *Store) ClaimDueWebhookOutboxDeliveries(ctx context.Context, now time.Time, leaseDuration time.Duration, limit int) ([]WebhookOutboxDelivery, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return nil, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if leaseDuration <= 0 || leaseDuration > 30*time.Minute {
		leaseDuration = 2 * time.Minute
	}
	if limit < 1 || limit > 100 {
		limit = 25
	}
	leaseUntil := now.Add(leaseDuration)
	rows, err := s.pool.Query(ctx, `WITH expired AS (
			UPDATE webhook_outbox_deliveries
			SET status='dead_letter', lease_until=NULL, last_error_code='lease_expired',
				last_error_message='A tentativa de entrega expirou antes de ser concluída.', updated_at=now()
			WHERE tenant_id=$1 AND status='processing' AND lease_until <= $2 AND attempts >= max_attempts
		), due AS (
			SELECT d.id
			FROM webhook_outbox_deliveries d
			JOIN webhook_subscriptions s ON s.tenant_id=d.tenant_id AND s.id=d.subscription_id
			WHERE d.tenant_id=$1 AND s.active AND d.attempts < d.max_attempts
			  AND ((d.status IN ('pending','retry_scheduled') AND d.next_attempt_at <= $2)
			       OR (d.status='processing' AND d.lease_until <= $2))
			ORDER BY d.next_attempt_at ASC, d.id ASC
			LIMIT $3 FOR UPDATE SKIP LOCKED
		)
		UPDATE webhook_outbox_deliveries d
		SET status='processing', attempts=d.attempts+1, last_attempt_at=$2,
			lease_until=$4, updated_at=now()
		FROM due, webhook_outbox_events e, webhook_subscriptions s
		WHERE d.tenant_id=$1 AND d.id=due.id
			AND e.tenant_id=d.tenant_id AND e.id=d.event_id
			AND s.tenant_id=d.tenant_id AND s.id=d.subscription_id
		RETURNING `+webhookOutboxDeliveryColumns, tenantID, now, limit, leaseUntil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]WebhookOutboxDelivery, 0)
	for rows.Next() {
		item, err := scanWebhookOutboxDelivery(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// RecordWebhookOutboxAttempt conclui ou reagenda uma entrega já reservada.
// O número da tentativa impede que um worker atrasado substitua o resultado de
// uma nova reserva. response_status e erros são normalizados antes de gravar.
func (s *Store) RecordWebhookOutboxAttempt(ctx context.Context, deliveryID int64, attempt int, result WebhookOutboxAttemptResult) (WebhookOutboxDelivery, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return WebhookOutboxDelivery{}, err
	}
	if deliveryID < 1 || attempt < 1 {
		return WebhookOutboxDelivery{}, errors.New("webhook outbox delivery id and attempt are required")
	}
	normalized, err := normalizeWebhookOutboxAttempt(result)
	if err != nil {
		return WebhookOutboxDelivery{}, err
	}
	now := time.Now().UTC()
	nextAttemptAt := now
	if normalized.nextAttemptAt != nil {
		nextAttemptAt = normalized.nextAttemptAt.UTC()
	}
	delivered := normalized.responseStatus >= 200 && normalized.responseStatus < 300 && normalized.errorCode == ""

	var delivery WebhookOutboxDelivery
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		delivery, err = scanWebhookOutboxDelivery(tx.QueryRow(ctx, `UPDATE webhook_outbox_deliveries d
			SET status=CASE
					WHEN $4 THEN 'delivered'
					WHEN $5 AND d.attempts < d.max_attempts THEN 'retry_scheduled'
					ELSE 'dead_letter'
				END,
				next_attempt_at=CASE WHEN $4 THEN now() ELSE $6 END,
				lease_until=NULL,
				response_status=NULLIF($7, 0),
				last_error_code=CASE WHEN $4 THEN '' ELSE $8 END,
				last_error_message=CASE WHEN $4 THEN '' ELSE $9 END,
				delivered_at=CASE WHEN $4 THEN now() ELSE NULL END,
				updated_at=now()
			FROM webhook_outbox_events e, webhook_subscriptions s
			WHERE d.tenant_id=$1 AND d.id=$2 AND d.attempts=$3 AND d.status='processing'
				AND e.tenant_id=d.tenant_id AND e.id=d.event_id
				AND s.tenant_id=d.tenant_id AND s.id=d.subscription_id
			RETURNING `+webhookOutboxDeliveryColumns,
			tenantID, deliveryID, attempt, delivered, normalized.retryable, nextAttemptAt,
			normalized.responseStatus, normalized.errorCode, normalized.errorMessage))
		if err != nil || delivery.Status != "delivered" {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE webhook_subscriptions
			SET last_delivery_at=now(), updated_at=now()
			WHERE tenant_id=$1 AND id=$2`, tenantID, delivery.SubscriptionID)
		return err
	})
	if err != nil {
		return WebhookOutboxDelivery{}, err
	}
	return delivery, nil
}

func normalizeWebhookOutboxAttempt(result WebhookOutboxAttemptResult) (normalizedWebhookOutboxAttempt, error) {
	if result.ResponseStatus != 0 && (result.ResponseStatus < 100 || result.ResponseStatus > 599) {
		return normalizedWebhookOutboxAttempt{}, fmt.Errorf("webhook outbox response status %d is invalid", result.ResponseStatus)
	}
	code := strings.ToLower(strings.TrimSpace(result.ErrorCode))
	if code != "" && !webhookOutboxErrorCodePattern.MatchString(code) {
		return normalizedWebhookOutboxAttempt{}, errors.New("webhook outbox error code is invalid")
	}
	message := sanitizeWebhookOutboxError(result.ErrorMessage, maxWebhookOutboxErrorMessage)
	if code == "" && message != "" {
		code = "delivery_failed"
	}
	if len(code) > maxWebhookOutboxErrorCode {
		return normalizedWebhookOutboxAttempt{}, errors.New("webhook outbox error code is too long")
	}
	if result.NextAttemptAt != nil {
		next := result.NextAttemptAt.UTC()
		result.NextAttemptAt = &next
	}
	return normalizedWebhookOutboxAttempt{
		responseStatus: result.ResponseStatus,
		errorCode:      code,
		errorMessage:   message,
		retryable:      result.Retryable,
		nextAttemptAt:  result.NextAttemptAt,
	}, nil
}

func scanWebhookOutboxDeliveryView(row interface{ Scan(...interface{}) error }) (WebhookOutboxDeliveryView, error) {
	var view WebhookOutboxDeliveryView
	err := row.Scan(&view.ID, &view.SubscriptionID, &view.SubscriptionName, &view.EndpointURL,
		&view.EventKey, &view.EventType, &view.Status, &view.Attempts, &view.MaxAttempts,
		&view.NextAttemptAt, &view.LastAttemptAt, &view.ResponseStatus, &view.LastErrorCode,
		&view.LastErrorMessage, &view.DeliveredAt, &view.EventOccurredAt, &view.CreatedAt, &view.UpdatedAt)
	return view, err
}

// ListWebhookOutboxDeliveries devolve o log de entregas da outbox do tenant,
// com filtros opcionais de assinatura, status e tipo de evento. A consulta é
// sempre limitada ao tenant do contexto e nunca expõe payload de evento.
func (s *Store) ListWebhookOutboxDeliveries(ctx context.Context, subscriptionID *int64, status, eventType string, limit, offset int) ([]WebhookOutboxDeliveryView, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	query := `SELECT d.id, d.subscription_id, COALESCE(ws.name, ''), d.endpoint_url, e.event_key, e.event_type,
		d.status, d.attempts, d.max_attempts, d.next_attempt_at, d.last_attempt_at, COALESCE(d.response_status, 0),
		d.last_error_code, d.last_error_message, d.delivered_at, e.occurred_at, d.created_at, d.updated_at
		FROM webhook_outbox_deliveries d
		JOIN webhook_outbox_events e ON e.tenant_id=d.tenant_id AND e.id=d.event_id
		LEFT JOIN webhook_subscriptions ws ON ws.tenant_id=d.tenant_id AND ws.id=d.subscription_id
		WHERE d.tenant_id=$1`
	args := []interface{}{tenantID}
	if subscriptionID != nil {
		args = append(args, *subscriptionID)
		query += ` AND d.subscription_id=$` + strconv.Itoa(len(args))
	}
	if status != "" {
		status = strings.TrimSpace(strings.ToLower(status))
		switch status {
		case "pending", "processing", "retry_scheduled", "delivered", "dead_letter":
		default:
			return nil, errors.New("invalid webhook delivery status")
		}
		args = append(args, status)
		query += ` AND d.status=$` + strconv.Itoa(len(args))
	}
	if eventType != "" {
		eventType = strings.ToLower(strings.TrimSpace(eventType))
		if !webhooks.IsKnownEvent(webhooks.Event(eventType)) {
			return nil, errors.New("invalid webhook delivery event type")
		}
		args = append(args, eventType)
		query += ` AND e.event_type=$` + strconv.Itoa(len(args))
	}
	args = append(args, limit, offset)
	query += ` ORDER BY d.id DESC LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]WebhookOutboxDeliveryView, 0)
	for rows.Next() {
		item, err := scanWebhookOutboxDeliveryView(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func sanitizeWebhookOutboxError(value string, limit int) string {
	value = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, strings.TrimSpace(value))
	if limit < 1 || len(value) <= limit {
		return value
	}
	return strings.TrimSpace(value[:limit])
}
