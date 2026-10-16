package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/notrevetecnologia/notreve-chat-community/internal/tenantctx"
)

// Preflight valida inconsistências legadas que não podem ser corrigidas de
// forma determinística por uma migration. Ele não altera dados. Backfills
// seguros (como tenant_id nulo -> tenant 1 em migrations antigas) continuam
// sendo responsabilidade das próprias migrations.
func Preflight(ctx context.Context, pool *pgxpool.Pool) error {
	ctx = tenantctx.WithRLSBypass(ctx)
	problems := make([]string, 0)

	if exists, err := relationExists(ctx, pool, "usuarios"); err != nil {
		return err
	} else if exists {
		var count int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM (
			SELECT lower(btrim(username)) normalized
			FROM usuarios
			WHERE btrim(username) <> ''
			GROUP BY lower(btrim(username))
			HAVING COUNT(*) > 1
		) d`).Scan(&count); err != nil {
			return fmt.Errorf("preflight usuarios: %w", err)
		}
		if count > 0 {
			problems = append(problems, fmt.Sprintf("usuarios contém %d login(s) duplicado(s) ignorando maiúsculas/minúsculas; o painel compartilhado exige identidade global única", count))
		}
	}

	if exists, err := relationExists(ctx, pool, "tenant_domains"); err != nil {
		return err
	} else if exists {
		if hasHostname, err := columnExists(ctx, pool, "tenant_domains", "hostname"); err != nil {
			return err
		} else if hasHostname {
			var count int
			if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM (
				SELECT lower(btrim(hostname)) normalized
				FROM tenant_domains
				WHERE btrim(hostname) <> ''
				GROUP BY lower(btrim(hostname))
				HAVING COUNT(*) > 1
			) d`).Scan(&count); err != nil {
				return fmt.Errorf("preflight tenant_domains: %w", err)
			}
			if count > 0 {
				problems = append(problems, fmt.Sprintf("tenant_domains contém %d hostname(s) duplicado(s)", count))
			}
		}
	}

	tenantsExist, err := relationExists(ctx, pool, "tenants")
	if err != nil {
		return err
	}
	if tenantsExist {
		rows, err := pool.Query(ctx, `SELECT c.table_name
			FROM information_schema.columns c
			JOIN information_schema.tables t
			  ON t.table_schema=c.table_schema AND t.table_name=c.table_name
			WHERE c.table_schema='public' AND c.column_name='tenant_id' AND t.table_type='BASE TABLE'
			ORDER BY c.table_name`)
		if err != nil {
			return fmt.Errorf("preflight tabelas tenant: %w", err)
		}
		var tables []string
		for rows.Next() {
			var table string
			if err := rows.Scan(&table); err != nil {
				rows.Close()
				return err
			}
			tables = append(tables, table)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()

		for _, table := range tables {
			quoted := pgx.Identifier{table}.Sanitize()
			query := `SELECT COUNT(*) FROM ` + quoted + ` x WHERE x.tenant_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenants t WHERE t.id=x.tenant_id)`
			var count int
			if err := pool.QueryRow(ctx, query).Scan(&count); err != nil {
				return fmt.Errorf("preflight %s: %w", table, err)
			}
			if count > 0 {
				problems = append(problems, fmt.Sprintf("%s contém %d registro(s) apontando para tenant inexistente", table, count))
			}
		}
	}

	if exists, err := relationExists(ctx, pool, "channels"); err != nil {
		return err
	} else if exists {
		if hasPublicID, err := columnExists(ctx, pool, "channels", "public_id"); err != nil {
			return err
		} else if hasPublicID {
			var count int
			if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM (
				SELECT public_id FROM channels WHERE btrim(public_id) <> ''
				GROUP BY public_id HAVING COUNT(*) > 1
			) d`).Scan(&count); err != nil {
				return fmt.Errorf("preflight channels: %w", err)
			}
			if count > 0 {
				problems = append(problems, fmt.Sprintf("channels contém %d public_id(s) duplicado(s)", count))
			}
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("preflight bloqueou a migration: %s", strings.Join(problems, "; "))
	}
	return nil
}

func relationExists(ctx context.Context, pool *pgxpool.Pool, name string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `SELECT to_regclass('public.' || $1) IS NOT NULL`, name).Scan(&exists)
	return exists, err
}

func columnExists(ctx context.Context, pool *pgxpool.Pool, table, column string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_schema='public' AND table_name=$1 AND column_name=$2
	)`, table, column).Scan(&exists)
	return exists, err
}
