### n8n Integration Guide for GroupScout

This guide provides comprehensive instructions for integrating n8n with GroupScout. It covers triggering the internal pipeline, pushing external leads, and managing automated digests.

---

### 1. Prerequisites

Before you begin, ensure you have the following:

- **GroupScout Server**: Running and accessible (via `go run cmd/server/main.go` or Docker).
- **API Token**: A user-defined secret for authentication.
    - **How to generate**: Run `openssl rand -hex 32` or use the Go tool: `go run -e "import 'crypto/rand'; import 'encoding/hex'; func main() { b := make([]byte, 32); rand.Read(b); println(hex.EncodeToString(b)) }"`
    - **Set it**: In your `.env` file: `API_TOKEN=your_generated_token`.
- **n8n Instance**:
    - **Self-Hosted**: Use the provided `docker-compose.yml`. n8n will be at `http://localhost:5678`.
    - **Cloud/External**: Ensure it can reach your server's IP. If GroupScout is in Docker, use `http://host.docker.internal:8080` (Mac/Win) or the container's IP.

---

### 2. Setting up Authentication in n8n

To simplify your workflows, create a "Header Auth" credential in n8n:

1.  In n8n, go to **Credentials** > **Add Credential**.
2.  Search for **Header Auth**.
3.  **Name**: `GroupScout API`.
4.  **Header Name**: `Authorization`.
5.  **Value**: `Bearer <YOUR_API_TOKEN>` (Replace `<YOUR_API_TOKEN>` with the token from your `.env`).

---

### 3. Triggering the Full Pipeline (`/run`)

Use this to start the GroupScout collection, enrichment, and notification process.

#### Node: **HTTP Request**
- **Method**: `POST`
- **URL**: `http://<groupscout-host>:8080/run`
- **Authentication**: `Predefined Credential` (Select your `GroupScout API` credential).
- **Body Parameters** (Optional):
    - **Mode**: `JSON`
    - **Property**: `bcbid_raw_input`
    - **Value**: `<Raw text for manual BC Bid processing>`

---

### 4. Pushing External Leads (`/n8n/webhook`)

Use this to send leads from other n8n-connected sources (RSS, Web Scrapers, Google Sheets) into GroupScout.

#### Node: **HTTP Request**
- **Method**: `POST`
- **URL**: `http://<groupscout-host>:8080/n8n/webhook`
- **Authentication**: `Predefined Credential`.
- **Body Mode**: `JSON`
- **JSON Payload Example**:
  ```json
  {
    "Title": "New Tech Hub Construction",
    "Location": "Surrey, BC",
    "Source": "Custom RSS Scraper",
    "ProjectValue": 25000000,
    "ProjectType": "Commercial",
    "PriorityScore": 90,
    "PriorityReason": "High value project near transit.",
    "SourceURL": "https://example.com/project",
    "GeneralContractor": "Major Build Inc.",
    "EstimatedCrewSize": 50,
    "OutOfTownCrewLikely": false
  }
  ```

#### Detailed Schema for `/n8n/webhook`:
| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `Title` | String | **Yes** | Name of the project or lead. |
| `Location` | String | No | City, region, or specific address. |
| `Source` | String | No | Source name (defaults to "n8n"). |
| `ProjectValue` | Number | No | Estimated total project value (Integer/Long). |
| `ProjectType` | String | No | e.g., "Commercial", "Hotel", "Residential". |
| `PriorityScore`| Number | No | Relevance score (0-100). |
| `PriorityReason`| String | No | Brief explanation for the score. |
| `SourceURL` | String | No | Direct link to the source document/page. |
| `GeneralContractor`| String | No | Name of the GC. |
| `Applicant` | String | No | Permit applicant name/contact. |
| `Contractor` | String | No | Trade contractor name/contact. |
| `EstimatedCrewSize` | Number | No | Estimated workers needed. |
| `EstimatedDurationMonths` | Number | No | Estimated project length. |
| `OutOfTownCrewLikely` | Boolean | No | `true` if crew likely from outside local area. |
| `SuggestedOutreachTiming` | String | No | e.g., "Immediate", "In 3 months". |
| `Notes` | String | No | Any additional context. |

---

### 5. Automated Weekly Digest (`/digest`)

Trigger a summary email of all "new" or "notified" leads from the last 7 days.

#### Node: **HTTP Request**
- **Method**: `POST`
- **URL**: `http://<groupscout-host>:8080/digest?to=sales@yourcompany.com`
- **Authentication**: `Predefined Credential`.

