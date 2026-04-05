# PROMPTS_PHASE15.md — PostgreSQL + pgvector Migration (TDD)

> Copy-paste prompts for each part of Phase 15.
> Every part follows **Red → Green → Refactor**: write the failing tests first, then implement until they pass.
> Parts must be done in order: A → B → C → D → E → F.
>
> **Test conventions in this project:**
> - Standard `testing` package only (no testify wrappers, even though it is in go.mod)
> - Table-driven tests using `[]struct{ name string; ... }` slices
> - `t.Errorf()` for assertions, not `t.Fatal` unless setup failure
> - Same package as production code (e.g. `package storage`, not `package storage_test`)
> - Integration tests (require Docker/Postgres): `//go:build integration` build tag at top of file
> - Run unit tests: `go test ./...`
> - Run integration tests: `go test -tags integration ./...`

---

## Part A — Postgres Container + Driver

```
You are working on a Go project called groupscout (module: github.com/alvindcastro/groupscout, Go 1.26).
Follow TDD: write the failing tests first, then implement until they pass.

## Context

The project currently uses SQLite via `modernc.org/sqlite` (pure Go, no CGO).
We are adding Postgres support via `github.com/jackc/pgx/v5/stdlib` (pure Go, registers as a
`database/sql`-compatible driver named "pgx"). SQLite stays as a local dev fallback.
The DATABASE_URL value determines which driver is used.

## Current internal/storage/db.go

```go
package storage

import (
    "database/sql"
    "strings"
    _ "modernc.org/sqlite"
)

func Open(dsn string) (*sql.DB, error) {
    db, err := sql.Open("sqlite", dsn)
    ...
}

func Migrate(db *sql.DB) error { ... }
```

## Current go.mod (relevant lines)
```
module github.com/alvindcastro/groupscout
go 1.26
require (
    modernc.org/sqlite v1.33.1
)
```

## Step 1 — Write this test file first (it must FAIL before you write any implementation)

Create `internal/storage/db_test.go`:

```go
package storage

import "testing"

func TestDriverName(t *testing.T) {
    tests := []struct {
        dsn  string
        want string
    }{
        {"groupscout.db", "sqlite"},
        {"./data/groupscout.db", "sqlite"},
        {"postgres://user:pass@localhost:5432/groupscout", "pgx"},
        {"postgresql://user:pass@localhost:5432/groupscout", "pgx"},
        {"postgres://localhost/groupscout?sslmode=disable", "pgx"},
    }
    for _, tt := range tests {
        t.Run(tt.dsn, func(t *testing.T) {
            got := DriverName(tt.dsn)
            if got != tt.want {
                t.Errorf("DriverName(%q) = %q, want %q", tt.dsn, got, tt.want)
            }
        })
    }
}
```

Create `internal/storage/db_integration_test.go` (requires Docker):

```go
//go:build integration

package storage

import (
    "os"
    "testing"
)

func TestOpen_postgres(t *testing.T) {
    dsn := os.Getenv("TEST_POSTGRES_URL")
    if dsn == "" {
        t.Skip("TEST_POSTGRES_URL not set")
    }
    db, err := Open(dsn)
    if err != nil {
        t.Fatalf("Open(%q) error: %v", dsn, err)
    }
    defer db.Close()
    if err := db.Ping(); err != nil {
        t.Fatalf("Ping() error: %v", err)
    }
}

func TestOpen_sqlite_still_works(t *testing.T) {
    db, err := Open(":memory:")
    if err != nil {
        t.Fatalf("Open(':memory:') error: %v", err)
    }
    defer db.Close()
}
```

## Step 2 — Confirm tests fail

Run `go test ./internal/storage/` — it should fail with "undefined: DriverName".

## Step 3 — Implement

1. Add `github.com/jackc/pgx/v5` to go.mod: `go get github.com/jackc/pgx/v5`

2. Add to `internal/storage/db.go`:
   - Import `_ "github.com/jackc/pgx/v5/stdlib"` alongside existing SQLite import
   - Add `DriverName(dsn string) string` — returns "pgx" if dsn has prefix "postgres://" or "postgresql://", else "sqlite"
   - Update `Open()` to call `sql.Open(DriverName(dsn), dsn)` instead of hardcoding "sqlite"

3. Add to `docker-compose.yml`:
   - Service `postgres` using image `pgvector/pgvector:pg17`
   - Environment: POSTGRES_DB=groupscout, POSTGRES_USER=groupscout, POSTGRES_PASSWORD=groupscout
   - Named volume `pgdata`
   - Health check: `pg_isready -U groupscout`
   - Port 5432:5432

4. Add to `.env.example`:
   - `# DATABASE_URL=postgres://groupscout:groupscout@localhost:5432/groupscout`
   - Keep existing SQLite line as the uncommented default

