package channel

import (
	"context"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

// Sender entrega mensagens de atendente para um cliente no canal. É o contrato
// de saída do WhatsApp (Notreve) e também a base do WhatsAppAdapter. Canais
// novos implementam Adapter, não Sender — este fica como contrato do WhatsApp
// (usado por campanhas, IA-monitora e envios que exigem um canal concreto).
type Sender interface {
	SendText(ctx context.Context, phone, text string) error
	SendTextReply(ctx context.Context, phone, text, replyToWAMsgID string) error
	SendMedia(ctx context.Context, phone, mediaType, dataB64, mimeType, fileName, caption string) error
	SendEdit(ctx context.Context, phone, waMsgID, newText string) error
	SendTyping(ctx context.Context, phone string, typing bool) error
	SendReaction(ctx context.Context, phone, waMsgID, emoji string) error
	SyncContacts(ctx context.Context) ([]store.Contato, error)
	CheckWhatsApp(ctx context.Context, phone string) (bool, error)
	GetAvatar(ctx context.Context, phone string) (string, error)
	// SendTextChannel envia por um canal específico (base URL + token), ignorando a allowlist
	// de resposta — usado por campanhas, que são ação explícita do admin. Retorna o id da mensagem.
	SendTextChannel(ctx context.Context, ch store.Channel, phone, text string) (string, error)
	// SendMediaChannel envia mídia por um canal específico (campanhas). Retorna o id da mensagem.
	SendMediaChannel(ctx context.Context, ch store.Channel, phone, mediaType, dataB64, mimeType, fileName, caption string) (string, error)
}
