package web

import (
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestConversationActionAllowedByState(t *testing.T) {
	cases := []struct {
		status string
		action string
		want   bool
	}{
		{store.ConversationStatusIA, convActionAccept, true},
		{store.ConversationStatusIA, convActionClose, true},
		{store.ConversationStatusIA, convActionTransfer, true},
		{store.ConversationStatusIA, convActionReturnToAI, false},
		{store.ConversationStatusIA, convActionReopen, false},

		{store.ConversationStatusWaiting, convActionAccept, true},
		{store.ConversationStatusWaiting, convActionClose, true},
		{store.ConversationStatusWaiting, convActionTransfer, true},
		{store.ConversationStatusWaiting, convActionReturnToAI, true},
		{store.ConversationStatusWaiting, convActionReopen, false},

		{store.ConversationStatusAttending, convActionAccept, false},
		{store.ConversationStatusAttending, convActionClose, true},
		{store.ConversationStatusAttending, convActionTransfer, true},
		{store.ConversationStatusAttending, convActionReturnToAI, true},
		{store.ConversationStatusAttending, convActionReopen, false},

		{store.ConversationStatusClosed, convActionAccept, false},
		{store.ConversationStatusClosed, convActionClose, false},
		{store.ConversationStatusClosed, convActionTransfer, false},
		{store.ConversationStatusClosed, convActionReturnToAI, false},
		{store.ConversationStatusClosed, convActionReopen, true},

		{"estado_desconhecido", convActionClose, true},
		{"estado_desconhecido", convActionTransfer, true},
		{store.ConversationStatusIA, "typing", false},
	}
	for _, tc := range cases {
		if got := conversationActionAllowed(tc.status, tc.action); got != tc.want {
			t.Fatalf("status=%q ação=%q: obtido=%v esperado=%v", tc.status, tc.action, got, tc.want)
		}
	}
}

func TestConversationActionAliasesResolve(t *testing.T) {
	aliases := map[string]string{
		"aceitar":     convActionAccept,
		"encerrar":    convActionClose,
		"reabrir":     convActionReopen,
		"devolver_ia": convActionReturnToAI,
		"transferir":  convActionTransfer,
	}
	for alias, canonical := range aliases {
		if got := canonicalConversationAction(alias); got != canonical {
			t.Fatalf("alias %q: obtido=%q esperado=%q", alias, got, canonical)
		}
	}
	if got := canonicalConversationAction("reaction"); got != "" {
		t.Fatalf("ação de mensagem não deveria virar transição: %q", got)
	}
}

func TestConversationStatusTransitionAllowed(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{store.ConversationStatusIA, store.ConversationStatusAttending, true},
		{store.ConversationStatusIA, store.ConversationStatusClosed, true},
		{store.ConversationStatusIA, store.ConversationStatusIA, true},
		{store.ConversationStatusWaiting, store.ConversationStatusAttending, true},
		{store.ConversationStatusAttending, store.ConversationStatusIA, true},
		{store.ConversationStatusAttending, store.ConversationStatusWaiting, true},
		{store.ConversationStatusClosed, store.ConversationStatusAttending, true},
		{store.ConversationStatusClosed, store.ConversationStatusIA, true},
		{store.ConversationStatusClosed, store.ConversationStatusWaiting, false},
		{"human", store.ConversationStatusAttending, true},
	}
	for _, tc := range cases {
		if got := conversationStatusTransitionAllowed(tc.from, tc.to); got != tc.want {
			t.Fatalf("transição %q→%q: obtido=%v esperado=%v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestCanManageConversationAllowsUnassignedQueue(t *testing.T) {
	atendente := &store.Usuario{ID: 7, Role: "atendente"}
	unassigned := store.Conversation{ID: 1, Status: store.ConversationStatusWaiting}
	if !canManageConversationAs(atendente, unassigned, true) {
		t.Fatal("atendente deveria resolver conversa da fila sem dono")
	}
	mine := store.Conversation{ID: 1, Status: store.ConversationStatusAttending, AtendenteID: intPtr(7)}
	if !canManageConversationAs(atendente, mine, true) {
		t.Fatal("atendente deveria resolver a própria conversa")
	}
	other := store.Conversation{ID: 1, Status: store.ConversationStatusAttending, AtendenteID: intPtr(8)}
	if canManageConversationAs(atendente, other, true) {
		t.Fatal("atendente não deveria mexer na conversa de outro")
	}
	if canManageConversationAs(atendente, unassigned, false) {
		t.Fatal("atendente sem visibilidade não deveria agir")
	}
	if !canManageConversationAs(&store.Usuario{ID: 2, Role: "supervisor"}, store.Conversation{Status: store.ConversationStatusIA}, true) {
		t.Fatal("supervisor com visibilidade deveria agir")
	}
	if !canManageConversationAs(&store.Usuario{ID: 3, Role: "admin"}, store.Conversation{Status: store.ConversationStatusClosed}, false) {
		t.Fatal("admin deveria agir independente da visibilidade")
	}
	if canManageConversationAs(nil, unassigned, true) {
		t.Fatal("usuário nulo não deveria agir")
	}
}
