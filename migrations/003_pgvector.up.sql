CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS lead_embeddings (
    lead_id    UUID PRIMARY KEY REFERENCES leads(id) ON DELETE CASCADE,
    model      TEXT NOT NULL,
    embedding  vector(512) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS lead_embeddings_ivfflat_idx
    ON lead_embeddings USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 10);
