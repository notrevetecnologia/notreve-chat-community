ALTER TABLE usuarios
    ADD COLUMN IF NOT EXISTS totp_secret TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS totp_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS totp_confirmed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_usuarios_super_admin_mfa
    ON usuarios (id)
    WHERE role = 'super_admin' AND active = true;