## Constraints
- Do NOT remove `modernc.org/sqlite` — SQLite must still work
- Do NOT change Migrate() yet — that is Part B
- Do NOT change leads.go or raw.go — that is Part C

## Done when
- `go test ./internal/storage/` passes (DriverName unit tests)
- `go test -tags integration ./internal/storage/` passes with TEST_POSTGRES_URL set
- `DATABASE_URL=groupscout.db go run ./cmd/server/ --run-once` still works
```

---

## Part B — Schema (Postgres-compatible migrations)

```
You are working on groupscout (module: github.com/alvindcastro/groupscout, Go 1.26).
Follow TDD: write the failing tests first, then implement until they pass.
Part A is complete: DriverName() exists, pgx/v5 is in go.mod, Postgres container is in docker-compose.yml.

## Current SQLite schema (from internal/storage/db.go)

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
    source                    TEXT, title TEXT, location TEXT,
    project_value             INTEGER,
    general_contractor        TEXT, project_type TEXT,
    estimated_crew_size       INTEGER, estimated_duration_months INTEGER,
    out_of_town_crew_likely   INTEGER DEFAULT 0,
    priority_score            INTEGER,
    priority_reason           TEXT, suggested_outreach_timing TEXT,
    applicant TEXT, contractor TEXT, source_url TEXT, notes TEXT,
    status                    TEXT DEFAULT 'new',
    created_at                DATETIME NOT NULL,
    updated_at                DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS outreach_log (
    id TEXT PRIMARY KEY, lead_id TEXT REFERENCES leads(id),
    contact TEXT, channel TEXT, notes TEXT, outcome TEXT,
    logged_at DATETIME NOT NULL
);
```

## Step 1 — Write this test file first (it must FAIL before you write any implementation)

Create `internal/storage/migrate_integration_test.go`:

```go
//go:build integration

package storage

import (
    "database/sql"
    "os"
    "testing"
)

func TestMigrate_postgres_tables_exist(t *testing.T) {
    dsn := os.Getenv("TEST_POSTGRES_URL")
    if dsn == "" {
        t.Skip("TEST_POSTGRES_URL not set")
    }
    db, err := Open(dsn)
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    defer db.Close()
    if err := Migrate(db, dsn); err != nil {
        t.Fatalf("Migrate: %v", err)
    }

    tables := []string{"raw_projects", "leads", "outreach_log"}
    for _, table := range tables {
        var count int
        err := db.QueryRow(
            `SELECT COUNT(*) FROM information_schema.tables
             WHERE table_schema='public' AND table_name=$1`, table,
        ).Scan(&count)
        if err != nil || count == 0 {
            t.Errorf("table %q does not exist after migration", table)
        }
    }
}

func TestMigrate_postgres_column_types(t *testing.T) {
    dsn := os.Getenv("TEST_POSTGRES_URL")
    if dsn == "" {
        t.Skip("TEST_POSTGRES_URL not set")
    }
    db, _ := Open(dsn)
    defer db.Close()
    Migrate(db, dsn)

    tests := []struct {
        table, column, wantType string
    }{
        {"leads", "id", "uuid"},
        {"leads", "out_of_town_crew_likely", "boolean"},
        {"leads", "created_at", "timestamp with time zone"},
        {"leads", "project_value", "bigint"},
        {"raw_projects", "raw_data", "jsonb"},
    }
    for _, tt := range tests {
        var dataType string
        err := db.QueryRow(`
            SELECT data_type FROM information_schema.columns
            WHERE table_name=$1 AND column_name=$2`, tt.table, tt.column,
        ).Scan(&dataType)
        if err != nil {
            t.Errorf("%s.%s: query error: %v", tt.table, tt.column, err)
            continue
        }
        if dataType != tt.wantType {
            t.Errorf("%s.%s type = %q, want %q", tt.table, tt.column, dataType, tt.wantType)
        }
    }
}

