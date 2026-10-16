package db

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func mustPoolConfig(t *testing.T, dsn string) *pgxpool.Config {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestSameDatabaseTarget(t *testing.T) {
	admin := mustPoolConfig(t, "postgres://admin:secret@postgres:5432/agente_ia?sslmode=disable")
	runtime := mustPoolConfig(t, "postgres://agente_app:secret2@postgres:5432/agente_ia?sslmode=disable")
	if !sameDatabaseTarget(admin, runtime) {
		t.Fatal("mesmo host, porta e banco deveriam ser aceitos")
	}
	otherDatabase := mustPoolConfig(t, "postgres://agente_app:secret2@postgres:5432/outro?sslmode=disable")
	if sameDatabaseTarget(admin, otherDatabase) {
		t.Fatal("bancos diferentes não podem compartilhar o bootstrap de role")
	}
	otherHost := mustPoolConfig(t, "postgres://agente_app:secret2@db2:5432/agente_ia?sslmode=disable")
	if sameDatabaseTarget(admin, otherHost) {
		t.Fatal("hosts diferentes não podem compartilhar o bootstrap de role")
	}
}
