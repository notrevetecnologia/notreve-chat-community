CREATE TABLE IF NOT EXISTS platform_audit_log (
  id BIGSERIAL PRIMARY KEY,
  actor_id INTEGER REFERENCES usuarios(id) ON DELETE SET NULL,
  actor_name TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL,
  path TEXT NOT NULL,
  target_tenant_id INTEGER REFERENCES tenants(id) ON DELETE SET NULL,
  http_status INTEGER NOT NULL DEFAULT 0,
  ip TEXT NOT NULL DEFAULT '',
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_platform_audit_created_at
  ON platform_audit_log (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_platform_audit_actor
  ON platform_audit_log (actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_platform_audit_tenant
  ON platform_audit_log (target_tenant_id, created_at DESC);
