-- 030 é a próxima sequência disponível. A migration 025 já é usada por
-- 025_channel_routing_foundation.sql e migrations são aplicadas por nome,
-- em ordem lexicográfica; nunca reutilizar uma sequência já publicada.
--
-- Este é somente o catálogo e o backfill dos tenants existentes. A decisão
-- de quais módulos um novo tenant receberá pertence ao provisionamento e ao
-- control plane do super_admin, não a um padrão implícito do banco.
INSERT INTO modulos (key, name, description, sort) VALUES
  ('api', 'API e webhooks', 'API pública para abrir conversas, enviar mensagens e receber eventos', 10),
  ('tools', 'Tools para agentes', 'Ações externas que os agentes podem consultar ou executar com segurança', 11)
ON CONFLICT (key) DO UPDATE SET
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  sort = EXCLUDED.sort;

-- Mantém o tenant já existente funcional após a evolução do catálogo. Não
-- reativa uma escolha já feita pelo super_admin para um tenant específico.
INSERT INTO tenant_modules (tenant_id, module_key, enabled)
SELECT t.id, m.key, TRUE
FROM tenants t
CROSS JOIN modulos m
WHERE m.key IN ('api', 'tools')
ON CONFLICT (tenant_id, module_key) DO NOTHING;
