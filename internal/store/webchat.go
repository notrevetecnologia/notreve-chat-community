package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// BindWebchatSession associa a sessão pública à conversa criada pelo
// processador. O vínculo evita depender apenas de telefone/visitor_id quando
// há mais de um canal webchat ou quando o visitante inicia um novo protocolo.
func (s *Store) BindWebchatSession(ctx context.Context, channelID int, visitorID, tokenHash string, conversationID int) error {
	var bound bool
	err := s.pool.QueryRow(ctx, `UPDATE webchat_sessions
		SET conversation_id=$1, last_seen=now()
		WHERE tenant_id=$2 AND channel_id=$3 AND visitor_id=$4 AND token_hash=$5
		  AND EXISTS (SELECT 1 FROM conversations c WHERE c.tenant_id=$2 AND c.id=$1 AND c.channel_id=$3)
		RETURNING true`, conversationID, TenantID(ctx), channelID, visitorID, tokenHash).Scan(&bound)
	if err == pgx.ErrNoRows {
		return pgx.ErrNoRows
	}
	return err
}

func (s *Store) WebchatSessionConversation(ctx context.Context, channelID int, visitorID, tokenHash string) (int, error) {
	var conversationID int
	err := s.pool.QueryRow(ctx, `SELECT conversation_id FROM webchat_sessions
		WHERE tenant_id=$1 AND channel_id=$2 AND visitor_id=$3 AND token_hash=$4
		AND conversation_id IS NOT NULL`, TenantID(ctx), channelID, visitorID, tokenHash).Scan(&conversationID)
	return conversationID, err
}
