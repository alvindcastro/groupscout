# PROMPTS_PHASE32_UI.md - Operator App Shell And Routing TDD Prompts

> Copy-paste strict TDD prompts for GroupScout UI Phase 32.
> Use these only when implementation starts. No production code should be written from this planning pass.
>
> Parts must be done in order unless a beads issue explicitly scopes a smaller slice.
> Every part follows Red -> Green -> Refactor.

## TDD Rules For Every Prompt

- [ ] Read `docs/planning/ui/UI_STRATEGY.md`, `docs/planning/ui/UI_DESIGN_SYSTEM.md`, `docs/planning/ui/UI_PHASE31_DESIGN_SYSTEM_CONTRACT.md`, `docs/planning/ui/UI_PHASE32_APP_SHELL_ROUTING_CONTRACT.md`, `docs/planning/ui/UI_API_ENDPOINTS.md`, and `docs/planning/ui/UI_TDD_PHASE_PLAN.md`.
- [ ] Read existing route, layout, component, and build tests near the target files before adding new tests.
- [ ] Write the smallest failing test first.
- [ ] Run the narrow test and capture the expected red failure.
- [ ] Implement only enough production code to pass.
- [ ] Run the narrow test again and capture green.
- [ ] Run the broader relevant suite.
- [ ] Refactor only while green.
- [ ] Do not call live Slack, email, LLM, browser, CRM, public websites, or paid APIs from unit tests.
- [ ] Use fakes, fixtures, component test harnesses, and deterministic route data.
- [ ] Keep browser-visible code free of `API_TOKEN`, database URLs, Slack/email/LLM provider keys, session secrets, and real `.env` values.

## Phase 32 - Operator App Shell And Routes

```text
Task 32-A1 - Route and layout tests first

Context:
- Phase 32 creates the first operator workspace routing surface.
- The app shell and route contract is `docs/planning/ui/UI_PHASE32_APP_SHELL_ROUTING_CONTRACT.md`.
- The first UI screen is the operator app, not a marketing landing page.
- Tests must define route behavior and layout landmarks before shell code is implemented.
- If the repository still has no checked-in frontend package, create only the minimum tested harness required for route and component tests to fail first.

TDD steps:
1. Write failing route tests for the primary paths before creating route components:
   - `/` redirects to or renders the Today route.
   - `/today` renders Today.
   - `/leads` renders Leads.
   - `/leads/:leadId` renders Lead Detail.
   - `/verification` renders Verification Queue.
   - `/pipeline` renders Pipeline.
   - `/settings` renders Settings.
   - Unknown routes render a not-found state inside the operator shell.
2. Write failing layout tests proving every route renders predictable landmarks:
   - left navigation
   - top utility bar
   - main content region
   - optional right rail region
3. Write failing tests for route labels, page headings, selected navigation state, and accessible names.
4. Write failing tests proving route rendering does not require live API data.
5. Run the narrow route/layout tests and capture the expected red failures.
6. Stop before implementation until the red failures are observed.

Acceptance:
- Route tests fail for missing routes before production shell code exists.
- Layout tests fail for missing landmarks before layout implementation exists.
- Tests use deterministic fixtures and no network calls.
- Unknown routes stay within the operator workspace, not a blank page or marketing route.
```

```text
Task 32-A2 - Operator app shell

Context:
- Build the compact operator workspace shell after Task 32-A1 has red tests.
- Use the Phase 31 design system primitives and dense product surface language.
- The shell must support future lead evidence review without needing backend data.

TDD steps:
1. Re-run the failing route/layout tests from Task 32-A1.
2. Implement the smallest app shell that satisfies the layout contract:
   - persistent left navigation on desktop
   - top utility bar
   - main content outlet
   - right rail container that can be empty, populated, or collapsed
3. Add only minimal placeholder route content needed to make landmarks and headings pass.
4. Verify keyboard tab order reaches navigation, utility actions, main content, and right rail controls.
5. Verify selected navigation state updates when routes change.
6. Run the narrow component and route tests until green.
7. Refactor only while tests remain green.

Acceptance:
- The shell is the first screen operators see.
- The shell does not rely on API data to render.
- Navigation, top bar, main, and right rail use accessible landmarks or labels.
- No marketing hero classes, decorative orbs, atmospheric gradients, or oversized display type appear in the operator shell.
```

