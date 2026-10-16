CREATE TABLE IF NOT EXISTS agent_failures (
  id BIGSERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  conversation_id INTEGER,
  channel TEXT NOT NULL DEFAULT '',
  phone TEXT NOT NULL DEFAULT '',
  origem TEXT NOT NULL DEFAULT 'pipeline',
  tipo TEXT NOT NULL DEFAULT 'outros',
  exception_class TEXT NOT NULL DEFAULT '',
  exception_code TEXT NOT NULL DEFAULT '',
  mensagem_erro TEXT NOT NULL DEFAULT '',
  traceback TEXT,
  payload JSONB,
  tentativas INTEGER NOT NULL DEFAULT 1 CHECK (tentativas >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolvido_em TIMESTAMPTZ,
  resolvido_por TEXT,
  resolucao_nota TEXT
);

CREATE INDEX IF NOT EXISTS idx_agent_failures_tenant_created
  ON agent_failures (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_failures_tenant_tipo
  ON agent_failures (tenant_id, tipo, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_failures_tenant_conversation
  ON agent_failures (tenant_id, conversation_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_failures_tenant_abertos
  ON agent_failures (tenant_id, created_at DESC)
  WHERE resolvido_em IS NULL;

ALTER TABLE agent_failures ENABLE ROW LEVEL SECURITY;
ALTER TABLE agent_failures FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_context_isolation ON agent_failures;
CREATE POLICY tenant_context_isolation ON agent_failures
  USING (tenant_row_visible(tenant_id)) WITH CHECK (tenant_row_visible(tenant_id));
