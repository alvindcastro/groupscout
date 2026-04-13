# Phase 30 — Advanced Audit & Verification

**Goal:** Transform the raw audit trail from Phase 27 into a proactive verification and quality assurance system. This phase focuses on AI-driven cross-referencing, source health monitoring, and closing the loop between extracted leads and their source material.

## 1. Brainstormed API Endpoints

### Verification & Feedback
- `POST /leads/{id}/verify` — Manually mark a lead as "Verified" or "Flagged" by a user.
    - Payload: `{ "status": "verified|flagged", "notes": "Optional user feedback", "corrections": { "field": "new_value" } }`
- `GET /leads/{id}/verification-report` — Generate an AI-driven report comparing the stored `Lead` fields against the `RawInput` payload.
    - Identifies hallucinations, missing data, or misinterpreted fields.
- `PATCH /leads/{id}/corrections` — Apply user-provided corrections to a lead while preserving the original extraction in a `raw_extraction` metadata field.

### Source Analysis & Health
- `GET /stats/extraction-accuracy` — Aggregate metrics on lead extraction accuracy, grouped by collector.
    - Metrics: Error rate, hallucination rate, field-level confidence scores.
- `GET /raw-inputs/{id}/diff` — If a URL has multiple raw input snapshots, compare the current payload with a previous one to identify what changed (e.g., permit status update).
- `GET /collectors/{name}/health` — Deep health check including extraction success rate and source structure changes (e.g., "PDF layout changed, extraction likely broken").

### Management
- `GET /audit/duplicates` — List leads derived from the same `RawInput` hash to find potential de-duplication failures.
- `DELETE /audit/raw-inputs/{id}` — Manually purge specific raw data for privacy or storage management.

## 2. Suggested Features

### Proactive AI Verification (PAV)
- After every enrichment, run a second, lightweight LLM pass (using a smaller, cheaper model like Claude 3 Haiku) to "grade" the primary model's work.
- Automatically flag leads with low verification scores for human review.

### Confidence Scoring
- Add a `confidence_score` (0.0-1.0) to every field in the `Lead` struct.
- Low-confidence fields are highlighted in Slack notifications.

### Source Drift Detection
- Detect when a source (e.g., Richmond PDF) changes its format before extraction fails.
- Compare HTML/PDF structure against a "gold standard" template.

## 3. TDD Strategy

### Test Verification Workflow
- **TestLeadVerificationUpdate**: Assert that `POST /leads/{id}/verify` correctly updates the lead's status and notes in the database.
- **TestLeadCorrectionsPreserveHistory**: Assert that applying a correction doesn't overwrite the `raw_input_id` or the ability to see the original "hallucinated" value.

### Test AI Verification Logic
- **TestVerificationReportGeneration**: Mock a scenario where a Lead has a fake field (e.g., "GeneralContractor: Mickey Mouse") and assert that the AI verification prompt identifies the mismatch with the raw PDF.
- **TestAccuracyMetricsCalculation**: Seed the DB with 10 leads (7 verified, 3 flagged) and assert `GET /stats/extraction-accuracy` returns 70% accuracy.

### Test Source Health
- **TestDriftDetection**: Provide a PDF with slightly shifted columns and assert that the health check flags a "Potential Layout Drift".

## 4. Implementation Path

### Phase 30.1 — Verification Loop
- [ ] Add `verification_status` and `verification_notes` to `leads` table.
- [ ] Implement `POST /leads/{id}/verify` endpoint.
- [ ] Add "Verify" button to Slack notification (if possible via URL) or log it via API.

### Phase 30.2 — AI Verification Agent
- [ ] Create `internal/enrichment/verifier.go`.
- [ ] Define verification prompt (compares JSON lead vs Raw payload).
- [ ] Store verification results in a new `verification_results` table linked to the lead.

### Phase 30.3 — Analytics & Reporting
- [ ] Build the `/stats/extraction-accuracy` aggregator.
- [ ] Create a CLI report `groupscout audit-report` for high-level quality overview.
