package store

import "context"

const (
	defaultWatchdogMinutes = 5
	maxWatchdogBatch       = 50
	defaultMediaTTLDays    = 90
)

// ListOrphanConversations retorna conversas em modo IA cuja última mensagem é
// do cliente e segue sem resposta há mais de `minutes`. Cobre travamentos
// silenciosos do pipeline (crash/deadlock/timeout entre gravar a entrada e
// enviar a resposta), que não geram exceção tratável em Process.
func (s *Store) ListOrphanConversations(ctx context.Context, minutes, limit int) ([]Conversation, error) {
	tenantID, err := tenantJobID(ctx)
	if err != nil {
		return nil, err
	}
	if minutes < 1 {
		minutes = defaultWatchdogMinutes
	}
	if limit < 1 || limit > maxWatchdogBatch {
		limit = maxWatchdogBatch
	}
	rows, err := s.pool.Query(ctx, convSelect+` WHERE c.tenant_id=$1 AND c.status='ia' AND c.bot_blocked=false
		AND (SELECT m.role FROM messages m WHERE m.tenant_id=c.tenant_id AND m.conversation_id=c.id ORDER BY m.id DESC LIMIT 1)='user'
		AND (SELECT m.created_at FROM messages m WHERE m.tenant_id=c.tenant_id AND m.conversation_id=c.id ORDER BY m.id DESC LIMIT 1) < now() - make_interval(mins => $2)
		ORDER BY c.updated_at ASC LIMIT $3`, tenantID, minutes, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		c, err := scanConv(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// PurgeOldMedia limpa o base64 de mensagens antigas (histórico preservado),
// liberando o maior acúmulo do banco. Só toca mídia mais velha que o TTL em
// conversas encerradas ou paradas há mais que o TTL — nunca em conversa ativa.
func (s *Store) PurgeOldMedia(ctx context.Context, ttlDays int) (int64, error) {
	tenantID, err := tenantJobID(ctx)
	if err != nil {
		return 0, err
	}
	if ttlDays < 1 {
		ttlDays = defaultMediaTTLDays
	}
	tag, err := s.pool.Exec(ctx, `UPDATE messages m SET media_b64=''
		WHERE m.tenant_id=$1 AND m.media_b64<>'' AND m.created_at < now() - make_interval(days => $2)
		AND EXISTS (SELECT 1 FROM conversations c WHERE c.tenant_id=m.tenant_id AND c.id=m.conversation_id
			AND (c.status='closed' OR c.updated_at < now() - make_interval(days => $2)))`, tenantID, ttlDays)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// PurgeCampaignMedia limpa a mídia de campanhas já concluídas/canceladas que
// não serão reenviadas, evitando que o base64 fique no banco indefinidamente.
func (s *Store) PurgeCampaignMedia(ctx context.Context, ttlDays int) (int64, error) {
	tenantID, err := tenantJobID(ctx)
	if err != nil {
		return 0, err
	}
	if ttlDays < 1 {
		ttlDays = defaultMediaTTLDays
	}
	tag, err := s.pool.Exec(ctx, `UPDATE campanhas SET media_b64=''
		WHERE tenant_id=$1 AND media_b64<>'' AND status IN ('concluida','cancelada')
		AND updated_at < now() - make_interval(days => $2)`, tenantID, ttlDays)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
