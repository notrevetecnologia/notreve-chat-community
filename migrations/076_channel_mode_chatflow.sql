-- O modo de atendimento (IA livre x chat flow) passa a ser decidido pelo CANAL.
-- channels.mode = 'ai' (default) usa channels.agent_id; channels.mode='chatflow'
-- usa channels.chatflow_id. Antes de remover agents.chatflow_id, faz backfill
-- dos canais que apontam para agentes com fluxo definido.
ALTER TABLE channels ADD COLUMN IF NOT EXISTS mode TEXT NOT NULL DEFAULT 'ai';
ALTER TABLE channels ADD COLUMN IF NOT EXISTS chatflow_id INTEGER REFERENCES chatflow(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_channels_chatflow ON channels (tenant_id, chatflow_id);

UPDATE channels c
SET chatflow_id = a.chatflow_id,
    mode = 'chatflow'
FROM agents a
WHERE c.agent_id = a.id
  AND c.tenant_id = a.tenant_id
  AND a.chatflow_id IS NOT NULL;

ALTER TABLE agents DROP COLUMN IF EXISTS chatflow_id;
