# PROMPTS_PHASE34_UI.md - Lead Detail And Evidence Review TDD Prompts

> Copy-paste strict TDD prompts for GroupScout UI Phase 34.
> Use these only when implementation starts. No production code should be written from this planning pass.
>
> Parts must be done in order unless a beads issue explicitly scopes a smaller slice.
> Every part follows Red -> Green -> Refactor.

## TDD Rules For Every Prompt

- [ ] Read `docs/planning/ui/UI_STRATEGY.md`, `docs/planning/ui/UI_DESIGN_SYSTEM.md`, `docs/planning/ui/UI_PHASE31_DESIGN_SYSTEM_CONTRACT.md`, `docs/planning/ui/UI_PHASE32_APP_SHELL_ROUTING_CONTRACT.md`, `docs/planning/ui/UI_PHASE33_MOCKED_LEAD_INBOX_CONTRACT.md`, `docs/planning/ui/UI_PHASE34_LEAD_DETAIL_EVIDENCE_CONTRACT.md`, `docs/planning/ui/UI_API_ENDPOINTS.md`, and `docs/planning/ui/UI_TDD_PHASE_PLAN.md`.
- [ ] Read existing route, layout, component, fixture, client, and build tests near the target files before adding new tests.
- [ ] Write the smallest failing test first.
- [ ] Run the narrow test and capture the expected red failure.
- [ ] Implement only enough production code to pass.
- [ ] Run the narrow test again and capture green.
- [ ] Run the broader relevant suite.
- [ ] Refactor only while green.
- [ ] Do not call live Slack, email, LLM, browser, CRM, public websites, paid APIs, `POST /run`, `POST /n8n/webhook`, `GET /leads/{id}/raw`, or direct database connections from unit or component tests.
- [ ] Use fakes, fixtures, component test harnesses, and deterministic route data.
- [ ] Keep browser-visible code free of `API_TOKEN`, database URLs, Slack/email/LLM provider keys, session secrets, and real `.env` values.

## Phase 34 - Lead Detail And Evidence Review

```text
Task 34-A1 - Detail fixture boundary and route loading

Context:
- Phase 34 implements `/leads/:leadId` as mocked lead detail and evidence review.
- The contract is `docs/planning/ui/UI_PHASE34_LEAD_DETAIL_EVIDENCE_CONTRACT.md`.
- `GET /api/leads/{id}` is planned for Phase 35 and does not exist in this repository today.
- The implemented `GET /leads/{id}/raw` returns raw bytes and must not be called by default detail loading.
- If the repository still has no checked-in frontend package, create only the minimum tested harness required for fixture-boundary tests to fail first.

TDD steps:
1. Write failing tests for a typed detail fixture shape:
   - lead id, title, source, status, score, priority reason, rationale, timing, notes, created/updated timestamps
   - source URL or explicit missing-source state
   - audit metadata with has_raw, raw link, payload type, source URL, collector, and collected timestamp
   - verification state
   - evidence items
   - outreach summary
   - activity events
2. Write failing tests proving `/leads/:leadId` passes the route ID into a data adapter such as `getLeadDetail(id)`.
3. Write failing tests for loading, populated, not-found, and adapter-error states.
4. Write failing tests proving the default detail data excludes raw payload body fields such as payload, raw_body, and raw_data.
5. Write failing tests proving no live `/api/leads/{id}`, `/leads/{id}/raw`, `/run`, `/n8n/webhook`, direct SQL, or browser-visible server token is required.
6. Run the narrow tests and capture the expected red failures.
7. Stop before implementation until the red failures are observed.

Acceptance:
- Detail fixtures cover source-backed, missing-evidence, weak-confidence, contradiction, no-outreach, and disabled-correction examples.
- The data boundary can later be replaced by a generated or typed `/api/leads/{id}` client.
- Browser-visible code does not expose server-only config.
```

