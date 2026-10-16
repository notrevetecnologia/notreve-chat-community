-- Detecção de "outro bot" no atendimento (segurança).
--
-- Quando ligada, a plataforma detecta padrões de automação (mensagens de texto
-- que casam com a lista editável bot_patterns e/ou rajada de mensagens no
-- intervalo) e **transfere a conversa para atendimento humano** com um aviso,
-- em vez de deixar a IA responder a outro bot. Não encerra (evita loop de
-- "encerrar -> bot responde de novo"). Padrão: desligado (não muda o modo livre).
ALTER TABLE settings ADD COLUMN IF NOT EXISTS detectar_bots BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS bot_patterns TEXT NOT NULL DEFAULT '';
