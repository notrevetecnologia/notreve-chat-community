package channel

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

// Outbound é a resposta de saída de um atendente (IA ou humano) para o cliente.
type Outbound struct {
	Text    string
	ReplyTo string // id da mensagem a responder, quando o canal suporta
}

// OutboundMedia é uma mídia de saída (upload do atendente).
type OutboundMedia struct {
	MediaType string
	DataB64   string
	Mime      string
	FileName  string
	Caption   string
}

// ErrUnsupported indica que o canal não suporta a operação (ex.: mídia).
var ErrUnsupported = fmt.Errorf("canal não suporta esta operação")

// Adapter é a porta de saída de um canal. Um canal implementa esta interface
// para ser usado pelo motor (IA/humano) e pelo painel SEM que o núcleo precise
// conhecer os detalhes do canal. Integrar um canal novo = implementar Adapter +
// registrar no Registry. O processamento de entrada (webhook → Inbound →
// Processor.Process) já é agnóstico de canal.
type Adapter interface {
	// Type devolve o identificador do canal (ex.: "whatsapp", "webchat", "telegram").
	Type() string
	// Send entrega uma resposta de texto ao cliente.
	Send(ctx context.Context, ch store.Channel, clientID string, msg Outbound) error
	// SendMedia entrega uma mídia (pode retornar ErrUnsupported).
	SendMedia(ctx context.Context, ch store.Channel, clientID string, media OutboundMedia) error
}

// Registry mantém os adaptadores por tipo e resolve o adaptador de uma conversa.
type Registry struct {
	adapters map[string]Adapter
}

func NewRegistry() *Registry {
	return &Registry{adapters: map[string]Adapter{}}
}

// Register adiciona (ou substitui) o adaptador de um tipo de canal.
func (r *Registry) Register(a Adapter) {
	if a != nil && a.Type() != "" {
		r.adapters[a.Type()] = a
	}
}

// For devolve o adaptador de um tipo de canal (nil se não registrado).
func (r *Registry) For(typeName string) Adapter {
	return r.adapters[typeName]
}

// --- Adaptadores prontos do núcleo ---

// WhatsAppAdapter adapta o Sender (Notreve WhatsApp) para a interface Adapter.
type WhatsAppAdapter struct {
	sender Sender
}

func NewWhatsAppAdapter(sender Sender) *WhatsAppAdapter {
	return &WhatsAppAdapter{sender: sender}
}

func (a *WhatsAppAdapter) Type() string { return "whatsapp" }

func (a *WhatsAppAdapter) Send(ctx context.Context, ch store.Channel, clientID string, msg Outbound) error {
	if a.sender == nil {
		return fmt.Errorf("sender de WhatsApp não configurado")
	}
	if msg.ReplyTo != "" {
		// Respostas do painel precisam usar o token da instância que recebeu a
		// conversa. O método legado SendTextReply resolve o canal ativo global e
		// pode enviar por outro número.
		if sender, ok := a.sender.(interface {
			SendTextChannelReply(context.Context, store.Channel, string, string, string) error
		}); ok {
			return sender.SendTextChannelReply(ctx, ch, clientID, msg.Text, msg.ReplyTo)
		}
		// Fallback seguro: preserva o canal, mesmo que o adaptador não suporte
		// reply nativo.
		_, err := a.sender.SendTextChannel(ctx, ch, clientID, msg.Text)
		return err
	}
	_, err := a.sender.SendTextChannel(ctx, ch, clientID, msg.Text)
	return err
}

func (a *WhatsAppAdapter) SendMedia(ctx context.Context, ch store.Channel, clientID string, media OutboundMedia) error {
	if a.sender == nil {
		return fmt.Errorf("sender de WhatsApp não configurado")
	}
	_, err := a.sender.SendMediaChannel(ctx, ch, clientID, media.MediaType, media.DataB64, media.Mime, media.FileName, media.Caption)
	return err
}

// TelegramAdapter adapta o canal Telegram para a interface Adapter.
type TelegramAdapter struct{}

func NewTelegramAdapter() *TelegramAdapter { return &TelegramAdapter{} }

func (a *TelegramAdapter) Type() string { return "telegram" }

func (a *TelegramAdapter) Send(ctx context.Context, ch store.Channel, clientID string, msg Outbound) error {
	chatID := telegramChatIDFromClient(clientID)
	if chatID == 0 {
		return fmt.Errorf("identificador de telegram inválido: %q", clientID)
	}
	return SendTelegramText(ctx, ch.Token, chatID, msg.Text)
}

func (a *TelegramAdapter) SendMedia(ctx context.Context, ch store.Channel, clientID string, media OutboundMedia) error {
	return ErrUnsupported
}

// WebChatAdapter: o webchat é passivo (o widget lê as mensagens persistidas no
// banco via /messages). Por isso Send não envia nada (a resposta já foi gravada
// como mensagem) — devolve nil; mídia fica pendente de operação específica.
type WebChatAdapter struct{}

func NewWebChatAdapter() *WebChatAdapter { return &WebChatAdapter{} }

func (a *WebChatAdapter) Type() string { return "webchat" }

func (a *WebChatAdapter) Send(ctx context.Context, ch store.Channel, clientID string, msg Outbound) error {
	return nil
}

func (a *WebChatAdapter) SendMedia(ctx context.Context, ch store.Channel, clientID string, media OutboundMedia) error {
	return ErrUnsupported
}

// telegramChatIDFromClient extrai o chat_id de um identificador "telegram:<id>".
func telegramChatIDFromClient(clientID string) int64 {
	const prefix = "telegram:"
	if !strings.HasPrefix(clientID, prefix) {
		return 0
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(clientID, prefix), 10, 64)
	if err != nil {
		return 0
	}
	return id
}
