-- Outbox persistente para webhooks de saída.
--
-- A publicação de um evento e a criação das entregas por assinatura passam a
-- ser duráveis. O worker de transporte será conectado a esta fila em uma
-- etapa posterior; esta migration não executa HTTP nem expõe rotas.

CREATE TABLE IF NOT EXISTS webhook_outbox_events (
  id BIGSERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  event_key TEXT NOT NULL,
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, event_key),
  CHECK (btrim(event_key) <> ''),
  CHECK (char_length(event_key) <= 200),
  CHECK (btrim(event_type) <> ''),
  CHECK (char_length(event_type) <= 100)
);

CREATE INDEX IF NOT EXISTS idx_webhook_outbox_events_tenant_created
  ON webhook_outbox_events (tenant_id, created_at DESC, id DESC);

-- Uma entrega é criada para cada assinatura ativa no instante em que o evento
-- entra na outbox. A chave única evita que uma repetição idempotente recrie
-- entregas para a mesma assinatura.
CREATE TABLE IF NOT EXISTS webhook_outbox_deliveries (
  id BIGSERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  event_id BIGINT NOT NULL,
  subscription_id BIGINT NOT NULL,
  endpoint_url TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'processing', 'retry_scheduled', 'delivered', 'dead_letter')),
  attempts INTEGER NOT NULL DEFAULT 0,
  max_attempts INTEGER NOT NULL DEFAULT 5,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_attempt_at TIMESTAMPTZ,
  lease_until TIMESTAMPTZ,
  response_status SMALLINT,
  last_error_code TEXT NOT NULL DEFAULT '',
  last_error_message TEXT NOT NULL DEFAULT '',
  delivered_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (tenant_id, event_id)
    REFERENCES webhook_outbox_events (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, subscription_id)
    REFERENCES webhook_subscriptions (tenant_id, id) ON DELETE CASCADE,
  UNIQUE (tenant_id, subscription_id, event_id),
  CHECK (attempts >= 0 AND attempts <= max_attempts),
  CHECK (max_attempts BETWEEN 1 AND 10),
  CHECK (response_status IS NULL OR response_status BETWEEN 100 AND 599),
  CHECK (btrim(endpoint_url) <> ''),
  CHECK (char_length(last_error_code) <= 64),
  CHECK (char_length(last_error_message) <= 1024)
);

-- Inclui leases vencidos para que uma queda do worker não deixe entregas
-- permanentemente presas em processamento.
CREATE INDEX IF NOT EXISTS idx_webhook_outbox_deliveries_claim
  ON webhook_outbox_deliveries (tenant_id, next_attempt_at, id)
  WHERE status IN ('pending', 'retry_scheduled');

CREATE INDEX IF NOT EXISTS idx_webhook_outbox_deliveries_lease
  ON webhook_outbox_deliveries (tenant_id, lease_until, id)
  WHERE status = 'processing';

CREATE INDEX IF NOT EXISTS idx_webhook_outbox_deliveries_subscription
  ON webhook_outbox_deliveries (tenant_id, subscription_id, created_at DESC, id DESC);
