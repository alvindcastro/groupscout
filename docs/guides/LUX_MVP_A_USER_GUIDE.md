# LUX MVP-A User Guide — AI Client Status Email Generator

## What This Does

You send it a project snapshot. It writes the client email. You review it in Slack and decide whether to send or edit.

Nothing reaches the client until you approve it.

---

## How to Trigger an Update

Send a POST request to the webhook with the project data as JSON. In practice this will come directly from your JobTread integration, but you can trigger it manually during setup or for one-off updates.

**Webhook URL:**
```
https://your-n8n-instance/webhook/lux-status-email
```

**Minimum required fields:**

```json
{
  "project_name": "Project Name",
  "client_name": "First Last",
  "phase": "Current Phase",
  "percent_complete": 62,
  "milestones": [...],
  "budget_variance": -4200,
  "open_items": [...],
  "next_site_visit": "2025-05-07"
}
```

Include `budget_total` when you have a meaningful overrun — the pipeline uses it to calculate whether the variance is significant enough to surface.

---

## What You'll See in Slack

When an update runs, a message posts to `#client-updates-review`:

```
[Notification copy — 2-3 sentences from Claude, casual team tone]

Subject: Hartwell Residence — Framing Update — May 7

Hi Sarah,

[Full email body]

[Approve & Send]  [Edit First]
```

The notification copy reads like a teammate flagging it, not a bot alert. It will end with a question — "who is reviewing this before it goes out?" — so ownership is always explicit.

---

## Reviewing the Draft

Read the email in Slack before deciding.

**Things to check:**
- Is the subject line specific? It should name the phase and date, not be generic
- Does the body match reality? Claude works from the data you sent — if the data was stale, the email will be too
- Is there a client action item? If yes, it appears as "One thing we need from you:" — confirm it's accurate and timely
- Budget: if there was an overrun, confirm the explanation is correct and the tone is calm

**If it looks right:** click **Approve & Send** (button triggers your send flow, if wired up) or copy and send manually.

**If it needs changes:** click **Edit First** or simply edit the draft in your email client before sending.

---

## What Claude Will and Won't Do

**Claude will:**
- Write in plain language — no construction jargon
- Suppress minor budget variance (within 5%) — clients don't need to know
- Surface a meaningful overrun plainly, with one-sentence explanation
- Call out client action items so they're impossible to miss
- Keep the body under 200 words
- Always sign off "Talk soon, / The LUX Team"

**Claude will not:**
- Mention internal LUX tasks (supplier follow-ups, subcontractor scheduling)
- Use percentages or completion numbers
- List all milestones — only the most relevant one per status
- Make up information not in the payload

---

## Payload Reference

| Field | Type | Notes |
|---|---|---|
| `project_name` | string | Used in subject line and throughout |
| `client_name` | string | First name used in greeting |
| `phase` | string | Current phase — used in subject line |
| `percent_complete` | number | Used internally for context; never shown to client |
| `milestones` | array | Each has `name`, `status` (complete/in_progress/upcoming), and `date` or `due` |
| `budget_variance` | number | Negative = over budget. In CAD. Positive = under budget. |
| `budget_total` | number | Required for variance % calculation. If omitted, large variances may not be surfaced correctly. |
| `delay_reason` | string | Optional. Explains schedule delays — Claude will work it into the email naturally. |
| `open_items` | array | Mix of client and internal tasks. Claude detects which are client-facing. |
| `next_site_visit` | string | ISO date. Mentioned in the email sign-off section. |

---

## Multiple Projects

The webhook handles one project per call. To send updates for multiple projects, send separate requests. You can trigger them sequentially — each generates its own Slack message.

---

## Related Docs

- [LUX_MVP_A_SETUP.md](LUX_MVP_A_SETUP.md) — initial setup and credential configuration
- [LUX_MVP_A_TROUBLESHOOTING.md](LUX_MVP_A_TROUBLESHOOTING.md) — when things go wrong
- [N8N_GUIDE.md](N8N_GUIDE.md) — n8n operations reference
