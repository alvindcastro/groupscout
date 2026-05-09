# PROMPTS_PHASE33_UI.md - Mocked Lead Inbox TDD Prompts

> Copy-paste strict TDD prompts for GroupScout UI Phase 33.
> Use these only when implementation starts. No production code should be written from this planning pass.
>
> Parts must be done in order unless a beads issue explicitly scopes a smaller slice.
> Every part follows Red -> Green -> Refactor.

## TDD Rules For Every Prompt

- [ ] Read `docs/planning/ui/UI_STRATEGY.md`, `docs/planning/ui/UI_DESIGN_SYSTEM.md`, `docs/planning/ui/UI_PHASE31_DESIGN_SYSTEM_CONTRACT.md`, `docs/planning/ui/UI_PHASE32_APP_SHELL_ROUTING_CONTRACT.md`, `docs/planning/ui/UI_PHASE33_MOCKED_LEAD_INBOX_CONTRACT.md`, `docs/planning/ui/UI_API_ENDPOINTS.md`, and `docs/planning/ui/UI_TDD_PHASE_PLAN.md`.
- [ ] Read existing route, layout, component, fixture, client, and build tests near the target files before adding new tests.
- [ ] Write the smallest failing test first.
- [ ] Run the narrow test and capture the expected red failure.
- [ ] Implement only enough production code to pass.
- [ ] Run the narrow test again and capture green.
- [ ] Run the broader relevant suite.
- [ ] Refactor only while green.
- [ ] Do not call live Slack, email, LLM, browser, CRM, public websites, paid APIs, `POST /run`, `POST /n8n/webhook`, or direct database connections from unit or component tests.
- [ ] Use fakes, fixtures, component test harnesses, and deterministic route data.
- [ ] Keep browser-visible code free of `API_TOKEN`, database URLs, Slack/email/LLM provider keys, session secrets, and real `.env` values.

## Phase 33 - Mocked Lead Inbox

```text
Task 33-A1 - Fixture boundary and typed lead summaries

Context:
- Phase 33 implements the `/leads` inbox with mocked data only.
- The contract is `docs/planning/ui/UI_PHASE33_MOCKED_LEAD_INBOX_CONTRACT.md`.
- `GET /api/leads` is planned for Phase 35 and does not exist in this repository today.
- If the repository still has no checked-in frontend package, create only the minimum tested harness required for fixture-boundary tests to fail first.

TDD steps:
1. Write failing tests for a typed lead summary fixture shape:
   - id
   - title
   - source
   - source URL or audit source URL
   - status
   - priority score
   - owner or unassigned
   - verification state
   - created date
   - suggested outreach timing
   - has_raw
2. Write failing tests for fixture response states:
   - loading
   - populated
   - empty
   - error
3. Write failing tests proving the inbox route uses a data adapter such as `listLeadSummaries(params)` instead of importing fixture arrays directly in table components.
4. Write failing tests proving no live `/api/leads`, `/run`, `/n8n/webhook`, direct SQL, or browser-visible server token is required.
5. Run the narrow tests and capture the expected red failures.
6. Stop before implementation until the red failures are observed.

Acceptance:
- Fixtures cover high-score unassigned, claimed, missing-evidence, weak-confidence, contradiction, dismissed or low-score, and non-title search examples.
- The data boundary can later be replaced by a generated or typed `/api/leads` client.
- Browser-visible code does not expose server-only config.
```

```text
Task 33-A2 - Table rendering and visual density

Context:
- The lead inbox is table-first and lives inside the Phase 32 operator shell.
- It should remain a dense operations surface, not a card grid or marketing page.

TDD steps:
1. Re-run the failing fixture-boundary tests from Task 33-A1.
2. Write failing table rendering tests for required columns:
   - score
   - lead title and compact secondary metadata
   - status
   - source
   - owner
   - verification
   - created date
   - timing
   - row actions or detail link
3. Write failing tests proving commercial status and verification state render as separate concepts.
4. Write failing tests for visible missing evidence, weak confidence, and contradiction states.
5. Write failing tests proving `/leads` renders inside the existing shell with the Leads nav selected.
6. Run the narrow tests and capture red.
7. Implement only enough table and route rendering to pass.
8. Run the narrow table and route tests until green.

Acceptance:
- Required columns render from typed fixture fields or documented defaults.
- Status and verification badges use text or shape/icon distinctions, not color alone.
- The first viewport is the operator inbox, not a hero or landing page.
```

```text
Task 33-A3 - Search and filters

Context:
- Operators need to narrow mocked lead fixtures the same way they will narrow live `/api/leads` results later.
- Search and filters should update typed params at the data boundary.

TDD steps:
1. Write failing tests for search across title, organization/contact fields where present, location, source URL, notes, and source label.
2. Write failing tests for filters:
   - status
   - source
   - owner or unassigned
   - verification state
   - minimum score
   - created date or date band
3. Write failing tests for reset behavior.
4. Write failing accessibility tests for labels, selected states, keyboard operation, and focus-visible treatment on every filter control.
5. Run the narrow tests and capture red.
6. Implement only enough search/filter behavior to pass against fixtures.
7. Run the narrow tests again and keep them green.

Acceptance:
- Search is case-insensitive and deterministic.
- Verification filters do not mutate or conflate commercial workflow status.
- Reset clears search, filters, and sort to the default mocked inbox view.
```

```text
Task 33-A4 - Sorting, keyboard navigation, and row selection

Context:
- The default inbox order should surface urgent, unowned, high-score leads first.
- Operators must be able to use the table by keyboard.

TDD steps:
1. Write failing tests for default ordering with deterministic fixtures.
2. Write failing tests for score sorting ascending and descending.
3. Write failing tests for created date sorting ascending and descending.
4. Write failing tests for sortable status, source, owner, verification, and timing columns where implemented.
5. Write failing tests for `aria-sort` or equivalent selected-sort state.
6. Write failing keyboard tests for moving through sort headers, rows, row actions, reset controls, and pagination or cursor controls when present.
7. Write failing row-selection tests for pointer and keyboard selection.
8. Run the narrow tests and capture red.
9. Implement only enough sorting and keyboard behavior to pass.
10. Run the narrow table tests and broader component suite.

Acceptance:
- Selected row state is visible and programmatically exposed.
- Sort state is not conveyed by color alone.
- Keyboard navigation reaches all interactive inbox controls in a predictable order.
```

```text
Task 33-A5 - Loading, empty, error, responsive, and build verification

Context:
- Route-level states must not collapse the app shell.
- Responsive behavior should keep filters and important lead states readable without overlap.

TDD steps:
1. Write failing tests proving loading, empty, and error states preserve:
   - primary navigation
   - top utility bar
   - main landmark
   - lead inbox heading
   - filter labels
   - table or table-state region
2. Write failing tests for the empty state and reset action.
3. Write failing tests for the error state and retry or recovery action.
4. Write failing responsive tests for desktop, tablet, and narrow viewports.
5. Write failing tests proving no route heading, filter label, button label, table header, status, verification label, or timing text overlaps incoherently.
6. Run the narrow tests and capture red.
7. Implement only enough state and responsive behavior to pass.
8. Run build, component tests, route tests, and static asset secret scan.

Acceptance:
- Loading, empty, and error states stay inside the operator shell.
- Narrow viewports keep filters, row labels, status, verification, and timing readable.
- Static assets do not contain automation tokens, database URLs, Slack/email/LLM keys, session secrets, or real `.env` values.
```
