# MVP-C User Guide — LUX LinkedIn Post Pipeline

How to trigger the pipeline, what to send, and what you'll get back in Slack.

---

## How It Works

Send a JSON payload to the webhook. The pipeline calls Claude, generates a LinkedIn post draft in LUX's voice, and posts it to `#content-review` for a human to approve before anything goes live.

Nothing is auto-posted. The Slack message has three action buttons: **Post to LinkedIn**, **Schedule via Buffer**, or **Edit First**.

---

## Triggering the Pipeline

**Webhook URL:** copy from the Webhook node in n8n after activating the workflow.

```
POST https://your-n8n/webhook/lux-linkedin-post
Content-Type: application/json
```

---

## Input Payloads

### Project Milestone

Use this when a project phase is complete or a notable jobsite event happened.

```json
{
  "type": "project_milestone",
  "project_name": "Hartwell Residence",
  "milestone": "Framing complete",
  "detail": "Wrapped framing on a 4,200 sqft custom home in 11 days despite a 3-day rain delay.",
  "team": ["Mike (lead framer)", "Delta Framing Co."],
  "next_phase": "Rough-in electrical and plumbing"
}
```

| Field | Required | Notes |
|---|---|---|
| `type` | Yes | Must be `project_milestone` |
| `project_name` | No | Claude won't use it unless it adds value; client privacy is the default |
| `milestone` | Yes | The phase or event name |
| `detail` | Yes | The concrete fact — numbers, durations, challenges. This is what the hook is built from |
| `team` | No | Credit where earned. Omit if nothing worth calling out |
| `next_phase` | No | Only include if it adds tension or momentum |

---

### Podcast Episode

Use this when a new Built Different episode drops.

```json
{
  "type": "podcast_episode",
  "show": "Built Different",
  "episode": 47,
  "guest": "Jordan Hale",
  "guest_title": "Owner, Hale Custom Builds",
  "topic": "How to price jobs without leaving money on the table",
  "key_takeaways": [
    "Stop quoting from gut feel — build a real cost model",
    "Clients who push hardest on price are often the hardest to work with",
    "Your overhead isn't a fixed number"
  ]
}
```

| Field | Required | Notes |
|---|---|---|
| `type` | Yes | Must be `podcast_episode` |
| `show` | Yes | Show name, e.g. `Built Different` |
| `episode` | Yes | Episode number |
| `guest` | Yes | Guest name — appears once naturally in the post |
| `guest_title` | Yes | Guest's role or company |
| `topic` | Yes | The episode topic in plain English |
| `key_takeaways` | Yes | 2–4 points. Claude picks the most scroll-stopping one — not all of them |

---

## What You Get in Slack

A message lands in `#content-review` with:

1. **Notification copy** — a casual 2–3 sentence note from Claude describing the draft and asking who will review it.
2. **The post** — the full LinkedIn draft, formatted with line breaks.
3. **Action buttons:**
   - **Post to LinkedIn** — marks approved (Buffer integration required to auto-post)
   - **Schedule via Buffer** — routes to Buffer for scheduled publish
   - **Edit First** — flags the draft for manual editing before posting

The post will not go anywhere until someone interacts with those buttons.

---

## Expected Output

### Project milestone post

```
Framing done. 4,200 sqft. 11 days. Through a 3-day rain delay.

That's what happens when you've got the right crew and a plan built to flex.

Next up: rough-in electrical and plumbing. The walls are going in — now the real coordination begins.

#LUXBuilds #CustomHome #BuiltDifferent
```

### Podcast episode post

```
Most contractors quote from gut feel.

Jordan Hale says that's why most contractors leave 15–20% on the table.

Episode 47 of Built Different gets into the real math behind pricing a job — and why the clients who push hardest on price are often the hardest to work with.

Worth a listen. Link in bio.

#BuiltDifferent #ConstructionBusiness #Podcast
```

---

## LUX Voice Rules (What Claude Enforces)

The system prompt hard-codes these. If you see any of the following in a generated draft, re-run or edit manually:

- "Excited to share" / "Thrilled to announce" / "Proud to present" — banned
- Em dashes used as connectors — banned
- Passive voice — banned
- More than 3 hashtags — banned
- Generic tags like `#Construction`, `#Business`, `#Entrepreneur` — banned
- Missing `#BuiltDifferent` — always required

---

## Tips

- **Be specific in `detail` and `key_takeaways`.** Claude builds the hook from the most concrete thing in the payload. Vague input produces vague hooks.
- **Don't pad `team` or `next_phase`.** If the next phase is boring, leave it out. Claude is instructed to skip it if it adds nothing.
- **For podcast posts:** lead your `key_takeaways` with the most uncomfortable or counterintuitive point. That's what Claude will amplify.
- **Re-triggering:** the pipeline has no dedup logic. Sending the same payload twice produces two Slack messages. This is intentional — use it to A/B prompt variations.
