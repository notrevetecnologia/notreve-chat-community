ALTER TABLE conversations
  ADD COLUMN IF NOT EXISTS channel_id INTEGER REFERENCES channels(id) ON DELETE SET NULL;

UPDATE conversations c
SET channel_id = (
  SELECT MIN(ch.id)
  FROM channels ch
  WHERE ch.tenant_id = c.tenant_id
    AND ch.type = c.channel
    AND ch.active
  HAVING COUNT(*) = 1
)
WHERE c.channel_id IS NULL
  AND (
    SELECT COUNT(*)
    FROM channels same_type
    WHERE same_type.tenant_id = c.tenant_id
      AND same_type.type = c.channel
      AND same_type.active
  ) = 1;

ALTER TABLE webchat_sessions
  ADD COLUMN IF NOT EXISTS conversation_id INTEGER REFERENCES conversations(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_conversations_tenant_channel_phone
  ON conversations (tenant_id, channel_id, customer_phone, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_conversations_tenant_channel
  ON conversations (tenant_id, channel_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_messages_tenant_conversation
  ON messages (tenant_id, conversation_id, id);

CREATE INDEX IF NOT EXISTS idx_webchat_sessions_tenant_channel_visitor
  ON webchat_sessions (tenant_id, channel_id, visitor_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_channels_public_id_nonempty
  ON channels (public_id)
  WHERE public_id <> '';