func TestMigrate_idempotent(t *testing.T) {
    dsn := os.Getenv("TEST_POSTGRES_URL")
    if dsn == "" {
        t.Skip("TEST_POSTGRES_URL not set")
    }
    db, _ := Open(dsn)
    defer db.Close()
    // Running twice must not error
    if err := Migrate(db, dsn); err != nil {
        t.Fatalf("first Migrate: %v", err)
    }
    if err := Migrate(db, dsn); err != nil {
        t.Fatalf("second Migrate: %v", err)
    }
}

func TestMigrate_sqlite_still_works(t *testing.T) {
    db, _ := Open(":memory:")
    defer db.Close()
    if err := Migrate(db, ":memory:"); err != nil {
        t.Fatalf("SQLite Migrate: %v", err)
    }
}
```

Note: `Migrate` signature changes to `Migrate(db *sql.DB, dsn string) error` so it can detect the driver.
Update all callers (cmd/server/main.go) accordingly.

## Step 2 — Confirm tests fail

Run `go test -tags integration ./internal/storage/` — should fail because Migrate() has wrong signature.

## Step 3 — Implement

1. Add `github.com/golang-migrate/migrate/v4` to go.mod.

2. Create `migrations/001_init.postgres.up.sql` with Postgres-native types:
   - `CREATE EXTENSION IF NOT EXISTS "pgcrypto";`
   - `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
   - `BOOLEAN` for out_of_town_crew_likely
   - `TIMESTAMPTZ DEFAULT NOW()` for all timestamp columns
   - `JSONB` for raw_data
   - `BIGINT` for project_value
   - All column names identical to the SQLite schema

3. Create `migrations/001_init.postgres.down.sql` — DROP TABLE in reverse FK order.

4. Update `Migrate(db *sql.DB, dsn string) error` in `internal/storage/db.go`:
   - Postgres path (DriverName(dsn) == "pgx"): use golang-migrate to run files from `migrations/` directory
   - SQLite path: keep the existing inline schema string approach unchanged

5. Update callers: in `cmd/server/main.go`, pass `dsn` as second argument to `Migrate`.

## Constraints
- Do NOT change leads.go or raw.go yet — that is Part C
- SQLite path must work unchanged with `:memory:` and a file path
- The Postgres migration must be idempotent (IF NOT EXISTS everywhere)

## Done when
- `go test -tags integration ./internal/storage/` passes all four tests
- `go test ./internal/storage/` (no tag) passes (SQLite path)
- `\d leads` in psql shows UUID, BOOLEAN, TIMESTAMPTZ, JSONB
```

---

## Part C — Storage Layer Fixes

```
You are working on groupscout (module: github.com/alvindcastro/groupscout, Go 1.26).
Follow TDD: write the failing tests first, then implement until they pass.
Parts A and B are complete. Postgres schema exists and migrations run.

## Problems in the current storage layer

### 1. internal/storage/leads.go — boolToInt() and integer scanning
```go
// Insert passes an int instead of bool:
boolToInt(l.OutOfTownCrewLikely)   // Postgres BOOLEAN rejects this

// ListNew and ListForDigest scan as int then convert:
var oot int
rows.Scan(..., &oot, ...)
l.OutOfTownCrewLikely = oot == 1  // works in SQLite, breaks in Postgres

// Helper at bottom of file:
func boolToInt(b bool) int { if b { return 1 }; return 0 }
```

### 2. Both files — SQLite ? placeholders
```go
// leads.go: VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
// raw.go:   VALUES (?, ?, ?, ?, ?, ?)
// Postgres requires $1, $2, $3... not ?
```

## Step 1 — Write these test files first (they must FAIL before you touch production code)

Create `internal/storage/leads_integration_test.go`:

