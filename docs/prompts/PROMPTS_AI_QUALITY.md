# AI Quality Task Prompts — groupscout

Use these prompts for the AI quality EvalOps phases. Every code prompt requires strict TDD: write failing tests, observe red, implement the smallest green change, run the broader suite, then update docs.

## Reusable code task prompt

> You are working in `groupscout`. Implement the requested AI quality behavior in Go using strict TDD. Start by reading `docs/planning/AI_QUALITY_EVALOPS.md`, `docs/guides/TDD_AI_QUALITY.md`, existing package tests, and the target files. Add the smallest failing test first. Run the narrow test and confirm it fails for the expected reason. Implement only enough code to pass. Run the narrow test, then `go test ./...`. Refactor only while green. Do not call live websites, Slack, Resend, Sentry, Anthropic, Ollama, or external APIs from unit tests. Use fixtures, fake clients, and `httptest`. In the final task note, include files changed, red evidence, green evidence, broader test command, behavior added, and residual risk.

## GQ2-T01 Code: eval case loader

> Implement `internal/evalops` case loading for `groupscout` quality cases. Start with failing tests for: one valid JSONL file; malformed JSON; missing `id`; duplicate IDs; unknown source type; unsupported risk level; and missing expected outcome. Confirm red. Implement a loader that returns typed cases and aggregated validation errors with line numbers. Acceptance requires deterministic ordering, no global state, and no live filesystem assumptions beyond `t.TempDir()` fixtures.

## GQ2-T02 Code: lead relevance scorer

> Implement the deterministic lead relevance scorer. Add failing table-driven tests for: high-value commercial permit kept; low-value residential renovation dropped; duplicate raw hash dropped; film production kept with unknown project value; consumer event dropped; and missing source evidence marked warning. Confirm red. Implement score-band logic only from the fixture contract. Acceptance requires explainable reasons, no LLM calls, and clear critical vs warning severity.

## GQ2-T03 Code: enrichment completeness scorer

> Implement enrichment completeness scoring for AI-enriched leads. Add failing tests for required fields: `estimated_room_nights`, `project_duration`, `lodging_need`, `rationale`, `source_evidence`, and `confidence`. Add tests for unsupported room-night certainty, missing unknown labels, and contradictory evidence. Confirm red. Implement minimal scorer logic. Acceptance requires unknowns to be allowed when evidence is insufficient, but confident unsupported claims must fail critical.

## GQ2-T04 Code: outreach safety scorer

> Implement outreach draft safety checks. Add failing tests for: source-backed personalized sentence; fabricated relationship claim; overly aggressive CTA; unredacted personal email/phone; missing human-review status; and safe fallback when no outreach should be drafted. Confirm red. Implement deterministic checks using string/rule helpers first. Acceptance requires outreach drafts to be marked `draft_for_review` and unsafe drafts to block release gates.

## GQ2-T05 Code: alert threshold scorer

> Implement quality checks for `alertd` stranded-passenger or disruption alert decisions. Add failing tests for: multi-signal severe disruption triggers priority; weather-only noise does not; stale data fails closed; missing NOTAM feed marks degraded not critical; threshold boundary behavior; and Slack alert text includes source signals. Confirm red. Implement the smallest scorer. Acceptance requires no false priority alert on a single weak signal.

## GQ2-T06 Code: Markdown and JUnit reports

> Implement report generation for eval results. Start with golden-file tests for JSON summary, Markdown summary, and JUnit XML. Include cases for all pass, warning-only, and critical failure. Confirm red. Implement deterministic report sorting by severity, case ID, and scorer name. Acceptance requires stable output across runs, no secrets in reports, and actionable failure messages.

## GQ3-T01 Code: local eval target HTTP server

> Implement `cmd/evaltarget` or an internal handler that Promptfoo can call over HTTP. Add `httptest` tests for valid request, invalid JSON, unknown case ID, timeout, scorer failure, trace ID propagation, and safe response shape. Confirm red. Implement the handler using interfaces for project execution and scoring. Acceptance requires `output`, `trace_id`, `scores`, `sources`, and `actions` fields in the response, with no raw secrets.

## GQ3-T03 Code: release gate command

> Implement a release gate that reads eval report JSON and threshold YAML. Add failing tests for: critical failure exits non-zero; warning-only exits zero by default; configurable warning-as-error; missing report file; malformed thresholds; and summary text. Confirm red. Implement command logic behind a testable package function. Acceptance requires deterministic exit codes and GitHub Actions-friendly output.

## GQ4-T01 Code: trace event model

> Implement safe trace event structs for collector, enrichment, LLM, notification, and alert stages. Add failing tests for redaction of API keys, webhook URLs, email addresses, phone numbers, raw PII, and oversized raw source text. Confirm red. Implement redaction and attribute mapping. Acceptance requires event payloads to be safe for logs, Loki, Sentry breadcrumbs, and OpenTelemetry attributes.

## GQ4-T02 Code: metric counters and histograms

> Implement metrics for AI quality and pipeline quality. Add failing tests using a fresh Prometheus registry for collector failures, enrichment skipped count, LLM error count, alert decision count, latency histogram observation, and cost/token counters. Confirm red. Implement metrics without using the default global registry in tests. Acceptance requires no duplicate metric registration panics.

## GQ4-T03 Code: review sample writer

> Implement a redacted production review sample writer. Add failing tests for writing JSONL, appending multiple samples, rejecting unredacted sensitive fields, rotating or limiting oversized samples, and preserving trace/case metadata. Confirm red. Implement a small writer that can be disabled by config. Acceptance requires samples to be usable as draft eval cases without leaking secrets.

## GQ5-T02 Code: sample-to-case helper

> Implement a helper that converts redacted review samples into draft eval cases. Add failing tests for required TODO fields, preservation of trace IDs, case ID generation, duplicate handling, and explicit `review_required=true`. Confirm red. Implement conversion without auto-marking cases as release-blocking. Acceptance requires a human to review expected outcomes before the case can enter the golden set.
