package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/notrevetecnologia/notreve-chat-community/internal/tenantctx"
)

// Migrate aplica as migrations de migrations/ em ordem e registra cada arquivo
// na mesma transação da alteração de schema. Isso impede migrations parcialmente
// aplicadas de serem repetidas em estado inconsistente após uma falha.
func Migrate(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	ctx = tenantctx.WithRLSBypass(ctx)
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (filename TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("criando schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		var n int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE filename=$1`, f).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}

		// A base (001) só roda em banco novo. Se já existe a tabela conversations,
		// considera a base aplicada e segue só com as features (002+).
		if f == "001_init.sql" {
			var tbl string
			_ = pool.QueryRow(ctx, `SELECT to_regclass('public.conversations')`).Scan(&tbl)
			if tbl != "" {
				if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ('001_init.sql')`); err != nil {
					return err
				}
				continue
			}
		}

		body, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return err
		}
		if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(body)); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, f)
			return err
		}); err != nil {
			return fmt.Errorf("migration %s: %w", f, err)
		}
	}
	return nil
}
