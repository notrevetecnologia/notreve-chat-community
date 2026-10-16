CREATE TABLE IF NOT EXISTS public_api_keys (
  id BIGSERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  prefix VARCHAR(32) NOT NULL UNIQUE,
  secret_hash BYTEA NOT NULL,
  scopes TEXT[] NOT NULL DEFAULT '{}'::TEXT[],
  active BOOLEAN NOT NULL DEFAULT TRUE,
  last_used_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (length(trim(name)) > 0),
  CHECK (length(prefix) >= 8),
  CHECK (revoked_at IS NULL OR NOT active)
);

CREATE INDEX IF NOT EXISTS idx_public_api_keys_tenant_active
  ON public_api_keys (tenant_id, active, id DESC);

CREATE TABLE IF NOT EXISTS webhook_subscriptions (
  id BIGSERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  endpoint_url TEXT NOT NULL,
  events TEXT[] NOT NULL DEFAULT '{}'::TEXT[],
  signing_secret_hash BYTEA,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  last_delivery_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  UNIQUE (tenant_id, endpoint_url),
  CHECK (length(trim(name)) > 0),
  CHECK (length(trim(endpoint_url)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_webhook_subscriptions_tenant_active
  ON webhook_subscriptions (tenant_id, active, id DESC);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
  id BIGSERIAL PRIMARY KEY,
  tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  subscription_id BIGINT NOT NULL,
  event_type TEXT NOT NULL,
  event_id TEXT NOT NULL DEFAULT '',
  payload JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'processing', 'failed', 'delivered', 'dead_letter')),
  attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_attempt_at TIMESTAMPTZ,
  response_status INTEGER,
  response_body TEXT,
  error_message TEXT,
  delivered_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (tenant_id, subscription_id)
    REFERENCES webhook_subscriptions (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_due
  ON webhook_deliveries (tenant_id, status, next_attempt_at, id);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_subscription
  ON webhook_deliveries (tenant_id, subscription_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_webhook_deliveries_event_id
  ON webhook_deliveries (tenant_id, subscription_id, event_id)
  WHERE event_id <> '';
