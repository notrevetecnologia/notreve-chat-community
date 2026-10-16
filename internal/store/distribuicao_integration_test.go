package store

import (
	"context"
	"testing"
)

func TestListAvailableAtendentesCapacidadeEFallback(t *testing.T) {
	pool := newSaaSIsolationPool(t)
	defer pool.Close()

	tenantID := createIsolationTenant(t, pool, "distrib")
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

	var onlineID, offlineID int
	if err := pool.QueryRow(ctx, `INSERT INTO atendentes (tenant_id, name, active, online, capacidade)
		VALUES ($1,'Online',true,true,2) RETURNING id`, tenantID).Scan(&onlineID); err != nil {
		t.Fatalf("criando atendente online: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO atendentes (tenant_id, name, active, online, capacidade)
		VALUES ($1,'Offline',true,false,0) RETURNING id`, tenantID).Scan(&offlineID); err != nil {
		t.Fatalf("criando atendente offline: %v", err)
	}
	var onlineUserID int
	if err := pool.QueryRow(ctx, `INSERT INTO usuarios (tenant_id, name, username, password_hash, role, atendente_id, active)
		VALUES ($1,'Online','online-distrib','x','atendente',$2,true) RETURNING id`, tenantID, onlineID).Scan(&onlineUserID); err != nil {
		t.Fatalf("criando usuário online: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO usuarios (tenant_id, name, username, password_hash, role, atendente_id, active)
		VALUES ($1,'Offline','offline-distrib','x','atendente',$2,true)`, tenantID, offlineID); err != nil {
		t.Fatalf("criando usuário offline: %v", err)
	}
	for _, phone := range []string{"5511900000001", "5511900000002"} {
		if _, err := pool.Exec(ctx, `INSERT INTO conversations (tenant_id, channel, customer_phone, status, atendente_id)
			VALUES ($1,'whatsapp',$2,'humano_atendendo',$3)`, tenantID, phone, onlineUserID); err != nil {
			t.Fatalf("criando conversa em carga: %v", err)
		}
	}

	available, err := s.ListAvailableAtendentes(ctx)
	if err != nil {
		t.Fatalf("listando disponíveis: %v", err)
	}
	if len(available) != 1 {
		t.Fatalf("esperava só o atendente online, obtido %d", len(available))
	}
	if available[0].Capacidade != 2 || available[0].Carga != 2 {
		t.Fatalf("capacidade/carga incorretas: %#v", available[0])
	}

	fallback, err := s.ListAtendentesFallback(ctx)
	if err != nil {
		t.Fatalf("listando fallback: %v", err)
	}
	if len(fallback) != 2 {
		t.Fatalf("fallback deveria incluir online e offline, obtido %d", len(fallback))
	}
}
