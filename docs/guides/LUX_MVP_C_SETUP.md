# MVP-C Setup — LUX LinkedIn Post Pipeline

This guide walks through configuring the n8n workflow that powers MVP-C: a two-input (project milestone / podcast episode) LinkedIn post generator that delivers draft copy to Slack for review.

The workflow calls the Anthropic API directly from n8n. It does not require the GroupScout Go server, Postgres, or Ollama — only n8n needs to be running.

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
| Slack webhook | Incoming webhook URL for `#content-review` |
| Slack Bot API token | Only needed if you add the Buffer scheduling step (optional) |

---

## 1. Import the Workflow

1. Open your n8n instance (`http://localhost:5678` or your hosted URL).
2. Go to **Workflows** > **Import from file**.
3. Select `docs/mvps/mvp-c/n8n_workflow.json`.
4. Click **Save**.

---

## 2. Configure Credentials

### Anthropic API (Header Auth)

1. Go to **Credentials** > **Add Credential** > search **Header Auth**.
2. Name: `Anthropic API`
3. Header Name: `x-api-key`
4. Value: your Anthropic API key (`sk-ant-...`)
5. Save.

Apply this credential to both Claude HTTP Request nodes:
- **Claude: Milestone Post**
- **Claude: Podcast Post**
- **Claude: Slack Notification Copy**

### Slack

1. Go to **Credentials** > **Add Credential** > search **Slack**.
2. Choose **Incoming Webhook** mode.
3. Paste your `#content-review` webhook URL.
4. Save and apply to the **Slack: Post to #content-review** node.

---

## 3. Set Environment Variables in n8n

If your n8n runs via Docker, add these to the `environment:` section of your `docker-compose.yml` (or the n8n `.env`):

```env
ANTHROPIC_API_KEY=sk-ant-...
```

The workflow references `$env.ANTHROPIC_API_KEY` in the HTTP Request nodes. Alternatively, hardcode the key directly into the credential (not recommended for shared instances).

---

## 4. Load Prompts

The workflow sends prompts inline from the HTTP Request node bodies. The source-of-truth prompt files live at:

```
docs/mvps/mvp-c/prompts/
├── system_brand_voice.txt   ← system parameter on every Claude call
├── user_milestone.txt       ← user message for project_milestone type
├── user_podcast.txt         ← user message for podcast_episode type
└── user_slack_notify.txt    ← user message for Slack copy (optional Prompt 3)
```

If you edit a prompt, update the corresponding HTTP Request node body in n8n to match. Use **Code** or **Set** nodes upstream to load prompt text dynamically if you want the workflow to read from files at runtime (see [Dynamic Prompt Loading](#dynamic-prompt-loading) below).

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

**Test mode (no activation needed):**

```bash
curl -X POST http://localhost:5678/webhook-test/lux-linkedin-post \
  -H "Content-Type: application/json" \
  -d @docs/mvps/mvp-c/payload_milestone.json
```

Only responds while the workflow editor is open in test mode.

---

## 6. Dynamic Prompt Loading (Optional)

To avoid copying prompt text into n8n and keep `docs/mvps/mvp-c/prompts/` as the single source of truth, add a **Read Binary File** node before each Claude HTTP Request node, reading the `.txt` file from disk. Then reference `$binary.data.toString()` in the request body.

This requires n8n to have read access to the groupscout repo directory. Set the file path to an absolute path or mount the directory via Docker volume.

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

- MVP-C makes three Anthropic API calls per trigger: one post generation call (`claude-opus-4-6`) and one Slack copy call (`claude-haiku-4-5-20251001`). The third call only fires if you enable Prompt 3.
- The IF node routes on `$json.type`. Any value other than `project_milestone` goes to the podcast branch.
- The Merge node uses `mergeByPosition` — both Claude branches produce one item, so merge order is deterministic.
- The Extract Post Code node pulls `content[0].text` from the Anthropic response and parses the JSON `post` field.

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
