-- Control plane da plataforma. Estas estruturas são globais e devem ser
-- operadas somente pelo super_admin; os tenants continuam isolados entre si.

-- settings nasceu como uma tabela de instância única. Cada nova empresa
-- precisa da própria linha de configuração, sem reutilizar o id legado 1.
CREATE SEQUENCE IF NOT EXISTS settings_id_seq;
SELECT setval(
  'settings_id_seq',
  GREATEST((SELECT COALESCE(MAX(id), 1) FROM settings), 1),
  true
);
ALTER TABLE settings ALTER COLUMN id SET DEFAULT nextval('settings_id_seq');
CREATE UNIQUE INDEX IF NOT EXISTS idx_settings_tenant_unique
  ON settings (tenant_id) WHERE tenant_id IS NOT NULL;

-- Limites complementares ao contrato original. Os quatro primeiros campos
-- continuam compatíveis com as telas e validações já existentes.
ALTER TABLE tenant_limits ADD COLUMN IF NOT EXISTS max_agentes INTEGER NOT NULL DEFAULT 3;
ALTER TABLE tenant_limits ADD COLUMN IF NOT EXISTS max_usuarios INTEGER NOT NULL DEFAULT 10;
ALTER TABLE tenant_limits ADD COLUMN IF NOT EXISTS max_conexoes_whatsapp INTEGER NOT NULL DEFAULT 3;
ALTER TABLE tenant_limits ADD COLUMN IF NOT EXISTS max_tools INTEGER NOT NULL DEFAULT 10;
ALTER TABLE tenant_limits ADD COLUMN IF NOT EXISTS max_api_keys INTEGER NOT NULL DEFAULT 3;
ALTER TABLE tenant_limits ADD COLUMN IF NOT EXISTS max_webhooks INTEGER NOT NULL DEFAULT 10;
ALTER TABLE tenant_limits ADD COLUMN IF NOT EXISTS creditos_ia_mensais BIGINT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS tenant_usage_counters (
  tenant_id INTEGER PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  period_start DATE NOT NULL DEFAULT date_trunc('month', now())::date,
  api_requests BIGINT NOT NULL DEFAULT 0 CHECK (api_requests >= 0),
  ai_credits_used BIGINT NOT NULL DEFAULT 0 CHECK (ai_credits_used >= 0),
  storage_bytes BIGINT NOT NULL DEFAULT 0 CHECK (storage_bytes >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO tenant_usage_counters (tenant_id)
SELECT id FROM tenants
ON CONFLICT (tenant_id) DO NOTHING;

-- Catálogo explícito dos módulos contratáveis. Os módulos nativos são
-- habilitados automaticamente quando uma empresa é provisionada pelo store.
INSERT INTO modulos (key, name, description, sort) VALUES
  ('campanhas', 'Campanhas', 'Campanhas, listas, grupos de canais e disparos', 12),
  ('api', 'API e webhooks', 'API pública para abrir conversas, enviar mensagens e receber eventos', 10),
  ('tools', 'Tools para agentes', 'Ações externas que os agentes podem consultar ou executar com segurança', 11)
ON CONFLICT (key) DO UPDATE SET
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  sort = EXCLUDED.sort;

-- Servidores são recursos da plataforma: o token fica somente no banco e
-- nenhuma query de listagem o seleciona. O admin da empresa receberá apenas
-- a conexão atribuída, sem conhecer credenciais ou URL de infraestrutura.
CREATE TABLE IF NOT EXISTS whatsapp_servers (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE CHECK (btrim(name) <> ''),
  base_url TEXT NOT NULL CHECK (btrim(base_url) <> ''),
  token TEXT NOT NULL DEFAULT '',
  active BOOLEAN NOT NULL DEFAULT TRUE,
  max_connections INTEGER NOT NULL DEFAULT 100 CHECK (max_connections > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_whatsapp_servers_active_capacity
  ON whatsapp_servers (active, id) WHERE active;

-- Uma conexão WhatsApp pertence a uma única empresa e a um único servidor do
-- pool. A trigger impede inconsistências mesmo se alguém tentar escrever no
-- banco sem passar pela camada Store.
CREATE TABLE IF NOT EXISTS channel_whatsapp_server_allocations (
  channel_id INTEGER PRIMARY KEY REFERENCES channels(id) ON DELETE CASCADE,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  server_id BIGINT NOT NULL REFERENCES whatsapp_servers(id) ON DELETE RESTRICT,
  allocated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_channel_whatsapp_server_allocations_server
  ON channel_whatsapp_server_allocations (server_id, tenant_id);

CREATE OR REPLACE FUNCTION validate_channel_whatsapp_server_allocation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  channel_tenant_id INTEGER;
  channel_type TEXT;
BEGIN
  SELECT tenant_id, type INTO channel_tenant_id, channel_type
  FROM channels
  WHERE id = NEW.channel_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'canal % não encontrado para alocação WhatsApp', NEW.channel_id;
  END IF;
  IF channel_tenant_id <> NEW.tenant_id THEN
    RAISE EXCEPTION 'tenant da alocação não corresponde ao tenant do canal';
  END IF;
  IF channel_type <> 'whatsapp' THEN
    RAISE EXCEPTION 'somente canais WhatsApp podem receber servidor do pool';
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_validate_channel_whatsapp_server_allocation
  ON channel_whatsapp_server_allocations;
CREATE TRIGGER trg_validate_channel_whatsapp_server_allocation
BEFORE INSERT OR UPDATE OF channel_id, tenant_id, server_id
ON channel_whatsapp_server_allocations
FOR EACH ROW EXECUTE FUNCTION validate_channel_whatsapp_server_allocation();
