-- A busca de ticket ativo compara a identidade normalizada do cliente. Este
-- índice acelera números salvos como telefone, JID ou LID sem juntar visitantes
-- webchat diferentes (que não possuem dígitos em sua identidade).
CREATE INDEX IF NOT EXISTS idx_conversations_reuse_identity
  ON conversations (
    tenant_id,
    channel,
    (regexp_replace(customer_phone, '[^0-9]', '', 'g')),
    updated_at DESC
  )
  WHERE customer_phone <> '';
