package store

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/notrevetecnologia/notreve-chat-community/internal/webhooks"
)

type WebhookSubscription struct {
	ID               int64      `json:"id"`
	TenantID         int        `json:"tenant_id"`
	Name             string     `json:"name"`
	EndpointURL      string     `json:"endpoint_url"`
	Events           []string   `json:"events"`
	HasSigningSecret bool       `json:"has_signing_secret"`
	Active           bool       `json:"active"`
	LastDeliveryAt   *time.Time `json:"last_delivery_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type WebhookSubscriptionInput struct {
	ID            int64
	Name          string
	EndpointURL   string
	Events        []string
	SigningSecret string
	Active        bool
}

type WebhookDelivery struct {
	ID             int64      `json:"id"`
	TenantID       int        `json:"tenant_id"`
	SubscriptionID int64      `json:"subscription_id"`
	EventType      string     `json:"event_type"`
	EventID        string     `json:"event_id"`
	Payload        []byte     `json:"payload"`
	Status         string     `json:"status"`
	Attempts       int        `json:"attempts"`
	NextAttemptAt  time.Time  `json:"next_attempt_at"`
	LastAttemptAt  *time.Time `json:"last_attempt_at,omitempty"`
	ResponseStatus int        `json:"response_status,omitempty"`
	ResponseBody   string     `json:"response_body,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func normalizeWebhookEvents(events []string) []string {
	seen := make(map[string]struct{}, len(events))
	result := make([]string, 0, len(events))
	for _, event := range events {
		event = strings.ToLower(strings.TrimSpace(event))
		if event == "" || len(event) > 100 {
			continue
		}
		if _, ok := seen[event]; ok {
			continue
		}
		seen[event] = struct{}{}
		result = append(result, event)
	}
	sort.Strings(result)
	return result
}

func normalizeWebhookEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if err := webhooks.ValidateURL(raw, nil); err != nil {
		return "", err
	}
	return raw, nil
}

func scanWebhookSubscription(row interface{ Scan(...interface{}) error }) (WebhookSubscription, error) {
	var subscription WebhookSubscription
	err := row.Scan(&subscription.ID, &subscription.TenantID, &subscription.Name, &subscription.EndpointURL,
		&subscription.Events, &subscription.HasSigningSecret, &subscription.Active, &subscription.LastDeliveryAt,
		&subscription.CreatedAt, &subscription.UpdatedAt)
	return subscription, err
}

const webhookSubscriptionColumns = `id, tenant_id, name, endpoint_url, events,
	(signing_secret_hash IS NOT NULL), active, last_delivery_at, created_at, updated_at`

func (s *Store) CreateWebhookSubscription(ctx context.Context, input WebhookSubscriptionInput) (WebhookSubscription, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return WebhookSubscription{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return WebhookSubscription{}, errors.New("webhook name required")
	}
	endpoint, err := normalizeWebhookEndpoint(input.EndpointURL)
	if err != nil {
		return WebhookSubscription{}, err
	}
	events := normalizeWebhookEvents(input.Events)
	var signingSecretHash, signingSecretCiphertext, signingSecretNonce []byte
	if strings.TrimSpace(input.SigningSecret) != "" {
		signingSecretHash = hashPublicSecret(input.SigningSecret)
		signingSecretCiphertext, signingSecretNonce, err = s.encryptWebhookSigningSecret(input.SigningSecret)
		if err != nil {
			return WebhookSubscription{}, err
		}
	}
	return scanWebhookSubscription(s.pool.QueryRow(ctx, `INSERT INTO webhook_subscriptions
		(tenant_id, name, endpoint_url, events, signing_secret_hash, signing_secret_ciphertext, signing_secret_nonce, active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING `+webhookSubscriptionColumns, tenantID, name, endpoint, events, signingSecretHash, signingSecretCiphertext, signingSecretNonce, input.Active))
}

