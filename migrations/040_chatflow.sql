CREATE TABLE IF NOT EXISTS chatflow (
  id SERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  nome TEXT NOT NULL,
  version INTEGER NOT NULL DEFAULT 1,
  ativo BOOLEAN NOT NULL DEFAULT FALSE,
  config JSONB NOT NULL DEFAULT '{}'::jsonb,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, nome),
  UNIQUE (tenant_id, version)
);

CREATE TABLE IF NOT EXISTS chatflow_steps (
  id SERIAL PRIMARY KEY,
  chatflow_id INTEGER NOT NULL REFERENCES chatflow(id) ON DELETE CASCADE,
  ord INTEGER NOT NULL DEFAULT 0,
  tipo TEXT NOT NULL,
  config JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chatflow_steps_flow ON chatflow_steps (chatflow_id, ord);

ALTER TABLE agents ADD COLUMN IF NOT EXISTS chatflow_id INTEGER REFERENCES chatflow(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_agents_chatflow ON agents (tenant_id, chatflow_id);

ALTER TABLE conversations ADD COLUMN IF NOT EXISTS chatflow_id INTEGER REFERENCES chatflow(id) ON DELETE SET NULL;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS chatflow_step INTEGER;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS chatflow_vars JSONB;
CREATE INDEX IF NOT EXISTS idx_conversations_chatflow ON conversations (tenant_id, chatflow_id);

INSERT INTO modulos (key, name, description, sort) VALUES
  ('chatflows', 'Chat Flow', 'Fluxos de atendimento híbridos com IA', 10)
ON CONFLICT (key) DO NOTHING;
