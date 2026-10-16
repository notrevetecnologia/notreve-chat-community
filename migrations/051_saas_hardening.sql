ALTER TABLE usuarios
  ADD COLUMN IF NOT EXISTS credentials_changed_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE OR REPLACE FUNCTION touch_usuario_credentials_changed_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.password_hash IS DISTINCT FROM OLD.password_hash
     OR NEW.active IS DISTINCT FROM OLD.active THEN
    NEW.credentials_changed_at := now();
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_usuarios_credentials_changed_at ON usuarios;
CREATE TRIGGER trg_usuarios_credentials_changed_at
BEFORE UPDATE OF password_hash, active ON usuarios
FOR EACH ROW
EXECUTE FUNCTION touch_usuario_credentials_changed_at();

UPDATE usuarios SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE channels SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE agents SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE providers SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE setores SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE atendentes SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE conversations SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE messages SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE contatos SET tenant_id = 1 WHERE tenant_id IS NULL;
UPDATE campanhas SET tenant_id = 1 WHERE tenant_id IS NULL;

ALTER TABLE usuarios ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE channels ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE agents ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE providers ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE setores ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE atendentes ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE conversations ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE messages ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE contatos ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE campanhas ALTER COLUMN tenant_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_usuarios_tenant_id_id ON usuarios (tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_channels_tenant_id_id ON channels (tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_agents_tenant_id_id ON agents (tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_providers_tenant_id_id ON providers (tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_setores_tenant_id_id ON setores (tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_atendentes_tenant_id_id ON atendentes (tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_conversations_tenant_id_id ON conversations (tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_contatos_tenant_id_id ON contatos (tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_campanhas_tenant_id_id ON campanhas (tenant_id, id);

CREATE OR REPLACE FUNCTION enforce_same_tenant_reference()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  reference_id integer;
  reference_exists boolean;
BEGIN
  reference_id := NULLIF(to_jsonb(NEW) ->> TG_ARGV[1], '')::integer;
  IF reference_id IS NULL THEN
    RETURN NEW;
  END IF;

  EXECUTE format(
    'SELECT EXISTS (SELECT 1 FROM %I WHERE tenant_id = $1 AND id = $2)',
    TG_ARGV[0]
  ) INTO reference_exists USING NEW.tenant_id, reference_id;

  IF NOT reference_exists THEN
    RAISE EXCEPTION 'referência % inválida para o tenant %', TG_ARGV[1], NEW.tenant_id
      USING ERRCODE = '23503';
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_usuarios_same_tenant_atendente ON usuarios;
CREATE TRIGGER trg_usuarios_same_tenant_atendente
BEFORE INSERT OR UPDATE OF tenant_id, atendente_id ON usuarios
FOR EACH ROW EXECUTE FUNCTION enforce_same_tenant_reference('atendentes', 'atendente_id');

DROP TRIGGER IF EXISTS trg_agents_same_tenant_provider ON agents;
CREATE TRIGGER trg_agents_same_tenant_provider
BEFORE INSERT OR UPDATE OF tenant_id, provider_id ON agents
FOR EACH ROW EXECUTE FUNCTION enforce_same_tenant_reference('providers', 'provider_id');

DROP TRIGGER IF EXISTS trg_conversations_same_tenant_channel ON conversations;
CREATE TRIGGER trg_conversations_same_tenant_channel
BEFORE INSERT OR UPDATE OF tenant_id, channel_id ON conversations
FOR EACH ROW EXECUTE FUNCTION enforce_same_tenant_reference('channels', 'channel_id');

DROP TRIGGER IF EXISTS trg_conversations_same_tenant_agent ON conversations;
CREATE TRIGGER trg_conversations_same_tenant_agent
BEFORE INSERT OR UPDATE OF tenant_id, agent_id ON conversations
FOR EACH ROW EXECUTE FUNCTION enforce_same_tenant_reference('agents', 'agent_id');

DROP TRIGGER IF EXISTS trg_conversations_same_tenant_setor ON conversations;
CREATE TRIGGER trg_conversations_same_tenant_setor
BEFORE INSERT OR UPDATE OF tenant_id, setor_id ON conversations
FOR EACH ROW EXECUTE FUNCTION enforce_same_tenant_reference('setores', 'setor_id');

DROP TRIGGER IF EXISTS trg_messages_same_tenant_conversation ON messages;
CREATE TRIGGER trg_messages_same_tenant_conversation
BEFORE INSERT OR UPDATE OF tenant_id, conversation_id ON messages
FOR EACH ROW EXECUTE FUNCTION enforce_same_tenant_reference('conversations', 'conversation_id');