func (s *Store) ListWebhookSubscriptions(ctx context.Context) ([]WebhookSubscription, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT `+webhookSubscriptionColumns+` FROM webhook_subscriptions
		WHERE tenant_id=$1 ORDER BY id DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]WebhookSubscription, 0)
	for rows.Next() {
		item, err := scanWebhookSubscription(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetWebhookSubscription(ctx context.Context, id int64) (WebhookSubscription, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return WebhookSubscription{}, err
	}
	return scanWebhookSubscription(s.pool.QueryRow(ctx, `SELECT `+webhookSubscriptionColumns+`
		FROM webhook_subscriptions WHERE tenant_id=$1 AND id=$2`, tenantID, id))
}

func (s *Store) UpdateWebhookSubscription(ctx context.Context, input WebhookSubscriptionInput) error {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return errors.New("webhook name required")
	}
	endpoint, err := normalizeWebhookEndpoint(input.EndpointURL)
	if err != nil {
		return err
	}
	events := normalizeWebhookEvents(input.Events)
	var tag pgconn.CommandTag
	if strings.TrimSpace(input.SigningSecret) != "" {
		ciphertext, nonce, err := s.encryptWebhookSigningSecret(input.SigningSecret)
		if err != nil {
			return err
		}
		tag, err = s.pool.Exec(ctx, `UPDATE webhook_subscriptions SET
			name=$1, endpoint_url=$2, events=$3, signing_secret_hash=$4, signing_secret_ciphertext=$5, signing_secret_nonce=$6, active=$7, updated_at=now()
			WHERE tenant_id=$8 AND id=$9`, name, endpoint, events, hashPublicSecret(input.SigningSecret), ciphertext, nonce, input.Active, tenantID, input.ID)
	} else {
		tag, err = s.pool.Exec(ctx, `UPDATE webhook_subscriptions SET
			name=$1, endpoint_url=$2, events=$3, active=$4, updated_at=now()
			WHERE tenant_id=$5 AND id=$6`, name, endpoint, events, input.Active, tenantID, input.ID)
	}
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

// WebhookSigningSecret devolve o segredo original apenas ao worker interno.
// O contexto de tenant é obrigatório e a consulta sempre inclui tenant_id,
// impedindo acesso cruzado entre empresas. O segredo não faz parte de nenhuma
// estrutura serializável nem é registrado em logs.
func (s *Store) WebhookSigningSecret(ctx context.Context, subscriptionID int64) (string, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return "", err
	}
	var ciphertext, nonce []byte
	err = s.pool.QueryRow(ctx, `SELECT signing_secret_ciphertext, signing_secret_nonce
		FROM webhook_subscriptions WHERE tenant_id=$1 AND id=$2`, tenantID, subscriptionID).Scan(&ciphertext, &nonce)
	if err != nil {
		return "", err
	}
	return s.decryptWebhookSigningSecret(ciphertext, nonce)
}

func (s *Store) SetWebhookSubscriptionActive(ctx context.Context, id int64, active bool) error {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE webhook_subscriptions SET active=$1, updated_at=now()
		WHERE tenant_id=$2 AND id=$3`, active, tenantID, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func (s *Store) DeleteWebhookSubscription(ctx context.Context, id int64) error {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM webhook_subscriptions WHERE tenant_id=$1 AND id=$2`, tenantID, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

const webhookDeliveryColumns = `id, tenant_id, subscription_id, event_type, event_id, payload, status, attempts,
	next_attempt_at, last_attempt_at, COALESCE(response_status, 0), COALESCE(response_body, ''),
	COALESCE(error_message, ''), delivered_at, created_at, updated_at`

func scanWebhookDelivery(row interface{ Scan(...interface{}) error }) (WebhookDelivery, error) {
	var delivery WebhookDelivery
	err := row.Scan(&delivery.ID, &delivery.TenantID, &delivery.SubscriptionID, &delivery.EventType, &delivery.EventID,
		&delivery.Payload, &delivery.Status, &delivery.Attempts, &delivery.NextAttemptAt, &delivery.LastAttemptAt,
		&delivery.ResponseStatus, &delivery.ResponseBody, &delivery.ErrorMessage, &delivery.DeliveredAt,
		&delivery.CreatedAt, &delivery.UpdatedAt)
	return delivery, err
}

func (s *Store) CreateWebhookDelivery(ctx context.Context, subscriptionID int64, eventType, eventID string, payload []byte, nextAttemptAt time.Time) (WebhookDelivery, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return WebhookDelivery{}, err
	}
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return WebhookDelivery{}, errors.New("webhook event type required")
	}
	if !json.Valid(payload) {
		return WebhookDelivery{}, errors.New("webhook payload must be valid json")
	}
	if nextAttemptAt.IsZero() {
		nextAttemptAt = time.Now().UTC()
	}
	eventID = strings.TrimSpace(eventID)
	return scanWebhookDelivery(s.pool.QueryRow(ctx, `INSERT INTO webhook_deliveries
		(tenant_id, subscription_id, event_type, event_id, payload, next_attempt_at)
		SELECT $1,$2,$3,$4,$5,$6
		WHERE EXISTS (SELECT 1 FROM webhook_subscriptions WHERE tenant_id=$1 AND id=$2)
		RETURNING `+webhookDeliveryColumns, tenantID, subscriptionID, eventType, eventID, payload, nextAttemptAt))
}

