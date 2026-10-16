package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"github.com/notrevetecnologia/notreve-chat-community/internal/webhooks"
)

const maxChannelHealthErrorLen = 1024

type Monitor struct {
	st             *store.Store
	webhooksMu     sync.Mutex
	webhooksSynced map[string]struct{}
}

func NewMonitor(st *store.Store) *Monitor {
	return &Monitor{st: st, webhooksSynced: make(map[string]struct{})}
}

type ChannelHealthChange struct {
	ChannelID      int
	ChannelType    string
	ChannelName    string
	PreviousStatus string
	Status         string
	Error          string
}

func healthTransition(previous, current string) bool {
	return previous != "" && previous != current && previous != "unknown" && current != "unknown"
}

func whatsappHealthStatus(connected, loggedIn bool, err error) (status, errMsg string) {
	if err != nil {
		return "disconnected", fmt.Sprintf("erro verificando a instância WhatsApp: %v", err)
	}
	if connected && loggedIn {
		return "connected", ""
	}
	return "disconnected", ""
}

func telegramHealthStatus(webhookURL string, err error) (status, errMsg string) {
	if err != nil {
		return "disconnected", fmt.Sprintf("erro consultando o webhook do Telegram: %v", err)
	}
	if webhookURL != "" {
		return "connected", ""
	}
	return "disconnected", ""
}

func (m *Monitor) CheckTenant(ctx context.Context) ([]ChannelHealthChange, error) {
	tenantID, err := store.TenantIDRequired(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := m.st.ListMonitorableChannelsSecure(ctx)
	if err != nil {
		return nil, err
	}
	webhookURL := ""
	if settings, settingsErr := m.st.GetSettings(ctx); settingsErr == nil {
		base := strings.TrimRight(strings.TrimSpace(settings.WebhookBaseURL), "/")
		if base != "" {
			webhookURL = base + "/notreve/webhook"
		}
	}
	var changes []ChannelHealthChange
	for _, ch := range channels {
		previous, err := m.st.GetChannelByID(ctx, ch.ID)
		if err != nil {
			Logger.Printf("monitor de canais [tenant %d]: canal %d não encontrado, pulando: %v", tenantID, ch.ID, err)
			continue
		}
		status := "unknown"
		errMsg := ""
		switch ch.Type {
		case "whatsapp":
			if strings.TrimSpace(ch.Token) == "" || strings.TrimSpace(ch.BaseURL) == "" {
				status = "unknown"
			} else {
				connected, loggedIn, _, _, _, statusErr := InstanciaStatus(ctx, ch.BaseURL, ch.Token)
				status, errMsg = whatsappHealthStatus(connected, loggedIn, statusErr)
				if connected && loggedIn && webhookURL != "" {
					m.syncWhatsAppWebhook(ctx, ch, webhookURL)
				}
			}
		case "telegram":
			if strings.TrimSpace(ch.Token) == "" {
				status = "unknown"
			} else {
				botWebhook, statusErr := TelegramWebhookInfo(ctx, ch.Token)
				status, errMsg = telegramHealthStatus(botWebhook, statusErr)
			}
		default:
			continue
		}
		if runes := []rune(errMsg); len(runes) > maxChannelHealthErrorLen {
			errMsg = string(runes[:maxChannelHealthErrorLen])
		}
		if err := m.st.SetChannelHealth(ctx, ch.ID, status, errMsg); err != nil {
			Logger.Printf("monitor de canais [tenant %d]: gravando saúde do canal %d: %v", tenantID, ch.ID, err)
		}
		if healthTransition(previous.HealthStatus, status) {
			changes = append(changes, ChannelHealthChange{
				ChannelID:      ch.ID,
				ChannelType:    ch.Type,
				ChannelName:    ch.Name,
				PreviousStatus: previous.HealthStatus,
				Status:         status,
				Error:          errMsg,
			})
		}
	}
	return changes, nil
}

func (m *Monitor) syncWhatsAppWebhook(ctx context.Context, ch store.Channel, webhookURL string) {
	key := fmt.Sprintf("%d:%s", ch.ID, webhookURL)
	m.webhooksMu.Lock()
	_, synced := m.webhooksSynced[key]
	m.webhooksMu.Unlock()
	if synced {
		return
	}
	if err := InstanciaSetWebhook(ctx, ch.BaseURL, ch.Token, webhookURL, []string{"Message", "ReadReceipt", "ChatPresence", "CallOffer"}); err != nil {
		Logger.Printf("monitor de canais: atualizando webhook do WhatsApp %d: %v", ch.ID, err)
		return
	}
	m.webhooksMu.Lock()
	m.webhooksSynced[key] = struct{}{}
	m.webhooksMu.Unlock()
}

func (m *Monitor) Run(ctx context.Context) ([]ChannelHealthChange, error) {
	changes, err := m.CheckTenant(ctx)
	if err != nil {
		return nil, err
	}
	for _, change := range changes {
		m.Alert(ctx, change)
	}
	return changes, nil
}

func (m *Monitor) Alert(ctx context.Context, change ChannelHealthChange) {
	payload := map[string]interface{}{
		"channel_id":      change.ChannelID,
		"channel_type":    change.ChannelType,
		"channel_name":    change.ChannelName,
		"previous_status": change.PreviousStatus,
		"status":          change.Status,
		"checked_at":      time.Now().UTC(),
	}
	if change.Error != "" {
		payload["error"] = change.Error
	}
	payloadJSON, _ := json.Marshal(payload)
	_, err := m.st.EnqueueWebhookOutboxEvent(ctx, store.WebhookOutboxEventInput{
		EventType:  webhooks.EventChannelStatusChanged,
		EventKey:   "channel.status_changed:" + strconv.Itoa(change.ChannelID) + ":" + strconv.FormatInt(time.Now().UnixNano(), 10),
		Payload:    payloadJSON,
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		Logger.Printf("monitor de canais: evento channel.status_changed não enfileirado: %v", err)
	}
	Logger.Printf("monitor de canais [tenant %d]: canal %d (%s) passou de %s para %s", store.TenantID(ctx), change.ChannelID, change.ChannelType, change.PreviousStatus, change.Status)
}
