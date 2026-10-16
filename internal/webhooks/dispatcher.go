package webhooks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Client  *http.Client
	Store   DeliveryStore
	Limits  Limits
	Clock   func() time.Time
	Sleeper Sleeper
}

type Dispatcher struct {
	client  *http.Client
	store   DeliveryStore
	limits  Limits
	clock   func() time.Time
	sleeper Sleeper
	limiter *requestLimiter
}

func NewDispatcher(config Config) (*Dispatcher, error) {
	limits, err := config.Limits.normalized()
	if err != nil {
		return nil, err
	}
	clock := config.Clock
	if clock == nil {
		clock = time.Now
	}
	sleeper := config.Sleeper
	if sleeper == nil {
		sleeper = sleepContext
	}
	client := config.Client
	if client == nil {
		client = &http.Client{}
	}
	return &Dispatcher{client: client, store: config.Store, limits: limits, clock: clock, sleeper: sleeper, limiter: &requestLimiter{calls: map[string][]time.Time{}}}, nil
}

func (d *Dispatcher) Dispatch(ctx context.Context, request Request) (Result, error) {
	if strings.TrimSpace(request.DeliveryID) == "" {
		request.DeliveryID = newDeliveryID()
	}
	if err := validateRequest(request, d.limits); err != nil {
		return Result{DeliveryID: request.DeliveryID, Status: StatusBlocked, Error: err.Error()}, err
	}
	createdAt := d.clock().UTC()
	body, err := marshalEnvelope(request, createdAt)
	if err != nil {
		return Result{DeliveryID: request.DeliveryID, Status: StatusBlocked, Error: err.Error()}, err
	}
	if d.store != nil {
		record := DeliveryRecord{ID: request.DeliveryID, TenantID: request.TenantID, EndpointID: request.EndpointID, Event: request.Event, URL: persistedURL(request.URL), PayloadSHA256: PayloadHash(body), CreatedAt: createdAt}
		if err := d.store.CreateDelivery(ctx, record); err != nil {
			return Result{DeliveryID: request.DeliveryID, Status: StatusPersistenceError, Error: err.Error()}, err
		}
	}
	for attempt := 1; attempt <= d.limits.MaxAttempts; attempt++ {
		result := Result{DeliveryID: request.DeliveryID, Attempts: attempt}
		if !d.limiter.Allow(limiterKey(request.URL), d.limits.RequestsPerMinute, d.clock()) {
			result.Status = StatusRateLimited
			result.Error = "limite de requisições por minuto atingido"
			if err := d.persist(ctx, DeliveryUpdate{ID: request.DeliveryID, Status: result.Status, Attempt: attempt, Error: result.Error, UpdatedAt: d.clock().UTC()}); err != nil {
				return persistenceResult(result, err)
			}
			return result, nil
		}
		response, requestErr := d.send(ctx, request, body, createdAt, attempt)
		result.HTTPStatus = response.status
		if requestErr == "" && response.status >= 200 && response.status < 300 {
			result.Status = StatusDelivered
			if err := d.persist(ctx, DeliveryUpdate{ID: request.DeliveryID, Status: result.Status, Attempt: attempt, HTTPStatus: result.HTTPStatus, CompletedAt: timePointer(d.clock().UTC()), UpdatedAt: d.clock().UTC()}); err != nil {
				return persistenceResult(result, err)
			}
			return result, nil
		}
		result.Error = requestErr
		if result.Error == "" {
			result.Error = fmt.Sprintf("endpoint respondeu HTTP %d", response.status)
		}
		retry := requestErr != "" || retryableStatus(response.status)
		if response.bodyTooLarge {
			retry = false
			result.Error = "resposta do endpoint excedeu o limite permitido"
		}
		if !retry || attempt == d.limits.MaxAttempts {
			result.Status = StatusFailed
			if err := d.persist(ctx, DeliveryUpdate{ID: request.DeliveryID, Status: result.Status, Attempt: attempt, HTTPStatus: result.HTTPStatus, Error: result.Error, CompletedAt: timePointer(d.clock().UTC()), UpdatedAt: d.clock().UTC()}); err != nil {
				return persistenceResult(result, err)
			}
			return result, nil
		}
		delay := response.retryAfter
		if delay <= 0 || delay > d.limits.MaxBackoff {
			delay = BackoffDelay(d.limits, attempt)
		}
		nextAttempt := d.clock().UTC().Add(delay)
		if err := d.persist(ctx, DeliveryUpdate{ID: request.DeliveryID, Status: StatusPending, Attempt: attempt, HTTPStatus: result.HTTPStatus, Error: result.Error, NextAttemptAt: &nextAttempt, UpdatedAt: d.clock().UTC()}); err != nil {
			return persistenceResult(result, err)
		}
		if err := d.sleeper(ctx, delay); err != nil {
			result.Status = StatusFailed
			result.Error = err.Error()
			return result, err
		}
	}
	return Result{DeliveryID: request.DeliveryID, Status: StatusFailed, Error: "dispatcher terminou sem resultado"}, errors.New("dispatcher terminou sem resultado")
}

