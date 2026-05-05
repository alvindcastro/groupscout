# LUX MVP-B Setup Guide — Automated Lead Follow-Up Sequence

## What You're Setting Up

An n8n workflow that accepts an inbound lead payload, classifies it with Claude, generates a personalized 3-email follow-up sequence routed by project type, logs all drafts to Airtable, and notifies the sales team in Slack. Nothing sends automatically — Airtable is the staging area, humans send.

> **MVP-B is standalone.** It does not require the GroupScout Go server, Postgres, or Ollama. Only n8n needs to be running.

---

## Starting n8n for Testing

MVP-B only needs the `n8n` service. Start it in isolation from the full stack:

```bash
# Start only n8n (no GroupScout server, no Postgres, no Ollama)
docker compose up -d n8n

# Confirm it's running
docker compose ps n8n

# View logs
docker compose logs -f n8n
```

n8n UI: `http://localhost:5678`

To stop when done testing:

```bash
docker compose stop n8n
```

---

## Prerequisites

| Requirement | Notes |
| --- | --- |
| n8n instance | `docker compose up -d n8n` — see above |
| Anthropic API key | `sk-ant-...` — needs access to `claude-sonnet-4-6` and `claude-haiku-4-5` |
| Airtable account | Free tier is sufficient. You'll need a Base ID and personal access token. |
| Slack app with Bot Token | Needs `chat:write` scope, invited to `#new-leads` |

---

## Step 1 — Import the Workflow

1. In n8n, go to **Workflows → Import from File**
2. Select `docs/mvps/mvp-b/n8n_workflow.json`
3. The workflow imports with all 16 nodes pre-wired

## Prompt Files

This workflow reads its prompts from `/workspace/groupscout/docs/mvps/mvp-b/prompts` inside the n8n container.

- With this repo's `docker-compose.yml`, the project root is mounted read-only at `/workspace/groupscout`
- Editing files under `docs/mvps/mvp-b/prompts/` changes the next workflow run without editing Code nodes in n8n
- If you run n8n outside this compose stack, mount the same path and allow the `fs` builtin in Code nodes

---

## Step 2 — Configure Credentials

### Anthropic API (HTTP Header Auth)

1. Go to **Credentials → New → HTTP Header Auth**
2. Name: `Anthropic API`
3. Header Name: `x-api-key`
4. Header Value: your `sk-ant-...` key
5. Assign this credential to the four HTTP Request nodes: **Classify Lead**, **Generate Commercial Sequence**, **Generate Residential Sequence**, and **Generate Slack Copy**

### Airtable

1. Go to **Credentials → New → Airtable Personal Access Token**
2. Generate a token at [airtable.com/create/tokens](https://airtable.com/create/tokens) with `data.records:write` scope for your base
3. Assign to the **Airtable: Create Lead Record** node
4. In the node settings, set your Base ID (found in your Airtable base URL: `airtable.com/BASE_ID/...`)

### Slack

1. Go to **Credentials → New → Slack OAuth2 API**
2. Follow the Slack app setup in [N8N_GUIDE.md § 11 — Slack Integration](N8N_GUIDE.md)
3. Ensure the bot is invited to `#new-leads` (`/invite @your-bot-name`)
4. Assign to the **Post to #new-leads** node

---

## Step 3 — Set Up Airtable

Create a new Airtable base with a table named exactly `Leads`. Add these fields:

| Field Name | Field Type |
|---|---|
| Name | Single line text |
| Company | Single line text |
| Project Type | Single select: `custom_home`, `commercial_renovation`, `addition_or_remodel`, `multi_family` |
| Source | Single line text |
| Budget Range | Single line text |
| Timeline | Single line text |
| Original Message | Long text |
| Lead Tier | Single select: `high`, `medium`, `low` |
| Key Detail | Single line text |
| Email 1 Subject | Single line text |
| Email 1 Body | Long text |
| Email 2 Subject | Single line text |
| Email 2 Body | Long text |
| Email 3 Subject | Single line text |
| Email 3 Body | Long text |
| Status | Single select: `New`, `Contacted`, `Qualified`, `Dead` |

> Field names are case-sensitive. The Airtable node maps by exact name.
> `Urgency Signal` is used in workflow logic and Slack copy, but is not written to Airtable in the current working export.

---

## Step 4 — Set the Airtable Base ID

In the **Airtable: Create Lead Record** node, update the `application` field with your Base ID. Alternatively, set an environment variable `AIRTABLE_BASE_ID` in your n8n instance settings (Settings → Environment Variables).

---

## Step 5 — Verify the Slack Channel

The **Post to #new-leads** node is pre-configured for `#new-leads`. If your channel name differs:

1. Open the **Post to #new-leads** node
2. Update the `channelId` field to your channel name or ID
3. Channel IDs are more reliable than names — find them in Slack under channel settings

---

## Step 6 — Test with Sample Payloads

### Test 1 — Commercial lead (routes to Prompt 3A)

```bash
curl -X POST https://your-n8n.domain/webhook/lux-lead-followup \
  -H "Content-Type: application/json" \
  -d @docs/mvps/mvp-b/payload.json
```

Expected: `{"status":"queued","lead":"Marcus Webb","tier":"high"}`

Verify:
- Airtable record created with `Lead Tier: high`, `Project Type: commercial_renovation`
- Email 1 body opens with a reference to "tenant spaces"
- Slack message in `#new-leads` flags this as a high-tier lead

### Test 2 — Residential lead (routes to Prompt 3B)

```bash
curl -X POST https://your-n8n.domain/webhook/lux-lead-followup \
  -H "Content-Type: application/json" \
  -d @docs/mvps/mvp-b/payload_alt.json
```

Expected: `{"status":"queued","lead":"Jennifer Okafor","tier":"medium"}`

Verify:
- Airtable record created with `Lead Tier: medium`, `Project Type: custom_home`
- Email 2 body covers a residential-specific insight (not permitting — that's the commercial path)
- Email 3 is noticeably softer than the commercial version

---

## Step 7 — Activate the Workflow

1. Click **Activate** in the top-right of the workflow editor
2. The webhook URL is live: `https://your-n8n.domain/webhook/lux-lead-followup`
3. Point your website contact form or CRM webhook to this URL

---

## Environment Variables (optional)

Set in n8n Settings → Environment Variables:

| Variable | Value | Used By |
|---|---|---|
| `AIRTABLE_BASE_ID` | `appXXXXXXXXXXXXXX` | Airtable node, Slack deep link |

---

*See [LUX_MVP_B_USER_GUIDE.md](LUX_MVP_B_USER_GUIDE.md) for day-to-day usage.*
*See [LUX_MVP_B_TROUBLESHOOTING.md](LUX_MVP_B_TROUBLESHOOTING.md) for debugging.*
*See [N8N_GUIDE.md](N8N_GUIDE.md) for n8n instance operations reference.*
