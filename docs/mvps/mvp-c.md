# MVP C — LinkedIn & Podcast Post Pipeline

## What It Does

Accepts a project milestone or podcast episode payload, calls Claude to generate a branded LinkedIn post in LUX's voice, and delivers the draft to Slack for review — optionally scheduling via Buffer.

---

## Problem It Solves

LUX has two consistent content streams — project wins and the *Built Different* podcast — but turning them into LinkedIn posts requires time and a consistent voice. This pipeline converts raw project or episode data into ready-to-post content in seconds, keeping the brand active without adding manual work.

---

## Input

Two supported input types:

**Project milestone:**
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

**Podcast episode:**
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

---

## Prompts

The pipeline uses three prompts in sequence. Each is a discrete n8n HTTP Request node so they can be tuned, swapped, or A/B tested independently.

---

### Prompt 1 — Brand Voice System Prompt (applied to every call as `system`)

This is the persistent identity layer. It never changes regardless of content type. Set it as the `system` parameter in every Anthropic API call.

```
You are the voice of LUX, a high-end construction company based in [city].
LUX builds custom homes and commercial projects for discerning clients,
and runs a business podcast called Built Different aimed at contractors
and construction business owners.

The LUX LinkedIn voice has these non-negotiable characteristics:

TONE
- Confident without being arrogant. LUX earns respect through results, not claims.
- Blue-collar roots, executive presence. The people writing this have swung hammers
  and sat in boardrooms. The writing reflects both.
- Direct. Say the thing. No throat-clearing, no hedging, no corporate softening.

WHAT NEVER APPEARS IN A LUX POST
- "Excited to share" / "Thrilled to announce" / "Proud to present"
- Em dashes (—) used as sentence connectors
- Passive voice
- Vague claims ("great work", "amazing team", "incredible results") without
  a specific fact behind them
- Hashtag spam — maximum 3 hashtags, always relevant, always lowercase except
  the first letter of each word

STRUCTURE
- Line 1 is always the hook. It earns the scroll-stop.
- Paragraphs are 1–3 sentences max. One blank line between each.
- The post ends with a single punchy line before the hashtags.
- Hashtags go on their own line at the very end.
- Maximum 150 words total including hashtags.

HASHTAG RULES
- Always include #BuiltDifferent
- Add 1–2 relevant tags based on content type (project type, trade, topic)
- Never use generic tags like #Construction #Business #Entrepreneur
```

---

### Prompt 2A — Project Milestone Post (user message)

Used when `input.type === "project_milestone"`. Sent as the `user` message alongside the system prompt above.

```
Write a LinkedIn post for LUX about a project milestone.

WHAT MAKES A GREAT MILESTONE POST
- Lead with the most concrete fact in the data: a number, a duration, a size,
  a challenge overcome. Not the milestone name — the fact behind it.
- The second paragraph gives brief context or credit where it's earned.
- The third paragraph (optional) teases what's next — but only if it adds tension
  or momentum. Skip it if it's boring.
- Never explain what framing is, what a foundation pour is, or any other trade
  term. The LUX audience knows construction.
- Do not use the project name in the post unless it adds something. Client privacy
  is default. Location is fine if it matters.

HOOKS THAT WORK (use as inspiration, not templates)
- "[Number] sqft. [X] days. [One obstacle]."
- "The [phase] is done. Here's what it took."
- "[Specific challenge]. [How it was handled in one sentence]."

HOOKS THAT DON'T WORK
- "We're proud to share that..."
- "Another milestone reached at LUX."
- "Big news from the jobsite."

Return a JSON object with exactly one key: "post".
The value is the full post text with \n for line breaks.
Return JSON only. No preamble, no markdown fences, no explanation.

Milestone data:
{milestone_json}
```

---

### Prompt 2B — Podcast Episode Post (user message)

Used when `input.type === "podcast_episode"`. Sent as the `user` message alongside the system prompt above.

```
Write a LinkedIn post for LUX promoting a new episode of Built Different,
their podcast for contractors and construction business owners.

WHAT MAKES A GREAT PODCAST POST
- Lead with the most counterintuitive, uncomfortable, or surprising thing
  from the episode. Not the topic — the tension inside the topic.
- Never say "we talked about X" or "in this episode". Show don't tell.
- The guest's name and title appear once, naturally, not in an intro sentence.
- The call to action is always "Link in bio." — never a URL, never "click here",
  never "tune in".
- Do not list all the takeaways. Pick the one that will make a contractor stop
  scrolling and think "wait, that's me."

HOOKS THAT WORK (use as inspiration, not templates)
- "[Counterintuitive claim]. [Who said it and why it matters in one sentence]."
- "Most [audience] [do the wrong thing]. [Guest name] explains why."
- "[Specific number or outcome]. [The insight behind it in one sentence]."

HOOKS THAT DON'T WORK
- "New episode alert!"
- "We sat down with [guest] to discuss..."
- "Check out our latest episode of Built Different!"

TONE NOTE
Built Different is peer-to-peer. LUX is talking to other contractors, not
to clients. The voice can be slightly more blunt and insider here than on
project milestone posts.

Return a JSON object with exactly one key: "post".
The value is the full post text with \n for line breaks.
Return JSON only. No preamble, no markdown fences, no explanation.

Episode data:
{episode_json}
```

