package web

import (
	"context"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

const (
	convActionAccept     = "accept"
	convActionClose      = "close"
	convActionReopen     = "reopen"
	convActionReturnToAI = "return_to_ai"
	convActionTransfer   = "transfer"
)

var conversationStateActions = map[string]map[string]bool{
	store.ConversationStatusIA: {
		convActionAccept:   true,
		convActionClose:    true,
		convActionTransfer: true,
	},
	store.ConversationStatusWaiting: {
		convActionAccept:     true,
		convActionClose:      true,
		convActionTransfer:   true,
		convActionReturnToAI: true,
	},
	store.ConversationStatusAttending: {
		convActionClose:      true,
		convActionTransfer:   true,
		convActionReturnToAI: true,
	},
	store.ConversationStatusClosed: {
		convActionReopen: true,
	},
}

var conversationStateTargets = map[string]map[string]bool{
	store.ConversationStatusIA: {
		store.ConversationStatusWaiting:   true,
		store.ConversationStatusAttending: true,
		store.ConversationStatusClosed:    true,
	},
	store.ConversationStatusWaiting: {
		store.ConversationStatusIA:        true,
		store.ConversationStatusAttending: true,
		store.ConversationStatusClosed:    true,
	},
	store.ConversationStatusAttending: {
		store.ConversationStatusIA:      true,
		store.ConversationStatusWaiting: true,
		store.ConversationStatusClosed:  true,
	},
	store.ConversationStatusClosed: {
		store.ConversationStatusIA:        true,
		store.ConversationStatusAttending: true,
	},
}

func canonicalConversationAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "accept", "aceitar":
		return convActionAccept
	case "close", "encerrar":
		return convActionClose
	case "reopen", "reabrir":
		return convActionReopen
	case "return_to_ai", "return-to-ai", "devolver_ia":
		return convActionReturnToAI
	case "transfer", "transferir":
		return convActionTransfer
	default:
		return ""
	}
}

func conversationActionAllowed(status, action string) bool {
	canonical := canonicalConversationAction(action)
	if canonical == "" {
		return false
	}
	actions, ok := conversationStateActions[store.NormalizeConversationStatus(status)]
	return ok && actions[canonical]
}

func conversationStatusTransitionAllowed(from, to string) bool {
	from = store.NormalizeConversationStatus(from)
	to = store.NormalizeConversationStatus(to)
	if from == to {
		return true
	}
	targets, ok := conversationStateTargets[from]
	return ok && targets[to]
}

func conversationTransitionMessage(status, action string) string {
	switch action {
	case convActionClose:
		return "esta conversa já está encerrada"
	case convActionReopen:
		return "só é possível reabrir um atendimento encerrado"
	case convActionAccept:
		return "esta conversa não está disponível para assumir"
	case convActionReturnToAI:
		return "esta conversa não pode ser devolvida para a IA"
	case convActionTransfer:
		return "não é possível transferir uma conversa encerrada"
	default:
		return "transição de atendimento inválida"
	}
}

func canManageConversationAs(user *store.Usuario, conv store.Conversation, visible bool) bool {
	if user == nil {
		return false
	}
	if user.Role == "admin" || user.Role == "super_admin" {
		return true
	}
	if !visible {
		return false
	}
	if user.Role == "supervisor" {
		return true
	}
	return conv.AtendenteID == nil || *conv.AtendenteID == user.ID
}

func (s *Server) canManageConversation(ctx context.Context, user *store.Usuario, conv store.Conversation) bool {
	if user == nil {
		return false
	}
	if user.Role == "admin" || user.Role == "super_admin" {
		return true
	}
	visible := s.canSee(user, conv, s.userSetores(ctx, user), s.modoAberto(ctx))
	return canManageConversationAs(user, conv, visible)
}
