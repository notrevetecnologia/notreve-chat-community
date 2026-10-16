-- Skills: pacotes de instruções especializadas carregadas sob demanda pela IA
-- (progressive disclosure) e vinculáveis a agentes (N:N).
CREATE TABLE IF NOT EXISTS skills (
  id SERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  nome TEXT NOT NULL,
  descricao TEXT NOT NULL DEFAULT '',
  instrucao_completa TEXT NOT NULL DEFAULT '',
  categoria TEXT NOT NULL DEFAULT 'geral',
  ativa BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, nome)
);

CREATE INDEX IF NOT EXISTS skills_tenant_idx ON skills (tenant_id, ativa);

CREATE TABLE IF NOT EXISTS agent_skills (
  id SERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  agent_id INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  skill_id INTEGER NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
  posicao INTEGER NOT NULL DEFAULT 0,
  ativa BOOLEAN NOT NULL DEFAULT true,
  UNIQUE (agent_id, skill_id)
);

CREATE INDEX IF NOT EXISTS agent_skills_agent_idx ON agent_skills (tenant_id, agent_id);

-- MCP: servidores externos (stdio ou HTTP/SSE) cujas tools são expostas à IA.
CREATE TABLE IF NOT EXISTS mcp_servers (
  id SERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  nome TEXT NOT NULL,
  descricao TEXT NOT NULL DEFAULT '',
  transport TEXT NOT NULL DEFAULT 'stdio',
  command TEXT NOT NULL DEFAULT '',
  args JSONB NOT NULL DEFAULT '[]'::jsonb,
  env JSONB NOT NULL DEFAULT '{}'::jsonb,
  url TEXT NOT NULL DEFAULT '',
  headers JSONB NOT NULL DEFAULT '{}'::jsonb,
  ativo BOOLEAN NOT NULL DEFAULT true,
  conectado BOOLEAN NOT NULL DEFAULT false,
  ultimo_erro TEXT NOT NULL DEFAULT '',
  server_name TEXT NOT NULL DEFAULT '',
  server_version TEXT NOT NULL DEFAULT '',
  capabilities JSONB NOT NULL DEFAULT '{}'::jsonb,
  ultima_conexao TIMESTAMPTZ,
  ultima_sincronizacao TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS mcp_servers_tenant_idx ON mcp_servers (tenant_id, ativo);

CREATE TABLE IF NOT EXISTS mcp_tools (
  id SERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  server_id INTEGER NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  input_schema JSONB NOT NULL DEFAULT '{}'::jsonb,
  ativa BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (server_id, name)
);

CREATE INDEX IF NOT EXISTS mcp_tools_server_idx ON mcp_tools (tenant_id, server_id);

CREATE TABLE IF NOT EXISTS agent_mcp_servers (
  id SERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  agent_id INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  server_id INTEGER NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
  UNIQUE (agent_id, server_id)
);

CREATE INDEX IF NOT EXISTS agent_mcp_servers_agent_idx ON agent_mcp_servers (tenant_id, agent_id);

-- RLS: as migrations 056/057 só aplicaram RLS às tabelas existentes na época.
-- As tabelas novas repetem a política tenant_row_visible para manter o isolamento.
DO $$
DECLARE
  target_table text;
BEGIN
  FOREACH target_table IN ARRAY ARRAY['skills', 'agent_skills', 'mcp_servers', 'mcp_tools', 'agent_mcp_servers']
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', target_table);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', target_table);
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', target_table);
    EXECUTE format('DROP POLICY IF EXISTS tenant_context_isolation ON %I', target_table);
    EXECUTE format(
      'CREATE POLICY tenant_context_isolation ON %I USING (tenant_row_visible(tenant_id)) WITH CHECK (tenant_row_visible(tenant_id))',
      target_table
    );
  END LOOP;
END;
$$;
