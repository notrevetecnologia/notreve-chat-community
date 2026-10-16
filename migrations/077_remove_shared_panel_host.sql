-- Remove o resíduo do modelo de "painel compartilhado" (domínios fixos da marca).
-- O painel passa a rodar no domínio de cada instalação/cliente; o Caddy cuida do TLS.

ALTER TABLE tenants DROP CONSTRAINT IF EXISTS tenants_dominio_not_shared_panel;

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

  IF normalized <> '' AND normalized !~ '^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$' THEN
    RAISE EXCEPTION 'domínio inválido: %', normalized;
  END IF;

  NEW.dominio := normalized;
  RETURN NEW;
END;
$$;

UPDATE tenants SET slug = 'default', name = 'Minha empresa'
 WHERE id = 1 AND slug = 'notreve';

DELETE FROM tenant_domains WHERE lower(hostname) = 'atendimento.notreve.com.br';

INSERT INTO tenant_domains (tenant_id, hostname, kind, is_primary)
VALUES (1, 'localhost', 'panel', FALSE)
ON CONFLICT (hostname) DO NOTHING;

INSERT INTO tenant_domains (tenant_id, hostname, kind, is_primary)
VALUES (1, '127.0.0.1', 'panel', FALSE)
ON CONFLICT (hostname) DO NOTHING;
