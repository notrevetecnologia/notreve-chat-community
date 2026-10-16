package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUnsafeRuntimeDatabaseRole = errors.New("runtime database role must not be superuser or bypass RLS")

func sameDatabaseTarget(admin, runtime *pgxpool.Config) bool {
	if admin == nil || runtime == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(admin.ConnConfig.Host), strings.TrimSpace(runtime.ConnConfig.Host)) &&
		admin.ConnConfig.Port == runtime.ConnConfig.Port &&
		strings.EqualFold(strings.TrimSpace(admin.ConnConfig.Database), strings.TrimSpace(runtime.ConnConfig.Database))
}

// EnsureRuntimeRole prepara o usuário usado pela aplicação depois que as
// migrations foram executadas pela conexão administrativa. O runtime jamais
// deve usar o POSTGRES_USER do container: superusers/BYPASSRLS ignoram RLS.
func EnsureRuntimeRole(ctx context.Context, admin *pgxpool.Pool, runtimeURL string) error {
	if admin == nil {
		return errors.New("admin database pool is required")
	}
	runtimeCfg, err := pgxpool.ParseConfig(strings.TrimSpace(runtimeURL))
	if err != nil {
		return fmt.Errorf("configurando role runtime: %w", err)
	}
	adminCfg := admin.Config()
	if !sameDatabaseTarget(adminCfg, runtimeCfg) {
		return errors.New("AGENTE_IA_ADMIN_DB e AGENTE_IA_DB precisam apontar para o mesmo banco")
	}

	username := strings.TrimSpace(runtimeCfg.ConnConfig.User)
	password := runtimeCfg.ConnConfig.Password
	database := strings.TrimSpace(runtimeCfg.ConnConfig.Database)
	adminUser := strings.TrimSpace(adminCfg.ConnConfig.User)
	if username == "" || password == "" || database == "" {
		return errors.New("AGENTE_IA_DB precisa conter usuário, senha e banco")
	}
	if strings.EqualFold(username, adminUser) {
		return fmt.Errorf("%w: runtime e administração usam o mesmo usuário", ErrUnsafeRuntimeDatabaseRole)
	}

	var exists bool
	err = admin.QueryRow(ctx, `SELECT true FROM pg_roles WHERE rolname=$1`, username).Scan(&exists)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("consultando role runtime: %w", err)
	}

	quotedRole := pgx.Identifier{username}.Sanitize()
	var passwordLiteral string
	if err := admin.QueryRow(ctx, `SELECT quote_literal($1)`, password).Scan(&passwordLiteral); err != nil {
		return fmt.Errorf("protegendo senha da role runtime: %w", err)
	}

	if !exists {
		statement := `CREATE ROLE ` + quotedRole + ` LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS PASSWORD ` + passwordLiteral
		if _, err := admin.Exec(ctx, statement); err != nil {
			return fmt.Errorf("criando role runtime: %w", err)
		}
	} else {
		// Corrige também uma role antiga criada com privilégios excessivos.
		statement := `ALTER ROLE ` + quotedRole + ` LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS PASSWORD ` + passwordLiteral
		if _, err := admin.Exec(ctx, statement); err != nil {
			return fmt.Errorf("endurecendo role runtime: %w", err)
		}
	}

	quotedDatabase := pgx.Identifier{database}.Sanitize()
	grants := []string{
		`GRANT CONNECT ON DATABASE ` + quotedDatabase + ` TO ` + quotedRole,
		`GRANT USAGE ON SCHEMA public TO ` + quotedRole,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO ` + quotedRole,
		`GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO ` + quotedRole,
		`GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO ` + quotedRole,
	}
	for _, statement := range grants {
		if _, err := admin.Exec(ctx, statement); err != nil {
			return fmt.Errorf("concedendo privilégios runtime: %w", err)
		}
	}

	// Objetos de migrations futuras são criados pelo usuário administrativo.
	// Os default privileges evitam que uma nova tabela/função quebre o runtime.
	quotedAdmin := pgx.Identifier{adminUser}.Sanitize()
	defaults := []string{
		`ALTER DEFAULT PRIVILEGES FOR ROLE ` + quotedAdmin + ` IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO ` + quotedRole,
		`ALTER DEFAULT PRIVILEGES FOR ROLE ` + quotedAdmin + ` IN SCHEMA public GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO ` + quotedRole,
		`ALTER DEFAULT PRIVILEGES FOR ROLE ` + quotedAdmin + ` IN SCHEMA public GRANT EXECUTE ON FUNCTIONS TO ` + quotedRole,
	}
	for _, statement := range defaults {
		if _, err := admin.Exec(ctx, statement); err != nil {
			return fmt.Errorf("configurando privilégios futuros runtime: %w", err)
		}
	}

	var finalSuperuser, finalBypass bool
	if err := admin.QueryRow(ctx, `SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname=$1`, username).Scan(&finalSuperuser, &finalBypass); err != nil {
		return fmt.Errorf("validando role runtime: %w", err)
	}
	if finalSuperuser || finalBypass {
		return ErrUnsafeRuntimeDatabaseRole
	}
	return nil
}
