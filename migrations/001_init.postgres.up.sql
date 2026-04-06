CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS raw_projects (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source       TEXT NOT NULL,
    external_id  TEXT,
    raw_data     JSONB NOT NULL,
    collected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    hash         TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS leads (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    raw_project_id            UUID REFERENCES raw_projects(id),
    source                    TEXT,
    title                     TEXT,
    location                  TEXT,
    project_value             BIGINT,
    general_contractor        TEXT,
    project_type              TEXT,
    estimated_crew_size       INTEGER,
    estimated_duration_months INTEGER,
    out_of_town_crew_likely   BOOLEAN DEFAULT FALSE,
    priority_score            INTEGER,
    priority_reason           TEXT,
    suggested_outreach_timing TEXT,
    applicant                 TEXT,
    contractor                TEXT,
    source_url                TEXT,
    notes                     TEXT,
    status                    TEXT DEFAULT 'new',
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS outreach_log (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lead_id   UUID REFERENCES leads(id),
    contact   TEXT,
    channel   TEXT,
    notes     TEXT,
    outcome   TEXT,
    logged_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