```text
Task 32-A3 - Today route

Context:
- Today is the default operator workspace route.
- It should provide a scan-friendly daily work surface without depending on live backend endpoints.

TDD steps:
1. Write failing tests that `/today` renders inside the operator shell.
2. Write failing tests for the Today heading, selected nav state, and main landmark name.
3. Write failing tests for deterministic placeholder sections such as priority work, recent activity, and verification needs.
4. Write failing empty-fixture tests proving the route still renders useful structure with no work items.
5. Run the narrow tests and capture red.
6. Implement only enough route content to pass.
7. Run route/component tests and keep them green.

Acceptance:
- `/` and `/today` land on the Today experience.
- Today content is compact, scannable, and fixture-backed.
- Empty data does not break the shell or route layout.
```

```text
Task 32-A4 - Leads route

Context:
- Leads is the list entry point for lead triage.
- Full inbox behavior belongs to later phases; Phase 32 only establishes route structure.

TDD steps:
1. Write failing tests that `/leads` renders inside the operator shell.
2. Write failing tests for the Leads heading, selected nav state, and main landmark name.
3. Write failing tests for stable placeholder regions for filters, lead list/table, and route-level actions.
4. Write failing tests proving no lead-list API call is required for initial render.
5. Run the narrow tests and capture red.
6. Implement the smallest route skeleton needed to pass.
7. Run route/component tests and keep them green.

Acceptance:
- Leads route structure exists without implementing full inbox behavior.
- Filter and list regions have accessible labels.
- Route content remains dense and operations-friendly.
```

```text
Task 32-A5 - Lead Detail route

Context:
- Lead Detail is the detail/evidence destination for a selected lead.
- Full evidence review belongs to later phases; Phase 32 only establishes routable structure.

TDD steps:
1. Write failing tests that `/leads/:leadId` renders inside the operator shell.
2. Write failing tests for Lead Detail heading, selected Leads nav state, and main landmark name.
3. Write failing tests proving the route reads the lead ID from the URL and displays a deterministic fixture-friendly identifier.
4. Write failing tests for stable placeholder regions for summary, evidence, notes, activity, and right-rail context.
5. Write failing tests for missing or invalid lead IDs if the router can reach that state.
6. Run the narrow tests and capture red.
7. Implement only enough route skeleton to pass.
8. Run route/component tests and keep them green.

Acceptance:
- Detail route is reachable from the router.
- Leads navigation remains selected for detail routes.
- Lead ID handling is deterministic and does not trigger live API calls.
- Evidence/right-rail space is reserved without implementing full review behavior.
```

```text
Task 32-A6 - Verification Queue route

Context:
- Verification Queue is the operator route for reviewing uncertain or incomplete lead data.
- Phase 32 establishes the route and layout only.

TDD steps:
1. Write failing tests that `/verification` renders inside the operator shell.
2. Write failing tests for Verification Queue heading, selected nav state, and main landmark name.
3. Write failing tests for stable placeholder regions for queue list, confidence/status summary, and review actions.
4. Write failing tests proving the route renders with deterministic empty fixtures.
5. Run the narrow tests and capture red.
6. Implement only enough route skeleton to pass.
7. Run route/component tests and keep them green.

Acceptance:
- Verification Queue is a first-class route, not hidden under Leads.
- The route supports future review actions without live backend data.
- Status labels do not rely on color alone.
```

