-- Permite restringir um agente a instâncias específicas sem perder a seleção
-- legada por tipo em agents.canais. Lista vazia significa todos os canais dos
-- tipos permitidos.
ALTER TABLE agents ADD COLUMN IF NOT EXISTS canal_ids TEXT NOT NULL DEFAULT '[]';
