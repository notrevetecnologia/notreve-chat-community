CREATE TABLE IF NOT EXISTS log_entries (
  id SERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  ts TIMESTAMPTZ NOT NULL DEFAULT now(),
  level TEXT NOT NULL DEFAULT 'INFO',
  module TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  extra JSONB NOT NULL DEFAULT '{}'::jsonb,
  traceback TEXT NOT NULL DEFAULT '',
  request_id TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS log_entries_tenant_ts_idx ON log_entries (tenant_id, ts DESC);
CREATE INDEX IF NOT EXISTS log_entries_tenant_level_idx ON log_entries (tenant_id, level);
CREATE INDEX IF NOT EXISTS log_entries_tenant_module_idx ON log_entries (tenant_id, module);

ALTER TABLE settings ADD COLUMN IF NOT EXISTS image_model TEXT NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN IF NOT EXISTS image_aspect_ratio TEXT NOT NULL DEFAULT '1:1';
ALTER TABLE settings ADD COLUMN IF NOT EXISTS logs_retencao_dias INTEGER NOT NULL DEFAULT 30;

DO $$
DECLARE
  target_table text;
BEGIN
  target_table := 'log_entries';
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', target_table);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', target_table);
  EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', target_table);
  EXECUTE format('DROP POLICY IF EXISTS tenant_context_isolation ON %I', target_table);
  EXECUTE format(
    'CREATE POLICY tenant_context_isolation ON %I USING (tenant_row_visible(tenant_id)) WITH CHECK (tenant_row_visible(tenant_id))',
    target_table
  );
END;
$$;
