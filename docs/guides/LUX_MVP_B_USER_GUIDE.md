# LUX MVP-B User Guide — Automated Lead Follow-Up Sequence

## What This Pipeline Does

When a lead submits a contact form (or a CRM entry is created), this pipeline:

1. Classifies the lead — tier (high/medium/low), project type, and key detail from their message
2. Routes to the right sequence — commercial or residential, with different email tone and content
3. Drafts three follow-up emails — calibrated to the lead's quality, language, and project category
4. Logs everything to Airtable — staged and ready for the sales team to review and send
5. Notifies `#new-leads` in Slack — with lead context and a flag if it needs same-day attention

Nothing sends automatically. Every email goes through a human before it reaches a lead.

---

## The Three Emails

| Email | When | Purpose |
|---|---|---|
| Email 1 | Same day | Acknowledge, show expertise, propose a 20-minute call |
| Email 2 | Day 3, if no reply | Add genuine value — a consideration they may not have thought about |
| Email 3 | Day 7, if no reply to 1 or 2 | Leave the door open, no pressure |

Emails 2 and 3 are staged in Airtable. Your CRM or email client handles the send timing — the pipeline does not auto-schedule.

---

## Finding Your Leads in Airtable

1. Open your Airtable base → `Leads` table
2. New leads arrive with `Status: New`
3. Key fields to review:

| Field | What to look at |
|---|---|
| Lead Tier | `high` = act same day; `medium` = standard follow-up; `low` = sequence runs, low manual priority |
| Key Detail | The phrase Claude extracted from their message — what Email 1 opens with |
| Email 1 Body | Open immediately — this goes out today |
| Project Type | Confirms the sequence path taken (commercial or residential) |

4. After reviewing, update `Status` to `Contacted` when Email 1 goes out

`Urgency Signal` is reflected in the Slack notification, not stored in Airtable in the current workflow.

---

## Reading the Slack Notification

When a lead arrives, `#new-leads` receives a message like:

```
New lead in — Marcus Webb, Webb Commercial Properties. Commercial renovation,
$250k–$500k budget, Q3 2025 timeline. Classified HIGH tier — sequence staged
but this one needs a same-day personal touch. Who's owning this?
```

- **High tier**: someone needs to personally reach out the same day, in addition to Email 1
- **Medium tier**: standard — let Email 1 go, review Email 2 before day 3
- **Low tier**: sequence runs, but don't burn time on heavy manual follow-up

---

## Sending the Emails

The pipeline drafts — your email client sends.

Recommended workflow:
1. Open Airtable → find the new lead record
2. Copy **Email 1 Subject** and **Email 1 Body** into your email client
3. Review for any detail that needs personalization beyond what Claude added
4. Send from your own address, not a no-reply
5. Update `Status` to `Contacted`
6. If no reply by day 3: send Email 2; if no reply by day 7: send Email 3

---

## What Makes a Good Email 1

Check before sending:

- [ ] Opens with the lead's specific words or project detail — not a generic opener
- [ ] Proposes a 20-minute call with a question about their availability
- [ ] No exclamation points
- [ ] Does not start with "I wanted to reach out" or "Just following up"
- [ ] Under 120 words
- [ ] Signed off: "Looking forward to it, / The LUX Team"

If any of these are off, edit before sending. Report the issue so the prompt can be tuned.

---

## Triggering the Pipeline Manually

If you need to run the pipeline for a lead that didn't come through the webhook (e.g., a phone inquiry you want to sequence):

```bash
curl -X POST https://your-n8n.domain/webhook/lux-lead-followup \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Lead Name",
    "company": "Company Name or null",
    "source": "phone_inquiry",
    "project_type": "commercial_renovation",
    "budget_range": "100k-250k",
    "timeline": "Q4 2025",
    "message": "Paste what they told you here."
  }'
```

The `message` field is the most important input for Email 1 quality — include everything they said.

---

## What the Pipeline Cannot Do

- It does not send emails — it drafts them
- It does not schedule sends — you manage timing in your email client
- It does not update Airtable when you send — update `Status` manually
- It does not handle replies — reply tracking requires a separate CRM integration

---

*See [LUX_MVP_B_SETUP.md](LUX_MVP_B_SETUP.md) for initial configuration.*
*See [LUX_MVP_B_TROUBLESHOOTING.md](LUX_MVP_B_TROUBLESHOOTING.md) for issues.*
