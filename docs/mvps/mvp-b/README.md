# MVP B — Automated Lead Follow-Up Sequence

## What It Does

Accepts an inbound lead payload (simulating a website contact form or CRM entry), calls Claude to generate a personalized 3-email follow-up sequence, logs all drafts to Airtable, and fires a Slack notification to the sales team.

---

## Files

```
mvp-b/
├── README.md                        <- this file
├── payload.json                     <- sample lead payload (commercial, high tier)
├── payload_alt.json                 <- sample lead payload (residential, medium tier)
├── prompts/
│   ├── system_brand_voice.txt       <- Prompt 1, applied to every API call as system
│   ├── user_classify.txt            <- Prompt 2, lead classification
│   ├── user_sequence_commercial.txt <- Prompt 3A, commercial sequence
│   ├── user_sequence_residential.txt <- Prompt 3B, residential sequence
│   └── user_slack_notify.txt        <- Prompt 4, internal Slack notification copy
└── n8n_workflow.json                <- importable n8n workflow
```

---

## Pipeline

```
Webhook (POST)
  → Code: Read Brand Voice (system prompt — Prompt 1)
  → Code: Read Classify Prompt (Prompt 2)
  → HTTP: Anthropic API (claude-haiku-4-5) → classification JSON
  → Code: Parse Classification
  → Code: Build Context (assemble lead_context_json + inject into sequence prompt)
  → IF: project_category in ["commercial_renovation", "multi_family"]
      → True:  HTTP: Anthropic API (claude-sonnet-4-6) → commercial sequence
      → False: HTTP: Anthropic API (claude-sonnet-4-6) → residential sequence
  → Merge
  → Code: Parse Sequence JSON (extract email_1, email_2, email_3)
  → Airtable: Create Lead Record (all 17 fields)
  → HTTP: Anthropic API (claude-haiku-4-5) → Slack notification copy
  → Code: Parse Slack JSON
  → Slack: Post to #new-leads (Block Kit with Airtable deep link)
  → Respond to Webhook (202 queued)
```

---

## Input Payloads

### payload.json — Commercial, high tier

Marcus Webb, Webb Commercial Properties. Clear budget ($250k–$500k), near-term timeline (Q3 2025), specific project description (two tenant spaces). Routes to Prompt 3A (commercial sequence). Classified as `high` tier.

### payload_alt.json — Residential, medium tier

Jennifer Okafor, referral, custom home. Vague timeline ("maybe next year"), budget stated but project in early stages. Routes to Prompt 3B (residential sequence). Classified as `medium` tier.

The alt payload exercises the residential path and soft-close email tone that differs significantly from the commercial sequence.

---

## Prompts

### Prompt 1 — Brand Voice (system)

`prompts/system_brand_voice.txt`

Applied as `system` to every Anthropic API call. Defines tone, forbidden patterns, personalization rules, and sign-off format. The same voice across classification, sequence generation, and Slack notification.

### Prompt 2 — Lead Classification (user)

`prompts/user_classify.txt`

Lightweight call (Haiku model) that runs before sequence generation. Returns `lead_tier`, `project_category`, `key_detail`, and `urgency_signal`. This structured output feeds Prompts 3A/3B so Claude mirrors the lead's language rather than working from raw freeform text.

### Prompt 3A — Commercial Sequence (user)

`prompts/user_sequence_commercial.txt`

Used when `project_category` is `commercial_renovation` or `multi_family`. Covers phased permitting, tenant coordination, and business continuity. Email 2 surfaces a commercial-specific insight the lead may not have considered.

### Prompt 3B — Residential Sequence (user)

`prompts/user_sequence_residential.txt`

Used when `project_category` is `custom_home` or `addition_or_remodel`. Warmer tone than commercial. Email 2 leads with a homeowner-specific consideration (design-build process, living in-home during construction, scope creep prevention).

### Prompt 4 — Slack Notification Copy (user)

`prompts/user_slack_notify.txt`

Short third call (Haiku model). Produces the internal team message. Flags high-tier leads explicitly and ends with a direct ownership question.

---

## Routing Logic

The IF node checks `project_category` against `commercial_renovation` and `multi_family` using a regex condition. All other values (`custom_home`, `addition_or_remodel`, `unknown`) route to the residential sequence.

The Code node between classification and the IF node assembles `lead_context_json` — this is the structured payload passed to Prompt 4, not the raw lead input.

---

## Airtable Schema

Table: `Leads`

| Field | Type |
|---|---|
| Name | Text |
| Company | Text |
| Project Type | Single select |
| Source | Text |
| Budget Range | Text |
| Timeline | Text |
| Original Message | Long text |
| Lead Tier | Single select (high / medium / low) |
| Key Detail | Text |
| Urgency Signal | Checkbox |
| Email 1 Subject | Text |
| Email 1 Body | Long text |
| Email 2 Subject | Text |
| Email 2 Body | Long text |
| Email 3 Subject | Text |
| Email 3 Body | Long text |
| Status | Single select (New / Contacted / Qualified / Dead) |

---

## Models Used

| Step | Model | Reason |
|---|---|---|
| Lead classification (Prompt 2) | claude-haiku-4-5-20251001 | Structured extraction, low latency |
| Sequence generation (Prompts 3A/3B) | claude-sonnet-4-6 | Quality, brand voice compliance |
| Slack notification (Prompt 4) | claude-haiku-4-5-20251001 | Short copy, no brand rules required |

---

## Build Notes

- Prompts are stored inline in Code nodes for portability; `prompts/` files are the canonical source for editing
- When editing a prompt in `prompts/`, sync the corresponding Code node in `n8n_workflow.json`
- The IF node uses a regex condition — `commercial_renovation|multi_family` — not a strict equals check; add new commercial categories here
- The Merge node is set to `passThrough` / `single` mode — it passes whichever branch completed, not both
- Human review is handled externally: Airtable gives the sales team a staging area; sequences do not auto-send

---

## Demo Script

1. Show both payloads side by side — commercial (Marcus Webb) and residential (Jennifer Okafor)
2. Trigger `payload.json`, walk each n8n node:
   - Classification output — point out `lead_tier: "high"` and the extracted `key_detail`
   - IF node routing to the commercial branch
   - Airtable record — show classification fields alongside the email drafts
   - Slack notification — note it flags this as high-tier and asks who owns it
3. Open Email 1 in Airtable — show Marcus's exact words ("two tenant spaces") in the first sentence
4. Open Email 2 — show the commercial-specific insight (phased permitting) that the residential path would never produce
5. Trigger `payload_alt.json` — show the IF node route to the residential branch, Email 2 is entirely different

**Key talking points:**
- "It classifies before it writes — the sequence is calibrated to lead quality and category, not just the name."
- "The `key_detail` extraction means Claude mirrors the lead's own language. Marcus said 'tenant spaces' — that's what shows up in Email 1."
- "Two prompts, two completely different sequences. The routing is built into the workflow, not hardcoded into one giant prompt."

---

*See `docs/guides/LUX_MVP_B_SETUP.md` for n8n import and credential setup.*
*See `docs/guides/LUX_MVP_B_USER_GUIDE.md` for day-to-day usage.*
*See `docs/guides/LUX_MVP_B_TROUBLESHOOTING.md` for debugging.*
