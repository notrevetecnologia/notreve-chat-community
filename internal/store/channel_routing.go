package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/notrevetecnologia/notreve-chat-community/internal/securesecret"
)

func scanRoutedChannel(row interface{ Scan(...interface{}) error }) (Channel, error) {
	var c Channel
	err := row.Scan(&c.ID, &c.TenantID, &c.Type, &c.Name, &c.BaseURL, &c.Token, &c.InstanceID,
		&c.Active, &c.RespondGrupos, &c.ResponderChamadas, &c.CanCampaign, &c.WidgetConfig, &c.PublicID, &c.CreatedAt)
	return c, err
}

const routedChannelColumns = `id, tenant_id, type, name, base_url, token, instance_id, active,
	respond_grupos, responder_chamadas, can_campaign, widget_config, public_id, created_at`

func (s *Store) ChannelByTokenRouted(ctx context.Context, identifier string) (Channel, error) {
	identifier = strings.TrimSpace(identifier)
	return s.uniqueRoutedChannelArgs(ctx,
		`WHERE type='whatsapp' AND active=true AND (instance_id=$1 OR token_hash=$2)`,
		identifier, securesecret.LookupHash(identifier))
}

// ChannelByNameRouted é o fallback para webhooks que identificam a instância
// pelo nome operacional em vez do instance_id/token.
func (s *Store) ChannelByNameRouted(ctx context.Context, name string) (Channel, error) {
	return s.uniqueRoutedChannelArgs(ctx,
		`WHERE type='whatsapp' AND active=true AND lower(trim(name))=lower(trim($1))`,
		strings.TrimSpace(name))
}

func (s *Store) ChannelByPublicIDRouted(ctx context.Context, publicID string) (Channel, error) {
	return s.uniqueRoutedChannelArgs(ctx, `WHERE public_id=$1 AND type='webchat' AND active=true`, strings.TrimSpace(publicID))
}

func (s *Store) ChannelByTelegramTokenRouted(ctx context.Context, publicID string) (Channel, error) {
	publicID = strings.TrimSpace(publicID)
	return s.uniqueRoutedChannelArgs(ctx, `WHERE type='telegram' AND active=true AND public_id=$1`, publicID)
}

func (s *Store) uniqueRoutedChannelArgs(ctx context.Context, predicate string, args ...interface{}) (Channel, error) {
	lookupCtx := WithGlobalDataAccess(ctx)
	rows, err := s.pool.Query(lookupCtx, `SELECT `+routedChannelColumns+` FROM channels `+predicate, args...)
	if err != nil {
		return Channel{}, err
	}
	defer rows.Close()
	var channel Channel
	count := 0
	for rows.Next() {
		count++
		if count > 1 {
			return Channel{}, fmt.Errorf("ambiguous channel identifier")
		}
		channel, err = scanRoutedChannel(rows)
		if err != nil {
			return Channel{}, err
		}
	}
	if err := rows.Err(); err != nil {
		return Channel{}, err
	}
	if count == 0 {
		return Channel{}, pgx.ErrNoRows
	}
	return decryptChannelSecrets(channel)
}

const reusableConversationWhere = `c.tenant_id=$1 AND c.channel_id=$2 AND c.channel=$4
	AND (c.customer_phone=$3 OR (
		$3 ~ '[0-9]' AND regexp_replace(c.customer_phone, '[^0-9]', '', 'g') <> ''
		AND regexp_replace(c.customer_phone, '[^0-9]', '', 'g') = regexp_replace($3, '[^0-9]', '', 'g')
	))
	AND (c.status <> 'closed' OR c.satisfacao IN ('aguardando','aguardando_operador'))`

