# PROMPTS_PHASE27.md — Input Audit & Verification Trail

> Copy-paste prompts for each part of Phase 27.
> Parts must be done in order: A → B → C → D → E.
>
> **Goal:** Implement a system to store and track all raw inputs (PDFs, API responses, etc.) to allow for verification of the lead enrichment and scoring process.
>
> **TDD conventions for this phase:**
> - Strict TDD: Write failing tests before implementation.
> - Commit each failing test before fixing it.
> - Use the standard `testing` package.
> - Mock external dependencies where necessary.

---

## Part A — Storage Architecture (Metadata & Payload)

```
Context:
- We need to store raw data (PDFs, JSON, etc.) used for lead enrichment.
- Database: SQLite (dev) / Postgres (prod).
- Metadata: hash (SHA256), payload_type, source_url, collector_name.
- Payload: stored as BLOB/bytea in the database for now.

Task A1 — Database Migration:
  1. Create migrations/006_audit_trail.up.sql:
     CREATE TABLE raw_inputs (
         id UUID PRIMARY KEY,
         hash TEXT NOT NULL,
         payload_type TEXT NOT NULL,
         payload BYTEA NOT NULL,
         source_url TEXT,
         collector_name TEXT,
         created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
     );
     CREATE INDEX idx_raw_inputs_hash ON raw_inputs(hash);
     ALTER TABLE leads ADD COLUMN raw_input_id UUID REFERENCES raw_inputs(id);

  2. Create migrations/006_audit_trail.down.sql:
     ALTER TABLE leads DROP COLUMN raw_input_id;
     DROP TABLE raw_inputs;

Verify A1:
  Run migrations: make migrate-up
  Check table structure in DB.
```

```
Task A2 — Storage Implementation (TDD):
  1. Create internal/storage/audit_test.go:
     - TestStoreRawInput: Assert that saving a RawInput returns a UUID and can be retrieved by ID.
     - TestGetByHash: Assert that searching by hash returns the correct record.
     - TestDuplicateHash: (Optional) Assert behavior when storing same hash (should we update or return existing?).
  2. Implement internal/storage/audit.go to satisfy the tests.
     - Use database/sql with pgx/sqlite drivers.

Verify A2:
  go test ./internal/storage/audit_test.go -v
```

---

## Part B — Collector Interface & Integration

```
Context:
- Collectors currently return RawProject which contains basic metadata.
- We need them to also return the actual raw data they fetched.

Task B1 — Update Collector Interface (TDD):
  1. Update internal/collector/collector.go:
     - Add RawData []byte and RawType string to RawProject struct.
  2. Update existing tests in internal/collector/ to assert RawData is populated.
  3. Implement changes in:
     - internal/collector/richmond.go (PDF content)
     - internal/collector/delta.go (PDF content)
     - internal/collector/bcbid.go (RSS XML)
     - internal/collector/news.go (API JSON)

Verify B1:
  go test ./internal/collector/...
```

---

## Part C — Enrichment Linking ✅ COMPLETE

```
Context:
- Enricher orchestrates the flow: Collect -> Score -> Enrich -> Store.
- Raw data is stored via AuditStore BEFORE enrichment so it is never lost.
- The resulting Lead always carries the audit trail ID.

Implementation (all in enricher.go → processProject):
  1. AuditStore is injected into Enricher via NewEnricher().
  2. processProject() calls auditStore.ExistsByHash() for dedup (NOT rawStore).
  3. If not a duplicate, auditStore.Store() is called with:
       - Hash         ← p.Hash
       - PayloadType  ← p.RawType
       - Payload      ← p.RawData
       - SourceURL    ← p.SourceURL
       - CollectorName ← p.Source
  4. The returned UUID is passed as rawInputID to toLeadRecord() and to
     skipped-lead records, ensuring EVERY lead (enriched or skipped) has
     a valid raw_input_id FK.

Tests (internal/enrichment/enricher_test.go):
  - TestToLeadRecord_rawInputIDPropagated
      Confirms toLeadRecord() sets Lead.RawInputID from the passed UUID string.
  - TestEnricher_processProject_storesAllAuditFields
      Confirms all five fields (Hash, PayloadType, Payload, SourceURL,
      CollectorName) from RawProject reach AuditStore.Store().
  - TestEnricher_processProject_callsStoreAndLinksID
      Confirms enriched lead.RawInputID == the UUID returned by auditStore.Store().
  - TestEnricher_processProject_linksIDToSkippedLead
      Confirms skipped leads also have RawInputID set.
  - TestEnricher_processProject_dedupCheck
      Confirms dedup uses auditStore.ExistsByHash().

Integration tests (internal/storage/leads_integration_test.go):
  - TestLeadStore_WithRawInputID
      Stores a RawInput, inserts a Lead with raw_input_id FK, reads it back,
      asserts the field round-trips correctly.

Verify:
  go test ./internal/enrichment/...
  go test ./internal/storage/... -tags integration  # requires TEST_POSTGRES_URL
```

---

## Part D — Verification API & CLI

```
Context:
- Users need to access the raw data for verification.

Task D1 — API Endpoint (TDD):
  1. Create test for GET /leads/{id}/raw in cmd/server/handler_test.go:
     - Assert 200 OK and correct Content-Type/Body for a valid lead.
     - Assert 404 for missing lead or missing raw input.
  2. Implement handler in cmd/server/main.go.

Task D2 — CLI Command:
  1. Implement `groupscout audit <lead_id>` in cmd/cli/:
     - Fetch lead, then fetch raw input.
     - Output metadata to stdout.
     - Provide --save flag to dump payload.

Verify D:
  curl -i http://localhost:8080/leads/<uuid>/raw
  groupscout audit <uuid> --save test.pdf
```

---

## Part E — Cleanup & Optimization

```
Context:
- Raw inputs can grow large. We need retention policies.

Task E1 — Cleanup Worker:
  1. Add a background worker that deletes raw_inputs older than X days (e.g., 30 days).
  2. Ensure it doesn't break leads that still reference them (or handle orphaned references).

Verify E1:
  Manual test: insert old record, run cleanup, assert it's gone.
```
