ALTER TABLE conversations ADD COLUMN IF NOT EXISTS tokens_prompt BIGINT NOT NULL DEFAULT 0;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS tokens_completion BIGINT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS conversation_llm_usage (
  id BIGSERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL,
  conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  agent_id INTEGER,
  model TEXT NOT NULL DEFAULT '',
  prompt_tokens INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_conv_usage_conv ON conversation_llm_usage(tenant_id, conversation_id);
CREATE INDEX IF NOT EXISTS idx_conv_usage_created ON conversation_llm_usage(tenant_id, created_at);
