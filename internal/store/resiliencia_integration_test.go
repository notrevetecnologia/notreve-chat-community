package store

import (
	"context"
	"testing"
)

func TestListOrphanConversationsDetectsUnansweredInbound(t *testing.T) {
	pool := newSaaSIsolationPool(t)
	defer pool.Close()

	tenantID := createIsolationTenant(t, pool, "watchdog")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM messages WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM conversations WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM settings WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, tenantID)
	})
	if _, err := pool.Exec(context.Background(), `INSERT INTO settings (tenant_id) VALUES ($1)`, tenantID); err != nil {
		t.Fatalf("criando settings: %v", err)
	}

	ctx := WithTenantID(context.Background(), tenantID)
	s := New(pool)

	var convID int
	if err := pool.QueryRow(ctx, `INSERT INTO conversations (tenant_id, channel, customer_phone, status)
		VALUES ($1,'whatsapp','5511999999999','ia') RETURNING id`, tenantID).Scan(&convID); err != nil {
		t.Fatalf("criando conversa: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO messages (tenant_id, conversation_id, role, content, source, created_at)
		VALUES ($1,$2,'user','oi, preciso de ajuda','customer', now() - interval '10 minutes')`, tenantID, convID); err != nil {
		t.Fatalf("inserindo mensagem do cliente: %v", err)
	}

	orphans, err := s.ListOrphanConversations(ctx, 5, 50)
	if err != nil {
		t.Fatalf("listando órfãs: %v", err)
	}
	if len(orphans) != 1 || orphans[0].ID != convID {
		t.Fatalf("esperava 1 órfã (#%d), obtido %#v", convID, orphans)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO messages (tenant_id, conversation_id, role, content, source)
		VALUES ($1,$2,'assistant','tudo bem?','ai')`, tenantID, convID); err != nil {
		t.Fatalf("inserindo resposta da IA: %v", err)
	}
	orphans, err = s.ListOrphanConversations(ctx, 5, 50)
	if err != nil {
		t.Fatalf("relendo órfãs: %v", err)
	}
	if len(orphans) != 0 {
		t.Fatalf("conversa respondida não deveria ser órfã: %#v", orphans)
	}
}

func TestPurgeOldMediaClearsClosedConversationMedia(t *testing.T) {
	pool := newSaaSIsolationPool(t)
	defer pool.Close()

	tenantID := createIsolationTenant(t, pool, "media-purge")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM messages WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM conversations WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM settings WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, tenantID)
	})
	if _, err := pool.Exec(context.Background(), `INSERT INTO settings (tenant_id) VALUES ($1)`, tenantID); err != nil {
		t.Fatalf("criando settings: %v", err)
	}

	ctx := WithTenantID(context.Background(), tenantID)
	s := New(pool)

	var closedID, openID int
	if err := pool.QueryRow(ctx, `INSERT INTO conversations (tenant_id, channel, customer_phone, status)
		VALUES ($1,'whatsapp','5511888888888','closed') RETURNING id`, tenantID).Scan(&closedID); err != nil {
		t.Fatalf("criando conversa encerrada: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO conversations (tenant_id, channel, customer_phone, status)
		VALUES ($1,'whatsapp','5511777777777','ia') RETURNING id`, tenantID).Scan(&openID); err != nil {
		t.Fatalf("criando conversa aberta: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO messages (tenant_id, conversation_id, role, content, source, media_b64, created_at)
		VALUES ($1,$2,'user','foto','customer','AAAA', now() - interval '40 days')`, tenantID, closedID); err != nil {
		t.Fatalf("inserindo mídia encerrada: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO messages (tenant_id, conversation_id, role, content, source, media_b64, created_at)
		VALUES ($1,$2,'user','foto','customer','BBBB', now() - interval '40 days')`, tenantID, openID); err != nil {
		t.Fatalf("inserindo mídia aberta: %v", err)
	}

	if _, err := s.PurgeOldMedia(ctx, 30); err != nil {
		t.Fatalf("purgando mídia: %v", err)
	}

	var closedMedia, openMedia string
	if err := pool.QueryRow(ctx, `SELECT media_b64 FROM messages WHERE tenant_id=$1 AND conversation_id=$2`, tenantID, closedID).Scan(&closedMedia); err != nil {
		t.Fatalf("lendo mídia encerrada: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT media_b64 FROM messages WHERE tenant_id=$1 AND conversation_id=$2`, tenantID, openID).Scan(&openMedia); err != nil {
		t.Fatalf("lendo mídia aberta: %v", err)
	}
	if closedMedia != "" {
		t.Fatalf("mídia de conversa encerrada deveria ter sido limpa, obtido %q", closedMedia)
	}
	if openMedia != "BBBB" {
		t.Fatalf("mídia de conversa ativa não pode ser limpa, obtido %q", openMedia)
	}
}
