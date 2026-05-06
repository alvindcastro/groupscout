# LUX MVP UI/UX Spec

## Product Position

LUX should feel like one operator workbench with three workstreams, not three unrelated demo tools. The common pattern is stable across all MVPs:

`structured input -> AI draft -> human review -> external action`

The unifying idea is not "AI writing." The unifying idea is controlled operational output:
- MVP A turns project facts into client-ready communication
- MVP B turns inbound leads into staged sales follow-up
- MVP C turns internal content inputs into publish-ready social drafts

## Shared UX Principles

1. Build around work items, not integrations.
2. Keep source facts visible beside generated output at all times.
3. Increase review friction with business risk.
4. Make ownership explicit on every item.
5. Standardize shell, navigation, filters, audit, and actions.
6. Let editing surfaces differ by artifact type.
7. Treat Slack and Airtable as sync targets, not the system of record.

## Product Shell

### Navigation

Primary nav:
- `Home`
- `Review Queue`
- `Client Updates`
- `Leads`
- `Content`
- `Activity`
- `Rules`

Top bar:
- Global search
- Quick create
- Notifications
- Team/user switch

### Layout Pattern

Default desktop pattern:
- Left column: list, filters, saved views
- Center: generated artifact and editor
- Right rail: source facts, flags, history, approvals, next actions

This should be the default shell for all three modules, but the center pane changes by artifact:
- MVP A: email review desk
- MVP B: sequence workbench
- MVP C: rendered content studio

## Home Dashboard

The home screen should answer one question: what needs attention now?

Top summary cards:
- `Needs Review`
- `Unassigned`
- `Due Today`
- `Blocked`

Priority sections:
- Client updates with client action items or surfaced budget risk
- High-tier leads without owner
- Content drafts waiting for post or schedule

The dashboard should also include:
- `My Work`
- `Recent Activity`
- `Quick Actions`

Quick actions:
- `New Client Update`
- `Import Lead`
- `Create Content Draft`

## Shared Work Item Model

Every generated unit should behave like a `WorkItem` even if the domain object differs.

Common fields:
- `id`
- `module`
- `type`
- `status`
- `priority`
- `owner`
- `reviewer`
- `source_system`
- `created_at`
- `updated_at`
- `due_at`

Common linked records:
- `Context Snapshot`
- `Generated Artifact`
- `Audit Events`
- `Comments`
- `Version History`

Shared flags:
- `has_client_action_items`
- `budget_variance_flag`
- `lead_tier`
- `project_category`
- `content_type`
- `urgency_signal`

## Shared Status Model

Cross-product base states:
- `New`
- `Needs Review`
- `In Edit`
- `Approved`
- `On Hold`
- `Rejected`
- `Completed`
- `Archived`

Module-specific terminal states:
- MVP A: `Sent`
- MVP B: `Owned`, `Email 1 Sent`, `Follow-up Due`, `Qualified`, `Dead`
- MVP C: `Scheduled`, `Posted`

The shell should present one common lifecycle language, while each module exposes its own operational state labels where necessary.

## Review Queue

The review queue is the real product center.

Queue row anatomy:
- Item title
- Module label
- Priority chip
- Status
- Owner
- Last updated
- Why it matters now

Priority reasons should be human-readable:
- `Client action needed`
- `Budget over threshold`
- `High-tier lead`
- `Follow-up due`
- `Ready to publish`
- `Execution failed`

Standard actions:
- `Approve`
- `Edit`
- `Assign`
- `Regenerate`
- `Hold`
- `Reject`

Module-specific terminal actions:
- MVP A: `Approve & Send`
- MVP B: `Assign Owner`, `Send Email 1`, `Mark Contacted`
- MVP C: `Post Now`, `Schedule`, `Edit First`

## Module A: Client Updates

### Operator Job

Review one client-facing email for factual accuracy, tone, action items, and budget handling before it reaches the client.

### UX Shape

This module should feel like a review desk, not a campaign tool.

The review surface should prioritize:
- project snapshot
- surfaced client action items
- budget visibility rule outcome
- final subject/body
- edit history and sent history

### Key Screens

- Client updates list
- Draft review detail
- History timeline per project

### Required Behaviors

- Flag whether the draft contains a client action item
- Flag whether budget variance was surfaced or suppressed
- Show internal-only vs client-visible context clearly
- Support inline editing of subject and body
- Record approver, timestamp, and final sent copy
- Provide draft diff against edited/sent version

### Minimum MVP Scope

- Queue of generated drafts
- Single draft review screen
- Inline edit
- Approve/send
- Regenerate
- History and version diff

## Module B: Leads

### Operator Job

Triage incoming leads, claim ownership, review classification quality, and work from a staged three-email sequence.

### UX Shape

This module should feel like a sales workbench, not a single-draft approval flow.

The list view matters more here than in the other modules. Prioritization and due dates should dominate before copy review.

### Key Screens

- Lead inbox
- Lead detail and sequence review
- Follow-up due view
- Owner/status board

### Required Behaviors

- Pin high-tier leads at the top
- Show `lead_tier`, `project_category`, `key_detail`, and `urgency_signal`
- Explain why the lead routed to commercial or residential copy
- Show the exact phrase used for personalization
- Support inline editing across Email 1, Email 2, and Email 3
- Track owner, send status, and due dates

### Minimum MVP Scope

- Prioritized lead list
- Filters by tier, owner, due status, and category
- Lead detail page
- Editable three-email sequence
- Owner assignment
- Follow-up scheduling visibility

## Module C: Content

### Operator Job

Generate, review, lightly edit, and decide whether to post or schedule a LinkedIn draft.

### UX Shape

This module should feel editorial. Preview quality matters more than source payload inspection, though source facts still need to be visible.

### Key Screens

- Draft creation form
- Content review detail
- Content library/history

### Required Behaviors

- Support `Project Milestone` and `Podcast Episode` as first-class entry types
- Switch form fields dynamically by content type
- Render the post with line breaks and hashtags as it would appear publicly
- Show simple rule checks:
  - `#BuiltDifferent present`
  - `3 or fewer hashtags`
  - `under 150 words`
- Support light editing before final action
- Track whether the item was approved, scheduled, or posted

### Minimum MVP Scope

- Type-specific input form
- Generated post preview
- Inline edit
- Post now / schedule action
- Content history

## Rules Center

`Rules` should be a first-class area because all three MVPs already depend on structured tone and review logic.

It should contain:
- Brand voice references by module
- Approval policies
- Routing rules
- Integration/channel notes
- Status definitions
- Regeneration guidance

This reduces hidden logic and keeps the product from feeling magical in the wrong way.

## Audit And Traceability

Every module should record:
- input snapshot
- model output
- human edits
- approval/rejection
- external action taken
- actor and timestamp

Module-specific emphasis:
- MVP A: wording diff matters
- MVP B: ownership and due state matter
- MVP C: publish/schedule history matters

## UX Risks

1. Over-normalizing the modules until they all feel the same.
2. Keeping Slack and Airtable as shadow systems after a new shell exists.
3. Hiding the source facts that justify the draft.
4. Leaving ownership ambiguous, especially for leads.
5. Using one editor pattern for email, sequence, and social copy.
6. Letting duplicate content drafts pile up in MVP C without warning.

## Recommended Build Order

If LUX turns this into a real product later, the first UI slice should be:
1. Shared shell and review queue
2. MVP A review desk
3. MVP B lead inbox and detail
4. MVP C content studio
5. Rules center and richer audit views

MVP A is the cleanest starting point because it has the simplest artifact shape and the clearest human approval boundary.
