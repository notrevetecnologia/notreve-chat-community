ALTER TABLE settings ADD COLUMN IF NOT EXISTS watchdog_ativo BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS watchdog_minutos INTEGER NOT NULL DEFAULT 5;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS media_ttl_dias INTEGER NOT NULL DEFAULT 90;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS media_purga_ativo BOOLEAN NOT NULL DEFAULT TRUE;

INSERT INTO mensagens (chave, titulo, texto) VALUES
  ('watchdog_fallback', 'Fallback do watchdog (mensagem órfã sem resposta)', 'Desculpe, tive um problema técnico ao processar sua mensagem. Já registrei o ocorrido. Pode repetir sua última mensagem, por favor? 😊')
ON CONFLICT (chave) DO NOTHING;
