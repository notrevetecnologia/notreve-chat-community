ALTER TABLE providers ADD COLUMN IF NOT EXISTS capabilities TEXT NOT NULL DEFAULT '["text"]';

UPDATE providers SET capabilities = '["text","vision"]'
WHERE capacidade = 'multimodal'
  AND (capabilities IS NULL OR capabilities = '' OR capabilities = '["text"]');
