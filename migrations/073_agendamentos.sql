-- Agendamentos: tarefas que o scheduler dispara em data/hora (once), a cada
-- intervalo (interval) ou por expressão cron. Persistidas por tenant e
-- re-agendadas no boot do app.
CREATE TABLE IF NOT EXISTS agendamentos (
  id SERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  nome TEXT NOT NULL,
  descricao TEXT NOT NULL DEFAULT '',
  tipo TEXT NOT NULL DEFAULT 'once',
  quando TEXT NOT NULL,
  acao TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  status TEXT NOT NULL DEFAULT 'pendente',
  max_execucoes INTEGER,
  execucoes INTEGER NOT NULL DEFAULT 0,
  proxima_execucao TIMESTAMPTZ,
  ultima_execucao TIMESTAMPTZ,
  resultado TEXT NOT NULL DEFAULT '',
  erro TEXT NOT NULL DEFAULT '',
  canal_id INTEGER REFERENCES channels(id) ON DELETE SET NULL,
  contato_id INTEGER REFERENCES contatos(id) ON DELETE SET NULL,
  telefone TEXT NOT NULL DEFAULT '',
  agente_id INTEGER REFERENCES agents(id) ON DELETE SET NULL,
  criado_por INTEGER REFERENCES usuarios(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS agendamentos_tenant_status_idx ON agendamentos (tenant_id, status, proxima_execucao);
CREATE INDEX IF NOT EXISTS agendamentos_tenant_created_idx ON agendamentos (tenant_id, created_at DESC);

INSERT INTO modulos (key, name, description, sort) VALUES
  ('agendamentos', 'Agendamentos', 'Tarefas agendadas: lembretes, follow-ups e automações', 12)
ON CONFLICT (key) DO NOTHING;

INSERT INTO tenant_modules (tenant_id, module_key, enabled)
SELECT t.id, 'agendamentos', TRUE
FROM tenants t
ON CONFLICT (tenant_id, module_key) DO NOTHING;

DO $$
DECLARE
  target_table text;
BEGIN
  target_table := 'agendamentos';
  EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', target_table);
  EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', target_table);
  EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', target_table);
  EXECUTE format('DROP POLICY IF EXISTS tenant_context_isolation ON %I', target_table);
  EXECUTE format(
    'CREATE POLICY tenant_context_isolation ON %I USING (tenant_row_visible(tenant_id)) WITH CHECK (tenant_row_visible(tenant_id))',
    target_table
  );
END;
$$;
