-- Fase 2C: troca de contexto auditada do super_admin, campos de gestão direta
-- no tenant (domínio e créditos) e leitura/edição de status/slug já existentes.

ALTER TABLE tenants ADD COLUMN IF NOT EXISTS dominio TEXT NOT NULL DEFAULT '';
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS creditos INTEGER NOT NULL DEFAULT 0 CHECK (creditos >= 0);

CREATE TABLE IF NOT EXISTS audit_context_switch (
  id BIGSERIAL PRIMARY KEY,
  actor_id INTEGER NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT,
  actor_name TEXT NOT NULL DEFAULT '',
  from_tenant_id INTEGER REFERENCES tenants(id) ON DELETE SET NULL,
  from_slug TEXT NOT NULL DEFAULT '',
  to_tenant_id INTEGER REFERENCES tenants(id) ON DELETE SET NULL,
  to_slug TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_context_switch_to_tenant
  ON audit_context_switch (to_tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_context_switch_actor
  ON audit_context_switch (actor_id, created_at DESC);
