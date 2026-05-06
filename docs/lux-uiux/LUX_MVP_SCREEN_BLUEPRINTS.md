# LUX MVP Screen Blueprints

These are low-fidelity wireframe notes for a unified LUX operator console. They are meant to be decision-oriented, not visual design comps.

## 1. Home

Purpose:
- Give operators a single place to see what needs attention across all three MVPs.

Primary CTA:
- `Open Review Queue`

Secondary CTA:
- `New Client Update`
- `Import Lead`
- `Create Content Draft`

```text
+----------------------------------------------------------------------------------+
| LUX Workbench                                      Search      Alerts   User     |
+--------------------+-------------------------------------------------------------+
| Home               | Needs Review | Unassigned | Due Today | Blocked            |
| Review Queue       +-------------------------------------------------------------+
| Client Updates     | Priority Now                                              |
| Leads              | [Client action needed] [High-tier leads] [Ready to post]  |
| Content            +-------------------------------------------------------------+
| Activity           | My Work                                                   |
| Rules              | [A] Hartwell update      [B] Marcus lead   [C] Episode 47 |
|                    +-------------------------------------------------------------+
|                    | Recent Activity                                            |
|                    | generated -> edited -> approved -> sent/scheduled/posted   |
+--------------------+-------------------------------------------------------------+
```

## 2. Review Queue

Purpose:
- Act as the cross-MVP inbox for all generated items.

Primary CTA:
- `Open Item`

Secondary CTA:
- `Assign`
- `Filter`

```text
+----------------------------------------------------------------------------------+
| Review Queue                                              Saved View: All Open   |
+---------------------------+------------------------------------------------------+
| Filters                   | Queue List                                           |
| Module                    | [A] Hartwell Residence    Client action needed       |
| Status                    |     Needs Review   Unassigned   12m ago              |
| Owner                     |------------------------------------------------------|
| Priority                  | [B] Marcus Webb lead      High-tier lead             |
| Due Today                 |     New            Unassigned   4m ago               |
|                           |------------------------------------------------------|
|                           | [C] Built Different Ep 47  Ready to publish          |
|                           |     Needs Review   Alvin        28m ago              |
+---------------------------+------------------------------------------------------+
```

## 3. MVP A Detail: Client Update Review Desk

Purpose:
- Review one client-facing email before sending.

Primary CTA:
- `Approve & Send`

Secondary CTA:
- `Edit First`
- `Regenerate`
- `Hold`

```text
+----------------------------------------------------------------------------------+
| [A] Hartwell Residence                 Needs Review     Client action needed      |
| Owner: Unassigned                      Source: JobTread    Last updated: 12m ago  |
+------------------------------+--------------------------------+-------------------+
| Project Facts                | Draft Email                    | Review Rail       |
| - Client: Sarah              | Subject                        | Flags             |
| - Phase: Framing             | Hartwell Residence ...         | - Client action   |
| - Next visit: May 7          |                                | - Budget hidden   |
| - Budget status: within 5%   | Body                           |                   |
|                              | Hi Sarah,                      | Actions           |
| Client-visible items         | ...                            | [Approve & Send]  |
| - Cabinet hardware choice    |                                | [Edit First]      |
|                              |                                | [Regenerate]      |
| Internal-only items          |                                |                   |
| - Supplier follow-up         |                                | History           |
| - Subcontractor schedule     |                                | v1 -> v2 -> sent  |
+------------------------------+--------------------------------+-------------------+
```

## 4. MVP B Detail: Lead Workbench

Purpose:
- Triage, assign, and work through a staged follow-up sequence.

Primary CTA:
- `Assign Owner`

Secondary CTA:
- `Send Email 1`
- `Mark Contacted`
- `Edit Sequence`

```text
+----------------------------------------------------------------------------------+
| [B] Marcus Webb / Webb Commercial Properties        High Tier   New              |
| Category: Commercial Renovation                     Urgency: Same-day touch       |
+------------------------------+--------------------------------+-------------------+
| Lead Summary                 | Sequence                        | Ops Rail          |
| - Source: web form           | Email 1                         | Owner             |
| - Budget: 250k-500k          | Subject ...                     | [Claim Owner]     |
| - Timeline: Q3 2025          | Body ...                        |                   |
| - Key detail: tenant spaces  |--------------------------------| Route             |
|                              | Email 2                         | Commercial path   |
| Original message             | Body ...                        | Why: project cat  |
| "two tenant spaces..."       |--------------------------------|                   |
|                              | Email 3                         | Follow-up         |
| Personalization source       | Body ...                        | Day 3 due         |
| "tenant spaces"              |                                 | Day 7 due         |
+------------------------------+--------------------------------+-------------------+
```

## 5. MVP C Detail: Content Studio

Purpose:
- Generate and review one social draft with a publishing decision.

Primary CTA:
- `Post Now`

Secondary CTA:
- `Schedule`
- `Edit First`
- `Regenerate`

```text
+----------------------------------------------------------------------------------+
| [C] Project Milestone Draft                  Needs Review      Ready to publish   |
+------------------------------+--------------------------------+-------------------+
| Content Input                | LinkedIn Preview               | Rule Checks       |
| Type: Project Milestone      | Framing done. 4,200 sqft...   | #BuiltDifferent   |
| Milestone: Framing complete  |                                | <= 3 hashtags     |
| Detail: 11 days, rain delay  | That's what happens when...   | < 150 words       |
| Team: Delta Framing          |                                |                   |
| Next phase: Rough-in         | #LUXBuilds #CustomHome        | Actions           |
|                              | #BuiltDifferent                | [Post Now]        |
|                              |                                | [Schedule]        |
|                              |                                | [Edit First]      |
+------------------------------+--------------------------------+-------------------+
```

## 6. Mobile/Responsive Priority

If this becomes responsive early, preserve these priorities:
- Home: cards first, queue second, activity third
- Review Queue: filters collapse into a drawer
- MVP A: draft must stay readable before side rails
- MVP B: summary and Email 1 should appear before Email 2 and Email 3
- MVP C: preview should stay ahead of metadata

## 7. Shared Interaction Rules

- Never hide the source data that created the draft.
- Regeneration must require a reason.
- Approval actions should log actor and time automatically.
- Editing should preserve previous versions.
- Ownership changes should appear in the activity feed immediately.

## 8. Future High-Fidelity Design Notes

When these wireframes move into real design:
- Keep the shell premium and operational, not startup-generic
- Let status color do real work, but avoid over-saturating the UI
- Make module labels unmistakable so risk level stays legible
- Give MVP C more editorial whitespace than A or B
