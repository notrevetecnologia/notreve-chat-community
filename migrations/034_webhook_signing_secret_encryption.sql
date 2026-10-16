-- Mantém o hash usado pelas telas/API e guarda uma cópia recuperável apenas
-- para o worker interno assinar entregas. A chave não é persistida no banco.
ALTER TABLE webhook_subscriptions
  ADD COLUMN IF NOT EXISTS signing_secret_ciphertext BYTEA,
  ADD COLUMN IF NOT EXISTS signing_secret_nonce BYTEA;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'webhook_subscriptions_signing_secret_cipher_parts'
      AND conrelid = 'webhook_subscriptions'::regclass
  ) THEN
    ALTER TABLE webhook_subscriptions
      ADD CONSTRAINT webhook_subscriptions_signing_secret_cipher_parts
      CHECK (
        (signing_secret_ciphertext IS NULL AND signing_secret_nonce IS NULL)
        OR (
          signing_secret_ciphertext IS NOT NULL
          AND octet_length(signing_secret_ciphertext) > 0
          AND signing_secret_nonce IS NOT NULL
          AND octet_length(signing_secret_nonce) = 12
        )
      );
  END IF;
END $$;
