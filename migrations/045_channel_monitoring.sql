ALTER TABLE channels ADD COLUMN IF NOT EXISTS health_status TEXT NOT NULL DEFAULT 'unknown';

ALTER TABLE channels DROP CONSTRAINT IF EXISTS channels_health_status_check;
ALTER TABLE channels ADD CONSTRAINT channels_health_status_check CHECK (health_status IN ('connected', 'disconnected', 'unknown'));

ALTER TABLE channels ADD COLUMN IF NOT EXISTS last_checked_at TIMESTAMPTZ;

ALTER TABLE channels ADD COLUMN IF NOT EXISTS last_error TEXT;

CREATE INDEX IF NOT EXISTS idx_channels_tenant_health ON channels (tenant_id, health_status);
CREATE INDEX IF NOT EXISTS idx_channels_monitorable ON channels (tenant_id, active, type) WHERE active=true AND type IN ('whatsapp', 'telegram');
