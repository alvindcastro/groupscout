# PROMPTS_PHASE15.md — PostgreSQL + pgvector Migration

> Copy-paste prompts for each part of Phase 15.
> Each prompt is self-contained — paste it with the relevant file(s) attached or quoted.
> Parts must be done in order: A → B → C → D → E → F.

---

## Part A — Postgres Container + Driver

```
You are working on a Go project called groupscout (module: github.com/alvindcastro/groupscout, Go 1.26).

## Context

The project currently uses SQLite via `modernc.org/sqlite` (pure Go, no CGO). We are migrating to PostgreSQL. The database layer uses `database/sql` throughout — we are NOT switching away from `database/sql`. We will use `github.com/jackc/pgx/v5/stdlib` as the Postgres driver, which registers as a `database/sql`-compatible driver. This keeps all existing store code working with minimal changes.

SQLite stays available as a local dev fallback, detected by the DATABASE_URL value.

## Current file: internal/storage/db.go

```go
package storage

import (
    "database/sql"
    "strings"

    _ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS raw_projects ( ... );
CREATE TABLE IF NOT EXISTS leads ( ... );
CREATE TABLE IF NOT EXISTS outreach_log ( ... );
`

func Open(dsn string) (*sql.DB, error) {
    db, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, err
    }
    if err := db.Ping(); err != nil {
        db.Close()
        return nil, err
    }
    return db, nil
}

func Migrate(db *sql.DB) error { ... }
```

## Current file: go.mod (relevant lines)

```
module github.com/alvindcastro/groupscout
go 1.26

require (
    modernc.org/sqlite v1.33.1
    ...
)
```

## Current file: config/config.go

Read this file first before making changes. It loads DATABASE_URL from the environment.

## Task

1. Add `github.com/jackc/pgx/v5` to go.mod (run `go get github.com/jackc/pgx/v5`).

2. Add a `DriverName(dsn string) string` helper function to `internal/storage/db.go`:
   - Returns `"pgx"` if dsn starts with `"postgres://"` or `"postgresql://"`
   - Returns `"sqlite"` otherwise

3. Update `Open(dsn string)` in `internal/storage/db.go` to:
   - Import `_ "github.com/jackc/pgx/v5/stdlib"` alongside the existing SQLite import
   - Call `sql.Open(DriverName(dsn), dsn)` instead of hardcoding `"sqlite"`
   - Keep all other logic identical

4. Update `docker-compose.yml`:
   - Add a `postgres` service using the `pgvector/pgvector:pg17` image
   - Named volume: `pgdata`
   - Environment: `POSTGRES_DB=groupscout`, `POSTGRES_USER=groupscout`, `POSTGRES_PASSWORD=groupscout`
   - Health check: `pg_isready -U groupscout`
   - Expose port `5432`

5. Update `.env.example`:
   - Add commented example: `# DATABASE_URL=postgres://groupscout:groupscout@localhost:5432/groupscout`
   - Keep the existing SQLite example as the default

## Constraints
- Do NOT remove the `modernc.org/sqlite` import — SQLite stays for local dev
- Do NOT change the Migrate() function yet — that is Part B
- Do NOT change any store files (leads.go, raw.go) — that is Part C
- `pgx/v5/stdlib` registers itself as driver name `"pgx"` — use that exact string

## Acceptance criteria
- `go build ./...` passes
- `DATABASE_URL=groupscout.db go run ./cmd/server/ --run-once` still works (SQLite path)
- `DATABASE_URL=postgres://groupscout:groupscout@localhost:5432/groupscout` connects successfully when Postgres container is running (no migrations yet, just a ping)
```

---

## Part B — Schema (Postgres-compatible migrations)

```
You are working on groupscout (module: github.com/alvindcastro/groupscout, Go 1.26).

## Context

Part A is complete. We now have:
- `pgx/v5` in go.mod
- `Open(dsn)` auto-selects the driver based on DATABASE_URL prefix
- `pgvector/pgvector:pg17` running in Docker Compose

We need to write Postgres-compatible SQL migrations and wire `golang-migrate/migrate` to run them.

## Current schema (SQLite, from internal/storage/db.go)

The current schema uses SQLite-specific types. Here is the exact DDL:

```sql
CREATE TABLE IF NOT EXISTS raw_projects (
    id           TEXT PRIMARY KEY,
    source       TEXT NOT NULL,
    external_id  TEXT,
    raw_data     TEXT NOT NULL,
    collected_at DATETIME NOT NULL,
    hash         TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS leads (
    id                        TEXT PRIMARY KEY,
    raw_project_id            TEXT REFERENCES raw_projects(id),
    source                    TEXT,
    title                     TEXT,
    location                  TEXT,
    project_value             INTEGER,
    general_contractor        TEXT,
    project_type              TEXT,
    estimated_crew_size       INTEGER,
    estimated_duration_months INTEGER,
    out_of_town_crew_likely   INTEGER DEFAULT 0,
    priority_score            INTEGER,
    priority_reason           TEXT,
    suggested_outreach_timing TEXT,
    applicant                 TEXT,
    contractor                TEXT,
    source_url                TEXT,
    notes                     TEXT,
    status                    TEXT DEFAULT 'new',
    created_at                DATETIME NOT NULL,
    updated_at                DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS outreach_log (
    id        TEXT PRIMARY KEY,
    lead_id   TEXT REFERENCES leads(id),
    contact   TEXT,
    channel   TEXT,
    notes     TEXT,
    outcome   TEXT,
    logged_at DATETIME NOT NULL
);
```

## Task

1. Create `migrations/001_init.postgres.up.sql` with Postgres-native types:
   - `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
   - `BOOLEAN` for out_of_town_crew_likely (not INTEGER)
   - `TIMESTAMPTZ DEFAULT NOW()` for all timestamp columns
   - `JSONB` for raw_data (not TEXT) — enables JSON indexing
   - `BIGINT` for project_value (not INTEGER)
   - Keep all column names identical to the SQLite schema
   - Add `CREATE EXTENSION IF NOT EXISTS "pgcrypto";` at the top (needed for gen_random_uuid())

2. Create `migrations/001_init.postgres.down.sql`:
   - DROP TABLE statements in reverse dependency order

3. Add `github.com/golang-migrate/migrate/v4` to go.mod.

4. Update `internal/storage/db.go` — replace the current `Migrate(db *sql.DB)` function:
   - For SQLite path: keep the existing inline schema approach (no migrate needed)
   - For Postgres path: use `golang-migrate` to run migrations from the `migrations/` directory
   - The function signature stays: `Migrate(db *sql.DB) error`
   - Detect which path to use via `DriverName()` applied to the DSN — pass dsn into Migrate or store it on a struct; your choice, keep it simple

## Constraints
- Do NOT change leads.go or raw.go yet — that is Part C
- The SQLite schema and Migrate() path must continue to work unchanged for local dev
- Migration files must be named following golang-migrate convention: `{version}_{name}.{direction}.sql`

## Acceptance criteria
- `go build ./...` passes
- SQLite path: `DATABASE_URL=groupscout.db go run ./cmd/server/ --run-once` works identically
- Postgres path: running with `DATABASE_URL=postgres://...` creates all three tables with correct Postgres types
- `\d leads` in psql shows UUID, BOOLEAN, TIMESTAMPTZ, JSONB columns
```

---

## Part C — Storage Layer Fixes

```
You are working on groupscout (module: github.com/alvindcastro/groupscout, Go 1.26).

## Context

Parts A and B are complete. The Postgres schema exists. Now we need to fix the Go storage layer — it has SQLite-specific patterns that don't work with Postgres.

## Problems to fix

### 1. Boolean handling in internal/storage/leads.go

Current code uses a `boolToInt()` helper and scans booleans as `int`:

```go
// Insert — passes integer instead of bool
boolToInt(l.OutOfTownCrewLikely),  // sends 0 or 1

// ListNew and ListForDigest — scans as int then converts
var oot int
rows.Scan(..., &oot, ...)
l.OutOfTownCrewLikely = oot == 1

// The helper at the bottom of the file:
func boolToInt(b bool) int {
    if b { return 1 }
    return 0
}
```

Postgres has a native BOOLEAN type. pgx handles `bool` natively — no conversion needed.

### 2. Parameter placeholders

SQLite uses `?`. Postgres uses `$1, $2, $3...`. All SQL in leads.go and raw.go must use `$N` style.

Current examples:
```go
// leads.go Insert
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)

// leads.go ListForDigest
AND created_at >= ?

// leads.go UpdateStatus
UPDATE leads SET status = ?, updated_at = ? WHERE id = ?

// raw.go Insert
INSERT INTO raw_projects (id, source, external_id, raw_data, collected_at, hash)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(hash) DO NOTHING

// raw.go ExistsByHash
SELECT COUNT(1) FROM raw_projects WHERE hash = ?
```

### 3. UUID generation

Current `NewUUID()` in raw.go generates UUIDs in Go. This works for both SQLite and Postgres. Keep it — do NOT switch to `gen_random_uuid()` in SQL. Go-side generation is simpler and avoids driver differences.

### 4. Time scanning

pgx/v5/stdlib scans `TIMESTAMPTZ` directly into `time.Time` — no changes needed there. But verify that `time.Now().UTC()` is what gets stored.

## Full current content of internal/storage/leads.go

[Attach the full leads.go file here]

## Full current content of internal/storage/raw.go

[Attach the full raw.go file here]

## Task

1. Update `internal/storage/leads.go`:
   - Remove `boolToInt()` helper function
   - In `Insert()`: pass `l.OutOfTownCrewLikely` (bool) directly — no conversion
   - In `ListNew()` and `ListForDigest()`: scan `out_of_town_crew_likely` directly into `l.OutOfTownCrewLikely` (bool) — remove `var oot int` and the `oot == 1` conversion
   - Replace ALL `?` placeholders with `$1, $2...` (numbered sequentially per query)
   - Rename struct and constructor: `sqliteLeadStore` → `sqlLeadStore`, `NewLeadStore` keeps same signature

2. Update `internal/storage/raw.go`:
   - Replace ALL `?` placeholders with `$1, $2...`
   - `ON CONFLICT(hash) DO NOTHING` is valid Postgres syntax — keep it
   - Rename struct: `sqliteRawStore` → `sqlRawStore`

## Constraints
- Keep all interface signatures identical (`LeadStore`, `RawProjectStore`)
- Keep `NewUUID()` in raw.go as-is
- Do NOT add any build tags or conditional compilation — one code path works for both drivers
- pgx/v5/stdlib with database/sql handles both `bool` and `time.Time` natively for Postgres

## Acceptance criteria
- `go test ./...` passes
- `go build ./...` passes
- Full pipeline end-to-end against Postgres: permits collected, enriched, stored, Slack notification sent
- Full pipeline end-to-end against SQLite still works (DATABASE_URL=groupscout.db)
```

---

## Part D — pgvector

```
You are working on groupscout (module: github.com/alvindcastro/groupscout, Go 1.26).

## Context

Parts A, B, C are complete. We have a working Postgres storage layer. Now we add pgvector for vector similarity search — this is the foundation for the RAG enrichment roadmap.

The project uses `database/sql` throughout. pgvector values are stored as `[]float32` and passed as JSON text in SQLite, or as the native `vector` type in Postgres.

## What pgvector provides

- A `vector(N)` column type in Postgres
- The `<=>` operator for cosine distance: `ORDER BY embedding <=> $1 LIMIT $2`
- An `ivfflat` index for fast approximate nearest-neighbor search
- The `pgvector/pgvector-go` Go package for encoding vectors

## Task

### 1. Create migration: migrations/003_pgvector.up.sql

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS lead_embeddings (
    lead_id    UUID PRIMARY KEY REFERENCES leads(id) ON DELETE CASCADE,
    model      TEXT NOT NULL,
    embedding  vector(512) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON lead_embeddings USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 10);
```

### 2. Create migration: migrations/003_pgvector.down.sql

Drop the table and extension (drop index is implicit with table drop).

### 3. Create internal/storage/embeddings.go

Define the interface and both implementations:

```go
package storage

import (
    "context"
    "math"
)

// EmbeddingStore persists and queries lead vector embeddings.
type EmbeddingStore interface {
    Save(ctx context.Context, leadID, model string, vec []float32) error
    // Similar returns up to k lead IDs whose embeddings are closest to vec.
    Similar(ctx context.Context, vec []float32, k int) ([]string, error)
}

// NewEmbeddingStore returns the correct implementation based on DATABASE_URL.
// Postgres URL → PostgresEmbeddingStore (uses pgvector <=> operator)
// SQLite path  → InMemoryEmbeddingStore (Go cosine similarity, loads all from DB)
func NewEmbeddingStore(db *sql.DB, dsn string) EmbeddingStore {
    if DriverName(dsn) == "pgx" {
        return &postgresEmbeddingStore{db: db}
    }
    return &inMemoryEmbeddingStore{db: db}
}
```

**PostgresEmbeddingStore:**
- `Save`: INSERT into `lead_embeddings`; use `github.com/pgvector/pgvector-go` to encode `[]float32` as a pgvector value
- `Similar`: `SELECT lead_id FROM lead_embeddings ORDER BY embedding <=> $1 LIMIT $2`; return lead IDs

**InMemoryEmbeddingStore (SQLite fallback):**
- Store embeddings as a JSON text blob in a `lead_embeddings_sqlite` table:
  `CREATE TABLE IF NOT EXISTS lead_embeddings_sqlite (lead_id TEXT PRIMARY KEY, model TEXT, embedding TEXT, created_at DATETIME)`
- `Save`: marshal `[]float32` to JSON, store as TEXT
- `Similar`: load ALL rows, unmarshal each embedding, compute cosine similarity in Go, return top-k

Cosine similarity helper (add to embeddings.go, unexported):
```go
func cosineSimilarity(a, b []float32) float32 {
    var dot, na, nb float32
    for i := range a {
        dot += a[i] * b[i]
        na += a[i] * a[i]
        nb += b[i] * b[i]
    }
    if na == 0 || nb == 0 {
        return 0
    }
    return dot / (float32(math.Sqrt(float64(na))) * float32(math.Sqrt(float64(nb))))
}
```

### 4. Add to go.mod

Run: `go get github.com/pgvector/pgvector-go`

## Constraints
- Do NOT change enricher.go or claude.go — the embedding calls will be wired in Phase 16 (RAG)
- The SQLite InMemoryEmbeddingStore is a dev fallback; performance is acceptable for <500 leads
- Lists = 10 in the ivfflat index is appropriate for small datasets; increase when leads > 10k

## Acceptance criteria
- `go build ./...` passes
- Postgres: `SELECT * FROM lead_embeddings LIMIT 1` returns rows after a manual Save() call
- Postgres: `EXPLAIN SELECT lead_id FROM lead_embeddings ORDER BY embedding <=> '[0.1,0.2,...]' LIMIT 3` uses the ivfflat index
- SQLite: `Similar()` returns lead IDs sorted by cosine similarity (unit test with known vectors)
```

---

## Part E — SQLite → Postgres Data Migration

```
You are working on groupscout (module: github.com/alvindcastro/groupscout, Go 1.26).

## Context

Parts A–D are complete. Postgres is fully wired. We now need a one-time script to migrate existing data from the SQLite file to Postgres. This is safe to skip if starting fresh (no production data yet).

## Current SQLite schema

Three tables: `raw_projects`, `leads`, `outreach_log`. See migrations/001_init.postgres.up.sql for the Postgres equivalents.

Key type differences to handle during migration:
- `out_of_town_crew_likely`: SQLite stores as INTEGER (0/1) → read as int, write as bool to Postgres
- `id` columns: SQLite stores as TEXT (UUID string) → Postgres UUID column accepts the same string format
- `raw_data`: SQLite stores as TEXT (JSON string) → Postgres JSONB accepts the same JSON string via `::jsonb` cast or direct insertion
- Timestamps: SQLite stores as TEXT (Go time.Time serialized) → parse with `time.Parse` and write as `time.Time` to Postgres

## Task

Create `scripts/migrate_to_postgres/main.go`:

```
Usage: migrate_to_postgres --sqlite groupscout.db --postgres "postgres://groupscout:groupscout@localhost:5432/groupscout"
```

The script should:
1. Open both databases (SQLite via `modernc.org/sqlite`, Postgres via `pgx/v5/stdlib`)
2. Migrate in this order (respect foreign keys): raw_projects → leads → outreach_log
3. For each table: SELECT all rows from SQLite, INSERT into Postgres in batches of 100
4. Use `ON CONFLICT (id) DO NOTHING` on all inserts — safe to re-run
5. Print progress: "Migrated N raw_projects", "Migrated N leads", "Migrated N outreach_log entries"
6. Print final row counts for both databases as a verification check

Handle the boolean conversion:
```go
// SQLite returns 0 or 1 for INTEGER columns
var ootInt int
rows.Scan(..., &ootInt, ...)
ootBool := ootInt == 1
// Then pass ootBool to Postgres INSERT
```

## Constraints
- This is a one-shot utility script — simple, no retry logic needed
- Use `database/sql` for both sides (same drivers already in go.mod)
- Do not import internal packages — copy the minimum SQL needed inline
- Add a `--dry-run` flag that prints counts without writing to Postgres

## Acceptance criteria
- Script compiles: `go build ./scripts/migrate_to_postgres/`
- Running against real data: row counts match between SQLite and Postgres for all three tables
- Re-running is idempotent (ON CONFLICT DO NOTHING prevents duplicates)
```

---

## Part F — Productionize

```
You are working on groupscout (module: github.com/alvindcastro/groupscout, Go 1.26).

## Context

Parts A–E are complete. The migration works. Now we clean up configs, Docker Compose, and docs so the Postgres path is the default for deployment while SQLite remains available for local dev.

## Task

### 1. Update .env.example

Read the current .env.example file first. Then:
- Set the primary DATABASE_URL example to Postgres format
- Keep the SQLite example as a comment for local dev
- Add any missing env vars introduced in Parts A–D (check config/config.go for new fields)

Target result:
```env
# --- Database ---
DATABASE_URL=postgres://groupscout:groupscout@localhost:5432/groupscout
# Local dev (SQLite): DATABASE_URL=groupscout.db
```

### 2. Update docker-compose.yml

Read the current docker-compose.yml first. Then:
- Ensure the `groupscout` app service has `depends_on: postgres: condition: service_healthy`
- Ensure the `postgres` service (added in Part A) has the correct health check
- Set `DATABASE_URL` in the app service environment to the Postgres URL
- Remove any SQLite file volume reference from the app service if present

### 3. Update docs/SETUP.md

Read the current SETUP.md first. Then add a section **"Postgres Setup (recommended)"** above any existing SQLite instructions:

```markdown
## Postgres Setup (recommended)

1. Start Postgres: `docker compose up postgres -d`
2. Wait for healthy: `docker compose ps`
3. Set in .env: `DATABASE_URL=postgres://groupscout:groupscout@localhost:5432/groupscout`
4. Run: `go run ./cmd/server/ --run-once`
   Migrations run automatically on startup.

## SQLite (local dev only)

Set `DATABASE_URL=groupscout.db`. No Docker required.
```

### 4. Verify the full Docker Compose stack

Run through this checklist manually and confirm each step:
- [ ] `docker compose up -d` — all services start, Postgres is healthy
- [ ] `DATABASE_URL=postgres://... go run ./cmd/server/ --run-once` — migrations run, pipeline completes
- [ ] Slack receives a digest message
- [ ] `psql -U groupscout -d groupscout -c "SELECT COUNT(*) FROM leads"` — returns > 0

## Constraints
- Do not remove SQLite support — it must still work with `DATABASE_URL=groupscout.db`
- Do not add complexity; these are config and doc changes only
```

---

## Tips for Using These Prompts

- **Attach files:** When a prompt says "read this file first" or "[Attach X here]", paste the full file content into your message alongside the prompt. Most AI assistants work best when given the actual code, not just descriptions.
- **One part at a time:** Complete and verify each part before starting the next. The parts have hard dependencies.
- **Verify steps are non-negotiable:** Each part ends with acceptance criteria — run them before moving on.
- **If something breaks:** Parts C and D touch the most code. If the pipeline breaks, check placeholder style (`$N` vs `?`) and boolean handling first.