func (s *Store) FindReusableConversationForChannel(ctx context.Context, channelID int, phone string) (Conversation, error) {
	if err := s.validateChannelReference(ctx, TenantID(ctx), channelID); err != nil {
		return Conversation{}, err
	}
	c, err := scanConv(s.pool.QueryRow(ctx, convSelect+` WHERE c.tenant_id=$1 AND c.channel_id=$2 AND (
		c.customer_phone=$3 OR (
			$3 ~ '[0-9]' AND regexp_replace(c.customer_phone, '[^0-9]', '', 'g') <> ''
			AND regexp_replace(c.customer_phone, '[^0-9]', '', 'g') = regexp_replace($3, '[^0-9]', '', 'g')
		)
	) AND (c.status <> 'closed' OR c.satisfacao IN ('aguardando','aguardando_operador'))
		ORDER BY c.updated_at DESC LIMIT 1`, TenantID(ctx), channelID, phone))
	if err == nil {
		c.ChannelID = &channelID
	}
	return c, err
}

func (s *Store) FindOrCreateConversationForChannel(ctx context.Context, c Conversation) (id int, reused bool, err error) {
	if c.ChannelID == nil {
		return 0, false, fmt.Errorf("channel id required")
	}
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return 0, false, err
	}
	if err := s.validateConversationReferences(ctx, tenantID, c); err != nil {
		return 0, false, err
	}
	identity := conversationIdentityKey(c.CustomerPhone)
	lockKey := fmt.Sprintf("conversation:%d:%s:%s", tenantID, strings.TrimSpace(c.Channel), identity)

	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
			return err
		}

		conversation, findErr := scanConv(tx.QueryRow(ctx, convSelect+` WHERE `+reusableConversationWhere+`
			ORDER BY c.updated_at DESC LIMIT 1 FOR UPDATE OF c`, tenantID, *c.ChannelID, c.CustomerPhone, c.Channel))
		if findErr == nil {
			id = conversation.ID
			reused = true
			_, err := tx.Exec(ctx, `UPDATE conversations
				SET channel_id=$2, channel=$3,
					customer_phone=CASE WHEN $4 ~ '[0-9]' THEN $4 ELSE customer_phone END,
					customer_name=CASE WHEN btrim($5) <> '' THEN $5 ELSE customer_name END,
					origem=CASE WHEN btrim($6) <> '' THEN $6 ELSE origem END,
					updated_at=now()
				WHERE tenant_id=$1 AND id=$7`, tenantID, *c.ChannelID, c.Channel,
				c.CustomerPhone, c.CustomerName, c.Origem, id)
			return err
		}
		if findErr != pgx.ErrNoRows {
			return findErr
		}

		hash := c.Hash
		if hash == "" {
			hash = newConversationHash()
		}
		return tx.QueryRow(ctx, `INSERT INTO conversations
			(tenant_id, channel_id, agent_id, channel, customer_name, customer_phone, status, setor_id, origem, hash)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
			tenantID, c.ChannelID, c.AgentID, c.Channel, c.CustomerName, c.CustomerPhone,
			c.Status, c.SetorID, c.Origem, hash).Scan(&id)
	})
	return id, reused, err
}

func conversationIdentityKey(phone string) string {
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	if digits.Len() >= 7 {
		return digits.String()
	}
	return strings.TrimSpace(phone)
}

func (s *Store) CreateConversationForChannel(ctx context.Context, c Conversation) (int, error) {
	var id int
	tenantID, err := TenantIDRequired(ctx)
	if err != nil {
		return 0, err
	}
	if err := s.validateConversationReferences(ctx, tenantID, c); err != nil {
		return 0, err
	}
	hash := c.Hash
	if hash == "" {
		hash = newConversationHash()
	}
	err = s.pool.QueryRow(ctx, `INSERT INTO conversations
		(tenant_id, channel_id, agent_id, channel, customer_name, customer_phone, status, setor_id, origem, hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		tenantID, c.ChannelID, c.AgentID, c.Channel, c.CustomerName, c.CustomerPhone,
		c.Status, c.SetorID, c.Origem, hash).Scan(&id)
	return id, err
}