---

### Prompt 3 — Slack Notification Copy (optional, run after post is generated)

If you want Claude to also write the Slack message contextually (not just template it), run this as a third lightweight call. Keeps the Slack message feeling human rather than robotic.

```
Write a brief internal Slack message notifying the LUX team that a LinkedIn
post draft is ready for review.

Rules:
- 2–3 sentences max
- Include: content type, what it's about, and a note that it needs review
  before posting
- Tone is casual internal — this is a team message, not a client communication
- Do not repeat the full post text
- End with a single line asking who will review it

Return a JSON object with one key: "slack_message".
Return JSON only. No preamble, no markdown fences.

Context:
{post_context_json}
```

Where `post_context_json` is a small object passed from the previous node:

```json
{
  "content_type": "podcast_episode",
  "about": "Episode 47 with Jordan Hale on job pricing",
  "post_preview": "Most contractors quote from gut feel..."
}
```

---

### Prompt Selection Logic in n8n

Use an **IF node** after the Webhook node to route to the correct user prompt:

```
Condition: {{ $json.type }} === "project_milestone"
  → True branch: HTTP Request node with Prompt 2A
  → False branch: HTTP Request node with Prompt 2B
```

Both branches then merge into the same JSON Parse → Slack nodes downstream.

---

## Output

**Project milestone post:**

```json
{
  "post": "Framing done. 4,200 sqft. 11 days. Through a 3-day rain delay.\n\nThat's what happens when you've got the right crew and a plan built to flex.\n\nNext up: rough-in electrical and plumbing. The walls are going in — now the real coordination begins.\n\nProud of the team on this one.\n\n#LUXBuilds #CustomHome #BuiltDifferent"
}
```

**Podcast episode post:**

```json
{
  "post": "Most contractors quote from gut feel.\n\nJordan Hale says that's why most contractors leave 15–20% on the table.\n\nEpisode 47 of Built Different gets into the real math behind pricing a job — and why the clients who push hardest on price are often the hardest to work with.\n\nWorth a listen. Link in bio.\n\n#BuiltDifferent #ConstructionBusiness #Podcast"
}
```

---

## Delivery

Slack message to `#content-review`:

```
📝 *LinkedIn Post Ready for Review*
*Type:* Project Milestone — Hartwell Residence
*Milestone:* Framing complete

---
Framing done. 4,200 sqft. 11 days. Through a 3-day rain delay.
...
---

✅ Post to LinkedIn   📅 Schedule via Buffer   ✏️ Edit First
```

---

## Build Path

**Tool:** n8n

| Step | Node | Config |
|---|---|---|
| 1 | Webhook | POST trigger, accepts milestone or episode JSON |
| 2 | IF | Route on `type` field: `project_milestone` → 3A, else → 3B |
| 3A | HTTP Request | Anthropic API — system: Prompt 1, user: Prompt 2A (milestone) |
| 3B | HTTP Request | Anthropic API — system: Prompt 1, user: Prompt 2B (podcast) |
| 4 | Merge | Rejoin both branches |
| 5 | JSON Parse | Extract `post` field |
| 6 | HTTP Request (optional) | Anthropic API — Prompt 3 for Slack copy |
| 7 | Slack | Post draft + notification copy to `#content-review` |
| 8 (optional) | Buffer | Schedule post if approved via Slack action |

**Estimated build time:** 3 hours (without Buffer or Prompt 3) / 5 hours (full pipeline)

---

## Demo Script (Loom)

1. Show both input payloads side by side — milestone and podcast episode
2. Trigger the milestone webhook first — show the Slack draft land
3. Point out the hook: specific numbers, short sentences, no filler
4. Trigger the podcast episode payload — show the second draft land
5. Point out voice consistency across both post types: same opener style, same hashtag pattern, same sentence length
6. Optional: show the Buffer scheduling step fire

**Key talking point:** "I reverse-engineered LUX's content types — project wins and *Built Different* episodes. The prompt handles both, and the voice is consistent across both. One pipeline, two content streams."

---

## Files

```
mvp-c/
├── README.md                  ← this file
├── payload_milestone.json     ← sample project milestone payload
├── payload_podcast.json       ← sample podcast episode payload
├── prompts/
│   ├── system_brand_voice.txt ← Prompt 1, applied to every API call
│   ├── user_milestone.txt     ← Prompt 2A, project milestone posts
│   ├── user_podcast.txt       ← Prompt 2B, podcast episode posts
│   └── user_slack_notify.txt  ← Prompt 3, Slack notification copy
└── n8n_workflow.json          ← exportable n8n workflow
```