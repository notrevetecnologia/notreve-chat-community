package store

import "strings"

const (
	ConversationStatusIA        = "ia"
	ConversationStatusWaiting   = "humano_aguardando"
	ConversationStatusAttending = "humano_atendendo"
	ConversationStatusClosed    = "closed"
)

func NormalizeConversationStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "ai", "ia":
		return ConversationStatusIA
	case "humano_aguardando", "aguardando", "human":
		return ConversationStatusWaiting
	case "humano_atendendo", "em_atendimento", "atendendo":
		return ConversationStatusAttending
	case "closed", "encerrada", "encerrado":
		return ConversationStatusClosed
	default:
		return ConversationStatusWaiting
	}
}
