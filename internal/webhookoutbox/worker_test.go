package webhookoutbox

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"github.com/notrevetecnologia/notreve-chat-community/internal/webhooks"
)

type fakeStore struct {
	claimed []store.WebhookOutboxDelivery
	records []store.WebhookOutboxAttemptResult
}

func (s *fakeStore) ClaimDueWebhookOutboxDeliveries(_ context.Context, _ time.Time, _ time.Duration, _ int) ([]store.WebhookOutboxDelivery, error) {
	return append([]store.WebhookOutboxDelivery(nil), s.claimed...), nil
}

func (s *fakeStore) RecordWebhookOutboxAttempt(_ context.Context, _ int64, attempt int, result store.WebhookOutboxAttemptResult) (store.WebhookOutboxDelivery, error) {
	s.records = append(s.records, result)
	status := "dead_letter"
	if result.ResponseStatus >= 200 && result.ResponseStatus < 300 && result.ErrorCode == "" {
		status = "delivered"
	} else if result.Retryable && attempt < s.claimed[0].MaxAttempts {
		status = "retry_scheduled"
	}
	return store.WebhookOutboxDelivery{Status: status}, nil
}

type fakeClient struct {
	requests []*http.Request
	response *http.Response
	err      error
}

func (c *fakeClient) Do(request *http.Request) (*http.Response, error) {
	c.requests = append(c.requests, request)
	return c.response, c.err
}

func testDelivery() store.WebhookOutboxDelivery {
	return store.WebhookOutboxDelivery{ID: 41, TenantID: 7, EventID: 8, SubscriptionID: 9,
		EndpointURL: "https://hooks.example.com/notreve", EventKey: "message.sent:42",
		EventType: webhooks.EventMessageSent, Payload: []byte(`{"message_id":42}`), Attempts: 1, MaxAttempts: 5}
}

func newTestWorker(t *testing.T, s *fakeStore, now time.Time) *Worker {
	t.Helper()
	worker, err := NewWorker(Config{Store: s, Clock: func() time.Time { return now }, InitialBackoff: 2 * time.Second, MaxBackoff: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return worker
}

func TestWorkerDeliversUnsignedWebhookWhenNoSecretExists(t *testing.T) {
	now := time.Date(2026, 8, 22, 18, 0, 0, 0, time.UTC)
	delivery := testDelivery()
	s := &fakeStore{claimed: []store.WebhookOutboxDelivery{delivery}}
	worker := newTestWorker(t, s, now)
	client := &fakeClient{response: &http.Response{StatusCode: http.StatusAccepted, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}}
	worker.client = client
	result, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Delivered != 1 || len(s.records) != 1 || s.records[0].ResponseStatus != http.StatusAccepted {
		t.Fatalf("unexpected result: %#v, records: %#v", result, s.records)
	}
	if len(client.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(client.requests))
	}
	request := client.requests[0]
	if _, exists := request.Header["X-Notreve-Webhook-Signature"]; exists {
		t.Fatal("unsigned subscription must omit the signature header")
	}
	if got := request.Header.Get("X-Notreve-Webhook-Id"); got != delivery.EventKey {
		t.Fatalf("event id header = %q", got)
	}
	body, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`"id":"message.sent:42"`)) {
		t.Fatalf("unexpected envelope: %s", body)
	}
}

func TestWorkerDeadLettersWhenSigningSecretIsUnrecoverable(t *testing.T) {
	delivery := testDelivery()
	delivery.HasSigningSecret = true
	s := &fakeStore{claimed: []store.WebhookOutboxDelivery{delivery}}
	worker := newTestWorker(t, s, time.Now())
	client := &fakeClient{response: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}}
	worker.client = client
	result, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.DeadLettered != 1 || len(s.records) != 1 || s.records[0].ErrorCode != "signing_secret_unrecoverable" {
		t.Fatalf("unexpected result: %#v, records: %#v", result, s.records)
	}
	if len(client.requests) != 0 {
		t.Fatal("a webhook requiring an unrecoverable signature must not be sent")
	}
}

func TestWorkerSchedulesExponentialRetryForTemporaryHTTPFailure(t *testing.T) {
	now := time.Date(2026, 8, 22, 18, 0, 0, 0, time.UTC)
	delivery := testDelivery()
	delivery.Attempts = 2
	s := &fakeStore{claimed: []store.WebhookOutboxDelivery{delivery}}
	worker := newTestWorker(t, s, now)
	worker.client = &fakeClient{response: &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("temporary")), Header: make(http.Header)}}
	result, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Retried != 1 || len(s.records) != 1 {
		t.Fatalf("unexpected result: %#v, records: %#v", result, s.records)
	}
	if !s.records[0].Retryable || s.records[0].NextAttemptAt == nil {
		t.Fatalf("retry was not scheduled: %#v", s.records[0])
	}
	want := now.Add(4 * time.Second)
	if !s.records[0].NextAttemptAt.Equal(want) {
		t.Fatalf("next attempt = %s, want %s", s.records[0].NextAttemptAt, want)
	}
}

func TestWorkerBlocksUnsafeEndpointWithoutSending(t *testing.T) {
	delivery := testDelivery()
	delivery.EndpointURL = "http://127.0.0.1:8080/webhook"
	s := &fakeStore{claimed: []store.WebhookOutboxDelivery{delivery}}
	worker := newTestWorker(t, s, time.Now())
	client := &fakeClient{}
	worker.client = client
	result, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.DeadLettered != 1 || len(s.records) != 1 || s.records[0].ErrorCode != "invalid_endpoint" {
		t.Fatalf("unexpected result: %#v, records: %#v", result, s.records)
	}
	if len(client.requests) != 0 {
		t.Fatal("unsafe endpoint must not receive a request")
	}
}

func TestNewWorkerRequiresStoreAndValidLease(t *testing.T) {
	if _, err := NewWorker(Config{}); err == nil {
		t.Fatal("worker without store should fail")
	}
	if _, err := NewWorker(Config{Store: &fakeStore{}, LeaseDuration: time.Second, RequestTimeout: time.Second}); err == nil {
		t.Fatal("lease equal to request timeout should fail")
	}
}
