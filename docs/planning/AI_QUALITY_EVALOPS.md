# AI Quality EvalOps — groupscout

This plan extends `groupscout` with a Go-first quality layer for AI enrichment, lead scoring, outreach drafts, source collection, and airport disruption alerts.

The intended open-source integration path is:

```text
Go eval harness + project fixtures
    → Promptfoo HTTP provider for offline evals and CI gates
    → OpenTelemetry/Prometheus/Langfuse-compatible traces and production review samples
```

## Scope

### In scope

- Golden lead cases for construction, film, event, bid, infrastructure, and airport disruption scenarios.
- Deterministic Go scorers for lead relevance, enrichment completeness, unsupported claims, source evidence, outreach safety, Slack formatting, and alert thresholds.
- Promptfoo-compatible local HTTP target for regression tests.
- Release gate that blocks critical regressions before a collector, LLM provider, prompt, or scoring rule change ships.
- Runtime monitoring events that can be sent to existing Prometheus/Grafana/Sentry/Loki/OpenTelemetry paths.

### Out of scope for the first pass

- Replacing the existing enrichment pipeline.
- Live scraping inside unit tests.
- Live Slack, SendGrid, Sentry, Anthropic, Ollama, or external API calls from tests.
- Fully automated outreach sending without review gates.

## Non-negotiable code rule

Every code task must follow [TDD_AI_QUALITY.md](../guides/TDD_AI_QUALITY.md): failing test first, confirm red, implement the smallest green change, run `go test ./...`, then refactor while green.

## Phase overview

| Phase | Outcome |
|---|---|
| GQ0 — EvalOps product frame | Quality goals, metrics, and release gates are explicit. |
| GQ1 — Golden lead and alert fixtures | Synthetic fixture set covers expected and adversarial lead scenarios. |
| GQ2 — Go eval case loader and scorers | Deterministic quality checks run without live services. |
| GQ3 — Promptfoo target and CI gate | Offline evals run through a local Go HTTP target and fail regressions. |
| GQ4 — Runtime telemetry and review sampling | Production-like runs emit safe trace, metric, and review events. |
| GQ5 — Feedback loop | Production failures become new golden cases. |

---

## GQ0 — EvalOps product frame

**Goal:** decide what "good AI output" means for `groupscout` before writing code.

### Quality dimensions

| Dimension | Measurable signal | Critical failure examples |
|---|---|---|
| Lead relevance | Expected keep/drop decision, score band, source type, project value, project category, duplicate hash behavior. | Unsupported high-priority keep, high-value lead dropped without reason, duplicate released as a new lead. |
| Source evidence | Every retained lead, enrichment claim, alert signal, and outreach personalization links to source evidence or an explicit unknown. | Fabricated source name, missing evidence for a priority decision, source text contradiction ignored. |
| Enrichment completeness | Required fields are present or explicitly unknown: `estimated_room_nights`, `project_duration`, `lodging_need`, `rationale`, `source_evidence`, and `confidence`. | Confident unsupported room-night estimate, required field omitted, contradiction converted into a confident claim. |
| Room-night rationale | Estimate uses a documented range, confidence, lodging-need reason, and evidence from project scale, duration, or crew/event signals. | Precise unsupported number, rationale conflicts with source facts, unknown evidence hidden as high confidence. |
| Outreach safety | Drafts stay `draft_for_review`, avoid fabricated relationships, avoid unredacted personal contact data, and use a safe CTA. | Send-ready outreach without review, personal email/phone leakage, fabricated relationship or urgency claim. |
| Alert correctness | Priority alerts require multi-signal disruption evidence; stale data fails closed; weak single signals stay non-priority or degraded. | False priority alert from weather-only noise, stale feed treated as fresh, missing critical signal hidden as normal. |
| Latency | Eval runs and runtime stages emit duration metrics with timeout behavior tested around external-call boundaries. | Timeout path blocks the pipeline indefinitely or skips failure reporting. |
| Cost | LLM/provider call counts, token estimates, and enrichment skip counts are measured where available. | Offline eval unexpectedly calls a live provider or expensive path bypasses configured gates. |

### Critical gates

Release gates fail closed for:

- unsupported high-priority lead claims,
- fabricated source names or missing required evidence,
- unsafe outreach drafts or any auto-send path without review,
- secret, webhook, API key, email, phone, or raw PII leakage,
- false airport disruption priority alerts,
- stale or insufficient alert data that produces a priority decision,
- unexpected live provider calls in deterministic evals.

Warning-level findings can pass by default only when the affected behavior is explicitly degraded, review-only, or configured as non-blocking.

### Deterministic vs live eval split

Deterministic evals are the default for local development and CI. They use JSONL fixtures, fake LLM/search/tool clients, `httptest`, and local report generation. These evals are release-blocking.