---

### 6. Scheduling with n8n

You can use the **Schedule** node in n8n to automate GroupScout runs at specific times.

#### Example: Run every Monday and Wednesday at 9:00 AM

1.  Add a **Schedule** node to your workflow.
2.  Set **Trigger Interval** to `Weeks`.
3.  **Days of the Week**: Select `Monday` and `Wednesday`.
4.  **Time**: Set to `09:00`.
5.  Connect this node to an **HTTP Request** node configured for the `/run` endpoint (as described in Section 3).

---

### 7. Troubleshooting & Tips

#### Common Errors
- **401 Unauthorized**: Check your `API_TOKEN` in `.env` matches the `Bearer` token in n8n.
- **400 Bad Request**: Your JSON body is invalid or missing the `Title` field.
- **Connection Refused**: 
    - If n8n is in Docker and GroupScout is on the host, use `http://host.docker.internal:8080`.
    - Check if the GroupScout server is actually running (`go run cmd/server/main.go`).

#### Advanced Workflow Example
1.  **RSS Read**: Check for new construction news.
2.  **AI/LLM (n8n node)**: Summarize the article into structured JSON.
3.  **GroupScout Webhook**: Send the structured JSON to GroupScout.
4.  **Slack (GroupScout Internal)**: GroupScout automatically notifies your team.

---

### 8. Docker Network Note
If you are using the provided `docker-compose.yml`, both services share the same network. You can reach GroupScout from n8n using:
- **URL**: `http://groupscout:8080/run` (or `/n8n/webhook`, `/digest`)

---

### 9. n8n Operations Reference

This section covers day-to-day n8n operation across all workflows running in this instance — GroupScout pipeline triggers, MVP-B LUX Lead Follow-Up Sequence, and the MVP-C LUX LinkedIn Post Pipeline.

#### Starting and Stopping n8n

```bash
# Start n8n (and GroupScout) via Docker Compose
docker compose up -d

# Stop all services
docker compose down

# Restart n8n only (e.g. after env var changes)
docker compose restart n8n

# Tail n8n logs
docker compose logs -f n8n
```

n8n UI is at `http://localhost:5678` once the container is running.

#### Workflow States

| State | Meaning |
|---|---|
| **Active** | Webhook and Schedule triggers are live. Executions fire automatically. |
| **Inactive** | Triggers are disabled. Workflow can still be run manually from the UI. |

Toggle a workflow's state with the switch in the top-right of the workflow editor, or from the **Workflows** list view.

Always activate a workflow after import. A workflow that was imported but left inactive will not respond to webhook calls.

#### Manual Test Runs

To test a workflow without hitting a live webhook:

1. Open the workflow in the editor.
2. Click **Test workflow** (top-right).
3. n8n waits for a trigger event — send a test payload using Bruno or curl:
   ```bash
   # GroupScout pipeline
   curl -X POST http://localhost:8080/run \
     -H "Authorization: Bearer $API_TOKEN"

   # MVP-B LUX Lead Follow-Up Sequence
   curl -X POST https://your-n8n/webhook-test/lux-lead-followup \
     -H "Content-Type: application/json" \
     -d @docs/mvps/mvp-b/payload.json

   # MVP-C LUX LinkedIn Post Pipeline
   curl -X POST https://your-n8n/webhook-test/lux-linkedin-post \
     -H "Content-Type: application/json" \
     -d @docs/mvps/mvp-c/payload_milestone.json
   ```
4. Execution result appears in the editor inline, node by node.

The `/webhook-test/` path is the test URL — it only responds while the editor is open in test mode. The production URL uses `/webhook/`.

#### Monitoring Executions

Go to **Executions** in the left sidebar to see all past runs across all workflows.

Key columns:
- **Status** — `Success`, `Error`, `Waiting`
- **Workflow** — which workflow triggered
- **Started** — timestamp
- **Duration** — how long it took

Click any execution to see the per-node input/output and error details. This is the primary debugging surface for failed Claude API calls or Slack delivery issues.

#### Retrying Failed Executions

From the **Executions** list:
1. Find the failed run.
2. Click the three-dot menu on the right.
3. Select **Retry**.

This replays the original trigger data — useful when a run fails due to a transient API error (Anthropic 529, Slack timeout) rather than bad input.

#### Credential Management

All credentials are stored encrypted in n8n's internal database. To view or update:

1. Go to **Credentials** in the left sidebar.
2. Click a credential to edit it.
3. Save — the change applies immediately to all workflows using that credential.

