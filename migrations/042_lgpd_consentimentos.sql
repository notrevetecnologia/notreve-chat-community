CREATE TABLE IF NOT EXISTS lgpd_consents (
  id SERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  contato_id INTEGER REFERENCES contatos(id) ON DELETE SET NULL,
  conversation_id INTEGER REFERENCES conversations(id) ON DELETE SET NULL,
  scope TEXT NOT NULL DEFAULT 'contato',
  base_legal TEXT NOT NULL DEFAULT 'consentimento',
  aceito BOOLEAN NOT NULL DEFAULT TRUE,
  canal TEXT NOT NULL DEFAULT '',
  token TEXT NOT NULL DEFAULT '',
  ip TEXT NOT NULL DEFAULT '',
  data TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_lgpd_consents_tenant ON lgpd_consents (tenant_id, data);
CREATE INDEX IF NOT EXISTS idx_lgpd_consents_contato ON lgpd_consents (tenant_id, contato_id);

CREATE TABLE IF NOT EXISTS lgpd_politica (
  id SERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  version INTEGER NOT NULL DEFAULT 1,
  tipo TEXT NOT NULL DEFAULT 'privacidade',
  texto TEXT NOT NULL DEFAULT '',
  url TEXT NOT NULL DEFAULT '',
  vigencia_inicio TIMESTAMPTZ NOT NULL DEFAULT now(),
  vigencia_fim TIMESTAMPTZ,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_lgpd_politica_tenant ON lgpd_politica (tenant_id, version);

CREATE TABLE IF NOT EXISTS audit_log (
  id SERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  usuario_id INTEGER REFERENCES usuarios(id) ON DELETE SET NULL,
  acao TEXT NOT NULL DEFAULT '',
  recurso TEXT NOT NULL DEFAULT '',
  entidade_id INTEGER,
  ip TEXT NOT NULL DEFAULT '',
  data TIMESTAMPTZ NOT NULL DEFAULT now(),
  metadados JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_audit_log_tenant ON audit_log (tenant_id, data);
CREATE INDEX IF NOT EXISTS idx_audit_log_usuario ON audit_log (tenant_id, usuario_id, data);

ALTER TABLE contatos ADD COLUMN IF NOT EXISTS documento_cifrado BYTEA;
ALTER TABLE contatos ADD COLUMN IF NOT EXISTS documento_nonce BYTEA;

INSERT INTO modulos (key, name, description, sort) VALUES
  ('lgpd', 'LGPD e privacidade', 'Consentimentos, política de privacidade e auditoria de dados', 12)
ON CONFLICT (key) DO UPDATE SET
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  sort = EXCLUDED.sort;

INSERT INTO tenant_modules (tenant_id, module_key, enabled)
SELECT t.id, m.key, TRUE
FROM tenants t
CROSS JOIN modulos m
WHERE m.key = 'lgpd'
ON CONFLICT (tenant_id, module_key) DO NOTHING;

DROP POLICY IF EXISTS tenant_isolation ON lgpd_consents;
CREATE POLICY tenant_isolation ON lgpd_consents USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON lgpd_politica;
CREATE POLICY tenant_isolation ON lgpd_politica USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON audit_log;
CREATE POLICY tenant_isolation ON audit_log USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
