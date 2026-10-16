ALTER TABLE channels
    ADD COLUMN IF NOT EXISTS instance_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_channels_whatsapp_instance_id
    ON channels (instance_id)
    WHERE type = 'whatsapp' AND instance_id <> '';
