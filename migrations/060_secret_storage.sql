-- Lookup de tokens de canais sem depender do valor reversível armazenado.
-- O conteúdo da coluna token passa a poder ser cifrado pela aplicação.
ALTER TABLE channels
    ADD COLUMN IF NOT EXISTS token_hash BYTEA;

CREATE INDEX IF NOT EXISTS idx_channels_token_hash_active
    ON channels (token_hash)
    WHERE active = true AND token_hash IS NOT NULL;

-- public_id deixa de ser exclusivo do WebChat: Telegram novo também usa um
-- identificador público aleatório no webhook e nunca expõe o bot token na URL.
CREATE UNIQUE INDEX IF NOT EXISTS idx_channels_public_id_nonempty
    ON channels (public_id)
    WHERE public_id <> '';
