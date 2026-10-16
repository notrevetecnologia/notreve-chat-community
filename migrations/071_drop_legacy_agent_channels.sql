-- channels.agent_id passa a ser a única fonte de verdade do vínculo
-- canal->agente. Antes de remover o modelo legado, faz backfill dos canais que
-- o agente já listava em canal_ids e que ainda não tinham agente definido,
-- preservando vínculos existentes.
UPDATE channels c
SET agent_id = a.id
FROM agents a,
     LATERAL jsonb_array_elements_text(
       CASE WHEN a.canal_ids ~ '^[[:space:]]*\['
            THEN a.canal_ids::jsonb
            ELSE '[]'::jsonb
       END
     ) AS elem(value)
WHERE c.agent_id IS NULL
  AND c.tenant_id = a.tenant_id
  AND c.id = elem.value::int;

ALTER TABLE agents DROP COLUMN IF EXISTS canal_ids;
ALTER TABLE agents DROP COLUMN IF EXISTS canais;
ALTER TABLE agents DROP COLUMN IF EXISTS channels_managed;
