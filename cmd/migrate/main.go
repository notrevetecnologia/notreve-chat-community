package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/db"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func main() {
	adminDSN := strings.TrimSpace(os.Getenv("AGENTE_IA_ADMIN_DB"))
	runtimeDSN := strings.TrimSpace(os.Getenv("AGENTE_IA_DB"))
	productionBootstrap := adminDSN != ""
	if adminDSN == "" {
		// Compatibilidade exclusiva com teste/CI: nesse modo o comando só aplica
		// migrations. Em produção o job recebe uma conexão administrativa separada.
		adminDSN = runtimeDSN
	}
	if adminDSN == "" {
		log.Fatal("AGENTE_IA_ADMIN_DB (ou AGENTE_IA_DB no CI) é obrigatório")
	}
	if productionBootstrap && runtimeDSN == "" {
		log.Fatal("AGENTE_IA_DB é obrigatório para preparar a role runtime")
	}

	dir := os.Getenv("AGENTE_IA_MIGRATIONS")
	if dir == "" {
		dir = filepath.Join(".", "migrations")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, adminDSN)
	if err != nil {
		log.Fatalf("conectando ao banco administrativo: %v", err)
	}
	defer pool.Close()
	if err := db.Preflight(ctx, pool); err != nil {
		log.Fatalf("preflight: %v", err)
	}
	if err := db.Migrate(ctx, pool, dir); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	if strings.TrimSpace(os.Getenv("AGENTE_IA_SECRETS_KEY")) != "" {
		if err := store.New(pool).MigrateLegacySecrets(ctx); err != nil {
			log.Fatalf("migrando segredos legados: %v", err)
		}
	}
	if productionBootstrap {
		if err := db.EnsureRuntimeRole(ctx, pool, runtimeDSN); err != nil {
			log.Fatalf("preparando role runtime: %v", err)
		}
	}
	log.Print("migrations e bootstrap do banco concluídos com sucesso")
}
