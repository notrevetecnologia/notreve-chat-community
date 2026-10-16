CREATE TABLE IF NOT EXISTS planos (
  id SERIAL PRIMARY KEY,
  tenant_owner INTEGER REFERENCES tenants(id) ON DELETE SET NULL,
  nome TEXT NOT NULL UNIQUE CHECK (btrim(nome) <> ''),
  preco NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (preco >= 0),
  intervalo TEXT NOT NULL DEFAULT 'mensal' CHECK (intervalo IN ('semanal', 'mensal', 'anual')),
  ativo BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS plano_recursos (
  id SERIAL PRIMARY KEY,
  plano_id INTEGER NOT NULL REFERENCES planos(id) ON DELETE CASCADE,
  recurso TEXT NOT NULL CHECK (btrim(recurso) <> ''),
  limite NUMERIC NOT NULL DEFAULT 0 CHECK (limite >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (plano_id, recurso)
);

CREATE INDEX IF NOT EXISTS idx_plano_recursos_plano ON plano_recursos (plano_id);

CREATE INDEX IF NOT EXISTS idx_planos_tenant_owner ON planos (tenant_owner);

ALTER TABLE tenants ADD COLUMN IF NOT EXISTS plano_id INTEGER REFERENCES planos(id);
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS trial_until TIMESTAMPTZ;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS limits_override JSONB;

CREATE INDEX IF NOT EXISTS idx_tenants_plano_id ON tenants (plano_id);

INSERT INTO planos (tenant_owner, nome, preco, intervalo, ativo)
VALUES (NULL, 'Padrão', 0, 'mensal', TRUE)
ON CONFLICT (nome) DO NOTHING;
