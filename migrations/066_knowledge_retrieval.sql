-- Recuperação por relevância da base de conhecimento (full-text do Postgres).
-- Aditivo e idempotente: não altera o conteúdo existente de knowledge_sources.

ALTER TABLE settings ADD COLUMN IF NOT EXISTS knowledge_top_k INTEGER NOT NULL DEFAULT 6;

CREATE TABLE IF NOT EXISTS knowledge_chunks (
  id BIGSERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  agent_id INTEGER NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  source_id BIGINT NOT NULL REFERENCES knowledge_sources(id) ON DELETE CASCADE,
  ord INTEGER NOT NULL DEFAULT 0,
  content TEXT NOT NULL DEFAULT '',
  content_chars INTEGER NOT NULL DEFAULT 0,
  search tsvector GENERATED ALWAYS AS (to_tsvector('portuguese', coalesce(content, ''))) STORED,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_tenant_agent ON knowledge_chunks (tenant_id, agent_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_source ON knowledge_chunks (source_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_search ON knowledge_chunks USING GIN (search);

ALTER TABLE knowledge_chunks ENABLE ROW LEVEL SECURITY;
ALTER TABLE knowledge_chunks FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON knowledge_chunks;
DROP POLICY IF EXISTS tenant_context_isolation ON knowledge_chunks;
CREATE POLICY tenant_context_isolation ON knowledge_chunks
  USING (tenant_row_visible(tenant_id)) WITH CHECK (tenant_row_visible(tenant_id));

-- Quebra o conteúdo da fonte em trechos de ~1200 caracteres, em limites de
-- palavra. SECURITY DEFINER para não depender do contexto RLS do chamador.
CREATE OR REPLACE FUNCTION knowledge_rebuild_chunks(p_source_id BIGINT)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
  src RECORD;
  words TEXT[];
  word TEXT;
  current_chunk TEXT := '';
  chunk_index INTEGER := 0;
  target_chars INTEGER := 1200;
BEGIN
  SELECT tenant_id, agent_id, content INTO src FROM knowledge_sources WHERE id = p_source_id;
  DELETE FROM knowledge_chunks WHERE source_id = p_source_id;
  IF src IS NULL OR btrim(coalesce(src.content, '')) = '' THEN
    RETURN;
  END IF;
  words := regexp_split_to_array(btrim(src.content), '\s+');
  FOREACH word IN ARRAY words LOOP
    IF word = '' THEN
      CONTINUE;
    END IF;
    IF length(current_chunk) + length(word) + 1 > target_chars AND length(current_chunk) > 0 THEN
      INSERT INTO knowledge_chunks (tenant_id, agent_id, source_id, ord, content, content_chars)
      VALUES (src.tenant_id, src.agent_id, p_source_id, chunk_index, current_chunk, length(current_chunk));
      chunk_index := chunk_index + 1;
      current_chunk := '';
    END IF;
    IF length(current_chunk) = 0 THEN
      current_chunk := word;
    ELSE
      current_chunk := current_chunk || ' ' || word;
    END IF;
  END LOOP;
  IF length(current_chunk) > 0 THEN
    INSERT INTO knowledge_chunks (tenant_id, agent_id, source_id, ord, content, content_chars)
    VALUES (src.tenant_id, src.agent_id, p_source_id, chunk_index, current_chunk, length(current_chunk));
  END IF;
END;
$$;

CREATE OR REPLACE FUNCTION knowledge_sources_sync_chunks()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
BEGIN
  PERFORM knowledge_rebuild_chunks(NEW.id);
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_knowledge_sources_sync_chunks ON knowledge_sources;
CREATE TRIGGER trg_knowledge_sources_sync_chunks
AFTER INSERT OR UPDATE OF content ON knowledge_sources
FOR EACH ROW EXECUTE FUNCTION knowledge_sources_sync_chunks();

-- Backfill das fontes já existentes.
DO $$
DECLARE
  rec RECORD;
BEGIN
  FOR rec IN SELECT id FROM knowledge_sources LOOP
    PERFORM knowledge_rebuild_chunks(rec.id);
  END LOOP;
END;
$$;
