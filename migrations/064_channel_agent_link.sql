-- Vínculo explícito entre um canal e o agente que o atende. Permite escolher
-- "qual agente responde neste número" sem depender da seleção por tipo/JSON.
-- Nulo mantém o comportamento anterior (agente da conversa -> canal_ids -> canais).
ALTER TABLE channels ADD COLUMN IF NOT EXISTS agent_id INTEGER REFERENCES agents(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS channels_tenant_agent_idx ON channels (tenant_id, agent_id);