Live evals are optional calibration runs. They require an explicit command, explicit credentials, and a human-reviewed output path. Live evals never run from unit tests and do not block local development unless a maintainer intentionally wires them into a manual release job.

- [x] **GQ0-T01 — Define quality dimensions**  
  **Type:** Documentation  
  **Files:** `docs/planning/AI_QUALITY_EVALOPS.md`, [docs/prompts/AI_DATA_STRATEGY.md](../prompts/AI_DATA_STRATEGY.md)  
  **Done when:** lead relevance, source evidence, enrichment completeness, room-night rationale, outreach safety, alert correctness, latency, and cost are defined.
- [x] **GQ0-T02 — Define critical failure gates**  
  **Type:** Documentation  
  **Done when:** critical failures include unsupported high-priority lead claims, fabricated source names, unsafe outreach, secret leakage, and false airport disruption priority alerts.
- [x] **GQ0-T03 — Define deterministic vs live eval split**  
  **Type:** Documentation  
  **Done when:** unit/regression evals use fixtures and fake LLMs; optional live evals are manually triggered and never block local development unless explicitly requested.
- [x] **GQ0-T04 — Add README documentation links**  
  **Type:** Documentation  
  **Done when:** README documentation section links this file, prompts file, and TDD policy.

### GQ0 phase gate

- [x] All quality dimensions have measurable signals.
- [x] Critical gates fail closed.
- [x] No code was added in this phase.

---

## GQ1 — Golden lead and alert fixtures

**Goal:** create reusable fixture cases before implementing scorers.

- [x] **GQ1-T01 — Create lead eval case schema**  
  **Type:** Documentation  
  **Target files:** `docs/evals/groupscout-case-schema.md`, `data/evals/groupscout/*.jsonl`  
  **Done when:** schema includes source, raw text, expected keep/drop decision, expected score band, expected enrichment fields, expected evidence, and forbidden claims.
- [x] **GQ1-T02 — Add construction and permit cases**  
  **Type:** Documentation  
  **Done when:** cases include large commercial project, low-value residential renovation, duplicate permit, malformed PDF text, and missing value.
- [x] **GQ1-T03 — Add film/event/bid/infrastructure cases**  
  **Type:** Documentation  
  **Done when:** cases cover Creative BC, VCC, Eventbrite, CivicInfo/BCBid, and infrastructure announcements.
- [x] **GQ1-T04 — Add airport disruption alert cases**  
  **Type:** Documentation  
  **Done when:** cases cover YVR delay spike, weather-only noise, NOTAM-only noise, multi-signal severe disruption, and stale-data fail-closed.
- [x] **GQ1-T05 — Add adversarial and privacy cases**  
  **Type:** Documentation  
  **Done when:** cases include prompt injection inside source text, PII in raw source, conflicting source dates, and suspicious contact data.

### GQ1 phase gate

- [x] At least 20 total cases are specified.
- [x] Every case has expected pass/fail behavior.
- [x] No fixture requires a live website or provider.

---

## GQ2 — Go eval case loader and deterministic scorers

**Goal:** implement local repeatable checks using Go.