```text
Task 32-A7 - Pipeline route

Context:
- Pipeline is the operator route for run status and ingestion health.
- Full async run APIs belong to later phases; Phase 32 only establishes routable structure.

TDD steps:
1. Write failing tests that `/pipeline` renders inside the operator shell.
2. Write failing tests for Pipeline heading, selected nav state, and main landmark name.
3. Write failing tests for stable placeholder regions for run controls, run history, and system messages.
4. Write failing tests proving the route does not call `POST /run` or any live pipeline endpoint during render.
5. Run the narrow tests and capture red.
6. Implement only enough route skeleton to pass.
7. Run route/component tests and keep them green.

Acceptance:
- Pipeline route is visible in primary navigation.
- Initial render never starts a pipeline run.
- Run controls are clearly placeholders or disabled until API support exists.
```

```text
Task 32-A8 - Settings route

Context:
- Settings is the operator route for configuration and account/system preferences.
- Phase 32 should not expose secrets or create live configuration mutations.

TDD steps:
1. Write failing tests that `/settings` renders inside the operator shell.
2. Write failing tests for Settings heading, selected nav state, and main landmark name.
3. Write failing tests for stable placeholder regions for workspace preferences, integrations, and system configuration.
4. Write failing tests proving no secret values, raw environment variables, or provider tokens render in the browser.
5. Run the narrow tests and capture red.
6. Implement only enough route skeleton to pass.
7. Run route/component tests and keep them green.

Acceptance:
- Settings is reachable from primary navigation.
- Settings structure is present without live mutation behavior.
- Browser-visible settings content does not expose secrets.
```

```text
Task 32-A9 - Responsive navigation and right-rail collapse

Context:
- The operator shell must remain usable across desktop, tablet, and narrow mobile viewports.
- The navigation and right rail should collapse predictably without overlapping content.

TDD steps:
1. Write failing responsive tests for desktop viewport:
   - left navigation is visible
   - right rail can be visible
   - main content has stable space and does not overlap shell chrome
2. Write failing responsive tests for tablet viewport:
   - navigation can collapse to an icon rail or drawer trigger
   - selected route remains clear
   - right rail can collapse without losing its accessible name
3. Write failing responsive tests for narrow viewport:
   - navigation collapses behind an accessible control
   - right rail collapses or moves below main content
   - no route heading, button label, tab, table header, or rail label overlaps another element
4. Write failing keyboard tests for opening and closing collapsed navigation and right rail controls.
5. Run the narrow responsive/component tests and capture red.
6. Implement only enough responsive shell behavior to pass.
7. Run responsive tests at all configured breakpoints.
8. Refactor only while tests remain green.

Acceptance:
- Desktop keeps the dense operator shell visible.
- Narrow viewports do not overlap nav, top bar, main content, or right rail.
- Collapsed controls have accessible names and keyboard behavior.
- Text remains inside its parent controls without viewport-width font scaling.
```

```text
Task 32-A10 - Build and component verification

Context:
- Phase 32 is complete only when route, layout, responsive, component, accessibility, and build checks pass.
- Verification should prove browser-visible assets are safe and the operator shell remains the first screen.

TDD steps:
1. Run the narrow route tests for Today, Leads, Lead Detail, Verification Queue, Pipeline, Settings, and not-found routes.
2. Run layout/component tests for shell landmarks, navigation state, top bar, main content, and right rail.
3. Run responsive tests for desktop, tablet, and narrow viewport behavior.
4. Run accessibility checks for landmark names, icon-button names, focus-visible behavior, selected nav state, and keyboard collapse controls.
5. Run the frontend build.
6. Scan built browser-visible assets for `API_TOKEN`, database URLs, Slack/email/LLM provider keys, session secrets, and real `.env` values.
7. Capture green command output for the narrow tests, broader relevant suite, and build.
8. File beads follow-ups for behavior discovered outside Phase 32 scope instead of expanding this phase.

Acceptance:
- All Phase 32 route and shell tests are green.
- The build succeeds.
- Browser-visible assets do not expose secrets.
- The first screen remains the operator workspace.
- Any incomplete inbox, evidence, pipeline, or settings behavior is tracked as follow-up work, not silently implemented in Phase 32.
```
