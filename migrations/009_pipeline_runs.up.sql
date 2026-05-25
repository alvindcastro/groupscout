CREATE TABLE IF NOT EXISTS pipeline_runs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status      TEXT NOT NULL,
    sources     JSONB NOT NULL DEFAULT '[]'::jsonb,
    counts      JSONB NOT NULL DEFAULT '{}'::jsonb,
    errors      JSONB NOT NULL DEFAULT '[]'::jsonb,
    request     JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pipeline_runs_status_started
    ON pipeline_runs (status, started_at DESC);
