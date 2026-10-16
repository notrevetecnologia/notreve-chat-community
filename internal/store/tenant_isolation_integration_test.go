package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/notrevetecnologia/notreve-chat-community/internal/db"
)

func newSaaSIsolationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping SaaS isolation integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("criando pool de teste: %v", err)
	}
	if err := db.Migrate(context.Background(), pool, filepath.Join("..", "..", "migrations")); err != nil {
		pool.Close()
		t.Fatalf("aplicando migrations no banco de teste: %v", err)
	}
	return pool
}

func createIsolationTenant(t *testing.T, pool *pgxpool.Pool, suffix string) int {
	t.Helper()
	var id int
	slug := fmt.Sprintf("isolation-%s-%d", suffix, time.Now().UnixNano())
	err := pool.QueryRow(context.Background(), `INSERT INTO tenants (slug, name, status, plan, timezone, locale)
		VALUES ($1,$2,'active','integration-test','UTC','pt-BR') RETURNING id`, slug, "Isolation "+suffix).Scan(&id)
	if err != nil {
		t.Fatalf("criando tenant %s: %v", suffix, err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO tenant_limits (tenant_id) VALUES ($1) ON CONFLICT DO NOTHING`, id); err != nil {
		t.Fatalf("criando limites do tenant %s: %v", suffix, err)
	}
	return id
}

func TestTenantIsolationAcrossCoreResources(t *testing.T) {
	pool := newSaaSIsolationPool(t)
	defer pool.Close()

	tenantA := createIsolationTenant(t, pool, "a")
	tenantB := createIsolationTenant(t, pool, "b")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1 OR id=$2`, tenantA, tenantB)
	})

	s := New(pool)
	ctxA := WithTenantID(context.Background(), tenantA)
	ctxB := WithTenantID(context.Background(), tenantB)

	channelA, err := s.CreateChannel(ctxA, Channel{Type: "webchat", Name: "Canal A", Active: true, WidgetConfig: "{}", PublicID: fmt.Sprintf("test-a-%d", time.Now().UnixNano())})
	if err != nil {
		t.Fatalf("criando canal A: %v", err)
	}
	channelB, err := s.CreateChannel(ctxB, Channel{Type: "webchat", Name: "Canal B", Active: true, WidgetConfig: "{}", PublicID: fmt.Sprintf("test-b-%d", time.Now().UnixNano())})
	if err != nil {
		t.Fatalf("criando canal B: %v", err)
	}
	if _, err := s.GetChannelByID(ctxB, channelA); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("tenant B acessou canal A: %v", err)
	}
	if _, err := s.GetChannelByID(ctxA, channelB); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("tenant A acessou canal B: %v", err)
	}

	conversationA, err := s.CreateConversation(ctxA, Conversation{ChannelID: &channelA, Channel: "webchat", CustomerName: "Cliente A", CustomerPhone: "visitor-a", Status: "ia"})
	if err != nil {
		t.Fatalf("criando conversa A: %v", err)
	}
	if _, err := s.CreateConversation(ctxB, Conversation{ChannelID: &channelA, Channel: "webchat", CustomerName: "Invasão", CustomerPhone: "visitor-x", Status: "ia"}); err == nil {
		t.Fatal("tenant B conseguiu criar conversa usando canal do tenant A")
	}
	if err := s.AddMessage(ctxA, Message{ConversationID: conversationA, Role: "user", Source: "webchat", Content: "mensagem A"}); err != nil {
		t.Fatalf("gravando mensagem A: %v", err)
	}
	if err := s.AddMessage(ctxB, Message{ConversationID: conversationA, Role: "user", Source: "webchat", Content: "mensagem indevida"}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("tenant B escreveu na conversa A: %v", err)
	}
	messagesB, err := s.ListMessages(ctxB, conversationA)
	if err != nil {
		t.Fatalf("listando mensagens pelo tenant B: %v", err)
	}
	if len(messagesB) != 0 {
		t.Fatalf("tenant B leu %d mensagens da conversa A", len(messagesB))
	}

	contactA, err := s.CreateContato(ctxA, Contato{Name: "Contato A", Phone: fmt.Sprintf("5511%d", time.Now().UnixNano()), Email: "a@example.test"})
	if err != nil {
		t.Fatalf("criando contato A: %v", err)
	}
	if _, err := s.GetContato(ctxB, contactA); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("tenant B acessou contato A: %v", err)
	}

	username := fmt.Sprintf("isolation-user-%d", time.Now().UnixNano())
	userA, err := s.CreateUsuario(ctxA, Usuario{Name: "Usuário A", Username: username, PasswordHash: "integration-test-hash", Role: "atendente", Active: true})
	if err != nil {
		t.Fatalf("criando usuário A: %v", err)
	}
	if _, err := s.GetUsuarioByID(ctxB, userA); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("tenant B acessou usuário A: %v", err)
	}

	var agentA int
	if err := pool.QueryRow(context.Background(), `INSERT INTO agents (tenant_id, name) VALUES ($1,$2) RETURNING id`, tenantA, "Agente A").Scan(&agentA); err != nil {
		t.Fatalf("criando agente A: %v", err)
	}
	if _, err := s.GetAgent(ctxB, agentA); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("tenant B acessou agente A: %v", err)
	}
}
