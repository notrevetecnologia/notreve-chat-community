package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

const telegramAPIBase = "https://api.telegram.org"
const maxTelegramWebhookBody = 1 << 20

var telegramHTTPClient = &http.Client{Transport: secretAwareHTTPTransport, Timeout: 20 * time.Second}

// Telegram é o canal de atendimento via bot do Telegram.
// A entrada chega por webhook do Bot API (setWebhook) e a saída de resposta é
// feita pelo endpoint sendMessage. O token do bot é guardado em channels.token
// e o webhook é endereçado como /telegram/webhook/<token>, de modo que o token
// resolve o canal (e o tenant) e também autentica a chamada.
type Telegram struct {
	proc *Processor
	st   *store.Store
	http *http.Client
}

func NewTelegram(proc *Processor, st *store.Store) *Telegram {
	return &Telegram{
		proc: proc,
		st:   st,
		http: &http.Client{Transport: secretAwareHTTPTransport, Timeout: 20 * time.Second},
	}
}

func telegramMethodURL(token, method string) string {
	return telegramAPIBase + "/bot" + token + "/" + method
}

type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message"`
}

type telegramMessage struct {
	MessageID int64        `json:"message_id"`
	From      telegramFrom `json:"from"`
	Chat      telegramChat `json:"chat"`
	Text      string       `json:"text"`
	Caption   string       `json:"caption"`

	Photo     []json.RawMessage `json:"photo"`
	Video     json.RawMessage   `json:"video"`
	Voice     json.RawMessage   `json:"voice"`
	Audio     json.RawMessage   `json:"audio"`
	Document  json.RawMessage   `json:"document"`
	Animation json.RawMessage   `json:"animation"`
	Sticker   json.RawMessage   `json:"sticker"`
	Location  json.RawMessage   `json:"location"`
	Contact   json.RawMessage   `json:"contact"`
}

type telegramFrom struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type telegramChat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

func telegramMediaPresent(raw json.RawMessage) bool {
	return len(raw) > 0 && string(raw) != "null"
}

func (m *telegramMessage) hasMedia() bool {
	if m == nil {
		return false
	}
	return len(m.Photo) > 0 ||
		telegramMediaPresent(m.Video) || telegramMediaPresent(m.Voice) ||
		telegramMediaPresent(m.Audio) || telegramMediaPresent(m.Document) ||
		telegramMediaPresent(m.Animation) || telegramMediaPresent(m.Sticker) ||
		telegramMediaPresent(m.Location) || telegramMediaPresent(m.Contact)
}