```go
//go:build integration

package storage

import (
    "context"
    "os"
    "testing"
    "time"
)

func newTestDB(t *testing.T) (*sql.DB, string) {
    t.Helper()
    dsn := os.Getenv("TEST_POSTGRES_URL")
    if dsn == "" {
        t.Skip("TEST_POSTGRES_URL not set")
    }
    db, err := Open(dsn)
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    if err := Migrate(db, dsn); err != nil {
        t.Fatalf("Migrate: %v", err)
    }
    t.Cleanup(func() {
        db.Exec("DELETE FROM leads")
        db.Exec("DELETE FROM raw_projects")
        db.Close()
    })
    return db, dsn
}

func TestLeadStore_Insert_and_ListNew(t *testing.T) {
    db, _ := newTestDB(t)
    store := NewLeadStore(db)
    ctx := context.Background()

    lead := &Lead{
        Source:                  "richmond_permits",
        Title:                   "Test Warehouse — 1234 No. 3 Road",
        Location:                "Richmond, BC",
        ProjectValue:            5_000_000,
        GeneralContractor:       "PCL Construction",
        ProjectType:             "industrial",
        EstimatedCrewSize:       80,
        EstimatedDurationMonths: 6,
        OutOfTownCrewLikely:     true,
        PriorityScore:           9,
        PriorityReason:          "Large industrial near YVR",
        Status:                  "new",
    }

    if err := store.Insert(ctx, lead); err != nil {
        t.Fatalf("Insert: %v", err)
    }
    if lead.ID == "" {
        t.Error("ID should be populated after Insert")
    }

    leads, err := store.ListNew(ctx)
    if err != nil {
        t.Fatalf("ListNew: %v", err)
    }
    if len(leads) == 0 {
        t.Fatal("expected at least one lead")
    }

    got := leads[0]
    if got.OutOfTownCrewLikely != true {
        t.Errorf("OutOfTownCrewLikely = %v, want true (bool round-trip failed)", got.OutOfTownCrewLikely)
    }
    if got.ProjectValue != 5_000_000 {
        t.Errorf("ProjectValue = %d, want 5000000", got.ProjectValue)
    }
    if got.PriorityScore != 9 {
        t.Errorf("PriorityScore = %d, want 9", got.PriorityScore)
    }
}

func TestLeadStore_bool_false_roundtrip(t *testing.T) {
    db, _ := newTestDB(t)
    store := NewLeadStore(db)
    ctx := context.Background()

    lead := &Lead{
        Source:              "test",
        Title:               "Local renovation",
        OutOfTownCrewLikely: false,
        Status:              "new",
    }
    store.Insert(ctx, lead)

    leads, _ := store.ListNew(ctx)
    for _, l := range leads {
        if l.Title == "Local renovation" && l.OutOfTownCrewLikely != false {
            t.Errorf("OutOfTownCrewLikely = %v, want false", l.OutOfTownCrewLikely)
        }
    }
}

func TestLeadStore_UpdateStatus(t *testing.T) {
    db, _ := newTestDB(t)
    store := NewLeadStore(db)
    ctx := context.Background()

    lead := &Lead{Source: "test", Title: "Status test", Status: "new"}
    store.Insert(ctx, lead)

    if err := store.UpdateStatus(ctx, lead.ID, "contacted"); err != nil {
        t.Fatalf("UpdateStatus: %v", err)
    }
    // Should not appear in ListNew (status != 'new')
    leads, _ := store.ListNew(ctx)
    for _, l := range leads {
        if l.ID == lead.ID {
            t.Error("lead with status 'contacted' should not appear in ListNew")
        }
    }
}
```

Create `internal/storage/raw_integration_test.go`:

```go
//go:build integration

package storage

import (
    "context"
    "testing"
    "time"

    "github.com/alvindcastro/groupscout/internal/collector"
)

func TestRawProjectStore_Insert_and_ExistsByHash(t *testing.T) {
    db, _ := newTestDB(t)
    store := NewRawProjectStore(db)
    ctx := context.Background()

    p := &collector.RawProject{
        Source:     "richmond_permits",
        ExternalID: "BP2026-001",
        Title:      "Test Permit",
        IssuedAt:   time.Now(),
        RawData:    map[string]any{"key": "value"},
    }
    p.Hash = HashProject(p.Source, p.ExternalID, p.Title, p.IssuedAt)

    if err := store.Insert(ctx, p); err != nil {
        t.Fatalf("Insert: %v", err)
    }

    exists, err := store.ExistsByHash(ctx, p.Hash)
    if err != nil {
        t.Fatalf("ExistsByHash: %v", err)
    }
    if !exists {
        t.Error("ExistsByHash = false, want true after Insert")
    }

    // Insert same hash again — must not error (ON CONFLICT DO NOTHING)
    if err := store.Insert(ctx, p); err != nil {
        t.Errorf("second Insert of same hash errored: %v", err)
    }
}
```

