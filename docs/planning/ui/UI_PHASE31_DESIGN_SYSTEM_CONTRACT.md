# Phase 31 UI Design System Contract

> Implementation contract for `groupscout-h80`.
> Source design reference: `/mnt/c/Users/alvin/WebstormProjects/groupscout-ui/DESIGN.md`.

## Current Scope

This repository does not currently contain a checked-in frontend package, component test harness, or static UI build directory. Phase 31 is therefore implemented here as the design-system contract that the first GroupScout UI code must test against before shipping components.

When a `web/` or equivalent frontend package is added, the first implementation task must convert this contract into failing component, token, accessibility, and static-asset safety tests before production UI code.

## Source Translation

The external `groupscout-ui/DESIGN.md` describes a Mintlify-inspired system with two distinct layers:

| Source layer | GroupScout decision |
|---|---|
| Atmospheric marketing heroes, sky gradients, cloud or rocket imagery, oversized display type, and product mockup depth. | Excluded from the operator workspace. Do not use these patterns for the first viewport, app shell, tables, detail views, or settings. |
| Dense documentation surfaces, three-column docs layout, hairline tables, Inter UI text, Geist Mono technical text, compact controls, dark code blocks, and mint active states. | Adopted for the GroupScout operator UI. Translate docs sidebar/prose/TOC into left navigation or filters, central work area, and right evidence or activity rail. |

GroupScout is a repeated-use sales operations tool, so density, evidence comparison, keyboard access, and clear semantic state are more important than brand spectacle.

## Token Contract

| Token group | Required GroupScout tokens | Rules |
|---|---|---|
| Typography | `font.interface`, `font.mono`, `text.body`, `text.table`, `text.caption`, `text.micro`, `text.code`, `text.button`, `text.heading` | Use Inter or system UI for prose. Use Geist Mono or monospace fallback only for IDs, source records, raw payload previews, API paths, and technical metadata. Do not use negative letter spacing in operator UI. |
| Color surfaces | `color.canvas`, `color.surface`, `color.surfaceSoft`, `color.surfaceCode`, `color.border`, `color.borderSoft`, `color.text`, `color.textMuted`, `color.textSubtle` | Default to flat white and soft gray surfaces with 1px borders. Avoid large tinted blocks except semantic rows, focused fields, or system status. |
| Accent | `color.accentMint`, `color.accentMintPressed`, `color.accentMintSoft` | Mint is reserved for active filters, focused inputs, verified evidence, successful confirmation, and primary confirmation controls. It is not a decorative background theme. |
| Semantic status | `status.new`, `status.notified`, `status.claimed`, `status.contacted`, `status.snoozed`, `status.flagged`, `status.verified`, `status.dismissed`, `status.won`, `status.lost`, `status.noResponse`, `status.error`, `status.warning`, `status.info` | Each status needs text, icon or shape, and color treatment. Color alone is insufficient. Commercial workflow state and verification state must remain visually distinguishable. |
| Spacing | `space.1` through `space.10`, based on a 4px unit | Tables, filters, and sidebars should use compact 4px and 8px increments. Use larger spacing only for page-level grouping, not hero-style whitespace. |
| Radius | `radius.xs` 4px, `radius.sm` 6px, `radius.md` 8px, `radius.panel` 10px to 12px, `radius.full` | Buttons may be pill-shaped. Inputs, tables, tabs, and evidence rows should stay at 4px to 8px. Avoid soft marketing-card rounding. |
| Focus | `focus.ring`, `focus.offset`, `focus.border` | Keyboard focus must be visible on every interactive primitive. Mint focus rings are allowed when paired with sufficient contrast and a non-color outline. |
| Motion | `motion.fast`, `motion.normal` | Motion is limited to focus, pressed, menu, and drawer transitions. No atmospheric background motion in the operator workspace. |

## Primitive Contract

