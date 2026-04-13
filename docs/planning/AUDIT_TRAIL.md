# Phase 27 — Input Audit & Verification Trail

**Goal:** Implement a system to store and track all raw inputs (PDFs, API responses, etc.) to allow for verification of the lead enrichment and scoring process. This builds trust by providing a direct link between an LLM-generated lead and the original source material.

## Brainstormed Tasks

### Part A — Storage Architecture
- [x] Define the `RawInput` schema for storing data (API response body, PDF content, source URL).
    - [x] Columns: `id` (UUID), `hash` (SHA256 of payload), `payload_type` (e.g. 'pdf', 'json', 'html'), `payload` (bytea/blob), `source_url` (text), `collector_name` (text), `created_at` (timestamp).
- [x] Choose storage backend: SQLite/Postgres for metadata + Disk/S3-compatible for larger payloads (PDFs).
    - [x] Start with database storage for simplicity, then move to disk if blob size becomes an issue.
- [x] Create database migration `006_audit_trail.up.sql` to add `raw_inputs` table.
- [x] Implement `internal/storage/audit.go` for managing raw input records.
    - [x] `Store(ctx, raw RawInput) (uuid.UUID, error)`
    - [x] `GetByID(ctx, id uuid.UUID) (*RawInput, error)`
    - [x] `GetByHash(ctx, hash string) (*RawInput, error)` — for de-duplication.

### Part B — Collector Integration
- [ ] Update `Collector` interface to return raw data alongside `RawProject`.
    - [ ] `RawProject` should have a `RawData []byte` field and a `RawType string` field.
- [ ] Richmond: Store the raw PDF content and source URL for each run.
- [ ] Delta: Store the raw PDF content and source URL.
- [ ] BC Bid: Store the raw RSS XML and individual item descriptions.
- [ ] News/Creative BC/VCC: Store the raw API/RSS responses.

### Part C — Enrichment Linking
- [x] Link `raw_input_id` to `leads` table to provide a direct link to the source data.
    - [x] Update migration `006_audit_trail.up.sql` to add `raw_input_id` to `leads` table.
- [ ] Update `enricher.go` to store raw input before calling LLM.
    - [ ] Check if hash exists first to avoid redundant storage.
    - [ ] Associate the `raw_input_id` with the generated `Lead` object.
- [ ] Ensure `ExistsByHash` still works but now points to the original raw input.

### Part D — Verification & Access
- [ ] Create `GET /leads/{id}/raw` endpoint to retrieve the raw input used for a specific lead.
    - [ ] Should return the raw payload with the correct `Content-Type`.
- [ ] Add a CLI command `groupscout audit <lead_id>` to dump the raw input for manual verification.
    - [ ] Flag `--save <path>` to save payload to a file.
    - [ ] Flag `--meta` to show source URL, collector, and fetch time.
- [ ] Update Slack notifications to include a link to the raw data (if exposed via internal reference).

### Part E — Retention & Privacy
- [ ] Implement a cleanup worker to purge raw inputs older than X days.
- [ ] Add `PII_STRIP` option to remove sensitive info before storage if required.
- [ ] Implement hashing logic to ensure we don't store identical payloads multiple times.

## Implementation Strategy — Agent Choreography

For a multi-agent execution of this phase, refer to [AGENT_CHOREOGRAPHY_PHASE27.md](./AGENT_CHOREOGRAPHY_PHASE27.md).

## Strict TDD Implementation Path
1.  **Test Storage Layer:** Write `audit_test.go` before `audit.go`. Assert `Store` and `GetByID` work.
2.  **Test Collector Changes:** Mock the collector to ensure it returns raw data.
3.  **Test Enrichment Integration:** Write a test where an enriched lead must have a valid `raw_input_id` pointing to the stored raw data.
4.  **Test API/CLI:** Assert `GET /leads/{id}/raw` returns the exact bytes stored.
