-- Marca agentes cujos canais passaram a ser geridos explicitamente por
-- channels.agent_id (aba Canais do agente). Enquanto falso, a resolução
-- mantém o legado (agents.canal_ids/canais) como fallback. Sem backfill:
-- agentes antigos só migram quando o vínculo é salvo pela nova tela.
ALTER TABLE agents ADD COLUMN IF NOT EXISTS channels_managed BOOLEAN NOT NULL DEFAULT false;
