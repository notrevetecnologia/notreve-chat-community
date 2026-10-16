-- Distribuicao por habilidade/nicho: especialidades por atendente.
ALTER TABLE atendentes ADD COLUMN IF NOT EXISTS habilidades TEXT[] NOT NULL DEFAULT '{}';

-- Filtros opcionais de segmento (tem_whatsapp e canal de conversa).
ALTER TABLE segmentos ADD COLUMN IF NOT EXISTS filter_tem_whatsapp BOOLEAN;
ALTER TABLE segmentos ADD COLUMN IF NOT EXISTS filter_canal TEXT NOT NULL DEFAULT '';
