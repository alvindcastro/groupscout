# UI Design System Adaptation

> GroupScout-specific interpretation of `/mnt/c/Users/alvin/WebstormProjects/groupscout-ui/DESIGN.md`.
> This is a planning document for the future operator UI.
> Phase 31 implementation contract: [UI_PHASE31_DESIGN_SYSTEM_CONTRACT.md](./UI_PHASE31_DESIGN_SYSTEM_CONTRACT.md).

## Source

The referenced UI design file describes a Mintlify-inspired system: Inter for interface text, Geist Mono for code or technical content, white and soft-gray documentation surfaces, hairline borders, compact controls, dark code blocks, and a mint-green accent.

For GroupScout, use the documentation and product-surface parts of that system. Do not copy the marketing hero style into the operator app.

## Product Translation

GroupScout is an operational lead-review tool. The UI should feel dense, calm, and review-oriented.

| Source pattern | GroupScout adaptation |
|---|---|
| 3-column documentation layout | Left navigation/filter rail, central table/detail area, right evidence/activity rail. |
| Docs prose and property rows | Lead evidence, source metadata, AI rationale, correction history, and raw audit references. |
| Code blocks and inline code | Raw payload previews, API IDs, source IDs, error traces, and collector details. |
| Mint accent | Verified state, active filters, focused fields, and primary confirmation controls. |
| Hairline tables | Lead inbox, outreach log, run history, and source health lists. |

## Visual Rules

- Build the first screen as the lead workspace, not a landing page.
- Prefer tables, split panes, tabs, segmented filters, menus, and evidence rows.
- Keep card usage narrow: repeated lead summaries, modal panels, and genuinely framed tools only.
- Use flat white surfaces with `#e5e5e5`-style dividers and subtle gray backgrounds.
- Use Inter for all UI copy and Geist Mono for raw data, IDs, code-like fields, and API payloads.
- Keep radii restrained: 4px to 8px for controls, 8px to 12px for panels where needed.
- Reserve mint green for active, verified, focused, or confirmed states.
- Use red, amber, blue, and green as semantic signals, not as decorative palette blocks.
- Keep text sizes operational: 13px to 16px for tables, forms, notes, and metadata.
- Do not use decorative gradient orbs, oversized marketing sections, or purely atmospheric imagery.

## Screen Layout Guidance

| Screen | Layout guidance |
|---|---|
| Today | Dense command surface: high-score new leads, aging claimed leads, failed runs, and current system health. |
| Leads | Table-first view with sticky filters and compact row actions. |
| Lead Detail | Summary and action bar on top, source evidence beside AI rationale, activity/outreach rail on the right. |
| Verification Queue | Review queue with confidence, contradiction, missing evidence, and raw audit access emphasized. |
| Pipeline Monitor | Compact status list; do not parse Prometheus metrics directly in browser code. |
| Settings | Sparse, grouped forms for properties, thresholds, session settings, and integration status. |

## Evidence Design

Every AI claim that affects a sales action should sit near source evidence:

- source name and URL,
- raw audit link or payload preview,
- collector name,
- collected timestamp,
- extracted field,
- AI rationale,
- confidence or verification state,
- reviewer correction and note history.

Use rows and definition-list style layouts instead of bulky cards. The operator should be able to compare source-backed facts and AI output quickly.

## Interaction Rules

- Claim, dismiss, snooze, flag, log contacted, mark won, and mark lost must be one or two deliberate clicks.
- Destructive or irreversible transitions need confirmation once the state model is finalized.
- Invalid transitions should be disabled or rejected with specific error copy.
- Browser code must use relative `/api/*` paths only.
- Do not expose `API_TOKEN`, database URLs, Slack tokens, email provider keys, LLM provider keys, or session secrets in static assets.

## Phase 31 Implementation Notes

The current repository has no checked-in frontend package or component test harness. Phase 31 is therefore implemented as a design-system contract in `UI_PHASE31_DESIGN_SYSTEM_CONTRACT.md` so the first frontend slice can start from failing token, primitive, component, accessibility, no-hero, and static-asset safety tests.

The contract is binding for the future `web/` app or equivalent frontend package:

- Token exports must cover typography, colors, spacing, radii, semantic statuses, and focus states.
- Primitive tests must cover buttons, badges, inputs, tabs, tables, evidence blocks, and code/raw-data blocks before production UI code.
- Operator screens must use dense workspace surfaces and must not import the source design's marketing hero, gradient, cloud, rocket, or oversized display patterns.
- Static build verification must fail if browser-visible assets contain automation tokens, database URLs, Slack/email/LLM provider keys, session secrets, or real `.env` values.
