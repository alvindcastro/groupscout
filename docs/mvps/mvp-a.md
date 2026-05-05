# MVP A — AI Client Status Update Generator

**Status:** Complete
[x] Bruno endpoints created (2026-05-05)

---

## Overview

n8n pipeline that accepts a JobTread project JSON payload, calls Claude twice (status email + Slack notification copy), and posts the draft to Slack for PM review before anything goes to the client.

**Webhook:** `POST /webhook/lux-status-email`
**Collection folder:** `api/bruno/lux-mvp-a/`

---

## Bruno Endpoints

| Seq | File | Method | Path |
|-----|------|--------|------|
| 1 | `Status Email - Standard Update.bru` | POST | `/webhook/lux-status-email` |
| 2 | `Status Email - Budget Overrun.bru` | POST | `/webhook/lux-status-email` |

**Scenario coverage:**

- **Standard Update** — Hartwell Residence (Framing phase, -$4,200 variance within ±5% threshold, one client action item)
- **Budget Overrun** — Mercer Street Commercial (Structural Steel phase, -$31,500 against $480k budget, supplier delay, two LUX action items)

---

## Bruno Variables

| Variable | Source | Value |
|----------|--------|-------|
| `n8n_url` | `api/bruno/environments/Local.bru` | `http://localhost:5678` |

No dynamic per-run variables required — both payloads are fully static.

---

## Pipeline Flow

```
Webhook
  → Code node (detect client action items, build context)
  → Anthropic API (Prompt 1: brand voice + Prompt 2: status email)
  → JSON Parse
  → Anthropic API (Prompt 3: Slack notification copy)
  → JSON Parse
  → Slack post to #client-updates-review
```

---

## Key Files

| Path | Purpose |
|------|---------|
| `docs/mvps/mvp-a/payload.json` | Standard update test payload |
| `docs/mvps/mvp-a/payload_alt.json` | Budget overrun test payload |
| `docs/mvps/mvp-a/prompts/system_brand_voice.txt` | Prompt 1: LUX brand voice system prompt |
| `docs/mvps/mvp-a/prompts/user_status_email.txt` | Prompt 2: status email generator |
| `docs/mvps/mvp-a/prompts/user_slack_notify.txt` | Prompt 3: internal Slack copy |
| `docs/mvps/mvp-a/n8n_workflow.json` | Importable n8n workflow |

---

## Output Shape

**Email node:** `{ "subject": "...", "body": "..." }`
**Slack node:** `{ "slack_message": "..." }`

Budget variance rules: suppress if within ±5% of total budget; surface plainly with explanation if negative and above 5%; never use the word "variance".