| Primitive | Required variants and states | Acceptance |
|---|---|---|
| Button | `primary`, `secondary`, `ghost`, `danger`, `icon`; default, pressed, disabled, loading, focus-visible | Compact 32px to 40px height. Primary should be black or mint only when confirming. Danger must not reuse mint. Icon buttons require accessible names. |
| Badge | workflow status, verification, source, score band, required/error, type/code | Badges must remain legible at table density. Verification and commercial state should not share identical treatments. |
| Input | text, search, select trigger, textarea; empty, filled, invalid, disabled, focus-visible | Search/filter inputs must fit table toolbars. Invalid fields need copy plus semantic styling. |
| Tabs | segmented, underline, and compact rail variants | Tabs must expose selected state, keyboard navigation, and focus-visible treatment. Use tabs for detail sections and evidence views, not as oversized marketing pills. |
| Table | header, sortable header, row, selected row, empty state, loading state, error state | Tables are the default lead-management surface. Row height should stay compact while preserving readable touch targets on narrow viewports. |
| EvidenceBlock | source metadata, AI claim, raw audit link, confidence, contradiction, missing evidence, correction note | Source evidence must sit adjacent to AI claims. Missing or contradictory evidence is explicit and cannot be hidden in a collapsed default state. |
| CodeBlock | dark raw payload preview, inline code, copy action | Raw data previews use monospace and must support wrapping or horizontal scroll without breaking the layout. |

## Component Test Contract

The first frontend implementation must add failing tests before component code for these behaviors:

| Area | Required failing test examples |
|---|---|
| Token availability | Typography, colors, spacing, radii, semantic statuses, and focus tokens are exported from the frontend design-system entrypoint. |
| Button | Renders each variant and state; disabled blocks action; loading exposes busy state; icon-only button has an accessible name. |
| Badge | Renders every lead workflow status and verification status with non-color text or shape distinction. |
| Input | Label association, invalid messaging, disabled behavior, and visible focus state. |
| Tabs | Keyboard navigation, selected state, and panel association. |
| Table | Sortable header semantics, selected row state, loading, empty, error, and dense row rendering. |
| EvidenceBlock | AI claim and source evidence render together; missing evidence, weak confidence, and contradictions are visible by default. |
| Focus states | Tab-through order reaches all controls and displays a visible focus indicator. |
| No hero patterns | Operator shell and first screen do not render marketing hero classes, atmospheric gradients, cloud/rocket imagery, oversized hero display type, or decorative orb backgrounds. |
| Static asset safety | Browser-visible files do not contain `API_TOKEN`, database URLs, Slack tokens, email provider keys, LLM provider keys, session secrets, or real `.env` values. |

## Static Asset Safety

Browser code must use relative `/api/*` paths. It must not embed automation bearer tokens or server-only integration secrets.

When static assets exist, add a build-time check equivalent to:

```bash
rg -n "API_TOKEN|DATABASE_URL|SLACK_WEBHOOK|SLACK_BOT_TOKEN|RESEND_API_KEY|CLAUDE_API_KEY|ANTHROPIC_API_KEY|GEMINI_API_KEY|OPENAI_API_KEY|OLLAMA_ENDPOINT|UI_SESSION_SECRET|postgres://|sqlite://|Bearer [A-Za-z0-9._-]+" web/dist
```

The command should return no matches. If the frontend build directory is not `web/dist`, update the path but keep the same intent.

## Operator Workspace Guardrails

| Do | Do not |
|---|---|
| Start on the operator workspace: Today, Leads, Verification, Pipeline, or Settings. | Start with a landing page, hero section, value proposition, or marketing CTA. |
| Use split panes, tables, sidebars, right rails, definition rows, and compact toolbars. | Use oversized hero display type, decorative gradients, or atmospheric imagery. |
| Pair AI claims with source evidence and raw audit references. | Separate model rationale from the source record needed to verify it. |
| Use mint sparingly for active, focused, verified, and confirmed states. | Let mint become the dominant page palette. |
| Keep static config public and non-secret. | Expose `API_TOKEN`, database URLs, Slack/email/LLM keys, or session secrets to browser JavaScript. |

## Phase 31 Done Criteria

Phase 31 is complete for this repository when:

| Criterion | Current result |
|---|---|
| `groupscout-ui/DESIGN.md` is translated into GroupScout operator rules. | Covered by this contract and `UI_DESIGN_SYSTEM.md`. |
| Token and component test requirements exist before UI code. | Covered by the token, primitive, and component test contracts above. |
| Buttons, badges, inputs, tabs, tables, evidence blocks, focus states, and semantic statuses are covered. | Covered by the primitive and test contracts above. |
| Static frontend asset secret checks are specified. | Covered by the static asset safety section. No checked-in static frontend assets exist in this repo today. |
| Marketing hero patterns are excluded from the operator workspace. | Covered by source translation, no-hero test requirements, and workspace guardrails. |
