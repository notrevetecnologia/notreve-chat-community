-- Estado "bot bloqueado" em conversas (segurança).
--
-- Marca uma conversa como provável outro bot para que mensagens futuras sejam
-- ignoradas (sem consumir a fila humana nem gastar IA) até um atendente
-- desbloquear. Padrão: não bloqueado.
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS bot_blocked BOOLEAN NOT NULL DEFAULT FALSE;

-- Índice leve para listar/contar conversas bloqueadas.
CREATE INDEX IF NOT EXISTS idx_conversations_bot_blocked
  ON conversations (tenant_id, bot_blocked);
