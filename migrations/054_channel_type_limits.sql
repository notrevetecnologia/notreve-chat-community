ALTER TABLE tenant_limits
  ADD COLUMN IF NOT EXISTS max_canais_whatsapp INTEGER NOT NULL DEFAULT 3 CHECK (max_canais_whatsapp >= 0),
  ADD COLUMN IF NOT EXISTS max_canais_telegram INTEGER NOT NULL DEFAULT 3 CHECK (max_canais_telegram >= 0),
  ADD COLUMN IF NOT EXISTS max_canais_webchat INTEGER NOT NULL DEFAULT 3 CHECK (max_canais_webchat >= 0);

UPDATE tenant_limits
SET max_canais_whatsapp = max_conexoes_whatsapp,
    max_canais_telegram = max_canais,
    max_canais_webchat = max_canais;
