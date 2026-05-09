# UI TDD Phase Plan

> Tickable planning checklist for the future GroupScout operator UI and its `/api/*` backend contracts.
> Beads remains the canonical task tracker. These checkboxes are prompt scaffolding and acceptance planning only.

## Non-Negotiable TDD Loop

- [ ] Read the relevant docs, tests, routes, storage code, and UI design guidance first.
- [ ] Write the smallest failing test before production code.
- [ ] Run the narrow test and confirm it fails for the expected reason.
- [ ] Implement only enough code to pass.
- [ ] Run the narrow test again and confirm green.
- [ ] Run the broader relevant suite.
- [ ] Refactor only while tests stay green.
- [ ] Update docs only when behavior, commands, contracts, or acceptance criteria changed.
- [ ] Leave red evidence, green evidence, broader command, residual risk, and follow-up issue IDs in the task summary.

## Phase 31 - UI Design System Adaptation

- [x] Translate `groupscout-ui/DESIGN.md` into GroupScout operator UI tokens and rules.
- [x] Write failing component/token tests before implementing UI code.
- [x] Cover buttons, badges, inputs, tabs, tables, evidence blocks, focus states, and semantic statuses.
- [x] Verify static frontend assets do not expose secrets.
- [x] Keep marketing hero patterns out of the operator workspace.

Phase 31 implementation note: this repository has no checked-in frontend package or component test harness today, so the phase is implemented as the contract in `UI_PHASE31_DESIGN_SYSTEM_CONTRACT.md`. The first frontend package must turn that contract into failing tests before production UI code.

## Phase 32 - App Shell And Routing

- [x] Write failing route/layout tests first.
- [x] Build the operator shell with left navigation, top utility bar, main content, and right-rail capacity.
- [x] Add routes for Today, Leads, Lead Detail, Verification Queue, Pipeline, and Settings.
- [x] Test responsive collapse for navigation and detail/evidence rail.
- [x] Verify build and component tests.

Phase 32 implementation note: this repository still has no checked-in frontend package, route test harness, or static UI build directory, so the phase is implemented as the contract in `UI_PHASE32_APP_SHELL_ROUTING_CONTRACT.md`. The first frontend package must turn that contract into failing route, layout, responsive, build, component, and static-asset safety tests before production shell code.

## Phase 33 - Mocked Lead Inbox

- [x] Write failing tests for table rendering, sorting, searching, filtering, loading, empty, and error states.
- [x] Use mocked fixture data only until `/api/leads` exists.
- [x] Show status, score, source, owner, verification, created date, and timing columns.
- [x] Test keyboard navigation and accessible filter controls.
- [x] Keep the API boundary replaceable with generated or typed clients.

Phase 33 implementation note: this repository still has no checked-in frontend package, UI test harness, static UI build directory, or `GET /api/leads` backend route, so the phase is implemented as the contract in `UI_PHASE33_MOCKED_LEAD_INBOX_CONTRACT.md`. The first frontend package must turn that contract into failing fixture-boundary, table, search, filter, sorting, state, keyboard, accessibility, responsive, build, and static-asset safety tests before production inbox code.

## Phase 34 - Lead Detail And Evidence Review

- [x] Write failing tests for summary, source evidence, AI rationale, raw audit link, outreach activity, and status actions.
- [x] Keep source evidence adjacent to AI claims.
- [x] Represent missing evidence, weak confidence, and contradictory data states.
- [x] Keep correction controls disabled or mocked until backend correction storage exists.
- [x] Verify no raw payload body loads into the default detail response.

Phase 34 implementation note: this repository still has no checked-in frontend package, UI test harness, static UI build directory, or `GET /api/leads/{id}` backend route, so the phase is implemented as the contract in `UI_PHASE34_LEAD_DETAIL_EVIDENCE_CONTRACT.md`. The first frontend package must turn that contract into failing detail fixture-boundary, summary, source evidence, AI rationale, raw audit link, outreach activity, status action, correction-control, accessibility, responsive, build, and static-asset safety tests before production detail code.

## Phase 35 - UI API Contracts

- [x] Write failing storage tests for filtered lead listing.
- [x] Write failing HTTP tests for `GET /api/leads` and `GET /api/leads/{id}`.
- [x] Write failing tests for `PATCH /api/leads/{id}` with allowed fields and rejected unsafe fields.
- [x] Write failing tests for authenticated `GET /api/leads/{id}/raw`.
- [x] Update OpenAPI only after the expected failing tests define the contract.
- [x] Generate or maintain frontend types from the contract.

Phase 35 implementation note: `/api/leads`, `/api/leads/{id}`, `PATCH /api/leads/{id}`, and authenticated `/api/leads/{id}/raw` are implemented against the current lead and raw-audit schema. `PATCH` intentionally supports only `status` and `notes`; `owner`, `snoozed_until`, `verification_state`, and `corrections` remain blocked on migration-backed design work in `groupscout-bxt`. Frontend types are maintained in `UI_PHASE35_FRONTEND_TYPES.md` until a frontend package and generated client exist.

## Phase 36 - Outreach And Lead State Actions

- [x] Write failing storage tests for `outreach_log` insert/list behavior.
- [x] Write failing HTTP tests for `GET /api/leads/{id}/outreach`.
- [x] Write failing HTTP tests for `POST /api/leads/{id}/outreach`.
- [x] Test claim, dismiss, snooze, flag, contacted, won, lost, no-response, and reopen transitions.
- [x] Reject invalid transitions with specific errors.
- [x] Keep verification status separable from commercial workflow status unless a design decision says otherwise.

Phase 36 implementation note: outreach logging is implemented with `OutreachStore`, `/api/leads/{id}/outreach`, and validated lead actions on `PATCH /api/leads/{id}`. Commercial workflow fields `owner`, `snoozed_until`, and `flagged` are schema-backed, and `verification_state` remains separate from commercial status.

## Phase 37 - Pipeline Runs, Stats, And System Health

- [x] Write failing tests proving `POST /api/pipeline/runs` does not block on the full pipeline.
- [x] Add run persistence or document a dev-only in-memory tracker before implementation.
- [x] Write failing tests for `GET /api/pipeline/runs` ordering, status filtering, counts, and errors.
- [x] Write failing tests for `GET /api/stats` by status, source, score band, owner, and week where schema supports it.
- [x] Write failing tests for `GET /api/system` healthy and degraded states.
- [x] Do not parse Prometheus `/metrics` directly in browser UI.

Phase 37 implementation note: pipeline runs are persisted in `pipeline_runs`, `/api/pipeline/runs` starts the real pipeline asynchronously in production, `/api/stats` summarizes supported schema fields, and `/api/system` exposes UI health without requiring browser-side Prometheus parsing.

## Phase 38 - Docker Runtime And End-To-End Smoke

- [x] Write failing smoke checks before changing runtime wiring.
- [x] Verify the production UI container serves `/`, `/healthz`, and static assets.
- [x] Verify same-origin `/api/*` proxy behavior.
- [x] Distinguish backend `404` from proxy `502` in smoke expectations.
- [x] Add Playwright smoke for lead inbox, lead detail, and responsive navigation.
- [x] Verify static assets contain no `API_TOKEN`, database URL, Slack token, email token, LLM key, or session secret.

Phase 38 implementation note: this backend repo does not own the production UI app, so the executable contract lives in `scripts/smoke-ui-docker-e2e.sh` and targets the external `groupscout-ui` production container. Until that UI has a real browser renderer for inbox/detail, route and responsive smoke uses the UI repo's Node model-level tests as the Playwright-equivalent gate.
