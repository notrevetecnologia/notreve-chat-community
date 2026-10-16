INSERT INTO mensagens (chave, titulo, texto) VALUES
  ('satisfacao_ia', 'Avaliação da assistente virtual', 'De 1 a 5, como você avalia o atendimento da nossa assistente virtual? Responda apenas com um número (1 = péssimo, 5 = excelente). 😊'),
  ('satisfacao_operador', 'Avaliação do atendente', 'De 1 a 5, como você avalia o atendimento do nosso atendente? Responda apenas com um número (1 = péssimo, 5 = excelente). 😊')
ON CONFLICT (chave) DO NOTHING;

CREATE TABLE IF NOT EXISTS webchat_sessions (
  channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  visitor_id TEXT NOT NULL,
  token_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (channel_id, visitor_id)
);
