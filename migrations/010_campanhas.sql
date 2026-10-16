ALTER TABLE channels ADD COLUMN IF NOT EXISTS can_campaign BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS segmentos (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS contato_segmentos (
  contato_id INTEGER NOT NULL REFERENCES contatos(id) ON DELETE CASCADE,
  segmento_id INTEGER NOT NULL REFERENCES segmentos(id) ON DELETE CASCADE,
  PRIMARY KEY (contato_id, segmento_id)
);

CREATE TABLE IF NOT EXISTS grupo_canais (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS grupo_canal (
  grupo_id INTEGER NOT NULL REFERENCES grupo_canais(id) ON DELETE CASCADE,
  channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  PRIMARY KEY (grupo_id, channel_id)
);

CREATE TABLE IF NOT EXISTS campanhas (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  message TEXT NOT NULL DEFAULT '',
  canal TEXT NOT NULL DEFAULT 'whatsapp',
  grupo_id INTEGER REFERENCES grupo_canais(id) ON DELETE SET NULL,
  channel_id INTEGER REFERENCES channels(id) ON DELETE SET NULL,
  segmento_id INTEGER REFERENCES segmentos(id) ON DELETE SET NULL,
  status TEXT NOT NULL DEFAULT 'rascunho',
  scheduled_at TIMESTAMPTZ,
  intervalo_segundos INTEGER NOT NULL DEFAULT 15,
  limite_diario INTEGER NOT NULL DEFAULT 50,
  variar_mensagem BOOLEAN NOT NULL DEFAULT FALSE,
  modo_teste BOOLEAN NOT NULL DEFAULT FALSE,
  total INTEGER NOT NULL DEFAULT 0,
  enviadas INTEGER NOT NULL DEFAULT 0,
  falhas INTEGER NOT NULL DEFAULT 0,
  criado_por INTEGER REFERENCES usuarios(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS campanha_envios (
  id SERIAL PRIMARY KEY,
  campanha_id INTEGER NOT NULL REFERENCES campanhas(id) ON DELETE CASCADE,
  contato_id INTEGER NOT NULL REFERENCES contatos(id) ON DELETE CASCADE,
  phone TEXT NOT NULL DEFAULT '',
  channel_id INTEGER REFERENCES channels(id) ON DELETE SET NULL,
  status TEXT NOT NULL DEFAULT 'pendente',
  erro TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  sent_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_campanha_envios_campanha ON campanha_envios(campanha_id, status);
CREATE INDEX IF NOT EXISTS idx_campanhas_status ON campanhas(status);

INSERT INTO modulos (key, name, description, sort) VALUES
  ('campanhas', 'Campanhas', 'Envio em massa com grupos de canais, agendamento e failover', 4)
ON CONFLICT (key) DO NOTHING;
