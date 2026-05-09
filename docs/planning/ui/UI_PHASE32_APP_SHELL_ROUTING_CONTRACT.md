# Phase 32 App Shell And Routing Contract

> Implementation contract for the future GroupScout operator shell.
> This repository still has no checked-in frontend package, route test harness, or static UI build directory, so Phase 32 is complete here as a route/layout contract. The first frontend package must convert this contract into failing route, layout, responsive, build, and component tests before production UI code.

## Current Scope

Phase 32 defines the operator app shell and route map that the future `web/` app or equivalent frontend package must satisfy.

Do not add untested production UI code from this repository state. When a frontend package exists, start with failing tests for this contract, confirm red evidence, and implement only enough shell and routing behavior to pass.

## Shell Contract

The operator workspace starts inside the product shell. It must not start with a marketing landing page, hero section, value proposition, decorative gradients, atmospheric imagery, oversized display type, or first-run sales copy.

| Shell area | Required behavior | Acceptance |
|---|---|---|
| Left navigation | Persistent primary navigation on desktop for Today, Leads, Verification Queue, Pipeline, and Settings. | Uses a `navigation` landmark or equivalent accessible structure; exposes active route state; remains keyboard reachable. |
| Top utility bar | Compact bar above the main workspace for search, sync/run status, environment or system state, and user/session utilities when available. | Uses predictable landmarks or labels; does not duplicate the left navigation as a second primary nav. |
| Main content | Route outlet for the current screen. | Uses one `main` landmark; page heading identifies the route; route loading and error states do not collapse the shell. |
| Right rail capacity | Reserved detail, evidence, activity, or system context rail. | Can be empty, hidden, or placeholder-backed until later phases; layout reserves a tested pattern for lead detail and evidence review. |
| Keyboard path | Focus moves through skip link, primary nav, top utilities, main route controls, and right rail controls when present. | Every interactive control has a visible focus state from the Phase 31 contract. |

## Route Contract

Routes must be stable enough for Slack/email links, operator bookmarks, and future generated route tests.

| Route | Screen | Required default content | Later-phase dependency |
|---|---|---|---|
| `/` | Today | Command-center route that can summarize new high-score leads, aging claimed leads, failed runs, and system health. | Stats and system APIs from Phase 37. |
| `/today` | Today | Explicit bookmarkable alias for the Today command-center route. | Stats and system APIs from Phase 37. |
| `/leads` | Leads | Table-first lead inbox route with filter and loading/empty/error capacity. | Mocked inbox in Phase 33 and live `/api/leads` in Phase 35. |
| `/leads/:leadId` | Lead Detail | Detail route that can show lead summary, actions, evidence, raw audit link, outreach, and activity regions. | Lead detail/evidence in Phase 34 and `/api/leads/{id}` in Phase 35. |
| `/verification` | Verification Queue | Queue route for missing evidence, weak confidence, contradictions, and manual review flags. | Verification data model and correction storage follow-ups. |
| `/pipeline` | Pipeline Monitor | Compact run and system monitor route. The primary navigation may use the shorter `Pipeline` label. | Pipeline runs, stats, and system APIs from Phase 37. |
| `/settings` | Settings | Grouped configuration route for properties, thresholds, routing, session/auth, and integration status. | Auth/session and settings persistence decisions. |

Route tests must prove unknown routes render a shell-safe not-found state without losing navigation, top utility access, or recovery links.

## Failing Test Contract

The first frontend implementation must add failing tests before production shell code for these behaviors:

| Area | Required failing test examples |
|---|---|
| Route rendering | `/`, `/today`, `/leads`, `/leads/:leadId`, `/verification`, `/pipeline`, `/settings`, and an unknown route render the expected route landmark, heading, and active nav state. |
| Shell layout | Left navigation, top utility bar, main content, and right rail capacity are present in the desktop shell without requiring live API data. |
| Route isolation | Loading, empty, and error placeholders for one route do not remove global navigation or top utilities. |
| Lead detail route | A lead ID in the URL is exposed to the detail loader boundary without loading raw audit payloads by default. |
| Navigation | Operators can move between primary routes by pointer and keyboard; the active route is exposed with text and state, not color alone. |
| Responsive navigation | Narrow viewports collapse the left nav into a tested drawer, rail, or menu pattern with accessible open/close controls. |
| Responsive right rail | Lead detail or evidence rail collapses below or behind a controlled panel on narrow viewports without hiding missing evidence or contradiction alerts by default. |
| No hero patterns | The first viewport stays in the operator workspace and does not render marketing hero, gradient orb, atmospheric image, cloud/rocket, or oversized display patterns. |
| Build verification | The frontend package builds, route/component tests run, and static asset secret scan passes after implementation. |

## Responsive Contract

| Viewport | Expected shell behavior |
|---|---|
| Desktop and wide laptop | Left navigation, top utility bar, main route content, and optional right rail can appear together. Content stays dense and scan-friendly. |
| Tablet | Navigation may narrow to an icon rail or drawer. Main content remains primary. Right rail may collapse into tabs or a side panel. |
| Mobile | Navigation is controlled by an accessible menu button. Right rail content moves below main content or into an explicit panel. Primary route heading and route actions remain visible without horizontal page scroll. |

Responsive tests must use stable viewport sizes and assert both visibility and accessibility state, not only screenshots.

## Build And Secret-Safety Verification

Phase 32 uses the Phase 31 static asset safety rule. Browser-visible assets must use relative `/api/*` paths and must not embed automation or server-only secrets.

When static assets exist, run a build-time scan equivalent to:

```bash
rg -n "API_TOKEN|DATABASE_URL|SLACK_WEBHOOK|SLACK_BOT_TOKEN|RESEND_API_KEY|CLAUDE_API_KEY|ANTHROPIC_API_KEY|GEMINI_API_KEY|OPENAI_API_KEY|OLLAMA_ENDPOINT|UI_SESSION_SECRET|postgres://|sqlite://|Bearer [A-Za-z0-9._-]+" web/dist
```

The command should return no matches. If the frontend build directory is not `web/dist`, update the path but keep the same intent.

## Phase 32 Done Criteria

Phase 32 is complete for this repository when:

| Criterion | Current result |
|---|---|
| Route/layout failing-test requirements exist before UI code. | Covered by the failing test contract above. |
| Operator shell behavior is specified for left nav, top utility bar, main content, and right rail capacity. | Covered by the shell contract above. |
| Routes exist in the plan for Today, Leads, Lead Detail, Verification Queue, Pipeline, and Settings. | Covered by the route contract above. |
| Responsive collapse behavior is specified for navigation and detail/evidence rail. | Covered by the responsive contract above. |
| Build, component-test, and static asset verification expectations are documented. | Covered by build and secret-safety verification above. No checked-in static frontend assets exist in this repo today. |
