// Package webhookoutbox executa a entrega confiável dos eventos já gravados na
// outbox. Ele depende de store e webhooks, mas nenhum desses pacotes depende
// dele, evitando um ciclo de importação entre persistência e transporte.
package webhookoutbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"github.com/notrevetecnologia/notreve-chat-community/internal/webhooks"
)

const (
	defaultBatchSize        = 25
	defaultLeaseDuration    = 2 * time.Minute
	defaultRequestTimeout   = 15 * time.Second
	defaultInitialBackoff   = time.Second
	defaultMaxBackoff       = time.Minute
	defaultMaxResponseBytes = 256 << 10
)

type DeliveryStore interface {
	ClaimDueWebhookOutboxDeliveries(context.Context, time.Time, time.Duration, int) ([]store.WebhookOutboxDelivery, error)
	RecordWebhookOutboxAttempt(context.Context, int64, int, store.WebhookOutboxAttemptResult) (store.WebhookOutboxDelivery, error)
}

type Config struct {
	Store            DeliveryStore
	Clock            func() time.Time
	BatchSize        int
	LeaseDuration    time.Duration
	RequestTimeout   time.Duration
	InitialBackoff   time.Duration
	MaxBackoff       time.Duration
	MaxResponseBytes int64
}

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Worker struct {
	store            DeliveryStore
	client           httpDoer
	clock            func() time.Time
	batchSize        int
	leaseDuration    time.Duration
	requestTimeout   time.Duration
	initialBackoff   time.Duration
	maxBackoff       time.Duration
	maxResponseBytes int64
}

type RunResult struct {
	Claimed      int
	Delivered    int
	Retried      int
	DeadLettered int
}