## Step 2 — Confirm tests fail

`go test -tags integration ./internal/storage/` — fails with placeholder or boolean errors.

## Step 3 — Implement

Update `internal/storage/leads.go`:
- Remove `boolToInt()` helper
- `Insert()`: pass `l.OutOfTownCrewLikely` (bool) directly; replace `?` with `$1, $2, ...` (21 params)
- `ListNew()`: scan `out_of_town_crew_likely` directly into `&l.OutOfTownCrewLikely`; remove `var oot int`; replace `?` in WHERE clause with `$1`
- `ListForDigest()`: same bool fix; replace `?` with `$1`
- `UpdateStatus()`: replace `?` with `$1, $2, $3`
- Rename struct `sqliteLeadStore` → `sqlLeadStore`

Update `internal/storage/raw.go`:
- `Insert()`: replace all `?` with `$1, $2, $3, $4, $5, $6`
- `ExistsByHash()`: replace `?` with `$1`
- Rename struct `sqliteRawStore` → `sqlRawStore`

## Constraints
- Keep all interface signatures identical (LeadStore, RawProjectStore)
- Keep NewUUID() exactly as-is — do NOT switch to gen_random_uuid() in SQL
- One code path for both drivers — no build tags in production files

## Done when
- `go test -tags integration ./internal/storage/` — all integration tests pass
- `go test ./internal/storage/` — unit tests pass
- Full pipeline against Postgres: `DATABASE_URL=postgres://... go run ./cmd/server/ --run-once`
```

---

## Part D — pgvector

```
You are working on groupscout (module: github.com/alvindcastro/groupscout, Go 1.26).
Follow TDD: write the failing tests first, then implement until they pass.
Parts A, B, C are complete. Postgres storage layer is working.

## What we are building

An EmbeddingStore interface with two implementations:
- PostgresEmbeddingStore: uses pgvector <=> cosine distance (Postgres only)
- InMemoryEmbeddingStore: loads all embeddings from SQLite, computes cosine similarity in Go

The interface is used later by the enrichment pipeline (Phase 16 RAG work).
We are only building the storage layer here — no enricher.go wiring yet.

## Step 1 — Write these test files first (they must FAIL before any implementation)

Create `internal/storage/embeddings_test.go` (unit tests, no Docker needed):

```go
package storage

import "testing"

func TestCosineSimilarity(t *testing.T) {
    tests := []struct {
        name    string
        a, b    []float32
        wantMin float32
        wantMax float32
    }{
        {
            name:    "identical vectors",
            a:       []float32{1, 0, 0},
            b:       []float32{1, 0, 0},
            wantMin: 0.999,
            wantMax: 1.001,
        },
        {
            name:    "orthogonal vectors",
            a:       []float32{1, 0, 0},
            b:       []float32{0, 1, 0},
            wantMin: -0.001,
            wantMax: 0.001,
        },
        {
            name:    "opposite vectors",
            a:       []float32{1, 0, 0},
            b:       []float32{-1, 0, 0},
            wantMin: -1.001,
            wantMax: -0.999,
        },
        {
            name:    "zero vector returns 0",
            a:       []float32{0, 0, 0},
            b:       []float32{1, 0, 0},
            wantMin: -0.001,
            wantMax: 0.001,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := cosineSimilarity(tt.a, tt.b)
            if got < tt.wantMin || got > tt.wantMax {
                t.Errorf("cosineSimilarity = %f, want [%f, %f]", got, tt.wantMin, tt.wantMax)
            }
        })
    }
}

func TestInMemoryEmbeddingStore_Similar(t *testing.T) {
    db, err := Open(":memory:")
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    defer db.Close()
    Migrate(db, ":memory:")

    store := NewEmbeddingStore(db, ":memory:")
    ctx := context.Background()

    // Seed three leads with known embeddings
    // vec1 and vec2 are similar; vec3 is different
    vec1 := []float32{1.0, 0.0, 0.0}
    vec2 := []float32{0.9, 0.1, 0.0}
    vec3 := []float32{0.0, 0.0, 1.0}

    store.Save(ctx, "lead-1", "test-model", vec1)
    store.Save(ctx, "lead-2", "test-model", vec2)
    store.Save(ctx, "lead-3", "test-model", vec3)

    // Query similar to vec1 — should return lead-1 and lead-2 first
    ids, err := store.Similar(ctx, vec1, 2)
    if err != nil {
        t.Fatalf("Similar: %v", err)
    }
    if len(ids) != 2 {
        t.Fatalf("Similar returned %d results, want 2", len(ids))
    }
    if ids[0] != "lead-1" {
        t.Errorf("top result = %q, want lead-1", ids[0])
    }
    if ids[1] != "lead-2" {
        t.Errorf("second result = %q, want lead-2", ids[1])
    }
}
```

