# MVP A — AI Client Status Update Generator

## What It Does

Accepts a structured project data payload (simulating a JobTread export), calls Claude to generate a professional client-facing status email, and posts a draft to Slack for PM review before sending.

---

## Problem It Solves

LUX PMs write client update emails manually — pulling from memory, JobTread, and scattered notes. This pipeline eliminates that draft time and ensures every update is consistently professional, regardless of who wrote it.

---

## Files

```
mvp-a/
├── README.md                    <- this file
├── payload.json                 <- sample project payload (framing phase, minor variance)
├── payload_alt.json             <- sample project payload (structural steel + budget overrun)
├── prompts/
│   ├── system_brand_voice.txt   <- Prompt 1, applied to every API call as system
│   ├── user_status_email.txt    <- Prompt 2, status email generation
│   └── user_slack_notify.txt    <- Prompt 3, internal Slack notification copy
└── n8n_workflow.json            <- importable n8n workflow
```

---

## Pipeline

```
Webhook (POST)
  → Code: Build Context (notification_context_json + client action item detection)
  → Code: Load Prompts (reads /workspace/groupscout/docs/mvps/mvp-a/prompts/*.txt)
  → HTTP: Anthropic API (claude-sonnet-4-6) → email subject + body
  → Code: Parse Email JSON
  → HTTP: Anthropic API (claude-haiku-4-5) → slack_message
  → Code: Parse Slack JSON
  → Slack: Post to #client-updates-review (Block Kit with Approve/Edit buttons)
  → Respond to Webhook (202 queued)
```

---

## Input Payloads

### payload.json — Standard update

Framing phase, 62% through, minor budget variance (within 5% — not surfaced to client), one client action item (cabinet hardware selection).

### payload_alt.json — Behind schedule + budget overrun

Structural steel phase, 34% complete. Supplier delay absorbed into schedule. Budget overrun at 6.6% of total — surfaced plainly with explanation. All open items are internal LUX tasks — not shown to client.

The alt payload is the important test: it exercises the budget variance rules and delay handling that matter most in real client communication.

---

## Prompts

### Prompt 1 — Brand Voice (system)

`prompts/system_brand_voice.txt`

The persistent identity layer applied to every Anthropic API call. Defines tone, forbidden patterns, structure rules, and sign-off format.

### Prompt 2 — Status Email Generator (user)

`prompts/user_status_email.txt`

Core generation prompt. Includes explicit rules for budget variance thresholds, milestone filtering, and client action item calling.

Placeholder: `{project_json}` — replaced at runtime by the Build Context Code node.

### Prompt 3 — Slack Notification Copy (user)

`prompts/user_slack_notify.txt`

Lightweight second call (Haiku model) that produces the internal team message accompanying the draft. Keeps tone casual, not bot-like.

Placeholder: `{notification_context_json}` — replaced at runtime with the context built by the Code node.

---

## Client Action Item Detection

The Build Context Code node scans `open_items` for client-facing keywords before the first API call:

```javascript
const clientKeywords = ['client', 'owner', 'select', 'confirm', 'approve', 'decide', 'choose'];
const hasClientItems = openItems.some(item =>
  clientKeywords.some(kw => item.toLowerCase().includes(kw))
);
```

The `has_client_action_items` flag is passed to Claude in the notification context, giving it signal to add urgency to the Slack message when the client needs to act.

---

## Expected Output

### Email (payload.json)

```json
{
  "subject": "Hartwell Residence — Framing Update — May 7",
  "body": "Hi Sarah,\n\nYour foundation is complete and framing is well underway — the structure is taking shape on site and we're on track to wrap by May 15th.\n\nNext up: we move into electrical rough-in starting May 28th, which wires the home for everything that comes after.\n\nOne thing we need from you: cabinet hardware selection. We'll send over the options this week — a quick decision keeps us on schedule.\n\nYour next site visit is May 7th. Looking forward to showing you the progress in person.\n\nTalk soon,\nThe LUX Team"
}
```

### Slack delivery (to `#client-updates-review`)

```
[Block Kit message]

[Notification copy — 2-3 sentences, casual, ends with "who is reviewing this?"]

Subject: Hartwell Residence — Framing Update — May 7

[Full email body]

[Approve & Send]  [Edit First]
```

---

## Build Notes

- Email generation uses `claude-sonnet-4-6` for quality and rule compliance
- Slack copy uses `claude-haiku-4-5-20251001` — no brand rules needed, just brevity
- Prompts are loaded at runtime from `prompts/` inside the `n8n` container via `/workspace/groupscout/docs/mvps/mvp-a/prompts`
- The workflow has no branching — both payloads run the same path; the budget variance rules live in Prompt 2
- Human-in-the-loop by design: the PM reviews and approves before anything reaches the client

---

## Demo Script

1. Show `payload.json` — explain fields and how they map to JobTread exports
2. Trigger the webhook
3. Walk the n8n execution: Build Context output → first API call → parsed email → second API call → parsed Slack message
4. Show the Slack message — note the notification copy feels like a teammate, not a bot
5. Show the full email draft — subject is specific, body under 200 words, no jargon
6. Trigger `payload_alt.json` — budget overrun + supplier delay
7. Show how the email handles the budget situation without alarming the client; note all open items are internal and correctly suppressed

**Key talking points:**
- "The PM stays in control. Claude drafts, the human sends."
- "The budget variance rules mean Claude never buries a problem or over-explains a non-issue."
- "The Code node detects client action items and flags them — Claude doesn't have to guess."

---

*See `docs/guides/LUX_MVP_A_SETUP.md` for n8n import and credential setup.*
*See `docs/guides/LUX_MVP_A_USER_GUIDE.md` for day-to-day usage.*
*See `docs/guides/LUX_MVP_A_TROUBLESHOOTING.md` for debugging.*