- [ ] **GQ2-T01 — Code: eval case loader**  
  **Files:** `internal/evalops/cases.go`, `internal/evalops/cases_test.go`  
  **Prompt:** [GQ2-T01](../prompts/PROMPTS_AI_QUALITY.md#gq2-t01-code-eval-case-loader)  
  **Done when:** valid JSONL loads, malformed cases fail with useful errors, duplicate IDs fail, and `go test ./internal/evalops` passes.
- [ ] **GQ2-T02 — Code: lead relevance scorer**  
  **Files:** `internal/evalops/lead_scorer.go`, `internal/evalops/lead_scorer_test.go`  
  **Prompt:** [GQ2-T02](../prompts/PROMPTS_AI_QUALITY.md#gq2-t02-code-lead-relevance-scorer)  
  **Done when:** keep/drop decisions, score bands, and duplicate behavior are tested.
- [ ] **GQ2-T03 — Code: enrichment completeness scorer**  
  **Prompt:** [GQ2-T03](../prompts/PROMPTS_AI_QUALITY.md#gq2-t03-code-enrichment-completeness-scorer)  
  **Done when:** required fields, rationale, source evidence, room-night estimate range, and unknown handling are tested.
- [ ] **GQ2-T04 — Code: outreach safety scorer**  
  **Prompt:** [GQ2-T04](../prompts/PROMPTS_AI_QUALITY.md#gq2-t04-code-outreach-safety-scorer)  
  **Done when:** fabricated claims, aggressive tone, missing review status, and PII leakage fail.
- [ ] **GQ2-T05 — Code: alert threshold scorer**  
  **Prompt:** [GQ2-T05](../prompts/PROMPTS_AI_QUALITY.md#gq2-t05-code-alert-threshold-scorer)  
  **Done when:** SPS/priority alert behavior is deterministic and stale or partial data fails safe.
- [ ] **GQ2-T06 — Code: Markdown and JUnit reports**  
  **Prompt:** [GQ2-T06](../prompts/PROMPTS_AI_QUALITY.md#gq2-t06-code-markdown-and-junit-reports)  
  **Done when:** reports are deterministic, include critical failures, and can be uploaded by CI.

### GQ2 phase gate

- [ ] `go test ./internal/evalops` passes.
- [ ] `go test ./...` passes.
- [ ] Every scorer has happy-path, invalid-input, and critical-failure tests.

---

## GQ3 — Promptfoo target and CI gate

**Goal:** let Promptfoo run `groupscout` quality checks without JS glue.

- [ ] **GQ3-T01 — Code: local eval target HTTP server**  
  **Files:** `cmd/evaltarget/main.go`, `internal/evalops/target.go`  
  **Prompt:** [GQ3-T01](../prompts/PROMPTS_AI_QUALITY.md#gq3-t01-code-local-eval-target-http-server)  
  **Done when:** `httptest` proves request validation, response shape, trace ID, timeout, and scorer output.
- [ ] **GQ3-T02 — Add Promptfoo YAML configs**  
  **Type:** Documentation/config  
  **Files:** `evals/promptfoo/groupscout.yaml`  
  **Done when:** Promptfoo can call `http://localhost:18080/eval/run` and pass case variables.
- [ ] **GQ3-T03 — Code: release gate command**  
  **Files:** `cmd/evalgate/main.go`, `internal/evalops/gate.go`  
  **Prompt:** [GQ3-T03](../prompts/PROMPTS_AI_QUALITY.md#gq3-t03-code-release-gate-command)  
  **Done when:** critical failures produce non-zero exit and warnings do not block unless configured.
- [ ] **GQ3-T04 — Add Makefile/CI commands**  
  **Type:** Documentation/config  
  **Done when:** `make eval-quality` and `make eval-gate` are documented.

### GQ3 phase gate

- [ ] A local eval run produces JSON, Markdown, and JUnit outputs.
- [ ] A seeded critical failure blocks the gate.
- [ ] CI instructions avoid live provider keys by default.

---

## GQ4 — Runtime telemetry and review sampling

**Goal:** monitor production-like runs and turn failures into future tests.

- [ ] **GQ4-T01 — Code: trace event model**  
  **Prompt:** [GQ4-T01](../prompts/PROMPTS_AI_QUALITY.md#gq4-t01-code-trace-event-model)  
  **Done when:** collector, enrichment, LLM, notification, and alert spans/events have redaction tests.
- [ ] **GQ4-T02 — Code: metric counters and histograms**  
  **Prompt:** [GQ4-T02](../prompts/PROMPTS_AI_QUALITY.md#gq4-t02-code-metric-counters-and-histograms)  
  **Done when:** tests cover collector failures, skipped enrichments, LLM errors, alert decisions, latency, and cost counters.
- [ ] **GQ4-T03 — Code: review sample writer**  
  **Prompt:** [GQ4-T03](../prompts/PROMPTS_AI_QUALITY.md#gq4-t03-code-review-sample-writer)  
  **Done when:** sampled failures write redacted JSONL with enough context to create a regression case.
- [ ] **GQ4-T04 — Add dashboard/alert checklist**  
  **Type:** Documentation  
  **Done when:** Grafana/Sentry/Loki alert candidates are documented for hallucination, drift, cost, collector failure, and webhook failure.

### GQ4 phase gate

- [ ] No traces or samples leak secrets or PII.
- [ ] Monitoring events include enough IDs to debug a failed lead without storing raw sensitive source data.

---

## GQ5 — Feedback loop

**Goal:** convert real failures into permanent regression coverage.

- [ ] **GQ5-T01 — Define production-to-case workflow**  
  **Type:** Documentation  
  **Done when:** a sampled issue can become a JSONL case with expected behavior and owner notes.
- [ ] **GQ5-T02 — Code: sample-to-case helper**  
  **Prompt:** [GQ5-T02](../prompts/PROMPTS_AI_QUALITY.md#gq5-t02-code-sample-to-case-helper)  
  **Done when:** redacted samples generate draft cases, never auto-commit them, and include TODO fields for human review.
- [ ] **GQ5-T03 — Add monthly calibration checklist**  
  **Type:** Documentation  
  **Done when:** thresholds, prompt versions, model versions, and source drift are reviewed on a schedule.

### GQ5 phase gate

- [ ] Every production issue class has a path to a regression test.
- [ ] New cases must be reviewed before becoming release-blocking.
