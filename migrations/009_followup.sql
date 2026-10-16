ALTER TABLE settings ADD COLUMN IF NOT EXISTS followup_ativo BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS followup_horas INTEGER NOT NULL DEFAULT 24;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS followup_enviado TIMESTAMPTZ;

INSERT INTO mensagens (chave, titulo, texto) VALUES
  ('followup', 'Follow-up automático (cliente parou de responder)', 'Olá {nome}! 😊 Passando pra ver se você ainda precisa de ajuda com o que conversamos. Se precisar, é só me chamar por aqui!')
ON CONFLICT (chave) DO NOTHING;
