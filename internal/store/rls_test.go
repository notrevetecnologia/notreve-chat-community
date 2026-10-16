package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/notrevetecnologia/notreve-chat-community/internal/db"
)

func newRLSAppPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("RLS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("RLS_TEST_DATABASE_URL not set; skipping RLS integration test")
	}
	pool, err := db.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("criando pool RLS de teste: %v", err)
	}
	var isSuper bool
	if err := pool.QueryRow(context.Background(), `SELECT rolsuper FROM pg_roles WHERE rolname=current_user`).Scan(&isSuper); err != nil {
		pool.Close()
		t.Fatalf("verificando role RLS: %v", err)
	}
	if isSuper {
		pool.Close()
		t.Fatal("RLS_TEST_DATABASE_URL não pode usar superuser")
	}
	return pool
}

func newRLSAdminPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_ADMIN_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_ADMIN_DATABASE_URL not set; skipping RLS integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("criando pool admin de teste: %v", err)
	}
	return pool
}

func createRLSFixture(t *testing.T, admin *pgxpool.Pool) (tenantA, tenantB, channelA, channelB int) {
	t.Helper()
	ctx := context.Background()
	nonce := time.Now().UnixNano()
	for suffix, target := range map[string]*int{"a": &tenantA, "b": &tenantB} {
		slug := fmt.Sprintf("rls-%s-%d", suffix, nonce)
		if err := admin.QueryRow(ctx, `INSERT INTO tenants (slug,name,status,plan,timezone,locale)
			VALUES ($1,$2,'active','rls-test','UTC','pt-BR') RETURNING id`, slug, "RLS "+suffix).Scan(target); err != nil {
			t.Fatalf("criando tenant RLS %s: %v", suffix, err)
		}
	}
	if err := admin.QueryRow(ctx, `INSERT INTO channels (tenant_id,type,name,active,widget_config,public_id)
		VALUES ($1,'webchat','RLS A',true,'{}',$2) RETURNING id`, tenantA, fmt.Sprintf("rls-a-%d", nonce)).Scan(&channelA); err != nil {
		t.Fatalf("criando canal RLS A: %v", err)
	}
	if err := admin.QueryRow(ctx, `INSERT INTO channels (tenant_id,type,name,active,widget_config,public_id)
		VALUES ($1,'webchat','RLS B',true,'{}',$2) RETURNING id`, tenantB, fmt.Sprintf("rls-b-%d", nonce)).Scan(&channelB); err != nil {
		t.Fatalf("criando canal RLS B: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1 OR id=$2`, tenantA, tenantB)
	})
	return
}

func TestRLSFailClosedAndTenantIsolation(t *testing.T) {
	admin := newRLSAdminPool(t)
	defer admin.Close()
	app := newRLSAppPool(t)
	defer app.Close()

	tenantA, tenantB, channelA, channelB := createRLSFixture(t, admin)
	ctxA := WithTenantID(context.Background(), tenantA)
	ctxB := WithTenantID(context.Background(), tenantB)

	var n int
	if err := app.QueryRow(ctxA, `SELECT COUNT(*) FROM channels WHERE id=$1 OR id=$2`, channelA, channelB).Scan(&n); err != nil {
		t.Fatalf("consulta tenant A: %v", err)
	}
	if n != 1 {
		t.Fatalf("tenant A viu %d canais entre A/B; esperado 1", n)
	}
	if err := app.QueryRow(ctxB, `SELECT COUNT(*) FROM channels WHERE id=$1 OR id=$2`, channelA, channelB).Scan(&n); err != nil {
		t.Fatalf("consulta tenant B: %v", err)
	}
	if n != 1 {
		t.Fatalf("tenant B viu %d canais entre A/B; esperado 1", n)
	}

	// Sem tenant e sem bypass explícito, RLS deve ser fail-closed.
	if err := app.QueryRow(context.Background(), `SELECT COUNT(*) FROM channels WHERE id=$1 OR id=$2`, channelA, channelB).Scan(&n); err != nil {
		t.Fatalf("consulta sem tenant: %v", err)
	}
	if n != 0 {
		t.Fatalf("consulta sem tenant viu %d canais; esperado 0", n)
	}

	// WITH CHECK deve impedir gravar uma linha de outro tenant.
	if _, err := app.Exec(ctxA, `INSERT INTO channels (tenant_id,type,name,active,widget_config,public_id)
		VALUES ($1,'webchat','RLS indevido',true,'{}',$2)`, tenantB, fmt.Sprintf("rls-x-%d", time.Now().UnixNano())); err == nil {
		t.Fatal("tenant A conseguiu inserir canal no tenant B")
	}

	// O bypass existe somente para fluxos globais deliberados.
	globalCtx := WithGlobalDataAccess(context.Background())
	if err := app.QueryRow(globalCtx, `SELECT COUNT(*) FROM channels WHERE id=$1 OR id=$2`, channelA, channelB).Scan(&n); err != nil {
		t.Fatalf("consulta global explícita: %v", err)
	}
	if n != 2 {
		t.Fatalf("bypass global explícito viu %d canais; esperado 2", n)
	}
}

func TestEveryTenantTableHasForcedRLSAndPolicy(t *testing.T) {
	app := newRLSAppPool(t)
	defer app.Close()
	rows, err := app.Query(context.Background(), `
		SELECT c.table_name, cls.relrowsecurity, cls.relforcerowsecurity,
		       EXISTS (
		         SELECT 1 FROM pg_policies p
		          WHERE p.schemaname='public' AND p.tablename=c.table_name AND p.policyname='tenant_context_isolation'
		       ) AS has_policy
		  FROM information_schema.columns c
		  JOIN pg_class cls ON cls.relname=c.table_name
		  JOIN pg_namespace ns ON ns.oid=cls.relnamespace AND ns.nspname='public'
		 WHERE c.table_schema='public' AND c.column_name='tenant_id' AND cls.relkind='r'
		 ORDER BY c.table_name`)
	if err != nil {
		t.Fatalf("auditando RLS: %v", err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var table string
		var enabled, forced, hasPolicy bool
		if err := rows.Scan(&table, &enabled, &forced, &hasPolicy); err != nil {
			t.Fatalf("lendo auditoria RLS: %v", err)
		}
		seen++
		if !enabled || !forced || !hasPolicy {
			t.Errorf("tabela %s sem hardening completo: enabled=%v forced=%v policy=%v", table, enabled, forced, hasPolicy)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("auditando RLS: %v", err)
	}
	if seen == 0 {
		t.Fatal("nenhuma tabela tenant-aware encontrada para auditoria RLS")
	}
}
