ALTER TABLE contatos DROP CONSTRAINT IF EXISTS contatos_phone_key;
CREATE UNIQUE INDEX IF NOT EXISTS idx_contatos_tenant_phone ON contatos (tenant_id, phone);