func (s *Store) GetWebhookDelivery(ctx context.Context, id int64) (WebhookDelivery, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return WebhookDelivery{}, err
	}
	return scanWebhookDelivery(s.pool.QueryRow(ctx, `SELECT `+webhookDeliveryColumns+`
		FROM webhook_deliveries WHERE tenant_id=$1 AND id=$2`, tenantID, id))
}

func (s *Store) ListWebhookDeliveries(ctx context.Context, subscriptionID *int64, status string, limit, offset int) ([]WebhookDelivery, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	query := `SELECT ` + webhookDeliveryColumns + ` FROM webhook_deliveries WHERE tenant_id=$1`
	args := []interface{}{tenantID}
	if subscriptionID != nil {
		args = append(args, *subscriptionID)
		query += ` AND subscription_id=$` + strconv.Itoa(len(args))
	}
	if status != "" {
		status = strings.TrimSpace(strings.ToLower(status))
		if !validWebhookDeliveryStatus(status) {
			return nil, errors.New("invalid webhook delivery status")
		}
		args = append(args, status)
		query += ` AND status=$` + strconv.Itoa(len(args))
	}
	args = append(args, limit, offset)
	query += ` ORDER BY id DESC LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]WebhookDelivery, 0)
	for rows.Next() {
		item, err := scanWebhookDelivery(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func validWebhookDeliveryStatus(status string) bool {
	switch status {
	case "pending", "processing", "failed", "delivered", "dead_letter":
		return true
	default:
		return false
	}
}

func (s *Store) ClaimDueWebhookDeliveries(ctx context.Context, subscriptionID *int64, now time.Time, limit int) ([]WebhookDelivery, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return nil, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit < 1 || limit > 100 {
		limit = 25
	}
	args := []interface{}{tenantID, now}
	query := `WITH due AS (
		SELECT id FROM webhook_deliveries
		WHERE tenant_id=$1 AND status IN ('pending', 'failed') AND next_attempt_at <= $2
		  AND EXISTS (SELECT 1 FROM webhook_subscriptions ws
			WHERE ws.tenant_id=webhook_deliveries.tenant_id AND ws.id=webhook_deliveries.subscription_id AND ws.active)`
	if subscriptionID != nil {
		args = append(args, *subscriptionID)
		query += ` AND subscription_id=$` + strconv.Itoa(len(args))
	}
	args = append(args, limit)
	limitArg := strconv.Itoa(len(args))
	query += ` ORDER BY id ASC LIMIT $` + limitArg + ` FOR UPDATE SKIP LOCKED
	)
	UPDATE webhook_deliveries d SET status='processing', last_attempt_at=$2, updated_at=now()
	FROM due WHERE d.tenant_id=$1 AND d.id=due.id
	RETURNING ` + webhookDeliveryColumns
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]WebhookDelivery, 0)
	for rows.Next() {
		item, err := scanWebhookDelivery(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) RecordWebhookDeliveryAttempt(ctx context.Context, id int64, responseStatus int, responseBody, deliveryError string, nextAttemptAt *time.Time, maxAttempts int) (WebhookDelivery, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return WebhookDelivery{}, err
	}
	if maxAttempts < 1 {
		maxAttempts = 5
	}
	now := time.Now().UTC()
	next := now
	if nextAttemptAt != nil && !nextAttemptAt.IsZero() {
		next = nextAttemptAt.UTC()
	}
	success := responseStatus >= 200 && responseStatus < 300 && strings.TrimSpace(deliveryError) == ""
	var delivery WebhookDelivery
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		delivery, err = scanWebhookDelivery(tx.QueryRow(ctx, `UPDATE webhook_deliveries SET
			attempts=attempts+1,
			status=CASE WHEN $3 THEN 'delivered' WHEN attempts+1 >= $4 THEN 'dead_letter' ELSE 'failed' END,
			next_attempt_at=CASE WHEN $3 THEN now() ELSE $5 END,
			response_status=$6, response_body=NULLIF($7,''),
			error_message=CASE WHEN $3 THEN NULL ELSE NULLIF($8,'') END,
			delivered_at=CASE WHEN $3 THEN now() ELSE NULL END,
			updated_at=now()
			WHERE tenant_id=$1 AND id=$2 AND status='processing'
			RETURNING `+webhookDeliveryColumns, tenantID, id, success, maxAttempts, next, responseStatus, responseBody, deliveryError))
		if err != nil {
			return err
		}
		if delivery.Status == "delivered" {
			_, err = tx.Exec(ctx, `UPDATE webhook_subscriptions SET last_delivery_at=now(), updated_at=now()
				WHERE tenant_id=$1 AND id=$2`, tenantID, delivery.SubscriptionID)
		}
		return err
	})
	if err != nil {
		return WebhookDelivery{}, err
	}
	return delivery, nil
}
