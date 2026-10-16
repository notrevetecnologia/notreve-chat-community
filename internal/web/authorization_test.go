package web

import (
	"net/http/httptest"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestCanSeeConversationScopesAtendente(t *testing.T) {
	s := &Server{}
	user := &store.Usuario{ID: 7, Role: "atendente"}
	assigned := store.Conversation{ID: 1, Status: "humano_atendendo", AtendenteID: intPtr(7)}
	if !s.canSee(user, assigned, nil, false) {
		t.Fatal("atendente deveria ver conversa atribuída a ele")
	}
	assigned.AtendenteID = intPtr(8)
	if s.canSee(user, assigned, nil, false) {
		t.Fatal("atendente não deveria ver conversa de outro atendente")
	}
	queued := store.Conversation{ID: 2, Status: "humano_aguardando", SetorID: intPtr(3)}
	if s.canSee(user, queued, []int{4}, false) {
		t.Fatal("atendente não deveria ver fila de outro setor")
	}
	queued.SetorID = intPtr(4)
	if !s.canSee(user, queued, []int{4}, false) {
		t.Fatal("atendente deveria ver fila do próprio setor")
	}
	queued.Status = "ia"
	if !s.canSee(user, queued, []int{4}, true) {
		t.Fatal("atendente com conversas_ver_ia deveria ver conversa em modo IA")
	}
	semVerIA := &store.Usuario{ID: 7, Role: "atendente", Permissions: "conversas_ver,conversas_responder"}
	if s.canSee(semVerIA, queued, []int{4}, true) {
		t.Fatal("atendente sem conversas_ver_ia não deveria ver conversa em modo IA")
	}
}

func TestCanSeeConversationAdmin(t *testing.T) {
	conv := store.Conversation{ID: 1, Status: "ia"}
	for _, role := range []string{"admin", "super_admin"} {
		if !(&Server{}).canSee(&store.Usuario{ID: 1, Role: role}, conv, nil, false) {
			t.Fatalf("papel %s deveria ver a conversa", role)
		}
	}
}

func TestCanSeeConversationScopesSupervisor(t *testing.T) {
	s := &Server{}
	user := &store.Usuario{ID: 7, Role: "supervisor"}
	sectorConversation := store.Conversation{ID: 1, Status: "ia", SetorID: intPtr(3)}
	if !s.canSee(user, sectorConversation, []int{3}, false) {
		t.Fatal("supervisor deveria ver conversa do setor vinculado")
	}
	if s.canSee(user, sectorConversation, []int{4}, false) {
		t.Fatal("supervisor não deveria ver conversa de outro setor")
	}
	withoutSector := store.Conversation{ID: 2, Status: "humano_aguardando"}
	if s.canSee(user, withoutSector, []int{3}, false) {
		t.Fatal("supervisor não deveria ver conversa sem setor")
	}
	assigned := store.Conversation{ID: 3, Status: "humano_atendendo", AtendenteID: intPtr(7)}
	if !s.canSee(user, assigned, nil, false) {
		t.Fatal("supervisor deveria ver conversa atribuída a ele")
	}
}

func TestSameOriginRequest(t *testing.T) {
	r := httptest.NewRequest("POST", "https://painel.example/config/salvar", nil)
	r.Host = "painel.example"
	r.Header.Set("Origin", "https://painel.example")
	if !sameOriginRequest(r) {
		t.Fatal("origem igual deveria ser aceita")
	}
	r.Header.Set("Origin", "https://evil.example")
	if sameOriginRequest(r) {
		t.Fatal("origem diferente deveria ser bloqueada")
	}
}

func intPtr(v int) *int { return &v }
