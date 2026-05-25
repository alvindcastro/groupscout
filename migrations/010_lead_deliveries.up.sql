CREATE TABLE IF NOT EXISTS lead_deliveries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key TEXT NOT NULL UNIQUE,
    schedule_key    TEXT NOT NULL,
    lead_id         UUID REFERENCES leads(id),
    channel         TEXT NOT NULL,
    status          TEXT NOT NULL,
    result          TEXT,
    sent_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_lead_deliveries_lead_status
    ON lead_deliveries (lead_id, status);

CREATE TABLE IF NOT EXISTS delivery_locks (
    name        TEXT PRIMARY KEY,
    owner       TEXT NOT NULL,
    acquired_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL
);