```text
Task 34-A2 - Summary, source evidence, and AI rationale

Context:
- Lead detail is where operators decide whether model output is source-backed enough to act on.
- Source evidence must sit adjacent to AI claims.

TDD steps:
1. Re-run the failing fixture-boundary tests from Task 34-A1.
2. Write failing rendering tests for the summary:
   - title
   - score
   - timing
   - room-night signal
   - property fit
   - commercial status
   - created/updated timestamps
3. Write failing rendering tests for source evidence:
   - source name
   - source URL
   - raw audit link metadata
   - collector name
   - collected timestamp
4. Write failing rendering tests for AI rationale:
   - priority reason
   - rationale
   - extracted fields
   - confidence or verification state
5. Write failing tests proving important AI claims render beside supporting source evidence or an explicit missing evidence state.
6. Run the narrow tests and capture red.
7. Implement only enough detail rendering to pass.
8. Run the narrow tests until green.

Acceptance:
- Operators can compare what the model claimed with what source evidence supports it.
- Rationale is never presented as fact without nearby source context.
- The first viewport is the operator detail workspace, not a hero or landing page.
```

```text
Task 34-A3 - Evidence risk states and correction controls

Context:
- Missing evidence, weak confidence, and contradictions must be visible before operators take action.
- Backend correction storage does not exist yet.

TDD steps:
1. Write failing tests for missing evidence:
   - missing source URL
   - missing raw audit metadata
   - disabled or mocked primary action state
2. Write failing tests for weak confidence:
   - weak rationale or confidence label
   - review-needed state
   - verification state separate from commercial status
3. Write failing tests for contradictory data:
   - source value
   - AI claim
   - visible contradiction label
   - correction control capacity
4. Write failing tests proving correction controls are disabled or mocked until backend correction storage exists.
5. Write failing accessibility tests for badges, alerts, disabled controls, and focus-visible behavior.
6. Run the narrow tests and capture red.
7. Implement only enough risk-state and correction-control UI to pass.
8. Run the narrow tests and broader component suite.

Acceptance:
- Missing, weak, and contradictory evidence states are visible by default.
- Status and verification are not conflated.
- Correction controls do not imply persistence that does not exist.
```

```text
Task 34-A4 - Outreach, activity, and status actions

Context:
- `outreach_log` exists in schema, but there is no storage abstraction or HTTP API for outreach activity yet.
- Backend status updates currently accept free-form status and do not enforce a UI state machine.

TDD steps:
1. Write failing tests for fixture-backed outreach summary:
   - draft or no-draft state
   - contact fields
   - copied/sent/logged state
   - no-outreach-yet state
2. Write failing tests for fixture-backed activity:
   - status history
   - notes
   - outreach attempts
   - reviewer correction events
   - deterministic ordering
3. Write failing tests for status action capacity:
   - claim
   - dismiss
   - snooze
   - flag for verification
   - contacted
   - won
   - lost
   - no-response
   - reopen
4. Write failing tests proving mutation actions are disabled, mocked, or fixture-only until Phase 36 defines validated transitions.
5. Write failing keyboard tests for action buttons, menus, tabs/panels, activity controls, and recovery links.
6. Run the narrow tests and capture red.
7. Implement only enough outreach, activity, and action UI to pass.
8. Run the narrow tests and broader component suite.

Acceptance:
- Operators can see outreach and activity context without live backend mutations.
- Actions have accessible names and do not call live email, Slack, CRM, or mutation endpoints.
- Activity remains readable on desktop and narrow viewports.
```

```text
Task 34-A5 - Raw audit safety, responsive layout, and build verification

Context:
- Raw audit access is deliberate and separate from default detail loading.
- Responsive behavior must keep evidence and contradiction states visible.

TDD steps:
1. Write failing tests proving the default detail route does not request or include raw payload body content.
2. Write failing tests for a raw audit link that uses metadata only and can later target authenticated `GET /api/leads/{id}/raw`.
3. Write failing responsive tests for desktop, tablet, and narrow viewports.
4. Write failing tests proving no summary label, action label, evidence item, contradiction text, raw audit link, outreach item, or activity text overlaps incoherently.
5. Write failing tests proving loading, not-found, missing-evidence, weak-confidence, contradiction, and error states preserve the Phase 32 shell.
6. Run the narrow tests and capture red.
7. Implement only enough raw-link safety and responsive behavior to pass.
8. Run build, component tests, route tests, and static asset secret scan.

Acceptance:
- Raw payload bodies are never loaded by the default detail response.
- Narrow viewports keep summary, evidence, raw audit link, outreach, activity, and contradiction states readable.
- Static assets do not contain automation tokens, database URLs, Slack/email/LLM keys, session secrets, or real `.env` values.
```