Create `internal/storage/embeddings_integration_test.go`:

```go
//go:build integration

package storage

import (
    "context"
    "os"
    "testing"
)

func TestPostgresEmbeddingStore_SaveAndSimilar(t *testing.T) {
    dsn := os.Getenv("TEST_POSTGRES_URL")
    if dsn == "" {
        t.Skip("TEST_POSTGRES_URL not set")
    }
    db, _ := Open(dsn)
    defer db.Close()
    Migrate(db, dsn)

    store := NewEmbeddingStore(db, dsn)
    ctx := context.Background()

    // Insert a lead to satisfy the FK constraint
    leadStore := NewLeadStore(db)
    lead := &Lead{Source: "test", Title: "pgvector test lead", Status: "new"}
    leadStore.Insert(ctx, lead)
    defer db.Exec("DELETE FROM lead_embeddings WHERE lead_id = $1", lead.ID)
    defer db.Exec("DELETE FROM leads WHERE id = $1", lead.ID)

    vec := make([]float32, 512)
    vec[0] = 1.0 // simple non-zero vector

    if err := store.Save(ctx, lead.ID, "voyage-3-lite", vec); err != nil {
        t.Fatalf("Save: %v", err)
    }

    ids, err := store.Similar(ctx, vec, 1)
    if err != nil {
        t.Fatalf("Similar: %v", err)
    }
    if len(ids) == 0 {
        t.Fatal("Similar returned no results")
    }
    if ids[0] != lead.ID {
        t.Errorf("Similar[0] = %q, want %q", ids[0], lead.ID)
    }
}

func TestPostgresEmbeddingStore_index_used(t *testing.T) {
    dsn := os.Getenv("TEST_POSTGRES_URL")
    if dsn == "" {
        t.Skip("TEST_POSTGRES_URL not set")
    }
    db, _ := Open(dsn)
    defer db.Close()
    Migrate(db, dsn)

    var plan string
    vec := make([]float32, 512)
    // pgvector-go encodes the vector; we just check the query plan mentions the index
    // This is a smoke test — the index name contains "ivfflat"
    db.QueryRow(`EXPLAIN SELECT lead_id FROM lead_embeddings
        ORDER BY embedding <=> $1 LIMIT 3`, "["+strings.Repeat("0,", 511)+"0]",
    ).Scan(&plan)
    // Just verify the query runs without error — index usage depends on data volume
    t.Logf("query plan: %s", plan)
}
```

## Step 2 — Confirm tests fail

`go test ./internal/storage/` — fails with "undefined: cosineSimilarity", "undefined: EmbeddingStore".

## Step 3 — Implement

1. Add to go.mod: `go get github.com/pgvector/pgvector-go`

2. Create `migrations/003_pgvector.up.sql`:
```sql
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
```

3. Create `migrations/003_pgvector.down.sql` — DROP TABLE lead_embeddings; DROP EXTENSION vector;

4. Create `internal/storage/embeddings.go` with:
   - `EmbeddingStore` interface (Save + Similar)
   - `NewEmbeddingStore(db, dsn)` factory
   - `postgresEmbeddingStore` — uses pgvector-go to encode []float32; queries with <=>
   - `inMemoryEmbeddingStore` — SQLite fallback table + Go cosineSimilarity
   - unexported `cosineSimilarity(a, b []float32) float32` helper

5. Update `Migrate()` in db.go to also run migration 003 for Postgres.
   (SQLite: create the `lead_embeddings_sqlite` table inline.)

## Constraints
- `inMemoryEmbeddingStore` stores embeddings in a separate SQLite table, not in-process memory,
  so they survive restarts
