# MVP-C Setup — LUX LinkedIn Post Pipeline

This guide walks through configuring the n8n workflow that powers MVP-C: a two-input (project milestone / podcast episode) LinkedIn post generator that delivers draft copy to Slack for review.

The workflow calls the Anthropic API directly from n8n. It does not require the GroupScout Go server, Postgres, or Ollama — only n8n needs to be running.

The current workflow export also includes runtime prompt loading plus tolerant JSON parsing in the `Extract Post` and `Build Slack Message` Code nodes so Claude responses like fenced JSON or `json { ... }` do not leak into Slack.

For general n8n operations — starting/stopping, monitoring executions, retrying failures, managing credentials and env vars — see `docs/guides/N8N_GUIDE.md` sections 9–11.

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
| Anthropic API key | `sk-ant-...` with access to `claude-opus-4-6` and `claude-haiku-4-5-20251001` |
| Slack bot token | `xoxb-...` token with `chat:write` permission for `#content-review` |
| Repo mounted into n8n | Required because `Load Prompts` reads files from `/workspace/groupscout/docs/mvps/mvp-c/prompts/` |
| Code node `fs` access | `NODE_FUNCTION_ALLOW_BUILTIN=fs` must be set for prompt loading |

---

## 1. Import the Workflow

1. Open your n8n instance (`http://localhost:5678` or your hosted URL).
2. Go to **Workflows** > **Import from file**.
3. Select `docs/mvps/mvp-c/n8n_workflow.json`.
4. Click **Save**.

---

## 2. Configure Credentials

### Anthropic API

1. Go to **Credentials** > **Add Credential** > search **Anthropic API**.
2. Name: `Anthropic account`
3. Paste your Anthropic API key (`sk-ant-...`).
4. Save.

Apply this credential to all three Claude HTTP Request nodes:
- **Claude: Milestone Post**
- **Claude: Podcast Post**
- **Claude: Slack Notification Copy**

### Slack

1. Go to **Credentials** > **Add Credential** > search **Slack**.
2. Use a bot token / Slack API credential.
3. Paste your bot token (`xoxb-...`).
4. Save and apply to the **Slack: Post to #content-review** node.
5. In Slack, invite the bot to the review channel:

```text
/invite @your-bot-name
```

---

## 3. Set Environment Variables in n8n

If your n8n runs via Docker, add these to the `environment:` section of your `docker-compose.yml` (or the n8n `.env`):

```env
NODE_FUNCTION_ALLOW_BUILTIN=fs
```

This workflow export does not read `$env.ANTHROPIC_API_KEY`; the Anthropic key lives in the n8n credential. The env var above is required because the `Load Prompts` Code node reads prompt files from disk.

---

## 4. Load Prompts

The workflow reads prompts at runtime from these source-of-truth files:

```
docs/mvps/mvp-c/prompts/
├── system_brand_voice.txt   ← system parameter on every Claude call
├── user_milestone.txt       ← user message for project_milestone type
├── user_podcast.txt         ← user message for podcast_episode type
└── user_slack_notify.txt    ← user message for Slack copy (optional Prompt 3)
```

If you edit one of these files, the workflow will pick up the new text on the next execution as long as n8n can read the repo mount.

---

## 5. Activate the Workflow

1. Toggle the workflow to **Active** in n8n.
2. Webhook URL: `http://localhost:5678/webhook/lux-linkedin-post`

**Test 1 — Project milestone**

```bash
curl -X POST http://localhost:5678/webhook/lux-linkedin-post \
  -H "Content-Type: application/json" \
  -d @docs/mvps/mvp-c/payload_milestone.json
```

Verify:
- [ ] Slack `#content-review` receives the post draft
- [ ] Post is specific to the milestone — not generic
- [ ] IF node routed to the milestone branch (`type: project_milestone`)
- [ ] `Build Slack Message` output contains a clean `notification_text`, not raw JSON

