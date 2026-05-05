# LUX MVP-A Setup Guide — AI Client Status Email Generator

## What You're Setting Up

An n8n workflow that accepts a project data payload, calls Claude to generate a professional client-facing status email, and posts the draft to Slack for PM review. Nothing reaches the client until a human approves it.

> **MVP-A is standalone.** It does not require the GroupScout Go server, Postgres, or Ollama. Only n8n needs to be running.

---

## Starting n8n for Testing

```bash
# Start only n8n — no other services needed
docker compose up -d n8n

# Confirm running
docker compose ps n8n

# Tail logs during test runs
docker compose logs -f n8n
```

n8n UI: `http://localhost:5678`

```bash
# Stop when done
docker compose stop n8n
```

---

## Prerequisites

| Requirement | Notes |
| --- | --- |
| n8n instance | `docker compose up -d n8n` — see above |
| Anthropic API key | `sk-ant-...` — needs access to `claude-sonnet-4-6` and `claude-haiku-4-5-20251001` |
| Slack app with Bot Token | Needs `chat:write` scope, invited to `#client-updates-review` |

---

## Step 1 — Import the Workflow

1. In n8n, go to **Workflows → Import from File**
2. Select `docs/mvps/mvp-a/n8n_workflow.json`
3. The workflow imports with all nodes pre-wired

---

## Step 2 — Configure Credentials

### Anthropic API (HTTP Header Auth)

1. Go to **Credentials → New → HTTP Header Auth**
2. Name: `Anthropic API`
3. Header Name: `x-api-key`
4. Header Value: your `sk-ant-...` key
5. Assign this credential to both **Generate Status Email** and **Generate Slack Copy** nodes

### Slack

1. Go to **Credentials → New → Slack OAuth2 API**
2. Follow the Slack app setup in [N8N_GUIDE.md § 11 — Slack Integration](N8N_GUIDE.md)
3. Ensure the bot is invited to `#client-updates-review` (`/invite @your-bot-name`)
4. Assign to the **Post to Slack** node

---

## Step 3 — Verify the Slack Channel

The **Post to Slack** node is pre-configured to post to `#client-updates-review`. If your channel name differs:

1. Open the **Post to Slack** node
2. Update the `channelId` field to your channel name or ID
3. Channel IDs are more reliable than names — find them in Slack under channel settings

---

## Step 4 — Test with Sample Payloads

### Test 1 — Standard update (no budget flag, client action item)

```bash
curl -X POST http://localhost:5678/webhook/lux-status-email \
  -H "Content-Type: application/json" \
  -d @docs/mvps/mvp-a/payload.json
```

Verify:
- [ ] Slack `#client-updates-review` receives a Block Kit message with Approve/Edit buttons
- [ ] Subject references "Framing"
- [ ] Body surfaces "One thing we need from you:" for the cabinet hardware selection
- [ ] Body does not contain percentages, jargon, or internal open items
- [ ] Under 200 words

### Test 2 — Budget overrun + supplier delay

```bash
curl -X POST http://localhost:5678/webhook/lux-status-email \
  -H "Content-Type: application/json" \
  -d @docs/mvps/mvp-a/payload_alt.json
```

Verify:
- [ ] Email addresses the steel delivery delay plainly with one sentence on how LUX is handling it
- [ ] Budget situation is mentioned (overrun is above 5% — it must surface)
- [ ] The word "variance" does not appear
- [ ] All open items are internal LUX tasks — none shown to client
- [ ] Slack notification feels like a teammate, not a bot

---

## Step 5 — Activate the Workflow

Toggle the workflow to **Active** in n8n. The webhook URL becomes live.

Webhook URL:
```
http://localhost:5678/webhook/lux-status-email
```

---

## Prompt Files

The prompts are embedded inline in n8n Code nodes for portability. The canonical source files are:

| File | Purpose |
| --- | --- |
| `docs/mvps/mvp-a/prompts/system_brand_voice.txt` | LUX client communication identity (system prompt) |
| `docs/mvps/mvp-a/prompts/user_status_email.txt` | Email generation instructions (user prompt) |
| `docs/mvps/mvp-a/prompts/user_slack_notify.txt` | Internal Slack notification copy (user prompt) |

If you edit a prompt file, also update the corresponding Code node in n8n.

---

## Models Used

| Step | Model | Reason |
| --- | --- | --- |
| Email generation | `claude-sonnet-4-6` | Needs brand voice compliance and multi-rule reasoning |
| Slack copy | `claude-haiku-4-5-20251001` | Lightweight — 2-3 sentences, no brand rules |

---

## Related Docs

- [N8N_GUIDE.md](N8N_GUIDE.md) — n8n instance setup, credential management, workflow operations
- [LUX_MVP_A_USER_GUIDE.md](LUX_MVP_A_USER_GUIDE.md) — day-to-day usage for the team
- [LUX_MVP_A_TROUBLESHOOTING.md](LUX_MVP_A_TROUBLESHOOTING.md) — debugging failed runs
