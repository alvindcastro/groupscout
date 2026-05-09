# GroupScout Eval Case Schema

This schema defines the synthetic golden cases used by GQ1 and loaded by the GQ2 eval harness.

The fixture files live in `data/evals/groupscout/*.jsonl`. Each line is one complete JSON object. Cases are synthetic, stable, and must not require live websites, live providers, credentials, or private source material.

## Files

- `data/evals/groupscout/lead_cases.jsonl` covers construction, permit, film, event, bid, and infrastructure lead quality.
- `data/evals/groupscout/alert_cases.jsonl` covers YVR disruption alert quality.
- `data/evals/groupscout/adversarial_cases.jsonl` covers prompt injection, PII, conflicting evidence, suspicious contact data, and secret-like text.

## Required Top-Level Fields

| Field | Type | Required | Notes |
|---|---:|---:|---|
| `id` | string | yes | Stable unique ID. Use lowercase prefixes such as `lead-`, `alert-`, or `adv-`. |
| `case_type` | string | yes | One of `lead` or `alert`. |
| `category` | string | yes | Domain grouping, for example `construction_permit`, `airport_disruption`, or `privacy`. |
| `risk_level` | string | yes | One of `critical`, `warning`, or `info`. Critical mismatches should fail the release gate in GQ3. |
| `source` | object | yes | Synthetic source metadata. |
| `raw` | object | yes | Synthetic source payload normalized enough for deterministic evaluation. |
| `expected` | object | yes | Expected decision, score range, evidence, and failure conditions. |
| `notes` | string | no | Human-readable intent for maintainers. |

## `source`

| Field | Type | Required | Notes |
|---|---:|---:|---|
| `system` | string | yes | Existing source name such as `richmond_permits`, `delta_permits`, `creativebc`, `vcc`, `eventbrite`, `bcbid`, `announcements`, or `yvr_disruption`. |
| `type` | string | yes | Payload type such as `permit_pdf`, `html`, `rss`, `news_release`, or `multi_signal_snapshot`. |
| `fixture_url` | string | yes | Non-live fixture URL under `https://fixtures.groupscout.local/...`. |
| `collected_at` | string | yes | RFC3339 timestamp for deterministic freshness checks. |

## `raw`

Lead cases use:

| Field | Type | Required | Notes |
|---|---:|---:|---|
| `title` | string | yes | Project title or source headline. |
| `text` | string | yes | Synthetic raw text. Include enough facts for evidence checks. |
| `issued_at` | string | yes | Source date as `YYYY-MM-DD`. |
| `location` | string | yes | City or address-level synthetic location. |
| `value_cad` | number or null | yes | CAD project value when known. Use null when the source omits value. |
| `metadata` | object | yes | Source-specific structured facts, such as duration, attendees, duplicate hash, or source subtype. |

Alert cases use:

| Field | Type | Required | Notes |
|---|---:|---:|---|
| `airport_code` | string | yes | `YVR` for GQ1 fixtures. |
| `observed_at` | string | yes | RFC3339 timestamp for the signal snapshot. |
| `text` | string | yes | Synthetic operator-readable summary. |
| `signals` | object | yes | Cancellation, weather, NOTAM, runway, and freshness inputs. |

## `expected`

| Field | Type | Required | Notes |
|---|---:|---:|---|
| `eval_result` | string | yes | Expected fixture result when the system behaves correctly. GQ1 uses `pass`. |
| `decision` | string | yes | Lead decisions: `keep`, `drop`, or `needs_review`. Alert decisions: `ignore`, `watch`, `soft_alert`, `hard_alert`, or `fail_closed`. |
| `release_blocking` | boolean | yes | Whether a mismatch should block release. |
| `severity_on_mismatch` | string | yes | One of `critical`, `warning`, or `info`. |
| `score_band` | object | yes | `min` and `max` inclusive priority score or SPS range. |
| `enrichment` | object or null | yes | Required lead enrichment expectations. Use null for alert-only cases. |
| `alert` | object or null | yes | Required alert expectations. Use null for lead-only cases. |
| `evidence` | array | yes | Evidence claims that must be supported by raw source text or explicit unknowns. |
| `forbidden_claims` | array | yes | Claims the system must not invent or preserve from hostile source text. |
| `privacy` | object | yes | Redaction and forbidden-pattern expectations. |
| `critical_failure_if` | array | yes | Concrete mismatch examples that should fail closed. |

Lead `enrichment` should include:

- `project_type`
- `estimated_room_nights` with `min`, `max`, and `unknown_allowed`
- `project_duration_months` with `min`, `max`, and `unknown_allowed`
- `lodging_need`
- `confidence`

Alert `alert` should include:

- `state`
- `priority`
- `stale_data`
- `required_signals`

## Loader Rules For GQ2

- Reject malformed JSONL with line-numbered errors.
- Reject missing `id`, `case_type`, `source`, `raw`, or `expected`.
- Reject duplicate IDs across all loaded files.
- Reject unknown `case_type`, `risk_level`, `decision`, or source `system`.
- Preserve file order for deterministic reports.
- Never dereference `fixture_url`; it is an identifier, not an HTTP target.
- Treat any unredacted secret-like, webhook-like, email, phone, or raw PII output as a critical mismatch when the case lists it under `privacy`.

## Draft Cases For GQ5

GQ5 draft cases are generated from redacted `ReviewSample` entries before they become golden fixtures. Drafts use the same case shape plus these review-only top-level fields:

| Field | Type | Required | Notes |
|---|---:|---:|---|
| `trace_id` | string | yes | Production trace ID copied from the redacted review sample. Also preserved under `raw.metadata.trace_id`. |
| `review_required` | boolean | yes | Always `true` for generated drafts. |

Draft generation rules:

- Draft IDs use a `draft-` prefix and deterministic duplicate suffixes such as `-2` or `-3`.
- `expected.release_blocking` is always `false` until a human reviewer promotes the case.
- Human-owned expected fields contain `TODO_REVIEW_*` placeholders for the decision, eval result, evidence, forbidden claims, privacy requirements, critical failure examples, and domain-specific expectations.
- `LoadCases` rejects the TODO decision, so drafts cannot enter the golden set until expected outcomes are reviewed and replaced with schema-valid values.
- Keep draft JSONL outside `data/evals/groupscout/` until review is complete. Promoted cases require fixture-count and changelog updates.