- Do NOT wire into enricher.go yet — that is Phase 16 RAG work
- Keep the 512-dimension size consistent with voyage-3-lite output

## Done when
- `go test ./internal/storage/` — all unit tests pass (cosineSimilarity + in-memory store)
- `go test -tags integration ./internal/storage/` — all integration tests pass
```

---

## Part E — SQLite → Postgres Data Migration Script

```
You are working on groupscout (module: github.com/alvindcastro/groupscout, Go 1.26).
Follow TDD: write the failing tests first, then implement until they pass.
Parts A–D are complete.

## What we are building

A one-shot CLI script at scripts/migrate_to_postgres/main.go that copies rows from
an existing groupscout.db (SQLite) to a Postgres database.

Key type differences to handle:
- out_of_town_crew_likely: SQLite stores as INTEGER (0/1) → read as int, pass as bool to Postgres
- id columns: TEXT UUID strings are valid in Postgres UUID columns — no conversion needed
- raw_data: SQLite TEXT (JSON string) → pass as-is; Postgres JSONB accepts JSON strings directly
- Timestamps: SQLite stores as TEXT — parse with time.Parse RFC3339, write as time.Time

## Step 1 — Write this test file first (it must FAIL before implementation)

Create `scripts/migrate_to_postgres/migrate_test.go`:

```go
package main

import "testing"

func TestParseArgs_defaults(t *testing.T) {
    args, err := parseArgs([]string{
        "--sqlite", "groupscout.db",
        "--postgres", "postgres://localhost/groupscout",
    })
    if err != nil {
        t.Fatalf("parseArgs: %v", err)
    }
    if args.sqliteDSN != "groupscout.db" {
        t.Errorf("sqliteDSN = %q, want groupscout.db", args.sqliteDSN)
    }
    if args.postgresDSN != "postgres://localhost/groupscout" {
        t.Errorf("postgresDSN wrong")
    }
    if args.dryRun != false {
        t.Error("dryRun should default to false")
    }
}

func TestParseArgs_dry_run(t *testing.T) {
    args, _ := parseArgs([]string{
        "--sqlite", "x.db",
        "--postgres", "postgres://localhost/x",
        "--dry-run",
    })
    if !args.dryRun {
        t.Error("dryRun should be true when flag set")
    }
}

func TestParseArgs_missing_required(t *testing.T) {
    _, err := parseArgs([]string{"--sqlite", "x.db"})
    if err == nil {
        t.Error("expected error when --postgres is missing")
    }
}
```

## Step 2 — Confirm tests fail

`go test ./scripts/migrate_to_postgres/` — fails because main.go does not exist yet.

## Step 3 — Implement

Create `scripts/migrate_to_postgres/main.go`:

```
Usage:
  migrate_to_postgres --sqlite groupscout.db --postgres "postgres://groupscout:groupscout@localhost:5432/groupscout" [--dry-run]
```

The script must:
1. Define `type args struct { sqliteDSN, postgresDSN string; dryRun bool }` and `parseArgs([]string) (args, error)`
2. Open both DBs using `database/sql` (same drivers in go.mod)
3. Migrate in FK order: raw_projects → leads → outreach_log
4. For each table: SELECT all from SQLite, INSERT into Postgres with `ON CONFLICT (id) DO NOTHING`
5. Handle out_of_town_crew_likely: scan as int, pass as bool
6. Print: "Migrated N raw_projects", "Migrated N leads", "Migrated N outreach_log entries"
7. Print row count comparison between SQLite and Postgres for each table
8. --dry-run: print counts from SQLite only, write nothing to Postgres

Batch size: 100 rows per INSERT for efficiency.
Do NOT import internal packages — copy the minimum SQL inline.

## Constraints
- Script must be idempotent (safe to run multiple times)
- Do NOT use ORM or migration framework — plain database/sql only

## Done when
- `go test ./scripts/migrate_to_postgres/` passes (parseArgs unit tests)
- Manual run against real data: row counts match between SQLite and Postgres
- Re-running produces identical output (idempotent)
```

---

## Part F — Productionize

```
You are working on groupscout (module: github.com/alvindcastro/groupscout, Go 1.26).
Parts A–E are complete. This part is config and documentation cleanup.
There is minimal new code, so tests here are smoke/integration checks.

## Step 1 — Write this integration test first

Create `internal/storage/integration_smoke_test.go`:

```go
//go:build integration

package storage

import (
    "context"
    "os"
    "testing"
)

// TestFullStack_postgres runs the minimum pipeline operations against Postgres
// to confirm the production config works end to end.
func TestFullStack_postgres(t *testing.T) {
    dsn := os.Getenv("TEST_POSTGRES_URL")
    if dsn == "" {
        t.Skip("TEST_POSTGRES_URL not set")
    }

    db, err := Open(dsn)
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    defer db.Close()

    if err := Migrate(db, dsn); err != nil {
        t.Fatalf("Migrate: %v", err)
    }

    ctx := context.Background()
    leadStore := NewLeadStore(db)
    rawStore := NewRawProjectStore(db)
    embStore := NewEmbeddingStore(db, dsn)

    // Raw project
    p := &collector.RawProject{
        Source: "smoke_test", Title: "Smoke test permit",
        ExternalID: "SMOKE-001", IssuedAt: time.Now(),
        RawData: map[string]any{"test": true},
    }
    p.Hash = HashProject(p.Source, p.ExternalID, p.Title, p.IssuedAt)
    if err := rawStore.Insert(ctx, p); err != nil {
        t.Errorf("rawStore.Insert: %v", err)
    }

    // Lead
    lead := &Lead{
        RawProjectID: p.Hash, Source: "smoke_test",
        Title: "Smoke test lead", OutOfTownCrewLikely: true,
        PriorityScore: 8, Status: "new",
    }
    if err := leadStore.Insert(ctx, lead); err != nil {
        t.Errorf("leadStore.Insert: %v", err)
    }

    // Embedding
    vec := make([]float32, 512)
    vec[0] = 1.0
    if err := embStore.Save(ctx, lead.ID, "test", vec); err != nil {
        t.Errorf("embStore.Save: %v", err)
    }

    // Cleanup
    db.Exec("DELETE FROM lead_embeddings WHERE lead_id = $1", lead.ID)
    db.Exec("DELETE FROM leads WHERE source = 'smoke_test'")
    db.Exec("DELETE FROM raw_projects WHERE source = 'smoke_test'")
}
```

## Step 2 — Confirm test passes (it should, given Parts A–D are done)

`go test -tags integration ./internal/storage/ -run TestFullStack_postgres`

## Step 3 — Config + docs

1. Read the current `.env.example`. Update it:
   - Postgres DATABASE_URL as the primary (uncommented) example
   - SQLite as a commented fallback

2. Read the current `docker-compose.yml`. Update it:
   - `groupscout` service: `depends_on: postgres: condition: service_healthy`
   - `groupscout` environment: `DATABASE_URL=postgres://groupscout:groupscout@postgres:5432/groupscout`
   - Remove any SQLite file volume mount from the app service

3. Read the current `docs/SETUP.md`. Add a **"Postgres Setup"** section before the SQLite section:
```
## Postgres Setup (recommended)
1. `docker compose up postgres -d` and wait for healthy: `docker compose ps`
2. Set DATABASE_URL=postgres://groupscout:groupscout@localhost:5432/groupscout in .env
3. `go run ./cmd/server/ --run-once` — migrations run automatically on first boot

## SQLite (local dev only)
DATABASE_URL=groupscout.db — no Docker required.
```

## Done when
- `go test -tags integration ./internal/storage/ -run TestFullStack` passes
- `docker compose up -d` → all services healthy
- `DATABASE_URL=postgres://... go run ./cmd/server/ --run-once` → Slack receives a digest
```

---

## Running All Tests

```bash
# Unit tests only (no Docker)
go test ./...

# Integration tests (requires: docker compose up postgres -d)
TEST_POSTGRES_URL=postgres://groupscout:groupscout@localhost:5432/groupscout \
  go test -tags integration ./...

# Single part
TEST_POSTGRES_URL=postgres://... go test -tags integration ./internal/storage/ -v -run TestLeadStore
```

## Tips for Using These Prompts

- **Attach the actual files** when a prompt says "current file" — paste the full content alongside the prompt.
- **Red first:** paste the test step into your AI, confirm the error, then paste the implement step.
- **Parts are hard-dependencies** — do not skip or reorder.
- **If a test is flaky:** check placeholder style (`$N` vs `?`) and boolean handling first — those are the two most common failure modes in Part C.