Credentials in use across this instance:

| Credential | Used By |
| --- | --- |
| `GroupScout API` (Header Auth) | GroupScout `/run`, `/n8n/webhook`, `/digest` |
| `Anthropic API` (Header Auth) | MVP-B: Classify Lead, Generate Commercial/Residential Sequence, Generate Slack Copy; MVP-C: all three Claude HTTP Request nodes |
| `Airtable` (Personal Access Token) | MVP-B: Create Lead Record node |
| `Slack — #new-leads` (Bot Token) | MVP-B: Post to #new-leads node |
| `Slack — #content-review` (Incoming Webhook) | MVP-C: Slack delivery node |
| `Slack — GroupScout` (Incoming Webhook) | GroupScout internal Slack notifications |

#### Environment Variables

n8n reads env vars at container start. To add or change a variable:

1. Update `docker-compose.yml` under `environment:` for the `n8n` service.
2. Run `docker compose up -d --force-recreate n8n` to apply.

Variables currently used by workflows:

```env
# GroupScout
API_TOKEN=...

# MVP-B (LUX Lead Follow-Up Sequence)
ANTHROPIC_API_KEY=sk-ant-...
AIRTABLE_BASE_ID=appXXXXXXXXXXXXXX

# MVP-C (LUX LinkedIn Post Pipeline)
ANTHROPIC_API_KEY=sk-ant-...
```

---

### 10. Workflows in This Instance

This n8n instance runs two distinct workflow families:

#### GroupScout — Lead Intelligence Pipeline

Triggers the groupscout Go server to collect, enrich, and deliver construction leads to the Sandman Hotel sales team.

| Workflow | Trigger | Purpose |
| --- | --- | --- |
| GroupScout Weekly Run | Schedule (Wed + Sun, 8am) | Full collector → enrichment → Slack digest |
| GroupScout Manual Push | Webhook POST `/n8n/webhook` | Inject external leads from RSS or scrapers |
| GroupScout Digest | Webhook POST `/digest` | Send weekly email summary |

See sections 3–6 above for node configuration details.

#### MVP-A — LUX Client Status Email Generator

Accepts a JobTread project payload, generates a professional client-facing status email via Claude, and posts the full draft to `#client-updates-review` for PM review and approval before anything reaches the client.

| Workflow | Trigger | Purpose |
| --- | --- | --- |
| LUX Client Status Email | Webhook POST `/webhook/lux-status-email` | Generate email → Slack PM review → Approve/Edit buttons |

Workflow file: `docs/mvps/mvp-a/n8n_workflow.json`

Full setup and user guide:
- `docs/guides/LUX_MVP_A_SETUP.md`
- `docs/guides/LUX_MVP_A_USER_GUIDE.md`
- `docs/guides/LUX_MVP_A_TROUBLESHOOTING.md`

#### MVP-B — LUX Lead Follow-Up Sequence Generator

Accepts an inbound lead payload, classifies it with Claude (tier, category, key detail), routes to a commercial or residential sequence generator, stages three email drafts in Airtable, and notifies `#new-leads` in Slack. Nothing sends automatically — Airtable is the staging area, humans send.

| Workflow | Trigger | Purpose |
| --- | --- | --- |
| LUX Lead Follow-Up Sequence | Webhook POST `/webhook/lux-lead-followup` | Classify → generate 3-email sequence → Airtable → Slack notification |

Workflow file: `docs/mvps/mvp-b/n8n_workflow.json`

Full setup and user guide:
- `docs/guides/LUX_MVP_B_SETUP.md`
- `docs/guides/LUX_MVP_B_USER_GUIDE.md`
- `docs/guides/LUX_MVP_B_TROUBLESHOOTING.md`

#### MVP-C — LUX LinkedIn Post Pipeline

Generates LinkedIn post drafts from project milestone or podcast episode data and delivers them to `#content-review` for human review.

| Workflow | Trigger | Purpose |
| --- | --- | --- |
| LUX LinkedIn Post Pipeline | Webhook POST `/webhook/lux-linkedin-post` | Generate post → Slack draft → action buttons |

Workflow file: `docs/mvps/mvp-c/n8n_workflow.json`

Full setup and user guide:
- `docs/guides/LUX_MVP_C_SETUP.md`
- `docs/guides/LUX_MVP_C_USER_GUIDE.md`
- `docs/guides/LUX_MVP_C_TROUBLESHOOTING.md`

