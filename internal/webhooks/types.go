package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Event string

const (
	EventConversationCreated   Event = "conversation.created"
	EventConversationUpdated   Event = "conversation.updated"
	EventConversationClosed    Event = "conversation.closed"
	EventMessageReceived       Event = "message.received"
	EventMessageSent           Event = "message.sent"
	EventContactCreated        Event = "contact.created"
	EventContactUpdated        Event = "contact.updated"
	EventAgentTransferred      Event = "agent.transferred"
	EventSatisfactionRequested Event = "satisfaction.requested"
	EventSatisfactionCompleted Event = "satisfaction.completed"
	EventCampaignCompleted     Event = "campaign.completed"
	EventChannelStatusChanged  Event = "channel.status_changed"
)

type EventDefinition struct {
	Key         Event  `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     int    `json:"version"`
}

func Catalog() []EventDefinition {
	return []EventDefinition{
		{Key: EventConversationCreated, Name: "Conversa criada", Description: "Uma nova conversa foi iniciada.", Version: 1},
		{Key: EventConversationUpdated, Name: "Conversa atualizada", Description: "Dados ou estado de uma conversa foram atualizados.", Version: 1},
		{Key: EventConversationClosed, Name: "Conversa encerrada", Description: "Uma conversa foi encerrada.", Version: 1},
		{Key: EventMessageReceived, Name: "Mensagem recebida", Description: "Uma mensagem de cliente foi recebida.", Version: 1},
		{Key: EventMessageSent, Name: "Mensagem enviada", Description: "Uma mensagem foi enviada ao cliente.", Version: 1},
		{Key: EventContactCreated, Name: "Contato criado", Description: "Um contato foi criado.", Version: 1},
		{Key: EventContactUpdated, Name: "Contato atualizado", Description: "Dados de um contato foram atualizados.", Version: 1},
		{Key: EventAgentTransferred, Name: "Atendimento transferido", Description: "Uma conversa mudou de agente ou setor.", Version: 1},
		{Key: EventSatisfactionRequested, Name: "Avaliação solicitada", Description: "Uma avaliação foi solicitada ao cliente.", Version: 1},
		{Key: EventSatisfactionCompleted, Name: "Avaliação concluída", Description: "Uma avaliação foi registrada.", Version: 1},
		{Key: EventCampaignCompleted, Name: "Campanha concluída", Description: "Uma campanha terminou seu processamento.", Version: 1},
		{Key: EventChannelStatusChanged, Name: "Status de canal alterado", Description: "O status operacional de um canal foi alterado.", Version: 1},
	}
}

func IsKnownEvent(event Event) bool {
	for _, definition := range Catalog() {
		if definition.Key == event {
			return true
		}
	}
	return false
}

type Request struct {
	DeliveryID   string
	TenantID     string
	EndpointID   string
	Event        Event
	URL          string
	Secret       []byte
	Payload      json.RawMessage
	Headers      map[string]string
	AllowedHosts []string
}

func NewRequest(deliveryID, tenantID, endpointID string, event Event, endpointURL string, secret []byte, payload interface{}) (Request, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Request{}, fmt.Errorf("payload inválido: %w", err)
	}
	return Request{DeliveryID: deliveryID, TenantID: tenantID, EndpointID: endpointID, Event: event, URL: endpointURL, Secret: append([]byte{}, secret...), Payload: encoded}, nil
}

type Limits struct {
	MaxAttempts        int
	InitialBackoff     time.Duration
	MaxBackoff         time.Duration
	MaxPayloadBytes    int
	MaxResponseBytes   int
	RequestTimeout     time.Duration
	RequestsPerMinute  int
	TimestampTolerance time.Duration
}

func DefaultLimits() Limits {
	return Limits{
		MaxAttempts:        5,
		InitialBackoff:     1 * time.Second,
		MaxBackoff:         1 * time.Minute,
		MaxPayloadBytes:    1 << 20,
		MaxResponseBytes:   256 << 10,
		RequestTimeout:     15 * time.Second,
		RequestsPerMinute:  120,
		TimestampTolerance: 5 * time.Minute,
	}
}

func (limits Limits) normalized() (Limits, error) {
	defaults := DefaultLimits()
	if limits.MaxAttempts == 0 {
		limits.MaxAttempts = defaults.MaxAttempts
	}
	if limits.InitialBackoff == 0 {
		limits.InitialBackoff = defaults.InitialBackoff
	}
	if limits.MaxBackoff == 0 {
		limits.MaxBackoff = defaults.MaxBackoff
	}
	if limits.MaxPayloadBytes == 0 {
		limits.MaxPayloadBytes = defaults.MaxPayloadBytes
	}
	if limits.MaxResponseBytes == 0 {
		limits.MaxResponseBytes = defaults.MaxResponseBytes
	}
	if limits.RequestTimeout == 0 {
		limits.RequestTimeout = defaults.RequestTimeout
	}
	if limits.RequestsPerMinute == 0 {
		limits.RequestsPerMinute = defaults.RequestsPerMinute
	}
	if limits.TimestampTolerance == 0 {
		limits.TimestampTolerance = defaults.TimestampTolerance
	}
	if limits.MaxAttempts < 1 || limits.MaxAttempts > 10 {
		return Limits{}, errors.New("max_attempts deve estar entre 1 e 10")
	}
	if limits.InitialBackoff < 0 || limits.InitialBackoff > 10*time.Minute || limits.MaxBackoff < limits.InitialBackoff || limits.MaxBackoff > 1*time.Hour {
		return Limits{}, errors.New("backoff inválido")
	}
	if limits.MaxPayloadBytes < 1024 || limits.MaxPayloadBytes > 10<<20 {
		return Limits{}, errors.New("limite de payload inválido")
	}
	if limits.MaxResponseBytes < 1024 || limits.MaxResponseBytes > 4<<20 {
		return Limits{}, errors.New("limite de resposta inválido")
	}
	if limits.RequestTimeout < time.Second || limits.RequestTimeout > 2*time.Minute {
		return Limits{}, errors.New("timeout inválido")
	}
	if limits.RequestsPerMinute < 1 || limits.RequestsPerMinute > 10000 {
		return Limits{}, errors.New("limite de requisições inválido")
	}
	if limits.TimestampTolerance < 0 || limits.TimestampTolerance > 24*time.Hour {
		return Limits{}, errors.New("tolerância de timestamp inválida")
	}
	return limits, nil
}

type DeliveryStatus string

const (
	StatusPending          DeliveryStatus = "pending"
	StatusDelivered        DeliveryStatus = "delivered"
	StatusFailed           DeliveryStatus = "failed"
	StatusBlocked          DeliveryStatus = "blocked"
	StatusRateLimited      DeliveryStatus = "rate_limited"
	StatusPersistenceError DeliveryStatus = "persistence_error"
)

type DeliveryRecord struct {
	ID            string
	TenantID      string
	EndpointID    string
	Event         Event
	URL           string
	PayloadSHA256 string
	CreatedAt     time.Time
}

type DeliveryUpdate struct {
	ID            string
	Status        DeliveryStatus
	Attempt       int
	HTTPStatus    int
	Error         string
	NextAttemptAt *time.Time
	CompletedAt   *time.Time
	UpdatedAt     time.Time
}

type DeliveryStore interface {
	CreateDelivery(context.Context, DeliveryRecord) error
	UpdateDelivery(context.Context, DeliveryUpdate) error
}

type Result struct {
	DeliveryID string
	Status     DeliveryStatus
	Attempts   int
	HTTPStatus int
	Error      string
}

type Sleeper func(context.Context, time.Duration) error

var eventPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)

func validateRequest(request Request, limits Limits) error {
	if strings.TrimSpace(request.DeliveryID) == "" {
		return errors.New("delivery_id é obrigatório")
	}
	if !eventPattern.MatchString(string(request.Event)) || !IsKnownEvent(request.Event) {
		return errors.New("evento não está no catálogo")
	}
	if len(request.Secret) < 16 || len(request.Secret) > 256 {
		return errors.New("segredo deve ter entre 16 e 256 bytes")
	}
	if !json.Valid(request.Payload) {
		return errors.New("payload deve ser um JSON válido")
	}
	if len(request.Payload) > limits.MaxPayloadBytes {
		return errors.New("payload excede o limite permitido")
	}
	if err := ValidateURL(request.URL, nil); err != nil {
		return err
	}
	for key, value := range request.Headers {
		if strings.TrimSpace(key) == "" || strings.ContainsAny(key+value, "\r\n") {
			return errors.New("header inválido")
		}
		if isReservedHeader(key) {
			return fmt.Errorf("header reservado: %s", key)
		}
	}
	return nil
}

func isReservedHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "content-type", "user-agent", "x-notreve-webhook-id", "x-notreve-webhook-event", "x-notreve-webhook-timestamp", "x-notreve-webhook-signature":
		return true
	default:
		return false
	}
}
