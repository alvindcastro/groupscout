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

- [ ] Write failing route/layout tests first.
- [ ] Build the operator shell with left navigation, top utility bar, main content, and right-rail capacity.
- [ ] Add routes for Today, Leads, Lead Detail, Verification Queue, Pipeline, and Settings.
- [ ] Test responsive collapse for navigation and detail/evidence rail.
- [ ] Verify build and component tests.

## Phase 33 - Mocked Lead Inbox

- [ ] Write failing tests for table rendering, sorting, searching, filtering, loading, empty, and error states.
- [ ] Use mocked fixture data only until `/api/leads` exists.
- [ ] Show status, score, source, owner, verification, created date, and timing columns.
- [ ] Test keyboard navigation and accessible filter controls.
- [ ] Keep the API boundary replaceable with generated or typed clients.

## Phase 34 - Lead Detail And Evidence Review

- [ ] Write failing tests for summary, source evidence, AI rationale, raw audit link, outreach activity, and status actions.
- [ ] Keep source evidence adjacent to AI claims.
- [ ] Represent missing evidence, weak confidence, and contradictory data states.
- [ ] Keep correction controls disabled or mocked until backend correction storage exists.
- [ ] Verify no raw payload body loads into the default detail response.

## Phase 35 - UI API Contracts

- [ ] Write failing storage tests for filtered lead listing.
- [ ] Write failing HTTP tests for `GET /api/leads` and `GET /api/leads/{id}`.
- [ ] Write failing tests for `PATCH /api/leads/{id}` with allowed fields and rejected unsafe fields.
- [ ] Write failing tests for authenticated `GET /api/leads/{id}/raw`.
- [ ] Update OpenAPI only after the expected failing tests define the contract.
- [ ] Generate or maintain frontend types from the contract.

## Phase 36 - Outreach And Lead State Actions

- [ ] Write failing storage tests for `outreach_log` insert/list behavior.
- [ ] Write failing HTTP tests for `GET /api/leads/{id}/outreach`.
- [ ] Write failing HTTP tests for `POST /api/leads/{id}/outreach`.
- [ ] Test claim, dismiss, snooze, flag, contacted, won, lost, no-response, and reopen transitions.
- [ ] Reject invalid transitions with specific errors.
- [ ] Keep verification status separable from commercial workflow status unless a design decision says otherwise.

## Phase 37 - Pipeline Runs, Stats, And System Health

- [ ] Write failing tests proving `POST /api/pipeline/runs` does not block on the full pipeline.
- [ ] Add run persistence or document a dev-only in-memory tracker before implementation.
- [ ] Write failing tests for `GET /api/pipeline/runs` ordering, status filtering, counts, and errors.
- [ ] Write failing tests for `GET /api/stats` by status, source, score band, owner, and week where schema supports it.
- [ ] Write failing tests for `GET /api/system` healthy and degraded states.
- [ ] Do not parse Prometheus `/metrics` directly in browser UI.

## Phase 38 - Docker Runtime And End-To-End Smoke

- [ ] Write failing smoke checks before changing runtime wiring.
- [ ] Verify the production UI container serves `/`, `/healthz`, and static assets.
- [ ] Verify same-origin `/api/*` proxy behavior.
- [ ] Distinguish backend `404` from proxy `502` in smoke expectations.
- [ ] Add Playwright smoke for lead inbox, lead detail, and responsive navigation.
- [ ] Verify static assets contain no `API_TOKEN`, database URL, Slack token, email token, LLM key, or session secret.