func (t *Telegram) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := strings.TrimSpace(r.PathValue("token"))
	if token == "" {
		http.Error(rw, "token ausente", http.StatusBadRequest)
		return
	}
	// O canal é resolvido pelo próprio token do webhook: além de identificar o
	// tenant e o bot, só quem conhece o token consegue chamar o endpoint.
	ch, err := t.st.ChannelByTelegramTokenRouted(r.Context(), token)
	if err != nil {
		Logger.Printf("telegram: webhook sem canal telegram ativo, ignorado")
		rw.WriteHeader(http.StatusOK)
		return
	}
	ctx := store.WithTenantID(r.Context(), ch.TenantID)

	raw, readErr := io.ReadAll(io.LimitReader(r.Body, maxTelegramWebhookBody+1))
	if readErr != nil {
		http.Error(rw, "erro lendo payload", http.StatusBadRequest)
		return
	}
	if len(raw) > maxTelegramWebhookBody {
		http.Error(rw, "payload muito grande", http.StatusRequestEntityTooLarge)
		return
	}
	Logger.Printf("telegram webhook recebido: %d bytes", len(raw))

	var update telegramUpdate
	if err := json.Unmarshal(raw, &update); err != nil {
		Logger.Printf("telegram: json inválido: %v", err)
		http.Error(rw, "bad payload", http.StatusBadRequest)
		return
	}
	if update.Message == nil {
		// Outros tipos de update (edited_message, callback_query, etc.) não são
		// atendidos neste incremento. Devolve 200 para o Telegram não reenviar.
		rw.WriteHeader(http.StatusOK)
		return
	}
	msg := update.Message
	if msg.From.IsBot {
		rw.WriteHeader(http.StatusOK)
		return
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		text = strings.TrimSpace(msg.Caption)
	}

	// Em grupos, o bot só responde (com texto) se o canal permitir responder
	// grupos. Conversa privada é o caso normal.
	if msg.Chat.Type != "private" {
		if !ch.RespondGrupos {
			rw.WriteHeader(http.StatusOK)
			return
		}
	}

	// Identificador do cliente: prefixado com "telegram:" para não colidir com
	// números de WhatsApp e permitir o processador achar/reusar a conversa por
	// chat_id.
	phone := "telegram:" + strconv.FormatInt(msg.Chat.ID, 10)
	name := strings.TrimSpace(strings.Join([]string{msg.From.FirstName, msg.From.LastName}, " "))
	if name == "" {
		name = msg.From.Username
	}
	channelID := ch.ID
	in := Inbound{
		Message:   text,
		Phone:     phone,
		Name:      name,
		WAMsgID:   strconv.FormatInt(msg.MessageID, 10),
		TenantID:  ch.TenantID,
		ChannelID: &channelID,
		Channel:   "telegram",
		Origem:    "telegram",
	}

	if text == "" && msg.hasMedia() {
		reply := "Por enquanto, só consigo entender mensagens escritas por aqui. 😊\nEnvie o texto da sua dúvida, por favor."
		if err := t.sendText(ctx, ch.Token, msg.Chat.ID, reply); err != nil {
			Logger.Printf("telegram: erro respondendo pedido de texto: %v", err)
		}
		rw.WriteHeader(http.StatusOK)
		return
	}
	if text == "" {
		rw.WriteHeader(http.StatusOK)
		return
	}

	res, err := t.proc.Process(ctx, in)
	if err != nil {
		Logger.Printf("telegram: erro processando mensagem de %s: %v", phone, err)
		http.Error(rw, "erro interno", http.StatusInternalServerError)
		return
	}
	if res.Handled && res.Reply != "" {
		if err := t.sendText(ctx, ch.Token, msg.Chat.ID, res.Reply); err != nil {
			Logger.Printf("telegram: erro enviando resposta a %s: %v", phone, err)
		}
	}

	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]interface{}{
		"conversation_id": res.ConversationID,
		"status":          res.Status,
		"reply_sent":      res.Handled && res.Reply != "",
	})
}

// sendText envia uma mensagem de texto para um chat via sendMessage do Bot API.
func (t *Telegram) sendText(ctx context.Context, token string, chatID int64, text string) error {
	payload, _ := json.Marshal(map[string]interface{}{"chat_id": chatID, "text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, telegramMethodURL(token, "sendMessage"), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxTelegramWebhookBody+1))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram sendMessage %d: %s", resp.StatusCode, truncateBytes(raw, 300))
	}
	return nil
}

// SendTelegramText envia texto via sendMessage para um chat. Usada para
// entregar respostas de atendente humano em conversas Telegram (o remetente do
// painel envia por aqui quando a conversa é do canal telegram).
func SendTelegramText(ctx context.Context, token string, chatID int64, text string) error {
	if strings.TrimSpace(token) == "" || chatID == 0 {
		return fmt.Errorf("telegram: token ou chat_id inválido")
	}
	payload, _ := json.Marshal(map[string]interface{}{"chat_id": chatID, "text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, telegramMethodURL(token, "sendMessage"), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := telegramHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxTelegramWebhookBody+1))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram sendMessage %d: %s", resp.StatusCode, truncateBytes(raw, 300))
	}
	return nil
}

// TelegramWebhookInfo consulta getWebhookInfo e devolve a URL do webhook
// configurado para o bot ("" quando não há). Usada pelo painel para exibir o
// status de "webhook configurado". A URL pode conter o token, então nunca deve
// ser exposta na resposta ao cliente do painel.
func TelegramWebhookInfo(ctx context.Context, token string) (string, error) {
	if strings.TrimSpace(token) == "" {
		return "", fmt.Errorf("token do bot vazio")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, telegramMethodURL(token, "getWebhookInfo"), nil)
	if err != nil {
		return "", err
	}
	resp, err := telegramHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxTelegramWebhookBody+1))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("telegram getWebhookInfo %d: %s", resp.StatusCode, truncateBytes(raw, 300))
	}
	var out struct {
		Result struct {
			URL string `json:"url"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	return out.Result.URL, nil
}
