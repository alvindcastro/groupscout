CREATE TABLE IF NOT EXISTS raw_inputs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hash TEXT UNIQUE NOT NULL,
    payload_type TEXT,
    payload BYTEA,
    source_url TEXT,
    collector_name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE leads ADD COLUMN IF NOT EXISTS raw_input_id UUID REFERENCES raw_inputs(id);
