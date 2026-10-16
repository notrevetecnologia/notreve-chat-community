-- Migration 002: todas as features posteriores à base (idempotente, sem segredos)

-- ---------- Providers (IAs) ----------
CREATE TABLE IF NOT EXISTS providers (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  provider_type TEXT NOT NULL DEFAULT 'openai_compat',
  base_url TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL DEFAULT '',
  api_key TEXT NOT NULL DEFAULT '',
  active BOOLEAN NOT NULL DEFAULT TRUE,
  safe_mode BOOLEAN NOT NULL DEFAULT TRUE,
  capacidade TEXT NOT NULL DEFAULT 'texto',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------- Canais ----------
CREATE TABLE IF NOT EXISTS channels (
  id SERIAL PRIMARY KEY,
  type TEXT NOT NULL DEFAULT 'whatsapp',
  name TEXT NOT NULL,
  base_url TEXT NOT NULL DEFAULT '',
  token TEXT NOT NULL DEFAULT '',
  active BOOLEAN NOT NULL DEFAULT TRUE,
  respond_grupos BOOLEAN NOT NULL DEFAULT FALSE,
  responder_chamadas BOOLEAN NOT NULL DEFAULT TRUE,
  widget_config TEXT NOT NULL DEFAULT '{}',
  public_id TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------- Integrações ----------
CREATE TABLE IF NOT EXISTS integrations (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  method TEXT NOT NULL DEFAULT 'POST',
  url TEXT NOT NULL DEFAULT '',
  headers TEXT NOT NULL DEFAULT '',
  parameters TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------- Setores ----------
CREATE TABLE IF NOT EXISTS setores (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  horario_abertura TEXT NOT NULL DEFAULT '09:00',
  horario_fechamento TEXT NOT NULL DEFAULT '18:00',
  dias_semana TEXT NOT NULL DEFAULT '1,2,3,4,5',
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO setores (name, description) VALUES
  ('Suporte', 'Dúvidas técnicas, uso dos produtos, problemas'),
  ('Financeiro', 'Pagamentos, boletos, notas fiscais, cobranças'),
  ('Comercial', 'Planos, preços, vendas e propostas')
ON CONFLICT (name) DO NOTHING;

-- ---------- Atendentes + vínculo setores ----------
CREATE TABLE IF NOT EXISTS atendentes (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  online BOOLEAN NOT NULL DEFAULT FALSE,
  horario_abertura TEXT NOT NULL DEFAULT '00:00',
  horario_fechamento TEXT NOT NULL DEFAULT '23:59',
  dias_semana TEXT NOT NULL DEFAULT '0,1,2,3,4,5,6',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS atendente_setores (
  atendente_id INTEGER NOT NULL REFERENCES atendentes(id) ON DELETE CASCADE,
  setor_id INTEGER NOT NULL REFERENCES setores(id) ON DELETE CASCADE,
  PRIMARY KEY (atendente_id, setor_id)
);

-- ---------- Usuários (login) ----------
CREATE TABLE IF NOT EXISTS usuarios (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'atendente',
  atendente_id INTEGER REFERENCES atendentes(id) ON DELETE SET NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------- Agentes ----------
ALTER TABLE agents ADD COLUMN IF NOT EXISTS knowledge_links TEXT NOT NULL DEFAULT '[]';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS knowledge_content TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS provider_id INTEGER REFERENCES providers(id) ON DELETE SET NULL;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS temperature_level TEXT NOT NULL DEFAULT 'equilibrado';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS tool_ids TEXT NOT NULL DEFAULT '[]';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS setor_ids TEXT NOT NULL DEFAULT '[]';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS canais TEXT NOT NULL DEFAULT '[]';

-- ---------- Conversas ----------
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS setor_id INTEGER REFERENCES setores(id) ON DELETE SET NULL;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS avatar_url TEXT NOT NULL DEFAULT '';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS avatar_tentado BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS typing BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS atendente_id INTEGER REFERENCES usuarios(id) ON DELETE SET NULL;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS motivo_encerramento TEXT NOT NULL DEFAULT '';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS encerrado_por INTEGER REFERENCES usuarios(id) ON DELETE SET NULL;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS satisfacao TEXT NOT NULL DEFAULT '';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS origem TEXT NOT NULL DEFAULT '';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS hash TEXT NOT NULL DEFAULT '';
UPDATE conversations SET hash = substr(md5(random()::text || clock_timestamp()::text || id::text), 1, 16) WHERE hash = '';

-- ---------- Avaliações ----------
CREATE TABLE IF NOT EXISTS avaliacoes (
  id SERIAL PRIMARY KEY,
  conversation_id INTEGER NOT NULL UNIQUE REFERENCES conversations(id) ON DELETE CASCADE,
  nota_ia INTEGER,
  nota_operador INTEGER,
  comentario TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------- Mensagens (templates) ----------
CREATE TABLE IF NOT EXISTS mensagens (
  id SERIAL PRIMARY KEY,
  chave TEXT NOT NULL UNIQUE,
  titulo TEXT NOT NULL DEFAULT '',
  texto TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO mensagens (chave, titulo, texto) VALUES
  ('transferencia', 'Transferência para setor', 'Claro! Vou te encaminhar pro nosso time de {setor}, eles já te atendem em instantes 😊'),
  ('fora_horario', 'Fora do horário de atendimento', 'Nossa equipe de {setor} atende {horario}. Estamos fora do horário agora, mas vou registrar seu atendimento e eles te respondem no próximo horário 😊'),
  ('humano', 'Transferência para atendente humano', 'Ah, essa é melhor com um atendente da nossa equipe. Já vou te transferir, ok? 😊'),
  ('fora_escopo', 'Pedido fora do escopo', 'Desculpe, mas isso está fora do que eu posso ajudar. Posso te encaminhar pra alguém do nosso time? 😊'),
  ('chamada', 'Chamada de voz/vídeo recebida', 'Olá! Aqui atendemos apenas por mensagem no WhatsApp. Como posso te ajudar por aqui? 😊'),
  ('satisfacao', 'Pesquisa de satisfação', 'Antes de encerrar, de 1 a 5, como você avalia o atendimento de hoje? Responda só com o número (1 = péssimo, 5 = excelente). 😊'),
  ('paciencia', 'Cliente aguardando humano (IA monitora)', 'Olá! Recebemos sua mensagem e estamos com bastante demanda agora. Já vou priorizar seu atendimento — em instantes um atendente humano vai te responder. 😊')
ON CONFLICT (chave) DO NOTHING;

-- ---------- Respostas rápidas ----------
CREATE TABLE IF NOT EXISTS respostas_rapidas (
  id SERIAL PRIMARY KEY,
  atalho TEXT NOT NULL UNIQUE,
  texto TEXT NOT NULL,
  user_id INTEGER REFERENCES usuarios(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------- Contatos ----------
CREATE TABLE IF NOT EXISTS contatos (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL DEFAULT '',
  phone TEXT NOT NULL UNIQUE,
  email TEXT NOT NULL DEFAULT '',
  notes TEXT NOT NULL DEFAULT '',
  tem_whatsapp BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------- Settings ----------
ALTER TABLE settings ADD COLUMN IF NOT EXISTS whatsapp_base_url TEXT NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN IF NOT EXISTS whatsapp_token TEXT NOT NULL DEFAULT '';
ALTER TABLE settings ADD COLUMN IF NOT EXISTS modo_aberto BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS ia_monitora BOOLEAN NOT NULL DEFAULT TRUE;

-- ---------- Renomear status para os novos nomes ----------
UPDATE conversations SET status='ia' WHERE status='ai';
UPDATE conversations SET status='humano_aguardando' WHERE status='human';
UPDATE conversations SET status='humano_atendendo' WHERE status='em_atendimento';
