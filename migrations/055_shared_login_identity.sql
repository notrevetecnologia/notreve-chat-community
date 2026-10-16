DO $$
BEGIN
  IF EXISTS (
    SELECT lower(username)
    FROM usuarios
    GROUP BY lower(username)
    HAVING COUNT(*) > 1
  ) THEN
    RAISE EXCEPTION 'existem usuários duplicados por maiúsculas/minúsculas; normalize os usernames antes de ativar o login compartilhado';
  END IF;
END;
$$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_usuarios_username_lower
  ON usuarios (lower(username));
