ALTER TABLE usuarios
  ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT FALSE;

CREATE OR REPLACE FUNCTION mark_usuario_password_as_temporary()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    IF NEW.role <> 'super_admin' THEN
      NEW.must_change_password := TRUE;
    END IF;
  ELSIF NEW.password_hash IS DISTINCT FROM OLD.password_hash THEN
    NEW.must_change_password := TRUE;
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_usuarios_temporary_password ON usuarios;
CREATE TRIGGER trg_usuarios_temporary_password
BEFORE INSERT OR UPDATE OF password_hash ON usuarios
FOR EACH ROW
EXECUTE FUNCTION mark_usuario_password_as_temporary();
