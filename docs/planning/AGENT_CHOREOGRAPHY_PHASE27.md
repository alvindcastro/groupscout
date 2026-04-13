# Agent Choreography — Phase 27 (Input Audit & Verification Trail)

This document outlines how specialized Junie subagents should collaborate to implement the Audit Trail system described in `AUDIT_TRAIL.md`.

## 🤖 Subagent Roles & Responsibilities

| Subagent | Role in Phase 27 |
| :--- | :--- |
| `database-architect` | Define the `raw_inputs` schema and create SQL migrations. Implement `internal/storage/audit.go`. |
| `collector-specialist` | Update the `Collector` interface and individual scrapers to return raw data (PDFs, API responses). |
| `enrichment-processor` | Update `enricher.go` to store raw inputs and link them to generated `Lead` objects. |
| `api-integrator` | Implement the `GET /leads/{id}/raw` endpoint and the `groupscout audit` CLI command. |
| `tdd-tester` | Ensure strict TDD by writing tests before implementation and verifying all integration points. |
| `infrastructure-specialist`| Handle any storage-related configurations (e.g., volume mounts for local disk storage). |

## 🚀 Execution Workflow

### Step 1: Foundation (Data & Storage)
- **Primary Agent:** `database-architect`
- **Co-Agent:** `tdd-tester`
- **Tasks:**
  1. Create database migration `006_audit_trail.up.sql` to add `raw_inputs` table and `raw_input_id` to `leads`.
  2. Implement `internal/storage/audit.go` with `Store`, `GetByID`, and `GetByHash`.
- **Verification:** `tdd-tester` should verify that binary payloads can be stored and retrieved correctly without data loss.

### Step 2: Ingestion (Collector Updates)
- **Primary Agent:** `collector-specialist`
- **Co-Agent:** `tdd-tester`
- **Tasks:**
  1. Update `Collector` interface in `internal/collector/collector.go` to include raw data in the return structure.
  2. Implement raw data capture for Delta, Richmond, BC Bid, etc.
- **Verification:** `tdd-tester` should mock scrapers to ensure they correctly populate the raw data fields before they are passed downstream.

### Step 3: Integration (Enrichment Linking)
- **Primary Agent:** `enrichment-processor`
- **Co-Agent:** `database-architect`
- **Tasks:**
  1. Update `enricher.go` to use `audit.Store` before LLM processing.
  2. Link the returned `raw_input_id` to the `Lead` object.
  3. Ensure de-duplication logic (hashing) is correctly applied.
- **Verification:** `enrichment-processor` should verify that a single raw input is only stored once even if multiple leads are generated from it.

### Step 4: Interface (API & CLI)
- **Primary Agent:** `api-integrator`
- **Co-Agent:** `tdd-tester`
- **Tasks:**
  1. Add `GET /leads/{id}/raw` to `internal/api/`.
  2. Implement the `groupscout audit` CLI command in `cmd/tools/`.
  3. Update Slack notifications to include a reference to the source material.
- **Verification:** `api-integrator` should use `curl` and the CLI to confirm that the raw payload matches the original source.

### Step 5: Lifecycle (Cleanup & Privacy)
- **Primary Agent:** `database-architect`
- **Co-Agent:** `infrastructure-specialist`
- **Tasks:**
  1. Implement a periodic cleanup worker to purge old raw inputs.
  2. Add `PII_STRIP` logic if necessary to comply with privacy requirements.
- **Verification:** `database-architect` should verify that the cleanup worker only deletes records older than the retention period.

## 🔗 Related Resources
- [AUDIT_TRAIL.md](./AUDIT_TRAIL.md) — Detailed feature requirements.
- [SUBAGENTS.md](../guides/SUBAGENTS.md) — General guide on using Junie subagents.
