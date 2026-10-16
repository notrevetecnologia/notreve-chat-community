package channel

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/notrevetecnologia/notreve-chat-community/internal/db"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func newPhantomTestStore(t *testing.T) (*store.Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping phantom conversation integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("criando pool de teste: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(context.Background(), pool, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("aplicando migrations: %v", err)
	}
	var tenantID int
	slug := fmt.Sprintf("phantom-%d", time.Now().UnixNano())
	if err := pool.QueryRow(context.Background(), `INSERT INTO tenants (slug, name, status, plan, timezone, locale)
		VALUES ($1,$2,'active','integration-test','UTC','pt-BR') RETURNING id`, slug, "Phantom").Scan(&tenantID); err != nil {
		t.Fatalf("criando tenant: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM messages WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM conversations WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM channels WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM settings WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, tenantID)
	})
	if _, err := pool.Exec(context.Background(), `INSERT INTO settings (tenant_id) VALUES ($1)`, tenantID); err != nil {
		t.Fatalf("criando settings: %v", err)
	}
	return store.New(pool), store.WithTenantID(context.Background(), tenantID)
}

func TestHandleCallDoesNotCreateConversation(t *testing.T) {
	st, ctx := newPhantomTestStore(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	ch := store.Channel{Type: "whatsapp", Name: "WhatsApp", BaseURL: server.URL, Token: "token", Active: true, ResponderChamadas: true}
	channelID, err := st.CreateChannel(ctx, ch)
	if err != nil {
		t.Fatalf("criando canal: %v", err)
	}
	ch.ID = channelID

	n := NewNotreve(NewProcessor(st, nil, nil), st)
	raw := []byte(`{"type":"CallOffer","event":{"CallCreator":"5511999999999@s.whatsapp.net","CallID":"call-123"}}`)
	n.handleCall(ctx, raw, server.URL, "token", ch)

	convs, err := st.ListConversations(ctx)
	if err != nil {
		t.Fatalf("listando conversas: %v", err)
	}
	if len(convs) != 0 {
		t.Fatalf("chamada não pode criar conversa, obtido %d", len(convs))
	}
}
