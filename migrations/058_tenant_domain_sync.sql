-- Mantém o domínio personalizado exibido no control plane sincronizado com o
-- roteador real de tenants. app.notreve.com.br é reservado ao painel comum.
CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_domains_hostname_lower
  ON tenant_domains (lower(hostname));

ALTER TABLE tenants
  DROP CONSTRAINT IF EXISTS tenants_dominio_not_shared_panel;
ALTER TABLE tenants
  ADD CONSTRAINT tenants_dominio_not_shared_panel
  CHECK (lower(btrim(dominio)) <> 'app.notreve.com.br');

CREATE OR REPLACE FUNCTION sync_tenant_custom_panel_domain()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  normalized text;
BEGIN
  normalized := lower(btrim(COALESCE(NEW.dominio, '')));
  normalized := regexp_replace(normalized, '^https?://', '');
  normalized := regexp_replace(normalized, '/+$', '');
  normalized := regexp_replace(normalized, '\.$', '');

  IF normalized = 'app.notreve.com.br' THEN
    RAISE EXCEPTION 'app.notreve.com.br é reservado ao painel compartilhado';
  END IF;
  IF normalized <> '' AND normalized !~ '^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$' THEN
    RAISE EXCEPTION 'domínio inválido: %', normalized;
  END IF;

  NEW.dominio := normalized;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_normalize_tenant_custom_domain ON tenants;
CREATE TRIGGER trg_normalize_tenant_custom_domain
BEFORE INSERT OR UPDATE OF dominio ON tenants
FOR EACH ROW
EXECUTE FUNCTION sync_tenant_custom_panel_domain();

CREATE OR REPLACE FUNCTION publish_tenant_custom_panel_domain()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  DELETE FROM tenant_domains
   WHERE tenant_id = NEW.id
     AND kind = 'custom_panel';

  IF btrim(NEW.dominio) <> '' THEN
    INSERT INTO tenant_domains (tenant_id, hostname, kind, is_primary)
    VALUES (NEW.id, lower(btrim(NEW.dominio)), 'custom_panel', TRUE);
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_publish_tenant_custom_domain ON tenants;
CREATE TRIGGER trg_publish_tenant_custom_domain
AFTER INSERT OR UPDATE OF dominio ON tenants
FOR EACH ROW
EXECUTE FUNCTION publish_tenant_custom_panel_domain();

-- Publica também os domínios já armazenados antes desta migration.
DO $$
DECLARE
  row_tenant record;
BEGIN
  FOR row_tenant IN
    SELECT id, lower(btrim(dominio)) AS dominio
      FROM tenants
     WHERE btrim(COALESCE(dominio, '')) <> ''
  LOOP
    IF row_tenant.dominio = 'app.notreve.com.br' THEN
      RAISE EXCEPTION 'tenant % usa domínio reservado app.notreve.com.br', row_tenant.id;
    END IF;
    DELETE FROM tenant_domains
     WHERE tenant_id = row_tenant.id
       AND kind = 'custom_panel';
    INSERT INTO tenant_domains (tenant_id, hostname, kind, is_primary)
    VALUES (row_tenant.id, row_tenant.dominio, 'custom_panel', TRUE);
  END LOOP;
END;
$$;