func BackoffDelay(limits Limits, attempt int) time.Duration {
	if attempt < 1 {
		return 0
	}
	initial := limits.InitialBackoff
	maximum := limits.MaxBackoff
	if initial <= 0 {
		initial = DefaultLimits().InitialBackoff
	}
	if maximum <= 0 {
		maximum = DefaultLimits().MaxBackoff
	}
	delay := initial
	for index := 1; index < attempt && delay < maximum; index++ {
		if delay > maximum/2 {
			return maximum
		}
		delay *= 2
	}
	if delay > maximum {
		return maximum
	}
	return delay
}

func (d *Dispatcher) persist(ctx context.Context, update DeliveryUpdate) error {
	if d.store == nil {
		return nil
	}
	return d.store.UpdateDelivery(ctx, update)
}

func (d *Dispatcher) send(ctx context.Context, request Request, body []byte, timestamp time.Time, attempt int) (responseInfo, string) {
	signature, err := Sign(request.Secret, timestamp, body)
	if err != nil {
		return responseInfo{}, err.Error()
	}
	requestContext, cancel := context.WithTimeout(ctx, d.limits.RequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestContext, http.MethodPost, request.URL, strings.NewReader(string(body)))
	if err != nil {
		return responseInfo{}, "não foi possível montar a requisição"
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Notreve-Webhooks/1")
	req.Header.Set("X-Notreve-Webhook-Id", request.DeliveryID)
	req.Header.Set("X-Notreve-Webhook-Event", string(request.Event))
	req.Header.Set("X-Notreve-Webhook-Timestamp", strconv.FormatInt(timestamp.Unix(), 10))
	req.Header.Set("X-Notreve-Webhook-Signature", signature)
	req.Header.Set("X-Notreve-Webhook-Attempt", strconv.Itoa(attempt))
	for key, value := range request.Headers {
		req.Header.Set(key, value)
	}
	client := d.clientFor(request)
	resp, err := client.Do(req)
	if err != nil {
		return responseInfo{}, "erro de transporte ao entregar webhook"
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, int64(d.limits.MaxResponseBytes)+1))
	if readErr != nil {
		return responseInfo{status: resp.StatusCode}, "erro ao ler resposta do endpoint"
	}
	info := responseInfo{status: resp.StatusCode, retryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), d.clock(), d.limits.MaxBackoff)}
	if len(data) > d.limits.MaxResponseBytes {
		info.bodyTooLarge = true
	}
	return info, ""
}

func (d *Dispatcher) clientFor(request Request) *http.Client {
	client := *d.client
	client.CheckRedirect = func(next *http.Request, _ []*http.Request) error {
		return ValidateURL(next.URL.String(), request.AllowedHosts)
	}
	transport, ok := d.client.Transport.(*http.Transport)
	if d.client.Transport == nil {
		transport, ok = http.DefaultTransport.(*http.Transport)
	}
	if ok && transport != nil {
		clone := transport.Clone()
		clone.DialContext = safeDialContext
		client.Transport = clone
	}
	return &client
}

type responseInfo struct {
	status       int
	retryAfter   time.Duration
	bodyTooLarge bool
}

func retryableStatus(status int) bool {
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

func marshalEnvelope(request Request, timestamp time.Time) ([]byte, error) {
	var data interface{}
	if err := json.Unmarshal(request.Payload, &data); err != nil {
		return nil, errors.New("payload deve ser um JSON válido")
	}
	envelope := struct {
		ID        string      `json:"id"`
		Event     Event       `json:"event"`
		Version   int         `json:"version"`
		Timestamp int64       `json:"timestamp"`
		Data      interface{} `json:"data"`
	}{ID: request.DeliveryID, Event: request.Event, Version: 1, Timestamp: timestamp.Unix(), Data: data}
	return json.Marshal(envelope)
}

func newDeliveryID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err == nil {
		return "wh_" + hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("wh_%d", time.Now().UnixNano())
}

func persistenceResult(result Result, err error) (Result, error) {
	result.Status = StatusPersistenceError
	result.Error = err.Error()
	return result, err
}

func timePointer(value time.Time) *time.Time {
	return &value
}

func persistedURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func limiterKey(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host + parsed.Path)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type requestLimiter struct {
	mu    sync.Mutex
	calls map[string][]time.Time
}

func (limiter *requestLimiter) Allow(key string, limit int, now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	cutoff := now.Add(-time.Minute)
	values := limiter.calls[key]
	kept := values[:0]
	for _, value := range values {
		if value.After(cutoff) {
			kept = append(kept, value)
		}
	}
	if len(kept) >= limit {
		limiter.calls[key] = kept
		return false
	}
	limiter.calls[key] = append(kept, now)
	return true
}

func (request Request) WithAllowedHosts(hosts ...string) Request {
	request.AllowedHosts = append([]string{}, hosts...)
	return request
}
