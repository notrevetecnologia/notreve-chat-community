ALTER TABLE usuarios ADD COLUMN IF NOT EXISTS permissions TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS modulos (
  key TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sort INTEGER NOT NULL DEFAULT 0
);

INSERT INTO modulos (key, name, description, sort) VALUES
  ('conversas', 'Atendimento (Conversas)', 'Inbox com as conversas e o chat de atendimento', 1),
  ('ia', 'Agentes de IA', 'Agentes de IA, provedores e aprendizado', 2),
  ('contatos', 'Contatos', 'Base de contatos e sincronização', 3),
  ('equipe', 'Setores e Equipe', 'Setores, atendentes e permissões', 4),
  ('canais', 'Canais', 'WhatsApp, webchat e outros canais', 5),
  ('integracoes', 'Integrações', 'Ferramentas externas que a IA pode chamar', 6),
  ('mensagens', 'Mensagens', 'Respostas padrão do sistema', 7),
  ('aprendizado', 'Aprendizado', 'Perguntas que a IA não soube responder', 8),
  ('config', 'Configurações', 'IA padrão, WhatsApp e módulos', 9)
ON CONFLICT (key) DO NOTHING;
