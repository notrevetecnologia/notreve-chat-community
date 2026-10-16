package store

import (
	"strings"
	"testing"
)

func TestConversationScopeFilterAdminSemRestricao(t *testing.T) {
	query := " WHERE c.tenant_id=$1"
	args := []interface{}{7}
	gotQuery, gotArgs := conversationScopeFilter(query, args, &Usuario{ID: 1, Role: "admin"}, nil, false, true)
	if gotQuery != query || len(gotArgs) != 1 {
		t.Fatalf("admin não deveria receber escopo: %q %#v", gotQuery, gotArgs)
	}
}

func TestConversationScopeFilterAtendenteSemPermissaoIA(t *testing.T) {
	query := " WHERE c.tenant_id=$1"
	gotQuery, gotArgs := conversationScopeFilter(query, []interface{}{7}, &Usuario{ID: 42, Role: "atendente"}, []int{3, 4}, false, false)
	if !strings.Contains(gotQuery, "c.status <> 'closed'") || !strings.Contains(gotQuery, "c.status <> 'ia'") {
		t.Fatalf("atendente sem conversas_ver_ia deveria excluir ia e closed: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "c.atendente_id = $2") || !strings.Contains(gotQuery, "c.setor_id = ANY($3)") {
		t.Fatalf("escopo do atendente incorreto: %q", gotQuery)
	}
	if len(gotArgs) != 3 {
		t.Fatalf("args inesperados: %#v", gotArgs)
	}
}

func TestConversationScopeFilterAtendenteModoAberto(t *testing.T) {
	gotQuery, _ := conversationScopeFilter(" WHERE c.tenant_id=$1", []interface{}{7}, &Usuario{ID: 42, Role: "atendente"}, []int{3}, true, true)
	if !strings.Contains(gotQuery, "OR (c.atendente_id IS NULL)") {
		t.Fatalf("modo aberto deveria aceitar não atribuídas: %q", gotQuery)
	}
	if strings.Contains(gotQuery, "c.status <> 'ia'") {
		t.Fatalf("com conversas_ver_ia não deveria excluir a fila da IA: %q", gotQuery)
	}
}

func TestConversationScopeFilterSupervisor(t *testing.T) {
	gotQuery, gotArgs := conversationScopeFilter(" WHERE c.tenant_id=$1", []interface{}{7}, &Usuario{ID: 8, Role: "supervisor"}, []int{1, 2}, false, true)
	if !strings.Contains(gotQuery, "c.atendente_id = $2") || !strings.Contains(gotQuery, "c.setor_id = ANY($3)") {
		t.Fatalf("escopo do supervisor incorreto: %q", gotQuery)
	}
	if strings.Contains(gotQuery, "c.status <> 'closed'") {
		t.Fatalf("supervisor deveria ver encerradas: %q", gotQuery)
	}
	if len(gotArgs) != 3 {
		t.Fatalf("args inesperados: %#v", gotArgs)
	}
}