func (s *Store) ListAgentsForTenant(ctx context.Context, tenantID int) ([]Agent, error) {
	ctx = WithTenantID(ctx, tenantID)
	rows, err := s.pool.Query(ctx, `SELECT `+agentCols+` FROM agents a
		LEFT JOIN providers p ON p.id=a.provider_id AND p.tenant_id=a.tenant_id
		WHERE a.tenant_id=$1 ORDER BY a.id ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Agent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetAgentForTenant(ctx context.Context, tenantID, id int) (Agent, error) {
	ctx = WithTenantID(ctx, tenantID)
	return scanAgent(s.pool.QueryRow(ctx, `SELECT `+agentCols+` FROM agents a
		LEFT JOIN providers p ON p.id=a.provider_id AND p.tenant_id=a.tenant_id
		WHERE a.tenant_id=$1 AND a.id=$2`, tenantID, id))
}

func (s *Store) AddMessageForTenant(ctx context.Context, tenantID int, m Message) error {
	ctx = WithTenantID(ctx, tenantID)
	tag, err := s.pool.Exec(ctx, `INSERT INTO messages
		(tenant_id, conversation_id, role, content, source, wa_msg_id, media_type, media_b64, media_mime, media_name, lat, lng, contact_name, contact_tel)
		SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14
		WHERE EXISTS (SELECT 1 FROM conversations WHERE tenant_id=$1 AND id=$2)`,
		tenantID, m.ConversationID, m.Role, m.Content, m.Source, m.WAMsgID, m.MediaType, m.MediaB64,
		m.MediaMime, m.MediaName, m.Lat, m.Lng, m.ContactName, m.ContactTel)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func (s *Store) MessageExistsWAMsgIDForTenant(ctx context.Context, tenantID int, waMsgID string) (bool, error) {
	ctx = WithTenantID(ctx, tenantID)
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE tenant_id=$1 AND wa_msg_id=$2 AND role='user'`, tenantID, waMsgID).Scan(&count)
	return count > 0, err
}

// MessageExistsWAMsgIDForChannel limita o dedupe à conexão que recebeu o
// evento. IDs de mensagem do WhatsApp não devem ser tratados como globais
// entre várias instâncias do mesmo tenant.
func (s *Store) MessageExistsWAMsgIDForChannel(ctx context.Context, tenantID, channelID int, waMsgID string) (bool, error) {
	ctx = WithTenantID(ctx, tenantID)
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*)
		FROM messages m
		JOIN conversations c ON c.tenant_id=m.tenant_id AND c.id=m.conversation_id
		WHERE m.tenant_id=$1 AND c.channel_id=$2 AND m.wa_msg_id=$3 AND m.role='user'`,
		tenantID, channelID, waMsgID).Scan(&count)
	return count > 0, err
}

func (s *Store) EnsureWebchatSessionForTenant(ctx context.Context, tenantID, channelID int, visitorID, tokenHash string) (bool, error) {
	ctx = WithTenantID(ctx, tenantID)
	if _, err := s.pool.Exec(ctx, `INSERT INTO webchat_sessions
		(tenant_id, channel_id, visitor_id, token_hash)
		SELECT $1,$2,$3,$4 WHERE EXISTS (SELECT 1 FROM channels WHERE tenant_id=$1 AND id=$2)
		ON CONFLICT (channel_id, visitor_id) DO NOTHING`, tenantID, channelID, visitorID, tokenHash); err != nil {
		return false, err
	}
	return s.ValidateWebchatSessionForTenant(ctx, tenantID, channelID, visitorID, tokenHash)
}

func (s *Store) ValidateWebchatSessionForTenant(ctx context.Context, tenantID, channelID int, visitorID, tokenHash string) (bool, error) {
	ctx = WithTenantID(ctx, tenantID)
	var valid bool
	err := s.pool.QueryRow(ctx, `UPDATE webchat_sessions SET last_seen=now()
		WHERE tenant_id=$1 AND channel_id=$2 AND visitor_id=$3 AND token_hash=$4
		RETURNING true`, tenantID, channelID, visitorID, tokenHash).Scan(&valid)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	return valid, err
}
