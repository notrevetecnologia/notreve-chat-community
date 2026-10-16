package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func TelegramSetWebhook(ctx context.Context, token, webhookURL string) error {
	token = strings.TrimSpace(token)
	webhookURL = strings.TrimSpace(webhookURL)
	if token == "" || webhookURL == "" {
		return fmt.Errorf("telegram: token e webhook URL são obrigatórios")
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"url": webhookURL,
		"allowed_updates": []string{"message"},
		"drop_pending_updates": false,
	})
	return telegramWebhookCall(ctx, token, "setWebhook", payload)
}

func TelegramDeleteWebhook(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("telegram: token é obrigatório")
	}
	payload, _ := json.Marshal(map[string]interface{}{"drop_pending_updates": false})
	return telegramWebhookCall(ctx, token, "deleteWebhook", payload)
}

func telegramWebhookCall(ctx context.Context, token, method string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, telegramMethodURL(token, method), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := telegramHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxTelegramWebhookBody+1))
	if err != nil {
		return err
	}
	if len(raw) > maxTelegramWebhookBody {
		return fmt.Errorf("telegram %s: resposta excedeu limite", method)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram %s HTTP %d: %s", method, resp.StatusCode, truncateBytes(raw, 300))
	}
	var out struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	if !out.OK {
		return fmt.Errorf("telegram %s: %s", method, strings.TrimSpace(out.Description))
	}
	return nil
}
