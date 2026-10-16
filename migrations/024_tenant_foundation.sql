CREATE TABLE IF NOT EXISTS tenants (
  id SERIAL PRIMARY KEY,
  slug TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('trial', 'active', 'suspended', 'closed')),
  plan TEXT NOT NULL DEFAULT 'legacy',
  timezone TEXT NOT NULL DEFAULT 'America/Sao_Paulo',
  locale TEXT NOT NULL DEFAULT 'pt-BR',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO tenants (id, slug, name, status, plan)
VALUES (1, 'notreve', 'Notreve Agente IA', 'active', 'legacy')
ON CONFLICT (id) DO NOTHING;

SELECT setval(
  pg_get_serial_sequence('tenants', 'id'),
  GREATEST((SELECT COALESCE(MAX(id), 1) FROM tenants), 1),
  true
);

CREATE TABLE IF NOT EXISTS tenant_domains (
  id SERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  hostname TEXT NOT NULL UNIQUE,
  kind TEXT NOT NULL DEFAULT 'panel',
  is_primary BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_domains_primary_kind
  ON tenant_domains (tenant_id, kind) WHERE is_primary;

INSERT INTO tenant_domains (tenant_id, hostname, kind, is_primary)
VALUES (1, 'atendimento.notreve.com.br', 'panel', TRUE)
ON CONFLICT (hostname) DO NOTHING;

ALTER TABLE settings ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE agents ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE providers ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE channels ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE integrations ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE setores ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE atendentes ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE atendente_setores ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE usuarios ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE messages ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE avaliacoes ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE mensagens ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE respostas_rapidas ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE contatos ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE aprendizados ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE segmentos ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE contato_segmentos ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE grupo_canais ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE grupo_canal ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE campanhas ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE campanha_envios ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE limites ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);
ALTER TABLE webchat_sessions ADD COLUMN IF NOT EXISTS tenant_id INTEGER REFERENCES tenants(id);

UPDATE settings SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE agents SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE providers SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE channels SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE integrations SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE setores SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE atendentes SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE atendente_setores SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE usuarios SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE conversations SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE messages SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE avaliacoes SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE mensagens SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE respostas_rapidas SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE contatos SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE aprendizados SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE segmentos SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE contato_segmentos SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE grupo_canais SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE grupo_canal SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE campanhas SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE campanha_envios SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE limites SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE webchat_sessions SET tenant_id = 1 WHERE tenant_id IS NULL;

ALTER TABLE settings ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE agents ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE providers ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE channels ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE integrations ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE setores ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE atendentes ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE atendente_setores ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE usuarios ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE conversations ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE messages ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE avaliacoes ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE mensagens ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE respostas_rapidas ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE contatos ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE aprendizados ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE segmentos ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE contato_segmentos ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE grupo_canais ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE grupo_canal ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE campanhas ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE campanha_envios ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE limites ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE webchat_sessions ALTER COLUMN tenant_id SET DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_settings_tenant_id ON settings (tenant_id);
CREATE INDEX IF NOT EXISTS idx_agents_tenant_id ON agents (tenant_id);
CREATE INDEX IF NOT EXISTS idx_providers_tenant_id ON providers (tenant_id);
CREATE INDEX IF NOT EXISTS idx_channels_tenant_id ON channels (tenant_id);
CREATE INDEX IF NOT EXISTS idx_integrations_tenant_id ON integrations (tenant_id);
CREATE INDEX IF NOT EXISTS idx_setores_tenant_id ON setores (tenant_id);
CREATE INDEX IF NOT EXISTS idx_atendentes_tenant_id ON atendentes (tenant_id);
CREATE INDEX IF NOT EXISTS idx_atendente_setores_tenant_id ON atendente_setores (tenant_id);
CREATE INDEX IF NOT EXISTS idx_usuarios_tenant_id ON usuarios (tenant_id);
CREATE INDEX IF NOT EXISTS idx_conversations_tenant_id ON conversations (tenant_id);
CREATE INDEX IF NOT EXISTS idx_messages_tenant_id ON messages (tenant_id);
CREATE INDEX IF NOT EXISTS idx_avaliacoes_tenant_id ON avaliacoes (tenant_id);
CREATE INDEX IF NOT EXISTS idx_mensagens_tenant_id ON mensagens (tenant_id);
CREATE INDEX IF NOT EXISTS idx_respostas_rapidas_tenant_id ON respostas_rapidas (tenant_id);
CREATE INDEX IF NOT EXISTS idx_contatos_tenant_id ON contatos (tenant_id);
CREATE INDEX IF NOT EXISTS idx_aprendizados_tenant_id ON aprendizados (tenant_id);
CREATE INDEX IF NOT EXISTS idx_segmentos_tenant_id ON segmentos (tenant_id);
CREATE INDEX IF NOT EXISTS idx_contato_segmentos_tenant_id ON contato_segmentos (tenant_id);
CREATE INDEX IF NOT EXISTS idx_grupo_canais_tenant_id ON grupo_canais (tenant_id);
CREATE INDEX IF NOT EXISTS idx_grupo_canal_tenant_id ON grupo_canal (tenant_id);
CREATE INDEX IF NOT EXISTS idx_campanhas_tenant_id ON campanhas (tenant_id);
CREATE INDEX IF NOT EXISTS idx_campanha_envios_tenant_id ON campanha_envios (tenant_id);
CREATE INDEX IF NOT EXISTS idx_limites_tenant_id ON limites (tenant_id);
CREATE INDEX IF NOT EXISTS idx_webchat_sessions_tenant_id ON webchat_sessions (tenant_id);

CREATE TABLE IF NOT EXISTS tenant_limits (
  tenant_id INTEGER PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  max_canais INTEGER NOT NULL DEFAULT 3,
  max_atendentes INTEGER NOT NULL DEFAULT 10,
  max_campanhas_ativas INTEGER NOT NULL DEFAULT 2,
  campanha_limite_diario INTEGER NOT NULL DEFAULT 100,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO tenant_limits (tenant_id, max_canais, max_atendentes, max_campanhas_ativas, campanha_limite_diario)
SELECT 1, max_canais, max_atendentes, max_campanhas_ativas, campanha_limite_diario
FROM limites
WHERE id = 1
ON CONFLICT (tenant_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS tenant_modules (
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  module_key TEXT NOT NULL REFERENCES modulos(key) ON DELETE CASCADE,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, module_key)
);

INSERT INTO tenant_modules (tenant_id, module_key, enabled)
SELECT 1, key, enabled
FROM modulos
ON CONFLICT (tenant_id, module_key) DO NOTHING;
