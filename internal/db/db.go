package db

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/notrevetecnologia/notreve-chat-community/internal/tenantctx"
)

func applyTenantContext(ctx context.Context, conn *pgx.Conn) bool {
	tenantValue := ""
	if tenantID, ok := tenantctx.ID(ctx); ok && tenantID > 0 {
		tenantValue = strconv.Itoa(tenantID)
	}
	bypassValue := "off"
	if tenantctx.RLSBypass(ctx) {
		bypassValue = "on"
	}
	_, err := conn.Exec(ctx, `SELECT set_config('app.tenant_id', $1, false), set_config('app.rls_bypass', $2, false)`, tenantValue, bypassValue)
	return err == nil
}

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("configurando pool: %w", err)
	}
	cfg.BeforeAcquire = applyTenantContext
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("criando pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping no banco: %w", err)
	}
	return pool, nil
}
