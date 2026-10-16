-- Um servidor em manutenção não recebe novas conexões, mas pode manter as
-- conexões já atribuídas. A coluna active permaneceu por compatibilidade com
-- o primeiro rollout do control plane; status é a fonte de verdade da UI.
ALTER TABLE whatsapp_servers
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';

UPDATE whatsapp_servers
SET status = CASE WHEN active THEN 'active' ELSE 'disabled' END
WHERE status NOT IN ('active', 'maintenance', 'disabled')
   OR status IS NULL;

ALTER TABLE whatsapp_servers
  DROP CONSTRAINT IF EXISTS whatsapp_servers_status_check;
ALTER TABLE whatsapp_servers
  ADD CONSTRAINT whatsapp_servers_status_check
  CHECK (status IN ('active', 'maintenance', 'disabled'));

-- Capacidade zero representa um servidor sem teto configurado. A seleção do
-- pool prioriza servidores limitados ainda saudáveis e usa esse servidor como
-- fallback, evitando concentração acidental quando há capacidade conhecida.
ALTER TABLE whatsapp_servers
  DROP CONSTRAINT IF EXISTS whatsapp_servers_max_connections_check;
ALTER TABLE whatsapp_servers
  ADD CONSTRAINT whatsapp_servers_max_connections_check
  CHECK (max_connections >= 0);

UPDATE whatsapp_servers
SET active = (status = 'active')
WHERE active IS DISTINCT FROM (status = 'active');

DROP INDEX IF EXISTS idx_whatsapp_servers_active_capacity;
CREATE INDEX IF NOT EXISTS idx_whatsapp_servers_status_capacity
  ON whatsapp_servers (status, id) WHERE status = 'active';
