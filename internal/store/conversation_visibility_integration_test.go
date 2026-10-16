package store

import (
	"context"
	"testing"
)

func TestListConversationsForUserIAVisibilityByPermission(t *testing.T) {
	pool := newSaaSIsolationPool(t)
	defer pool.Close()

	tenantID := createIsolationTenant(t, pool, "veria")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM conversations WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM usuarios WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM atendentes WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM settings WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, tenantID)
	})
	if _, err := pool.Exec(context.Background(), `INSERT INTO settings (tenant_id) VALUES ($1)`, tenantID); err != nil {
		t.Fatalf("criando settings: %v", err)
	}

	ctx := WithTenantID(context.Background(), tenantID)
	s := New(pool)

	var atendenteID int
	if err := pool.QueryRow(ctx, `INSERT INTO atendentes (tenant_id, name, active, online)
		VALUES ($1,'Ana',true,true) RETURNING id`, tenantID).Scan(&atendenteID); err != nil {
		t.Fatalf("criando atendente: %v", err)
	}
	var userID int
	if err := pool.QueryRow(ctx, `INSERT INTO usuarios (tenant_id, name, username, password_hash, role, atendente_id, active)
		VALUES ($1,'Ana','ana-veria','x','atendente',$2,true) RETURNING id`, tenantID, atendenteID).Scan(&userID); err != nil {
		t.Fatalf("criando usuário: %v", err)
	}

	var iaID, waitingID int
	if err := pool.QueryRow(ctx, `INSERT INTO conversations (tenant_id, channel, customer_phone, status)
		VALUES ($1,'whatsapp','5511999999999','ia') RETURNING id`, tenantID).Scan(&iaID); err != nil {
		t.Fatalf("criando conversa ia: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO conversations (tenant_id, channel, customer_phone, status)
		VALUES ($1,'whatsapp','5511888888888','humano_aguardando') RETURNING id`, tenantID).Scan(&waitingID); err != nil {
		t.Fatalf("criando conversa aguardando: %v", err)
	}

	user := &Usuario{ID: userID, Role: "atendente"}

	semPermissao, err := s.ListConversationsForUser(ctx, "", user, nil, true, false)
	if err != nil {
		t.Fatalf("listando sem permissão: %v", err)
	}
	if containsConversationID(semPermissao, iaID) {
		t.Fatal("sem conversas_ver_ia a fila ia não deveria aparecer")
	}
	if !containsConversationID(semPermissao, waitingID) {
		t.Fatal("conversa aguardando humano deveria aparecer")
	}

	comPermissao, err := s.ListConversationsForUser(ctx, "", user, nil, true, true)
	if err != nil {
		t.Fatalf("listando com permissão: %v", err)
	}
	if !containsConversationID(comPermissao, iaID) {
		t.Fatal("com conversas_ver_ia a fila ia deveria aparecer")
	}
	if !containsConversationID(comPermissao, waitingID) {
		t.Fatal("conversa aguardando humano deveria continuar aparecendo")
	}
}

func containsConversationID(conversations []Conversation, id int) bool {
	for _, conversation := range conversations {
		if conversation.ID == id {
			return true
		}
	}
	return false
}
