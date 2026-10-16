package store

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func tenantSetting(ctx context.Context) (string, error) {
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(tenantID), nil
}

func (s *Store) WithTenantConn(ctx context.Context, fn func(context.Context, *pgxpool.Conn) error) error {
	tenant, err := tenantSetting(ctx)
	if err != nil {
		return err
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	defer conn.Exec(context.Background(), `SELECT set_config('app.tenant_id', '', false)`)
	if _, err := conn.Exec(ctx, `SELECT set_config('app.tenant_id', $1, false)`, tenant); err != nil {
		return err
	}
	return fn(ctx, conn)
}

func (s *Store) WithTenantTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	tenant, err := tenantSetting(ctx)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenant); err != nil {
		return err
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