func NewWorker(config Config) (*Worker, error) {
	if config.Store == nil {
		return nil, errors.New("webhook outbox store is required")
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.BatchSize == 0 {
		config.BatchSize = defaultBatchSize
	}
	if config.LeaseDuration == 0 {
		config.LeaseDuration = defaultLeaseDuration
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = defaultRequestTimeout
	}
	if config.InitialBackoff == 0 {
		config.InitialBackoff = defaultInitialBackoff
	}
	if config.MaxBackoff == 0 {
		config.MaxBackoff = defaultMaxBackoff
	}
	if config.MaxResponseBytes == 0 {
		config.MaxResponseBytes = defaultMaxResponseBytes
	}
	if config.BatchSize < 1 || config.BatchSize > 100 {
		return nil, errors.New("webhook outbox batch size must be between 1 and 100")
	}
	if config.RequestTimeout < time.Second || config.RequestTimeout > 2*time.Minute {
		return nil, errors.New("webhook outbox request timeout is invalid")
	}
	if config.LeaseDuration <= config.RequestTimeout || config.LeaseDuration > 30*time.Minute {
		return nil, errors.New("webhook outbox lease duration must exceed request timeout and be at most 30 minutes")
	}
	if config.InitialBackoff < time.Second || config.MaxBackoff < config.InitialBackoff || config.MaxBackoff > time.Hour {
		return nil, errors.New("webhook outbox retry backoff is invalid")
	}
	if config.MaxResponseBytes < 1024 || config.MaxResponseBytes > 4<<20 {
		return nil, errors.New("webhook outbox response limit is invalid")
	}
	return &Worker{
		store: config.Store, client: webhooks.NewSecureOutboundHTTPClient(), clock: config.Clock,
		batchSize: config.BatchSize, leaseDuration: config.LeaseDuration, requestTimeout: config.RequestTimeout,
		initialBackoff: config.InitialBackoff, maxBackoff: config.MaxBackoff, maxResponseBytes: config.MaxResponseBytes,
	}, nil
}

// RunOnce executa um lote para o tenant presente no contexto. O chamador deve
// percorrer os tenants ativos com store.WithTenantID; o worker não escolhe um
// tenant padrão para não vazar entregas entre empresas.
func (w *Worker) RunOnce(ctx context.Context) (RunResult, error) {
	deliveries, err := w.store.ClaimDueWebhookOutboxDeliveries(ctx, w.clock().UTC(), w.leaseDuration, w.batchSize)
	if err != nil {
		return RunResult{}, err
	}
	result := RunResult{Claimed: len(deliveries)}
	for _, delivery := range deliveries {
		attempt := w.deliver(ctx, delivery)
		updated, err := w.store.RecordWebhookOutboxAttempt(ctx, delivery.ID, delivery.Attempts, attempt)
		if err != nil {
			return result, fmt.Errorf("record webhook outbox delivery %d: %w", delivery.ID, err)
		}
		switch updated.Status {
		case "delivered":
			result.Delivered++
		case "retry_scheduled":
			result.Retried++
		case "dead_letter":
			result.DeadLettered++
		}
	}
	return result, nil
}

func (w *Worker) deliver(ctx context.Context, delivery store.WebhookOutboxDelivery) store.WebhookOutboxAttemptResult {
	// signing_secret_hash é propositalmente irreversível. Nunca se deve usar o
	// hash como segredo HMAC nem enviar uma assinatura simulada.
	if delivery.HasSigningSecret {
		return store.WebhookOutboxAttemptResult{ErrorCode: "signing_secret_unrecoverable", ErrorMessage: "A assinatura do webhook está configurada, mas o segredo de saída não pode ser recuperado com segurança."}
	}
	if err := webhooks.ValidateURL(delivery.EndpointURL, nil); err != nil {
		return store.WebhookOutboxAttemptResult{ErrorCode: "invalid_endpoint", ErrorMessage: "O endpoint do webhook não atende às regras de segurança."}
	}
	body, err := marshalEnvelope(delivery, w.clock().UTC())
	if err != nil {
		return store.WebhookOutboxAttemptResult{ErrorCode: "invalid_payload", ErrorMessage: "O payload persistido do webhook não é um JSON válido."}
	}
	requestContext, cancel := context.WithTimeout(ctx, w.requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestContext, http.MethodPost, delivery.EndpointURL, strings.NewReader(string(body)))
	if err != nil {
		return store.WebhookOutboxAttemptResult{ErrorCode: "invalid_endpoint", ErrorMessage: "Não foi possível preparar a requisição para o endpoint do webhook."}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Notreve-Webhooks/1")
	req.Header.Set("X-Notreve-Webhook-Id", delivery.EventKey)
	req.Header.Set("X-Notreve-Webhook-Delivery-Id", strconv.FormatInt(delivery.ID, 10))
	req.Header.Set("X-Notreve-Webhook-Event", string(delivery.EventType))
	req.Header.Set("X-Notreve-Webhook-Timestamp", strconv.FormatInt(w.clock().UTC().Unix(), 10))
	req.Header.Set("X-Notreve-Webhook-Attempt", strconv.Itoa(delivery.Attempts))
	// Sem segredo configurado, X-Notreve-Webhook-Signature é deliberadamente omitido.

	response, err := w.client.Do(req)
	if err != nil {
		return w.retryResult(0, "transport_error", "Não foi possível entregar o webhook ao endpoint.", delivery)
	}
	defer response.Body.Close()
	read, err := io.Copy(io.Discard, io.LimitReader(response.Body, w.maxResponseBytes+1))
	if err != nil {
		return w.retryResult(response.StatusCode, "response_read_error", "Não foi possível ler a resposta do endpoint do webhook.", delivery)
	}
	if read > w.maxResponseBytes {
		return store.WebhookOutboxAttemptResult{ResponseStatus: response.StatusCode, ErrorCode: "response_body_too_large", ErrorMessage: "A resposta do endpoint do webhook excede o limite permitido."}
	}
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return store.WebhookOutboxAttemptResult{ResponseStatus: response.StatusCode}
	}
	if retryableHTTPStatus(response.StatusCode) {
		return w.retryResult(response.StatusCode, "http_retryable", "O endpoint do webhook respondeu com falha temporária.", delivery, response.Header.Get("Retry-After"))
	}
	return store.WebhookOutboxAttemptResult{ResponseStatus: response.StatusCode, ErrorCode: "http_non_retryable", ErrorMessage: "O endpoint do webhook respondeu com uma falha não recuperável."}
}

func (w *Worker) retryResult(responseStatus int, errorCode, errorMessage string, delivery store.WebhookOutboxDelivery, retryAfter ...string) store.WebhookOutboxAttemptResult {
	result := store.WebhookOutboxAttemptResult{ResponseStatus: responseStatus, ErrorCode: errorCode, ErrorMessage: errorMessage, Retryable: true}
	if delivery.Attempts >= delivery.MaxAttempts {
		return result
	}
	delay := time.Duration(0)
	if len(retryAfter) > 0 {
		delay = parseRetryAfter(retryAfter[0], w.clock().UTC(), w.maxBackoff)
	}
	if delay <= 0 {
		delay = webhooks.BackoffDelay(webhooks.Limits{InitialBackoff: w.initialBackoff, MaxBackoff: w.maxBackoff}, delivery.Attempts)
	}
	nextAttempt := w.clock().UTC().Add(delay)
	result.NextAttemptAt = &nextAttempt
	return result
}

func marshalEnvelope(delivery store.WebhookOutboxDelivery, timestamp time.Time) ([]byte, error) {
	var data any
	if err := json.Unmarshal(delivery.Payload, &data); err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		ID        string         `json:"id"`
		Event     webhooks.Event `json:"event"`
		Version   int            `json:"version"`
		Timestamp int64          `json:"timestamp"`
		Data      any            `json:"data"`
	}{ID: delivery.EventKey, Event: delivery.EventType, Version: 1, Timestamp: timestamp.Unix(), Data: data})
}

func retryableHTTPStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooEarly || status == http.StatusTooManyRequests || status >= 500 && status <= 599
}

func parseRetryAfter(value string, now time.Time, maximum time.Duration) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		delay := time.Duration(seconds) * time.Second
		if delay > maximum {
			return maximum
		}
		return delay
	}
	if when, err := http.ParseTime(value); err == nil && when.After(now) {
		delay := when.Sub(now)
		if delay > maximum {
			return maximum
		}
		return delay
	}
	return 0
}
