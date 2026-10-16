CREATE OR REPLACE FUNCTION current_app_tenant_id() RETURNS integer
LANGUAGE sql STABLE
AS $$
  SELECT NULLIF(current_setting('app.tenant_id', true), '')::integer
$$;

-- NOTA: esta migration apenas REGISTRA o schema/hardening de isolamento por
-- tenant (definição de policies). A ativação do Row Level Security e a
-- imposição de NOT NULL foram adiadas para uma migration futura, que só deve
-- rodar depois que TODAS as queries do store setarem app.tenant_id via
-- WithTenantConn/WithTenantTx. Habilitar RLS agora quebraria as ~266 queries
-- diretas que ainda usam apenas o filtro tenant_id=$1.
--
-- Enable (futuro):
--   ALTER TABLE <tabela> ENABLE ROW LEVEL SECURITY;
--   ALTER TABLE <tabela> ALTER COLUMN tenant_id SET NOT NULL;

DROP POLICY IF EXISTS tenant_isolation ON settings;
CREATE POLICY tenant_isolation ON settings USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON providers;
CREATE POLICY tenant_isolation ON providers USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON agents;
CREATE POLICY tenant_isolation ON agents USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON channels;
CREATE POLICY tenant_isolation ON channels USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON integrations;
CREATE POLICY tenant_isolation ON integrations USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON setores;
CREATE POLICY tenant_isolation ON setores USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON atendentes;
CREATE POLICY tenant_isolation ON atendentes USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON atendente_setores;
CREATE POLICY tenant_isolation ON atendente_setores USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON conversations;
CREATE POLICY tenant_isolation ON conversations USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON messages;
CREATE POLICY tenant_isolation ON messages USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON avaliacoes;
CREATE POLICY tenant_isolation ON avaliacoes USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON mensagens;
CREATE POLICY tenant_isolation ON mensagens USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON respostas_rapidas;
CREATE POLICY tenant_isolation ON respostas_rapidas USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON contatos;
CREATE POLICY tenant_isolation ON contatos USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON aprendizados;
CREATE POLICY tenant_isolation ON aprendizados USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON segmentos;
CREATE POLICY tenant_isolation ON segmentos USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON contato_segmentos;
CREATE POLICY tenant_isolation ON contato_segmentos USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON grupo_canais;
CREATE POLICY tenant_isolation ON grupo_canais USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON grupo_canal;
CREATE POLICY tenant_isolation ON grupo_canal USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON campanhas;
CREATE POLICY tenant_isolation ON campanhas USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON campanha_envios;
CREATE POLICY tenant_isolation ON campanha_envios USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON webchat_sessions;
CREATE POLICY tenant_isolation ON webchat_sessions USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
DROP POLICY IF EXISTS tenant_isolation ON knowledge_sources;
CREATE POLICY tenant_isolation ON knowledge_sources USING (tenant_id = current_app_tenant_id()) WITH CHECK (tenant_id = current_app_tenant_id());