**Test 2 — Podcast episode**

```bash
curl -X POST http://localhost:5678/webhook/lux-linkedin-post \
  -H "Content-Type: application/json" \
  -d @docs/mvps/mvp-c/payload_podcast.json
```

Verify:
- [ ] Post tone and content differs from the milestone version
- [ ] IF node routed to the podcast branch (any `type` other than `project_milestone`)
- [ ] Slack notification copy is different — references the episode, not a build milestone
- [ ] Slack does not show a `json { ... }` blob

**Test mode (no activation needed):**

```bash
curl -X POST http://localhost:5678/webhook-test/lux-linkedin-post \
  -H "Content-Type: application/json" \
  -d @docs/mvps/mvp-c/payload_milestone.json
```

Only responds while the workflow editor is open in test mode.

### Execution Inspection Checklist

After each test, open the run in **Executions** and inspect these nodes:

| Node | What to confirm |
| --- | --- |
| `Load Prompts` | `prompts.system_brand_voice`, `prompts.user_milestone`, `prompts.user_podcast`, `prompts.user_slack_notify` are populated |
| `Route by Type` | The correct branch fired based on `type` |
| `Extract Post` | Output includes `post`, `content_type`, `about`, and `post_preview` |
| `Claude: Slack Notification Copy` | `content[0].text` may be plain JSON, fenced JSON, or prefixed text; all are acceptable |
| `Build Slack Message` | `notification_text` is human-readable Slack copy, not a raw JSON object |
| `Slack: Post to #content-review` | Node succeeds and posts to the expected channel |

---

## 6. Runtime Prompt Loading

The exported workflow now includes a `Load Prompts` Code node that reads the files from:

```text
/workspace/groupscout/docs/mvps/mvp-c/prompts/
```

For Docker Compose, make sure the `n8n` service has both of these:

- a read-only repo mount: `./:/workspace/groupscout:ro`
- Code node builtin access for `fs`: `NODE_FUNCTION_ALLOW_BUILTIN=fs`

After changing `docker-compose.yml`, restart n8n:

```bash
docker compose up -d n8n
```

---

## 7. Bruno API Collection (Alternative to curl)

The `api/bruno/lux-mvp-c/` folder in the GroupScout repo contains pre-built Bruno requests for both input types.

| Request | File | Payload |
| --- | --- | --- |
| Milestone Post | `lux-mvp-c/Milestone Post.bru` | `docs/mvps/mvp-c/payload_milestone.json` |
| Podcast Episode Post | `lux-mvp-c/Podcast Episode Post.bru` | `docs/mvps/mvp-c/payload_podcast.json` |

**Setup:**
1. Open the `api/bruno/` collection in Bruno
2. Select the **Local** environment (`n8n_url` is pre-set to `http://localhost:5678`)
3. Run either request — no auth required

To test without activating the workflow, change the URL from `/webhook/` to `/webhook-test/` (n8n editor must be open).

---

## Architecture Notes

- MVP-C makes two Anthropic API calls per trigger in the current export: one post generation call (`claude-opus-4-6`) and one Slack copy call (`claude-haiku-4-5-20251001`).
- The IF node routes on `$json.type`. Any value other than `project_milestone` goes to the podcast branch.
- The Merge node uses `append` — only one Claude branch runs for a given payload, so the merge simply passes through whichever branch produced output.
- The `Extract Post` and `Build Slack Message` Code nodes both use tolerant JSON extraction so fenced JSON and prefixed `json` responses do not break Slack output.

---

## File Reference

```
docs/mvps/mvp-c/
├── payload_milestone.json     ← test payload: project milestone
├── payload_podcast.json       ← test payload: podcast episode
├── prompts/
│   ├── system_brand_voice.txt
│   ├── user_milestone.txt
│   ├── user_podcast.txt
│   └── user_slack_notify.txt
└── n8n_workflow.json          ← importable workflow
```
