ALTER TABLE integrations ADD COLUMN IF NOT EXISTS identity_config TEXT NOT NULL DEFAULT '';
ALTER TABLE integrations ADD COLUMN IF NOT EXISTS identity_verify_url TEXT NOT NULL DEFAULT '';
ALTER TABLE integrations ADD COLUMN IF NOT EXISTS identity_verify_method TEXT NOT NULL DEFAULT 'POST';
ALTER TABLE integrations ADD COLUMN IF NOT EXISTS identity_verify_param TEXT NOT NULL DEFAULT '';
ALTER TABLE integrations ADD COLUMN IF NOT EXISTS identity_match_field TEXT NOT NULL DEFAULT '';
ALTER TABLE integrations ADD COLUMN IF NOT EXISTS identity_client_id_field TEXT NOT NULL DEFAULT '';