The workflow families are independent — they share credentials storage and the Slack workspace but do not share data or trigger each other.

---

### 11. Slack Integration

This instance uses Slack for three distinct notification purposes. Each uses a different credential type and posts to a different channel.

#### Slack Channels and Credential Types

| Channel | Workflow | Credential Type | Purpose |
| --- | --- | --- | --- |
| `#new-leads` | MVP-B | Bot Token (OAuth2) | Lead arrival notification with Airtable link |
| `#client-updates-review` | MVP-A | Bot Token (OAuth2) | Email draft with Approve/Edit action buttons |
| `#content-review` | MVP-C | Incoming Webhook | LinkedIn post draft for review |
| GroupScout channel | GroupScout | Incoming Webhook | Weekly lead digest |

Bot Tokens support interactive components (buttons, actions). Incoming Webhooks are simpler but cannot receive interactions.

---

#### Creating a Slack App (Bot Token — MVP-A and MVP-B)

MVP-A and MVP-B use a Slack Bot Token because their messages include action buttons (Approve/Edit, View in Airtable).

1. Go to [api.slack.com/apps](https://api.slack.com/apps) → **Create New App** → **From scratch**
2. Name: `LUX n8n Bot` — Workspace: your Slack workspace
3. Go to **OAuth & Permissions** → **Bot Token Scopes** → Add:
   - `chat:write` — post messages to channels
   - `chat:write.public` — post to public channels without joining (optional)
4. Go to **Install App** → **Install to Workspace** → Authorize
5. Copy the **Bot User OAuth Token** (`xoxb-...`) — this is the value you paste into n8n

**Invite the bot to each channel it needs to post to:**
```
/invite @lux-n8n-bot
```
Run this in both `#new-leads` and `#client-updates-review`.

**In n8n:**
1. Go to **Credentials → New → Slack API**
2. Name: `Slack — LUX Bot`
3. Paste the `xoxb-...` token
4. Assign to the **Post to #new-leads** node (MVP-B) and **Post to Slack** node (MVP-A)

---

#### Creating an Incoming Webhook (MVP-C and GroupScout)

MVP-C and GroupScout use Incoming Webhooks — simpler to set up, no bot required, no interactive components.

1. Go to [api.slack.com/apps](https://api.slack.com/apps) → select or create an app
2. Go to **Incoming Webhooks** → toggle **Activate Incoming Webhooks** on
3. Click **Add New Webhook to Workspace** → select the target channel → **Allow**
4. Copy the webhook URL (`https://hooks.slack.com/services/...`)

**In n8n:**
1. Go to **Credentials → New → Slack Incoming Webhook**
2. Name: `Slack — #content-review` (or `Slack — GroupScout`)
3. Paste the webhook URL
4. Assign to the relevant Slack node

---

#### Testing Slack Connectivity

**Test a Bot Token credential:**
```bash
curl -X POST https://slack.com/api/chat.postMessage \
  -H "Authorization: Bearer xoxb-your-token" \
  -H "Content-Type: application/json" \
  -d '{"channel": "#new-leads", "text": "n8n Slack test — MVP-B"}'
```
Expected response: `{"ok":true,...}`

**Test an Incoming Webhook:**
```bash
curl -X POST https://hooks.slack.com/services/YOUR/WEBHOOK/URL \
  -H "Content-Type: application/json" \
  -d '{"text": "n8n Slack test — GroupScout"}'
```
Expected response: `ok`

---

#### Common Slack Errors

| Error | Cause | Fix |
| --- | --- | --- |
| `channel_not_found` | Bot not invited to channel | Run `/invite @your-bot` in the channel |
| `invalid_auth` | Token expired or revoked | Reinstall the Slack app, get a new `xoxb-` token |
| `not_in_channel` | Same as `channel_not_found` | Same fix |
| `no_service` | Incoming webhook deleted or disabled | Recreate the webhook in the Slack app settings |
| `missing_scope` | Bot token missing `chat:write` | Add scope in OAuth & Permissions, reinstall app |

---

#### Channel ID vs Channel Name

n8n's Slack node accepts both channel names (`#new-leads`) and channel IDs (`C0123ABCDEF`). Channel IDs are more reliable — if a channel is renamed, the name-based reference breaks but the ID still works.

To find a channel ID: right-click the channel in Slack → **View channel details** → scroll to the bottom of the About tab — the ID is listed there, or visible in the channel URL.

Update the `channelId` field in the relevant Slack node if you switch to using IDs.
